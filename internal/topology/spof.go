package topology

import (
	"fmt"
	"math"
	"sort"

	"github.com/DocHoax/watchdog/pkg/model"
)

// SPOFAnalyzer evaluates Single Points of Failure across a topology graph.
type SPOFAnalyzer struct{}

// NewSPOFAnalyzer creates a new SPOFAnalyzer.
func NewSPOFAnalyzer() *SPOFAnalyzer {
	return &SPOFAnalyzer{}
}

// AnalyzeNode evaluates a single node for Single Point of Failure characteristics.
func (a *SPOFAnalyzer) AnalyzeNode(g *Graph, nodeID string) *SPOFAnalysis {
	node, exists := g.GetNode(nodeID)
	if !exists {
		return nil
	}

	directDependents := g.GetDirectDependents(nodeID)
	transitiveDependents := g.GetTransitiveDependents(nodeID, 20)

	var reasoning []string
	var affectedServices []string
	serviceSet := make(map[string]bool)

	for _, dep := range transitiveDependents {
		if dep.Type == NodeTypeService || dep.Type == NodeTypeDatabase || dep.Type == NodeTypeQueue {
			if !serviceSet[dep.ID] {
				serviceSet[dep.ID] = true
				affectedServices = append(affectedServices, dep.Name)
			}
		}
	}
	sort.Strings(affectedServices)

	// Check for alternative paths and redundancy
	alternativePaths := 0
	redundancy := RedundancyNone

	// Check if there are other nodes of the same type providing similar services
	allNodes := g.GetAllNodes()
	sameTypePeers := 0
	for _, peer := range allNodes {
		if peer.ID != node.ID && peer.Type == node.Type {
			sameTypePeers++
		}
	}

	if len(directDependents) == 0 {
		redundancy = RedundancyRedundant
		reasoning = append(reasoning, "No dependent services rely on this node.")
	} else {
		// Check for alternative connectivity if this node were removed
		totalDepCount := len(transitiveDependents)
		if sameTypePeers > 0 {
			// Check if direct dependents also connect to any of the same-type peers
			peerOverlapCount := 0
			for _, dep := range directDependents {
				outFromDep := g.GetDirectDependencies(dep.ID)
				for _, out := range outFromDep {
					if out.ID != node.ID && out.Type == node.Type {
						peerOverlapCount++
						alternativePaths++
						break
					}
				}
			}

			if peerOverlapCount == len(directDependents) {
				redundancy = RedundancyRedundant
				reasoning = append(reasoning, fmt.Sprintf("All %d dependent(s) have redundant paths to alternate %s instances.", len(directDependents), node.Type))
			} else if peerOverlapCount > 0 {
				redundancy = RedundancyPartial
				reasoning = append(reasoning, fmt.Sprintf("%d of %d dependent(s) have alternative paths to peer %s instances.", peerOverlapCount, len(directDependents), node.Type))
			} else {
				redundancy = RedundancyNone
				reasoning = append(reasoning, fmt.Sprintf("Zero of %d dependent(s) have fallback paths to peer %s instances.", len(directDependents), node.Type))
			}
		} else {
			redundancy = RedundancyNone
			reasoning = append(reasoning, fmt.Sprintf("Unique %s in topology with no backup/failover peer instances.", node.Type))
		}

		if totalDepCount > 0 {
			reasoning = append(reasoning, fmt.Sprintf("Failure would transitively impact %d downstream node(s).", totalDepCount))
		}
	}

	// Calculate criticality score (0 - 100)
	score := 0.0

	// 1. Transitive dependents weight (up to 40 pts)
	score += math.Min(40.0, float64(len(transitiveDependents))*8.0)

	// 2. Direct dependents weight (up to 20 pts)
	score += math.Min(20.0, float64(len(directDependents))*5.0)

	// 3. Node Type structural importance (up to 20 pts)
	switch node.Type {
	case NodeTypeDatabase:
		score += 20.0
	case NodeTypePhysicalHost, NodeTypeK8sNode:
		score += 18.0
	case NodeTypeStorage, NodeTypeQueue:
		score += 15.0
	case NodeTypeService:
		score += 12.0
	case NodeTypeVM, NodeTypeContainer:
		score += 8.0
	default:
		score += 5.0
	}

	// 4. Redundancy modifier
	switch redundancy {
	case RedundancyNone:
		if len(directDependents) > 0 {
			score += 15.0
		}
	case RedundancyPartial:
		score += 5.0
	case RedundancyRedundant:
		score -= 15.0
	}

	// 5. Current health status modifier
	switch node.Status {
	case NodeStatusCritical:
		score += 15.0
		reasoning = append(reasoning, "Node is currently in CRITICAL operational state.")
	case NodeStatusDegraded:
		score += 10.0
		reasoning = append(reasoning, "Node is currently in DEGRADED operational state.")
	}

	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	score = math.Round(score*10) / 10.0

	// Assign risk level based on criticality score
	var riskLevel model.Severity
	switch {
	case score >= 70.0:
		riskLevel = model.SeverityCritical
	case score >= 35.0:
		riskLevel = model.SeverityWarning
	default:
		riskLevel = model.SeverityInfo
	}

	return &SPOFAnalysis{
		NodeID:                    node.ID,
		NodeName:                  node.Name,
		NodeType:                  node.Type,
		Status:                    node.Status,
		CriticalityScore:          score,
		DependentsCount:           len(directDependents),
		TransitiveDependentsCount: len(transitiveDependents),
		AffectedServices:          affectedServices,
		RedundancyLevel:           redundancy,
		AlternativePaths:          alternativePaths,
		RiskLevel:                 riskLevel,
		Reasoning:                 reasoning,
	}
}

// FindAllSPOFs returns all nodes evaluated for SPOF, sorted by CriticalityScore descending.
func (a *SPOFAnalyzer) FindAllSPOFs(g *Graph, minCriticality float64) []SPOFAnalysis {
	nodes := g.GetAllNodes()
	var results []SPOFAnalysis

	for _, n := range nodes {
		analysis := a.AnalyzeNode(g, n.ID)
		if analysis != nil && analysis.CriticalityScore >= minCriticality {
			results = append(results, *analysis)
		}
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].CriticalityScore != results[j].CriticalityScore {
			return results[i].CriticalityScore > results[j].CriticalityScore
		}
		return results[i].NodeID < results[j].NodeID
	})

	return results
}
