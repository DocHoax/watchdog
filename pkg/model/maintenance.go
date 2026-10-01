package model

import (
	"fmt"
	"strings"
	"time"
)

// MaintenanceStatus represents the lifecycle status of a maintenance window.
type MaintenanceStatus string

const (
	MaintenanceStatusDraft     MaintenanceStatus = "draft"
	MaintenanceStatusScheduled MaintenanceStatus = "scheduled"
	MaintenanceStatusActive    MaintenanceStatus = "active"
	MaintenanceStatusCompleted MaintenanceStatus = "completed"
	MaintenanceStatusCancelled MaintenanceStatus = "cancelled"
	MaintenanceStatusExpired   MaintenanceStatus = "expired"
)

// IsValid reports whether the maintenance status is valid.
func (s MaintenanceStatus) IsValid() bool {
	switch s {
	case MaintenanceStatusDraft,
		MaintenanceStatusScheduled,
		MaintenanceStatusActive,
		MaintenanceStatusCompleted,
		MaintenanceStatusCancelled,
		MaintenanceStatusExpired:
		return true
	default:
		return false
	}
}

// CanTransitionTo reports whether a transition from status s to target is allowed.
func (s MaintenanceStatus) CanTransitionTo(target MaintenanceStatus) bool {
	if s == target {
		return true
	}
	switch s {
	case MaintenanceStatusDraft:
		return target == MaintenanceStatusScheduled || target == MaintenanceStatusCancelled
	case MaintenanceStatusScheduled:
		return target == MaintenanceStatusActive || target == MaintenanceStatusCancelled || target == MaintenanceStatusExpired
	case MaintenanceStatusActive:
		return target == MaintenanceStatusCompleted || target == MaintenanceStatusCancelled || target == MaintenanceStatusExpired
	case MaintenanceStatusCompleted, MaintenanceStatusCancelled, MaintenanceStatusExpired:
		// Terminal states
		return false
	default:
		return false
	}
}

// MaintenanceTargetScope represents the scope of entities covered by a maintenance window.
type MaintenanceTargetScope string

const (
	MaintenanceTargetScopeOrganization MaintenanceTargetScope = "org"
	MaintenanceTargetScopeFleetGroup   MaintenanceTargetScope = "fleet_group"
	MaintenanceTargetScopeNode         MaintenanceTargetScope = "node"
	MaintenanceTargetScopeSelector     MaintenanceTargetScope = "selector"
)

// IsValid reports whether the maintenance target scope is recognized.
func (s MaintenanceTargetScope) IsValid() bool {
	switch s {
	case MaintenanceTargetScopeOrganization,
		MaintenanceTargetScopeFleetGroup,
		MaintenanceTargetScopeNode,
		MaintenanceTargetScopeSelector:
		return true
	default:
		return false
	}
}

// RecurrenceFrequency specifies the repetition interval for recurring maintenance windows.
type RecurrenceFrequency string

const (
	RecurrenceFrequencyDaily   RecurrenceFrequency = "daily"
	RecurrenceFrequencyWeekly  RecurrenceFrequency = "weekly"
	RecurrenceFrequencyMonthly RecurrenceFrequency = "monthly"
)

// IsValid reports whether the recurrence frequency is recognized.
func (f RecurrenceFrequency) IsValid() bool {
	switch f {
	case RecurrenceFrequencyDaily, RecurrenceFrequencyWeekly, RecurrenceFrequencyMonthly:
		return true
	default:
		return false
	}
}

// RecurrenceSchedule defines repetition rules for a maintenance window.
type RecurrenceSchedule struct {
	Frequency      RecurrenceFrequency `json:"frequency" yaml:"frequency"`
	Interval       int                 `json:"interval" yaml:"interval"` // e.g. every 1 week, every 2 days (must be >= 1)
	DaysOfWeek     []time.Weekday      `json:"days_of_week,omitempty" yaml:"days_of_week,omitempty"`
	DayOfMonth     int                 `json:"day_of_month,omitempty" yaml:"day_of_month,omitempty"` // 1-31 (clamped for short months)
	MaxOccurrences int                 `json:"max_occurrences,omitempty" yaml:"max_occurrences,omitempty"`
	Until          *time.Time          `json:"until,omitempty" yaml:"until,omitempty"`
	Duration       time.Duration       `json:"duration" yaml:"duration"` // duration of each recurring occurrence
}

// Validate verifies the recurrence schedule configuration.
func (r *RecurrenceSchedule) Validate() error {
	if r == nil {
		return nil
	}
	if !r.Frequency.IsValid() {
		return fmt.Errorf("invalid recurrence frequency: %s", r.Frequency)
	}
	if r.Interval < 1 {
		return fmt.Errorf("recurrence interval must be >= 1, got %d", r.Interval)
	}
	if r.Interval > 365 {
		return fmt.Errorf("recurrence interval cannot exceed 365, got %d", r.Interval)
	}
	if r.Duration <= 0 {
		return fmt.Errorf("recurrence occurrence duration must be > 0")
	}
	if r.Duration > 7*24*time.Hour {
		return fmt.Errorf("recurrence occurrence duration cannot exceed 7 days")
	}

	if r.Frequency == RecurrenceFrequencyWeekly {
		if len(r.DaysOfWeek) == 0 {
			return fmt.Errorf("weekly recurrence requires at least one day of week")
		}
		for _, d := range r.DaysOfWeek {
			if d < time.Sunday || d > time.Saturday {
				return fmt.Errorf("invalid day of week: %d", d)
			}
		}
	}

	if r.Frequency == RecurrenceFrequencyMonthly {
		if r.DayOfMonth < 1 || r.DayOfMonth > 31 {
			return fmt.Errorf("monthly recurrence day of month must be between 1 and 31, got %d", r.DayOfMonth)
		}
	}

	if r.MaxOccurrences < 0 {
		return fmt.Errorf("max_occurrences cannot be negative, got %d", r.MaxOccurrences)
	}
	if r.MaxOccurrences > 1000 {
		return fmt.Errorf("max_occurrences cannot exceed 1000, got %d", r.MaxOccurrences)
	}

	return nil
}

// MaintenanceSchedule configures the time window and recurrence rules.
type MaintenanceSchedule struct {
	StartTime  time.Time           `json:"start_time" yaml:"start_time"` // UTC normalized
	EndTime    time.Time           `json:"end_time" yaml:"end_time"`     // UTC normalized
	TimeZone   string              `json:"time_zone,omitempty" yaml:"time_zone,omitempty"` // IANA time zone e.g. "America/New_York", "UTC"
	Recurrence *RecurrenceSchedule `json:"recurrence,omitempty" yaml:"recurrence,omitempty"`
}

// Validate verifies the schedule bounds, time zone, and recurrence.
func (s *MaintenanceSchedule) Validate() error {
	if s.StartTime.IsZero() {
		return fmt.Errorf("start_time cannot be zero")
	}
	if s.EndTime.IsZero() {
		return fmt.Errorf("end_time cannot be zero")
	}
	if !s.EndTime.After(s.StartTime) {
		return fmt.Errorf("end_time (%s) must be strictly after start_time (%s)", s.EndTime.Format(time.RFC3339), s.StartTime.Format(time.RFC3339))
	}

	if s.TimeZone != "" {
		if _, err := time.LoadLocation(s.TimeZone); err != nil {
			return fmt.Errorf("invalid IANA time_zone '%s': %w", s.TimeZone, err)
		}
	}

	if s.Recurrence != nil {
		if err := s.Recurrence.Validate(); err != nil {
			return fmt.Errorf("invalid recurrence: %w", err)
		}
	}

	return nil
}

// MaintenanceWindow represents a planned or active maintenance period.
type MaintenanceWindow struct {
	ID                   string                 `json:"id" yaml:"id"`
	OrgID                string                 `json:"org_id" yaml:"org_id"`
	Name                 string                 `json:"name" yaml:"name"`
	Description          string                 `json:"description,omitempty" yaml:"description,omitempty"`
	Status               MaintenanceStatus      `json:"status" yaml:"status"`
	TargetScope          MaintenanceTargetScope `json:"target_scope" yaml:"target_scope"`
	TargetID             string                 `json:"target_id,omitempty" yaml:"target_id,omitempty"`
	TargetSelector       string                 `json:"target_selector,omitempty" yaml:"target_selector,omitempty"`
	CategoryRestrictions []PolicyCategory       `json:"category_restrictions,omitempty" yaml:"category_restrictions,omitempty"`
	SeverityThreshold    Severity               `json:"severity_threshold,omitempty" yaml:"severity_threshold,omitempty"` // Maximum severity suppressed
	Schedule             MaintenanceSchedule    `json:"schedule" yaml:"schedule"`
	SuppressAlerts       bool                   `json:"suppress_alerts" yaml:"suppress_alerts"`
	SuppressFindings     bool                   `json:"suppress_findings" yaml:"suppress_findings"`
	AllowCriticalAlerts  bool                   `json:"allow_critical_alerts" yaml:"allow_critical_alerts"` // Hard safety default true
	CreatedAt            time.Time              `json:"created_at" yaml:"created_at"`
	UpdatedAt            time.Time              `json:"updated_at" yaml:"updated_at"`
	CreatedBy            string                 `json:"created_by,omitempty" yaml:"created_by,omitempty"`
	Metadata             map[string]string      `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// Validate checks the structural integrity of the maintenance window.
func (w *MaintenanceWindow) Validate() error {
	if w == nil {
		return fmt.Errorf("nil maintenance window")
	}
	if err := ValidateIdentifier(w.ID); err != nil {
		return fmt.Errorf("invalid maintenance window id: %w", err)
	}
	if err := ValidateIdentifier(w.OrgID); err != nil {
		return fmt.Errorf("invalid org_id: %w", err)
	}
	if strings.TrimSpace(w.Name) == "" {
		return fmt.Errorf("maintenance window name cannot be empty")
	}
	if len(w.Name) > 255 {
		return fmt.Errorf("maintenance window name exceeds 255 characters")
	}
	if !w.Status.IsValid() {
		return fmt.Errorf("invalid maintenance status: %s", w.Status)
	}
	if !w.TargetScope.IsValid() {
		return fmt.Errorf("invalid target scope: %s", w.TargetScope)
	}

	switch w.TargetScope {
	case MaintenanceTargetScopeOrganization:
		if w.TargetID == "" {
			w.TargetID = w.OrgID
		} else if w.TargetID != w.OrgID {
			return fmt.Errorf("org scope target_id (%s) must match org_id (%s)", w.TargetID, w.OrgID)
		}
	case MaintenanceTargetScopeFleetGroup, MaintenanceTargetScopeNode:
		if strings.TrimSpace(w.TargetID) == "" {
			return fmt.Errorf("target_id is required for target scope %s", w.TargetScope)
		}
		if err := ValidateIdentifier(w.TargetID); err != nil {
			return fmt.Errorf("invalid target_id: %w", err)
		}
	case MaintenanceTargetScopeSelector:
		if strings.TrimSpace(w.TargetSelector) == "" {
			return fmt.Errorf("target_selector is required for selector target scope")
		}
	}

	for _, cat := range w.CategoryRestrictions {
		if !cat.IsValid() {
			return fmt.Errorf("invalid category restriction: %s", cat)
		}
	}

	if w.SeverityThreshold != "" && !w.SeverityThreshold.IsValid() {
		return fmt.Errorf("invalid severity threshold: %s", w.SeverityThreshold)
	}

	if err := w.Schedule.Validate(); err != nil {
		return fmt.Errorf("invalid schedule: %w", err)
	}

	return nil
}

// MaintenanceWindowFilter defines query parameters for listing maintenance windows.
type MaintenanceWindowFilter struct {
	OrgID          string                 `json:"org_id,omitempty"`
	TargetScope    MaintenanceTargetScope `json:"target_scope,omitempty"`
	TargetID       string                 `json:"target_id,omitempty"`
	Status         MaintenanceStatus      `json:"status,omitempty"`
	ActiveAt       *time.Time             `json:"active_at,omitempty"`
	From           *time.Time             `json:"from,omitempty"`
	To             *time.Time             `json:"to,omitempty"`
	IncludeExpired bool                   `json:"include_expired,omitempty"`
	Limit          int                    `json:"limit,omitempty"`
	Offset         int                    `json:"offset,omitempty"`
}
