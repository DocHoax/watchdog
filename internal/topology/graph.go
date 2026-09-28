package topology

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

// Graph represents a directed, cycle-aware in-memory service and dependency topology graph.
type Graph struct {
	mu           sync.RWMutex
	nodes        map[string]TopologyNode
	outEdges     map[string]map[string][]Dependency // sourceID -> targetID -> slice of dependencies
	inEdges      map[string]map[string][]Dependency // targetID -> sourceID -> slice of dependencies
	lastModified time.Time
}

// NewGraph creates an initialized, thread-safe Graph instance.
func NewGraph() *Graph {
	return &Graph{
		nodes:        make(map[string]TopologyNode),
		outEdges:     make(map[string]map[string][]Dependency),
		inEdges:      make(map[string]map[string][]Dependency),
		lastModified: time.Now().UTC(),
	}
}

// AddNode inserts or updates a node in the graph.
func (g *Graph) AddNode(node TopologyNode) {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := time.Now().UTC()
	if existing, exists := g.nodes[node.ID]; exists {
		if node.FirstObserved.IsZero() {
			node.FirstObserved = existing.FirstObserved
		}
		if node.LastObserved.IsZero() {
			node.LastObserved = now
		}
		if node.Tags == nil && existing.Tags != nil {
			node.Tags = existing.Tags
		}
		if node.Metadata == nil && existing.Metadata != nil {
			node.Metadata = existing.Metadata
		}
	} else {
		if node.FirstObserved.IsZero() {
			node.FirstObserved = now
		}
		if node.LastObserved.IsZero() {
			node.LastObserved = now
		}
	}

	if node.Tags == nil {
		node.Tags = make(map[string]string)
	}
	if node.Metadata == nil {
		node.Metadata = make(map[string]string)
	}

	g.nodes[node.ID] = node
	g.lastModified = now
}

// GetNode retrieves a node by ID.
func (g *Graph) GetNode(id string) (TopologyNode, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	node, exists := g.nodes[id]
	return node, exists
}

// HasNode checks if a node exists.
func (g *Graph) HasNode(id string) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()

	_, exists := g.nodes[id]
	return exists
}

// RemoveNode removes a node and all connected incoming/outgoing edges.
func (g *Graph) RemoveNode(id string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, exists := g.nodes[id]; !exists {
		return false
	}

	delete(g.nodes, id)

	// Clean outEdges from this node
	if targets, exists := g.outEdges[id]; exists {
		for targetID := range targets {
			if sources, ok := g.inEdges[targetID]; ok {
				delete(sources, id)
				if len(sources) == 0 {
					delete(g.inEdges, targetID)
				}
			}
		}
		delete(g.outEdges, id)
	}

	// Clean inEdges to this node
	if sources, exists := g.inEdges[id]; exists {
		for sourceID := range sources {
			if targets, ok := g.outEdges[sourceID]; ok {
				delete(targets, id)
				if len(targets) == 0 {
					delete(g.outEdges, sourceID)
				}
			}
		}
		delete(g.inEdges, id)
	}

	g.lastModified = time.Now().UTC()
	return true
}

// NodeCount returns the total number of nodes in the graph.
func (g *Graph) NodeCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.nodes)
}

// EdgeCount returns the total number of directed dependency edges in the graph.
func (g *Graph) EdgeCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()

	count := 0
	for _, targets := range g.outEdges {
		for _, deps := range targets {
			count += len(deps)
		}
	}
	return count
}

// AddDependency inserts or updates a directed dependency edge.
func (g *Graph) AddDependency(dep Dependency) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if dep.ID == "" {
		dep.ID = fmt.Sprintf("%s->%s:%s", dep.SourceID, dep.TargetID, dep.Type)
	}
	if dep.Weight <= 0 {
		dep.Weight = 1.0
	}
	if dep.Confidence == "" {
		dep.Confidence = ConfidenceHigh
	}
	now := time.Now().UTC()
	if dep.FirstObserved.IsZero() {
		dep.FirstObserved = now
	}
	if dep.LastObserved.IsZero() {
		dep.LastObserved = now
	}
	if dep.Metadata == nil {
		dep.Metadata = make(map[string]string)
	}

	// Ensure source outEdges map
	if _, ok := g.outEdges[dep.SourceID]; !ok {
		g.outEdges[dep.SourceID] = make(map[string][]Dependency)
	}
	// Ensure target inEdges map
	if _, ok := g.inEdges[dep.TargetID]; !ok {
		g.inEdges[dep.TargetID] = make(map[string][]Dependency)
	}

	// Update or append in outEdges
	existingOut := g.outEdges[dep.SourceID][dep.TargetID]
	replaced := false
	for i, existing := range existingOut {
		if existing.Type == dep.Type {
			if dep.FirstObserved.IsZero() {
				dep.FirstObserved = existing.FirstObserved
			}
			existingOut[i] = dep
			replaced = true
			break
		}
	}
	if !replaced {
		existingOut = append(existingOut, dep)
	}
	g.outEdges[dep.SourceID][dep.TargetID] = existingOut

	// Update or append in inEdges
	existingIn := g.inEdges[dep.TargetID][dep.SourceID]
	replacedIn := false
	for i, existing := range existingIn {
		if existing.Type == dep.Type {
			if dep.FirstObserved.IsZero() {
				dep.FirstObserved = existing.FirstObserved
			}
			existingIn[i] = dep
			replacedIn = true
			break
		}
	}
	if !replacedIn {
		existingIn = append(existingIn, dep)
	}
	g.inEdges[dep.TargetID][dep.SourceID] = existingIn

	g.lastModified = now
}

// RemoveDependency removes a specific dependency relationship between two nodes.
func (g *Graph) RemoveDependency(sourceID, targetID string, relType RelationshipType) bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	removed := false

	if targets, ok := g.outEdges[sourceID]; ok {
		if deps, ok := targets[targetID]; ok {
			var filtered []Dependency
			for _, d := range deps {
				if relType != "" && d.Type != relType {
					filtered = append(filtered, d)
				} else {
					removed = true
				}
			}
			if len(filtered) > 0 {
				targets[targetID] = filtered
			} else {
				delete(targets, targetID)
				if len(targets) == 0 {
					delete(g.outEdges, sourceID)
				}
			}
		}
	}

	if sources, ok := g.inEdges[targetID]; ok {
		if deps, ok := sources[sourceID]; ok {
			var filtered []Dependency
			for _, d := range deps {
				if relType != "" && d.Type != relType {
					filtered = append(filtered, d)
				}
			}
			if len(filtered) > 0 {
				sources[sourceID] = filtered
			} else {
				delete(sources, sourceID)
				if len(sources) == 0 {
					delete(g.inEdges, targetID)
				}
			}
		}
	}

	if removed {
		g.lastModified = time.Now().UTC()
	}
	return removed
}

// GetAllNodes returns all nodes in the graph, deterministically sorted by ID.
func (g *Graph) GetAllNodes() []TopologyNode {
	g.mu.RLock()
	defer g.mu.RUnlock()

	nodes := make([]TopologyNode, 0, len(g.nodes))
	for _, n := range g.nodes {
		nodes = append(nodes, n)
	}
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].ID < nodes[j].ID
	})
	return nodes
}

// GetAllDependencies returns all dependency edges in the graph, deterministically sorted.
func (g *Graph) GetAllDependencies() []Dependency {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var deps []Dependency
	for _, targets := range g.outEdges {
		for _, list := range targets {
			deps = append(deps, list...)
		}
	}
	sort.Slice(deps, func(i, j int) bool {
		if deps[i].SourceID != deps[j].SourceID {
			return deps[i].SourceID < deps[j].SourceID
		}
		if deps[i].TargetID != deps[j].TargetID {
			return deps[i].TargetID < deps[j].TargetID
		}
		return deps[i].Type < deps[j].Type
	})
	return deps
}

// GetDirectDependencies returns all nodes that the given node depends on (outgoing edges).
func (g *Graph) GetDirectDependencies(nodeID string) []TopologyNode {
	g.mu.RLock()
	defer g.mu.RUnlock()

	targets, ok := g.outEdges[nodeID]
	if !ok {
		return nil
	}

	var res []TopologyNode
	for targetID := range targets {
		if node, exists := g.nodes[targetID]; exists {
			res = append(res, node)
		}
	}
	sort.Slice(res, func(i, j int) bool {
		return res[i].ID < res[j].ID
	})
	return res
}

// GetDirectDependents returns all nodes that depend on the given node (incoming edges).
func (g *Graph) GetDirectDependents(nodeID string) []TopologyNode {
	g.mu.RLock()
	defer g.mu.RUnlock()

	sources, ok := g.inEdges[nodeID]
	if !ok {
		return nil
	}

	var res []TopologyNode
	for sourceID := range sources {
		if node, exists := g.nodes[sourceID]; exists {
			res = append(res, node)
		}
	}
	sort.Slice(res, func(i, j int) bool {
		return res[i].ID < res[j].ID
	})
	return res
}

// GetOutDependencies returns all outgoing dependency edges from the given node.
func (g *Graph) GetOutDependencies(nodeID string) []Dependency {
	g.mu.RLock()
	defer g.mu.RUnlock()

	targets, ok := g.outEdges[nodeID]
	if !ok {
		return nil
	}

	var res []Dependency
	for _, list := range targets {
		res = append(res, list...)
	}
	sort.Slice(res, func(i, j int) bool {
		if res[i].TargetID != res[j].TargetID {
			return res[i].TargetID < res[j].TargetID
		}
		return res[i].Type < res[j].Type
	})
	return res
}

// GetInDependencies returns all incoming dependency edges to the given node.
func (g *Graph) GetInDependencies(nodeID string) []Dependency {
	g.mu.RLock()
	defer g.mu.RUnlock()

	sources, ok := g.inEdges[nodeID]
	if !ok {
		return nil
	}

	var res []Dependency
	for _, list := range sources {
		res = append(res, list...)
	}
	sort.Slice(res, func(i, j int) bool {
		if res[i].SourceID != res[j].SourceID {
			return res[i].SourceID < res[j].SourceID
		}
		return res[i].Type < res[j].Type
	})
	return res
}

// GetTransitiveDependencies returns all upstream nodes reachable from nodeID (bounded by maxDepth).
// Cycle-safe via visited tracking.
func (g *Graph) GetTransitiveDependencies(nodeID string, maxDepth int) []TopologyNode {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if maxDepth <= 0 {
		maxDepth = 20
	}

	visited := make(map[string]bool)
	visited[nodeID] = true

	queue := []string{nodeID}
	depth := 0

	var result []TopologyNode

	for len(queue) > 0 && depth < maxDepth {
		levelSize := len(queue)
		for i := 0; i < levelSize; i++ {
			curr := queue[i]
			if targets, ok := g.outEdges[curr]; ok {
				for targetID := range targets {
					if !visited[targetID] {
						visited[targetID] = true
						if node, exists := g.nodes[targetID]; exists {
							result = append(result, node)
						}
						queue = append(queue, targetID)
					}
				}
			}
		}
		queue = queue[levelSize:]
		depth++
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result
}

// GetTransitiveDependents returns all downstream nodes that depend on nodeID (bounded by maxDepth).
// Cycle-safe via visited tracking.
func (g *Graph) GetTransitiveDependents(nodeID string, maxDepth int) []TopologyNode {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if maxDepth <= 0 {
		maxDepth = 20
	}

	visited := make(map[string]bool)
	visited[nodeID] = true

	queue := []string{nodeID}
	depth := 0

	var result []TopologyNode

	for len(queue) > 0 && depth < maxDepth {
		levelSize := len(queue)
		for i := 0; i < levelSize; i++ {
			curr := queue[i]
			if sources, ok := g.inEdges[curr]; ok {
				for sourceID := range sources {
					if !visited[sourceID] {
						visited[sourceID] = true
						if node, exists := g.nodes[sourceID]; exists {
							result = append(result, node)
						}
						queue = append(queue, sourceID)
					}
				}
			}
		}
		queue = queue[levelSize:]
		depth++
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result
}

// FindShortestPath calculates the shortest directed dependency path between source and target using BFS.
func (g *Graph) FindShortestPath(sourceID, targetID string) (*DependencyPath, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if sourceID == targetID {
		sourceNode, exists := g.nodes[sourceID]
		if !exists {
			return nil, false
		}
		return &DependencyPath{
			SourceID:    sourceID,
			TargetID:    targetID,
			Nodes:       []TopologyNode{sourceNode},
			Edges:       nil,
			TotalWeight: 0,
			Hops:        0,
		}, true
	}

	type step struct {
		nodeID string
		edge   *Dependency
		parent int
	}

	queue := []step{{nodeID: sourceID, parent: -1}}
	visited := make(map[string]bool)
	visited[sourceID] = true

	targetIdx := -1

	for head := 0; head < len(queue); head++ {
		curr := queue[head]
		if curr.nodeID == targetID {
			targetIdx = head
			break
		}

		if targets, ok := g.outEdges[curr.nodeID]; ok {
			// Sort target IDs for deterministic traversal
			targetIDs := make([]string, 0, len(targets))
			for tid := range targets {
				targetIDs = append(targetIDs, tid)
			}
			sort.Strings(targetIDs)

			for _, tid := range targetIDs {
				if !visited[tid] {
					visited[tid] = true
					deps := targets[tid]
					var chosenEdge *Dependency
					if len(deps) > 0 {
						chosenEdge = &deps[0]
					}
					queue = append(queue, step{
						nodeID: tid,
						edge:   chosenEdge,
						parent: head,
					})
				}
			}
		}
	}

	if targetIdx == -1 {
		return nil, false
	}

	// Reconstruct path
	var pathSteps []step
	currIdx := targetIdx
	for currIdx != -1 {
		pathSteps = append(pathSteps, queue[currIdx])
		currIdx = queue[currIdx].parent
	}

	// Reverse steps
	for i, j := 0, len(pathSteps)-1; i < j; i, j = i+1, j-1 {
		pathSteps[i], pathSteps[j] = pathSteps[j], pathSteps[i]
	}

	var nodes []TopologyNode
	var edges []Dependency
	var totalWeight float64

	for i, s := range pathSteps {
		if node, exists := g.nodes[s.nodeID]; exists {
			nodes = append(nodes, node)
		} else {
			nodes = append(nodes, TopologyNode{ID: s.nodeID, Name: s.nodeID, Type: NodeTypeCustom, Status: NodeStatusUnknown})
		}
		if i > 0 && s.edge != nil {
			edges = append(edges, *s.edge)
			totalWeight += s.edge.Weight
		}
	}

	return &DependencyPath{
		SourceID:    sourceID,
		TargetID:    targetID,
		Nodes:       nodes,
		Edges:       edges,
		TotalWeight: totalWeight,
		Hops:        len(edges),
	}, true
}

// FindAllPaths finds all directed paths from sourceID to targetID up to maxDepth.
func (g *Graph) FindAllPaths(sourceID, targetID string, maxDepth int) []DependencyPath {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if maxDepth <= 0 {
		maxDepth = 10
	}

	var results []DependencyPath
	visited := make(map[string]bool)

	var currentNodes []string
	var currentEdges []Dependency

	var dfs func(curr string, depth int)
	dfs = func(curr string, depth int) {
		currentNodes = append(currentNodes, curr)
		visited[curr] = true

		if curr == targetID {
			// Construct DependencyPath
			var nodes []TopologyNode
			var weight float64
			for _, nid := range currentNodes {
				if n, exists := g.nodes[nid]; exists {
					nodes = append(nodes, n)
				} else {
					nodes = append(nodes, TopologyNode{ID: nid, Name: nid, Type: NodeTypeCustom, Status: NodeStatusUnknown})
				}
			}
			edgesCopy := make([]Dependency, len(currentEdges))
			copy(edgesCopy, currentEdges)
			for _, e := range edgesCopy {
				weight += e.Weight
			}

			results = append(results, DependencyPath{
				SourceID:    sourceID,
				TargetID:    targetID,
				Nodes:       nodes,
				Edges:       edgesCopy,
				TotalWeight: weight,
				Hops:        len(edgesCopy),
			})
		} else if depth < maxDepth {
			if targets, ok := g.outEdges[curr]; ok {
				targetIDs := make([]string, 0, len(targets))
				for tid := range targets {
					targetIDs = append(targetIDs, tid)
				}
				sort.Strings(targetIDs)

				for _, tid := range targetIDs {
					if !visited[tid] {
						deps := targets[tid]
						for _, dep := range deps {
							currentEdges = append(currentEdges, dep)
							dfs(tid, depth+1)
							currentEdges = currentEdges[:len(currentEdges)-1]
						}
					}
				}
			}
		}

		visited[curr] = false
		currentNodes = currentNodes[:len(currentNodes)-1]
	}

	dfs(sourceID, 0)
	return results
}

// GetConnectedComponents returns the weakly connected components of the graph.
func (g *Graph) GetConnectedComponents() [][]string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	visited := make(map[string]bool)
	var components [][]string

	// Sort node IDs for deterministic iteration
	allNodeIDs := make([]string, 0, len(g.nodes))
	for id := range g.nodes {
		allNodeIDs = append(allNodeIDs, id)
	}
	sort.Strings(allNodeIDs)

	for _, startID := range allNodeIDs {
		if visited[startID] {
			continue
		}

		var comp []string
		queue := []string{startID}
		visited[startID] = true

		for len(queue) > 0 {
			curr := queue[0]
			queue = queue[1:]
			comp = append(comp, curr)

			// Undirected exploration: check outEdges and inEdges
			if targets, ok := g.outEdges[curr]; ok {
				for tid := range targets {
					if !visited[tid] {
						visited[tid] = true
						queue = append(queue, tid)
					}
				}
			}
			if sources, ok := g.inEdges[curr]; ok {
				for sid := range sources {
					if !visited[sid] {
						visited[sid] = true
						queue = append(queue, sid)
					}
				}
			}
		}

		sort.Strings(comp)
		components = append(components, comp)
	}

	sort.Slice(components, func(i, j int) bool {
		if len(components[i]) != len(components[j]) {
			return len(components[i]) > len(components[j])
		}
		if len(components[i]) > 0 && len(components[j]) > 0 {
			return components[i][0] < components[j][0]
		}
		return false
	})

	return components
}

// Summary calculates structural metrics and summary statistics for the graph.
func (g *Graph) Summary() TopologyGraphSummary {
	g.mu.RLock()
	defer g.mu.RUnlock()

	nodeTypeCounts := make(map[string]int)
	statusCounts := make(map[string]int)
	relTypeCounts := make(map[string]int)
	totalDeps := 0

	for _, n := range g.nodes {
		nodeTypeCounts[string(n.Type)]++
		statusCounts[string(n.Status)]++
	}

	for _, targets := range g.outEdges {
		for _, deps := range targets {
			totalDeps += len(deps)
			for _, d := range deps {
				relTypeCounts[string(d.Type)]++
			}
		}
	}

	totalNodes := len(g.nodes)
	var density float64
	if totalNodes > 1 {
		possibleEdges := float64(totalNodes * (totalNodes - 1))
		density = float64(totalDeps) / possibleEdges
		density = math.Round(density*10000) / 10000.0
	}

	// Calculate components safely
	g.mu.RUnlock()
	components := g.GetConnectedComponents()
	g.mu.RLock()

	return TopologyGraphSummary{
		TotalNodes:             totalNodes,
		TotalDependencies:      totalDeps,
		NodeTypeCounts:         nodeTypeCounts,
		RelationshipTypeCounts: relTypeCounts,
		StatusCounts:           statusCounts,
		ConnectedComponents:   len(components),
		Density:                density,
		EvaluatedAt:            time.Now().UTC(),
	}
}

// SubGraph creates a new Graph containing only the specified node IDs and the edges between them.
func (g *Graph) SubGraph(nodeIDs []string) *Graph {
	g.mu.RLock()
	defer g.mu.RUnlock()

	sub := NewGraph()
	included := make(map[string]bool)

	for _, id := range nodeIDs {
		if node, exists := g.nodes[id]; exists {
			sub.AddNode(node)
			included[id] = true
		}
	}

	for srcID := range included {
		if targets, ok := g.outEdges[srcID]; ok {
			for dstID, deps := range targets {
				if included[dstID] {
					for _, dep := range deps {
						sub.AddDependency(dep)
					}
				}
			}
		}
	}

	return sub
}

// Filter returns a filtered subgraph according to the provided TopologyFilter.
func (g *Graph) Filter(filter TopologyFilter) *Graph {
	g.mu.RLock()
	defer g.mu.RUnlock()

	typeFilterMap := make(map[NodeType]bool)
	for _, t := range filter.Types {
		typeFilterMap[t] = true
	}

	statusFilterMap := make(map[NodeStatus]bool)
	for _, s := range filter.Statuses {
		statusFilterMap[s] = true
	}

	var matchingIDs []string
	searchLower := strings.ToLower(filter.Search)

	for _, n := range g.nodes {
		if len(typeFilterMap) > 0 && !typeFilterMap[n.Type] {
			continue
		}
		if len(statusFilterMap) > 0 && !statusFilterMap[n.Status] {
			continue
		}
		if filter.Source != "" && n.Source != filter.Source {
			continue
		}
		if filter.HostID != "" && n.HostID != filter.HostID {
			continue
		}
		if len(filter.TagFilter) > 0 {
			tagMatch := true
			for k, v := range filter.TagFilter {
				if n.Tags[k] != v {
					tagMatch = false
					break
				}
			}
			if !tagMatch {
				continue
			}
		}
		if searchLower != "" {
			if !strings.Contains(strings.ToLower(n.ID), searchLower) &&
				!strings.Contains(strings.ToLower(n.Name), searchLower) {
				continue
			}
		}
		matchingIDs = append(matchingIDs, n.ID)
	}

	g.mu.RUnlock()
	sub := g.SubGraph(matchingIDs)
	g.mu.RLock()
	return sub
}

// Clone creates a deep copy of the Graph.
func (g *Graph) Clone() *Graph {
	g.mu.RLock()
	defer g.mu.RUnlock()

	clone := NewGraph()
	for _, n := range g.nodes {
		clone.AddNode(n)
	}
	for _, targets := range g.outEdges {
		for _, deps := range targets {
			for _, dep := range deps {
				clone.AddDependency(dep)
			}
		}
	}
	return clone
}

// ExportDOT renders the topology graph into Graphviz DOT syntax.
func (g *Graph) ExportDOT() string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var sb strings.Builder
	sb.WriteString("digraph WatchdogTopology {\n")
	sb.WriteString("  rankdir=LR;\n")
	sb.WriteString("  node [shape=box, style=\"rounded,filled\", fontname=\"Helvetica\"];\n")
	sb.WriteString("  edge [fontname=\"Helvetica\", fontsize=10];\n\n")

	nodes := g.GetAllNodes()
	for _, n := range nodes {
		fillColor := "#e0f2fe" // light blue for healthy
		switch n.Status {
		case NodeStatusCritical:
			fillColor = "#fee2e2" // red
		case NodeStatusDegraded:
			fillColor = "#fef3c7" // yellow
		case NodeStatusOffline:
			fillColor = "#f3f4f6" // gray
		}
		sb.WriteString(fmt.Sprintf("  \"%s\" [label=\"%s\\n(%s)\", fillcolor=\"%s\"];\n",
			n.ID, n.Name, n.Type, fillColor))
	}

	sb.WriteString("\n")
	deps := g.GetAllDependencies()
	for _, d := range deps {
		style := "solid"
		if d.Confidence == ConfidenceLow {
			style = "dashed"
		}
		sb.WriteString(fmt.Sprintf("  \"%s\" -> \"%s\" [label=\"%s\", style=\"%s\"];\n",
			d.SourceID, d.TargetID, d.Type, style))
	}

	sb.WriteString("}\n")
	return sb.String()
}
