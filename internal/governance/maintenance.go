package governance

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/DocHoax/watchdog/internal/storage"
	"github.com/DocHoax/watchdog/pkg/model"
)

// MaintenanceEngine evaluates maintenance window schedules, recurrence occurrences,
// target scope matching, and lifecycle transitions.
type MaintenanceEngine struct {
	store storage.ReadOnlyStorage
	clock Clock
}

// NewMaintenanceEngine constructs a new MaintenanceEngine.
func NewMaintenanceEngine(store storage.ReadOnlyStorage, clock Clock) *MaintenanceEngine {
	if clock == nil {
		clock = RealClock{}
	}
	return &MaintenanceEngine{
		store: store,
		clock: clock,
	}
}

// IsWindowActive evaluates whether a maintenance window is active at the specified timestamp t.
func (e *MaintenanceEngine) IsWindowActive(w *model.MaintenanceWindow, t time.Time) (bool, error) {
	if w == nil {
		return false, fmt.Errorf("%w: nil maintenance window", ErrInvalidInput)
	}

	// Status lifecycle guard: only Scheduled and Active windows can be active
	if w.Status != model.MaintenanceStatusScheduled && w.Status != model.MaintenanceStatusActive {
		return false, nil
	}

	loc := time.UTC
	if w.Schedule.TimeZone != "" {
		loadedLoc, err := time.LoadLocation(w.Schedule.TimeZone)
		if err != nil {
			return false, fmt.Errorf("invalid timezone %q: %w", w.Schedule.TimeZone, err)
		}
		loc = loadedLoc
	}

	tInLoc := t.In(loc)
	startInLoc := w.Schedule.StartTime.In(loc)
	endInLoc := w.Schedule.EndTime.In(loc)

	// Non-recurring window
	if w.Schedule.Recurrence == nil {
		if t.Before(w.Schedule.StartTime) || t.After(w.Schedule.EndTime) {
			return false, nil
		}
		return true, nil
	}

	// Recurring window evaluation
	rec := w.Schedule.Recurrence

	// Check overall start and until bounds
	if t.Before(w.Schedule.StartTime) {
		return false, nil
	}
	if rec.Until != nil && t.After(*rec.Until) {
		return false, nil
	}

	duration := rec.Duration
	if duration <= 0 {
		duration = w.Schedule.EndTime.Sub(w.Schedule.StartTime)
	}
	if duration <= 0 {
		duration = time.Hour
	}

	switch rec.Frequency {
	case model.RecurrenceFrequencyDaily:
		return isDailyRecurrenceActive(startInLoc, endInLoc, tInLoc, rec, duration, loc)
	case model.RecurrenceFrequencyWeekly:
		return isWeeklyRecurrenceActive(startInLoc, endInLoc, tInLoc, rec, duration, loc)
	case model.RecurrenceFrequencyMonthly:
		return isMonthlyRecurrenceActive(startInLoc, endInLoc, tInLoc, rec, duration, loc)
	default:
		return false, fmt.Errorf("unsupported recurrence frequency: %s", rec.Frequency)
	}
}

// isDailyRecurrenceActive checks if t falls within any daily recurrence occurrence.
func isDailyRecurrenceActive(start, end, t time.Time, rec *model.RecurrenceSchedule, duration time.Duration, loc *time.Location) (bool, error) {
	interval := rec.Interval
	if interval < 1 {
		interval = 1
	}

	// Start date in location
	startDate := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, loc)
	tDate := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)

	daysDiff := int(tDate.Sub(startDate).Hours() / 24)
	if daysDiff < 0 {
		return false, nil
	}

	// Check current day and previous days (in case occurrence duration crosses midnight)
	daysToCheck := int(duration.Hours()/24) + 1
	for d := 0; d <= daysToCheck; d++ {
		candidateDays := daysDiff - d
		if candidateDays < 0 {
			continue
		}
		if candidateDays%interval == 0 {
			occurrenceIdx := candidateDays / interval
			if rec.MaxOccurrences > 0 && occurrenceIdx >= rec.MaxOccurrences {
				continue
			}

			// Occurrence start time on candidate day
			candStart := time.Date(
				startDate.Year(), startDate.Month(), startDate.Day()+candidateDays,
				start.Hour(), start.Minute(), start.Second(), start.Nanosecond(),
				loc,
			)
			candEnd := candStart.Add(duration)

			if !t.Before(candStart) && !t.After(candEnd) {
				return true, nil
			}
		}
	}

	return false, nil
}

// isWeeklyRecurrenceActive checks if t falls within any weekly recurrence occurrence.
func isWeeklyRecurrenceActive(start, end, t time.Time, rec *model.RecurrenceSchedule, duration time.Duration, loc *time.Location) (bool, error) {
	interval := rec.Interval
	if interval < 1 {
		interval = 1
	}

	// Start week normalized to Sunday
	startDayOffset := int(start.Weekday())
	startWeekSunday := time.Date(start.Year(), start.Month(), start.Day()-startDayOffset, 0, 0, 0, 0, loc)

	tDayOffset := int(t.Weekday())
	tWeekSunday := time.Date(t.Year(), t.Month(), t.Day()-tDayOffset, 0, 0, 0, 0, loc)

	weeksDiff := int(tWeekSunday.Sub(startWeekSunday).Hours() / (24 * 7))
	if weeksDiff < 0 {
		return false, nil
	}

	// Check current week and previous week (if duration extends across week boundary)
	weeksToCheck := int(duration.Hours()/(24*7)) + 1
	for w := 0; w <= weeksToCheck; w++ {
		candidateWeek := weeksDiff - w
		if candidateWeek < 0 {
			continue
		}
		if candidateWeek%interval == 0 {
			weekIntervalIdx := candidateWeek / interval

			// Determine week base Sunday
			weekSunday := startWeekSunday.AddDate(0, 0, candidateWeek*7)

			for _, dayOfWeek := range rec.DaysOfWeek {
				candDay := weekSunday.AddDate(0, 0, int(dayOfWeek))
				candStart := time.Date(
					candDay.Year(), candDay.Month(), candDay.Day(),
					start.Hour(), start.Minute(), start.Second(), start.Nanosecond(),
					loc,
				)

				// Skip candidate occurrences before original start time
				if candStart.Before(start) {
					continue
				}

				if rec.MaxOccurrences > 0 {
					// Count total occurrences from start up to this one
					totalBefore := countWeeklyOccurrencesBefore(start, candStart, rec, interval, loc)
					if totalBefore >= rec.MaxOccurrences {
						continue
					}
				}

				candEnd := candStart.Add(duration)
				if !t.Before(candStart) && !t.After(candEnd) {
					return true, nil
				}
			}
			_ = weekIntervalIdx
		}
	}

	return false, nil
}

// countWeeklyOccurrencesBefore counts how many weekly occurrences occurred between start and target.
func countWeeklyOccurrencesBefore(start, target time.Time, rec *model.RecurrenceSchedule, interval int, loc *time.Location) int {
	count := 0
	startDayOffset := int(start.Weekday())
	startWeekSunday := time.Date(start.Year(), start.Month(), start.Day()-startDayOffset, 0, 0, 0, 0, loc)

	tDayOffset := int(target.Weekday())
	tWeekSunday := time.Date(target.Year(), target.Month(), target.Day()-tDayOffset, 0, 0, 0, 0, loc)

	weeksDiff := int(tWeekSunday.Sub(startWeekSunday).Hours() / (24 * 7))

	for w := 0; w <= weeksDiff; w += interval {
		weekSunday := startWeekSunday.AddDate(0, 0, w*7)
		for _, dayOfWeek := range rec.DaysOfWeek {
			candDay := weekSunday.AddDate(0, 0, int(dayOfWeek))
			candStart := time.Date(
				candDay.Year(), candDay.Month(), candDay.Day(),
				start.Hour(), start.Minute(), start.Second(), start.Nanosecond(),
				loc,
			)
			if candStart.Before(start) {
				continue
			}
			if candStart.Before(target) {
				count++
			}
		}
	}
	return count
}

// isMonthlyRecurrenceActive checks if t falls within a monthly recurrence occurrence with day-of-month clamping.
func isMonthlyRecurrenceActive(start, end, t time.Time, rec *model.RecurrenceSchedule, duration time.Duration, loc *time.Location) (bool, error) {
	interval := rec.Interval
	if interval < 1 {
		interval = 1
	}

	targetDay := rec.DayOfMonth
	if targetDay < 1 {
		targetDay = start.Day()
	}
	if targetDay > 31 {
		targetDay = 31
	}

	monthsDiff := (t.Year()-start.Year())*12 + int(t.Month()-start.Month())
	if monthsDiff < 0 {
		return false, nil
	}

	// Check current month and previous month
	monthsToCheck := int(duration.Hours()/(24*28)) + 1
	for m := 0; m <= monthsToCheck; m++ {
		candidateMonthDiff := monthsDiff - m
		if candidateMonthDiff < 0 {
			continue
		}
		if candidateMonthDiff%interval == 0 {
			occurrenceIdx := candidateMonthDiff / interval
			if rec.MaxOccurrences > 0 && occurrenceIdx >= rec.MaxOccurrences {
				continue
			}

			// Target month and year
			totalMonths := int(start.Month()) - 1 + candidateMonthDiff
			candYear := start.Year() + totalMonths/12
			candMonth := time.Month((totalMonths % 12) + 1)

			// Clamp day to max days in candMonth
			maxDays := daysInMonth(candYear, candMonth, loc)
			clampedDay := targetDay
			if clampedDay > maxDays {
				clampedDay = maxDays
			}

			candStart := time.Date(
				candYear, candMonth, clampedDay,
				start.Hour(), start.Minute(), start.Second(), start.Nanosecond(),
				loc,
			)

			if candStart.Before(start) {
				continue
			}

			candEnd := candStart.Add(duration)
			if !t.Before(candStart) && !t.After(candEnd) {
				return true, nil
			}
		}
	}

	return false, nil
}

// daysInMonth returns the number of days in the specified year and month.
func daysInMonth(year int, month time.Month, loc *time.Location) int {
	firstOfNextMonth := time.Date(year, month+1, 1, 0, 0, 0, 0, loc)
	lastOfThisMonth := firstOfNextMonth.AddDate(0, 0, -1)
	return lastOfThisMonth.Day()
}

// MatchesNode determines whether a maintenance window's target scope covers a specific node.
func (e *MaintenanceEngine) MatchesNode(
	w *model.MaintenanceWindow,
	node *model.FleetNode,
	ownership *model.NodeOwnershipMetadata,
	groupIDs []string,
	groupPaths []string,
) (bool, error) {
	if w == nil || node == nil {
		return false, fmt.Errorf("%w: nil window or node", ErrInvalidInput)
	}

	// Extract node organization
	nodeOrg := model.DefaultOrganizationID
	if node.Metadata != nil {
		if val, ok := node.Metadata["org_id"]; ok && val != "" {
			nodeOrg = val
		} else if val, ok := node.Metadata["organization_id"]; ok && val != "" {
			nodeOrg = val
		}
	}

	// Multi-tenant isolation: window org must match node org
	if w.OrgID != model.DefaultOrganizationID && nodeOrg != w.OrgID {
		return false, nil
	}

	switch w.TargetScope {
	case model.MaintenanceTargetScopeOrganization:
		return true, nil

	case model.MaintenanceTargetScopeFleetGroup:
		return slices.Contains(groupIDs, w.TargetID), nil

	case model.MaintenanceTargetScopeNode:
		return node.Identity.NodeID == w.TargetID, nil

	case model.MaintenanceTargetScopeSelector:
		if strings.TrimSpace(w.TargetSelector) == "" {
			return true, nil
		}
		return MatchesNode(w.TargetSelector, node, ownership, groupIDs, groupPaths)

	default:
		return false, fmt.Errorf("unrecognized target scope: %s", w.TargetScope)
	}
}

// GetActiveWindowsForNode finds all maintenance windows currently active and targeting a node at timestamp t.
func (e *MaintenanceEngine) GetActiveWindowsForNode(
	ctx context.Context,
	orgID string,
	node *model.FleetNode,
	ownership *model.NodeOwnershipMetadata,
	groupIDs []string,
	groupPaths []string,
	t time.Time,
) ([]model.MaintenanceWindow, error) {
	if node == nil {
		return nil, fmt.Errorf("%w: nil node", ErrInvalidInput)
	}
	if orgID == "" {
		orgID = model.DefaultOrganizationID
	}

	windows, err := e.store.ListMaintenanceWindows(ctx, model.MaintenanceWindowFilter{
		OrgID: orgID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list maintenance windows: %w", err)
	}

	var activeWindows []model.MaintenanceWindow
	for _, w := range windows {
		// 1. Target scope match
		matches, err := e.MatchesNode(&w, node, ownership, groupIDs, groupPaths)
		if err != nil {
			return nil, fmt.Errorf("failed to match window %s against node %s: %w", w.ID, node.Identity.NodeID, err)
		}
		if !matches {
			continue
		}

		// 2. Active schedule match
		active, err := e.IsWindowActive(&w, t)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate window %s schedule: %w", w.ID, err)
		}
		if active {
			activeWindows = append(activeWindows, w)
		}
	}

	return activeWindows, nil
}

// TransitionWindowStatus validates and updates the lifecycle status of a maintenance window.
func (e *MaintenanceEngine) TransitionWindowStatus(w *model.MaintenanceWindow, newStatus model.MaintenanceStatus, now time.Time) error {
	if w == nil {
		return fmt.Errorf("%w: nil window", ErrInvalidInput)
	}
	if !newStatus.IsValid() {
		return fmt.Errorf("%w: invalid status %s", ErrInvalidInput, newStatus)
	}

	if w.Status == newStatus {
		return nil
	}

	if !w.Status.CanTransitionTo(newStatus) {
		return fmt.Errorf("%w: cannot transition maintenance window from %s to %s", ErrInvalidMaintenanceTransition, w.Status, newStatus)
	}

	w.Status = newStatus
	w.UpdatedAt = now
	return nil
}

// ComputeNextOccurrences generates the next N occurrences of a maintenance window starting from reference time.
func (e *MaintenanceEngine) ComputeNextOccurrences(w *model.MaintenanceWindow, from time.Time, limit int) ([]model.MaintenanceSchedule, error) {
	if w == nil {
		return nil, fmt.Errorf("%w: nil window", ErrInvalidInput)
	}
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}

	loc := time.UTC
	if w.Schedule.TimeZone != "" {
		loadedLoc, err := time.LoadLocation(w.Schedule.TimeZone)
		if err != nil {
			return nil, fmt.Errorf("invalid timezone %q: %w", w.Schedule.TimeZone, err)
		}
		loc = loadedLoc
	}

	var occurrences []model.MaintenanceSchedule

	// Non-recurring
	if w.Schedule.Recurrence == nil {
		if !w.Schedule.EndTime.Before(from) {
			occurrences = append(occurrences, model.MaintenanceSchedule{
				StartTime: w.Schedule.StartTime,
				EndTime:   w.Schedule.EndTime,
				TimeZone:  w.Schedule.TimeZone,
			})
		}
		return occurrences, nil
	}

	rec := w.Schedule.Recurrence
	duration := rec.Duration
	if duration <= 0 {
		duration = w.Schedule.EndTime.Sub(w.Schedule.StartTime)
	}
	if duration <= 0 {
		duration = time.Hour
	}

	start := w.Schedule.StartTime.In(loc)
	interval := rec.Interval
	if interval < 1 {
		interval = 1
	}

	switch rec.Frequency {
	case model.RecurrenceFrequencyDaily:
		for i := 0; len(occurrences) < limit; i++ {
			if rec.MaxOccurrences > 0 && i >= rec.MaxOccurrences {
				break
			}
			candStart := start.AddDate(0, 0, i*interval)
			candEnd := candStart.Add(duration)
			if rec.Until != nil && candStart.After(*rec.Until) {
				break
			}
			if !candEnd.Before(from) {
				occurrences = append(occurrences, model.MaintenanceSchedule{
					StartTime: candStart.UTC(),
					EndTime:   candEnd.UTC(),
					TimeZone:  w.Schedule.TimeZone,
				})
			}
		}

	case model.RecurrenceFrequencyWeekly:
		startDayOffset := int(start.Weekday())
		startWeekSunday := time.Date(start.Year(), start.Month(), start.Day()-startDayOffset, 0, 0, 0, 0, loc)

		occCount := 0
		for wIdx := 0; len(occurrences) < limit && occCount < 1000; wIdx++ {
			weekSunday := startWeekSunday.AddDate(0, 0, wIdx*interval*7)
			for _, dayOfWeek := range rec.DaysOfWeek {
				candDay := weekSunday.AddDate(0, 0, int(dayOfWeek))
				candStart := time.Date(
					candDay.Year(), candDay.Month(), candDay.Day(),
					start.Hour(), start.Minute(), start.Second(), start.Nanosecond(),
					loc,
				)
				if candStart.Before(start) {
					continue
				}
				if rec.MaxOccurrences > 0 && occCount >= rec.MaxOccurrences {
					break
				}
				occCount++

				candEnd := candStart.Add(duration)
				if rec.Until != nil && candStart.After(*rec.Until) {
					return occurrences, nil
				}
				if !candEnd.Before(from) {
					occurrences = append(occurrences, model.MaintenanceSchedule{
						StartTime: candStart.UTC(),
						EndTime:   candEnd.UTC(),
						TimeZone:  w.Schedule.TimeZone,
					})
					if len(occurrences) >= limit {
						break
					}
				}
			}
		}

	case model.RecurrenceFrequencyMonthly:
		targetDay := rec.DayOfMonth
		if targetDay < 1 {
			targetDay = start.Day()
		}
		for i := 0; len(occurrences) < limit; i++ {
			if rec.MaxOccurrences > 0 && i >= rec.MaxOccurrences {
				break
			}
			totalMonths := int(start.Month()) - 1 + i*interval
			candYear := start.Year() + totalMonths/12
			candMonth := time.Month((totalMonths % 12) + 1)

			maxDays := daysInMonth(candYear, candMonth, loc)
			clampedDay := targetDay
			if clampedDay > maxDays {
				clampedDay = maxDays
			}

			candStart := time.Date(
				candYear, candMonth, clampedDay,
				start.Hour(), start.Minute(), start.Second(), start.Nanosecond(),
				loc,
			)
			if candStart.Before(start) {
				continue
			}

			candEnd := candStart.Add(duration)
			if rec.Until != nil && candStart.After(*rec.Until) {
				break
			}
			if !candEnd.Before(from) {
				occurrences = append(occurrences, model.MaintenanceSchedule{
					StartTime: candStart.UTC(),
					EndTime:   candEnd.UTC(),
					TimeZone:  w.Schedule.TimeZone,
				})
			}
		}
	}

	return occurrences, nil
}
