package model

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// PolicyStatus represents the lifecycle state of a policy.
type PolicyStatus string

const (
	PolicyStatusDraft    PolicyStatus = "draft"
	PolicyStatusActive   PolicyStatus = "active"
	PolicyStatusDisabled PolicyStatus = "disabled"
	PolicyStatusArchived PolicyStatus = "archived"
)

// IsValid reports whether the policy status is recognized.
func (s PolicyStatus) IsValid() bool {
	switch s {
	case PolicyStatusDraft, PolicyStatusActive, PolicyStatusDisabled, PolicyStatusArchived:
		return true
	default:
		return false
	}
}

// CanTransitionTo determines if the state transition from s to target is allowed.
func (s PolicyStatus) CanTransitionTo(target PolicyStatus) bool {
	if s == target {
		return true
	}
	switch s {
	case PolicyStatusDraft:
		return target == PolicyStatusActive || target == PolicyStatusArchived
	case PolicyStatusActive:
		return target == PolicyStatusDisabled || target == PolicyStatusArchived || target == PolicyStatusDraft
	case PolicyStatusDisabled:
		return target == PolicyStatusActive || target == PolicyStatusArchived || target == PolicyStatusDraft
	case PolicyStatusArchived:
		// Terminal state: no outbound transitions permitted
		return false
	default:
		return false
	}
}

// PolicyCategory categorizes the domain and scope of a policy.
type PolicyCategory string

const (
	PolicyCategoryResourceThresholds    PolicyCategory = "resource_thresholds"
	PolicyCategoryAnomalyDetection      PolicyCategory = "anomaly_detection"
	PolicyCategoryCapacityPlanning      PolicyCategory = "capacity_planning"
	PolicyCategoryIncidentSeverity      PolicyCategory = "incident_severity"
	PolicyCategoryOperationalCompliance PolicyCategory = "operational_compliance"
)

// IsValid reports whether the policy category is recognized.
func (c PolicyCategory) IsValid() bool {
	switch c {
	case PolicyCategoryResourceThresholds,
		PolicyCategoryAnomalyDetection,
		PolicyCategoryCapacityPlanning,
		PolicyCategoryIncidentSeverity,
		PolicyCategoryOperationalCompliance:
		return true
	default:
		return false
	}
}

// InheritanceMode dictates how rules in this policy interact with inherited ancestor policies.
type InheritanceMode string

const (
	// InheritanceModeInheritAndOverride merges ancestor rules and overrides matching rules by ID/Metric.
	InheritanceModeInheritAndOverride InheritanceMode = "inherit_and_override"
	// InheritanceModeStrictOverride completely displaces ancestor rules in the same category.
	InheritanceModeStrictOverride InheritanceMode = "strict_override"
	// InheritanceModeAdditive appends rules to ancestor rules without replacement.
	InheritanceModeAdditive InheritanceMode = "additive"
)

// IsValid reports whether the inheritance mode is recognized.
func (m InheritanceMode) IsValid() bool {
	switch m {
	case InheritanceModeInheritAndOverride, InheritanceModeStrictOverride, InheritanceModeAdditive:
		return true
	default:
		return false
	}
}

// EnforcementMode defines how strictly the policy is applied.
type EnforcementMode string

const (
	// EnforcementModeEnforce actively applies evaluation, alerts, and automated actions.
	EnforcementModeEnforce EnforcementMode = "enforce"
	// EnforcementModeAdvisory produces advisory notifications without escalating incident severity.
	EnforcementModeAdvisory EnforcementMode = "advisory"
	// EnforcementModeDryRun evaluates and records outcomes without publishing alerts or notifications.
	EnforcementModeDryRun EnforcementMode = "dry_run"
)

// IsValid reports whether the enforcement mode is recognized.
func (m EnforcementMode) IsValid() bool {
	switch m {
	case EnforcementModeEnforce, EnforcementModeAdvisory, EnforcementModeDryRun:
		return true
	default:
		return false
	}
}

// PolicyTargetType specifies the scope level to which a policy is assigned.
type PolicyTargetType string

const (
	TargetTypeOrganization PolicyTargetType = "org"
	TargetTypeFleetGroup   PolicyTargetType = "fleet_group"
	TargetTypeNode         PolicyTargetType = "node"
)

// IsValid reports whether the target type is recognized.
func (t PolicyTargetType) IsValid() bool {
	switch t {
	case TargetTypeOrganization, TargetTypeFleetGroup, TargetTypeNode:
		return true
	default:
		return false
	}
}

// PolicyRuleType identifies the configuration contract of a rule.
type PolicyRuleType string

const (
	RuleTypeResourceThreshold    PolicyRuleType = "resource_threshold"
	RuleTypeAnomalyDetection     PolicyRuleType = "anomaly_detection"
	RuleTypeCapacityPlanning     PolicyRuleType = "capacity_planning"
	RuleTypeIncidentSeverity     PolicyRuleType = "incident_severity"
	RuleTypeOperationalCompliance PolicyRuleType = "operational_compliance"
)

// IsValid reports whether the rule type is recognized.
func (t PolicyRuleType) IsValid() bool {
	switch t {
	case RuleTypeResourceThreshold,
		RuleTypeAnomalyDetection,
		RuleTypeCapacityPlanning,
		RuleTypeIncidentSeverity,
		RuleTypeOperationalCompliance:
		return true
	default:
		return false
	}
}

// ResourceThresholdRuleConfig defines thresholds and actions for resource telemetry.
type ResourceThresholdRuleConfig struct {
	Metric            string  `json:"metric" yaml:"metric"`                         // e.g., cpu_usage_pct, memory_used_pct
	WarningThreshold  float64 `json:"warning_threshold" yaml:"warning_threshold"`   // Warning breach boundary
	CriticalThreshold float64 `json:"critical_threshold" yaml:"critical_threshold"` // Critical breach boundary
	Unit              string  `json:"unit,omitempty" yaml:"unit,omitempty"`         // %, MB, GB, ms
	DurationWindow    string  `json:"duration_window,omitempty" yaml:"duration_window"` // e.g., 1m, 5m
	RecoveryThreshold float64 `json:"recovery_threshold,omitempty" yaml:"recovery_threshold,omitempty"`
	Action            string  `json:"action,omitempty" yaml:"action,omitempty"`     // alert, log, escalate
}

// Validate checks resource threshold parameters.
func (c *ResourceThresholdRuleConfig) Validate() error {
	if c == nil {
		return fmt.Errorf("nil resource threshold config")
	}
	if strings.TrimSpace(c.Metric) == "" {
		return fmt.Errorf("metric cannot be empty")
	}
	if c.CriticalThreshold < c.WarningThreshold && c.CriticalThreshold != 0 {
		// Note: For descending metrics (e.g. disk_available_pct), warning can be higher than critical.
		// We allow both directional orientations as long as they are distinct.
	}
	return nil
}

// AnomalyDetectionRuleConfig defines parameters for automated anomaly detection.
type AnomalyDetectionRuleConfig struct {
	Metric          string  `json:"metric" yaml:"metric"`
	Sensitivity     string  `json:"sensitivity" yaml:"sensitivity"` // low, medium, high
	ZScoreThreshold float64 `json:"z_score_threshold" yaml:"z_score_threshold"`
	MinDuration     string  `json:"min_duration,omitempty" yaml:"min_duration,omitempty"`
	ExcludedHours   []int   `json:"excluded_hours,omitempty" yaml:"excluded_hours,omitempty"` // 0-23
	AutoBaseline    bool    `json:"auto_baseline" yaml:"auto_baseline"`
}

// Validate checks anomaly detection configuration.
func (c *AnomalyDetectionRuleConfig) Validate() error {
	if c == nil {
		return fmt.Errorf("nil anomaly detection config")
	}
	if strings.TrimSpace(c.Metric) == "" {
		return fmt.Errorf("metric cannot be empty")
	}
	if c.ZScoreThreshold < 0 {
		return fmt.Errorf("z_score_threshold must be non-negative")
	}
	for _, h := range c.ExcludedHours {
		if h < 0 || h > 23 {
			return fmt.Errorf("excluded hour must be between 0 and 23, got %d", h)
		}
	}
	return nil
}

// CapacityPlanningRuleConfig defines forecasting and exhaustion parameters.
type CapacityPlanningRuleConfig struct {
	Metric                   string `json:"metric" yaml:"metric"`
	HorizonDays              int    `json:"horizon_days" yaml:"horizon_days"`                             // e.g. 30, 90, 180
	WarningDaysToExhaustion  int    `json:"warning_days_to_exhaustion" yaml:"warning_days_to_exhaustion"`   // e.g. 30
	CriticalDaysToExhaustion int    `json:"critical_days_to_exhaustion" yaml:"critical_days_to_exhaustion"` // e.g. 14
	GrowthModel              string `json:"growth_model,omitempty" yaml:"growth_model,omitempty"`         // linear, exponential, seasonal
	MinDataPoints            int    `json:"min_data_points,omitempty" yaml:"min_data_points,omitempty"`
}

// Validate checks capacity planning parameters.
func (c *CapacityPlanningRuleConfig) Validate() error {
	if c == nil {
		return fmt.Errorf("nil capacity planning config")
	}
	if strings.TrimSpace(c.Metric) == "" {
		return fmt.Errorf("metric cannot be empty")
	}
	if c.HorizonDays <= 0 {
		return fmt.Errorf("horizon_days must be greater than 0")
	}
	if c.WarningDaysToExhaustion <= 0 {
		return fmt.Errorf("warning_days_to_exhaustion must be greater than 0")
	}
	if c.CriticalDaysToExhaustion <= 0 {
		return fmt.Errorf("critical_days_to_exhaustion must be greater than 0")
	}
	if c.CriticalDaysToExhaustion > c.WarningDaysToExhaustion {
		return fmt.Errorf("critical_days_to_exhaustion (%d) cannot exceed warning_days_to_exhaustion (%d)",
			c.CriticalDaysToExhaustion, c.WarningDaysToExhaustion)
	}
	return nil
}

// IncidentSeverityRuleConfig defines incident escalation and routing parameters.
type IncidentSeverityRuleConfig struct {
	Condition            string   `json:"condition" yaml:"condition"`                                       // e.g. duration > 10m
	EscalateToSeverity   Severity `json:"escalate_to_severity" yaml:"escalate_to_severity"`                 // INFO, WARNING, CRITICAL
	NotificationChannel  string   `json:"notification_channel,omitempty" yaml:"notification_channel,omitempty"`
	AutoEscalationAfter  string   `json:"auto_escalation_after,omitempty" yaml:"auto_escalation_after,omitempty"`
}

// Validate checks incident severity parameters.
func (c *IncidentSeverityRuleConfig) Validate() error {
	if c == nil {
		return fmt.Errorf("nil incident severity config")
	}
	if strings.TrimSpace(c.Condition) == "" {
		return fmt.Errorf("condition cannot be empty")
	}
	if c.EscalateToSeverity != "" {
		switch c.EscalateToSeverity {
		case SeverityInfo, SeverityWarning, SeverityCritical:
		default:
			return fmt.Errorf("invalid escalate_to_severity: %s", c.EscalateToSeverity)
		}
	}
	return nil
}

// OperationalComplianceRuleConfig defines compliance and posture auditing checks.
type OperationalComplianceRuleConfig struct {
	CheckType         string   `json:"check_type" yaml:"check_type"` // heartbeat_freshness, mandatory_tags, collector_version, allowed_ports, approved_regions
	ExpectedValue     string   `json:"expected_value,omitempty" yaml:"expected_value,omitempty"`
	ExpectedValues    []string `json:"expected_values,omitempty" yaml:"expected_values,omitempty"`
	MaxAgeSeconds     int64    `json:"max_age_seconds,omitempty" yaml:"max_age_seconds,omitempty"`
	ViolationSeverity Severity `json:"violation_severity" yaml:"violation_severity"`
}

// Validate checks compliance check parameters.
func (c *OperationalComplianceRuleConfig) Validate() error {
	if c == nil {
		return fmt.Errorf("nil operational compliance config")
	}
	if strings.TrimSpace(c.CheckType) == "" {
		return fmt.Errorf("check_type cannot be empty")
	}
	if c.ViolationSeverity == "" {
		c.ViolationSeverity = SeverityWarning
	} else {
		switch c.ViolationSeverity {
		case SeverityInfo, SeverityWarning, SeverityCritical:
		default:
			return fmt.Errorf("invalid violation_severity: %s", c.ViolationSeverity)
		}
	}
	return nil
}

// PolicyRule defines an individual rule within a policy revision.
type PolicyRule struct {
	ID                    string                           `json:"id" yaml:"id"`
	Name                  string                           `json:"name" yaml:"name"`
	Description           string                           `json:"description,omitempty" yaml:"description,omitempty"`
	Type                  PolicyRuleType                   `json:"type" yaml:"type"`
	Severity              Severity                         `json:"severity,omitempty" yaml:"severity,omitempty"`
	Enabled               bool                             `json:"enabled" yaml:"enabled"`
	ResourceThreshold     *ResourceThresholdRuleConfig     `json:"resource_threshold,omitempty" yaml:"resource_threshold,omitempty"`
	AnomalyDetection      *AnomalyDetectionRuleConfig      `json:"anomaly_detection,omitempty" yaml:"anomaly_detection,omitempty"`
	CapacityPlanning      *CapacityPlanningRuleConfig      `json:"capacity_planning,omitempty" yaml:"capacity_planning,omitempty"`
	IncidentSeverity      *IncidentSeverityRuleConfig      `json:"incident_severity,omitempty" yaml:"incident_severity,omitempty"`
	OperationalCompliance *OperationalComplianceRuleConfig `json:"operational_compliance,omitempty" yaml:"operational_compliance,omitempty"`
	Metadata              map[string]string                `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// Validate ensures the integrity and correctness of a rule definition.
func (r *PolicyRule) Validate() error {
	if r == nil {
		return fmt.Errorf("nil policy rule")
	}
	if err := ValidateIdentifier(r.ID); err != nil {
		return fmt.Errorf("invalid rule id: %w", err)
	}
	if strings.TrimSpace(r.Name) == "" {
		return fmt.Errorf("rule name cannot be empty")
	}
	if !r.Type.IsValid() {
		return fmt.Errorf("invalid rule type: %s", r.Type)
	}
	if r.Severity != "" {
		switch r.Severity {
		case SeverityInfo, SeverityWarning, SeverityCritical:
		default:
			return fmt.Errorf("invalid severity: %s", r.Severity)
		}
	}

	switch r.Type {
	case RuleTypeResourceThreshold:
		if r.ResourceThreshold == nil {
			return fmt.Errorf("resource_threshold rule requires resource_threshold config")
		}
		if err := r.ResourceThreshold.Validate(); err != nil {
			return fmt.Errorf("invalid resource_threshold config: %w", err)
		}
	case RuleTypeAnomalyDetection:
		if r.AnomalyDetection == nil {
			return fmt.Errorf("anomaly_detection rule requires anomaly_detection config")
		}
		if err := r.AnomalyDetection.Validate(); err != nil {
			return fmt.Errorf("invalid anomaly_detection config: %w", err)
		}
	case RuleTypeCapacityPlanning:
		if r.CapacityPlanning == nil {
			return fmt.Errorf("capacity_planning rule requires capacity_planning config")
		}
		if err := r.CapacityPlanning.Validate(); err != nil {
			return fmt.Errorf("invalid capacity_planning config: %w", err)
		}
	case RuleTypeIncidentSeverity:
		if r.IncidentSeverity == nil {
			return fmt.Errorf("incident_severity rule requires incident_severity config")
		}
		if err := r.IncidentSeverity.Validate(); err != nil {
			return fmt.Errorf("invalid incident_severity config: %w", err)
		}
	case RuleTypeOperationalCompliance:
		if r.OperationalCompliance == nil {
			return fmt.Errorf("operational_compliance rule requires operational_compliance config")
		}
		if err := r.OperationalCompliance.Validate(); err != nil {
			return fmt.Errorf("invalid operational_compliance config: %w", err)
		}
	}
	return nil
}

// PolicyRevision represents an immutable, versioned snapshot of policy rules and targeting.
type PolicyRevision struct {
	PolicyID        string            `json:"policy_id" yaml:"policy_id"`
	Revision        int               `json:"revision" yaml:"revision"`
	CreatedAt       time.Time         `json:"created_at" yaml:"created_at"`
	CreatedBy       string            `json:"created_by,omitempty" yaml:"created_by,omitempty"`
	ChangeSummary   string            `json:"change_summary,omitempty" yaml:"change_summary,omitempty"`
	ContentDigest   string            `json:"content_digest" yaml:"content_digest"`
	Rules           []PolicyRule      `json:"rules" yaml:"rules"`
	Selector        string            `json:"selector,omitempty" yaml:"selector,omitempty"`
	Priority        int               `json:"priority" yaml:"priority"`
	InheritanceMode InheritanceMode   `json:"inheritance_mode" yaml:"inheritance_mode"`
	EnforcementMode EnforcementMode   `json:"enforcement_mode" yaml:"enforcement_mode"`
	Metadata        map[string]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// ComputeDigest computes a deterministic SHA-256 hash of the revision's contents.
func (pr *PolicyRevision) ComputeDigest() string {
	if pr == nil {
		return ""
	}

	// Create a canonical copy of rules sorted by ID to ensure deterministic serialization
	rulesCopy := make([]PolicyRule, len(pr.Rules))
	copy(rulesCopy, pr.Rules)
	sort.Slice(rulesCopy, func(i, j int) bool {
		return rulesCopy[i].ID < rulesCopy[j].ID
	})

	canonical := struct {
		PolicyID        string            `json:"policy_id"`
		Revision        int               `json:"revision"`
		Rules           []PolicyRule      `json:"rules"`
		Selector        string            `json:"selector"`
		Priority        int               `json:"priority"`
		InheritanceMode InheritanceMode   `json:"inheritance_mode"`
		EnforcementMode EnforcementMode   `json:"enforcement_mode"`
		Metadata        map[string]string `json:"metadata,omitempty"`
	}{
		PolicyID:        pr.PolicyID,
		Revision:        pr.Revision,
		Rules:           rulesCopy,
		Selector:        pr.Selector,
		Priority:        pr.Priority,
		InheritanceMode: pr.InheritanceMode,
		EnforcementMode: pr.EnforcementMode,
		Metadata:        pr.Metadata,
	}

	raw, _ := json.Marshal(canonical)
	hash := sha256.Sum256(raw)
	return hex.EncodeToString(hash[:])
}

// Validate checks the revision's structural integrity, rules, and digest.
func (pr *PolicyRevision) Validate() error {
	if pr == nil {
		return fmt.Errorf("nil policy revision")
	}
	if err := ValidateIdentifier(pr.PolicyID); err != nil {
		return fmt.Errorf("invalid policy_id: %w", err)
	}
	if pr.Revision < 1 {
		return fmt.Errorf("revision must be greater than or equal to 1, got %d", pr.Revision)
	}
	if pr.InheritanceMode == "" {
		pr.InheritanceMode = InheritanceModeInheritAndOverride
	} else if !pr.InheritanceMode.IsValid() {
		return fmt.Errorf("invalid inheritance_mode: %s", pr.InheritanceMode)
	}
	if pr.EnforcementMode == "" {
		pr.EnforcementMode = EnforcementModeEnforce
	} else if !pr.EnforcementMode.IsValid() {
		return fmt.Errorf("invalid enforcement_mode: %s", pr.EnforcementMode)
	}

	ruleIDs := make(map[string]bool, len(pr.Rules))
	for i := range pr.Rules {
		if err := pr.Rules[i].Validate(); err != nil {
			return fmt.Errorf("rule[%d] invalid: %w", i, err)
		}
		if ruleIDs[pr.Rules[i].ID] {
			return fmt.Errorf("duplicate rule id %q in revision", pr.Rules[i].ID)
		}
		ruleIDs[pr.Rules[i].ID] = true
	}

	expectedDigest := pr.ComputeDigest()
	if pr.ContentDigest == "" {
		pr.ContentDigest = expectedDigest
	} else if pr.ContentDigest != expectedDigest {
		return fmt.Errorf("content digest mismatch: expected %s, got %s", expectedDigest, pr.ContentDigest)
	}

	return nil
}

// Policy represents a named, categorized declarative governance policy entity.
type Policy struct {
	ID             string            `json:"id" yaml:"id"`
	OrgID          string            `json:"org_id" yaml:"org_id"`
	Name           string            `json:"name" yaml:"name"`
	DisplayName    string            `json:"display_name,omitempty" yaml:"display_name,omitempty"`
	Description    string            `json:"description,omitempty" yaml:"description,omitempty"`
	Category       PolicyCategory    `json:"category" yaml:"category"`
	Status         PolicyStatus      `json:"status" yaml:"status"`
	ActiveRevision int               `json:"active_revision" yaml:"active_revision"`
	CreatedAt      time.Time         `json:"created_at" yaml:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at" yaml:"updated_at"`
	Metadata       map[string]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// Validate ensures all required policy fields are sound.
func (p *Policy) Validate() error {
	if p == nil {
		return fmt.Errorf("nil policy")
	}
	if err := ValidateIdentifier(p.ID); err != nil {
		return fmt.Errorf("invalid policy id: %w", err)
	}
	if p.OrgID == "" {
		p.OrgID = DefaultOrganizationID
	} else if err := ValidateIdentifier(p.OrgID); err != nil {
		return fmt.Errorf("invalid policy org_id: %w", err)
	}
	if strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("policy name cannot be empty")
	}
	if len(p.Name) > 255 {
		return fmt.Errorf("policy name exceeds 255 characters")
	}
	if !p.Category.IsValid() {
		return fmt.Errorf("invalid policy category: %s", p.Category)
	}
	if p.Status == "" {
		p.Status = PolicyStatusDraft
	} else if !p.Status.IsValid() {
		return fmt.Errorf("invalid policy status: %s", p.Status)
	}
	if p.ActiveRevision < 0 {
		return fmt.Errorf("active_revision cannot be negative: %d", p.ActiveRevision)
	}
	return nil
}

// CanTransitionTo checks if a lifecycle state transition is legal for this policy.
func (p *Policy) CanTransitionTo(target PolicyStatus) bool {
	if p == nil {
		return false
	}
	return p.Status.CanTransitionTo(target)
}

// PolicyAssignment maps a policy to an organization, fleet group, or specific node.
type PolicyAssignment struct {
	ID         string            `json:"id" yaml:"id"`
	PolicyID   string            `json:"policy_id" yaml:"policy_id"`
	OrgID      string            `json:"org_id" yaml:"org_id"`
	TargetType PolicyTargetType  `json:"target_type" yaml:"target_type"`
	TargetID   string            `json:"target_id" yaml:"target_id"`
	AssignedAt time.Time         `json:"assigned_at" yaml:"assigned_at"`
	AssignedBy string            `json:"assigned_by,omitempty" yaml:"assigned_by,omitempty"`
	Enabled    bool              `json:"enabled" yaml:"enabled"`
	Metadata   map[string]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// Validate ensures structural validity of the assignment.
func (a *PolicyAssignment) Validate() error {
	if a == nil {
		return fmt.Errorf("nil policy assignment")
	}
	if err := ValidateIdentifier(a.ID); err != nil {
		return fmt.Errorf("invalid assignment id: %w", err)
	}
	if err := ValidateIdentifier(a.PolicyID); err != nil {
		return fmt.Errorf("invalid policy_id: %w", err)
	}
	if a.OrgID == "" {
		a.OrgID = DefaultOrganizationID
	} else if err := ValidateIdentifier(a.OrgID); err != nil {
		return fmt.Errorf("invalid org_id: %w", err)
	}
	if !a.TargetType.IsValid() {
		return fmt.Errorf("invalid target_type: %s", a.TargetType)
	}
	if err := ValidateIdentifier(a.TargetID); err != nil {
		return fmt.Errorf("invalid target_id: %w", err)
	}
	return nil
}
