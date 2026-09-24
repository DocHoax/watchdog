package alerts

import (
	"context"
	"sync"
	"time"

	"github.com/DocHoax/watchdog/internal/config"
	"github.com/DocHoax/watchdog/internal/storage"
	"github.com/DocHoax/watchdog/pkg/model"
)

// AlertCallback is a function invoked when an alert fires or resolves.
type AlertCallback func(event model.AlertEvent, isResolved bool)

// Engine orchestrates alert evaluation, cooldown state, storage persistence, and notifications.
type Engine struct {
	mu          sync.RWMutex
	cfg         *config.Config
	storage     storage.Storage
	tracker     *StateTracker
	builtinEval *BuiltinRulesEvaluator
	customEval  *CustomRuleEvaluator
	callbacks   []AlertCallback
}

// NewEngine creates a new alert engine.
func NewEngine(cfg *config.Config, store storage.Storage) *Engine {
	return &Engine{
		cfg:         cfg,
		storage:     store,
		tracker:     NewStateTracker(),
		builtinEval: &BuiltinRulesEvaluator{},
		customEval:  &CustomRuleEvaluator{},
	}
}

// RegisterRule adds a custom user-defined AlertRule.
func (e *Engine) RegisterRule(rule model.AlertRule) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.customEval.Rules = append(e.customEval.Rules, rule)
}

// AddCallback registers a listener for alert events.
func (e *Engine) AddCallback(cb AlertCallback) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.callbacks = append(e.callbacks, cb)
}

// Evaluate runs all rule evaluations against the snapshot and processes cooldowns and resolutions.
func (e *Engine) Evaluate(ctx context.Context, snapshot *model.SystemSnapshot) ([]model.AlertEvent, []model.AlertEvent, error) {
	e.mu.RLock()
	cfg := e.cfg
	e.mu.RUnlock()

	now := time.Now()
	if snapshot != nil && !snapshot.Timestamp.IsZero() {
		now = snapshot.Timestamp
	}

	// 1. Gather all evaluated alert candidates
	var evaluations []EvaluatedAlert
	evaluations = append(evaluations, e.builtinEval.Evaluate(snapshot, cfg)...)

	e.mu.RLock()
	customResults := e.customEval.Evaluate(snapshot, cfg)
	callbacks := make([]AlertCallback, len(e.callbacks))
	copy(callbacks, e.callbacks)
	e.mu.RUnlock()

	evaluations = append(evaluations, customResults...)

	// 2. Process evaluations through state tracker
	var newFired []model.AlertEvent
	var resolved []model.AlertEvent

	for _, eval := range evaluations {
		f, r := e.tracker.ProcessEvaluation(eval, now)
		if f != nil {
			newFired = append(newFired, *f)
			// Persist to storage if available
			if e.storage != nil {
				_ = e.storage.SaveAlertEvent(ctx, *f)
			}
			// Dispatch callbacks
			for _, cb := range callbacks {
				cb(*f, false)
			}
		}
		if r != nil {
			resolved = append(resolved, *r)
			// Update storage status
			if e.storage != nil {
				_ = e.storage.UpdateAlertStatus(ctx, r.ID, model.AlertStatusResolved, r.ResolvedAt)
			}
			// Dispatch callbacks
			for _, cb := range callbacks {
				cb(*r, true)
			}
		}
	}

	return newFired, resolved, nil
}

// GetActiveAlerts returns currently firing alerts.
func (e *Engine) GetActiveAlerts() []model.AlertEvent {
	return e.tracker.GetActiveEvents()
}

// Silence mutes alerts for the specified rule for a duration.
func (e *Engine) Silence(ruleID string, dur time.Duration) {
	until := time.Now().Add(dur)
	e.tracker.Silence(ruleID, until)
}

// Reset clears all tracker states.
func (e *Engine) Reset() {
	e.tracker.Reset()
}
