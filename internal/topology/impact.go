package topology

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// ImpactAnalyzer calculates the blast radius and downstream consequences of node failures.
type ImpactAnalyzer struct{}

// NewImpactAnalyzer creates a new ImpactAnalyzer.
func NewImpactAnalyzer() *ImpactAnalyzer {
	return &ImpactAnalyzer{}
}

// AnalyzeImpact calculates the blast radius and downstream consequences if targetNodeID fails.
func (ia *ImpactAnalyzer) AnalyzeImpact(g *Graph, targetNodeID string) *TopologyImpactAnalysis {
	target, exists := g.GetNode(targetNodeID)
	if !exists {
		return nil
	}

	directDependents := g.GetDirectDependents(targetNodeID)
	transitiveDependents := g.GetTransitiveDependents(targetNodeID, 20)

	// Calculate max impact depth using BFS from targetNodeID
	maxDepth := 0
	if len(directDependents) > 0 {
		visited := make(map[string]int)
		visited[targetNodeID] = 0
		queue := []string{targetNodeID}

		for len(queue) > 0 {
			curr := queue[0]
			queue = queue[1:]
			currDepth := visited[curr]
			if currDepth > maxDepth {
				maxDepth = currDepth
			}

			// Explore upstream dependents
			deps := g.GetDirectDependents(curr)
			for _, d := range deps {
				if _, seen := visited[d.ID]; !seen {
					visited[d.ID] = currDepth + 1
					queue = append(queue, d.ID)
				}
			}
		}
	}

	affectedNodeTypes := make(map[string]int)
	affectedStatuses := make(map[string]int)
	criticalCount := 0
	warningCount := 0

	for _, dep := range transitiveDependents {
		affectedNodeTypes[string(dep.Type)]++
		affectedStatuses[string(dep.Status)]++

		if dep.Status == NodeStatusCritical || dep.Type == NodeTypeDatabase || dep.Type == NodeTypePhysicalHost {
			criticalCount++
		} else if dep.Status == NodeStatusDegraded {
			warningCount++
		}
	}

	// Calculate blast radius score (0-100)
	totalNodes := g.NodeCount()
	fractionAffected := 0.0
	if totalNodes > 1 {
		fractionAffected = float64(len(transitiveDependents)) / float64(totalNodes-1)
	}

	// Blast radius score computation:
	// Base: fraction of total fleet affected (up to 40 pts)
	// Volume: raw count of dependents (up to 30 pts)
	// Depth: propagation depth (up to 15 pts)
	// Criticality: critical infrastructure affected (up to 15 pts)
	score := (fractionAffected * 40.0) +
		math.Min(30.0, float64(len(transitiveDependents))*3.0) +
		math.Min(15.0, float64(maxDepth)*5.0) +
		math.Min(15.0, float64(criticalCount)*5.0)

	if score > 100.0 {
		score = 100.0
	}
	score = math.Round(score*10) / 10.0

	// Classify blast radius level
	var level BlastRadiusLevel
	switch {
	case score >= 75.0:
		level = BlastRadiusCritical
	case score >= 50.0:
		level = BlastRadiusHigh
	case score >= 25.0:
		level = BlastRadiusMedium
	case score > 5.0:
		level = BlastRadiusLow
	default:
		level = BlastRadiusMinimal
	}

	summary := fmt.Sprintf("Failure of '%s' (%s) impacts %d direct dependent(s) and %d total transitive entity(ies) across %d hop(s) with %s blast radius.",
		target.Name, target.Type, len(directDependents), len(transitiveDependents), maxDepth, level)

	// Sort dependents deterministically
	sort.Slice(directDependents, func(i, j int) bool {
		return directDependents[i].ID < directDependents[j].ID
	})
	sort.Slice(transitiveDependents, func(i, j int) bool {
		return transitiveDependents[i].ID < transitiveDependents[j].ID
	})

	return &TopologyImpactAnalysis{
		TargetNodeID:          target.ID,
		TargetNodeName:        target.Name,
		TargetNodeType:        target.Type,
		TargetNodeStatus:      target.Status,
		DirectDependents:      directDependents,
		TransitiveDependents:  transitiveDependents,
		MaxImpactDepth:        maxDepth,
		BlastRadiusScore:      score,
		BlastRadiusLevel:      level,
		AffectedNodeTypes:     affectedNodeTypes,
		AffectedStatuses:      affectedStatuses,
		CriticalNodesAffected: criticalCount,
		WarningNodesAffected:  warningCount,
		Summary:               summary,
		AnalyzedAt:            time.Now().UTC(),
	}
}
