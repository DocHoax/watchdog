package fleet

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/DocHoax/watchdog/pkg/model"
)

func TestFleetClient_RegisterAndHeartbeat(t *testing.T) {
	var receivedAuth, receivedNodeID, receivedVersion string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		receivedNodeID = r.Header.Get("X-Watchdog-Node-ID")
		receivedVersion = r.Header.Get("X-Watchdog-Agent-Version")

		switch r.URL.Path {
		case "/api/v1/fleet/register":
			var req model.NodeRegistrationRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			resp := model.NodeRegistrationResponse{
				Registered:               true,
				NodeID:                   req.Identity.NodeID,
				HeartbeatIntervalSeconds: 15,
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		case "/api/v1/heartbeat":
			var req model.HeartbeatRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			resp := model.HeartbeatResponse{
				Acknowledged:          true,
				NodeStatus:            model.NodeStatusHealthy,
				NextHeartbeatInterval: 15,
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	client := NewFleetClient(ClientConfig{
		Endpoint: ts.URL,
		Token:    "secret-token-123",
		NodeID:   "test-node-01",
		Version:  "v1.0.0",
		Timeout:  2 * time.Second,
	})

	ctx := context.Background()

	// 1. Register
	regReq := &model.NodeRegistrationRequest{
		Identity: model.NodeIdentity{
			NodeID:   "test-node-01",
			Hostname: "test-host",
		},
	}
	regResp, err := client.Register(ctx, regReq)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if !regResp.Registered || regResp.NodeID != "test-node-01" {
		t.Errorf("Unexpected register response: %+v", regResp)
	}

	if receivedAuth != "Bearer secret-token-123" {
		t.Errorf("Expected auth header 'Bearer secret-token-123', got %s", receivedAuth)
	}
	if receivedNodeID != "test-node-01" {
		t.Errorf("Expected node ID header 'test-node-01', got %s", receivedNodeID)
	}
	if receivedVersion != "v1.0.0" {
		t.Errorf("Expected version header 'v1.0.0', got %s", receivedVersion)
	}

	// 2. Heartbeat
	hbReq := &model.HeartbeatRequest{
		NodeID:    "test-node-01",
		Status:    model.NodeStatusHealthy,
		Timestamp: time.Now().UTC(),
	}
	hbResp, err := client.SendHeartbeat(ctx, hbReq)
	if err != nil {
		t.Fatalf("SendHeartbeat failed: %v", err)
	}
	if !hbResp.Acknowledged {
		t.Errorf("Expected heartbeat acknowledged")
	}
}

func TestFleetClient_TelemetryAndBuffering(t *testing.T) {
	var failServer atomic.Bool
	failServer.Store(false)
	var telemetryCount atomic.Int32

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if failServer.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{
					"code":    "SERVICE_UNAVAILABLE",
					"message": "server maintenance",
				},
			})
			return
		}

		if r.URL.Path == "/api/v1/telemetry" {
			telemetryCount.Add(1)
			w.WriteHeader(http.StatusOK)
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	buf := NewTelemetryBuffer(10, 1024*1024)
	client := NewFleetClient(ClientConfig{
		Endpoint: ts.URL,
		Token:    "token-xyz",
		NodeID:   "test-node-01",
		Buffer:   buf,
	})

	ctx := context.Background()

	sub1 := &model.TelemetrySubmission{NodeID: "test-node-01", Timestamp: time.Now().UTC()}

	// 1. Successful telemetry submission
	if err := client.SendTelemetry(ctx, sub1); err != nil {
		t.Fatalf("SendTelemetry failed: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("Buffer should be empty on success, got %d", buf.Len())
	}
	if telemetryCount.Load() != 1 {
		t.Errorf("Expected 1 telemetry count, got %d", telemetryCount.Load())
	}

	// 2. Simulate server failure -> telemetry should be buffered
	failServer.Store(true)
	sub2 := &model.TelemetrySubmission{NodeID: "test-node-01", Timestamp: time.Now().UTC()}
	err := client.SendTelemetry(ctx, sub2)
	if err == nil {
		t.Errorf("Expected SendTelemetry to return error when server fails")
	}
	if buf.Len() != 1 {
		t.Fatalf("Expected 1 item in buffer after failure, got %d", buf.Len())
	}

	// 3. Flush buffer while server is still failing -> should fail and keep items in buffer
	sent, err := client.FlushBuffer(ctx, 10)
	if err == nil {
		t.Errorf("Expected FlushBuffer to return error when server fails")
	}
	if sent != 0 {
		t.Errorf("Expected 0 sent, got %d", sent)
	}
	if buf.Len() != 1 {
		t.Errorf("Expected item preserved in buffer, got %d", buf.Len())
	}

	// 4. Server recovers -> FlushBuffer succeeds
	failServer.Store(false)
	sent, err = client.FlushBuffer(ctx, 10)
	if err != nil {
		t.Fatalf("FlushBuffer failed after server recovery: %v", err)
	}
	if sent != 1 {
		t.Errorf("Expected 1 sent item, got %d", sent)
	}
	if buf.Len() != 0 {
		t.Errorf("Expected empty buffer after flush, got %d", buf.Len())
	}
}

func TestFleetClient_CRUDAndQueries(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/node" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(model.NodeIdentity{
				NodeID:   "local-node-id",
				Hostname: "test-host",
				OS:       "linux",
			})
		case r.URL.Path == "/api/v1/fleet/summary" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(model.FleetSummary{
				TotalNodes:   5,
				HealthyNodes: 4,
				WarningNodes: 1,
			})
		case r.URL.Path == "/api/v1/fleet" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(model.FleetListResponse{
				Total: 1,
				Count: 1,
				Nodes: []model.FleetNode{
					{
						Identity: model.NodeIdentity{
							NodeID:   "node-abc",
							Hostname: "host-abc",
						},
						Status: model.NodeStatusHealthy,
					},
				},
			})
		case r.URL.Path == "/api/v1/fleet/node-abc" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(model.NodeDetailResponse{
				Node: model.FleetNode{
					Identity: model.NodeIdentity{
						NodeID:   "node-abc",
						Hostname: "host-abc",
					},
					Status: model.NodeStatusHealthy,
				},
			})
		case r.URL.Path == "/api/v1/fleet/node-abc" && r.Method == http.MethodDelete:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"deleted": true,
				"node_id": "node-abc",
			})
		default:
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{
					"code":    "NOT_FOUND",
					"message": "not found",
				},
			})
		}
	}))
	defer ts.Close()

	client := NewFleetClient(ClientConfig{
		Endpoint: ts.URL,
		Token:    "token-123",
		NodeID:   "test-node",
	})
	ctx := context.Background()

	if client.Endpoint() != ts.URL {
		t.Errorf("Expected endpoint %s, got %s", ts.URL, client.Endpoint())
	}
	if client.NodeID() != "test-node" {
		t.Errorf("Expected node ID test-node, got %s", client.NodeID())
	}

	// 1. GetNodeIdentity
	identity, err := client.GetNodeIdentity(ctx)
	if err != nil {
		t.Fatalf("GetNodeIdentity failed: %v", err)
	}
	if identity.NodeID != "local-node-id" {
		t.Errorf("Expected node ID local-node-id, got %s", identity.NodeID)
	}

	// 2. GetSummary
	summary, err := client.GetSummary(ctx)
	if err != nil {
		t.Fatalf("GetSummary failed: %v", err)
	}
	if summary.TotalNodes != 5 || summary.HealthyNodes != 4 {
		t.Errorf("Unexpected summary: %+v", summary)
	}

	// 3. ListNodes
	listResp, err := client.ListNodes(ctx, model.FleetFilter{Status: model.NodeStatusHealthy, Limit: 10})
	if err != nil {
		t.Fatalf("ListNodes failed: %v", err)
	}
	if listResp.Total != 1 || len(listResp.Nodes) != 1 || listResp.Nodes[0].Identity.NodeID != "node-abc" {
		t.Errorf("Unexpected list response: %+v", listResp)
	}

	// 4. GetNode
	nodeDetail, err := client.GetNode(ctx, "node-abc")
	if err != nil {
		t.Fatalf("GetNode failed: %v", err)
	}
	if nodeDetail.Node.Identity.NodeID != "node-abc" {
		t.Errorf("Expected node ID node-abc, got %s", nodeDetail.Node.Identity.NodeID)
	}

	// 5. DeleteNode
	if err := client.DeleteNode(ctx, "node-abc"); err != nil {
		t.Fatalf("DeleteNode failed: %v", err)
	}

	// 6. DeleteNode error with missing ID
	if err := client.DeleteNode(ctx, ""); err == nil {
		t.Errorf("Expected error on empty node ID")
	}

	// 7. GetNode error with missing ID
	if _, err := client.GetNode(ctx, ""); err == nil {
		t.Errorf("Expected error on empty node ID")
	}

	// 8. Error on not found
	_, err = client.GetNode(ctx, "non-existent")
	if err == nil {
		t.Errorf("Expected error for non-existent node")
	}
}
