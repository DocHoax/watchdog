package incidents

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// GenerateIncidentFindings analyzes an incident and generates non-invasive advisory findings.
func GenerateIncidentFindings(inc *Incident) []IntelligenceFinding {
	if inc == nil {
		return nil
	}

	var findings []IntelligenceFinding
	now := time.Now().UTC()

	// 1. Fleet Pattern Finding
	if inc.Scope == IncidentScopeFleet || inc.Scope == IncidentScopeMultiNode {
		findingID := generateFindingID(inc.ID, "fleet_pattern")
		category := FindingCategoryFleetPattern
		title := fmt.Sprintf("Multi-node correlated degradation across %d node(s)", len(inc.AffectedNodes))
		desc := fmt.Sprintf("Incident %s exhibits correlated operational signals across multiple nodes with %s scope and %.1f%% fleet impact.", inc.ID, inc.Scope, inc.Impact.FleetPercentage)

		var evidence []string
		for _, s := range inc.RootSignals {
			evidence = append(evidence, fmt.Sprintf("[%s] %s on node %s: %s", s.Type, s.Source, s.NodeID, s.Description))
			if len(evidence) >= 8 {
				break
			}
		}

		suggestions := []string{
			"Inspect cluster-wide control plane and network fabrics connecting affected nodes.",
			"Check shared configuration management and recent rollout deployments across the impacted node group.",
			"Review centralized dependency logs (e.g., DNS, authentication, or shared storage volumes).",
		}

		conf := FindingConfidenceHigh
		if inc.Confidence == "medium" {
			conf = FindingConfidenceMedium
		} else if inc.Confidence == "low" {
			conf = FindingConfidenceLow
		}

		findings = append(findings, IntelligenceFinding{
			ID:                     findingID,
			Category:               category,
			Severity:               inc.Severity,
			Confidence:             conf,
			Title:                  title,
			Description:            desc,
			AffectedNodes:          inc.AffectedNodes,
			SupportingEvidence:     evidence,
			NonInvasiveSuggestions: suggestions,
			DetectedAt:             now,
		})
	}

	// 2. Resource-Specific Subsystem Findings
	for _, subsystem := range inc.Impact.Subsystems {
		findingID := generateFindingID(inc.ID, "subsystem_"+subsystem)
		var cat FindingCategory
		var title string
		var suggestions []string

		switch subsystem {
		case "cpu":
			cat = FindingCategoryPerformanceDegradation
			title = fmt.Sprintf("High CPU utilization and scheduler pressure detected (%s)", inc.ID)
			suggestions = []string{
				"Inspect top CPU-consuming user-space threads and process execution hierarchies.",
				"Check kernel scheduler wait times and run-queue latency metrics.",
				"Evaluate whether bursty periodic background jobs or cron scripts are running concurrently.",
			}
		case "memory", "swap":
			cat = FindingCategoryResourceExhaustion
			title = fmt.Sprintf("Memory pressure and buffer/cache exhaustion detected (%s)", inc.ID)
			suggestions = []string{
				"Inspect resident memory set sizes (RSS) of top processes.",
				"Review system kernel dmesg/journal for early OOM killer invocations or slab leaks.",
				"Verify memory limits and paging rates across container cgroups.",
			}
		case "disk":
			cat = FindingCategoryResourceExhaustion
			title = fmt.Sprintf("Storage capacity or disk I/O saturation detected (%s)", inc.ID)
			suggestions = []string{
				"Verify storage volume free space and inode utilization levels.",
				"Check device queue depth and I/O await latency across block devices.",
				"Review application logging directories for uncontrolled log growth or rotated log retention.",
			}
		case "network":
			cat = FindingCategoryPerformanceDegradation
			title = fmt.Sprintf("Network interface degradation or packet drop anomaly detected (%s)", inc.ID)
			suggestions = []string{
				"Inspect interface error counters, packet drops, and carrier transitions via ethtool/ip.",
				"Check TCP connection state tables and socket exhaustion metrics.",
				"Validate upstream gateway and switch port duplex/flow-control configurations.",
			}
		default:
			cat = FindingCategoryStabilityRisk
			title = fmt.Sprintf("Operational degradation in %s subsystem (%s)", subsystem, inc.ID)
			suggestions = []string{
				"Review subsystem diagnostic health checks and telemetry baselines.",
				"Examine operating system system logs for hardware or subsystem warning events.",
			}
		}

		var evidence []string
		for _, s := range inc.RootSignals {
			if strings.Contains(strings.ToLower(s.Description+" "+s.Source), subsystem) {
				evidence = append(evidence, fmt.Sprintf("[%s] %s on %s: %s", s.Type, s.Source, s.NodeID, s.Description))
			}
		}

		conf := FindingConfidenceHigh
		if len(evidence) < 2 {
			conf = FindingConfidenceMedium
		}

		findings = append(findings, IntelligenceFinding{
			ID:                     findingID,
			Category:               cat,
			Severity:               inc.Severity,
			Confidence:             conf,
			Title:                  title,
			Description:            fmt.Sprintf("Correlated degradation detected affecting the %s subsystem within incident %s.", subsystem, inc.ID),
			AffectedNodes:          inc.AffectedNodes,
			SupportingEvidence:     evidence,
			NonInvasiveSuggestions: suggestions,
			DetectedAt:             now,
		})
	}

	// 3. Predictive / Capacity Risks Finding
	var hasPrediction bool
	var predEvidence []string
	for _, s := range inc.RootSignals {
		if s.Type == SignalTypePrediction || s.Type == SignalTypeCapacityRisk {
			hasPrediction = true
			predEvidence = append(predEvidence, s.Description)
		}
	}

	if hasPrediction {
		findingID := generateFindingID(inc.ID, "capacity_risk")
		findings = append(findings, IntelligenceFinding{
			ID:                     findingID,
			Category:               FindingCategoryCapacityRisk,
			Severity:               inc.Severity,
			Confidence:             FindingConfidenceHigh,
			Title:                  fmt.Sprintf("Resource capacity threshold exhaustion projected (%s)", inc.ID),
			Description:            fmt.Sprintf("Linear threshold projection indicates resources involved in incident %s will exceed capacity limits.", inc.ID),
			AffectedNodes:          inc.AffectedNodes,
			SupportingEvidence:     predEvidence,
			NonInvasiveSuggestions: []string{
				"Review capacity trends and plan non-disruptive resource scaling or quota adjustments.",
				"Evaluate workload distribution across other nodes in the fleet.",
				"Analyze historical consumption patterns to identify sudden consumption spikes.",
			},
			DetectedAt:             now,
		})
	}

	return findings
}

func generateFindingID(incidentID, topic string) string {
	raw := fmt.Sprintf("%s:%s", incidentID, topic)
	hash := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("find-inc-%s", hex.EncodeToString(hash[:6]))
}
