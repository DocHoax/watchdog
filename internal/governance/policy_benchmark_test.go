package governance

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/DocHoax/watchdog/internal/storage"
	"github.com/DocHoax/watchdog/pkg/model"
)

func BenchmarkSelector_ParseAndEvaluate(b *testing.B) {
	node := &model.FleetNode{
		Identity: model.NodeIdentity{
			NodeID:   "bench-node-01",
			Hostname: "db-primary-us-east-1a",
			Tags: map[string]string{
				"env":    "production",
				"tier":   "database",
				"region": "us-east",
			},
		},
		Status: model.NodeStatusHealthy,
	}
	ownership := &model.NodeOwnershipMetadata{
		Environment: "production",
	}
	expr := `tags.env == "production" AND tags.tier IN ["database", "backend"] AND hostname != ""`

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		matched, err := MatchesNode(expr, node, ownership, []string{"grp-1"}, []string{"/root/db"})
		if err != nil || !matched {
			b.Fatalf("unexpected match result: matched=%v, err=%v", matched, err)
		}
	}
}

func BenchmarkPolicyRevision_ComputeDigest(b *testing.B) {
	rules := make([]model.PolicyRule, 20)
	for i := range 20 {
		rules[i] = model.PolicyRule{
			ID:      fmt.Sprintf("rule-bench-%02d", i),
			Name:    fmt.Sprintf("Benchmark Rule %d", i),
			Type:    model.RuleTypeResourceThreshold,
			Enabled: true,
			ResourceThreshold: &model.ResourceThresholdRuleConfig{
				Metric:            "cpu_usage_pct",
				WarningThreshold:  70.0 + float64(i),
				CriticalThreshold: 85.0 + float64(i),
			},
		}
	}

	rev := &model.PolicyRevision{
		PolicyID:        "pol-bench-digest",
		Revision:        1,
		Priority:        100,
		InheritanceMode: model.InheritanceModeInheritAndOverride,
		EnforcementMode: model.EnforcementModeEnforce,
		Selector:        `tags.env == "production"`,
		Rules:           rules,
		Metadata: map[string]string{
			"owner": "sre-team",
		},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = rev.ComputeDigest()
	}
}

func BenchmarkPolicyResolver_ResolveNodePolicies_100Rules(b *testing.B) {
	store, err := storage.NewSQLiteStorage(storage.Config{Path: ":memory:"})
	if err != nil {
		b.Fatalf("failed to create sqlite memory store: %v", err)
	}
	ctx := context.Background()
	now := time.Now().UTC()

	org := &model.Organization{
		ID:     "org-bench",
		Name:   "Benchmark Org",
		Status: model.OrgStatusActive,
	}
	if err := store.SaveOrganization(ctx, org); err != nil {
		b.Fatalf("SaveOrganization failed: %v", err)
	}

	grpRoot := &model.FleetGroup{
		ID:        "grp-bench-root",
		OrgID:     "org-bench",
		Name:      "Root",
		Path:      "/grp-bench-root",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := store.SaveFleetGroup(ctx, grpRoot); err != nil {
		b.Fatalf("SaveFleetGroup root failed: %v", err)
	}

	grpLeaf := &model.FleetGroup{
		ID:            "grp-bench-leaf",
		OrgID:         "org-bench",
		ParentGroupID: "grp-bench-root",
		Name:          "Leaf",
		Path:          "/grp-bench-root/grp-bench-leaf",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := store.SaveFleetGroup(ctx, grpLeaf); err != nil {
		b.Fatalf("SaveFleetGroup leaf failed: %v", err)
	}

	node := &model.FleetNode{
		Identity: model.NodeIdentity{
			NodeID:   "node-bench-01",
			Hostname: "srv-bench-01",
			Tags: map[string]string{
				"env": "production",
			},
		},
		Status:        model.NodeStatusHealthy,
		LastHeartbeat: now,
		Metadata: map[string]string{
			"org_id": "org-bench",
		},
	}
	if err := store.SaveFleetNode(ctx, node); err != nil {
		b.Fatalf("SaveFleetNode failed: %v", err)
	}

	if err := store.AddGroupMember(ctx, &model.FleetGroupMember{
		GroupID: "grp-bench-leaf",
		NodeID:  "node-bench-01",
		Role:    model.MembershipRolePrimary,
		AddedAt: now,
	}); err != nil {
		b.Fatalf("AddGroupMember failed: %v", err)
	}

	polOrg := &model.Policy{
		ID:             "pol-bench-org",
		OrgID:          "org-bench",
		Name:           "Org Policy",
		Category:       model.PolicyCategoryResourceThresholds,
		Status:         model.PolicyStatusActive,
		ActiveRevision: 1,
	}
	if err := store.SavePolicy(ctx, polOrg); err != nil {
		b.Fatalf("SavePolicy org failed: %v", err)
	}

	polLeaf := &model.Policy{
		ID:             "pol-bench-leaf",
		OrgID:          "org-bench",
		Name:           "Leaf Policy",
		Category:       model.PolicyCategoryResourceThresholds,
		Status:         model.PolicyStatusActive,
		ActiveRevision: 1,
	}
	if err := store.SavePolicy(ctx, polLeaf); err != nil {
		b.Fatalf("SavePolicy leaf failed: %v", err)
	}

	orgRules := make([]model.PolicyRule, 50)
	for i := range 50 {
		orgRules[i] = model.PolicyRule{
			ID:      fmt.Sprintf("rule-org-%02d", i),
			Name:    fmt.Sprintf("Org Rule %d", i),
			Type:    model.RuleTypeResourceThreshold,
			Enabled: true,
			ResourceThreshold: &model.ResourceThresholdRuleConfig{
				Metric:            fmt.Sprintf("metric_%02d", i),
				WarningThreshold:  70.0,
				CriticalThreshold: 85.0,
			},
		}
	}
	leafRules := make([]model.PolicyRule, 50)
	for i := range 50 {
		leafRules[i] = model.PolicyRule{
			ID:      fmt.Sprintf("rule-leaf-%02d", i),
			Name:    fmt.Sprintf("Leaf Rule %d", i),
			Type:    model.RuleTypeResourceThreshold,
			Enabled: true,
			ResourceThreshold: &model.ResourceThresholdRuleConfig{
				Metric:            fmt.Sprintf("metric_%02d", i),
				WarningThreshold:  80.0,
				CriticalThreshold: 95.0,
			},
		}
	}

	revOrg := &model.PolicyRevision{
		PolicyID:        "pol-bench-org",
		Revision:        1,
		Priority:        100,
		InheritanceMode: model.InheritanceModeInheritAndOverride,
		EnforcementMode: model.EnforcementModeEnforce,
		Rules:           orgRules,
	}
	revOrg.ComputeDigest()
	if err := store.SavePolicyRevision(ctx, revOrg); err != nil {
		b.Fatalf("SavePolicyRevision org failed: %v", err)
	}

	revLeaf := &model.PolicyRevision{
		PolicyID:        "pol-bench-leaf",
		Revision:        1,
		Priority:        100,
		InheritanceMode: model.InheritanceModeInheritAndOverride,
		EnforcementMode: model.EnforcementModeEnforce,
		Rules:           leafRules,
	}
	revLeaf.ComputeDigest()
	if err := store.SavePolicyRevision(ctx, revLeaf); err != nil {
		b.Fatalf("SavePolicyRevision leaf failed: %v", err)
	}

	if err := store.SavePolicyAssignment(ctx, &model.PolicyAssignment{
		ID:         "asgn-bench-org",
		PolicyID:   "pol-bench-org",
		OrgID:      "org-bench",
		TargetType: model.TargetTypeOrganization,
		TargetID:   "org-bench",
		Enabled:    true,
	}); err != nil {
		b.Fatalf("SavePolicyAssignment org failed: %v", err)
	}

	if err := store.SavePolicyAssignment(ctx, &model.PolicyAssignment{
		ID:         "asgn-bench-leaf",
		PolicyID:   "pol-bench-leaf",
		OrgID:      "org-bench",
		TargetType: model.TargetTypeFleetGroup,
		TargetID:   "grp-bench-leaf",
		Enabled:    true,
	}); err != nil {
		b.Fatalf("SavePolicyAssignment leaf failed: %v", err)
	}

	resolver := NewPolicyResolver(store)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		res, err := resolver.ResolveNodePolicies(ctx, "node-bench-01")
		if err != nil || len(res.EffectiveRules) != 50 {
			b.Fatalf("failed resolution: len=%d, err=%v", len(res.EffectiveRules), err)
		}
	}
}

func BenchmarkDetectEffectiveRuleConflicts_50Rules(b *testing.B) {
	effectiveRules := make([]EffectiveRule, 50)
	for i := range 50 {
		effectiveRules[i] = EffectiveRule{
			SourcePolicyID: fmt.Sprintf("pol-%d", i%5),
			SourceRevision: 1,
			HierarchyLevel: 10,
			Priority:       100,
			Category:       model.PolicyCategoryResourceThresholds,
			Rule: model.PolicyRule{
				ID:      fmt.Sprintf("rule-cpu-%d", i),
				Type:    model.RuleTypeResourceThreshold,
				Enabled: true,
				ResourceThreshold: &model.ResourceThresholdRuleConfig{
					Metric:            fmt.Sprintf("metric_%d", i%10),
					WarningThreshold:  70.0 + float64(i),
					CriticalThreshold: 85.0 + float64(i),
				},
			},
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = DetectEffectiveRuleConflicts(effectiveRules)
	}
}
