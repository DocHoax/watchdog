package mcp

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/DocHoax/watchdog/internal/config"
	"github.com/DocHoax/watchdog/internal/fleet"
	"github.com/DocHoax/watchdog/internal/storage"
	"github.com/DocHoax/watchdog/pkg/model"
)

func TestResourceRegistry(t *testing.T) {
	ctx := context.Background()
	localID := model.NodeIdentity{
		NodeID:    "test-node-01",
		Hostname:  "test-host",
		CreatedAt: time.Now(),
	}

	store, err := storage.NewSQLiteStorage(storage.Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("failed to initialize sqlite: %v", err)
	}
	defer store.Close()

	fleetSvc := fleet.NewFleetService(store, config.FleetConfig{
		HeartbeatInterval: 30 * time.Second,
		TelemetryInterval: 60 * time.Second,
		StaleThreshold:    2 * time.Minute,
		OfflineThreshold:  10 * time.Minute,
		RateLimitRate:     10.0,
		RateLimitBurst:    20,
	})

	_, err = fleetSvc.RegisterNode(ctx, &model.NodeRegistrationRequest{
		Identity: localID,
	})
	if err != nil {
		t.Fatalf("failed to register node: %v", err)
	}

	registry := NewResourceRegistry(fleetSvc, store, nil, nil, nil, localID)

	t.Run("ListResources", func(t *testing.T) {
		resources, err := registry.ListResources(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resources) < 5 {
			t.Fatalf("expected at least 5 resources, got %d", len(resources))
		}

		uris := make(map[string]bool)
		for _, r := range resources {
			uris[r.URI] = true
		}

		expected := []string{
			"watchdog://fleet",
			"watchdog://fleet/test-node-01",
			"watchdog://fleet/test-node-01/health",
			"watchdog://fleet/test-node-01/snapshot",
			"watchdog://fleet/test-node-01/alerts",
			"intelligence://fleet/summary",
			"intelligence://incidents/active",
			"intelligence://nodes/test-node-01/summary",
		}
		for _, uri := range expected {
			if !uris[uri] {
				t.Errorf("expected resource URI %s in list", uri)
			}
		}
	})

	t.Run("ReadResource fleet overview", func(t *testing.T) {
		mcpCtx := MCPContext{RequestID: "req-1"}
		res, jerr := registry.ReadResource(ctx, mcpCtx, "watchdog://fleet")
		if jerr != nil {
			t.Fatalf("unexpected error: %v", jerr)
		}
		if len(res.Contents) != 1 {
			t.Fatalf("expected 1 content block, got %d", len(res.Contents))
		}
		if !strings.Contains(res.Contents[0].Text, "total_nodes") && !strings.Contains(res.Contents[0].Text, "TotalNodes") {
			t.Errorf("expected content to contain fleet summary, got: %s", res.Contents[0].Text)
		}
	})

	t.Run("ReadResource intelligence fleet summary", func(t *testing.T) {
		mcpCtx := MCPContext{RequestID: "req-intel-fleet"}
		res, jerr := registry.ReadResource(ctx, mcpCtx, "intelligence://fleet/summary")
		if jerr != nil {
			t.Fatalf("unexpected error: %v", jerr)
		}
		if len(res.Contents) != 1 {
			t.Fatalf("expected 1 content block, got %d", len(res.Contents))
		}
		if !strings.Contains(res.Contents[0].Text, "total_nodes") && !strings.Contains(res.Contents[0].Text, "TotalNodes") {
			t.Errorf("expected content to contain fleet intelligence, got: %s", res.Contents[0].Text)
		}
	})

	t.Run("ReadResource intelligence active incidents", func(t *testing.T) {
		mcpCtx := MCPContext{RequestID: "req-intel-inc"}
		res, jerr := registry.ReadResource(ctx, mcpCtx, "intelligence://incidents/active")
		if jerr != nil {
			t.Fatalf("unexpected error: %v", jerr)
		}
		if len(res.Contents) != 1 {
			t.Fatalf("expected 1 content block, got %d", len(res.Contents))
		}
	})

	t.Run("ReadResource intelligence node summary", func(t *testing.T) {
		mcpCtx := MCPContext{RequestID: "req-intel-node"}
		res, jerr := registry.ReadResource(ctx, mcpCtx, "intelligence://nodes/test-node-01/summary")
		if jerr != nil {
			t.Fatalf("unexpected error: %v", jerr)
		}
		if len(res.Contents) != 1 {
			t.Fatalf("expected 1 content block, got %d", len(res.Contents))
		}
		if !strings.Contains(res.Contents[0].Text, "test-node-01") {
			t.Errorf("expected node intelligence to contain test-node-01, got: %s", res.Contents[0].Text)
		}
	})

	t.Run("ReadResource node details", func(t *testing.T) {
		mcpCtx := MCPContext{RequestID: "req-2"}
		res, jerr := registry.ReadResource(ctx, mcpCtx, "watchdog://fleet/test-node-01")
		if jerr != nil {
			t.Fatalf("unexpected error: %v", jerr)
		}
		if !strings.Contains(res.Contents[0].Text, "test-node-01") {
			t.Errorf("expected content to contain node_id, got: %s", res.Contents[0].Text)
		}
	})

	t.Run("ReadResource node health", func(t *testing.T) {
		mcpCtx := MCPContext{RequestID: "req-3"}
		res, jerr := registry.ReadResource(ctx, mcpCtx, "watchdog://fleet/test-node-01/health")
		if jerr != nil {
			t.Fatalf("unexpected error: %v", jerr)
		}
		if !strings.Contains(res.Contents[0].Text, "test-node-01") {
			t.Errorf("expected content to contain health data, got: %s", res.Contents[0].Text)
		}
	})

	t.Run("ReadResource not found", func(t *testing.T) {
		mcpCtx := MCPContext{RequestID: "req-4"}
		_, jerr := registry.ReadResource(ctx, mcpCtx, "watchdog://fleet/nonexistent-node")
		if jerr == nil {
			t.Fatalf("expected error for nonexistent node")
		}
		if jerr.Code != CodeNodeNotFound {
			t.Errorf("expected CodeNodeNotFound (%d), got %d", CodeNodeNotFound, jerr.Code)
		}

		_, jerr = registry.ReadResource(ctx, mcpCtx, "other://invalid/uri")
		if jerr == nil {
			t.Fatalf("expected error for invalid URI prefix")
		}
		if jerr.Code != CodeResourceNotFound {
			t.Errorf("expected CodeResourceNotFound (%d), got %d", CodeResourceNotFound, jerr.Code)
		}
	})
}
