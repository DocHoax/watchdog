package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DocHoax/watchdog/internal/config"
	"github.com/DocHoax/watchdog/internal/intelligence"
	"github.com/DocHoax/watchdog/internal/logger"
	"github.com/DocHoax/watchdog/internal/storage"
	"github.com/DocHoax/watchdog/pkg/model"
)

type mockIntelFleetService struct {
	nodes map[string]*model.NodeDetailResponse
	list  []model.FleetNode
}

func (m *mockIntelFleetService) GetNode(_ context.Context, nodeID string) (*model.NodeDetailResponse, error) {
	if n, ok := m.nodes[nodeID]; ok {
		return n, nil
	}
	return nil, intelligence.ErrNodeNotFound
}

func (m *mockIntelFleetService) ListNodes(_ context.Context, _ model.FleetFilter) (*model.FleetListResponse, error) {
	return &model.FleetListResponse{
		Nodes: m.list,
		Total: len(m.list),
	}, nil
}

func (m *mockIntelFleetService) GetFleetSummary(_ context.Context) (*model.FleetSummary, error) {
	return &model.FleetSummary{
		TotalNodes: len(m.list),
	}, nil
}

func (m *mockIntelFleetService) RegisterNode(_ context.Context, req *model.NodeRegistrationRequest) (*model.NodeRegistrationResponse, error) {
	return &model.NodeRegistrationResponse{
		Registered: true,
		NodeID:     "node-01",
	}, nil
}

func (m *mockIntelFleetService) ProcessHeartbeat(_ context.Context, _ *model.HeartbeatRequest) (*model.HeartbeatResponse, error) {
	return &model.HeartbeatResponse{Acknowledged: true, NodeStatus: model.NodeStatusHealthy}, nil
}

func (m *mockIntelFleetService) IngestTelemetry(_ context.Context, _ *model.TelemetrySubmission) error {
	return nil
}

func (m *mockIntelFleetService) DeleteNode(_ context.Context, _ string) error {
	return nil
}

func setupIntelligenceTestServer(t *testing.T) (*Server, storage.Storage, http.Handler) {
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
			Enabled: true,
			NodeID:  "node-01",
		},
	}

	now := time.Now()
	mockFleet := &mockIntelFleetService{
		list: []model.FleetNode{
			{
				Identity: model.NodeIdentity{NodeID: "node-01", Hostname: "srv-01"},
				Status:   model.NodeStatusHealthy,
				Summary: &model.NodeSummary{
					CPUUsagePercent:    25.0,
					MemoryUsagePercent: 45.0,
					DiskUsagePercent:   50.0,
				},
				LastHeartbeat: now,
			},
		},
		nodes: map[string]*model.NodeDetailResponse{
			"node-01": {
				Node: model.FleetNode{
					Identity:      model.NodeIdentity{NodeID: "node-01", Hostname: "srv-01"},
					Status:        model.NodeStatusHealthy,
					LastHeartbeat: now,
				},
				LatestSnapshot: &model.SystemSnapshot{
					Timestamp: now,
					CPU: &model.CPUInfo{
						OverallUsage: 25.0,
						LogicalCores: 4,
					},
					Memory: &model.MemoryInfo{
						UsedPercent:    45.0,
						TotalBytes:     16 * 1024 * 1024 * 1024,
						AvailableBytes: 9 * 1024 * 1024 * 1024,
					},
					Disk: &model.DiskInfo{
						Partitions: []model.PartitionInfo{
							{Mountpoint: "/", UsedPercent: 50.0},
						},
					},
				},
				ActiveAlerts: []model.AlertEvent{
					{
						ID:       "alt-01",
						RuleName: "HighCPU",
						Message:  "CPU elevated",
						Severity: model.SeverityWarning,
						FiredAt:  now.Add(-5 * time.Minute),
						IsActive: true,
					},
				},
			},
		},
	}

	srv := NewServer(cfg, nil, store, nil, nil, nil)
	srv.fleetService = mockFleet
	srv.intelService = intelligence.NewService(store, mockFleet, nil, logger.GetDefault(), nil)

	mux := http.NewServeMux()
	mux.Handle("/api/v1/intelligence", srv.authMiddleware(http.HandlerFunc(srv.handleIntelligenceRoute)))
	mux.Handle("/api/v1/intelligence/", srv.authMiddleware(http.HandlerFunc(srv.handleIntelligenceRoute)))

	handler := RequestIDMiddleware(
		PanicRecoveryMiddleware(
			MaxBodySizeMiddleware(DefaultMaxRequestBodySize)(
				RequestLoggerMiddleware(mux),
			),
			srv.auditLog,
		),
	)

	return srv, store, handler
}

func TestServer_IntelligenceEndpoints(t *testing.T) {
	_, store, handler := setupIntelligenceTestServer(t)
	defer store.Close()

	t.Run("GET /api/v1/intelligence/fleet - 200 OK", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/intelligence/fleet", nil)
		req.Header.Set("Authorization", "Bearer test-secret-token")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}

		var summary intelligence.FleetHealthSummary
		if err := json.Unmarshal(w.Body.Bytes(), &summary); err != nil {
			t.Fatalf("failed to decode FleetHealthSummary: %v", err)
		}
		if summary.TotalNodes != 1 {
			t.Errorf("expected 1 total node, got %d", summary.TotalNodes)
		}
		if summary.AverageScore < 80.0 {
			t.Errorf("expected average score >= 80.0, got %.2f", summary.AverageScore)
		}
	})

	t.Run("GET /api/v1/intelligence (root alias) - 200 OK", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/intelligence", nil)
		req.Header.Set("Authorization", "Bearer test-secret-token")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("GET /api/v1/intelligence/nodes/node-01 - 200 OK", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/intelligence/nodes/node-01", nil)
		req.Header.Set("Authorization", "Bearer test-secret-token")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}

		var nodeSummary intelligence.NodeHealthSummary
		if err := json.Unmarshal(w.Body.Bytes(), &nodeSummary); err != nil {
			t.Fatalf("failed to decode NodeHealthSummary: %v", err)
		}
		if nodeSummary.NodeID != "node-01" {
			t.Errorf("expected NodeID node-01, got %s", nodeSummary.NodeID)
		}
		if nodeSummary.HealthScore.Score <= 0 {
			t.Errorf("expected valid health score, got %.2f", nodeSummary.HealthScore.Score)
		}
	})

	t.Run("GET /api/v1/intelligence/nodes/node-non-existent - 404 Not Found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/intelligence/nodes/node-non-existent", nil)
		req.Header.Set("Authorization", "Bearer test-secret-token")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404 Not Found, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("GET /api/v1/intelligence/nodes/node-01/trends - 200 OK", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/intelligence/nodes/node-01/trends?window=1h", nil)
		req.Header.Set("Authorization", "Bearer test-secret-token")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}

		var trends []intelligence.HealthTrend
		if err := json.Unmarshal(w.Body.Bytes(), &trends); err != nil {
			t.Fatalf("failed to decode trends: %v", err)
		}
	})

	t.Run("GET /api/v1/intelligence/nodes/node-01/baselines - 200 OK", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/intelligence/nodes/node-01/baselines?window=24h", nil)
		req.Header.Set("Authorization", "Bearer test-secret-token")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}

		var baselines []intelligence.HistoricalBaseline
		if err := json.Unmarshal(w.Body.Bytes(), &baselines); err != nil {
			t.Fatalf("failed to decode baselines: %v", err)
		}
	})

	t.Run("GET /api/v1/intelligence/incidents - 200 OK", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/intelligence/incidents", nil)
		req.Header.Set("Authorization", "Bearer test-secret-token")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}

		var incidents []intelligence.Incident
		if err := json.Unmarshal(w.Body.Bytes(), &incidents); err != nil {
			t.Fatalf("failed to decode incidents: %v", err)
		}
	})

	t.Run("GET /api/v1/intelligence/incidents/inc-non-existent - 404 Not Found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/intelligence/incidents/inc-non-existent", nil)
		req.Header.Set("Authorization", "Bearer test-secret-token")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404 Not Found, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("GET /api/v1/intelligence/correlations - 200 OK", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/intelligence/correlations?window=30m", nil)
		req.Header.Set("Authorization", "Bearer test-secret-token")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}

		var correlations []intelligence.Correlation
		if err := json.Unmarshal(w.Body.Bytes(), &correlations); err != nil {
			t.Fatalf("failed to decode correlations: %v", err)
		}
	})

	t.Run("GET /api/v1/intelligence/findings - 200 OK", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/intelligence/findings?severity=info", nil)
		req.Header.Set("Authorization", "Bearer test-secret-token")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}

		var findings []intelligence.IntelligenceFinding
		if err := json.Unmarshal(w.Body.Bytes(), &findings); err != nil {
			t.Fatalf("failed to decode findings: %v", err)
		}
	})

	t.Run("GET /api/v1/intelligence/root-cause - 400 Bad Request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/intelligence/root-cause", nil)
		req.Header.Set("Authorization", "Bearer test-secret-token")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("GET /api/v1/intelligence/root-cause/inc-non-existent - 404 Not Found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/intelligence/root-cause/inc-non-existent", nil)
		req.Header.Set("Authorization", "Bearer test-secret-token")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404 Not Found, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("GET /api/v1/intelligence/fleet/predictions - 200 OK", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/intelligence/fleet/predictions", nil)
		req.Header.Set("Authorization", "Bearer test-secret-token")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("GET /api/v1/intelligence/recurrence - 200 OK", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/intelligence/recurrence", nil)
		req.Header.Set("Authorization", "Bearer test-secret-token")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("POST /api/v1/intelligence/fleet - 405 Method Not Allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/intelligence/fleet", nil)
		req.Header.Set("Authorization", "Bearer test-secret-token")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		if w.Code != http.StatusMethodNotAllowed {
			t.Fatalf("expected 405 Method Not Allowed, got %d", w.Code)
		}
	})

	t.Run("GET /api/v1/intelligence/fleet - 401 Unauthorized", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/intelligence/fleet", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 Unauthorized, got %d", w.Code)
		}
	})

	t.Run("GET /api/v1/intelligence/unknown-path - 404 Not Found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/intelligence/unknown-path", nil)
		req.Header.Set("Authorization", "Bearer test-secret-token")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404 Not Found, got %d", w.Code)
		}
	})
}
