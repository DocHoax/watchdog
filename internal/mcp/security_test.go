package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DocHoax/watchdog/internal/config"
	"github.com/DocHoax/watchdog/pkg/model"
)

func TestIsLoopback(t *testing.T) {
	tests := []struct {
		addr     string
		expected bool
	}{
		{"", true},
		{"localhost", true},
		{"127.0.0.1", true},
		{"127.0.0.2", true},
		{"::1", true},
		{"0.0.0.0", false},
		{"192.168.1.50", false},
		{"10.0.0.1", false},
		{"example.com", false},
	}

	for _, tt := range tests {
		got := IsLoopback(tt.addr)
		if got != tt.expected {
			t.Errorf("IsLoopback(%q) = %v; expected %v", tt.addr, got, tt.expected)
		}
	}
}

func TestValidateMCPSecurity(t *testing.T) {
	t.Run("nil config returns error", func(t *testing.T) {
		if err := ValidateMCPSecurity(nil); err == nil {
			t.Errorf("expected error for nil config")
		}
	})

	t.Run("stdio transport is always valid", func(t *testing.T) {
		cfg := &config.MCPConfig{
			Transport: "stdio",
		}
		if err := ValidateMCPSecurity(cfg); err != nil {
			t.Errorf("unexpected error for stdio transport: %v", err)
		}
	})

	t.Run("invalid transport rejected", func(t *testing.T) {
		cfg := &config.MCPConfig{
			Transport: "websocket",
			Port:      8444,
		}
		if err := ValidateMCPSecurity(cfg); err == nil || !strings.Contains(err.Error(), "unsupported MCP transport") {
			t.Errorf("expected unsupported transport error, got: %v", err)
		}
	})

	t.Run("invalid port rejected", func(t *testing.T) {
		cfg := &config.MCPConfig{
			Transport:   "http",
			BindAddress: "127.0.0.1",
			Port:        0,
		}
		if err := ValidateMCPSecurity(cfg); err == nil || !strings.Contains(err.Error(), "invalid MCP port") {
			t.Errorf("expected invalid port error, got: %v", err)
		}
	})

	t.Run("incomplete TLS cert without key rejected", func(t *testing.T) {
		cfg := &config.MCPConfig{
			Transport:   "http",
			BindAddress: "127.0.0.1",
			Port:        8444,
			TLSCert:     "cert-data",
		}
		if err := ValidateMCPSecurity(cfg); err == nil || !strings.Contains(err.Error(), "incomplete MCP TLS") {
			t.Errorf("expected incomplete TLS error, got: %v", err)
		}
	})

	t.Run("incomplete TLS key without cert rejected", func(t *testing.T) {
		cfg := &config.MCPConfig{
			Transport:   "http",
			BindAddress: "127.0.0.1",
			Port:        8444,
			TLSKey:      "key-data",
		}
		if err := ValidateMCPSecurity(cfg); err == nil || !strings.Contains(err.Error(), "incomplete MCP TLS") {
			t.Errorf("expected incomplete TLS error, got: %v", err)
		}
	})

	t.Run("loopback binding valid without token or TLS", func(t *testing.T) {
		cfg := &config.MCPConfig{
			Transport:   "sse",
			BindAddress: "127.0.0.1",
			Port:        8444,
		}
		if err := ValidateMCPSecurity(cfg); err != nil {
			t.Errorf("unexpected error for loopback sse: %v", err)
		}
	})

	t.Run("non-loopback binding requires token", func(t *testing.T) {
		cfg := &config.MCPConfig{
			Transport:   "http",
			BindAddress: "0.0.0.0",
			Port:        8444,
			TLSCert:     "cert",
			TLSKey:      "key",
		}
		if err := ValidateMCPSecurity(cfg); err == nil || !strings.Contains(err.Error(), "without authentication") {
			t.Errorf("expected missing token error, got: %v", err)
		}
	})

	t.Run("non-loopback binding requires TLS", func(t *testing.T) {
		cfg := &config.MCPConfig{
			Transport:   "http",
			BindAddress: "192.168.1.100",
			Port:        8444,
			Token:       "secret-token",
		}
		if err := ValidateMCPSecurity(cfg); err == nil || !strings.Contains(err.Error(), "without TLS") {
			t.Errorf("expected missing TLS error, got: %v", err)
		}
	})

	t.Run("non-loopback binding valid with token and TLS", func(t *testing.T) {
		cfg := &config.MCPConfig{
			Transport:   "http",
			BindAddress: "0.0.0.0",
			Port:        8444,
			Token:       "secret-token",
			TLSCert:     "cert-data",
			TLSKey:      "key-data",
		}
		if err := ValidateMCPSecurity(cfg); err != nil {
			t.Errorf("unexpected error when non-loopback has token and TLS: %v", err)
		}
	})
}

func TestSecurity_StrictReadOnlyAllowlist(t *testing.T) {
	registry := NewToolRegistry(nil, nil, nil, nil, nil, model.NodeIdentity{
		NodeID:   "local-node",
		Hostname: "localhost",
	})

	mcpCtx := MCPContext{
		RequestID: "sec-req-1",
		ClientID:  "sec-client",
		Actor:     "audit-tester",
		Timestamp: time.Now().UTC(),
	}

	// 1. Verify all 8 allowed tools are present in AllowedReadOperations
	expectedTools := []string{
		"list_nodes",
		"get_node",
		"get_node_health",
		"get_node_snapshot",
		"get_fleet_health",
		"get_node_metrics",
		"get_recent_diagnostics",
		"get_active_alerts",
	}

	for _, toolName := range expectedTools {
		if !AllowedReadOperations[toolName] {
			t.Errorf("expected tool %q to be in AllowedReadOperations", toolName)
		}
	}

	// 2. Verify all mutation / shell execution tool names are rejected
	disallowedTools := []string{
		"delete_node",
		"register_node",
		"save_alert",
		"update_status",
		"execute_command",
		"exec",
		"shell",
		"run_script",
		"modify_config",
		"purge_audit_events",
		"vacuum",
		"reboot",
		"kill_process",
	}

	for _, toolName := range disallowedTools {
		t.Run("Reject_"+toolName, func(t *testing.T) {
			if AllowedReadOperations[toolName] {
				t.Fatalf("tool %q MUST NOT be in AllowedReadOperations", toolName)
			}
			_, rpcErr := registry.Execute(context.Background(), mcpCtx, toolName, nil)
			if rpcErr == nil {
				t.Errorf("expected rejection error for disallowed tool %q, got nil", toolName)
			}
			if rpcErr != nil && rpcErr.Code != CodeMethodNotFound {
				t.Errorf("expected CodeMethodNotFound (%d) for %q, got %d", CodeMethodNotFound, toolName, rpcErr.Code)
			}
		})
	}
}

func TestSecurity_PathTraversalRejection(t *testing.T) {
	registry := NewToolRegistry(nil, nil, nil, nil, nil, model.NodeIdentity{
		NodeID:   "local-node",
		Hostname: "localhost",
	})

	mcpCtx := MCPContext{
		RequestID: "sec-req-2",
		ClientID:  "sec-client",
		Actor:     "audit-tester",
		Timestamp: time.Now().UTC(),
	}

	maliciousNodeIDs := []string{
		"../etc/passwd",
		"..\\..\\windows\\system32",
		"node/sub",
		"node\\sub",
		"node\x00inject",
		"",
		"   ",
		"node; rm -rf /",
	}

	for _, nodeID := range maliciousNodeIDs {
		t.Run("Traversal_"+nodeID, func(t *testing.T) {
			_, rpcErr := registry.Execute(context.Background(), mcpCtx, "get_node", map[string]any{
				"node_id": nodeID,
			})
			if rpcErr == nil {
				t.Errorf("expected validation error for malicious node_id %q, got success", nodeID)
			}
		})
	}
}

func TestSecurity_AuthAndTransportHardening(t *testing.T) {
	cfg := config.DefaultConfig().MCP
	cfg.Enabled = true
	cfg.Transport = "sse"
	cfg.Port = 18443
	cfg.BindAddress = "127.0.0.1"
	cfg.Token = "audit-secure-token-12345"

	srv := NewServer(cfg, nil, nil, nil, nil, nil, nil, model.NodeIdentity{
		NodeID:   "test-node",
		Hostname: "test-host",
	})

	// 1. Verify unauthenticated request to /mcp returns 401 Unauthorized
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401 for unauthenticated request, got %d", w.Code)
	}

	// 2. Verify invalid token returns 401 Unauthorized
	req = httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer invalid-token")
	w = httptest.NewRecorder()

	srv.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401 for invalid token, got %d", w.Code)
	}

	// 3. Verify valid token returns 200 OK
	req = httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer audit-secure-token-12345")
	w = httptest.NewRecorder()

	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 for valid token, got %d", w.Code)
	}
}
