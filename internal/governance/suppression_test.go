package governance

import (
	"context"
	"testing"
	"time"

	"github.com/DocHoax/watchdog/internal/storage"
	"github.com/DocHoax/watchdog/pkg/model"
)

func setupTestSuppressionStore(t *testing.T) storage.Storage {
	t.Helper()
	store, err := storage.NewSQLiteStorage(storage.Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("failed to create sqlite storage: %v", err)
	}
	return store
}

func TestSuppressionEngine_SecurityAndIntegrityExemptions(t *testing.T) {
	ctx := context.Background()
	store := setupTestSuppressionStore(t)
	defer store.Close()

	now := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)
	engine := NewSuppressionEngine(store, nil, NewMockClock(now))

	// Add an active maintenance window covering the whole org
	mw := &model.MaintenanceWindow{
		ID:          "mw-all",
		OrgID:       "org-corp",
		Name:        "Global Maintenance",
		Status:      model.MaintenanceStatusActive,
		TargetScope: model.MaintenanceTargetScopeOrganization,
		Schedule: model.MaintenanceSchedule{
			StartTime: now.Add(-time.Hour),
			EndTime:   now.Add(time.Hour),
		},
		SuppressAlerts:      true,
		SuppressFindings:    true,
		AllowCriticalAlerts: false, // even if false, security must exempt!
	}
	if err := store.SaveMaintenanceWindow(ctx, mw); err != nil {
		t.Fatalf("failed to save maintenance window: %v", err)
	}

	testCases := []struct {
		name       string
		category   string
		ruleName   string
		ruleID     string
		message    string
		details    map[string]string
		wantExempt bool
		wantReason model.SuppressionReason
	}{
		{
			name:       "Security category event",
			category:   "security",
			ruleName:   "CPU High",
			wantExempt: true,
			wantReason: model.SuppressionReasonSecurityExemption,
		},
		{
			name:       "Auth failure in category",
			category:   "auth_compliance",
			ruleName:   "Login Audit",
			wantExempt: true,
			wantReason: model.SuppressionReasonSecurityExemption,
		},
		{
			name:       "Vulnerability keyword in category",
			category:   "vulnerability_scan",
			ruleName:   "CVE Check",
			wantExempt: true,
			wantReason: model.SuppressionReasonSecurityExemption,
		},
		{
			name:       "Integrity category event",
			category:   "integrity",
			ruleName:   "File integrity",
			wantExempt: true,
			wantReason: model.SuppressionReasonIntegrityExemption,
		},
		{
			name:       "Tamper in category",
			category:   "system_tamper",
			ruleName:   "Binary checksum",
			wantExempt: true,
			wantReason: model.SuppressionReasonIntegrityExemption,
		},
		{
			name:       "Corruption keyword in category",
			category:   "disk_corruption",
			ruleName:   "SMART check",
			wantExempt: true,
			wantReason: model.SuppressionReasonIntegrityExemption,
		},
		{
			name:       "Privilege escalation in rule name",
			category:   "system",
			ruleName:   "detect_privilege_escalation",
			wantExempt: true,
			wantReason: model.SuppressionReasonSecurityExemption,
		},
		{
			name:       "Unauthorized keyword in message",
			category:   "resource",
			ruleName:   "Process Audit",
			message:    "unauthorized process detected running as root",
			wantExempt: true,
			wantReason: model.SuppressionReasonSecurityExemption,
		},
		{
			name:       "Data corruption in message",
			category:   "storage",
			ruleName:   "ZFS Pool",
			message:    "data_corruption detected on sector 42",
			wantExempt: true,
			wantReason: model.SuppressionReasonIntegrityExemption,
		},
		{
			name:       "Explicit security tag in details",
			category:   "resource",
			ruleName:   "Memory Threshold",
			details:    map[string]string{"exemption": "security"},
			wantExempt: true,
			wantReason: model.SuppressionReasonSecurityExemption,
		},
		{
			name:       "Explicit integrity tag in details",
			category:   "resource",
			ruleName:   "Disk Full",
			details:    map[string]string{"integrity": "true"},
			wantExempt: true,
			wantReason: model.SuppressionReasonIntegrityExemption,
		},
		{
			name:       "Standard resource alert - NOT exempt",
			category:   "resource",
			ruleName:   "High Memory Utilization",
			message:    "Node memory utilization at 95%",
			wantExempt: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dec, err := engine.Evaluate(ctx, SuppressionEvaluationRequest{
				OrgID:         "org-corp",
				NodeID:        "node-1",
				Category:      tc.category,
				RuleName:      tc.ruleName,
				RuleID:        tc.ruleID,
				Message:       tc.message,
				Severity:      model.SeverityWarning,
				Timestamp:     now,
				CustomDetails: tc.details,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.wantExempt {
				if dec.Outcome != model.SuppressionOutcomeExempt {
					t.Errorf("expected outcome %s, got %s", model.SuppressionOutcomeExempt, dec.Outcome)
				}
				if dec.Reason != tc.wantReason {
					t.Errorf("expected reason %s, got %s", tc.wantReason, dec.Reason)
				}
			} else {
				if dec.Outcome != model.SuppressionOutcomeSuppressed {
					t.Errorf("expected standard alert to be suppressed by active window, got %s", dec.Outcome)
				}
			}
		})
	}
}

func TestSuppressionEngine_SeverityThresholdsAndGuardrails(t *testing.T) {
	ctx := context.Background()
	store := setupTestSuppressionStore(t)
	defer store.Close()

	now := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)
	engine := NewSuppressionEngine(store, nil, NewMockClock(now))

	node := &model.FleetNode{
		Identity: model.NodeIdentity{NodeID: "node-prod-01"},
		Metadata: map[string]string{"org_id": "org-corp"},
	}
	if err := store.SaveFleetNode(ctx, node); err != nil {
		t.Fatalf("failed to save fleet node: %v", err)
	}

	// Window with SeverityThreshold = WARNING and AllowCriticalAlerts = true
	mw := &model.MaintenanceWindow{
		ID:                  "mw-1",
		OrgID:               "org-corp",
		Name:                "Patching Window",
		Status:              model.MaintenanceStatusActive,
		TargetScope:         model.MaintenanceTargetScopeNode,
		TargetID:            "node-prod-01",
		Schedule:            model.MaintenanceSchedule{StartTime: now.Add(-time.Hour), EndTime: now.Add(time.Hour)},
		SuppressAlerts:      true,
		SuppressFindings:    true,
		AllowCriticalAlerts: true,
		SeverityThreshold:   model.SeverityWarning,
	}
	if err := store.SaveMaintenanceWindow(ctx, mw); err != nil {
		t.Fatalf("failed to save maintenance window: %v", err)
	}

	// 1. Info severity -> should be suppressed
	dec, err := engine.Evaluate(ctx, SuppressionEvaluationRequest{
		OrgID:     "org-corp",
		Node:      node,
		Category:  "resource",
		Severity:  model.SeverityInfo,
		Timestamp: now,
	})
	if err != nil || dec.Outcome != model.SuppressionOutcomeSuppressed {
		t.Fatalf("expected INFO alert to be suppressed, got %v (outcome=%s, err=%v)", dec, dec.Outcome, err)
	}

	// 2. Warning severity -> should be suppressed (within threshold)
	dec, err = engine.Evaluate(ctx, SuppressionEvaluationRequest{
		OrgID:     "org-corp",
		Node:      node,
		Category:  "resource",
		Severity:  model.SeverityWarning,
		Timestamp: now,
	})
	if err != nil || dec.Outcome != model.SuppressionOutcomeSuppressed {
		t.Fatalf("expected WARNING alert to be suppressed, got %v (outcome=%s, err=%v)", dec, dec.Outcome, err)
	}

	// 3. Critical severity -> should NOT be suppressed (AllowCriticalAlerts = true & exceeds threshold)
	dec, err = engine.Evaluate(ctx, SuppressionEvaluationRequest{
		OrgID:     "org-corp",
		Node:      node,
		Category:  "resource",
		Severity:  model.SeverityCritical,
		Timestamp: now,
	})
	if err != nil || dec.Outcome != model.SuppressionOutcomeNotSuppressed {
		t.Fatalf("expected CRITICAL alert NOT to be suppressed, got %v (outcome=%s, err=%v)", dec, dec.Outcome, err)
	}
	if dec.Reason != model.SuppressionReasonSeverityExceedsThreshold {
		t.Errorf("expected reason %s, got %s", model.SuppressionReasonSeverityExceedsThreshold, dec.Reason)
	}
}

func TestSuppressionEngine_CategoryRestrictions(t *testing.T) {
	ctx := context.Background()
	store := setupTestSuppressionStore(t)
	defer store.Close()

	now := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)
	engine := NewSuppressionEngine(store, nil, NewMockClock(now))

	mw := &model.MaintenanceWindow{
		ID:          "mw-cat",
		OrgID:       "org-corp",
		Name:        "Database Maintenance",
		Status:      model.MaintenanceStatusActive,
		TargetScope: model.MaintenanceTargetScopeOrganization,
		Schedule:    model.MaintenanceSchedule{StartTime: now.Add(-time.Hour), EndTime: now.Add(time.Hour)},
		SuppressAlerts:       true,
		CategoryRestrictions: []model.PolicyCategory{model.PolicyCategoryResourceThresholds, model.PolicyCategoryAnomalyDetection},
	}
	if err := store.SaveMaintenanceWindow(ctx, mw); err != nil {
		t.Fatalf("failed to save maintenance window: %v", err)
	}

	// 1. Resource category -> suppressed
	dec, err := engine.Evaluate(ctx, SuppressionEvaluationRequest{
		OrgID:     "org-corp",
		NodeID:    "node-1",
		Category:  "resource",
		Severity:  model.SeverityWarning,
		Timestamp: now,
	})
	if err != nil || dec.Outcome != model.SuppressionOutcomeSuppressed {
		t.Fatalf("expected resource alert to be suppressed, got outcome=%s", dec.Outcome)
	}

	// 2. Incident severity category -> not covered, so NOT suppressed
	dec, err = engine.Evaluate(ctx, SuppressionEvaluationRequest{
		OrgID:     "org-corp",
		NodeID:    "node-1",
		Category:  "incident_severity",
		Severity:  model.SeverityWarning,
		Timestamp: now,
	})
	if err != nil || dec.Outcome != model.SuppressionOutcomeNotSuppressed {
		t.Fatalf("expected incident_severity alert NOT to be suppressed, got outcome=%s", dec.Outcome)
	}
	if dec.Reason != model.SuppressionReasonCategoryNotCovered {
		t.Errorf("expected reason %s, got %s", model.SuppressionReasonCategoryNotCovered, dec.Reason)
	}
}

func TestSuppressionEngine_SuppressAlertsVsFindingsFlags(t *testing.T) {
	ctx := context.Background()
	store := setupTestSuppressionStore(t)
	defer store.Close()

	now := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)
	engine := NewSuppressionEngine(store, nil, NewMockClock(now))

	// Window only suppresses findings, NOT alerts
	mw := &model.MaintenanceWindow{
		ID:               "mw-findings-only",
		OrgID:            "org-corp",
		Name:             "Audit Maintenance",
		Status:           model.MaintenanceStatusActive,
		TargetScope:      model.MaintenanceTargetScopeOrganization,
		Schedule:         model.MaintenanceSchedule{StartTime: now.Add(-time.Hour), EndTime: now.Add(time.Hour)},
		SuppressAlerts:   false,
		SuppressFindings: true,
	}
	if err := store.SaveMaintenanceWindow(ctx, mw); err != nil {
		t.Fatalf("failed to save maintenance window: %v", err)
	}

	// Evaluate Alert -> NotSuppressed
	alert := &model.AlertEvent{
		ID:       "alert-1",
		RuleID:   "rule-cpu",
		RuleName: "CPU Alert",
		Severity: model.SeverityWarning,
		Message:  "CPU high",
		FiredAt:  now,
	}
	dec, err := engine.EvaluateAlert(ctx, "org-corp", nil, nil, nil, nil, alert, "resource", now)
	if err != nil || dec.Outcome != model.SuppressionOutcomeNotSuppressed {
		t.Fatalf("expected alert NOT to be suppressed when SuppressAlerts=false, got outcome=%s", dec.Outcome)
	}
	if dec.Reason != model.SuppressionReasonWindowAlertsNotSuppressed {
		t.Errorf("expected reason %s, got %s", model.SuppressionReasonWindowAlertsNotSuppressed, dec.Reason)
	}

	// Evaluate Finding -> Suppressed
	finding := &model.ComplianceFinding{
		ID:           "finding-1",
		TargetNodeID: "node-1",
		RuleID:       "rule-audit",
		Category:     model.PolicyCategoryResourceThresholds,
		Severity:     model.SeverityWarning,
		Message:      "Audit threshold exceeded",
	}
	decFinding, err := engine.EvaluateFinding(ctx, "org-corp", nil, nil, nil, nil, finding, now)
	if err != nil || decFinding.Outcome != model.SuppressionOutcomeSuppressed {
		t.Fatalf("expected finding to be suppressed when SuppressFindings=true, got outcome=%s", decFinding.Outcome)
	}
}

func TestSuppressionEngine_MultiTenantIsolation(t *testing.T) {
	ctx := context.Background()
	store := setupTestSuppressionStore(t)
	defer store.Close()

	now := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)
	engine := NewSuppressionEngine(store, nil, NewMockClock(now))

	// Window belongs to org-alpha
	mw := &model.MaintenanceWindow{
		ID:               "mw-alpha",
		OrgID:            "org-alpha",
		Name:             "Alpha Maintenance",
		Status:           model.MaintenanceStatusActive,
		TargetScope:      model.MaintenanceTargetScopeOrganization,
		Schedule:         model.MaintenanceSchedule{StartTime: now.Add(-time.Hour), EndTime: now.Add(time.Hour)},
		SuppressAlerts:   true,
		SuppressFindings: true,
	}
	if err := store.SaveMaintenanceWindow(ctx, mw); err != nil {
		t.Fatalf("failed to save maintenance window: %v", err)
	}

	// Evaluation for org-beta should NOT match alpha's window
	dec, err := engine.Evaluate(ctx, SuppressionEvaluationRequest{
		OrgID:     "org-beta",
		NodeID:    "node-beta-1",
		Category:  "resource",
		Severity:  model.SeverityWarning,
		Timestamp: now,
	})
	if err != nil || dec.Outcome != model.SuppressionOutcomeNotSuppressed {
		t.Fatalf("expected cross-tenant alert NOT to be suppressed, got outcome=%s", dec.Outcome)
	}
	if dec.Reason != model.SuppressionReasonNoActiveWindow {
		t.Errorf("expected reason %s, got %s", model.SuppressionReasonNoActiveWindow, dec.Reason)
	}
}

func TestSuppressionEngine_RecordDecision(t *testing.T) {
	ctx := context.Background()
	store := setupTestSuppressionStore(t)
	defer store.Close()

	now := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)
	engine := NewSuppressionEngine(store, nil, NewMockClock(now))

	dec, err := engine.EvaluateIncident(ctx, "org-corp", "node-1", "inc-100", model.SeverityWarning, "resource", "High Latency", now)
	if err != nil {
		t.Fatalf("unexpected evaluate error: %v", err)
	}

	if err := engine.RecordDecision(ctx, store, dec); err != nil {
		t.Fatalf("failed to record decision: %v", err)
	}

	retrieved, err := store.GetSuppressionDecision(ctx, dec.ID)
	if err != nil {
		t.Fatalf("failed to retrieve decision from store: %v", err)
	}
	if retrieved == nil {
		t.Fatalf("expected retrieved decision, got nil")
	}
	if retrieved.ID != dec.ID || retrieved.IncidentID != "inc-100" {
		t.Errorf("retrieved decision mismatch: got %+v, want %+v", retrieved, dec)
	}
}
