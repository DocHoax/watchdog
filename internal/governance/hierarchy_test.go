package governance

import (
	"reflect"
	"testing"

	"github.com/DocHoax/watchdog/pkg/model"
)

func TestDetectCycle(t *testing.T) {
	groupMap := map[string]*model.FleetGroup{
		"grp-root": {ID: "grp-root", ParentGroupID: ""},
		"grp-a":    {ID: "grp-a", ParentGroupID: "grp-root"},
		"grp-b":    {ID: "grp-b", ParentGroupID: "grp-a"},
		"grp-c":    {ID: "grp-c", ParentGroupID: "grp-b"},
	}

	// Empty IDs
	if DetectCycle(groupMap, "", "grp-root") {
		t.Errorf("expected false for empty targetGroupID")
	}
	if DetectCycle(groupMap, "grp-a", "") {
		t.Errorf("expected false for empty candidateParentID")
	}

	// Self-parenting
	if !DetectCycle(groupMap, "grp-a", "grp-a") {
		t.Errorf("expected cycle detection when target == candidate")
	}

	// Valid parent assignment
	if DetectCycle(groupMap, "grp-new", "grp-c") {
		t.Errorf("expected false for new group child of grp-c")
	}

	// Direct cycle: making grp-root child of grp-a
	if !DetectCycle(groupMap, "grp-root", "grp-a") {
		t.Errorf("expected cycle when assigning grp-root child of grp-a")
	}

	// Transitive cycle: making grp-root child of grp-c
	if !DetectCycle(groupMap, "grp-root", "grp-c") {
		t.Errorf("expected cycle when assigning grp-root child of grp-c")
	}

	// Subtree cycle: making grp-a child of grp-c
	if !DetectCycle(groupMap, "grp-a", "grp-c") {
		t.Errorf("expected cycle when assigning grp-a child of grp-c")
	}

	// Non-existent parent in graph
	if DetectCycle(groupMap, "grp-a", "grp-nonexistent") {
		t.Errorf("expected false when candidate parent is not found")
	}
}

func TestComputeGroupPath(t *testing.T) {
	if ComputeGroupPath(nil, nil) != "" {
		t.Errorf("expected empty path for nil group")
	}

	groupMap := map[string]*model.FleetGroup{
		"grp-root": {ID: "grp-root", ParentGroupID: ""},
		"grp-tier1": {ID: "grp-tier1", ParentGroupID: "grp-root"},
		"grp-tier2": {ID: "grp-tier2", ParentGroupID: "grp-tier1"},
	}

	// Test root path
	rootPath := ComputeGroupPath(groupMap["grp-root"], groupMap)
	if rootPath != "/grp-root" {
		t.Errorf("expected /grp-root, got %s", rootPath)
	}

	// Test leaf path
	leafPath := ComputeGroupPath(groupMap["grp-tier2"], groupMap)
	if leafPath != "/grp-root/grp-tier1/grp-tier2" {
		t.Errorf("expected /grp-root/grp-tier1/grp-tier2, got %s", leafPath)
	}

	// Test orphaned group path
	orphan := &model.FleetGroup{ID: "grp-orphan", ParentGroupID: "grp-missing"}
	orphanPath := ComputeGroupPath(orphan, groupMap)
	if orphanPath != "/grp-orphan" {
		t.Errorf("expected /grp-orphan for orphaned group, got %s", orphanPath)
	}
}

func TestBuildHierarchyTree(t *testing.T) {
	groups := []model.FleetGroup{
		{ID: "grp-root-b", Name: "Root B", ParentGroupID: ""},
		{ID: "grp-root-a", Name: "Root A", ParentGroupID: ""},
		{ID: "grp-a1", Name: "Child A1", ParentGroupID: "grp-root-a"},
		{ID: "grp-a2", Name: "Child A2", ParentGroupID: "grp-root-a"},
		{ID: "grp-a1-leaf", Name: "Leaf A1-1", ParentGroupID: "grp-a1"},
	}

	groupMembers := map[string][]string{
		"grp-root-a":   {"node-1"},
		"grp-a1":       {"node-2", "node-3"},
		"grp-a1-leaf":  {"node-3", "node-4"}, // node-3 shared across levels
		"grp-a2":       {"node-5"},
		"grp-root-b":   {"node-6"},
	}

	forest := BuildHierarchyTree(groups, groupMembers)
	if len(forest) != 2 {
		t.Fatalf("expected 2 root trees, got %d", len(forest))
	}

	// Verify root alphabetical sorting: "Root A" before "Root B"
	if forest[0].Group.ID != "grp-root-a" || forest[1].Group.ID != "grp-root-b" {
		t.Fatalf("expected roots sorted by name, got [%s, %s]", forest[0].Group.ID, forest[1].Group.ID)
	}

	rootA := forest[0]
	if rootA.SubgroupCount != 3 { // grp-a1, grp-a2, grp-a1-leaf
		t.Errorf("expected SubgroupCount 3 for Root A, got %d", rootA.SubgroupCount)
	}

	// Unique nodes in root A tree: node-1, node-2, node-3, node-4, node-5 (5 distinct nodes)
	if rootA.TotalNodeCount != 5 {
		t.Errorf("expected TotalNodeCount 5 for Root A, got %d", rootA.TotalNodeCount)
	}

	// Check child ordering under Root A: "Child A1" before "Child A2"
	if len(rootA.Children) != 2 {
		t.Fatalf("expected 2 children under Root A, got %d", len(rootA.Children))
	}
	if rootA.Children[0].Group.ID != "grp-a1" || rootA.Children[1].Group.ID != "grp-a2" {
		t.Fatalf("children not sorted properly: [%s, %s]", rootA.Children[0].Group.ID, rootA.Children[1].Group.ID)
	}

	childA1 := rootA.Children[0]
	if childA1.SubgroupCount != 1 { // grp-a1-leaf
		t.Errorf("expected SubgroupCount 1 for Child A1, got %d", childA1.SubgroupCount)
	}
	// Child A1 nodes: node-2, node-3, node-4 (3 distinct)
	if childA1.TotalNodeCount != 3 {
		t.Errorf("expected TotalNodeCount 3 for Child A1, got %d", childA1.TotalNodeCount)
	}
}

func TestCollectSubtreeGroupIDs_And_NodeIDs(t *testing.T) {
	groups := []model.FleetGroup{
		{ID: "grp-root", ParentGroupID: ""},
		{ID: "grp-sub1", ParentGroupID: "grp-root"},
		{ID: "grp-sub2", ParentGroupID: "grp-root"},
		{ID: "grp-leaf1", ParentGroupID: "grp-sub1"},
	}

	groupMembers := map[string][]string{
		"grp-root":  {"node-a"},
		"grp-sub1":  {"node-b", "node-c"},
		"grp-sub2":  {"node-d"},
		"grp-leaf1": {"node-c", "node-e"}, // node-c duplicated
	}

	// Empty
	if CollectSubtreeGroupIDs("", groups) != nil {
		t.Errorf("expected nil for empty group ID")
	}

	// Subtree from grp-sub1
	sub1GroupIDs := CollectSubtreeGroupIDs("grp-sub1", groups)
	expectedSub1GroupIDs := []string{"grp-sub1", "grp-leaf1"}
	if !reflect.DeepEqual(sub1GroupIDs, expectedSub1GroupIDs) {
		t.Errorf("expected group IDs %v, got %v", expectedSub1GroupIDs, sub1GroupIDs)
	}

	// Subtree nodes from grp-sub1: node-b, node-c, node-e (sorted & unique)
	sub1NodeIDs := CollectSubtreeNodeIDs("grp-sub1", groups, groupMembers)
	expectedSub1NodeIDs := []string{"node-b", "node-c", "node-e"}
	if !reflect.DeepEqual(sub1NodeIDs, expectedSub1NodeIDs) {
		t.Errorf("expected node IDs %v, got %v", expectedSub1NodeIDs, sub1NodeIDs)
	}

	// Entire tree from grp-root
	allNodes := CollectSubtreeNodeIDs("grp-root", groups, groupMembers)
	expectedAllNodes := []string{"node-a", "node-b", "node-c", "node-d", "node-e"}
	if !reflect.DeepEqual(allNodes, expectedAllNodes) {
		t.Errorf("expected all node IDs %v, got %v", expectedAllNodes, allNodes)
	}
}
