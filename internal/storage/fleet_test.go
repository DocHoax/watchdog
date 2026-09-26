package storage

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/DocHoax/watchdog/pkg/model"
)

func TestSQLiteStorage_FleetNodes_CRUD(t *testing.T) {
	store, err := NewSQLiteStorage(Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("failed to create sqlite storage: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	now := time.Now().UTC()

	node1 := &model.FleetNode{
		Identity: model.NodeIdentity{
			NodeID:       "node-alpha-1",
			Hostname:     "web-prod-01",
			OS:           "linux",
			Platform:     "ubuntu",
			PlatformVer:  "22.04",
			Arch:         "amd64",
			KernelVer:    "5.15.0-generic",
			Version:      "v1.0.0",
			CPUCores:     8,
			TotalMemory:  16 * 1024 * 1024 * 1024,
			IPAddresses:  []string{"192.168.1.10", "10.0.0.10"},
			MACAddresses: []string{"00:11:22:33:44:55"},
			Tags:         map[string]string{"env": "prod", "tier": "frontend"},
			CreatedAt:    now.Add(-1 * time.Hour),
		},
		Status:        model.NodeStatusHealthy,
		StatusMessage: "All systems nominal",
		RegisteredAt:  now.Add(-1 * time.Hour),
		LastHeartbeat: now.Add(-30 * time.Second),
		Metadata:      map[string]string{"region": "us-east-1", "datacenter": "dc1"},
	}

	node2 := &model.FleetNode{
		Identity: model.NodeIdentity{
			NodeID:       "node-beta-2",
			Hostname:     "db-prod-01",
			OS:           "linux",
			Platform:     "debian",
			PlatformVer:  "12.0",
			Arch:         "amd64",
			KernelVer:    "6.1.0",
			Version:      "v1.0.0",
			CPUCores:     16,
			TotalMemory:  64 * 1024 * 1024 * 1024,
			IPAddresses:  []string{"192.168.1.20"},
			MACAddresses: []string{"00:11:22:33:44:66"},
			Tags:         map[string]string{"env": "prod", "tier": "database"},
			CreatedAt:    now.Add(-2 * time.Hour),
		},
		Status:        model.NodeStatusWarning,
		StatusMessage: "High memory utilization",
		RegisteredAt:  now.Add(-2 * time.Hour),
		LastHeartbeat: now.Add(-10 * time.Second),
		Metadata:      map[string]string{"region": "us-west-2"},
	}

	// 1. Save nodes
	if err := store.SaveFleetNode(ctx, node1); err != nil {
		t.Fatalf("failed to save node1: %v", err)
	}
	if err := store.SaveFleetNode(ctx, node2); err != nil {
		t.Fatalf("failed to save node2: %v", err)
	}

	// 2. Get node by ID
	retrieved, err := store.GetFleetNode(ctx, "node-alpha-1")
	if err != nil {
		t.Fatalf("failed to get node1: %v", err)
	}
	if retrieved == nil {
		t.Fatalf("expected node1, got nil")
	}
	if retrieved.Identity.Hostname != "web-prod-01" {
		t.Errorf("expected hostname web-prod-01, got %s", retrieved.Identity.Hostname)
	}
	if retrieved.Identity.Tags["tier"] != "frontend" {
		t.Errorf("expected tag tier=frontend, got %v", retrieved.Identity.Tags)
	}
	if retrieved.Metadata["region"] != "us-east-1" {
		t.Errorf("expected metadata region=us-east-1, got %v", retrieved.Metadata)
	}
	if len(retrieved.Identity.IPAddresses) != 2 {
		t.Errorf("expected 2 IP addresses, got %d", len(retrieved.Identity.IPAddresses))
	}

	// 3. Update existing node (UPSERT)
	node1.Status = model.NodeStatusCritical
	node1.StatusMessage = "Disk almost full"
	node1.LastHeartbeat = now
	if err := store.SaveFleetNode(ctx, node1); err != nil {
		t.Fatalf("failed to update node1: %v", err)
	}

	updated, err := store.GetFleetNode(ctx, "node-alpha-1")
	if err != nil {
		t.Fatalf("failed to get updated node1: %v", err)
	}
	if updated.Status != model.NodeStatusCritical {
		t.Errorf("expected status critical, got %s", updated.Status)
	}
	if updated.StatusMessage != "Disk almost full" {
		t.Errorf("expected status message 'Disk almost full', got %s", updated.StatusMessage)
	}

	// 4. List nodes with filter
	nodes, total, err := store.ListFleetNodes(ctx, model.FleetFilter{})
	if err != nil {
		t.Fatalf("failed to list nodes: %v", err)
	}
	if total != 2 || len(nodes) != 2 {
		t.Errorf("expected 2 nodes total, got total=%d len=%d", total, len(nodes))
	}

	// 5. Filter by status
	critNodes, critTotal, err := store.ListFleetNodes(ctx, model.FleetFilter{Status: model.NodeStatusCritical})
	if err != nil {
		t.Fatalf("failed to list critical nodes: %v", err)
	}
	if critTotal != 1 || len(critNodes) != 1 || critNodes[0].Identity.NodeID != "node-alpha-1" {
		t.Errorf("expected 1 critical node (node-alpha-1), got %v", critNodes)
	}

	// 6. Search filter (by hostname or IP)
	searchNodes, searchTotal, err := store.ListFleetNodes(ctx, model.FleetFilter{Search: "db-prod"})
	if err != nil {
		t.Fatalf("failed to search nodes: %v", err)
	}
	if searchTotal != 1 || len(searchNodes) != 1 || searchNodes[0].Identity.NodeID != "node-beta-2" {
		t.Errorf("expected db-prod-01 node, got %v", searchNodes)
	}

	// 7. Delete node
	if err := store.DeleteFleetNode(ctx, "node-alpha-1"); err != nil {
		t.Fatalf("failed to delete node1: %v", err)
	}
	deleted, err := store.GetFleetNode(ctx, "node-alpha-1")
	if err != nil {
		t.Fatalf("unexpected error getting deleted node: %v", err)
	}
	if deleted != nil {
		t.Errorf("expected nil for deleted node, got %v", deleted)
	}
}

func TestSQLiteStorage_TelemetrySubmissions(t *testing.T) {
	store, err := NewSQLiteStorage(Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("failed to create sqlite storage: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	now := time.Now().UTC()

	// Register a fleet node first
	node := &model.FleetNode{
		Identity: model.NodeIdentity{
			NodeID:   "node-telemetry-test",
			Hostname: "telemetry-host",
			OS:       "linux",
			Platform: "ubuntu",
			Version:  "v1.0.0",
		},
		Status:        model.NodeStatusHealthy,
		RegisteredAt:  now.Add(-10 * time.Minute),
		LastHeartbeat: now.Add(-1 * time.Minute),
	}
	if err := store.SaveFleetNode(ctx, node); err != nil {
		t.Fatalf("failed to save node: %v", err)
	}

	// 1. Submit telemetry batches
	sub1 := &model.TelemetrySubmission{
		NodeID:    "node-telemetry-test",
		Timestamp: now.Add(-5 * time.Minute),
		Sequence:  1,
		Snapshot: &model.SystemSnapshot{
			Timestamp: now.Add(-5 * time.Minute),
			Host: model.HostInfo{
				Hostname:       "telemetry-host",
				PlatformFamily: "ubuntu",
			},
			CPU: &model.CPUInfo{
				TotalUsage: 25.5,
				Cores:      4,
			},
			Memory: &model.MemoryInfo{
				UsedPercent: 60.0,
			},
			Load: &model.LoadInfo{
				Load1: 1.25,
			},
			Disks: []model.DiskInfo{
				{UsedPercent: 45.0},
			},
		},
		Metrics: map[string]float64{
			"cpu_usage": 25.5,
			"mem_usage": 60.0,
		},
		Tags: map[string]string{"env": "test"},
	}

	sub2 := &model.TelemetrySubmission{
		NodeID:    "node-telemetry-test",
		Timestamp: now,
		Sequence:  2,
		Snapshot: &model.SystemSnapshot{
			Timestamp: now,
			Host: model.HostInfo{
				Hostname:       "telemetry-host",
				PlatformFamily: "ubuntu",
			},
			CPU: &model.CPUInfo{
				TotalUsage: 88.0,
				Cores:      4,
			},
			Memory: &model.MemoryInfo{
				UsedPercent: 82.5,
			},
			Load: &model.LoadInfo{
				Load1: 3.50,
			},
			Disks: []model.DiskInfo{
				{UsedPercent: 50.0},
			},
		},
		ActiveAlerts: []model.AlertEvent{
			{
				ID:          "alert-01",
				RuleName:    "high-cpu",
				Severity:    model.SeverityWarning,
				Status:      model.AlertStatusFiring,
				TriggeredAt: now,
			},
		},
		Metrics: map[string]float64{
			"cpu_usage": 88.0,
			"mem_usage": 82.5,
		},
		Tags: map[string]string{"env": "test"},
	}

	if err := store.SaveTelemetrySubmission(ctx, sub1); err != nil {
		t.Fatalf("failed to save telemetry sub1: %v", err)
	}
	if err := store.SaveTelemetrySubmission(ctx, sub2); err != nil {
		t.Fatalf("failed to save telemetry sub2: %v", err)
	}

	// 2. Query telemetry submissions
	subs, err := store.GetNodeTelemetrySubmissions(ctx, "node-telemetry-test", now.Add(-10*time.Minute), 10)
	if err != nil {
		t.Fatalf("failed to query telemetry: %v", err)
	}
	if len(subs) != 2 {
		t.Fatalf("expected 2 telemetry submissions, got %d", len(subs))
	}

	// Ordered by timestamp DESC: sub2, then sub1
	if subs[0].Sequence != 2 {
		t.Errorf("expected first submission sequence 2, got %d", subs[0].Sequence)
	}
	if len(subs[0].ActiveAlerts) != 1 {
		t.Errorf("expected 1 active alert in sub2, got %d", len(subs[0].ActiveAlerts))
	}
	if subs[0].Snapshot.CPU.TotalUsage != 88.0 {
		t.Errorf("expected 88.0 CPU usage in sub2, got %f", subs[0].Snapshot.CPU.TotalUsage)
	}

	// 3. Verify node summary was updated in fleet_nodes
	updatedNode, err := store.GetFleetNode(ctx, "node-telemetry-test")
	if err != nil {
		t.Fatalf("failed to get updated node: %v", err)
	}
	if updatedNode.Summary == nil {
		t.Fatalf("expected node summary to be populated, got nil")
	}
	if updatedNode.Summary.CPUUsagePercent != 88.0 {
		t.Errorf("expected summary CPU 88.0, got %f", updatedNode.Summary.CPUUsagePercent)
	}
	if updatedNode.Summary.ActiveAlertsCount != 1 {
		t.Errorf("expected summary active alerts count 1, got %d", updatedNode.Summary.ActiveAlertsCount)
	}

	// 4. Prune telemetry older than 2 minutes (should delete sub1, keep sub2)
	pruned, err := store.PruneFleetTelemetry(ctx, 2*time.Minute)
	if err != nil {
		t.Fatalf("failed to prune telemetry: %v", err)
	}
	if pruned != 1 {
		t.Errorf("expected 1 pruned submission, got %d", pruned)
	}

	remainingSubs, err := store.GetNodeTelemetrySubmissions(ctx, "node-telemetry-test", time.Time{}, 10)
	if err != nil {
		t.Fatalf("failed to query remaining telemetry: %v", err)
	}
	if len(remainingSubs) != 1 || remainingSubs[0].Sequence != 2 {
		t.Errorf("expected only sub2 to remain, got %v", remainingSubs)
	}
}

func TestSQLiteStorage_FleetNodes_ConcurrentOperations(t *testing.T) {
	store, err := NewSQLiteStorage(Config{Path: ":memory:", MaxConns: 10})
	if err != nil {
		t.Fatalf("failed to create sqlite storage: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	const numNodes = 20
	var wg sync.WaitGroup
	errCh := make(chan error, numNodes*2)

	for i := 0; i < numNodes; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			node := &model.FleetNode{
				Identity: model.NodeIdentity{
					NodeID:   fmt.Sprintf("node-concurrent-%d", idx),
					Hostname: fmt.Sprintf("worker-%d", idx),
					OS:       "linux",
					Platform: "alpine",
					Version:  "v1.0.0",
				},
				Status:        model.NodeStatusHealthy,
				RegisteredAt:  time.Now().UTC(),
				LastHeartbeat: time.Now().UTC(),
			}
			if err := store.SaveFleetNode(ctx, node); err != nil {
				errCh <- fmt.Errorf("failed saving node %d: %w", idx, err)
				return
			}

			sub := &model.TelemetrySubmission{
				NodeID:    fmt.Sprintf("node-concurrent-%d", idx),
				Timestamp: time.Now().UTC(),
				Sequence:  1,
				Metrics:   map[string]float64{"cpu": float64(idx * 2)},
			}
			if err := store.SaveTelemetrySubmission(ctx, sub); err != nil {
				errCh <- fmt.Errorf("failed saving telemetry for node %d: %w", idx, err)
				return
			}
		}(i)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("concurrent operation error: %v", err)
	}

	nodes, total, err := store.ListFleetNodes(ctx, model.FleetFilter{Limit: 100})
	if err != nil {
		t.Fatalf("failed to list nodes: %v", err)
	}
	if total != numNodes || len(nodes) != numNodes {
		t.Errorf("expected %d nodes, got total=%d len=%d", numNodes, total, len(nodes))
	}
}
