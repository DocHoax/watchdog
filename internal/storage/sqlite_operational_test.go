package storage

import (
	"context"
	"testing"
	"time"

	"github.com/DocHoax/watchdog/pkg/model"
)

func TestSQLiteStorage_MaintenanceWindow_Lifecycle(t *testing.T) {
	store := newTestStorage(t)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Millisecond)
	future := now.Add(4 * time.Hour)

	win := &model.MaintenanceWindow{
		ID:          "win-test-001",
		OrgID:       "org-acme",
		Name:        "Database Cluster Patching",
		Description: "Scheduled OS and PostgreSQL minor upgrade",
		Status:      model.MaintenanceStatusScheduled,
		TargetScope: model.MaintenanceTargetScopeFleetGroup,
		TargetID:    "group-db",
		CategoryRestrictions: []model.PolicyCategory{
			model.PolicyCategoryResourceThresholds,
			model.PolicyCategoryAnomalyDetection,
		},
		SeverityThreshold: model.SeverityWarning,
		Schedule: model.MaintenanceSchedule{
			StartTime: now,
			EndTime:   future,
			TimeZone:  "America/New_York",
			Recurrence: &model.RecurrenceSchedule{
				Frequency:      model.RecurrenceFrequencyWeekly,
				Interval:       1,
				DaysOfWeek:     []time.Weekday{time.Saturday},
				MaxOccurrences: 12,
				Duration:       4 * time.Hour,
			},
		},
		SuppressAlerts:      true,
		SuppressFindings:    true,
		AllowCriticalAlerts: true,
		CreatedAt:           now,
		UpdatedAt:           now,
		CreatedBy:           "admin-user",
		Metadata: map[string]string{
			"change_ticket": "CHG-99882",
		},
	}

	// 1. Save maintenance window
	if err := store.SaveMaintenanceWindow(ctx, win); err != nil {
		t.Fatalf("failed to save maintenance window: %v", err)
	}

	// 2. Get maintenance window
	got, err := store.GetMaintenanceWindow(ctx, "win-test-001")
	if err != nil {
		t.Fatalf("failed to get maintenance window: %v", err)
	}
	if got.ID != win.ID || got.Name != win.Name || got.Status != win.Status {
		t.Errorf("mismatched window attributes: got %+v, want %+v", got, win)
	}
	if got.Schedule.Recurrence == nil || got.Schedule.Recurrence.Frequency != model.RecurrenceFrequencyWeekly {
		t.Errorf("recurrence schedule not preserved: got %+v", got.Schedule.Recurrence)
	}
	if len(got.CategoryRestrictions) != 2 {
		t.Errorf("category restrictions not preserved: got %v", got.CategoryRestrictions)
	}

	// 3. List maintenance windows with filters
	list, err := store.ListMaintenanceWindows(ctx, model.MaintenanceWindowFilter{
		OrgID:  "org-acme",
		Status: model.MaintenanceStatusScheduled,
	})
	if err != nil {
		t.Fatalf("failed to list maintenance windows: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("expected 1 window, got %d", len(list))
	}

	// ActiveAt filter
	activeTime := now.Add(1 * time.Hour)
	listActive, err := store.ListMaintenanceWindows(ctx, model.MaintenanceWindowFilter{
		OrgID:    "org-acme",
		ActiveAt: &activeTime,
	})
	if err != nil {
		t.Fatalf("failed to list active maintenance windows: %v", err)
	}
	if len(listActive) != 1 {
		t.Errorf("expected 1 active window, got %d", len(listActive))
	}

	// 4. Update status
	got.Status = model.MaintenanceStatusActive
	if err := store.SaveMaintenanceWindow(ctx, got); err != nil {
		t.Fatalf("failed to update maintenance window: %v", err)
	}
	updated, err := store.GetMaintenanceWindow(ctx, "win-test-001")
	if err != nil {
		t.Fatalf("failed to get updated window: %v", err)
	}
	if updated.Status != model.MaintenanceStatusActive {
		t.Errorf("expected status %s, got %s", model.MaintenanceStatusActive, updated.Status)
	}

	// 5. Delete maintenance window
	if err := store.DeleteMaintenanceWindow(ctx, "win-test-001"); err != nil {
		t.Fatalf("failed to delete maintenance window: %v", err)
	}
	if _, err := store.GetMaintenanceWindow(ctx, "win-test-001"); err == nil {
		t.Errorf("expected error getting deleted maintenance window")
	}
}

func TestSQLiteStorage_EscalationPolicy_Lifecycle(t *testing.T) {
	store := newTestStorage(t)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Millisecond)

	policy := &model.EscalationPolicy{
		ID:          "esc-test-001",
		OrgID:       "org-acme",
		Name:        "SRE Critical Escalation",
		Description: "Multi-tier escalation policy for critical infrastructure incidents",
		Enabled:     true,
		SeverityLevels: []model.Severity{
			model.SeverityCritical,
			model.SeverityWarning,
		},
		Stages: []model.EscalationStage{
			{
				StageNumber:  1,
				DelayMinutes: 0,
				Channel:      model.EscalationChannelSlack,
				Targets: []model.EscalationTarget{
					{
						TargetType:  model.EscalationTargetTeam,
						TargetID:    "team-sre",
						Name:        "SRE On-Call Primary",
						ContactInfo: "#sre-alerts",
					},
				},
			},
			{
				StageNumber:  2,
				DelayMinutes: 15,
				Channel:      model.EscalationChannelPagerDuty,
				Targets: []model.EscalationTarget{
					{
						TargetType:  model.EscalationTargetSchedule,
						TargetID:    "pd-schedule-tier2",
						Name:        "Tier 2 Escalation Schedule",
						ContactInfo: "P998877",
					},
				},
				FallbackTarget: &model.EscalationTarget{
					TargetType:  model.EscalationTargetUser,
					TargetID:    "user-sre-lead",
					Name:        "SRE Lead",
					ContactInfo: "lead@acme.corp",
				},
			},
		},
		CreatedAt: now,
		UpdatedAt: now,
		Metadata: map[string]string{
			"team": "sre",
		},
	}

	// 1. Save escalation policy
	if err := store.SaveEscalationPolicy(ctx, policy); err != nil {
		t.Fatalf("failed to save escalation policy: %v", err)
	}

	// 2. Get escalation policy
	got, err := store.GetEscalationPolicy(ctx, "esc-test-001")
	if err != nil {
		t.Fatalf("failed to get escalation policy: %v", err)
	}
	if got.ID != policy.ID || got.Name != policy.Name || !got.Enabled {
		t.Errorf("mismatched escalation policy: got %+v, want %+v", got, policy)
	}
	if len(got.Stages) != 2 {
		t.Fatalf("expected 2 stages, got %d", len(got.Stages))
	}
	if got.Stages[1].FallbackTarget == nil || got.Stages[1].FallbackTarget.TargetID != "user-sre-lead" {
		t.Errorf("fallback target not preserved: got %+v", got.Stages[1].FallbackTarget)
	}

	// 3. List escalation policies
	enabled := true
	list, err := store.ListEscalationPolicies(ctx, model.EscalationPolicyFilter{
		OrgID:    "org-acme",
		Enabled:  &enabled,
		Severity: model.SeverityCritical,
	})
	if err != nil {
		t.Fatalf("failed to list escalation policies: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("expected 1 escalation policy, got %d", len(list))
	}

	// 4. Delete escalation policy
	if err := store.DeleteEscalationPolicy(ctx, "esc-test-001"); err != nil {
		t.Fatalf("failed to delete escalation policy: %v", err)
	}
	if _, err := store.GetEscalationPolicy(ctx, "esc-test-001"); err == nil {
		t.Errorf("expected error getting deleted escalation policy")
	}
}

func TestSQLiteStorage_SuppressionDecision_Lifecycle(t *testing.T) {
	store := newTestStorage(t)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Millisecond)

	decision1 := model.SuppressionDecision{
		ID:          "sup-test-001",
		OrgID:       "org-acme",
		AlertID:     "alt-001",
		IncidentID:  "inc-001",
		NodeID:      "node-001",
		WindowID:    "win-001",
		RuleID:      "rule-high-cpu",
		RuleName:    "High CPU Utilization",
		Category:    string(model.PolicyCategoryResourceThresholds),
		Severity:    model.SeverityWarning,
		Outcome:     model.SuppressionOutcomeSuppressed,
		Reason:      model.SuppressionReasonInMaintenanceWindow,
		Message:     "Alert suppressed due to active database maintenance window",
		EvaluatedAt: now,
		Details: map[string]string{
			"window_name": "Database Upgrade",
		},
	}

	decision2 := model.SuppressionDecision{
		ID:          "sup-test-002",
		OrgID:       "org-acme",
		AlertID:     "alt-002",
		NodeID:      "node-002",
		RuleID:      "rule-disk-space",
		RuleName:    "Disk Space Full",
		Category:    string(model.PolicyCategoryCapacityPlanning),
		Severity:    model.SeverityCritical,
		Outcome:     model.SuppressionOutcomeExempt,
		Reason:      model.SuppressionReasonSeverityExceedsThreshold,
		Message:     "Critical alert exempt from maintenance window suppression",
		EvaluatedAt: now.Add(5 * time.Minute),
	}

	// 1. Batch save
	if err := store.SaveSuppressionDecisions(ctx, []model.SuppressionDecision{decision1, decision2}); err != nil {
		t.Fatalf("failed to save suppression decisions: %v", err)
	}

	// 2. Get single decision
	got, err := store.GetSuppressionDecision(ctx, "sup-test-001")
	if err != nil {
		t.Fatalf("failed to get suppression decision: %v", err)
	}
	if got.ID != decision1.ID || got.Outcome != decision1.Outcome || got.Reason != decision1.Reason {
		t.Errorf("mismatched decision attributes: got %+v, want %+v", got, decision1)
	}
	if got.Details["window_name"] != "Database Upgrade" {
		t.Errorf("details not preserved: got %v", got.Details)
	}

	// 3. List decisions with filters
	list, err := store.ListSuppressionDecisions(ctx, model.SuppressionFilter{
		OrgID:   "org-acme",
		Outcome: model.SuppressionOutcomeSuppressed,
	})
	if err != nil {
		t.Fatalf("failed to list suppression decisions: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("expected 1 suppressed decision, got %d", len(list))
	}

	// Filter by node
	listNode, err := store.ListSuppressionDecisions(ctx, model.SuppressionFilter{
		NodeID: "node-002",
	})
	if err != nil {
		t.Fatalf("failed to list suppression decisions by node: %v", err)
	}
	if len(listNode) != 1 || listNode[0].ID != "sup-test-002" {
		t.Errorf("expected decision2, got %+v", listNode)
	}

	// 4. Prune decisions
	pruned, err := store.PruneSuppressionDecisions(ctx, 1*time.Minute)
	if err != nil {
		t.Fatalf("failed to prune suppression decisions: %v", err)
	}
	if pruned != 0 {
		// Evaluated recently, so none should be pruned with 1 minute retention
		t.Errorf("expected 0 pruned, got %d", pruned)
	}
}
