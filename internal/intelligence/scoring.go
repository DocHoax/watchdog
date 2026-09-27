package intelligence

import (
	"fmt"
	"math"
	"time"

	"github.com/DocHoax/watchdog/pkg/model"
)

// ScoringConfig holds thresholds and weighting options for health score calculation.
type ScoringConfig struct {
	CPUWarningThreshold      float64
	CPUCriticalThreshold     float64
	CPULoadWarningThreshold  float64
	CPULoadCriticalThreshold float64

	MemWarningThreshold      float64
	MemCriticalThreshold     float64
	SwapWarningThreshold     float64
	SwapCriticalThreshold    float64

	DiskWarningThreshold     float64
	DiskCriticalThreshold    float64
	InodeWarningThreshold    float64
	InodeCriticalThreshold   float64

	MaxCPUDeduction         float64
	MaxMemDeduction         float64
	MaxDiskDeduction        float64
	MaxAlertDeduction       float64
	MaxDiagnosticDeduction  float64
	MaxAnomalyDeduction     float64
}

// DefaultScoringConfig returns sensible production defaults for the health scoring engine.
func DefaultScoringConfig() ScoringConfig {
	return ScoringConfig{
		CPUWarningThreshold:      85.0,
		CPUCriticalThreshold:     95.0,
		CPULoadWarningThreshold:  1.5,
		CPULoadCriticalThreshold: 3.0,

		MemWarningThreshold:      85.0,
		MemCriticalThreshold:     95.0,
		SwapWarningThreshold:     50.0,
		SwapCriticalThreshold:    80.0,

		DiskWarningThreshold:     85.0,
		DiskCriticalThreshold:    95.0,
		InodeWarningThreshold:    85.0,
		InodeCriticalThreshold:   95.0,

		MaxCPUDeduction:        25.0,
		MaxMemDeduction:        25.0,
		MaxDiskDeduction:       20.0,
		MaxAlertDeduction:      30.0,
		MaxDiagnosticDeduction: 25.0,
		MaxAnomalyDeduction:    15.0,
	}
}

// Scorer calculates deterministic and explainable 0-100 health scores with full factor breakdowns.
type Scorer struct {
	cfg ScoringConfig
}

// NewScorer instantiates a new health scoring engine.
func NewScorer(cfg *ScoringConfig) *Scorer {
	if cfg == nil {
		c := DefaultScoringConfig()
		cfg = &c
	}
	return &Scorer{cfg: *cfg}
}

// ScoreEvaluationInput provides all available telemetry and status signals for a node.
type ScoreEvaluationInput struct {
	NodeID       string
	Hostname     string
	Status       model.NodeStatus
	Snapshot     *model.SystemSnapshot
	Diagnostics  *model.DiagnosticReport
	ActiveAlerts []model.AlertEvent
	Anomalies    *model.AnomalyReport
	EvaluatedAt  time.Time
}

// Calculate evaluates input telemetry signals and produces a normalized, explainable HealthScore.
func (s *Scorer) Calculate(input ScoreEvaluationInput) HealthScore {
	evalTime := input.EvaluatedAt
	if evalTime.IsZero() {
		evalTime = time.Now()
	}

	// Handle offline or stale nodes directly: score is 0.0 with explicit factor explanation
	if input.Status == model.NodeStatusOffline || input.Status == model.NodeStatusStale {
		explanation := fmt.Sprintf("Node is %s; no recent heartbeat received", input.Status)
		return HealthScore{
			Score:            0.0,
			NormalizedStatus: input.Status,
			Trajectory:       TrajectoryUnknown,
			Breakdown: []FactorContribution{
				{
					Name:        "Node Availability",
					Category:    "availability",
					Weight:      1.0,
					Score:       0.0,
					Deduction:   100.0,
					Impact:      ImpactNegative,
					Explanation: explanation,
				},
			},
			PrimaryConcerns: []string{explanation},
			EvaluatedAt:     evalTime,
		}
	}

	var breakdown []FactorContribution
	var primaryConcerns []string
	totalDeductions := 0.0

	// 1. CPU Subsystem Evaluation (Max 25 pts deduction)
	cpuDeduction, cpuFactors, cpuConcerns := s.evaluateCPU(input.Snapshot)
	totalDeductions += cpuDeduction
	breakdown = append(breakdown, cpuFactors...)
	primaryConcerns = append(primaryConcerns, cpuConcerns...)

	// 2. Memory Subsystem Evaluation (Max 25 pts deduction)
	memDeduction, memFactors, memConcerns := s.evaluateMemory(input.Snapshot)
	totalDeductions += memDeduction
	breakdown = append(breakdown, memFactors...)
	primaryConcerns = append(primaryConcerns, memConcerns...)

	// 3. Disk Subsystem Evaluation (Max 20 pts deduction)
	diskDeduction, diskFactors, diskConcerns := s.evaluateDisk(input.Snapshot)
	totalDeductions += diskDeduction
	breakdown = append(breakdown, diskFactors...)
	primaryConcerns = append(primaryConcerns, diskConcerns...)

	// 4. Active Alerts Evaluation (Max 30 pts deduction)
	alertDeduction, alertFactors, alertConcerns := s.evaluateAlerts(input.ActiveAlerts)
	totalDeductions += alertDeduction
	breakdown = append(breakdown, alertFactors...)
	primaryConcerns = append(primaryConcerns, alertConcerns...)

	// 5. Diagnostics Evaluation (Max 25 pts deduction)
	diagDeduction, diagFactors, diagConcerns := s.evaluateDiagnostics(input.Diagnostics)
	totalDeductions += diagDeduction
	breakdown = append(breakdown, diagFactors...)
	primaryConcerns = append(primaryConcerns, diagConcerns...)

	// 6. Anomalies Evaluation (Max 15 pts deduction)
	anomDeduction, anomFactors, anomConcerns := s.evaluateAnomalies(input.Anomalies)
	totalDeductions += anomDeduction
	breakdown = append(breakdown, anomFactors...)
	primaryConcerns = append(primaryConcerns, anomConcerns...)

	// Compute base score (clamped between 0.0 and 100.0)
	rawScore := math.Max(0.0, math.Min(100.0, 100.0-totalDeductions))

	// Apply authoritative status capping
	finalScore := rawScore
	effectiveStatus := input.Status
	if effectiveStatus == "" {
		effectiveStatus = model.NodeStatusHealthy
	}

	if effectiveStatus == model.NodeStatusCritical && finalScore > 49.0 {
		cappingDeduction := finalScore - 49.0
		finalScore = 49.0
		breakdown = append(breakdown, FactorContribution{
			Name:        "Authoritative Status Capping (Critical)",
			Category:    "status_override",
			Weight:      1.0,
			Score:       49.0,
			Deduction:   cappingDeduction,
			Impact:      ImpactNegative,
			Explanation: "Health score capped at 49.0 due to authoritative CRITICAL node status",
		})
	} else if effectiveStatus == model.NodeStatusWarning && finalScore > 79.0 {
		cappingDeduction := finalScore - 79.0
		finalScore = 79.0
		breakdown = append(breakdown, FactorContribution{
			Name:        "Authoritative Status Capping (Warning)",
			Category:    "status_override",
			Weight:      1.0,
			Score:       79.0,
			Deduction:   cappingDeduction,
			Impact:      ImpactNegative,
			Explanation: "Health score capped at 79.0 due to authoritative WARNING node status",
		})
	}

	// Determine normalized status if not authoritative or if degradation dropped score further
	normalizedStatus := effectiveStatus
	if normalizedStatus == model.NodeStatusHealthy {
		if finalScore < 50.0 {
			normalizedStatus = model.NodeStatusCritical
		} else if finalScore < 80.0 {
			normalizedStatus = model.NodeStatusWarning
		}
	}

	// If no concerns were identified, add a positive contribution
	if len(breakdown) == 0 {
		breakdown = append(breakdown, FactorContribution{
			Name:        "All Subsystems Nominal",
			Category:    "general",
			Weight:      1.0,
			Score:       100.0,
			Deduction:   0.0,
			Impact:      ImpactPositive,
			Explanation: "All CPU, memory, storage, and health checks are operating within standard parameters",
		})
	}

	return HealthScore{
		Score:            math.Round(finalScore*10) / 10,
		NormalizedStatus: normalizedStatus,
		Trajectory:       TrajectoryStable, // Default; overridden when historical points are supplied
		Breakdown:        breakdown,
		PrimaryConcerns:  primaryConcerns,
		EvaluatedAt:      evalTime,
	}
}

// evaluateCPU checks utilization and load averages.
func (s *Scorer) evaluateCPU(snap *model.SystemSnapshot) (float64, []FactorContribution, []string) {
	if snap == nil || snap.CPU == nil {
		return 0.0, nil, nil
	}

	var factors []FactorContribution
	var concerns []string
	deduction := 0.0

	// 1. Overall CPU Usage
	usage := snap.CPU.OverallUsage
	if usage >= s.cfg.CPUCriticalThreshold {
		d := 15.0
		deduction += d
		msg := fmt.Sprintf("Critical CPU utilization at %.1f%% (threshold >= %.1f%%)", usage, s.cfg.CPUCriticalThreshold)
		concerns = append(concerns, msg)
		factors = append(factors, FactorContribution{
			Name:        "CPU Utilization Critical",
			Category:    "cpu",
			Weight:      0.6,
			Score:       math.Max(0, 100-usage),
			Deduction:   d,
			Impact:      ImpactNegative,
			Explanation: msg,
		})
	} else if usage >= s.cfg.CPUWarningThreshold {
		d := 8.0
		deduction += d
		msg := fmt.Sprintf("High CPU utilization at %.1f%% (threshold >= %.1f%%)", usage, s.cfg.CPUWarningThreshold)
		concerns = append(concerns, msg)
		factors = append(factors, FactorContribution{
			Name:        "CPU Utilization Warning",
			Category:    "cpu",
			Weight:      0.4,
			Score:       math.Max(0, 100-usage),
			Deduction:   d,
			Impact:      ImpactNegative,
			Explanation: msg,
		})
	}

	// 2. Load Average per Core
	if snap.CPU.LogicalCores > 0 {
		loadRatio := snap.CPU.LoadAverage.Load1 / float64(snap.CPU.LogicalCores)
		if loadRatio >= s.cfg.CPULoadCriticalThreshold {
			d := 10.0
			deduction += d
			msg := fmt.Sprintf("Critical 1m load average ratio at %.2fx per core (Load1: %.2f across %d cores)",
				loadRatio, snap.CPU.LoadAverage.Load1, snap.CPU.LogicalCores)
			concerns = append(concerns, msg)
			factors = append(factors, FactorContribution{
				Name:        "CPU Load Average Critical",
				Category:    "cpu",
				Weight:      0.4,
				Score:       math.Max(0, 100-(loadRatio*20)),
				Deduction:   d,
				Impact:      ImpactNegative,
				Explanation: msg,
			})
		} else if loadRatio >= s.cfg.CPULoadWarningThreshold {
			d := 5.0
			deduction += d
			msg := fmt.Sprintf("Elevated 1m load average ratio at %.2fx per core (Load1: %.2f across %d cores)",
				loadRatio, snap.CPU.LoadAverage.Load1, snap.CPU.LogicalCores)
			concerns = append(concerns, msg)
			factors = append(factors, FactorContribution{
				Name:        "CPU Load Average Elevated",
				Category:    "cpu",
				Weight:      0.3,
				Score:       math.Max(0, 100-(loadRatio*20)),
				Deduction:   d,
				Impact:      ImpactNegative,
				Explanation: msg,
			})
		}
	}

	finalDeduction := math.Min(deduction, s.cfg.MaxCPUDeduction)
	return finalDeduction, factors, concerns
}

// evaluateMemory checks RAM and swap usage.
func (s *Scorer) evaluateMemory(snap *model.SystemSnapshot) (float64, []FactorContribution, []string) {
	if snap == nil || snap.Memory == nil {
		return 0.0, nil, nil
	}

	var factors []FactorContribution
	var concerns []string
	deduction := 0.0

	// 1. RAM Utilization
	memPct := snap.Memory.UsedPercent
	if memPct >= s.cfg.MemCriticalThreshold {
		d := 15.0
		deduction += d
		msg := fmt.Sprintf("Critical memory usage at %.1f%% (threshold >= %.1f%%)", memPct, s.cfg.MemCriticalThreshold)
		concerns = append(concerns, msg)
		factors = append(factors, FactorContribution{
			Name:        "Memory Utilization Critical",
			Category:    "memory",
			Weight:      0.6,
			Score:       math.Max(0, 100-memPct),
			Deduction:   d,
			Impact:      ImpactNegative,
			Explanation: msg,
		})
	} else if memPct >= s.cfg.MemWarningThreshold {
		d := 8.0
		deduction += d
		msg := fmt.Sprintf("Elevated memory usage at %.1f%% (threshold >= %.1f%%)", memPct, s.cfg.MemWarningThreshold)
		concerns = append(concerns, msg)
		factors = append(factors, FactorContribution{
			Name:        "Memory Utilization Warning",
			Category:    "memory",
			Weight:      0.4,
			Score:       math.Max(0, 100-memPct),
			Deduction:   d,
			Impact:      ImpactNegative,
			Explanation: msg,
		})
	}

	// 2. Swap Utilization (only if swap is configured)
	if snap.Memory.SwapTotalBytes > 0 {
		swapPct := snap.Memory.SwapUsedPercent
		if swapPct >= s.cfg.SwapCriticalThreshold {
			d := 10.0
			deduction += d
			msg := fmt.Sprintf("Critical swap usage at %.1f%% (threshold >= %.1f%%)", swapPct, s.cfg.SwapCriticalThreshold)
			concerns = append(concerns, msg)
			factors = append(factors, FactorContribution{
				Name:        "Swap Utilization Critical",
				Category:    "memory",
				Weight:      0.4,
				Score:       math.Max(0, 100-swapPct),
				Deduction:   d,
				Impact:      ImpactNegative,
				Explanation: msg,
			})
		} else if swapPct >= s.cfg.SwapWarningThreshold {
			d := 5.0
			deduction += d
			msg := fmt.Sprintf("Active swap usage at %.1f%% (threshold >= %.1f%%)", swapPct, s.cfg.SwapWarningThreshold)
			concerns = append(concerns, msg)
			factors = append(factors, FactorContribution{
				Name:        "Swap Utilization Warning",
				Category:    "memory",
				Weight:      0.2,
				Score:       math.Max(0, 100-swapPct),
				Deduction:   d,
				Impact:      ImpactNegative,
				Explanation: msg,
			})
		}
	}

	finalDeduction := math.Min(deduction, s.cfg.MaxMemDeduction)
	return finalDeduction, factors, concerns
}

// evaluateDisk checks partition capacity and inode exhaustion.
func (s *Scorer) evaluateDisk(snap *model.SystemSnapshot) (float64, []FactorContribution, []string) {
	if snap == nil || snap.Disk == nil {
		return 0.0, nil, nil
	}

	var factors []FactorContribution
	var concerns []string
	deduction := 0.0

	// Check partitions
	maxDiskPct := 0.0
	worstMount := ""
	for _, p := range snap.Disk.Partitions {
		if p.UsedPercent > maxDiskPct {
			maxDiskPct = p.UsedPercent
			worstMount = p.Mountpoint
		}
		if p.InodesPct >= s.cfg.InodeCriticalThreshold {
			d := 10.0
			deduction += d
			msg := fmt.Sprintf("Critical inode exhaustion on %s at %.1f%%", p.Mountpoint, p.InodesPct)
			concerns = append(concerns, msg)
			factors = append(factors, FactorContribution{
				Name:        fmt.Sprintf("Inode Exhaustion (%s)", p.Mountpoint),
				Category:    "disk",
				Weight:      0.5,
				Score:       math.Max(0, 100-p.InodesPct),
				Deduction:   d,
				Impact:      ImpactNegative,
				Explanation: msg,
			})
		} else if p.InodesPct >= s.cfg.InodeWarningThreshold {
			d := 4.0
			deduction += d
			msg := fmt.Sprintf("Elevated inode usage on %s at %.1f%%", p.Mountpoint, p.InodesPct)
			concerns = append(concerns, msg)
			factors = append(factors, FactorContribution{
				Name:        fmt.Sprintf("Inode Usage Warning (%s)", p.Mountpoint),
				Category:    "disk",
				Weight:      0.2,
				Score:       math.Max(0, 100-p.InodesPct),
				Deduction:   d,
				Impact:      ImpactNegative,
				Explanation: msg,
			})
		}
	}

	// Aggregate disk usage check
	diskUsage := snap.Disk.UsedPercent
	if maxDiskPct > diskUsage {
		diskUsage = maxDiskPct
	}

	if diskUsage >= s.cfg.DiskCriticalThreshold {
		d := 15.0
		deduction += d
		msg := fmt.Sprintf("Critical disk utilization at %.1f%% (mount: %s)", diskUsage, worstMount)
		concerns = append(concerns, msg)
		factors = append(factors, FactorContribution{
			Name:        "Disk Utilization Critical",
			Category:    "disk",
			Weight:      0.6,
			Score:       math.Max(0, 100-diskUsage),
			Deduction:   d,
			Impact:      ImpactNegative,
			Explanation: msg,
		})
	} else if diskUsage >= s.cfg.DiskWarningThreshold {
		d := 7.0
		deduction += d
		msg := fmt.Sprintf("High disk utilization at %.1f%% (mount: %s)", diskUsage, worstMount)
		concerns = append(concerns, msg)
		factors = append(factors, FactorContribution{
			Name:        "Disk Utilization Warning",
			Category:    "disk",
			Weight:      0.3,
			Score:       math.Max(0, 100-diskUsage),
			Deduction:   d,
			Impact:      ImpactNegative,
			Explanation: msg,
		})
	}

	finalDeduction := math.Min(deduction, s.cfg.MaxDiskDeduction)
	return finalDeduction, factors, concerns
}

// evaluateAlerts evaluates active alert events.
func (s *Scorer) evaluateAlerts(alerts []model.AlertEvent) (float64, []FactorContribution, []string) {
	if len(alerts) == 0 {
		return 0.0, nil, nil
	}

	var factors []FactorContribution
	var concerns []string
	deduction := 0.0

	critCount := 0
	warnCount := 0

	for _, a := range alerts {
		if !a.IsActive {
			continue
		}
		if a.Severity == model.SeverityCritical {
			critCount++
			d := 12.0
			deduction += d
			msg := fmt.Sprintf("Active Critical Alert [%s]: %s", a.RuleName, a.Message)
			concerns = append(concerns, msg)
			factors = append(factors, FactorContribution{
				Name:        fmt.Sprintf("Alert: %s", a.RuleName),
				Category:    "alert",
				Weight:      0.5,
				Score:       0.0,
				Deduction:   d,
				Impact:      ImpactNegative,
				Explanation: msg,
			})
		} else if a.Severity == model.SeverityWarning {
			warnCount++
			d := 5.0
			deduction += d
			msg := fmt.Sprintf("Active Warning Alert [%s]: %s", a.RuleName, a.Message)
			concerns = append(concerns, msg)
			factors = append(factors, FactorContribution{
				Name:        fmt.Sprintf("Alert: %s", a.RuleName),
				Category:    "alert",
				Weight:      0.3,
				Score:       50.0,
				Deduction:   d,
				Impact:      ImpactNegative,
				Explanation: msg,
			})
		}
	}

	finalDeduction := math.Min(deduction, s.cfg.MaxAlertDeduction)
	return finalDeduction, factors, concerns
}

// evaluateDiagnostics evaluates diagnostic report check failures and warnings.
func (s *Scorer) evaluateDiagnostics(diag *model.DiagnosticReport) (float64, []FactorContribution, []string) {
	if diag == nil || len(diag.Results) == 0 {
		return 0.0, nil, nil
	}

	var factors []FactorContribution
	var concerns []string
	deduction := 0.0

	for _, r := range diag.Results {
		if r.Status == model.StatusFail || r.Severity == model.SeverityCritical {
			d := 8.0
			deduction += d
			msg := fmt.Sprintf("Diagnostic check failed [%s]: %s", r.Name, r.Description)
			concerns = append(concerns, msg)
			factors = append(factors, FactorContribution{
				Name:        fmt.Sprintf("Diagnostic Failure: %s", r.Name),
				Category:    "diagnostic",
				Weight:      0.4,
				Score:       0.0,
				Deduction:   d,
				Impact:      ImpactNegative,
				Explanation: msg,
			})
		} else if r.Status == model.StatusWarning || r.Severity == model.SeverityWarning {
			d := 3.0
			deduction += d
			msg := fmt.Sprintf("Diagnostic check warning [%s]: %s", r.Name, r.Description)
			concerns = append(concerns, msg)
			factors = append(factors, FactorContribution{
				Name:        fmt.Sprintf("Diagnostic Warning: %s", r.Name),
				Category:    "diagnostic",
				Weight:      0.2,
				Score:       60.0,
				Deduction:   d,
				Impact:      ImpactNegative,
				Explanation: msg,
			})
		}
	}

	finalDeduction := math.Min(deduction, s.cfg.MaxDiagnosticDeduction)
	return finalDeduction, factors, concerns
}

// evaluateAnomalies evaluates statistical anomaly deviations.
func (s *Scorer) evaluateAnomalies(anom *model.AnomalyReport) (float64, []FactorContribution, []string) {
	if anom == nil || len(anom.Scores) == 0 {
		return 0.0, nil, nil
	}

	var factors []FactorContribution
	var concerns []string
	deduction := 0.0

	for _, score := range anom.Scores {
		if !score.IsAnomaly {
			continue
		}
		if score.Severity == model.SeverityCritical {
			d := 7.0
			deduction += d
			msg := fmt.Sprintf("Critical anomaly on %s: %s", score.MetricName, score.Explanation)
			concerns = append(concerns, msg)
			factors = append(factors, FactorContribution{
				Name:        fmt.Sprintf("Anomaly: %s", score.MetricName),
				Category:    "anomaly",
				Weight:      0.3,
				Score:       20.0,
				Deduction:   d,
				Impact:      ImpactNegative,
				Explanation: msg,
			})
		} else if score.Severity == model.SeverityWarning {
			d := 3.0
			deduction += d
			msg := fmt.Sprintf("Statistical anomaly on %s: %s", score.MetricName, score.Explanation)
			concerns = append(concerns, msg)
			factors = append(factors, FactorContribution{
				Name:        fmt.Sprintf("Anomaly: %s", score.MetricName),
				Category:    "anomaly",
				Weight:      0.2,
				Score:       50.0,
				Deduction:   d,
				Impact:      ImpactNegative,
				Explanation: msg,
			})
		}
	}

	finalDeduction := math.Min(deduction, s.cfg.MaxAnomalyDeduction)
	return finalDeduction, factors, concerns
}
