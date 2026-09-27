package intelligence

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/DocHoax/watchdog/pkg/model"
)

// FleetPatternAnalyzer analyzes fleet-wide health summaries, anomalies, and diagnostics for cross-node patterns.
type FleetPatternAnalyzer struct{}

// NewFleetPatternAnalyzer instantiates a new fleet pattern analyzer.
func NewFleetPatternAnalyzer() *FleetPatternAnalyzer {
	return &FleetPatternAnalyzer{}
}

// AnalyzeFleet examines aggregated node summaries to discover cross-node correlations and patterns.
func (fpa *FleetPatternAnalyzer) AnalyzeFleet(summaries []NodeHealthSummary) []IntelligenceFinding {
	if len(summaries) == 0 {
		return nil
	}

	var findings []IntelligenceFinding
	now := time.Now()

	// 1. Detect Concurrent High CPU Pressure across multiple nodes
	var highCPUNodes []string
	var highCPUDetails []string
	for _, s := range summaries {
		for _, f := range s.HealthScore.Breakdown {
			if f.Category == "cpu" && f.Deduction >= 8.0 {
				highCPUNodes = append(highCPUNodes, s.NodeID)
				highCPUDetails = append(highCPUDetails, fmt.Sprintf("%s (%s): %s", s.Hostname, s.NodeID, f.Explanation))
				break
			}
		}
	}

	if len(highCPUNodes) >= 3 || (len(summaries) <= 3 && len(highCPUNodes) >= 2) {
		findingID := generateFindingID("cpu-cluster", highCPUNodes)
		findings = append(findings, IntelligenceFinding{
			ID:            findingID,
			Category:      FindingCategoryFleetPattern,
			Severity:      model.SeverityWarning,
			Confidence:    FindingConfidenceHigh,
			Title:         fmt.Sprintf("Simultaneous CPU Pressure Detected across %d Nodes", len(highCPUNodes)),
			Description:   "Multiple nodes in the fleet exhibit concurrent elevated CPU utilization or load average spikes.",
			AffectedNodes: highCPUNodes,
			SupportingEvidence: append([]string{
				fmt.Sprintf("%d of %d evaluated nodes are experiencing CPU pressure simultaneously", len(highCPUNodes), len(summaries)),
			}, highCPUDetails...),
			NonInvasiveSuggestions: []string{
				"Inspect scheduled cron jobs or distributed batch workloads running at this time",
				"Review cluster load balancing and request distribution across affected instances",
			},
			DetectedAt: now,
		})
	}

	// 2. Detect Concurrent Memory / Swap Pressure across multiple nodes
	var highMemNodes []string
	var highMemDetails []string
	for _, s := range summaries {
		for _, f := range s.HealthScore.Breakdown {
			if f.Category == "memory" && f.Deduction >= 8.0 {
				highMemNodes = append(highMemNodes, s.NodeID)
				highMemDetails = append(highMemDetails, fmt.Sprintf("%s (%s): %s", s.Hostname, s.NodeID, f.Explanation))
				break
			}
		}
	}

	if len(highMemNodes) >= 3 || (len(summaries) <= 3 && len(highMemNodes) >= 2) {
		findingID := generateFindingID("mem-cluster", highMemNodes)
		findings = append(findings, IntelligenceFinding{
			ID:            findingID,
			Category:      FindingCategoryResourceExhaustion,
			Severity:      model.SeverityWarning,
			Confidence:    FindingConfidenceHigh,
			Title:         fmt.Sprintf("Widespread Memory Pressure Detected across %d Nodes", len(highMemNodes)),
			Description:   "Multiple nodes are experiencing high memory utilization or active swap pagination.",
			AffectedNodes: highMemNodes,
			SupportingEvidence: append([]string{
				fmt.Sprintf("%d of %d evaluated nodes have memory or swap deductions applied", len(highMemNodes), len(summaries)),
			}, highMemDetails...),
			NonInvasiveSuggestions: []string{
				"Investigate JVM/Go runtime heap allocations and potential memory leaks in distributed services",
				"Evaluate kernel memory fragmentation and swap usage metrics",
			},
			DetectedAt: now,
		})
	}

	// 3. Detect Correlated Diagnostic Check Failures across Fleet
	diagnosticFailures := make(map[string][]string) // checkName -> []nodeID
	for _, s := range summaries {
		for _, f := range s.HealthScore.Breakdown {
			if f.Category == "diagnostic" && f.Deduction > 0 {
				diagnosticFailures[f.Name] = append(diagnosticFailures[f.Name], s.NodeID)
			}
		}
	}

	for checkName, affectedNodes := range diagnosticFailures {
		if len(affectedNodes) >= 2 {
			findingID := generateFindingID("diag-"+checkName, affectedNodes)
			findings = append(findings, IntelligenceFinding{
				ID:            findingID,
				Category:      FindingCategoryFleetPattern,
				Severity:      model.SeverityWarning,
				Confidence:    FindingConfidenceMedium,
				Title:         fmt.Sprintf("Correlated Diagnostic Failure: %s across %d Nodes", checkName, len(affectedNodes)),
				Description:   fmt.Sprintf("The diagnostic check '%s' failed simultaneously on multiple nodes in the fleet.", checkName),
				AffectedNodes: affectedNodes,
				SupportingEvidence: []string{
					fmt.Sprintf("Check '%s' failed on nodes: %v", checkName, affectedNodes),
				},
				NonInvasiveSuggestions: []string{
					"Verify shared upstream dependencies, DNS resolvers, or common configuration files",
					"Review host network routing and common security group rules",
				},
				DetectedAt: now,
			})
		}
	}

	// 4. Detect Degrading Fleet Trajectory
	degradingCount := 0
	var degradingNodes []string
	for _, s := range summaries {
		if s.HealthScore.Trajectory == TrajectoryDegrading {
			degradingCount++
			degradingNodes = append(degradingNodes, s.NodeID)
		}
	}

	if degradingCount >= 3 || (len(summaries) <= 3 && degradingCount >= 2) {
		findingID := generateFindingID("fleet-degradation", degradingNodes)
		findings = append(findings, IntelligenceFinding{
			ID:            findingID,
			Category:      FindingCategoryStabilityRisk,
			Severity:      model.SeverityCritical,
			Confidence:    FindingConfidenceHigh,
			Title:         fmt.Sprintf("Fleet Degradation Trend: %d Nodes Exhibiting Downward Health Trajectory", degradingCount),
			Description:   "A significant cluster of nodes in the fleet is actively losing health score points over recent evaluations.",
			AffectedNodes: degradingNodes,
			SupportingEvidence: []string{
				fmt.Sprintf("%d of %d nodes have degrading health trajectories", degradingCount, len(summaries)),
			},
			NonInvasiveSuggestions: []string{
				"Prioritize triage of lowest-scoring nodes to prevent cascading service degradation",
				"Inspect recent application deployments or traffic spikes",
			},
			DetectedAt: now,
		})
	}

	return findings
}

func generateFindingID(prefix string, nodes []string) string {
	h := sha256.New()
	_, _ = fmt.Fprintf(h, "%s:%v", prefix, nodes)
	return fmt.Sprintf("find-%s-%s", prefix, hex.EncodeToString(h.Sum(nil))[:8])
}
