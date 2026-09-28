package topology

import (
	"testing"
)

func TestImpactAnalyzer_CascadingFailure(t *testing.T) {
	g := NewGraph()

	// Cascade: Web -> API -> DB -> Storage
	g.AddNode(TopologyNode{ID: "storage", Name: "Ceph Storage", Type: NodeTypeStorage, Status: NodeStatusHealthy})
	g.AddNode(TopologyNode{ID: "db", Name: "Postgres", Type: NodeTypeDatabase, Status: NodeStatusHealthy})
	g.AddNode(TopologyNode{ID: "api", Name: "API Gateway", Type: NodeTypeService, Status: NodeStatusHealthy})
	g.AddNode(TopologyNode{ID: "web", Name: "Frontend", Type: NodeTypeService, Status: NodeStatusHealthy})

	g.AddDependency(Dependency{SourceID: "web", TargetID: "api", Type: RelDependsOn})
	g.AddDependency(Dependency{SourceID: "api", TargetID: "db", Type: RelDependsOn})
	g.AddDependency(Dependency{SourceID: "db", TargetID: "storage", Type: RelDependsOn})

	analyzer := NewImpactAnalyzer()

	// Impact of storage failure: should cascade to db, api, web (3 transitive dependents, max depth 3)
	impact := analyzer.AnalyzeImpact(g, "storage")
	if impact == nil {
		t.Fatalf("expected non-nil impact analysis")
	}

	if len(impact.DirectDependents) != 1 {
		t.Errorf("expected 1 direct dependent (db), got %d", len(impact.DirectDependents))
	}

	if len(impact.TransitiveDependents) != 3 {
		t.Errorf("expected 3 transitive dependents (db, api, web), got %d", len(impact.TransitiveDependents))
	}

	if impact.MaxImpactDepth != 3 {
		t.Errorf("expected MaxImpactDepth 3, got %d", impact.MaxImpactDepth)
	}

	if impact.BlastRadiusLevel != BlastRadiusCritical && impact.BlastRadiusLevel != BlastRadiusHigh {
		t.Errorf("expected High/Critical blast radius level for total system storage, got %s", impact.BlastRadiusLevel)
	}
}

func TestImpactAnalyzer_LeafNodeFailure(t *testing.T) {
	g := NewGraph()

	g.AddNode(TopologyNode{ID: "web", Name: "Frontend", Type: NodeTypeService, Status: NodeStatusHealthy})
	g.AddNode(TopologyNode{ID: "db", Name: "DB", Type: NodeTypeDatabase, Status: NodeStatusHealthy})
	g.AddDependency(Dependency{SourceID: "web", TargetID: "db", Type: RelDependsOn})

	analyzer := NewImpactAnalyzer()

	// Impact of leaf node (web): 0 dependents
	impact := analyzer.AnalyzeImpact(g, "web")
	if impact == nil {
		t.Fatalf("expected non-nil impact")
	}

	if len(impact.DirectDependents) != 0 {
		t.Errorf("expected 0 direct dependents for leaf node, got %d", len(impact.DirectDependents))
	}

	if impact.BlastRadiusLevel != BlastRadiusMinimal {
		t.Errorf("expected minimal blast radius level for leaf node, got %s", impact.BlastRadiusLevel)
	}
}
