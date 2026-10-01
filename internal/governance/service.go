package governance

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/DocHoax/watchdog/internal/incidents"
	"github.com/DocHoax/watchdog/internal/storage"
	"github.com/DocHoax/watchdog/pkg/model"
)

// ReadOnlyGovernanceService defines the read-only contract for organizations, groups, node metadata, and policies.
type ReadOnlyGovernanceService interface {
	GetOrganization(ctx context.Context, id string) (*model.Organization, error)
	ListOrganizations(ctx context.Context) ([]model.Organization, error)
	GetFleetGroup(ctx context.Context, id string) (*model.FleetGroup, error)
	ListFleetGroups(ctx context.Context, orgID string) ([]model.FleetGroup, error)
	GetHierarchy(ctx context.Context, orgID string) ([]*model.FleetGroupHierarchyNode, error)
	GetGroupMembers(ctx context.Context, groupID string) ([]model.FleetGroupMember, error)
	GetNodeGroups(ctx context.Context, nodeID string) ([]model.FleetGroup, error)
	GetSubtreeNodes(ctx context.Context, groupID string) ([]string, error)
	GetNodeOwnership(ctx context.Context, nodeID string) (*model.NodeOwnershipMetadata, error)
	ResolveNodeOwnership(ctx context.Context, nodeID string) (*model.ResolvedOwnership, error)

	// Policy Queries & Resolution
	GetPolicy(ctx context.Context, id string) (*model.Policy, error)
	ListPolicies(ctx context.Context, filter model.PolicyFilter) ([]model.Policy, error)
	GetPolicyRevision(ctx context.Context, policyID string, revision int) (*model.PolicyRevision, error)
	ListPolicyRevisions(ctx context.Context, policyID string) ([]model.PolicyRevision, error)
	GetAssignment(ctx context.Context, id string) (*model.PolicyAssignment, error)
	ListAssignments(ctx context.Context, filter model.PolicyAssignmentFilter) ([]model.PolicyAssignment, error)
	ResolveNodePolicies(ctx context.Context, nodeID string) (*ResolvedPolicySet, error)
	EvaluateNodeCompliance(ctx context.Context, nodeID string) (*NodeComplianceReport, error)
	ExplainResolution(ctx context.Context, nodeID string) (*ResolutionExplanation, error)
	DetectPolicyConflicts(ctx context.Context, policyIDA string, revA int, policyIDB string, revB int) ([]PolicyConflict, error)

	// Runtime Evaluation & Findings Queries
	GetEvaluationExecution(ctx context.Context, id string) (*model.EvaluationExecution, error)
	GetLatestNodeEvaluation(ctx context.Context, orgID, targetNodeID string) (*model.EvaluationExecution, error)
	ListEvaluationExecutions(ctx context.Context, filter model.EvaluationFilter) ([]model.EvaluationExecution, error)
	GetComplianceFinding(ctx context.Context, id string) (*model.ComplianceFinding, error)
	ListComplianceFindings(ctx context.Context, filter model.FindingFilter) ([]model.ComplianceFinding, error)

	// Compliance Summaries & Hierarchical Rollups
	GetNodeComplianceSummary(ctx context.Context, orgID, nodeID string) (*model.ComplianceRollupSummary, error)
	GetGroupComplianceSummary(ctx context.Context, orgID, groupID string, includeSubgroups bool) (*model.ComplianceRollupSummary, error)
	GetOrgComplianceSummary(ctx context.Context, orgID string) (*model.ComplianceRollupSummary, error)

	// Isolated Policy Simulation
	SimulatePolicyChanges(ctx context.Context, req *model.SimulationRequest) (*model.SimulationResult, error)

	// Maintenance Windows
	GetMaintenanceWindow(ctx context.Context, id string) (*model.MaintenanceWindow, error)
	ListMaintenanceWindows(ctx context.Context, filter model.MaintenanceWindowFilter) ([]model.MaintenanceWindow, error)
	EvaluateMaintenanceWindows(ctx context.Context, orgID string, evalTime time.Time) ([]model.MaintenanceWindow, error)

	// Escalation Policies
	GetEscalationPolicy(ctx context.Context, id string) (*model.EscalationPolicy, error)
	ListEscalationPolicies(ctx context.Context, filter model.EscalationPolicyFilter) ([]model.EscalationPolicy, error)
	EvaluateIncidentEscalation(ctx context.Context, orgID string, incident *incidents.Incident, evalTime time.Time) (*model.EscalationEvaluationResult, error)

	// Alert & Finding Suppression
	EvaluateSuppression(ctx context.Context, req SuppressionEvaluationRequest) (*model.SuppressionDecision, error)
	GetSuppressionDecision(ctx context.Context, id string) (*model.SuppressionDecision, error)
	ListSuppressionDecisions(ctx context.Context, filter model.SuppressionFilter) ([]model.SuppressionDecision, error)

	// Audit Trail
	QueryAuditEvents(ctx context.Context, filter storage.AuditFilter) ([]model.AuditEvent, error)
}

// GovernanceService defines the full administrative and query interface for fleet governance.
type GovernanceService interface {
	ReadOnlyGovernanceService

	// Organization Management
	CreateOrganization(ctx context.Context, org *model.Organization) error
	UpdateOrganization(ctx context.Context, org *model.Organization) error
	DeleteOrganization(ctx context.Context, id string) error

	// Fleet Group Hierarchy
	CreateFleetGroup(ctx context.Context, group *model.FleetGroup) error
	UpdateFleetGroup(ctx context.Context, group *model.FleetGroup) error
	DeleteFleetGroup(ctx context.Context, id string) error

	// Group Membership
	AddMember(ctx context.Context, member *model.FleetGroupMember) error
	RemoveMember(ctx context.Context, groupID string, nodeID string) error
	SetMembers(ctx context.Context, groupID string, nodeIDs []string) error

	// Node Ownership & Lifecycle Metadata
	SetNodeOwnership(ctx context.Context, nodeID string, meta *model.NodeOwnershipMetadata) error

	// Policy Lifecycle Management
	CreatePolicy(ctx context.Context, policy *model.Policy) error
	UpdatePolicy(ctx context.Context, policy *model.Policy) error
	DeletePolicy(ctx context.Context, id string) error
	SetPolicyStatus(ctx context.Context, id string, status model.PolicyStatus) error
	SetActiveRevision(ctx context.Context, id string, revision int) error

	// Policy Revisions
	PublishRevision(ctx context.Context, rev *model.PolicyRevision) error

	// Policy Assignments
	CreateAssignment(ctx context.Context, asgn *model.PolicyAssignment) error
	DeleteAssignment(ctx context.Context, id string) error
	SetAssignmentEnabled(ctx context.Context, id string, enabled bool) error

	// Runtime Evaluation Operations
	EvaluateNode(ctx context.Context, nodeID string, trigger model.EvaluationTriggerType) (*model.EvaluationExecution, error)
	EvaluateFleetGroup(ctx context.Context, orgID, groupID string, includeSubgroups bool, trigger model.EvaluationTriggerType) ([]*model.EvaluationExecution, error)
	EvaluateOrganization(ctx context.Context, orgID string, trigger model.EvaluationTriggerType) ([]*model.EvaluationExecution, error)

	// Maintenance Windows
	CreateMaintenanceWindow(ctx context.Context, window *model.MaintenanceWindow) error
	UpdateMaintenanceWindow(ctx context.Context, window *model.MaintenanceWindow) error
	DeleteMaintenanceWindow(ctx context.Context, id string) error
	CancelMaintenanceWindow(ctx context.Context, id string, reason string) error

	// Escalation Policies
	CreateEscalationPolicy(ctx context.Context, policy *model.EscalationPolicy) error
	UpdateEscalationPolicy(ctx context.Context, policy *model.EscalationPolicy) error
	DeleteEscalationPolicy(ctx context.Context, id string) error

	// Audit Trail
	RecordAuditEvent(ctx context.Context, event model.AuditEvent) error
}

type governanceService struct {
	store             storage.Storage
	resolver          *PolicyResolver
	acquisition       *DataAcquisitionProvider
	evaluators        *EvaluatorRegistry
	reconciler        *FindingReconciler
	aggregator        *ComplianceAggregator
	simulation        *SimulationEngine
	maintenanceEngine *MaintenanceEngine
	suppressionEngine *SuppressionEngine
	ownershipResolver *OwnershipResolver
	escalationEngine  *EscalationEngine
	auditRecorder     *AuditRecorder
	clock             Clock
	mu                sync.RWMutex
}

// NewGovernanceService constructs an instance of GovernanceService backed by storage.
func NewGovernanceService(store storage.Storage) GovernanceService {
	return NewGovernanceServiceWithClock(store, RealClock{})
}

// NewGovernanceServiceWithClock constructs a GovernanceService with a custom Clock for deterministic testing.
func NewGovernanceServiceWithClock(store storage.Storage, clock Clock) GovernanceService {
	if clock == nil {
		clock = RealClock{}
	}
	resolver := NewPolicyResolver(store)
	acquisition := NewDataAcquisitionProvider(store, resolver, clock, nil)
	evaluators := NewEvaluatorRegistry()
	reconciler := NewFindingReconciler(store, clock)
	aggregator := NewComplianceAggregator(store, clock)
	simulation := NewSimulationEngine(store, evaluators, clock)
	maintenanceEngine := NewMaintenanceEngine(store, clock)
	suppressionEngine := NewSuppressionEngine(store, maintenanceEngine, clock)
	ownershipResolver := NewOwnershipResolver(store, clock)
	escalationEngine := NewEscalationEngine(store, clock)
	auditRecorder := NewAuditRecorder(store, clock)

	return &governanceService{
		store:             store,
		resolver:          resolver,
		acquisition:       acquisition,
		evaluators:        evaluators,
		reconciler:        reconciler,
		aggregator:        aggregator,
		simulation:        simulation,
		maintenanceEngine: maintenanceEngine,
		suppressionEngine: suppressionEngine,
		ownershipResolver: ownershipResolver,
		escalationEngine:  escalationEngine,
		auditRecorder:     auditRecorder,
		clock:             clock,
	}
}

// CreateOrganization validates and persists a new organization.
func (s *governanceService) CreateOrganization(ctx context.Context, org *model.Organization) error {
	if org == nil {
		return fmt.Errorf("%w: nil organization", ErrInvalidInput)
	}
	if err := org.Validate(); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if already exists
	if existing, _ := s.store.GetOrganization(ctx, org.ID); existing != nil {
		return fmt.Errorf("%w: organization %q", ErrOrganizationExists, org.ID)
	}

	if org.CreatedAt.IsZero() {
		org.CreatedAt = time.Now().UTC()
	}
	org.UpdatedAt = org.CreatedAt

	return s.store.SaveOrganization(ctx, org)
}

// GetOrganization retrieves an organization by ID.
func (s *governanceService) GetOrganization(ctx context.Context, id string) (*model.Organization, error) {
	if id == "" {
		return nil, fmt.Errorf("%w: empty organization id", ErrInvalidIdentifier)
	}

	org, err := s.store.GetOrganization(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrOrganizationNotFound, err)
	}
	return org, nil
}

// ListOrganizations returns all organizations in the system.
func (s *governanceService) ListOrganizations(ctx context.Context) ([]model.Organization, error) {
	return s.store.ListOrganizations(ctx)
}

// UpdateOrganization updates existing organization details.
func (s *governanceService) UpdateOrganization(ctx context.Context, org *model.Organization) error {
	if org == nil {
		return fmt.Errorf("%w: nil organization", ErrInvalidInput)
	}
	if err := org.Validate(); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	existing, err := s.store.GetOrganization(ctx, org.ID)
	if err != nil || existing == nil {
		return fmt.Errorf("%w: %s", ErrOrganizationNotFound, org.ID)
	}

	org.CreatedAt = existing.CreatedAt
	org.UpdatedAt = time.Now().UTC()

	return s.store.SaveOrganization(ctx, org)
}

// DeleteOrganization removes an organization, guarding the default organization.
func (s *governanceService) DeleteOrganization(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("%w: empty organization id", ErrInvalidIdentifier)
	}
	if id == model.DefaultOrganizationID {
		return ErrDefaultOrgImmutable
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := s.store.GetOrganization(ctx, id); err != nil {
		return fmt.Errorf("%w: %s", ErrOrganizationNotFound, id)
	}

	return s.store.DeleteOrganization(ctx, id)
}

// CreateFleetGroup creates a new fleet group after validating relationships and acyclicity.
func (s *governanceService) CreateFleetGroup(ctx context.Context, group *model.FleetGroup) error {
	if group == nil {
		return fmt.Errorf("%w: nil fleet group", ErrInvalidInput)
	}
	if err := group.Validate(); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Verify organization exists
	if _, err := s.store.GetOrganization(ctx, group.OrgID); err != nil {
		return fmt.Errorf("%w: org_id %q does not exist", ErrOrganizationNotFound, group.OrgID)
	}

	// Verify existing ID not duplicated
	if existing, _ := s.store.GetFleetGroup(ctx, group.ID); existing != nil {
		return fmt.Errorf("%w: group %q", ErrFleetGroupExists, group.ID)
	}

	// Verify parent group if provided
	if group.ParentGroupID != "" {
		parent, err := s.store.GetFleetGroup(ctx, group.ParentGroupID)
		if err != nil || parent == nil {
			return fmt.Errorf("%w: parent_group_id %q", ErrParentNotFound, group.ParentGroupID)
		}
		if parent.OrgID != group.OrgID {
			return fmt.Errorf("%w: parent org is %q, child org is %q", ErrCrossOrgParent, parent.OrgID, group.OrgID)
		}
	}

	allGroups, err := s.store.ListFleetGroups(ctx, group.OrgID)
	if err != nil {
		return fmt.Errorf("failed to list fleet groups for hierarchy validation: %w", err)
	}

	groupMap := make(map[string]*model.FleetGroup, len(allGroups)+1)
	for i := range allGroups {
		groupMap[allGroups[i].ID] = &allGroups[i]
	}

	if group.ParentGroupID != "" {
		if DetectCycle(groupMap, group.ID, group.ParentGroupID) {
			return fmt.Errorf("%w: group %s -> parent %s", ErrCycleDetected, group.ID, group.ParentGroupID)
		}
	}

	// Compute canonical path
	groupMap[group.ID] = group
	group.Path = ComputeGroupPath(group, groupMap)

	if group.CreatedAt.IsZero() {
		group.CreatedAt = time.Now().UTC()
	}
	group.UpdatedAt = group.CreatedAt

	return s.store.SaveFleetGroup(ctx, group)
}

// GetFleetGroup retrieves a fleet group by ID.
func (s *governanceService) GetFleetGroup(ctx context.Context, id string) (*model.FleetGroup, error) {
	if id == "" {
		return nil, fmt.Errorf("%w: empty group id", ErrInvalidIdentifier)
	}
	group, err := s.store.GetFleetGroup(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrFleetGroupNotFound, err)
	}
	return group, nil
}

// ListFleetGroups returns groups for a given org or all groups if orgID is empty.
func (s *governanceService) ListFleetGroups(ctx context.Context, orgID string) ([]model.FleetGroup, error) {
	return s.store.ListFleetGroups(ctx, orgID)
}

// UpdateFleetGroup updates an existing group, ensuring cycle prevention and re-computing paths.
func (s *governanceService) UpdateFleetGroup(ctx context.Context, group *model.FleetGroup) error {
	if group == nil {
		return fmt.Errorf("%w: nil fleet group", ErrInvalidInput)
	}
	if err := group.Validate(); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	existing, err := s.store.GetFleetGroup(ctx, group.ID)
	if err != nil || existing == nil {
		return fmt.Errorf("%w: %s", ErrFleetGroupNotFound, group.ID)
	}

	if group.ParentGroupID != "" {
		parent, err := s.store.GetFleetGroup(ctx, group.ParentGroupID)
		if err != nil || parent == nil {
			return fmt.Errorf("%w: parent_group_id %q", ErrParentNotFound, group.ParentGroupID)
		}
		if parent.OrgID != group.OrgID {
			return fmt.Errorf("%w: parent org is %q, child org is %q", ErrCrossOrgParent, parent.OrgID, group.OrgID)
		}
	}

	allGroups, err := s.store.ListFleetGroups(ctx, group.OrgID)
	if err != nil {
		return fmt.Errorf("failed to list fleet groups: %w", err)
	}

	groupMap := make(map[string]*model.FleetGroup, len(allGroups))
	for i := range allGroups {
		if allGroups[i].ID == group.ID {
			groupMap[group.ID] = group
		} else {
			groupMap[allGroups[i].ID] = &allGroups[i]
		}
	}

	if group.ParentGroupID != "" {
		if DetectCycle(groupMap, group.ID, group.ParentGroupID) {
			return fmt.Errorf("%w: group %s -> parent %s", ErrCycleDetected, group.ID, group.ParentGroupID)
		}
	}

	group.Path = ComputeGroupPath(group, groupMap)
	group.CreatedAt = existing.CreatedAt
	group.UpdatedAt = time.Now().UTC()

	return s.store.SaveFleetGroup(ctx, group)
}

// DeleteFleetGroup deletes a group and detaches any children.
func (s *governanceService) DeleteFleetGroup(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("%w: empty group id", ErrInvalidIdentifier)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := s.store.GetFleetGroup(ctx, id); err != nil {
		return fmt.Errorf("%w: %s", ErrFleetGroupNotFound, id)
	}

	return s.store.DeleteFleetGroup(ctx, id)
}

// GetHierarchy generates the complete hierarchical group forest for an organization.
func (s *governanceService) GetHierarchy(ctx context.Context, orgID string) ([]*model.FleetGroupHierarchyNode, error) {
	if orgID == "" {
		orgID = model.DefaultOrganizationID
	}

	groups, err := s.store.ListFleetGroups(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to list groups for hierarchy: %w", err)
	}

	groupMembers := make(map[string][]string, len(groups))
	for _, g := range groups {
		members, err := s.store.GetGroupMembers(ctx, g.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch members for group %q: %w", g.ID, err)
		}
		var nodeIDs []string
		for _, m := range members {
			nodeIDs = append(nodeIDs, m.NodeID)
		}
		groupMembers[g.ID] = nodeIDs
	}

	return BuildHierarchyTree(groups, groupMembers), nil
}

// AddMember adds a node to a fleet group.
func (s *governanceService) AddMember(ctx context.Context, member *model.FleetGroupMember) error {
	if member == nil {
		return fmt.Errorf("%w: nil member", ErrInvalidInput)
	}
	if err := member.Validate(); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Verify group exists
	if _, err := s.store.GetFleetGroup(ctx, member.GroupID); err != nil {
		return fmt.Errorf("%w: group %s", ErrFleetGroupNotFound, member.GroupID)
	}

	return s.store.AddGroupMember(ctx, member)
}

// RemoveMember removes a node from a fleet group.
func (s *governanceService) RemoveMember(ctx context.Context, groupID string, nodeID string) error {
	if groupID == "" || nodeID == "" {
		return fmt.Errorf("%w: empty group_id or node_id", ErrInvalidIdentifier)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	return s.store.RemoveGroupMember(ctx, groupID, nodeID)
}

// SetMembers atomically updates the member nodes of a fleet group.
func (s *governanceService) SetMembers(ctx context.Context, groupID string, nodeIDs []string) error {
	if groupID == "" {
		return fmt.Errorf("%w: empty group_id", ErrInvalidIdentifier)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := s.store.GetFleetGroup(ctx, groupID); err != nil {
		return fmt.Errorf("%w: group %s", ErrFleetGroupNotFound, groupID)
	}

	return s.store.SetGroupMembers(ctx, groupID, nodeIDs)
}

// GetGroupMembers retrieves all member assignments for a group.
func (s *governanceService) GetGroupMembers(ctx context.Context, groupID string) ([]model.FleetGroupMember, error) {
	if groupID == "" {
		return nil, fmt.Errorf("%w: empty group_id", ErrInvalidIdentifier)
	}
	return s.store.GetGroupMembers(ctx, groupID)
}

// GetNodeGroups retrieves all fleet groups containing a node.
func (s *governanceService) GetNodeGroups(ctx context.Context, nodeID string) ([]model.FleetGroup, error) {
	if nodeID == "" {
		return nil, fmt.Errorf("%w: empty node_id", ErrInvalidIdentifier)
	}
	return s.store.GetNodeGroups(ctx, nodeID)
}

// GetSubtreeNodes returns all deduplicated node IDs in a group and its recursive children.
func (s *governanceService) GetSubtreeNodes(ctx context.Context, groupID string) ([]string, error) {
	if groupID == "" {
		return nil, fmt.Errorf("%w: empty group_id", ErrInvalidIdentifier)
	}

	group, err := s.store.GetFleetGroup(ctx, groupID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrFleetGroupNotFound, err)
	}

	groups, err := s.store.ListFleetGroups(ctx, group.OrgID)
	if err != nil {
		return nil, fmt.Errorf("failed to list groups: %w", err)
	}

	groupMembers := make(map[string][]string, len(groups))
	for _, g := range groups {
		members, err := s.store.GetGroupMembers(ctx, g.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch group members for %s: %w", g.ID, err)
		}
		var nodeIDs []string
		for _, m := range members {
			nodeIDs = append(nodeIDs, m.NodeID)
		}
		groupMembers[g.ID] = nodeIDs
	}

	return CollectSubtreeNodeIDs(groupID, groups, groupMembers), nil
}

// SetNodeOwnership associates structured ownership and lifecycle metadata with a fleet node.
func (s *governanceService) SetNodeOwnership(ctx context.Context, nodeID string, meta *model.NodeOwnershipMetadata) error {
	if nodeID == "" {
		return fmt.Errorf("%w: empty node_id", ErrInvalidIdentifier)
	}
	if meta != nil {
		if err := meta.Validate(); err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidInput, err)
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	node, err := s.store.GetFleetNode(ctx, nodeID)
	if err != nil || node == nil {
		return fmt.Errorf("%w: node %s", ErrNodeNotFound, nodeID)
	}

	if node.Metadata == nil {
		node.Metadata = make(map[string]string)
	}

	// Remove existing governance metadata keys
	for k := range node.Metadata {
		if strings.HasPrefix(k, model.MetaPrefixGovernance) {
			delete(node.Metadata, k)
		}
	}

	// Overlay new governance metadata
	if meta != nil {
		node.Metadata = meta.MergeIntoMetadata(node.Metadata)
	}

	return s.store.SaveFleetNode(ctx, node)
}

// GetNodeOwnership extracts ownership metadata from a node's stored properties.
func (s *governanceService) GetNodeOwnership(ctx context.Context, nodeID string) (*model.NodeOwnershipMetadata, error) {
	if nodeID == "" {
		return nil, fmt.Errorf("%w: empty node_id", ErrInvalidIdentifier)
	}

	node, err := s.store.GetFleetNode(ctx, nodeID)
	if err != nil || node == nil {
		return nil, fmt.Errorf("%w: node %s", ErrNodeNotFound, nodeID)
	}

	return model.NodeOwnershipFromMetadata(node.Metadata), nil
}

// CreatePolicy validates and persists a new policy definition.
func (s *governanceService) CreatePolicy(ctx context.Context, policy *model.Policy) error {
	if policy == nil {
		return fmt.Errorf("%w: nil policy", ErrInvalidInput)
	}
	if err := policy.Validate(); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Verify organization exists
	if _, err := s.store.GetOrganization(ctx, policy.OrgID); err != nil {
		return fmt.Errorf("%w: org_id %q does not exist", ErrOrganizationNotFound, policy.OrgID)
	}

	// Verify existing policy ID not duplicated
	if existing, _ := s.store.GetPolicy(ctx, policy.ID); existing != nil {
		return fmt.Errorf("%w: policy %q", ErrPolicyExists, policy.ID)
	}

	if policy.Status == "" {
		policy.Status = model.PolicyStatusDraft
	}
	if policy.CreatedAt.IsZero() {
		policy.CreatedAt = time.Now().UTC()
	}
	policy.UpdatedAt = policy.CreatedAt

	return s.store.SavePolicy(ctx, policy)
}

// GetPolicy retrieves a policy by ID.
func (s *governanceService) GetPolicy(ctx context.Context, id string) (*model.Policy, error) {
	if id == "" {
		return nil, fmt.Errorf("%w: empty policy id", ErrInvalidIdentifier)
	}

	policy, err := s.store.GetPolicy(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrPolicyNotFound, err)
	}
	return policy, nil
}

// ListPolicies lists policies matching the given filter.
func (s *governanceService) ListPolicies(ctx context.Context, filter model.PolicyFilter) ([]model.Policy, error) {
	return s.store.ListPolicies(ctx, filter)
}

// UpdatePolicy validates and updates mutable policy attributes with lifecycle transition validation.
func (s *governanceService) UpdatePolicy(ctx context.Context, policy *model.Policy) error {
	if policy == nil {
		return fmt.Errorf("%w: nil policy", ErrInvalidInput)
	}
	if err := policy.Validate(); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	existing, err := s.store.GetPolicy(ctx, policy.ID)
	if err != nil || existing == nil {
		return fmt.Errorf("%w: policy %s", ErrPolicyNotFound, policy.ID)
	}

	// Validate lifecycle transition if status is changing
	if policy.Status != existing.Status {
		if !existing.CanTransitionTo(policy.Status) {
			return fmt.Errorf("%w: cannot transition from %s to %s", ErrInvalidLifecycleTransition, existing.Status, policy.Status)
		}
	}

	policy.CreatedAt = existing.CreatedAt
	policy.UpdatedAt = time.Now().UTC()

	return s.store.SavePolicy(ctx, policy)
}

// DeletePolicy removes a policy and cascades to its revisions and assignments.
func (s *governanceService) DeletePolicy(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("%w: empty policy id", ErrInvalidIdentifier)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := s.store.GetPolicy(ctx, id); err != nil {
		return fmt.Errorf("%w: policy %s", ErrPolicyNotFound, id)
	}

	return s.store.DeletePolicy(ctx, id)
}

// SetPolicyStatus applies a validated lifecycle status transition to a policy.
func (s *governanceService) SetPolicyStatus(ctx context.Context, id string, status model.PolicyStatus) error {
	if id == "" {
		return fmt.Errorf("%w: empty policy id", ErrInvalidIdentifier)
	}
	if !status.IsValid() {
		return fmt.Errorf("%w: invalid status %q", ErrInvalidInput, status)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	existing, err := s.store.GetPolicy(ctx, id)
	if err != nil || existing == nil {
		return fmt.Errorf("%w: policy %s", ErrPolicyNotFound, id)
	}

	if existing.Status != status {
		if !existing.CanTransitionTo(status) {
			return fmt.Errorf("%w: cannot transition from %s to %s", ErrInvalidLifecycleTransition, existing.Status, status)
		}
		existing.Status = status
		existing.UpdatedAt = time.Now().UTC()
		return s.store.SavePolicy(ctx, existing)
	}

	return nil
}

// SetActiveRevision updates the active revision pointer for a policy after verifying the revision exists.
func (s *governanceService) SetActiveRevision(ctx context.Context, id string, revision int) error {
	if id == "" {
		return fmt.Errorf("%w: empty policy id", ErrInvalidIdentifier)
	}
	if revision < 1 {
		return fmt.Errorf("%w: revision must be >= 1", ErrInvalidInput)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	existing, err := s.store.GetPolicy(ctx, id)
	if err != nil || existing == nil {
		return fmt.Errorf("%w: policy %s", ErrPolicyNotFound, id)
	}

	// Verify revision exists
	if _, err := s.store.GetPolicyRevision(ctx, id, revision); err != nil {
		return fmt.Errorf("%w: policy %s revision %d", ErrPolicyRevisionNotFound, id, revision)
	}

	existing.ActiveRevision = revision
	existing.UpdatedAt = time.Now().UTC()

	return s.store.SavePolicy(ctx, existing)
}

// PublishRevision creates an immutable versioned revision with verified SHA-256 digest and selector validation.
func (s *governanceService) PublishRevision(ctx context.Context, rev *model.PolicyRevision) error {
	if rev == nil {
		return fmt.Errorf("%w: nil revision", ErrInvalidInput)
	}
	if rev.PolicyID == "" {
		return fmt.Errorf("%w: empty policy_id", ErrInvalidIdentifier)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Verify parent policy exists
	if _, err := s.store.GetPolicy(ctx, rev.PolicyID); err != nil {
		return fmt.Errorf("%w: policy %s", ErrPolicyNotFound, rev.PolicyID)
	}

	// Auto-assign revision number if not specified
	if rev.Revision <= 0 {
		revisions, err := s.store.ListPolicyRevisions(ctx, rev.PolicyID)
		if err != nil {
			return fmt.Errorf("failed to list revisions for auto-increment: %w", err)
		}
		nextRev := 1
		for _, r := range revisions {
			if r.Revision >= nextRev {
				nextRev = r.Revision + 1
			}
		}
		rev.Revision = nextRev
	}

	// Validate selector expression syntax if provided
	if rev.Selector != "" {
		if _, err := ParseSelector(rev.Selector); err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidSelector, err)
		}
	}

	// Compute and verify content digest
	computedDigest := rev.ComputeDigest()
	if rev.ContentDigest != "" && rev.ContentDigest != computedDigest {
		return fmt.Errorf("%w: provided %q, computed %q", ErrPolicyRevisionDigestMismatch, rev.ContentDigest, computedDigest)
	}
	rev.ContentDigest = computedDigest

	if err := rev.Validate(); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}

	if rev.CreatedAt.IsZero() {
		rev.CreatedAt = time.Now().UTC()
	}

	return s.store.SavePolicyRevision(ctx, rev)
}

// GetPolicyRevision retrieves an immutable revision of a policy.
func (s *governanceService) GetPolicyRevision(ctx context.Context, policyID string, revision int) (*model.PolicyRevision, error) {
	if policyID == "" {
		return nil, fmt.Errorf("%w: empty policy_id", ErrInvalidIdentifier)
	}
	if revision < 1 {
		return nil, fmt.Errorf("%w: revision must be >= 1", ErrInvalidInput)
	}

	rev, err := s.store.GetPolicyRevision(ctx, policyID, revision)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrPolicyRevisionNotFound, err)
	}
	return rev, nil
}

// ListPolicyRevisions lists all published revisions for a given policy in ascending order.
func (s *governanceService) ListPolicyRevisions(ctx context.Context, policyID string) ([]model.PolicyRevision, error) {
	if policyID == "" {
		return nil, fmt.Errorf("%w: empty policy_id", ErrInvalidIdentifier)
	}

	if _, err := s.store.GetPolicy(ctx, policyID); err != nil {
		return nil, fmt.Errorf("%w: policy %s", ErrPolicyNotFound, policyID)
	}

	return s.store.ListPolicyRevisions(ctx, policyID)
}

// CreateAssignment assigns a policy to an organization, fleet group, or direct node with tenant validation.
func (s *governanceService) CreateAssignment(ctx context.Context, asgn *model.PolicyAssignment) error {
	if asgn == nil {
		return fmt.Errorf("%w: nil assignment", ErrInvalidInput)
	}
	if err := asgn.Validate(); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Verify organization exists
	if _, err := s.store.GetOrganization(ctx, asgn.OrgID); err != nil {
		return fmt.Errorf("%w: org_id %q does not exist", ErrOrganizationNotFound, asgn.OrgID)
	}

	// Verify policy exists and belongs to same organization
	policy, err := s.store.GetPolicy(ctx, asgn.PolicyID)
	if err != nil || policy == nil {
		return fmt.Errorf("%w: policy %s", ErrPolicyNotFound, asgn.PolicyID)
	}
	if policy.OrgID != asgn.OrgID {
		return fmt.Errorf("%w: policy org %q != assignment org %q", ErrCrossOrgAssignment, policy.OrgID, asgn.OrgID)
	}

	// Validate target scope exists and belongs to same organization
	switch asgn.TargetType {
	case model.TargetTypeOrganization:
		if asgn.TargetID != asgn.OrgID {
			return fmt.Errorf("%w: target_id %q must match org_id %q for organization assignment", ErrInvalidInput, asgn.TargetID, asgn.OrgID)
		}
	case model.TargetTypeFleetGroup:
		grp, err := s.store.GetFleetGroup(ctx, asgn.TargetID)
		if err != nil || grp == nil {
			return fmt.Errorf("%w: fleet group %s", ErrFleetGroupNotFound, asgn.TargetID)
		}
		if grp.OrgID != asgn.OrgID {
			return fmt.Errorf("%w: fleet group org %q != assignment org %q", ErrCrossOrgAssignment, grp.OrgID, asgn.OrgID)
		}
	case model.TargetTypeNode:
			node, err := s.store.GetFleetNode(ctx, asgn.TargetID)
			if err != nil || node == nil {
				return fmt.Errorf("%w: fleet node %s", ErrNodeNotFound, asgn.TargetID)
			}
			nodeOrg := model.DefaultOrganizationID
			if node.Metadata != nil {
				if val, ok := node.Metadata["org_id"]; ok && val != "" {
					nodeOrg = val
				} else if val, ok := node.Metadata["organization_id"]; ok && val != "" {
					nodeOrg = val
				}
			}
			if asgn.OrgID != model.DefaultOrganizationID && nodeOrg != asgn.OrgID {
				return fmt.Errorf("%w: node org %q != assignment org %q", ErrCrossOrgAssignment, nodeOrg, asgn.OrgID)
			}
	}

	// Verify assignment ID not duplicated
	if existing, _ := s.store.GetPolicyAssignment(ctx, asgn.ID); existing != nil {
		return fmt.Errorf("%w: assignment %q", ErrPolicyAssignmentExists, asgn.ID)
	}

	if asgn.AssignedAt.IsZero() {
		asgn.AssignedAt = time.Now().UTC()
	}

	return s.store.SavePolicyAssignment(ctx, asgn)
}

// GetAssignment retrieves a single policy assignment by ID.
func (s *governanceService) GetAssignment(ctx context.Context, id string) (*model.PolicyAssignment, error) {
	if id == "" {
		return nil, fmt.Errorf("%w: empty assignment id", ErrInvalidIdentifier)
	}

	asgn, err := s.store.GetPolicyAssignment(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrPolicyAssignmentNotFound, err)
	}
	return asgn, nil
}

// ListAssignments queries policy assignments according to filter criteria.
func (s *governanceService) ListAssignments(ctx context.Context, filter model.PolicyAssignmentFilter) ([]model.PolicyAssignment, error) {
	return s.store.ListPolicyAssignments(ctx, filter)
}

// DeleteAssignment removes a policy assignment by ID.
func (s *governanceService) DeleteAssignment(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("%w: empty assignment id", ErrInvalidIdentifier)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := s.store.GetPolicyAssignment(ctx, id); err != nil {
		return fmt.Errorf("%w: assignment %s", ErrPolicyAssignmentNotFound, id)
	}

	return s.store.DeletePolicyAssignment(ctx, id)
}

// SetAssignmentEnabled updates the active state of an assignment.
func (s *governanceService) SetAssignmentEnabled(ctx context.Context, id string, enabled bool) error {
	if id == "" {
		return fmt.Errorf("%w: empty assignment id", ErrInvalidIdentifier)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	existing, err := s.store.GetPolicyAssignment(ctx, id)
	if err != nil || existing == nil {
		return fmt.Errorf("%w: assignment %s", ErrPolicyAssignmentNotFound, id)
	}

	existing.Enabled = enabled
	return s.store.SavePolicyAssignment(ctx, existing)
}

// ResolveNodePolicies resolves all effective policies for a node according to hierarchical precedence and inheritance.
func (s *governanceService) ResolveNodePolicies(ctx context.Context, nodeID string) (*ResolvedPolicySet, error) {
	if nodeID == "" {
		return nil, fmt.Errorf("%w: empty node_id", ErrInvalidIdentifier)
	}
	return s.resolver.ResolveNodePolicies(ctx, nodeID)
}

// EvaluateNodeCompliance evaluates the compliance posture of a node against its effective policies.
func (s *governanceService) EvaluateNodeCompliance(ctx context.Context, nodeID string) (*NodeComplianceReport, error) {
	if nodeID == "" {
		return nil, fmt.Errorf("%w: empty node_id", ErrInvalidIdentifier)
	}
	return s.resolver.EvaluateNodeCompliance(ctx, nodeID)
}

// ExplainResolution produces a step-by-step diagnostic breakdown of policy resolution for a node.
func (s *governanceService) ExplainResolution(ctx context.Context, nodeID string) (*ResolutionExplanation, error) {
	if nodeID == "" {
		return nil, fmt.Errorf("%w: empty node_id", ErrInvalidIdentifier)
	}
	return s.resolver.ExplainResolution(ctx, nodeID)
}

// DetectPolicyConflicts checks for potential conflicts and divergent configurations between two policy revisions.
func (s *governanceService) DetectPolicyConflicts(ctx context.Context, policyIDA string, revA int, policyIDB string, revB int) ([]PolicyConflict, error) {
	if policyIDA == "" || policyIDB == "" {
		return nil, fmt.Errorf("%w: empty policy identifier", ErrInvalidIdentifier)
	}
	if revA < 1 || revB < 1 {
		return nil, fmt.Errorf("%w: revision numbers must be >= 1", ErrInvalidInput)
	}

	rA, err := s.store.GetPolicyRevision(ctx, policyIDA, revA)
	if err != nil {
		return nil, fmt.Errorf("%w: policy %s revision %d", ErrPolicyRevisionNotFound, policyIDA, revA)
	}
	rB, err := s.store.GetPolicyRevision(ctx, policyIDB, revB)
	if err != nil {
		return nil, fmt.Errorf("%w: policy %s revision %d", ErrPolicyRevisionNotFound, policyIDB, revB)
	}

	return DetectPolicyPairConflicts(rA, rB), nil
}

// EvaluateNode executes an evaluation of effective policies against a target node, persists the execution record, and reconciles findings.
func (s *governanceService) EvaluateNode(ctx context.Context, nodeID string, trigger model.EvaluationTriggerType) (*model.EvaluationExecution, error) {
	if strings.TrimSpace(nodeID) == "" {
		return nil, fmt.Errorf("%w: empty node_id", ErrInvalidIdentifier)
	}
	if trigger == "" {
		trigger = model.EvaluationTriggerOnDemand
	} else if !trigger.IsValid() {
		return nil, fmt.Errorf("%w: invalid trigger type %s", ErrInvalidInput, trigger)
	}

	evalCtx, err := s.acquisition.BuildEvaluationContext(ctx, nodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to build evaluation context for node %s: %w", nodeID, err)
	}

	start := s.clock.Now()
	results, err := s.evaluators.EvaluateContext(ctx, evalCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate policies for node %s: %w", nodeID, err)
	}

	summary := model.CalculateSummary(results)
	var status model.EvaluationStatus
	switch {
	case summary.NonCompliantRules > 0:
		status = model.EvaluationStatusNonCompliant
	case summary.WarningRules > 0:
		status = model.EvaluationStatusWarning
	case summary.ErrorRules > 0:
		status = model.EvaluationStatusError
	case summary.InsufficientDataRules > 0:
		status = model.EvaluationStatusInsufficientData
	case summary.TotalRules == 0 || summary.NotApplicableRules == summary.TotalRules:
		status = model.EvaluationStatusNotApplicable
	default:
		status = model.EvaluationStatusCompliant
	}

	execID := fmt.Sprintf("exec-%s-%d", nodeID, start.UnixNano())
	exec := &model.EvaluationExecution{
		ID:           execID,
		OrgID:        evalCtx.OrgID,
		TargetNodeID: nodeID,
		TriggerType:  trigger,
		EvaluatedAt:  start,
		DurationNs:   s.clock.Since(start).Nanoseconds(),
		Status:       status,
		Results:      results,
		Summary:      summary,
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.store.SaveEvaluationExecution(ctx, exec); err != nil {
		return nil, fmt.Errorf("failed to persist evaluation execution: %w", err)
	}

	if _, err := s.reconciler.ReconcileExecution(ctx, exec); err != nil {
		return nil, fmt.Errorf("failed to reconcile findings: %w", err)
	}

	return exec, nil
}

// EvaluateFleetGroup evaluates all member nodes within a fleet group and optionally its recursive subgroups.
func (s *governanceService) EvaluateFleetGroup(ctx context.Context, orgID, groupID string, includeSubgroups bool, trigger model.EvaluationTriggerType) ([]*model.EvaluationExecution, error) {
	if strings.TrimSpace(groupID) == "" {
		return nil, fmt.Errorf("%w: empty group_id", ErrInvalidIdentifier)
	}
	if orgID == "" {
		orgID = model.DefaultOrganizationID
	}

	group, err := s.store.GetFleetGroup(ctx, groupID)
	if err != nil || group == nil {
		return nil, fmt.Errorf("%w: group %s", ErrFleetGroupNotFound, groupID)
	}
	if group.OrgID != orgID {
		return nil, fmt.Errorf("%w: group org %q != requested org %q", ErrCrossOrgAssignment, group.OrgID, orgID)
	}

	var nodeIDs []string
	if includeSubgroups {
		nodes, err := s.GetSubtreeNodes(ctx, groupID)
		if err != nil {
			return nil, fmt.Errorf("failed to get subtree nodes: %w", err)
		}
		nodeIDs = nodes
	} else {
		members, err := s.store.GetGroupMembers(ctx, groupID)
		if err != nil {
			return nil, fmt.Errorf("failed to get group members: %w", err)
		}
		for _, m := range members {
			nodeIDs = append(nodeIDs, m.NodeID)
		}
	}

	var executions []*model.EvaluationExecution
	for _, nid := range nodeIDs {
		exec, err := s.EvaluateNode(ctx, nid, trigger)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate node %s: %w", nid, err)
		}
		executions = append(executions, exec)
	}

	return executions, nil
}

// EvaluateOrganization evaluates all fleet nodes belonging to the specified organization.
func (s *governanceService) EvaluateOrganization(ctx context.Context, orgID string, trigger model.EvaluationTriggerType) ([]*model.EvaluationExecution, error) {
	if orgID == "" {
		orgID = model.DefaultOrganizationID
	}

	if _, err := s.store.GetOrganization(ctx, orgID); err != nil {
		return nil, fmt.Errorf("%w: organization %s", ErrOrganizationNotFound, orgID)
	}

	nodes, _, err := s.store.ListFleetNodes(ctx, model.FleetFilter{})
	if err != nil {
		return nil, fmt.Errorf("failed to list fleet nodes: %w", err)
	}

	var orgNodeIDs []string
	for _, n := range nodes {
		nodeOrg := model.DefaultOrganizationID
		if n.Metadata != nil {
			if val, ok := n.Metadata["org_id"]; ok && val != "" {
				nodeOrg = val
			} else if val, ok := n.Metadata["organization_id"]; ok && val != "" {
				nodeOrg = val
			}
		}
		if nodeOrg == orgID {
			orgNodeIDs = append(orgNodeIDs, n.Identity.NodeID)
		}
	}

	var executions []*model.EvaluationExecution
	for _, nid := range orgNodeIDs {
		exec, err := s.EvaluateNode(ctx, nid, trigger)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate node %s: %w", nid, err)
		}
		executions = append(executions, exec)
	}

	return executions, nil
}

// GetEvaluationExecution retrieves an evaluation execution by its unique identifier.
func (s *governanceService) GetEvaluationExecution(ctx context.Context, id string) (*model.EvaluationExecution, error) {
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("%w: empty evaluation execution id", ErrInvalidIdentifier)
	}
	return s.store.GetEvaluationExecution(ctx, id)
}

// GetLatestNodeEvaluation returns the most recent evaluation execution for a specific node in an organization.
func (s *governanceService) GetLatestNodeEvaluation(ctx context.Context, orgID, targetNodeID string) (*model.EvaluationExecution, error) {
	if strings.TrimSpace(targetNodeID) == "" {
		return nil, fmt.Errorf("%w: empty target_node_id", ErrInvalidIdentifier)
	}
	if orgID == "" {
		orgID = model.DefaultOrganizationID
	}
	return s.store.GetLatestNodeEvaluation(ctx, orgID, targetNodeID)
}

// ListEvaluationExecutions queries evaluation records according to filter criteria.
func (s *governanceService) ListEvaluationExecutions(ctx context.Context, filter model.EvaluationFilter) ([]model.EvaluationExecution, error) {
	return s.store.ListEvaluationExecutions(ctx, filter)
}

// GetComplianceFinding retrieves a single compliance finding by ID.
func (s *governanceService) GetComplianceFinding(ctx context.Context, id string) (*model.ComplianceFinding, error) {
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("%w: empty finding id", ErrInvalidIdentifier)
	}
	return s.store.GetComplianceFinding(ctx, id)
}

// ListComplianceFindings queries compliance findings according to filter parameters.
func (s *governanceService) ListComplianceFindings(ctx context.Context, filter model.FindingFilter) ([]model.ComplianceFinding, error) {
	return s.store.ListComplianceFindings(ctx, filter)
}

// GetNodeComplianceSummary aggregates the latest compliance posture and open findings for a single node.
func (s *governanceService) GetNodeComplianceSummary(ctx context.Context, orgID, nodeID string) (*model.ComplianceRollupSummary, error) {
	if strings.TrimSpace(nodeID) == "" {
		return nil, fmt.Errorf("%w: empty node_id", ErrInvalidIdentifier)
	}
	if orgID == "" {
		orgID = model.DefaultOrganizationID
	}
	return s.aggregator.AggregateNodeSummary(ctx, orgID, nodeID)
}

// GetGroupComplianceSummary computes aggregated compliance rollup for a fleet group.
func (s *governanceService) GetGroupComplianceSummary(ctx context.Context, orgID, groupID string, includeSubgroups bool) (*model.ComplianceRollupSummary, error) {
	if strings.TrimSpace(groupID) == "" {
		return nil, fmt.Errorf("%w: empty group_id", ErrInvalidIdentifier)
	}
	if orgID == "" {
		orgID = model.DefaultOrganizationID
	}
	return s.aggregator.AggregateGroupSummary(ctx, orgID, groupID, includeSubgroups)
}

// GetOrgComplianceSummary computes tenant-wide compliance rollup across all organization nodes.
func (s *governanceService) GetOrgComplianceSummary(ctx context.Context, orgID string) (*model.ComplianceRollupSummary, error) {
	if orgID == "" {
		orgID = model.DefaultOrganizationID
	}
	return s.aggregator.AggregateOrgSummary(ctx, orgID)
}

// SimulatePolicyChanges executes candidate policy revisions and assignments in an isolated in-memory overlay.
func (s *governanceService) SimulatePolicyChanges(ctx context.Context, req *model.SimulationRequest) (*model.SimulationResult, error) {
	if req == nil {
		return nil, fmt.Errorf("%w: nil simulation request", ErrInvalidInput)
	}
	return s.simulation.SimulatePolicyChanges(ctx, req)
}

// ResolveNodeOwnership resolves operational ownership metadata across hierarchical levels for a node.
func (s *governanceService) ResolveNodeOwnership(ctx context.Context, nodeID string) (*model.ResolvedOwnership, error) {
	if strings.TrimSpace(nodeID) == "" {
		return nil, fmt.Errorf("%w: empty node id", ErrInvalidIdentifier)
	}
	return s.ownershipResolver.ResolveNodeOwnership(ctx, nodeID)
}

// CreateMaintenanceWindow validates, persists, and audits a new maintenance window.
func (s *governanceService) CreateMaintenanceWindow(ctx context.Context, window *model.MaintenanceWindow) error {
	if window == nil {
		return fmt.Errorf("%w: nil maintenance window", ErrInvalidInput)
	}
	if err := window.Validate(); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Verify organization exists
	if _, err := s.store.GetOrganization(ctx, window.OrgID); err != nil {
		return fmt.Errorf("%w: org_id %q does not exist", ErrOrganizationNotFound, window.OrgID)
	}

	// Verify ID uniqueness
	if existing, _ := s.store.GetMaintenanceWindow(ctx, window.ID); existing != nil {
		return fmt.Errorf("%w: maintenance window %q", ErrMaintenanceWindowExists, window.ID)
	}

	if window.Status == "" {
		window.Status = model.MaintenanceStatusDraft
	}
	now := s.clock.Now().UTC()
	if window.CreatedAt.IsZero() {
		window.CreatedAt = now
	}
	window.UpdatedAt = now

	if err := s.store.SaveMaintenanceWindow(ctx, window); err != nil {
		return err
	}

	_ = s.auditRecorder.RecordEvent(
		ctx,
		model.EventGovernanceMaintenanceCreated,
		model.AuditSeverityNotice,
		model.AuditOutcomeSuccess,
		window.ID,
		"create",
		fmt.Sprintf("Created maintenance window %q (%s)", window.Name, window.ID),
		model.AuditActor{Type: model.ActorTypeCLI, Identity: "governance-service"},
		map[string]string{
			"org_id":      window.OrgID,
			"window_id":   window.ID,
			"window_name": window.Name,
			"status":      string(window.Status),
		},
	)

	return nil
}

// GetMaintenanceWindow retrieves a maintenance window by its unique ID.
func (s *governanceService) GetMaintenanceWindow(ctx context.Context, id string) (*model.MaintenanceWindow, error) {
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("%w: empty maintenance window id", ErrInvalidIdentifier)
	}
	win, err := s.store.GetMaintenanceWindow(ctx, id)
	if err != nil || win == nil {
		return nil, fmt.Errorf("%w: %v", ErrMaintenanceWindowNotFound, err)
	}
	return win, nil
}

// ListMaintenanceWindows queries maintenance windows according to filter criteria.
func (s *governanceService) ListMaintenanceWindows(ctx context.Context, filter model.MaintenanceWindowFilter) ([]model.MaintenanceWindow, error) {
	return s.store.ListMaintenanceWindows(ctx, filter)
}

// UpdateMaintenanceWindow validates and updates a mutable maintenance window with lifecycle state transition checks.
func (s *governanceService) UpdateMaintenanceWindow(ctx context.Context, window *model.MaintenanceWindow) error {
	if window == nil {
		return fmt.Errorf("%w: nil maintenance window", ErrInvalidInput)
	}
	if err := window.Validate(); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	existing, err := s.store.GetMaintenanceWindow(ctx, window.ID)
	if err != nil || existing == nil {
		return fmt.Errorf("%w: maintenance window %s", ErrMaintenanceWindowNotFound, window.ID)
	}

	if window.Status != existing.Status {
		if !existing.CanTransitionTo(window.Status) {
			return fmt.Errorf("%w: cannot transition from %s to %s", ErrInvalidMaintenanceTransition, existing.Status, window.Status)
		}
	}

	window.CreatedAt = existing.CreatedAt
	window.UpdatedAt = s.clock.Now().UTC()

	if err := s.store.SaveMaintenanceWindow(ctx, window); err != nil {
		return err
	}

	_ = s.auditRecorder.RecordEvent(
		ctx,
		model.EventGovernanceMaintenanceUpdated,
		model.AuditSeverityNotice,
		model.AuditOutcomeSuccess,
		window.ID,
		"update",
		fmt.Sprintf("Updated maintenance window %q (%s)", window.Name, window.ID),
		model.AuditActor{Type: model.ActorTypeCLI, Identity: "governance-service"},
		map[string]string{
			"org_id":      window.OrgID,
			"window_id":   window.ID,
			"window_name": window.Name,
			"status":      string(window.Status),
		},
	)

	return nil
}

// DeleteMaintenanceWindow removes a maintenance window from storage.
func (s *governanceService) DeleteMaintenanceWindow(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("%w: empty maintenance window id", ErrInvalidIdentifier)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	existing, err := s.store.GetMaintenanceWindow(ctx, id)
	if err != nil || existing == nil {
		return fmt.Errorf("%w: maintenance window %s", ErrMaintenanceWindowNotFound, id)
	}

	if err := s.store.DeleteMaintenanceWindow(ctx, id); err != nil {
		return err
	}

	_ = s.auditRecorder.RecordEvent(
		ctx,
		model.EventGovernanceMaintenanceEnded,
		model.AuditSeverityNotice,
		model.AuditOutcomeSuccess,
		id,
		"delete",
		fmt.Sprintf("Deleted maintenance window %q", id),
		model.AuditActor{Type: model.ActorTypeCLI, Identity: "governance-service"},
		map[string]string{
			"org_id":    existing.OrgID,
			"window_id": id,
		},
	)

	return nil
}

// CancelMaintenanceWindow cancels an active or scheduled maintenance window.
func (s *governanceService) CancelMaintenanceWindow(ctx context.Context, id string, reason string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("%w: empty maintenance window id", ErrInvalidIdentifier)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	existing, err := s.store.GetMaintenanceWindow(ctx, id)
	if err != nil || existing == nil {
		return fmt.Errorf("%w: maintenance window %s", ErrMaintenanceWindowNotFound, id)
	}

	if !existing.CanTransitionTo(model.MaintenanceStatusCancelled) {
		return fmt.Errorf("%w: cannot transition from %s to %s", ErrInvalidMaintenanceTransition, existing.Status, model.MaintenanceStatusCancelled)
	}

	existing.Status = model.MaintenanceStatusCancelled
	existing.UpdatedAt = s.clock.Now().UTC()
	if existing.Metadata == nil {
		existing.Metadata = make(map[string]string)
	}
	if reason != "" {
		existing.Metadata["cancellation_reason"] = reason
	}

	if err := s.store.SaveMaintenanceWindow(ctx, existing); err != nil {
		return err
	}

	_ = s.auditRecorder.RecordEvent(
		ctx,
		model.EventGovernanceMaintenanceCancelled,
		model.AuditSeverityWarning,
		model.AuditOutcomeSuccess,
		id,
		"cancel",
		fmt.Sprintf("Cancelled maintenance window %q (%s): %s", existing.Name, id, reason),
		model.AuditActor{Type: model.ActorTypeCLI, Identity: "governance-service"},
		map[string]string{
			"org_id":    existing.OrgID,
			"window_id": id,
			"reason":    reason,
		},
	)

	return nil
}

// EvaluateMaintenanceWindows calculates all active maintenance windows for an organization at evalTime.
func (s *governanceService) EvaluateMaintenanceWindows(ctx context.Context, orgID string, evalTime time.Time) ([]model.MaintenanceWindow, error) {
	if orgID == "" {
		orgID = model.DefaultOrganizationID
	}
	return s.maintenanceEngine.EvaluateActiveWindows(ctx, orgID, evalTime)
}

// CreateEscalationPolicy validates, persists, and audits a new escalation policy.
func (s *governanceService) CreateEscalationPolicy(ctx context.Context, policy *model.EscalationPolicy) error {
	if policy == nil {
		return fmt.Errorf("%w: nil escalation policy", ErrInvalidInput)
	}
	if err := policy.Validate(); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := s.store.GetOrganization(ctx, policy.OrgID); err != nil {
		return fmt.Errorf("%w: org_id %q does not exist", ErrOrganizationNotFound, policy.OrgID)
	}

	if existing, _ := s.store.GetEscalationPolicy(ctx, policy.ID); existing != nil {
		return fmt.Errorf("%w: escalation policy %q", ErrEscalationPolicyExists, policy.ID)
	}

	now := s.clock.Now().UTC()
	if policy.CreatedAt.IsZero() {
		policy.CreatedAt = now
	}
	policy.UpdatedAt = now

	if err := s.store.SaveEscalationPolicy(ctx, policy); err != nil {
		return err
	}

	_ = s.auditRecorder.RecordEvent(
		ctx,
		model.EventGovernanceEscalationUpdated,
		model.AuditSeverityNotice,
		model.AuditOutcomeSuccess,
		policy.ID,
		"create",
		fmt.Sprintf("Created escalation policy %q (%s)", policy.Name, policy.ID),
		model.AuditActor{Type: model.ActorTypeCLI, Identity: "governance-service"},
		map[string]string{
			"org_id":      policy.OrgID,
			"policy_id":   policy.ID,
			"policy_name": policy.Name,
		},
	)

	return nil
}

// GetEscalationPolicy retrieves an escalation policy by ID.
func (s *governanceService) GetEscalationPolicy(ctx context.Context, id string) (*model.EscalationPolicy, error) {
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("%w: empty escalation policy id", ErrInvalidIdentifier)
	}
	policy, err := s.store.GetEscalationPolicy(ctx, id)
	if err != nil || policy == nil {
		return nil, fmt.Errorf("%w: %v", ErrEscalationPolicyNotFound, err)
	}
	return policy, nil
}

// ListEscalationPolicies queries escalation policies matching filter criteria.
func (s *governanceService) ListEscalationPolicies(ctx context.Context, filter model.EscalationPolicyFilter) ([]model.EscalationPolicy, error) {
	return s.store.ListEscalationPolicies(ctx, filter)
}

// UpdateEscalationPolicy validates and updates an existing escalation policy.
func (s *governanceService) UpdateEscalationPolicy(ctx context.Context, policy *model.EscalationPolicy) error {
	if policy == nil {
		return fmt.Errorf("%w: nil escalation policy", ErrInvalidInput)
	}
	if err := policy.Validate(); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	existing, err := s.store.GetEscalationPolicy(ctx, policy.ID)
	if err != nil || existing == nil {
		return fmt.Errorf("%w: escalation policy %s", ErrEscalationPolicyNotFound, policy.ID)
	}

	policy.CreatedAt = existing.CreatedAt
	policy.UpdatedAt = s.clock.Now().UTC()

	if err := s.store.SaveEscalationPolicy(ctx, policy); err != nil {
		return err
	}

	_ = s.auditRecorder.RecordEvent(
		ctx,
		model.EventGovernanceEscalationUpdated,
		model.AuditSeverityNotice,
		model.AuditOutcomeSuccess,
		policy.ID,
		"update",
		fmt.Sprintf("Updated escalation policy %q (%s)", policy.Name, policy.ID),
		model.AuditActor{Type: model.ActorTypeCLI, Identity: "governance-service"},
		map[string]string{
			"org_id":      policy.OrgID,
			"policy_id":   policy.ID,
			"policy_name": policy.Name,
		},
	)

	return nil
}

// DeleteEscalationPolicy deletes an escalation policy by ID.
func (s *governanceService) DeleteEscalationPolicy(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("%w: empty escalation policy id", ErrInvalidIdentifier)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	existing, err := s.store.GetEscalationPolicy(ctx, id)
	if err != nil || existing == nil {
		return fmt.Errorf("%w: escalation policy %s", ErrEscalationPolicyNotFound, id)
	}

	if err := s.store.DeleteEscalationPolicy(ctx, id); err != nil {
		return err
	}

	_ = s.auditRecorder.RecordEvent(
		ctx,
		model.EventGovernanceEscalationUpdated,
		model.AuditSeverityNotice,
		model.AuditOutcomeSuccess,
		id,
		"delete",
		fmt.Sprintf("Deleted escalation policy %q", id),
		model.AuditActor{Type: model.ActorTypeCLI, Identity: "governance-service"},
		map[string]string{
			"org_id":    existing.OrgID,
			"policy_id": id,
		},
	)

	return nil
}

// EvaluateIncidentEscalation evaluates the escalation timeline and active notification targets for an incident.
func (s *governanceService) EvaluateIncidentEscalation(ctx context.Context, orgID string, incident *incidents.Incident, evalTime time.Time) (*model.EscalationEvaluationResult, error) {
	return s.escalationEngine.EvaluateIncident(ctx, orgID, incident, evalTime)
}

// EvaluateSuppression evaluates policy-driven alert and finding suppression and persists the resulting decision record.
func (s *governanceService) EvaluateSuppression(ctx context.Context, req SuppressionEvaluationRequest) (*model.SuppressionDecision, error) {
	decision, err := s.suppressionEngine.Evaluate(ctx, req)
	if decision != nil {
		s.mu.Lock()
		_ = s.store.SaveSuppressionDecision(ctx, decision)
		s.mu.Unlock()
	}
	return decision, err
}

// GetSuppressionDecision retrieves a suppression decision by ID.
func (s *governanceService) GetSuppressionDecision(ctx context.Context, id string) (*model.SuppressionDecision, error) {
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("%w: empty suppression decision id", ErrInvalidIdentifier)
	}
	decision, err := s.store.GetSuppressionDecision(ctx, id)
	if err != nil || decision == nil {
		return nil, fmt.Errorf("%w: %v", ErrSuppressionDecisionNotFound, err)
	}
	return decision, nil
}

// ListSuppressionDecisions queries suppression decisions matching filter criteria.
func (s *governanceService) ListSuppressionDecisions(ctx context.Context, filter model.SuppressionFilter) ([]model.SuppressionDecision, error) {
	return s.store.ListSuppressionDecisions(ctx, filter)
}

// RecordAuditEvent persists and sanitizes a governance audit event.
func (s *governanceService) RecordAuditEvent(ctx context.Context, event model.AuditEvent) error {
	return s.auditRecorder.Record(ctx, event)
}

// QueryAuditEvents queries audit log events matching the specified filter.
func (s *governanceService) QueryAuditEvents(ctx context.Context, filter storage.AuditFilter) ([]model.AuditEvent, error) {
	return s.store.QueryAuditEvents(ctx, filter)
}
