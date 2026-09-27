package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DocHoax/watchdog/internal/config"
	"github.com/DocHoax/watchdog/pkg/model"
)

func setupFleetMockServer(t *testing.T) *httptest.Server {
	t.Helper()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/api/v1/fleet/summary" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(model.FleetSummary{
				TotalNodes:    10,
				HealthyNodes:  8,
				WarningNodes:  1,
				CriticalNodes: 1,
				AvgCPUPercent: 25.4,
				AvgMemoryPct:  48.2,
				TotalAlerts:   2,
				LastUpdated:   time.Now().UTC(),
			})

		case r.URL.Path == "/api/v1/fleet" && r.Method == http.MethodGet:
			q := r.URL.Query()
			status := q.Get("status")
			if status == "non-existent" {
				_ = json.NewEncoder(w).Encode(model.FleetListResponse{
					Total: 0,
					Count: 0,
					Nodes: []model.FleetNode{},
				})
				return
			}
			_ = json.NewEncoder(w).Encode(model.FleetListResponse{
				Total: 1,
				Count: 1,
				Nodes: []model.FleetNode{
					{
						Identity: model.NodeIdentity{
							NodeID:   "node-mock-01",
							Hostname: "host-mock-01",
							OS:       "linux",
							Tags:     map[string]string{"env": "test"},
						},
						Status:        model.NodeStatusHealthy,
						LastHeartbeat: time.Now().UTC().Add(-10 * time.Second),
						Summary: &model.NodeSummary{
							CPUUsagePercent:    15.0,
							MemoryUsagePercent: 30.0,
							DiskUsagePercent:   45.0,
							ActiveAlertsCount:  0,
						},
					},
				},
			})

		case r.URL.Path == "/api/v1/fleet/node-mock-01" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(model.NodeDetailResponse{
				Node: model.FleetNode{
					Identity: model.NodeIdentity{
						NodeID:      "node-mock-01",
						Hostname:    "host-mock-01",
						OS:          "linux",
						Platform:    "ubuntu",
						PlatformVer: "22.04",
						Arch:        "amd64",
						CPUCores:    8,
						TotalMemory: 16 * 1024 * 1024 * 1024,
						Version:     "1.0.0",
						Tags:        map[string]string{"env": "test"},
					},
					Status:        model.NodeStatusHealthy,
					RegisteredAt:  time.Now().UTC().Add(-24 * time.Hour),
					LastHeartbeat: time.Now().UTC().Add(-5 * time.Second),
					Summary: &model.NodeSummary{
						CPUUsagePercent:    15.0,
						MemoryUsagePercent: 30.0,
						DiskUsagePercent:   45.0,
						ActiveAlertsCount:  1,
						DiagnosticStatus:   "HEALTHY",
					},
				},
				ActiveAlerts: []model.AlertEvent{
					{
						ID:       "alert-1",
						RuleName: "HighCPU",
						Severity: model.SeverityWarning,
						Message:  "CPU exceeded 80%",
						FiredAt:  time.Now().UTC().Add(-5 * time.Minute),
						IsActive: true,
					},
				},
			})

		case r.URL.Path == "/api/v1/fleet/register" && r.Method == http.MethodPost:
			var req model.NodeRegistrationRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			_ = json.NewEncoder(w).Encode(model.NodeRegistrationResponse{
				Registered:               true,
				NodeID:                   req.Identity.NodeID,
				RegisteredAt:             time.Now().UTC(),
				HeartbeatIntervalSeconds: 15,
				TelemetryIntervalSeconds: 30,
				Message:                  "Welcome to fleet",
			})

		case r.URL.Path == "/api/v1/heartbeat" && r.Method == http.MethodPost:
			var req model.HeartbeatRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			_ = json.NewEncoder(w).Encode(model.HeartbeatResponse{
				Acknowledged:          true,
				NodeStatus:            req.Status,
				NextHeartbeatInterval: 15,
				Timestamp:             time.Now().UTC(),
			})

		case r.URL.Path == "/api/v1/fleet/node-mock-01" && r.Method == http.MethodDelete:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"deleted": true,
				"node_id": "node-mock-01",
			})

		default:
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{
					"code":    "NOT_FOUND",
					"message": "resource not found",
				},
			})
		}
	}))

	return ts
}

func TestCmd_Fleet_Status(t *testing.T) {
	ts := setupFleetMockServer(t)
	defer ts.Close()

	fleetServerURL = ts.URL
	fleetToken = "test-token"
	fleetTokenFile = ""
	fleetTokenEnv = ""
	fleetInsecureTLS = false
	fleetTimeout = 5 * time.Second
	globalCfg = config.DefaultConfig()

	// 1. Human readable status
	fleetJSON = false
	out, err := captureStdout(func() error {
		return runFleetStatus(fleetStatusCmd, nil)
	})
	if err != nil {
		t.Fatalf("runFleetStatus returned error: %v", err)
	}
	if !strings.Contains(out, "Watchdog Fleet Summary") || !strings.Contains(out, "Total Nodes:       10") {
		t.Errorf("Unexpected status output: %s", out)
	}

	// 2. JSON status
	fleetJSON = true
	outJSON, err := captureStdout(func() error {
		return runFleetStatus(fleetStatusCmd, nil)
	})
	if err != nil {
		t.Fatalf("runFleetStatus JSON returned error: %v", err)
	}
	var sum model.FleetSummary
	if err := json.Unmarshal([]byte(outJSON), &sum); err != nil {
		t.Fatalf("Failed to parse JSON summary: %v", err)
	}
	if sum.TotalNodes != 10 || sum.HealthyNodes != 8 {
		t.Errorf("Unexpected parsed summary: %+v", sum)
	}
}

func TestCmd_Fleet_List(t *testing.T) {
	ts := setupFleetMockServer(t)
	defer ts.Close()

	fleetServerURL = ts.URL
	fleetToken = "test-token"
	fleetTokenFile = ""
	fleetTokenEnv = ""
	fleetInsecureTLS = false
	fleetTimeout = 5 * time.Second
	fleetFilterStatus = ""
	fleetFilterSearch = ""
	fleetFilterLimit = 10
	fleetFilterOffset = 0
	fleetFilterSortBy = ""
	fleetFilterSortDirection = ""
	fleetFilterSince = ""
	globalCfg = config.DefaultConfig()

	// 1. Table list
	fleetJSON = false
	out, err := captureStdout(func() error {
		return runFleetList(fleetListCmd, nil)
	})
	if err != nil {
		t.Fatalf("runFleetList returned error: %v", err)
	}
	if !strings.Contains(out, "node-mock-01") || !strings.Contains(out, "host-mock-01") {
		t.Errorf("Expected node-mock-01 in list table, got: %s", out)
	}

	// 2. JSON list
	fleetJSON = true
	outJSON, err := captureStdout(func() error {
		return runFleetList(fleetListCmd, nil)
	})
	if err != nil {
		t.Fatalf("runFleetList JSON returned error: %v", err)
	}
	var listResp model.FleetListResponse
	if err := json.Unmarshal([]byte(outJSON), &listResp); err != nil {
		t.Fatalf("Failed to parse JSON list response: %v", err)
	}
	if listResp.Total != 1 || listResp.Nodes[0].Identity.NodeID != "node-mock-01" {
		t.Errorf("Unexpected parsed list: %+v", listResp)
	}

	// 3. Empty list
	fleetJSON = false
	fleetFilterStatus = "non-existent"
	outEmpty, err := captureStdout(func() error {
		return runFleetList(fleetListCmd, nil)
	})
	if err != nil {
		t.Fatalf("runFleetList empty returned error: %v", err)
	}
	if !strings.Contains(outEmpty, "No fleet nodes found") {
		t.Errorf("Expected 'No fleet nodes found', got: %s", outEmpty)
	}
}

func TestCmd_Fleet_Get(t *testing.T) {
	ts := setupFleetMockServer(t)
	defer ts.Close()

	fleetServerURL = ts.URL
	fleetToken = "test-token"
	fleetTokenFile = ""
	fleetTokenEnv = ""
	fleetInsecureTLS = false
	fleetTimeout = 5 * time.Second
	globalCfg = config.DefaultConfig()

	// 1. Human readable get
	fleetJSON = false
	out, err := captureStdout(func() error {
		return runFleetGet(fleetGetCmd, []string{"node-mock-01"})
	})
	if err != nil {
		t.Fatalf("runFleetGet returned error: %v", err)
	}
	if !strings.Contains(out, "Watchdog Fleet Node Detail") || !strings.Contains(out, "node-mock-01") {
		t.Errorf("Expected node detail header and ID, got: %s", out)
	}
	if !strings.Contains(out, "HighCPU") {
		t.Errorf("Expected alert HighCPU in output, got: %s", out)
	}

	// 2. JSON get
	fleetJSON = true
	outJSON, err := captureStdout(func() error {
		return runFleetGet(fleetGetCmd, []string{"node-mock-01"})
	})
	if err != nil {
		t.Fatalf("runFleetGet JSON returned error: %v", err)
	}
	var detail model.NodeDetailResponse
	if err := json.Unmarshal([]byte(outJSON), &detail); err != nil {
		t.Fatalf("Failed to parse JSON detail response: %v", err)
	}
	if detail.Node.Identity.NodeID != "node-mock-01" {
		t.Errorf("Unexpected node ID in detail: %s", detail.Node.Identity.NodeID)
	}

	// 3. 404 get
	err404 := runFleetGet(fleetGetCmd, []string{"unknown-node"})
	if err404 == nil {
		t.Errorf("Expected error for unknown node")
	}
}

func TestCmd_Fleet_Register(t *testing.T) {
	ts := setupFleetMockServer(t)
	defer ts.Close()

	fleetServerURL = ts.URL
	fleetToken = "test-token"
	fleetTokenFile = ""
	fleetTokenEnv = ""
	fleetInsecureTLS = false
	fleetTimeout = 5 * time.Second
	regNodeID = "reg-test-01"
	regHostname = "reg-host"
	regTags = []string{"env=staging", "owner=sre"}
	regMetadata = []string{"cloud=aws", "region=us-west-2"}
	globalCfg = config.DefaultConfig()

	// 1. Human readable register
	fleetJSON = false
	out, err := captureStdout(func() error {
		return runFleetRegister(fleetRegisterCmd, nil)
	})
	if err != nil {
		t.Fatalf("runFleetRegister returned error: %v", err)
	}
	if !strings.Contains(out, "Successfully registered node with fleet controller") {
		t.Errorf("Expected register success message, got: %s", out)
	}

	// 2. JSON register
	fleetJSON = true
	outJSON, err := captureStdout(func() error {
		return runFleetRegister(fleetRegisterCmd, nil)
	})
	if err != nil {
		t.Fatalf("runFleetRegister JSON returned error: %v", err)
	}
	var regResp model.NodeRegistrationResponse
	if err := json.Unmarshal([]byte(outJSON), &regResp); err != nil {
		t.Fatalf("Failed to parse JSON register response: %v", err)
	}
	if !regResp.Registered || regResp.NodeID != "reg-test-01" {
		t.Errorf("Unexpected register response: %+v", regResp)
	}
}

func TestCmd_Fleet_Heartbeat(t *testing.T) {
	ts := setupFleetMockServer(t)
	defer ts.Close()

	fleetServerURL = ts.URL
	fleetToken = "test-token"
	fleetTokenFile = ""
	fleetTokenEnv = ""
	fleetInsecureTLS = false
	fleetTimeout = 5 * time.Second
	hbNodeID = "hb-test-01"
	hbStatus = "healthy"
	globalCfg = config.DefaultConfig()

	// 1. Human readable heartbeat
	fleetJSON = false
	out, err := captureStdout(func() error {
		return runFleetHeartbeat(fleetHeartbeatCmd, nil)
	})
	if err != nil {
		t.Fatalf("runFleetHeartbeat returned error: %v", err)
	}
	if !strings.Contains(out, "Heartbeat acknowledged by fleet server") {
		t.Errorf("Expected heartbeat ack, got: %s", out)
	}

	// 2. JSON heartbeat
	fleetJSON = true
	outJSON, err := captureStdout(func() error {
		return runFleetHeartbeat(fleetHeartbeatCmd, nil)
	})
	if err != nil {
		t.Fatalf("runFleetHeartbeat JSON returned error: %v", err)
	}
	var hbResp model.HeartbeatResponse
	if err := json.Unmarshal([]byte(outJSON), &hbResp); err != nil {
		t.Fatalf("Failed to parse JSON heartbeat response: %v", err)
	}
	if !hbResp.Acknowledged {
		t.Errorf("Expected heartbeat acknowledged, got: %+v", hbResp)
	}
}

func TestCmd_Fleet_Deregister(t *testing.T) {
	ts := setupFleetMockServer(t)
	defer ts.Close()

	fleetServerURL = ts.URL
	fleetToken = "test-token"
	fleetTokenFile = ""
	fleetTokenEnv = ""
	fleetInsecureTLS = false
	fleetTimeout = 5 * time.Second
	deregForce = true
	globalCfg = config.DefaultConfig()

	// 1. Human readable deregister
	fleetJSON = false
	out, err := captureStdout(func() error {
		return runFleetDeregister(fleetDeregisterCmd, []string{"node-mock-01"})
	})
	if err != nil {
		t.Fatalf("runFleetDeregister returned error: %v", err)
	}
	if !strings.Contains(out, "Successfully deregistered node") {
		t.Errorf("Expected deregister success, got: %s", out)
	}

	// 2. JSON deregister
	fleetJSON = true
	outJSON, err := captureStdout(func() error {
		return runFleetDeregister(fleetDeregisterCmd, []string{"node-mock-01"})
	})
	if err != nil {
		t.Fatalf("runFleetDeregister JSON returned error: %v", err)
	}
	var delMap map[string]any
	if err := json.Unmarshal([]byte(outJSON), &delMap); err != nil {
		t.Fatalf("Failed to parse JSON delete response: %v", err)
	}
	if delMap["deleted"] != true {
		t.Errorf("Expected deleted=true, got %v", delMap)
	}
}

func TestCmd_Fleet_MissingServerURL(t *testing.T) {
	fleetServerURL = ""
	globalCfg = config.DefaultConfig()
	globalCfg.Fleet.ServerURL = ""

	err := runFleetStatus(fleetStatusCmd, nil)
	if err == nil {
		t.Errorf("Expected error when fleet server URL is missing")
	}
	if GetExitCode(err) != ExitConfigError {
		t.Errorf("Expected ExitConfigError, got %d", GetExitCode(err))
	}
}
