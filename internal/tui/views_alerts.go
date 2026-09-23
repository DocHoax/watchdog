package tui

import (
	"fmt"
	"strings"
)

func (m Model) renderAlertsView() string {
	var sb strings.Builder

	// Top Active Alerts Card
	sb.WriteString(SubTitleStyle.Render("🚨 Active System Incidents & Alerts") + "\n")
	if len(m.activeAlerts) == 0 {
		sb.WriteString(CardStyle.Width(m.width - 4).Render(
			SuccessStyle.Render("✔ No active alert conditions detected. All metrics within nominal thresholds."),
		) + "\n\n")
	} else {
		var alertsSb strings.Builder
		alertsSb.WriteString(MutedStyle.Render(fmt.Sprintf("  %-8s %-20s %-16s %-10s %-10s %s", "SEV", "RULE NAME", "METRIC", "VAL", "THRESH", "MESSAGE")) + "\n")
		alertsSb.WriteString("  " + strings.Repeat("-", m.width-12) + "\n")

		for _, a := range m.activeAlerts {
			badge := SeverityBadge(a.Severity)
			alertsSb.WriteString(fmt.Sprintf("  %s %-20s %-16s %8.1f %8.1f %s (Fired: %s)\n",
				badge,
				TruncateString(a.RuleName, 18),
				TruncateString(a.MetricName, 14),
				a.ActualValue,
				a.Threshold,
				TruncateString(a.Message, m.width-85),
				a.FiredAt.Format("15:04:05"),
			))
		}
		sb.WriteString(CardStyle.Width(m.width - 4).Render(alertsSb.String()) + "\n\n")
	}

	// Statistical Anomaly Detection Section
	sb.WriteString(SubTitleStyle.Render("📈 Statistical Anomaly Detector (EWMA & Z-Score)") + "\n")
	if m.anomalyReport == nil || len(m.anomalyReport.Scores) == 0 {
		sb.WriteString(CardStyle.Width(m.width - 4).Render(
			MutedStyle.Render("Gathering baseline statistical samples for time-series anomaly detection..."),
		))
	} else {
		var anomSb strings.Builder
		anomSb.WriteString(MutedStyle.Render(fmt.Sprintf("  %-8s %-18s %-10s %-10s %-10s %-8s %s",
			"SEV", "METRIC", "CURRENT", "BASELINE", "STDDEV", "Z-SCORE", "EXPLANATION")) + "\n")
		anomSb.WriteString("  " + strings.Repeat("-", m.width-12) + "\n")

		hasAnomalies := false
		for _, score := range m.anomalyReport.Scores {
			if !score.IsAnomaly {
				continue
			}
			hasAnomalies = true
			badge := SeverityBadge(score.Severity)
			anomSb.WriteString(fmt.Sprintf("  %s %-18s %10.2f %10.2f %10.2f %+7.2fσ %s\n",
				badge,
				TruncateString(score.MetricName, 16),
				score.CurrentValue,
				score.Mean,
				score.StdDev,
				score.ZScore,
				TruncateString(score.Explanation, m.width-80),
			))
		}

		if !hasAnomalies {
			anomSb.WriteString(SuccessStyle.Render("  ✔ No statistical deviations exceeding anomaly thresholds (Z-score nominal).") + "\n")
		}

		sb.WriteString(CardStyle.Width(m.width - 4).Render(anomSb.String()))
	}

	return sb.String()
}
