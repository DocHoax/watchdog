package diagnostics

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/DocHoax/watchdog/internal/config"
	"github.com/DocHoax/watchdog/pkg/model"
)

func createBenchmarkSnapshot() *model.SystemSnapshot {
	var partitions []model.PartitionInfo
	for i := 0; i < 5; i++ {
		partitions = append(partitions, model.PartitionInfo{
			Mountpoint:  fmt.Sprintf("/mnt/data%d", i),
			Device:      fmt.Sprintf("/dev/sda%d", i+1),
			FSType:      "ext4",
			TotalBytes:  500 * 1024 * 1024 * 1024,
			UsedBytes:   250 * 1024 * 1024 * 1024,
			FreeBytes:   250 * 1024 * 1024 * 1024,
			UsedPercent: 50.0,
			InodesTotal: 1000000,
			InodesFree:  800000,
			InodesUsed:  200000,
			InodesPct:   20.0,
		})
	}

	return &model.SystemSnapshot{
		Timestamp: time.Now(),
		System: &model.SystemInfo{
			Hostname: "bench-node-01",
			OS:       "linux",
		},
		CPU: &model.CPUInfo{
			OverallUsage:  65.5,
			PhysicalCores: 16,
			LogicalCores:  32,
			LoadAverage: model.LoadAvg{
				Load1:  4.2,
				Load5:  3.8,
				Load15: 3.1,
			},
		},
		Memory: &model.MemoryInfo{
			TotalBytes:      64 * 1024 * 1024 * 1024,
			AvailableBytes:  32 * 1024 * 1024 * 1024,
			UsedBytes:       32 * 1024 * 1024 * 1024,
			UsedPercent:     50.0,
			SwapTotalBytes:  16 * 1024 * 1024 * 1024,
			SwapUsedBytes:   1 * 1024 * 1024 * 1024,
			SwapUsedPercent: 6.25,
		},
		Disk: &model.DiskInfo{
			TotalBytes: 2500 * 1024 * 1024 * 1024,
			UsedBytes:  1250 * 1024 * 1024 * 1024,
			Partitions: partitions,
		},
		Processes: &model.ProcessSummary{
			TotalCount:   350,
			RunningCount: 8,
			ZombieCount:  0,
		},
		Docker: &model.DockerSummary{
			ContainersTotal: 12,
			RunningCount:    10,
			StoppedCount:    2,
		},
	}
}

// BenchmarkDiagnostics_EngineEvaluation benchmarks evaluating all registered diagnostic rules.
func BenchmarkDiagnostics_EngineEvaluation(b *testing.B) {
	cfg := config.DefaultConfig()
	engine := NewEngine(cfg)
	snapshot := createBenchmarkSnapshot()
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		report, err := engine.Run(ctx, snapshot)
		if err != nil {
			b.Fatalf("diagnostic evaluation failed: %v", err)
		}
		if report == nil {
			b.Fatalf("expected non-nil report")
		}
	}
}
