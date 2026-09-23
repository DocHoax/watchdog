package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/watchdog-cli/watchdog/pkg/model"
)

func (m Model) renderDiagnosticsView() string {
	if m.lastDiagReport == nil {
		if m.diagRunning {
			return lipgloss.NewStyle().Padding(2, 4).Render("Automated diagnostics scan in progress...")
		}
		return lipgloss.NewStyle().Padding(2, 4).Render("No diagnostic report available. Press 'r' to run scan.")
	}

	diag := m.lastDiagReport
	var sb strings.Builder

	// Top Summary Card
	cardWidth := (m.width - 12) / 4
	if cardWidth < 18 {
		cardWidth = 18
	}

	statusCard := CardStyle.Width(cardWidth).Render(
		lipgloss.JoinVertical(lipgloss.Left,
			MutedStyle.Render("Overall Status"),
			StatusBadge(diag.OverallStatus),
		),
	)
	passedCard := CardStyle.Width(cardWidth).Render(
		lipgloss.JoinVertical(lipgloss.Left,
			MutedStyle.Render("Passed Checks"),
			SuccessStyle.Render(fmt.Sprintf("%d", diag.PassedChecks)),
		),
	)
	warningCard := CardStyle.Width(cardWidth).Render(
		lipgloss.JoinVertical(lipgloss.Left,
			MutedStyle.Render("Warnings"),
			WarningStyle.Render(fmt.Sprintf("%d", diag.WarningChecks)),
		),
	)
	criticalCard := CardStyle.Width(cardWidth).Render(
		lipgloss.JoinVertical(lipgloss.Left,
			MutedStyle.Render("Critical"),
			DangerStyle.Render(fmt.Sprintf("%d", diag.CriticalChecks)),
		),
	)

	summaryRow := lipgloss.JoinHorizontal(lipgloss.Top, statusCard, passedCard, warningCard, criticalCard)
	sb.WriteString(summaryRow + "\n\n")

	// Table Header
	header := fmt.Sprintf("  %-8s %-16s %-26s %s", "STATUS", "CATEGORY", "CHECK NAME", "DESCRIPTION")
	sb.WriteString(TableHeaderStyle.Width(m.width - 4).Render(header) + "\n")

	// Diagnostic Check Rows
	for i, check := range diag.Results {
		badge := StatusBadge(check.Status)
		rowStr := fmt.Sprintf("  %s %-16s %-26s %s",
			badge,
			TruncateString(check.Category, 14),
			TruncateString(check.Name, 24),
			TruncateString(check.Description, m.width-60),
		)

		if i == m.selectedDiagIdx {
			sb.WriteString(TableRowSelectedStyle.Width(m.width - 4).Render(rowStr) + "\n")
		} else {
			sb.WriteString(rowStr + "\n")
		}
	}

	// Bottom Selected Diagnostic Item Inspector
	if len(diag.Results) > 0 && m.selectedDiagIdx < len(diag.Results) {
		selected := diag.Results[m.selectedDiagIdx]
		inspector := m.renderDiagnosticDetail(selected)
		sb.WriteString("\n" + CardStyle.Width(m.width-4).Render(inspector))
	}

	return sb.String()
}

func (m Model) renderDiagnosticDetail(check model.DiagnosticResult) string {
	var sb strings.Builder
	sb.WriteString(SubTitleStyle.Render(fmt.Sprintf("🔍 Diagnostic Check: %s [%s]", check.Name, check.Category)) + "\n")
	sb.WriteString(fmt.Sprintf("  Status     : %s\n", StatusBadge(check.Status)))
	sb.WriteString(fmt.Sprintf("  Description: %s\n", check.Description))
	if check.Recommendation != "" {
		sb.WriteString(fmt.Sprintf("  %sAction/Remediation:%s %s\n",
			WarningStyle.Render(""),
			BoldStyle.Render(""),
			SuccessStyle.Render(check.Recommendation)))
	}
	return sb.String()
}
