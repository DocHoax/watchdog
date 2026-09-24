package alerts

import (
	"context"
	"testing"
	"time"

	"github.com/DocHoax/watchdog/internal/config"
	"github.com/DocHoax/watchdog/internal/storage"
	"github.com/DocHoax/watchdog/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Scenario 1: OK → ALERTING: threshold crossed, fires after duration
func TestAlert_Transition1_OKToAlerting(t *testing.T) {
	ctx := context.Background()
	cfg := config.DefaultConfig()
	cfg.Alerts.CPU.Enabled = true
	cfg.Alerts.CPU.Threshold = 80.0
	cfg.Alerts.CPU.Duration = 15 * time.Second
	cfg.Alerts.CPU.Cooldown = 1 * time.Minute

	engine := NewEngine(cfg, nil)
	t0 := time.Date(2026, 9, 23, 10, 0, 0, 0, time.UTC)

	// Step 1: OK state
	snap0 := &model.SystemSnapshot{
		Timestamp: t0,
		CPU:       &model.CPUInfo{OverallUsage: 50.0},
	}
	fired, resolved, err := engine.Evaluate(ctx, snap0)
	require.NoError(t, err)
	assert.Empty(t, fired)
	assert.Empty(t, resolved)
	assert.Empty(t, engine.GetActiveAlerts())

	// Step 2: Metric crosses threshold at t0+5s, duration (15s) not met yet
	t1 := t0.Add(5 * time.Second)
	snap1 := &model.SystemSnapshot{
		Timestamp: t1,
		CPU:       &model.CPUInfo{OverallUsage: 85.0},
	}
	fired, resolved, err = engine.Evaluate(ctx, snap1)
	require.NoError(t, err)
	assert.Empty(t, fired, "alert should not fire before duration is met")
	assert.Empty(t, resolved)

	// Step 3: Metric still above threshold at t0+15s (10s elapsed < 15s)
	t2 := t0.Add(15 * time.Second)
	snap2 := &model.SystemSnapshot{
		Timestamp: t2,
		CPU:       &model.CPUInfo{OverallUsage: 88.0},
	}
	fired, resolved, err = engine.Evaluate(ctx, snap2)
	require.NoError(t, err)
	assert.Empty(t, fired, "alert should not fire at 10s of sustained high CPU")
	assert.Empty(t, resolved)

	// Step 4: Metric sustained above threshold at t0+21s (16s elapsed >= 15s duration) -> FIRES
	t3 := t0.Add(21 * time.Second)
	snap3 := &model.SystemSnapshot{
		Timestamp: t3,
		CPU:       &model.CPUInfo{OverallUsage: 90.0},
	}
	fired, resolved, err = engine.Evaluate(ctx, snap3)
	require.NoError(t, err)
	require.Len(t, fired, 1, "alert should fire after duration threshold is met")
	assert.Equal(t, "builtin-cpu-usage", fired[0].RuleID)
	assert.Equal(t, 90.0, fired[0].ActualValue)
	assert.True(t, fired[0].IsActive)
	assert.Equal(t, t3, fired[0].FiredAt)

	active := engine.GetActiveAlerts()
	require.Len(t, active, 1)
	assert.Equal(t, "builtin-cpu-usage", active[0].RuleID)
}

// Scenario 2: ALERTING → OK: metric drops below threshold, recovers
func TestAlert_Transition2_AlertingToOK(t *testing.T) {
	ctx := context.Background()
	cfg := config.DefaultConfig()
	cfg.Alerts.CPU.Enabled = true
	cfg.Alerts.CPU.Threshold = 75.0
	cfg.Alerts.CPU.Duration = 0 // Instant trigger
	cfg.Alerts.CPU.Cooldown = 1 * time.Minute

	engine := NewEngine(cfg, nil)
	t0 := time.Date(2026, 9, 23, 11, 0, 0, 0, time.UTC)

	// Trigger alert
	snapAlert := &model.SystemSnapshot{
		Timestamp: t0,
		CPU:       &model.CPUInfo{OverallUsage: 85.0},
	}
	fired, _, err := engine.Evaluate(ctx, snapAlert)
	require.NoError(t, err)
	require.Len(t, fired, 1)
	assert.Len(t, engine.GetActiveAlerts(), 1)

	// Metric recovers to normal value below threshold
	tRecover := t0.Add(30 * time.Second)
	snapRecover := &model.SystemSnapshot{
		Timestamp: tRecover,
		CPU:       &model.CPUInfo{OverallUsage: 40.0},
	}
	fired, resolved, err := engine.Evaluate(ctx, snapRecover)
	require.NoError(t, err)
	assert.Empty(t, fired)
	require.Len(t, resolved, 1, "active alert must resolve when metric drops below threshold")
	assert.Equal(t, "builtin-cpu-usage", resolved[0].RuleID)
	assert.False(t, resolved[0].IsActive)
	require.NotNil(t, resolved[0].ResolvedAt)
	assert.Equal(t, tRecover, *resolved[0].ResolvedAt)
	assert.Empty(t, engine.GetActiveAlerts(), "no active alerts should remain after resolution")
}

// Scenario 3: Flapping: metric oscillates around threshold, hysteresis prevents spam
func TestAlert_Transition3_FlappingHysteresis(t *testing.T) {
	ctx := context.Background()
	cfg := config.DefaultConfig()
	cfg.Alerts.CPU.Enabled = true
	cfg.Alerts.CPU.Threshold = 80.0
	cfg.Alerts.CPU.Duration = 10 * time.Second // Requires 10s sustained
	cfg.Alerts.CPU.Cooldown = 1 * time.Minute

	engine := NewEngine(cfg, nil)
	baseTime := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)

	// Simulate rapid flapping oscillation: spike for 4s, drop for 2s, repeat 10 times
	currTime := baseTime
	totalFired := 0

	for i := range 10 {
		// Jump above threshold
		snapHigh := &model.SystemSnapshot{
			Timestamp: currTime,
			CPU:       &model.CPUInfo{OverallUsage: 90.0},
		}
		fired, _, _ := engine.Evaluate(ctx, snapHigh)
		totalFired += len(fired)

		// Advance 4s (less than 10s duration)
		currTime = currTime.Add(4 * time.Second)
		snapHigh2 := &model.SystemSnapshot{
			Timestamp: currTime,
			CPU:       &model.CPUInfo{OverallUsage: 92.0},
		}
		fired, _, _ = engine.Evaluate(ctx, snapHigh2)
		totalFired += len(fired)

		// Drop below threshold (resets duration timer)
		currTime = currTime.Add(2 * time.Second)
		snapLow := &model.SystemSnapshot{
			Timestamp: currTime,
			CPU:       &model.CPUInfo{OverallUsage: 45.0},
		}
		fired, _, _ = engine.Evaluate(ctx, snapLow)
		totalFired += len(fired)

		currTime = currTime.Add(2 * time.Second)
		assert.Equal(t, 0, totalFired, "flapping oscillation iteration %d should not have fired any alerts", i)
	}

	assert.Equal(t, 0, totalFired, "hysteresis successfully prevented false alerts during metric flapping")
	assert.Empty(t, engine.GetActiveAlerts())
}

// Scenario 4: Cooldown: alert fires, resolves, then threshold crossed again during cooldown
func TestAlert_Transition4_CooldownSuppression(t *testing.T) {
	ctx := context.Background()
	cfg := config.DefaultConfig()
	cfg.Alerts.Memory.Enabled = true
	cfg.Alerts.Memory.Threshold = 80.0
	cfg.Alerts.Memory.Duration = 0
	cfg.Alerts.Memory.Cooldown = 60 * time.Second // 60s cooldown

	engine := NewEngine(cfg, nil)
	t0 := time.Date(2026, 9, 23, 13, 0, 0, 0, time.UTC)

	// 1. Initial trigger at t0
	snap1 := &model.SystemSnapshot{
		Timestamp: t0,
		Memory:    &model.MemoryInfo{TotalBytes: 1000, UsedPercent: 85.0},
	}
	fired, _, _ := engine.Evaluate(ctx, snap1)
	require.Len(t, fired, 1, "first spike should fire alert")

	// 2. Resolve at t0 + 10s
	tResolve := t0.Add(10 * time.Second)
	snap2 := &model.SystemSnapshot{
		Timestamp: tResolve,
		Memory:    &model.MemoryInfo{TotalBytes: 1000, UsedPercent: 50.0},
	}
	_, resolved, _ := engine.Evaluate(ctx, snap2)
	require.Len(t, resolved, 1, "should resolve when usage drops")
	assert.Empty(t, engine.GetActiveAlerts())

	// 3. Spike again at t0 + 25s (within 60s cooldown from t0)
	tDuringCooldown := t0.Add(25 * time.Second)
	snap3 := &model.SystemSnapshot{
		Timestamp: tDuringCooldown,
		Memory:    &model.MemoryInfo{TotalBytes: 1000, UsedPercent: 90.0},
	}
	fired, resolved, _ = engine.Evaluate(ctx, snap3)
	assert.Empty(t, fired, "alert should be suppressed during active cooldown window")
	assert.Empty(t, resolved)

	// 4. Spike again at t0 + 65s (cooldown expired: 65s > 60s)
	tAfterCooldown := t0.Add(65 * time.Second)
	snap4 := &model.SystemSnapshot{
		Timestamp: tAfterCooldown,
		Memory:    &model.MemoryInfo{TotalBytes: 1000, UsedPercent: 92.0},
	}
	fired, _, _ = engine.Evaluate(ctx, snap4)
	require.Len(t, fired, 1, "alert should fire again once cooldown expires")
	assert.Equal(t, "builtin-mem-usage", fired[0].RuleID)
}

// Scenario 5: Multiple simultaneous alerts: CPU, memory, disk all fire concurrently
func TestAlert_Transition5_MultipleSimultaneousAlerts(t *testing.T) {
	ctx := context.Background()
	cfg := config.DefaultConfig()
	cfg.Alerts.CPU.Enabled = true
	cfg.Alerts.CPU.Threshold = 80.0
	cfg.Alerts.CPU.Duration = 0

	cfg.Alerts.Memory.Enabled = true
	cfg.Alerts.Memory.Threshold = 80.0
	cfg.Alerts.Memory.Duration = 0

	cfg.Alerts.Disk.Enabled = true
	cfg.Alerts.Disk.Threshold = 85.0
	cfg.Alerts.Disk.Duration = 0

	engine := NewEngine(cfg, nil)
	t0 := time.Date(2026, 9, 23, 14, 0, 0, 0, time.UTC)

	// All 3 subsystems exceed thresholds simultaneously
	snapHigh := &model.SystemSnapshot{
		Timestamp: t0,
		CPU:       &model.CPUInfo{OverallUsage: 95.0},
		Memory:    &model.MemoryInfo{TotalBytes: 16000, UsedPercent: 91.0},
		Disk: &model.DiskInfo{
			Partitions: []model.PartitionInfo{
				{Mountpoint: "/var", TotalBytes: 100000, UsedPercent: 88.0},
			},
		},
	}

	fired, resolved, err := engine.Evaluate(ctx, snapHigh)
	require.NoError(t, err)
	require.Len(t, fired, 3, "expected 3 concurrent alerts fired")
	assert.Empty(t, resolved)

	ruleIDs := make(map[string]bool)
	for _, f := range fired {
		ruleIDs[f.RuleID] = true
	}
	assert.True(t, ruleIDs["builtin-cpu-usage"])
	assert.True(t, ruleIDs["builtin-mem-usage"])
	assert.True(t, ruleIDs["builtin-disk-var"])

	active := engine.GetActiveAlerts()
	assert.Len(t, active, 3)

	// All 3 recover simultaneously
	tRecover := t0.Add(20 * time.Second)
	snapRecover := &model.SystemSnapshot{
		Timestamp: tRecover,
		CPU:       &model.CPUInfo{OverallUsage: 30.0},
		Memory:    &model.MemoryInfo{TotalBytes: 16000, UsedPercent: 40.0},
		Disk: &model.DiskInfo{
			Partitions: []model.PartitionInfo{
				{Mountpoint: "/var", TotalBytes: 100000, UsedPercent: 60.0},
			},
		},
	}

	fired, resolved, err = engine.Evaluate(ctx, snapRecover)
	require.NoError(t, err)
	assert.Empty(t, fired)
	require.Len(t, resolved, 3, "expected all 3 alerts to resolve concurrently")
	assert.Empty(t, engine.GetActiveAlerts())
}

// Scenario 6: Alert history persistence: alerts saved to SQLite and queryable
func TestAlert_Transition6_SQLiteHistoryPersistence(t *testing.T) {
	ctx := context.Background()
	cfg := config.DefaultConfig()
	cfg.Alerts.CPU.Enabled = true
	cfg.Alerts.CPU.Threshold = 70.0
	cfg.Alerts.CPU.Duration = 0
	cfg.Alerts.CPU.Cooldown = 10 * time.Minute

	cfg.Alerts.Memory.Enabled = true
	cfg.Alerts.Memory.Threshold = 75.0
	cfg.Alerts.Memory.Duration = 0
	cfg.Alerts.Memory.Cooldown = 10 * time.Minute

	store, err := storage.NewSQLiteStorage(storage.Config{
		Path:     ":memory:",
		Hostname: "validation-host",
	})
	require.NoError(t, err)
	defer store.Close()

	engine := NewEngine(cfg, store)
	t0 := time.Date(2026, 9, 23, 15, 0, 0, 0, time.UTC)

	// 1. CPU alert fires
	snap1 := &model.SystemSnapshot{
		Timestamp: t0,
		CPU:       &model.CPUInfo{OverallUsage: 88.0},
	}
	fired1, _, err := engine.Evaluate(ctx, snap1)
	require.NoError(t, err)
	require.Len(t, fired1, 1)

	// Verify active alert in SQLite
	activeInDB, err := store.GetActiveAlerts(ctx)
	require.NoError(t, err)
	require.Len(t, activeInDB, 1)
	assert.Equal(t, "builtin-cpu-usage", activeInDB[0].RuleID)
	assert.Equal(t, model.SeverityCritical, activeInDB[0].Severity)

	// 2. CPU resolves, Memory fires at t0 + 30s
	t2 := t0.Add(30 * time.Second)
	snap2 := &model.SystemSnapshot{
		Timestamp: t2,
		CPU:       &model.CPUInfo{OverallUsage: 45.0},
		Memory:    &model.MemoryInfo{TotalBytes: 8000, UsedPercent: 90.0},
	}
	fired2, resolved2, err := engine.Evaluate(ctx, snap2)
	require.NoError(t, err)
	require.Len(t, fired2, 1)
	require.Len(t, resolved2, 1)
	assert.Equal(t, "builtin-mem-usage", fired2[0].RuleID)
	assert.Equal(t, "builtin-cpu-usage", resolved2[0].RuleID)

	// Verify active alerts in DB is now only Memory
	activeInDB, err = store.GetActiveAlerts(ctx)
	require.NoError(t, err)
	require.Len(t, activeInDB, 1)
	assert.Equal(t, "builtin-mem-usage", activeInDB[0].RuleID)

	// Verify full alert history in DB has 2 records
	history, err := store.GetAlertHistory(ctx, 10, 0)
	require.NoError(t, err)
	require.Len(t, history, 2)

	// History is sorted triggered_at DESC: memory first, then CPU
	assert.Equal(t, "builtin-mem-usage", history[0].RuleID)
	assert.True(t, history[0].IsActive)
	assert.Nil(t, history[0].ResolvedAt)

	assert.Equal(t, "builtin-cpu-usage", history[1].RuleID)
	assert.False(t, history[1].IsActive)
	require.NotNil(t, history[1].ResolvedAt)
	assert.Equal(t, t2.UnixMilli(), history[1].ResolvedAt.UnixMilli())
}
