package intelligence

import (
	"testing"
	"time"

	"github.com/DocHoax/watchdog/internal/storage"
	"github.com/DocHoax/watchdog/pkg/model"
)

func TestEvaluateNodeCapacity_Normal(t *testing.T) {
	nodeID := "node-cap-01"
	hostname := "host-cap-01"
	status := model.NodeStatusHealthy

	now := time.Now().UTC()
	metricPoints := make(map[string][]storage.MetricPoint)

	// CPU: constant ~40%
	var cpuPoints []storage.MetricPoint
	for i := 0; i < 20; i++ {
		cpuPoints = append(cpuPoints, storage.MetricPoint{
			Timestamp: now.Add(time.Duration(i) * time.Minute),
			Value:     40.0,
		})
	}
	metricPoints["cpu.usage_percent"] = cpuPoints

	// Memory: increasing from 60% to 70% (slope 0.5/min)
	var memPoints []storage.MetricPoint
	for i := 0; i < 20; i++ {
		memPoints = append(memPoints, storage.MetricPoint{
			Timestamp: now.Add(time.Duration(i) * time.Minute),
			Value:     60.0 + (float64(i) * 0.5),
		})
	}
	metricPoints["memory.usage_percent"] = memPoints

	// Swap: constant 10%
	var swapPoints []storage.MetricPoint
	for i := 0; i < 20; i++ {
		swapPoints = append(swapPoints, storage.MetricPoint{
			Timestamp: now.Add(time.Duration(i) * time.Minute),
			Value:     10.0,
		})
	}
	metricPoints["swap.usage_percent"] = swapPoints

	// Disk: constant 50%
	var diskPoints []storage.MetricPoint
	for i := 0; i < 20; i++ {
		diskPoints = append(diskPoints, storage.MetricPoint{
			Timestamp: now.Add(time.Duration(i) * time.Minute),
			Value:     50.0,
		})
	}
	metricPoints["disk.usage_percent"] = diskPoints

	horizon := 2 * time.Hour
	report := EvaluateNodeCapacity(nodeID, hostname, status, metricPoints, horizon, 1*time.Hour, nil)

	if report.NodeID != nodeID {
		t.Errorf("expected node ID %s, got: %s", nodeID, report.NodeID)
	}
	if len(report.Forecasts) != 4 {
		t.Fatalf("expected 4 forecasts (cpu, memory, swap, disk), got: %d", len(report.Forecasts))
	}
	if len(report.Predictions) != 8 {
		t.Fatalf("expected 8 predictions (2 per resource), got: %d", len(report.Predictions))
	}

	// Verify Memory forecast: current 69.5, slope ~0.5, warning is 85.0 -> TimeToWarning ~30m
	var memForecast *CapacityForecast
	for _, fc := range report.Forecasts {
		if fc.Resource == CapacityResourceMemory {
			memForecast = &fc
			break
		}
	}
	if memForecast == nil {
		t.Fatalf("memory forecast not found")
	}
	if memForecast.TrendSlopePerMinute < 0.49 || memForecast.TrendSlopePerMinute > 0.51 {
		t.Errorf("expected memory trend slope ~0.5, got: %f", memForecast.TrendSlopePerMinute)
	}
	if memForecast.TimeToWarning == nil {
		t.Errorf("expected non-nil TimeToWarning for memory")
	}
}

func TestEvaluateNodeCapacity_InsufficientSamples(t *testing.T) {
	nodeID := "node-cap-02"
	hostname := "host-cap-02"
	status := model.NodeStatusHealthy

	metricPoints := map[string][]storage.MetricPoint{
		"cpu.usage_percent": {
			{Timestamp: time.Now().UTC(), Value: 30.0},
		},
	}

	report := EvaluateNodeCapacity(nodeID, hostname, status, metricPoints, 24*time.Hour, 1*time.Hour, nil)
	if len(report.Forecasts) != 4 {
		t.Fatalf("expected 4 forecasts, got: %d", len(report.Forecasts))
	}

	cpuFC := report.Forecasts[0]
	if cpuFC.Confidence != PredictionConfidenceInsufficientData {
		t.Errorf("expected insufficient data confidence, got: %s", cpuFC.Confidence)
	}
	if len(cpuFC.Evidence) == 0 {
		t.Errorf("expected evidence indicating insufficient samples")
	}
}

func TestAggregateFleetCapacity(t *testing.T) {
	t.Run("Empty reports", func(t *testing.T) {
		summary := AggregateFleetCapacity(nil, 24*time.Hour)
		if summary.TotalNodes != 0 {
			t.Errorf("expected 0 total nodes, got: %d", summary.TotalNodes)
		}
		if len(summary.TopCapacityRisks) != 0 {
			t.Errorf("expected 0 top capacity risks, got: %d", len(summary.TopCapacityRisks))
		}
	})

	t.Run("Multi-node capacity pressure", func(t *testing.T) {
		now := time.Now().UTC()
		dur := 30 * time.Minute

		node1 := NodeCapacityReport{
			NodeID:      "node-01",
			Hostname:    "host-01",
			Status:      model.NodeStatusHealthy,
			EvaluatedAt: now,
			Forecasts: []CapacityForecast{
				{
					Resource:                         CapacityResourceCPU,
					CurrentUtilization:               85.0, // Exceeds warning (80%)
					WarningThreshold:                 80.0,
					CriticalThreshold:                95.0,
					TrendSlopePerMinute:              0.2,
					ProjectedUtilizationAfterHorizon: 95.0,
					TimeToWarning:                    nil,
					TimeToCritical:                   &dur,
				},
				{
					Resource:                         CapacityResourceMemory,
					CurrentUtilization:               50.0,
					WarningThreshold:                 85.0,
					CriticalThreshold:                95.0,
					TrendSlopePerMinute:              0.0,
					ProjectedUtilizationAfterHorizon: 50.0,
				},
			},
		}

		node2 := NodeCapacityReport{
			NodeID:      "node-02",
			Hostname:    "host-02",
			Status:      model.NodeStatusHealthy,
			EvaluatedAt: now,
			Forecasts: []CapacityForecast{
				{
					Resource:                         CapacityResourceCPU,
					CurrentUtilization:               40.0,
					WarningThreshold:                 80.0,
					CriticalThreshold:                95.0,
					TrendSlopePerMinute:              0.0,
					ProjectedUtilizationAfterHorizon: 40.0,
				},
				{
					Resource:                         CapacityResourceMemory,
					CurrentUtilization:               90.0, // Exceeds warning (85%)
					WarningThreshold:                 85.0,
					CriticalThreshold:                95.0,
					TrendSlopePerMinute:              0.1,
					ProjectedUtilizationAfterHorizon: 96.0,
					TimeToCritical:                   &dur,
				},
			},
		}

		summary := AggregateFleetCapacity([]NodeCapacityReport{node1, node2}, 24*time.Hour)
		if summary.TotalNodes != 2 {
			t.Errorf("expected 2 nodes, got: %d", summary.TotalNodes)
		}
		if summary.CPUPressurePercent != 50.0 {
			t.Errorf("expected 50%% CPU pressure, got: %f", summary.CPUPressurePercent)
		}
		if summary.MemoryPressurePercent != 50.0 {
			t.Errorf("expected 50%% memory pressure, got: %f", summary.MemoryPressurePercent)
		}
		if summary.NodesApproachingCritical != 2 {
			t.Errorf("expected 2 nodes approaching critical, got: %d", summary.NodesApproachingCritical)
		}
		if len(summary.TopCapacityRisks) != 2 {
			t.Errorf("expected 2 top capacity risk entries, got: %d", len(summary.TopCapacityRisks))
		}
		if len(summary.Findings) == 0 {
			t.Errorf("expected findings generated for fleet capacity pressure")
		}
	})
}
