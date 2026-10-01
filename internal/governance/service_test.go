package governance

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DocHoax/watchdog/internal/storage"
	"github.com/DocHoax/watchdog/pkg/model"
)

func setupTestService(t *testing.T) (GovernanceService, storage.Storage) {
	t.Helper()
	store, err := storage.NewSQLiteStorage(storage.Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	svc := NewGovernanceService(store)
	return svc, store
}

func TestGovernanceService_Organization_Lifecycle(t *testing.T) {
	svc, store := setupTestService(t)
	defer store.Close()

	ctx := context.Background()

	// 1. Get default organization (seeded by storage)
	defOrg, err := svc.GetOrganization(ctx, model.DefaultOrganizationID)
	if err != nil {
		t.Fatalf("failed to get default organization: %v", err)
	}
	if defOrg.ID != model.DefaultOrganizationID {
		t.Errorf("expected default org id %s, got %s", model.DefaultOrganizationID, defOrg.ID)
	}

	// 2. Prevent deleting default organization
	if err := svc.DeleteOrganization(ctx, model.DefaultOrganizationID); !errors.Is(err, ErrDefaultOrgImmutable) {
		t.Errorf("expected ErrDefaultOrgImmutable, got %v", err)
	}

	// 3. Create custom organization
	org := &model.Organization{
		ID:          "org-enterprise",
		Name:        "Enterprise Cloud",
		DisplayName: "Enterprise Cloud Division",
		Description: "Multi-region cloud infrastructure",
		Status:      model.OrgStatusActive,
	}
	if err := svc.CreateOrganization(ctx, org); err != nil {
		t.Fatalf("failed to create organization: %v", err)
	}

	// Duplicate creation fails
	if err := svc.CreateOrganization(ctx, org); !errors.Is(err, ErrOrganizationExists) {
		t.Errorf("expected ErrOrganizationExists, got %v", err)
	}

	// 4. Update organization
	org.DisplayName = "Enterprise Global Cloud Division"
	if err := svc.UpdateOrganization(ctx, org); err != nil {
		t.Fatalf("failed to update organization: %v", err)
	}

	fetched, err := svc.GetOrganization(ctx, "org-enterprise")
	if err != nil {
		t.Fatalf("failed to get updated organization: %v", err)
	}
	if fetched.DisplayName != "Enterprise Global Cloud Division" {
		t.Errorf("expected updated display name, got %s", fetched.DisplayName)
	}

	// 5. List organizations
	orgs, err := svc.ListOrganizations(ctx)
	if err != nil {
		t.Fatalf("failed to list organizations: %v", err)
	}
	if len(orgs) < 2 { // default + org-enterprise
		t.Errorf("expected at least 2 organizations, got %d", len(orgs))
	}

	// 6. Delete custom organization
	if err := svc.DeleteOrganization(ctx, "org-enterprise"); err != nil {
		t.Fatalf("failed to delete organization: %v", err)
	}

	if _, err := svc.GetOrganization(ctx, "org-enterprise"); !errors.Is(err, ErrOrganizationNotFound) {
		t.Errorf("expected ErrOrganizationNotFound, got %v", err)
	}
}

func TestGovernanceService_FleetGroup_Hierarchy_And_CyclePrevention(t *testing.T) {
	svc, store := setupTestService(t)
	defer store.Close()

	ctx := context.Background()

	// 1. Create root group
	rootGroup := &model.FleetGroup{
		ID:    "grp-root",
		OrgID: model.DefaultOrganizationID,
		Name:  "Global Infrastructure",
		Type:  model.GroupTypeDepartment,
	}
	if err := svc.CreateFleetGroup(ctx, rootGroup); err != nil {
		t.Fatalf("failed to create root group: %v", err)
	}

	// 2. Create child group
	childGroup := &model.FleetGroup{
		ID:            "grp-prod",
		OrgID:         model.DefaultOrganizationID,
		ParentGroupID: "grp-root",
		Name:          "Production Tier",
		Type:          model.GroupTypeEnvironment,
	}
	if err := svc.CreateFleetGroup(ctx, childGroup); err != nil {
		t.Fatalf("failed to create child group: %v", err)
	}

	// 3. Create grandchild group
	leafGroup := &model.FleetGroup{
		ID:            "grp-us-east",
		OrgID:         model.DefaultOrganizationID,
		ParentGroupID: "grp-prod",
		Name:          "US East Region",
		Type:          model.GroupTypeRegion,
	}
	if err := svc.CreateFleetGroup(ctx, leafGroup); err != nil {
		t.Fatalf("failed to create leaf group: %v", err)
	}

	// Verify paths
	gLeaf, err := svc.GetFleetGroup(ctx, "grp-us-east")
	if err != nil {
		t.Fatalf("failed to get leaf group: %v", err)
	}
	if gLeaf.Path != "/grp-root/grp-prod/grp-us-east" {
		t.Errorf("expected path /grp-root/grp-prod/grp-us-east, got %s", gLeaf.Path)
	}

	// 4. Test cycle prevention during creation
	cycleGroup := &model.FleetGroup{
		ID:            "grp-cycle",
		OrgID:         model.DefaultOrganizationID,
		ParentGroupID: "grp-us-east",
		Name:          "Cycle Group",
		Type:          model.GroupTypeCustom,
	}
	if err := svc.CreateFleetGroup(ctx, cycleGroup); err != nil {
		t.Fatalf("failed to create cycle candidate: %v", err)
	}

	// Now try to update root group to have grp-cycle as parent (root -> prod -> us-east -> cycle -> root)
	rootGroup.ParentGroupID = "grp-cycle"
	if err := svc.UpdateFleetGroup(ctx, rootGroup); !errors.Is(err, ErrCycleDetected) {
		t.Errorf("expected ErrCycleDetected when creating cycle, got %v", err)
	}

	// 5. Test cross-org parent rejection
	// Create another organization
	secOrg := &model.Organization{
		ID:   "org-secondary",
		Name: "Secondary Org",
	}
	if err := svc.CreateOrganization(ctx, secOrg); err != nil {
		t.Fatalf("failed to create secondary org: %v", err)
	}

	crossOrgGroup := &model.FleetGroup{
		ID:            "grp-cross",
		OrgID:         "org-secondary",
		ParentGroupID: "grp-root", // grp-root is in default org
		Name:          "Cross Org Group",
	}
	if err := svc.CreateFleetGroup(ctx, crossOrgGroup); !errors.Is(err, ErrCrossOrgParent) {
		t.Errorf("expected ErrCrossOrgParent, got %v", err)
	}
}

func TestGovernanceService_Membership_And_Subtree(t *testing.T) {
	svc, store := setupTestService(t)
	defer store.Close()

	ctx := context.Background()

	// Create group hierarchy
	_ = svc.CreateFleetGroup(ctx, &model.FleetGroup{
		ID:    "grp-all",
		OrgID: model.DefaultOrganizationID,
		Name:  "All Nodes",
		Type:  model.GroupTypeCustom,
	})
	_ = svc.CreateFleetGroup(ctx, &model.FleetGroup{
		ID:            "grp-eu",
		OrgID:         model.DefaultOrganizationID,
		ParentGroupID: "grp-all",
		Name:          "EU Region",
		Type:          model.GroupTypeRegion,
	})

	// Add members
	m1 := &model.FleetGroupMember{
		GroupID: "grp-all",
		NodeID:  "node-global-01",
		Role:    model.MembershipRolePrimary,
		AddedAt: time.Now().UTC(),
	}
	if err := svc.AddMember(ctx, m1); err != nil {
		t.Fatalf("failed to add member: %v", err)
	}

	// Atomic SetMembers on grp-eu
	if err := svc.SetMembers(ctx, "grp-eu", []string{"node-eu-01", "node-eu-02"}); err != nil {
		t.Fatalf("failed to set members: %v", err)
	}

	// Fetch members for grp-eu
	members, err := svc.GetGroupMembers(ctx, "grp-eu")
	if err != nil {
		t.Fatalf("failed to get members: %v", err)
	}
	if len(members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(members))
	}

	// Check node groups
	nodeGroups, err := svc.GetNodeGroups(ctx, "node-eu-01")
	if err != nil {
		t.Fatalf("failed to get node groups: %v", err)
	}
	if len(nodeGroups) != 1 || nodeGroups[0].ID != "grp-eu" {
		t.Errorf("expected node to belong to grp-eu, got %+v", nodeGroups)
	}

	// Check subtree nodes for grp-all (should include node-global-01, node-eu-01, node-eu-02)
	subtreeNodes, err := svc.GetSubtreeNodes(ctx, "grp-all")
	if err != nil {
		t.Fatalf("failed to get subtree nodes: %v", err)
	}
	if len(subtreeNodes) != 3 {
		t.Errorf("expected 3 subtree nodes under grp-all, got %d (%v)", len(subtreeNodes), subtreeNodes)
	}

	// Remove member
	if err := svc.RemoveMember(ctx, "grp-all", "node-global-01"); err != nil {
		t.Fatalf("failed to remove member: %v", err)
	}
	remainingSubtree, _ := svc.GetSubtreeNodes(ctx, "grp-all")
	if len(remainingSubtree) != 2 {
		t.Errorf("expected 2 subtree nodes after removal, got %d", len(remainingSubtree))
	}
}

func TestGovernanceService_NodeOwnershipMetadata(t *testing.T) {
	svc, store := setupTestService(t)
	defer store.Close()

	ctx := context.Background()

	// Seed a fleet node
	node := &model.FleetNode{
		Identity: model.NodeIdentity{
			NodeID:   "node-srv-01",
			Hostname: "srv-01.company.internal",
		},
		Status:       model.NodeStatusHealthy,
		RegisteredAt: time.Now().UTC(),
		Metadata: map[string]string{
			"env": "production",
		},
	}
	if err := store.SaveFleetNode(ctx, node); err != nil {
		t.Fatalf("failed to save fleet node: %v", err)
	}

	// 1. Get initial ownership (should be nil or empty)
	initialMeta, err := svc.GetNodeOwnership(ctx, "node-srv-01")
	if err != nil {
		t.Fatalf("failed to get initial node ownership: %v", err)
	}
	if initialMeta != nil {
		t.Errorf("expected nil initial ownership, got %+v", initialMeta)
	}

	// 2. Set ownership metadata
	meta := &model.NodeOwnershipMetadata{
		OwnerTeam:           "Payments Core",
		ContactEmail:        "payments-core@company.com",
		ContactChannel:      "#payments-alerts",
		Environment:         "production",
		Region:              "us-east-1",
		DataClassification:  "pci-dss",
		CostCenter:          "CC-8800",
		BusinessCriticality: model.CriticalityMissionCritical,
		Lifecycle:           model.LifecycleActive,
		CustomProperties: map[string]string{
			"cluster": "payment-prod-01",
		},
	}

	if err := svc.SetNodeOwnership(ctx, "node-srv-01", meta); err != nil {
		t.Fatalf("failed to set node ownership: %v", err)
	}

	// 3. Get node ownership
	retrievedMeta, err := svc.GetNodeOwnership(ctx, "node-srv-01")
	if err != nil {
		t.Fatalf("failed to get updated node ownership: %v", err)
	}
	if retrievedMeta == nil {
		t.Fatalf("expected non-nil node ownership")
	}
	if retrievedMeta.OwnerTeam != "Payments Core" ||
		retrievedMeta.BusinessCriticality != model.CriticalityMissionCritical ||
		retrievedMeta.Lifecycle != model.LifecycleActive ||
		retrievedMeta.CustomProperties["cluster"] != "payment-prod-01" {
		t.Errorf("retrieved ownership does not match saved: %+v", retrievedMeta)
	}

	// Verify underlying node preserved non-governance metadata ("env": "production")
	updatedNode, _ := store.GetFleetNode(ctx, "node-srv-01")
	if updatedNode.Metadata["env"] != "production" {
		t.Errorf("expected original 'env' metadata to be preserved, got %+v", updatedNode.Metadata)
	}

	// 4. Test clear ownership by passing nil
	if err := svc.SetNodeOwnership(ctx, "node-srv-01", nil); err != nil {
		t.Fatalf("failed to clear node ownership: %v", err)
	}
	clearedMeta, _ := svc.GetNodeOwnership(ctx, "node-srv-01")
	if clearedMeta != nil {
		t.Errorf("expected nil metadata after clearing, got %+v", clearedMeta)
	}
	nodeAfterClear, _ := store.GetFleetNode(ctx, "node-srv-01")
	if nodeAfterClear.Metadata["env"] != "production" {
		t.Errorf("expected original non-governance metadata to remain intact, got %+v", nodeAfterClear.Metadata)
	}
}

func TestGovernanceService_GetHierarchy(t *testing.T) {
	svc, store := setupTestService(t)
	defer store.Close()

	ctx := context.Background()

	_ = svc.CreateFleetGroup(ctx, &model.FleetGroup{
		ID:    "grp-corp",
		OrgID: model.DefaultOrganizationID,
		Name:  "Corporate Network",
		Type:  model.GroupTypeDepartment,
	})
	_ = svc.CreateFleetGroup(ctx, &model.FleetGroup{
		ID:            "grp-finance",
		OrgID:         model.DefaultOrganizationID,
		ParentGroupID: "grp-corp",
		Name:          "Finance Subnet",
		Type:          model.GroupTypeDepartment,
	})

	_ = svc.AddMember(ctx, &model.FleetGroupMember{
		GroupID: "grp-corp",
		NodeID:  "node-corp-1",
		Role:    model.MembershipRolePrimary,
	})
	_ = svc.AddMember(ctx, &model.FleetGroupMember{
		GroupID: "grp-finance",
		NodeID:  "node-fin-1",
		Role:    model.MembershipRolePrimary,
	})

	hierarchy, err := svc.GetHierarchy(ctx, model.DefaultOrganizationID)
	if err != nil {
		t.Fatalf("failed to get hierarchy: %v", err)
	}

	if len(hierarchy) != 1 {
		t.Fatalf("expected 1 root hierarchy node, got %d", len(hierarchy))
	}

	rootNode := hierarchy[0]
	if rootNode.Group.ID != "grp-corp" {
		t.Errorf("expected root group grp-corp, got %s", rootNode.Group.ID)
	}
	if rootNode.SubgroupCount != 1 {
		t.Errorf("expected 1 subgroup under grp-corp, got %d", rootNode.SubgroupCount)
	}
	if rootNode.TotalNodeCount != 2 {
		t.Errorf("expected 2 total nodes under grp-corp, got %d", rootNode.TotalNodeCount)
	}
}
