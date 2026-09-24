package anomaly

import (
	"math"
	"testing"
	"time"

	"github.com/DocHoax/watchdog/internal/config"
	"github.com/DocHoax/watchdog/pkg/model"
)

func TestRollingWindowAndStats(t *testing.T) {
	window := NewRollingWindow(5)

	// Add 5 samples: 10, 20, 30, 40, 50
	values := []float64{10, 20, 30, 40, 50}
	for _, v := range values {
		window.Add(v)
	}

	if window.Count() != 5 {
		t.Fatalf("expected count 5, got %d", window.Count())
	}

	if window.Mean() != 30.0 {
		t.Errorf("expected mean 30.0, got %f", window.Mean())
	}

	minV, maxV := window.MinMax()
	if minV != 10.0 || maxV != 50.0 {
		t.Errorf("expected min 10, max 50, got min %f, max %f", minV, maxV)
	}

	// Standard deviation of [10, 20, 30, 40, 50] is sqrt(1000/4) = sqrt(250) ≈ 15.811
	stdDev := window.StdDev()
	if math.Abs(stdDev-15.811) > 0.01 {
		t.Errorf("expected stddev ≈ 15.811, got %f", stdDev)
	}

	// Overwrite oldest item (10) with 60 -> window: [60, 20, 30, 40, 50] -> sum = 200, mean = 40
	window.Add(60)
	if window.Count() != 5 {
		t.Errorf("expected count 5 after rollover, got %d", window.Count())
	}
	if window.Mean() != 40.0 {
		t.Errorf("expected mean 40.0 after rollover, got %f", window.Mean())
	}
}

func TestEWMATracker(t *testing.T) {
	ewma := NewEWMATracker(0.5)

	v1 := ewma.Update(100.0)
	if v1 != 100.0 {
		t.Errorf("expected initial EWMA 100.0, got %f", v1)
	}

	v2 := ewma.Update(200.0) // 0.5*200 + 0.5*100 = 150
	if v2 != 150.0 {
		t.Errorf("expected second EWMA 150.0, got %f", v2)
	}
}

func TestAnomalyDetectorSpike(t *testing.T) {
	cfg := &config.AnomalyConfig{
		WindowSize:      20,
		ZScoreThreshold: 3.0,
		Alpha:           0.2,
	}

	detector := NewDetector(cfg)
	now := time.Now()

	// Train baseline: 15 samples around 20.0 (e.g. 19.5, 20.5, 20.0...)
	for i := 0; i < 15; i++ {
		val := 20.0 + (float64(i%3)-1.0)*0.5 // 19.5, 20.0, 20.5
		score := detector.Feed("cpu_usage", val, now.Add(time.Duration(i)*time.Second))
		if score.IsAnomaly {
			t.Errorf("unexpected anomaly during baseline training at sample %d", i)
		}
	}

	// Feed a massive sudden spike: 95.0
	spikeScore := detector.Feed("cpu_usage", 95.0, now.Add(16*time.Second))
	if !spikeScore.IsAnomaly {
		t.Fatalf("expected 95.0 to trigger an anomaly, got Z-score: %f, explanation: %s", spikeScore.ZScore, spikeScore.Explanation)
	}
	if spikeScore.Severity != model.SeverityCritical {
		t.Errorf("expected critical severity for massive spike, got %s", spikeScore.Severity)
	}
	if spikeScore.ZScore < 3.0 {
		t.Errorf("expected Z-score >= 3.0, got %f", spikeScore.ZScore)
	}
}

func TestDetectorSnapshotEvaluation(t *testing.T) {
	detector := NewDetector(&config.AnomalyConfig{
		WindowSize:      30,
		ZScoreThreshold: 2.5,
	})

	now := time.Now()

	// Feed 12 steady snapshots
	for i := 0; i < 12; i++ {
		snap := &model.SystemSnapshot{
			Timestamp: now.Add(time.Duration(i) * time.Second),
			CPU: &model.CPUInfo{
				OverallUsage: 15.0 + float64(i%2),
				LogicalCores: 8,
				LoadAverage: model.LoadAvg{
					Load1: 1.0,
				},
			},
			Memory: &model.MemoryInfo{
				TotalBytes:  16 * 1024 * 1024 * 1024,
				UsedPercent: 40.0,
			},
			Disk: &model.DiskInfo{
				UsedPercent: 50.0,
			},
			Network: &model.NetworkInfo{
				TotalRxRate: 1024 * 50,
				TotalTxRate: 1024 * 20,
			},
			Processes: &model.ProcessSummary{
				TotalCount:  100,
				ZombieCount: 0,
			},
		}

		report := detector.FeedSnapshot(snap)
		if report.AnomaliesCount > 0 {
			t.Errorf("unexpected anomaly in steady snapshots: %v", report.Scores)
		}
	}

	// Feed anomalous snapshot (huge network RX surge & process surge)
	anomalySnap := &model.SystemSnapshot{
		Timestamp: now.Add(15 * time.Second),
		CPU: &model.CPUInfo{
			OverallUsage: 15.0,
			LogicalCores: 8,
			LoadAverage: model.LoadAvg{
				Load1: 1.0,
			},
		},
		Memory: &model.MemoryInfo{
			TotalBytes:  16 * 1024 * 1024 * 1024,
			UsedPercent: 40.0,
		},
		Disk: &model.DiskInfo{
			UsedPercent: 50.0,
		},
		Network: &model.NetworkInfo{
			TotalRxRate: 1024 * 1024 * 100, // 100 MB/s surge from 50 KB/s
			TotalTxRate: 1024 * 20,
		},
		Processes: &model.ProcessSummary{
			TotalCount:  500, // 500 procs surge from 100
			ZombieCount: 0,
		},
	}

	report := detector.FeedSnapshot(anomalySnap)
	if report.AnomaliesCount == 0 {
		t.Fatalf("expected anomalies detected in surge snapshot, got 0")
	}

	t.Logf("Detected %d anomalies out of %d checked metrics", report.AnomaliesCount, report.TotalChecked)
}
