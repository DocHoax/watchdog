package diagnostics

import (
	"context"
	"testing"
	"time"

	"github.com/watchdog-cli/watchdog/internal/config"
	"github.com/watchdog-cli/watchdog/pkg/model"
)

func TestDiagnosticEngineHealthySnapshot(t *testing.T) {
	cfg := config.DefaultConfig()
	engine := NewEngine(cfg)

	snapshot := &model.SystemSnapshot{
		Timestamp: time.Now(),
		CPU: &model.CPUInfo{
			OverallUsage: 25.0,
			LogicalCores: 8,
			LoadAverage: model.LoadAvg{
				Load1:  2.0,
				Load5:  1.5,
				Load15: 1.0,
			},
		},
		Memory: &model.MemoryInfo{
			UsedPercent:     50.0,
			SwapTotalBytes:  4 * 1024 * 1024 * 1024,
			SwapUsedBytes:   100 * 1024 * 1024,
			SwapUsedPercent: 2.5,
		},
		Disk: &model.DiskInfo{
			Partitions: []model.PartitionInfo{
				{
					Mountpoint:  "/",
					UsedPercent: 60.0,
					InodesTotal: 1000000,
					InodesFree:  800000,
					InodesPct:   20.0,
				},
			},
		},
		Processes: &model.ProcessSummary{
			TotalCount:  100,
			ZombieCount: 0,
		},
	}

	report, err := engine.Run(context.Background(), snapshot)
	if err != nil {
		t.Fatalf("engine.Run failed: %v", err)
	}

	if report == nil {
		t.Fatalf("expected non-nil report")
	}

	if report.TotalChecks == 0 {
		t.Fatalf("expected > 0 checks evaluated, got %d", report.TotalChecks)
	}

	// In healthy state, critical checks should be 0
	if report.CriticalChecks != 0 {
		t.Errorf("expected 0 critical checks in healthy state, got %d", report.CriticalChecks)
	}
}

func TestDiagnosticEngineDegradedSnapshot(t *testing.T) {
	cfg := config.DefaultConfig()
	engine := NewEngine(cfg)

	snapshot := &model.SystemSnapshot{
		Timestamp: time.Now(),
		CPU: &model.CPUInfo{
			OverallUsage: 98.5, // Critical > 90%
			LogicalCores: 4,
			LoadAverage: model.LoadAvg{
				Load1: 12.0, // Load ratio 3.0 > 2.5 Critical
			},
		},
		Memory: &model.MemoryInfo{
			UsedPercent:     96.0, // Critical > 92%
			SwapTotalBytes:  4 * 1024 * 1024 * 1024,
			SwapUsedBytes:   3 * 1024 * 1024 * 1024,
			SwapUsedPercent: 75.0,
		},
		Disk: &model.DiskInfo{
			Partitions: []model.PartitionInfo{
				{
					Mountpoint:  "/",
					UsedPercent: 95.0, // Critical > 90%
					InodesTotal: 1000,
					InodesFree:  20,
					InodesPct:   98.0, // Critical > 95%
				},
			},
		},
		Processes: &model.ProcessSummary{
			TotalCount:  200,
			ZombieCount: 15, // Warning > 10
		},
		Docker: &model.DockerSummary{
			Available: true,
			Containers: []model.DockerContainer{
				{
					ID:           "c1",
					Names:        []string{"failing-app"},
					State:        "running",
					RestartCount: 15, // Fail >= 5
				},
			},
		},
	}

	report, err := engine.Run(context.Background(), snapshot)
	if err != nil {
		t.Fatalf("engine.Run failed: %v", err)
	}

	if report.OverallStatus != model.StatusFail {
		t.Errorf("expected overall status FAIL, got %s", report.OverallStatus)
	}

	if report.CriticalChecks == 0 {
		t.Errorf("expected multiple critical checks, got 0")
	}

	if report.WarningChecks == 0 {
		t.Errorf("expected warning checks, got 0")
	}
}

func TestCustomDiagnosticRule(t *testing.T) {
	cfg := config.DefaultConfig()
	engine := NewEngine(cfg)

	customRule := &mockRule{
		id:       "custom-security-check",
		category: "Security",
		name:     "Mock Security Check",
		status:   model.StatusPass,
	}

	engine.RegisterRule(customRule)

	snapshot := &model.SystemSnapshot{
		Timestamp: time.Now(),
	}

	report, err := engine.Run(context.Background(), snapshot)
	if err != nil {
		t.Fatalf("engine.Run failed: %v", err)
	}

	found := false
	for _, r := range report.Results {
		if r.ID == "custom-security-check" {
			found = true
			if r.Status != model.StatusPass {
				t.Errorf("expected custom rule status PASS, got %s", r.Status)
			}
		}
	}

	if !found {
		t.Errorf("custom rule was not executed or not found in report results")
	}
}

type mockRule struct {
	id       string
	category string
	name     string
	status   model.DiagnosticStatus
}

func (m *mockRule) ID() string       { return m.id }
func (m *mockRule) Category() string { return m.category }
func (m *mockRule) Name() string     { return m.name }
func (m *mockRule) Evaluate(ctx context.Context, snapshot *model.SystemSnapshot, cfg *config.Config) model.DiagnosticResult {
	return model.DiagnosticResult{
		ID:          m.id,
		Category:    m.category,
		Name:        m.name,
		Status:      m.status,
		Severity:    model.SeverityInfo,
		Description: "Mock rule executed successfully",
	}
}
