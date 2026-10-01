package governance

import (
	"context"
	"testing"
	"time"

	"github.com/DocHoax/watchdog/internal/storage"
	"github.com/DocHoax/watchdog/pkg/model"
)

func TestRollupExecutionsAndFindings(t *testing.T) {
	t0 := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)

	exec1 := &model.EvaluationExecution{
		ID:           "exec-01",
		OrgID:        "org-acme",
		TargetNodeID: "node-01",
		Status:       model.EvaluationStatusCompliant,
		Results: []model.EvaluationResult{
			{
				RuleID:   "rule-cpu",
				Category: model.PolicyCategoryResourceThresholds,
				Status:   model.EvaluationStatusCompliant,
			},
			{
				RuleID:   "rule-mem",
				Category: model.PolicyCategoryResourceThresholds,
				Status:   model.EvaluationStatusCompliant,
			},
		},
	}

	exec2 := &model.EvaluationExecution{
		ID:           "exec-02",
		OrgID:        "org-acme",
		TargetNodeID: "node-02",
		Status:       model.EvaluationStatusNonCompliant,
		Results: []model.EvaluationResult{
			{
				RuleID:   "rule-cpu",
				Category: model.PolicyCategoryResourceThresholds,
				Status:   model.EvaluationStatusNonCompliant,
			},
			{
				RuleID:   "rule-heartbeat",
				Category: model.PolicyCategoryOperationalCompliance,
				Status:   model.EvaluationStatusCompliant,
			},
		},
	}

	exec3 := &model.EvaluationExecution{
		ID:           "exec-03",
		OrgID:        "org-acme",
		TargetNodeID: "node-03",
		Status:       model.EvaluationStatusWarning,
		Results: []model.EvaluationResult{
			{
				RuleID:   "rule-disk",
				Category: model.PolicyCategoryResourceThresholds,
				Status:   model.EvaluationStatusWarning,
			},
		},
	}

	findings := []model.ComplianceFinding{
		{
			ID:           "fnd-01",
			TargetNodeID: "node-02",
			Severity:     model.SeverityCritical,
			Status:       model.FindingStatusOpen,
		},
		{
			ID:           "fnd-02",
			TargetNodeID: "node-03",
			Severity:     model.SeverityWarning,
			Status:       model.FindingStatusRecurring,
		},
	}

	t.Run("Full rollup math calculation", func(t *testing.T) {
		summary := RollupExecutionsAndFindings(
			model.ComplianceRollupLevelFleetGroup,
			"group-prod",
			"org-acme",
			4, // totalNodes = 4, evaluatedNodes = 3
			[]*model.EvaluationExecution{exec1, exec2, exec3},
			findings,
			t0,
		)

		if summary.TotalNodes != 4 {
			t.Errorf("expected total nodes 4, got %d", summary.TotalNodes)
		}
		if summary.CompliantNodes != 1 {
			t.Errorf("expected compliant nodes 1, got %d", summary.CompliantNodes)
		}
		if summary.NonCompliantNodes != 1 {
			t.Errorf("expected non-compliant nodes 1, got %d", summary.NonCompliantNodes)
		}
		if summary.WarningNodes != 1 {
			t.Errorf("expected warning nodes 1, got %d", summary.WarningNodes)
		}
		if summary.TotalRulesEvaluated != 5 {
			t.Errorf("expected total rules evaluated 5, got %d", summary.TotalRulesEvaluated)
		}
		if summary.TotalFindingsOpen != 2 {
			t.Errorf("expected open findings 2, got %d", summary.TotalFindingsOpen)
		}

		// CoverageRatio = 3 / 4 = 0.75
		expectedCoverage := 0.75
		if summary.CoverageRatio != expectedCoverage {
			t.Errorf("expected coverage ratio %f, got %f", expectedCoverage, summary.CoverageRatio)
		}

		// Total active rules = 3 compliant (2 resource + 1 heartbeat) + 1 non-compliant (1 cpu) + 1 warning (1 disk) = 5
		// ComplianceRatio = 3 / 5 = 0.60
		expectedCompliance := 3.0 / 5.0
		if summary.ComplianceRatio != expectedCompliance {
			t.Errorf("expected compliance ratio %f, got %f", expectedCompliance, summary.ComplianceRatio)
		}

		// Resource thresholds category summary: 2 compliant + 1 violation + 1 warning = 4 rules
		// Category compliance ratio = 2 / 4 = 0.5
		resCat, ok := summary.BreakdownByCategory[model.PolicyCategoryResourceThresholds]
		if !ok {
			t.Fatalf("missing resource thresholds breakdown")
		}
		if resCat.TotalRules != 4 || resCat.CompliantRules != 2 || resCat.Violations != 1 || resCat.Warnings != 1 {
			t.Errorf("unexpected resource category breakdown: %+v", resCat)
		}
		if resCat.ComplianceRatio != 0.5 {
			t.Errorf("expected resource compliance ratio 0.5, got %f", resCat.ComplianceRatio)
		}

		// Severities
		if summary.BreakdownBySeverity[model.SeverityCritical] != 1 {
			t.Errorf("expected 1 critical finding, got %d", summary.BreakdownBySeverity[model.SeverityCritical])
		}
		if summary.BreakdownBySeverity[model.SeverityWarning] != 1 {
			t.Errorf("expected 1 warning finding, got %d", summary.BreakdownBySeverity[model.SeverityWarning])
		}
		if summary.BreakdownBySeverity[model.SeverityInfo] != 0 {
			t.Errorf("expected 0 info findings, got %d", summary.BreakdownBySeverity[model.SeverityInfo])
		}
	})

	t.Run("Empty executions and nodes edge case", func(t *testing.T) {
		summary := RollupExecutionsAndFindings(
			model.ComplianceRollupLevelNode,
			"node-empty",
			"org-acme",
			0,
			nil,
			nil,
			t0,
		)

		if summary.CoverageRatio != 0.0 {
			t.Errorf("expected coverage ratio 0.0, got %f", summary.CoverageRatio)
		}
		if summary.ComplianceRatio != 0.0 {
			t.Errorf("expected compliance ratio 0.0, got %f", summary.ComplianceRatio)
		}
		if summary.TotalFindingsOpen != 0 {
			t.Errorf("expected 0 open findings, got %d", summary.TotalFindingsOpen)
		}
	})
}

func TestComplianceAggregator_IntegrationWithStore(t *testing.T) {
	ctx := context.Background()
	store, err := storage.NewSQLiteStorage(storage.Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("failed to create sqlite store: %v", err)
	}
	defer store.Close()

	clock := NewMockClock(time.Date(2026, 10, 1, 14, 0, 0, 0, time.UTC))
	aggregator := NewComplianceAggregator(store, clock)

	orgID := "org-finance"
	node1 := &model.FleetNode{
		Identity: model.NodeIdentity{
			NodeID:   "node-fin-01",
			Hostname: "fin-01.internal",
		},
		Metadata: map[string]string{
			"org_id": orgID,
		},
	}
	node2 := &model.FleetNode{
		Identity: model.NodeIdentity{
			NodeID:   "node-fin-02",
			Hostname: "fin-02.internal",
		},
		Metadata: map[string]string{
			"org_id": orgID,
		},
	}
	if err := store.SaveFleetNode(ctx, node1); err != nil {
		t.Fatalf("failed to save node1: %v", err)
	}
	if err := store.SaveFleetNode(ctx, node2); err != nil {
		t.Fatalf("failed to save node2: %v", err)
	}

	// Create group
	group := &model.FleetGroup{
		ID:    "group-fin-db",
		OrgID: orgID,
		Name:  "Finance DBs",
		Type:  model.GroupTypeDepartment,
	}
	if err := store.SaveFleetGroup(ctx, group); err != nil {
		t.Fatalf("failed to save group: %v", err)
	}
	if err := store.SetGroupMembers(ctx, "group-fin-db", []string{"node-fin-01", "node-fin-02"}); err != nil {
		t.Fatalf("failed to set group members: %v", err)
	}

	// Save evaluation for node1
	exec1 := &model.EvaluationExecution{
		ID:           "exec-fin-01",
		OrgID:        orgID,
		TargetNodeID: "node-fin-01",
		TriggerType:  model.EvaluationTriggerScheduled,
		Status:       model.EvaluationStatusCompliant,
		EvaluatedAt:  clock.Now(),
		Results: []model.EvaluationResult{
			{
				RuleID:   "rule-fin-res",
				Category: model.PolicyCategoryResourceThresholds,
				Status:   model.EvaluationStatusCompliant,
			},
		},
	}
	if err := store.SaveEvaluationExecution(ctx, exec1); err != nil {
		t.Fatalf("failed to save exec1: %v", err)
	}

	// Save evaluation for node2 with finding
	exec2 := &model.EvaluationExecution{
		ID:           "exec-fin-02",
		OrgID:        orgID,
		TargetNodeID: "node-fin-02",
		TriggerType:  model.EvaluationTriggerScheduled,
		Status:       model.EvaluationStatusNonCompliant,
		EvaluatedAt:  clock.Now(),
		Results: []model.EvaluationResult{
			{
				RuleID:   "rule-fin-res",
				Category: model.PolicyCategoryResourceThresholds,
				Status:   model.EvaluationStatusNonCompliant,
			},
		},
	}
	if err := store.SaveEvaluationExecution(ctx, exec2); err != nil {
		t.Fatalf("failed to save exec2: %v", err)
	}

	finding := model.ComplianceFinding{
		ID:              "fnd-fin-02",
		OrgID:           orgID,
		TargetNodeID:    "node-fin-02",
		PolicyID:        "pol-fin",
		RuleID:          "rule-fin-res",
		Category:        model.PolicyCategoryResourceThresholds,
		Severity:        model.SeverityCritical,
		EnforcementMode: model.EnforcementModeEnforce,
		Status:          model.FindingStatusOpen,
		FirstSeenAt:     clock.Now(),
		LastSeenAt:      clock.Now(),
		OccurrenceCount: 1,
	}
	if err := store.SaveComplianceFindings(ctx, []model.ComplianceFinding{finding}); err != nil {
		t.Fatalf("failed to save findings: %v", err)
	}

	t.Run("AggregateNodeSummary", func(t *testing.T) {
		summary, err := aggregator.AggregateNodeSummary(ctx, orgID, "node-fin-01")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if summary.ScopeLevel != model.ComplianceRollupLevelNode {
			t.Errorf("expected level node, got %s", summary.ScopeLevel)
		}
		if summary.CompliantNodes != 1 || summary.NonCompliantNodes != 0 {
			t.Errorf("expected compliant node, got %+v", summary)
		}
		if summary.ComplianceRatio != 1.0 {
			t.Errorf("expected compliance ratio 1.0, got %f", summary.ComplianceRatio)
		}
	})

	t.Run("AggregateGroupSummary", func(t *testing.T) {
		summary, err := aggregator.AggregateGroupSummary(ctx, orgID, "group-fin-db", false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if summary.ScopeLevel != model.ComplianceRollupLevelFleetGroup {
			t.Errorf("expected level fleet group, got %s", summary.ScopeLevel)
		}
		if summary.TotalNodes != 2 {
			t.Errorf("expected 2 nodes, got %d", summary.TotalNodes)
		}
		if summary.CompliantNodes != 1 || summary.NonCompliantNodes != 1 {
			t.Errorf("expected 1 compliant and 1 non-compliant, got %+v", summary)
		}
		if summary.ComplianceRatio != 0.5 {
			t.Errorf("expected compliance ratio 0.5, got %f", summary.ComplianceRatio)
		}
		if summary.CoverageRatio != 1.0 {
			t.Errorf("expected coverage ratio 1.0, got %f", summary.CoverageRatio)
		}
	})

	t.Run("AggregateOrgSummary", func(t *testing.T) {
		summary, err := aggregator.AggregateOrgSummary(ctx, orgID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if summary.ScopeLevel != model.ComplianceRollupLevelOrg {
			t.Errorf("expected level org, got %s", summary.ScopeLevel)
		}
		if summary.TotalNodes != 2 {
			t.Errorf("expected 2 org nodes, got %d", summary.TotalNodes)
		}
		if summary.TotalFindingsOpen != 1 {
			t.Errorf("expected 1 open finding, got %d", summary.TotalFindingsOpen)
		}
	})

	t.Run("AggregateFleetSummary", func(t *testing.T) {
		summary, err := aggregator.AggregateFleetSummary(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if summary.ScopeLevel != model.ComplianceRollupLevelFleet {
			t.Errorf("expected level fleet, got %s", summary.ScopeLevel)
		}
		if summary.TotalNodes != 2 {
			t.Errorf("expected 2 fleet nodes, got %d", summary.TotalNodes)
		}
	})
}
