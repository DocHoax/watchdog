package anomaly

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/watchdog-cli/watchdog/internal/config"
	"github.com/watchdog-cli/watchdog/pkg/model"
)

// BenchmarkAnomaly_Feed100Metrics benchmarks feeding 100 metric points to the anomaly detector.
func BenchmarkAnomaly_Feed100Metrics(b *testing.B) {
	cfg := &config.AnomalyConfig{
		Enabled:         true,
		WindowSize:      60,
		ZScoreThreshold: 3.0,
		Alpha:           0.2,
	}
	detector := NewDetector(cfg)
	now := time.Now()

	// Pre-generate 100 metric names and values
	metricNames := make([]string, 100)
	metricValues := make([]float64, 100)
	r := rand.New(rand.NewSource(42))
	for i := 0; i < 100; i++ {
		metricNames[i] = fmt.Sprintf("metric_stream_%d", i)
		metricValues[i] = 50.0 + r.Float64()*10.0
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		for j := 0; j < 100; j++ {
			_ = detector.Feed(metricNames[j], metricValues[j], now)
		}
	}
}

// BenchmarkAnomaly_FeedSnapshot benchmarks feeding a full system snapshot through anomaly detector.
func BenchmarkAnomaly_FeedSnapshot(b *testing.B) {
	cfg := &config.AnomalyConfig{
		Enabled:         true,
		WindowSize:      60,
		ZScoreThreshold: 3.0,
		Alpha:           0.2,
	}
	detector := NewDetector(cfg)

	snapshot := &model.SystemSnapshot{
		Timestamp: time.Now(),
		CPU: &model.CPUInfo{
			OverallUsage: 45.0,
			LogicalCores: 8,
			LoadAverage: model.LoadAvg{
				Load1: 2.1,
			},
		},
		Memory: &model.MemoryInfo{
			TotalBytes:      16 * 1024 * 1024 * 1024,
			UsedPercent:     62.5,
			SwapTotalBytes:  4 * 1024 * 1024 * 1024,
			SwapUsedPercent: 10.0,
		},
		Disk: &model.DiskInfo{
			UsedPercent: 70.0,
			IOCounters: []model.DiskIOInfo{
				{ReadRate: 1024 * 1024, WriteRate: 2048 * 1024},
			},
		},
		Network: &model.NetworkInfo{
			TotalRxRate: 500 * 1024,
			TotalTxRate: 250 * 1024,
		},
		Processes: &model.ProcessSummary{
			TotalCount:  150,
			ZombieCount: 0,
		},
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		report := detector.FeedSnapshot(snapshot)
		if report == nil {
			b.Fatalf("expected non-nil anomaly report")
		}
	}
}
