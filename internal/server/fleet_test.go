package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DocHoax/watchdog/internal/config"
	"github.com/DocHoax/watchdog/internal/storage"
	"github.com/DocHoax/watchdog/pkg/model"
)

func setupFleetTestServer(t *testing.T) (*Server, storage.Storage) {
	t.Helper()
	store, err := storage.NewSQLiteStorage(storage.Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("failed to create sqlite storage: %v", err)
	}

	cfg := &config.Config{
		Agent: config.AgentConfig{
			Enabled:     true,
			BindAddress: "127.0.0.1",
			Port:        8080,
			Token:       "test-secret-token",
		},
		Fleet: config.FleetConfig{
			Enabled:           true,
			NodeID:            "test-local-node-id",
			HeartbeatInterval: 10 * time.Second,
			TelemetryInterval: 30 * time.Second,
			StaleThreshold:    200 * time.Millisecond,
			OfflineThreshold:  500 * time.Millisecond,
			RateLimitRate:     10.0,
			RateLimitBurst:    20,
			Tags:              map[string]string{"env": "test-suite"},
		},
	}

	srv := NewServer(cfg, nil, store, nil, nil, nil)
	return srv, store
}

func TestServer_HandleNodeIdentity(t *testing.T) {
	srv, store := setupFleetTestServer(t)
	defer store.Close()

	// 1. Method Not Allowed
	reqPost := httptest.NewRequest(http.MethodPost, "/api/v1/node", nil)
	wPost := httptest.NewRecorder()
	srv.handleNodeIdentity(wPost, reqPost)
	if wPost.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405 Method Not Allowed, got %d", wPost.Code)
	}

	// 2. GET /api/v1/node
	reqGet := httptest.NewRequest(http.MethodGet, "/api/v1/node", nil)
	wGet := httptest.NewRecorder()
	srv.handleNodeIdentity(wGet, reqGet)
	if wGet.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d: %s", wGet.Code, wGet.Body.String())
	}

	var identity model.NodeIdentity
	if err := json.Unmarshal(wGet.Body.Bytes(), &identity); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if identity.NodeID != "test-local-node-id" {
		t.Errorf("Expected node ID 'test-local-node-id', got '%s'", identity.NodeID)
	}
	if identity.Tags["env"] != "test-suite" {
		t.Errorf("Expected tag env=test-suite, got %v", identity.Tags)
	}
	if identity.Hostname == "" || identity.OS == "" {
		t.Errorf("Expected non-empty Hostname and OS: %+v", identity)
	}
}

func TestServer_HandleHeartbeat(t *testing.T) {
	srv, store := setupFleetTestServer(t)
	defer store.Close()

	// 1. Method Not Allowed
	reqGet := httptest.NewRequest(http.MethodGet, "/api/v1/heartbeat", nil)
	wGet := httptest.NewRecorder()
	srv.handleHeartbeat(wGet, reqGet)
	if wGet.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405 Method Not Allowed, got %d", wGet.Code)
	}

	// 2. Invalid JSON payload
	reqBad := httptest.NewRequest(http.MethodPost, "/api/v1/heartbeat", bytes.NewBufferString("invalid json"))
	wBad := httptest.NewRecorder()
	srv.handleHeartbeat(wBad, reqBad)
	if wBad.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request, got %d", wBad.Code)
	}

	// 3. Missing Node ID
	hbNoID := model.HeartbeatRequest{
		Status: model.NodeStatusHealthy,
	}
	bodyNoID, _ := json.Marshal(hbNoID)
	reqNoID := httptest.NewRequest(http.MethodPost, "/api/v1/heartbeat", bytes.NewReader(bodyNoID))
	wNoID := httptest.NewRecorder()
	srv.handleHeartbeat(wNoID, reqNoID)
	if wNoID.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request on missing node ID, got %d", wNoID.Code)
	}

	// 4. Valid Heartbeat
	hbValid := model.HeartbeatRequest{
		NodeID:             "remote-node-1",
		Timestamp:          time.Now().UTC(),
		Status:             model.NodeStatusHealthy,
		CPUUsagePercent:    12.5,
		MemoryUsagePercent: 45.0,
		DiskUsagePercent:   30.0,
	}
	bodyValid, _ := json.Marshal(hbValid)
	reqValid := httptest.NewRequest(http.MethodPost, "/api/v1/heartbeat", bytes.NewReader(bodyValid))
	wValid := httptest.NewRecorder()
	srv.handleHeartbeat(wValid, reqValid)
	if wValid.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d: %s", wValid.Code, wValid.Body.String())
	}

	var hbResp model.HeartbeatResponse
	if err := json.Unmarshal(wValid.Body.Bytes(), &hbResp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if !hbResp.Acknowledged || hbResp.NodeStatus != model.NodeStatusHealthy {
		t.Errorf("Unexpected heartbeat response: %+v", hbResp)
	}

	// 5. Node ID supplied via header
	hbHeader := model.HeartbeatRequest{
		Status: model.NodeStatusHealthy,
	}
	bodyHeader, _ := json.Marshal(hbHeader)
	reqHeader := httptest.NewRequest(http.MethodPost, "/api/v1/heartbeat", bytes.NewReader(bodyHeader))
	reqHeader.Header.Set("X-Watchdog-Node-ID", "header-node-99")
	wHeader := httptest.NewRecorder()
	srv.handleHeartbeat(wHeader, reqHeader)
	if wHeader.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK with header node ID, got %d", wHeader.Code)
	}
}

func TestServer_HandleTelemetry(t *testing.T) {
	srv, store := setupFleetTestServer(t)
	defer store.Close()

	// 1. Method Not Allowed
	reqGet := httptest.NewRequest(http.MethodGet, "/api/v1/telemetry", nil)
	wGet := httptest.NewRecorder()
	srv.handleTelemetry(wGet, reqGet)
	if wGet.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405 Method Not Allowed, got %d", wGet.Code)
	}

	// 2. Invalid payload
	reqBad := httptest.NewRequest(http.MethodPost, "/api/v1/telemetry", bytes.NewBufferString("not-json"))
	wBad := httptest.NewRecorder()
	srv.handleTelemetry(wBad, reqBad)
	if wBad.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request, got %d", wBad.Code)
	}

	// 3. Missing Node ID
	subNoID := model.TelemetrySubmission{
		Timestamp: time.Now().UTC(),
	}
	bodyNoID, _ := json.Marshal(subNoID)
	reqNoID := httptest.NewRequest(http.MethodPost, "/api/v1/telemetry", bytes.NewReader(bodyNoID))
	wNoID := httptest.NewRecorder()
	srv.handleTelemetry(wNoID, reqNoID)
	if wNoID.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request on missing node ID, got %d", wNoID.Code)
	}

	// 4. Valid Telemetry
	subValid := model.TelemetrySubmission{
		NodeID:    "remote-node-2",
		Timestamp: time.Now().UTC(),
		Snapshot: &model.SystemSnapshot{
			Timestamp: time.Now().UTC(),
			CPU: &model.CPUInfo{
				OverallUsage: 35.0,
			},
		},
	}
	bodyValid, _ := json.Marshal(subValid)
	reqValid := httptest.NewRequest(http.MethodPost, "/api/v1/telemetry", bytes.NewReader(bodyValid))
	wValid := httptest.NewRecorder()
	srv.handleTelemetry(wValid, reqValid)
	if wValid.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d: %s", wValid.Code, wValid.Body.String())
	}

	var telemResp model.TelemetryResponse
	if err := json.Unmarshal(wValid.Body.Bytes(), &telemResp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if !telemResp.Accepted || telemResp.PointsCount != 1 {
		t.Errorf("Unexpected telemetry response: %+v", telemResp)
	}
}

func TestServer_HandleFleetRoutes_CRUD(t *testing.T) {
	srv, store := setupFleetTestServer(t)
	defer store.Close()

	// 1. Register a Node via POST /api/v1/fleet/register
	regReq := model.NodeRegistrationRequest{
		Identity: model.NodeIdentity{
			NodeID:   "fleet-node-01",
			Hostname: "worker-01.infra",
			OS:       "linux",
			Platform: "debian",
			Tags:     map[string]string{"role": "db"},
		},
		Metadata: map[string]string{"datacenter": "us-east"},
	}
	bodyReg, _ := json.Marshal(regReq)
	reqReg := httptest.NewRequest(http.MethodPost, "/api/v1/fleet/register", bytes.NewReader(bodyReg))
	wReg := httptest.NewRecorder()
	srv.handleFleetRoute(wReg, reqReg)
	if wReg.Code != http.StatusOK {
		t.Fatalf("Register node failed with %d: %s", wReg.Code, wReg.Body.String())
	}

	var regResp model.NodeRegistrationResponse
	if err := json.Unmarshal(wReg.Body.Bytes(), &regResp); err != nil {
		t.Fatalf("Failed to decode register response: %v", err)
	}
	if !regResp.Registered || regResp.NodeID != "fleet-node-01" {
		t.Errorf("Unexpected register response: %+v", regResp)
	}

	// 2. List Nodes via GET /api/v1/fleet
	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/fleet?limit=10&status=healthy", nil)
	wList := httptest.NewRecorder()
	srv.handleFleetRoute(wList, reqList)
	if wList.Code != http.StatusOK {
		t.Fatalf("List fleet failed with %d: %s", wList.Code, wList.Body.String())
	}

	var listResp model.FleetListResponse
	if err := json.Unmarshal(wList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("Failed to decode list response: %v", err)
	}
	if listResp.Total != 1 || len(listResp.Nodes) != 1 {
		t.Fatalf("Expected 1 node in list, got total=%d count=%d", listResp.Total, len(listResp.Nodes))
	}
	if listResp.Nodes[0].Identity.NodeID != "fleet-node-01" {
		t.Errorf("Expected node ID fleet-node-01, got %s", listResp.Nodes[0].Identity.NodeID)
	}

	// 3. Get Fleet Summary via GET /api/v1/fleet/summary
	reqSum := httptest.NewRequest(http.MethodGet, "/api/v1/fleet/summary", nil)
	wSum := httptest.NewRecorder()
	srv.handleFleetRoute(wSum, reqSum)
	if wSum.Code != http.StatusOK {
		t.Fatalf("Get summary failed with %d: %s", wSum.Code, wSum.Body.String())
	}

	var summary model.FleetSummary
	if err := json.Unmarshal(wSum.Body.Bytes(), &summary); err != nil {
		t.Fatalf("Failed to decode summary response: %v", err)
	}
	if summary.TotalNodes != 1 || summary.HealthyNodes != 1 {
		t.Errorf("Expected 1 total healthy node in summary, got %+v", summary)
	}

	// 4. Get Node Detail via GET /api/v1/fleet/fleet-node-01
	reqDetail := httptest.NewRequest(http.MethodGet, "/api/v1/fleet/fleet-node-01", nil)
	wDetail := httptest.NewRecorder()
	srv.handleFleetRoute(wDetail, reqDetail)
	if wDetail.Code != http.StatusOK {
		t.Fatalf("Get node detail failed with %d: %s", wDetail.Code, wDetail.Body.String())
	}

	var detail model.NodeDetailResponse
	if err := json.Unmarshal(wDetail.Body.Bytes(), &detail); err != nil {
		t.Fatalf("Failed to decode node detail: %v", err)
	}
	if detail.Node.Identity.NodeID != "fleet-node-01" {
		t.Errorf("Expected node ID fleet-node-01, got %s", detail.Node.Identity.NodeID)
	}
	if detail.Node.Metadata["datacenter"] != "us-east" {
		t.Errorf("Expected datacenter us-east, got %v", detail.Node.Metadata)
	}

	// 5. Get Non-existent Node -> 404
	req404 := httptest.NewRequest(http.MethodGet, "/api/v1/fleet/non-existent-node", nil)
	w404 := httptest.NewRecorder()
	srv.handleFleetRoute(w404, req404)
	if w404.Code != http.StatusNotFound {
		t.Errorf("Expected 404 Not Found, got %d", w404.Code)
	}

	// 6. Delete Node via DELETE /api/v1/fleet/fleet-node-01
	reqDel := httptest.NewRequest(http.MethodDelete, "/api/v1/fleet/fleet-node-01", nil)
	wDel := httptest.NewRecorder()
	srv.handleFleetRoute(wDel, reqDel)
	if wDel.Code != http.StatusOK {
		t.Fatalf("Delete node failed with %d: %s", wDel.Code, wDel.Body.String())
	}

	// Verify deletion
	reqCheck := httptest.NewRequest(http.MethodGet, "/api/v1/fleet/fleet-node-01", nil)
	wCheck := httptest.NewRecorder()
	srv.handleFleetRoute(wCheck, reqCheck)
	if wCheck.Code != http.StatusNotFound {
		t.Errorf("Expected 404 Not Found after delete, got %d", wCheck.Code)
	}

	// 7. Delete Non-existent Node -> 404
	reqDel404 := httptest.NewRequest(http.MethodDelete, "/api/v1/fleet/non-existent-node", nil)
	wDel404 := httptest.NewRecorder()
	srv.handleFleetRoute(wDel404, reqDel404)
	if wDel404.Code != http.StatusNotFound {
		t.Errorf("Expected 404 Not Found on delete non-existent, got %d", wDel404.Code)
	}
}
