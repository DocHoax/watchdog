package governance

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

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
}

type governanceService struct {
	store    storage.Storage
	resolver *PolicyResolver
	mu       sync.RWMutex
}

// NewGovernanceService constructs an instance of GovernanceService backed by storage.
func NewGovernanceService(store storage.Storage) GovernanceService {
	return &governanceService{
		store:    store,
		resolver: NewPolicyResolver(store),
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
