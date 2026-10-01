package governance

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/DocHoax/watchdog/internal/storage"
	"github.com/DocHoax/watchdog/pkg/model"
)

// ResolvedPolicySet represents the complete, hierarchically merged effective policy for a node.
type ResolvedPolicySet struct {
	NodeID          string                `json:"node_id"`
	OrgID           string                `json:"org_id"`
	ResolvedAt      time.Time             `json:"resolved_at"`
	EffectiveRules  []EffectiveRule       `json:"effective_rules"`
	AppliedPolicies []AppliedPolicyInfo   `json:"applied_policies"`
	AuditTrail      []ResolutionAuditStep `json:"audit_trail"`
	Conflicts       []PolicyConflict      `json:"conflicts,omitempty"`
}

// AppliedPolicyInfo summarizes a policy revision that was considered during resolution.
type AppliedPolicyInfo struct {
	PolicyID          string                 `json:"policy_id"`
	PolicyName        string                 `json:"policy_name"`
	Revision          int                    `json:"revision"`
	Category          model.PolicyCategory   `json:"category"`
	TargetType        model.PolicyTargetType `json:"target_type"`
	TargetID          string                 `json:"target_id"`
	Priority          int                    `json:"priority"`
	InheritanceMode   model.InheritanceMode  `json:"inheritance_mode"`
	EnforcementMode   model.EnforcementMode  `json:"enforcement_mode"`
	Selector          string                 `json:"selector,omitempty"`
	SelectorMatched   bool                   `json:"selector_matched"`
	MatchedRulesCount int                    `json:"matched_rules_count"`
}

// EffectiveRule represents an active rule in the resolved policy set with full provenance.
type EffectiveRule struct {
	Rule             model.PolicyRule       `json:"rule"`
	Category         model.PolicyCategory   `json:"category"`
	SourcePolicyID   string                 `json:"source_policy_id"`
	SourcePolicyName string                 `json:"source_policy_name"`
	SourceRevision   int                    `json:"source_revision"`
	SourceTargetType model.PolicyTargetType `json:"source_target_type"`
	SourceTargetID   string                 `json:"source_target_id"`
	EnforcementMode  model.EnforcementMode  `json:"enforcement_mode"`
	Priority         int                    `json:"priority"`
	HierarchyLevel   int                    `json:"hierarchy_level"`
	OverrodeRule     *EffectiveRuleSummary  `json:"overrode_rule,omitempty"`
}

// EffectiveRuleSummary summarizes a rule that was overridden during inheritance.
type EffectiveRuleSummary struct {
	RuleID     string                 `json:"rule_id"`
	PolicyID   string                 `json:"policy_id"`
	Revision   int                    `json:"revision"`
	TargetType model.PolicyTargetType `json:"target_type"`
	TargetID   string                 `json:"target_id"`
}

// ResolutionAuditStep captures a discrete decision in the resolution engine pipeline.
type ResolutionAuditStep struct {
	Step       int                    `json:"step"`
	Action     string                 `json:"action"` // evaluated_assignment, selector_matched, selector_skipped, rule_applied, rule_overridden, category_displaced
	PolicyID   string                 `json:"policy_id,omitempty"`
	Revision   int                    `json:"revision,omitempty"`
	TargetType model.PolicyTargetType `json:"target_type,omitempty"`
	TargetID   string                 `json:"target_id,omitempty"`
	RuleID     string                 `json:"rule_id,omitempty"`
	Details    string                 `json:"details"`
}

// NodeComplianceReport aggregates compliance audit results for a node against operational compliance rules.
type NodeComplianceReport struct {
	NodeID      string                `json:"node_id"`
	OrgID       string                `json:"org_id"`
	EvaluatedAt time.Time             `json:"evaluated_at"`
	Compliant   bool                  `json:"compliant"`
	Violations  []ComplianceViolation `json:"violations,omitempty"`
	Summary     ComplianceSummary     `json:"summary"`
}

// ComplianceViolation details an individual rule breach during compliance evaluation.
type ComplianceViolation struct {
	RuleID          string                `json:"rule_id"`
	PolicyID        string                `json:"policy_id"`
	Category        model.PolicyCategory  `json:"category"`
	Severity        model.Severity        `json:"severity"`
	EnforcementMode model.EnforcementMode `json:"enforcement_mode"`
	Message         string                `json:"message"`
	ActualValue     string                `json:"actual_value,omitempty"`
	ExpectedValue   string                `json:"expected_value,omitempty"`
}

// ComplianceSummary summarizes the compliance scan counts.
type ComplianceSummary struct {
	TotalRulesChecked  int `json:"total_rules_checked"`
	PassedRules        int `json:"passed_rules"`
	ViolationsCount    int `json:"violations_count"`
	CriticalViolations int `json:"critical_violations"`
	WarningViolations  int `json:"warning_violations"`
	AdvisoryViolations int `json:"advisory_violations"`
}

// ResolutionExplanation provides human and machine-readable explainability for policy resolution.
type ResolutionExplanation struct {
	NodeID         string                `json:"node_id"`
	ResolvedAt     time.Time             `json:"resolved_at"`
	HierarchyPath  []HierarchyLevelInfo  `json:"hierarchy_path"`
	EffectiveRules []EffectiveRule       `json:"effective_rules"`
	AuditTrail     []ResolutionAuditStep `json:"audit_trail"`
}

// HierarchyLevelInfo captures one step in the node's organizational ancestry.
type HierarchyLevelInfo struct {
	Level                int                    `json:"level"`
	Type                 model.PolicyTargetType `json:"type"`
	ID                   string                 `json:"id"`
	Name                 string                 `json:"name"`
	AppliedPoliciesCount int                    `json:"applied_policies_count"`
}

// PolicyResolver coordinates target scoping, selector evaluation, and hierarchical rule merging.
type PolicyResolver struct {
	store storage.ReadOnlyStorage
}

// NewPolicyResolver creates a new PolicyResolver instance.
func NewPolicyResolver(store storage.ReadOnlyStorage) *PolicyResolver {
	return &PolicyResolver{
		store: store,
	}
}

type candidateAssignment struct {
	assignment     model.PolicyAssignment
	policy         model.Policy
	revision       model.PolicyRevision
	hierarchyLevel int
}

// ResolveNodePolicies determines the effective policy rule set for a fleet node.
func (r *PolicyResolver) ResolveNodePolicies(ctx context.Context, nodeID string) (*ResolvedPolicySet, error) {
	if strings.TrimSpace(nodeID) == "" {
		return nil, fmt.Errorf("%w: empty node id", ErrInvalidInput)
	}

	node, err := r.store.GetFleetNode(ctx, nodeID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNodeNotFound, err)
	}

	// 1. Determine ownership and org ID
	ownership := model.NodeOwnershipFromMetadata(node.Metadata)
	orgID := model.DefaultOrganizationID
	if node.Metadata != nil {
		if val, ok := node.Metadata["org_id"]; ok && val != "" {
			orgID = val
		} else if val, ok := node.Metadata["organization_id"]; ok && val != "" {
			orgID = val
		}
	}

	// 2. Discover node group membership and hierarchy paths
	directGroups, err := r.store.GetNodeGroups(ctx, nodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to get node groups: %w", err)
	}

	// Collect group chains with depths
	type groupDepthInfo struct {
		group model.FleetGroup
		depth int
	}

	groupMap := make(map[string]groupDepthInfo)
	var groupIDs []string
	var groupPaths []string

	for _, dg := range directGroups {
		// Traverse up to root parent
		curr := dg
		var chain []model.FleetGroup
		visited := make(map[string]bool)

		for {
			if visited[curr.ID] {
				break
			}
			visited[curr.ID] = true
			chain = append([]model.FleetGroup{curr}, chain...) // root first

			if curr.ParentGroupID == "" {
				break
			}
			parent, err := r.store.GetFleetGroup(ctx, curr.ParentGroupID)
			if err != nil || parent == nil {
				break
			}
			curr = *parent
		}

		for depth, g := range chain {
			if existing, ok := groupMap[g.ID]; !ok || depth > existing.depth {
				groupMap[g.ID] = groupDepthInfo{group: g, depth: depth}
			}
			if !slices.Contains(groupIDs, g.ID) {
				groupIDs = append(groupIDs, g.ID)
			}
			if g.Path != "" && !slices.Contains(groupPaths, g.Path) {
				groupPaths = append(groupPaths, g.Path)
			}
		}
	}

	var auditTrail []ResolutionAuditStep
	stepCounter := 1

	addAudit := func(action, polID string, rev int, targetType model.PolicyTargetType, targetID, ruleID, details string) {
		auditTrail = append(auditTrail, ResolutionAuditStep{
			Step:       stepCounter,
			Action:     action,
			PolicyID:   polID,
			Revision:   rev,
			TargetType: targetType,
			TargetID:   targetID,
			RuleID:     ruleID,
			Details:    details,
		})
		stepCounter++
	}

	// 3. Query assignments across hierarchy levels:
	// Level 1: Organization
	// Level 2..99: Groups (ordered by depth)
	// Level 100: Direct Node
	var candidates []candidateAssignment

	// 3a. Organization assignments
	orgAsgns, err := r.store.ListPolicyAssignments(ctx, model.PolicyAssignmentFilter{
		OrgID:       orgID,
		TargetType:  model.TargetTypeOrganization,
		TargetID:    orgID,
		EnabledOnly: true,
	})
	if err == nil {
		for _, asgn := range orgAsgns {
			cand, err := r.loadCandidate(ctx, asgn, 1)
			if err == nil && cand != nil {
				candidates = append(candidates, *cand)
			}
		}
	}

	// 3b. Group assignments
	if len(groupIDs) > 0 {
		groupAsgns, err := r.store.GetAssignmentsForTargets(ctx, orgID, model.TargetTypeFleetGroup, groupIDs)
		if err == nil {
			for _, asgn := range groupAsgns {
				depth := 0
				if gInfo, ok := groupMap[asgn.TargetID]; ok {
					depth = gInfo.depth
				}
				level := 2 + depth
				if level > 99 {
					level = 99
				}
				cand, err := r.loadCandidate(ctx, asgn, level)
				if err == nil && cand != nil {
					candidates = append(candidates, *cand)
				}
			}
		}
	}

	// 3c. Direct Node assignments
	nodeAsgns, err := r.store.ListPolicyAssignments(ctx, model.PolicyAssignmentFilter{
		OrgID:       orgID,
		TargetType:  model.TargetTypeNode,
		TargetID:    nodeID,
		EnabledOnly: true,
	})
	if err == nil {
		for _, asgn := range nodeAsgns {
			cand, err := r.loadCandidate(ctx, asgn, 100)
			if err == nil && cand != nil {
				candidates = append(candidates, *cand)
			}
		}
	}

	// 4. Evaluate selectors and filter applicable candidates
	var applicable []candidateAssignment
	var appliedPolicies []AppliedPolicyInfo

	for _, cand := range candidates {
		addAudit("evaluated_assignment", cand.policy.ID, cand.revision.Revision, cand.assignment.TargetType, cand.assignment.TargetID, "",
			fmt.Sprintf("Evaluating assignment %s on policy %q (rev %d, level %d, priority %d)",
				cand.assignment.ID, cand.policy.Name, cand.revision.Revision, cand.hierarchyLevel, cand.revision.Priority))

		matched, err := MatchesNode(cand.revision.Selector, node, ownership, groupIDs, groupPaths)
		if err != nil {
			addAudit("selector_error", cand.policy.ID, cand.revision.Revision, cand.assignment.TargetType, cand.assignment.TargetID, "",
				fmt.Sprintf("Selector evaluation failed: %v", err))
			continue
		}

		if !matched {
			addAudit("selector_skipped", cand.policy.ID, cand.revision.Revision, cand.assignment.TargetType, cand.assignment.TargetID, "",
				fmt.Sprintf("Selector %q did not match node %s", cand.revision.Selector, nodeID))
			appliedPolicies = append(appliedPolicies, AppliedPolicyInfo{
				PolicyID:          cand.policy.ID,
				PolicyName:        cand.policy.Name,
				Revision:          cand.revision.Revision,
				Category:          cand.policy.Category,
				TargetType:        cand.assignment.TargetType,
				TargetID:          cand.assignment.TargetID,
				Priority:          cand.revision.Priority,
				InheritanceMode:   cand.revision.InheritanceMode,
				EnforcementMode:   cand.revision.EnforcementMode,
				Selector:          cand.revision.Selector,
				SelectorMatched:   false,
				MatchedRulesCount: 0,
			})
			continue
		}

		addAudit("selector_matched", cand.policy.ID, cand.revision.Revision, cand.assignment.TargetType, cand.assignment.TargetID, "",
			fmt.Sprintf("Selector %q matched node %s", cand.revision.Selector, nodeID))

		applicable = append(applicable, cand)
		appliedPolicies = append(appliedPolicies, AppliedPolicyInfo{
			PolicyID:          cand.policy.ID,
			PolicyName:        cand.policy.Name,
			Revision:          cand.revision.Revision,
			Category:          cand.policy.Category,
			TargetType:        cand.assignment.TargetType,
			TargetID:          cand.assignment.TargetID,
			Priority:          cand.revision.Priority,
			InheritanceMode:   cand.revision.InheritanceMode,
			EnforcementMode:   cand.revision.EnforcementMode,
			Selector:          cand.revision.Selector,
			SelectorMatched:   true,
			MatchedRulesCount: len(cand.revision.Rules),
		})
	}

	// 5. Sort applicable candidates deterministically by:
	// Hierarchy level ASC (low precedence first)
	// Priority ASC (low priority first, so higher priority is evaluated later and overrides)
	// PolicyID ASC (deterministic tie-breaking)
	sort.Slice(applicable, func(i, j int) bool {
		if applicable[i].hierarchyLevel != applicable[j].hierarchyLevel {
			return applicable[i].hierarchyLevel < applicable[j].hierarchyLevel
		}
		if applicable[i].revision.Priority != applicable[j].revision.Priority {
			return applicable[i].revision.Priority < applicable[j].revision.Priority
		}
		return applicable[i].policy.ID < applicable[j].policy.ID
	})

	// 6. Merge rules according to inheritance mode
	// Map from rule identity key -> EffectiveRule
	effectiveMap := make(map[string]EffectiveRule)
	// Category index to track all keys belonging to each category (for strict_override)
	categoryKeys := make(map[model.PolicyCategory][]string)

	for _, cand := range applicable {
		rev := cand.revision
		pol := cand.policy
		asgn := cand.assignment

		if rev.InheritanceMode == model.InheritanceModeStrictOverride {
			// Strict override displaces all previous rules in the category
			if oldKeys, ok := categoryKeys[pol.Category]; ok {
				for _, k := range oldKeys {
					delete(effectiveMap, k)
				}
				categoryKeys[pol.Category] = nil
				addAudit("category_displaced", pol.ID, rev.Revision, asgn.TargetType, asgn.TargetID, "",
					fmt.Sprintf("Strict override displaced previous rules in category %s", pol.Category))
			}
		}

		for _, rule := range rev.Rules {
			if !rule.Enabled {
				continue
			}

			ruleKey := getRuleKey(pol.Category, rule)

			var overrode *EffectiveRuleSummary
			if existing, found := effectiveMap[ruleKey]; found {
				overrode = &EffectiveRuleSummary{
					RuleID:     existing.Rule.ID,
					PolicyID:   existing.SourcePolicyID,
					Revision:   existing.SourceRevision,
					TargetType: existing.SourceTargetType,
					TargetID:   existing.SourceTargetID,
				}
				addAudit("rule_overridden", pol.ID, rev.Revision, asgn.TargetType, asgn.TargetID, rule.ID,
					fmt.Sprintf("Rule %q overrode earlier rule %q from policy %q@v%d",
						rule.ID, existing.Rule.ID, existing.SourcePolicyID, existing.SourceRevision))
			} else {
				addAudit("rule_applied", pol.ID, rev.Revision, asgn.TargetType, asgn.TargetID, rule.ID,
					fmt.Sprintf("Rule %q applied to effective policy set", rule.ID))
			}

			eff := EffectiveRule{
				Rule:             rule,
				Category:         pol.Category,
				SourcePolicyID:   pol.ID,
				SourcePolicyName: pol.Name,
				SourceRevision:   rev.Revision,
				SourceTargetType: asgn.TargetType,
				SourceTargetID:   asgn.TargetID,
				EnforcementMode:  rev.EnforcementMode,
				Priority:         rev.Priority,
				HierarchyLevel:   cand.hierarchyLevel,
				OverrodeRule:     overrode,
			}

			if rev.InheritanceMode == model.InheritanceModeAdditive && overrode != nil {
				// For additive mode, append with unique key to preserve both
				additiveKey := fmt.Sprintf("%s:%s:%d", ruleKey, pol.ID, rev.Revision)
				effectiveMap[additiveKey] = eff
				categoryKeys[pol.Category] = append(categoryKeys[pol.Category], additiveKey)
			} else {
				effectiveMap[ruleKey] = eff
				if !slices.Contains(categoryKeys[pol.Category], ruleKey) {
					categoryKeys[pol.Category] = append(categoryKeys[pol.Category], ruleKey)
				}
			}
		}
	}

	// 7. Flatten effective rules sorted deterministically
	var effectiveRules []EffectiveRule
	for _, eff := range effectiveMap {
		effectiveRules = append(effectiveRules, eff)
	}

	sort.Slice(effectiveRules, func(i, j int) bool {
		if effectiveRules[i].Category != effectiveRules[j].Category {
			return effectiveRules[i].Category < effectiveRules[j].Category
		}
		if effectiveRules[i].HierarchyLevel != effectiveRules[j].HierarchyLevel {
			return effectiveRules[i].HierarchyLevel > effectiveRules[j].HierarchyLevel
		}
		if effectiveRules[i].Priority != effectiveRules[j].Priority {
			return effectiveRules[i].Priority > effectiveRules[j].Priority
		}
		return effectiveRules[i].Rule.ID < effectiveRules[j].Rule.ID
	})

	// 8. Run conflict detection on effective rules
	conflicts := DetectEffectiveRuleConflicts(effectiveRules)

	return &ResolvedPolicySet{
		NodeID:          nodeID,
		OrgID:           orgID,
		ResolvedAt:      time.Now().UTC(),
		EffectiveRules:  effectiveRules,
		AppliedPolicies: appliedPolicies,
		AuditTrail:      auditTrail,
		Conflicts:       conflicts,
	}, nil
}

func (r *PolicyResolver) loadCandidate(ctx context.Context, asgn model.PolicyAssignment, level int) (*candidateAssignment, error) {
	pol, err := r.store.GetPolicy(ctx, asgn.PolicyID)
	if err != nil {
		return nil, err
	}
	if pol.Status != model.PolicyStatusActive || pol.ActiveRevision <= 0 {
		return nil, nil // Policy not active
	}
	if pol.OrgID != asgn.OrgID {
		return nil, fmt.Errorf("%w: policy org %q != assignment org %q", ErrCrossOrgAssignment, pol.OrgID, asgn.OrgID)
	}

	rev, err := r.store.GetPolicyRevision(ctx, pol.ID, pol.ActiveRevision)
	if err != nil {
		return nil, err
	}

	return &candidateAssignment{
		assignment:     asgn,
		policy:         *pol,
		revision:       *rev,
		hierarchyLevel: level,
	}, nil
}

func getRuleKey(cat model.PolicyCategory, rule model.PolicyRule) string {
	switch rule.Type {
	case model.RuleTypeResourceThreshold:
		if rule.ResourceThreshold != nil && rule.ResourceThreshold.Metric != "" {
			return fmt.Sprintf("%s:metric:%s", cat, strings.ToLower(rule.ResourceThreshold.Metric))
		}
	case model.RuleTypeAnomalyDetection:
		if rule.AnomalyDetection != nil && rule.AnomalyDetection.Metric != "" {
			return fmt.Sprintf("%s:metric:%s", cat, strings.ToLower(rule.AnomalyDetection.Metric))
		}
	case model.RuleTypeCapacityPlanning:
		if rule.CapacityPlanning != nil && rule.CapacityPlanning.Metric != "" {
			return fmt.Sprintf("%s:metric:%s", cat, strings.ToLower(rule.CapacityPlanning.Metric))
		}
	case model.RuleTypeOperationalCompliance:
		if rule.OperationalCompliance != nil && rule.OperationalCompliance.CheckType != "" {
			return fmt.Sprintf("%s:check:%s", cat, strings.ToLower(rule.OperationalCompliance.CheckType))
		}
	}
	return fmt.Sprintf("%s:rule:%s", cat, rule.ID)
}

// EvaluateNodeCompliance checks a node against all active operational compliance rules in its effective policy set.
func (r *PolicyResolver) EvaluateNodeCompliance(ctx context.Context, nodeID string) (*NodeComplianceReport, error) {
	resolved, err := r.ResolveNodePolicies(ctx, nodeID)
	if err != nil {
		return nil, err
	}

	node, err := r.store.GetFleetNode(ctx, nodeID)
	if err != nil {
		return nil, err
	}

	var violations []ComplianceViolation
	totalChecked := 0
	passedCount := 0

	for _, eff := range resolved.EffectiveRules {
		if eff.Category != model.PolicyCategoryOperationalCompliance || eff.Rule.Type != model.RuleTypeOperationalCompliance {
			continue
		}
		cfg := eff.Rule.OperationalCompliance
		if cfg == nil {
			continue
		}

		totalChecked++
		violation := evaluateComplianceCheck(node, eff.Rule.ID, eff.SourcePolicyID, eff.Category, cfg, eff.EnforcementMode)
		if violation != nil {
			violations = append(violations, *violation)
		} else {
			passedCount++
		}
	}

	summary := ComplianceSummary{
		TotalRulesChecked: totalChecked,
		PassedRules:       passedCount,
		ViolationsCount:   len(violations),
	}

	for _, v := range violations {
		if v.EnforcementMode == model.EnforcementModeAdvisory || v.EnforcementMode == model.EnforcementModeDryRun {
			summary.AdvisoryViolations++
		} else {
			switch v.Severity {
			case model.SeverityCritical:
				summary.CriticalViolations++
			case model.SeverityWarning:
				summary.WarningViolations++
			default:
				summary.AdvisoryViolations++
			}
		}
	}

	return &NodeComplianceReport{
		NodeID:      nodeID,
		OrgID:       resolved.OrgID,
		EvaluatedAt: time.Now().UTC(),
		Compliant:   len(violations) == 0,
		Violations:  violations,
		Summary:     summary,
	}, nil
}

func evaluateComplianceCheck(node *model.FleetNode, ruleID, polID string, cat model.PolicyCategory, cfg *model.OperationalComplianceRuleConfig, enf model.EnforcementMode) *ComplianceViolation {
	checkType := strings.ToLower(strings.TrimSpace(cfg.CheckType))

	switch checkType {
	case "heartbeat_freshness":
		if cfg.MaxAgeSeconds > 0 {
			age := 0.0
			if !node.LastHeartbeat.IsZero() {
				age = time.Since(node.LastHeartbeat).Seconds()
			}
			if node.LastHeartbeat.IsZero() || age > float64(cfg.MaxAgeSeconds) {
				actual := "never"
				if !node.LastHeartbeat.IsZero() {
					actual = fmt.Sprintf("%.1fs", age)
				}
				return &ComplianceViolation{
					RuleID:          ruleID,
					PolicyID:        polID,
					Category:        cat,
					Severity:        cfg.ViolationSeverity,
					EnforcementMode: enf,
					Message:         fmt.Sprintf("Node heartbeat age %s exceeds maximum allowable freshness of %ds", actual, cfg.MaxAgeSeconds),
					ActualValue:     actual,
					ExpectedValue:   fmt.Sprintf("<= %ds", cfg.MaxAgeSeconds),
				}
			}
		}
	case "collector_version":
		if cfg.ExpectedValue != "" && node.Identity.Version != cfg.ExpectedValue {
			return &ComplianceViolation{
				RuleID:          ruleID,
				PolicyID:        polID,
				Category:        cat,
				Severity:        cfg.ViolationSeverity,
				EnforcementMode: enf,
				Message:         fmt.Sprintf("Node collector version %q does not match required version %q", node.Identity.Version, cfg.ExpectedValue),
				ActualValue:     node.Identity.Version,
				ExpectedValue:   cfg.ExpectedValue,
			}
		}
		if len(cfg.ExpectedValues) > 0 && !slices.Contains(cfg.ExpectedValues, node.Identity.Version) {
			return &ComplianceViolation{
				RuleID:          ruleID,
				PolicyID:        polID,
				Category:        cat,
				Severity:        cfg.ViolationSeverity,
				EnforcementMode: enf,
				Message:         fmt.Sprintf("Node collector version %q is not among approved versions %v", node.Identity.Version, cfg.ExpectedValues),
				ActualValue:     node.Identity.Version,
				ExpectedValue:   strings.Join(cfg.ExpectedValues, ", "),
			}
		}
	case "mandatory_tags":
		if cfg.ExpectedValue != "" {
			reqTags := strings.Split(cfg.ExpectedValue, ",")
			var missing []string
			for _, rt := range reqTags {
				k := strings.TrimSpace(rt)
				if k != "" {
					if _, has := node.Identity.Tags[k]; !has {
						missing = append(missing, k)
					}
				}
			}
			if len(missing) > 0 {
				return &ComplianceViolation{
					RuleID:          ruleID,
					PolicyID:        polID,
					Category:        cat,
					Severity:        cfg.ViolationSeverity,
					EnforcementMode: enf,
					Message:         fmt.Sprintf("Node missing mandatory tag keys: %v", missing),
					ActualValue:     fmt.Sprintf("%d tags present", len(node.Identity.Tags)),
					ExpectedValue:   cfg.ExpectedValue,
				}
			}
		}
	case "approved_platforms":
		if len(cfg.ExpectedValues) > 0 && !slices.Contains(cfg.ExpectedValues, node.Identity.Platform) {
			return &ComplianceViolation{
				RuleID:          ruleID,
				PolicyID:        polID,
				Category:        cat,
				Severity:        cfg.ViolationSeverity,
				EnforcementMode: enf,
				Message:         fmt.Sprintf("Node platform %q is not among approved platforms %v", node.Identity.Platform, cfg.ExpectedValues),
				ActualValue:     node.Identity.Platform,
				ExpectedValue:   strings.Join(cfg.ExpectedValues, ", "),
			}
		}
	}

	return nil
}

// ExplainResolution produces a structured explainability report for how policies resolved on a node.
func (r *PolicyResolver) ExplainResolution(ctx context.Context, nodeID string) (*ResolutionExplanation, error) {
	resolved, err := r.ResolveNodePolicies(ctx, nodeID)
	if err != nil {
		return nil, err
	}

	node, err := r.store.GetFleetNode(ctx, nodeID)
	if err != nil {
		return nil, err
	}

	orgID := model.DefaultOrganizationID
	if node.Metadata != nil {
		if val, ok := node.Metadata["org_id"]; ok && val != "" {
			orgID = val
		} else if val, ok := node.Metadata["organization_id"]; ok && val != "" {
			orgID = val
		}
	}

	var hierarchyPath []HierarchyLevelInfo

	// Org level
	org, err := r.store.GetOrganization(ctx, orgID)
	orgName := orgID
	if err == nil && org != nil {
		orgName = org.DisplayName
		if orgName == "" {
			orgName = org.Name
		}
	}
	hierarchyPath = append(hierarchyPath, HierarchyLevelInfo{
		Level:                1,
		Type:                 model.TargetTypeOrganization,
		ID:                   orgID,
		Name:                 orgName,
		AppliedPoliciesCount: countPoliciesForTarget(resolved.AppliedPolicies, model.TargetTypeOrganization, orgID),
	})

	// Group levels
	directGroups, _ := r.store.GetNodeGroups(ctx, nodeID)
	for _, g := range directGroups {
		hierarchyPath = append(hierarchyPath, HierarchyLevelInfo{
			Level:                2,
			Type:                 model.TargetTypeFleetGroup,
			ID:                   g.ID,
			Name:                 g.Name,
			AppliedPoliciesCount: countPoliciesForTarget(resolved.AppliedPolicies, model.TargetTypeFleetGroup, g.ID),
		})
	}

	// Node level
	hierarchyPath = append(hierarchyPath, HierarchyLevelInfo{
		Level:                100,
		Type:                 model.TargetTypeNode,
		ID:                   nodeID,
		Name:                 node.Identity.Hostname,
		AppliedPoliciesCount: countPoliciesForTarget(resolved.AppliedPolicies, model.TargetTypeNode, nodeID),
	})

	return &ResolutionExplanation{
		NodeID:         nodeID,
		ResolvedAt:     resolved.ResolvedAt,
		HierarchyPath:  hierarchyPath,
		EffectiveRules: resolved.EffectiveRules,
		AuditTrail:     resolved.AuditTrail,
	}, nil
}

func countPoliciesForTarget(applied []AppliedPolicyInfo, targetType model.PolicyTargetType, targetID string) int {
	count := 0
	for _, a := range applied {
		if a.TargetType == targetType && a.TargetID == targetID && a.SelectorMatched {
			count++
		}
	}
	return count
}
