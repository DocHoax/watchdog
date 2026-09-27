package incidents

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/DocHoax/watchdog/pkg/model"
)

// CalculateImpact builds an explainable blast-radius analysis across infrastructure nodes and subsystems.
func CalculateImpact(
	incidentID string,
	affectedNodeIDs []string,
	nodeHostnames map[string]string,
	nodeTags map[string]map[string]string,
	signals []IncidentSignal,
	totalFleetNodes int,
) ImpactAnalysis {
	now := time.Now().UTC()

	// Deduplicate affected nodes
	nodeMap := make(map[string]bool)
	for _, n := range affectedNodeIDs {
		if strings.TrimSpace(n) != "" {
			nodeMap[n] = true
		}
	}

	for _, s := range signals {
		if strings.TrimSpace(s.NodeID) != "" {
			nodeMap[s.NodeID] = true
		}
	}

	var dedupedNodes []string
	var hostnames []string
	for n := range nodeMap {
		dedupedNodes = append(dedupedNodes, n)
		if h, ok := nodeHostnames[n]; ok && h != "" {
			hostnames = append(hostnames, h)
		} else {
			hostnames = append(hostnames, n)
		}
	}
	sort.Strings(dedupedNodes)
	sort.Strings(hostnames)

	if totalFleetNodes <= 0 {
		totalFleetNodes = len(dedupedNodes)
		if totalFleetNodes == 0 {
			totalFleetNodes = 1
		}
	}

	var fleetPct float64
	if totalFleetNodes > 0 {
		fleetPct = (float64(len(dedupedNodes)) / float64(totalFleetNodes)) * 100.0
		if fleetPct > 100.0 {
			fleetPct = 100.0
		}
	}

	// Identify affected subsystems and resources
	subsystemsMap := make(map[string]bool)
	resourcesMap := make(map[string]bool)
	sevDist := make(map[string]int)
	var criticalCount, warningCount int

	for _, s := range signals {
		sevKey := strings.ToLower(string(s.Severity))
		sevDist[sevKey]++
		if s.Severity == model.SeverityCritical {
			criticalCount++
		} else if s.Severity == model.SeverityWarning {
			warningCount++
		}

		subsystem := detectSubsystem(s)
		if subsystem != "" {
			subsystemsMap[subsystem] = true
		}

		if s.Source != "" {
			resourcesMap[s.Source] = true
		}
	}

	var subsystems []string
	for k := range subsystemsMap {
		subsystems = append(subsystems, k)
	}
	sort.Strings(subsystems)

	var resources []string
	for k := range resourcesMap {
		resources = append(resources, k)
	}
	sort.Strings(resources)

	// Determine common tags and tags distribution
	commonTags, tagDist := analyzeTags(dedupedNodes, nodeTags)

	// Determine scope and blast radius
	var scope IncidentScope
	var blastRadius string

	if len(dedupedNodes) == 0 || (len(dedupedNodes) == 1 && totalFleetNodes > 1) {
		scope = IncidentScopeNode
		blastRadius = "isolated_node"
	} else if fleetPct >= 30.0 || (totalFleetNodes <= 3 && len(dedupedNodes) == totalFleetNodes) {
		scope = IncidentScopeFleet
		blastRadius = "fleet_wide"
	} else {
		scope = IncidentScopeMultiNode
		if len(commonTags) > 0 {
			blastRadius = "tag_cluster"
		} else {
			blastRadius = "multi_node_cluster"
		}
	}

	summary := fmt.Sprintf(
		"Blast radius: %s affecting %d/%d nodes (%.1f%% of fleet) across %d subsystem(s): [%s]",
		blastRadius, len(dedupedNodes), totalFleetNodes, fleetPct, len(subsystems), strings.Join(subsystems, ", "),
	)

	impactScope := ImpactScope{
		Subsystems:           subsystems,
		Resources:            resources,
		AffectedNodeIDs:      dedupedNodes,
		AffectedHostnames:    hostnames,
		TotalFleetNodes:      totalFleetNodes,
		FleetPercentage:      fleetPct,
		SeverityDistribution: sevDist,
		CommonTags:           commonTags,
		TagsDistribution:     tagDist,
	}

	return ImpactAnalysis{
		IncidentID:           incidentID,
		Scope:                scope,
		Impact:               impactScope,
		EstimatedBlastRadius: blastRadius,
		CriticalNodesCount:   criticalCount,
		WarningNodesCount:    warningCount,
		AnalyzedAt:           now,
		Summary:              summary,
	}
}

func detectSubsystem(sig IncidentSignal) string {
	lowerDesc := strings.ToLower(sig.Description + " " + sig.Source)
	if strings.Contains(lowerDesc, "cpu") || strings.Contains(lowerDesc, "load") {
		return "cpu"
	}
	if strings.Contains(lowerDesc, "mem") || strings.Contains(lowerDesc, "oom") {
		return "memory"
	}
	if strings.Contains(lowerDesc, "swap") {
		return "swap"
	}
	if strings.Contains(lowerDesc, "disk") || strings.Contains(lowerDesc, "fs") || strings.Contains(lowerDesc, "filesystem") || strings.Contains(lowerDesc, "io") {
		return "disk"
	}
	if strings.Contains(lowerDesc, "net") || strings.Contains(lowerDesc, "rx") || strings.Contains(lowerDesc, "tx") || strings.Contains(lowerDesc, "packet") || strings.Contains(lowerDesc, "conn") {
		return "network"
	}
	if strings.Contains(lowerDesc, "proc") || strings.Contains(lowerDesc, "fd") || strings.Contains(lowerDesc, "zombie") {
		return "process"
	}
	if strings.Contains(lowerDesc, "security") || strings.Contains(lowerDesc, "cert") || strings.Contains(lowerDesc, "auth") {
		return "security"
	}
	return "system"
}

func analyzeTags(nodeIDs []string, nodeTags map[string]map[string]string) (map[string]string, map[string][]string) {
	if len(nodeIDs) == 0 || nodeTags == nil {
		return nil, nil
	}

	tagDist := make(map[string][]string)
	tagCount := make(map[string]map[string]int)

	for _, n := range nodeIDs {
		tags := nodeTags[n]
		for k, v := range tags {
			tagDist[k] = append(tagDist[k], fmt.Sprintf("%s=%s", n, v))
			if tagCount[k] == nil {
				tagCount[k] = make(map[string]int)
			}
			tagCount[k][v]++
		}
	}

	commonTags := make(map[string]string)
	for k, values := range tagCount {
		for v, count := range values {
			if count == len(nodeIDs) {
				commonTags[k] = v
			}
		}
	}

	return commonTags, tagDist
}
