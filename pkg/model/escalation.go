package model

import (
	"fmt"
	"strings"
	"time"
)

// EscalationChannel defines the communication channel for an escalation stage.
type EscalationChannel string

const (
	EscalationChannelEmail     EscalationChannel = "email"
	EscalationChannelSlack     EscalationChannel = "slack"
	EscalationChannelPagerDuty EscalationChannel = "pagerduty"
	EscalationChannelWebhook   EscalationChannel = "webhook"
)

// IsValid reports whether the escalation channel is recognized.
func (c EscalationChannel) IsValid() bool {
	switch c {
	case EscalationChannelEmail, EscalationChannelSlack, EscalationChannelPagerDuty, EscalationChannelWebhook:
		return true
	default:
		return false
	}
}

// EscalationTargetType specifies the category of recipient.
type EscalationTargetType string

const (
	EscalationTargetTeam     EscalationTargetType = "team"
	EscalationTargetUser     EscalationTargetType = "user"
	EscalationTargetRole     EscalationTargetType = "role"
	EscalationTargetSchedule EscalationTargetType = "schedule"
)

// IsValid reports whether the target type is recognized.
func (t EscalationTargetType) IsValid() bool {
	switch t {
	case EscalationTargetTeam, EscalationTargetUser, EscalationTargetRole, EscalationTargetSchedule:
		return true
	default:
		return false
	}
}

// EscalationTarget defines a notification recipient for an escalation stage.
type EscalationTarget struct {
	TargetType  EscalationTargetType `json:"target_type" yaml:"target_type"`
	TargetID    string               `json:"target_id" yaml:"target_id"`
	Name        string               `json:"name" yaml:"name"`
	ContactInfo string               `json:"contact_info" yaml:"contact_info"` // e.g. email address or channel webhook
}

// Validate checks the target attributes.
func (t *EscalationTarget) Validate() error {
	if t == nil {
		return fmt.Errorf("nil escalation target")
	}
	if !t.TargetType.IsValid() {
		return fmt.Errorf("invalid escalation target type: %s", t.TargetType)
	}
	if strings.TrimSpace(t.TargetID) == "" {
		return fmt.Errorf("target_id cannot be empty")
	}
	if strings.TrimSpace(t.Name) == "" {
		return fmt.Errorf("target name cannot be empty")
	}
	return nil
}

// EscalationStage represents a step in an escalation timeline.
type EscalationStage struct {
	StageNumber    int                `json:"stage_number" yaml:"stage_number"` // 1, 2, 3...
	DelayMinutes   int                `json:"delay_minutes" yaml:"delay_minutes"` // Elapsed time before triggering
	Targets        []EscalationTarget `json:"targets" yaml:"targets"`
	Channel        EscalationChannel  `json:"channel" yaml:"channel"`
	FallbackTarget *EscalationTarget  `json:"fallback_target,omitempty" yaml:"fallback_target,omitempty"`
}

// Validate checks the stage parameters.
func (s *EscalationStage) Validate() error {
	if s == nil {
		return fmt.Errorf("nil escalation stage")
	}
	if s.StageNumber < 1 {
		return fmt.Errorf("stage_number must be >= 1, got %d", s.StageNumber)
	}
	if s.DelayMinutes < 0 {
		return fmt.Errorf("delay_minutes cannot be negative, got %d", s.DelayMinutes)
	}
	if len(s.Targets) == 0 && s.FallbackTarget == nil {
		return fmt.Errorf("stage %d must specify at least one target or fallback target", s.StageNumber)
	}
	for i, t := range s.Targets {
		if err := t.Validate(); err != nil {
			return fmt.Errorf("stage %d target[%d] invalid: %w", s.StageNumber, i, err)
		}
	}
	if s.Channel == "" {
		s.Channel = EscalationChannelEmail
	} else if !s.Channel.IsValid() {
		return fmt.Errorf("invalid escalation channel: %s", s.Channel)
	}
	if s.FallbackTarget != nil {
		if err := s.FallbackTarget.Validate(); err != nil {
			return fmt.Errorf("stage %d fallback target invalid: %w", s.StageNumber, err)
		}
	}
	return nil
}

// EscalationPolicy configures how alerts and incidents are escalated over time.
type EscalationPolicy struct {
	ID             string            `json:"id" yaml:"id"`
	OrgID          string            `json:"org_id" yaml:"org_id"`
	Name           string            `json:"name" yaml:"name"`
	Description    string            `json:"description,omitempty" yaml:"description,omitempty"`
	Enabled        bool              `json:"enabled" yaml:"enabled"`
	SeverityLevels []Severity        `json:"severity_levels" yaml:"severity_levels"` // e.g. Critical, Warning
	Stages         []EscalationStage `json:"stages" yaml:"stages"`
	CreatedAt      time.Time         `json:"created_at" yaml:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at" yaml:"updated_at"`
	Metadata       map[string]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// Validate checks the structural validity of the escalation policy.
func (p *EscalationPolicy) Validate() error {
	if p == nil {
		return fmt.Errorf("nil escalation policy")
	}
	if err := ValidateIdentifier(p.ID); err != nil {
		return fmt.Errorf("invalid escalation policy id: %w", err)
	}
	if err := ValidateIdentifier(p.OrgID); err != nil {
		return fmt.Errorf("invalid org_id: %w", err)
	}
	if strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("escalation policy name cannot be empty")
	}
	if len(p.Name) > 255 {
		return fmt.Errorf("escalation policy name exceeds 255 characters")
	}
	if len(p.SeverityLevels) == 0 {
		return fmt.Errorf("escalation policy must cover at least one severity level")
	}
	for _, sev := range p.SeverityLevels {
		if !sev.IsValid() {
			return fmt.Errorf("invalid severity level in escalation policy: %s", sev)
		}
	}
	if len(p.Stages) == 0 {
		return fmt.Errorf("escalation policy must contain at least one stage")
	}

	prevDelay := -1
	for i, stage := range p.Stages {
		if stage.StageNumber != i+1 {
			return fmt.Errorf("stages must be numbered sequentially starting from 1, stage[%d] has number %d", i, stage.StageNumber)
		}
		if stage.DelayMinutes < prevDelay {
			return fmt.Errorf("stage %d delay (%d min) must be >= previous stage delay (%d min)", stage.StageNumber, stage.DelayMinutes, prevDelay)
		}
		prevDelay = stage.DelayMinutes
		if err := stage.Validate(); err != nil {
			return fmt.Errorf("invalid stage %d: %w", stage.StageNumber, err)
		}
	}

	return nil
}

// EscalationPolicyFilter defines query parameters for listing escalation policies.
type EscalationPolicyFilter struct {
	OrgID    string   `json:"org_id,omitempty"`
	Enabled  *bool    `json:"enabled,omitempty"`
	Severity Severity `json:"severity,omitempty"`
	Limit    int      `json:"limit,omitempty"`
	Offset   int      `json:"offset,omitempty"`
}

// OwnershipSourceLevel indicates where resolved ownership metadata originated.
type OwnershipSourceLevel string

const (
	OwnershipLevelNode         OwnershipSourceLevel = "node"
	OwnershipLevelFleetGroup   OwnershipSourceLevel = "fleet_group"
	OwnershipLevelService      OwnershipSourceLevel = "service"
	OwnershipLevelOrganization OwnershipSourceLevel = "org"
)

// IsValid reports whether the ownership source level is recognized.
func (l OwnershipSourceLevel) IsValid() bool {
	switch l {
	case OwnershipLevelNode, OwnershipLevelFleetGroup, OwnershipLevelService, OwnershipLevelOrganization:
		return true
	default:
		return false
	}
}

// ResolvedOwnership captures the hierarchical, deterministic ownership resolution for a node.
type ResolvedOwnership struct {
	NodeID              string               `json:"node_id" yaml:"node_id"`
	OrgID               string               `json:"org_id" yaml:"org_id"`
	OwnerTeam           string               `json:"owner_team,omitempty" yaml:"owner_team,omitempty"`
	ContactEmail        string               `json:"contact_email,omitempty" yaml:"contact_email,omitempty"`
	ContactChannel      string               `json:"contact_channel,omitempty" yaml:"contact_channel,omitempty"`
	Environment         string               `json:"environment,omitempty" yaml:"environment,omitempty"`
	Region              string               `json:"region,omitempty" yaml:"region,omitempty"`
	DataClassification  string               `json:"data_classification,omitempty" yaml:"data_classification,omitempty"`
	CostCenter          string               `json:"cost_center,omitempty" yaml:"cost_center,omitempty"`
	BusinessCriticality BusinessCriticality  `json:"business_criticality" yaml:"business_criticality"`
	Lifecycle           NodeLifecycleStatus  `json:"lifecycle" yaml:"lifecycle"`
	SourceLevel         OwnershipSourceLevel `json:"source_level" yaml:"source_level"`
	SourceID            string               `json:"source_id,omitempty" yaml:"source_id,omitempty"` // Group ID or Org ID
	HierarchyPath       string               `json:"hierarchy_path,omitempty" yaml:"hierarchy_path,omitempty"`
	CustomProperties    map[string]string    `json:"custom_properties,omitempty" yaml:"custom_properties,omitempty"`
	ResolvedAt          time.Time            `json:"resolved_at" yaml:"resolved_at"`
}

// EscalationEvaluationResult holds the read-only evaluated escalation state for an incident.
type EscalationEvaluationResult struct {
	IncidentID            string             `json:"incident_id" yaml:"incident_id"`
	PolicyID              string             `json:"policy_id" yaml:"policy_id"`
	PolicyName            string             `json:"policy_name" yaml:"policy_name"`
	CurrentStage          int                `json:"current_stage" yaml:"current_stage"`
	ElapsedTimeMinutes    int                `json:"elapsed_time_minutes" yaml:"elapsed_time_minutes"`
	ActiveTargets         []EscalationTarget `json:"active_targets" yaml:"active_targets"`
	Channel               EscalationChannel  `json:"channel" yaml:"channel"`
	NextStageNumber       *int               `json:"next_stage_number,omitempty" yaml:"next_stage_number,omitempty"`
	NextStageDelayMinutes *int               `json:"next_stage_delay_minutes,omitempty" yaml:"next_stage_delay_minutes,omitempty"`
	MinutesUntilNextStage *int               `json:"minutes_until_next_stage,omitempty" yaml:"minutes_until_next_stage,omitempty"`
	EvaluatedAt           time.Time          `json:"evaluated_at" yaml:"evaluated_at"`
}
