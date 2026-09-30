package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DocHoax/watchdog/internal/config"
	"github.com/DocHoax/watchdog/internal/topology"
	"github.com/DocHoax/watchdog/pkg/model"
	"gopkg.in/yaml.v3"
)

func resetTopologyFlags() {
	topoServerURL = ""
	topoToken = ""
	topoTokenFile = ""
	topoTokenEnv = ""
	topoInsecureTLS = false
	topoTimeout = 10 * time.Second
	topoFormat = "text"
	topoTypes = nil
	topoStatuses = nil
	topoSource = ""
	topoHostID = ""
	topoSearch = ""
	topoMaxDepth = 0
	topoTags = nil
	topoMinCriticality = 0.0
	topoOutFile = ""

	topoNodeID = ""
	topoNodeName = ""
	topoNodeType = "service"
	topoNodeStatus = "healthy"
	topoNodeHostID = ""
	topoNodeTags = nil
	topoNodeMetadata = nil

	topoDepSource = ""
	topoDepTarget = ""
	topoDepType = "depends_on"
	topoDepConfidence = "high"
	topoDepWeight = 1.0
	topoDepDependents = false

	globalCfg = config.DefaultConfig()
}

func setupTopologyMockServer(t *testing.T) *httptest.Server {
	t.Helper()
	now := time.Date(2026, 9, 28, 10, 0, 0, 0, time.UTC)

	sampleNodes := []topology.TopologyNode{
		{
			ID:            "web-srv",
			Name:          "Web Frontend",
			Type:          topology.NodeTypeService,
			Status:        topology.NodeStatusHealthy,
			HostID:        "host-01",
			Source:        "declared",
			Tags:          map[string]string{"env": "prod", "tier": "frontend"},
			Metadata:      map[string]string{"port": "8080"},
			FirstObserved: now.Add(-24 * time.Hour),
			LastObserved:  now,
		},
		{
			ID:            "api-srv",
			Name:          "API Gateway",
			Type:          topology.NodeTypeService,
			Status:        topology.NodeStatusDegraded,
			HostID:        "host-01",
			Source:        "telemetry",
			Tags:          map[string]string{"env": "prod", "tier": "backend"},
			Metadata:      map[string]string{"version": "v2.1.0"},
			FirstObserved: now.Add(-48 * time.Hour),
			LastObserved:  now,
		},
		{
			ID:            "db-primary",
			Name:          "Primary Postgres DB",
			Type:          topology.NodeTypeDatabase,
			Status:        topology.NodeStatusHealthy,
			HostID:        "host-02",
			Source:        "declared",
			Tags:          map[string]string{"env": "prod", "tier": "data"},
			Metadata:      map[string]string{"engine": "postgres"},
			FirstObserved: now.Add(-72 * time.Hour),
			LastObserved:  now,
		},
	}

	sampleDeps := []topology.Dependency{
		{
			SourceID:      "web-srv",
			TargetID:      "api-srv",
			Type:          topology.RelCommunicatesWith,
			Confidence:    topology.ConfidenceHigh,
			Weight:        1.0,
			FirstObserved: now.Add(-24 * time.Hour),
			LastObserved:  now,
		},
		{
			SourceID:      "api-srv",
			TargetID:      "db-primary",
			Type:          topology.RelDependsOn,
			Confidence:    topology.ConfidenceHigh,
			Weight:        1.5,
			FirstObserved: now.Add(-48 * time.Hour),
			LastObserved:  now,
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v1/topology/summary" && r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(topology.TopologyGraphSummary{
				TotalNodes:        3,
				TotalDependencies: 2,
				NodeTypeCounts: map[string]int{
					"service":  2,
					"database": 1,
				},
				StatusCounts: map[string]int{
					"healthy":  2,
					"degraded": 1,
				},
				RelationshipTypeCounts: map[string]int{
					"communicates_with": 1,
					"depends_on":        1,
				},
				Density:             0.3333,
				ConnectedComponents: 1,
				EvaluatedAt:         now,
			})

		case r.URL.Path == "/api/v1/topology/dot" && r.Method == http.MethodGet:
			dotNotation := "digraph WatchdogTopology {\n  \"web-srv\" -> \"api-srv\";\n  \"api-srv\" -> \"db-primary\";\n}\n"
			w.Header().Set("Content-Type", "text/vnd.graphviz")
			_, _ = w.Write([]byte(dotNotation))

		case r.URL.Path == "/api/v1/topology/path" && r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			src := r.URL.Query().Get("source")
			dst := r.URL.Query().Get("target")
			if src == "web-srv" && dst == "db-primary" {
				_ = json.NewEncoder(w).Encode(topology.DependencyPath{
					SourceID:    "web-srv",
					TargetID:    "db-primary",
					Hops:        2,
					TotalWeight: 2.5,
					Nodes: []topology.TopologyNode{
						sampleNodes[0],
						sampleNodes[1],
						sampleNodes[2],
					},
					Edges: sampleDeps,
				})
			} else {
				w.WriteHeader(http.StatusNotFound)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"error": map[string]any{
						"code":    "PATH_NOT_FOUND",
						"message": fmt.Sprintf("no path found between %s and %s", src, dst),
					},
				})
			}

		case r.URL.Path == "/api/v1/topology/spof" && r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			minCrit := r.URL.Query().Get("min_criticality")
			if minCrit == "0.990000" || minCrit == "0.99" {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"spofs": []topology.SPOFAnalysis{},
					"count": 0,
				})
			} else {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"spofs": []topology.SPOFAnalysis{
						{
							NodeID:                    "api-srv",
							NodeName:                  "API Gateway",
							NodeType:                  topology.NodeTypeService,
							Status:                    topology.NodeStatusDegraded,
							CriticalityScore:          0.85,
							RiskLevel:                 model.SeverityCritical,
							RedundancyLevel:           topology.RedundancyNone,
							DependentsCount:           1,
							TransitiveDependentsCount: 1,
							AlternativePaths:          0,
							AffectedServices:          []string{"web-srv"},
							Reasoning:                 []string{"Single gateway node with no redundant failover route"},
						},
					},
					"count": 1,
				})
			}

		case strings.HasPrefix(r.URL.Path, "/api/v1/topology/spof/") && r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			nodeID := strings.TrimPrefix(r.URL.Path, "/api/v1/topology/spof/")
			if nodeID == "api-srv" {
				_ = json.NewEncoder(w).Encode(topology.SPOFAnalysis{
					NodeID:                    "api-srv",
					NodeName:                  "API Gateway",
					NodeType:                  topology.NodeTypeService,
					Status:                    topology.NodeStatusDegraded,
					CriticalityScore:          0.85,
					RiskLevel:                 model.SeverityCritical,
					RedundancyLevel:           topology.RedundancyNone,
					DependentsCount:           1,
					TransitiveDependentsCount: 1,
					AlternativePaths:          0,
					AffectedServices:          []string{"web-srv"},
					Reasoning:                 []string{"Single gateway node with no redundant failover route"},
				})
			} else {
				w.WriteHeader(http.StatusNotFound)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"error": map[string]any{
						"code":    "NODE_NOT_FOUND",
						"message": fmt.Sprintf("node %s not found", nodeID),
					},
				})
			}

		case strings.HasPrefix(r.URL.Path, "/api/v1/topology/impact/") && r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			nodeID := strings.TrimPrefix(r.URL.Path, "/api/v1/topology/impact/")
			if nodeID == "db-primary" {
				_ = json.NewEncoder(w).Encode(topology.TopologyImpactAnalysis{
					TargetNodeID:         "db-primary",
					TargetNodeName:       "Primary Postgres DB",
					TargetNodeType:       topology.NodeTypeDatabase,
					TargetNodeStatus:     topology.NodeStatusHealthy,
					DirectDependents:     []topology.TopologyNode{sampleNodes[1]},
					TransitiveDependents: []topology.TopologyNode{sampleNodes[1], sampleNodes[0]},
					MaxImpactDepth:       2,
					BlastRadiusScore:     0.75,
					BlastRadiusLevel:     topology.BlastRadiusHigh,
					Summary:              "Outage impacts 2 upstream services",
					AnalyzedAt:           now,
				})
			} else {
				w.WriteHeader(http.StatusNotFound)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"error": map[string]any{
						"code":    "NODE_NOT_FOUND",
						"message": fmt.Sprintf("node %s not found", nodeID),
					},
				})
			}

		case strings.HasPrefix(r.URL.Path, "/api/v1/topology/nodes/") && r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			nodeID := strings.TrimPrefix(r.URL.Path, "/api/v1/topology/nodes/")
			for _, n := range sampleNodes {
				if n.ID == nodeID {
					_ = json.NewEncoder(w).Encode(n)
					return
				}
			}
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{
					"code":    "NODE_NOT_FOUND",
					"message": fmt.Sprintf("node %s not found", nodeID),
				},
			})

		case r.URL.Path == "/api/v1/topology/nodes" && r.Method == http.MethodPost:
			w.Header().Set("Content-Type", "application/json")
			var node topology.TopologyNode
			if err := json.NewDecoder(r.Body).Decode(&node); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			node.FirstObserved = now
			node.LastObserved = now
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(node)

		case strings.HasPrefix(r.URL.Path, "/api/v1/topology/nodes/") && r.Method == http.MethodDelete:
			w.Header().Set("Content-Type", "application/json")
			nodeID := strings.TrimPrefix(r.URL.Path, "/api/v1/topology/nodes/")
			if nodeID == "non-existent" {
				w.WriteHeader(http.StatusNotFound)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"error": map[string]any{
						"code":    "NODE_NOT_FOUND",
						"message": fmt.Sprintf("node %s not found", nodeID),
					},
				})
				return
			}
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})

		case strings.HasPrefix(r.URL.Path, "/api/v1/topology/dependencies/") && r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			nodeID := strings.TrimPrefix(r.URL.Path, "/api/v1/topology/dependencies/")
			switch nodeID {
			case "web-srv":
				_ = json.NewEncoder(w).Encode(map[string]any{
					"node_id":      "web-srv",
					"dependencies": []topology.TopologyNode{sampleNodes[1]},
					"count":        1,
				})
			case "db-primary":
				_ = json.NewEncoder(w).Encode(map[string]any{
					"node_id":      "db-primary",
					"dependencies": []topology.TopologyNode{},
					"count":        0,
				})
			default:
				w.WriteHeader(http.StatusNotFound)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"error": map[string]any{
						"code":    "NODE_NOT_FOUND",
						"message": fmt.Sprintf("node %s not found", nodeID),
					},
				})
			}

		case strings.HasPrefix(r.URL.Path, "/api/v1/topology/dependents/") && r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			nodeID := strings.TrimPrefix(r.URL.Path, "/api/v1/topology/dependents/")
			switch nodeID {
			case "api-srv":
				_ = json.NewEncoder(w).Encode(map[string]any{
					"node_id":    "api-srv",
					"dependents": []topology.TopologyNode{sampleNodes[0]},
					"count":      1,
				})
			case "web-srv":
				_ = json.NewEncoder(w).Encode(map[string]any{
					"node_id":    "web-srv",
					"dependents": []topology.TopologyNode{},
					"count":      0,
				})
			default:
				w.WriteHeader(http.StatusNotFound)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"error": map[string]any{
						"code":    "NODE_NOT_FOUND",
						"message": fmt.Sprintf("node %s not found", nodeID),
					},
				})
			}

		case r.URL.Path == "/api/v1/topology/dependencies" && r.Method == http.MethodPost:
			w.Header().Set("Content-Type", "application/json")
			var dep topology.Dependency
			if err := json.NewDecoder(r.Body).Decode(&dep); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			dep.FirstObserved = now
			dep.LastObserved = now
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(dep)

		case r.URL.Path == "/api/v1/topology/dependencies" && r.Method == http.MethodDelete:
			w.Header().Set("Content-Type", "application/json")
			src := r.URL.Query().Get("source")
			dst := r.URL.Query().Get("target")
			if src == "web-srv" && dst == "api-srv" {
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
			} else {
				w.WriteHeader(http.StatusNotFound)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"error": map[string]any{
						"code":    "DEPENDENCY_NOT_FOUND",
						"message": fmt.Sprintf("dependency from %s to %s not found", src, dst),
					},
				})
			}

		case r.URL.Path == "/api/v1/topology" && r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			search := r.URL.Query().Get("search")
			if search == "nonexistent" {
				_ = json.NewEncoder(w).Encode(topology.TopologyGraphResponse{
					Nodes:        []topology.TopologyNode{},
					Dependencies: []topology.Dependency{},
				})
			} else {
				_ = json.NewEncoder(w).Encode(topology.TopologyGraphResponse{
					Nodes:        sampleNodes,
					Dependencies: sampleDeps,
				})
			}

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

func TestCmd_Topology_MissingServerURL(t *testing.T) {
	resetTopologyFlags()
	globalCfg = nil

	_, err := getTopologyClient()
	if err == nil {
		t.Fatalf("expected error when server URL is not configured, got nil")
	}
	if !strings.Contains(err.Error(), "fleet server URL is required") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestCmd_Topology_TokenResolution(t *testing.T) {
	resetTopologyFlags()
	ts := setupTopologyMockServer(t)
	defer ts.Close()

	// 1. Token from CLI flag
	topoServerURL = ts.URL
	topoToken = "cli-token"
	client, err := getTopologyClient()
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	if client.Endpoint() != ts.URL {
		t.Errorf("expected endpoint %q, got %q", ts.URL, client.Endpoint())
	}

	// 2. Token from environment variable
	resetTopologyFlags()
	topoServerURL = ts.URL
	topoTokenEnv = "TEST_TOPO_TOKEN_ENV"
	t.Setenv("TEST_TOPO_TOKEN_ENV", "env-secret-token")
	client, err = getTopologyClient()
	if err != nil {
		t.Fatalf("failed to resolve token from env: %v", err)
	}
	if client == nil {
		t.Fatal("expected client to be non-nil")
	}

	// 3. Token from token file
	resetTopologyFlags()
	topoServerURL = ts.URL
	tmpFile := filepath.Join(t.TempDir(), "token.txt")
	if err := os.WriteFile(tmpFile, []byte("file-secret-token\n"), 0600); err != nil {
		t.Fatalf("failed to write token file: %v", err)
	}
	topoTokenFile = tmpFile
	client, err = getTopologyClient()
	if err != nil {
		t.Fatalf("failed to resolve token from file: %v", err)
	}
	if client == nil {
		t.Fatal("expected client to be non-nil")
	}

	// 4. Insecure TLS
	resetTopologyFlags()
	topoServerURL = ts.URL
	topoInsecureTLS = true
	client, err = getTopologyClient()
	if err != nil {
		t.Fatalf("failed with insecure TLS: %v", err)
	}
	if client == nil {
		t.Fatal("expected client to be non-nil")
	}
}

func TestCmd_Topology_Graph(t *testing.T) {
	resetTopologyFlags()
	ts := setupTopologyMockServer(t)
	defer ts.Close()

	topoServerURL = ts.URL

	// 1. Text output
	out, err := captureStdout(func() error {
		return runTopologyGraph(topologyGraphCmd, nil)
	})
	if err != nil {
		t.Fatalf("runTopologyGraph failed: %v", err)
	}

	if !strings.Contains(out, "Topology Graph (3 Nodes, 2 Dependencies)") {
		t.Errorf("expected header in text output, got: %s", out)
	}
	if !strings.Contains(out, "web-srv") || !strings.Contains(out, "api-srv") || !strings.Contains(out, "db-primary") {
		t.Errorf("expected node IDs in output, got: %s", out)
	}
	if !strings.Contains(out, "communicates_with") || !strings.Contains(out, "depends_on") {
		t.Errorf("expected dependency types in output, got: %s", out)
	}

	// 2. JSON output
	topoFormat = "json"
	outJSON, err := captureStdout(func() error {
		return runTopologyGraph(topologyGraphCmd, nil)
	})
	if err != nil {
		t.Fatalf("runTopologyGraph JSON failed: %v", err)
	}
	var graphResp topology.TopologyGraphResponse
	if err := json.Unmarshal([]byte(outJSON), &graphResp); err != nil {
		t.Fatalf("failed to decode JSON response: %v", err)
	}
	if len(graphResp.Nodes) != 3 || len(graphResp.Dependencies) != 2 {
		t.Errorf("unexpected graph response: %+v", graphResp)
	}

	// 3. YAML output
	topoFormat = "yaml"
	outYAML, err := captureStdout(func() error {
		return runTopologyGraph(topologyGraphCmd, nil)
	})
	if err != nil {
		t.Fatalf("runTopologyGraph YAML failed: %v", err)
	}
	var yamlResp topology.TopologyGraphResponse
	if err := yaml.Unmarshal([]byte(outYAML), &yamlResp); err != nil {
		t.Fatalf("failed to decode YAML response: %v", err)
	}
	if len(yamlResp.Nodes) != 3 {
		t.Errorf("unexpected YAML decoded nodes: %+v", yamlResp)
	}

	// 4. Empty result
	topoFormat = "text"
	topoSearch = "nonexistent"
	outEmpty, err := captureStdout(func() error {
		return runTopologyGraph(topologyGraphCmd, nil)
	})
	if err != nil {
		t.Fatalf("runTopologyGraph empty search failed: %v", err)
	}
	if !strings.Contains(outEmpty, "No topology nodes found matching the specified filters.") {
		t.Errorf("expected empty message, got: %s", outEmpty)
	}
}

func TestCmd_Topology_Summary(t *testing.T) {
	resetTopologyFlags()
	ts := setupTopologyMockServer(t)
	defer ts.Close()

	topoServerURL = ts.URL

	// 1. Text output
	out, err := captureStdout(func() error {
		return runTopologySummary(topologySummaryCmd, nil)
	})
	if err != nil {
		t.Fatalf("runTopologySummary failed: %v", err)
	}

	if !strings.Contains(out, "Topology Graph Summary") {
		t.Errorf("expected summary header, got: %s", out)
	}
	if !strings.Contains(out, "Total Nodes:          3") {
		t.Errorf("expected total nodes 3, got: %s", out)
	}
	if !strings.Contains(out, "Total Dependencies:   2") {
		t.Errorf("expected total deps 2, got: %s", out)
	}

	// 2. JSON output
	topoFormat = "json"
	outJSON, err := captureStdout(func() error {
		return runTopologySummary(topologySummaryCmd, nil)
	})
	if err != nil {
		t.Fatalf("runTopologySummary JSON failed: %v", err)
	}
	var summary topology.TopologyGraphSummary
	if err := json.Unmarshal([]byte(outJSON), &summary); err != nil {
		t.Fatalf("failed to decode summary JSON: %v", err)
	}
	if summary.TotalNodes != 3 || summary.TotalDependencies != 2 {
		t.Errorf("unexpected summary data: %+v", summary)
	}
}

func TestCmd_Topology_DOT(t *testing.T) {
	resetTopologyFlags()
	ts := setupTopologyMockServer(t)
	defer ts.Close()

	topoServerURL = ts.URL

	// 1. Stdout output
	out, err := captureStdout(func() error {
		return runTopologyDOT(topologyDOTCmd, nil)
	})
	if err != nil {
		t.Fatalf("runTopologyDOT failed: %v", err)
	}

	if !strings.Contains(out, "digraph WatchdogTopology") {
		t.Errorf("expected DOT digraph in stdout, got: %s", out)
	}

	// 2. Output to file
	outFile := filepath.Join(t.TempDir(), "topo.dot")
	topoOutFile = outFile
	outMsg, err := captureStdout(func() error {
		return runTopologyDOT(topologyDOTCmd, nil)
	})
	if err != nil {
		t.Fatalf("runTopologyDOT with --out failed: %v", err)
	}

	if !strings.Contains(outMsg, "Graphviz DOT notation written to") {
		t.Errorf("expected confirmation message, got: %s", outMsg)
	}

	fileContent, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("failed to read output DOT file: %v", err)
	}
	if !strings.Contains(string(fileContent), "digraph WatchdogTopology") {
		t.Errorf("unexpected DOT file content: %s", string(fileContent))
	}
}

func TestCmd_Topology_Path(t *testing.T) {
	resetTopologyFlags()
	ts := setupTopologyMockServer(t)
	defer ts.Close()

	topoServerURL = ts.URL

	// 1. Text format
	out, err := captureStdout(func() error {
		return runTopologyPath(topologyPathCmd, []string{"web-srv", "db-primary"})
	})
	if err != nil {
		t.Fatalf("runTopologyPath failed: %v", err)
	}

	if !strings.Contains(out, "Shortest Dependency Path: web-srv ➔ db-primary") {
		t.Errorf("expected path header, got: %s", out)
	}
	if !strings.Contains(out, "Length (Hops):   2") {
		t.Errorf("expected 2 hops, got: %s", out)
	}
	if !strings.Contains(out, "web-srv ➔ api-srv ➔ db-primary") {
		t.Errorf("expected route string, got: %s", out)
	}

	// 2. JSON format
	topoFormat = "json"
	outJSON, err := captureStdout(func() error {
		return runTopologyPath(topologyPathCmd, []string{"web-srv", "db-primary"})
	})
	if err != nil {
		t.Fatalf("runTopologyPath JSON failed: %v", err)
	}
	var pathRes topology.DependencyPath
	if err := json.Unmarshal([]byte(outJSON), &pathRes); err != nil {
		t.Fatalf("failed to decode path JSON: %v", err)
	}
	if pathRes.Hops != 2 || pathRes.SourceID != "web-srv" || pathRes.TargetID != "db-primary" {
		t.Errorf("unexpected path result: %+v", pathRes)
	}

	// 3. Path not found
	topoFormat = "text"
	_, err = captureStdout(func() error {
		return runTopologyPath(topologyPathCmd, []string{"web-srv", "unknown-target"})
	})
	if err == nil {
		t.Fatalf("expected error for non-existent path, got nil")
	}
}

func TestCmd_Topology_SPOF(t *testing.T) {
	resetTopologyFlags()
	ts := setupTopologyMockServer(t)
	defer ts.Close()

	topoServerURL = ts.URL

	// 1. List all SPOFs (text)
	outAll, err := captureStdout(func() error {
		return runTopologySPOF(topologySPOFCmd, nil)
	})
	if err != nil {
		t.Fatalf("runTopologySPOF all failed: %v", err)
	}

	if !strings.Contains(outAll, "Single Points of Failure (1 Detected") {
		t.Errorf("expected SPOF table header, got: %s", outAll)
	}
	if !strings.Contains(outAll, "api-srv") {
		t.Errorf("expected api-srv in SPOF output, got: %s", outAll)
	}

	// 2. List all SPOFs (JSON)
	topoFormat = "json"
	outJSON, err := captureStdout(func() error {
		return runTopologySPOF(topologySPOFCmd, nil)
	})
	if err != nil {
		t.Fatalf("runTopologySPOF all JSON failed: %v", err)
	}
	var spofList []topology.SPOFAnalysis
	if err := json.Unmarshal([]byte(outJSON), &spofList); err != nil {
		t.Fatalf("failed to decode SPOF list JSON: %v", err)
	}
	if len(spofList) != 1 || spofList[0].NodeID != "api-srv" {
		t.Errorf("unexpected SPOF list: %+v", spofList)
	}

	// 3. List with high min-criticality (empty result)
	topoFormat = "text"
	topoMinCriticality = 0.99
	outEmpty, err := captureStdout(func() error {
		return runTopologySPOF(topologySPOFCmd, nil)
	})
	if err != nil {
		t.Fatalf("runTopologySPOF empty failed: %v", err)
	}
	if !strings.Contains(outEmpty, "No single points of failure detected matching criticality >= 0.99.") {
		t.Errorf("expected empty SPOF message, got: %s", outEmpty)
	}

	// 4. Single node SPOF inspection (text)
	resetTopologyFlags()
	topoServerURL = ts.URL
	outNode, err := captureStdout(func() error {
		return runTopologySPOF(topologySPOFCmd, []string{"api-srv"})
	})
	if err != nil {
		t.Fatalf("runTopologySPOF node failed: %v", err)
	}
	if !strings.Contains(outNode, "SPOF Analysis for Node: api-srv (API Gateway)") {
		t.Errorf("expected single node SPOF header, got: %s", outNode)
	}
	if !strings.Contains(outNode, "Criticality Score:          0.85 / 1.00") {
		t.Errorf("expected criticality score in output, got: %s", outNode)
	}

	// 5. Single node SPOF inspection (JSON)
	topoFormat = "json"
	outNodeJSON, err := captureStdout(func() error {
		return runTopologySPOF(topologySPOFCmd, []string{"api-srv"})
	})
	if err != nil {
		t.Fatalf("runTopologySPOF node JSON failed: %v", err)
	}
	var nodeSPOF topology.SPOFAnalysis
	if err := json.Unmarshal([]byte(outNodeJSON), &nodeSPOF); err != nil {
		t.Fatalf("failed to decode single node SPOF JSON: %v", err)
	}
	if nodeSPOF.NodeID != "api-srv" || nodeSPOF.CriticalityScore != 0.85 {
		t.Errorf("unexpected single node SPOF data: %+v", nodeSPOF)
	}

	// 6. Unknown node SPOF
	_, err = captureStdout(func() error {
		return runTopologySPOF(topologySPOFCmd, []string{"unknown-node"})
	})
	if err == nil {
		t.Fatalf("expected error for unknown node SPOF, got nil")
	}
}

func TestCmd_Topology_Impact(t *testing.T) {
	resetTopologyFlags()
	ts := setupTopologyMockServer(t)
	defer ts.Close()

	topoServerURL = ts.URL

	// 1. Text output
	out, err := captureStdout(func() error {
		return runTopologyImpact(topologyImpactCmd, []string{"db-primary"})
	})
	if err != nil {
		t.Fatalf("runTopologyImpact failed: %v", err)
	}

	if !strings.Contains(out, "Upstream Blast Radius Analysis for Node: db-primary (Primary Postgres DB)") {
		t.Errorf("expected impact header, got: %s", out)
	}
	if !strings.Contains(out, "Total Blast Radius Nodes:   2") {
		t.Errorf("expected total blast radius nodes 2, got: %s", out)
	}
	if !strings.Contains(out, "Blast Radius Score:         0.75") {
		t.Errorf("expected score 0.75, got: %s", out)
	}

	// 2. JSON output
	topoFormat = "json"
	outJSON, err := captureStdout(func() error {
		return runTopologyImpact(topologyImpactCmd, []string{"db-primary"})
	})
	if err != nil {
		t.Fatalf("runTopologyImpact JSON failed: %v", err)
	}
	var impact topology.TopologyImpactAnalysis
	if err := json.Unmarshal([]byte(outJSON), &impact); err != nil {
		t.Fatalf("failed to decode impact JSON: %v", err)
	}
	if impact.TargetNodeID != "db-primary" || len(impact.TransitiveDependents) != 2 {
		t.Errorf("unexpected impact assessment: %+v", impact)
	}

	// 3. Unknown node impact
	topoFormat = "text"
	_, err = captureStdout(func() error {
		return runTopologyImpact(topologyImpactCmd, []string{"non-existent"})
	})
	if err == nil {
		t.Fatalf("expected error for unknown node impact, got nil")
	}
}

func TestCmd_Topology_Nodes(t *testing.T) {
	resetTopologyFlags()
	ts := setupTopologyMockServer(t)
	defer ts.Close()

	topoServerURL = ts.URL

	// 1. Nodes Get (text)
	outGet, err := captureStdout(func() error {
		return runTopologyNodeGet(topologyNodesGetCmd, []string{"web-srv"})
	})
	if err != nil {
		t.Fatalf("runTopologyNodeGet failed: %v", err)
	}
	if !strings.Contains(outGet, "Topology Node: web-srv") {
		t.Errorf("expected node get header, got: %s", outGet)
	}
	if !strings.Contains(outGet, "Name:        Web Frontend") {
		t.Errorf("expected name in output, got: %s", outGet)
	}

	// 2. Nodes Get (JSON)
	topoFormat = "json"
	outGetJSON, err := captureStdout(func() error {
		return runTopologyNodeGet(topologyNodesGetCmd, []string{"web-srv"})
	})
	if err != nil {
		t.Fatalf("runTopologyNodeGet JSON failed: %v", err)
	}
	var node topology.TopologyNode
	if err := json.Unmarshal([]byte(outGetJSON), &node); err != nil {
		t.Fatalf("failed to decode node JSON: %v", err)
	}
	if node.ID != "web-srv" || node.Type != topology.NodeTypeService {
		t.Errorf("unexpected node data: %+v", node)
	}

	// 3. Nodes Add (text)
	resetTopologyFlags()
	topoServerURL = ts.URL
	topoNodeID = "redis-cache"
	topoNodeName = "Redis Cache"
	topoNodeType = "cache"
	topoNodeStatus = "healthy"
	topoNodeHostID = "host-03"
	topoNodeTags = []string{"env=prod", "cluster=main"}
	topoNodeMetadata = []string{"port=6379", "maxmemory=4gb"}

	outAdd, err := captureStdout(func() error {
		return runTopologyNodeAdd(topologyNodesAddCmd, nil)
	})
	if err != nil {
		t.Fatalf("runTopologyNodeAdd failed: %v", err)
	}
	if !strings.Contains(outAdd, "Successfully declared node \"redis-cache\"") {
		t.Errorf("expected node add confirmation, got: %s", outAdd)
	}

	// 4. Nodes Add (JSON)
	topoFormat = "json"
	outAddJSON, err := captureStdout(func() error {
		return runTopologyNodeAdd(topologyNodesAddCmd, nil)
	})
	if err != nil {
		t.Fatalf("runTopologyNodeAdd JSON failed: %v", err)
	}
	var addedNode topology.TopologyNode
	if err := json.Unmarshal([]byte(outAddJSON), &addedNode); err != nil {
		t.Fatalf("failed to decode added node JSON: %v", err)
	}
	if addedNode.ID != "redis-cache" || addedNode.Type != topology.NodeTypeCache {
		t.Errorf("unexpected added node data: %+v", addedNode)
	}

	// 5. Nodes Delete (success)
	resetTopologyFlags()
	topoServerURL = ts.URL
	outDel, err := captureStdout(func() error {
		return runTopologyNodeDelete(topologyNodesDeleteCmd, []string{"web-srv"})
	})
	if err != nil {
		t.Fatalf("runTopologyNodeDelete failed: %v", err)
	}
	if !strings.Contains(outDel, "Successfully removed node \"web-srv\"") {
		t.Errorf("expected delete confirmation, got: %s", outDel)
	}

	// 6. Nodes Delete (error on non-existent)
	_, err = captureStdout(func() error {
		return runTopologyNodeDelete(topologyNodesDeleteCmd, []string{"non-existent"})
	})
	if err == nil {
		t.Fatalf("expected error deleting non-existent node, got nil")
	}
}

func TestCmd_Topology_Dependencies(t *testing.T) {
	resetTopologyFlags()
	ts := setupTopologyMockServer(t)
	defer ts.Close()

	topoServerURL = ts.URL

	// 1. Dependencies list downstream (text)
	outList, err := captureStdout(func() error {
		return runTopologyDependenciesList(topologyDependenciesListCmd, []string{"web-srv"})
	})
	if err != nil {
		t.Fatalf("runTopologyDependenciesList failed: %v", err)
	}
	if !strings.Contains(outList, "Downstream Dependencies of Node \"web-srv\" (1):") {
		t.Errorf("expected downstream header, got: %s", outList)
	}
	if !strings.Contains(outList, "api-srv") {
		t.Errorf("expected api-srv in dependencies list, got: %s", outList)
	}

	// 2. Dependencies list downstream (empty)
	outEmpty, err := captureStdout(func() error {
		return runTopologyDependenciesList(topologyDependenciesListCmd, []string{"db-primary"})
	})
	if err != nil {
		t.Fatalf("runTopologyDependenciesList empty failed: %v", err)
	}
	if !strings.Contains(outEmpty, "Node \"db-primary\" has no downstream dependencies.") {
		t.Errorf("expected empty message, got: %s", outEmpty)
	}

	// 3. Dependencies list upstream / dependents (text)
	topoDepDependents = true
	outDependents, err := captureStdout(func() error {
		return runTopologyDependenciesList(topologyDependenciesListCmd, []string{"api-srv"})
	})
	if err != nil {
		t.Fatalf("runTopologyDependenciesList dependents failed: %v", err)
	}
	if !strings.Contains(outDependents, "Upstream Dependents of Node \"api-srv\" (1):") {
		t.Errorf("expected dependents header, got: %s", outDependents)
	}
	if !strings.Contains(outDependents, "web-srv") {
		t.Errorf("expected web-srv in dependents list, got: %s", outDependents)
	}

	// 4. Dependencies list upstream (empty)
	topoDepDependents = true
	outDependentsEmpty, err := captureStdout(func() error {
		return runTopologyDependenciesList(topologyDependenciesListCmd, []string{"web-srv"})
	})
	if err != nil {
		t.Fatalf("runTopologyDependenciesList dependents empty failed: %v", err)
	}
	if !strings.Contains(outDependentsEmpty, "No upstream dependents rely on node \"web-srv\".") {
		t.Errorf("expected no dependents message, got: %s", outDependentsEmpty)
	}

	// 5. Dependency Add (text)
	resetTopologyFlags()
	topoServerURL = ts.URL
	topoDepSource = "web-srv"
	topoDepTarget = "redis-cache"
	topoDepType = "communicates_with"
	topoDepConfidence = "high"
	topoDepWeight = 1.0

	outDepAdd, err := captureStdout(func() error {
		return runTopologyDependencyAdd(topologyDependenciesAddCmd, nil)
	})
	if err != nil {
		t.Fatalf("runTopologyDependencyAdd failed: %v", err)
	}
	if !strings.Contains(outDepAdd, "Successfully declared dependency: web-srv ──[communicates_with]──> redis-cache") {
		t.Errorf("expected dependency add confirmation, got: %s", outDepAdd)
	}

	// 6. Dependency Delete (success)
	resetTopologyFlags()
	topoServerURL = ts.URL
	topoDepSource = "web-srv"
	topoDepTarget = "api-srv"
	outDepDel, err := captureStdout(func() error {
		return runTopologyDependencyDelete(topologyDependenciesDeleteCmd, nil)
	})
	if err != nil {
		t.Fatalf("runTopologyDependencyDelete failed: %v", err)
	}
	if !strings.Contains(outDepDel, "Successfully removed dependency: web-srv ➔ api-srv") {
		t.Errorf("expected dependency delete confirmation, got: %s", outDepDel)
	}

	// 7. Dependency Delete (not found error)
	topoDepSource = "web-srv"
	topoDepTarget = "unknown-srv"
	_, err = captureStdout(func() error {
		return runTopologyDependencyDelete(topologyDependenciesDeleteCmd, nil)
	})
	if err == nil {
		t.Fatalf("expected error deleting non-existent dependency, got nil")
	}
}
