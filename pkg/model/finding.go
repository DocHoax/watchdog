package model

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"time"
)

// FindingStatus represents the operational lifecycle state of a compliance finding.
type FindingStatus string

const (
	FindingStatusOpen          FindingStatus = "open"
	FindingStatusRecurring     FindingStatus = "recurring"
	FindingStatusResolved      FindingStatus = "resolved"
	FindingStatusIndeterminate FindingStatus = "indeterminate"
)

// IsValid reports whether the finding status is recognized.
func (s FindingStatus) IsValid() bool {
	switch s {
	case FindingStatusOpen,
		FindingStatusRecurring,
		FindingStatusResolved,
		FindingStatusIndeterminate:
		return true
	default:
		return false
	}
}

// ComputeFindingID generates a deterministic unique hash for finding deduplication.
func ComputeFindingID(orgID, targetNodeID, policyID, ruleID string) string {
	raw := fmt.Sprintf("%s:%s:%s:%s",
		strings.TrimSpace(orgID),
		strings.TrimSpace(targetNodeID),
		strings.TrimSpace(policyID),
		strings.TrimSpace(ruleID),
	)
	hash := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("fnd_%x", hash[:16])
}

// ComplianceFinding represents a tracked policy violation or warning across evaluation runs.
type ComplianceFinding struct {
	ID              string            `json:"id" yaml:"id"`
	OrgID           string            `json:"org_id" yaml:"org_id"`
	TargetNodeID    string            `json:"target_node_id" yaml:"target_node_id"`
	PolicyID        string            `json:"policy_id" yaml:"policy_id"`
	PolicyRevision  int               `json:"policy_revision" yaml:"policy_revision"`
	RuleID          string            `json:"rule_id" yaml:"rule_id"`
	RuleName        string            `json:"rule_name,omitempty" yaml:"rule_name,omitempty"`
	Category        PolicyCategory    `json:"category" yaml:"category"`
	Severity        Severity          `json:"severity" yaml:"severity"`
	EnforcementMode EnforcementMode   `json:"enforcement_mode" yaml:"enforcement_mode"`
	Status          FindingStatus     `json:"status" yaml:"status"`
	FirstSeenAt     time.Time         `json:"first_seen_at" yaml:"first_seen_at"`
	LastSeenAt      time.Time         `json:"last_seen_at" yaml:"last_seen_at"`
	ResolvedAt      *time.Time        `json:"resolved_at,omitempty" yaml:"resolved_at,omitempty"`
	OccurrenceCount int               `json:"occurrence_count" yaml:"occurrence_count"`
	Message         string            `json:"message" yaml:"message"`
	ObservedValue   string            `json:"observed_value,omitempty" yaml:"observed_value,omitempty"`
	ExpectedValue   string            `json:"expected_value,omitempty" yaml:"expected_value,omitempty"`
	ContextData     map[string]string `json:"context_data,omitempty" yaml:"context_data,omitempty"`
}

// Validate checks the structural soundness of a compliance finding.
func (f *ComplianceFinding) Validate() error {
	if f == nil {
		return fmt.Errorf("nil compliance finding")
	}
	if strings.TrimSpace(f.ID) == "" {
		return fmt.Errorf("finding id cannot be empty")
	}
	if f.OrgID == "" {
		f.OrgID = DefaultOrganizationID
	} else if err := ValidateIdentifier(f.OrgID); err != nil {
		return fmt.Errorf("invalid org_id: %w", err)
	}
	if strings.TrimSpace(f.TargetNodeID) == "" {
		return fmt.Errorf("target_node_id cannot be empty")
	}
	if strings.TrimSpace(f.PolicyID) == "" {
		return fmt.Errorf("policy_id cannot be empty")
	}
	if strings.TrimSpace(f.RuleID) == "" {
		return fmt.Errorf("rule_id cannot be empty")
	}
	if !f.Status.IsValid() {
		return fmt.Errorf("invalid finding status: %s", f.Status)
	}
	if !f.Category.IsValid() {
		return fmt.Errorf("invalid policy category: %s", f.Category)
	}
	if !f.Severity.IsValid() {
		return fmt.Errorf("invalid severity: %s", f.Severity)
	}
	if !f.EnforcementMode.IsValid() {
		return fmt.Errorf("invalid enforcement mode: %s", f.EnforcementMode)
	}
	if f.OccurrenceCount <= 0 {
		return fmt.Errorf("occurrence count must be positive")
	}
	return nil
}

// FindingFilter defines query criteria for listing compliance findings.
type FindingFilter struct {
	OrgID        string         `json:"org_id,omitempty"`
	TargetNodeID string         `json:"target_node_id,omitempty"`
	PolicyID     string         `json:"policy_id,omitempty"`
	RuleID       string         `json:"rule_id,omitempty"`
	Category     PolicyCategory `json:"category,omitempty"`
	Severity     Severity       `json:"severity,omitempty"`
	Status       FindingStatus  `json:"status,omitempty"`
	Since        time.Time      `json:"since,omitempty"`
	Until        time.Time      `json:"until,omitempty"`
	Limit        int            `json:"limit,omitempty"`
	Offset       int            `json:"offset,omitempty"`
}

// ComplianceRollupLevel identifies the aggregation tier for compliance metrics.
type ComplianceRollupLevel string

const (
	ComplianceRollupLevelNode       ComplianceRollupLevel = "node"
	ComplianceRollupLevelFleetGroup ComplianceRollupLevel = "fleet_group"
	ComplianceRollupLevelOrg        ComplianceRollupLevel = "org"
	ComplianceRollupLevelFleet      ComplianceRollupLevel = "fleet"
)

// IsValid reports whether the rollup level is recognized.
func (l ComplianceRollupLevel) IsValid() bool {
	switch l {
	case ComplianceRollupLevelNode,
		ComplianceRollupLevelFleetGroup,
		ComplianceRollupLevelOrg,
		ComplianceRollupLevelFleet:
		return true
	default:
		return false
	}
}

// CategoryComplianceSummary summarizes rule results and compliance within a single category.
type CategoryComplianceSummary struct {
	Category        PolicyCategory `json:"category" yaml:"category"`
	TotalRules      int            `json:"total_rules" yaml:"total_rules"`
	CompliantRules  int            `json:"compliant_rules" yaml:"compliant_rules"`
	Violations      int            `json:"violations" yaml:"violations"`
	Warnings        int            `json:"warnings" yaml:"warnings"`
	ComplianceRatio float64        `json:"compliance_ratio" yaml:"compliance_ratio"`
}

// ComplianceRollupSummary represents aggregated compliance metrics at a node, group, or organization level.
type ComplianceRollupSummary struct {
	ScopeLevel          ComplianceRollupLevel                     `json:"scope_level" yaml:"scope_level"`
	ScopeID             string                                    `json:"scope_id" yaml:"scope_id"`
	OrgID               string                                    `json:"org_id" yaml:"org_id"`
	EvaluatedAt         time.Time                                 `json:"evaluated_at" yaml:"evaluated_at"`
	TotalNodes          int                                       `json:"total_nodes" yaml:"total_nodes"`
	CompliantNodes      int                                       `json:"compliant_nodes" yaml:"compliant_nodes"`
	NonCompliantNodes   int                                       `json:"non_compliant_nodes" yaml:"non_compliant_nodes"`
	WarningNodes        int                                       `json:"warning_nodes" yaml:"warning_nodes"`
	TotalRulesEvaluated int                                       `json:"total_rules_evaluated" yaml:"total_rules_evaluated"`
	TotalFindingsOpen   int                                       `json:"total_findings_open" yaml:"total_findings_open"`
	ComplianceRatio     float64                                   `json:"compliance_ratio" yaml:"compliance_ratio"` // [0.0 .. 1.0]
	CoverageRatio       float64                                   `json:"coverage_ratio" yaml:"coverage_ratio"`     // [0.0 .. 1.0]
	BreakdownByCategory map[PolicyCategory]CategoryComplianceSummary `json:"breakdown_by_category,omitempty" yaml:"breakdown_by_category,omitempty"`
	BreakdownBySeverity map[Severity]int                          `json:"breakdown_by_severity,omitempty" yaml:"breakdown_by_severity,omitempty"`
}
