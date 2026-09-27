package incidents

import (
	"testing"
	"time"

	"github.com/DocHoax/watchdog/pkg/model"
)

func TestSignalCorrelator_CorrelateSignals(t *testing.T) {
	correlator := NewSignalCorrelator()
	now := time.Now().UTC()

	alerts := []model.AlertEvent{
		{
			ID:          "alt-1",
			RuleID:      "rule-cpu",
			RuleName:    "HighCPUUsage",
			MetricName:  "cpu_usage_pct",
			Severity:    model.SeverityCritical,
			ActualValue: 98.5,
			Threshold:   90.0,
			Message:     "CPU sustained over 90%",
			FiredAt:     now,
			IsActive:    true,
		},
		{
			ID:       "alt-inactive",
			RuleName: "InactiveAlert",
			IsActive: false, // Inactive alerts must be skipped
		},
	}

	diag := &model.DiagnosticReport{
		GeneratedAt: now,
		Results: []model.DiagnosticResult{
			{
				Name:        "CheckDiskSpace",
				Category:    "storage",
				Status:      model.StatusFail,
				Severity:    model.SeverityCritical,
				Description: "Root filesystem partition usage is 96%",
				Timestamp:   now,
			},
			{
				Name:     "CheckDNS",
				Category: "network",
				Status:   model.StatusPass, // Passing checks must be skipped
			},
		},
	}

	anom := &model.AnomalyReport{
		GeneratedAt: now,
		Scores: []model.AnomalyScore{
			{
				MetricName:   "net_tx_errors",
				IsAnomaly:    true,
				Severity:     model.SeverityWarning,
				CurrentValue: 150.0,
				Mean:         5.0,
				StdDev:       2.0,
				ZScore:       72.5,
				DeviationPct: 2900.0,
				Explanation:  "Unprecedented error spike in network transmission",
				DetectedAt:   now,
			},
			{
				MetricName: "mem_used_pct",
				IsAnomaly:  false, // Non-anomalies must be skipped
			},
		},
	}

	timeToCrossing := 30 * time.Minute
	preds := []Prediction{
		{
			ID:                       "pred-1",
			NodeID:                   "node-1",
			Metric:                   "disk_used_percent",
			CurrentValue:             88.0,
			TargetThreshold:          95.0,
			Direction:                PredictionDirectionApproaching,
			SlopePerMinute:           0.2,
			RSquared:                 0.95,
			Confidence:               PredictionConfidenceHigh,
			EstimatedTimeToThreshold: &timeToCrossing,
			GeneratedAt:              now,
		},
	}

	signals := correlator.CorrelateSignals("node-1", "srv-01", alerts, diag, anom, preds)

	if len(signals) != 4 {
		t.Fatalf("expected 4 signals, got %d", len(signals))
	}

	// Verify alert signal
	sigAlert := signals[0]
	if sigAlert.Type != SignalTypeAlert || sigAlert.Source != "HighCPUUsage" || sigAlert.Severity != model.SeverityCritical {
		t.Errorf("unexpected alert signal: %+v", sigAlert)
	}

	// Verify diagnostic signal
	sigDiag := signals[1]
	if sigDiag.Type != SignalTypeDiagnostic || sigDiag.Source != "CheckDiskSpace" || sigDiag.Severity != model.SeverityCritical {
		t.Errorf("unexpected diag signal: %+v", sigDiag)
	}

	// Verify anomaly signal
	sigAnom := signals[2]
	if sigAnom.Type != SignalTypeAnomaly || sigAnom.Source != "net_tx_errors" || sigAnom.Severity != model.SeverityWarning {
		t.Errorf("unexpected anomaly signal: %+v", sigAnom)
	}

	// Verify prediction signal (timeToThreshold < 1h elevates to Critical)
	sigPred := signals[3]
	if sigPred.Type != SignalTypePrediction || sigPred.Source != "disk_used_percent" || sigPred.Severity != model.SeverityCritical {
		t.Errorf("unexpected prediction signal: %+v", sigPred)
	}
}
