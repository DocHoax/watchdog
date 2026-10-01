package governance

import (
	"context"
	"testing"
	"time"

	"github.com/DocHoax/watchdog/internal/incidents"
	"github.com/DocHoax/watchdog/internal/storage"
	"github.com/DocHoax/watchdog/pkg/model"
)

func setupTestEscalationStore(t *testing.T) storage.Storage {
	t.Helper()
	store, err := storage.NewSQLiteStorage(storage.Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("failed to initialize sqlite storage: %v", err)
	}
	return store
}

func TestEscalationEngine_MultiStageProgression(t *testing.T) {
	ctx := context.Background()
	store := setupTestEscalationStore(t)
	defer store.Close()

	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	engine := NewEscalationEngine(store, NewMockClock(now))

	// Setup a 3-stage escalation policy
	policy := &model.EscalationPolicy{
		ID:             "esc-critical",
		OrgID:          "org-acme",
		Name:           "Critical Incident Escalation",
		Enabled:        true,
		SeverityLevels: []model.Severity{model.SeverityCritical},
		Stages: []model.EscalationStage{
			{
				StageNumber:  1,
				DelayMinutes: 0, // Immediately
				Channel:      model.EscalationChannelSlack,
				Targets: []model.EscalationTarget{
					{
						TargetType:  model.EscalationTargetTeam,
						TargetID:    "team-primary-oncall",
						Name:        "Primary On-Call",
						ContactInfo: "#oncall-critical",
					},
				},
			},
			{
				StageNumber:  2,
				DelayMinutes: 15, // After 15 minutes
				Channel:      model.EscalationChannelPagerDuty,
				Targets: []model.EscalationTarget{
					{
						TargetType:  model.EscalationTargetRole,
						TargetID:    "role-lead",
						Name:        "Engineering Lead",
						ContactInfo: "pd:lead-escalation",
					},
				},
			},
			{
				StageNumber:  3,
				DelayMinutes: 30, // After 30 minutes
				Channel:      model.EscalationChannelEmail,
				Targets: []model.EscalationTarget{
					{
						TargetType:  model.EscalationTargetRole,
						TargetID:    "role-vp",
						Name:        "VP Engineering",
						ContactInfo: "vp-eng@acme.corp",
					},
				},
			},
		},
	}
	if err := store.SaveEscalationPolicy(ctx, policy); err != nil {
		t.Fatalf("failed to save escalation policy: %v", err)
	}

	incident := &incidents.Incident{
		ID:        "inc-001",
		Title:     "Payment Gateway Outage",
		Severity:  model.SeverityCritical,
		StartTime: now,
		Metadata:  map[string]string{"org_id": "org-acme"},
	}

	// 1. At T = 0m (Immediately upon creation)
	res0, err := engine.EvaluateIncident(ctx, "org-acme", incident, now)
	if err != nil {
		t.Fatalf("unexpected error at T=0: %v", err)
	}
	if res0.CurrentStage != 1 {
		t.Errorf("at T=0 expected CurrentStage=1, got %d", res0.CurrentStage)
	}
	if res0.Channel != model.EscalationChannelSlack {
		t.Errorf("at T=0 expected channel slack, got %s", res0.Channel)
	}
	if len(res0.ActiveTargets) != 1 || res0.ActiveTargets[0].TargetID != "team-primary-oncall" {
		t.Errorf("at T=0 expected primary oncall target, got %+v", res0.ActiveTargets)
	}
	if res0.NextStageNumber == nil || *res0.NextStageNumber != 2 {
		t.Errorf("at T=0 expected NextStageNumber=2, got %v", res0.NextStageNumber)
	}
	if res0.MinutesUntilNextStage == nil || *res0.MinutesUntilNextStage != 15 {
		t.Errorf("at T=0 expected MinutesUntilNextStage=15, got %v", res0.MinutesUntilNextStage)
	}

	// 2. At T = 10m (Stage 1 still active, 5m until Stage 2)
	t10 := now.Add(10 * time.Minute)
	res10, err := engine.EvaluateIncident(ctx, "org-acme", incident, t10)
	if err != nil {
		t.Fatalf("unexpected error at T=10: %v", err)
	}
	if res10.CurrentStage != 1 {
		t.Errorf("at T=10 expected CurrentStage=1, got %d", res10.CurrentStage)
	}
	if res10.MinutesUntilNextStage == nil || *res10.MinutesUntilNextStage != 5 {
		t.Errorf("at T=10 expected MinutesUntilNextStage=5, got %v", res10.MinutesUntilNextStage)
	}

	// 3. At T = 15m (Transition to Stage 2)
	t15 := now.Add(15 * time.Minute)
	res15, err := engine.EvaluateIncident(ctx, "org-acme", incident, t15)
	if err != nil {
		t.Fatalf("unexpected error at T=15: %v", err)
	}
	if res15.CurrentStage != 2 {
		t.Errorf("at T=15 expected CurrentStage=2, got %d", res15.CurrentStage)
	}
	if res15.Channel != model.EscalationChannelPagerDuty {
		t.Errorf("at T=15 expected channel pagerduty, got %s", res15.Channel)
	}
	if res15.NextStageNumber == nil || *res15.NextStageNumber != 3 {
		t.Errorf("at T=15 expected NextStageNumber=3, got %v", res15.NextStageNumber)
	}
	if res15.MinutesUntilNextStage == nil || *res15.MinutesUntilNextStage != 15 {
		t.Errorf("at T=15 expected MinutesUntilNextStage=15, got %v", res15.MinutesUntilNextStage)
	}

	// 4. At T = 35m (Final Stage 3 reached)
	t35 := now.Add(35 * time.Minute)
	res35, err := engine.EvaluateIncident(ctx, "org-acme", incident, t35)
	if err != nil {
		t.Fatalf("unexpected error at T=35: %v", err)
	}
	if res35.CurrentStage != 3 {
		t.Errorf("at T=35 expected CurrentStage=3, got %d", res35.CurrentStage)
	}
	if res35.Channel != model.EscalationChannelEmail {
		t.Errorf("at T=35 expected channel email, got %s", res35.Channel)
	}
	if res35.NextStageNumber != nil {
		t.Errorf("at final stage expected NextStageNumber=nil, got %v", *res35.NextStageNumber)
	}
	if res35.MinutesUntilNextStage != nil {
		t.Errorf("at final stage expected MinutesUntilNextStage=nil, got %v", *res35.MinutesUntilNextStage)
	}
}

func TestEscalationEngine_DelayedFirstStageAndFallback(t *testing.T) {
	ctx := context.Background()
	store := setupTestEscalationStore(t)
	defer store.Close()

	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	engine := NewEscalationEngine(store, NewMockClock(now))

	// Policy where Stage 1 has a 5-minute delay
	policy := &model.EscalationPolicy{
		ID:             "esc-warning",
		OrgID:          "org-acme",
		Name:           "Warning Escalation",
		Enabled:        true,
		SeverityLevels: []model.Severity{model.SeverityWarning},
		Stages: []model.EscalationStage{
			{
				StageNumber:  1,
				DelayMinutes: 5,
				Channel:      model.EscalationChannelEmail,
				Targets:      nil, // Empty targets, tests FallbackTarget
				FallbackTarget: &model.EscalationTarget{
					TargetType:  model.EscalationTargetRole,
					TargetID:    "role-ops-fallback",
					Name:        "Ops Fallback",
					ContactInfo: "ops-fallback@acme.corp",
				},
			},
		},
	}
	if err := store.SaveEscalationPolicy(ctx, policy); err != nil {
		t.Fatalf("failed to save escalation policy: %v", err)
	}

	incident := &incidents.Incident{
		ID:        "inc-002",
		Title:     "High Memory Warning",
		Severity:  model.SeverityWarning,
		StartTime: now,
		Metadata:  map[string]string{"org_id": "org-acme"},
	}

	// At T = 2m (Before Stage 1 triggers)
	t2 := now.Add(2 * time.Minute)
	res2, err := engine.EvaluateIncident(ctx, "org-acme", incident, t2)
	if err != nil {
		t.Fatalf("unexpected error at T=2: %v", err)
	}
	if res2.CurrentStage != 0 {
		t.Errorf("at T=2 expected CurrentStage=0 (pre-stage), got %d", res2.CurrentStage)
	}
	if len(res2.ActiveTargets) != 0 {
		t.Errorf("at T=2 expected 0 active targets, got %d", len(res2.ActiveTargets))
	}
	if res2.NextStageNumber == nil || *res2.NextStageNumber != 1 {
		t.Errorf("at T=2 expected NextStageNumber=1, got %v", res2.NextStageNumber)
	}
	if res2.MinutesUntilNextStage == nil || *res2.MinutesUntilNextStage != 3 {
		t.Errorf("at T=2 expected MinutesUntilNextStage=3, got %v", res2.MinutesUntilNextStage)
	}

	// At T = 6m (Stage 1 active, fallback target utilized)
	t6 := now.Add(6 * time.Minute)
	res6, err := engine.EvaluateIncident(ctx, "org-acme", incident, t6)
	if err != nil {
		t.Fatalf("unexpected error at T=6: %v", err)
	}
	if res6.CurrentStage != 1 {
		t.Errorf("at T=6 expected CurrentStage=1, got %d", res6.CurrentStage)
	}
	if len(res6.ActiveTargets) != 1 || res6.ActiveTargets[0].TargetID != "role-ops-fallback" {
		t.Errorf("at T=6 expected fallback target, got %+v", res6.ActiveTargets)
	}
}

func TestEscalationEngine_DisabledPolicyAndIsolation(t *testing.T) {
	ctx := context.Background()
	store := setupTestEscalationStore(t)
	defer store.Close()

	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	engine := NewEscalationEngine(store, NewMockClock(now))

	// Disabled policy
	policy := &model.EscalationPolicy{
		ID:             "esc-disabled",
		OrgID:          "org-acme",
		Name:           "Disabled Policy",
		Enabled:        false,
		SeverityLevels: []model.Severity{model.SeverityCritical},
		Stages: []model.EscalationStage{
			{
				StageNumber:  1,
				DelayMinutes: 0,
				Channel:      model.EscalationChannelEmail,
				Targets: []model.EscalationTarget{
					{TargetType: model.EscalationTargetTeam, TargetID: "team-ops", Name: "Ops", ContactInfo: "ops@acme.corp"},
				},
			},
		},
	}
	if err := store.SaveEscalationPolicy(ctx, policy); err != nil {
		t.Fatalf("failed to save policy: %v", err)
	}

	incident := &incidents.Incident{
		ID:        "inc-003",
		Severity:  model.SeverityCritical,
		StartTime: now,
	}

	// Should fail to find enabled policy
	_, err := engine.EvaluateIncident(ctx, "org-acme", incident, now)
	if err == nil {
		t.Errorf("expected error when no enabled policy exists, got nil")
	}

	// Should reject evaluation on disabled policy directly
	_, err = engine.EvaluatePolicy(ctx, policy, "inc-003", model.SeverityCritical, now, now)
	if err == nil {
		t.Errorf("expected error when evaluating disabled policy directly, got nil")
	}
}
