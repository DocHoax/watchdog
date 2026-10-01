package model

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// DefaultOrganizationID is the identifier for the default single-tenant organization.
const DefaultOrganizationID = "default"

// Identifier regex: alphanumeric, dash, underscore, dot, between 1 and 128 characters.
var validIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_\-\.]{1,128}$`)

// ValidateIdentifier verifies that an ID meets naming, character, and length constraints.
func ValidateIdentifier(id string) error {
	if id == "" {
		return fmt.Errorf("identifier cannot be empty")
	}
	if len(id) > 128 {
		return fmt.Errorf("identifier exceeds maximum length of 128 characters: %s", id)
	}
	if !validIDPattern.MatchString(id) {
		return fmt.Errorf("identifier contains invalid characters (allowed: [a-zA-Z0-9_-.], got: %s)", id)
	}
	return nil
}

// OrganizationStatus represents the lifecycle state of an organization.
type OrganizationStatus string

const (
	OrgStatusActive    OrganizationStatus = "active"
	OrgStatusSuspended OrganizationStatus = "suspended"
	OrgStatusArchived  OrganizationStatus = "archived"
)

// IsValid reports whether the organization status is valid.
func (s OrganizationStatus) IsValid() bool {
	switch s {
	case OrgStatusActive, OrgStatusSuspended, OrgStatusArchived:
		return true
	default:
		return false
	}
}

// Organization represents a multi-tenant or enterprise organization boundary.
type Organization struct {
	ID          string             `json:"id" yaml:"id"`
	Name        string             `json:"name" yaml:"name"`
	DisplayName string             `json:"display_name,omitempty" yaml:"display_name,omitempty"`
	Description string             `json:"description,omitempty" yaml:"description,omitempty"`
	Status      OrganizationStatus `json:"status" yaml:"status"`
	CreatedAt   time.Time          `json:"created_at" yaml:"created_at"`
	UpdatedAt   time.Time          `json:"updated_at" yaml:"updated_at"`
	Metadata    map[string]string  `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// Validate ensures all required organization fields are sound.
func (o *Organization) Validate() error {
	if o == nil {
		return fmt.Errorf("nil organization")
	}
	if err := ValidateIdentifier(o.ID); err != nil {
		return fmt.Errorf("invalid organization id: %w", err)
	}
	if strings.TrimSpace(o.Name) == "" {
		return fmt.Errorf("organization name cannot be empty")
	}
	if len(o.Name) > 255 {
		return fmt.Errorf("organization name exceeds 255 characters")
	}
	if o.Status == "" {
		o.Status = OrgStatusActive
	} else if !o.Status.IsValid() {
		return fmt.Errorf("invalid organization status: %s", o.Status)
	}
	return nil
}

// GroupType categorizes the organizational dimension of a fleet group.
type GroupType string

const (
	GroupTypeEnvironment GroupType = "environment"
	GroupTypeDepartment  GroupType = "department"
	GroupTypeRegion      GroupType = "region"
	GroupTypeApplication GroupType = "application"
	GroupTypeTier        GroupType = "tier"
	GroupTypeCustom      GroupType = "custom"
)

// IsValid reports whether the group type is recognized.
func (g GroupType) IsValid() bool {
	switch g {
	case GroupTypeEnvironment, GroupTypeDepartment, GroupTypeRegion, GroupTypeApplication, GroupTypeTier, GroupTypeCustom:
		return true
	default:
		return false
	}
}

// FleetGroup represents a hierarchical node aggregation within an organization.
type FleetGroup struct {
	ID            string            `json:"id" yaml:"id"`
	OrgID         string            `json:"org_id" yaml:"org_id"`
	ParentGroupID string            `json:"parent_group_id,omitempty" yaml:"parent_group_id,omitempty"`
	Name          string            `json:"name" yaml:"name"`
	DisplayName   string            `json:"display_name,omitempty" yaml:"display_name,omitempty"`
	Description   string            `json:"description,omitempty" yaml:"description,omitempty"`
	Type          GroupType         `json:"type" yaml:"type"`
	Path          string            `json:"path,omitempty" yaml:"path,omitempty"`
	CreatedAt     time.Time         `json:"created_at" yaml:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at" yaml:"updated_at"`
	Metadata      map[string]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// Validate checks the structural integrity of a fleet group.
func (g *FleetGroup) Validate() error {
	if g == nil {
		return fmt.Errorf("nil fleet group")
	}
	if err := ValidateIdentifier(g.ID); err != nil {
		return fmt.Errorf("invalid group id: %w", err)
	}
	if g.OrgID == "" {
		g.OrgID = DefaultOrganizationID
	} else if err := ValidateIdentifier(g.OrgID); err != nil {
		return fmt.Errorf("invalid group org_id: %w", err)
	}
	if g.ParentGroupID != "" {
		if err := ValidateIdentifier(g.ParentGroupID); err != nil {
			return fmt.Errorf("invalid parent_group_id: %w", err)
		}
		if g.ParentGroupID == g.ID {
			return fmt.Errorf("group cannot be its own parent: %s", g.ID)
		}
	}
	if strings.TrimSpace(g.Name) == "" {
		return fmt.Errorf("group name cannot be empty")
	}
	if len(g.Name) > 255 {
		return fmt.Errorf("group name exceeds 255 characters")
	}
	if g.Type == "" {
		g.Type = GroupTypeCustom
	} else if !g.Type.IsValid() {
		return fmt.Errorf("invalid group type: %s", g.Type)
	}
	return nil
}

// MembershipRole specifies the relationship of a node to a fleet group.
type MembershipRole string

const (
	MembershipRolePrimary   MembershipRole = "primary"
	MembershipRoleSecondary MembershipRole = "secondary"
	MembershipRoleObserver  MembershipRole = "observer"
)

// IsValid reports whether the membership role is recognized.
func (r MembershipRole) IsValid() bool {
	switch r {
	case MembershipRolePrimary, MembershipRoleSecondary, MembershipRoleObserver:
		return true
	default:
		return false
	}
}

// FleetGroupMember defines a node's membership in a fleet group.
type FleetGroupMember struct {
	GroupID   string            `json:"group_id" yaml:"group_id"`
	NodeID    string            `json:"node_id" yaml:"node_id"`
	AddedAt   time.Time         `json:"added_at" yaml:"added_at"`
	AddedBy   string            `json:"added_by,omitempty" yaml:"added_by,omitempty"`
	Role      MembershipRole    `json:"role" yaml:"role"`
	Metadata  map[string]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// Validate checks the integrity of a group membership link.
func (m *FleetGroupMember) Validate() error {
	if m == nil {
		return fmt.Errorf("nil group member")
	}
	if err := ValidateIdentifier(m.GroupID); err != nil {
		return fmt.Errorf("invalid group_id: %w", err)
	}
	if err := ValidateIdentifier(m.NodeID); err != nil {
		return fmt.Errorf("invalid node_id: %w", err)
	}
	if m.Role == "" {
		m.Role = MembershipRolePrimary
	} else if !m.Role.IsValid() {
		return fmt.Errorf("invalid membership role: %s", m.Role)
	}
	return nil
}

// BusinessCriticality represents the impact level of a service or infrastructure component.
type BusinessCriticality string

const (
	CriticalityMissionCritical BusinessCriticality = "mission_critical"
	CriticalityHigh            BusinessCriticality = "high"
	CriticalityMedium          BusinessCriticality = "medium"
	CriticalityLow             BusinessCriticality = "low"
)

// IsValid reports whether the criticality level is recognized.
func (c BusinessCriticality) IsValid() bool {
	switch c {
	case CriticalityMissionCritical, CriticalityHigh, CriticalityMedium, CriticalityLow:
		return true
	default:
		return false
	}
}

// NodeLifecycleStatus represents the maintenance/decommission status of a node.
type NodeLifecycleStatus string

const (
	LifecycleActive         NodeLifecycleStatus = "active"
	LifecycleMaintenance    NodeLifecycleStatus = "maintenance"
	LifecycleDraining       NodeLifecycleStatus = "draining"
	LifecycleDecommissioned NodeLifecycleStatus = "decommissioned"
)

// IsValid reports whether the lifecycle status is recognized.
func (l NodeLifecycleStatus) IsValid() bool {
	switch l {
	case LifecycleActive, LifecycleMaintenance, LifecycleDraining, LifecycleDecommissioned:
		return true
	default:
		return false
	}
}

// NodeOwnershipMetadata holds organizational ownership and operational context for a node.
type NodeOwnershipMetadata struct {
	OwnerTeam           string              `json:"owner_team,omitempty" yaml:"owner_team,omitempty"`
	ContactEmail        string              `json:"contact_email,omitempty" yaml:"contact_email,omitempty"`
	ContactChannel      string              `json:"contact_channel,omitempty" yaml:"contact_channel,omitempty"`
	Environment         string              `json:"environment,omitempty" yaml:"environment,omitempty"`
	Region              string              `json:"region,omitempty" yaml:"region,omitempty"`
	DataClassification  string              `json:"data_classification,omitempty" yaml:"data_classification,omitempty"`
	CostCenter          string              `json:"cost_center,omitempty" yaml:"cost_center,omitempty"`
	BusinessCriticality BusinessCriticality `json:"business_criticality,omitempty" yaml:"business_criticality,omitempty"`
	Lifecycle           NodeLifecycleStatus `json:"lifecycle,omitempty" yaml:"lifecycle,omitempty"`
	CustomProperties    map[string]string   `json:"custom_properties,omitempty" yaml:"custom_properties,omitempty"`
}

// Validate checks ownership fields and sets sensible defaults.
func (o *NodeOwnershipMetadata) Validate() error {
	if o == nil {
		return nil
	}
	if o.BusinessCriticality != "" && !o.BusinessCriticality.IsValid() {
		return fmt.Errorf("invalid business criticality: %s", o.BusinessCriticality)
	}
	if o.Lifecycle != "" && !o.Lifecycle.IsValid() {
		return fmt.Errorf("invalid node lifecycle: %s", o.Lifecycle)
	}
	if len(o.OwnerTeam) > 255 {
		return fmt.Errorf("owner_team exceeds 255 characters")
	}
	if len(o.CostCenter) > 64 {
		return fmt.Errorf("cost_center exceeds 64 characters")
	}
	return nil
}

// Prefix constants for node ownership metadata serialization.
const (
	MetaPrefixGovernance   = "governance."
	MetaKeyOwnerTeam       = "governance.owner_team"
	MetaKeyContactEmail    = "governance.contact_email"
	MetaKeyContactChannel  = "governance.contact_channel"
	MetaKeyEnvironment     = "governance.environment"
	MetaKeyRegion          = "governance.region"
	MetaKeyClassification  = "governance.data_classification"
	MetaKeyCostCenter      = "governance.cost_center"
	MetaKeyCriticality     = "governance.business_criticality"
	MetaKeyLifecycle       = "governance.lifecycle"
	MetaPrefixCustom       = "governance.custom."
)

// ToMetadata converts the structured ownership attributes into standard string key-values.
func (o *NodeOwnershipMetadata) ToMetadata() map[string]string {
	if o == nil {
		return nil
	}
	m := make(map[string]string)
	if o.OwnerTeam != "" {
		m[MetaKeyOwnerTeam] = o.OwnerTeam
	}
	if o.ContactEmail != "" {
		m[MetaKeyContactEmail] = o.ContactEmail
	}
	if o.ContactChannel != "" {
		m[MetaKeyContactChannel] = o.ContactChannel
	}
	if o.Environment != "" {
		m[MetaKeyEnvironment] = o.Environment
	}
	if o.Region != "" {
		m[MetaKeyRegion] = o.Region
	}
	if o.DataClassification != "" {
		m[MetaKeyClassification] = o.DataClassification
	}
	if o.CostCenter != "" {
		m[MetaKeyCostCenter] = o.CostCenter
	}
	if o.BusinessCriticality != "" {
		m[MetaKeyCriticality] = string(o.BusinessCriticality)
	}
	if o.Lifecycle != "" {
		m[MetaKeyLifecycle] = string(o.Lifecycle)
	}
	for k, v := range o.CustomProperties {
		m[MetaPrefixCustom+k] = v
	}
	return m
}

// MergeIntoMetadata overlays ownership properties into a destination metadata map.
func (o *NodeOwnershipMetadata) MergeIntoMetadata(target map[string]string) map[string]string {
	if target == nil {
		target = make(map[string]string)
	}
	if o == nil {
		return target
	}
	for k, v := range o.ToMetadata() {
		target[k] = v
	}
	return target
}

// NodeOwnershipFromMetadata extracts structured ownership metadata from a generic string map.
// Supports both canonical governance-prefixed keys (e.g. "governance.owner_team") and direct keys (e.g. "owner_team").
func NodeOwnershipFromMetadata(meta map[string]string) *NodeOwnershipMetadata {
	if meta == nil {
		return nil
	}
	res := &NodeOwnershipMetadata{
		OwnerTeam:           meta[MetaKeyOwnerTeam],
		ContactEmail:        meta[MetaKeyContactEmail],
		ContactChannel:      meta[MetaKeyContactChannel],
		Environment:         meta[MetaKeyEnvironment],
		Region:              meta[MetaKeyRegion],
		DataClassification:  meta[MetaKeyClassification],
		CostCenter:          meta[MetaKeyCostCenter],
		BusinessCriticality: BusinessCriticality(meta[MetaKeyCriticality]),
		Lifecycle:           NodeLifecycleStatus(meta[MetaKeyLifecycle]),
		CustomProperties:    make(map[string]string),
	}

	// Fallback to unprefixed keys if canonical keys are empty
	if res.OwnerTeam == "" {
		res.OwnerTeam = meta["owner_team"]
	}
	if res.ContactEmail == "" {
		res.ContactEmail = meta["contact_email"]
	}
	if res.ContactChannel == "" {
		res.ContactChannel = meta["contact_channel"]
	}
	if res.Environment == "" {
		res.Environment = meta["environment"]
	}
	if res.Region == "" {
		res.Region = meta["region"]
	}
	if res.DataClassification == "" {
		res.DataClassification = meta["data_classification"]
	}
	if res.CostCenter == "" {
		res.CostCenter = meta["cost_center"]
	}
	if res.BusinessCriticality == "" {
		res.BusinessCriticality = BusinessCriticality(meta["business_criticality"])
	}
	if res.Lifecycle == "" {
		res.Lifecycle = NodeLifecycleStatus(meta["lifecycle"])
	}

	for k, v := range meta {
		if customKey, ok := strings.CutPrefix(k, MetaPrefixCustom); ok {
			res.CustomProperties[customKey] = v
		}
	}

	// If empty, return nil or basic struct
	if res.OwnerTeam == "" && res.ContactEmail == "" && res.ContactChannel == "" &&
		res.Environment == "" && res.Region == "" && res.DataClassification == "" &&
		res.CostCenter == "" && res.BusinessCriticality == "" && res.Lifecycle == "" &&
		len(res.CustomProperties) == 0 {
		return nil
	}

	return res
}

// FleetGroupHierarchyNode represents a node in a group hierarchy tree.
type FleetGroupHierarchyNode struct {
	Group          FleetGroup                 `json:"group"`
	Children       []*FleetGroupHierarchyNode `json:"children,omitempty"`
	DirectMembers  []string                   `json:"direct_members,omitempty"`
	TotalNodeCount int                        `json:"total_node_count"`
	SubgroupCount  int                        `json:"subgroup_count"`
}
