package storage

import (
	"context"
	"testing"
	"time"

	"github.com/DocHoax/watchdog/pkg/model"
)

func TestPolicyStorage_CRUD(t *testing.T) {
	store, err := NewSQLiteStorage(Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("failed to init in-memory sqlite: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// 1. Create a policy
	pol := &model.Policy{
		ID:             "pol-1",
		OrgID:          "default",
		Name:           "cpu-memory-limits",
		DisplayName:    "CPU & Memory Limits",
		Description:    "Enforce host thresholds",
		Category:       model.PolicyCategoryResourceThresholds,
		Status:         model.PolicyStatusDraft,
		ActiveRevision: 0,
		Metadata: map[string]string{
			"env": "production",
		},
	}

	if err := store.SavePolicy(ctx, pol); err != nil {
		t.Fatalf("SavePolicy failed: %v", err)
	}

	// 2. Read back policy
	fetched, err := store.GetPolicy(ctx, "pol-1")
	if err != nil {
		t.Fatalf("GetPolicy failed: %v", err)
	}
	if fetched.Name != pol.Name || fetched.Category != pol.Category || fetched.Status != pol.Status {
		t.Errorf("GetPolicy mismatch: got %+v, want %+v", fetched, pol)
	}
	if fetched.Metadata["env"] != "production" {
		t.Errorf("GetPolicy metadata mismatch: got %v", fetched.Metadata)
	}

	// 3. Update policy status and active revision
	pol.Status = model.PolicyStatusActive
	pol.ActiveRevision = 1
	if err := store.SavePolicy(ctx, pol); err != nil {
		t.Fatalf("SavePolicy update failed: %v", err)
	}

	fetched, err = store.GetPolicy(ctx, "pol-1")
	if err != nil {
		t.Fatalf("GetPolicy after update failed: %v", err)
	}
	if fetched.Status != model.PolicyStatusActive || fetched.ActiveRevision != 1 {
		t.Errorf("GetPolicy updated values mismatch: got status=%s, active_rev=%d", fetched.Status, fetched.ActiveRevision)
	}

	// 4. List policies with filter
	list, err := store.ListPolicies(ctx, model.PolicyFilter{
		OrgID:    "default",
		Category: model.PolicyCategoryResourceThresholds,
		Status:   model.PolicyStatusActive,
	})
	if err != nil {
		t.Fatalf("ListPolicies failed: %v", err)
	}
	if len(list) != 1 || list[0].ID != "pol-1" {
		t.Errorf("ListPolicies returned unexpected count or items: %v", list)
	}

	// Search filter
	list, err = store.ListPolicies(ctx, model.PolicyFilter{
		Search: "limits",
	})
	if err != nil {
		t.Fatalf("ListPolicies search failed: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("ListPolicies search returned unexpected count: %d", len(list))
	}

	// 5. Delete policy
	if err := store.DeletePolicy(ctx, "pol-1"); err != nil {
		t.Fatalf("DeletePolicy failed: %v", err)
	}

	_, err = store.GetPolicy(ctx, "pol-1")
	if err == nil {
		t.Errorf("expected error getting deleted policy, got nil")
	}
}

func TestPolicyRevisionStorage(t *testing.T) {
	store, err := NewSQLiteStorage(Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("failed to init in-memory sqlite: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	pol := &model.Policy{
		ID:        "pol-rev-test",
		OrgID:     "default",
		Name:      "rev-test",
		Category:  model.PolicyCategoryResourceThresholds,
		Status:    model.PolicyStatusActive,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := store.SavePolicy(ctx, pol); err != nil {
		t.Fatalf("SavePolicy failed: %v", err)
	}

	rev1 := &model.PolicyRevision{
		PolicyID:        "pol-rev-test",
		Revision:        1,
		CreatedAt:       time.Now().UTC(),
		CreatedBy:       "admin@example.com",
		ChangeSummary:   "Initial version",
		Selector:        "env == 'prod'",
		Priority:        100,
		InheritanceMode: model.InheritanceModeInheritAndOverride,
		EnforcementMode: model.EnforcementModeEnforce,
		Rules: []model.PolicyRule{
			{
				ID:       "rule-cpu-high",
				Name:     "CPU High Threshold",
				Type:     model.RuleTypeResourceThreshold,
				Severity: model.SeverityWarning,
				Enabled:  true,
				ResourceThreshold: &model.ResourceThresholdRuleConfig{
					Metric:            "cpu_usage_pct",
					WarningThreshold:  75.0,
					CriticalThreshold: 90.0,
					DurationWindow:    "5m",
					Action:            "alert",
				},
			},
		},
	}
	rev1.ComputeDigest()

	if err := store.SavePolicyRevision(ctx, rev1); err != nil {
		t.Fatalf("SavePolicyRevision v1 failed: %v", err)
	}

	rev2 := &model.PolicyRevision{
		PolicyID:        "pol-rev-test",
		Revision:        2,
		CreatedAt:       time.Now().UTC().Add(time.Hour),
		CreatedBy:       "operator@example.com",
		ChangeSummary:   "Tighten CPU critical threshold",
		Selector:        "env == 'prod'",
		Priority:        150,
		InheritanceMode: model.InheritanceModeInheritAndOverride,
		EnforcementMode: model.EnforcementModeEnforce,
		Rules: []model.PolicyRule{
			{
				ID:       "rule-cpu-high",
				Name:     "CPU High Threshold",
				Type:     model.RuleTypeResourceThreshold,
				Severity: model.SeverityCritical,
				Enabled:  true,
				ResourceThreshold: &model.ResourceThresholdRuleConfig{
					Metric:            "cpu_usage_pct",
					WarningThreshold:  75.0,
					CriticalThreshold: 85.0,
					DurationWindow:    "5m",
					Action:            "alert",
				},
			},
		},
	}
	rev2.ComputeDigest()

	if err := store.SavePolicyRevision(ctx, rev2); err != nil {
		t.Fatalf("SavePolicyRevision v2 failed: %v", err)
	}

	// Fetch rev1
	fetched1, err := store.GetPolicyRevision(ctx, "pol-rev-test", 1)
	if err != nil {
		t.Fatalf("GetPolicyRevision v1 failed: %v", err)
	}
	if fetched1.Revision != 1 || fetched1.ContentDigest != rev1.ContentDigest {
		t.Errorf("GetPolicyRevision v1 digest/rev mismatch: got %+v", fetched1)
	}
	if len(fetched1.Rules) != 1 || fetched1.Rules[0].ResourceThreshold.CriticalThreshold != 90.0 {
		t.Errorf("GetPolicyRevision v1 rules mismatch: %+v", fetched1.Rules)
	}

	// Fetch rev2
	fetched2, err := store.GetPolicyRevision(ctx, "pol-rev-test", 2)
	if err != nil {
		t.Fatalf("GetPolicyRevision v2 failed: %v", err)
	}
	if fetched2.Revision != 2 || fetched2.Rules[0].ResourceThreshold.CriticalThreshold != 85.0 {
		t.Errorf("GetPolicyRevision v2 rules mismatch: %+v", fetched2.Rules)
	}

	// List all revisions
	revList, err := store.ListPolicyRevisions(ctx, "pol-rev-test")
	if err != nil {
		t.Fatalf("ListPolicyRevisions failed: %v", err)
	}
	if len(revList) != 2 {
		t.Fatalf("expected 2 revisions, got %d", len(revList))
	}
	if revList[0].Revision != 1 || revList[1].Revision != 2 {
		t.Errorf("revisions not in ascending order: %v", revList)
	}

	// Delete policy should cascade and remove revisions
	if err := store.DeletePolicy(ctx, "pol-rev-test"); err != nil {
		t.Fatalf("DeletePolicy failed: %v", err)
	}

	revListAfter, err := store.ListPolicyRevisions(ctx, "pol-rev-test")
	if err != nil {
		t.Fatalf("ListPolicyRevisions after delete failed: %v", err)
	}
	if len(revListAfter) != 0 {
		t.Errorf("expected 0 revisions after cascade delete, got %d", len(revListAfter))
	}
}

func TestPolicyAssignmentStorage(t *testing.T) {
	store, err := NewSQLiteStorage(Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("failed to init in-memory sqlite: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	pol := &model.Policy{
		ID:        "pol-asgn-test",
		OrgID:     "org-1",
		Name:      "asgn-test",
		Category:  model.PolicyCategoryOperationalCompliance,
		Status:    model.PolicyStatusActive,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := store.SavePolicy(ctx, pol); err != nil {
		t.Fatalf("SavePolicy failed: %v", err)
	}

	asgnOrg := &model.PolicyAssignment{
		ID:         "asgn-1",
		PolicyID:   "pol-asgn-test",
		OrgID:      "org-1",
		TargetType: model.TargetTypeOrganization,
		TargetID:   "org-1",
		AssignedAt: time.Now().UTC(),
		AssignedBy: "sec-admin",
		Enabled:    true,
	}
	if err := store.SavePolicyAssignment(ctx, asgnOrg); err != nil {
		t.Fatalf("SavePolicyAssignment org failed: %v", err)
	}

	asgnGroup := &model.PolicyAssignment{
		ID:         "asgn-2",
		PolicyID:   "pol-asgn-test",
		OrgID:      "org-1",
		TargetType: model.TargetTypeFleetGroup,
		TargetID:   "grp-db",
		AssignedAt: time.Now().UTC(),
		AssignedBy: "sec-admin",
		Enabled:    true,
	}
	if err := store.SavePolicyAssignment(ctx, asgnGroup); err != nil {
		t.Fatalf("SavePolicyAssignment group failed: %v", err)
	}

	asgnNode := &model.PolicyAssignment{
		ID:         "asgn-3",
		PolicyID:   "pol-asgn-test",
		OrgID:      "org-1",
		TargetType: model.TargetTypeNode,
		TargetID:   "node-101",
		AssignedAt: time.Now().UTC(),
		AssignedBy: "sec-admin",
		Enabled:    false, // Disabled
	}
	if err := store.SavePolicyAssignment(ctx, asgnNode); err != nil {
		t.Fatalf("SavePolicyAssignment node failed: %v", err)
	}

	// 1. Get single assignment
	fetched, err := store.GetPolicyAssignment(ctx, "asgn-1")
	if err != nil {
		t.Fatalf("GetPolicyAssignment failed: %v", err)
	}
	if fetched.TargetType != model.TargetTypeOrganization || !fetched.Enabled {
		t.Errorf("GetPolicyAssignment mismatch: %+v", fetched)
	}

	// 2. List with EnabledOnly
	list, err := store.ListPolicyAssignments(ctx, model.PolicyAssignmentFilter{
		OrgID:       "org-1",
		EnabledOnly: true,
	})
	if err != nil {
		t.Fatalf("ListPolicyAssignments failed: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("expected 2 enabled assignments, got %d", len(list))
	}

	// 3. GetAssignmentsForTargets
	targets := []string{"grp-db", "grp-cache"}
	groupAsgns, err := store.GetAssignmentsForTargets(ctx, "org-1", model.TargetTypeFleetGroup, targets)
	if err != nil {
		t.Fatalf("GetAssignmentsForTargets failed: %v", err)
	}
	if len(groupAsgns) != 1 || groupAsgns[0].TargetID != "grp-db" {
		t.Errorf("GetAssignmentsForTargets returned unexpected results: %v", groupAsgns)
	}

	// Check disabled assignment is excluded in GetAssignmentsForTargets
	nodeAsgns, err := store.GetAssignmentsForTargets(ctx, "org-1", model.TargetTypeNode, []string{"node-101"})
	if err != nil {
		t.Fatalf("GetAssignmentsForTargets for disabled node failed: %v", err)
	}
	if len(nodeAsgns) != 0 {
		t.Errorf("expected 0 assignments for disabled node, got %d", len(nodeAsgns))
	}

	// 4. Delete assignment
	if err := store.DeletePolicyAssignment(ctx, "asgn-3"); err != nil {
		t.Fatalf("DeletePolicyAssignment failed: %v", err)
	}

	_, err = store.GetPolicyAssignment(ctx, "asgn-3")
	if err == nil {
		t.Errorf("expected error getting deleted assignment, got nil")
	}
}
