package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DocHoax/watchdog/internal/audit"
	"github.com/DocHoax/watchdog/internal/config"
	"github.com/DocHoax/watchdog/internal/incidents"
	"github.com/DocHoax/watchdog/internal/storage"
	"github.com/DocHoax/watchdog/pkg/model"
)

func setupIncidentTestServer(t *testing.T) (*Server, storage.Storage, incidents.Service, http.Handler) {
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

	auditLog := audit.New(config.AuditConfig{Enabled: true}, store, nil)
	srv := NewServer(cfg, nil, store, nil, nil, nil)
	srv.auditLog = auditLog

	incSvc := incidents.NewService(store, 15*time.Minute)
	srv.SetIncidentService(incSvc)

	mux := http.NewServeMux()
	mux.Handle("/api/v1/incidents", srv.authMiddleware(http.HandlerFunc(srv.handleIncidentsRoute)))
	mux.Handle("/api/v1/incidents/", srv.authMiddleware(http.HandlerFunc(srv.handleIncidentsRoute)))

	handler := RequestIDMiddleware(
		PanicRecoveryMiddleware(
			MaxBodySizeMiddleware(DefaultMaxRequestBodySize)(
				RequestLoggerMiddleware(mux),
			),
			srv.auditLog,
		),
	)

	return srv, store, incSvc, handler
}

func seedTestIncidents(t *testing.T, store storage.Storage) {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC()

	incs := []*incidents.Incident{
		{
			ID:              "inc-001",
			Title:           "High Memory Usage on Worker Node",
			Status:          incidents.IncidentStatusDetected,
			Severity:        model.SeverityCritical,
			Scope:           incidents.IncidentScopeNode,
			StartTime:       now.Add(-2 * time.Hour),
			AffectedNodes:   []string{"node-01"},
			PrimarySymptoms: []string{"Memory pressure", "High swap usage"},
			RootSignals: []incidents.IncidentSignal{
				{
					ID:          "sig-01",
					Type:        incidents.SignalTypeAlert,
					Source:      "HighMemoryAlert",
					NodeID:      "node-01",
					Severity:    model.SeverityCritical,
					Description: "Memory exceeded 90%",
					Timestamp:   now.Add(-2 * time.Hour),
				},
			},
			Impact: incidents.ImpactScope{
				FleetPercentage: 10.0,
				TotalFleetNodes: 10,
				AffectedNodeIDs: []string{"node-01"},
				Subsystems:      []string{"memory", "swap"},
			},
			SeverityExplanation: incidents.SeverityExplanation{
				BaseScore:          85,
				CalculatedSeverity: model.SeverityCritical,
				Confidence:         "high",
				Reasoning:          []string{"Critical alert on memory"},
			},
			Findings: []incidents.IntelligenceFinding{
				{
					ID:          "find-01",
					Category:    incidents.FindingCategoryResourceExhaustion,
					Severity:    model.SeverityCritical,
					Confidence:  incidents.FindingConfidenceHigh,
					Title:       "Memory exhaustion imminent",
					Description: "Check leaking processes",
					DetectedAt:  now.Add(-2 * time.Hour),
				},
			},
		},
		{
			ID:              "inc-002",
			Title:           "Network Latency Spike Across Fleet",
			Status:          incidents.IncidentStatusInvestigating,
			Severity:        model.SeverityWarning,
			Scope:           incidents.IncidentScopeFleet,
			StartTime:       now.Add(-1 * time.Hour),
			AffectedNodes:   []string{"node-01", "node-02", "node-03"},
			PrimarySymptoms: []string{"High network RTT", "Packet drops"},
			RootSignals: []incidents.IncidentSignal{
				{
					ID:          "sig-02",
					Type:        incidents.SignalTypeAnomaly,
					Source:      "NetworkLatencyAnomaly",
					NodeID:      "node-02",
					Severity:    model.SeverityWarning,
					Description: "RTT > 150ms",
					Timestamp:   now.Add(-1 * time.Hour),
				},
			},
			Impact: incidents.ImpactScope{
				FleetPercentage: 30.0,
				TotalFleetNodes: 10,
				AffectedNodeIDs: []string{"node-01", "node-02", "node-03"},
				Subsystems:      []string{"network"},
			},
		},
		{
			ID:              "inc-003",
			Title:           "Disk Space Warning",
			Status:          incidents.IncidentStatusResolved,
			Severity:        model.SeverityInfo,
			Scope:           incidents.IncidentScopeNode,
			StartTime:       now.Add(-5 * time.Hour),
			ResolvedAt:      &now,
			AffectedNodes:   []string{"node-03"},
			PrimarySymptoms: []string{"Disk usage > 80%"},
		},
	}

	for _, inc := range incs {
		if err := store.SaveIncident(ctx, inc); err != nil {
			t.Fatalf("failed to seed incident %s: %v", inc.ID, err)
		}
	}

	// Seed timeline for inc-001
	timeline := []incidents.IncidentTimelineEntry{
		{
			ID:          "tl-001",
			IncidentID:  "inc-001",
			Timestamp:   now.Add(-2 * time.Hour),
			EventType:   incidents.TimelineEventAlertFired,
			Source:      "HighMemoryAlert",
			NodeID:      "node-01",
			Severity:    model.SeverityCritical,
			Title:       "Memory alert fired",
			Description: "Memory above 90%",
		},
		{
			ID:          "tl-002",
			IncidentID:  "inc-001",
			Timestamp:   now.Add(-100 * time.Minute),
			EventType:   incidents.TimelineEventDiagnosticFailed,
			Source:      "OOMCheck",
			NodeID:      "node-01",
			Severity:    model.SeverityWarning,
			Title:       "OOM events logged",
			Description: "Kernel killed worker process",
		},
	}
	if err := store.SaveTimelineEntries(ctx, timeline); err != nil {
		t.Fatalf("failed to seed timeline entries: %v", err)
	}
}

func TestServer_Incidents_Authentication(t *testing.T) {
	_, store, _, handler := setupIncidentTestServer(t)
	defer store.Close()

	endpoints := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/incidents"},
		{http.MethodGet, "/api/v1/incidents/summary"},
		{http.MethodGet, "/api/v1/incidents/similar?id=inc-001"},
		{http.MethodGet, "/api/v1/incidents/inc-001"},
		{http.MethodGet, "/api/v1/incidents/inc-001/timeline"},
		{http.MethodGet, "/api/v1/incidents/inc-001/related"},
		{http.MethodGet, "/api/v1/incidents/inc-001/impact"},
		{http.MethodGet, "/api/v1/incidents/inc-001/findings"},
		{http.MethodGet, "/api/v1/incidents/inc-001/investigate"},
		{http.MethodPost, "/api/v1/incidents/inc-001/status"},
	}

	for _, ep := range endpoints {
		t.Run(ep.method+" "+ep.path+" - Unauthorized", func(t *testing.T) {
			req := httptest.NewRequest(ep.method, ep.path, bytes.NewBufferString("{}"))
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != http.StatusUnauthorized {
				t.Errorf("expected 401 Unauthorized, got %d", w.Code)
			}
		})
	}
}

func TestServer_Incidents_ServiceUnavailable(t *testing.T) {
	srv, store, _, handler := setupIncidentTestServer(t)
	defer store.Close()
	srv.SetIncidentService(nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/incidents", nil)
	req.Header.Set("Authorization", "Bearer test-secret-token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 Service Unavailable, got %d: %s", w.Code, w.Body.String())
	}
}

func TestServer_Incidents_MethodNotAllowed(t *testing.T) {
	_, store, _, handler := setupIncidentTestServer(t)
	defer store.Close()

	getEndpoints := []string{
		"/api/v1/incidents",
		"/api/v1/incidents/summary",
		"/api/v1/incidents/similar",
		"/api/v1/incidents/inc-001",
		"/api/v1/incidents/inc-001/timeline",
		"/api/v1/incidents/inc-001/related",
		"/api/v1/incidents/inc-001/impact",
		"/api/v1/incidents/inc-001/findings",
		"/api/v1/incidents/inc-001/investigate",
	}

	for _, p := range getEndpoints {
		t.Run("POST on "+p+" - 405 Method Not Allowed", func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, p, bytes.NewBufferString("{}"))
			req.Header.Set("Authorization", "Bearer test-secret-token")
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != http.StatusMethodNotAllowed {
				t.Errorf("expected 405 Method Not Allowed, got %d", w.Code)
			}
			if w.Header().Get("Allow") == "" {
				t.Errorf("expected Allow header to be set")
			}
		})
	}

	t.Run("GET on /api/v1/incidents/inc-001/status - 405 Method Not Allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/incidents/inc-001/status", nil)
		req.Header.Set("Authorization", "Bearer test-secret-token")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405 Method Not Allowed, got %d", w.Code)
		}
		if w.Header().Get("Allow") != "POST" {
			t.Errorf("expected Allow: POST, got %s", w.Header().Get("Allow"))
		}
	})
}

func TestServer_Incidents_ListAndFilter(t *testing.T) {
	_, store, _, handler := setupIncidentTestServer(t)
	defer store.Close()
	seedTestIncidents(t, store)

	t.Run("GET /api/v1/incidents - List all", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/incidents", nil)
		req.Header.Set("Authorization", "Bearer test-secret-token")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		total, ok := resp["total"].(float64)
		if !ok || int(total) != 3 {
			t.Errorf("expected total=3, got %v", resp["total"])
		}

		incs, ok := resp["incidents"].([]any)
		if !ok || len(incs) != 3 {
			t.Errorf("expected 3 incidents, got %d", len(incs))
		}
	})

	t.Run("GET /api/v1/incidents?status=detected,investigating - Filter status", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/incidents?status=detected,investigating", nil)
		req.Header.Set("Authorization", "Bearer test-secret-token")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}

		var resp struct {
			Incidents []incidents.Incident `json:"incidents"`
			Total     int                  `json:"total"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.Total != 2 || len(resp.Incidents) != 2 {
			t.Fatalf("expected 2 incidents, got total=%d, len=%d", resp.Total, len(resp.Incidents))
		}
	})

	t.Run("GET /api/v1/incidents?status=invalid_status - 400 Bad Request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/incidents?status=invalid_status", nil)
		req.Header.Set("Authorization", "Bearer test-secret-token")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request, got %d", w.Code)
		}
	})

	t.Run("GET /api/v1/incidents?severity=critical - Filter severity", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/incidents?severity=critical", nil)
		req.Header.Set("Authorization", "Bearer test-secret-token")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}

		var resp struct {
			Incidents []incidents.Incident `json:"incidents"`
			Total     int                  `json:"total"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.Total != 1 || len(resp.Incidents) != 1 || resp.Incidents[0].ID != "inc-001" {
			t.Fatalf("expected 1 critical incident inc-001, got total=%d, incidents=%+v", resp.Total, resp.Incidents)
		}
	})

	t.Run("GET /api/v1/incidents?scope=fleet - Filter scope", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/incidents?scope=fleet", nil)
		req.Header.Set("Authorization", "Bearer test-secret-token")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}

		var resp struct {
			Incidents []incidents.Incident `json:"incidents"`
			Total     int                  `json:"total"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.Total != 1 || resp.Incidents[0].ID != "inc-002" {
			t.Fatalf("expected inc-002, got %+v", resp.Incidents)
		}
	})

	t.Run("GET /api/v1/incidents?node_id=node-01 - Filter node", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/incidents?node_id=node-01", nil)
		req.Header.Set("Authorization", "Bearer test-secret-token")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}

		var resp struct {
			Incidents []incidents.Incident `json:"incidents"`
			Total     int                  `json:"total"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.Total != 2 {
			t.Fatalf("expected 2 incidents for node-01, got %d", resp.Total)
		}
	})

	t.Run("GET /api/v1/incidents?search=Latency - Search query", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/incidents?search=Latency", nil)
		req.Header.Set("Authorization", "Bearer test-secret-token")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}

		var resp struct {
			Incidents []incidents.Incident `json:"incidents"`
			Total     int                  `json:"total"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.Total != 1 || resp.Incidents[0].ID != "inc-002" {
			t.Fatalf("expected inc-002 for search Latency, got %+v", resp.Incidents)
		}
	})

	t.Run("GET /api/v1/incidents?limit=invalid - 400 Bad Request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/incidents?limit=invalid", nil)
		req.Header.Set("Authorization", "Bearer test-secret-token")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request for invalid limit, got %d", w.Code)
		}
	})

	t.Run("GET /api/v1/incidents?offset=invalid - 400 Bad Request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/incidents?offset=invalid", nil)
		req.Header.Set("Authorization", "Bearer test-secret-token")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request for invalid offset, got %d", w.Code)
		}
	})

	t.Run("GET /api/v1/incidents?start_time=invalid_time - 400 Bad Request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/incidents?start_time=invalid_time", nil)
		req.Header.Set("Authorization", "Bearer test-secret-token")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request for invalid start_time, got %d", w.Code)
		}
	})

	t.Run("GET /api/v1/incidents?end_time=invalid_time - 400 Bad Request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/incidents?end_time=invalid_time", nil)
		req.Header.Set("Authorization", "Bearer test-secret-token")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request for invalid end_time, got %d", w.Code)
		}
	})

	t.Run("GET /api/v1/incidents?limit=1&offset=0 - Pagination", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/incidents?limit=1&offset=0", nil)
		req.Header.Set("Authorization", "Bearer test-secret-token")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}

		var resp struct {
			Incidents []incidents.Incident `json:"incidents"`
			Total     int                  `json:"total"`
			Limit     int                  `json:"limit"`
			Offset    int                  `json:"offset"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.Total != 3 || len(resp.Incidents) != 1 || resp.Limit != 1 || resp.Offset != 0 {
			t.Fatalf("pagination verification failed: %+v", resp)
		}
	})
}

func TestServer_Incidents_Summary(t *testing.T) {
	_, store, _, handler := setupIncidentTestServer(t)
	defer store.Close()
	seedTestIncidents(t, store)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/incidents/summary", nil)
	req.Header.Set("Authorization", "Bearer test-secret-token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var summary incidents.IncidentSummary
	if err := json.Unmarshal(w.Body.Bytes(), &summary); err != nil {
		t.Fatalf("failed to decode IncidentSummary: %v", err)
	}

	if summary.TotalCount != 3 {
		t.Errorf("expected 3 total incidents, got %d", summary.TotalCount)
	}
	if summary.DetectedCount != 1 {
		t.Errorf("expected 1 detected incident, got %d", summary.DetectedCount)
	}
	if summary.InvestigatingCount != 1 {
		t.Errorf("expected 1 investigating incident, got %d", summary.InvestigatingCount)
	}
	if summary.ResolvedCount != 1 {
		t.Errorf("expected 1 resolved incident, got %d", summary.ResolvedCount)
	}
	if summary.CriticalCount != 1 {
		t.Errorf("expected 1 critical incident, got %d", summary.CriticalCount)
	}
	if len(summary.TopAffectedNodes) == 0 {
		t.Errorf("expected top affected nodes to be populated")
	}
}

func TestServer_Incidents_Similar(t *testing.T) {
	_, store, _, handler := setupIncidentTestServer(t)
	defer store.Close()
	seedTestIncidents(t, store)

	t.Run("GET /api/v1/incidents/similar - Missing ID", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/incidents/similar", nil)
		req.Header.Set("Authorization", "Bearer test-secret-token")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request, got %d", w.Code)
		}
	})

	t.Run("GET /api/v1/incidents/similar?id=inc-001&min_similarity=1.5 - Invalid min_similarity", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/incidents/similar?id=inc-001&min_similarity=1.5", nil)
		req.Header.Set("Authorization", "Bearer test-secret-token")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request, got %d", w.Code)
		}
	})

	t.Run("GET /api/v1/incidents/similar?id=non-existent - 404 Not Found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/incidents/similar?id=non-existent", nil)
		req.Header.Set("Authorization", "Bearer test-secret-token")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404 Not Found, got %d", w.Code)
		}
	})

	t.Run("GET /api/v1/incidents/similar?id=inc-001 - 200 OK", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/incidents/similar?id=inc-001", nil)
		req.Header.Set("Authorization", "Bearer test-secret-token")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp["incident_id"] != "inc-001" {
			t.Errorf("expected incident_id inc-001, got %v", resp["incident_id"])
		}
	})
}

func TestServer_Incidents_GetIncident(t *testing.T) {
	_, store, _, handler := setupIncidentTestServer(t)
	defer store.Close()
	seedTestIncidents(t, store)

	t.Run("GET /api/v1/incidents/inc-001 - 200 OK", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/incidents/inc-001", nil)
		req.Header.Set("Authorization", "Bearer test-secret-token")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}

		var inc incidents.Incident
		if err := json.Unmarshal(w.Body.Bytes(), &inc); err != nil {
			t.Fatalf("failed to decode Incident: %v", err)
		}

		if inc.ID != "inc-001" || inc.Title != "High Memory Usage on Worker Node" {
			t.Errorf("unexpected incident data: %+v", inc)
		}
	})

	t.Run("GET /api/v1/incidents/non-existent - 404 Not Found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/incidents/non-existent", nil)
		req.Header.Set("Authorization", "Bearer test-secret-token")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404 Not Found, got %d", w.Code)
		}
	})
}

func TestServer_Incidents_Timeline(t *testing.T) {
	_, store, _, handler := setupIncidentTestServer(t)
	defer store.Close()
	seedTestIncidents(t, store)

	t.Run("GET /api/v1/incidents/inc-001/timeline - 200 OK", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/incidents/inc-001/timeline", nil)
		req.Header.Set("Authorization", "Bearer test-secret-token")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}

		var resp struct {
			IncidentID string                            `json:"incident_id"`
			Timeline   []incidents.IncidentTimelineEntry `json:"timeline"`
			Count      int                               `json:"count"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode timeline response: %v", err)
		}

		if resp.IncidentID != "inc-001" || resp.Count != 2 || len(resp.Timeline) != 2 {
			t.Fatalf("unexpected timeline response: %+v", resp)
		}
	})

	t.Run("GET /api/v1/incidents/non-existent/timeline - 404 Not Found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/incidents/non-existent/timeline", nil)
		req.Header.Set("Authorization", "Bearer test-secret-token")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404 Not Found, got %d", w.Code)
		}
	})
}

func TestServer_Incidents_Subresources(t *testing.T) {
	_, store, _, handler := setupIncidentTestServer(t)
	defer store.Close()
	seedTestIncidents(t, store)

	t.Run("GET /api/v1/incidents/inc-001/related - 200 OK", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/incidents/inc-001/related", nil)
		req.Header.Set("Authorization", "Bearer test-secret-token")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("GET /api/v1/incidents/inc-001/impact - 200 OK", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/incidents/inc-001/impact", nil)
		req.Header.Set("Authorization", "Bearer test-secret-token")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}

		var impact incidents.ImpactAnalysis
		if err := json.Unmarshal(w.Body.Bytes(), &impact); err != nil {
			t.Fatalf("failed to decode ImpactAnalysis: %v", err)
		}
		if impact.IncidentID != "inc-001" {
			t.Errorf("expected incident_id inc-001, got %s", impact.IncidentID)
		}
	})

	t.Run("GET /api/v1/incidents/inc-001/findings - 200 OK", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/incidents/inc-001/findings", nil)
		req.Header.Set("Authorization", "Bearer test-secret-token")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}

		var resp struct {
			Findings []incidents.IntelligenceFinding `json:"findings"`
			Count    int                            `json:"count"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode findings: %v", err)
		}
		if resp.Count != 1 || len(resp.Findings) != 1 {
			t.Errorf("expected 1 finding, got %d", resp.Count)
		}
	})

	t.Run("GET /api/v1/incidents/inc-001/investigate - 200 OK", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/incidents/inc-001/investigate", nil)
		req.Header.Set("Authorization", "Bearer test-secret-token")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}

		var report incidents.IncidentInvestigationReport
		if err := json.Unmarshal(w.Body.Bytes(), &report); err != nil {
			t.Fatalf("failed to decode investigation report: %v", err)
		}
		if report.Incident.ID != "inc-001" {
			t.Errorf("expected report for inc-001, got %s", report.Incident.ID)
		}
	})

	t.Run("GET /api/v1/incidents/inc-001/unknown_subresource - 404 Not Found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/incidents/inc-001/unknown_subresource", nil)
		req.Header.Set("Authorization", "Bearer test-secret-token")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404 Not Found, got %d", w.Code)
		}
	})
}

func TestServer_Incidents_StatusTransition(t *testing.T) {
	_, store, _, handler := setupIncidentTestServer(t)
	defer store.Close()
	seedTestIncidents(t, store)

	t.Run("POST /api/v1/incidents/inc-001/status - Malformed JSON", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/incidents/inc-001/status", bytes.NewBufferString("invalid json"))
		req.Header.Set("Authorization", "Bearer test-secret-token")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request, got %d", w.Code)
		}
	})

	t.Run("POST /api/v1/incidents/inc-001/status - Invalid Status string", func(t *testing.T) {
		body := `{"status": "flying", "reason": "invalid"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/incidents/inc-001/status", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer test-secret-token")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request, got %d", w.Code)
		}
	})

	t.Run("POST /api/v1/incidents/non-existent/status - 404 Not Found", func(t *testing.T) {
		body := `{"status": "acknowledged", "reason": "triaging"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/incidents/non-existent/status", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer test-secret-token")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404 Not Found, got %d", w.Code)
		}
	})

	t.Run("POST /api/v1/incidents/inc-001/status - Invalid transition (detected -> reopened)", func(t *testing.T) {
		body := `{"status": "reopened", "reason": "trying to reopen detected incident"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/incidents/inc-001/status", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer test-secret-token")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request for invalid transition, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("POST /api/v1/incidents/inc-001/status - Valid transition (detected -> acknowledged)", func(t *testing.T) {
		body := `{"status": "acknowledged", "reason": "Operator triaging issue"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/incidents/inc-001/status", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer test-secret-token")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}

		var updated incidents.Incident
		if err := json.Unmarshal(w.Body.Bytes(), &updated); err != nil {
			t.Fatalf("failed to decode updated incident: %v", err)
		}
		if updated.Status != incidents.IncidentStatusAcknowledged {
			t.Errorf("expected status acknowledged, got %s", updated.Status)
		}

		// Verify audit log
		events, err := store.QueryAuditEvents(context.Background(), storage.AuditFilter{Limit: 10})
		if err != nil || len(events) == 0 {
			t.Fatalf("expected audit events to be recorded, err: %v, count: %d", err, len(events))
		}

		found := false
		for _, ev := range events {
			if ev.EventType == model.EventIncidentStatusChange {
				found = true
				if ev.Metadata["incident_id"] != "inc-001" || ev.Metadata["new_status"] != "acknowledged" {
					t.Errorf("audit metadata mismatch: %+v", ev.Metadata)
				}
				break
			}
		}
		if !found {
			t.Errorf("expected EventIncidentStatusChange in audit log")
		}
	})
}
