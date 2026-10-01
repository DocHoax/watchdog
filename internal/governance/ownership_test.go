package governance

import (
	"context"
	"testing"
	"time"

	"github.com/DocHoax/watchdog/internal/storage"
	"github.com/DocHoax/watchdog/pkg/model"
)

func setupTestOwnershipStore(t *testing.T) storage.Storage {
	t.Helper()
	store, err := storage.NewSQLiteStorage(storage.Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("failed to initialize sqlite storage: %v", err)
	}
	return store
}

func TestOwnershipResolver_HierarchicalCascading(t *testing.T) {
	ctx := context.Background()
	store := setupTestOwnershipStore(t)
	defer store.Close()

	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	resolver := NewOwnershipResolver(store, NewMockClock(now))

	// 1. Setup Organization with base defaults
	org := &model.Organization{
		ID:     "org-acme",
		Name:   "Acme Corp",
		Status: model.OrgStatusActive,
		Metadata: map[string]string{
			model.MetaKeyOwnerTeam:       "infra-core",
			model.MetaKeyContactEmail:    "infra-core@acme.corp",
			model.MetaKeyCostCenter:      "CC-001",
			model.MetaKeyCriticality:     string(model.CriticalityLow),
			model.MetaPrefixCustom + "tier": "standard",
		},
	}
	if err := store.SaveOrganization(ctx, org); err != nil {
		t.Fatalf("failed to save org: %v", err)
	}

	// 2. Setup Parent Group (Root): overrides CostCenter and sets Environment
	parentGroup := &model.FleetGroup{
		ID:        "grp-root",
		OrgID:     "org-acme",
		Name:      "Production Tier",
		Type:      model.GroupTypeTier,
		Path:      "/grp-root",
		Metadata: map[string]string{
			model.MetaKeyCostCenter:          "CC-PROD",
			model.MetaKeyEnvironment:         "production",
			model.MetaPrefixCustom + "tier":  "enterprise", // overrides org tier
			model.MetaPrefixCustom + "sla":   "99.99",
		},
	}
	if err := store.SaveFleetGroup(ctx, parentGroup); err != nil {
		t.Fatalf("failed to save parent group: %v", err)
	}

	// 3. Setup Child Group (Leaf): overrides OwnerTeam and ContactChannel
	childGroup := &model.FleetGroup{
		ID:            "grp-leaf",
		OrgID:         "org-acme",
		ParentGroupID: "grp-root",
		Name:          "Payment Processing",
		Type:          model.GroupTypeApplication,
		Path:          "/grp-root/grp-leaf",
		Metadata: map[string]string{
			model.MetaKeyOwnerTeam:       "payments-team",
			model.MetaKeyContactChannel:  "#payments-ops",
			model.MetaKeyCriticality:     string(model.CriticalityMissionCritical),
		},
	}
	if err := store.SaveFleetGroup(ctx, childGroup); err != nil {
		t.Fatalf("failed to save child group: %v", err)
	}

	// 4. Setup Node with specific Node-level override (Region and Custom Property)
	node := &model.FleetNode{
		Identity: model.NodeIdentity{
			NodeID:   "node-pay-01",
			Hostname: "pay-worker-01.us-east.acme.corp",
		},
		Metadata: map[string]string{
			"org_id":                   "org-acme",
			model.MetaKeyRegion:              "us-east-1",
			model.MetaPrefixCustom + "rack": "rack-42",
		},
	}
	if err := store.SaveFleetNode(ctx, node); err != nil {
		t.Fatalf("failed to save fleet node: %v", err)
	}
	if err := store.AddGroupMember(ctx, &model.FleetGroupMember{
		GroupID: childGroup.ID,
		NodeID:  node.Identity.NodeID,
		Role:    model.MembershipRolePrimary,
	}); err != nil {
		t.Fatalf("failed to add node to group: %v", err)
	}

	// Resolve ownership
	resolved, err := resolver.ResolveNodeOwnership(ctx, "node-pay-01")
	if err != nil {
		t.Fatalf("unexpected error resolving ownership: %v", err)
	}

	if resolved == nil {
		t.Fatalf("expected non-nil resolved ownership")
	}

	// Verify cascading resolution:
	// OwnerTeam: resolved from leaf group "payments-team"
	if resolved.OwnerTeam != "payments-team" {
		t.Errorf("expected OwnerTeam 'payments-team', got %q", resolved.OwnerTeam)
	}
	if resolved.SourceLevel != model.OwnershipLevelFleetGroup || resolved.SourceID != "grp-leaf" {
		t.Errorf("expected SourceLevel=fleet_group (grp-leaf), got %s (%s)", resolved.SourceLevel, resolved.SourceID)
	}

	// ContactEmail: fell back to Org "infra-core@acme.corp"
	if resolved.ContactEmail != "infra-core@acme.corp" {
		t.Errorf("expected ContactEmail 'infra-core@acme.corp', got %q", resolved.ContactEmail)
	}

	// ContactChannel: from leaf group "#payments-ops"
	if resolved.ContactChannel != "#payments-ops" {
		t.Errorf("expected ContactChannel '#payments-ops', got %q", resolved.ContactChannel)
	}

	// Environment: from parent group "production"
	if resolved.Environment != "production" {
		t.Errorf("expected Environment 'production', got %q", resolved.Environment)
	}

	// CostCenter: from parent group "CC-PROD" (overrode Org's "CC-001")
	if resolved.CostCenter != "CC-PROD" {
		t.Errorf("expected CostCenter 'CC-PROD', got %q", resolved.CostCenter)
	}

	// Region: from direct node metadata "us-east-1"
	if resolved.Region != "us-east-1" {
		t.Errorf("expected Region 'us-east-1', got %q", resolved.Region)
	}

	// BusinessCriticality: from leaf group "mission_critical" (overrode Org's "low")
	if resolved.BusinessCriticality != model.CriticalityMissionCritical {
		t.Errorf("expected BusinessCriticality 'mission_critical', got %q", resolved.BusinessCriticality)
	}

	// HierarchyPath: from leaf group path
	if resolved.HierarchyPath != "/grp-root/grp-leaf" {
		t.Errorf("expected HierarchyPath '/grp-root/grp-leaf', got %q", resolved.HierarchyPath)
	}

	// Custom properties merged:
	// rack: from node ("rack-42")
	// sla: from parent group ("99.99")
	// tier: from parent group ("enterprise", overrode org's "standard")
	if resolved.CustomProperties["rack"] != "rack-42" {
		t.Errorf("expected custom rack 'rack-42', got %q", resolved.CustomProperties["rack"])
	}
	if resolved.CustomProperties["sla"] != "99.99" {
		t.Errorf("expected custom sla '99.99', got %q", resolved.CustomProperties["sla"])
	}
	if resolved.CustomProperties["tier"] != "enterprise" {
		t.Errorf("expected custom tier 'enterprise', got %q", resolved.CustomProperties["tier"])
	}

	// ResolvedAt
	if !resolved.ResolvedAt.Equal(now) {
		t.Errorf("expected ResolvedAt %v, got %v", now, resolved.ResolvedAt)
	}
}

func TestOwnershipResolver_DirectNodeOverrides(t *testing.T) {
	ctx := context.Background()
	store := setupTestOwnershipStore(t)
	defer store.Close()

	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	resolver := NewOwnershipResolver(store, NewMockClock(now))

	// Node with full explicit ownership
	node := &model.FleetNode{
		Identity: model.NodeIdentity{
			NodeID:   "node-standalone",
			Hostname: "standalone.corp",
		},
		Metadata: map[string]string{
			"org_id":                      "org-standalone",
			model.MetaKeyOwnerTeam:       "secops",
			model.MetaKeyContactEmail:    "secops@acme.corp",
			model.MetaKeyContactChannel:  "#secops-incidents",
			model.MetaKeyEnvironment:     "staging",
			model.MetaKeyRegion:          "eu-central-1",
			model.MetaKeyCostCenter:      "CC-SEC",
			model.MetaKeyCriticality:     string(model.CriticalityHigh),
			model.MetaKeyLifecycle:       string(model.LifecycleMaintenance),
		},
	}
	if err := store.SaveFleetNode(ctx, node); err != nil {
		t.Fatalf("failed to save fleet node: %v", err)
	}

	resolved, err := resolver.ResolveNodeOwnership(ctx, "node-standalone")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resolved.OwnerTeam != "secops" {
		t.Errorf("expected OwnerTeam 'secops', got %q", resolved.OwnerTeam)
	}
	if resolved.SourceLevel != model.OwnershipLevelNode || resolved.SourceID != "node-standalone" {
		t.Errorf("expected SourceLevel=node (node-standalone), got %s (%s)", resolved.SourceLevel, resolved.SourceID)
	}
	if resolved.Lifecycle != model.LifecycleMaintenance {
		t.Errorf("expected Lifecycle 'maintenance', got %q", resolved.Lifecycle)
	}
	if resolved.BusinessCriticality != model.CriticalityHigh {
		t.Errorf("expected Criticality 'high', got %q", resolved.BusinessCriticality)
	}
}

func TestOwnershipResolver_ValidationAndErrors(t *testing.T) {
	ctx := context.Background()
	store := setupTestOwnershipStore(t)
	defer store.Close()

	resolver := NewOwnershipResolver(store, nil)

	// Empty node ID
	_, err := resolver.ResolveNodeOwnership(ctx, "")
	if err == nil {
		t.Errorf("expected error for empty node id, got nil")
	}

	// Non-existent node
	_, err = resolver.ResolveNodeOwnership(ctx, "node-non-existent")
	if err == nil {
		t.Errorf("expected ErrNodeNotFound for non-existent node, got nil")
	}

	// Nil node in ResolveOwnershipForNode
	_, err = resolver.ResolveOwnershipForNode(ctx, nil)
	if err == nil {
		t.Errorf("expected error for nil node, got nil")
	}
}
