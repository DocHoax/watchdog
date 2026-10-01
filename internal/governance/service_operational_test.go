package governance

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DocHoax/watchdog/internal/incidents"
	"github.com/DocHoax/watchdog/internal/storage"
	"github.com/DocHoax/watchdog/pkg/model"
)

func TestGovernanceService_MaintenanceWindow_Lifecycle(t *testing.T) {
	svc, store := setupTestService(t)
	defer store.Close()

	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Millisecond)

	win := &model.MaintenanceWindow{
		ID:          "win-ops-001",
		OrgID:       model.DefaultOrganizationID,
		Name:        "Weekly Kernel Upgrade",
		Description: "Operating system patches for production cluster",
		Status:      model.MaintenanceStatusScheduled,
		TargetScope: model.MaintenanceTargetScopeFleetGroup,
		TargetID:    "grp-prod",
		Schedule: model.MaintenanceSchedule{
			StartTime: now.Add(1 * time.Hour),
			EndTime:   now.Add(5 * time.Hour),
			TimeZone:  "UTC",
		},
		SuppressAlerts:      true,
		SuppressFindings:    true,
		AllowCriticalAlerts: true,
	}

	// 1. Create maintenance window
	if err := svc.CreateMaintenanceWindow(ctx, win); err != nil {
		t.Fatalf("failed to create maintenance window: %v", err)
	}

	// Duplicate creation should fail
	if err := svc.CreateMaintenanceWindow(ctx, win); !errors.Is(err, ErrMaintenanceWindowExists) {
		t.Errorf("expected ErrMaintenanceWindowExists, got %v", err)
	}

	// 2. Get maintenance window
	got, err := svc.GetMaintenanceWindow(ctx, "win-ops-001")
	if err != nil {
		t.Fatalf("failed to get maintenance window: %v", err)
	}
	if got.ID != "win-ops-001" || got.Name != "Weekly Kernel Upgrade" {
		t.Errorf("mismatched window fields: %+v", got)
	}

	// 3. List maintenance windows
	list, err := svc.ListMaintenanceWindows(ctx, model.MaintenanceWindowFilter{
		OrgID:  model.DefaultOrganizationID,
		Status: model.MaintenanceStatusScheduled,
	})
	if err != nil {
		t.Fatalf("failed to list maintenance windows: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("expected 1 window, got %d", len(list))
	}

	// 4. Update maintenance window
	got.Description = "Updated description for kernel rollout"
	if err := svc.UpdateMaintenanceWindow(ctx, got); err != nil {
		t.Fatalf("failed to update maintenance window: %v", err)
	}
	updated, _ := svc.GetMaintenanceWindow(ctx, "win-ops-001")
	if updated.Description != "Updated description for kernel rollout" {
		t.Errorf("expected updated description, got %s", updated.Description)
	}

	// 5. Evaluate active windows at evalTime (active period)
	activeList, err := svc.EvaluateMaintenanceWindows(ctx, model.DefaultOrganizationID, now.Add(2*time.Hour))
	if err != nil {
		t.Fatalf("failed to evaluate active windows: %v", err)
	}
	if len(activeList) != 1 {
		t.Errorf("expected 1 active window during scheduled time, got %d", len(activeList))
	}

	// 6. Cancel maintenance window
	if err := svc.CancelMaintenanceWindow(ctx, "win-ops-001", "Deployment postponed"); err != nil {
		t.Fatalf("failed to cancel maintenance window: %v", err)
	}
	cancelled, _ := svc.GetMaintenanceWindow(ctx, "win-ops-001")
	if cancelled.Status != model.MaintenanceStatusCancelled {
		t.Errorf("expected status cancelled, got %s", cancelled.Status)
	}
	if cancelled.Metadata["cancel_reason"] != "Deployment postponed" {
		t.Errorf("expected cancel reason in metadata, got %v", cancelled.Metadata)
	}

	// 7. Delete maintenance window
	if err := svc.DeleteMaintenanceWindow(ctx, "win-ops-001"); err != nil {
		t.Fatalf("failed to delete maintenance window: %v", err)
	}
	if _, err := svc.GetMaintenanceWindow(ctx, "win-ops-001"); !errors.Is(err, ErrMaintenanceWindowNotFound) {
		t.Errorf("expected ErrMaintenanceWindowNotFound, got %v", err)
	}
}

func TestGovernanceService_EscalationPolicy_Lifecycle(t *testing.T) {
	svc, store := setupTestService(t)
	defer store.Close()

	ctx := context.Background()

	policy := &model.EscalationPolicy{
		ID:          "esc-ops-001",
		OrgID:       model.DefaultOrganizationID,
		Name:        "Platform Escalation Chain",
		Description: "Escalation rules for platform team",
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
						TargetID:    "team-platform",
						Name:        "Platform On-Call",
						ContactInfo: "#platform-alerts",
					},
				},
			},
			{
				StageNumber:  2,
				DelayMinutes: 10,
				Channel:      model.EscalationChannelPagerDuty,
				Targets: []model.EscalationTarget{
					{
						TargetType:  model.EscalationTargetUser,
						TargetID:    "user-lead",
						Name:        "Platform Lead",
						ContactInfo: "lead@corp.internal",
					},
				},
			},
		},
	}

	// 1. Create escalation policy
	if err := svc.CreateEscalationPolicy(ctx, policy); err != nil {
		t.Fatalf("failed to create escalation policy: %v", err)
	}

	// Duplicate creation
	if err := svc.CreateEscalationPolicy(ctx, policy); !errors.Is(err, ErrEscalationPolicyExists) {
		t.Errorf("expected ErrEscalationPolicyExists, got %v", err)
	}

	// 2. Get escalation policy
	got, err := svc.GetEscalationPolicy(ctx, "esc-ops-001")
	if err != nil {
		t.Fatalf("failed to get escalation policy: %v", err)
	}
	if got.ID != "esc-ops-001" || len(got.Stages) != 2 {
		t.Errorf("unexpected escalation policy data: %+v", got)
	}

	// 3. List escalation policies
	list, err := svc.ListEscalationPolicies(ctx, model.EscalationPolicyFilter{
		OrgID: model.DefaultOrganizationID,
	})
	if err != nil {
		t.Fatalf("failed to list escalation policies: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("expected 1 escalation policy, got %d", len(list))
	}

	// 4. Update escalation policy
	got.Description = "Updated description"
	if err := svc.UpdateEscalationPolicy(ctx, got); err != nil {
		t.Fatalf("failed to update escalation policy: %v", err)
	}
	updated, _ := svc.GetEscalationPolicy(ctx, "esc-ops-001")
	if updated.Description != "Updated description" {
		t.Errorf("expected updated description, got %s", updated.Description)
	}

	// 5. Evaluate incident escalation
	incidentTime := time.Now().UTC().Add(-15 * time.Minute)
	inc := &incidents.Incident{
		ID:        "inc-001",
		Title:     "API Gateway Outage",
		Severity:  model.SeverityCritical,
		Status:    incidents.IncidentStatusInvestigating,
		StartTime: incidentTime,
	}

	res, err := svc.EvaluateIncidentEscalation(ctx, model.DefaultOrganizationID, inc, time.Now().UTC())
	if err != nil {
		t.Fatalf("failed to evaluate escalation: %v", err)
	}
	if res.PolicyID != "esc-ops-001" || res.CurrentStage != 2 {
		t.Errorf("expected stage 2 for incident > 10m old, got stage %d", res.CurrentStage)
	}
	if res.Channel != model.EscalationChannelPagerDuty {
		t.Errorf("expected pagerduty channel, got %s", res.Channel)
	}

	// 6. Delete escalation policy
	if err := svc.DeleteEscalationPolicy(ctx, "esc-ops-001"); err != nil {
		t.Fatalf("failed to delete escalation policy: %v", err)
	}
	if _, err := svc.GetEscalationPolicy(ctx, "esc-ops-001"); !errors.Is(err, ErrEscalationPolicyNotFound) {
		t.Errorf("expected ErrEscalationPolicyNotFound, got %v", err)
	}
}

func TestGovernanceService_Ownership_Resolution(t *testing.T) {
	svc, store := setupTestService(t)
	defer store.Close()

	ctx := context.Background()

	// 1. Create fleet group with ownership metadata
	group := &model.FleetGroup{
		ID:    "grp-infra",
		OrgID: model.DefaultOrganizationID,
		Name:  "Infrastructure Group",
		Type:  model.GroupTypeEnvironment,
		Metadata: map[string]string{
			"owner_team":      "Infra Team",
			"contact_channel": "#infra-ops",
			"environment":     "production",
		},
	}
	if err := svc.CreateFleetGroup(ctx, group); err != nil {
		t.Fatalf("failed to create group: %v", err)
	}

	// 2. Create node and assign to group
	node := &model.FleetNode{
		Identity: model.NodeIdentity{
			NodeID:   "node-srv-infra",
			Hostname: "infra-01.company.internal",
		},
		Status:       model.NodeStatusHealthy,
		RegisteredAt: time.Now().UTC(),
		Metadata: map[string]string{
			"contact_email": "ops-lead@company.com", // Node-level override for contact_email only
		},
	}
	if err := store.SaveFleetNode(ctx, node); err != nil {
		t.Fatalf("failed to save node: %v", err)
	}

	if err := svc.AddMember(ctx, &model.FleetGroupMember{
		GroupID: "grp-infra",
		NodeID:  "node-srv-infra",
		Role:    model.MembershipRolePrimary,
	}); err != nil {
		t.Fatalf("failed to add node to group: %v", err)
	}

	// 3. Resolve ownership
	resolved, err := svc.ResolveNodeOwnership(ctx, "node-srv-infra")
	if err != nil {
		t.Fatalf("failed to resolve node ownership: %v", err)
	}

	if resolved.OwnerTeam != "Infra Team" {
		t.Errorf("expected owner team 'Infra Team' from group, got %s", resolved.OwnerTeam)
	}
	if resolved.ContactEmail != "ops-lead@company.com" {
		t.Errorf("expected contact email 'ops-lead@company.com' from node metadata, got %s", resolved.ContactEmail)
	}
	if resolved.ContactChannel != "#infra-ops" {
		t.Errorf("expected contact channel '#infra-ops' from group metadata, got %s", resolved.ContactChannel)
	}
}

func TestGovernanceService_Suppression_And_Audit(t *testing.T) {
	svc, store := setupTestService(t)
	defer store.Close()

	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Millisecond)

	// Create active maintenance window
	win := &model.MaintenanceWindow{
		ID:          "win-active-001",
		OrgID:       model.DefaultOrganizationID,
		Name:        "Emergency Maintenance",
		Status:      model.MaintenanceStatusActive,
		TargetScope: model.MaintenanceTargetScopeOrganization,
		TargetID:    model.DefaultOrganizationID,
		Schedule: model.MaintenanceSchedule{
			StartTime: now.Add(-1 * time.Hour),
			EndTime:   now.Add(1 * time.Hour),
			TimeZone:  "UTC",
		},
		SuppressAlerts:      true,
		SuppressFindings:    true,
		AllowCriticalAlerts: true,
	}
	if err := svc.CreateMaintenanceWindow(ctx, win); err != nil {
		t.Fatalf("failed to create window: %v", err)
	}

	// 1. Evaluate suppression for warning alert (should be suppressed)
	decision, err := svc.EvaluateSuppression(ctx, SuppressionEvaluationRequest{
		OrgID:     model.DefaultOrganizationID,
		AlertID:   "alt-warn-001",
		NodeID:    "node-srv-01",
		RuleID:    "rule-high-mem",
		RuleName:  "High Memory",
		Category:  string(model.PolicyCategoryResourceThresholds),
		Severity:  model.SeverityWarning,
		Message:   "Memory usage at 85%",
		Timestamp: now,
	})
	if err != nil {
		t.Fatalf("failed to evaluate suppression: %v", err)
	}
	if decision.Outcome != model.SuppressionOutcomeSuppressed {
		t.Errorf("expected Suppressed outcome for warning in active window, got %s", decision.Outcome)
	}

	// 2. Evaluate suppression for critical security alert (should be exempt)
	secDecision, err := svc.EvaluateSuppression(ctx, SuppressionEvaluationRequest{
		OrgID:     model.DefaultOrganizationID,
		AlertID:   "alt-sec-001",
		NodeID:    "node-srv-01",
		RuleID:    "rule-cve-exploit",
		RuleName:  "CVE Exploit Detected",
		Category:  string(model.PolicyCategoryOperationalCompliance),
		Severity:  model.SeverityCritical,
		Message:   "CVE-2026-9999 privilege escalation exploit attempted",
		Timestamp: now,
	})
	if err != nil {
		t.Fatalf("failed to evaluate security suppression: %v", err)
	}
	if secDecision.Outcome != model.SuppressionOutcomeExempt {
		t.Errorf("expected Exempt outcome for critical security event, got %s", secDecision.Outcome)
	}

	// 3. Get suppression decision by ID
	gotDecision, err := svc.GetSuppressionDecision(ctx, decision.ID)
	if err != nil {
		t.Fatalf("failed to get suppression decision: %v", err)
	}
	if gotDecision.ID != decision.ID || gotDecision.AlertID != "alt-warn-001" {
		t.Errorf("mismatched decision: %+v", gotDecision)
	}

	// 4. List suppression decisions
	decList, err := svc.ListSuppressionDecisions(ctx, model.SuppressionFilter{
		OrgID: model.DefaultOrganizationID,
	})
	if err != nil {
		t.Fatalf("failed to list suppression decisions: %v", err)
	}
	if len(decList) != 2 {
		t.Errorf("expected 2 suppression decisions, got %d", len(decList))
	}

	// 5. Test Audit Trail & Credential Redaction
	auditEvent := model.AuditEvent{
		EventType: "governance.test_event",
		Severity:  "info",
		Outcome:   "success",
		Actor: model.AuditActor{
			Type:     "user",
			Identity: "admin-alice",
		},
		Resource: "win-active-001",
		Action:   "create",
		Message:  "User admin-alice configured token Bearer secret-auth-token-1234567890",
		Metadata: map[string]string{
			"api_key":      "super-secret-api-key-xyz",
			"public_info":  "cluster-a",
		},
	}
	if err := svc.RecordAuditEvent(ctx, auditEvent); err != nil {
		t.Fatalf("failed to record audit event: %v", err)
	}

	// Query audit events
	auditLogs, err := svc.QueryAuditEvents(ctx, storage.AuditFilter{
		EventType: "governance.test_event",
	})
	if err != nil {
		t.Fatalf("failed to query audit events: %v", err)
	}
	if len(auditLogs) != 1 {
		t.Fatalf("expected 1 audit log, got %d", len(auditLogs))
	}

	rec := auditLogs[0]
	if rec.Metadata["api_key"] != RedactedPlaceholder {
		t.Errorf("expected api_key to be redacted, got %s", rec.Metadata["api_key"])
	}
	if rec.Metadata["public_info"] != "cluster-a" {
		t.Errorf("expected public_info to remain unredacted, got %s", rec.Metadata["public_info"])
	}
}
