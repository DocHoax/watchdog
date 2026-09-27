package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/DocHoax/watchdog/internal/alerts"
	"github.com/DocHoax/watchdog/internal/config"
	"github.com/DocHoax/watchdog/internal/fleet"
	"github.com/DocHoax/watchdog/internal/storage"
	"github.com/DocHoax/watchdog/pkg/model"
)

func TestToolDefinitions(t *testing.T) {
	tools := ToolDefinitions()
	if len(tools) != 13 {
		t.Fatalf("expected 13 tool definitions, got %d", len(tools))
	}

	expectedTools := map[string]bool{
		"list_nodes":                false,
		"get_node":                  false,
		"get_node_health":           false,
		"get_node_snapshot":         false,
		"get_fleet_health":          false,
		"get_node_metrics":          false,
		"get_recent_diagnostics":    false,
		"get_active_alerts":         false,
		"get_fleet_intelligence":    false,
		"get_node_intelligence":     false,
		"get_fleet_incidents":       false,
		"get_intelligence_findings": false,
		"get_node_trends":           false,
	}

	for _, tool := range tools {
		if _, ok := expectedTools[tool.Name]; !ok {
			t.Errorf("unexpected tool name: %s", tool.Name)
		}
		expectedTools[tool.Name] = true

		if tool.Description == "" {
			t.Errorf("tool %s missing description", tool.Name)
		}
		if tool.InputSchema.Type != "object" {
			t.Errorf("tool %s input schema type must be 'object', got %s", tool.Name, tool.InputSchema.Type)
		}
		if tool.InputSchema.Properties == nil {
			t.Errorf("tool %s missing properties schema", tool.Name)
		}
	}

	for name, found := range expectedTools {
		if !found {
			t.Errorf("expected tool %s not found in definitions", name)
		}
	}
}

func TestToolRegistry_FleetMode(t *testing.T) {
	ctx := context.Background()
	localID := model.NodeIdentity{
		NodeID:    "node-prod-01",
		Hostname:  "prod-host-01",
		CreatedAt: time.Now(),
	}

	store, err := storage.NewSQLiteStorage(storage.Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("failed to create sqlite: %v", err)
	}
	defer store.Close()

	fleetSvc := fleet.NewFleetService(store, config.FleetConfig{
		RateLimitRate:  100.0,
		RateLimitBurst: 200,
	})

	// Register a test node
	_, err = fleetSvc.RegisterNode(ctx, &model.NodeRegistrationRequest{
		Identity: localID,
	})
	if err != nil {
		t.Fatalf("failed to register node: %v", err)
	}

	// Register a secondary node
	node2ID := model.NodeIdentity{
		NodeID:    "node-prod-02",
		Hostname:  "prod-host-02",
		CreatedAt: time.Now(),
	}
	_, err = fleetSvc.RegisterNode(ctx, &model.NodeRegistrationRequest{
		Identity: node2ID,
	})
	if err != nil {
		t.Fatalf("failed to register second node: %v", err)
	}

	// Ingest telemetry for node 1
	_ = fleetSvc.IngestTelemetry(ctx, &model.TelemetrySubmission{
		NodeID:    "node-prod-01",
		Timestamp: time.Now(),
		Snapshot: &model.SystemSnapshot{
			Timestamp: time.Now(),
			System: &model.SystemInfo{
				Hostname: "prod-host-01",
				Platform: "linux",
			},
			CPU: &model.CPUInfo{
				OverallUsage: 35.5,
			},
			Memory: &model.MemoryInfo{
				UsedPercent: 60.0,
			},
		},
		Metrics: map[string]float64{
			"cpu":    35.5,
			"memory": 60.0,
		},
	})

	registry := NewToolRegistry(fleetSvc, store, nil, nil, nil, localID)
	mcpCtx := MCPContext{RequestID: "req-1", ClientID: "ai-agent"}

	t.Run("list_nodes", func(t *testing.T) {
		res, jerr := registry.Execute(ctx, mcpCtx, "list_nodes", map[string]any{
			"limit": 10,
		})
		if jerr != nil {
			t.Fatalf("unexpected error: %v", jerr)
		}
		if len(res.Content) != 1 {
			t.Fatalf("expected 1 content block, got %d", len(res.Content))
		}

		var listResp model.FleetListResponse
		if err := json.Unmarshal([]byte(res.Content[0].Text), &listResp); err != nil {
			t.Fatalf("failed to unmarshal list_nodes response: %v", err)
		}
		if listResp.Total < 2 {
			t.Errorf("expected at least 2 nodes, got %d", listResp.Total)
		}
	})

	t.Run("get_node valid", func(t *testing.T) {
		res, jerr := registry.Execute(ctx, mcpCtx, "get_node", map[string]any{
			"node_id": "node-prod-01",
		})
		if jerr != nil {
			t.Fatalf("unexpected error: %v", jerr)
		}
		if !strings.Contains(res.Content[0].Text, "node-prod-01") {
			t.Errorf("expected node_id in response text, got: %s", res.Content[0].Text)
		}
	})

	t.Run("get_node not found", func(t *testing.T) {
		_, jerr := registry.Execute(ctx, mcpCtx, "get_node", map[string]any{
			"node_id": "node-nonexistent",
		})
		if jerr == nil {
			t.Fatalf("expected error for nonexistent node")
		}
		if jerr.Code != CodeNodeNotFound {
			t.Errorf("expected CodeNodeNotFound, got: %d", jerr.Code)
		}
	})

	t.Run("get_node invalid ID format", func(t *testing.T) {
		_, jerr := registry.Execute(ctx, mcpCtx, "get_node", map[string]any{
			"node_id": "node/../invalid",
		})
		if jerr == nil {
			t.Fatalf("expected error for invalid node ID")
		}
		if jerr.Code != CodeInvalidNodeID {
			t.Errorf("expected CodeInvalidNodeID, got: %d", jerr.Code)
		}
	})

	t.Run("get_node_health", func(t *testing.T) {
		res, jerr := registry.Execute(ctx, mcpCtx, "get_node_health", map[string]any{
			"node_id": "node-prod-01",
		})
		if jerr != nil {
			t.Fatalf("unexpected error: %v", jerr)
		}
		if !strings.Contains(res.Content[0].Text, "node-prod-01") {
			t.Errorf("expected health response to contain node-prod-01, got: %s", res.Content[0].Text)
		}
	})

	t.Run("get_node_snapshot", func(t *testing.T) {
		res, jerr := registry.Execute(ctx, mcpCtx, "get_node_snapshot", map[string]any{
			"node_id": "node-prod-01",
		})
		if jerr != nil {
			t.Fatalf("unexpected error: %v", jerr)
		}
		if !strings.Contains(res.Content[0].Text, "prod-host-01") {
			t.Errorf("expected snapshot to contain hostname prod-host-01, got: %s", res.Content[0].Text)
		}
	})

	t.Run("get_fleet_health", func(t *testing.T) {
		res, jerr := registry.Execute(ctx, mcpCtx, "get_fleet_health", map[string]any{})
		if jerr != nil {
			t.Fatalf("unexpected error: %v", jerr)
		}
		if !strings.Contains(res.Content[0].Text, "total_nodes") && !strings.Contains(res.Content[0].Text, "TotalNodes") {
			t.Errorf("expected fleet health summary, got: %s", res.Content[0].Text)
		}
	})

	t.Run("get_node_metrics valid", func(t *testing.T) {
		res, jerr := registry.Execute(ctx, mcpCtx, "get_node_metrics", map[string]any{
			"node_id": "node-prod-01",
			"metric":  "cpu",
			"since":   "1h",
		})
		if jerr != nil {
			t.Fatalf("unexpected error: %v", jerr)
		}
		if len(res.Content) != 1 {
			t.Fatalf("expected 1 content block")
		}
	})

	t.Run("get_node_metrics invalid metric", func(t *testing.T) {
		_, jerr := registry.Execute(ctx, mcpCtx, "get_node_metrics", map[string]any{
			"node_id": "node-prod-01",
			"metric":  "bad/metric/name!",
		})
		if jerr == nil {
			t.Fatalf("expected error for invalid metric name")
		}
		if jerr.Code != CodeInvalidParams {
			t.Errorf("expected CodeInvalidParams, got: %d", jerr.Code)
		}
	})

	t.Run("get_recent_diagnostics", func(t *testing.T) {
		res, jerr := registry.Execute(ctx, mcpCtx, "get_recent_diagnostics", map[string]any{
			"severity": "warn",
			"limit":    10,
		})
		if jerr != nil {
			t.Fatalf("unexpected error: %v", jerr)
		}
		if len(res.Content) != 1 {
			t.Fatalf("expected 1 content block")
		}
	})

	t.Run("get_active_alerts", func(t *testing.T) {
		res, jerr := registry.Execute(ctx, mcpCtx, "get_active_alerts", map[string]any{
			"severity": "critical",
			"limit":    5,
		})
		if jerr != nil {
			t.Fatalf("unexpected error: %v", jerr)
		}
		if len(res.Content) != 1 {
			t.Fatalf("expected 1 content block")
		}
	})

	t.Run("get_fleet_intelligence", func(t *testing.T) {
		res, jerr := registry.Execute(ctx, mcpCtx, "get_fleet_intelligence", map[string]any{})
		if jerr != nil {
			t.Fatalf("unexpected error: %v", jerr)
		}
		if len(res.Content) != 1 {
			t.Fatalf("expected 1 content block")
		}
		if !strings.Contains(res.Content[0].Text, "total_nodes") && !strings.Contains(res.Content[0].Text, "TotalNodes") {
			t.Errorf("expected fleet intelligence summary, got: %s", res.Content[0].Text)
		}
	})

	t.Run("get_node_intelligence valid", func(t *testing.T) {
		res, jerr := registry.Execute(ctx, mcpCtx, "get_node_intelligence", map[string]any{
			"node_id": "node-prod-01",
		})
		if jerr != nil {
			t.Fatalf("unexpected error: %v", jerr)
		}
		if len(res.Content) != 1 {
			t.Fatalf("expected 1 content block")
		}
		if !strings.Contains(res.Content[0].Text, "node-prod-01") {
			t.Errorf("expected node intelligence to contain node-prod-01, got: %s", res.Content[0].Text)
		}
	})

	t.Run("get_node_intelligence not found", func(t *testing.T) {
		_, jerr := registry.Execute(ctx, mcpCtx, "get_node_intelligence", map[string]any{
			"node_id": "nonexistent-node",
		})
		if jerr == nil {
			t.Fatalf("expected error for nonexistent node")
		}
		if jerr.Code != CodeNodeNotFound {
			t.Errorf("expected CodeNodeNotFound, got: %d", jerr.Code)
		}
	})

	t.Run("get_fleet_incidents", func(t *testing.T) {
		res, jerr := registry.Execute(ctx, mcpCtx, "get_fleet_incidents", map[string]any{})
		if jerr != nil {
			t.Fatalf("unexpected error: %v", jerr)
		}
		if len(res.Content) != 1 {
			t.Fatalf("expected 1 content block")
		}
	})

	t.Run("get_intelligence_findings", func(t *testing.T) {
		res, jerr := registry.Execute(ctx, mcpCtx, "get_intelligence_findings", map[string]any{
			"category":     "fleet_pattern",
			"min_severity": "warning",
		})
		if jerr != nil {
			t.Fatalf("unexpected error: %v", jerr)
		}
		if len(res.Content) != 1 {
			t.Fatalf("expected 1 content block")
		}
	})

	t.Run("get_node_trends", func(t *testing.T) {
		res, jerr := registry.Execute(ctx, mcpCtx, "get_node_trends", map[string]any{
			"node_id": "node-prod-01",
			"window":  "1h",
		})
		if jerr != nil {
			t.Fatalf("unexpected error: %v", jerr)
		}
		if len(res.Content) != 1 {
			t.Fatalf("expected 1 content block")
		}
		if !strings.Contains(res.Content[0].Text, "node-prod-01") {
			t.Errorf("expected trends to contain node-prod-01, got: %s", res.Content[0].Text)
		}
	})

	t.Run("unknown tool name", func(t *testing.T) {
		_, jerr := registry.Execute(ctx, mcpCtx, "execute_shell_command", map[string]any{})
		if jerr == nil {
			t.Fatalf("expected error for unknown tool")
		}
		if jerr.Code != CodeMethodNotFound {
			t.Errorf("expected CodeMethodNotFound, got: %d", jerr.Code)
		}
	})
}

func TestToolRegistry_StandaloneMode(t *testing.T) {
	ctx := context.Background()
	localID := model.NodeIdentity{
		NodeID:    "standalone-node",
		Hostname:  "localhost",
		CreatedAt: time.Now(),
	}

	alertEng := alerts.NewEngine(config.DefaultConfig(), nil)
	registry := NewToolRegistry(nil, nil, nil, nil, alertEng, localID)
	mcpCtx := MCPContext{RequestID: "req-sa", ClientID: "standalone-client"}

	t.Run("list_nodes standalone", func(t *testing.T) {
		res, jerr := registry.Execute(ctx, mcpCtx, "list_nodes", map[string]any{})
		if jerr != nil {
			t.Fatalf("unexpected error: %v", jerr)
		}
		if !strings.Contains(res.Content[0].Text, "standalone-node") {
			t.Errorf("expected standalone node in list: %s", res.Content[0].Text)
		}
	})

	t.Run("get_node standalone local", func(t *testing.T) {
		res, jerr := registry.Execute(ctx, mcpCtx, "get_node", map[string]any{
			"node_id": "self",
		})
		if jerr != nil {
			t.Fatalf("unexpected error: %v", jerr)
		}
		if !strings.Contains(res.Content[0].Text, "standalone-node") {
			t.Errorf("expected standalone node details: %s", res.Content[0].Text)
		}
	})

	t.Run("get_node_health standalone", func(t *testing.T) {
		res, jerr := registry.Execute(ctx, mcpCtx, "get_node_health", map[string]any{
			"node_id": "localhost",
		})
		if jerr != nil {
			t.Fatalf("unexpected error: %v", jerr)
		}
		if !strings.Contains(res.Content[0].Text, "standalone-node") {
			t.Errorf("expected standalone node health: %s", res.Content[0].Text)
		}
	})

	t.Run("get_fleet_health standalone", func(t *testing.T) {
		res, jerr := registry.Execute(ctx, mcpCtx, "get_fleet_health", map[string]any{})
		if jerr != nil {
			t.Fatalf("unexpected error: %v", jerr)
		}
		if !strings.Contains(res.Content[0].Text, "1") {
			t.Errorf("expected total_nodes 1: %s", res.Content[0].Text)
		}
	})
}
