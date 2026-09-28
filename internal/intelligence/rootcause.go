package intelligence

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/DocHoax/watchdog/internal/topology"
	"github.com/DocHoax/watchdog/pkg/model"
)

// RootCauseConfidence rates the analytical confidence in a root cause candidate.
type RootCauseConfidence string

const (
	RootCauseConfidenceHigh   RootCauseConfidence = "high"
	RootCauseConfidenceMedium RootCauseConfidence = "medium"
	RootCauseConfidenceLow    RootCauseConfidence = "low"
)

// RootCauseFactorCategory identifies the analytical dimension of an RCA factor.
type RootCauseFactorCategory string

const (
	FactorTemporalPrecedence RootCauseFactorCategory = "temporal_precedence"
	FactorTopologyCentrality RootCauseFactorCategory = "topology_centrality"
	FactorFaultSeverity      RootCauseFactorCategory = "fault_severity"
	FactorBlastRadius        RootCauseFactorCategory = "blast_radius_explanation"
	FactorHistoricalContext  RootCauseFactorCategory = "historical_context"
)

// RootCauseFactor represents an explainable component in the multi-factor scoring formula.
type RootCauseFactor struct {
	Category     RootCauseFactorCategory `json:"category"`
	Name         string                  `json:"name"`
	Score        float64                 `json:"score"`
	Weight       float64                 `json:"weight"`
	Contribution float64                 `json:"contribution"`
	Explanation  string                  `json:"explanation"`
}

// RootCauseCandidate represents an infrastructure entity or service evaluated as a potential root cause.
type RootCauseCandidate struct {
	NodeID                      string              `json:"node_id"`
	NodeName                    string              `json:"node_name"`
	NodeType                    topology.NodeType   `json:"node_type"`
	Status                      topology.NodeStatus `json:"status"`
	TotalScore                  float64             `json:"total_score"`
	Confidence                  RootCauseConfidence `json:"confidence"`
	Rank                        int                 `json:"rank"`
	Factors                     []RootCauseFactor   `json:"factors"`
	EarliestSignalTime          *time.Time          `json:"earliest_signal_time,omitempty"`
	EarliestSignalType          string              `json:"earliest_signal_type,omitempty"`
	DirectDownstreamCount       int                 `json:"direct_downstream_count"`
	TransitiveDownstreamCount   int                 `json:"transitive_downstream_count"`
	ExplainedAffectedPercentage float64             `json:"explained_affected_percentage"`
	IsSPOF                      bool                `json:"is_spof"`
	SPOFCriticality             float64             `json:"spof_criticality"`
	Reasoning                   []string            `json:"reasoning"`
}

// PropagationHop represents a single step in a fault propagation chain.
type PropagationHop struct {
	SourceID     string `json:"source_id"`
	SourceName   string `json:"source_name"`
	TargetID     string `json:"target_id"`
	TargetName   string `json:"target_name"`
	Relationship string `json:"relationship"`
}

// PropagationChain traces the causal or structural path from a root candidate to affected downstream entities.
type PropagationChain struct {
	RootNodeID      string           `json:"root_node_id"`
	RootNodeName    string           `json:"root_node_name"`
	LeafNodeID      string           `json:"leaf_node_id"`
	LeafNodeName    string           `json:"leaf_node_name"`
	Hops            []PropagationHop `json:"hops"`
	PathDescription string           `json:"path_description"`
}

// RootCauseReport provides a comprehensive, explainable root-cause determination dossier.
type RootCauseReport struct {
	IncidentID                 string               `json:"incident_id"`
	IncidentTitle              string               `json:"incident_title"`
	IncidentSeverity           model.Severity       `json:"incident_severity"`
	IncidentStartTime          time.Time            `json:"incident_start_time"`
	PrimaryRootCause           *RootCauseCandidate  `json:"primary_root_cause"`
	AlternativeCandidates      []RootCauseCandidate `json:"alternative_candidates"`
	PropagationChains          []PropagationChain   `json:"propagation_chains"`
	BlastRadiusExplanation     string               `json:"blast_radius_explanation"`
	NonInvasiveRecommendations []string             `json:"non_invasive_recommendations"`
	GeneratedAt                time.Time            `json:"generated_at"`
	Methodology                string               `json:"methodology"`
}

// RootCauseWeights configures the weighting coefficients in the multi-factor scoring formula.
type RootCauseWeights struct {
	TemporalWeight    float64 `json:"temporal_weight"`     // w1: default 0.25
	TopologyWeight    float64 `json:"topology_weight"`     // w2: default 0.30
	SeverityWeight    float64 `json:"severity_weight"`     // w3: default 0.20
	BlastRadiusWeight float64 `json:"blast_radius_weight"` // w4: default 0.15
	HistoricalWeight  float64 `json:"historical_weight"`   // w5: default 0.10
}

// DefaultRootCauseWeights returns the standard calibrated weighting coefficients.
func DefaultRootCauseWeights() RootCauseWeights {
	return RootCauseWeights{
		TemporalWeight:    0.25,
		TopologyWeight:    0.30,
		SeverityWeight:    0.20,
		BlastRadiusWeight: 0.15,
		HistoricalWeight:  0.10,
	}
}

// RootCauseEngine executes multi-factor root-cause determinations using graph topology and telemetry signals.
type RootCauseEngine struct {
	weights      RootCauseWeights
	spofAnalyzer *topology.SPOFAnalyzer
}

// NewRootCauseEngine instantiates a new RootCauseEngine.
func NewRootCauseEngine(weights ...RootCauseWeights) *RootCauseEngine {
	w := DefaultRootCauseWeights()
	if len(weights) > 0 {
		w = weights[0]
		// Normalize weights if sum is non-zero
		sum := w.TemporalWeight + w.TopologyWeight + w.SeverityWeight + w.BlastRadiusWeight + w.HistoricalWeight
		if sum > 0 && math.Abs(sum-1.0) > 0.001 {
			w.TemporalWeight /= sum
			w.TopologyWeight /= sum
			w.SeverityWeight /= sum
			w.BlastRadiusWeight /= sum
			w.HistoricalWeight /= sum
		}
	}
	return &RootCauseEngine{
		weights:      w,
		spofAnalyzer: topology.NewSPOFAnalyzer(),
	}
}

// candidateSignalInfo holds preprocessed telemetry information for a candidate node.
type candidateSignalInfo struct {
	earliestTime   *time.Time
	earliestType   string
	maxSeverity    model.Severity
	hasAlert       bool
	hasDiagnostic  bool
	hasAnomaly     bool
	hasPrediction  bool
	isFlapping     bool
	diagnosticMsgs []string
	alertTitles    []string
}

// AnalyzeIncident evaluates root-cause candidates for an intelligence incident cluster.
func (e *RootCauseEngine) AnalyzeIncident(
	incident *Incident,
	g *topology.Graph,
	predictions []Prediction,
	recurrence []RecurrencePattern,
) *RootCauseReport {
	if incident == nil {
		return nil
	}

	return e.AnalyzeIncidentWithSignals(
		incident.ID,
		incident.Title,
		incident.Severity,
		incident.StartTime,
		incident.AffectedNodes,
		incident.RelatedAlerts,
		incident.RelatedAnomalies,
		incident.Timeline,
		g,
		predictions,
		recurrence,
	)
}

// AnalyzeIncidentWithSignals performs deterministic root-cause analysis across all identified candidate entities.
func (e *RootCauseEngine) AnalyzeIncidentWithSignals(
	incidentID string,
	title string,
	severity model.Severity,
	startTime time.Time,
	affectedNodes []string,
	alerts []model.AlertEvent,
	anomalies []string,
	timeline []IncidentTimelineEvent,
	g *topology.Graph,
	predictions []Prediction,
	recurrence []RecurrencePattern,
) *RootCauseReport {
	now := time.Now().UTC()

	// 1. Harvest Candidate Node IDs:
	// Includes all directly affected nodes, upstream dependencies in topology, and nodes from signals.
	candidateSet := make(map[string]bool)
	for _, n := range affectedNodes {
		if strings.TrimSpace(n) != "" {
			candidateSet[strings.TrimSpace(n)] = true
		}
	}

	for _, tl := range timeline {
		if strings.TrimSpace(tl.NodeID) != "" {
			candidateSet[strings.TrimSpace(tl.NodeID)] = true
		}
	}

	// Expand upstream dependencies via topology graph (e.g. if A is affected and depends on DB, DB is a candidate)
	if g != nil {
		for _, affID := range affectedNodes {
			upstream := g.GetTransitiveDependencies(affID, 10)
			for _, u := range upstream {
				candidateSet[u.ID] = true
			}
		}
	}

	if len(candidateSet) == 0 && len(affectedNodes) == 0 {
		return &RootCauseReport{
			IncidentID:                 incidentID,
			IncidentTitle:              title,
			IncidentSeverity:           severity,
			IncidentStartTime:          startTime,
			GeneratedAt:                now,
			Methodology:                "Multi-Factor Deterministic Scoring Formula: Score(C) = w1*S_time + w2*S_topo + w3*S_sev + w4*S_blast + w5*S_hist",
			BlastRadiusExplanation:     "Zero candidate infrastructure entities identified.",
			NonInvasiveRecommendations: []string{"Verify telemetry ingestion and node health check connectivity."},
		}
	}

	// 2. Pre-process signal maps per candidate
	signalMap := make(map[string]*candidateSignalInfo)
	for cID := range candidateSet {
		signalMap[cID] = &candidateSignalInfo{
			maxSeverity: model.SeverityInfo,
		}
	}

	// Map timeline events (which correlate alerts, diagnostics, and node events)
	for _, tl := range timeline {
		if tl.NodeID == "" {
			continue
		}
		info, exists := signalMap[tl.NodeID]
		if !exists {
			info = &candidateSignalInfo{maxSeverity: model.SeverityInfo}
			signalMap[tl.NodeID] = info
		}
		if info.earliestTime == nil || tl.Timestamp.Before(*info.earliestTime) {
			t := tl.Timestamp
			info.earliestTime = &t
			info.earliestType = tl.EventType + ": " + tl.Description
		}
		if tl.Severity == model.SeverityCritical || (tl.Severity == model.SeverityWarning && info.maxSeverity != model.SeverityCritical) {
			info.maxSeverity = tl.Severity
		}
	}

	// Map predictions
	for _, p := range predictions {
		if info, exists := signalMap[p.NodeID]; exists {
			info.hasPrediction = true
		}
	}

	// Map recurrence
	for _, r := range recurrence {
		if info, exists := signalMap[r.TargetID]; exists {
			if r.OccurrenceCount >= 2 {
				info.isFlapping = true
			}
		}
	}

	// Find global earliest event time across candidates
	var globalEarliest *time.Time
	var globalLatest *time.Time
	for _, info := range signalMap {
		if info.earliestTime != nil {
			if globalEarliest == nil || info.earliestTime.Before(*globalEarliest) {
				globalEarliest = info.earliestTime
			}
			if globalLatest == nil || info.earliestTime.After(*globalLatest) {
				globalLatest = info.earliestTime
			}
		}
	}
	if globalEarliest == nil {
		t := startTime
		globalEarliest = &t
		globalLatest = &t
	}

	// 3. Evaluate Multi-Factor Scores for each Candidate
	var candidates []RootCauseCandidate

	for candidateID := range candidateSet {
		cand := e.evaluateCandidate(
			candidateID,
			signalMap[candidateID],
			globalEarliest,
			globalLatest,
			affectedNodes,
			g,
		)
		candidates = append(candidates, cand)
	}

	// Sort candidates descending by TotalScore
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].TotalScore != candidates[j].TotalScore {
			return candidates[i].TotalScore > candidates[j].TotalScore
		}
		return candidates[i].NodeID < candidates[j].NodeID
	})

	// Assign 1-based ranks
	for i := range candidates {
		candidates[i].Rank = i + 1
	}

	var primary *RootCauseCandidate
	var alternatives []RootCauseCandidate
	if len(candidates) > 0 {
		p := candidates[0]
		primary = &p
		if len(candidates) > 1 {
			alternatives = candidates[1:]
		}
	}

	// 4. Trace Fault Propagation Chains from Primary Root Cause
	var propagationChains []PropagationChain
	if primary != nil && g != nil {
		propagationChains = e.buildPropagationChains(primary.NodeID, affectedNodes, g)
	}

	// 5. Build Non-Invasive Recommendations
	var recommendations []string
	if primary != nil {
		recommendations = e.generateNonInvasiveRecommendations(primary, g)
	} else {
		recommendations = []string{"Monitor telemetry snapshots and verify alert rule threshold baselines."}
	}

	blastDesc := ""
	if primary != nil {
		blastDesc = fmt.Sprintf(
			"Primary candidate %s (%s) transitively impacts %d downstream node(s), explaining %.1f%% of affected incident nodes.",
			primary.NodeName, primary.NodeType, primary.TransitiveDownstreamCount, primary.ExplainedAffectedPercentage,
		)
	}

	return &RootCauseReport{
		IncidentID:                 incidentID,
		IncidentTitle:              title,
		IncidentSeverity:           severity,
		IncidentStartTime:          startTime,
		PrimaryRootCause:           primary,
		AlternativeCandidates:      alternatives,
		PropagationChains:          propagationChains,
		BlastRadiusExplanation:     blastDesc,
		NonInvasiveRecommendations: recommendations,
		GeneratedAt:                now,
		Methodology: fmt.Sprintf(
			"Multi-Factor Deterministic Formula: Score(C) = %.2f*S_time + %.2f*S_topo + %.2f*S_sev + %.2f*S_blast + %.2f*S_hist",
			e.weights.TemporalWeight, e.weights.TopologyWeight, e.weights.SeverityWeight, e.weights.BlastRadiusWeight, e.weights.HistoricalWeight,
		),
	}
}

// evaluateCandidate calculates the multi-factor scores and reasoning for a single candidate entity.
func (e *RootCauseEngine) evaluateCandidate(
	nodeID string,
	info *candidateSignalInfo,
	globalEarliest *time.Time,
	globalLatest *time.Time,
	affectedNodes []string,
	g *topology.Graph,
) RootCauseCandidate {
	var reasoning []string

	// Resolve node info from topology graph if available
	nodeName := nodeID
	nodeType := topology.NodeTypeService
	nodeStatus := topology.NodeStatusHealthy

	if g != nil {
		if tn, exists := g.GetNode(nodeID); exists {
			nodeName = tn.Name
			nodeType = tn.Type
			nodeStatus = tn.Status
		}
	}

	// --- Factor 1: Temporal Precedence (S_time, 0-100) ---
	sTime := 50.0
	timeExplanation := "No discrete timestamp available; assigned median temporal baseline."
	if info != nil && info.earliestTime != nil && globalEarliest != nil {
		delta := info.earliestTime.Sub(*globalEarliest).Seconds()
		if delta <= 0 {
			sTime = 100.0
			timeExplanation = fmt.Sprintf("Earliest observed signal at T+0s (%s).", info.earliestType)
			reasoning = append(reasoning, "Earliest chronological signal observed across all incident nodes.")
		} else {
			totalSpan := globalLatest.Sub(*globalEarliest).Seconds()
			if totalSpan <= 0 {
				sTime = 100.0
				timeExplanation = "First event observed at incident inception."
			} else {
				// Linear decay from 100 down to 20 over the signal spread
				decay := (delta / totalSpan) * 80.0
				sTime = math.Max(20.0, 100.0-decay)
				timeExplanation = fmt.Sprintf("Signal observed +%.0fs after initial incident onset (%s).", delta, info.earliestType)
			}
		}
	}
	sTime = math.Round(sTime*10) / 10.0

	// --- Factor 2: Topology Centrality & Upstream Reachability (S_topo, 0-100) ---
	sTopo := 40.0
	topoExplanation := "Entity evaluated without active topology graph context."
	directDownstream := 0
	transitiveDownstream := 0
	isSPOF := false
	spofCriticality := 0.0

	if g != nil {
		directDeps := g.GetDirectDependents(nodeID)
		directDownstream = len(directDeps)
		transDeps := g.GetTransitiveDependents(nodeID, 20)
		transitiveDownstream = len(transDeps)

		// Check structural importance by node type
		baseTypeScore := 20.0
		switch nodeType {
		case topology.NodeTypeDatabase:
			baseTypeScore = 40.0
		case topology.NodeTypePhysicalHost, topology.NodeTypeK8sNode:
			baseTypeScore = 35.0
		case topology.NodeTypeStorage, topology.NodeTypeQueue:
			baseTypeScore = 30.0
		case topology.NodeTypeService:
			baseTypeScore = 25.0
		default:
			baseTypeScore = 15.0
		}

		// Check SPOF analysis
		if e.spofAnalyzer != nil {
			spof := e.spofAnalyzer.AnalyzeNode(g, nodeID)
			if spof != nil {
				spofCriticality = spof.CriticalityScore
				if spof.RedundancyLevel == topology.RedundancyNone && spof.DependentsCount > 0 {
					isSPOF = true
					reasoning = append(reasoning, fmt.Sprintf("Identified as Single Point of Failure (SPOF) with %d dependents and zero backup redundancy.", spof.DependentsCount))
				}
			}
		}

		// Check if this node is an upstream root (has no upstream dependencies among affected nodes)
		isUpstreamRoot := true
		upstreamFromNode := g.GetDirectDependencies(nodeID)
		for _, up := range upstreamFromNode {
			for _, aff := range affectedNodes {
				if up.ID == aff {
					isUpstreamRoot = false
					break
				}
			}
			if !isUpstreamRoot {
				break
			}
		}

		// Calculate topology score
		calcTopo := baseTypeScore + math.Min(30.0, float64(transitiveDownstream)*6.0)
		if isSPOF {
			calcTopo += 20.0
		}
		if isUpstreamRoot && len(affectedNodes) > 1 {
			calcTopo += 15.0
			reasoning = append(reasoning, "Upstream root in topology graph; does not depend on other degraded incident nodes.")
		}

		sTopo = math.Min(100.0, math.Max(10.0, calcTopo))
		topoExplanation = fmt.Sprintf("Structural score %.1f with %d direct and %d transitive downstream dependents.", sTopo, directDownstream, transitiveDownstream)
	}
	sTopo = math.Round(sTopo*10) / 10.0

	// --- Factor 3: Fault Severity & Anomaly Magnitude (S_sev, 0-100) ---
	sSev := 25.0
	sevExplanation := "No active critical diagnostic or alert thresholds breached."
	if info != nil {
		switch info.maxSeverity {
		case model.SeverityCritical:
			sSev = 95.0
			sevExplanation = "Critical alert or diagnostic failure actively firing on node."
			reasoning = append(reasoning, "Critical severity fault actively detected on node.")
		case model.SeverityWarning:
			sSev = 65.0
			sevExplanation = "Warning alert or diagnostic degradation detected on node."
			reasoning = append(reasoning, "Warning severity signals detected on node.")
		default:
			if nodeStatus == topology.NodeStatusCritical {
				sSev = 90.0
				sevExplanation = "Node status is in CRITICAL operational state."
			} else if nodeStatus == topology.NodeStatusDegraded {
				sSev = 60.0
				sevExplanation = "Node status is in DEGRADED operational state."
			}
		}
	}
	sSev = math.Round(sSev*10) / 10.0

	// --- Factor 4: Blast Radius Explanation (S_blast, 0-100) ---
	sBlast := 30.0
	blastExplanation := "Evaluated without graph propagation context."
	explainedAffectedPct := 0.0

	if len(affectedNodes) > 0 {
		explainedCount := 0
		for _, affID := range affectedNodes {
			if affID == nodeID {
				explainedCount++
				continue
			}
			if g != nil {
				// If affected node is downstream of candidate (affID depends on nodeID)
				if g.IsReachable(affID, nodeID) {
					explainedCount++
				}
			}
		}

		explainedAffectedPct = (float64(explainedCount) / float64(len(affectedNodes))) * 100.0
		sBlast = math.Min(100.0, math.Max(10.0, explainedAffectedPct))
		blastExplanation = fmt.Sprintf("Explains %d of %d (%.1f%%) affected incident nodes via downstream dependency propagation.", explainedCount, len(affectedNodes), explainedAffectedPct)
		if explainedAffectedPct >= 80.0 {
			reasoning = append(reasoning, fmt.Sprintf("High blast radius coverage: explains %.1f%% of downstream degraded nodes.", explainedAffectedPct))
		}
	}
	sBlast = math.Round(sBlast*10) / 10.0

	// --- Factor 5: Historical Recurrence & Capacity Forecast (S_hist, 0-100) ---
	sHist := 20.0
	histExplanation := "No prior flapping or near-term capacity exhaustion forecast."
	if info != nil {
		if info.isFlapping {
			sHist += 40.0
			histExplanation = "Historical recurrence pattern detected; node exhibits periodic flapping."
			reasoning = append(reasoning, "Node has a confirmed statistical recurrence pattern (flapping).")
		}
		if info.hasPrediction {
			sHist += 35.0
			if histExplanation == "No prior flapping or near-term capacity exhaustion forecast." {
				histExplanation = "Predictive capacity model projects imminent resource exhaustion."
			} else {
				histExplanation += " Predictive capacity models indicate near-term exhaustion."
			}
			reasoning = append(reasoning, "Predictive threshold regression indicates approaching resource exhaustion.")
		}
	}
	sHist = math.Min(100.0, math.Max(10.0, sHist))
	sHist = math.Round(sHist*10) / 10.0

	// --- Total Weighted Score Calculation ---
	factors := []RootCauseFactor{
		{
			Category:     FactorTemporalPrecedence,
			Name:         "Temporal Precedence",
			Score:        sTime,
			Weight:       e.weights.TemporalWeight,
			Contribution: math.Round(sTime*e.weights.TemporalWeight*10) / 10.0,
			Explanation:  timeExplanation,
		},
		{
			Category:     FactorTopologyCentrality,
			Name:         "Topology Centrality",
			Score:        sTopo,
			Weight:       e.weights.TopologyWeight,
			Contribution: math.Round(sTopo*e.weights.TopologyWeight*10) / 10.0,
			Explanation:  topoExplanation,
		},
		{
			Category:     FactorFaultSeverity,
			Name:         "Fault Severity",
			Score:        sSev,
			Weight:       e.weights.SeverityWeight,
			Contribution: math.Round(sSev*e.weights.SeverityWeight*10) / 10.0,
			Explanation:  sevExplanation,
		},
		{
			Category:     FactorBlastRadius,
			Name:         "Blast Radius Coverage",
			Score:        sBlast,
			Weight:       e.weights.BlastRadiusWeight,
			Contribution: math.Round(sBlast*e.weights.BlastRadiusWeight*10) / 10.0,
			Explanation:  blastExplanation,
		},
		{
			Category:     FactorHistoricalContext,
			Name:         "Historical & Capacity Context",
			Score:        sHist,
			Weight:       e.weights.HistoricalWeight,
			Contribution: math.Round(sHist*e.weights.HistoricalWeight*10) / 10.0,
			Explanation:  histExplanation,
		},
	}

	totalScore := 0.0
	for _, f := range factors {
		totalScore += f.Score * f.Weight
	}
	totalScore = math.Round(totalScore*10) / 10.0

	var confidence RootCauseConfidence
	switch {
	case totalScore >= 75.0:
		confidence = RootCauseConfidenceHigh
	case totalScore >= 50.0:
		confidence = RootCauseConfidenceMedium
	default:
		confidence = RootCauseConfidenceLow
	}

	var earliestTime *time.Time
	earliestType := ""
	if info != nil && info.earliestTime != nil {
		earliestTime = info.earliestTime
		earliestType = info.earliestType
	}

	return RootCauseCandidate{
		NodeID:                      nodeID,
		NodeName:                    nodeName,
		NodeType:                    nodeType,
		Status:                      nodeStatus,
		TotalScore:                  totalScore,
		Confidence:                  confidence,
		Factors:                     factors,
		EarliestSignalTime:          earliestTime,
		EarliestSignalType:          earliestType,
		DirectDownstreamCount:       directDownstream,
		TransitiveDownstreamCount:   transitiveDownstream,
		ExplainedAffectedPercentage: explainedAffectedPct,
		IsSPOF:                      isSPOF,
		SPOFCriticality:             spofCriticality,
		Reasoning:                   reasoning,
	}
}

// buildPropagationChains discovers directed dependency propagation chains from the root candidate to affected nodes.
func (e *RootCauseEngine) buildPropagationChains(rootID string, affectedNodes []string, g *topology.Graph) []PropagationChain {
	var chains []PropagationChain

	for _, leafID := range affectedNodes {
		if leafID == rootID {
			continue
		}

		// Find shortest dependency path from leaf to root (since leaf depends on intermediate which depends on root)
		path, found := g.FindShortestPath(leafID, rootID)
		if !found || len(path.Nodes) < 2 {
			continue
		}

		// Reverse path to show root -> intermediate -> leaf propagation
		var hops []PropagationHop
		var descParts []string

		for i := len(path.Nodes) - 1; i >= 0; i-- {
			curr := path.Nodes[i]
			descParts = append(descParts, fmt.Sprintf("%s (%s)", curr.Name, curr.Type))
			if i > 0 {
				prev := path.Nodes[i-1]
				hops = append(hops, PropagationHop{
					SourceID:     curr.ID,
					SourceName:   curr.Name,
					TargetID:     prev.ID,
					TargetName:   prev.Name,
					Relationship: "cascades_impact_to",
				})
			}
		}

		leafNode, _ := g.GetNode(leafID)
		rootNode, _ := g.GetNode(rootID)

		chains = append(chains, PropagationChain{
			RootNodeID:      rootID,
			RootNodeName:    rootNode.Name,
			LeafNodeID:      leafID,
			LeafNodeName:    leafNode.Name,
			Hops:            hops,
			PathDescription: strings.Join(descParts, " -> "),
		})
	}

	// Sort chains by hop count ascending
	sort.Slice(chains, func(i, j int) bool {
		return len(chains[i].Hops) < len(chains[j].Hops)
	})

	// Limit to top 5 propagation chains to avoid dossier overload
	if len(chains) > 5 {
		chains = chains[:5]
	}

	return chains
}

// generateNonInvasiveRecommendations produces safe, read-only operational suggestions.
func (e *RootCauseEngine) generateNonInvasiveRecommendations(candidate *RootCauseCandidate, g *topology.Graph) []string {
	var recs []string

	recs = append(recs, fmt.Sprintf(
		"Inspect telemetry and health metrics on primary root candidate %s (%s).",
		candidate.NodeName, candidate.NodeType,
	))

	switch candidate.NodeType {
	case topology.NodeTypeDatabase:
		recs = append(recs, fmt.Sprintf(
			"Inspect database connection pools, active locks, replication lag, and slow query logs on %s.",
			candidate.NodeName,
		))
	case topology.NodeTypePhysicalHost, topology.NodeTypeK8sNode:
		recs = append(recs, fmt.Sprintf(
			"Verify CPU saturation, memory pressure (OOM events), and filesystem inode/disk usage on host %s.",
			candidate.NodeName,
		))
	case topology.NodeTypeStorage:
		recs = append(recs, fmt.Sprintf(
			"Verify IOPS latency, disk queue depth, and storage mount point responsiveness on %s.",
			candidate.NodeName,
		))
	case topology.NodeTypeQueue:
		recs = append(recs, fmt.Sprintf(
			"Review queue consumer lag, message backlog depth, and dead-letter queue growth on %s.",
			candidate.NodeName,
		))
	default:
		recs = append(recs, fmt.Sprintf(
			"Verify process response latency and upstream dependency response times for service %s.",
			candidate.NodeName,
		))
	}

	if candidate.IsSPOF {
		recs = append(recs, fmt.Sprintf(
			"SPOF Warning: %s has zero redundancy. Consider provisioning standby replica nodes to mitigate single-node blast radius.",
			candidate.NodeName,
		))
	}

	recs = append(recs, "Examine non-invasive rate-of-change trends and statistical baselines for unexpected deviations.")

	return recs
}
