package intelligence

import (
	"fmt"
	"testing"
	"time"

	"github.com/DocHoax/watchdog/internal/storage"
	"github.com/DocHoax/watchdog/pkg/model"
)

func BenchmarkNodePrediction(b *testing.B) {
	now := time.Now().UTC()
	var points []storage.MetricPoint
	for i := 0; i < 60; i++ {
		points = append(points, storage.MetricPoint{
			Timestamp: now.Add(time.Duration(i) * time.Minute),
			Value:     50.0 + (float64(i) * 0.2),
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = CalculatePrediction("bench-node-01", "cpu.usage_percent", points, 85.0, 24*time.Hour, 1*time.Hour, nil)
	}
}

func BenchmarkCapacityForecast(b *testing.B) {
	now := time.Now().UTC()
	metrics := map[string][]storage.MetricPoint{
		"cpu.usage_percent":    make([]storage.MetricPoint, 60),
		"memory.usage_percent": make([]storage.MetricPoint, 60),
		"swap.usage_percent":   make([]storage.MetricPoint, 60),
		"disk.usage_percent":   make([]storage.MetricPoint, 60),
	}

	for k := range metrics {
		for i := 0; i < 60; i++ {
			metrics[k][i] = storage.MetricPoint{
				Timestamp: now.Add(time.Duration(i) * time.Minute),
				Value:     40.0 + float64(i)*0.1,
			}
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = EvaluateNodeCapacity("bench-node-01", "bench-host-01", model.NodeStatusHealthy, metrics, 24*time.Hour, 1*time.Hour, nil)
	}
}

func BenchmarkFleetPrediction100Nodes(b *testing.B) {
	now := time.Now().UTC()
	reports := make([]NodeCapacityReport, 100)

	for n := 0; n < 100; n++ {
		reports[n] = NodeCapacityReport{
			NodeID:      fmt.Sprintf("node-%03d", n),
			Hostname:    fmt.Sprintf("host-%03d", n),
			Status:      model.NodeStatusHealthy,
			EvaluatedAt: now,
			Forecasts: []CapacityForecast{
				{
					Resource:                         CapacityResourceCPU,
					CurrentUtilization:               60.0 + float64(n%30),
					WarningThreshold:                 80.0,
					CriticalThreshold:                95.0,
					TrendSlopePerMinute:              0.05,
					ProjectedUtilizationAfterHorizon: 70.0 + float64(n%30),
				},
				{
					Resource:                         CapacityResourceMemory,
					CurrentUtilization:               70.0 + float64(n%25),
					WarningThreshold:                 85.0,
					CriticalThreshold:                95.0,
					TrendSlopePerMinute:              0.1,
					ProjectedUtilizationAfterHorizon: 80.0 + float64(n%25),
				},
			},
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = AggregateFleetCapacity(reports, 24*time.Hour)
	}
}

func BenchmarkRecurrenceDetection(b *testing.B) {
	now := time.Now().UTC()
	incidents := make([]Incident, 50)
	for i := 0; i < 50; i++ {
		incidents[i] = Incident{
			ID:              generateIncidentID([]string{fmt.Sprintf("node-%02d", i%5)}, "cpu_spike", now.Add(-time.Duration(50-i)*time.Hour)),
			Title:           "CPU Spike Alert",
			AffectedNodes:   []string{fmt.Sprintf("node-%02d", i%5)},
			PrimarySymptoms: []string{"cpu_spike"},
			Severity:        model.SeverityWarning,
			StartTime:       now.Add(-time.Duration(50-i) * time.Hour),
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = DetectRecurrencePatterns(incidents, 7*24*time.Hour, nil)
	}
}

func BenchmarkPredictionConfidence(b *testing.B) {
	cfg := DefaultForecastConfig()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = evaluateConfidence(0.92, 25, &cfg, true)
	}
}
