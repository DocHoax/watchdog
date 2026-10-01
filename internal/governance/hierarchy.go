package governance

import (
	"sort"
	"strings"

	"github.com/DocHoax/watchdog/pkg/model"
)

// DetectCycle checks if assigning candidateParentID as the parent of targetGroupID would introduce a cycle.
func DetectCycle(groupMap map[string]*model.FleetGroup, targetGroupID string, candidateParentID string) bool {
	if targetGroupID == "" || candidateParentID == "" {
		return false
	}
	if targetGroupID == candidateParentID {
		return true
	}

	visited := make(map[string]bool)
	current := candidateParentID

	for current != "" {
		if current == targetGroupID {
			return true
		}
		if visited[current] {
			// Found an existing cycle in the graph
			return true
		}
		visited[current] = true

		parent, exists := groupMap[current]
		if !exists || parent == nil {
			break
		}
		current = parent.ParentGroupID
	}

	return false
}

// ComputeGroupPath generates the canonical materialized path for a fleet group.
// Example: "/grp-root/grp-tier1/grp-leaf"
func ComputeGroupPath(group *model.FleetGroup, groupMap map[string]*model.FleetGroup) string {
	if group == nil {
		return ""
	}

	var segments []string
	visited := make(map[string]bool)
	currentID := group.ID

	for currentID != "" {
		if visited[currentID] {
			break // Guard against infinite loop
		}
		visited[currentID] = true
		segments = append(segments, currentID)

		g, ok := groupMap[currentID]
		if !ok || g == nil {
			break
		}
		currentID = g.ParentGroupID
	}

	// Reverse segments so root comes first
	for i, j := 0, len(segments)-1; i < j; i, j = i+1, j-1 {
		segments[i], segments[j] = segments[j], segments[i]
	}

	return "/" + strings.Join(segments, "/")
}

// BuildHierarchyTree constructs a forest of hierarchy trees from a flat slice of groups and member mappings.
func BuildHierarchyTree(groups []model.FleetGroup, groupMembers map[string][]string) []*model.FleetGroupHierarchyNode {
	groupMap := make(map[string]model.FleetGroup, len(groups))
	childrenMap := make(map[string][]model.FleetGroup)

	for _, g := range groups {
		groupMap[g.ID] = g
	}

	var rootGroups []model.FleetGroup
	for _, g := range groups {
		if g.ParentGroupID == "" {
			rootGroups = append(rootGroups, g)
		} else if _, exists := groupMap[g.ParentGroupID]; !exists {
			// Parent does not exist in this scope, treat as root
			rootGroups = append(rootGroups, g)
		} else {
			childrenMap[g.ParentGroupID] = append(childrenMap[g.ParentGroupID], g)
		}
	}

	// Sort roots by name for deterministic ordering
	sort.Slice(rootGroups, func(i, j int) bool {
		return rootGroups[i].Name < rootGroups[j].Name
	})

	var roots []*model.FleetGroupHierarchyNode
	for _, root := range rootGroups {
		node := buildTreeNode(root, childrenMap, groupMembers)
		roots = append(roots, node)
	}

	return roots
}

func buildTreeNode(
	group model.FleetGroup,
	childrenMap map[string][]model.FleetGroup,
	groupMembers map[string][]string,
) *model.FleetGroupHierarchyNode {
	direct := groupMembers[group.ID]
	if direct == nil {
		direct = []string{}
	}

	node := &model.FleetGroupHierarchyNode{
		Group:         group,
		DirectMembers: direct,
	}

	children := childrenMap[group.ID]
	sort.Slice(children, func(i, j int) bool {
		return children[i].Name < children[j].Name
	})

	uniqueNodes := make(map[string]bool)
	for _, nid := range direct {
		uniqueNodes[nid] = true
	}

	totalSubgroups := 0

	for _, child := range children {
		childNode := buildTreeNode(child, childrenMap, groupMembers)
		node.Children = append(node.Children, childNode)
		totalSubgroups += 1 + childNode.SubgroupCount

		// Collect all node IDs from child subtree
		collectChildNodes(childNode, uniqueNodes)
	}

	node.SubgroupCount = totalSubgroups
	node.TotalNodeCount = len(uniqueNodes)

	return node
}

func collectChildNodes(node *model.FleetGroupHierarchyNode, accumulator map[string]bool) {
	if node == nil {
		return
	}
	for _, nid := range node.DirectMembers {
		accumulator[nid] = true
	}
	for _, child := range node.Children {
		collectChildNodes(child, accumulator)
	}
}

// CollectSubtreeGroupIDs traverses child relationships to gather all descendant group IDs including the root.
func CollectSubtreeGroupIDs(rootGroupID string, groups []model.FleetGroup) []string {
	if rootGroupID == "" {
		return nil
	}

	childrenMap := make(map[string][]string)
	for _, g := range groups {
		if g.ParentGroupID != "" {
			childrenMap[g.ParentGroupID] = append(childrenMap[g.ParentGroupID], g.ID)
		}
	}

	var result []string
	visited := make(map[string]bool)

	var walk func(id string)
	walk = func(id string) {
		if visited[id] {
			return
		}
		visited[id] = true
		result = append(result, id)

		for _, childID := range childrenMap[id] {
			walk(childID)
		}
	}

	walk(rootGroupID)
	return result
}

// CollectSubtreeNodeIDs retrieves all unique node IDs in a group and its descendants.
func CollectSubtreeNodeIDs(rootGroupID string, groups []model.FleetGroup, groupMembers map[string][]string) []string {
	subtreeGroupIDs := CollectSubtreeGroupIDs(rootGroupID, groups)
	uniqueNodes := make(map[string]bool)

	for _, gid := range subtreeGroupIDs {
		for _, nid := range groupMembers[gid] {
			uniqueNodes[nid] = true
		}
	}

	var result []string
	for nid := range uniqueNodes {
		result = append(result, nid)
	}
	sort.Strings(result)
	return result
}
