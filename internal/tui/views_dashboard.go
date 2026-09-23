package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) renderDashboardView() string {
	if m.currentSnapshot == nil {
		return lipgloss.NewStyle().Padding(2, 4).Render("Waiting for system metrics data...")
	}

	snap := m.currentSnapshot

	// Top Host Info Row
	var hostStr string
	if snap.System != nil {
		hostStr = fmt.Sprintf(" Host: %s | OS: %s (%s %s) | Uptime: %s | Boot: %s",
			BoldStyle.Render(snap.System.Hostname),
			snap.System.OS, snap.System.Platform, snap.System.PlatformVersion,
			SuccessStyle.Render(FormatDuration(snap.System.Uptime)),
			snap.System.BootTime.Format("2006-01-02 15:04:05"),
		)
	} else {
		hostStr = " Host Information Unavailable"
	}
	hostBar := CardStyle.Width(m.width - 4).Render(hostStr)

	// Available width calculation
	cardWidth := (m.width - 8) / 2
	if cardWidth < 36 {
		cardWidth = 36
	}

	// 1. CPU Card
	var cpuContent string
	if snap.CPU != nil {
		bar := RenderProgressBar(snap.CPU.OverallUsage, cardWidth-16)
		spark := RenderSparkline(m.cpuHistory, 100.0)
		cpuContent = fmt.Sprintf(
			"%s  %5.1f%%\n\nHistory : %s\nCores   : %d logical\nLoad Avg: %.2f / %.2f / %.2f",
			bar, snap.CPU.OverallUsage,
			InfoStyle.Render(spark),
			snap.CPU.LogicalCores,
			snap.CPU.LoadAverage.Load1, snap.CPU.LoadAverage.Load5, snap.CPU.LoadAverage.Load15,
		)
	} else {
		cpuContent = "CPU data unavailable"
	}
	cpuCard := CardStyle.Width(cardWidth).Render(
		lipgloss.JoinVertical(lipgloss.Left,
			SubTitleStyle.Render("⚡ CPU Utilization"),
			"",
			cpuContent,
		),
	)

	// 2. Memory Card
	var memContent string
	if snap.Memory != nil {
		bar := RenderProgressBar(snap.Memory.UsedPercent, cardWidth-16)
		spark := RenderSparkline(m.memHistory, 100.0)
		swapStr := "None"
		if snap.Memory.SwapTotalBytes > 0 {
			swapStr = fmt.Sprintf("%s / %s (%.1f%%)",
				FormatBytes(snap.Memory.SwapUsedBytes),
				FormatBytes(snap.Memory.SwapTotalBytes),
				snap.Memory.SwapUsedPercent)
		}
		memContent = fmt.Sprintf(
			"%s  %5.1f%%\n\nHistory : %s\nUsed/Tot: %s / %s\nAvail   : %s\nSwap    : %s",
			bar, snap.Memory.UsedPercent,
			InfoStyle.Render(spark),
			FormatBytes(snap.Memory.UsedBytes), FormatBytes(snap.Memory.TotalBytes),
			SuccessStyle.Render(FormatBytes(snap.Memory.AvailableBytes)),
			swapStr,
		)
	} else {
		memContent = "Memory data unavailable"
	}
	memCard := CardStyle.Width(cardWidth).Render(
		lipgloss.JoinVertical(lipgloss.Left,
			SubTitleStyle.Render("🧠 Memory Utilization"),
			"",
			memContent,
		),
	)

	// 3. Disk Card
	var diskContent string
	if snap.Disk != nil {
		bar := RenderProgressBar(snap.Disk.UsedPercent, cardWidth-16)
		var totalRead, totalWrite float64
		for _, io := range snap.Disk.IOCounters {
			totalRead += io.ReadRate
			totalWrite += io.WriteRate
		}
		diskContent = fmt.Sprintf(
			"%s  %5.1f%%\n\nUsed/Tot: %s / %s\nFree    : %s\nRead I/O: %s\nWrite I/O: %s",
			bar, snap.Disk.UsedPercent,
			FormatBytes(snap.Disk.UsedBytes), FormatBytes(snap.Disk.TotalBytes),
			SuccessStyle.Render(FormatBytes(snap.Disk.FreeBytes)),
			InfoStyle.Render(FormatRate(totalRead)),
			WarningStyle.Render(FormatRate(totalWrite)),
		)
	} else {
		diskContent = "Disk data unavailable"
	}
	diskCard := CardStyle.Width(cardWidth).Render(
		lipgloss.JoinVertical(lipgloss.Left,
			SubTitleStyle.Render("💾 Storage & Disk I/O"),
			"",
			diskContent,
		),
	)

	// 4. Network Card
	var netContent string
	if snap.Network != nil {
		rxSpark := RenderSparkline(m.netRxHist, 0)
		txSpark := RenderSparkline(m.netTxHist, 0)
		netContent = fmt.Sprintf(
			"Rx Rate : %s  %s\nTx Rate : %s  %s\n\nTotal Rx: %s\nTotal Tx: %s\nActive  : %d interfaces",
			InfoStyle.Render(fmt.Sprintf("%-12s", FormatRate(snap.Network.TotalRxRate))),
			InfoStyle.Render(rxSpark),
			WarningStyle.Render(fmt.Sprintf("%-12s", FormatRate(snap.Network.TotalTxRate))),
			WarningStyle.Render(txSpark),
			FormatBytes(snap.Network.TotalBytesRecv),
			FormatBytes(snap.Network.TotalBytesSent),
			len(snap.Network.Interfaces),
		)
	} else {
		netContent = "Network data unavailable"
	}
	netCard := CardStyle.Width(cardWidth).Render(
		lipgloss.JoinVertical(lipgloss.Left,
			SubTitleStyle.Render("🌐 Network Traffic"),
			"",
			netContent,
		),
	)

	// Quick Health Summary Banner
	var healthSummary string
	if m.lastDiagReport != nil {
		diag := m.lastDiagReport
		healthSummary = fmt.Sprintf("Health Status: %s | Passed: %s | Warnings: %s | Critical: %s | Active Alerts: %s",
			StatusBadge(diag.OverallStatus),
			SuccessStyle.Render(fmt.Sprintf("%d", diag.PassedChecks)),
			WarningStyle.Render(fmt.Sprintf("%d", diag.WarningChecks)),
			DangerStyle.Render(fmt.Sprintf("%d", diag.CriticalChecks)),
			WarningStyle.Render(fmt.Sprintf("%d", len(m.activeAlerts))),
		)
	} else {
		healthSummary = "Diagnostics running..."
	}
	healthCard := CardStyle.Width(m.width - 4).Render(healthSummary)

	// Mini Process Preview
	var topProcsSb strings.Builder
	topProcsSb.WriteString(SubTitleStyle.Render("🔥 Top Processes (CPU / Memory)") + "\n")
	topProcsSb.WriteString(MutedStyle.Render(fmt.Sprintf("  %-8s %-20s %-8s %-8s %-12s %s", "PID", "NAME", "CPU %", "MEM %", "RSS", "COMMAND")) + "\n")

	if len(m.filteredProcs) > 0 {
		limit := 4
		if len(m.filteredProcs) < limit {
			limit = len(m.filteredProcs)
		}
		for i := 0; i < limit; i++ {
			p := m.filteredProcs[i]
			cmd := p.CommandLine
			if len(cmd) > 30 {
				cmd = cmd[:27] + "..."
			}
			topProcsSb.WriteString(fmt.Sprintf("  %-8d %-20s %6.1f%% %6.1f%% %-12s %s\n",
				p.PID, TruncateString(p.Name, 18), p.CPUPercent, p.MemoryPercent, FormatBytes(p.MemoryRSS), cmd))
		}
	} else {
		topProcsSb.WriteString("  No process data available\n")
	}
	procCard := CardStyle.Width(m.width - 4).Render(topProcsSb.String())

	// Grid layout
	row1 := lipgloss.JoinHorizontal(lipgloss.Top, cpuCard, memCard)
	row2 := lipgloss.JoinHorizontal(lipgloss.Top, diskCard, netCard)

	return lipgloss.JoinVertical(lipgloss.Left,
		hostBar,
		"",
		row1,
		"",
		row2,
		"",
		healthCard,
		"",
		procCard,
	)
}
