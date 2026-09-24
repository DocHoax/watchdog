package alerts

import (
	"context"
	"testing"
	"time"

	"github.com/DocHoax/watchdog/internal/config"
	"github.com/DocHoax/watchdog/pkg/model"
)

func createAlertBenchmarkSnapshot() *model.SystemSnapshot {
	return &model.SystemSnapshot{
		Timestamp: time.Now(),
		CPU: &model.CPUInfo{
			OverallUsage:  82.5,
			PhysicalCores: 8,
			LogicalCores:  16,
		},
		Memory: &model.MemoryInfo{
			TotalBytes:      32 * 1024 * 1024 * 1024,
			UsedPercent:     86.0,
			SwapTotalBytes:  8 * 1024 * 1024 * 1024,
			SwapUsedPercent: 12.0,
		},
		Disk: &model.DiskInfo{
			Partitions: []model.PartitionInfo{
				{Mountpoint: "/", UsedPercent: 78.0},
				{Mountpoint: "/var", UsedPercent: 92.0},
			},
		},
	}
}

// BenchmarkAlert_EvaluateAllRules benchmarks evaluating all alert rules against a system snapshot.
func BenchmarkAlert_EvaluateAllRules(b *testing.B) {
	cfg := config.DefaultConfig()
	cfg.Alerts.CPU.Enabled = true
	cfg.Alerts.CPU.Threshold = 80.0
	cfg.Alerts.Memory.Enabled = true
	cfg.Alerts.Memory.Threshold = 85.0
	cfg.Alerts.Disk.Enabled = true
	cfg.Alerts.Disk.Threshold = 90.0

	engine := NewEngine(cfg, nil)
	snapshot := createAlertBenchmarkSnapshot()
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _, err := engine.Evaluate(ctx, snapshot)
		if err != nil {
			b.Fatalf("alert evaluation failed: %v", err)
		}
	}
}
