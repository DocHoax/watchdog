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

// RuleDiffType classifies the nature of change in a policy rule during simulation.
type RuleDiffType string

const (
	RuleDiffTypeAdded     RuleDiffType = "added"
	RuleDiffTypeRemoved   RuleDiffType = "removed"
	RuleDiffTypeModified  RuleDiffType = "modified"
	RuleDiffTypeUnchanged RuleDiffType = "unchanged"
)

// IsValid validates whether the rule diff type is recognized.
func (d RuleDiffType) IsValid() bool {
	switch d {
	case RuleDiffTypeAdded, RuleDiffTypeRemoved, RuleDiffTypeModified, RuleDiffTypeUnchanged:
		return true
	default:
		return false
	}
}

// RuleImpactDiff documents the evaluation status shift for a specific rule on a node.
type RuleImpactDiff struct {
	PolicyID        string           `json:"policy_id" yaml:"policy_id"`
	RuleID          string           `json:"rule_id" yaml:"rule_id"`
	RuleName        string           `json:"rule_name" yaml:"rule_name"`
	Category        PolicyCategory   `json:"category" yaml:"category"`
	Severity        Severity         `json:"severity" yaml:"severity"`
	EnforcementMode EnforcementMode  `json:"enforcement_mode" yaml:"enforcement_mode"`
	DiffType        RuleDiffType     `json:"diff_type" yaml:"diff_type"`
	BaselineStatus  EvaluationStatus `json:"baseline_status,omitempty" yaml:"baseline_status,omitempty"`
	ProposedStatus  EvaluationStatus `json:"proposed_status,omitempty" yaml:"proposed_status,omitempty"`
	BaselineMessage string           `json:"baseline_message,omitempty" yaml:"baseline_message,omitempty"`
	ProposedMessage string           `json:"proposed_message,omitempty" yaml:"proposed_message,omitempty"`
}

// NodeSimulationImpact details the before/after compliance delta on an individual node.
type NodeSimulationImpact struct {
	NodeID                string           `json:"node_id" yaml:"node_id"`
	Hostname              string           `json:"hostname,omitempty" yaml:"hostname,omitempty"`
	BaselineStatus        EvaluationStatus `json:"baseline_status" yaml:"baseline_status"`
	ProposedStatus        EvaluationStatus `json:"proposed_status" yaml:"proposed_status"`
	BaselineRuleCount     int              `json:"baseline_rule_count" yaml:"baseline_rule_count"`
	ProposedRuleCount     int              `json:"proposed_rule_count" yaml:"proposed_rule_count"`
	NewFindingsCount      int              `json:"new_findings_count" yaml:"new_findings_count"`
	ResolvedFindingsCount int              `json:"resolved_findings_count" yaml:"resolved_findings_count"`
	RuleDiffs             []RuleImpactDiff `json:"rule_diffs,omitempty" yaml:"rule_diffs,omitempty"`
}

// SimulationRequest specifies candidate policy and assignment modifications for what-if evaluation.
type SimulationRequest struct {
	OrgID               string             `json:"org_id" yaml:"org_id"`
	TargetNodeIDs       []string           `json:"target_node_ids,omitempty" yaml:"target_node_ids,omitempty"`
	TargetGroupID       string             `json:"target_group_id,omitempty" yaml:"target_group_id,omitempty"`
	IncludeSubgroups    bool               `json:"include_subgroups,omitempty" yaml:"include_subgroups,omitempty"`
	ProposedPolicies    []Policy           `json:"proposed_policies,omitempty" yaml:"proposed_policies,omitempty"`
	ProposedRevisions   []PolicyRevision   `json:"proposed_revisions,omitempty" yaml:"proposed_revisions,omitempty"`
	ProposedAssignments []PolicyAssignment `json:"proposed_assignments,omitempty" yaml:"proposed_assignments,omitempty"`
}

// Validate checks the structural integrity of the simulation request.
func (r *SimulationRequest) Validate() error {
	if r == nil {
		return fmt.Errorf("nil simulation request")
	}
	if r.OrgID == "" {
		r.OrgID = DefaultOrganizationID
	} else if err := ValidateIdentifier(r.OrgID); err != nil {
		return fmt.Errorf("invalid org_id: %w", err)
	}
	for i, p := range r.ProposedPolicies {
		if err := p.Validate(); err != nil {
			return fmt.Errorf("proposed policy[%d] invalid: %w", i, err)
		}
	}
	for i, rev := range r.ProposedRevisions {
		if err := rev.Validate(); err != nil {
			return fmt.Errorf("proposed revision[%d] invalid: %w", i, err)
		}
	}
	for i, a := range r.ProposedAssignments {
		if err := a.Validate(); err != nil {
			return fmt.Errorf("proposed assignment[%d] invalid: %w", i, err)
		}
	}
	return nil
}

// SimulationResult encapsulates the comparative compliance outcome of a policy simulation.
type SimulationResult struct {
	OrgID                   string                  `json:"org_id" yaml:"org_id"`
	SimulatedAt             time.Time               `json:"simulated_at" yaml:"simulated_at"`
	DurationNs              int64                   `json:"duration_ns" yaml:"duration_ns"`
	TotalNodes              int                     `json:"total_nodes" yaml:"total_nodes"`
	EvaluatedNodes          int                     `json:"evaluated_nodes" yaml:"evaluated_nodes"`
	BaselineComplianceRatio float64                 `json:"baseline_compliance_ratio" yaml:"baseline_compliance_ratio"`
	ProposedComplianceRatio float64                 `json:"proposed_compliance_ratio" yaml:"proposed_compliance_ratio"`
	ComplianceRatioDelta    float64                 `json:"compliance_ratio_delta" yaml:"compliance_ratio_delta"`
	BaselineRollup          ComplianceRollupSummary `json:"baseline_rollup" yaml:"baseline_rollup"`
	ProposedRollup          ComplianceRollupSummary `json:"proposed_rollup" yaml:"proposed_rollup"`
	NewFindings             []ComplianceFinding     `json:"new_findings,omitempty" yaml:"new_findings,omitempty"`
	ResolvedFindings        []ComplianceFinding     `json:"resolved_findings,omitempty" yaml:"resolved_findings,omitempty"`
	UnchangedFindings       []ComplianceFinding     `json:"unchanged_findings,omitempty" yaml:"unchanged_findings,omitempty"`
	NodeImpacts             []NodeSimulationImpact  `json:"node_impacts,omitempty" yaml:"node_impacts,omitempty"`
}
