package topology

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTopologyClient_AllMethods(t *testing.T) {
	var receivedAuth string

	sampleNode := TopologyNode{
		ID:     "srv-api",
		Name:   "API Service",
		Type:   NodeTypeService,
		Status: NodeStatusHealthy,
	}
	sampleDep := Dependency{
		SourceID: "srv-web",
		TargetID: "srv-api",
		Type:     RelCommunicatesWith,
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")

		switch {
		case r.URL.Path == "/api/v1/topology" && r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(TopologyGraphResponse{
				Nodes:        []TopologyNode{sampleNode},
				Dependencies: []Dependency{sampleDep},
				Summary: TopologyGraphSummary{
					TotalNodes:        1,
					TotalDependencies: 1,
				},
			})

		case r.URL.Path == "/api/v1/topology/summary" && r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(TopologyGraphSummary{
				TotalNodes:        10,
				TotalDependencies: 15,
				Density:           0.15,
			})

		case r.URL.Path == "/api/v1/topology/dot" && r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "text/vnd.graphviz")
			_, _ = w.Write([]byte("digraph G { \"srv-web\" -> \"srv-api\"; }"))

		case r.URL.Path == "/api/v1/topology/path" && r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(DependencyPath{
				SourceID: "srv-web",
				TargetID: "srv-api",
				Nodes:    []TopologyNode{sampleNode},
				Hops:     1,
			})

		case r.URL.Path == "/api/v1/topology/spof" && r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"spofs": []SPOFAnalysis{
					{
						NodeID:           "srv-api",
						CriticalityScore: 85.0,
					},
				},
				"count": 1,
			})

		case r.URL.Path == "/api/v1/topology/spof/srv-api" && r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(SPOFAnalysis{
				NodeID:           "srv-api",
				CriticalityScore: 85.0,
			})

		case r.URL.Path == "/api/v1/topology/impact/srv-api" && r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(TopologyImpactAnalysis{
				TargetNodeID:     "srv-api",
				DirectDependents: []TopologyNode{sampleNode},
				BlastRadiusScore: 75.0,
			})

		case r.URL.Path == "/api/v1/topology/nodes/srv-api" && r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(sampleNode)

		case r.URL.Path == "/api/v1/topology/dependencies/srv-api" && r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"node_id":      "srv-api",
				"dependencies": []TopologyNode{},
				"count":        0,
			})

		case r.URL.Path == "/api/v1/topology/dependents/srv-api" && r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"node_id":    "srv-api",
				"dependents": []TopologyNode{sampleNode},
				"count":      1,
			})

		case r.URL.Path == "/api/v1/topology/nodes" && r.Method == http.MethodPost:
			w.Header().Set("Content-Type", "application/json")
			var node TopologyNode
			_ = json.NewDecoder(r.Body).Decode(&node)
			_ = json.NewEncoder(w).Encode(node)

		case r.URL.Path == "/api/v1/topology/dependencies" && r.Method == http.MethodPost:
			w.Header().Set("Content-Type", "application/json")
			var dep Dependency
			_ = json.NewDecoder(r.Body).Decode(&dep)
			_ = json.NewEncoder(w).Encode(dep)

		case r.URL.Path == "/api/v1/topology/nodes/srv-api" && r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusOK)

		case r.URL.Path == "/api/v1/topology/dependencies" && r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusOK)

		default:
			w.WriteHeader(http.StatusNotFound)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{
					"code":    "NOT_FOUND",
					"message": "resource not found",
				},
			})
		}
	}))
	defer ts.Close()

	client := NewClient(ClientConfig{
		Endpoint: ts.URL,
		Token:    "topology-secret-token",
		Timeout:  3 * time.Second,
	})
	ctx := context.Background()

	if client.Endpoint() != ts.URL {
		t.Errorf("expected endpoint %s, got %s", ts.URL, client.Endpoint())
	}

	// 1. GetTopology
	graphResp, err := client.GetTopology(ctx, TopologyFilter{
		Types:     []NodeType{NodeTypeService},
		Statuses:  []NodeStatus{NodeStatusHealthy},
		Source:    "declared",
		HostID:    "host-1",
		Search:    "API",
		MaxDepth:  3,
		TagFilter: map[string]string{"env": "prod"},
	})
	if err != nil {
		t.Fatalf("GetTopology failed: %v", err)
	}
	if len(graphResp.Nodes) != 1 {
		t.Errorf("unexpected GetTopology response: %+v", graphResp)
	}
	if receivedAuth != "Bearer topology-secret-token" {
		t.Errorf("expected auth Bearer topology-secret-token, got %s", receivedAuth)
	}

	// 2. GetTopologySummary
	summary, err := client.GetTopologySummary(ctx)
	if err != nil {
		t.Fatalf("GetTopologySummary failed: %v", err)
	}
	if summary.TotalNodes != 10 || summary.TotalDependencies != 15 {
		t.Errorf("unexpected GetTopologySummary response: %+v", summary)
	}

	// 3. ExportDOT
	dot, err := client.ExportDOT(ctx)
	if err != nil {
		t.Fatalf("ExportDOT failed: %v", err)
	}
	if dot != "digraph G { \"srv-web\" -> \"srv-api\"; }" {
		t.Errorf("unexpected ExportDOT response: %s", dot)
	}

	// 4. FindPath
	path, err := client.FindPath(ctx, "srv-web", "srv-api")
	if err != nil {
		t.Fatalf("FindPath failed: %v", err)
	}
	if path.SourceID != "srv-web" || path.TargetID != "srv-api" || len(path.Nodes) != 1 {
		t.Errorf("unexpected FindPath response: %+v", path)
	}

	// 5. FindAllSPOFs
	spofs, err := client.FindAllSPOFs(ctx, 50.0)
	if err != nil {
		t.Fatalf("FindAllSPOFs failed: %v", err)
	}
	if len(spofs) != 1 || spofs[0].NodeID != "srv-api" {
		t.Errorf("unexpected FindAllSPOFs response: %+v", spofs)
	}

	// 6. AnalyzeSPOF
	spof, err := client.AnalyzeSPOF(ctx, "srv-api")
	if err != nil {
		t.Fatalf("AnalyzeSPOF failed: %v", err)
	}
	if spof.NodeID != "srv-api" || spof.CriticalityScore != 85.0 {
		t.Errorf("unexpected AnalyzeSPOF response: %+v", spof)
	}

	// 7. AnalyzeImpact
	impact, err := client.AnalyzeImpact(ctx, "srv-api")
	if err != nil {
		t.Fatalf("AnalyzeImpact failed: %v", err)
	}
	if impact.TargetNodeID != "srv-api" || impact.BlastRadiusScore != 75.0 {
		t.Errorf("unexpected AnalyzeImpact response: %+v", impact)
	}

	// 8. GetNode
	node, err := client.GetNode(ctx, "srv-api")
	if err != nil {
		t.Fatalf("GetNode failed: %v", err)
	}
	if node.ID != "srv-api" {
		t.Errorf("expected node ID srv-api, got %s", node.ID)
	}

	// 9. GetDependencies
	deps, err := client.GetDependencies(ctx, "srv-api")
	if err != nil {
		t.Fatalf("GetDependencies failed: %v", err)
	}
	if len(deps) != 0 {
		t.Errorf("expected 0 dependencies, got %d", len(deps))
	}

	// 10. GetDependents
	dependents, err := client.GetDependents(ctx, "srv-api")
	if err != nil {
		t.Fatalf("GetDependents failed: %v", err)
	}
	if len(dependents) != 1 {
		t.Errorf("expected 1 dependent, got %d", len(dependents))
	}

	// 11. AddDeclaredNode
	createdNode, err := client.AddDeclaredNode(ctx, sampleNode)
	if err != nil {
		t.Fatalf("AddDeclaredNode failed: %v", err)
	}
	if createdNode.ID != "srv-api" {
		t.Errorf("expected node ID srv-api, got %s", createdNode.ID)
	}

	// 12. AddDeclaredDependency
	createdDep, err := client.AddDeclaredDependency(ctx, sampleDep)
	if err != nil {
		t.Fatalf("AddDeclaredDependency failed: %v", err)
	}
	if createdDep.SourceID != "srv-web" || createdDep.TargetID != "srv-api" {
		t.Errorf("unexpected AddDeclaredDependency response: %+v", createdDep)
	}

	// 13. RemoveNode
	if err := client.RemoveNode(ctx, "srv-api"); err != nil {
		t.Fatalf("RemoveNode failed: %v", err)
	}

	// 14. RemoveDependency
	if err := client.RemoveDependency(ctx, "srv-web", "srv-api", RelCommunicatesWith); err != nil {
		t.Fatalf("RemoveDependency failed: %v", err)
	}

	// Validation error checks
	if _, err := client.FindPath(ctx, "", "srv-api"); err == nil {
		t.Errorf("expected error on empty source in FindPath")
	}
	if _, err := client.AnalyzeSPOF(ctx, ""); err == nil {
		t.Errorf("expected error on empty nodeID in AnalyzeSPOF")
	}
	if _, err := client.AnalyzeImpact(ctx, ""); err == nil {
		t.Errorf("expected error on empty nodeID in AnalyzeImpact")
	}
	if _, err := client.GetNode(ctx, ""); err == nil {
		t.Errorf("expected error on empty nodeID in GetNode")
	}
	if _, err := client.GetDependencies(ctx, ""); err == nil {
		t.Errorf("expected error on empty nodeID in GetDependencies")
	}
	if _, err := client.GetDependents(ctx, ""); err == nil {
		t.Errorf("expected error on empty nodeID in GetDependents")
	}
	if err := client.RemoveNode(ctx, ""); err == nil {
		t.Errorf("expected error on empty nodeID in RemoveNode")
	}
	if err := client.RemoveDependency(ctx, "", "srv-api", RelCommunicatesWith); err == nil {
		t.Errorf("expected error on empty sourceID in RemoveDependency")
	}

	// 404 error check
	if _, err := client.GetNode(ctx, "unknown-node"); err == nil {
		t.Errorf("expected error for unknown node")
	}
}
