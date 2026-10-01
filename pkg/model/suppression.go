package model

import (
	"fmt"
	"time"
)

// SuppressionOutcome indicates whether an alert or notification was suppressed.
type SuppressionOutcome string

const (
	// SuppressionOutcomeSuppressed indicates the alert notification was suppressed.
	SuppressionOutcomeSuppressed SuppressionOutcome = "suppressed"
	// SuppressionOutcomeNotSuppressed indicates no active suppression rule matched.
	SuppressionOutcomeNotSuppressed SuppressionOutcome = "not_suppressed"
	// SuppressionOutcomeExempt indicates the alert is strictly exempt from suppression (e.g., critical security/integrity).
	SuppressionOutcomeExempt SuppressionOutcome = "exempt"
	// SuppressionOutcomeIndeterminate indicates evaluation could not definitively determine suppression status.
	SuppressionOutcomeIndeterminate SuppressionOutcome = "indeterminate"
)

// IsValid reports whether the suppression outcome is valid.
func (o SuppressionOutcome) IsValid() bool {
	switch o {
	case SuppressionOutcomeSuppressed,
		SuppressionOutcomeNotSuppressed,
		SuppressionOutcomeExempt,
		SuppressionOutcomeIndeterminate:
		return true
	default:
		return false
	}
}

// SuppressionReason provides a structured code explaining the suppression decision.
type SuppressionReason string

const (
	SuppressionReasonInMaintenanceWindow         SuppressionReason = "in_maintenance_window"
	SuppressionReasonOutsideMaintenanceWindow    SuppressionReason = "outside_maintenance_window"
	SuppressionReasonNoActiveWindow              SuppressionReason = "no_active_window"
	SuppressionReasonSecurityExemption           SuppressionReason = "security_exemption"
	SuppressionReasonIntegrityExemption          SuppressionReason = "integrity_exemption"
	SuppressionReasonSeverityExceedsThreshold    SuppressionReason = "severity_exceeds_threshold"
	SuppressionReasonCategoryNotCovered          SuppressionReason = "category_not_covered"
	SuppressionReasonTargetMismatch              SuppressionReason = "target_mismatch"
	SuppressionReasonWindowAlertsNotSuppressed   SuppressionReason = "window_alerts_not_suppressed"
	SuppressionReasonEvaluationError             SuppressionReason = "evaluation_error"
)

// IsValid reports whether the suppression reason is recognized.
func (r SuppressionReason) IsValid() bool {
	switch r {
	case SuppressionReasonInMaintenanceWindow,
		SuppressionReasonOutsideMaintenanceWindow,
		SuppressionReasonNoActiveWindow,
		SuppressionReasonSecurityExemption,
		SuppressionReasonIntegrityExemption,
		SuppressionReasonSeverityExceedsThreshold,
		SuppressionReasonCategoryNotCovered,
		SuppressionReasonTargetMismatch,
		SuppressionReasonWindowAlertsNotSuppressed,
		SuppressionReasonEvaluationError:
		return true
	default:
		return false
	}
}

// SuppressionDecision records the deterministic evaluation of alert suppression.
type SuppressionDecision struct {
	ID          string             `json:"id" yaml:"id"`
	OrgID       string             `json:"org_id" yaml:"org_id"`
	AlertID     string             `json:"alert_id,omitempty" yaml:"alert_id,omitempty"`
	IncidentID  string             `json:"incident_id,omitempty" yaml:"incident_id,omitempty"`
	NodeID      string             `json:"node_id,omitempty" yaml:"node_id,omitempty"`
	WindowID    string             `json:"window_id,omitempty" yaml:"window_id,omitempty"`
	RuleID      string             `json:"rule_id,omitempty" yaml:"rule_id,omitempty"`
	RuleName    string             `json:"rule_name,omitempty" yaml:"rule_name,omitempty"`
	Category    string             `json:"category,omitempty" yaml:"category,omitempty"`
	Severity    Severity           `json:"severity" yaml:"severity"`
	Outcome     SuppressionOutcome `json:"outcome" yaml:"outcome"`
	Reason      SuppressionReason  `json:"reason" yaml:"reason"`
	Message     string             `json:"message,omitempty" yaml:"message,omitempty"`
	EvaluatedAt time.Time          `json:"evaluated_at" yaml:"evaluated_at"`
	Details     map[string]string  `json:"details,omitempty" yaml:"details,omitempty"`
}

// Validate checks the structural integrity of the suppression decision.
func (d *SuppressionDecision) Validate() error {
	if d == nil {
		return fmt.Errorf("nil suppression decision")
	}
	if err := ValidateIdentifier(d.ID); err != nil {
		return fmt.Errorf("invalid suppression decision id: %w", err)
	}
	if err := ValidateIdentifier(d.OrgID); err != nil {
		return fmt.Errorf("invalid org_id: %w", err)
	}
	if !d.Outcome.IsValid() {
		return fmt.Errorf("invalid suppression outcome: %s", d.Outcome)
	}
	if !d.Reason.IsValid() {
		return fmt.Errorf("invalid suppression reason: %s", d.Reason)
	}
	if !d.Severity.IsValid() {
		return fmt.Errorf("invalid severity: %s", d.Severity)
	}
	if d.EvaluatedAt.IsZero() {
		return fmt.Errorf("evaluated_at cannot be zero")
	}
	return nil
}

// SuppressionFilter defines query parameters for retrieving suppression decisions.
type SuppressionFilter struct {
	OrgID      string             `json:"org_id,omitempty"`
	NodeID     string             `json:"node_id,omitempty"`
	WindowID   string             `json:"window_id,omitempty"`
	AlertID    string             `json:"alert_id,omitempty"`
	IncidentID string             `json:"incident_id,omitempty"`
	Outcome    SuppressionOutcome `json:"outcome,omitempty"`
	Reason     SuppressionReason  `json:"reason,omitempty"`
	Severity   Severity           `json:"severity,omitempty"`
	Category   string             `json:"category,omitempty"`
	From       *time.Time         `json:"from,omitempty"`
	To         *time.Time         `json:"to,omitempty"`
	Since      *time.Time         `json:"since,omitempty"`
	Until      *time.Time         `json:"until,omitempty"`
	Limit      int                `json:"limit,omitempty"`
	Offset     int                `json:"offset,omitempty"`
}
