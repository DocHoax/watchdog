package governance

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/DocHoax/watchdog/internal/storage"
	"github.com/DocHoax/watchdog/pkg/model"
)

// FindingReconciler coordinates findings lifecycle transitions based on evaluation execution results.
type FindingReconciler struct {
	store storage.Storage
	clock Clock
}

// NewFindingReconciler creates a new FindingReconciler instance.
func NewFindingReconciler(store storage.Storage, clock Clock) *FindingReconciler {
	if clock == nil {
		clock = RealClock{}
	}
	return &FindingReconciler{
		store: store,
		clock: clock,
	}
}

// ReconcileExecution processes an EvaluationExecution against existing persisted findings and saves updates.
func (r *FindingReconciler) ReconcileExecution(
	ctx context.Context,
	exec *model.EvaluationExecution,
) ([]model.ComplianceFinding, error) {
	if exec == nil {
		return nil, fmt.Errorf("%w: nil evaluation execution", ErrInvalidInput)
	}

	// Fetch existing findings for target node
	existing, err := r.store.ListComplianceFindings(ctx, model.FindingFilter{
		OrgID:        exec.OrgID,
		TargetNodeID: exec.TargetNodeID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list existing findings for node %s: %w", exec.TargetNodeID, err)
	}

	reconciled := ReconcileResults(
		exec.OrgID,
		exec.TargetNodeID,
		exec.Results,
		existing,
		exec.EvaluatedAt,
	)

	if len(reconciled) > 0 && r.store != nil {
		if err := r.store.SaveComplianceFindings(ctx, reconciled); err != nil {
			return nil, fmt.Errorf("failed to save reconciled findings: %w", err)
		}
	}

	return reconciled, nil
}

// ReconcileResults performs pure in-memory state reconciliation between rule evaluation results and existing findings.
func ReconcileResults(
	orgID string,
	targetNodeID string,
	results []model.EvaluationResult,
	existingFindings []model.ComplianceFinding,
	evaluatedAt time.Time,
) []model.ComplianceFinding {
	if orgID == "" {
		orgID = model.DefaultOrganizationID
	}
	if evaluatedAt.IsZero() {
		evaluatedAt = time.Now().UTC()
	}

	findingMap := make(map[string]model.ComplianceFinding, len(existingFindings))
	for _, f := range existingFindings {
		findingMap[f.ID] = f
	}

	updatedMap := make(map[string]model.ComplianceFinding)

	for _, res := range results {
		if strings.TrimSpace(res.PolicyID) == "" || strings.TrimSpace(res.RuleID) == "" {
			continue
		}

		findingID := model.ComputeFindingID(orgID, targetNodeID, res.PolicyID, res.RuleID)
		existing, exists := findingMap[findingID]

		switch res.Status {
		case model.EvaluationStatusNonCompliant, model.EvaluationStatusWarning:
			if !exists {
				// 1. New finding (Open)
				fnd := model.ComplianceFinding{
					ID:              findingID,
					OrgID:           orgID,
					TargetNodeID:    targetNodeID,
					PolicyID:        res.PolicyID,
					PolicyRevision:  res.PolicyRevision,
					RuleID:          res.RuleID,
					RuleName:        res.RuleName,
					Category:        res.Category,
					Severity:        res.Severity,
					EnforcementMode: res.EnforcementMode,
					Status:          model.FindingStatusOpen,
					FirstSeenAt:     evaluatedAt,
					LastSeenAt:      evaluatedAt,
					OccurrenceCount: 1,
					Message:         res.Message,
					ObservedValue:   res.ObservedValue,
					ExpectedValue:   res.ExpectedValue,
					ContextData:     make(map[string]string),
				}
				updatedMap[findingID] = fnd
				findingMap[findingID] = fnd
			} else {
				// 2. Existing finding: update lifecycle status
				existing.PolicyRevision = res.PolicyRevision
				existing.RuleName = res.RuleName
				existing.Category = res.Category
				existing.Severity = res.Severity
				existing.EnforcementMode = res.EnforcementMode
				existing.LastSeenAt = evaluatedAt
				existing.OccurrenceCount++
				existing.Message = res.Message
				existing.ObservedValue = res.ObservedValue
				existing.ExpectedValue = res.ExpectedValue

				if existing.Status == model.FindingStatusResolved {
					// Reopen resolved finding
					existing.Status = model.FindingStatusOpen
					existing.ResolvedAt = nil
				} else if existing.Status == model.FindingStatusOpen {
					// Advance to recurring
					existing.Status = model.FindingStatusRecurring
				} else if existing.Status == model.FindingStatusIndeterminate {
					existing.Status = model.FindingStatusRecurring
				}

				updatedMap[findingID] = existing
				findingMap[findingID] = existing
			}

		case model.EvaluationStatusCompliant:
			if exists && (existing.Status == model.FindingStatusOpen || existing.Status == model.FindingStatusRecurring || existing.Status == model.FindingStatusIndeterminate) {
				// 3. Auto-resolve open/recurring finding
				resolvedTime := evaluatedAt
				existing.Status = model.FindingStatusResolved
				existing.ResolvedAt = &resolvedTime
				existing.LastSeenAt = evaluatedAt
				existing.ObservedValue = res.ObservedValue
				existing.ExpectedValue = res.ExpectedValue
				existing.Message = fmt.Sprintf("resolved: %s", res.Message)

				updatedMap[findingID] = existing
				findingMap[findingID] = existing
			}

		case model.EvaluationStatusInsufficientData, model.EvaluationStatusError:
			if exists && (existing.Status == model.FindingStatusOpen || existing.Status == model.FindingStatusRecurring) {
				// 4. Signal degraded: transition to indeterminate
				existing.Status = model.FindingStatusIndeterminate
				existing.LastSeenAt = evaluatedAt
				existing.Message = fmt.Sprintf("signal indeterminate: %s", res.Message)

				updatedMap[findingID] = existing
				findingMap[findingID] = existing
			}
		}
	}

	result := make([]model.ComplianceFinding, 0, len(updatedMap))
	for _, f := range updatedMap {
		result = append(result, f)
	}

	return result
}
