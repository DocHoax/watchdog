package governance

import (
	"context"
	"testing"
	"time"

	"github.com/DocHoax/watchdog/internal/storage"
	"github.com/DocHoax/watchdog/pkg/model"
)

func setupTestStorage(t *testing.T) (storage.Storage, context.Context) {
	store, err := storage.NewSQLiteStorage(storage.Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("failed to create sqlite memory store: %v", err)
	}
	ctx := context.Background()

	// Create default Org
	org := &model.Organization{
		ID:          "org-prod",
		Name:        "Production Organization",
		DisplayName: "Prod Org",
		Status:      model.OrgStatusActive,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	if err := store.SaveOrganization(ctx, org); err != nil {
		t.Fatalf("SaveOrganization failed: %v", err)
	}

	// Create Root Group (level 2)
	rootGrp := &model.FleetGroup{
		ID:        "grp-root",
		OrgID:     "org-prod",
		Name:      "Root Infrastructure",
		Path:      "/grp-root",
		Depth:     1,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := store.SaveFleetGroup(ctx, rootGrp); err != nil {
		t.Fatalf("SaveFleetGroup root failed: %v", err)
	}

	// Create Child Group (level 3)
	childGrp := &model.FleetGroup{
		ID:            "grp-web",
		OrgID:         "org-prod",
		ParentGroupID: "grp-root",
		Name:          "Web Tier",
		Path:          "/grp-root/grp-web",
		Depth:         2,
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}
	if err := store.SaveFleetGroup(ctx, childGrp); err != nil {
		t.Fatalf("SaveFleetGroup child failed: %v", err)
	}

	// Create Node
	now := time.Now().UTC()
	node := &model.FleetNode{
		Identity: model.NodeIdentity{
			NodeID:      "node-web-01",
			Hostname:    "web-01.prod.internal",
			OS:          "linux",
			Platform:    "ubuntu",
			Arch:        "amd64",
			Version:     "v1.4.2",
			CPUCores:    8,
			TotalMemory: 32 * 1024 * 1024 * 1024,
			Tags: map[string]string{
				"env":  "production",
				"tier": "web",
			},
		},
		Status:        model.NodeStatusHealthy,
		LastHeartbeat: now,
		Metadata: map[string]string{
			"org_id": "org-prod",
			"env":    "production",
		},
	}
	if err := store.SaveFleetNode(ctx, node); err != nil {
		t.Fatalf("SaveFleetNode failed: %v", err)
	}

	// Add node to group
	if err := store.AddGroupMember(ctx, "grp-web", "node-web-01"); err != nil {
		t.Fatalf("AddGroupMember failed: %v", err)
	}

	return store, ctx
}

func TestPolicyResolver_HierarchicalResolution(t *testing.T) {
	store, ctx := setupTestStorage(t)
	defer store.Close()

	resolver := NewPolicyResolver(store)

	// 1. Create Org Baseline Policy (Level 1)
	orgPol := &model.Policy{
		ID:             "pol-org-baseline",
		OrgID:          "org-prod",
		Name:           "org-baseline",
		Category:       model.PolicyCategoryResourceThresholds,
		Status:         model.PolicyStatusActive,
		ActiveRevision: 1,
	}
	if err := store.SavePolicy(ctx, orgPol); err != nil {
		t.Fatalf("SavePolicy org failed: %v", err)
	}

	orgRev := &model.PolicyRevision{
		PolicyID:        "pol-org-baseline",
		Revision:        1,
		Priority:        100,
		InheritanceMode: model.InheritanceModeInheritAndOverride,
		EnforcementMode: model.EnforcementModeEnforce,
		Rules: []model.PolicyRule{
			{
				ID:       "rule-cpu",
				Name:     "CPU Threshold",
				Type:     model.RuleTypeResourceThreshold,
				Severity: model.SeverityWarning,
				Enabled:  true,
				ResourceThreshold: &model.ResourceThresholdRuleConfig{
					Metric:            "cpu_usage_pct",
					WarningThreshold:  70.0,
					CriticalThreshold: 85.0,
				},
			},
			{
				ID:       "rule-mem",
				Name:     "Memory Threshold",
				Type:     model.RuleTypeResourceThreshold,
				Severity: model.SeverityCritical,
				Enabled:  true,
				ResourceThreshold: &model.ResourceThresholdRuleConfig{
					Metric:            "memory_usage_pct",
					WarningThreshold:  80.0,
					CriticalThreshold: 90.0,
				},
			},
		},
	}
	orgRev.ComputeDigest()
	if err := store.SavePolicyRevision(ctx, orgRev); err != nil {
		t.Fatalf("SavePolicyRevision org failed: %v", err)
	}

	asgnOrg := &model.PolicyAssignment{
		ID:         "asgn-org-1",
		PolicyID:   "pol-org-baseline",
		OrgID:      "org-prod",
		TargetType: model.TargetTypeOrganization,
		TargetID:   "org-prod",
		Enabled:    true,
	}
	if err := store.SavePolicyAssignment(ctx, asgnOrg); err != nil {
		t.Fatalf("SavePolicyAssignment org failed: %v", err)
	}

	// 2. Create Child Group Policy (Level 3) overriding CPU rule
	grpPol := &model.Policy{
		ID:             "pol-web-override",
		OrgID:          "org-prod",
		Name:           "web-override",
		Category:       model.PolicyCategoryResourceThresholds,
		Status:         model.PolicyStatusActive,
		ActiveRevision: 1,
	}
	if err := store.SavePolicy(ctx, grpPol); err != nil {
		t.Fatalf("SavePolicy grp failed: %v", err)
	}

	grpRev := &model.PolicyRevision{
		PolicyID:        "pol-web-override",
		Revision:        1,
		Priority:        100,
		InheritanceMode: model.InheritanceModeInheritAndOverride,
		EnforcementMode: model.EnforcementModeEnforce,
		Rules: []model.PolicyRule{
			{
				ID:       "rule-cpu-tight",
				Name:     "Tight CPU Threshold",
				Type:     model.RuleTypeResourceThreshold,
				Severity: model.SeverityCritical,
				Enabled:  true,
				ResourceThreshold: &model.ResourceThresholdRuleConfig{
					Metric:            "cpu_usage_pct", // Same metric -> overrides org baseline
					WarningThreshold:  60.0,
					CriticalThreshold: 75.0,
				},
			},
		},
	}
	grpRev.ComputeDigest()
	if err := store.SavePolicyRevision(ctx, grpRev); err != nil {
		t.Fatalf("SavePolicyRevision grp failed: %v", err)
	}

	asgnGrp := &model.PolicyAssignment{
		ID:         "asgn-grp-1",
		PolicyID:   "pol-web-override",
		OrgID:      "org-prod",
		TargetType: model.TargetTypeFleetGroup,
		TargetID:   "grp-web",
		Enabled:    true,
	}
	if err := store.SavePolicyAssignment(ctx, asgnGrp); err != nil {
		t.Fatalf("SavePolicyAssignment grp failed: %v", err)
	}

	// Resolve node policies
	resolved, err := resolver.ResolveNodePolicies(ctx, "node-web-01")
	if err != nil {
		t.Fatalf("ResolveNodePolicies failed: %v", err)
	}

	if resolved.NodeID != "node-web-01" || resolved.OrgID != "org-prod" {
		t.Errorf("unexpected resolved header: %+v", resolved)
	}

	// We expect 2 effective rules: overridden CPU rule (from group) + inherited Memory rule (from org)
	if len(resolved.EffectiveRules) != 2 {
		t.Fatalf("expected 2 effective rules, got %d", len(resolved.EffectiveRules))
	}

	var cpuRule, memRule *EffectiveRule
	for i := range resolved.EffectiveRules {
		r := &resolved.EffectiveRules[i]
		if r.Rule.Type == model.RuleTypeResourceThreshold {
			if r.Rule.ResourceThreshold.Metric == "cpu_usage_pct" {
				cpuRule = r
			} else if r.Rule.ResourceThreshold.Metric == "memory_usage_pct" {
				memRule = r
			}
		}
	}

	if cpuRule == nil || memRule == nil {
		t.Fatalf("missing cpu or mem rule in effective set: %+v", resolved.EffectiveRules)
	}

	// Check that CPU rule was overridden by group (Level 3, Warning=60, Critical=75)
	if cpuRule.SourcePolicyID != "pol-web-override" || cpuRule.HierarchyLevel != 3 {
		t.Errorf("expected CPU rule to originate from pol-web-override at level 3, got %s (level %d)", cpuRule.SourcePolicyID, cpuRule.HierarchyLevel)
	}
	if cpuRule.Rule.ResourceThreshold.CriticalThreshold != 75.0 {
		t.Errorf("expected CPU critical threshold 75.0, got %.1f", cpuRule.Rule.ResourceThreshold.CriticalThreshold)
	}
	if cpuRule.OverrodeRule == nil || cpuRule.OverrodeRule.SourcePolicyID != "pol-org-baseline" {
		t.Errorf("expected OverrodeRule provenance to track pol-org-baseline, got %+v", cpuRule.OverrodeRule)
	}

	// Check that Memory rule was inherited from Org (Level 1, Warning=80, Critical=90)
	if memRule.SourcePolicyID != "pol-org-baseline" || memRule.HierarchyLevel != 1 {
		t.Errorf("expected Memory rule to originate from pol-org-baseline at level 1, got %s", memRule.SourcePolicyID)
	}
}

func TestPolicyResolver_InheritanceModes(t *testing.T) {
	t.Run("strict override clears ancestor rules in category", func(t *testing.T) {
		store, ctx := setupTestStorage(t)
		defer store.Close()
		resolver := NewPolicyResolver(store)

		// 1. Org policy with 2 rules
		orgPol := &model.Policy{
			ID: "pol-org", OrgID: "org-prod", Name: "org", Category: model.PolicyCategoryResourceThresholds,
			Status: model.PolicyStatusActive, ActiveRevision: 1,
		}
		_ = store.SavePolicy(ctx, orgPol)
		orgRev := &model.PolicyRevision{
			PolicyID: "pol-org", Revision: 1, Priority: 100, InheritanceMode: model.InheritanceModeInheritAndOverride,
			Rules: []model.PolicyRule{
				{
					ID: "r1", Type: model.RuleTypeResourceThreshold, Enabled: true,
					ResourceThreshold: &model.ResourceThresholdRuleConfig{Metric: "cpu", WarningThreshold: 80, CriticalThreshold: 90},
				},
				{
					ID: "r2", Type: model.RuleTypeResourceThreshold, Enabled: true,
					ResourceThreshold: &model.ResourceThresholdRuleConfig{Metric: "mem", WarningThreshold: 80, CriticalThreshold: 90},
				},
			},
		}
		_ = store.SavePolicyRevision(ctx, orgRev)
		_ = store.SavePolicyAssignment(ctx, &model.PolicyAssignment{
			ID: "asgn-org", PolicyID: "pol-org", OrgID: "org-prod", TargetType: model.TargetTypeOrganization, TargetID: "org-prod", Enabled: true,
		})

		// 2. Node policy with Strict Override and only 1 rule (disk)
		nodePol := &model.Policy{
			ID: "pol-node", OrgID: "org-prod", Name: "node", Category: model.PolicyCategoryResourceThresholds,
			Status: model.PolicyStatusActive, ActiveRevision: 1,
		}
		_ = store.SavePolicy(ctx, nodePol)
		nodeRev := &model.PolicyRevision{
			PolicyID: "pol-node", Revision: 1, Priority: 100, InheritanceMode: model.InheritanceModeStrictOverride,
			Rules: []model.PolicyRule{
				{
					ID: "r-disk", Type: model.RuleTypeResourceThreshold, Enabled: true,
					ResourceThreshold: &model.ResourceThresholdRuleConfig{Metric: "disk", WarningThreshold: 75, CriticalThreshold: 85},
				},
			},
		}
		_ = store.SavePolicyRevision(ctx, nodeRev)
		_ = store.SavePolicyAssignment(ctx, &model.PolicyAssignment{
			ID: "asgn-node", PolicyID: "pol-node", OrgID: "org-prod", TargetType: model.TargetTypeNode, TargetID: "node-web-01", Enabled: true,
		})

		resolved, err := resolver.ResolveNodePolicies(ctx, "node-web-01")
		if err != nil {
			t.Fatalf("ResolveNodePolicies failed: %v", err)
		}

		// Because strict override was used on node level (Level 100), all ancestor rules in ResourceThresholds category are replaced
		if len(resolved.EffectiveRules) != 1 {
			t.Fatalf("expected 1 effective rule due to strict override, got %d: %+v", len(resolved.EffectiveRules), resolved.EffectiveRules)
		}
		if resolved.EffectiveRules[0].Rule.ResourceThreshold.Metric != "disk" {
			t.Errorf("expected disk rule, got %s", resolved.EffectiveRules[0].Rule.ResourceThreshold.Metric)
		}
	})

	t.Run("additive mode accumulates rules", func(t *testing.T) {
		store, ctx := setupTestStorage(t)
		defer store.Close()
		resolver := NewPolicyResolver(store)

		// Org rule
		orgPol := &model.Policy{
			ID: "pol-org", OrgID: "org-prod", Name: "org", Category: model.PolicyCategoryOperationalCompliance,
			Status: model.PolicyStatusActive, ActiveRevision: 1,
		}
		_ = store.SavePolicy(ctx, orgPol)
		orgRev := &model.PolicyRevision{
			PolicyID: "pol-org", Revision: 1, Priority: 100, InheritanceMode: model.InheritanceModeAdditive,
			Rules: []model.PolicyRule{
				{
					ID: "c1", Type: model.RuleTypeOperationalCompliance, Enabled: true,
					OperationalCompliance: &model.OperationalComplianceRuleConfig{CheckType: "heartbeat_freshness", MaxAgeSeconds: 300},
				},
			},
		}
		_ = store.SavePolicyRevision(ctx, orgRev)
		_ = store.SavePolicyAssignment(ctx, &model.PolicyAssignment{
			ID: "asgn-org", PolicyID: "pol-org", OrgID: "org-prod", TargetType: model.TargetTypeOrganization, TargetID: "org-prod", Enabled: true,
		})

		// Node additive rule with same checkType
		nodePol := &model.Policy{
			ID: "pol-node", OrgID: "org-prod", Name: "node", Category: model.PolicyCategoryOperationalCompliance,
			Status: model.PolicyStatusActive, ActiveRevision: 1,
		}
		_ = store.SavePolicy(ctx, nodePol)
		nodeRev := &model.PolicyRevision{
			PolicyID: "pol-node", Revision: 1, Priority: 100, InheritanceMode: model.InheritanceModeAdditive,
			Rules: []model.PolicyRule{
				{
					ID: "c2", Type: model.RuleTypeOperationalCompliance, Enabled: true,
					OperationalCompliance: &model.OperationalComplianceRuleConfig{CheckType: "heartbeat_freshness", MaxAgeSeconds: 120},
				},
			},
		}
		_ = store.SavePolicyRevision(ctx, nodeRev)
		_ = store.SavePolicyAssignment(ctx, &model.PolicyAssignment{
			ID: "asgn-node", PolicyID: "pol-node", OrgID: "org-prod", TargetType: model.TargetTypeNode, TargetID: "node-web-01", Enabled: true,
		})

		resolved, err := resolver.ResolveNodePolicies(ctx, "node-web-01")
		if err != nil {
			t.Fatalf("ResolveNodePolicies failed: %v", err)
		}

		if len(resolved.EffectiveRules) != 2 {
			t.Fatalf("expected 2 additive rules, got %d", len(resolved.EffectiveRules))
		}
	})
}

func TestPolicyResolver_SelectorFiltering(t *testing.T) {
	store, ctx := setupTestStorage(t)
	defer store.Close()
	resolver := NewPolicyResolver(store)

	// Policy matching staging environment only
	polStaging := &model.Policy{
		ID: "pol-staging", OrgID: "org-prod", Name: "staging-rules", Category: model.PolicyCategoryResourceThresholds,
		Status: model.PolicyStatusActive, ActiveRevision: 1,
	}
	_ = store.SavePolicy(ctx, polStaging)
	revStaging := &model.PolicyRevision{
		PolicyID: "pol-staging", Revision: 1, Priority: 100, Selector: `env == "staging"`,
		Rules: []model.PolicyRule{
			{
				ID: "r-stg", Type: model.RuleTypeResourceThreshold, Enabled: true,
				ResourceThreshold: &model.ResourceThresholdRuleConfig{Metric: "cpu", WarningThreshold: 90, CriticalThreshold: 95},
			},
		},
	}
	_ = store.SavePolicyRevision(ctx, revStaging)
	_ = store.SavePolicyAssignment(ctx, &model.PolicyAssignment{
		ID: "asgn-stg", PolicyID: "pol-staging", OrgID: "org-prod", TargetType: model.TargetTypeOrganization, TargetID: "org-prod", Enabled: true,
	})

	// Policy matching production environment
	polProd := &model.Policy{
		ID: "pol-prod", OrgID: "org-prod", Name: "prod-rules", Category: model.PolicyCategoryResourceThresholds,
		Status: model.PolicyStatusActive, ActiveRevision: 1,
	}
	_ = store.SavePolicy(ctx, polProd)
	revProd := &model.PolicyRevision{
		PolicyID: "pol-prod", Revision: 1, Priority: 100, Selector: `env == "production"`,
		Rules: []model.PolicyRule{
			{
				ID: "r-prd", Type: model.RuleTypeResourceThreshold, Enabled: true,
				ResourceThreshold: &model.ResourceThresholdRuleConfig{Metric: "cpu", WarningThreshold: 70, CriticalThreshold: 80},
			},
		},
	}
	_ = store.SavePolicyRevision(ctx, revProd)
	_ = store.SavePolicyAssignment(ctx, &model.PolicyAssignment{
		ID: "asgn-prd", PolicyID: "pol-prod", OrgID: "org-prod", TargetType: model.TargetTypeOrganization, TargetID: "org-prod", Enabled: true,
	})

	resolved, err := resolver.ResolveNodePolicies(ctx, "node-web-01")
	if err != nil {
		t.Fatalf("ResolveNodePolicies failed: %v", err)
	}

	// node-web-01 has env=production, so only pol-prod should match
	if len(resolved.EffectiveRules) != 1 {
		t.Fatalf("expected 1 rule matched, got %d", len(resolved.EffectiveRules))
	}
	if resolved.EffectiveRules[0].SourcePolicyID != "pol-prod" {
		t.Errorf("expected pol-prod, got %s", resolved.EffectiveRules[0].SourcePolicyID)
	}
}

func TestPolicyResolver_ComplianceEvaluation(t *testing.T) {
	store, ctx := setupTestStorage(t)
	defer store.Close()
	resolver := NewPolicyResolver(store)

	pol := &model.Policy{
		ID: "pol-compliance", OrgID: "org-prod", Name: "compliance-checks", Category: model.PolicyCategoryOperationalCompliance,
		Status: model.PolicyStatusActive, ActiveRevision: 1,
	}
	_ = store.SavePolicy(ctx, pol)

	rev := &model.PolicyRevision{
		PolicyID: "pol-compliance", Revision: 1, Priority: 100,
		Rules: []model.PolicyRule{
			{
				ID: "comp-hb", Name: "Heartbeat Freshness", Type: model.RuleTypeOperationalCompliance, Enabled: true,
				OperationalCompliance: &model.OperationalComplianceRuleConfig{
					CheckType:         "heartbeat_freshness",
					MaxAgeSeconds:     300,
					ViolationSeverity: model.SeverityCritical,
				},
			},
			{
				ID: "comp-ver", Name: "Collector Version", Type: model.RuleTypeOperationalCompliance, Enabled: true,
				OperationalCompliance: &model.OperationalComplianceRuleConfig{
					CheckType:         "collector_version",
					ExpectedValue:     "v1.4.2",
					ViolationSeverity: model.SeverityWarning,
				},
			},
			{
				ID: "comp-tags", Name: "Mandatory Tags", Type: model.RuleTypeOperationalCompliance, Enabled: true,
				OperationalCompliance: &model.OperationalComplianceRuleConfig{
					CheckType:         "mandatory_tags",
					ExpectedValue:     "env, tier, owner",
					ViolationSeverity: model.SeverityWarning,
				},
			},
			{
				ID: "comp-os", Name: "Approved Platforms", Type: model.RuleTypeOperationalCompliance, Enabled: true,
				OperationalCompliance: &model.OperationalComplianceRuleConfig{
					CheckType:         "approved_platforms",
					ExpectedValue:     "ubuntu, debian, rhel",
					ViolationSeverity: model.SeverityCritical,
				},
			},
		},
	}
	_ = store.SavePolicyRevision(ctx, rev)
	_ = store.SavePolicyAssignment(ctx, &model.PolicyAssignment{
		ID: "asgn-comp", PolicyID: "pol-compliance", OrgID: "org-prod", TargetType: model.TargetTypeOrganization, TargetID: "org-prod", Enabled: true,
	})

	report, err := resolver.EvaluateNodeCompliance(ctx, "node-web-01")
	if err != nil {
		t.Fatalf("EvaluateNodeCompliance failed: %v", err)
	}

	if report.NodeID != "node-web-01" {
		t.Errorf("expected node ID node-web-01, got %s", report.NodeID)
	}

	// node-web-01 has tags: env=production, tier=web. It is missing tag "owner".
	// Heartbeat: fresh (0s <= 300s) -> Pass
	// Collector Version: v1.4.2 matches v1.4.2 -> Pass
	// Approved Platforms: ubuntu is in [ubuntu, debian, rhel] -> Pass
	// Mandatory Tags: missing 'owner' -> Fail (Warning)
	if report.IsCompliant {
		t.Errorf("expected non-compliant report due to missing tag 'owner'")
	}
	if len(report.Violations) != 1 {
		t.Fatalf("expected 1 violation, got %d: %+v", len(report.Violations), report.Violations)
	}
	if report.Violations[0].RuleID != "comp-tags" {
		t.Errorf("expected violation on comp-tags, got %s", report.Violations[0].RuleID)
	}
	if report.Violations[0].Severity != model.SeverityWarning {
		t.Errorf("expected warning severity, got %s", report.Violations[0].Severity)
	}
}

func TestPolicyResolver_ExplainResolution(t *testing.T) {
	store, ctx := setupTestStorage(t)
	defer store.Close()
	resolver := NewPolicyResolver(store)

	pol := &model.Policy{
		ID: "pol-explain", OrgID: "org-prod", Name: "explain-test", Category: model.PolicyCategoryResourceThresholds,
		Status: model.PolicyStatusActive, ActiveRevision: 1,
	}
	_ = store.SavePolicy(ctx, pol)
	rev := &model.PolicyRevision{
		PolicyID: "pol-explain", Revision: 1, Priority: 50,
		Rules: []model.PolicyRule{
			{
				ID: "r-cpu", Type: model.RuleTypeResourceThreshold, Enabled: true,
				ResourceThreshold: &model.ResourceThresholdRuleConfig{Metric: "cpu", WarningThreshold: 70, CriticalThreshold: 85},
			},
		},
	}
	_ = store.SavePolicyRevision(ctx, rev)
	_ = store.SavePolicyAssignment(ctx, &model.PolicyAssignment{
		ID: "asgn-explain", PolicyID: "pol-explain", OrgID: "org-prod", TargetType: model.TargetTypeOrganization, TargetID: "org-prod", Enabled: true,
	})

	explanation, err := resolver.ExplainResolution(ctx, "node-web-01")
	if err != nil {
		t.Fatalf("ExplainResolution failed: %v", err)
	}

	if explanation.NodeID != "node-web-01" {
		t.Errorf("expected node ID node-web-01, got %s", explanation.NodeID)
	}
	if len(explanation.Steps) == 0 {
		t.Errorf("expected diagnostic steps in explanation")
	}
	if len(explanation.ResolvedSet.EffectiveRules) != 1 {
		t.Errorf("expected 1 effective rule in resolved set")
	}
}
