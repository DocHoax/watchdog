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

// ReadOnlyGovernanceService defines the read-only contract for organizations, groups, and node metadata.
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
}

type governanceService struct {
	store storage.Storage
	mu    sync.RWMutex
}

// NewGovernanceService constructs an instance of GovernanceService backed by storage.
func NewGovernanceService(store storage.Storage) GovernanceService {
	return &governanceService{
		store: store,
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
	allGroups, err := s.store.ListFleetGroups(ctx, group.OrgID)
	if err != nil {
		return fmt.Errorf("failed to list fleet groups for hierarchy validation: %w", err)
	}

	groupMap := make(map[string]*model.FleetGroup, len(allGroups)+1)
	for i := range allGroups {
		groupMap[allGroups[i].ID] = &allGroups[i]
	}

	if group.ParentGroupID != "" {
		parent, exists := groupMap[group.ParentGroupID]
		if !exists {
			return fmt.Errorf("%w: parent_group_id %q", ErrParentNotFound, group.ParentGroupID)
		}
		if parent.OrgID != group.OrgID {
			return fmt.Errorf("%w: parent org is %q, child org is %q", ErrCrossOrgParent, parent.OrgID, group.OrgID)
		}
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
		parent, exists := groupMap[group.ParentGroupID]
		if !exists {
			return fmt.Errorf("%w: parent_group_id %q", ErrParentNotFound, group.ParentGroupID)
		}
		if parent.OrgID != group.OrgID {
			return fmt.Errorf("%w: parent org is %q, child org is %q", ErrCrossOrgParent, parent.OrgID, group.OrgID)
		}
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
