package intelligence

import (
	"testing"
	"time"

	"github.com/DocHoax/watchdog/internal/topology"
	"github.com/DocHoax/watchdog/pkg/model"
)

func TestRootCauseEngine_BasicCascadingFailure(t *testing.T) {
	g := topology.NewGraph()

	// Topology: api-gw -> auth-svc -> db-primary
	g.AddNode(topology.TopologyNode{ID: "db-primary", Name: "PostgreSQL DB", Type: topology.NodeTypeDatabase, Status: topology.NodeStatusCritical})
	g.AddNode(topology.TopologyNode{ID: "auth-svc", Name: "Auth Service", Type: topology.NodeTypeService, Status: topology.NodeStatusDegraded})
	g.AddNode(topology.TopologyNode{ID: "api-gw", Name: "API Gateway", Type: topology.NodeTypeService, Status: topology.NodeStatusDegraded})

	g.AddDependency(topology.Dependency{SourceID: "auth-svc", TargetID: "db-primary", Type: topology.RelDependsOn})
	g.AddDependency(topology.Dependency{SourceID: "api-gw", TargetID: "auth-svc", Type: topology.RelDependsOn})

	t0 := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)

	timeline := []IncidentTimelineEvent{
		{
			Timestamp:   t0,
			NodeID:      "db-primary",
			EventType:   "alert_fired",
			Description: "Database disk space 99% full",
			Severity:    model.SeverityCritical,
		},
		{
			Timestamp:   t0.Add(15 * time.Second),
			NodeID:      "auth-svc",
			EventType:   "alert_fired",
			Description: "P99 latency exceeding 2500ms",
			Severity:    model.SeverityWarning,
		},
		{
			Timestamp:   t0.Add(30 * time.Second),
			NodeID:      "api-gw",
			EventType:   "alert_fired",
			Description: "HTTP 500 rate at 12%",
			Severity:    model.SeverityWarning,
		},
	}

	engine := NewRootCauseEngine()
	report := engine.AnalyzeIncidentWithSignals(
		"inc-001",
		"Database Disk Outage & Service Degradation",
		model.SeverityCritical,
		t0,
		[]string{"db-primary", "auth-svc", "api-gw"},
		nil,
		nil,
		timeline,
		g,
		nil,
		nil,
	)

	if report == nil {
		t.Fatalf("expected non-nil RootCauseReport")
	}

	if report.PrimaryRootCause == nil {
		t.Fatalf("expected PrimaryRootCause to be identified")
	}

	if report.PrimaryRootCause.NodeID != "db-primary" {
		t.Errorf("expected primary root cause to be db-primary, got %s", report.PrimaryRootCause.NodeID)
	}

	if report.PrimaryRootCause.Confidence != RootCauseConfidenceHigh {
		t.Errorf("expected High confidence on primary root cause, got %s", report.PrimaryRootCause.Confidence)
	}

	if report.PrimaryRootCause.TotalScore < 80.0 {
		t.Errorf("expected high total score (>=80.0), got %.1f", report.PrimaryRootCause.TotalScore)
	}

	if len(report.PropagationChains) == 0 {
		t.Errorf("expected propagation chains to be identified")
	}

	// Verify propagation path
	foundGatewayChain := false
	for _, chain := range report.PropagationChains {
		if chain.LeafNodeID == "api-gw" {
			foundGatewayChain = true
			if len(chain.Hops) != 2 {
				t.Errorf("expected 2 hops in propagation chain to api-gw, got %d", len(chain.Hops))
			}
		}
	}
	if !foundGatewayChain {
		t.Errorf("expected propagation chain to api-gw")
	}

	if len(report.NonInvasiveRecommendations) == 0 {
		t.Errorf("expected non-invasive recommendations")
	}
}

func TestRootCauseEngine_CustomWeightsAndSafety(t *testing.T) {
	customWeights := RootCauseWeights{
		TemporalWeight:    0.40,
		TopologyWeight:    0.20,
		SeverityWeight:    0.20,
		BlastRadiusWeight: 0.10,
		HistoricalWeight:  0.10,
	}

	engine := NewRootCauseEngine(customWeights)

	// Test nil incident safety
	nilReport := engine.AnalyzeIncident(nil, nil, nil, nil)
	if nilReport != nil {
		t.Errorf("expected nil report for nil incident input")
	}

	// Test empty incident
	emptyReport := engine.AnalyzeIncidentWithSignals(
		"inc-empty",
		"Empty Test",
		model.SeverityInfo,
		time.Now(),
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)
	if emptyReport == nil || emptyReport.PrimaryRootCause != nil {
		t.Errorf("expected safe empty report without primary root cause")
	}
}
