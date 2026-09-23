package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/watchdog-cli/watchdog/pkg/model"
)

func (m Model) renderProcessesView() string {
	var sb strings.Builder

	// Top controls / filter bar
	sortName := "CPU % (c)"
	switch m.processSort {
	case SortMemory:
		sortName = "Memory % (m)"
	case SortPID:
		sortName = "PID (p)"
	case SortName:
		sortName = "Name (n)"
	}

	totalCount := 0
	if m.currentSnapshot != nil && m.currentSnapshot.Processes != nil {
		totalCount = m.currentSnapshot.Processes.TotalCount
	}

	filterBar := fmt.Sprintf(" Filter [/]: %s | Sort: %s | Total: %d | Matched: %d | Keys: [c]pu [m]em [p]id [n]ame [k]ill",
		m.processFilter.View(),
		BoldStyle.Render(sortName),
		totalCount,
		len(m.filteredProcs),
	)
	sb.WriteString(CardStyle.Width(m.width - 4).Render(filterBar) + "\n\n")

	// Table Header
	header := fmt.Sprintf("  %-8s %-8s %-12s %-20s %-8s %-8s %-12s %-8s %s",
		"PID", "PPID", "USER", "NAME", "CPU %", "MEM %", "RSS", "THREADS", "COMMAND")
	sb.WriteString(TableHeaderStyle.Width(m.width - 4).Render(header) + "\n")

	// Table Rows
	if len(m.filteredProcs) == 0 {
		sb.WriteString(lipgloss.NewStyle().Padding(2, 4).Render("No processes matching filter criteria."))
	} else {
		maxVisibleRows := m.height - 18
		if maxVisibleRows < 5 {
			maxVisibleRows = 5
		}

		endIdx := m.procScrollOff + maxVisibleRows
		if endIdx > len(m.filteredProcs) {
			endIdx = len(m.filteredProcs)
		}

		for i := m.procScrollOff; i < endIdx; i++ {
			p := m.filteredProcs[i]
			cmd := p.CommandLine
			if cmd == "" {
				cmd = p.Name
			}
			cmdMaxLen := m.width - 96
			if cmdMaxLen < 15 {
				cmdMaxLen = 15
			}
			cmd = TruncateString(cmd, cmdMaxLen)

			rowStr := fmt.Sprintf("  %-8d %-8d %-12s %-20s %6.1f%% %6.1f%% %-12s %-8d %s",
				p.PID, p.PPID, TruncateString(p.Username, 11), TruncateString(p.Name, 18),
				p.CPUPercent, p.MemoryPercent, FormatBytes(p.MemoryRSS), p.NumThreads, cmd)

			if i == m.selectedProcIdx {
				sb.WriteString(TableRowSelectedStyle.Width(m.width - 4).Render(rowStr) + "\n")
			} else {
				sb.WriteString(rowStr + "\n")
			}
		}
	}

	// Bottom Selected Process Inspection Card
	if len(m.filteredProcs) > 0 && m.selectedProcIdx < len(m.filteredProcs) {
		selected := m.filteredProcs[m.selectedProcIdx]
		details := m.renderProcessDetails(selected)
		sb.WriteString("\n" + CardStyle.Width(m.width-4).Render(details))
	}

	return sb.String()
}

func (m Model) renderProcessDetails(p model.ProcessInfo) string {
	var sb strings.Builder
	sb.WriteString(SubTitleStyle.Render(fmt.Sprintf("🔍 Process Details: %s (PID: %d, PPID: %d)", p.Name, p.PID, p.PPID)) + "\n")
	sb.WriteString(fmt.Sprintf("  User: %-12s | Status: %-10s | CPU: %5.1f%% | Memory: %5.1f%% (RSS: %s, VMS: %s)\n",
		p.Username, p.Status, p.CPUPercent, p.MemoryPercent, FormatBytes(p.MemoryRSS), FormatBytes(p.MemoryVMS)))
	sb.WriteString(fmt.Sprintf("  Threads: %-8d | Nice: %-4d | Started: %s | I/O: Read %s/s, Write %s/s\n",
		p.NumThreads, p.Nice, p.CreateTime.Format("2006-01-02 15:04:05"),
		FormatBytes(uint64(p.ReadBytesSec)), FormatBytes(uint64(p.WriteBytesSec))))
	if p.CommandLine != "" {
		sb.WriteString(fmt.Sprintf("  Command: %s\n", TruncateString(p.CommandLine, m.width-16)))
	}
	if p.ExePath != "" {
		sb.WriteString(fmt.Sprintf("  Binary : %s\n", TruncateString(p.ExePath, m.width-16)))
	}
	return sb.String()
}
