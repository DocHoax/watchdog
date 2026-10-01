package topology

import (
	"fmt"
	"strings"
	"testing"
)

func TestGraph_AddAndGetNode(t *testing.T) {
	g := NewGraph()

	node := TopologyNode{
		ID:     "node-1",
		Name:   "Auth Service",
		Type:   NodeTypeService,
		Status: NodeStatusHealthy,
		Source: "test",
	}

	g.AddNode(node)

	retrieved, exists := g.GetNode("node-1")
	if !exists {
		t.Fatalf("expected node-1 to exist")
	}
	if retrieved.Name != "Auth Service" {
		t.Errorf("expected Name 'Auth Service', got '%s'", retrieved.Name)
	}
	if retrieved.Type != NodeTypeService {
		t.Errorf("expected Type NodeTypeService, got '%s'", retrieved.Type)
	}

	if g.NodeCount() != 1 {
		t.Errorf("expected NodeCount 1, got %d", g.NodeCount())
	}
}

func TestGraph_RemoveNode(t *testing.T) {
	g := NewGraph()

	n1 := TopologyNode{ID: "svc-a", Name: "Service A", Type: NodeTypeService, Status: NodeStatusHealthy}
	n2 := TopologyNode{ID: "svc-b", Name: "Service B", Type: NodeTypeService, Status: NodeStatusHealthy}
	g.AddNode(n1)
	g.AddNode(n2)

	g.AddDependency(Dependency{
		SourceID: "svc-a",
		TargetID: "svc-b",
		Type:     RelDependsOn,
	})

	if g.EdgeCount() != 1 {
		t.Fatalf("expected EdgeCount 1, got %d", g.EdgeCount())
	}

	removed := g.RemoveNode("svc-b")
	if !removed {
		t.Errorf("expected RemoveNode to return true")
	}

	if g.HasNode("svc-b") {
		t.Errorf("expected svc-b to be removed")
	}
	if g.EdgeCount() != 0 {
		t.Errorf("expected edges to be cleaned up, got %d", g.EdgeCount())
	}
}

func TestGraph_TraversalsAndCycles(t *testing.T) {
	g := NewGraph()

	// A -> B -> C -> D
	// D -> B (Cycle!)
	nodes := []TopologyNode{
		{ID: "A", Name: "A", Type: NodeTypeService, Status: NodeStatusHealthy},
		{ID: "B", Name: "B", Type: NodeTypeService, Status: NodeStatusHealthy},
		{ID: "C", Name: "C", Type: NodeTypeService, Status: NodeStatusHealthy},
		{ID: "D", Name: "D", Type: NodeTypeDatabase, Status: NodeStatusHealthy},
	}
	for _, n := range nodes {
		g.AddNode(n)
	}

	g.AddDependency(Dependency{SourceID: "A", TargetID: "B", Type: RelDependsOn})
	g.AddDependency(Dependency{SourceID: "B", TargetID: "C", Type: RelDependsOn})
	g.AddDependency(Dependency{SourceID: "C", TargetID: "D", Type: RelDependsOn})
	g.AddDependency(Dependency{SourceID: "D", TargetID: "B", Type: RelDependsOn}) // Cycle

	// Upstream dependencies from A: should find B, C, D without infinite loop
	deps := g.GetTransitiveDependencies("A", 10)
	if len(deps) != 3 {
		t.Errorf("expected 3 transitive dependencies from A, got %d", len(deps))
	}

	// Downstream dependents on D: should find C, B, A without infinite loop
	dependents := g.GetTransitiveDependents("D", 10)
	if len(dependents) != 3 {
		t.Errorf("expected 3 transitive dependents on D, got %d", len(dependents))
	}
}

func TestGraph_FindShortestPath(t *testing.T) {
	g := NewGraph()

	// 1 -> 2 -> 4
	// 1 -> 3 -> 5 -> 4
	for i := 1; i <= 5; i++ {
		g.AddNode(TopologyNode{ID: fmt.Sprintf("n%d", i), Name: fmt.Sprintf("Node %d", i), Type: NodeTypeService})
	}

	g.AddDependency(Dependency{SourceID: "n1", TargetID: "n2", Type: RelDependsOn, Weight: 1.0})
	g.AddDependency(Dependency{SourceID: "n2", TargetID: "n4", Type: RelDependsOn, Weight: 1.0})
	g.AddDependency(Dependency{SourceID: "n1", TargetID: "n3", Type: RelDependsOn, Weight: 1.0})
	g.AddDependency(Dependency{SourceID: "n3", TargetID: "n5", Type: RelDependsOn, Weight: 1.0})
	g.AddDependency(Dependency{SourceID: "n5", TargetID: "n4", Type: RelDependsOn, Weight: 1.0})

	path, found := g.FindShortestPath("n1", "n4")
	if !found {
		t.Fatalf("expected path to be found")
	}

	if path.Hops != 2 {
		t.Errorf("expected 2 hops (n1 -> n2 -> n4), got %d hops", path.Hops)
	}
	if len(path.Nodes) != 3 {
		t.Errorf("expected 3 nodes in path, got %d", len(path.Nodes))
	}
}

func TestGraph_FindAllPaths(t *testing.T) {
	g := NewGraph()

	// S -> A -> T
	// S -> B -> T
	nodes := []string{"S", "A", "B", "T"}
	for _, n := range nodes {
		g.AddNode(TopologyNode{ID: n, Name: n, Type: NodeTypeService})
	}

	g.AddDependency(Dependency{SourceID: "S", TargetID: "A", Type: RelDependsOn})
	g.AddDependency(Dependency{SourceID: "A", TargetID: "T", Type: RelDependsOn})
	g.AddDependency(Dependency{SourceID: "S", TargetID: "B", Type: RelDependsOn})
	g.AddDependency(Dependency{SourceID: "B", TargetID: "T", Type: RelDependsOn})

	paths := g.FindAllPaths("S", "T", 5)
	if len(paths) != 2 {
		t.Errorf("expected 2 distinct paths from S to T, got %d", len(paths))
	}
}

func TestGraph_ConnectedComponents(t *testing.T) {
	g := NewGraph()

	// Component 1: C1-1 <-> C1-2
	g.AddNode(TopologyNode{ID: "c1-1", Name: "c1-1", Type: NodeTypeService})
	g.AddNode(TopologyNode{ID: "c1-2", Name: "c1-2", Type: NodeTypeService})
	g.AddDependency(Dependency{SourceID: "c1-1", TargetID: "c1-2", Type: RelCommunicatesWith})

	// Component 2: C2-1
	g.AddNode(TopologyNode{ID: "c2-1", Name: "c2-1", Type: NodeTypeService})

	components := g.GetConnectedComponents()
	if len(components) != 2 {
		t.Errorf("expected 2 connected components, got %d", len(components))
	}
}

func TestGraph_SummaryAndDOT(t *testing.T) {
	g := NewGraph()

	g.AddNode(TopologyNode{ID: "host-1", Name: "Host 1", Type: NodeTypePhysicalHost, Status: NodeStatusHealthy})
	g.AddNode(TopologyNode{ID: "db-1", Name: "PostgreSQL Primary", Type: NodeTypeDatabase, Status: NodeStatusCritical})
	g.AddDependency(Dependency{SourceID: "db-1", TargetID: "host-1", Type: RelRunsOn, Confidence: ConfidenceHigh})

	summary := g.Summary()
	if summary.TotalNodes != 2 {
		t.Errorf("expected 2 nodes, got %d", summary.TotalNodes)
	}
	if summary.TotalDependencies != 1 {
		t.Errorf("expected 1 dependency, got %d", summary.TotalDependencies)
	}
	if summary.StatusCounts[string(NodeStatusCritical)] != 1 {
		t.Errorf("expected 1 critical node in status counts")
	}

	dot := g.ExportDOT()
	if !strings.Contains(dot, "digraph WatchdogTopology") {
		t.Errorf("DOT export missing digraph declaration")
	}
	if !strings.Contains(dot, "PostgreSQL Primary") {
		t.Errorf("DOT export missing node name")
	}
}

func TestGraph_Filter(t *testing.T) {
	g := NewGraph()

	g.AddNode(TopologyNode{ID: "srv-1", Name: "Web", Type: NodeTypeService, Status: NodeStatusHealthy, Source: "telemetry"})
	g.AddNode(TopologyNode{ID: "db-1", Name: "DB", Type: NodeTypeDatabase, Status: NodeStatusCritical, Source: "telemetry"})
	g.AddNode(TopologyNode{ID: "cache-1", Name: "Redis", Type: NodeTypeCache, Status: NodeStatusHealthy, Source: "declared"})

	filtered := g.Filter(TopologyFilter{
		Types: []NodeType{NodeTypeDatabase, NodeTypeCache},
	})

	if filtered.NodeCount() != 2 {
		t.Errorf("expected 2 filtered nodes, got %d", filtered.NodeCount())
	}
	if filtered.HasNode("srv-1") {
		t.Errorf("expected srv-1 to be excluded by type filter")
	}
}

func TestGraph_EdgeCases_EmptyAndSelfLoops(t *testing.T) {
	g := NewGraph()

	// Empty graph checks
	if g.NodeCount() != 0 || g.EdgeCount() != 0 {
		t.Errorf("expected empty graph, got nodes=%d edges=%d", g.NodeCount(), g.EdgeCount())
	}
	if _, found := g.GetNode("missing"); found {
		t.Errorf("expected GetNode on missing node to return false")
	}
	if g.RemoveNode("missing") {
		t.Errorf("expected RemoveNode on missing node to return false")
	}
	if _, found := g.FindShortestPath("src", "dst"); found {
		t.Errorf("expected FindShortestPath on missing nodes to return false")
	}
	if deps := g.GetTransitiveDependencies("missing", 5); len(deps) != 0 {
		t.Errorf("expected 0 dependencies for missing node")
	}

	// Self-loop check
	g.AddNode(TopologyNode{ID: "self-node", Name: "Self Loop", Type: NodeTypeService})
	g.AddDependency(Dependency{SourceID: "self-node", TargetID: "self-node", Type: RelCommunicatesWith})

	path, found := g.FindShortestPath("self-node", "self-node")
	if !found {
		t.Fatalf("expected self-loop path to be found")
	}
	if path.Hops != 0 || len(path.Nodes) != 1 {
		t.Errorf("expected 0 hops for self-path, got hops=%d nodes=%d", path.Hops, len(path.Nodes))
	}

	// Transitive dependencies with self loop should terminate cleanly
	transDeps := g.GetTransitiveDependencies("self-node", 10)
	if len(transDeps) != 0 {
		t.Errorf("expected 0 other transitive dependencies, got %d", len(transDeps))
	}
}

func TestGraph_MultiEdges_And_Confidence(t *testing.T) {
	g := NewGraph()

	g.AddNode(TopologyNode{ID: "svc-a", Name: "Service A", Type: NodeTypeService})
	g.AddNode(TopologyNode{ID: "svc-b", Name: "Service B", Type: NodeTypeService})

	// Add 2 different relationship types between svc-a and svc-b
	g.AddDependency(Dependency{SourceID: "svc-a", TargetID: "svc-b", Type: RelCommunicatesWith, Confidence: ConfidenceHigh})
	g.AddDependency(Dependency{SourceID: "svc-a", TargetID: "svc-b", Type: RelDependsOn, Confidence: ConfidenceMedium})

	if g.EdgeCount() != 2 {
		t.Errorf("expected 2 distinct edges, got %d", g.EdgeCount())
	}

	outDeps := g.GetOutDependencies("svc-a")
	if len(outDeps) != 2 {
		t.Errorf("expected 2 outgoing dependencies, got %d", len(outDeps))
	}

	// Remove one relationship type
	removed := g.RemoveDependency("svc-a", "svc-b", RelCommunicatesWith)
	if !removed {
		t.Errorf("expected RemoveDependency to return true")
	}
	if g.EdgeCount() != 1 {
		t.Errorf("expected 1 edge remaining, got %d", g.EdgeCount())
	}

	remaining := g.GetOutDependencies("svc-a")
	if len(remaining) != 1 || remaining[0].Type != RelDependsOn {
		t.Errorf("expected RelDependsOn edge to remain, got %+v", remaining)
	}
}

func TestGraph_DeepLinearChain(t *testing.T) {
	g := NewGraph()
	chainLength := 100

	for i := 0; i < chainLength; i++ {
		g.AddNode(TopologyNode{
			ID:   fmt.Sprintf("node-%03d", i),
			Name: fmt.Sprintf("Node %03d", i),
			Type: NodeTypeService,
		})
		if i > 0 {
			g.AddDependency(Dependency{
				SourceID: fmt.Sprintf("node-%03d", i-1),
				TargetID: fmt.Sprintf("node-%03d", i),
				Type:     RelDependsOn,
			})
		}
	}

	if g.NodeCount() != chainLength {
		t.Fatalf("expected %d nodes, got %d", chainLength, g.NodeCount())
	}
	if g.EdgeCount() != chainLength-1 {
		t.Fatalf("expected %d edges, got %d", chainLength-1, g.EdgeCount())
	}

	// Test bounded traversal
	bounded := g.GetTransitiveDependencies("node-000", 5)
	if len(bounded) != 5 {
		t.Errorf("expected 5 bounded dependencies, got %d", len(bounded))
	}

	// Test shortest path across 50 hops
	path, found := g.FindShortestPath("node-000", "node-050")
	if !found {
		t.Fatalf("expected path from node-000 to node-050")
	}
	if path.Hops != 50 {
		t.Errorf("expected 50 hops, got %d", path.Hops)
	}
	if len(path.Nodes) != 51 {
		t.Errorf("expected 51 nodes in path, got %d", len(path.Nodes))
	}
}

func TestGraph_CloneAndSubGraph(t *testing.T) {
	g := NewGraph()
	g.AddNode(TopologyNode{ID: "n1", Name: "N1", Type: NodeTypeService})
	g.AddNode(TopologyNode{ID: "n2", Name: "N2", Type: NodeTypeDatabase})
	g.AddNode(TopologyNode{ID: "n3", Name: "N3", Type: NodeTypeCache})
	g.AddDependency(Dependency{SourceID: "n1", TargetID: "n2", Type: RelDependsOn})
	g.AddDependency(Dependency{SourceID: "n2", TargetID: "n3", Type: RelCommunicatesWith})

	// Subgraph with only n1 and n2
	sub := g.SubGraph([]string{"n1", "n2"})
	if sub.NodeCount() != 2 || sub.EdgeCount() != 1 {
		t.Errorf("expected subGraph to have 2 nodes and 1 edge, got %d nodes, %d edges", sub.NodeCount(), sub.EdgeCount())
	}
	if sub.HasNode("n3") {
		t.Errorf("expected subGraph not to have n3")
	}

	// Clone graph
	clone := g.Clone()
	if clone.NodeCount() != 3 || clone.EdgeCount() != 2 {
		t.Errorf("expected clone to have 3 nodes and 2 edges")
	}

	// Modifying clone should not mutate original
	clone.RemoveNode("n1")
	if !g.HasNode("n1") {
		t.Errorf("modifying clone mutated original graph")
	}
}
