package incidents

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/DocHoax/watchdog/internal/intelligence"
	"github.com/DocHoax/watchdog/pkg/model"
)

// IncidentClusterer groups correlated signals into unified operational incidents.
type IncidentClusterer struct {
	correlator *SignalCorrelator
	window     time.Duration
}

// NewIncidentClusterer instantiates an incident clustering engine.
func NewIncidentClusterer(window time.Duration) *IncidentClusterer {
	if window <= 0 {
		window = 15 * time.Minute
	}
	return &IncidentClusterer{
		correlator: NewSignalCorrelator(CorrelationConfig{Window: window}),
		window:     window,
	}
}

// NodeSignalsBundle bundles all observability inputs for a node.
type NodeSignalsBundle struct {
	NodeID      string
	Hostname    string
	Tags        map[string]string
	Alerts      []model.AlertEvent
	Diagnostics *model.DiagnosticReport
	Anomalies   *model.AnomalyReport
	Predictions []intelligence.Prediction
}

// ClusterFleetSignals aggregates multi-node signals into node, multi-node, and fleet incidents.
func (ic *IncidentClusterer) ClusterFleetSignals(nodes []NodeSignalsBundle, totalFleetNodes int) []Incident {
	if len(nodes) == 0 {
		return nil
	}

	nodeHostnames := make(map[string]string)
	nodeTagsMap := make(map[string]map[string]string)

	var allSignals []IncidentSignal
	for _, n := range nodes {
		nodeHostnames[n.NodeID] = n.Hostname
		nodeTagsMap[n.NodeID] = n.Tags
		sigs := ic.correlator.CorrelateSignals(n.NodeID, n.Hostname, n.Alerts, n.Diagnostics, n.Anomalies, n.Predictions)
		allSignals = append(allSignals, sigs...)
	}

	if len(allSignals) == 0 {
		return nil
	}

	// 1. Group signals by canonical symptom/source across nodes within time window
	type clusterKey struct {
		sourceType SignalType
		sourceName string
	}

	groupedSignals := make(map[clusterKey][]IncidentSignal)
	for _, s := range allSignals {
		key := clusterKey{sourceType: s.Type, sourceName: s.Source}
		groupedSignals[key] = append(groupedSignals[key], s)
	}

	// Also group remaining disparate signals per node
	nodeGroupedSignals := make(map[string][]IncidentSignal)
	var multiNodeClusters [][]IncidentSignal

	for _, sigs := range groupedSignals {
		// If signals span multiple nodes, create a multi-node/fleet cluster
		uniqueNodes := make(map[string]bool)
		for _, s := range sigs {
			uniqueNodes[s.NodeID] = true
		}

		if len(uniqueNodes) > 1 {
			multiNodeClusters = append(multiNodeClusters, sigs)
		} else {
			for _, s := range sigs {
				nodeGroupedSignals[s.NodeID] = append(nodeGroupedSignals[s.NodeID], s)
			}
		}
	}

	var incidents []Incident
	now := time.Now().UTC()

	// 2. Build multi-node / fleet incidents
	for _, clusterSigs := range multiNodeClusters {
		if len(clusterSigs) == 0 {
			continue
		}

		var earliestTime time.Time
		var affectedNodes []string
		var symptoms []string
		nodeSet := make(map[string]bool)

		for _, s := range clusterSigs {
			if earliestTime.IsZero() || s.Timestamp.Before(earliestTime) {
				earliestTime = s.Timestamp
			}
			if !nodeSet[s.NodeID] && s.NodeID != "" {
				nodeSet[s.NodeID] = true
				affectedNodes = append(affectedNodes, s.NodeID)
			}
			symptoms = append(symptoms, fmt.Sprintf("%s on %s: %s", s.Source, s.NodeID, s.Description))
		}
		sort.Strings(affectedNodes)

		scope := IncidentScopeMultiNode
		if totalFleetNodes > 0 && float64(len(affectedNodes))/float64(totalFleetNodes) >= 0.30 {
			scope = IncidentScopeFleet
		}

		incID := GenerateIncidentID(scope, strings.Join(affectedNodes, ","), clusterSigs[0].Source, earliestTime)
		sev, score, explanation := CalculateSeverity(clusterSigs, len(affectedNodes), totalFleetNodes, 0)
		impact := CalculateImpact(incID, affectedNodes, nodeHostnames, nodeTagsMap, clusterSigs, totalFleetNodes)

		tb := NewTimelineBuilder(incID)
		for _, s := range clusterSigs {
			tb.AddSignal(s)
		}
		timeline := tb.Build()

		title := fmt.Sprintf("Cluster degradation: %s across %d nodes", clusterSigs[0].Source, len(affectedNodes))
		summary := fmt.Sprintf("Multi-node incident involving %s detected across nodes: %s", clusterSigs[0].Source, strings.Join(affectedNodes, ", "))

		inc := Incident{
			ID:                  incID,
			Title:               title,
			Summary:             summary,
			Status:              IncidentStatusDetected,
			Severity:            sev,
			Scope:               scope,
			Confidence:          explanation.Confidence,
			StartTime:           earliestTime,
			AffectedNodes:       affectedNodes,
			PrimarySymptoms:     symptoms,
			RootSignals:         clusterSigs,
			SeverityScore:       score,
			SeverityExplanation: explanation,
			Impact:              impact.Impact,
			Timeline:            timeline,
			CreatedAt:           now,
			UpdatedAt:           now,
		}
		inc.Findings = GenerateIncidentFindings(&inc)
		incidents = append(incidents, inc)
	}

	// 3. Build single-node incidents for unclustered signals
	for nodeID, sigs := range nodeGroupedSignals {
		if len(sigs) == 0 {
			continue
		}

		var earliestTime time.Time
		var symptoms []string
		for _, s := range sigs {
			if earliestTime.IsZero() || s.Timestamp.Before(earliestTime) {
				earliestTime = s.Timestamp
			}
			symptoms = append(symptoms, fmt.Sprintf("%s: %s", s.Source, s.Description))
		}

		hostname := nodeHostnames[nodeID]
		if hostname == "" {
			hostname = nodeID
		}

		incID := GenerateIncidentID(IncidentScopeNode, nodeID, sigs[0].Source, earliestTime)
		sev, score, explanation := CalculateSeverity(sigs, 1, totalFleetNodes, 0)
		impact := CalculateImpact(incID, []string{nodeID}, nodeHostnames, nodeTagsMap, sigs, totalFleetNodes)

		tb := NewTimelineBuilder(incID)
		for _, s := range sigs {
			tb.AddSignal(s)
		}
		timeline := tb.Build()

		title := fmt.Sprintf("Degradation cluster on %s: %s (%d signals)", hostname, sigs[0].Source, len(sigs))
		summary := fmt.Sprintf("Node %s is experiencing %d co-occurring degradation signals (severity: %s)", hostname, len(sigs), sev)

		inc := Incident{
			ID:                  incID,
			Title:               title,
			Summary:             summary,
			Status:              IncidentStatusDetected,
			Severity:            sev,
			Scope:               IncidentScopeNode,
			Confidence:          explanation.Confidence,
			StartTime:           earliestTime,
			AffectedNodes:       []string{nodeID},
			PrimarySymptoms:     symptoms,
			RootSignals:         sigs,
			SeverityScore:       score,
			SeverityExplanation: explanation,
			Impact:              impact.Impact,
			Timeline:            timeline,
			CreatedAt:           now,
			UpdatedAt:           now,
		}
		inc.Findings = GenerateIncidentFindings(&inc)
		incidents = append(incidents, inc)
	}

	// Sort incidents by severity descending, then start time descending
	sort.Slice(incidents, func(i, j int) bool {
		if incidents[i].SeverityScore != incidents[j].SeverityScore {
			return incidents[i].SeverityScore > incidents[j].SeverityScore
		}
		return incidents[i].StartTime.After(incidents[j].StartTime)
	})

	return incidents
}
