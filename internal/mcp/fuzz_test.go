package mcp

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/DocHoax/watchdog/internal/config"
	"github.com/DocHoax/watchdog/pkg/model"
)

// FuzzJSONRPCParse fuzzes JSON-RPC 2.0 unmarshaling and error formatting.
func FuzzJSONRPCParse(f *testing.F) {
	seeds := [][]byte{
		[]byte(`{"jsonrpc":"2.0","id":1,"method":"ping"}`),
		[]byte(`{"jsonrpc":"2.0","id":"abc-123","method":"tools/list"}`),
		[]byte(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"list_nodes","arguments":{"limit":10}}}`),
		[]byte(`{"jsonrpc":"2.0","id":3,"method":"resources/read","params":{"uri":"watchdog://fleet"}}`),
		[]byte(`{"jsonrpc":"1.0","id":4,"method":"ping"}`),
		[]byte(`{"jsonrpc":"2.0","method":"notifications/initialized"}`),
		[]byte(`{"invalid":json`),
		[]byte(`null`),
		[]byte(`""`),
		[]byte(`{"jsonrpc":"2.0","id":99999999999999999999,"method":"unknown"}`),
	}
	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		var req JSONRPCRequest
		if err := json.Unmarshal(data, &req); err != nil {
			return // Expected for malformed input
		}

		// Ensure that handling valid or invalid requests never panics
		srv := NewServer(
			config.DefaultConfig().MCP,
			nil, nil, nil, nil, nil, nil,
			model.NodeIdentity{NodeID: "fuzz-node", Hostname: "fuzz-host"},
		)

		mcpCtx := MCPContext{
			RequestID: "fuzz-req-id",
			ClientID:  "fuzz-client",
			Actor:     "fuzz",
			Timestamp: time.Now().UTC(),
		}

		resp := srv.HandleRequest(context.Background(), mcpCtx, &req)
		if resp != nil {
			// Ensure response marshals cleanly
			_, _ = json.Marshal(resp)
		}
	})
}

// FuzzValidateNodeID tests node ID validation against arbitrary strings.
func FuzzValidateNodeID(f *testing.F) {
	seeds := []string{
		"node-1",
		"node.prod_01",
		"550e8400-e29b-41d4-a716-446655440000",
		"",
		"   ",
		"../node-1",
		"node/1",
		"node\\1",
		"node\x00inject",
		"node; rm -rf /",
		"node' OR '1'='1",
		"node<script>alert(1)</script>",
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", // 129 chars
	}
	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		err := ValidateNodeID(input)
		if err == nil {
			// Invariant: validated node IDs must be 1-128 chars and contain only safe chars
			if len(input) == 0 || len(input) > 128 {
				t.Errorf("ValidateNodeID allowed invalid length %d for %q", len(input), input)
			}
		}
	})
}

// FuzzValidateMetricName tests metric name normalization and filtering.
func FuzzValidateMetricName(f *testing.F) {
	seeds := []string{
		"cpu",
		"memory",
		"disk",
		"load1",
		"cpu_usage_pct",
		"net_rx_bytes_sec",
		"",
		"drop table metrics;",
		"cpu; cat /etc/passwd",
		"cpu\x00name",
		"a/b/c",
		"metric!@#$%^&*()",
	}
	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		canonical, err := ValidateMetricName(input)
		if err == nil {
			if canonical == "" {
				t.Errorf("ValidateMetricName returned empty canonical for %q", input)
			}
		}
	})
}

// FuzzParseFlexibleDuration tests flexible duration parsing robustness.
func FuzzParseFlexibleDuration(f *testing.F) {
	seeds := []string{
		"15m",
		"1h",
		"24h",
		"7d",
		"30D",
		"0s",
		"-5m",
		"invalid",
		"999999999999999h",
		"",
		"10000000d",
	}
	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		dur, err := ParseFlexibleDuration(input, time.Hour)
		if err == nil {
			if dur < 0 {
				t.Errorf("ParseFlexibleDuration allowed negative duration: %v for %q", dur, input)
			}
		}
	})
}

// FuzzToolExecution tests ToolRegistry.Execute against arbitrary tool names and argument payloads.
func FuzzToolExecution(f *testing.F) {
	seeds := [][]byte{
		[]byte(`{"name":"list_nodes","args":{"limit":10,"status":"healthy"}}`),
		[]byte(`{"name":"get_node","args":{"node_id":"local-node"}}`),
		[]byte(`{"name":"get_node_health","args":{"node_id":"local-node"}}`),
		[]byte(`{"name":"get_node_snapshot","args":{"node_id":"local-node"}}`),
		[]byte(`{"name":"get_fleet_health","args":{}}`),
		[]byte(`{"name":"get_node_metrics","args":{"node_id":"local-node","metric":"cpu","since":"1h","limit":50}}`),
		[]byte(`{"name":"get_recent_diagnostics","args":{"severity":"WARNING","limit":10}}`),
		[]byte(`{"name":"get_active_alerts","args":{"severity":"CRITICAL","limit":5}}`),
		[]byte(`{"name":"non_existent_tool","args":{}}`),
		[]byte(`{"name":"execute_command","args":{"cmd":"ls"}}`),
		[]byte(`{"name":"delete_node","args":{"node_id":"local-node"}}`),
	}
	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		var payload struct {
			Name string         `json:"name"`
			Args map[string]any `json:"args"`
		}
		if err := json.Unmarshal(data, &payload); err != nil {
			return
		}

		registry := NewToolRegistry(nil, nil, nil, nil, nil, model.NodeIdentity{
			NodeID:   "fuzz-node",
			Hostname: "fuzz-host",
		})

		mcpCtx := MCPContext{
			RequestID: "fuzz-req",
			ClientID:  "fuzz-client",
			Actor:     "fuzz",
			Timestamp: time.Now().UTC(),
		}

		// Execute must never panic
		res, rpcErr := registry.Execute(context.Background(), mcpCtx, payload.Name, payload.Args)
		if !AllowedReadOperations[payload.Name] {
			if rpcErr == nil {
				t.Errorf("expected error for non-allowlisted tool %q, got success", payload.Name)
			}
		}
		if res != nil {
			for _, content := range res.Content {
				if content.Text == "" {
					t.Errorf("empty text content returned for tool %q", payload.Name)
				}
			}
		}
	})
}
