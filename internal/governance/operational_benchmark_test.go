package governance

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/DocHoax/watchdog/internal/storage"
	"github.com/DocHoax/watchdog/pkg/model"
)

func BenchmarkMaintenanceEngine_IsWindowActive_WeeklyRecurrence(b *testing.B) {
	clock := NewMockClock(time.Date(2026, 10, 1, 14, 30, 0, 0, time.UTC))
	engine := NewMaintenanceEngine(nil, clock)

	start := time.Date(2026, 1, 1, 2, 0, 0, 0, time.UTC)
	end := start.Add(4 * time.Hour)

	window := &model.MaintenanceWindow{
		ID:          "win-bench-01",
		OrgID:       model.DefaultOrganizationID,
		Name:        "Weekly Weekend Maintenance",
		Status:      model.MaintenanceStatusActive,
		TargetScope: model.MaintenanceTargetScopeOrganization,
		Schedule: model.MaintenanceSchedule{
			StartTime: start,
			EndTime:   end,
			TimeZone:  "America/New_York",
			Recurrence: &model.RecurrenceSchedule{
				Frequency:      model.RecurrenceFrequencyWeekly,
				Interval:       1,
				DaysOfWeek:     []time.Weekday{time.Saturday, time.Sunday},
				Duration:       4 * time.Hour,
				MaxOccurrences: 52,
			},
		},
	}

	evalTime := time.Date(2026, 6, 6, 3, 0, 0, 0, time.UTC) // Saturday during maintenance

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		active, err := engine.IsWindowActive(window, evalTime)
		if err != nil {
			b.Fatalf("IsWindowActive failed: %v", err)
		}
		if !active {
			b.Fatalf("expected window to be active")
		}
	}
}

func BenchmarkSuppressionEngine_EvaluateAlert(b *testing.B) {
	ctx := context.Background()
	store, err := storage.NewSQLiteStorage(storage.Config{Path: ":memory:"})
	if err != nil {
		b.Fatalf("failed to create storage: %v", err)
	}
	defer store.Close()

	clock := NewMockClock(time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC))
	engine := NewSuppressionEngine(store, nil, clock)

	// Seed an active maintenance window
	now := clock.Now()
	win := &model.MaintenanceWindow{
		ID:          "win-bench-sup",
		OrgID:       model.DefaultOrganizationID,
		Name:        "Active Cluster Maintenance",
		Status:      model.MaintenanceStatusActive,
		TargetScope: model.MaintenanceTargetScopeFleetGroup,
		TargetID:    "grp-bench",
		CategoryRestrictions: []model.PolicyCategory{
			model.PolicyCategoryResourceThresholds,
		},
		SeverityThreshold: model.SeverityWarning,
		Schedule: model.MaintenanceSchedule{
			StartTime: now.Add(-1 * time.Hour),
			EndTime:   now.Add(1 * time.Hour),
			TimeZone:  "UTC",
		},
		SuppressAlerts:      true,
		SuppressFindings:    true,
		AllowCriticalAlerts: true,
	}
	if err := store.SaveMaintenanceWindow(ctx, win); err != nil {
		b.Fatalf("failed to save window: %v", err)
	}

	req := SuppressionEvaluationRequest{
		OrgID:     model.DefaultOrganizationID,
		AlertID:   "alt-bench-01",
		NodeID:    "node-bench-01",
		GroupIDs:  []string{"grp-bench"},
		RuleID:    "rule-high-cpu",
		RuleName:  "High CPU",
		Category:  string(model.PolicyCategoryResourceThresholds),
		Severity:  model.SeverityWarning,
		Message:   "CPU at 88%",
		Timestamp: now,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		dec, err := engine.Evaluate(ctx, req)
		if err != nil {
			b.Fatalf("Evaluate failed: %v", err)
		}
		if dec.Outcome != model.SuppressionOutcomeSuppressed {
			b.Fatalf("expected suppressed outcome, got %s", dec.Outcome)
		}
	}
}

func BenchmarkOwnershipResolver_ResolveNodeOwnership_10LevelHierarchy(b *testing.B) {
	ctx := context.Background()
	store, err := storage.NewSQLiteStorage(storage.Config{Path: ":memory:"})
	if err != nil {
		b.Fatalf("failed to create storage: %v", err)
	}
	defer store.Close()

	clock := NewMockClock(time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC))
	resolver := NewOwnershipResolver(store, clock)

	// Build 10-level nested fleet group hierarchy
	parentID := ""
	for level := 1; level <= 10; level++ {
		groupID := fmt.Sprintf("grp-level-%d", level)
		group := &model.FleetGroup{
			ID:            groupID,
			OrgID:         model.DefaultOrganizationID,
			ParentGroupID: parentID,
			Name:          fmt.Sprintf("Hierarchy Level %d", level),
			Type:          model.GroupTypeEnvironment,
			Metadata: map[string]string{
				fmt.Sprintf("level_%d_tag", level): fmt.Sprintf("val_%d", level),
			},
		}
		if level == 5 {
			group.Metadata["owner_team"] = "Platform SRE"
			group.Metadata["contact_channel"] = "#platform-sre"
		}
		if err := store.SaveFleetGroup(ctx, group); err != nil {
			b.Fatalf("failed to save group: %v", err)
		}
		parentID = groupID
	}

	// Create leaf node in level 10 group
	node := &model.FleetNode{
		Identity: model.NodeIdentity{
			NodeID:   "node-deep-01",
			Hostname: "deep-01.corp",
		},
		Status:       model.NodeStatusHealthy,
		RegisteredAt: time.Now().UTC(),
		Metadata: map[string]string{
			"env": "production",
		},
	}
	if err := store.SaveFleetNode(ctx, node); err != nil {
		b.Fatalf("failed to save node: %v", err)
	}
	if err := store.AddGroupMember(ctx, &model.FleetGroupMember{
		GroupID: "grp-level-10",
		NodeID:  "node-deep-01",
		Role:    model.MembershipRolePrimary,
	}); err != nil {
		b.Fatalf("failed to add member: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		res, err := resolver.ResolveNodeOwnership(ctx, "node-deep-01")
		if err != nil {
			b.Fatalf("ResolveNodeOwnership failed: %v", err)
		}
		if res.OwnerTeam != "Platform SRE" {
			b.Fatalf("expected owner team Platform SRE, got %s", res.OwnerTeam)
		}
	}
}

func BenchmarkAuditRecorder_SanitizeAuditEvent(b *testing.B) {
	event := model.AuditEvent{
		ID:        "aud-bench-01",
		Timestamp: time.Now().UTC(),
		EventType: "governance.policy.updated",
		Severity:  "info",
		Outcome:   "success",
		Actor: model.AuditActor{
			Type:     "user",
			Identity: "alice@company.com",
		},
		Resource: "pol-prod-01",
		Action:   "update",
		Message:  "User alice updated policy using Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.e30.t-ID manually with password=SuperSecretPassword123!",
		Metadata: map[string]string{
			"api_key":      "sk-live-999888777666555444",
			"secret_token": "tok-xyz-998877",
			"region":       "us-west-2",
			"cluster":      "prod-k8s",
		},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		sanitized := SanitizeAuditEvent(event)
		if sanitized.Metadata["api_key"] != RedactedPlaceholder {
			b.Fatalf("expected redacted api_key")
		}
	}
}
