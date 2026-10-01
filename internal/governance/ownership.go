package governance

import (
	"context"
	"fmt"
	"strings"

	"github.com/DocHoax/watchdog/internal/storage"
	"github.com/DocHoax/watchdog/pkg/model"
)

// OwnershipResolver performs hierarchical, deterministic resolution of node operational ownership,
// metadata tags, escalation contacts, and business classification attributes across organizational boundaries.
type OwnershipResolver struct {
	store storage.ReadOnlyStorage
	clock Clock
}

// NewOwnershipResolver constructs a new OwnershipResolver.
func NewOwnershipResolver(store storage.ReadOnlyStorage, clock Clock) *OwnershipResolver {
	if clock == nil {
		clock = RealClock{}
	}
	return &OwnershipResolver{
		store: store,
		clock: clock,
	}
}

type ownershipLevel struct {
	sourceLevel model.OwnershipSourceLevel
	sourceID    string
	metadata    *model.NodeOwnershipMetadata
	rawMeta     map[string]string
}

// ResolveNodeOwnership resolves the effective ownership metadata for a node identified by nodeID.
func (r *OwnershipResolver) ResolveNodeOwnership(ctx context.Context, nodeID string) (*model.ResolvedOwnership, error) {
	if strings.TrimSpace(nodeID) == "" {
		return nil, fmt.Errorf("%w: empty node id", ErrInvalidInput)
	}
	if r.store == nil {
		return nil, fmt.Errorf("%w: nil storage backend", ErrInvalidInput)
	}

	node, err := r.store.GetFleetNode(ctx, nodeID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNodeNotFound, err)
	}
	if node == nil {
		return nil, fmt.Errorf("%w: node %s", ErrNodeNotFound, nodeID)
	}

	return r.ResolveOwnershipForNode(ctx, node)
}

// ResolveOwnershipForNode resolves the effective ownership for a given FleetNode instance.
func (r *OwnershipResolver) ResolveOwnershipForNode(ctx context.Context, node *model.FleetNode) (*model.ResolvedOwnership, error) {
	if node == nil {
		return nil, fmt.Errorf("%w: nil fleet node", ErrInvalidInput)
	}

	orgID := model.DefaultOrganizationID
	if node.Metadata != nil {
		if val, ok := node.Metadata["org_id"]; ok && val != "" {
			orgID = val
		} else if val, ok := node.Metadata["organization_id"]; ok && val != "" {
			orgID = val
		}
	}

	// 1. Discover node group memberships and build leaf-to-root group ancestry chains
	var leafToRootGroups []model.FleetGroup
	var deepestPath string

	if r.store != nil && node.Identity.NodeID != "" {
		directGroups, err := r.store.GetNodeGroups(ctx, node.Identity.NodeID)
		if err == nil && len(directGroups) > 0 {
			type groupWithDepth struct {
				group model.FleetGroup
				depth int
			}
			var allChainGroups []groupWithDepth
			visited := make(map[string]bool)

			for _, dg := range directGroups {
				curr := dg
				chainDepth := 0
				var branch []model.FleetGroup

				for {
					if visited[curr.ID] {
						break
					}
					visited[curr.ID] = true
					branch = append(branch, curr)
					chainDepth++

					if curr.Path != "" && len(curr.Path) > len(deepestPath) {
						deepestPath = curr.Path
					}

					if curr.ParentGroupID == "" {
						break
					}
					parent, pErr := r.store.GetFleetGroup(ctx, curr.ParentGroupID)
					if pErr != nil || parent == nil {
						break
					}
					curr = *parent
				}

				// branch has directGroup at index 0 (leaf) up to root at end
				for d, g := range branch {
					allChainGroups = append(allChainGroups, groupWithDepth{group: g, depth: d})
				}
			}

			// Deduplicate preserving leaf-first order
			seenID := make(map[string]bool)
			for _, gd := range allChainGroups {
				if !seenID[gd.group.ID] {
					seenID[gd.group.ID] = true
					leafToRootGroups = append(leafToRootGroups, gd.group)
				}
			}
		}
	}

	// 2. Fetch Organization entity
	var org *model.Organization
	if r.store != nil {
		if o, err := r.store.GetOrganization(ctx, orgID); err == nil && o != nil {
			org = o
		}
	}

	return r.ResolveHierarchy(node, leafToRootGroups, org, deepestPath), nil
}

// ResolveHierarchy performs pure in-memory merging of ownership metadata across hierarchy levels.
func (r *OwnershipResolver) ResolveHierarchy(
	node *model.FleetNode,
	leafToRootGroups []model.FleetGroup,
	org *model.Organization,
	hierarchyPath string,
) *model.ResolvedOwnership {
	now := r.clock.Now().UTC()

	orgID := model.DefaultOrganizationID
	nodeID := ""
	if node != nil {
		nodeID = node.Identity.NodeID
		if node.Metadata != nil {
			if val, ok := node.Metadata["org_id"]; ok && val != "" {
				orgID = val
			} else if val, ok := node.Metadata["organization_id"]; ok && val != "" {
				orgID = val
			}
		}
	} else if org != nil {
		orgID = org.ID
	}

	var levels []ownershipLevel

	// Level 1: Direct Node Metadata
	if node != nil && node.Metadata != nil {
		nodeOwnership := model.NodeOwnershipFromMetadata(node.Metadata)
		levels = append(levels, ownershipLevel{
			sourceLevel: model.OwnershipLevelNode,
			sourceID:    nodeID,
			metadata:    nodeOwnership,
			rawMeta:     node.Metadata,
		})
	}

	// Level 2: Fleet Group Hierarchy (Leaf to Root)
	for _, g := range leafToRootGroups {
		groupOwnership := model.NodeOwnershipFromMetadata(g.Metadata)
		levels = append(levels, ownershipLevel{
			sourceLevel: model.OwnershipLevelFleetGroup,
			sourceID:    g.ID,
			metadata:    groupOwnership,
			rawMeta:     g.Metadata,
		})
		if hierarchyPath == "" && g.Path != "" {
			hierarchyPath = g.Path
		}
	}

	// Level 3: Service Context
	serviceName := ""
	if node != nil && node.Metadata != nil {
		if s, ok := node.Metadata["service"]; ok && s != "" {
			serviceName = s
		} else if s, ok := node.Metadata["service_name"]; ok && s != "" {
			serviceName = s
		} else if s, ok := node.Metadata[model.MetaPrefixGovernance+"service"]; ok && s != "" {
			serviceName = s
		}
	}
	if serviceName != "" {
		serviceMeta := make(map[string]string)
		if node != nil && node.Metadata != nil {
			for k, v := range node.Metadata {
				if after, ok := strings.CutPrefix(k, "service."); ok {
					serviceMeta[model.MetaPrefixGovernance+after] = v
				}
			}
		}
		serviceOwnership := model.NodeOwnershipFromMetadata(serviceMeta)
		if serviceOwnership != nil || len(serviceMeta) > 0 {
			levels = append(levels, ownershipLevel{
				sourceLevel: model.OwnershipLevelService,
				sourceID:    serviceName,
				metadata:    serviceOwnership,
				rawMeta:     serviceMeta,
			})
		}
	}

	// Level 4: Organization Defaults
	if org != nil && org.Metadata != nil {
		orgOwnership := model.NodeOwnershipFromMetadata(org.Metadata)
		levels = append(levels, ownershipLevel{
			sourceLevel: model.OwnershipLevelOrganization,
			sourceID:    org.ID,
			metadata:    orgOwnership,
			rawMeta:     org.Metadata,
		})
	}

	res := &model.ResolvedOwnership{
		NodeID:           nodeID,
		OrgID:            orgID,
		HierarchyPath:    hierarchyPath,
		CustomProperties: make(map[string]string),
		ResolvedAt:       now,
	}

	// Cascading field resolution (Highest priority first: Node -> Group leaf..root -> Service -> Org)
	for _, lvl := range levels {
		if lvl.metadata != nil {
			if res.OwnerTeam == "" && lvl.metadata.OwnerTeam != "" {
				res.OwnerTeam = lvl.metadata.OwnerTeam
				if res.SourceLevel == "" {
					res.SourceLevel = lvl.sourceLevel
					res.SourceID = lvl.sourceID
				}
			}
			if res.ContactEmail == "" && lvl.metadata.ContactEmail != "" {
				res.ContactEmail = lvl.metadata.ContactEmail
			}
			if res.ContactChannel == "" && lvl.metadata.ContactChannel != "" {
				res.ContactChannel = lvl.metadata.ContactChannel
			}
			if res.Environment == "" && lvl.metadata.Environment != "" {
				res.Environment = lvl.metadata.Environment
			}
			if res.Region == "" && lvl.metadata.Region != "" {
				res.Region = lvl.metadata.Region
			}
			if res.DataClassification == "" && lvl.metadata.DataClassification != "" {
				res.DataClassification = lvl.metadata.DataClassification
			}
			if res.CostCenter == "" && lvl.metadata.CostCenter != "" {
				res.CostCenter = lvl.metadata.CostCenter
			}
			if res.BusinessCriticality == "" && lvl.metadata.BusinessCriticality.IsValid() {
				res.BusinessCriticality = lvl.metadata.BusinessCriticality
			}
			if res.Lifecycle == "" && lvl.metadata.Lifecycle.IsValid() {
				res.Lifecycle = lvl.metadata.Lifecycle
			}
			for k, v := range lvl.metadata.CustomProperties {
				if _, exists := res.CustomProperties[k]; !exists {
					res.CustomProperties[k] = v
				}
			}
		}

		// Also check rawMeta for non-prefixed standard fallback keys
		if res.Environment == "" && lvl.rawMeta != nil {
			if env, ok := lvl.rawMeta["environment"]; ok && env != "" {
				res.Environment = env
			} else if env, ok := lvl.rawMeta["env"]; ok && env != "" {
				res.Environment = env
			}
		}
		if res.Region == "" && lvl.rawMeta != nil {
			if reg, ok := lvl.rawMeta["region"]; ok && reg != "" {
				res.Region = reg
			}
		}
	}

	// Apply Sensible Defaults if unresolved
	if res.BusinessCriticality == "" {
		res.BusinessCriticality = model.CriticalityMedium
	}
	if res.Lifecycle == "" {
		res.Lifecycle = model.LifecycleActive
	}
	if res.SourceLevel == "" {
		if len(levels) > 0 {
			res.SourceLevel = levels[0].sourceLevel
			res.SourceID = levels[0].sourceID
		} else {
			res.SourceLevel = model.OwnershipLevelOrganization
			res.SourceID = orgID
		}
	}

	return res
}
