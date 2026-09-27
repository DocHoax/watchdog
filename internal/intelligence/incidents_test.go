package intelligence

import (
	"testing"
	"time"

	"github.com/DocHoax/watchdog/pkg/model"
)

func TestIncidentClusterer(t *testing.T) {
	now := time.Now()
	clusterer := NewIncidentClusterer(15 * time.Minute)

	t.Run("No Events", func(t *testing.T) {
		incidents := clusterer.ClusterNodeEvents("node-01", "web-01", nil, nil, nil)
		if len(incidents) != 0 {
			t.Errorf("expected 0 incidents, got %d", len(incidents))
		}
	})

	t.Run("Clustered Alert, Diagnostic, and Anomaly", func(t *testing.T) {
		alerts := []model.AlertEvent{
			{
				ID:       "alt-1",
				RuleName: "HighCPU",
				Message:  "CPU above 95%",
				Severity: model.SeverityCritical,
				FiredAt:  now.Add(-10 * time.Minute),
				IsActive: true,
			},
		}

		diag := &model.DiagnosticReport{
			GeneratedAt: now.Add(-8 * time.Minute),
			Results: []model.DiagnosticResult{
				{
					Name:        "Systemd Service Failure",
					Description: "watchdog-agent degraded",
					Status:      model.StatusFail,
					Severity:    model.SeverityCritical,
					Timestamp:   now.Add(-8 * time.Minute),
				},
			},
		}

		anom := &model.AnomalyReport{
			GeneratedAt: now.Add(-5 * time.Minute),
			Scores: []model.AnomalyScore{
				{
					MetricName:  "load_avg",
					Explanation: "Z-score 3.8",
					Severity:    model.SeverityWarning,
					IsAnomaly:   true,
					DetectedAt:  now.Add(-5 * time.Minute),
				},
			},
		}

		incidents := clusterer.ClusterNodeEvents("node-01", "web-01", alerts, diag, anom)
		if len(incidents) != 1 {
			t.Fatalf("expected 1 incident, got %d", len(incidents))
		}

		inc := incidents[0]
		if inc.Severity != model.SeverityCritical {
			t.Errorf("expected Critical severity, got %s", inc.Severity)
		}
		if inc.Status != IncidentStatusOpen {
			t.Errorf("expected Open status, got %s", inc.Status)
		}
		if len(inc.Timeline) != 3 {
			t.Fatalf("expected 3 timeline events, got %d", len(inc.Timeline))
		}

		// Verify chronological ordering
		if !inc.Timeline[0].Timestamp.Before(inc.Timeline[1].Timestamp) ||
			!inc.Timeline[1].Timestamp.Before(inc.Timeline[2].Timestamp) {
			t.Errorf("timeline not ordered chronologically: %+v", inc.Timeline)
		}
	})
}

func TestFleetPatternAnalyzer(t *testing.T) {
	fpa := NewFleetPatternAnalyzer()

	t.Run("Concurrent CPU Pressure Pattern", func(t *testing.T) {
		summaries := []NodeHealthSummary{
			{
				NodeID:   "node-01",
				Hostname: "srv-01",
				HealthScore: HealthScore{
					Breakdown: []FactorContribution{
						{Name: "CPU Utilization", Category: "cpu", Deduction: 15.0, Explanation: "High CPU usage"},
					},
				},
			},
			{
				NodeID:   "node-02",
				Hostname: "srv-02",
				HealthScore: HealthScore{
					Breakdown: []FactorContribution{
						{Name: "CPU Utilization", Category: "cpu", Deduction: 15.0, Explanation: "High CPU usage"},
					},
				},
			},
			{
				NodeID:   "node-03",
				Hostname: "srv-03",
				HealthScore: HealthScore{
					Breakdown: []FactorContribution{
						{Name: "CPU Utilization", Category: "cpu", Deduction: 15.0, Explanation: "High CPU usage"},
					},
				},
			},
		}

		findings := fpa.AnalyzeFleet(summaries)
		if len(findings) == 0 {
			t.Fatalf("expected fleet pattern findings, got 0")
		}

		var foundCPUPattern bool
		for _, f := range findings {
			if f.Category == FindingCategoryFleetPattern && len(f.AffectedNodes) == 3 {
				foundCPUPattern = true
				break
			}
		}
		if !foundCPUPattern {
			t.Errorf("expected CPU fleet pattern finding across 3 nodes, findings: %+v", findings)
		}
	})

	t.Run("Fleet Degradation Trend", func(t *testing.T) {
		summaries := []NodeHealthSummary{
			{
				NodeID:   "node-01",
				Hostname: "srv-01",
				HealthScore: HealthScore{
					Trajectory: TrajectoryDegrading,
				},
			},
			{
				NodeID:   "node-02",
				Hostname: "srv-02",
				HealthScore: HealthScore{
					Trajectory: TrajectoryDegrading,
				},
			},
			{
				NodeID:   "node-03",
				Hostname: "srv-03",
				HealthScore: HealthScore{
					Trajectory: TrajectoryDegrading,
				},
			},
		}

		findings := fpa.AnalyzeFleet(summaries)
		var foundDegradation bool
		for _, f := range findings {
			if f.Category == FindingCategoryStabilityRisk {
				foundDegradation = true
				break
			}
		}
		if !foundDegradation {
			t.Errorf("expected stability risk finding for degrading fleet, findings: %+v", findings)
		}
	})
}
