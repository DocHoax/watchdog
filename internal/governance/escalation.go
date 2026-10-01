package governance

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/DocHoax/watchdog/internal/incidents"
	"github.com/DocHoax/watchdog/internal/storage"
	"github.com/DocHoax/watchdog/pkg/model"
)

// EscalationEngine evaluates incident escalation policies over time, determining current notification stages,
// active recipient targets, escalation channels, and time to subsequent stages.
type EscalationEngine struct {
	store storage.ReadOnlyStorage
	clock Clock
}

// NewEscalationEngine constructs a new EscalationEngine.
func NewEscalationEngine(store storage.ReadOnlyStorage, clock Clock) *EscalationEngine {
	if clock == nil {
		clock = RealClock{}
	}
	return &EscalationEngine{
		store: store,
		clock: clock,
	}
}

// EvaluateIncident evaluates the escalation state of an active operational incident against the organization's policies.
func (e *EscalationEngine) EvaluateIncident(ctx context.Context, orgID string, incident *incidents.Incident, evalTime time.Time) (*model.EscalationEvaluationResult, error) {
	if incident == nil {
		return nil, fmt.Errorf("%w: nil incident", ErrInvalidInput)
	}

	if orgID == "" {
		if incident.Metadata != nil && incident.Metadata["org_id"] != "" {
			orgID = incident.Metadata["org_id"]
		} else {
			orgID = model.DefaultOrganizationID
		}
	}

	if evalTime.IsZero() {
		evalTime = e.clock.Now().UTC()
	}

	// Find matching policy for organization and severity
	policy, err := e.FindMatchingPolicy(ctx, orgID, incident.Severity)
	if err != nil {
		return nil, err
	}

	startTime := incident.StartTime
	if startTime.IsZero() {
		startTime = incident.CreatedAt
	}
	if startTime.IsZero() {
		startTime = evalTime
	}

	return e.EvaluatePolicy(ctx, policy, incident.ID, incident.Severity, startTime, evalTime)
}

// FindMatchingPolicy retrieves the best matching active escalation policy for an organization and severity.
func (e *EscalationEngine) FindMatchingPolicy(ctx context.Context, orgID string, severity model.Severity) (*model.EscalationPolicy, error) {
	if e.store == nil {
		return nil, fmt.Errorf("%w: nil storage backend", ErrInvalidInput)
	}
	if orgID == "" {
		orgID = model.DefaultOrganizationID
	}

	enabled := true
	policies, err := e.store.ListEscalationPolicies(ctx, model.EscalationPolicyFilter{
		OrgID:   orgID,
		Enabled: &enabled,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query escalation policies: %w", err)
	}

	if len(policies) == 0 {
		return nil, fmt.Errorf("%w: no active escalation policy for org %s", ErrEscalationPolicyNotFound, orgID)
	}

	// Filter policies covering this severity
	for _, p := range policies {
		if len(p.SeverityLevels) == 0 || slices.Contains(p.SeverityLevels, severity) {
			return &p, nil
		}
	}

	// Fallback to first enabled policy if no severity-specific one matched
	return &policies[0], nil
}

// EvaluatePolicy evaluates the escalation stage for a specific policy, incident, and elapsed duration.
func (e *EscalationEngine) EvaluatePolicy(
	ctx context.Context,
	policy *model.EscalationPolicy,
	incidentID string,
	severity model.Severity,
	incidentStartTime time.Time,
	evalTime time.Time,
) (*model.EscalationEvaluationResult, error) {
	if policy == nil {
		return nil, fmt.Errorf("%w: nil escalation policy", ErrInvalidInput)
	}
	if !policy.Enabled {
		return nil, fmt.Errorf("%w: policy %s is disabled", ErrInvalidInput, policy.ID)
	}
	if len(policy.Stages) == 0 {
		return nil, fmt.Errorf("%w: policy %s has no stages", ErrInvalidInput, policy.ID)
	}

	if evalTime.IsZero() {
		evalTime = e.clock.Now().UTC()
	}
	if incidentStartTime.IsZero() {
		incidentStartTime = evalTime
	}

	elapsedDuration := evalTime.Sub(incidentStartTime)
	if elapsedDuration < 0 {
		elapsedDuration = 0
	}
	elapsedMinutes := int(elapsedDuration.Minutes())

	return e.EvaluateByElapsedMinutes(policy, incidentID, elapsedMinutes, evalTime)
}

// EvaluateByElapsedMinutes calculates the escalation stage given an elapsed time in minutes.
func (e *EscalationEngine) EvaluateByElapsedMinutes(
	policy *model.EscalationPolicy,
	incidentID string,
	elapsedMinutes int,
	evalTime time.Time,
) (*model.EscalationEvaluationResult, error) {
	if policy == nil {
		return nil, fmt.Errorf("%w: nil escalation policy", ErrInvalidInput)
	}
	if len(policy.Stages) == 0 {
		return nil, fmt.Errorf("%w: policy %s has no stages", ErrInvalidInput, policy.ID)
	}

	// Determine active stage based on elapsedMinutes
	activeStageIdx := -1
	for i, stage := range policy.Stages {
		if elapsedMinutes >= stage.DelayMinutes {
			activeStageIdx = i
		} else {
			break
		}
	}

	res := &model.EscalationEvaluationResult{
		IncidentID:         incidentID,
		PolicyID:           policy.ID,
		PolicyName:         policy.Name,
		ElapsedTimeMinutes: elapsedMinutes,
		EvaluatedAt:        evalTime,
	}

	if activeStageIdx == -1 {
		// Before stage 1 delay threshold
		firstStage := policy.Stages[0]
		res.CurrentStage = 0
		res.Channel = firstStage.Channel
		res.ActiveTargets = nil

		nextStageNum := firstStage.StageNumber
		nextStageDelay := firstStage.DelayMinutes
		minsUntil := max(firstStage.DelayMinutes-elapsedMinutes, 0)

		res.NextStageNumber = &nextStageNum
		res.NextStageDelayMinutes = &nextStageDelay
		res.MinutesUntilNextStage = &minsUntil
		return res, nil
	}

	currentStage := policy.Stages[activeStageIdx]
	res.CurrentStage = currentStage.StageNumber
	res.Channel = currentStage.Channel
	res.ActiveTargets = currentStage.Targets

	// Fallback targets if no primary targets specified
	if len(res.ActiveTargets) == 0 && currentStage.FallbackTarget != nil {
		res.ActiveTargets = []model.EscalationTarget{*currentStage.FallbackTarget}
	}

	// Calculate next stage progression
	if activeStageIdx+1 < len(policy.Stages) {
		nextStage := policy.Stages[activeStageIdx+1]
		nextStageNum := nextStage.StageNumber
		nextStageDelay := nextStage.DelayMinutes
		minsUntil := max(nextStage.DelayMinutes-elapsedMinutes, 0)

		res.NextStageNumber = &nextStageNum
		res.NextStageDelayMinutes = &nextStageDelay
		res.MinutesUntilNextStage = &minsUntil
	} else {
		// Final stage reached
		res.NextStageNumber = nil
		res.NextStageDelayMinutes = nil
		res.MinutesUntilNextStage = nil
	}

	return res, nil
}
