package governance

import (
	"context"
	"testing"
	"time"

	"github.com/DocHoax/watchdog/internal/storage"
	"github.com/DocHoax/watchdog/pkg/model"
)

func TestFindingsLifecycle_ReconcileResults(t *testing.T) {
	t0 := time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)
	orgID := "org-acme"
	nodeID := "node-01"

	ruleResultBreach := model.EvaluationResult{
		RuleID:          "rule-cpu",
		RuleName:        "High CPU Rule",
		PolicyID:        "pol-res",
		PolicyRevision:  1,
		Category:        model.PolicyCategoryResourceThresholds,
		Severity:        model.SeverityCritical,
		EnforcementMode: model.EnforcementModeEnforce,
		Status:          model.EvaluationStatusNonCompliant,
		ObservedValue:   "95%",
		ExpectedValue:   "<= 80%",
		Message:         "CPU at 95% exceeds 80%",
	}

	ruleResultPass := model.EvaluationResult{
		RuleID:          "rule-cpu",
		RuleName:        "High CPU Rule",
		PolicyID:        "pol-res",
		PolicyRevision:  1,
		Category:        model.PolicyCategoryResourceThresholds,
		Severity:        model.SeverityCritical,
		EnforcementMode: model.EnforcementModeEnforce,
		Status:          model.EvaluationStatusCompliant,
		ObservedValue:   "45%",
		ExpectedValue:   "<= 80%",
		Message:         "CPU at 45% is normal",
	}

	// Step 1: Initial violation -> Open finding
	reconciled1 := ReconcileResults(orgID, nodeID, []model.EvaluationResult{ruleResultBreach}, nil, t0)
	if len(reconciled1) != 1 {
		t.Fatalf("expected 1 reconciled finding, got %d", len(reconciled1))
	}
	fnd1 := reconciled1[0]
	if fnd1.Status != model.FindingStatusOpen {
		t.Errorf("expected Open status, got %s", fnd1.Status)
	}
	if fnd1.OccurrenceCount != 1 {
		t.Errorf("expected occurrence count 1, got %d", fnd1.OccurrenceCount)
	}
	if !fnd1.FirstSeenAt.Equal(t0) || !fnd1.LastSeenAt.Equal(t0) {
		t.Errorf("expected timestamps to equal t0")
	}

	// Step 2: Next evaluation continues to breach -> Recurring finding
	t1 := t0.Add(5 * time.Minute)
	reconciled2 := ReconcileResults(orgID, nodeID, []model.EvaluationResult{ruleResultBreach}, []model.ComplianceFinding{fnd1}, t1)
	if len(reconciled2) != 1 {
		t.Fatalf("expected 1 reconciled finding, got %d", len(reconciled2))
	}
	fnd2 := reconciled2[0]
	if fnd2.Status != model.FindingStatusRecurring {
		t.Errorf("expected Recurring status, got %s", fnd2.Status)
	}
	if fnd2.OccurrenceCount != 2 {
		t.Errorf("expected occurrence count 2, got %d", fnd2.OccurrenceCount)
	}
	if !fnd2.LastSeenAt.Equal(t1) {
		t.Errorf("expected LastSeenAt to update to t1")
	}

	// Step 3: Next evaluation is compliant -> Resolved finding
	t2 := t1.Add(5 * time.Minute)
	reconciled3 := ReconcileResults(orgID, nodeID, []model.EvaluationResult{ruleResultPass}, []model.ComplianceFinding{fnd2}, t2)
	if len(reconciled3) != 1 {
		t.Fatalf("expected 1 reconciled finding, got %d", len(reconciled3))
	}
	fnd3 := reconciled3[0]
	if fnd3.Status != model.FindingStatusResolved {
		t.Errorf("expected Resolved status, got %s", fnd3.Status)
	}
	if fnd3.ResolvedAt == nil || !fnd3.ResolvedAt.Equal(t2) {
		t.Errorf("expected ResolvedAt to be set to t2")
	}

	// Step 4: Next evaluation breaches again -> Reopened (Open status)
	t3 := t2.Add(5 * time.Minute)
	reconciled4 := ReconcileResults(orgID, nodeID, []model.EvaluationResult{ruleResultBreach}, []model.ComplianceFinding{fnd3}, t3)
	if len(reconciled4) != 1 {
		t.Fatalf("expected 1 reconciled finding, got %d", len(reconciled4))
	}
	fnd4 := reconciled4[0]
	if fnd4.Status != model.FindingStatusOpen {
		t.Errorf("expected reopened finding to have Open status, got %s", fnd4.Status)
	}
	if fnd4.ResolvedAt != nil {
		t.Errorf("expected ResolvedAt to be cleared on reopening")
	}
	if fnd4.OccurrenceCount != 3 {
		t.Errorf("expected occurrence count 3, got %d", fnd4.OccurrenceCount)
	}

	// Step 5: Insufficient data -> Indeterminate status
	t4 := t3.Add(5 * time.Minute)
	ruleResultMissing := model.EvaluationResult{
		RuleID:   "rule-cpu",
		PolicyID: "pol-res",
		Status:   model.EvaluationStatusInsufficientData,
		Message:  "missing telemetry",
	}
	reconciled5 := ReconcileResults(orgID, nodeID, []model.EvaluationResult{ruleResultMissing}, []model.ComplianceFinding{fnd4}, t4)
	if len(reconciled5) != 1 {
		t.Fatalf("expected 1 reconciled finding, got %d", len(reconciled5))
	}
	if reconciled5[0].Status != model.FindingStatusIndeterminate {
		t.Errorf("expected Indeterminate status on missing data, got %s", reconciled5[0].Status)
	}
}

func TestFindingReconciler_ReconcileExecution_Persisted(t *testing.T) {
	ctx := context.Background()
	store, err := storage.NewSQLiteStorage(storage.Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("failed to create sqlite store: %v", err)
	}
	defer store.Close()

	clock := NewMockClock(time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC))
	reconciler := NewFindingReconciler(store, clock)

	exec := &model.EvaluationExecution{
		ID:           "exec-01",
		OrgID:        "org-acme",
		TargetNodeID: "node-prod-01",
		TriggerType:  model.EvaluationTriggerScheduled,
		EvaluatedAt:  clock.Now(),
		Results: []model.EvaluationResult{
			{
				RuleID:          "rule-mem",
				RuleName:        "Memory Breach",
				PolicyID:        "pol-res",
				PolicyRevision:  1,
				Category:        model.PolicyCategoryResourceThresholds,
				Severity:        model.SeverityCritical,
				EnforcementMode: model.EnforcementModeEnforce,
				Status:          model.EvaluationStatusNonCompliant,
				ObservedValue:   "92%",
				ExpectedValue:   "<= 80%",
				Message:         "Memory high",
			},
		},
	}

	reconciled, err := reconciler.ReconcileExecution(ctx, exec)
	if err != nil {
		t.Fatalf("unexpected error reconciling: %v", err)
	}
	if len(reconciled) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(reconciled))
	}

	// Verify finding in storage
	findings, err := store.ListComplianceFindings(ctx, model.FindingFilter{
		OrgID:        "org-acme",
		TargetNodeID: "node-prod-01",
	})
	if err != nil {
		t.Fatalf("failed to list findings: %v", err)
	}
	if len(findings) != 1 || findings[0].Status != model.FindingStatusOpen {
		t.Errorf("expected 1 open finding in store, got %+v", findings)
	}
}
