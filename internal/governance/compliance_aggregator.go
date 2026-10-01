package governance

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/DocHoax/watchdog/internal/storage"
	"github.com/DocHoax/watchdog/pkg/model"
)

// ComplianceAggregator compiles hierarchical compliance metrics and rollups across nodes, groups, and organizations.
type ComplianceAggregator struct {
	store storage.ReadOnlyStorage
	clock Clock
}

// NewComplianceAggregator constructs a new ComplianceAggregator.
func NewComplianceAggregator(store storage.ReadOnlyStorage, clock Clock) *ComplianceAggregator {
	if clock == nil {
		clock = RealClock{}
	}
	return &ComplianceAggregator{
		store: store,
		clock: clock,
	}
}

// AggregateNodeSummary computes the rollup summary for a single node based on its latest evaluation and findings.
func (a *ComplianceAggregator) AggregateNodeSummary(
	ctx context.Context,
	orgID string,
	nodeID string,
) (*model.ComplianceRollupSummary, error) {
	if strings.TrimSpace(nodeID) == "" {
		return nil, fmt.Errorf("%w: empty node id", ErrInvalidInput)
	}
	if orgID == "" {
		orgID = model.DefaultOrganizationID
	}

	exec, _ := a.store.GetLatestNodeEvaluation(ctx, orgID, nodeID)

	findings, err := a.store.ListComplianceFindings(ctx, model.FindingFilter{
		OrgID:        orgID,
		TargetNodeID: nodeID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list findings for node %s: %w", nodeID, err)
	}

	var openFindings []model.ComplianceFinding
	for _, f := range findings {
		if f.Status == model.FindingStatusOpen || f.Status == model.FindingStatusRecurring {
			openFindings = append(openFindings, f)
		}
	}

	var execs []*model.EvaluationExecution
	if exec != nil {
		execs = append(execs, exec)
	}

	return RollupExecutionsAndFindings(
		model.ComplianceRollupLevelNode,
		nodeID,
		orgID,
		1,
		execs,
		openFindings,
		a.clock.Now(),
	), nil
}

// AggregateGroupSummary computes aggregated compliance rollup for a fleet group and its member nodes.
func (a *ComplianceAggregator) AggregateGroupSummary(
	ctx context.Context,
	orgID string,
	groupID string,
	includeSubgroups bool,
) (*model.ComplianceRollupSummary, error) {
	if strings.TrimSpace(groupID) == "" {
		return nil, fmt.Errorf("%w: empty group id", ErrInvalidInput)
	}
	if orgID == "" {
		orgID = model.DefaultOrganizationID
	}

	targetGroupIDs := map[string]bool{groupID: true}
	if includeSubgroups {
		allGroups, err := a.store.ListFleetGroups(ctx, orgID)
		if err == nil {
			added := true
			for added {
				added = false
				for _, g := range allGroups {
					if !targetGroupIDs[g.ID] && targetGroupIDs[g.ParentGroupID] {
						targetGroupIDs[g.ID] = true
						added = true
					}
				}
			}
		}
	}

	nodeSet := make(map[string]bool)
	for gid := range targetGroupIDs {
		members, err := a.store.GetGroupMembers(ctx, gid)
		if err != nil {
			continue
		}
		for _, m := range members {
			nodeSet[m.NodeID] = true
		}
	}

	var execs []*model.EvaluationExecution
	for nodeID := range nodeSet {
		exec, err := a.store.GetLatestNodeEvaluation(ctx, orgID, nodeID)
		if err == nil && exec != nil {
			execs = append(execs, exec)
		}
	}

	findings, err := a.store.ListComplianceFindings(ctx, model.FindingFilter{
		OrgID: orgID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list findings: %w", err)
	}

	var groupOpenFindings []model.ComplianceFinding
	for _, f := range findings {
		if nodeSet[f.TargetNodeID] && (f.Status == model.FindingStatusOpen || f.Status == model.FindingStatusRecurring) {
			groupOpenFindings = append(groupOpenFindings, f)
		}
	}

	return RollupExecutionsAndFindings(
		model.ComplianceRollupLevelFleetGroup,
		groupID,
		orgID,
		len(nodeSet),
		execs,
		groupOpenFindings,
		a.clock.Now(),
	), nil
}

// AggregateOrgSummary computes organization-wide compliance rollup across all organization nodes.
func (a *ComplianceAggregator) AggregateOrgSummary(
	ctx context.Context,
	orgID string,
) (*model.ComplianceRollupSummary, error) {
	if orgID == "" {
		orgID = model.DefaultOrganizationID
	}

	allNodes, _, err := a.store.ListFleetNodes(ctx, model.FleetFilter{})
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	var orgNodes []model.FleetNode
	for _, n := range allNodes {
		nodeOrg := model.DefaultOrganizationID
		if n.Metadata != nil {
			if v, ok := n.Metadata["org_id"]; ok && v != "" {
				nodeOrg = v
			} else if v, ok := n.Metadata["organization_id"]; ok && v != "" {
				nodeOrg = v
			}
		}
		if nodeOrg == orgID {
			orgNodes = append(orgNodes, n)
		}
	}

	var execs []*model.EvaluationExecution
	for _, node := range orgNodes {
		exec, err := a.store.GetLatestNodeEvaluation(ctx, orgID, node.Identity.NodeID)
		if err == nil && exec != nil {
			execs = append(execs, exec)
		}
	}

	findings, err := a.store.ListComplianceFindings(ctx, model.FindingFilter{
		OrgID: orgID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list org findings: %w", err)
	}

	var openFindings []model.ComplianceFinding
	for _, f := range findings {
		if f.Status == model.FindingStatusOpen || f.Status == model.FindingStatusRecurring {
			openFindings = append(openFindings, f)
		}
	}

	return RollupExecutionsAndFindings(
		model.ComplianceRollupLevelOrg,
		orgID,
		orgID,
		len(orgNodes),
		execs,
		openFindings,
		a.clock.Now(),
	), nil
}

// AggregateFleetSummary computes global fleet-wide compliance rollup across all organizations and nodes.
func (a *ComplianceAggregator) AggregateFleetSummary(
	ctx context.Context,
) (*model.ComplianceRollupSummary, error) {
	allNodes, _, err := a.store.ListFleetNodes(ctx, model.FleetFilter{})
	if err != nil {
		return nil, fmt.Errorf("failed to list fleet nodes: %w", err)
	}

	var execs []*model.EvaluationExecution
	for _, node := range allNodes {
		nodeOrg := model.DefaultOrganizationID
		if node.Metadata != nil {
			if v, ok := node.Metadata["org_id"]; ok && v != "" {
				nodeOrg = v
			} else if v, ok := node.Metadata["organization_id"]; ok && v != "" {
				nodeOrg = v
			}
		}
		exec, err := a.store.GetLatestNodeEvaluation(ctx, nodeOrg, node.Identity.NodeID)
		if err == nil && exec != nil {
			execs = append(execs, exec)
		}
	}

	findings, err := a.store.ListComplianceFindings(ctx, model.FindingFilter{})
	if err != nil {
		return nil, fmt.Errorf("failed to list fleet findings: %w", err)
	}

	var openFindings []model.ComplianceFinding
	for _, f := range findings {
		if f.Status == model.FindingStatusOpen || f.Status == model.FindingStatusRecurring {
			openFindings = append(openFindings, f)
		}
	}

	return RollupExecutionsAndFindings(
		model.ComplianceRollupLevelFleet,
		"fleet",
		"",
		len(allNodes),
		execs,
		openFindings,
		a.clock.Now(),
	), nil
}

// RollupExecutionsAndFindings builds a complete ComplianceRollupSummary from executions and open findings.
func RollupExecutionsAndFindings(
	level model.ComplianceRollupLevel,
	scopeID string,
	orgID string,
	totalNodes int,
	executions []*model.EvaluationExecution,
	openFindings []model.ComplianceFinding,
	evaluatedAt time.Time,
) *model.ComplianceRollupSummary {
	if evaluatedAt.IsZero() {
		evaluatedAt = time.Now().UTC()
	}

	categorySummaries := make(map[model.PolicyCategory]*model.CategoryComplianceSummary)
	initCategory := func(cat model.PolicyCategory) *model.CategoryComplianceSummary {
		if s, ok := categorySummaries[cat]; ok {
			return s
		}
		s := &model.CategoryComplianceSummary{
			Category: cat,
		}
		categorySummaries[cat] = s
		return s
	}

	for _, cat := range []model.PolicyCategory{
		model.PolicyCategoryResourceThresholds,
		model.PolicyCategoryAnomalyDetection,
		model.PolicyCategoryCapacityPlanning,
		model.PolicyCategoryIncidentSeverity,
		model.PolicyCategoryOperationalCompliance,
	} {
		initCategory(cat)
	}

	severityBreakdown := make(map[model.Severity]int)
	for _, sev := range []model.Severity{
		model.SeverityCritical,
		model.SeverityWarning,
		model.SeverityInfo,
	} {
		severityBreakdown[sev] = 0
	}

	var compliantNodes, nonCompliantNodes, warningNodes int
	var totalRulesEvaluated, totalCompliantRules, totalViolations, totalWarnings int

	for _, exec := range executions {
		switch exec.Status {
		case model.EvaluationStatusCompliant:
			compliantNodes++
		case model.EvaluationStatusNonCompliant:
			nonCompliantNodes++
		case model.EvaluationStatusWarning:
			warningNodes++
		}

		for _, r := range exec.Results {
			if !r.Category.IsValid() {
				continue
			}
			catSum := initCategory(r.Category)
			totalRulesEvaluated++
			catSum.TotalRules++

			switch r.Status {
			case model.EvaluationStatusCompliant:
				totalCompliantRules++
				catSum.CompliantRules++
			case model.EvaluationStatusNonCompliant:
				totalViolations++
				catSum.Violations++
			case model.EvaluationStatusWarning:
				totalWarnings++
				catSum.Warnings++
			}
		}
	}

	for _, fnd := range openFindings {
		if _, ok := severityBreakdown[fnd.Severity]; ok {
			severityBreakdown[fnd.Severity]++
		}
	}

	// Calculate category compliance ratios
	catMap := make(map[model.PolicyCategory]model.CategoryComplianceSummary, len(categorySummaries))
	for cat, s := range categorySummaries {
		denom := s.CompliantRules + s.Violations + s.Warnings
		if denom > 0 {
			s.ComplianceRatio = float64(s.CompliantRules) / float64(denom)
		} else {
			s.ComplianceRatio = 1.0
		}
		catMap[cat] = *s
	}

	// Calculate overall compliance and coverage ratios
	var complianceRatio float64
	totalActiveRules := totalCompliantRules + totalViolations + totalWarnings
	if totalActiveRules > 0 {
		complianceRatio = float64(totalCompliantRules) / float64(totalActiveRules)
	} else if len(executions) > 0 {
		complianceRatio = 1.0
	}

	var coverageRatio float64
	if totalNodes > 0 {
		coverageRatio = float64(len(executions)) / float64(totalNodes)
		if coverageRatio > 1.0 {
			coverageRatio = 1.0
		}
	}

	return &model.ComplianceRollupSummary{
		ScopeLevel:          level,
		ScopeID:             scopeID,
		OrgID:               orgID,
		EvaluatedAt:         evaluatedAt,
		TotalNodes:          totalNodes,
		CompliantNodes:      compliantNodes,
		NonCompliantNodes:   nonCompliantNodes,
		WarningNodes:        warningNodes,
		TotalRulesEvaluated: totalRulesEvaluated,
		TotalFindingsOpen:   len(openFindings),
		ComplianceRatio:     complianceRatio,
		CoverageRatio:       coverageRatio,
		BreakdownByCategory: catMap,
		BreakdownBySeverity: severityBreakdown,
	}
}
