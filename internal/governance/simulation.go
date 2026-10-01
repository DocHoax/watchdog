package governance

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/DocHoax/watchdog/internal/incidents"
	"github.com/DocHoax/watchdog/internal/storage"
	"github.com/DocHoax/watchdog/pkg/model"
)

// SimulationEngine performs isolated, ephemeral what-if policy evaluations without persisting mutations or triggering alerts.
type SimulationEngine struct {
	store      storage.ReadOnlyStorage
	evaluators *EvaluatorRegistry
	clock      Clock
}

// NewSimulationEngine constructs a new SimulationEngine with the provided store, evaluator registry, and clock.
func NewSimulationEngine(store storage.ReadOnlyStorage, evaluators *EvaluatorRegistry, clock Clock) *SimulationEngine {
	if evaluators == nil {
		evaluators = NewEvaluatorRegistry()
	}
	if clock == nil {
		clock = RealClock{}
	}
	return &SimulationEngine{
		store:      store,
		evaluators: evaluators,
		clock:      clock,
	}
}

// SimulatePolicyChanges executes candidate policies, revisions, and assignments in an isolated in-memory overlay.
func (e *SimulationEngine) SimulatePolicyChanges(ctx context.Context, req *model.SimulationRequest) (*model.SimulationResult, error) {
	if req == nil {
		return nil, fmt.Errorf("%w: nil simulation request", ErrInvalidInput)
	}
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}

	start := e.clock.Now()
	orgID := req.OrgID
	if orgID == "" {
		orgID = model.DefaultOrganizationID
	}

	// 1. Identify target nodes to evaluate
	targetNodes, err := e.identifyTargetNodes(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to identify simulation target nodes: %w", err)
	}

	if len(targetNodes) == 0 {
		return &model.SimulationResult{
			OrgID:                   orgID,
			SimulatedAt:             start,
			DurationNs:              e.clock.Since(start).Nanoseconds(),
			TotalNodes:              0,
			EvaluatedNodes:          0,
			BaselineComplianceRatio: 1.0,
			ProposedComplianceRatio: 1.0,
			ComplianceRatioDelta:    0.0,
			BaselineRollup: model.ComplianceRollupSummary{
				ScopeLevel:      model.ComplianceRollupLevelOrg,
				ScopeID:         orgID,
				OrgID:           orgID,
				EvaluatedAt:     start,
				ComplianceRatio: 1.0,
				CoverageRatio:   1.0,
			},
			ProposedRollup: model.ComplianceRollupSummary{
				ScopeLevel:      model.ComplianceRollupLevelOrg,
				ScopeID:         orgID,
				OrgID:           orgID,
				EvaluatedAt:     start,
				ComplianceRatio: 1.0,
				CoverageRatio:   1.0,
			},
		}, nil
	}

	// 2. Setup Baseline and Overlay Resolvers & Context Providers
	baselineResolver := NewPolicyResolver(e.store)
	baselineAcquisition := NewDataAcquisitionProvider(e.store, baselineResolver, e.clock, nil)

	overlayStore := newSimulationOverlayStore(e.store, req.ProposedPolicies, req.ProposedRevisions, req.ProposedAssignments)
	proposedResolver := NewPolicyResolver(overlayStore)
	proposedAcquisition := NewDataAcquisitionProvider(overlayStore, proposedResolver, e.clock, nil)

	var baselineExecs []*model.EvaluationExecution
	var proposedExecs []*model.EvaluationExecution
	var nodeImpacts []model.NodeSimulationImpact

	var allBaselineFindings []model.ComplianceFinding
	var allProposedFindings []model.ComplianceFinding

	// 3. Evaluate each node under both Baseline and Proposed overlays
	for _, node := range targetNodes {
		nodeID := node.Identity.NodeID

		// --- Baseline Evaluation ---
		baseCtx, err := baselineAcquisition.BuildEvaluationContext(ctx, nodeID)
		if err != nil {
			baseResolved, rerr := baselineResolver.ResolveNodePolicies(ctx, nodeID)
			if rerr != nil {
				baseResolved = &ResolvedPolicySet{NodeID: nodeID, OrgID: orgID}
			}
			baseCtx = &EvaluationContext{
				Node:              &node,
				OrgID:             orgID,
				ResolvedPolicySet: baseResolved,
				Clock:             e.clock,
			}
		}
		baseResults, _ := e.evaluators.EvaluateContext(ctx, baseCtx)
		baseSummary := model.CalculateSummary(baseResults)
		baseExecStatus := model.EvaluationStatusCompliant
		if baseSummary.NonCompliantRules > 0 {
			baseExecStatus = model.EvaluationStatusNonCompliant
		} else if baseSummary.WarningRules > 0 {
			baseExecStatus = model.EvaluationStatusWarning
		}

		baseExec := &model.EvaluationExecution{
			ID:           "sim-baseline-" + nodeID,
			OrgID:        orgID,
			TargetNodeID: nodeID,
			TriggerType:  model.EvaluationTriggerSimulation,
			EvaluatedAt:  start,
			Status:       baseExecStatus,
			Results:      baseResults,
			Summary:      baseSummary,
		}
		baselineExecs = append(baselineExecs, baseExec)

		// Generate baseline findings
		baseFindings := resultsToFindings(orgID, nodeID, baseResults, start)
		allBaselineFindings = append(allBaselineFindings, baseFindings...)

		// --- Proposed Evaluation ---
		propCtx, err := proposedAcquisition.BuildEvaluationContext(ctx, nodeID)
		if err != nil {
			propResolved, rerr := proposedResolver.ResolveNodePolicies(ctx, nodeID)
			if rerr != nil {
				propResolved = &ResolvedPolicySet{NodeID: nodeID, OrgID: orgID}
			}
			propCtx = &EvaluationContext{
				Node:              &node,
				OrgID:             orgID,
				ResolvedPolicySet: propResolved,
				Clock:             e.clock,
			}
		}
		propResults, _ := e.evaluators.EvaluateContext(ctx, propCtx)
		propSummary := model.CalculateSummary(propResults)
		propExecStatus := model.EvaluationStatusCompliant
		if propSummary.NonCompliantRules > 0 {
			propExecStatus = model.EvaluationStatusNonCompliant
		} else if propSummary.WarningRules > 0 {
			propExecStatus = model.EvaluationStatusWarning
		}

		propExec := &model.EvaluationExecution{
			ID:           "sim-proposed-" + nodeID,
			OrgID:        orgID,
			TargetNodeID: nodeID,
			TriggerType:  model.EvaluationTriggerSimulation,
			EvaluatedAt:  start,
			Status:       propExecStatus,
			Results:      propResults,
			Summary:      propSummary,
		}
		proposedExecs = append(proposedExecs, propExec)

		// Generate proposed findings
		propFindings := resultsToFindings(orgID, nodeID, propResults, start)
		allProposedFindings = append(allProposedFindings, propFindings...)

		// Calculate rule diffs and node impact
		ruleDiffs := diffRuleResults(baseResults, propResults)
		newCount, resolvedCount := countFindingDelta(baseResults, propResults)

		nodeImpact := model.NodeSimulationImpact{
			NodeID:                nodeID,
			Hostname:              node.Identity.Hostname,
			BaselineStatus:        baseExecStatus,
			ProposedStatus:        propExecStatus,
			BaselineRuleCount:     len(baseResults),
			ProposedRuleCount:     len(propResults),
			NewFindingsCount:      newCount,
			ResolvedFindingsCount: resolvedCount,
			RuleDiffs:             ruleDiffs,
		}
		nodeImpacts = append(nodeImpacts, nodeImpact)
	}

	// 4. Calculate Aggregate Rollups
	baseRollup := RollupExecutionsAndFindings(
		model.ComplianceRollupLevelOrg,
		orgID,
		orgID,
		len(targetNodes),
		baselineExecs,
		allBaselineFindings,
		start,
	)

	propRollup := RollupExecutionsAndFindings(
		model.ComplianceRollupLevelOrg,
		orgID,
		orgID,
		len(targetNodes),
		proposedExecs,
		allProposedFindings,
		start,
	)

	// 5. Partition Findings (New, Resolved, Unchanged)
	newFindings, resolvedFindings, unchangedFindings := partitionFindings(allBaselineFindings, allProposedFindings)

	duration := e.clock.Since(start)

	return &model.SimulationResult{
		OrgID:                   orgID,
		SimulatedAt:             start,
		DurationNs:              duration.Nanoseconds(),
		TotalNodes:              len(targetNodes),
		EvaluatedNodes:          len(targetNodes),
		BaselineComplianceRatio: baseRollup.ComplianceRatio,
		ProposedComplianceRatio: propRollup.ComplianceRatio,
		ComplianceRatioDelta:    propRollup.ComplianceRatio - baseRollup.ComplianceRatio,
		BaselineRollup:          *baseRollup,
		ProposedRollup:          *propRollup,
		NewFindings:             newFindings,
		ResolvedFindings:        resolvedFindings,
		UnchangedFindings:       unchangedFindings,
		NodeImpacts:             nodeImpacts,
	}, nil
}

// identifyTargetNodes discovers all nodes matching the target scope of the SimulationRequest.
func (e *SimulationEngine) identifyTargetNodes(ctx context.Context, req *model.SimulationRequest) ([]model.FleetNode, error) {
	orgID := req.OrgID
	if orgID == "" {
		orgID = model.DefaultOrganizationID
	}

	// 1. Direct node IDs specified
	if len(req.TargetNodeIDs) > 0 {
		var nodes []model.FleetNode
		for _, nodeID := range req.TargetNodeIDs {
			node, err := e.store.GetFleetNode(ctx, nodeID)
			if err != nil {
				continue
			}
			nodes = append(nodes, *node)
		}
		return nodes, nil
	}

	// 2. Target Group specified
	if strings.TrimSpace(req.TargetGroupID) != "" {
		targetGroupIDs := map[string]bool{req.TargetGroupID: true}
		if req.IncludeSubgroups {
			allGroups, err := e.store.ListFleetGroups(ctx, orgID)
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
			members, err := e.store.GetGroupMembers(ctx, gid)
			if err != nil {
				continue
			}
			for _, m := range members {
				nodeSet[m.NodeID] = true
			}
		}

		var nodes []model.FleetNode
		for nodeID := range nodeSet {
			node, err := e.store.GetFleetNode(ctx, nodeID)
			if err == nil && node != nil {
				nodes = append(nodes, *node)
			}
		}
		return nodes, nil
	}

	// 3. Organization-wide (all nodes belonging to orgID)
	allNodes, _, err := e.store.ListFleetNodes(ctx, model.FleetFilter{})
	if err != nil {
		return nil, fmt.Errorf("failed to list fleet nodes: %w", err)
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

	return orgNodes, nil
}

// resultsToFindings converts non-compliant or warning EvaluationResults to ComplianceFinding objects.
func resultsToFindings(orgID, nodeID string, results []model.EvaluationResult, evaluatedAt time.Time) []model.ComplianceFinding {
	var findings []model.ComplianceFinding
	for _, r := range results {
		if r.Status == model.EvaluationStatusNonCompliant || r.Status == model.EvaluationStatusWarning {
			findingID := model.ComputeFindingID(orgID, nodeID, r.PolicyID, r.RuleID)
			findings = append(findings, model.ComplianceFinding{
				ID:              findingID,
				OrgID:           orgID,
				TargetNodeID:    nodeID,
				PolicyID:        r.PolicyID,
				RuleID:          r.RuleID,
				Category:        r.Category,
				Severity:        r.Severity,
				EnforcementMode: r.EnforcementMode,
				Status:          model.FindingStatusOpen,
				FirstSeenAt:     evaluatedAt,
				LastSeenAt:      evaluatedAt,
				OccurrenceCount: 1,
				ObservedValue:   r.ObservedValue,
				ExpectedValue:   r.ExpectedValue,
				Message:         r.Message,
			})
		}
	}
	return findings
}

// diffRuleResults calculates the granular before/after diff for evaluated rules on a single node.
func diffRuleResults(baseline, proposed []model.EvaluationResult) []model.RuleImpactDiff {
	baseMap := make(map[string]model.EvaluationResult)
	for _, r := range baseline {
		key := r.PolicyID + ":" + r.RuleID
		baseMap[key] = r
	}

	propMap := make(map[string]model.EvaluationResult)
	for _, r := range proposed {
		key := r.PolicyID + ":" + r.RuleID
		propMap[key] = r
	}

	var diffs []model.RuleImpactDiff

	// Check proposed rules
	for key, p := range propMap {
		if b, ok := baseMap[key]; ok {
			// Rule exists in both
			diffType := model.RuleDiffTypeUnchanged
			if b.Status != p.Status || b.Message != p.Message {
				diffType = model.RuleDiffTypeModified
			}
			diffs = append(diffs, model.RuleImpactDiff{
				PolicyID:        p.PolicyID,
				RuleID:          p.RuleID,
				RuleName:        p.RuleName,
				Category:        p.Category,
				Severity:        p.Severity,
				EnforcementMode: p.EnforcementMode,
				DiffType:        diffType,
				BaselineStatus:  b.Status,
				ProposedStatus:  p.Status,
				BaselineMessage: b.Message,
				ProposedMessage: p.Message,
			})
		} else {
			// Rule is newly added
			diffs = append(diffs, model.RuleImpactDiff{
				PolicyID:        p.PolicyID,
				RuleID:          p.RuleID,
				RuleName:        p.RuleName,
				Category:        p.Category,
				Severity:        p.Severity,
				EnforcementMode: p.EnforcementMode,
				DiffType:        model.RuleDiffTypeAdded,
				ProposedStatus:  p.Status,
				ProposedMessage: p.Message,
			})
		}
	}

	// Check removed rules (present in baseline but not proposed)
	for key, b := range baseMap {
		if _, ok := propMap[key]; !ok {
			diffs = append(diffs, model.RuleImpactDiff{
				PolicyID:        b.PolicyID,
				RuleID:          b.RuleID,
				RuleName:        b.RuleName,
				Category:        b.Category,
				Severity:        b.Severity,
				EnforcementMode: b.EnforcementMode,
				DiffType:        model.RuleDiffTypeRemoved,
				BaselineStatus:  b.Status,
				BaselineMessage: b.Message,
			})
		}
	}

	return diffs
}

// countFindingDelta computes the counts of newly introduced findings and resolved findings on a node.
func countFindingDelta(baseline, proposed []model.EvaluationResult) (newCount int, resolvedCount int) {
	baseMap := make(map[string]model.EvaluationStatus)
	for _, r := range baseline {
		baseMap[r.PolicyID+":"+r.RuleID] = r.Status
	}

	propMap := make(map[string]model.EvaluationStatus)
	for _, r := range proposed {
		propMap[r.PolicyID+":"+r.RuleID] = r.Status
	}

	// Count new findings: proposed is non_compliant or warning, but baseline was compliant or not present
	for key, pStatus := range propMap {
		if pStatus == model.EvaluationStatusNonCompliant || pStatus == model.EvaluationStatusWarning {
			bStatus, exists := baseMap[key]
			if !exists || (bStatus != model.EvaluationStatusNonCompliant && bStatus != model.EvaluationStatusWarning) {
				newCount++
			}
		}
	}

	// Count resolved findings: baseline was non_compliant or warning, but proposed is compliant or no longer applicable/present
	for key, bStatus := range baseMap {
		if bStatus == model.EvaluationStatusNonCompliant || bStatus == model.EvaluationStatusWarning {
			pStatus, exists := propMap[key]
			if !exists || pStatus == model.EvaluationStatusCompliant {
				resolvedCount++
			}
		}
	}

	return newCount, resolvedCount
}

// partitionFindings classifies findings into New, Resolved, and Unchanged sets.
func partitionFindings(baseline, proposed []model.ComplianceFinding) (newF, resolvedF, unchangedF []model.ComplianceFinding) {
	baseMap := make(map[string]model.ComplianceFinding)
	for _, f := range baseline {
		baseMap[f.ID] = f
	}

	propMap := make(map[string]model.ComplianceFinding)
	for _, f := range proposed {
		propMap[f.ID] = f
	}

	for id, pf := range propMap {
		if _, ok := baseMap[id]; ok {
			unchangedF = append(unchangedF, pf)
		} else {
			newF = append(newF, pf)
		}
	}

	for id, bf := range baseMap {
		if _, ok := propMap[id]; !ok {
			resolvedFinding := bf
			resolvedFinding.Status = model.FindingStatusResolved
			resolvedF = append(resolvedF, resolvedFinding)
		}
	}

	return newF, resolvedF, unchangedF
}

// ============================================================================
// Ephemeral In-Memory Storage Overlay
// ============================================================================

type simulationOverlayStore struct {
	base        storage.ReadOnlyStorage
	policies    map[string]model.Policy
	revisions   map[string]model.PolicyRevision   // key: policyID:revision
	assignments map[string]model.PolicyAssignment // key: assignmentID
}

func newSimulationOverlayStore(
	base storage.ReadOnlyStorage,
	proposedPolicies []model.Policy,
	proposedRevisions []model.PolicyRevision,
	proposedAssignments []model.PolicyAssignment,
) *simulationOverlayStore {
	s := &simulationOverlayStore{
		base:        base,
		policies:    make(map[string]model.Policy),
		revisions:   make(map[string]model.PolicyRevision),
		assignments: make(map[string]model.PolicyAssignment),
	}

	for _, p := range proposedPolicies {
		s.policies[p.ID] = p
	}
	for _, r := range proposedRevisions {
		key := fmt.Sprintf("%s:%d", r.PolicyID, r.Revision)
		s.revisions[key] = r
	}
	for _, a := range proposedAssignments {
		s.assignments[a.ID] = a
	}

	return s
}

func (s *simulationOverlayStore) Ping(ctx context.Context) error {
	return s.base.Ping(ctx)
}

func (s *simulationOverlayStore) QueryMetrics(ctx context.Context, q storage.TimeRangeQuery) ([]storage.MetricPoint, error) {
	return s.base.QueryMetrics(ctx, q)
}

func (s *simulationOverlayStore) GetMetricAggregate(ctx context.Context, metric string, start, end time.Time) (*storage.MetricAggregate, error) {
	return s.base.GetMetricAggregate(ctx, metric, start, end)
}

func (s *simulationOverlayStore) GetAvailableMetrics(ctx context.Context) ([]string, error) {
	return s.base.GetAvailableMetrics(ctx)
}

func (s *simulationOverlayStore) GetAlertHistory(ctx context.Context, limit, offset int) ([]model.AlertEvent, error) {
	return s.base.GetAlertHistory(ctx, limit, offset)
}

func (s *simulationOverlayStore) GetActiveAlerts(ctx context.Context) ([]model.AlertEvent, error) {
	return s.base.GetActiveAlerts(ctx)
}

func (s *simulationOverlayStore) GetLatestDiagnosticReport(ctx context.Context) (*model.DiagnosticReport, error) {
	return s.base.GetLatestDiagnosticReport(ctx)
}

func (s *simulationOverlayStore) GetDiagnosticHistory(ctx context.Context, limit int) ([]model.DiagnosticReport, error) {
	return s.base.GetDiagnosticHistory(ctx, limit)
}

func (s *simulationOverlayStore) QueryAuditEvents(ctx context.Context, filter storage.AuditFilter) ([]model.AuditEvent, error) {
	return s.base.QueryAuditEvents(ctx, filter)
}

func (s *simulationOverlayStore) CountAuditEvents(ctx context.Context, filter storage.AuditFilter) (int64, error) {
	return s.base.CountAuditEvents(ctx, filter)
}

func (s *simulationOverlayStore) GetFleetNode(ctx context.Context, nodeID string) (*model.FleetNode, error) {
	return s.base.GetFleetNode(ctx, nodeID)
}

func (s *simulationOverlayStore) ListFleetNodes(ctx context.Context, filter model.FleetFilter) ([]model.FleetNode, int, error) {
	return s.base.ListFleetNodes(ctx, filter)
}

func (s *simulationOverlayStore) GetNodeTelemetrySubmissions(ctx context.Context, nodeID string, since time.Time, limit int) ([]model.TelemetrySubmission, error) {
	return s.base.GetNodeTelemetrySubmissions(ctx, nodeID, since, limit)
}

func (s *simulationOverlayStore) GetDatabaseSize() (int64, error) {
	return s.base.GetDatabaseSize()
}

func (s *simulationOverlayStore) GetIncident(ctx context.Context, id string) (*incidents.Incident, error) {
	return s.base.GetIncident(ctx, id)
}

func (s *simulationOverlayStore) ListIncidents(ctx context.Context, filter incidents.IncidentFilter) ([]incidents.Incident, int, error) {
	return s.base.ListIncidents(ctx, filter)
}

func (s *simulationOverlayStore) GetTimeline(ctx context.Context, incidentID string, filter incidents.TimelineFilter) ([]incidents.IncidentTimelineEntry, error) {
	return s.base.GetTimeline(ctx, incidentID, filter)
}

func (s *simulationOverlayStore) GetIncidentHistory(ctx context.Context, lookback time.Duration) ([]incidents.Incident, error) {
	return s.base.GetIncidentHistory(ctx, lookback)
}

func (s *simulationOverlayStore) GetOrganization(ctx context.Context, id string) (*model.Organization, error) {
	return s.base.GetOrganization(ctx, id)
}

func (s *simulationOverlayStore) ListOrganizations(ctx context.Context) ([]model.Organization, error) {
	return s.base.ListOrganizations(ctx)
}

func (s *simulationOverlayStore) GetFleetGroup(ctx context.Context, id string) (*model.FleetGroup, error) {
	return s.base.GetFleetGroup(ctx, id)
}

func (s *simulationOverlayStore) ListFleetGroups(ctx context.Context, orgID string) ([]model.FleetGroup, error) {
	return s.base.ListFleetGroups(ctx, orgID)
}

func (s *simulationOverlayStore) GetGroupMembers(ctx context.Context, groupID string) ([]model.FleetGroupMember, error) {
	return s.base.GetGroupMembers(ctx, groupID)
}

func (s *simulationOverlayStore) GetNodeGroups(ctx context.Context, nodeID string) ([]model.FleetGroup, error) {
	return s.base.GetNodeGroups(ctx, nodeID)
}

func (s *simulationOverlayStore) GetPolicy(ctx context.Context, id string) (*model.Policy, error) {
	if p, ok := s.policies[id]; ok {
		return &p, nil
	}
	return s.base.GetPolicy(ctx, id)
}

func (s *simulationOverlayStore) ListPolicies(ctx context.Context, filter model.PolicyFilter) ([]model.Policy, error) {
	basePolicies, err := s.base.ListPolicies(ctx, filter)
	if err != nil {
		basePolicies = nil
	}

	policyMap := make(map[string]model.Policy)
	for _, p := range basePolicies {
		policyMap[p.ID] = p
	}
	for _, p := range s.policies {
		if filter.OrgID != "" && p.OrgID != filter.OrgID {
			continue
		}
		if filter.Category != "" && p.Category != filter.Category {
			continue
		}
		if filter.Status != "" && p.Status != filter.Status {
			continue
		}
		policyMap[p.ID] = p
	}

	result := make([]model.Policy, 0, len(policyMap))
	for _, p := range policyMap {
		result = append(result, p)
	}
	return result, nil
}

func (s *simulationOverlayStore) GetPolicyRevision(ctx context.Context, policyID string, revision int) (*model.PolicyRevision, error) {
	key := fmt.Sprintf("%s:%d", policyID, revision)
	if r, ok := s.revisions[key]; ok {
		return &r, nil
	}
	return s.base.GetPolicyRevision(ctx, policyID, revision)
}

func (s *simulationOverlayStore) ListPolicyRevisions(ctx context.Context, policyID string) ([]model.PolicyRevision, error) {
	baseRevs, err := s.base.ListPolicyRevisions(ctx, policyID)
	if err != nil {
		baseRevs = nil
	}

	revMap := make(map[int]model.PolicyRevision)
	for _, r := range baseRevs {
		revMap[r.Revision] = r
	}
	for _, r := range s.revisions {
		if r.PolicyID == policyID {
			revMap[r.Revision] = r
		}
	}

	result := make([]model.PolicyRevision, 0, len(revMap))
	for _, r := range revMap {
		result = append(result, r)
	}
	return result, nil
}

func (s *simulationOverlayStore) GetPolicyAssignment(ctx context.Context, id string) (*model.PolicyAssignment, error) {
	if a, ok := s.assignments[id]; ok {
		return &a, nil
	}
	return s.base.GetPolicyAssignment(ctx, id)
}

func (s *simulationOverlayStore) ListPolicyAssignments(ctx context.Context, filter model.PolicyAssignmentFilter) ([]model.PolicyAssignment, error) {
	baseAsgns, err := s.base.ListPolicyAssignments(ctx, filter)
	if err != nil {
		baseAsgns = nil
	}

	asgnMap := make(map[string]model.PolicyAssignment)
	for _, a := range baseAsgns {
		asgnMap[a.ID] = a
	}
	for _, a := range s.assignments {
		if filter.OrgID != "" && a.OrgID != filter.OrgID {
			continue
		}
		if filter.PolicyID != "" && a.PolicyID != filter.PolicyID {
			continue
		}
		if filter.TargetType != "" && a.TargetType != filter.TargetType {
			continue
		}
		if filter.TargetID != "" && a.TargetID != filter.TargetID {
			continue
		}
		if filter.EnabledOnly && !a.Enabled {
			continue
		}
		asgnMap[a.ID] = a
	}

	result := make([]model.PolicyAssignment, 0, len(asgnMap))
	for _, a := range asgnMap {
		result = append(result, a)
	}
	return result, nil
}

func (s *simulationOverlayStore) GetAssignmentsForTargets(ctx context.Context, orgID string, targetType model.PolicyTargetType, targetIDs []string) ([]model.PolicyAssignment, error) {
	baseAsgns, err := s.base.GetAssignmentsForTargets(ctx, orgID, targetType, targetIDs)
	if err != nil {
		baseAsgns = nil
	}

	asgnMap := make(map[string]model.PolicyAssignment)
	for _, a := range baseAsgns {
		asgnMap[a.ID] = a
	}

	for _, a := range s.assignments {
		if orgID != "" && a.OrgID != orgID {
			continue
		}
		if a.TargetType != targetType {
			continue
		}
		if !a.Enabled {
			continue
		}
		if slices.Contains(targetIDs, a.TargetID) {
			asgnMap[a.ID] = a
		}
	}

	result := make([]model.PolicyAssignment, 0, len(asgnMap))
	for _, a := range asgnMap {
		result = append(result, a)
	}
	return result, nil
}

func (s *simulationOverlayStore) GetEvaluationExecution(ctx context.Context, id string) (*model.EvaluationExecution, error) {
	return s.base.GetEvaluationExecution(ctx, id)
}

func (s *simulationOverlayStore) GetLatestNodeEvaluation(ctx context.Context, orgID, targetNodeID string) (*model.EvaluationExecution, error) {
	return s.base.GetLatestNodeEvaluation(ctx, orgID, targetNodeID)
}

func (s *simulationOverlayStore) ListEvaluationExecutions(ctx context.Context, filter model.EvaluationFilter) ([]model.EvaluationExecution, error) {
	return s.base.ListEvaluationExecutions(ctx, filter)
}

func (s *simulationOverlayStore) GetComplianceFinding(ctx context.Context, id string) (*model.ComplianceFinding, error) {
	return s.base.GetComplianceFinding(ctx, id)
}

func (s *simulationOverlayStore) ListComplianceFindings(ctx context.Context, filter model.FindingFilter) ([]model.ComplianceFinding, error) {
	return s.base.ListComplianceFindings(ctx, filter)
}

func (s *simulationOverlayStore) GetMaintenanceWindow(ctx context.Context, id string) (*model.MaintenanceWindow, error) {
	return s.base.GetMaintenanceWindow(ctx, id)
}

func (s *simulationOverlayStore) ListMaintenanceWindows(ctx context.Context, filter model.MaintenanceWindowFilter) ([]model.MaintenanceWindow, error) {
	return s.base.ListMaintenanceWindows(ctx, filter)
}

func (s *simulationOverlayStore) GetEscalationPolicy(ctx context.Context, id string) (*model.EscalationPolicy, error) {
	return s.base.GetEscalationPolicy(ctx, id)
}

func (s *simulationOverlayStore) ListEscalationPolicies(ctx context.Context, filter model.EscalationPolicyFilter) ([]model.EscalationPolicy, error) {
	return s.base.ListEscalationPolicies(ctx, filter)
}

func (s *simulationOverlayStore) GetSuppressionDecision(ctx context.Context, id string) (*model.SuppressionDecision, error) {
	return s.base.GetSuppressionDecision(ctx, id)
}

func (s *simulationOverlayStore) ListSuppressionDecisions(ctx context.Context, filter model.SuppressionFilter) ([]model.SuppressionDecision, error) {
	return s.base.ListSuppressionDecisions(ctx, filter)
}

