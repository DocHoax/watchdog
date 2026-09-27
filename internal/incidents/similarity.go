package incidents

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/DocHoax/watchdog/pkg/model"
)

// SimilarIncidentResult represents an incident with its similarity score and match breakdown.
type SimilarIncidentResult struct {
	Incident        Incident                `json:"incident"`
	SimilarityScore float64                 `json:"similarity_score"`
	Breakdown       SimilarityScoreBreakdown `json:"breakdown"`
	Explanation     string                  `json:"explanation"`
}

// SimilarityScoreBreakdown details how the similarity score was derived.
type SimilarityScoreBreakdown struct {
	SymptomSimilarity   float64 `json:"symptom_similarity"`
	SubsystemSimilarity float64 `json:"subsystem_similarity"`
	NodeSimilarity      float64 `json:"node_similarity"`
	SeveritySimilarity  float64 `json:"severity_similarity"`
	Weights             map[string]float64 `json:"weights"`
}

// CalculateSimilarity computes a normalized [0.0, 1.0] similarity score between two incidents.
func CalculateSimilarity(a, b Incident) (float64, SimilarityScoreBreakdown, string) {
	// 1. Symptom Jaccard similarity
	symptomsA := make(map[string]bool)
	for _, s := range a.PrimarySymptoms {
		symptomsA[canonicalSymptom(s)] = true
	}
	for _, sig := range a.RootSignals {
		symptomsA[canonicalSymptom(sig.Source)] = true
	}

	symptomsB := make(map[string]bool)
	for _, s := range b.PrimarySymptoms {
		symptomsB[canonicalSymptom(s)] = true
	}
	for _, sig := range b.RootSignals {
		symptomsB[canonicalSymptom(sig.Source)] = true
	}
	symptomSim := jaccardSimilarity(symptomsA, symptomsB)

	// 2. Subsystem Jaccard similarity
	subsA := sliceToSet(a.Impact.Subsystems)
	subsB := sliceToSet(b.Impact.Subsystems)
	subsystemSim := jaccardSimilarity(subsA, subsB)

	// 3. Node overlap Jaccard similarity
	nodesA := sliceToSet(a.AffectedNodes)
	nodesB := sliceToSet(b.AffectedNodes)
	nodeSim := jaccardSimilarity(nodesA, nodesB)

	// 4. Severity similarity
	severitySim := 0.0
	if a.Severity == b.Severity {
		severitySim = 1.0
	} else if (a.Severity == model.SeverityCritical && b.Severity == model.SeverityWarning) ||
		(a.Severity == model.SeverityWarning && b.Severity == model.SeverityCritical) {
		severitySim = 0.5
	} else if (a.Severity == model.SeverityWarning && b.Severity == model.SeverityInfo) ||
		(a.Severity == model.SeverityInfo && b.Severity == model.SeverityWarning) {
		severitySim = 0.5
	}

	// Weights: Symptoms (40%), Subsystems (30%), Nodes (20%), Severity (10%)
	wSymptom := 0.40
	wSubsystem := 0.30
	wNode := 0.20
	wSeverity := 0.10

	composite := (symptomSim * wSymptom) +
		(subsystemSim * wSubsystem) +
		(nodeSim * wNode) +
		(severitySim * wSeverity)

	composite = math.Round(composite*1000) / 1000.0

	breakdown := SimilarityScoreBreakdown{
		SymptomSimilarity:   math.Round(symptomSim*1000) / 1000.0,
		SubsystemSimilarity: math.Round(subsystemSim*1000) / 1000.0,
		NodeSimilarity:      math.Round(nodeSim*1000) / 1000.0,
		SeveritySimilarity:  math.Round(severitySim*1000) / 1000.0,
		Weights: map[string]float64{
			"symptoms":   wSymptom,
			"subsystems": wSubsystem,
			"nodes":      wNode,
			"severity":   wSeverity,
		},
	}

	explanation := fmt.Sprintf(
		"Similarity score %.2f derived from: symptoms (%.2f), subsystems (%.2f), nodes (%.2f), severity (%.2f)",
		composite, symptomSim, subsystemSim, nodeSim, severitySim,
	)

	return composite, breakdown, explanation
}

// FindSimilarIncidents searches candidates and returns those matching above the threshold, sorted descending.
func FindSimilarIncidents(target Incident, candidates []Incident, minSimilarity float64, limit int) []SimilarIncidentResult {
	if minSimilarity <= 0 {
		minSimilarity = 0.20
	}
	if limit <= 0 {
		limit = 10
	}

	var results []SimilarIncidentResult
	for _, cand := range candidates {
		if cand.ID == target.ID {
			continue
		}
		score, breakdown, explanation := CalculateSimilarity(target, cand)
		if score >= minSimilarity {
			results = append(results, SimilarIncidentResult{
				Incident:        cand,
				SimilarityScore: score,
				Breakdown:       breakdown,
				Explanation:     explanation,
			})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].SimilarityScore != results[j].SimilarityScore {
			return results[i].SimilarityScore > results[j].SimilarityScore
		}
		return results[i].Incident.StartTime.After(results[j].Incident.StartTime)
	})

	if len(results) > limit {
		results = results[:limit]
	}

	return results
}

func canonicalSymptom(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	// Strip node-specific prefixes if formatted as "source on node: desc"
	if idx := strings.Index(s, ":"); idx != -1 {
		s = strings.TrimSpace(s[:idx])
	}
	return s
}

func sliceToSet(slice []string) map[string]bool {
	set := make(map[string]bool)
	for _, item := range slice {
		trimmed := strings.TrimSpace(item)
		if trimmed != "" {
			set[trimmed] = true
		}
	}
	return set
}

func jaccardSimilarity(a, b map[string]bool) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1.0
	}
	if len(a) == 0 || len(b) == 0 {
		return 0.0
	}

	intersection := 0
	unionMap := make(map[string]bool)

	for k := range a {
		unionMap[k] = true
		if b[k] {
			intersection++
		}
	}
	for k := range b {
		unionMap[k] = true
	}

	if len(unionMap) == 0 {
		return 1.0
	}

	return float64(intersection) / float64(len(unionMap))
}
