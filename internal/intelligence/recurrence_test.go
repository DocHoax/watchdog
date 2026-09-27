package intelligence

import (
	"testing"
	"time"

	"github.com/DocHoax/watchdog/pkg/model"
)

func TestDetectRecurrencePatterns_InsufficientOccurrences(t *testing.T) {
	now := time.Now().UTC()
	incidents := []Incident{
		{
			ID:              "inc-01",
			Title:           "High CPU Alert",
			AffectedNodes:   []string{"node-01"},
			PrimarySymptoms: []string{"cpu_spike"},
			Severity:        model.SeverityWarning,
			StartTime:       now.Add(-2 * time.Hour),
		},
		{
			ID:              "inc-02",
			Title:           "High CPU Alert 2",
			AffectedNodes:   []string{"node-01"},
			PrimarySymptoms: []string{"cpu_spike"},
			Severity:        model.SeverityWarning,
			StartTime:       now.Add(-1 * time.Hour),
		},
	}

	patterns := DetectRecurrencePatterns(incidents, 24*time.Hour, nil)
	if len(patterns) != 0 {
		t.Errorf("expected 0 patterns for 2 occurrences (< 3 required), got: %d", len(patterns))
	}
}

func TestDetectRecurrencePatterns_RegularIntervals_HighConfidence(t *testing.T) {
	now := time.Now().UTC()
	var incidents []Incident

	// 5 incidents spaced exactly 1 hour apart
	for i := 0; i < 5; i++ {
		incidents = append(incidents, Incident{
			ID:              generateIncidentID([]string{"node-01"}, "memory_exhaustion", now.Add(-time.Duration(5-i)*time.Hour)),
			Title:           "Memory Exhaustion Event",
			AffectedNodes:   []string{"node-01"},
			PrimarySymptoms: []string{"memory_exhaustion"},
			Severity:        model.SeverityCritical,
			StartTime:       now.Add(-time.Duration(5-i) * time.Hour),
		})
	}

	patterns := DetectRecurrencePatterns(incidents, 24*time.Hour, nil)
	if len(patterns) == 0 {
		t.Fatalf("expected at least 1 recurrence pattern")
	}

	var nodePattern *RecurrencePattern
	for _, p := range patterns {
		if p.Scope == RecurrenceScopeNode && p.TargetID == "node-01" {
			nodePattern = &p
			break
		}
	}

	if nodePattern == nil {
		t.Fatalf("node-level recurrence pattern not found")
	}
	if nodePattern.OccurrenceCount != 5 {
		t.Errorf("expected 5 occurrences, got: %d", nodePattern.OccurrenceCount)
	}
	if nodePattern.AverageInterval.Round(time.Minute) != 1*time.Hour {
		t.Errorf("expected ~1h average interval, got: %v", nodePattern.AverageInterval)
	}
	if nodePattern.CoefficientOfVariation > 0.05 {
		t.Errorf("expected CV close to 0.0 for periodic incidents, got: %f", nodePattern.CoefficientOfVariation)
	}
	if nodePattern.Confidence != PredictionConfidenceHigh {
		t.Errorf("expected high confidence for regular intervals, got: %s", nodePattern.Confidence)
	}
}

func TestDetectRecurrencePatterns_IrregularIntervals(t *testing.T) {
	now := time.Now().UTC()
	// 3 incidents with very erratic intervals (e.g. 5 min, then 10 hours)
	incidents := []Incident{
		{
			ID:              "inc-irr-01",
			Title:           "Disk Flap",
			AffectedNodes:   []string{"node-02"},
			PrimarySymptoms: []string{"disk_full"},
			Severity:        model.SeverityWarning,
			StartTime:       now.Add(-12 * time.Hour),
		},
		{
			ID:              "inc-irr-02",
			Title:           "Disk Flap",
			AffectedNodes:   []string{"node-02"},
			PrimarySymptoms: []string{"disk_full"},
			Severity:        model.SeverityWarning,
			StartTime:       now.Add(-11 * time.Hour).Add(-55 * time.Minute), // 5 min after first
		},
		{
			ID:              "inc-irr-03",
			Title:           "Disk Flap",
			AffectedNodes:   []string{"node-02"},
			PrimarySymptoms: []string{"disk_full"},
			Severity:        model.SeverityWarning,
			StartTime:       now.Add(-1 * time.Hour), // ~11 hours after second
		},
	}

	patterns := DetectRecurrencePatterns(incidents, 24*time.Hour, nil)
	if len(patterns) == 0 {
		t.Fatalf("expected patterns found")
	}

	var nodePattern *RecurrencePattern
	for _, p := range patterns {
		if p.Scope == RecurrenceScopeNode && p.TargetID == "node-02" {
			nodePattern = &p
			break
		}
	}
	if nodePattern == nil {
		t.Fatalf("expected node pattern for node-02")
	}
	if nodePattern.CoefficientOfVariation <= 0.40 {
		t.Errorf("expected high CV for erratic intervals, got: %f", nodePattern.CoefficientOfVariation)
	}
}

func TestGenerateRecurrenceFindings(t *testing.T) {
	now := time.Now().UTC()
	patterns := []RecurrencePattern{
		{
			ID:                     "rec-test-01",
			Scope:                  RecurrenceScopeNode,
			TargetID:               "node-prod-01",
			EventType:              "oom_killer",
			OccurrenceCount:        4,
			FirstOccurrence:        now.Add(-8 * time.Hour),
			MostRecentOccurrence:   now.Add(-1 * time.Hour),
			AverageInterval:        2 * time.Hour,
			MedianInterval:         2 * time.Hour,
			CoefficientOfVariation: 0.1,
			Confidence:             PredictionConfidenceHigh,
			Summary:                "OOM killer recurring every 2 hours",
		},
	}

	findings := GenerateRecurrenceFindings(patterns)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}

	f := findings[0]
	if f.Category != FindingCategoryRecurringIncident {
		t.Errorf("expected category %s, got: %s", FindingCategoryRecurringIncident, f.Category)
	}
	if f.Severity != model.SeverityCritical {
		t.Errorf("expected critical severity for high confidence OOM recurrence, got: %s", f.Severity)
	}
}
