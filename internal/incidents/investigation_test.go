package incidents

import (
	"testing"
	"time"

	"github.com/DocHoax/watchdog/pkg/model"
)

func TestAnalyzeRecurrence(t *testing.T) {
	baseTime := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)

	target := Incident{
		ID:              "inc-target",
		Title:           "High CPU Utilization",
		Severity:        model.SeverityCritical,
		PrimarySymptoms: []string{"High CPU load"},
		AffectedNodes:   []string{"node-1"},
		RootSignals: []IncidentSignal{
			{Source: "cpu_usage", Type: SignalTypeAlert, Severity: model.SeverityCritical},
		},
		Impact: ImpactScope{
			Subsystems: []string{"cpu"},
		},
		StartTime: baseTime,
	}

	t.Run("Isolated Incident when no history", func(t *testing.T) {
		rec := AnalyzeRecurrence(target, nil)
		if rec.Periodicity != PeriodicityIsolated {
			t.Errorf("expected isolated periodicity, got %s", rec.Periodicity)
		}
		if rec.OccurrenceCount != 0 {
			t.Errorf("expected 0 occurrence count, got %d", rec.OccurrenceCount)
		}
		if rec.IsFlapping {
			t.Errorf("expected isFlapping to be false")
		}
	})

	t.Run("Strict Periodic Recurrence", func(t *testing.T) {
		// 4 incidents occurring at exact 1-hour intervals (12:00, 11:00, 10:00, 09:00)
		history := []Incident{
			target,
			{
				ID:              "inc-h1",
				Title:           "High CPU Utilization",
				Severity:        model.SeverityCritical,
				PrimarySymptoms: []string{"High CPU load"},
				AffectedNodes:   []string{"node-1"},
				RootSignals: []IncidentSignal{
					{Source: "cpu_usage", Type: SignalTypeAlert, Severity: model.SeverityCritical},
				},
				Impact: ImpactScope{
					Subsystems: []string{"cpu"},
				},
				StartTime: baseTime.Add(-1 * time.Hour),
			},
			{
				ID:              "inc-h2",
				Title:           "High CPU Utilization",
				Severity:        model.SeverityCritical,
				PrimarySymptoms: []string{"High CPU load"},
				AffectedNodes:   []string{"node-1"},
				RootSignals: []IncidentSignal{
					{Source: "cpu_usage", Type: SignalTypeAlert, Severity: model.SeverityCritical},
				},
				Impact: ImpactScope{
					Subsystems: []string{"cpu"},
				},
				StartTime: baseTime.Add(-2 * time.Hour),
			},
			{
				ID:              "inc-h3",
				Title:           "High CPU Utilization",
				Severity:        model.SeverityCritical,
				PrimarySymptoms: []string{"High CPU load"},
				AffectedNodes:   []string{"node-1"},
				RootSignals: []IncidentSignal{
					{Source: "cpu_usage", Type: SignalTypeAlert, Severity: model.SeverityCritical},
				},
				Impact: ImpactScope{
					Subsystems: []string{"cpu"},
				},
				StartTime: baseTime.Add(-3 * time.Hour),
			},
		}

		rec := AnalyzeRecurrence(target, history)
		if rec.OccurrenceCount != 4 {
			t.Fatalf("expected 4 occurrences, got %d", rec.OccurrenceCount)
		}
		if rec.Periodicity != PeriodicityPeriodic {
			t.Errorf("expected Periodic classification, got %s", rec.Periodicity)
		}
		if rec.CoefficientOfVariation != 0.0 {
			t.Errorf("expected CV 0.0 for identical intervals, got %.3f", rec.CoefficientOfVariation)
		}
		if rec.AverageInterval != 1*time.Hour {
			t.Errorf("expected average interval 1h, got %v", rec.AverageInterval)
		}
		if rec.MedianInterval != 1*time.Hour {
			t.Errorf("expected median interval 1h, got %v", rec.MedianInterval)
		}
	})

	t.Run("Flapping Detection within 2-hour window", func(t *testing.T) {
		// 3 occurrences within 30 minutes total span
		flappingHistory := []Incident{
			target,
			{
				ID:              "inc-f1",
				Title:           "High CPU Utilization",
				Severity:        model.SeverityCritical,
				PrimarySymptoms: []string{"High CPU load"},
				AffectedNodes:   []string{"node-1"},
				RootSignals: []IncidentSignal{
					{Source: "cpu_usage", Type: SignalTypeAlert, Severity: model.SeverityCritical},
				},
				Impact: ImpactScope{
					Subsystems: []string{"cpu"},
				},
				StartTime: baseTime.Add(-10 * time.Minute),
			},
			{
				ID:              "inc-f2",
				Title:           "High CPU Utilization",
				Severity:        model.SeverityCritical,
				PrimarySymptoms: []string{"High CPU load"},
				AffectedNodes:   []string{"node-1"},
				RootSignals: []IncidentSignal{
					{Source: "cpu_usage", Type: SignalTypeAlert, Severity: model.SeverityCritical},
				},
				Impact: ImpactScope{
					Subsystems: []string{"cpu"},
				},
				StartTime: baseTime.Add(-25 * time.Minute),
			},
		}

		rec := AnalyzeRecurrence(target, flappingHistory)
		if !rec.IsFlapping {
			t.Errorf("expected isFlapping to be true for 3 occurrences within 25 min")
		}
	})

	t.Run("Irregular Recurrence", func(t *testing.T) {
		// Large intervals with high variance
		irregularHistory := []Incident{
			target,
			{
				ID:              "inc-i1",
				Title:           "High CPU Utilization",
				Severity:        model.SeverityCritical,
				PrimarySymptoms: []string{"High CPU load"},
				AffectedNodes:   []string{"node-1"},
				RootSignals: []IncidentSignal{
					{Source: "cpu_usage", Type: SignalTypeAlert, Severity: model.SeverityCritical},
				},
				Impact: ImpactScope{
					Subsystems: []string{"cpu"},
				},
				StartTime: baseTime.Add(-10 * time.Minute),
			},
			{
				ID:              "inc-i2",
				Title:           "High CPU Utilization",
				Severity:        model.SeverityCritical,
				PrimarySymptoms: []string{"High CPU load"},
				AffectedNodes:   []string{"node-1"},
				RootSignals: []IncidentSignal{
					{Source: "cpu_usage", Type: SignalTypeAlert, Severity: model.SeverityCritical},
				},
				Impact: ImpactScope{
					Subsystems: []string{"cpu"},
				},
				StartTime: baseTime.Add(-48 * time.Hour),
			},
		}

		rec := AnalyzeRecurrence(target, irregularHistory)
		if rec.Periodicity != PeriodicityIrregular {
			t.Errorf("expected Irregular periodicity, got %s (CV: %.2f)", rec.Periodicity, rec.CoefficientOfVariation)
		}
	})
}

func TestGenerateIncidentFindings(t *testing.T) {
	now := time.Now().UTC()

	incFleet := Incident{
		ID:              "inc-fleet-1",
		Title:           "Fleet-wide Memory Saturation",
		Scope:           IncidentScopeFleet,
		Severity:        model.SeverityCritical,
		Confidence:      "high",
		AffectedNodes:   []string{"node-1", "node-2", "node-3"},
		PrimarySymptoms: []string{"OOM killer active", "Swap exhausted"},
		RootSignals: []IncidentSignal{
			{
				ID:          "sig-1",
				Type:        SignalTypeAlert,
				Source:      "memory_oom",
				NodeID:      "node-1",
				Severity:    model.SeverityCritical,
				Description: "memory pressure exceeding threshold",
				Timestamp:   now,
			},
			{
				ID:          "sig-2",
				Type:        SignalTypePrediction,
				Source:      "memory_capacity",
				NodeID:      "node-2",
				Severity:    model.SeverityCritical,
				Description: "linear regression projects memory exhaustion within 25 minutes",
				Timestamp:   now,
			},
		},
		Impact: ImpactScope{
			FleetPercentage: 75.0,
			Subsystems:      []string{"memory", "cpu"},
		},
		StartTime: now,
	}

	findings := GenerateIncidentFindings(&incFleet)
	if len(findings) < 3 {
		t.Fatalf("expected at least 3 findings (fleet pattern, subsystems, capacity risk), got %d", len(findings))
	}

	foundFleetPattern := false
	foundSubsystemMem := false
	foundCapacityRisk := false

	for _, f := range findings {
		switch f.Category {
		case FindingCategoryFleetPattern:
			foundFleetPattern = true
			if len(f.NonInvasiveSuggestions) == 0 {
				t.Errorf("expected non-invasive suggestions in fleet pattern finding")
			}
		case FindingCategoryResourceExhaustion:
			foundSubsystemMem = true
		case FindingCategoryCapacityRisk:
			foundCapacityRisk = true
		}
	}

	if !foundFleetPattern {
		t.Errorf("expected fleet pattern finding")
	}
	if !foundSubsystemMem {
		t.Errorf("expected resource exhaustion finding for memory")
	}
	if !foundCapacityRisk {
		t.Errorf("expected capacity risk finding")
	}
}

func TestBuildInvestigationReport(t *testing.T) {
	now := time.Now().UTC()

	var timeline []IncidentTimelineEntry
	for i := 0; i < 15; i++ {
		timeline = append(timeline, IncidentTimelineEntry{
			ID:          "tl-" + string(rune('a'+i)),
			Timestamp:   now.Add(time.Duration(i) * time.Minute),
			EventType:   TimelineEventSignalDetected,
			Source:      "sensor",
			Severity:    model.SeverityWarning,
			Title:       "Signal",
			Description: "Sensor signal",
		})
	}

	inc := Incident{
		ID:              "inc-inv-1",
		Title:           "Cluster Degradation",
		Scope:           IncidentScopeMultiNode,
		Severity:        model.SeverityCritical,
		Confidence:      "high",
		AffectedNodes:   []string{"node-1", "node-2"},
		PrimarySymptoms: []string{"Disk latency high"},
		RootSignals: []IncidentSignal{
			{
				ID:          "sig-1",
				Type:        SignalTypeAlert,
				Source:      "disk_io",
				NodeID:      "node-1",
				Severity:    model.SeverityCritical,
				Description: "disk I/O wait > 50ms",
			},
		},
		Timeline: timeline,
		Impact: ImpactScope{
			FleetPercentage: 20.0,
			Subsystems:      []string{"disk"},
		},
		StartTime: now,
	}

	report := BuildInvestigationReport(inc, []Incident{inc}, nil, nil, 10)

	if report.Incident.ID != "inc-inv-1" {
		t.Errorf("expected incident ID inc-inv-1, got %s", report.Incident.ID)
	}
	if len(report.TimelineHighlights) != 10 {
		t.Errorf("expected 10 timeline highlights (5 first + 5 last), got %d", len(report.TimelineHighlights))
	}
	if report.Recurrence == nil {
		t.Errorf("expected recurrence analysis to be populated")
	}
	if len(report.Findings) == 0 {
		t.Errorf("expected findings to be populated")
	}
	if report.Summary == "" {
		t.Errorf("expected non-empty summary")
	}
}
