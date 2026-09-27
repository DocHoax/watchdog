package fleet

import (
	"context"
	"testing"
	"time"

	"github.com/DocHoax/watchdog/internal/config"
	"github.com/DocHoax/watchdog/internal/storage"
	"github.com/DocHoax/watchdog/pkg/model"
)

func setupTestService(t *testing.T) (FleetService, storage.Storage) {
	t.Helper()
	store, err := storage.NewSQLiteStorage(storage.Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("failed to create sqlite storage: %v", err)
	}

	fleetCfg := config.FleetConfig{
		Enabled:           true,
		HeartbeatInterval: 10 * time.Second,
		TelemetryInterval: 30 * time.Second,
		StaleThreshold:    200 * time.Millisecond,
		OfflineThreshold:  500 * time.Millisecond,
		RateLimitRate:     10.0,
		RateLimitBurst:    20,
	}

	svc := NewFleetService(store, fleetCfg)
	return svc, store
}

func TestFleetService_RegisterAndGetNode(t *testing.T) {
	svc, store := setupTestService(t)
	defer store.Close()

	ctx := context.Background()

	// 1. Invalid registration
	_, err := svc.RegisterNode(ctx, nil)
	if err == nil {
		t.Errorf("Expected error on nil registration request")
	}

	_, err = svc.RegisterNode(ctx, &model.NodeRegistrationRequest{})
	if err == nil {
		t.Errorf("Expected error on empty node ID")
	}

	// 2. Valid registration
	req := &model.NodeRegistrationRequest{
		Identity: model.NodeIdentity{
			NodeID:      "node-123",
			Hostname:    "host-123",
			OS:          "linux",
			Platform:    "ubuntu",
			Version:     "v1.0.0",
			CPUCores:    4,
			TotalMemory: 8 * 1024 * 1024 * 1024,
			Tags:        map[string]string{"env": "test"},
		},
		Metadata: map[string]string{"rack": "A1"},
	}

	resp, err := svc.RegisterNode(ctx, req)
	if err != nil {
		t.Fatalf("Failed to register node: %v", err)
	}
	if !resp.Registered || resp.NodeID != "node-123" {
		t.Errorf("Unexpected registration response: %+v", resp)
	}
	if resp.HeartbeatIntervalSeconds != 10 {
		t.Errorf("Expected HeartbeatIntervalSeconds 10, got %d", resp.HeartbeatIntervalSeconds)
	}

	// 3. Get registered node
	nodeDetail, err := svc.GetNode(ctx, "node-123")
	if err != nil {
		t.Fatalf("Failed to get node: %v", err)
	}
	if nodeDetail.Node.Identity.NodeID != "node-123" {
		t.Errorf("Expected node ID node-123, got %s", nodeDetail.Node.Identity.NodeID)
	}
	if nodeDetail.Node.Metadata["rack"] != "A1" {
		t.Errorf("Expected metadata rack=A1, got %v", nodeDetail.Node.Metadata)
	}
	if nodeDetail.Node.Status != model.NodeStatusHealthy {
		t.Errorf("Expected status healthy, got %s", nodeDetail.Node.Status)
	}

	// 4. Get non-existent node
	_, err = svc.GetNode(ctx, "non-existent")
	if err != ErrNodeNotFound {
		t.Errorf("Expected ErrNodeNotFound, got %v", err)
	}
}

func TestFleetService_ProcessHeartbeat_AndLifecycle(t *testing.T) {
	svc, store := setupTestService(t)
	defer store.Close()

	ctx := context.Background()

	hb := &model.HeartbeatRequest{
		NodeID:             "node-hb-1",
		Timestamp:          time.Now().UTC(),
		Status:             model.NodeStatusHealthy,
		CPUUsagePercent:    25.5,
		MemoryUsagePercent: 60.0,
		DiskUsagePercent:   40.0,
		Load1:              1.5,
		ActiveAlertsCount:  0,
		Version:            "v1.0.0",
		Tags:               map[string]string{"role": "worker"},
	}

	// 1. Process heartbeat (auto-registers node)
	resp, err := svc.ProcessHeartbeat(ctx, hb)
	if err != nil {
		t.Fatalf("Failed to process heartbeat: %v", err)
	}
	if !resp.Acknowledged || resp.NodeStatus != model.NodeStatusHealthy {
		t.Errorf("Unexpected heartbeat response: %+v", resp)
	}

	nodeDetail, err := svc.GetNode(ctx, "node-hb-1")
	if err != nil {
		t.Fatalf("Failed to get node: %v", err)
	}
	if nodeDetail.Node.Summary == nil || nodeDetail.Node.Summary.CPUUsagePercent != 25.5 {
		t.Errorf("Expected summary CPU 25.5, got %v", nodeDetail.Node.Summary)
	}

	// 2. Wait for status to become Stale (StaleThreshold = 200ms)
	time.Sleep(250 * time.Millisecond)
	nodeDetail, err = svc.GetNode(ctx, "node-hb-1")
	if err != nil {
		t.Fatalf("Failed to get node: %v", err)
	}
	if nodeDetail.Node.Status != model.NodeStatusStale {
		t.Errorf("Expected status stale, got %s", nodeDetail.Node.Status)
	}

	// 3. Wait for status to become Offline (OfflineThreshold = 500ms)
	time.Sleep(300 * time.Millisecond)
	nodeDetail, err = svc.GetNode(ctx, "node-hb-1")
	if err != nil {
		t.Fatalf("Failed to get node: %v", err)
	}
	if nodeDetail.Node.Status != model.NodeStatusOffline {
		t.Errorf("Expected status offline, got %s", nodeDetail.Node.Status)
	}
}

func TestFleetService_TelemetryAndSummary(t *testing.T) {
	svc, store := setupTestService(t)
	defer store.Close()

	ctx := context.Background()

	// Register node
	_, err := svc.RegisterNode(ctx, &model.NodeRegistrationRequest{
		Identity: model.NodeIdentity{
			NodeID:   "node-telem-1",
			Hostname: "host-telem",
			Tags:     map[string]string{"env": "prod"},
		},
	})
	if err != nil {
		t.Fatalf("Failed to register node: %v", err)
	}

	// Ingest telemetry
	sub := &model.TelemetrySubmission{
		NodeID:    "node-telem-1",
		Timestamp: time.Now().UTC(),
		Snapshot: &model.SystemSnapshot{
			Timestamp: time.Now().UTC(),
			CPU: &model.CPUInfo{
				OverallUsage: 45.0,
				LoadAverage:  model.LoadAvg{Load1: 2.0},
			},
			Memory: &model.MemoryInfo{
				UsedPercent: 70.0,
			},
			Disk: &model.DiskInfo{
				UsedPercent: 55.0,
			},
		},
		Diagnostics: &model.DiagnosticReport{
			OverallStatus: model.StatusPass,
			TotalChecks:   5,
			PassedChecks:  5,
		},
	}

	if err := svc.IngestTelemetry(ctx, sub); err != nil {
		t.Fatalf("Failed to ingest telemetry: %v", err)
	}

	// Get Node with telemetry
	nodeDetail, err := svc.GetNode(ctx, "node-telem-1")
	if err != nil {
		t.Fatalf("Failed to get node: %v", err)
	}
	if nodeDetail.LatestSnapshot == nil || nodeDetail.LatestSnapshot.CPU.OverallUsage != 45.0 {
		t.Errorf("Expected snapshot CPU 45.0, got %v", nodeDetail.LatestSnapshot)
	}
	if len(nodeDetail.RecentTelemetry) != 1 {
		t.Errorf("Expected 1 recent telemetry submission, got %d", len(nodeDetail.RecentTelemetry))
	}

	// List Nodes
	listResp, err := svc.ListNodes(ctx, model.FleetFilter{Limit: 10})
	if err != nil {
		t.Fatalf("Failed to list nodes: %v", err)
	}
	if listResp.Total != 1 || len(listResp.Nodes) != 1 {
		t.Errorf("Expected 1 node in list, got total=%d count=%d", listResp.Total, len(listResp.Nodes))
	}

	// Get Fleet Summary
	summary, err := svc.GetFleetSummary(ctx)
	if err != nil {
		t.Fatalf("Failed to get fleet summary: %v", err)
	}
	if summary.TotalNodes != 1 || summary.HealthyNodes != 1 {
		t.Errorf("Expected 1 total healthy node in summary, got %+v", summary)
	}

	// Delete Node
	if err := svc.DeleteNode(ctx, "node-telem-1"); err != nil {
		t.Fatalf("Failed to delete node: %v", err)
	}

	_, err = svc.GetNode(ctx, "node-telem-1")
	if err != ErrNodeNotFound {
		t.Errorf("Expected ErrNodeNotFound after delete, got %v", err)
	}
}
