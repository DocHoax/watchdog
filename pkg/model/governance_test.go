package model

import (
	"testing"
	"time"
)

func TestValidateIdentifier(t *testing.T) {
	validIDs := []string{
		"default",
		"org-1",
		"group_prod.us-east",
		"node-123.abc",
		"A123_456-789",
	}
	for _, id := range validIDs {
		if err := ValidateIdentifier(id); err != nil {
			t.Errorf("expected valid identifier for %s, got %v", id, err)
		}
	}

	invalidIDs := []string{
		"",
		"has space",
		"special$char",
		"slash/path",
		"colon:id",
		string(make([]byte, 129)), // > 128 chars
	}
	for _, id := range invalidIDs {
		if err := ValidateIdentifier(id); err == nil {
			t.Errorf("expected invalid identifier error for %q, got nil", id)
		}
	}
}

func TestOrganization_Validate(t *testing.T) {
	now := time.Now()

	org := &Organization{
		ID:          "org-devops",
		Name:        "DevOps Platform",
		DisplayName: "Platform Operations",
		Description: "Core platform and infrastructure services",
		Status:      OrgStatusActive,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := org.Validate(); err != nil {
		t.Fatalf("expected valid organization, got %v", err)
	}

	// Test default status assignment
	orgNoStatus := &Organization{
		ID:   "org-default",
		Name: "Default Organization",
	}
	if err := orgNoStatus.Validate(); err != nil {
		t.Fatalf("expected valid organization without explicit status, got %v", err)
	}
	if orgNoStatus.Status != OrgStatusActive {
		t.Errorf("expected default status %s, got %s", OrgStatusActive, orgNoStatus.Status)
	}

	// Test invalid cases
	invalidOrg := &Organization{
		ID:   "bad id with spaces",
		Name: "DevOps",
	}
	if err := invalidOrg.Validate(); err == nil {
		t.Errorf("expected validation error for invalid ID")
	}

	emptyNameOrg := &Organization{
		ID:   "org-valid",
		Name: "   ",
	}
	if err := emptyNameOrg.Validate(); err == nil {
		t.Errorf("expected validation error for empty name")
	}

	invalidStatusOrg := &Organization{
		ID:     "org-valid",
		Name:   "Org",
		Status: OrganizationStatus("unknown_status"),
	}
	if err := invalidStatusOrg.Validate(); err == nil {
		t.Errorf("expected validation error for invalid status")
	}
}

func TestFleetGroup_Validate(t *testing.T) {
	now := time.Now()

	group := &FleetGroup{
		ID:            "grp-us-east",
		OrgID:         "org-platform",
		ParentGroupID: "grp-prod",
		Name:          "US East Region",
		Type:          GroupTypeRegion,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := group.Validate(); err != nil {
		t.Fatalf("expected valid group, got %v", err)
	}

	// Test self-parent rejection
	selfParentGroup := &FleetGroup{
		ID:            "grp-self",
		OrgID:         "org-platform",
		ParentGroupID: "grp-self",
		Name:          "Self Parent",
		Type:          GroupTypeCustom,
	}
	if err := selfParentGroup.Validate(); err == nil {
		t.Errorf("expected error for self-parent group")
	}

	// Test default organization assignment
	noOrgGroup := &FleetGroup{
		ID:   "grp-default-org",
		Name: "No Org Group",
	}
	if err := noOrgGroup.Validate(); err != nil {
		t.Fatalf("expected validation success with default org, got %v", err)
	}
	if noOrgGroup.OrgID != DefaultOrganizationID {
		t.Errorf("expected default org ID %s, got %s", DefaultOrganizationID, noOrgGroup.OrgID)
	}
	if noOrgGroup.Type != GroupTypeCustom {
		t.Errorf("expected default type %s, got %s", GroupTypeCustom, noOrgGroup.Type)
	}
}

func TestFleetGroupMember_Validate(t *testing.T) {
	member := &FleetGroupMember{
		GroupID: "grp-prod",
		NodeID:  "node-01",
		AddedAt: time.Now(),
		Role:    MembershipRolePrimary,
	}
	if err := member.Validate(); err != nil {
		t.Fatalf("expected valid member, got %v", err)
	}

	memberDefaultRole := &FleetGroupMember{
		GroupID: "grp-prod",
		NodeID:  "node-02",
	}
	if err := memberDefaultRole.Validate(); err != nil {
		t.Fatalf("expected valid member with default role, got %v", err)
	}
	if memberDefaultRole.Role != MembershipRolePrimary {
		t.Errorf("expected default role %s, got %s", MembershipRolePrimary, memberDefaultRole.Role)
	}

	invalidMember := &FleetGroupMember{
		GroupID: "",
		NodeID:  "node-01",
	}
	if err := invalidMember.Validate(); err == nil {
		t.Errorf("expected error on empty group ID")
	}
}

func TestNodeOwnershipMetadata_Validate(t *testing.T) {
	meta := &NodeOwnershipMetadata{
		OwnerTeam:           "Site Reliability Engineering",
		ContactEmail:        "sre@company.internal",
		ContactChannel:      "#sre-alerts",
		Environment:         "production",
		Region:              "us-east-1",
		DataClassification:  "confidential",
		CostCenter:          "CC-4091",
		BusinessCriticality: CriticalityMissionCritical,
		Lifecycle:           LifecycleActive,
	}
	if err := meta.Validate(); err != nil {
		t.Fatalf("expected valid ownership metadata, got %v", err)
	}

	invalidMeta := &NodeOwnershipMetadata{
		BusinessCriticality: BusinessCriticality("invalid_crit"),
	}
	if err := invalidMeta.Validate(); err == nil {
		t.Errorf("expected error on invalid business criticality")
	}

	invalidLifecycle := &NodeOwnershipMetadata{
		Lifecycle: NodeLifecycleStatus("invalid_life"),
	}
	if err := invalidLifecycle.Validate(); err == nil {
		t.Errorf("expected error on invalid lifecycle status")
	}
}
