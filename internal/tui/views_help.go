package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) renderHelpModal() string {
	var sb strings.Builder

	sb.WriteString(TitleStyle.Render(" WATCHDOG - INTERACTIVE KEYBOARD SHORTCUTS ") + "\n\n")

	shortcuts := [][]string{
		{"Tab / Shift+Tab", "Cycle through tabs forwards / backwards"},
		{"1 - 6", "Jump directly to tab (1:Dash, 2:Proc, 3:Diag, 4:Alert, 5:Net, 6:Cont)"},
		{"Up / Down (k/j)", "Navigate items / scroll tables"},
		{"/", "Filter / search processes by name, command line, or PID"},
		{"c", "Sort processes by CPU % descending"},
		{"m", "Sort processes by Memory % descending"},
		{"p", "Sort processes by PID ascending"},
		{"n", "Sort processes by Name"},
		{"k", "Kill / terminate selected process (prompts confirmation)"},
		{"r", "Force immediate snapshot collection and diagnostic re-scan"},
		{"?", "Toggle this Help overlay modal"},
		{"q / Ctrl+C", "Exit Watchdog TUI"},
	}

	for _, sc := range shortcuts {
		keyStr := BoldStyle.Render(fmtKey(sc[0], 20))
		descStr := sc[1]
		sb.WriteString("  " + keyStr + " : " + descStr + "\n")
	}

	sb.WriteString("\n" + MutedStyle.Render("  Press '?' or 'Esc' to close this help dialog."))

	modalWidth := 72
	if modalWidth > m.width-4 {
		modalWidth = m.width - 4
	}

	modal := lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(ColorPrimary).
		Background(ColorCardBg).
		Padding(1, 2).
		Width(modalWidth).
		Render(sb.String())

	return modal
}

func fmtKey(k string, width int) string {
	if len(k) < width {
		return k + strings.Repeat(" ", width-len(k))
	}
	return k
}
