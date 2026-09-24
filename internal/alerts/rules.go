package alerts

import (
	"fmt"
	"strings"
	"time"

	"github.com/DocHoax/watchdog/internal/config"
	"github.com/DocHoax/watchdog/pkg/model"
)

// EvaluatedAlert represents the raw result of a rule evaluation before cooldown checks.
type EvaluatedAlert struct {
	RuleID      string
	RuleName    string
	MetricName  string
	Severity    model.Severity
	Triggered   bool
	ActualValue float64
	Threshold   float64
	Duration    time.Duration
	Cooldown    time.Duration
	Message     string
}

// RuleEvaluator evaluates a specific metric or custom condition against a snapshot.
type RuleEvaluator interface {
	Evaluate(snapshot *model.SystemSnapshot, cfg *config.Config) []EvaluatedAlert
}

// BuiltinRulesEvaluator evaluates standard thresholds configured in config.AlertsConfig.
type BuiltinRulesEvaluator struct{}

// Evaluate checks CPU, Memory, Disk, Process, and Network thresholds.
func (b *BuiltinRulesEvaluator) Evaluate(snapshot *model.SystemSnapshot, cfg *config.Config) []EvaluatedAlert {
	var results []EvaluatedAlert
	if snapshot == nil || cfg == nil {
		return results
	}

	// 1. CPU Usage
	if cfg.Alerts.CPU.Enabled && snapshot.CPU != nil {
		usage := snapshot.CPU.OverallUsage
		thresh := cfg.Alerts.CPU.Threshold
		triggered := usage >= thresh
		msg := fmt.Sprintf("CPU usage is at %.1f%% (threshold: %.1f%%)", usage, thresh)

		sev := model.SeverityWarning
		if usage >= min(thresh+10.0, 99.0) {
			sev = model.SeverityCritical
		}

		results = append(results, EvaluatedAlert{
			RuleID:      "builtin-cpu-usage",
			RuleName:    "High CPU Usage",
			MetricName:  "cpu_usage_pct",
			Severity:    sev,
			Triggered:   triggered,
			ActualValue: usage,
			Threshold:   thresh,
			Duration:    cfg.Alerts.CPU.Duration,
			Cooldown:    cfg.Alerts.CPU.Cooldown,
			Message:     msg,
		})
	}

	// 2. Memory Usage
	if cfg.Alerts.Memory.Enabled && snapshot.Memory != nil && snapshot.Memory.TotalBytes > 0 {
		usage := snapshot.Memory.UsedPercent
		thresh := cfg.Alerts.Memory.Threshold
		triggered := usage >= thresh
		msg := fmt.Sprintf("Memory usage is at %.1f%% (threshold: %.1f%%)", usage, thresh)

		sev := model.SeverityWarning
		if usage >= min(thresh+8.0, 99.0) {
			sev = model.SeverityCritical
		}

		results = append(results, EvaluatedAlert{
			RuleID:      "builtin-mem-usage",
			RuleName:    "High Memory Usage",
			MetricName:  "mem_used_pct",
			Severity:    sev,
			Triggered:   triggered,
			ActualValue: usage,
			Threshold:   thresh,
			Duration:    cfg.Alerts.Memory.Duration,
			Cooldown:    cfg.Alerts.Memory.Cooldown,
			Message:     msg,
		})
	}

	// 3. Disk Space Usage
	if cfg.Alerts.Disk.Enabled && snapshot.Disk != nil {
		thresh := cfg.Alerts.Disk.Threshold
		for _, p := range snapshot.Disk.Partitions {
			if p.TotalBytes == 0 {
				continue
			}
			triggered := p.UsedPercent >= thresh
			ruleID := fmt.Sprintf("builtin-disk-%s", sanitizeRuleID(p.Mountpoint))
			msg := fmt.Sprintf("Disk partition '%s' usage is at %.1f%% (threshold: %.1f%%)", p.Mountpoint, p.UsedPercent, thresh)

			sev := model.SeverityWarning
			if p.UsedPercent >= min(thresh+5.0, 99.0) {
				sev = model.SeverityCritical
			}

			results = append(results, EvaluatedAlert{
				RuleID:      ruleID,
				RuleName:    fmt.Sprintf("High Disk Space (%s)", p.Mountpoint),
				MetricName:  fmt.Sprintf("disk_used_pct_%s", sanitizeRuleID(p.Mountpoint)),
				Severity:    sev,
				Triggered:   triggered,
				ActualValue: p.UsedPercent,
				Threshold:   thresh,
				Duration:    cfg.Alerts.Disk.Duration,
				Cooldown:    cfg.Alerts.Disk.Cooldown,
				Message:     msg,
			})
		}
	}

	// 4. Process Rogues
	if cfg.Alerts.Process.Enabled && snapshot.Processes != nil {
		cpuThresh := cfg.Alerts.Process.CPUThreshold
		memThresh := cfg.Alerts.Process.MemoryThreshold

		for _, p := range snapshot.Processes.Processes {
			if cpuThresh > 0 && p.CPUPercent >= cpuThresh {
				ruleID := fmt.Sprintf("builtin-proc-cpu-%d", p.PID)
				results = append(results, EvaluatedAlert{
					RuleID:      ruleID,
					RuleName:    fmt.Sprintf("High Process CPU (%s)", p.Name),
					MetricName:  "process_cpu_pct",
					Severity:    model.SeverityWarning,
					Triggered:   true,
					ActualValue: p.CPUPercent,
					Threshold:   cpuThresh,
					Duration:    0,
					Cooldown:    cfg.Alerts.Process.Cooldown,
					Message:     fmt.Sprintf("Process '%s' (PID %d) is consuming %.1f%% CPU (threshold: %.1f%%)", p.Name, p.PID, p.CPUPercent, cpuThresh),
				})
			}

			memPct := float64(p.MemoryPercent)
			if memThresh > 0 && memPct >= memThresh {
				ruleID := fmt.Sprintf("builtin-proc-mem-%d", p.PID)
				results = append(results, EvaluatedAlert{
					RuleID:      ruleID,
					RuleName:    fmt.Sprintf("High Process Memory (%s)", p.Name),
					MetricName:  "process_mem_pct",
					Severity:    model.SeverityWarning,
					Triggered:   true,
					ActualValue: memPct,
					Threshold:   memThresh,
					Duration:    0,
					Cooldown:    cfg.Alerts.Process.Cooldown,
					Message:     fmt.Sprintf("Process '%s' (PID %d) is consuming %.1f%% RAM (threshold: %.1f%%)", p.Name, p.PID, memPct, memThresh),
				})
			}
		}
	}

	// 5. Network Error Packets
	if cfg.Alerts.Network.Enabled && snapshot.Network != nil {
		thresh := cfg.Alerts.Network.Threshold
		var totalErrors uint64
		for _, io := range snapshot.Network.IOStats {
			totalErrors += io.ErrIn + io.ErrOut + io.DropIn + io.DropOut
		}

		if totalErrors > uint64(thresh) {
			results = append(results, EvaluatedAlert{
				RuleID:      "builtin-net-errors",
				RuleName:    "Network Packet Errors/Drops",
				MetricName:  "net_errors_total",
				Severity:    model.SeverityWarning,
				Triggered:   true,
				ActualValue: float64(totalErrors),
				Threshold:   thresh,
				Duration:    cfg.Alerts.Network.Duration,
				Cooldown:    cfg.Alerts.Network.Cooldown,
				Message:     fmt.Sprintf("Network interface packet errors/drops detected: %d (threshold: %.0f)", totalErrors, thresh),
			})
		}
	}

	return results
}

// CustomRuleEvaluator evaluates user-defined AlertRules.
type CustomRuleEvaluator struct {
	Rules []model.AlertRule
}

// Evaluate checks user-defined AlertRules against the snapshot.
func (c *CustomRuleEvaluator) Evaluate(snapshot *model.SystemSnapshot, cfg *config.Config) []EvaluatedAlert {
	var results []EvaluatedAlert
	if snapshot == nil {
		return results
	}

	for _, rule := range c.Rules {
		if !rule.Enabled {
			continue
		}

		val, ok := extractMetricValue(snapshot, rule.Metric)
		if !ok {
			continue
		}

		triggered := compareValues(val, rule.Operator, rule.Threshold)
		dur, _ := time.ParseDuration(rule.Duration)
		cd, _ := time.ParseDuration(rule.Cooldown)
		if cd <= 0 {
			cd = 5 * time.Minute
		}

		msg := fmt.Sprintf("Alert '%s': metric %s value %.2f %s threshold %.2f",
			rule.Name, rule.Metric, val, rule.Operator, rule.Threshold)

		results = append(results, EvaluatedAlert{
			RuleID:      rule.ID,
			RuleName:    rule.Name,
			MetricName:  rule.Metric,
			Severity:    rule.Severity,
			Triggered:   triggered,
			ActualValue: val,
			Threshold:   rule.Threshold,
			Duration:    dur,
			Cooldown:    cd,
			Message:     msg,
		})
	}

	return results
}

func extractMetricValue(snapshot *model.SystemSnapshot, metric string) (float64, bool) {
	switch strings.ToLower(metric) {
	case "cpu_usage", "cpu_usage_pct", "cpu":
		if snapshot.CPU != nil {
			return snapshot.CPU.OverallUsage, true
		}
	case "load1":
		if snapshot.CPU != nil {
			return snapshot.CPU.LoadAverage.Load1, true
		}
	case "load5":
		if snapshot.CPU != nil {
			return snapshot.CPU.LoadAverage.Load5, true
		}
	case "load15":
		if snapshot.CPU != nil {
			return snapshot.CPU.LoadAverage.Load15, true
		}
	case "mem_usage", "mem_used_pct", "memory":
		if snapshot.Memory != nil {
			return snapshot.Memory.UsedPercent, true
		}
	case "swap_usage", "swap_used_pct", "swap":
		if snapshot.Memory != nil {
			return snapshot.Memory.SwapUsedPercent, true
		}
	case "disk_usage", "disk_used_pct", "disk":
		if snapshot.Disk != nil {
			return snapshot.Disk.UsedPercent, true
		}
	case "processes_count", "procs":
		if snapshot.Processes != nil {
			return float64(snapshot.Processes.TotalCount), true
		}
	case "zombie_count", "zombies":
		if snapshot.Processes != nil {
			return float64(snapshot.Processes.ZombieCount), true
		}
	}
	return 0, false
}

func compareValues(actual float64, op string, thresh float64) bool {
	switch op {
	case ">":
		return actual > thresh
	case ">=":
		return actual >= thresh
	case "<":
		return actual < thresh
	case "<=":
		return actual <= thresh
	case "==":
		return actual == thresh
	case "!=":
		return actual != thresh
	default:
		return actual >= thresh
	}
}

func sanitizeRuleID(s string) string {
	s = strings.ReplaceAll(s, "/", "-")
	s = strings.ReplaceAll(s, "\\", "-")
	s = strings.ReplaceAll(s, ":", "")
	s = strings.Trim(s, "-")
	if s == "" {
		return "root"
	}
	return s
}
