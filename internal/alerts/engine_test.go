package alerts

import (
	"context"
	"testing"
	"time"

	"github.com/watchdog-cli/watchdog/internal/config"
	"github.com/watchdog-cli/watchdog/internal/storage"
	"github.com/watchdog-cli/watchdog/pkg/model"
)

func TestAlertEngineBuiltinThresholds(t *testing.T) {
	ctx := context.Background()
	cfg := config.DefaultConfig()
	cfg.Alerts.CPU.Threshold = 80.0
	cfg.Alerts.CPU.Duration = 0 // immediate fire for test
	cfg.Alerts.CPU.Cooldown = 1 * time.Minute

	store, err := storage.NewSQLiteStorage(storage.Config{
		Path:     ":memory:",
		Hostname: "test-host",
	})
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer store.Close()

	engine := NewEngine(cfg, store)

	// Step 1: Healthy Snapshot -> No alerts
	healthySnap := &model.SystemSnapshot{
		Timestamp: time.Now(),
		CPU: &model.CPUInfo{
			OverallUsage: 45.0,
		},
	}

	fired, resolved, err := engine.Evaluate(ctx, healthySnap)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	if len(fired) != 0 || len(resolved) != 0 {
		t.Fatalf("expected 0 fired/resolved on healthy, got %d fired, %d resolved", len(fired), len(resolved))
	}

	// Step 2: High CPU Snapshot -> Trigger Alert
	alertTime := time.Now()
	highCPUSnap := &model.SystemSnapshot{
		Timestamp: alertTime,
		CPU: &model.CPUInfo{
			OverallUsage: 92.5,
		},
	}

	fired, resolved, err = engine.Evaluate(ctx, highCPUSnap)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	if len(fired) != 1 {
		t.Fatalf("expected 1 fired alert, got %d", len(fired))
	}
	if fired[0].RuleID != "builtin-cpu-usage" {
		t.Errorf("expected rule ID 'builtin-cpu-usage', got '%s'", fired[0].RuleID)
	}
	if fired[0].Severity != model.SeverityCritical {
		t.Errorf("expected critical severity for 92.5%% CPU, got %s", fired[0].Severity)
	}

	// Verify active alerts
	active := engine.GetActiveAlerts()
	if len(active) != 1 {
		t.Fatalf("expected 1 active alert, got %d", len(active))
	}

	// Step 3: Still High CPU within cooldown -> Should not fire duplicate event
	stillHighSnap := &model.SystemSnapshot{
		Timestamp: alertTime.Add(10 * time.Second),
		CPU: &model.CPUInfo{
			OverallUsage: 95.0,
		},
	}
	fired, resolved, err = engine.Evaluate(ctx, stillHighSnap)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	if len(fired) != 0 {
		t.Errorf("expected 0 new fired alerts due to cooldown/active state, got %d", len(fired))
	}

	// Step 4: Metric drops below threshold -> Auto-resolve
	resolvedTime := alertTime.Add(30 * time.Second)
	recoveredSnap := &model.SystemSnapshot{
		Timestamp: resolvedTime,
		CPU: &model.CPUInfo{
			OverallUsage: 30.0,
		},
	}
	fired, resolved, err = engine.Evaluate(ctx, recoveredSnap)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	if len(resolved) != 1 {
		t.Fatalf("expected 1 resolved alert, got %d", len(resolved))
	}
	if resolved[0].RuleID != "builtin-cpu-usage" {
		t.Errorf("expected resolved rule ID 'builtin-cpu-usage', got '%s'", resolved[0].RuleID)
	}
	if len(engine.GetActiveAlerts()) != 0 {
		t.Errorf("expected 0 active alerts after resolution")
	}
}

func TestAlertDurationRequirement(t *testing.T) {
	ctx := context.Background()
	cfg := config.DefaultConfig()
	cfg.Alerts.Memory.Threshold = 80.0
	cfg.Alerts.Memory.Duration = 20 * time.Second // Must sustain for 20s
	cfg.Alerts.Memory.Cooldown = 1 * time.Minute

	engine := NewEngine(cfg, nil)
	t0 := time.Now()

	// t0: Memory jumps to 85% -> First trigger, but duration (20s) not met yet
	snap0 := &model.SystemSnapshot{
		Timestamp: t0,
		Memory: &model.MemoryInfo{
			TotalBytes:  16 * 1024 * 1024 * 1024,
			UsedPercent: 85.0,
		},
	}
	fired, _, _ := engine.Evaluate(ctx, snap0)
	if len(fired) != 0 {
		t.Errorf("alert fired before duration elapsed: got %d", len(fired))
	}

	// t0 + 10s: Still at 85% -> 10s elapsed < 20s -> still shouldn't fire
	snap1 := &model.SystemSnapshot{
		Timestamp: t0.Add(10 * time.Second),
		Memory: &model.MemoryInfo{
			TotalBytes:  16 * 1024 * 1024 * 1024,
			UsedPercent: 85.0,
		},
	}
	fired, _, _ = engine.Evaluate(ctx, snap1)
	if len(fired) != 0 {
		t.Errorf("alert fired at 10s (required 20s): got %d", len(fired))
	}

	// t0 + 25s: Still at 85% -> 25s elapsed >= 20s -> should fire!
	snap2 := &model.SystemSnapshot{
		Timestamp: t0.Add(25 * time.Second),
		Memory: &model.MemoryInfo{
			TotalBytes:  16 * 1024 * 1024 * 1024,
			UsedPercent: 85.0,
		},
	}
	fired, _, _ = engine.Evaluate(ctx, snap2)
	if len(fired) != 1 {
		t.Fatalf("expected alert to fire after 25s sustained, got %d", len(fired))
	}
}

func TestCustomAlertRule(t *testing.T) {
	ctx := context.Background()
	cfg := config.DefaultConfig()
	engine := NewEngine(cfg, nil)

	engine.RegisterRule(model.AlertRule{
		ID:        "zombie-proc-alert",
		Name:      "Zombie Processes Detected",
		Metric:    "zombies",
		Operator:  ">",
		Threshold: 0,
		Severity:  model.SeverityWarning,
		Duration:  "0s",
		Cooldown:  "5m",
		Enabled:   true,
	})

	snap := &model.SystemSnapshot{
		Timestamp: time.Now(),
		Processes: &model.ProcessSummary{
			TotalCount:  120,
			ZombieCount: 3,
		},
	}

	fired, _, err := engine.Evaluate(ctx, snap)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	found := false
	for _, f := range fired {
		if f.RuleID == "zombie-proc-alert" {
			found = true
			if f.ActualValue != 3.0 {
				t.Errorf("expected actual value 3, got %f", f.ActualValue)
			}
		}
	}
	if !found {
		t.Errorf("custom zombie alert rule did not fire")
	}
}

func TestAlertSilencing(t *testing.T) {
	ctx := context.Background()
	cfg := config.DefaultConfig()
	cfg.Alerts.CPU.Threshold = 75.0
	cfg.Alerts.CPU.Duration = 0
	cfg.Alerts.CPU.Cooldown = 1 * time.Minute

	engine := NewEngine(cfg, nil)

	// Silence CPU alert for 10 minutes
	engine.Silence("builtin-cpu-usage", 10*time.Minute)

	snap := &model.SystemSnapshot{
		Timestamp: time.Now(),
		CPU: &model.CPUInfo{
			OverallUsage: 99.0,
		},
	}

	fired, _, _ := engine.Evaluate(ctx, snap)
	if len(fired) != 0 {
		t.Errorf("alert fired while silenced: got %d", len(fired))
	}
}
