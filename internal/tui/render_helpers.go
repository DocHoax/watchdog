package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/DocHoax/watchdog/pkg/model"
	"github.com/charmbracelet/lipgloss"
)

// RenderProgressBar renders an ASCII/Unicode progress bar with custom colors.
func RenderProgressBar(pct float64, width int) string {
	if width <= 2 {
		width = 10
	}
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}

	innerBarWidth := width - 2
	filled := int((pct / 100.0) * float64(innerBarWidth))
	if filled > innerBarWidth {
		filled = innerBarWidth
	}
	empty := innerBarWidth - filled

	var barColor lipgloss.Color
	if pct >= 85.0 {
		barColor = ColorDanger
	} else if pct >= 70.0 {
		barColor = ColorWarning
	} else {
		barColor = ColorSuccess
	}

	filledStr := lipgloss.NewStyle().Foreground(barColor).Render(strings.Repeat("█", filled))
	emptyStr := lipgloss.NewStyle().Foreground(ColorBorder).Render(strings.Repeat("░", empty))

	return fmt.Sprintf("[%s%s]", filledStr, emptyStr)
}

// RenderSparkline creates a mini unicode sparkline string from a slice of floats.
func RenderSparkline(values []float64, maxVal float64) string {
	if len(values) == 0 {
		return ""
	}
	sparks := []rune{' ', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

	if maxVal <= 0 {
		for _, v := range values {
			if v > maxVal {
				maxVal = v
			}
		}
	}
	if maxVal <= 0 {
		maxVal = 1.0
	}

	var sb strings.Builder
	for _, v := range values {
		idx := int((v / maxVal) * float64(len(sparks)-1))
		if idx < 0 {
			idx = 0
		}
		if idx >= len(sparks) {
			idx = len(sparks) - 1
		}
		sb.WriteRune(sparks[idx])
	}
	return sb.String()
}

// FormatBytes formats byte counts into human-readable strings.
func FormatBytes(b uint64) string {
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

// FormatRate formats bytes/sec into readable rate strings.
func FormatRate(r float64) string {
	return FormatBytes(uint64(r)) + "/s"
}

// FormatDuration formats duration into readable string.
func FormatDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60
	secs := int(d.Seconds()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, mins)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm %ds", hours, mins, secs)
	}
	if mins > 0 {
		return fmt.Sprintf("%dm %ds", mins, secs)
	}
	return fmt.Sprintf("%ds", secs)
}

// TruncateString truncates strings exceeding maxLen.
func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// StatusBadge returns a formatted status pill.
func StatusBadge(status model.DiagnosticStatus) string {
	switch status {
	case model.StatusPass:
		return PassBadge.Render(" PASS ")
	case model.StatusWarning:
		return WarnBadge.Render(" WARN ")
	case model.StatusFail:
		return CritBadge.Render(" CRIT ")
	default:
		return MutedStyle.Render(string(status))
	}
}

// SeverityBadge returns a formatted severity pill.
func SeverityBadge(sev model.Severity) string {
	switch sev {
	case model.SeverityCritical:
		return CritBadge.Render(" CRIT ")
	case model.SeverityWarning:
		return WarnBadge.Render(" WARN ")
	case model.SeverityInfo:
		return PassBadge.Render(" INFO ")
	default:
		return MutedStyle.Render(string(sev))
	}
}
