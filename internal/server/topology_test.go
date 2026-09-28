package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DocHoax/watchdog/internal/config"
	"github.com/DocHoax/watchdog/internal/storage"
	"github.com/DocHoax/watchdog/internal/topology"
)

func setupTopologyTestServer(t *testing.T) (*Server, storage.Storage, topology.Service, http.Handler) {
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

	topoSvc := topology.NewService()

	// Populate topology with test nodes and dependencies
	topoSvc.AddDeclaredNode(topology.TopologyNode{
		ID:       "web-srv",
		Name:     "Web Server",
		Type:     topology.NodeTypeService,
		Status:   topology.NodeStatusHealthy,
		HostID:   "host-01",
		Source:   "declared",
		Tags:     map[string]string{"env": "prod", "tier": "frontend"},
		Metadata: map[string]string{"port": "8080"},
	})
	topoSvc.AddDeclaredNode(topology.TopologyNode{
		ID:       "api-srv",
		Name:     "API Service",
		Type:     topology.NodeTypeService,
		Status:   topology.NodeStatusDegraded,
		HostID:   "host-01",
		Source:   "declared",
		Tags:     map[string]string{"env": "prod", "tier": "backend"},
	})
	topoSvc.AddDeclaredNode(topology.TopologyNode{
		ID:       "db-srv",
		Name:     "Database Primary",
		Type:     topology.NodeTypeDatabase,
		Status:   topology.NodeStatusHealthy,
		HostID:   "host-02",
		Source:   "declared",
		Tags:     map[string]string{"env": "prod", "tier": "data"},
	})

	topoSvc.AddDeclaredDependency(topology.Dependency{
		SourceID:   "web-srv",
		TargetID:   "api-srv",
		Type:       topology.RelCommunicatesWith,
		Confidence: topology.ConfidenceHigh,
		Weight:     1.0,
	})
	topoSvc.AddDeclaredDependency(topology.Dependency{
		SourceID:   "api-srv",
		TargetID:   "db-srv",
		Type:       topology.RelDependsOn,
		Confidence: topology.ConfidenceHigh,
		Weight:     1.0,
	})

	srv := NewServer(cfg, nil, store, nil, nil, nil)
	srv.topoService = topoSvc

	mux := http.NewServeMux()
	mux.Handle("/api/v1/topology", srv.authMiddleware(http.HandlerFunc(srv.handleTopologyRoute)))
	mux.Handle("/api/v1/topology/", srv.authMiddleware(http.HandlerFunc(srv.handleTopologyRoute)))

	handler := RequestIDMiddleware(
		PanicRecoveryMiddleware(
			MaxBodySizeMiddleware(DefaultMaxRequestBodySize)(
				RequestLoggerMiddleware(mux),
			),
			srv.auditLog,
		),
	)

	return srv, store, topoSvc, handler
}

func TestServer_TopologyEndpoints(t *testing.T) {
	_, store, _, handler := setupTopologyTestServer(t)
	defer store.Close()

	t.Run("GET /api/v1/topology - 200 OK", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/topology", nil)
		req.Header.Set("Authorization", "Bearer test-secret-token")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}

		var resp topology.TopologyGraphResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode TopologyGraphResponse: %v", err)
		}
		if len(resp.Nodes) != 3 {
			t.Errorf("expected 3 nodes, got %d", len(resp.Nodes))
		}
		if len(resp.Dependencies) != 2 {
			t.Errorf("expected 2 dependencies, got %d", len(resp.Dependencies))
		}
		if resp.Summary.TotalNodes != 3 {
			t.Errorf("expected 3 total nodes in summary, got %d", resp.Summary.TotalNodes)
		}
	})

	t.Run("GET /api/v1/topology with filters - 200 OK", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/topology?type=service&status=healthy&tag:tier=frontend", nil)
		req.Header.Set("Authorization", "Bearer test-secret-token")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}

		var resp topology.TopologyGraphResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode TopologyGraphResponse: %v", err)
		}
		if len(resp.Nodes) != 1 {
			t.Errorf("expected 1 node matching filter, got %d", len(resp.Nodes))
		}
		if len(resp.Nodes) > 0 && resp.Nodes[0].ID != "web-srv" {
			t.Errorf("expected node web-srv, got %s", resp.Nodes[0].ID)
		}
	})

	t.Run("GET /api/v1/topology/summary - 200 OK", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/topology/summary", nil)
		req.Header.Set("Authorization", "Bearer test-secret-token")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}

		var summary topology.TopologyGraphSummary
		if err := json.Unmarshal(w.Body.Bytes(), &summary); err != nil {
			t.Fatalf("failed to decode summary: %v", err)
		}
		if summary.TotalNodes != 3 {
			t.Errorf("expected 3 total nodes, got %d", summary.TotalNodes)
		}
		if summary.TotalDependencies != 2 {
			t.Errorf("expected 2 total dependencies, got %d", summary.TotalDependencies)
		}
	})

	t.Run("GET /api/v1/topology/dot - 200 OK", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/topology/dot", nil)
		req.Header.Set("Authorization", "Bearer test-secret-token")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}
		if ct := w.Header().Get("Content-Type"); ct != "text/vnd.graphviz; charset=utf-8" {
			t.Errorf("unexpected content type: %s", ct)
		}
		body := w.Body.String()
		if !bytes.Contains([]byte(body), []byte("digraph")) {
			t.Errorf("expected DOT output with 'digraph', got: %s", body)
		}
	})

	t.Run("GET /api/v1/topology/path - 200 OK", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/topology/path?source=web-srv&target=db-srv", nil)
		req.Header.Set("Authorization", "Bearer test-secret-token")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}

		var path topology.DependencyPath
		if err := json.Unmarshal(w.Body.Bytes(), &path); err != nil {
			t.Fatalf("failed to decode DependencyPath: %v", err)
		}
		if len(path.Nodes) != 3 {
			t.Errorf("expected path of 3 nodes (web-srv -> api-srv -> db-srv), got %d nodes", len(path.Nodes))
		}
	})

	t.Run("GET /api/v1/topology/path - 404 Path Not Found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/topology/path?source=db-srv&target=web-srv", nil)
		req.Header.Set("Authorization", "Bearer test-secret-token")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404 Not Found, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("GET /api/v1/topology/path - 400 Missing Parameter", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/topology/path?source=web-srv", nil)
		req.Header.Set("Authorization", "Bearer test-secret-token")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("GET /api/v1/topology/spof - 200 OK", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/topology/spof?min_criticality=0.0", nil)
		req.Header.Set("Authorization", "Bearer test-secret-token")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}

		var res struct {
			SPOFs []topology.SPOFAnalysis `json:"spofs"`
			Count int                     `json:"count"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
			t.Fatalf("failed to decode SPOF list: %v", err)
		}
	})

	t.Run("GET /api/v1/topology/spof/{id} - 200 OK", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/topology/spof/api-srv", nil)
		req.Header.Set("Authorization", "Bearer test-secret-token")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}

		var spof topology.SPOFAnalysis
		if err := json.Unmarshal(w.Body.Bytes(), &spof); err != nil {
			t.Fatalf("failed to decode SPOF analysis: %v", err)
		}
		if spof.NodeID != "api-srv" {
			t.Errorf("expected node api-srv, got %s", spof.NodeID)
		}
	})

	t.Run("GET /api/v1/topology/spof/{id} - 404 Not Found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/topology/spof/non-existent", nil)
		req.Header.Set("Authorization", "Bearer test-secret-token")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404 Not Found, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("GET /api/v1/topology/impact/{id} - 200 OK", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/topology/impact/db-srv", nil)
		req.Header.Set("Authorization", "Bearer test-secret-token")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}

		var impact topology.TopologyImpactAnalysis
		if err := json.Unmarshal(w.Body.Bytes(), &impact); err != nil {
			t.Fatalf("failed to decode TopologyImpactAnalysis: %v", err)
		}
		if impact.TargetNodeID != "db-srv" {
			t.Errorf("expected db-srv, got %s", impact.TargetNodeID)
		}
		if len(impact.DirectDependents) != 1 {
			t.Errorf("expected 1 direct dependent (api-srv), got %d", len(impact.DirectDependents))
		}
		if len(impact.TransitiveDependents) != 2 {
			t.Errorf("expected 2 transitive dependents (api-srv, web-srv), got %d", len(impact.TransitiveDependents))
		}
	})

	t.Run("GET /api/v1/topology/nodes/{id} - 200 OK", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/topology/nodes/web-srv", nil)
		req.Header.Set("Authorization", "Bearer test-secret-token")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}

		var node topology.TopologyNode
		if err := json.Unmarshal(w.Body.Bytes(), &node); err != nil {
			t.Fatalf("failed to decode node: %v", err)
		}
		if node.ID != "web-srv" {
			t.Errorf("expected web-srv, got %s", node.ID)
		}
	})

	t.Run("GET /api/v1/topology/nodes/{id} - 404 Not Found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/topology/nodes/missing-node", nil)
		req.Header.Set("Authorization", "Bearer test-secret-token")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404 Not Found, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("POST /api/v1/topology/nodes - 201 Created", func(t *testing.T) {
		newNode := topology.TopologyNode{
			ID:     "redis-cache",
			Name:   "Redis Cache",
			Type:   topology.NodeTypeDatabase,
			Status: topology.NodeStatusHealthy,
		}
		body, _ := json.Marshal(newNode)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/topology/nodes", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer test-secret-token")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created, got %d: %s", w.Code, w.Body.String())
		}

		// Verify added
		getReq := httptest.NewRequest(http.MethodGet, "/api/v1/topology/nodes/redis-cache", nil)
		getReq.Header.Set("Authorization", "Bearer test-secret-token")
		getW := httptest.NewRecorder()
		handler.ServeHTTP(getW, getReq)
		if getW.Code != http.StatusOK {
			t.Fatalf("expected 200 OK on get, got %d", getW.Code)
		}
	})

	t.Run("POST /api/v1/topology/dependencies - 201 Created", func(t *testing.T) {
		newDep := topology.Dependency{
			SourceID: "web-srv",
			TargetID: "redis-cache",
			Type:     topology.RelCommunicatesWith,
		}
		body, _ := json.Marshal(newDep)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/topology/dependencies", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer test-secret-token")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("GET /api/v1/topology/dependencies/{id} - 200 OK", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/topology/dependencies/web-srv", nil)
		req.Header.Set("Authorization", "Bearer test-secret-token")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}

		var res struct {
			NodeID       string                  `json:"node_id"`
			Dependencies []topology.TopologyNode `json:"dependencies"`
			Count        int                     `json:"count"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
			t.Fatalf("failed to decode dependencies: %v", err)
		}
		if res.Count < 1 {
			t.Errorf("expected at least 1 dependency, got %d", res.Count)
		}
	})

	t.Run("GET /api/v1/topology/dependents/{id} - 200 OK", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/topology/dependents/api-srv", nil)
		req.Header.Set("Authorization", "Bearer test-secret-token")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}

		var res struct {
			NodeID     string                  `json:"node_id"`
			Dependents []topology.TopologyNode `json:"dependents"`
			Count      int                     `json:"count"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
			t.Fatalf("failed to decode dependents: %v", err)
		}
		if res.Count != 1 {
			t.Errorf("expected 1 dependent for api-srv (web-srv), got %d", res.Count)
		}
	})

	t.Run("DELETE /api/v1/topology/dependencies - 200 OK", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/topology/dependencies?source=web-srv&target=redis-cache", nil)
		req.Header.Set("Authorization", "Bearer test-secret-token")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("DELETE /api/v1/topology/nodes/{id} - 200 OK", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/topology/nodes/redis-cache", nil)
		req.Header.Set("Authorization", "Bearer test-secret-token")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("GET /api/v1/topology - 401 Unauthorized", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/topology", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 Unauthorized, got %d", w.Code)
		}
	})
}
