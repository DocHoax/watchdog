package model

import (
	"fmt"
	"strings"
	"time"
)

// EvaluationStatus represents the result outcome of evaluating a policy rule or overall execution.
type EvaluationStatus string

const (
	EvaluationStatusCompliant        EvaluationStatus = "compliant"
	EvaluationStatusNonCompliant     EvaluationStatus = "non_compliant"
	EvaluationStatusWarning          EvaluationStatus = "warning"
	EvaluationStatusNotApplicable    EvaluationStatus = "not_applicable"
	EvaluationStatusInsufficientData EvaluationStatus = "insufficient_data"
	EvaluationStatusError            EvaluationStatus = "evaluation_error"
)

// IsValid reports whether the evaluation status is recognized.
func (s EvaluationStatus) IsValid() bool {
	switch s {
	case EvaluationStatusCompliant,
		EvaluationStatusNonCompliant,
		EvaluationStatusWarning,
		EvaluationStatusNotApplicable,
		EvaluationStatusInsufficientData,
		EvaluationStatusError:
		return true
	default:
		return false
	}
}

// EvaluationTriggerType specifies what triggered the evaluation.
type EvaluationTriggerType string

const (
	EvaluationTriggerScheduled  EvaluationTriggerType = "scheduled"
	EvaluationTriggerOnDemand   EvaluationTriggerType = "on_demand"
	EvaluationTriggerEvent      EvaluationTriggerType = "event"
	EvaluationTriggerSimulation EvaluationTriggerType = "simulation"
)

// IsValid reports whether the trigger type is recognized.
func (t EvaluationTriggerType) IsValid() bool {
	switch t {
	case EvaluationTriggerScheduled,
		EvaluationTriggerOnDemand,
		EvaluationTriggerEvent,
		EvaluationTriggerSimulation:
		return true
	default:
		return false
	}
}

// DataFreshnessStatus indicates the freshness and validity of telemetry inputs during evaluation.
type DataFreshnessStatus string

const (
	DataFreshnessFresh       DataFreshnessStatus = "fresh"
	DataFreshnessStale       DataFreshnessStatus = "stale"
	DataFreshnessMissing     DataFreshnessStatus = "missing"
	DataFreshnessInvalid     DataFreshnessStatus = "invalid"
	DataFreshnessUnsupported DataFreshnessStatus = "unsupported"
)

// IsValid reports whether the data freshness status is recognized.
func (f DataFreshnessStatus) IsValid() bool {
	switch f {
	case DataFreshnessFresh,
		DataFreshnessStale,
		DataFreshnessMissing,
		DataFreshnessInvalid,
		DataFreshnessUnsupported:
		return true
	default:
		return false
	}
}

// EvaluationResult details the evaluation outcome for a single policy rule against a target node.
type EvaluationResult struct {
	RuleID          string              `json:"rule_id" yaml:"rule_id"`
	RuleName        string              `json:"rule_name,omitempty" yaml:"rule_name,omitempty"`
	PolicyID        string              `json:"policy_id" yaml:"policy_id"`
	PolicyRevision  int                 `json:"policy_revision" yaml:"policy_revision"`
	Category        PolicyCategory      `json:"category" yaml:"category"`
	RuleType        PolicyRuleType      `json:"rule_type" yaml:"rule_type"`
	Status          EvaluationStatus    `json:"status" yaml:"status"`
	Severity        Severity            `json:"severity" yaml:"severity"`
	EnforcementMode EnforcementMode     `json:"enforcement_mode" yaml:"enforcement_mode"`
	ObservedValue   string              `json:"observed_value,omitempty" yaml:"observed_value,omitempty"`
	ExpectedValue   string              `json:"expected_value,omitempty" yaml:"expected_value,omitempty"`
	Message         string              `json:"message" yaml:"message"`
	DataFreshness   DataFreshnessStatus `json:"data_freshness" yaml:"data_freshness"`
	EvaluatedAt     time.Time           `json:"evaluated_at" yaml:"evaluated_at"`
	Details         map[string]string   `json:"details,omitempty" yaml:"details,omitempty"`
}

// Validate checks the structural soundness of an evaluation result.
func (r *EvaluationResult) Validate() error {
	if r == nil {
		return fmt.Errorf("nil evaluation result")
	}
	if strings.TrimSpace(r.RuleID) == "" {
		return fmt.Errorf("rule_id cannot be empty")
	}
	if strings.TrimSpace(r.PolicyID) == "" {
		return fmt.Errorf("policy_id cannot be empty")
	}
	if !r.Status.IsValid() {
		return fmt.Errorf("invalid evaluation status: %s", r.Status)
	}
	if !r.Category.IsValid() {
		return fmt.Errorf("invalid policy category: %s", r.Category)
	}
	if !r.RuleType.IsValid() {
		return fmt.Errorf("invalid rule type: %s", r.RuleType)
	}
	if r.DataFreshness != "" && !r.DataFreshness.IsValid() {
		return fmt.Errorf("invalid data freshness status: %s", r.DataFreshness)
	}
	return nil
}

// EvaluationSummary aggregates rule counts and computes overall compliance and coverage ratios.
type EvaluationSummary struct {
	TotalRules            int     `json:"total_rules"`
	CompliantRules        int     `json:"compliant_rules"`
	NonCompliantRules     int     `json:"non_compliant_rules"`
	WarningRules          int     `json:"warning_rules"`
	NotApplicableRules    int     `json:"not_applicable_rules"`
	InsufficientDataRules int     `json:"insufficient_data_rules"`
	ErrorRules            int     `json:"error_rules"`
	ComplianceRatio       float64 `json:"compliance_ratio"` // [0.0 .. 1.0]
	CoverageRatio         float64 `json:"coverage_ratio"`   // [0.0 .. 1.0]
}

// CalculateSummary aggregates a list of EvaluationResult items into an EvaluationSummary.
func CalculateSummary(results []EvaluationResult) EvaluationSummary {
	var s EvaluationSummary
	s.TotalRules = len(results)
	for _, r := range results {
		switch r.Status {
		case EvaluationStatusCompliant:
			s.CompliantRules++
		case EvaluationStatusNonCompliant:
			s.NonCompliantRules++
		case EvaluationStatusWarning:
			s.WarningRules++
		case EvaluationStatusNotApplicable:
			s.NotApplicableRules++
		case EvaluationStatusInsufficientData:
			s.InsufficientDataRules++
		case EvaluationStatusError:
			s.ErrorRules++
		}
	}

	evaluatedCount := s.CompliantRules + s.NonCompliantRules + s.WarningRules
	if evaluatedCount > 0 {
		s.ComplianceRatio = float64(s.CompliantRules) / float64(evaluatedCount)
	} else {
		s.ComplianceRatio = 1.0
	}

	applicableCount := s.TotalRules - s.NotApplicableRules
	if applicableCount > 0 {
		s.CoverageRatio = float64(evaluatedCount) / float64(applicableCount)
	} else {
		s.CoverageRatio = 1.0
	}

	return s
}

// EvaluationExecution records an entire evaluation run for a target node.
type EvaluationExecution struct {
	ID           string                `json:"id" yaml:"id"`
	OrgID        string                `json:"org_id" yaml:"org_id"`
	TargetNodeID string                `json:"target_node_id" yaml:"target_node_id"`
	TriggerType  EvaluationTriggerType `json:"trigger_type" yaml:"trigger_type"`
	EvaluatedAt  time.Time             `json:"evaluated_at" yaml:"evaluated_at"`
	DurationNs   int64                 `json:"duration_ns" yaml:"duration_ns"`
	Status       EvaluationStatus      `json:"status" yaml:"status"`
	Results      []EvaluationResult    `json:"results" yaml:"results"`
	Summary      EvaluationSummary     `json:"summary" yaml:"summary"`
	Metadata     map[string]string     `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// Validate checks the structural soundness of an evaluation execution.
func (e *EvaluationExecution) Validate() error {
	if e == nil {
		return fmt.Errorf("nil evaluation execution")
	}
	if err := ValidateIdentifier(e.ID); err != nil {
		return fmt.Errorf("invalid evaluation execution id: %w", err)
	}
	if e.OrgID == "" {
		e.OrgID = DefaultOrganizationID
	} else if err := ValidateIdentifier(e.OrgID); err != nil {
		return fmt.Errorf("invalid org_id: %w", err)
	}
	if strings.TrimSpace(e.TargetNodeID) == "" {
		return fmt.Errorf("target_node_id cannot be empty")
	}
	if !e.TriggerType.IsValid() {
		return fmt.Errorf("invalid trigger_type: %s", e.TriggerType)
	}
	if !e.Status.IsValid() {
		return fmt.Errorf("invalid overall evaluation status: %s", e.Status)
	}
	for i := range e.Results {
		if err := e.Results[i].Validate(); err != nil {
			return fmt.Errorf("result[%d] invalid: %w", i, err)
		}
	}
	return nil
}

// EvaluationFilter defines query parameters for listing evaluation executions.
type EvaluationFilter struct {
	OrgID        string                `json:"org_id,omitempty"`
	TargetNodeID string                `json:"target_node_id,omitempty"`
	TriggerType  EvaluationTriggerType `json:"trigger_type,omitempty"`
	Status       EvaluationStatus      `json:"status,omitempty"`
	Since        time.Time             `json:"since,omitempty"`
	Until        time.Time             `json:"until,omitempty"`
	Limit        int                   `json:"limit,omitempty"`
	Offset       int                   `json:"offset,omitempty"`
}
