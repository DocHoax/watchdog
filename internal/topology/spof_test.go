package topology

import (
	"testing"

	"github.com/DocHoax/watchdog/pkg/model"
)

func TestSPOFAnalyzer_SingleDatabaseWithMultipleDependents(t *testing.T) {
	g := NewGraph()

	// Setup a classic SPOF: 1 DB with 4 microservices depending on it and no replica
	g.AddNode(TopologyNode{
		ID:     "db-primary",
		Name:   "Postgres DB",
		Type:   NodeTypeDatabase,
		Status: NodeStatusHealthy,
	})

	for _, svc := range []string{"auth-svc", "payment-svc", "order-svc", "inventory-svc"} {
		g.AddNode(TopologyNode{ID: svc, Name: svc, Type: NodeTypeService, Status: NodeStatusHealthy})
		g.AddDependency(Dependency{
			SourceID: svc,
			TargetID: "db-primary",
			Type:     RelDependsOn,
		})
	}

	analyzer := NewSPOFAnalyzer()
	analysis := analyzer.AnalyzeNode(g, "db-primary")

	if analysis == nil {
		t.Fatalf("expected non-nil SPOF analysis")
	}

	if analysis.DependentsCount != 4 {
		t.Errorf("expected 4 direct dependents, got %d", analysis.DependentsCount)
	}

	if analysis.RedundancyLevel != RedundancyNone {
		t.Errorf("expected RedundancyNone, got %s", analysis.RedundancyLevel)
	}

	if analysis.CriticalityScore < 50.0 {
		t.Errorf("expected high CriticalityScore (>= 50), got %.1f", analysis.CriticalityScore)
	}

	if analysis.RiskLevel != model.SeverityWarning && analysis.RiskLevel != model.SeverityCritical {
		t.Errorf("expected Warning/Critical severity, got %s", analysis.RiskLevel)
	}
}

func TestSPOFAnalyzer_RedundantDatabases(t *testing.T) {
	g := NewGraph()

	// Two DB instances (primary and replica)
	g.AddNode(TopologyNode{ID: "db-1", Name: "Postgres 1", Type: NodeTypeDatabase, Status: NodeStatusHealthy})
	g.AddNode(TopologyNode{ID: "db-2", Name: "Postgres 2", Type: NodeTypeDatabase, Status: NodeStatusHealthy})

	// Service connects to both
	g.AddNode(TopologyNode{ID: "api-svc", Name: "API", Type: NodeTypeService, Status: NodeStatusHealthy})
	g.AddDependency(Dependency{SourceID: "api-svc", TargetID: "db-1", Type: RelDependsOn})
	g.AddDependency(Dependency{SourceID: "api-svc", TargetID: "db-2", Type: RelDependsOn})

	analyzer := NewSPOFAnalyzer()
	analysis := analyzer.AnalyzeNode(g, "db-1")

	if analysis.RedundancyLevel != RedundancyRedundant {
		t.Errorf("expected RedundancyRedundant when dependent connects to both instances, got %s", analysis.RedundancyLevel)
	}
}

func TestSPOFAnalyzer_FindAllSPOFs(t *testing.T) {
	g := NewGraph()

	g.AddNode(TopologyNode{ID: "db-main", Name: "Main DB", Type: NodeTypeDatabase, Status: NodeStatusCritical})
	g.AddNode(TopologyNode{ID: "leaf-node", Name: "Leaf Node", Type: NodeTypeService, Status: NodeStatusHealthy})
	g.AddDependency(Dependency{SourceID: "leaf-node", TargetID: "db-main", Type: RelDependsOn})

	analyzer := NewSPOFAnalyzer()
	spofs := analyzer.FindAllSPOFs(g, 10.0)

	if len(spofs) == 0 {
		t.Fatalf("expected at least 1 SPOF detected")
	}

	if spofs[0].NodeID != "db-main" {
		t.Errorf("expected highest criticality node to be db-main, got %s", spofs[0].NodeID)
	}
}
