package tui

import (
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Initializing Watchdog TUI..."
	}

	if m.showHelp {
		return lipgloss.Place(
			m.width,
			m.height,
			lipgloss.Center,
			lipgloss.Center,
			m.renderHelpModal(),
		)
	}

	var sb strings.Builder

	// 1. Header & Navigation Tabs
	var tabs []string
	for i, name := range TabNames {
		if Tab(i) == m.activeTab {
			tabs = append(tabs, ActiveTabStyle.Render(name))
		} else {
			tabs = append(tabs, TabStyle.Render(name))
		}
	}

	tabRow := lipgloss.JoinHorizontal(lipgloss.Top, tabs...)
	titleBadge := TitleStyle.Render(" 🐺 WATCHDOG ")
	headerContent := lipgloss.JoinHorizontal(lipgloss.Center, titleBadge, "  ", tabRow)
	sb.WriteString(HeaderStyle.Width(m.width).Render(headerContent) + "\n")

	// 2. Main Content View according to activeTab
	var content string
	switch m.activeTab {
	case TabDashboard:
		content = m.renderDashboardView()
	case TabProcesses:
		content = m.renderProcessesView()
	case TabDiagnostics:
		content = m.renderDiagnosticsView()
	case TabAlerts:
		content = m.renderAlertsView()
	case TabNetwork:
		content = m.renderNetworkView()
	case TabContainers:
		content = m.renderContainersView()
	default:
		content = "Unknown tab view."
	}

	sb.WriteString(content + "\n")

	// 3. Footer Bar
	footerLeft := " [Tab] Switch Tab | [?] Help | [r] Refresh | [q] Quit"
	if m.activeTab == TabProcesses {
		footerLeft = " [Tab] Tabs | [/] Filter | [c] CPU | [m] Mem | [p] PID | [k] Kill | [?] Help | [q] Quit"
	}

	// Status toast message
	footerCenter := ""
	if m.statusMessage != "" && time.Now().Before(m.statusExpiry) {
		footerCenter = WarningStyle.Render(" ℹ " + m.statusMessage)
	}

	timestamp := time.Now().Format("15:04:05 MST")
	footerRight := MutedStyle.Render(timestamp)

	// Combine footer elements
	availableForLeft := m.width - lipgloss.Width(footerRight) - lipgloss.Width(footerCenter) - 6
	if availableForLeft < 10 {
		availableForLeft = 10
	}

	footerRow := lipgloss.JoinHorizontal(lipgloss.Top,
		MutedStyle.Render(TruncateString(footerLeft, availableForLeft)),
		" ",
		footerCenter,
		" ",
		footerRight,
	)

	sb.WriteString(FooterStyle.Width(m.width).Render(footerRow))

	return sb.String()
}
