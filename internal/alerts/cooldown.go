package alerts

import (
	"sync"
	"time"

	"github.com/DocHoax/watchdog/pkg/model"
	"github.com/google/uuid"
)

// AlertState maintains the temporal lifecycle for a single alert rule.
type AlertState struct {
	RuleID           string
	FirstTriggeredAt time.Time
	LastFiredAt      time.Time
	SilencedUntil    time.Time
	IsActive         bool
	CurrentEvent     *model.AlertEvent
}

// StateTracker manages active alerts, cooldown periods, and resolution state.
type StateTracker struct {
	mu     sync.RWMutex
	states map[string]*AlertState
}

// NewStateTracker creates an empty alert state tracker.
func NewStateTracker() *StateTracker {
	return &StateTracker{
		states: make(map[string]*AlertState),
	}
}

// ProcessEvaluation updates the internal state for an evaluated rule and returns any fired or resolved events.
func (st *StateTracker) ProcessEvaluation(eval EvaluatedAlert, now time.Time) (fired *model.AlertEvent, resolved *model.AlertEvent) {
	st.mu.Lock()
	defer st.mu.Unlock()

	state, exists := st.states[eval.RuleID]
	if !exists {
		state = &AlertState{
			RuleID: eval.RuleID,
		}
		st.states[eval.RuleID] = state
	}

	// Check if rule is silenced
	if !state.SilencedUntil.IsZero() && now.Before(state.SilencedUntil) {
		return nil, nil
	}

	if eval.Triggered {
		// Rule condition is currently met
		if state.FirstTriggeredAt.IsZero() {
			state.FirstTriggeredAt = now
		}

		durationMet := true
		if eval.Duration > 0 {
			durationMet = now.Sub(state.FirstTriggeredAt) >= eval.Duration
		}

		if durationMet && !state.IsActive {
			// Check cooldown from last fire time
			cooldownMet := state.LastFiredAt.IsZero() || now.Sub(state.LastFiredAt) >= eval.Cooldown

			if cooldownMet {
				event := &model.AlertEvent{
					ID:          uuid.New().String(),
					RuleID:      eval.RuleID,
					RuleName:    eval.RuleName,
					Severity:    eval.Severity,
					Message:     eval.Message,
					MetricName:  eval.MetricName,
					ActualValue: eval.ActualValue,
					Threshold:   eval.Threshold,
					FiredAt:     now,
					IsActive:    true,
				}

				state.IsActive = true
				state.LastFiredAt = now
				state.CurrentEvent = event
				fired = event
			}
		} else if state.IsActive && state.CurrentEvent != nil {
			// Update latest metric value while active
			state.CurrentEvent.ActualValue = eval.ActualValue
		}
	} else {
		// Rule condition is NOT met
		state.FirstTriggeredAt = time.Time{}

		if state.IsActive {
			// Resolve active alert
			resTime := now
			if state.CurrentEvent != nil {
				state.CurrentEvent.IsActive = false
				state.CurrentEvent.ResolvedAt = &resTime
				resolved = state.CurrentEvent
			} else {
				resolved = &model.AlertEvent{
					ID:         uuid.New().String(),
					RuleID:     eval.RuleID,
					RuleName:   eval.RuleName,
					Severity:   eval.Severity,
					Message:    eval.Message + " (Resolved)",
					MetricName: eval.MetricName,
					FiredAt:    state.LastFiredAt,
					ResolvedAt: &resTime,
					IsActive:   false,
				}
			}

			state.IsActive = false
			state.CurrentEvent = nil
		}
	}

	return fired, resolved
}

// Silence mutes alerts for a rule until the given expiration time.
func (st *StateTracker) Silence(ruleID string, until time.Time) {
	st.mu.Lock()
	defer st.mu.Unlock()

	state, exists := st.states[ruleID]
	if !exists {
		state = &AlertState{RuleID: ruleID}
		st.states[ruleID] = state
	}
	state.SilencedUntil = until
}

// GetActiveEvents returns all currently active alert events.
func (st *StateTracker) GetActiveEvents() []model.AlertEvent {
	st.mu.RLock()
	defer st.mu.RUnlock()

	var events []model.AlertEvent
	for _, state := range st.states {
		if state.IsActive && state.CurrentEvent != nil {
			events = append(events, *state.CurrentEvent)
		}
	}
	return events
}

// Reset clears all alert states.
func (st *StateTracker) Reset() {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.states = make(map[string]*AlertState)
}
