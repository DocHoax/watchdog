package governance

import (
	"context"
	"testing"
	"time"

	"github.com/DocHoax/watchdog/internal/storage"
	"github.com/DocHoax/watchdog/pkg/model"
)

func TestSimulationEngine_DiffsAndImpacts(t *testing.T) {
	ctx := context.Background()
	store, err := storage.NewSQLiteStorage(storage.Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("failed to create sqlite store: %v", err)
	}
	defer store.Close()

	clock := NewMockClock(time.Date(2026, 10, 1, 15, 0, 0, 0, time.UTC))
	registry := NewEvaluatorRegistry()
	engine := NewSimulationEngine(store, registry, clock)

	orgID := "org-sim-test"

	// 1. Setup Node in Store
	node := &model.FleetNode{
		Identity: model.NodeIdentity{
			NodeID:   "node-sim-01",
			Hostname: "sim-01.internal",
		},
		Metadata: map[string]string{
			"org_id": orgID,
		},
	}
	if err := store.SaveFleetNode(ctx, node); err != nil {
		t.Fatalf("failed to save fleet node: %v", err)
	}

	// 2. Submit Telemetry: CPU is 85%
	sub := model.TelemetrySubmission{
		NodeID:    "node-sim-01",
		Timestamp: clock.Now(),
		Metrics: map[string]float64{
			"cpu_usage_pct":    85.0,
			"memory_usage_pct": 50.0,
			"disk_usage_pct":   40.0,
		},
	}
	if err := store.SaveTelemetrySubmission(ctx, &sub); err != nil {
		t.Fatalf("failed to save telemetry: %v", err)
	}

	// 3. Baseline Policy: CPU CriticalThreshold = 80 (Node is non-compliant in baseline)
	basePolicy := &model.Policy{
		ID:             "pol-cpu",
		OrgID:          orgID,
		Name:           "CPU Policy",
		Category:       model.PolicyCategoryResourceThresholds,
		Status:         model.PolicyStatusActive,
		ActiveRevision: 1,
	}
	if err := store.SavePolicy(ctx, basePolicy); err != nil {
		t.Fatalf("failed to save base policy: %v", err)
	}

	baseRev := &model.PolicyRevision{
		PolicyID:        "pol-cpu",
		Revision:        1,
		Priority:        100,
		InheritanceMode: model.InheritanceModeInheritAndOverride,
		EnforcementMode: model.EnforcementModeEnforce,
		Rules: []model.PolicyRule{
			{
				ID:       "rule-cpu-limit",
				Name:     "CPU Limit",
				Type:     model.RuleTypeResourceThreshold,
				Severity: model.SeverityCritical,
				Enabled:  true,
				ResourceThreshold: &model.ResourceThresholdRuleConfig{
					Metric:            "cpu_usage_pct",
					WarningThreshold:  70,
					CriticalThreshold: 80,
				},
			},
		},
	}
	if err := store.SavePolicyRevision(ctx, baseRev); err != nil {
		t.Fatalf("failed to save base revision: %v", err)
	}

	baseAsgn := &model.PolicyAssignment{
		ID:         "asgn-cpu-org",
		PolicyID:   "pol-cpu",
		OrgID:      orgID,
		TargetType: model.TargetTypeOrganization,
		TargetID:   orgID,
		Enabled:    true,
	}
	if err := store.SavePolicyAssignment(ctx, baseAsgn); err != nil {
		t.Fatalf("failed to save base assignment: %v", err)
	}

	// 4. Run Simulation: Proposed Revision relaxes CPU CriticalThreshold to 95 and Warning to 90 (Node becomes compliant)
	// and adds a new rule for Disk (<80) which also passes.
	simReq := &model.SimulationRequest{
		OrgID:         orgID,
		TargetNodeIDs: []string{"node-sim-01"},
		ProposedPolicies: []model.Policy{
			{
				ID:             "pol-cpu",
				OrgID:          orgID,
				Name:           "CPU Policy",
				Category:       model.PolicyCategoryResourceThresholds,
				Status:         model.PolicyStatusActive,
				ActiveRevision: 2, // Candidate revision 2
			},
		},
		ProposedRevisions: []model.PolicyRevision{
			{
				PolicyID:        "pol-cpu",
				Revision:        2,
				Priority:        100,
				InheritanceMode: model.InheritanceModeInheritAndOverride,
				EnforcementMode: model.EnforcementModeEnforce,
				Rules: []model.PolicyRule{
					{
						ID:       "rule-cpu-limit",
						Name:     "CPU Limit",
						Type:     model.RuleTypeResourceThreshold,
						Severity: model.SeverityCritical,
						Enabled:  true,
						ResourceThreshold: &model.ResourceThresholdRuleConfig{
							Metric:            "cpu_usage_pct",
							WarningThreshold:  90,
							CriticalThreshold: 95,
						},
					},
					{
						ID:       "rule-disk-limit",
						Name:     "Disk Limit",
						Type:     model.RuleTypeResourceThreshold,
						Severity: model.SeverityWarning,
						Enabled:  true,
						ResourceThreshold: &model.ResourceThresholdRuleConfig{
							Metric:            "disk_usage_pct",
							WarningThreshold:  80,
							CriticalThreshold: 95,
						},
					},
				},
			},
		},
	}

	simResult, err := engine.SimulatePolicyChanges(ctx, simReq)
	if err != nil {
		t.Fatalf("simulation failed: %v", err)
	}

	// 5. Verify Simulation Result Metrics
	if simResult.TotalNodes != 1 || simResult.EvaluatedNodes != 1 {
		t.Errorf("expected 1 evaluated node, got total=%d evaluated=%d", simResult.TotalNodes, simResult.EvaluatedNodes)
	}

	// In baseline: 85% cpu >= 80% threshold -> non-compliant (compliance ratio = 0.0)
	if simResult.BaselineComplianceRatio != 0.0 {
		t.Errorf("expected baseline compliance ratio 0.0, got %f", simResult.BaselineComplianceRatio)
	}

	// In proposed: 85% cpu < 90% (compliant) and 40% disk < 80% (compliant)
	// Compliant rules = 2, total = 2 -> compliance ratio = 2 / 2 = 1.0
	if simResult.ProposedComplianceRatio != 1.0 {
		t.Errorf("expected proposed compliance ratio 1.0, got %f", simResult.ProposedComplianceRatio)
	}

	if simResult.ComplianceRatioDelta <= 0 {
		t.Errorf("expected positive compliance ratio delta, got %f", simResult.ComplianceRatioDelta)
	}

	// 6. Verify Node Impact
	if len(simResult.NodeImpacts) != 1 {
		t.Fatalf("expected 1 node impact, got %d", len(simResult.NodeImpacts))
	}
	impact := simResult.NodeImpacts[0]
	if impact.NodeID != "node-sim-01" {
		t.Errorf("expected node ID node-sim-01, got %s", impact.NodeID)
	}
	if impact.BaselineStatus != model.EvaluationStatusNonCompliant {
		t.Errorf("expected baseline non-compliant, got %s", impact.BaselineStatus)
	}
	if impact.ProposedStatus != model.EvaluationStatusCompliant {
		t.Errorf("expected proposed compliant, got %s", impact.ProposedStatus)
	}

	// Check Rule Diffs
	if len(impact.RuleDiffs) != 2 {
		t.Fatalf("expected 2 rule diffs, got %d", len(impact.RuleDiffs))
	}

	var cpuDiff, diskDiff *model.RuleImpactDiff
	for i := range impact.RuleDiffs {
		d := &impact.RuleDiffs[i]
		if d.RuleID == "rule-cpu-limit" {
			cpuDiff = d
		} else if d.RuleID == "rule-disk-limit" {
			diskDiff = d
		}
	}

	if cpuDiff == nil || cpuDiff.DiffType != model.RuleDiffTypeModified {
		t.Errorf("expected modified cpu rule diff, got %+v", cpuDiff)
	}
	if diskDiff == nil || diskDiff.DiffType != model.RuleDiffTypeAdded {
		t.Errorf("expected added disk rule diff, got %+v", diskDiff)
	}

	// 7. Verify Resolved Findings
	if len(simResult.ResolvedFindings) != 1 {
		t.Errorf("expected 1 resolved finding for baseline non-compliance, got %d", len(simResult.ResolvedFindings))
	}
}

func TestSimulationEngine_NoMutationGuarantee(t *testing.T) {
	ctx := context.Background()
	store, err := storage.NewSQLiteStorage(storage.Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("failed to create sqlite store: %v", err)
	}
	defer store.Close()

	clock := NewMockClock(time.Date(2026, 10, 1, 15, 0, 0, 0, time.UTC))
	registry := NewEvaluatorRegistry()
	engine := NewSimulationEngine(store, registry, clock)

	orgID := "org-immutability-check"

	// Create candidate policy revision
	simReq := &model.SimulationRequest{
		OrgID: orgID,
		ProposedPolicies: []model.Policy{
			{
				ID:             "pol-ephemeral",
				OrgID:          orgID,
				Name:           "Ephemeral Policy",
				Category:       model.PolicyCategoryOperationalCompliance,
				Status:         model.PolicyStatusActive,
				ActiveRevision: 1,
			},
		},
		ProposedRevisions: []model.PolicyRevision{
			{
				PolicyID:        "pol-ephemeral",
				Revision:        1,
				Priority:        100,
				InheritanceMode: model.InheritanceModeInheritAndOverride,
				EnforcementMode: model.EnforcementModeEnforce,
				Rules: []model.PolicyRule{
					{
						ID:       "rule-ephemeral",
						Name:     "Ephemeral Rule",
						Type:     model.RuleTypeOperationalCompliance,
						Severity: model.SeverityCritical,
						Enabled:  true,
						OperationalCompliance: &model.OperationalComplianceRuleConfig{
							CheckType:         "heartbeat_freshness",
							MaxAgeSeconds:     300,
							ViolationSeverity: model.SeverityCritical,
						},
					},
				},
			},
		},
		ProposedAssignments: []model.PolicyAssignment{
			{
				ID:         "asgn-ephemeral",
				PolicyID:   "pol-ephemeral",
				OrgID:      orgID,
				TargetType: model.TargetTypeOrganization,
				TargetID:   orgID,
				Enabled:    true,
			},
		},
	}

	// Execute simulation
	_, err = engine.SimulatePolicyChanges(ctx, simReq)
	if err != nil {
		t.Fatalf("simulation failed: %v", err)
	}

	// Verify that underlying store has zero policies, revisions, assignments, evaluations, or findings
	policies, err := store.ListPolicies(ctx, model.PolicyFilter{OrgID: orgID})
	if err != nil || len(policies) != 0 {
		t.Errorf("expected 0 persisted policies, got %d, err: %v", len(policies), err)
	}

	revs, err := store.ListPolicyRevisions(ctx, "pol-ephemeral")
	if err != nil || len(revs) != 0 {
		t.Errorf("expected 0 persisted revisions, got %d, err: %v", len(revs), err)
	}

	asgns, err := store.ListPolicyAssignments(ctx, model.PolicyAssignmentFilter{OrgID: orgID})
	if err != nil || len(asgns) != 0 {
		t.Errorf("expected 0 persisted assignments, got %d, err: %v", len(asgns), err)
	}

	evals, err := store.ListEvaluationExecutions(ctx, model.EvaluationFilter{OrgID: orgID})
	if err != nil || len(evals) != 0 {
		t.Errorf("expected 0 persisted evaluations, got %d, err: %v", len(evals), err)
	}

	findings, err := store.ListComplianceFindings(ctx, model.FindingFilter{OrgID: orgID})
	if err != nil || len(findings) != 0 {
		t.Errorf("expected 0 persisted findings, got %d, err: %v", len(findings), err)
	}
}

func TestSimulationEngine_ValidationErrors(t *testing.T) {
	engine := NewSimulationEngine(nil, nil, nil)
	ctx := context.Background()

	// Nil request
	if _, err := engine.SimulatePolicyChanges(ctx, nil); err == nil {
		t.Errorf("expected error for nil simulation request")
	}

	// Invalid org ID
	invalidReq := &model.SimulationRequest{
		OrgID: "bad org id with spaces!",
	}
	if _, err := engine.SimulatePolicyChanges(ctx, invalidReq); err == nil {
		t.Errorf("expected error for invalid org ID")
	}
}
