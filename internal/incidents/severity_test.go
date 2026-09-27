package incidents

import (
	"testing"
	"time"

	"github.com/DocHoax/watchdog/pkg/model"
)

func TestCalculateSeverity_ClassificationAndPoints(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name               string
		signals            []IncidentSignal
		affectedNodesCount int
		totalFleetNodes    int
		recurrenceCount    int
		wantSeverity       model.Severity
		wantMinScore       float64
		wantMaxScore       float64
		wantConfidence     string
	}{
		{
			name: "Single Critical Alert with Low Node Impact",
			signals: []IncidentSignal{
				{
					ID:        "sig-1",
					Type:      SignalTypeAlert,
					Source:    "HighCPU",
					Severity:  model.SeverityCritical,
					Timestamp: now,
				},
			},
			affectedNodesCount: 1,
			totalFleetNodes:    10,
			recurrenceCount:    0,
			wantSeverity:       model.SeverityCritical,
			wantMinScore:       35.0,
			wantMaxScore:       35.0,
			wantConfidence:     "medium", // score >= 25, signals = 1
		},
		{
			name: "Warning Alert with High Fleet Blast Radius (>=50%)",
			signals: []IncidentSignal{
				{
					ID:        "sig-1",
					Type:      SignalTypeAlert,
					Source:    "DiskFilling",
					Severity:  model.SeverityWarning,
					Timestamp: now,
				},
			},
			affectedNodesCount: 5,
			totalFleetNodes:    10,
			recurrenceCount:    0,
			wantSeverity:       model.SeverityWarning,
			wantMinScore:       45.0, // 15 (warn alert) + 30 (50% fleet) = 45
			wantMaxScore:       45.0,
			wantConfidence:     "medium",
		},
		{
			name: "Critical Alert + Critical Diagnostic + High Anomaly Cascade",
			signals: []IncidentSignal{
				{ID: "sig-1", Type: SignalTypeAlert, Source: "Alt1", Severity: model.SeverityCritical, Timestamp: now},
				{ID: "sig-2", Type: SignalTypeAlert, Source: "Alt2", Severity: model.SeverityCritical, Timestamp: now},
				{ID: "sig-3", Type: SignalTypeDiagnostic, Source: "Diag1", Severity: model.SeverityCritical, Timestamp: now},
				{ID: "sig-4", Type: SignalTypeAnomaly, Source: "cpu_usage", Severity: model.SeverityCritical, Timestamp: now},
				{ID: "sig-5", Type: SignalTypeAnomaly, Source: "mem_usage", Severity: model.SeverityWarning, Timestamp: now},
				{ID: "sig-6", Type: SignalTypePrediction, Source: "disk_util", Severity: model.SeverityCritical, Timestamp: now},
			},
			affectedNodesCount: 6,
			totalFleetNodes:    10,
			recurrenceCount:    6,
			wantSeverity:       model.SeverityCritical,
			wantMinScore:       100.0, // Clamped to 100
			wantMaxScore:       100.0,
			wantConfidence:     "high",
		},
		{
			name: "Informational Low Signals",
			signals: []IncidentSignal{
				{
					ID:        "sig-1",
					Type:      SignalTypeDiagnostic,
					Source:    "DiagInfo",
					Severity:  model.SeverityInfo,
					Timestamp: now,
				},
			},
			affectedNodesCount: 1,
			totalFleetNodes:    10,
			recurrenceCount:    0,
			wantSeverity:       model.SeverityInfo,
			wantMinScore:       0.0,
			wantMaxScore:       0.0,
			wantConfidence:     "low",
		},
		{
			name: "Recurrence and Flapping Contribution",
			signals: []IncidentSignal{
				{
					ID:        "sig-1",
					Type:      SignalTypeAlert,
					Source:    "FlappingAlert",
					Severity:  model.SeverityWarning,
					Timestamp: now,
				},
			},
			affectedNodesCount: 1,
			totalFleetNodes:    10,
			recurrenceCount:    5,
			wantSeverity:       model.SeverityWarning,
			wantMinScore:       30.0, // 15 (warn alert) + 15 (recurrence >= 5) = 30
			wantMaxScore:       30.0,
			wantConfidence:     "medium",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sev, score, expl := CalculateSeverity(tt.signals, tt.affectedNodesCount, tt.totalFleetNodes, tt.recurrenceCount)

			if sev != tt.wantSeverity {
				t.Errorf("CalculateSeverity() severity = %v, want %v", sev, tt.wantSeverity)
			}
			if score < tt.wantMinScore || score > tt.wantMaxScore {
				t.Errorf("CalculateSeverity() score = %.1f, want between %.1f and %.1f", score, tt.wantMinScore, tt.wantMaxScore)
			}
			if expl.Confidence != tt.wantConfidence {
				t.Errorf("CalculateSeverity() confidence = %v, want %v", expl.Confidence, tt.wantConfidence)
			}
			if expl.BaseScore != score {
				t.Errorf("CalculateSeverity() expl.BaseScore = %.1f, want score %.1f", expl.BaseScore, score)
			}
			if expl.CalculatedSeverity != sev {
				t.Errorf("CalculateSeverity() expl.CalculatedSeverity = %v, want %v", expl.CalculatedSeverity, sev)
			}
		})
	}
}

func TestCalculateSeverity_FactorContributionsAndReasoning(t *testing.T) {
	now := time.Now().UTC()
	signals := []IncidentSignal{
		{ID: "sig-1", Type: SignalTypeAlert, Source: "CriticalRule", Severity: model.SeverityCritical, Timestamp: now},
		{ID: "sig-2", Type: SignalTypeAlert, Source: "SecondaryRule", Severity: model.SeverityWarning, Timestamp: now},
		{ID: "sig-3", Type: SignalTypeDiagnostic, Source: "CheckMemory", Severity: model.SeverityWarning, Timestamp: now},
		{ID: "sig-4", Type: SignalTypeAnomaly, Source: "metric_cpu", Severity: model.SeverityCritical, Timestamp: now},
	}

	sev, score, expl := CalculateSeverity(signals, 4, 10, 3)

	if sev != model.SeverityCritical {
		t.Fatalf("expected critical severity, got %v", sev)
	}

	if len(expl.Factors) == 0 {
		t.Fatalf("expected non-empty factors in severity explanation")
	}

	if len(expl.Reasoning) == 0 {
		t.Fatalf("expected non-empty reasoning in severity explanation")
	}

	// Verify factor breakdown categories
	hasAlertFactor := false
	hasDiagFactor := false
	hasAnomalyFactor := false
	hasBlastFactor := false
	hasRecurrenceFactor := false

	for _, f := range expl.Factors {
		switch f.Category {
		case "alert_severity":
			hasAlertFactor = true
		case "diagnostic_failure":
			hasDiagFactor = true
		case "anomaly":
			hasAnomalyFactor = true
		case "blast_radius":
			hasBlastFactor = true
		case "recurrence_pattern":
			hasRecurrenceFactor = true
		}
		if f.Points <= 0 {
			t.Errorf("expected positive points for factor %s, got %.2f", f.Name, f.Points)
		}
	}

	if !hasAlertFactor || !hasDiagFactor || !hasAnomalyFactor || !hasBlastFactor || !hasRecurrenceFactor {
		t.Errorf("missing expected factor categories in %+v", expl.Factors)
	}

	if score > 100.0 || score < 0.0 {
		t.Errorf("score out of range [0, 100]: %.2f", score)
	}
}
