package model

import (
	"testing"
	"time"
)

func TestMaintenanceStatus_Transitions(t *testing.T) {
	tests := []struct {
		from   MaintenanceStatus
		to     MaintenanceStatus
		valid  bool
	}{
		{MaintenanceStatusDraft, MaintenanceStatusScheduled, true},
		{MaintenanceStatusDraft, MaintenanceStatusCancelled, true},
		{MaintenanceStatusDraft, MaintenanceStatusActive, false},
		{MaintenanceStatusScheduled, MaintenanceStatusActive, true},
		{MaintenanceStatusScheduled, MaintenanceStatusCancelled, true},
		{MaintenanceStatusScheduled, MaintenanceStatusExpired, true},
		{MaintenanceStatusScheduled, MaintenanceStatusDraft, false},
		{MaintenanceStatusActive, MaintenanceStatusCompleted, true},
		{MaintenanceStatusActive, MaintenanceStatusCancelled, true},
		{MaintenanceStatusActive, MaintenanceStatusExpired, true},
		{MaintenanceStatusActive, MaintenanceStatusDraft, false},
		{MaintenanceStatusCompleted, MaintenanceStatusActive, false},
		{MaintenanceStatusCancelled, MaintenanceStatusDraft, false},
		{MaintenanceStatusExpired, MaintenanceStatusScheduled, false},
	}

	for _, tt := range tests {
		if got := tt.from.CanTransitionTo(tt.to); got != tt.valid {
			t.Errorf("%s -> %s: expected %v, got %v", tt.from, tt.to, tt.valid, got)
		}
	}
}

func TestMaintenanceWindow_Validation(t *testing.T) {
	now := time.Now().UTC()
	future := now.Add(2 * time.Hour)

	validWin := &MaintenanceWindow{
		ID:          "win-001",
		OrgID:       "org-main",
		Name:        "Database Upgrade Window",
		Status:      MaintenanceStatusDraft,
		TargetScope: MaintenanceTargetScopeOrganization,
		TargetID:    "org-main",
		Schedule: MaintenanceSchedule{
			StartTime: now,
			EndTime:   future,
			TimeZone:  "America/New_York",
			Recurrence: &RecurrenceSchedule{
				Frequency:      RecurrenceFrequencyWeekly,
				Interval:       1,
				DaysOfWeek:     []time.Weekday{time.Saturday, time.Sunday},
				MaxOccurrences: 10,
				Duration:       4 * time.Hour,
			},
		},
		SuppressAlerts:      true,
		AllowCriticalAlerts: true,
		CreatedAt:           now,
		UpdatedAt:           now,
	}

	if err := validWin.Validate(); err != nil {
		t.Fatalf("expected valid window, got error: %v", err)
	}

	// Invalid Org ID
	badOrg := *validWin
	badOrg.OrgID = "invalid org id"
	if err := badOrg.Validate(); err == nil {
		t.Errorf("expected error for invalid org id")
	}

	// Invalid EndTime before StartTime
	badSchedule := *validWin
	badSchedule.Schedule.EndTime = now.Add(-1 * time.Hour)
	if err := badSchedule.Validate(); err == nil {
		t.Errorf("expected error for end_time before start_time")
	}

	// Invalid TimeZone
	badTZ := *validWin
	badTZ.Schedule.TimeZone = "Invalid/NonExistent_Zone"
	if err := badTZ.Validate(); err == nil {
		t.Errorf("expected error for invalid timezone")
	}

	// Invalid TargetScope fleet group without target_id
	badScope := *validWin
	badScope.TargetScope = MaintenanceTargetScopeFleetGroup
	badScope.TargetID = ""
	if err := badScope.Validate(); err == nil {
		t.Errorf("expected error for fleet_group scope without target_id")
	}

	// Invalid TargetScope selector without selector
	badSel := *validWin
	badSel.TargetScope = MaintenanceTargetScopeSelector
	badSel.TargetSelector = ""
	if err := badSel.Validate(); err == nil {
		t.Errorf("expected error for selector scope without target_selector")
	}
}
