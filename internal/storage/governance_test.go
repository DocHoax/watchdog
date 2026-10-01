package storage

import (
	"context"
	"testing"
	"time"

	"github.com/DocHoax/watchdog/pkg/model"
)

func TestSQLiteStorage_Organizations_CRUD(t *testing.T) {
	store, err := NewSQLiteStorage(Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Verify default organization is present from migrations
	defOrg, err := store.GetOrganization(ctx, model.DefaultOrganizationID)
	if err != nil {
		t.Fatalf("expected default organization, got %v", err)
	}
	if defOrg.ID != model.DefaultOrganizationID {
		t.Errorf("expected default org id, got %s", defOrg.ID)
	}

	// Cannot delete default organization
	if err := store.DeleteOrganization(ctx, model.DefaultOrganizationID); err == nil {
		t.Errorf("expected error deleting default organization")
	}

	// Create new organization
	now := time.Now().UTC().Truncate(time.Millisecond)
	org1 := &model.Organization{
		ID:          "org-devops",
		Name:        "DevOps Platform",
		DisplayName: "Core Infrastructure",
		Description: "Global DevOps & SRE organization",
		Status:      model.OrgStatusActive,
		CreatedAt:   now,
		UpdatedAt:   now,
		Metadata:    map[string]string{"tier": "enterprise", "contact": "devops@example.com"},
	}

	if err := store.SaveOrganization(ctx, org1); err != nil {
		t.Fatalf("failed to save organization: %v", err)
	}

	// Retrieve
	fetched, err := store.GetOrganization(ctx, "org-devops")
	if err != nil {
		t.Fatalf("failed to get organization: %v", err)
	}
	if fetched.Name != "DevOps Platform" || fetched.DisplayName != "Core Infrastructure" {
		t.Errorf("mismatched organization fields: %+v", fetched)
	}
	if fetched.Metadata["tier"] != "enterprise" {
		t.Errorf("expected tier=enterprise in metadata, got %v", fetched.Metadata)
	}

	// Update organization
	fetched.DisplayName = "Core Platform Services"
	fetched.Status = model.OrgStatusSuspended
	if err := store.SaveOrganization(ctx, fetched); err != nil {
		t.Fatalf("failed to update organization: %v", err)
	}

	updated, err := store.GetOrganization(ctx, "org-devops")
	if err != nil {
		t.Fatalf("failed to get updated org: %v", err)
	}
	if updated.DisplayName != "Core Platform Services" || updated.Status != model.OrgStatusSuspended {
		t.Errorf("update not reflected: %+v", updated)
	}

	// List organizations
	orgs, err := store.ListOrganizations(ctx)
	if err != nil {
		t.Fatalf("failed to list organizations: %v", err)
	}
	if len(orgs) != 2 {
		t.Fatalf("expected 2 organizations, got %d", len(orgs))
	}

	// Delete organization
	if err := store.DeleteOrganization(ctx, "org-devops"); err != nil {
		t.Fatalf("failed to delete organization: %v", err)
	}

	if _, err := store.GetOrganization(ctx, "org-devops"); err == nil {
		t.Errorf("expected not found error after deletion")
	}

	// Delete non-existent organization
	if err := store.DeleteOrganization(ctx, "org-nonexistent"); err == nil {
		t.Errorf("expected error deleting non-existent organization")
	}
}

func TestSQLiteStorage_FleetGroups_CRUD(t *testing.T) {
	store, err := NewSQLiteStorage(Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Millisecond)

	rootGroup := &model.FleetGroup{
		ID:          "grp-prod",
		OrgID:       model.DefaultOrganizationID,
		Name:        "Production",
		DisplayName: "Production Fleet",
		Description: "All production workloads",
		Type:        model.GroupTypeEnvironment,
		Path:        "/grp-prod",
		CreatedAt:   now,
		UpdatedAt:   now,
		Metadata:    map[string]string{"sla": "99.99"},
	}

	if err := store.SaveFleetGroup(ctx, rootGroup); err != nil {
		t.Fatalf("failed to save root group: %v", err)
	}

	subGroup := &model.FleetGroup{
		ID:            "grp-prod-us-east",
		OrgID:         model.DefaultOrganizationID,
		ParentGroupID: "grp-prod",
		Name:          "Prod US East",
		DisplayName:   "Production - US East",
		Type:          model.GroupTypeRegion,
		Path:          "/grp-prod/grp-prod-us-east",
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := store.SaveFleetGroup(ctx, subGroup); err != nil {
		t.Fatalf("failed to save subgroup: %v", err)
	}

	// Get group
	g, err := store.GetFleetGroup(ctx, "grp-prod-us-east")
	if err != nil {
		t.Fatalf("failed to get subgroup: %v", err)
	}
	if g.ParentGroupID != "grp-prod" || g.Type != model.GroupTypeRegion {
		t.Errorf("unexpected group properties: %+v", g)
	}

	// List groups by org
	groups, err := store.ListFleetGroups(ctx, model.DefaultOrganizationID)
	if err != nil {
		t.Fatalf("failed to list fleet groups: %v", err)
	}
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups for default org, got %d", len(groups))
	}

	// Delete root group and verify child parent is detached (parent_group_id set to NULL)
	if err := store.DeleteFleetGroup(ctx, "grp-prod"); err != nil {
		t.Fatalf("failed to delete root group: %v", err)
	}

	child, err := store.GetFleetGroup(ctx, "grp-prod-us-east")
	if err != nil {
		t.Fatalf("failed to get child group: %v", err)
	}
	if child.ParentGroupID != "" {
		t.Errorf("expected parent_group_id to be empty after parent deletion, got %q", child.ParentGroupID)
	}
}

func TestSQLiteStorage_FleetGroupMembers_CRUD(t *testing.T) {
	store, err := NewSQLiteStorage(Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	group := &model.FleetGroup{
		ID:   "grp-analytics",
		Name: "Analytics Cluster",
		Type: model.GroupTypeApplication,
	}
	if err := store.SaveFleetGroup(ctx, group); err != nil {
		t.Fatalf("failed to save group: %v", err)
	}

	// Add members
	m1 := &model.FleetGroupMember{
		GroupID:  "grp-analytics",
		NodeID:   "node-001",
		Role:     model.MembershipRolePrimary,
		AddedBy:  "admin",
		Metadata: map[string]string{"weight": "100"},
	}
	if err := store.AddGroupMember(ctx, m1); err != nil {
		t.Fatalf("failed to add member: %v", err)
	}

	m2 := &model.FleetGroupMember{
		GroupID: "grp-analytics",
		NodeID:  "node-002",
		Role:    model.MembershipRoleSecondary,
	}
	if err := store.AddGroupMember(ctx, m2); err != nil {
		t.Fatalf("failed to add member 2: %v", err)
	}

	// Get members
	members, err := store.GetGroupMembers(ctx, "grp-analytics")
	if err != nil {
		t.Fatalf("failed to get group members: %v", err)
	}
	if len(members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(members))
	}
	if members[0].NodeID != "node-001" || members[0].Metadata["weight"] != "100" {
		t.Errorf("unexpected member 0: %+v", members[0])
	}

	// GetNodeGroups
	nodeGroups, err := store.GetNodeGroups(ctx, "node-001")
	if err != nil {
		t.Fatalf("failed to get node groups: %v", err)
	}
	if len(nodeGroups) != 1 || nodeGroups[0].ID != "grp-analytics" {
		t.Errorf("unexpected node groups: %+v", nodeGroups)
	}

	// Remove member
	if err := store.RemoveGroupMember(ctx, "grp-analytics", "node-002"); err != nil {
		t.Fatalf("failed to remove member: %v", err)
	}
	membersAfterRemove, err := store.GetGroupMembers(ctx, "grp-analytics")
	if err != nil {
		t.Fatalf("failed to get members: %v", err)
	}
	if len(membersAfterRemove) != 1 {
		t.Fatalf("expected 1 member after removal, got %d", len(membersAfterRemove))
	}

	// SetGroupMembers bulk replacement
	if err := store.SetGroupMembers(ctx, "grp-analytics", []string{"node-010", "node-020", "node-030"}); err != nil {
		t.Fatalf("failed to set group members: %v", err)
	}

	bulkMembers, err := store.GetGroupMembers(ctx, "grp-analytics")
	if err != nil {
		t.Fatalf("failed to get group members after bulk set: %v", err)
	}
	if len(bulkMembers) != 3 {
		t.Fatalf("expected 3 members, got %d", len(bulkMembers))
	}
}
