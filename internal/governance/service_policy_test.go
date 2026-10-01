package governance

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DocHoax/watchdog/pkg/model"
)

func TestGovernanceService_Policy_Lifecycle(t *testing.T) {
	svc, store := setupTestService(t)
	defer store.Close()
	ctx := context.Background()

	// 1. Validation errors on CreatePolicy
	if err := svc.CreatePolicy(ctx, nil); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput for nil policy, got %v", err)
	}
	invalidPolicy := &model.Policy{
		ID:       "invalid-pol",
		OrgID:    "non-existent-org",
		Name:     "Test Policy",
		Category: model.PolicyCategoryResourceThresholds,
	}
	if err := svc.CreatePolicy(ctx, invalidPolicy); !errors.Is(err, ErrOrganizationNotFound) {
		t.Errorf("expected ErrOrganizationNotFound, got %v", err)
	}

	// 2. Create valid policy
	pol := &model.Policy{
		ID:          "pol-cpu-guard",
		OrgID:       model.DefaultOrganizationID,
		Name:        "CPU Usage Guard",
		Description: "Enforces CPU usage limits across fleet",
		Category:    model.PolicyCategoryResourceThresholds,
		Status:      model.PolicyStatusDraft,
	}
	if err := svc.CreatePolicy(ctx, pol); err != nil {
		t.Fatalf("CreatePolicy failed: %v", err)
	}

	// 3. Duplicate policy ID rejected
	if err := svc.CreatePolicy(ctx, pol); !errors.Is(err, ErrPolicyExists) {
		t.Errorf("expected ErrPolicyExists, got %v", err)
	}

	// 4. GetPolicy
	fetched, err := svc.GetPolicy(ctx, "pol-cpu-guard")
	if err != nil {
		t.Fatalf("GetPolicy failed: %v", err)
	}
	if fetched.Name != "CPU Usage Guard" {
		t.Errorf("expected name 'CPU Usage Guard', got %s", fetched.Name)
	}

	// Empty ID get
	if _, err := svc.GetPolicy(ctx, ""); !errors.Is(err, ErrInvalidIdentifier) {
		t.Errorf("expected ErrInvalidIdentifier on empty policy ID, got %v", err)
	}
	// Non-existent get
	if _, err := svc.GetPolicy(ctx, "non-existent"); !errors.Is(err, ErrPolicyNotFound) {
		t.Errorf("expected ErrPolicyNotFound, got %v", err)
	}

	// 5. UpdatePolicy
	pol.Description = "Updated description"
	if err := svc.UpdatePolicy(ctx, pol); err != nil {
		t.Fatalf("UpdatePolicy failed: %v", err)
	}
	fetched, err = svc.GetPolicy(ctx, "pol-cpu-guard")
	if err != nil || fetched.Description != "Updated description" {
		t.Errorf("expected description updated, got %v", fetched)
	}

	// 6. ListPolicies with filter
	list, err := svc.ListPolicies(ctx, model.PolicyFilter{
		OrgID:    model.DefaultOrganizationID,
		Category: model.PolicyCategoryResourceThresholds,
	})
	if err != nil {
		t.Fatalf("ListPolicies failed: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 policy in list, got %d", len(list))
	}

	// 7. Policy status lifecycle transitions
	// Draft -> Active -> Disabled -> Archived
	if err := svc.SetPolicyStatus(ctx, "pol-cpu-guard", model.PolicyStatusActive); err != nil {
		t.Fatalf("transition to Active failed: %v", err)
	}
	if err := svc.SetPolicyStatus(ctx, "pol-cpu-guard", model.PolicyStatusDisabled); err != nil {
		t.Fatalf("transition to Disabled failed: %v", err)
	}
	if err := svc.SetPolicyStatus(ctx, "pol-cpu-guard", model.PolicyStatusArchived); err != nil {
		t.Fatalf("transition to Archived failed: %v", err)
	}

	// Invalid transition: Archived -> Draft
	if err := svc.SetPolicyStatus(ctx, "pol-cpu-guard", model.PolicyStatusDraft); !errors.Is(err, ErrInvalidLifecycleTransition) {
		t.Errorf("expected ErrInvalidLifecycleTransition from archived to draft, got %v", err)
	}

	// 8. DeletePolicy
	if err := svc.DeletePolicy(ctx, "pol-cpu-guard"); err != nil {
		t.Fatalf("DeletePolicy failed: %v", err)
	}
	if _, err := svc.GetPolicy(ctx, "pol-cpu-guard"); !errors.Is(err, ErrPolicyNotFound) {
		t.Errorf("expected ErrPolicyNotFound after delete, got %v", err)
	}
	if err := svc.DeletePolicy(ctx, "non-existent"); !errors.Is(err, ErrPolicyNotFound) {
		t.Errorf("expected ErrPolicyNotFound on deleting non-existent policy, got %v", err)
	}
}

func TestGovernanceService_Revision_Publishing_And_Digests(t *testing.T) {
	svc, store := setupTestService(t)
	defer store.Close()
	ctx := context.Background()

	pol := &model.Policy{
		ID:       "pol-rev-test",
		OrgID:    model.DefaultOrganizationID,
		Name:     "Revision Test Policy",
		Category: model.PolicyCategoryResourceThresholds,
		Status:   model.PolicyStatusDraft,
	}
	if err := svc.CreatePolicy(ctx, pol); err != nil {
		t.Fatalf("CreatePolicy failed: %v", err)
	}

	// 1. Publish revision with auto-increment
	rev1 := &model.PolicyRevision{
		PolicyID:        "pol-rev-test",
		Priority:        100,
		InheritanceMode: model.InheritanceModeInheritAndOverride,
		EnforcementMode: model.EnforcementModeEnforce,
		Rules: []model.PolicyRule{
			{
				ID:   "r-1",
				Name: "Rule 1",
				Type: model.RuleTypeResourceThreshold,
				ResourceThreshold: &model.ResourceThresholdRuleConfig{
					Metric:            "cpu_usage_pct",
					WarningThreshold:  70,
					CriticalThreshold: 85,
				},
			},
		},
	}
	if err := svc.PublishRevision(ctx, rev1); err != nil {
		t.Fatalf("PublishRevision 1 failed: %v", err)
	}
	if rev1.Revision != 1 {
		t.Errorf("expected auto-assigned revision 1, got %d", rev1.Revision)
	}
	if rev1.ContentDigest == "" {
		t.Errorf("expected computed ContentDigest, got empty")
	}

	// 2. Publish revision 2 with auto-increment and selector validation
	rev2 := &model.PolicyRevision{
		PolicyID:        "pol-rev-test",
		Selector:        `env == "production" and cpu_cores >= 4`,
		Priority:        200,
		InheritanceMode: model.InheritanceModeStrictOverride,
		EnforcementMode: model.EnforcementModeEnforce,
		Rules: []model.PolicyRule{
			{
				ID:   "r-2",
				Name: "Rule 2",
				Type: model.RuleTypeResourceThreshold,
				ResourceThreshold: &model.ResourceThresholdRuleConfig{
					Metric:            "cpu_usage_pct",
					WarningThreshold:  65,
					CriticalThreshold: 80,
				},
			},
		},
	}
	if err := svc.PublishRevision(ctx, rev2); err != nil {
		t.Fatalf("PublishRevision 2 failed: %v", err)
	}
	if rev2.Revision != 2 {
		t.Errorf("expected auto-assigned revision 2, got %d", rev2.Revision)
	}

	// 3. Digest mismatch check
	rev3 := &model.PolicyRevision{
		PolicyID:        "pol-rev-test",
		Revision:        3,
		ContentDigest:   "0000000000000000000000000000000000000000000000000000000000000000",
		InheritanceMode: model.InheritanceModeInheritAndOverride,
		EnforcementMode: model.EnforcementModeEnforce,
		Rules: []model.PolicyRule{
			{
				ID:   "r-3",
				Name: "Rule 3",
				Type: model.RuleTypeResourceThreshold,
				ResourceThreshold: &model.ResourceThresholdRuleConfig{
					Metric:            "cpu_usage_pct",
					WarningThreshold:  50,
					CriticalThreshold: 75,
				},
			},
		},
	}
	if err := svc.PublishRevision(ctx, rev3); !errors.Is(err, ErrPolicyRevisionDigestMismatch) {
		t.Errorf("expected ErrPolicyRevisionDigestMismatch, got %v", err)
	}

	// 4. Invalid selector syntax rejected
	revInvalidSelector := &model.PolicyRevision{
		PolicyID:        "pol-rev-test",
		Revision:        3,
		Selector:        `env == [unterminated`,
		InheritanceMode: model.InheritanceModeInheritAndOverride,
		EnforcementMode: model.EnforcementModeEnforce,
		Rules: []model.PolicyRule{
			{
				ID:   "r-3",
				Name: "Rule 3",
				Type: model.RuleTypeResourceThreshold,
				ResourceThreshold: &model.ResourceThresholdRuleConfig{
					Metric:            "cpu_usage_pct",
					WarningThreshold:  50,
					CriticalThreshold: 75,
				},
			},
		},
	}
	if err := svc.PublishRevision(ctx, revInvalidSelector); !errors.Is(err, ErrInvalidSelector) {
		t.Errorf("expected ErrInvalidSelector, got %v", err)
	}

	// 5. Set active revision and verify
	if err := svc.SetActiveRevision(ctx, "pol-rev-test", 2); err != nil {
		t.Fatalf("SetActiveRevision failed: %v", err)
	}
	polFetched, err := svc.GetPolicy(ctx, "pol-rev-test")
	if err != nil || polFetched.ActiveRevision != 2 {
		t.Fatalf("expected ActiveRevision=2, got %v", polFetched)
	}

	// SetActiveRevision with non-existent revision
	if err := svc.SetActiveRevision(ctx, "pol-rev-test", 99); !errors.Is(err, ErrPolicyRevisionNotFound) {
		t.Errorf("expected ErrPolicyRevisionNotFound, got %v", err)
	}

	// 6. GetPolicyRevision and ListPolicyRevisions
	r1Fetched, err := svc.GetPolicyRevision(ctx, "pol-rev-test", 1)
	if err != nil || r1Fetched.Revision != 1 {
		t.Errorf("GetPolicyRevision 1 failed: %v", err)
	}
	revsList, err := svc.ListPolicyRevisions(ctx, "pol-rev-test")
	if err != nil || len(revsList) != 2 {
		t.Fatalf("expected 2 revisions in list, got %d (err: %v)", len(revsList), err)
	}
}

func TestGovernanceService_Assignment_Operations_And_Tenant_Isolation(t *testing.T) {
	svc, store := setupTestService(t)
	defer store.Close()
	ctx := context.Background()

	// Create a secondary organization
	orgSec := &model.Organization{
		ID:     "org-sec",
		Name:   "Security Org",
		Status: model.OrgStatusActive,
	}
	if err := svc.CreateOrganization(ctx, orgSec); err != nil {
		t.Fatalf("CreateOrganization failed: %v", err)
	}

	// Create policy in default org
	polDef := &model.Policy{
		ID:       "pol-def",
		OrgID:    model.DefaultOrganizationID,
		Name:     "Default Policy",
		Category: model.PolicyCategoryResourceThresholds,
		Status:   model.PolicyStatusActive,
	}
	if err := svc.CreatePolicy(ctx, polDef); err != nil {
		t.Fatalf("CreatePolicy failed: %v", err)
	}

	// Create policy in sec org
	polSec := &model.Policy{
		ID:       "pol-sec",
		OrgID:    "org-sec",
		Name:     "Sec Policy",
		Category: model.PolicyCategoryResourceThresholds,
		Status:   model.PolicyStatusActive,
	}
	if err := svc.CreatePolicy(ctx, polSec); err != nil {
		t.Fatalf("CreatePolicy failed: %v", err)
	}

	// Create fleet group in default org
	grpDef := &model.FleetGroup{
		ID:    "grp-def",
		OrgID: model.DefaultOrganizationID,
		Name:  "Default Group",
	}
	if err := svc.CreateFleetGroup(ctx, grpDef); err != nil {
		t.Fatalf("CreateFleetGroup failed: %v", err)
	}

	// Create fleet group in sec org
	grpSec := &model.FleetGroup{
		ID:    "grp-sec",
		OrgID: "org-sec",
		Name:  "Sec Group",
	}
	if err := svc.CreateFleetGroup(ctx, grpSec); err != nil {
		t.Fatalf("CreateFleetGroup failed: %v", err)
	}

	// Create fleet node
	node := &model.FleetNode{
		Identity: model.NodeIdentity{
			NodeID:   "node-tenant-01",
			Hostname: "tenant-01",
		},
		Status:        model.NodeStatusHealthy,
		LastHeartbeat: time.Now().UTC(),
		Metadata: map[string]string{
			"org_id": "org-sec",
		},
	}
	if err := store.SaveFleetNode(ctx, node); err != nil {
		t.Fatalf("SaveFleetNode failed: %v", err)
	}

	// 1. Cross-org policy assignment rejection
	asgnCrossOrg := &model.PolicyAssignment{
		ID:         "asgn-cross-1",
		PolicyID:   "pol-def", // policy in default org
		OrgID:      "org-sec", // assigned in org-sec
		TargetType: model.TargetTypeOrganization,
		TargetID:   "org-sec",
		Enabled:    true,
	}
	if err := svc.CreateAssignment(ctx, asgnCrossOrg); !errors.Is(err, ErrCrossOrgAssignment) {
		t.Errorf("expected ErrCrossOrgAssignment, got %v", err)
	}

	// 2. Cross-org fleet group assignment rejection
	asgnCrossGrp := &model.PolicyAssignment{
		ID:         "asgn-cross-2",
		PolicyID:   "pol-def",
		OrgID:      model.DefaultOrganizationID,
		TargetType: model.TargetTypeFleetGroup,
		TargetID:   "grp-sec", // group belongs to org-sec
		Enabled:    true,
	}
	if err := svc.CreateAssignment(ctx, asgnCrossGrp); !errors.Is(err, ErrCrossOrgAssignment) {
		t.Errorf("expected ErrCrossOrgAssignment on cross-group target, got %v", err)
	}

	// 3. TargetTypeOrganization mismatch target ID
	asgnMismatch := &model.PolicyAssignment{
		ID:         "asgn-mismatch",
		PolicyID:   "pol-def",
		OrgID:      model.DefaultOrganizationID,
		TargetType: model.TargetTypeOrganization,
		TargetID:   "different-target-id",
		Enabled:    true,
	}
	if err := svc.CreateAssignment(ctx, asgnMismatch); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput when org target ID doesn't match org_id, got %v", err)
	}

	// 4. Valid assignment creations
	asgnOrg := &model.PolicyAssignment{
		ID:         "asgn-org-valid",
		PolicyID:   "pol-def",
		OrgID:      model.DefaultOrganizationID,
		TargetType: model.TargetTypeOrganization,
		TargetID:   model.DefaultOrganizationID,
		Enabled:    true,
	}
	if err := svc.CreateAssignment(ctx, asgnOrg); err != nil {
		t.Fatalf("CreateAssignment Org failed: %v", err)
	}

	asgnGrp := &model.PolicyAssignment{
		ID:         "asgn-grp-valid",
		PolicyID:   "pol-def",
		OrgID:      model.DefaultOrganizationID,
		TargetType: model.TargetTypeFleetGroup,
		TargetID:   "grp-def",
		Enabled:    true,
	}
	if err := svc.CreateAssignment(ctx, asgnGrp); err != nil {
		t.Fatalf("CreateAssignment Grp failed: %v", err)
	}

	// 5. GetAssignment and ListAssignments
	fetchedAsgn, err := svc.GetAssignment(ctx, "asgn-org-valid")
	if err != nil || fetchedAsgn.PolicyID != "pol-def" {
		t.Fatalf("GetAssignment failed: %v", err)
	}
	asgnList, err := svc.ListAssignments(ctx, model.PolicyAssignmentFilter{
		OrgID:    model.DefaultOrganizationID,
		PolicyID: "pol-def",
	})
	if err != nil || len(asgnList) != 2 {
		t.Fatalf("expected 2 assignments, got %d (err: %v)", len(asgnList), err)
	}

	// 6. SetAssignmentEnabled
	if err := svc.SetAssignmentEnabled(ctx, "asgn-org-valid", false); err != nil {
		t.Fatalf("SetAssignmentEnabled failed: %v", err)
	}
	fetchedAsgn, err = svc.GetAssignment(ctx, "asgn-org-valid")
	if err != nil || fetchedAsgn.Enabled != false {
		t.Fatalf("expected assignment disabled, got %v", fetchedAsgn)
	}

	// 7. DeleteAssignment
	if err := svc.DeleteAssignment(ctx, "asgn-org-valid"); err != nil {
		t.Fatalf("DeleteAssignment failed: %v", err)
	}
	if _, err := svc.GetAssignment(ctx, "asgn-org-valid"); !errors.Is(err, ErrPolicyAssignmentNotFound) {
		t.Errorf("expected ErrPolicyAssignmentNotFound after delete, got %v", err)
	}
}

func TestGovernanceService_Resolution_And_Compliance(t *testing.T) {
	svc, store := setupTestService(t)
	defer store.Close()
	ctx := context.Background()

	// Setup node and hierarchy
	node := &model.FleetNode{
		Identity: model.NodeIdentity{
			NodeID:   "node-svc-01",
			Hostname: "svc-01",
		},
		Status:        model.NodeStatusHealthy,
		LastHeartbeat: time.Now().UTC(),
		Metadata: map[string]string{
			"env": "production",
		},
	}
	if err := store.SaveFleetNode(ctx, node); err != nil {
		t.Fatalf("SaveFleetNode failed: %v", err)
	}

	pol := &model.Policy{
		ID:             "pol-svc-res",
		OrgID:          model.DefaultOrganizationID,
		Name:           "Service Resolution Policy",
		Category:       model.PolicyCategoryOperationalCompliance,
		Status:         model.PolicyStatusActive,
		ActiveRevision: 1,
	}
	if err := svc.CreatePolicy(ctx, pol); err != nil {
		t.Fatalf("CreatePolicy failed: %v", err)
	}

	rev := &model.PolicyRevision{
		PolicyID:        "pol-svc-res",
		Revision:        1,
		Priority:        100,
		InheritanceMode: model.InheritanceModeInheritAndOverride,
		EnforcementMode: model.EnforcementModeEnforce,
		Rules: []model.PolicyRule{
			{
				ID:      "comp-hb-svc",
				Name:    "Heartbeat Check",
				Type:    model.RuleTypeOperationalCompliance,
				Enabled: true,
				OperationalCompliance: &model.OperationalComplianceRuleConfig{
					CheckType:         "heartbeat_freshness",
					MaxAgeSeconds:     300,
					ViolationSeverity: model.SeverityCritical,
				},
			},
		},
	}
	if err := svc.PublishRevision(ctx, rev); err != nil {
		t.Fatalf("PublishRevision failed: %v", err)
	}

	asgn := &model.PolicyAssignment{
		ID:         "asgn-svc-res",
		PolicyID:   "pol-svc-res",
		OrgID:      model.DefaultOrganizationID,
		TargetType: model.TargetTypeOrganization,
		TargetID:   model.DefaultOrganizationID,
		Enabled:    true,
	}
	if err := svc.CreateAssignment(ctx, asgn); err != nil {
		t.Fatalf("CreateAssignment failed: %v", err)
	}

	// 1. ResolveNodePolicies
	res, err := svc.ResolveNodePolicies(ctx, "node-svc-01")
	if err != nil {
		t.Fatalf("ResolveNodePolicies failed: %v", err)
	}
	if len(res.EffectiveRules) != 1 {
		t.Fatalf("expected 1 effective rule, got %d", len(res.EffectiveRules))
	}

	// 2. EvaluateNodeCompliance
	comp, err := svc.EvaluateNodeCompliance(ctx, "node-svc-01")
	if err != nil {
		t.Fatalf("EvaluateNodeCompliance failed: %v", err)
	}
	if !comp.Compliant {
		t.Errorf("expected node to be compliant, got violations: %+v", comp.Violations)
	}

	// 3. ExplainResolution
	exp, err := svc.ExplainResolution(ctx, "node-svc-01")
	if err != nil {
		t.Fatalf("ExplainResolution failed: %v", err)
	}
	if exp.NodeID != "node-svc-01" || len(exp.AuditTrail) == 0 {
		t.Errorf("invalid explanation output: %+v", exp)
	}

	// 4. DetectPolicyConflicts between two revisions
	// Create another policy with conflicting compliance rule
	polB := &model.Policy{
		ID:             "pol-svc-b",
		OrgID:          model.DefaultOrganizationID,
		Name:           "Service Conflict Policy",
		Category:       model.PolicyCategoryOperationalCompliance,
		Status:         model.PolicyStatusActive,
		ActiveRevision: 1,
	}
	if err := svc.CreatePolicy(ctx, polB); err != nil {
		t.Fatalf("CreatePolicy polB failed: %v", err)
	}
	revB := &model.PolicyRevision{
		PolicyID:        "pol-svc-b",
		Revision:        1,
		Priority:        100,
		InheritanceMode: model.InheritanceModeInheritAndOverride,
		EnforcementMode: model.EnforcementModeEnforce,
		Rules: []model.PolicyRule{
			{
				ID:      "comp-hb-b",
				Name:    "Heartbeat Check B",
				Type:    model.RuleTypeOperationalCompliance,
				Enabled: true,
				OperationalCompliance: &model.OperationalComplianceRuleConfig{
					CheckType:         "heartbeat_freshness",
					MaxAgeSeconds:     60,
					ViolationSeverity: model.SeverityCritical,
				},
			},
		},
	}
	if err := svc.PublishRevision(ctx, revB); err != nil {
		t.Fatalf("PublishRevision revB failed: %v", err)
	}

	conflicts, err := svc.DetectPolicyConflicts(ctx, "pol-svc-res", 1, "pol-svc-b", 1)
	if err != nil {
		t.Fatalf("DetectPolicyConflicts failed: %v", err)
	}
	if len(conflicts) != 1 {
		t.Fatalf("expected 1 potential conflict, got %d", len(conflicts))
	}
	if conflicts[0].Severity != ConflictSeverityPotential {
		t.Errorf("expected potential severity, got %s", conflicts[0].Severity)
	}
}
