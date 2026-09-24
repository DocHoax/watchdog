package storage

import (
	"context"
	"testing"
	"time"

	"github.com/DocHoax/watchdog/pkg/model"
)

func TestStorageLifecycle(t *testing.T) {
	ctx := context.Background()
	store, err := NewSQLiteStorage(Config{
		Path:     ":memory:",
		Hostname: "test-host",
	})
	if err != nil {
		t.Fatalf("failed to create in-memory sqlite store: %v", err)
	}
	defer store.Close()

	if err := store.Ping(ctx); err != nil {
		t.Fatalf("ping failed: %v", err)
	}
}

func TestMetricStorageAndAggregates(t *testing.T) {
	ctx := context.Background()
	store, err := NewSQLiteStorage(Config{
		Path:     ":memory:",
		Hostname: "test-host",
	})
	if err != nil {
		t.Fatalf("failed to init store: %v", err)
	}
	defer store.Close()

	now := time.Now().Truncate(time.Second)
	var points []MetricPoint

	values := []float64{10.0, 20.0, 30.0, 40.0, 50.0, 60.0, 70.0, 80.0, 90.0, 100.0}
	for i, v := range values {
		points = append(points, MetricPoint{
			Timestamp: now.Add(time.Duration(i) * time.Minute),
			Metric:    "cpu_usage_pct",
			Value:     v,
			Hostname:  "test-host",
		})
	}

	if err := store.SaveMetricPoints(ctx, points); err != nil {
		t.Fatalf("failed to save metric points: %v", err)
	}

	// Test Query
	queried, err := store.QueryMetrics(ctx, TimeRangeQuery{
		Metric:    "cpu_usage_pct",
		StartTime: now.Add(-time.Minute),
		EndTime:   now.Add(15 * time.Minute),
	})
	if err != nil {
		t.Fatalf("failed to query metrics: %v", err)
	}

	if len(queried) != len(values) {
		t.Fatalf("expected %d points, got %d", len(values), len(queried))
	}

	// Test Aggregates
	agg, err := store.GetMetricAggregate(ctx, "cpu_usage_pct", now.Add(-time.Minute), now.Add(15*time.Minute))
	if err != nil {
		t.Fatalf("failed to get aggregate: %v", err)
	}

	if agg.Count != int64(len(values)) {
		t.Errorf("expected count %d, got %d", len(values), agg.Count)
	}
	if agg.Min != 10.0 {
		t.Errorf("expected min 10.0, got %f", agg.Min)
	}
	if agg.Max != 100.0 {
		t.Errorf("expected max 100.0, got %f", agg.Max)
	}
	if agg.Avg != 55.0 {
		t.Errorf("expected avg 55.0, got %f", agg.Avg)
	}
	if agg.P50 != 55.0 {
		t.Logf("p50 is %f", agg.P50)
	}

	// Test available metrics list
	metrics, err := store.GetAvailableMetrics(ctx)
	if err != nil {
		t.Fatalf("failed to get available metrics: %v", err)
	}
	if len(metrics) != 1 || metrics[0] != "cpu_usage_pct" {
		t.Errorf("expected ['cpu_usage_pct'], got %v", metrics)
	}
}

func TestSnapshotStorage(t *testing.T) {
	ctx := context.Background()
	store, err := NewSQLiteStorage(Config{
		Path:     ":memory:",
		Hostname: "test-host",
	})
	if err != nil {
		t.Fatalf("failed to init store: %v", err)
	}
	defer store.Close()

	snapshot := &model.SystemSnapshot{
		Timestamp: time.Now(),
		System: &model.SystemInfo{
			Hostname: "box1",
		},
		CPU: &model.CPUInfo{
			OverallUsage: 45.5,
			LoadAverage: model.LoadAvg{
				Load1: 1.2,
				Load5: 1.5,
			},
		},
		Memory: &model.MemoryInfo{
			UsedPercent: 62.0,
			UsedBytes:   8 * 1024 * 1024 * 1024,
		},
		Disk: &model.DiskInfo{
			UsedPercent: 75.0,
		},
		Network: &model.NetworkInfo{
			TotalRxRate: 1024 * 100,
			TotalTxRate: 1024 * 50,
		},
		Processes: &model.ProcessSummary{
			TotalCount: 150,
		},
	}

	if err := store.SaveSnapshot(ctx, snapshot); err != nil {
		t.Fatalf("failed to save snapshot: %v", err)
	}

	metrics, err := store.GetAvailableMetrics(ctx)
	if err != nil {
		t.Fatalf("failed to get available metrics: %v", err)
	}

	if len(metrics) < 5 {
		t.Errorf("expected at least 5 metrics from snapshot, got %d: %v", len(metrics), metrics)
	}
}

func TestAlertsStorage(t *testing.T) {
	ctx := context.Background()
	store, err := NewSQLiteStorage(Config{
		Path:     ":memory:",
		Hostname: "test-host",
	})
	if err != nil {
		t.Fatalf("failed to init store: %v", err)
	}
	defer store.Close()

	now := time.Now()
	alert := model.AlertEvent{
		ID:          "alert-1",
		RuleID:      "high-cpu",
		RuleName:    "High CPU Rule",
		Severity:    model.SeverityCritical,
		Message:     "CPU exceeded 90%",
		MetricName:  "cpu_usage_pct",
		ActualValue: 95.2,
		Threshold:   90.0,
		FiredAt:     now,
		IsActive:    true,
	}

	if err := store.SaveAlertEvent(ctx, alert); err != nil {
		t.Fatalf("failed to save alert: %v", err)
	}

	active, err := store.GetActiveAlerts(ctx)
	if err != nil {
		t.Fatalf("failed to get active alerts: %v", err)
	}
	if len(active) != 1 {
		t.Fatalf("expected 1 active alert, got %d", len(active))
	}
	if active[0].ID != "alert-1" {
		t.Errorf("expected alert ID 'alert-1', got '%s'", active[0].ID)
	}

	// Resolve alert
	resolvedTime := now.Add(5 * time.Minute)
	if err := store.UpdateAlertStatus(ctx, "alert-1", model.AlertStatusResolved, &resolvedTime); err != nil {
		t.Fatalf("failed to update alert status: %v", err)
	}

	activeAfter, _ := store.GetActiveAlerts(ctx)
	if len(activeAfter) != 0 {
		t.Errorf("expected 0 active alerts after resolution, got %d", len(activeAfter))
	}

	history, err := store.GetAlertHistory(ctx, 10, 0)
	if err != nil {
		t.Fatalf("failed to get alert history: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("expected 1 historical alert, got %d", len(history))
	}
	if history[0].IsActive {
		t.Errorf("expected alert to be inactive in history")
	}
}

func TestDiagnosticsStorage(t *testing.T) {
	ctx := context.Background()
	store, err := NewSQLiteStorage(Config{
		Path:     ":memory:",
		Hostname: "test-host",
	})
	if err != nil {
		t.Fatalf("failed to init store: %v", err)
	}
	defer store.Close()

	report := &model.DiagnosticReport{
		OverallStatus:  model.StatusWarning,
		TotalChecks:    10,
		PassedChecks:   8,
		WarningChecks:  2,
		CriticalChecks: 0,
		GeneratedAt:    time.Now(),
		Results: []model.DiagnosticResult{
			{
				ID:          "chk-cpu",
				Category:    "CPU",
				Name:        "CPU Utilization Check",
				Status:      model.StatusPass,
				Severity:    model.SeverityInfo,
				Description: "CPU usage is normal",
				MetricValue: "25%",
			},
			{
				ID:          "chk-disk",
				Category:    "Disk",
				Name:        "Root Partition Free Space",
				Status:      model.StatusWarning,
				Severity:    model.SeverityWarning,
				Description: "Disk usage above 80%",
				MetricValue: "85%",
			},
		},
	}

	if err := store.SaveDiagnosticReport(ctx, report); err != nil {
		t.Fatalf("failed to save diagnostic report: %v", err)
	}

	latest, err := store.GetLatestDiagnosticReport(ctx)
	if err != nil {
		t.Fatalf("failed to get latest report: %v", err)
	}
	if latest == nil {
		t.Fatalf("expected non-nil latest report")
	}
	if latest.OverallStatus != model.StatusWarning {
		t.Errorf("expected status WARNING, got %s", latest.OverallStatus)
	}
	if len(latest.Results) != 2 {
		t.Errorf("expected 2 check results, got %d", len(latest.Results))
	}
}

func TestPruneAndVacuum(t *testing.T) {
	ctx := context.Background()
	store, err := NewSQLiteStorage(Config{
		Path:     ":memory:",
		Hostname: "test-host",
	})
	if err != nil {
		t.Fatalf("failed to init store: %v", err)
	}
	defer store.Close()

	oldTime := time.Now().Add(-48 * time.Hour)
	newTime := time.Now()

	_ = store.SaveMetricPoints(ctx, []MetricPoint{
		{Timestamp: oldTime, Metric: "m1", Value: 10},
		{Timestamp: newTime, Metric: "m1", Value: 20},
	})

	deleted, err := store.PruneOlderThan(ctx, 24*time.Hour)
	if err != nil {
		t.Fatalf("prune failed: %v", err)
	}
	if deleted != 1 {
		t.Errorf("expected 1 deleted row, got %d", deleted)
	}

	if err := store.Vacuum(ctx); err != nil {
		t.Fatalf("vacuum failed: %v", err)
	}
}
