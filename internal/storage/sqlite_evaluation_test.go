package storage

import (
	"context"
	"testing"
	"time"

	"github.com/DocHoax/watchdog/pkg/model"
)

func newTestStorage(t *testing.T) *SQLiteStorage {
	t.Helper()
	store, err := NewSQLiteStorage(Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("failed to create in-memory sqlite storage: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})
	return store
}

func TestSQLiteStorage_EvaluationExecution_Lifecycle(t *testing.T) {
	store := newTestStorage(t)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Millisecond)

	exec := &model.EvaluationExecution{
		ID:           "exec-001",
		OrgID:        "org-acme",
		TargetNodeID: "node-01",
		TriggerType:  model.EvaluationTriggerOnDemand,
		EvaluatedAt:  now,
		DurationNs:   12500000,
		Status:       model.EvaluationStatusCompliant,
		Results: []model.EvaluationResult{
			{
				RuleID:          "rule-cpu",
				RuleName:        "CPU Utilization Rule",
				PolicyID:        "pol-res",
				PolicyRevision:  1,
				Category:        model.PolicyCategoryResourceThresholds,
				RuleType:        model.RuleTypeResourceThreshold,
				Status:          model.EvaluationStatusCompliant,
				Severity:        model.SeverityCritical,
				EnforcementMode: model.EnforcementModeEnforce,
				ObservedValue:   "45%",
				ExpectedValue:   "<= 85%",
				Message:         "CPU within threshold",
				DataFreshness:   model.DataFreshnessFresh,
				EvaluatedAt:     now,
			},
		},
		Summary: model.EvaluationSummary{
			TotalRules:      1,
			CompliantRules:  1,
			ComplianceRatio: 1.0,
			CoverageRatio:   1.0,
		},
		Metadata: map[string]string{
			"env": "production",
		},
	}

	// 1. Save
	if err := store.SaveEvaluationExecution(ctx, exec); err != nil {
		t.Fatalf("failed to save evaluation execution: %v", err)
	}

	// 2. Get by ID
	retrieved, err := store.GetEvaluationExecution(ctx, "exec-001")
	if err != nil {
		t.Fatalf("failed to get evaluation execution: %v", err)
	}
	if retrieved.ID != exec.ID {
		t.Errorf("expected ID %s, got %s", exec.ID, retrieved.ID)
	}
	if retrieved.OrgID != exec.OrgID {
		t.Errorf("expected OrgID %s, got %s", exec.OrgID, retrieved.OrgID)
	}
	if retrieved.Status != exec.Status {
		t.Errorf("expected Status %s, got %s", exec.Status, retrieved.Status)
	}
	if len(retrieved.Results) != 1 || retrieved.Results[0].RuleID != "rule-cpu" {
		t.Errorf("unexpected results: %+v", retrieved.Results)
	}
	if retrieved.Summary.ComplianceRatio != 1.0 {
		t.Errorf("expected compliance ratio 1.0, got %f", retrieved.Summary.ComplianceRatio)
	}

	// 3. GetLatestNodeEvaluation
	latest, err := store.GetLatestNodeEvaluation(ctx, "org-acme", "node-01")
	if err != nil {
		t.Fatalf("failed to get latest node evaluation: %v", err)
	}
	if latest.ID != "exec-001" {
		t.Errorf("expected latest ID exec-001, got %s", latest.ID)
	}

	// Save a newer execution
	exec2 := &model.EvaluationExecution{
		ID:           "exec-002",
		OrgID:        "org-acme",
		TargetNodeID: "node-01",
		TriggerType:  model.EvaluationTriggerScheduled,
		EvaluatedAt:  now.Add(time.Hour),
		DurationNs:   15000000,
		Status:       model.EvaluationStatusNonCompliant,
		Results: []model.EvaluationResult{
			{
				RuleID:          "rule-cpu",
				PolicyID:        "pol-res",
				PolicyRevision:  1,
				Category:        model.PolicyCategoryResourceThresholds,
				RuleType:        model.RuleTypeResourceThreshold,
				Status:          model.EvaluationStatusNonCompliant,
				Severity:        model.SeverityCritical,
				EnforcementMode: model.EnforcementModeEnforce,
				DataFreshness:   model.DataFreshnessFresh,
				EvaluatedAt:     now.Add(time.Hour),
			},
		},
		Summary: model.EvaluationSummary{
			TotalRules:        1,
			NonCompliantRules: 1,
			ComplianceRatio:   0.0,
			CoverageRatio:     1.0,
		},
	}
	if err := store.SaveEvaluationExecution(ctx, exec2); err != nil {
		t.Fatalf("failed to save exec2: %v", err)
	}

	latest, err = store.GetLatestNodeEvaluation(ctx, "org-acme", "node-01")
	if err != nil {
		t.Fatalf("failed to get latest node evaluation: %v", err)
	}
	if latest.ID != "exec-002" {
		t.Errorf("expected latest ID exec-002, got %s", latest.ID)
	}

	// 4. List with filter
	list, err := store.ListEvaluationExecutions(ctx, model.EvaluationFilter{
		OrgID:  "org-acme",
		Status: model.EvaluationStatusNonCompliant,
	})
	if err != nil {
		t.Fatalf("failed to list evaluations: %v", err)
	}
	if len(list) != 1 || list[0].ID != "exec-002" {
		t.Errorf("expected 1 non-compliant execution, got %d", len(list))
	}

	// 5. Prune
	pruned, err := store.PruneEvaluationExecutions(ctx, 30*time.Minute)
	if err != nil {
		t.Fatalf("failed to prune evaluations: %v", err)
	}
	// exec-001 is older than 30m compared to now+1h? Note Prune calculates cutoff from time.Now().
	_ = pruned
}

func TestSQLiteStorage_ComplianceFinding_Lifecycle(t *testing.T) {
	store := newTestStorage(t)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Millisecond)

	finding := &model.ComplianceFinding{
		ID:              model.ComputeFindingID("org-acme", "node-01", "pol-res", "rule-cpu"),
		OrgID:           "org-acme",
		TargetNodeID:    "node-01",
		PolicyID:        "pol-res",
		PolicyRevision:  1,
		RuleID:          "rule-cpu",
		RuleName:        "CPU Rule",
		Category:        model.PolicyCategoryResourceThresholds,
		Severity:        model.SeverityCritical,
		EnforcementMode: model.EnforcementModeEnforce,
		Status:          model.FindingStatusOpen,
		FirstSeenAt:     now,
		LastSeenAt:      now,
		OccurrenceCount: 1,
		Message:         "CPU at 92% exceeds limit of 85%",
		ObservedValue:   "92%",
		ExpectedValue:   "<= 85%",
		ContextData: map[string]string{
			"core_count": "8",
		},
	}

	// 1. Save single finding
	if err := store.SaveComplianceFinding(ctx, finding); err != nil {
		t.Fatalf("failed to save compliance finding: %v", err)
	}

	// 2. Get by ID
	retrieved, err := store.GetComplianceFinding(ctx, finding.ID)
	if err != nil {
		t.Fatalf("failed to get finding: %v", err)
	}
	if retrieved.ID != finding.ID {
		t.Errorf("expected ID %s, got %s", finding.ID, retrieved.ID)
	}
	if retrieved.OccurrenceCount != 1 {
		t.Errorf("expected OccurrenceCount 1, got %d", retrieved.OccurrenceCount)
	}
	if retrieved.ContextData["core_count"] != "8" {
		t.Errorf("expected context data core_count 8, got %s", retrieved.ContextData["core_count"])
	}

	// 3. Update finding to Recurring
	finding.Status = model.FindingStatusRecurring
	finding.OccurrenceCount = 2
	finding.LastSeenAt = now.Add(10 * time.Minute)
	if err := store.SaveComplianceFinding(ctx, finding); err != nil {
		t.Fatalf("failed to update finding: %v", err)
	}

	retrieved, err = store.GetComplianceFinding(ctx, finding.ID)
	if err != nil {
		t.Fatalf("failed to get updated finding: %v", err)
	}
	if retrieved.Status != model.FindingStatusRecurring {
		t.Errorf("expected status recurring, got %s", retrieved.Status)
	}
	if retrieved.OccurrenceCount != 2 {
		t.Errorf("expected occurrence count 2, got %d", retrieved.OccurrenceCount)
	}

	// 4. Batch Save
	resolvedTime := now.Add(20 * time.Minute)
	finding.Status = model.FindingStatusResolved
	finding.ResolvedAt = &resolvedTime

	finding2 := &model.ComplianceFinding{
		ID:              model.ComputeFindingID("org-acme", "node-01", "pol-sec", "rule-ssh"),
		OrgID:           "org-acme",
		TargetNodeID:    "node-01",
		PolicyID:        "pol-sec",
		PolicyRevision:  2,
		RuleID:          "rule-ssh",
		RuleName:        "SSH Port Rule",
		Category:        model.PolicyCategoryOperationalCompliance,
		Severity:        model.SeverityCritical,
		EnforcementMode: model.EnforcementModeEnforce,
		Status:          model.FindingStatusOpen,
		FirstSeenAt:     now,
		LastSeenAt:      now,
		OccurrenceCount: 1,
		Message:         "SSH root login permitted",
	}

	if err := store.SaveComplianceFindings(ctx, []model.ComplianceFinding{*finding, *finding2}); err != nil {
		t.Fatalf("failed to batch save findings: %v", err)
	}

	// 5. List findings with filter
	list, err := store.ListComplianceFindings(ctx, model.FindingFilter{
		OrgID:  "org-acme",
		Status: model.FindingStatusOpen,
	})
	if err != nil {
		t.Fatalf("failed to list open findings: %v", err)
	}
	if len(list) != 1 || list[0].RuleID != "rule-ssh" {
		t.Errorf("expected 1 open finding for rule-ssh, got %d", len(list))
	}

	// 6. Delete finding
	if err := store.DeleteComplianceFinding(ctx, finding2.ID); err != nil {
		t.Fatalf("failed to delete finding: %v", err)
	}

	_, err = store.GetComplianceFinding(ctx, finding2.ID)
	if err == nil {
		t.Errorf("expected error getting deleted finding, got nil")
	}
}
