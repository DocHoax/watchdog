package governance

import (
	"context"
	"testing"
	"time"

	"github.com/DocHoax/watchdog/internal/storage"
	"github.com/DocHoax/watchdog/pkg/model"
)

func TestMaintenanceEngine_NonRecurring(t *testing.T) {
	engine := NewMaintenanceEngine(nil, nil)

	base := time.Date(2026, 6, 15, 14, 0, 0, 0, time.UTC)
	start := base
	end := base.Add(2 * time.Hour)

	win := &model.MaintenanceWindow{
		ID:     "win-1",
		OrgID:  model.DefaultOrganizationID,
		Status: model.MaintenanceStatusActive,
		Schedule: model.MaintenanceSchedule{
			StartTime: start,
			EndTime:   end,
		},
	}

	// Before start
	active, err := engine.IsWindowActive(win, base.Add(-10*time.Minute))
	if err != nil || active {
		t.Errorf("expected not active before start, got %v, err=%v", active, err)
	}

	// Exactly at start
	active, err = engine.IsWindowActive(win, start)
	if err != nil || !active {
		t.Errorf("expected active at start, got %v, err=%v", active, err)
	}

	// Mid window
	active, err = engine.IsWindowActive(win, base.Add(1*time.Hour))
	if err != nil || !active {
		t.Errorf("expected active mid-window, got %v, err=%v", active, err)
	}

	// Exactly at end
	active, err = engine.IsWindowActive(win, end)
	if err != nil || !active {
		t.Errorf("expected active at end, got %v, err=%v", active, err)
	}

	// After end
	active, err = engine.IsWindowActive(win, base.Add(2*time.Hour+1*time.Minute))
	if err != nil || active {
		t.Errorf("expected not active after end, got %v, err=%v", active, err)
	}
}

func TestMaintenanceEngine_StatusGuards(t *testing.T) {
	engine := NewMaintenanceEngine(nil, nil)
	now := time.Now().UTC()

	win := &model.MaintenanceWindow{
		ID:     "win-draft",
		OrgID:  model.DefaultOrganizationID,
		Status: model.MaintenanceStatusDraft,
		Schedule: model.MaintenanceSchedule{
			StartTime: now.Add(-1 * time.Hour),
			EndTime:   now.Add(1 * time.Hour),
		},
	}

	// Draft status cannot be active
	active, err := engine.IsWindowActive(win, now)
	if err != nil || active {
		t.Errorf("expected draft window to be inactive, got active=%v, err=%v", active, err)
	}

	// Cancelled cannot be active
	win.Status = model.MaintenanceStatusCancelled
	active, err = engine.IsWindowActive(win, now)
	if err != nil || active {
		t.Errorf("expected cancelled window to be inactive, got active=%v, err=%v", active, err)
	}

	// Scheduled can be active if time matches
	win.Status = model.MaintenanceStatusScheduled
	active, err = engine.IsWindowActive(win, now)
	if err != nil || !active {
		t.Errorf("expected scheduled window to be active during window, got active=%v, err=%v", active, err)
	}
}

func TestMaintenanceEngine_DailyRecurrence(t *testing.T) {
	engine := NewMaintenanceEngine(nil, nil)

	// Daily window every day at 02:00 - 04:00 UTC starting 2026-06-01
	start := time.Date(2026, 6, 1, 2, 0, 0, 0, time.UTC)
	end := time.Date(2026, 6, 1, 4, 0, 0, 0, time.UTC)

	win := &model.MaintenanceWindow{
		ID:     "win-daily",
		OrgID:  model.DefaultOrganizationID,
		Status: model.MaintenanceStatusActive,
		Schedule: model.MaintenanceSchedule{
			StartTime: start,
			EndTime:   end,
			Recurrence: &model.RecurrenceSchedule{
				Frequency: model.RecurrenceFrequencyDaily,
				Interval:  2, // Every 2 days: June 1, June 3, June 5...
				Duration:  2 * time.Hour,
			},
		},
	}

	// June 1 at 03:00 (Day 0) -> Active
	active, err := engine.IsWindowActive(win, time.Date(2026, 6, 1, 3, 0, 0, 0, time.UTC))
	if err != nil || !active {
		t.Errorf("expected active on Day 0, got %v, err=%v", active, err)
	}

	// June 2 at 03:00 (Day 1, interval=2) -> Inactive
	active, err = engine.IsWindowActive(win, time.Date(2026, 6, 2, 3, 0, 0, 0, time.UTC))
	if err != nil || active {
		t.Errorf("expected inactive on Day 1 with interval=2, got %v, err=%v", active, err)
	}

	// June 3 at 03:00 (Day 2) -> Active
	active, err = engine.IsWindowActive(win, time.Date(2026, 6, 3, 3, 0, 0, 0, time.UTC))
	if err != nil || !active {
		t.Errorf("expected active on Day 2, got %v, err=%v", active, err)
	}

	// MaxOccurrences check: max 2 occurrences (June 1 and June 3 only)
	win.Schedule.Recurrence.MaxOccurrences = 2
	active, err = engine.IsWindowActive(win, time.Date(2026, 6, 5, 3, 0, 0, 0, time.UTC))
	if err != nil || active {
		t.Errorf("expected inactive on Day 4 due to MaxOccurrences=2, got %v, err=%v", active, err)
	}
}

func TestMaintenanceEngine_WeeklyRecurrence(t *testing.T) {
	engine := NewMaintenanceEngine(nil, nil)

	// Weekly on Tue and Thu from 22:00 to 02:00 next day (4 hours)
	// 2026-06-02 is Tuesday
	start := time.Date(2026, 6, 2, 22, 0, 0, 0, time.UTC)
	end := time.Date(2026, 6, 3, 2, 0, 0, 0, time.UTC)

	win := &model.MaintenanceWindow{
		ID:     "win-weekly",
		OrgID:  model.DefaultOrganizationID,
		Status: model.MaintenanceStatusActive,
		Schedule: model.MaintenanceSchedule{
			StartTime: start,
			EndTime:   end,
			Recurrence: &model.RecurrenceSchedule{
				Frequency:  model.RecurrenceFrequencyWeekly,
				Interval:   1,
				DaysOfWeek: []time.Weekday{time.Tuesday, time.Thursday},
				Duration:   4 * time.Hour,
			},
		},
	}

	// Tuesday 23:00 UTC -> Active
	active, err := engine.IsWindowActive(win, time.Date(2026, 6, 2, 23, 0, 0, 0, time.UTC))
	if err != nil || !active {
		t.Errorf("expected active Tuesday 23:00, got %v, err=%v", active, err)
	}

	// Wednesday 01:00 UTC (Tuesday's overnight occurrence) -> Active
	active, err = engine.IsWindowActive(win, time.Date(2026, 6, 3, 1, 0, 0, 0, time.UTC))
	if err != nil || !active {
		t.Errorf("expected active Wednesday 01:00 (crossing midnight), got %v, err=%v", active, err)
	}

	// Wednesday 12:00 UTC -> Inactive
	active, err = engine.IsWindowActive(win, time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC))
	if err != nil || active {
		t.Errorf("expected inactive Wednesday noon, got %v, err=%v", active, err)
	}

	// Thursday 22:30 UTC -> Active
	active, err = engine.IsWindowActive(win, time.Date(2026, 6, 4, 22, 30, 0, 0, time.UTC))
	if err != nil || !active {
		t.Errorf("expected active Thursday 22:30, got %v, err=%v", active, err)
	}
}

func TestMaintenanceEngine_MonthlyRecurrence_Clamping(t *testing.T) {
	engine := NewMaintenanceEngine(nil, nil)

	// Monthly on the 31st of the month at 04:00 for 2 hours
	// Start on 2026-01-31 (January has 31 days)
	start := time.Date(2026, 1, 31, 4, 0, 0, 0, time.UTC)
	end := time.Date(2026, 1, 31, 6, 0, 0, 0, time.UTC)

	win := &model.MaintenanceWindow{
		ID:     "win-monthly",
		OrgID:  model.DefaultOrganizationID,
		Status: model.MaintenanceStatusActive,
		Schedule: model.MaintenanceSchedule{
			StartTime: start,
			EndTime:   end,
			Recurrence: &model.RecurrenceSchedule{
				Frequency:  model.RecurrenceFrequencyMonthly,
				Interval:   1,
				DayOfMonth: 31,
				Duration:   2 * time.Hour,
			},
		},
	}

	// Jan 31 at 05:00 -> Active
	active, err := engine.IsWindowActive(win, time.Date(2026, 1, 31, 5, 0, 0, 0, time.UTC))
	if err != nil || !active {
		t.Errorf("expected active Jan 31, got %v, err=%v", active, err)
	}

	// Feb 2026 has 28 days -> Clamped to Feb 28 at 05:00 -> Active
	active, err = engine.IsWindowActive(win, time.Date(2026, 2, 28, 5, 0, 0, 0, time.UTC))
	if err != nil || !active {
		t.Errorf("expected active Feb 28 (clamped from 31), got %v, err=%v", active, err)
	}

	// Feb 27 -> Inactive
	active, err = engine.IsWindowActive(win, time.Date(2026, 2, 27, 5, 0, 0, 0, time.UTC))
	if err != nil || active {
		t.Errorf("expected inactive Feb 27, got %v, err=%v", active, err)
	}

	// Apr 2026 has 30 days -> Clamped to Apr 30 at 05:00 -> Active
	active, err = engine.IsWindowActive(win, time.Date(2026, 4, 30, 5, 0, 0, 0, time.UTC))
	if err != nil || !active {
		t.Errorf("expected active Apr 30 (clamped from 31), got %v, err=%v", active, err)
	}
}

func TestMaintenanceEngine_TimezoneHandling(t *testing.T) {
	engine := NewMaintenanceEngine(nil, nil)

	// Maintenance window defined in America/New_York (UTC-4 in Daylight Saving Time)
	// Window: 22:00 EDT to 02:00 EDT next day
	// In UTC: 2026-07-01 22:00 EDT = 2026-07-02 02:00 UTC to 2026-07-02 06:00 UTC
	nyLoc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("failed to load timezone: %v", err)
	}

	startNY := time.Date(2026, 7, 1, 22, 0, 0, 0, nyLoc)
	endNY := time.Date(2026, 7, 2, 2, 0, 0, 0, nyLoc)

	win := &model.MaintenanceWindow{
		ID:     "win-tz",
		OrgID:  model.DefaultOrganizationID,
		Status: model.MaintenanceStatusActive,
		Schedule: model.MaintenanceSchedule{
			StartTime: startNY.UTC(),
			EndTime:   endNY.UTC(),
			TimeZone:  "America/New_York",
		},
	}

	// Test with UTC time 2026-07-02 03:00 UTC (which is 23:00 EDT on July 1) -> Active
	active, err := engine.IsWindowActive(win, time.Date(2026, 7, 2, 3, 0, 0, 0, time.UTC))
	if err != nil || !active {
		t.Errorf("expected active across timezone boundary, got %v, err=%v", active, err)
	}

	// Invalid timezone returns error
	win.Schedule.TimeZone = "Invalid/NonExistent_Zone"
	_, err = engine.IsWindowActive(win, time.Now())
	if err == nil {
		t.Errorf("expected error for invalid timezone, got nil")
	}
}

func TestMaintenanceEngine_MatchesNode(t *testing.T) {
	engine := NewMaintenanceEngine(nil, nil)

	node := &model.FleetNode{
		Identity: model.NodeIdentity{
			NodeID:   "node-prod-001",
			Hostname: "db-primary.prod.acme.com",
			OS:       "linux",
			Platform: "ubuntu",
			Arch:     "amd64",
			Tags: map[string]string{
				"env":  "production",
				"tier": "database",
			},
		},
		Metadata: map[string]string{
			"org_id": "org-acme",
		},
	}

	ownership := &model.NodeOwnershipMetadata{
		OwnerTeam:           "db-team",
		ContactEmail:        "db-team@acme.com",
		Environment:         "production",
		BusinessCriticality: model.CriticalityMissionCritical,
	}

	groupIDs := []string{"group-db", "group-all-prod"}
	groupPaths := []string{"/infrastructure/databases", "/environments/production"}

	// 1. Organization Scope - Matching Org
	winOrg := &model.MaintenanceWindow{
		OrgID:       "org-acme",
		TargetScope: model.MaintenanceTargetScopeOrganization,
	}
	matched, err := engine.MatchesNode(winOrg, node, ownership, groupIDs, groupPaths)
	if err != nil || !matched {
		t.Errorf("expected org scope to match, got %v, err=%v", matched, err)
	}

	// 2. Organization Scope - Cross-Org Mismatch
	winCrossOrg := &model.MaintenanceWindow{
		OrgID:       "org-other",
		TargetScope: model.MaintenanceTargetScopeOrganization,
	}
	matched, err = engine.MatchesNode(winCrossOrg, node, ownership, groupIDs, groupPaths)
	if err != nil || matched {
		t.Errorf("expected cross-org window to NOT match node, got %v, err=%v", matched, err)
	}

	// 3. Fleet Group Scope
	winGroup := &model.MaintenanceWindow{
		OrgID:       "org-acme",
		TargetScope: model.MaintenanceTargetScopeFleetGroup,
		TargetID:    "group-db",
	}
	matched, err = engine.MatchesNode(winGroup, node, ownership, groupIDs, groupPaths)
	if err != nil || !matched {
		t.Errorf("expected group-db to match node, got %v, err=%v", matched, err)
	}

	winOtherGroup := &model.MaintenanceWindow{
		OrgID:       "org-acme",
		TargetScope: model.MaintenanceTargetScopeFleetGroup,
		TargetID:    "group-cache",
	}
	matched, err = engine.MatchesNode(winOtherGroup, node, ownership, groupIDs, groupPaths)
	if err != nil || matched {
		t.Errorf("expected group-cache to NOT match node, got %v, err=%v", matched, err)
	}

	// 4. Node Scope
	winNode := &model.MaintenanceWindow{
		OrgID:       "org-acme",
		TargetScope: model.MaintenanceTargetScopeNode,
		TargetID:    "node-prod-001",
	}
	matched, err = engine.MatchesNode(winNode, node, ownership, groupIDs, groupPaths)
	if err != nil || !matched {
		t.Errorf("expected node-prod-001 to match node, got %v, err=%v", matched, err)
	}

	// 5. Selector Scope (AST match)
	winSelector := &model.MaintenanceWindow{
		OrgID:          "org-acme",
		TargetScope:    model.MaintenanceTargetScopeSelector,
		TargetSelector: `tag.env == "production" && tag.tier == "database"`,
	}
	matched, err = engine.MatchesNode(winSelector, node, ownership, groupIDs, groupPaths)
	if err != nil || !matched {
		t.Errorf("expected selector match for database in production, got %v, err=%v", matched, err)
	}

	winSelectorNoMatch := &model.MaintenanceWindow{
		OrgID:          "org-acme",
		TargetScope:    model.MaintenanceTargetScopeSelector,
		TargetSelector: `tag.env == "staging"`,
	}
	matched, err = engine.MatchesNode(winSelectorNoMatch, node, ownership, groupIDs, groupPaths)
	if err != nil || matched {
		t.Errorf("expected selector non-match for staging, got %v, err=%v", matched, err)
	}
}

func TestMaintenanceEngine_TransitionWindowStatus(t *testing.T) {
	engine := NewMaintenanceEngine(nil, nil)
	now := time.Now().UTC()

	win := &model.MaintenanceWindow{
		ID:        "win-lifecycle",
		Status:    model.MaintenanceStatusDraft,
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Draft -> Scheduled: valid
	if err := engine.TransitionWindowStatus(win, model.MaintenanceStatusScheduled, now); err != nil {
		t.Errorf("expected valid transition Draft -> Scheduled, got %v", err)
	}
	if win.Status != model.MaintenanceStatusScheduled {
		t.Errorf("expected status Scheduled, got %s", win.Status)
	}

	// Scheduled -> Active: valid
	if err := engine.TransitionWindowStatus(win, model.MaintenanceStatusActive, now); err != nil {
		t.Errorf("expected valid transition Scheduled -> Active, got %v", err)
	}

	// Active -> Completed: valid
	if err := engine.TransitionWindowStatus(win, model.MaintenanceStatusCompleted, now); err != nil {
		t.Errorf("expected valid transition Active -> Completed, got %v", err)
	}

	// Completed -> Active: invalid (terminal state)
	if err := engine.TransitionWindowStatus(win, model.MaintenanceStatusActive, now); err == nil {
		t.Errorf("expected error for invalid transition Completed -> Active, got nil")
	}
}

func TestMaintenanceEngine_ComputeNextOccurrences(t *testing.T) {
	engine := NewMaintenanceEngine(nil, nil)

	start := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)
	end := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)

	win := &model.MaintenanceWindow{
		ID:     "win-compute",
		OrgID:  model.DefaultOrganizationID,
		Status: model.MaintenanceStatusActive,
		Schedule: model.MaintenanceSchedule{
			StartTime: start,
			EndTime:   end,
			Recurrence: &model.RecurrenceSchedule{
				Frequency: model.RecurrenceFrequencyDaily,
				Interval:  1,
				Duration:  2 * time.Hour,
			},
		},
	}

	from := time.Date(2026, 6, 5, 0, 0, 0, 0, time.UTC)
	schedules, err := engine.ComputeNextOccurrences(win, from, 5)
	if err != nil {
		t.Fatalf("failed to compute next occurrences: %v", err)
	}
	if len(schedules) != 5 {
		t.Fatalf("expected 5 occurrences, got %d", len(schedules))
	}
	if schedules[0].StartTime != time.Date(2026, 6, 5, 10, 0, 0, 0, time.UTC) {
		t.Errorf("unexpected first occurrence start time: %v", schedules[0].StartTime)
	}
}

func TestMaintenanceEngine_GetActiveWindowsForNode(t *testing.T) {
	store, err := storage.NewSQLiteStorage(storage.Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)

	win1 := &model.MaintenanceWindow{
		ID:          "win-active-node",
		OrgID:       "org-acme",
		Name:        "Node Patching",
		Status:      model.MaintenanceStatusActive,
		TargetScope: model.MaintenanceTargetScopeNode,
		TargetID:    "node-001",
		Schedule: model.MaintenanceSchedule{
			StartTime: now.Add(-1 * time.Hour),
			EndTime:   now.Add(1 * time.Hour),
		},
		SuppressAlerts: true,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	win2 := &model.MaintenanceWindow{
		ID:          "win-other-group",
		OrgID:       "org-acme",
		Name:        "Web Fleet Update",
		Status:      model.MaintenanceStatusActive,
		TargetScope: model.MaintenanceTargetScopeFleetGroup,
		TargetID:    "group-web",
		Schedule: model.MaintenanceSchedule{
			StartTime: now.Add(-1 * time.Hour),
			EndTime:   now.Add(1 * time.Hour),
		},
		SuppressAlerts: true,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := store.SaveMaintenanceWindow(ctx, win1); err != nil {
		t.Fatalf("failed to save win1: %v", err)
	}
	if err := store.SaveMaintenanceWindow(ctx, win2); err != nil {
		t.Fatalf("failed to save win2: %v", err)
	}

	engine := NewMaintenanceEngine(store, nil)

	node := &model.FleetNode{
		Identity: model.NodeIdentity{
			NodeID:   "node-001",
			Hostname: "db.acme.corp",
		},
		Metadata: map[string]string{
			"org_id": "org-acme",
		},
	}

	activeWins, err := engine.GetActiveWindowsForNode(ctx, "org-acme", node, nil, []string{"group-db"}, nil, now)
	if err != nil {
		t.Fatalf("failed to get active windows for node: %v", err)
	}

	if len(activeWins) != 1 || activeWins[0].ID != "win-active-node" {
		t.Fatalf("expected 1 active window 'win-active-node', got %+v", activeWins)
	}
}
