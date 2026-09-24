package tui

import (
	"fmt"
	"testing"
	"time"

	"github.com/DocHoax/watchdog/internal/config"
	"github.com/DocHoax/watchdog/pkg/model"
)

func createBenchmarkTUIModel() Model {
	cfg := config.DefaultConfig()
	m := NewModel(cfg, nil, nil, nil, nil, nil)
	m.width = 140
	m.height = 45

	var procs []model.ProcessInfo
	for i := 1; i <= 100; i++ {
		procs = append(procs, model.ProcessInfo{
			PID:           int32(i * 100),
			Name:          fmt.Sprintf("process_%d", i),
			CPUPercent:    float64(i%20) + 0.5,
			MemoryPercent: float32(i%15) + 0.2,
			MemoryRSS:     uint64(i) * 10 * 1024 * 1024,
			Username:      "system",
			Status:        "Running",
			NumThreads:    4,
			CommandLine:   fmt.Sprintf("/usr/bin/process_%d --config=/etc/conf.yaml", i),
		})
	}

	var cores []model.CPUCoreInfo
	for i := 0; i < 8; i++ {
		cores = append(cores, model.CPUCoreInfo{
			Index:    i,
			UsagePct: 50.0 + float64(i*5),
		})
	}

	m.currentSnapshot = &model.SystemSnapshot{
		Timestamp: time.Now(),
		System: &model.SystemInfo{
			Hostname: "bench-tui-node",
			OS:       "linux",
			Uptime:   360000,
		},
		CPU: &model.CPUInfo{
			OverallUsage:  58.5,
			PhysicalCores: 8,
			LogicalCores:  16,
			Cores:         cores,
			LoadAverage: model.LoadAvg{
				Load1:  2.5,
				Load5:  2.1,
				Load15: 1.8,
			},
		},
		Memory: &model.MemoryInfo{
			TotalBytes:      32 * 1024 * 1024 * 1024,
			AvailableBytes:  16 * 1024 * 1024 * 1024,
			UsedBytes:       16 * 1024 * 1024 * 1024,
			UsedPercent:     50.0,
			SwapTotalBytes:  8 * 1024 * 1024 * 1024,
			SwapUsedBytes:   512 * 1024 * 1024,
			SwapUsedPercent: 6.25,
		},
		Disk: &model.DiskInfo{
			TotalBytes: 500 * 1024 * 1024 * 1024,
			UsedBytes:  250 * 1024 * 1024 * 1024,
			Partitions: []model.PartitionInfo{
				{Mountpoint: "/", TotalBytes: 500 * 1024 * 1024 * 1024, UsedBytes: 250 * 1024 * 1024 * 1024, UsedPercent: 50.0},
			},
		},
		Processes: &model.ProcessSummary{
			TotalCount:   len(procs),
			RunningCount: 5,
			Processes:    procs,
		},
	}

	for i := 0; i < 60; i++ {
		m.cpuHistory = append(m.cpuHistory, float64(40+(i%30)))
		m.memHistory = append(m.memHistory, float64(50+(i%10)))
		m.diskReadHist = append(m.diskReadHist, float64(1024*1024*(i%5)))
		m.diskWriteHist = append(m.diskWriteHist, float64(2048*1024*(i%3)))
		m.netRxHist = append(m.netRxHist, float64(500*1024*(i%4)))
		m.netTxHist = append(m.netTxHist, float64(300*1024*(i%2)))
	}

	return m
}

// BenchmarkTUI_DashboardRender benchmarks frame render time for the main dashboard view.
func BenchmarkTUI_DashboardRender(b *testing.B) {
	m := createBenchmarkTUIModel()
	m.activeTab = TabDashboard

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		view := m.View()
		if len(view) == 0 {
			b.Fatalf("expected non-empty rendered view")
		}
	}
}

// BenchmarkTUI_ProcessesRender benchmarks frame render time for the processes view table.
func BenchmarkTUI_ProcessesRender(b *testing.B) {
	m := createBenchmarkTUIModel()
	m.activeTab = TabProcesses
	m.filteredProcs = m.currentSnapshot.Processes.Processes

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		view := m.View()
		if len(view) == 0 {
			b.Fatalf("expected non-empty rendered view")
		}
	}
}
