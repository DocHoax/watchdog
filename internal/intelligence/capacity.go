package intelligence

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/DocHoax/watchdog/internal/storage"
	"github.com/DocHoax/watchdog/pkg/model"
)

// ResourceThresholds holds warning and critical boundary percentages for capacity forecasting.
type ResourceThresholds struct {
	Warning  float64
	Critical float64
}

// DefaultResourceThresholds returns standard operational safety thresholds for hardware resources.
var DefaultResourceThresholds = map[CapacityResource]ResourceThresholds{
	CapacityResourceCPU:    {Warning: 80.0, Critical: 95.0},
	CapacityResourceMemory: {Warning: 85.0, Critical: 95.0},
	CapacityResourceSwap:   {Warning: 50.0, Critical: 80.0},
	CapacityResourceDisk:   {Warning: 85.0, Critical: 95.0},
}

// ResourceMetricMap maps capacity resources to their canonical telemetry metric names.
var ResourceMetricMap = map[CapacityResource]string{
	CapacityResourceCPU:    "cpu.usage_percent",
	CapacityResourceMemory: "memory.usage_percent",
	CapacityResourceSwap:   "swap.usage_percent",
	CapacityResourceDisk:   "disk.usage_percent",
}

// EvaluateNodeCapacity computes multi-resource capacity forecasts and predictions for a node.
func EvaluateNodeCapacity(
	nodeID string,
	hostname string,
	status model.NodeStatus,
	metricPoints map[string][]storage.MetricPoint,
	horizon time.Duration,
	observationWindow time.Duration,
	cfg *ForecastConfig,
) *NodeCapacityReport {
	if cfg == nil {
		c := DefaultForecastConfig()
		cfg = &c
	}
	if horizon <= 0 {
		horizon = 24 * time.Hour
	}

	report := &NodeCapacityReport{
		NodeID:      nodeID,
		Hostname:    hostname,
		Status:      status,
		Forecasts:   make([]CapacityForecast, 0, 4),
		Predictions: make([]Prediction, 0, 8),
		EvaluatedAt: time.Now().UTC(),
	}

	resources := []CapacityResource{
		CapacityResourceCPU,
		CapacityResourceMemory,
		CapacityResourceSwap,
		CapacityResourceDisk,
	}

	for _, res := range resources {
		metricName := ResourceMetricMap[res]
		thresholds := DefaultResourceThresholds[res]
		points := metricPoints[metricName]

		forecast := evaluateResourceForecast(nodeID, res, metricName, points, thresholds, horizon, observationWindow, cfg)
		report.Forecasts = append(report.Forecasts, forecast)

		// Create threshold predictions for warning and critical levels
		predWarn := CalculatePrediction(nodeID, metricName, points, thresholds.Warning, horizon, observationWindow, cfg)
		predCrit := CalculatePrediction(nodeID, metricName, points, thresholds.Critical, horizon, observationWindow, cfg)
		report.Predictions = append(report.Predictions, predWarn, predCrit)
	}

	return report
}

// evaluateResourceForecast builds a CapacityForecast for a specific subsystem.
func evaluateResourceForecast(
	nodeID string,
	res CapacityResource,
	metricName string,
	points []storage.MetricPoint,
	thresholds ResourceThresholds,
	horizon time.Duration,
	observationWindow time.Duration,
	cfg *ForecastConfig,
) CapacityForecast {
	forecast := CapacityForecast{
		Resource:          res,
		Unit:              "%",
		WarningThreshold:  thresholds.Warning,
		CriticalThreshold: thresholds.Critical,
		Horizon:           horizon,
		Confidence:        PredictionConfidenceInsufficientData,
		Evidence:          make([]string, 0),
	}

	// Filter valid points
	var valid []storage.MetricPoint
	var sum float64
	for _, p := range points {
		if !math.IsNaN(p.Value) && !math.IsInf(p.Value, 0) {
			valid = append(valid, p)
			sum += p.Value
		}
	}

	if len(valid) < cfg.MinSamples {
		if len(valid) > 0 {
			forecast.CurrentUtilization = sanitizeFloat(valid[len(valid)-1].Value)
			forecast.BaselineUtilization = sanitizeFloat(sum / float64(len(valid)))
		}
		forecast.Evidence = append(forecast.Evidence,
			fmt.Sprintf("Insufficient samples for %s: %d points available, %d required", res, len(valid), cfg.MinSamples),
		)
		return forecast
	}

	currentVal := valid[len(valid)-1].Value
	baselineVal := sum / float64(len(valid))
	forecast.CurrentUtilization = sanitizeFloat(currentVal)
	forecast.BaselineUtilization = sanitizeFloat(baselineVal)

	// Run predictions for warning and critical
	predWarn := CalculatePrediction(nodeID, metricName, points, thresholds.Warning, horizon, observationWindow, cfg)
	predCrit := CalculatePrediction(nodeID, metricName, points, thresholds.Critical, horizon, observationWindow, cfg)

	forecast.TrendSlopePerMinute = predWarn.SlopePerMinute
	forecast.Confidence = predWarn.Confidence
	forecast.TimeToWarning = predWarn.EstimatedTimeToThreshold
	forecast.TimeToCritical = predCrit.EstimatedTimeToThreshold

	// Calculate projected utilization after horizon
	horizonMinutes := horizon.Minutes()
	projected := currentVal + (predWarn.SlopePerMinute * horizonMinutes)
	if projected < 0.0 {
		projected = 0.0
	} else if projected > 100.0 {
		projected = 100.0
	}
	forecast.ProjectedUtilizationAfterHorizon = sanitizeFloat(projected)

	// Build explainable evidence
	forecast.Evidence = append(forecast.Evidence,
		fmt.Sprintf("Current: %.1f%%, Baseline: %.1f%%, Rate: %+.4f%%/min (R²: %.2f)",
			currentVal, baselineVal, predWarn.SlopePerMinute, predWarn.RSquared),
		fmt.Sprintf("Projected utilization in %v: %.1f%%", horizon.Round(time.Minute), projected),
	)

	if forecast.TimeToWarning != nil {
		forecast.Evidence = append(forecast.Evidence,
			fmt.Sprintf("Projected to cross warning threshold (%.0f%%) in %v", thresholds.Warning, forecast.TimeToWarning.Round(time.Second)),
		)
	}
	if forecast.TimeToCritical != nil {
		forecast.Evidence = append(forecast.Evidence,
			fmt.Sprintf("Projected to cross critical threshold (%.0f%%) in %v", thresholds.Critical, forecast.TimeToCritical.Round(time.Second)),
		)
	}

	return forecast
}

// AggregateFleetCapacity aggregates individual node capacity reports into a FleetCapacitySummary.
func AggregateFleetCapacity(reports []NodeCapacityReport, horizon time.Duration) *FleetCapacitySummary {
	now := time.Now().UTC()
	totalNodes := len(reports)
	summary := &FleetCapacitySummary{
		EvaluatedAt:      now,
		TotalNodes:       totalNodes,
		TopCapacityRisks: make([]NodeCapacityReport, 0),
		FleetPredictions: make([]Prediction, 0),
		Findings:         make([]IntelligenceFinding, 0),
	}

	if totalNodes == 0 {
		return summary
	}

	var cpuPressureCount, memPressureCount, diskPressureCount int
	approachingWarnSet := make(map[string]bool)
	approachingCritSet := make(map[string]bool)

	// Collect predictions and evaluate risk ranking
	type nodeRiskScore struct {
		report    NodeCapacityReport
		riskScore float64
	}
	var scoredNodes []nodeRiskScore

	for _, rep := range reports {
		var maxRisk float64
		for _, fc := range rep.Forecasts {
			// Check pressure (current >= warning or projected >= warning)
			if fc.CurrentUtilization >= fc.WarningThreshold || fc.ProjectedUtilizationAfterHorizon >= fc.WarningThreshold {
				switch fc.Resource {
				case CapacityResourceCPU:
					cpuPressureCount++
				case CapacityResourceMemory:
					memPressureCount++
				case CapacityResourceDisk:
					diskPressureCount++
				}
			}

			if fc.TimeToWarning != nil {
				approachingWarnSet[rep.NodeID] = true
			}
			if fc.TimeToCritical != nil {
				approachingCritSet[rep.NodeID] = true
			}

			// Compute risk score for ranking
			score := fc.CurrentUtilization
			if fc.TrendSlopePerMinute > 0 {
				score += fc.TrendSlopePerMinute * 60.0 // 1-hour projected boost
			}
			if score > maxRisk {
				maxRisk = score
			}
		}

		for _, p := range rep.Predictions {
			if p.Direction == PredictionDirectionApproaching || p.Direction == PredictionDirectionAlreadyExceeded {
				summary.FleetPredictions = append(summary.FleetPredictions, p)
			}
		}

		scoredNodes = append(scoredNodes, nodeRiskScore{
			report:    rep,
			riskScore: maxRisk,
		})
	}

	summary.CPUPressurePercent = sanitizeFloat((float64(cpuPressureCount) / float64(totalNodes)) * 100.0)
	summary.MemoryPressurePercent = sanitizeFloat((float64(memPressureCount) / float64(totalNodes)) * 100.0)
	summary.DiskPressurePercent = sanitizeFloat((float64(diskPressureCount) / float64(totalNodes)) * 100.0)
	summary.NodesApproachingWarning = len(approachingWarnSet)
	summary.NodesApproachingCritical = len(approachingCritSet)

	// Sort nodes by risk score descending
	sort.Slice(scoredNodes, func(i, j int) bool {
		return scoredNodes[i].riskScore > scoredNodes[j].riskScore
	})

	topLimit := 10
	if len(scoredNodes) < topLimit {
		topLimit = len(scoredNodes)
	}
	for i := 0; i < topLimit; i++ {
		if scoredNodes[i].riskScore > 50.0 { // Only include nodes with non-trivial utilization
			summary.TopCapacityRisks = append(summary.TopCapacityRisks, scoredNodes[i].report)
		}
	}

	// Synthesize findings
	if summary.NodesApproachingCritical > 0 {
		var critNodeIDs []string
		for nodeID := range approachingCritSet {
			critNodeIDs = append(critNodeIDs, nodeID)
		}
		summary.Findings = append(summary.Findings, IntelligenceFinding{
			ID:          fmt.Sprintf("find-cap-crit-%d", now.Unix()),
			Category:    FindingCategoryCapacityRisk,
			Severity:    model.SeverityCritical,
			Confidence:  FindingConfidenceHigh,
			Title:       fmt.Sprintf("%d node(s) projected to reach critical capacity within %v", summary.NodesApproachingCritical, horizon),
			Description: fmt.Sprintf("Forecasting models project critical resource exhaustion on %d host(s).", summary.NodesApproachingCritical),
			AffectedNodes: critNodeIDs,
			SupportingEvidence: []string{
				fmt.Sprintf("Critical threshold crossings projected within forecast horizon (%v)", horizon),
				fmt.Sprintf("Memory pressure: %.1f%% of fleet, Disk pressure: %.1f%% of fleet", summary.MemoryPressurePercent, summary.DiskPressurePercent),
			},
			NonInvasiveSuggestions: []string{
				"Inspect process memory footprints and verify active workload limits",
				"Audit disk partition growth rates and verify log retention policies",
			},
			DetectedAt: now,
		})
	}

	if summary.MemoryPressurePercent > 50.0 || summary.DiskPressurePercent > 50.0 {
		summary.Findings = append(summary.Findings, IntelligenceFinding{
			ID:          fmt.Sprintf("find-fleet-press-%d", now.Unix()),
			Category:    FindingCategoryFleetCapacityPressure,
			Severity:    model.SeverityWarning,
			Confidence:  FindingConfidenceMedium,
			Title:       "High fleet-wide capacity pressure detected",
			Description: fmt.Sprintf("More than 50%% of cluster nodes are operating near or projected to reach capacity limits (Memory: %.1f%%, Disk: %.1f%%).", summary.MemoryPressurePercent, summary.DiskPressurePercent),
			SupportingEvidence: []string{
				fmt.Sprintf("Fleet Memory Pressure: %.1f%% of nodes", summary.MemoryPressurePercent),
				fmt.Sprintf("Fleet Disk Pressure: %.1f%% of nodes", summary.DiskPressurePercent),
				fmt.Sprintf("Fleet CPU Pressure: %.1f%% of nodes", summary.CPUPressurePercent),
			},
			NonInvasiveSuggestions: []string{
				"Review cluster workload distribution across nodes",
				"Consider provisioning additional nodes or rebalancing high-utilization workloads",
			},
			DetectedAt: now,
		})
	}

	return summary
}
