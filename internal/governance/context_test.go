package governance

import (
	"context"
	"testing"
	"time"

	"github.com/DocHoax/watchdog/internal/incidents"
	"github.com/DocHoax/watchdog/internal/storage"
	"github.com/DocHoax/watchdog/pkg/model"
)

func TestEvaluationContext_MetricsAndFreshness(t *testing.T) {
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	clock := NewMockClock(now)
	cfg := DefaultFreshnessConfig()

	lastTel := now.Add(-2 * time.Minute)
	ec := &EvaluationContext{
		Node: &model.FleetNode{
			Identity: model.NodeIdentity{
				NodeID:   "node-01",
				Hostname: "srv-01",
			},
			LastHeartbeat: now.Add(-1 * time.Minute),
			LastTelemetry: &lastTel,
			Summary: &model.NodeSummary{
				CPUUsagePercent:    45.0,
				MemoryUsagePercent: 60.0,
				DiskUsagePercent:   75.0,
			},
		},
		LatestTelemetry: &model.TelemetrySubmission{
			NodeID:    "node-01",
			Timestamp: now.Add(-30 * time.Second),
			Snapshot: &model.SystemSnapshot{
				Timestamp: now.Add(-30 * time.Second),
				CPU: &model.CPUInfo{
					OverallUsage: 55.0,
					LoadAverage: model.LoadAvg{
						Load1:  2.5,
						Load5:  1.8,
						Load15: 1.2,
					},
				},
				Memory: &model.MemoryInfo{
					UsedPercent:    70.0,
					AvailableBytes: 4 * 1024 * 1024 * 1024,
					TotalBytes:     16 * 1024 * 1024 * 1024,
				},
				Disk: &model.DiskInfo{
					UsedPercent: 80.0,
				},
			},
			Metrics: map[string]float64{
				"custom_queue_depth": 142.0,
			},
		},
		TelemetryHistory: []model.TelemetrySubmission{
			{
				Timestamp: now.Add(-10 * time.Minute),
				Metrics:   map[string]float64{"custom_queue_depth": 100.0},
			},
			{
				Timestamp: now.Add(-5 * time.Minute),
				Metrics:   map[string]float64{"custom_queue_depth": 120.0},
			},
			{
				Timestamp: now.Add(-30 * time.Second),
				Metrics:   map[string]float64{"custom_queue_depth": 142.0},
			},
		},
		Clock:           clock,
		FreshnessConfig: cfg,
	}

	t.Run("GetMetricValue from snapshot", func(t *testing.T) {
		val, freshness, found := ec.GetMetricValue("cpu_usage_pct")
		if !found {
			t.Fatalf("expected to find cpu_usage_pct")
		}
		if val != 55.0 {
			t.Errorf("expected CPU 55.0, got %f", val)
		}
		if freshness != model.DataFreshnessFresh {
			t.Errorf("expected Fresh data, got %s", freshness)
		}
	})

	t.Run("GetMetricValue from custom metrics", func(t *testing.T) {
		val, freshness, found := ec.GetMetricValue("custom_queue_depth")
		if !found {
			t.Fatalf("expected to find custom_queue_depth")
		}
		if val != 142.0 {
			t.Errorf("expected 142.0, got %f", val)
		}
		if freshness != model.DataFreshnessFresh {
			t.Errorf("expected Fresh data, got %s", freshness)
		}
	})

	t.Run("GetMetricValue stale when time advances", func(t *testing.T) {
		clock.Advance(10 * time.Minute)
		_, freshness, found := ec.GetMetricValue("cpu_usage_pct")
		if !found {
			t.Fatalf("expected to find metric")
		}
		if freshness != model.DataFreshnessStale {
			t.Errorf("expected Stale data after 10m advance, got %s", freshness)
		}
		clock.SetTime(now) // Reset clock
	})

	t.Run("GetMetricHistory", func(t *testing.T) {
		pts := ec.GetMetricHistory("custom_queue_depth")
		if len(pts) != 3 {
			t.Fatalf("expected 3 history points, got %d", len(pts))
		}
		if pts[0].Value != 100.0 || pts[2].Value != 142.0 {
			t.Errorf("unexpected history points order/values: %+v", pts)
		}
	})

	t.Run("GetHeartbeatFreshness", func(t *testing.T) {
		age, freshness := ec.GetHeartbeatFreshness()
		if age != 1*time.Minute {
			t.Errorf("expected heartbeat age 1m, got %s", age)
		}
		if freshness != model.DataFreshnessFresh {
			t.Errorf("expected Fresh heartbeat, got %s", freshness)
		}
	})
}

func TestDataAcquisitionProvider_BuildEvaluationContext(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	clock := NewMockClock(now)

	store, err := storage.NewSQLiteStorage(storage.Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("failed to create sqlite store: %v", err)
	}
	defer store.Close()

	// Seed node
	lastTelNode := now.Add(-30 * time.Second)
	node := &model.FleetNode{
		Identity: model.NodeIdentity{
			NodeID:   "node-prod-01",
			Hostname: "web-01.acme.corp",
			OS:       "linux",
			Platform: "ubuntu",
			Version:  "1.4.2",
			Tags:     map[string]string{"env": "production", "tier": "frontend"},
		},
		Status:        model.NodeStatusHealthy,
		RegisteredAt:  now.Add(-24 * time.Hour),
		LastHeartbeat: now.Add(-30 * time.Second),
		LastTelemetry: &lastTelNode,
		Metadata: map[string]string{
			"org_id": "org-acme",
		},
	}
	if err := store.SaveFleetNode(ctx, node); err != nil {
		t.Fatalf("failed to save fleet node: %v", err)
	}

	// Seed telemetry
	sub := &model.TelemetrySubmission{
		NodeID:    "node-prod-01",
		Timestamp: now.Add(-30 * time.Second),
		Sequence:  1,
		Snapshot: &model.SystemSnapshot{
			Timestamp: now.Add(-30 * time.Second),
			CPU:       &model.CPUInfo{OverallUsage: 42.0},
			Memory:    &model.MemoryInfo{UsedPercent: 65.0},
			Disk:      &model.DiskInfo{UsedPercent: 50.0},
		},
	}
	if err := store.SaveTelemetrySubmission(ctx, sub); err != nil {
		t.Fatalf("failed to save telemetry: %v", err)
	}

	// Seed incident
	inc := &incidents.Incident{
		ID:            "inc-001",
		Title:         "High CPU load",
		Status:        incidents.IncidentStatusInvestigating,
		Severity:      model.SeverityCritical,
		Scope:         incidents.IncidentScopeNode,
		Confidence:    "high",
		StartTime:     now.Add(-10 * time.Minute),
		UpdatedAt:     now.Add(-2 * time.Minute),
		AffectedNodes: []string{"node-prod-01"},
	}
	if err := store.SaveIncident(ctx, inc); err != nil {
		t.Fatalf("failed to save incident: %v", err)
	}

	resolver := NewPolicyResolver(store)
	provider := NewDataAcquisitionProvider(store, resolver, clock, nil)

	evalCtx, err := provider.BuildEvaluationContext(ctx, "node-prod-01")
	if err != nil {
		t.Fatalf("failed to build evaluation context: %v", err)
	}

	if evalCtx.Node.Identity.NodeID != "node-prod-01" {
		t.Errorf("expected node-prod-01, got %s", evalCtx.Node.Identity.NodeID)
	}
	if evalCtx.OrgID != "org-acme" {
		t.Errorf("expected org-acme, got %s", evalCtx.OrgID)
	}
	if evalCtx.LatestTelemetry == nil || evalCtx.LatestTelemetry.Snapshot.CPU.OverallUsage != 42.0 {
		t.Errorf("expected telemetry CPU 42.0, got %+v", evalCtx.LatestTelemetry)
	}
	if len(evalCtx.ActiveIncidents) != 1 || evalCtx.ActiveIncidents[0].ID != "inc-001" {
		t.Errorf("expected 1 active incident, got %d", len(evalCtx.ActiveIncidents))
	}
	if evalCtx.DataFreshness["telemetry"] != model.DataFreshnessFresh {
		t.Errorf("expected fresh telemetry, got %s", evalCtx.DataFreshness["telemetry"])
	}
}
