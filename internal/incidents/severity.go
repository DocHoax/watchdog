package incidents

import (
	"fmt"
	"math"

	"github.com/DocHoax/watchdog/pkg/model"
)

// CalculateSeverity computes a deterministic 0-100 composite score, severity classification, and explainable breakdown.
func CalculateSeverity(signals []IncidentSignal, affectedNodesCount, totalFleetNodes int, recurrenceCount int) (model.Severity, float64, SeverityExplanation) {
	var factors []SeverityFactorContribution
	var reasoning []string
	var totalScore float64

	var hasCriticalAlert bool
	var hasCriticalDiag bool
	var hasWarningAlert bool
	var hasWarningDiag bool
	var alertCount int
	var diagCount int
	var anomalyCount int
	var capacityRiskCount int

	for _, sig := range signals {
		switch sig.Type {
		case SignalTypeAlert:
			alertCount++
			switch sig.Severity {
			case model.SeverityCritical:
				hasCriticalAlert = true
			case model.SeverityWarning:
				hasWarningAlert = true
			}
		case SignalTypeDiagnostic:
			diagCount++
			switch sig.Severity {
			case model.SeverityCritical:
				hasCriticalDiag = true
			case model.SeverityWarning:
				hasWarningDiag = true
			}
		case SignalTypeAnomaly:
			anomalyCount++
		case SignalTypeCapacityRisk, SignalTypePrediction:
			capacityRiskCount++
		}
	}

	// 1. Alert Severity Factor
	if hasCriticalAlert {
		pts := 35.0
		totalScore += pts
		factors = append(factors, SeverityFactorContribution{
			Name:        "Critical Alerts Firing",
			Category:    "alert_severity",
			Weight:      0.35,
			Points:      pts,
			Description: fmt.Sprintf("%d active alert(s) with CRITICAL severity detected", alertCount),
		})
		reasoning = append(reasoning, "Critical alert is currently firing across affected infrastructure")
	} else if hasWarningAlert {
		pts := 15.0
		totalScore += pts
		factors = append(factors, SeverityFactorContribution{
			Name:        "Warning Alerts Firing",
			Category:    "alert_severity",
			Weight:      0.15,
			Points:      pts,
			Description: fmt.Sprintf("%d active alert(s) with WARNING severity detected", alertCount),
		})
		reasoning = append(reasoning, "Warning alert is firing across affected infrastructure")
	}

	// Extra points for multiple alerts
	if alertCount > 1 {
		extra := math.Min(float64(alertCount-1)*5.0, 15.0)
		totalScore += extra
		factors = append(factors, SeverityFactorContribution{
			Name:        "Multi-Alert Cascade",
			Category:    "alert_volume",
			Weight:      0.15,
			Points:      extra,
			Description: fmt.Sprintf("Multiple alerts (%d) co-occurring simultaneously", alertCount),
		})
	}

	// 2. Diagnostic Check Failures
	if hasCriticalDiag {
		pts := 30.0
		totalScore += pts
		factors = append(factors, SeverityFactorContribution{
			Name:        "Critical Diagnostic Failures",
			Category:    "diagnostic_failure",
			Weight:      0.30,
			Points:      pts,
			Description: fmt.Sprintf("%d diagnostic check(s) flagged CRITICAL status", diagCount),
		})
		reasoning = append(reasoning, "Critical system diagnostic rules failed")
	} else if hasWarningDiag {
		pts := 15.0
		totalScore += pts
		factors = append(factors, SeverityFactorContribution{
			Name:        "Warning Diagnostic Failures",
			Category:    "diagnostic_failure",
			Weight:      0.15,
			Points:      pts,
			Description: fmt.Sprintf("%d diagnostic check(s) flagged WARNING status", diagCount),
		})
		reasoning = append(reasoning, "Diagnostic checks identified system degradation")
	}

	// 3. Statistical Anomalies
	if anomalyCount > 0 {
		pts := math.Min(float64(anomalyCount)*10.0, 20.0)
		totalScore += pts
		factors = append(factors, SeverityFactorContribution{
			Name:        "Telemetry Metric Anomalies",
			Category:    "anomaly",
			Weight:      0.20,
			Points:      pts,
			Description: fmt.Sprintf("%d metric anomalies detected deviating from baselines", anomalyCount),
		})
		reasoning = append(reasoning, fmt.Sprintf("%d statistical metric anomalies detected", anomalyCount))
	}

	// 4. Blast Radius / Node Coverage
	if totalFleetNodes > 0 {
		pct := (float64(affectedNodesCount) / float64(totalFleetNodes)) * 100.0
		if pct >= 50.0 {
			pts := 30.0
			totalScore += pts
			factors = append(factors, SeverityFactorContribution{
				Name:        "High Fleet Blast Radius",
				Category:    "blast_radius",
				Weight:      0.30,
				Points:      pts,
				Description: fmt.Sprintf("Incident impacts %.1f%% of fleet (%d/%d nodes)", pct, affectedNodesCount, totalFleetNodes),
			})
			reasoning = append(reasoning, "High blast radius: over 50% of the fleet is affected")
		} else if pct >= 20.0 || affectedNodesCount > 3 {
			pts := 20.0
			totalScore += pts
			factors = append(factors, SeverityFactorContribution{
				Name:        "Multi-Node Cluster Impact",
				Category:    "blast_radius",
				Weight:      0.20,
				Points:      pts,
				Description: fmt.Sprintf("Incident impacts %d nodes (%.1f%% of fleet)", affectedNodesCount, pct),
			})
			reasoning = append(reasoning, "Multi-node cluster impact observed")
		} else if affectedNodesCount > 1 {
			pts := 10.0
			totalScore += pts
			factors = append(factors, SeverityFactorContribution{
				Name:        "Multi-Node Impact",
				Category:    "blast_radius",
				Weight:      0.10,
				Points:      pts,
				Description: fmt.Sprintf("Incident impacts %d nodes", affectedNodesCount),
			})
		}
	}

	// 5. Capacity Risks / Predictions
	if capacityRiskCount > 0 {
		pts := math.Min(float64(capacityRiskCount)*10.0, 20.0)
		totalScore += pts
		factors = append(factors, SeverityFactorContribution{
			Name:        "Capacity Exhaustion Forecast",
			Category:    "capacity_risk",
			Weight:      0.20,
			Points:      pts,
			Description: fmt.Sprintf("%d capacity threshold exhaustion risks projected", capacityRiskCount),
		})
		reasoning = append(reasoning, "Resource capacity exhaustion projected within observation horizon")
	}

	// 6. Recurrence / Flapping
	if recurrenceCount >= 5 {
		pts := 15.0
		totalScore += pts
		factors = append(factors, SeverityFactorContribution{
			Name:        "Frequent Incident Recurrence",
			Category:    "recurrence_pattern",
			Weight:      0.15,
			Points:      pts,
			Description: fmt.Sprintf("Observed %d historical occurrences (flapping pattern)", recurrenceCount),
		})
		reasoning = append(reasoning, "Frequent recurring operational pattern detected across historical lookback")
	} else if recurrenceCount >= 3 {
		pts := 10.0
		totalScore += pts
		factors = append(factors, SeverityFactorContribution{
			Name:        "Periodic Incident Recurrence",
			Category:    "recurrence_pattern",
			Weight:      0.10,
			Points:      pts,
			Description: fmt.Sprintf("Observed %d historical occurrences", recurrenceCount),
		})
	}

	// Clamp total score
	if totalScore > 100.0 {
		totalScore = 100.0
	}
	totalScore = math.Round(totalScore*10) / 10

	// Determine classification
	var sev model.Severity
	if hasCriticalAlert || hasCriticalDiag || totalScore >= 60.0 {
		sev = model.SeverityCritical
	} else if hasWarningAlert || hasWarningDiag || totalScore >= 25.0 {
		sev = model.SeverityWarning
	} else {
		sev = model.SeverityInfo
	}

	// Determine confidence
	var confidence string
	signalCount := len(signals)
	if signalCount >= 3 || (hasCriticalAlert && hasCriticalDiag) || totalScore >= 50.0 {
		confidence = "high"
	} else if signalCount >= 2 || totalScore >= 25.0 {
		confidence = "medium"
	} else {
		confidence = "low"
	}

	explanation := SeverityExplanation{
		CalculatedSeverity: sev,
		BaseScore:          totalScore,
		Confidence:         confidence,
		Factors:            factors,
		Reasoning:          reasoning,
	}

	return sev, totalScore, explanation
}
