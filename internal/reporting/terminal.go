package reporting

import (
	"fmt"
	"strings"

	"github.com/watchdog-cli/watchdog/pkg/model"
)

// ANSI color codes
const (
	colorReset  = "\033[0m"
	colorBold   = "\033[1m"
	colorDim    = "\033[2m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorCyan   = "\033[36m"
	colorWhite  = "\033[37m"
)

// GenerateTerminal formats a complete system report for ANSI-compatible terminals.
func GenerateTerminal(data *model.ReportData) string {
	if data == nil {
		return "No report data available.\n"
	}

	var sb strings.Builder

	// Header Banner
	sb.WriteString(colorBold + colorCyan + "================================================================================\n" + colorReset)
	sb.WriteString(colorBold + colorWhite + fmt.Sprintf("  WATCHDOG SYSTEM HEALTH REPORT - %s\n", strings.ToUpper(data.Host.Hostname)) + colorReset)
	sb.WriteString(colorDim + fmt.Sprintf("  Generated: %s | OS: %s (%s %s)\n",
		data.GeneratedAt.Format("2006-01-02 15:04:05 MST"),
		data.Host.OS, data.Host.Platform, data.Host.PlatformVersion) + colorReset)
	sb.WriteString(colorBold + colorCyan + "================================================================================\n\n" + colorReset)

	// Section 1: System Metrics Overview
	sb.WriteString(colorBold + colorWhite + "--- [ 1. SYSTEM METRICS OVERVIEW ] ---\n" + colorReset)

	// CPU
	cpuBar := renderProgressBar(data.CPU.OverallUsage, 24)
	sb.WriteString(fmt.Sprintf("  CPU Usage    : %s %5.1f%% (%d cores, Load: %.2f / %.2f / %.2f)\n",
		cpuBar, data.CPU.OverallUsage, data.CPU.LogicalCores,
		data.CPU.LoadAverage.Load1, data.CPU.LoadAverage.Load5, data.CPU.LoadAverage.Load15))

	// Memory
	memBar := renderProgressBar(data.Memory.UsedPercent, 24)
	sb.WriteString(fmt.Sprintf("  Memory Usage : %s %5.1f%% (%s / %s, Avail: %s)\n",
		memBar, data.Memory.UsedPercent,
		formatBytes(data.Memory.UsedBytes), formatBytes(data.Memory.TotalBytes), formatBytes(data.Memory.AvailableBytes)))

	// Swap
	if data.Memory.SwapTotalBytes > 0 {
		swapBar := renderProgressBar(data.Memory.SwapUsedPercent, 24)
		sb.WriteString(fmt.Sprintf("  Swap Usage   : %s %5.1f%% (%s / %s)\n",
			swapBar, data.Memory.SwapUsedPercent,
			formatBytes(data.Memory.SwapUsedBytes), formatBytes(data.Memory.SwapTotalBytes)))
	}

	// Disk
	diskBar := renderProgressBar(data.Disk.UsedPercent, 24)
	sb.WriteString(fmt.Sprintf("  Root Disk    : %s %5.1f%% (%s / %s)\n",
		diskBar, data.Disk.UsedPercent,
		formatBytes(data.Disk.UsedBytes), formatBytes(data.Disk.TotalBytes)))

	// Network
	sb.WriteString(fmt.Sprintf("  Network I/O  : Rx: %s/s | Tx: %s/s (Total Rx: %s, Tx: %s)\n",
		formatBytes(uint64(data.Network.TotalRxRate)),
		formatBytes(uint64(data.Network.TotalTxRate)),
		formatBytes(data.Network.TotalBytesRecv),
		formatBytes(data.Network.TotalBytesSent)))

	sb.WriteString("\n")

	// Section 2: Diagnostics Summary
	if data.Diagnostics != nil {
		sb.WriteString(colorBold + colorWhite + "--- [ 2. AUTOMATED DIAGNOSTICS ] ---\n" + colorReset)
		statusColor := colorGreen
		switch data.Diagnostics.OverallStatus {
		case model.StatusFail:
			statusColor = colorRed
		case model.StatusWarning:
			statusColor = colorYellow
		}
		sb.WriteString(fmt.Sprintf("  Overall Health: %s%s[%s]%s (Passed: %d, Warnings: %d, Critical: %d)\n\n",
			colorBold, statusColor, data.Diagnostics.OverallStatus, colorReset,
			data.Diagnostics.PassedChecks, data.Diagnostics.WarningChecks, data.Diagnostics.CriticalChecks))

		for _, check := range data.Diagnostics.Results {
			icon := colorGreen + "[PASS]" + colorReset
			if check.Status == model.StatusFail {
				icon = colorRed + colorBold + "[CRIT]" + colorReset
			} else if check.Status == model.StatusWarning {
				icon = colorYellow + colorBold + "[WARN]" + colorReset
			}

			sb.WriteString(fmt.Sprintf("  %s %-24s : %s\n", icon, check.Name, check.Description))
			if (check.Status == model.StatusWarning || check.Status == model.StatusFail) && check.Recommendation != "" {
				sb.WriteString(fmt.Sprintf("         %s-> Action:%s %s\n", colorCyan, colorReset, check.Recommendation))
			}
		}
		sb.WriteString("\n")
	}

	// Section 3: Statistical Anomalies
	if data.Anomalies != nil && len(data.Anomalies.Scores) > 0 {
		anomalyCount := 0
		for _, a := range data.Anomalies.Scores {
			if a.IsAnomaly {
				anomalyCount++
			}
		}

		if anomalyCount > 0 {
			sb.WriteString(colorBold + colorWhite + "--- [ 3. STATISTICAL ANOMALIES DETECTED ] ---\n" + colorReset)
			for _, a := range data.Anomalies.Scores {
				if !a.IsAnomaly {
					continue
				}
				sevColor := colorYellow
				if a.Severity == model.SeverityCritical {
					sevColor = colorRed
				}
				sb.WriteString(fmt.Sprintf("  %s%-8s%s %-18s : %s\n",
					sevColor+colorBold, fmt.Sprintf("[%s]", a.Severity), colorReset,
					a.MetricName, a.Explanation))
			}
			sb.WriteString("\n")
		}
	}

	// Section 4: Active Alerts
	if len(data.ActiveAlerts) > 0 {
		sb.WriteString(colorBold + colorWhite + "--- [ 4. ACTIVE ALERTS ] ---\n" + colorReset)
		for _, alert := range data.ActiveAlerts {
			sevColor := colorYellow
			if alert.Severity == model.SeverityCritical {
				sevColor = colorRed
			}
			sb.WriteString(fmt.Sprintf("  %s%-8s%s %-20s : %s (Fired: %s)\n",
				sevColor+colorBold, fmt.Sprintf("[%s]", alert.Severity), colorReset,
				alert.RuleName, alert.Message, alert.FiredAt.Format("15:04:05")))
		}
		sb.WriteString("\n")
	}

	// Section 5: Top Processes
	if len(data.TopProcesses) > 0 {
		sb.WriteString(colorBold + colorWhite + "--- [ 5. TOP RESOURCE-CONSUMING PROCESSES ] ---\n" + colorReset)
		sb.WriteString(fmt.Sprintf("  %-8s %-20s %-8s %-8s %-12s %s\n", "PID", "NAME", "CPU %", "MEM %", "RSS", "COMMAND"))
		sb.WriteString("  " + strings.Repeat("-", 76) + "\n")

		limit := 8
		if len(data.TopProcesses) < limit {
			limit = len(data.TopProcesses)
		}

		for i := 0; i < limit; i++ {
			p := data.TopProcesses[i]
			cmd := p.CommandLine
			if len(cmd) > 28 {
				cmd = cmd[:25] + "..."
			}
			if cmd == "" {
				cmd = p.Name
			}
			sb.WriteString(fmt.Sprintf("  %-8d %-20s %6.1f%% %6.1f%% %-12s %s\n",
				p.PID, truncate(p.Name, 18), p.CPUPercent, p.MemoryPercent, formatBytes(p.MemoryRSS), cmd))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func renderProgressBar(pct float64, width int) string {
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}

	filled := int((pct / 100.0) * float64(width))
	if filled > width {
		filled = width
	}
	empty := width - filled

	barColor := colorGreen
	if pct >= 85.0 {
		barColor = colorRed
	} else if pct >= 70.0 {
		barColor = colorYellow
	}

	return fmt.Sprintf("[%s%s%s%s]", barColor, strings.Repeat("█", filled), strings.Repeat("░", empty), colorReset)
}

func formatBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-2] + ".."
}
