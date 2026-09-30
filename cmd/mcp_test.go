package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/DocHoax/watchdog/internal/config"
	"github.com/DocHoax/watchdog/internal/mcp"
)

func TestCmd_MCP_Status(t *testing.T) {
	globalCfg = config.DefaultConfig()

	// 1. Text mode
	mcpJSON = false
	out, err := captureStdout(func() error {
		return runMCPStatus(mcpStatusCmd, nil)
	})
	if err != nil {
		t.Fatalf("runMCPStatus failed: %v", err)
	}
	if !strings.Contains(out, "Watchdog Model Context Protocol (MCP) Server Status") {
		t.Errorf("expected header in text output, got: %s", out)
	}
	if !strings.Contains(out, "Registered Tools:           23") {
		t.Errorf("expected 23 tools, got: %s", out)
	}
	if !strings.Contains(out, "Registered Resources:       12") {
		t.Errorf("expected 12 resources, got: %s", out)
	}
	if !strings.Contains(out, "Registered Prompts:         12") {
		t.Errorf("expected 12 prompts, got: %s", out)
	}

	// 2. JSON mode
	mcpJSON = true
	outJSON, err := captureStdout(func() error {
		return runMCPStatus(mcpStatusCmd, nil)
	})
	if err != nil {
		t.Fatalf("runMCPStatus JSON failed: %v", err)
	}
	var status map[string]any
	if err := json.Unmarshal([]byte(outJSON), &status); err != nil {
		t.Fatalf("failed to unmarshal status JSON: %v", err)
	}
	if status["protocol_version"] != mcp.ProtocolVersion {
		t.Errorf("expected protocol version %s, got %v", mcp.ProtocolVersion, status["protocol_version"])
	}
	if status["server_name"] != mcp.ServerName {
		t.Errorf("expected server name %s, got %v", mcp.ServerName, status["server_name"])
	}
	if status["tools_count"] != float64(23) {
		t.Errorf("expected 23 tools count, got %v", status["tools_count"])
	}
	if status["resources_count"] != float64(12) {
		t.Errorf("expected 12 resources count, got %v", status["resources_count"])
	}
	if status["prompts_count"] != float64(12) {
		t.Errorf("expected 12 prompts count, got %v", status["prompts_count"])
	}
}

func TestCmd_MCP_Tools(t *testing.T) {
	// 1. Text mode
	mcpJSON = false
	out, err := captureStdout(func() error {
		return runMCPTools(mcpToolsCmd, nil)
	})
	if err != nil {
		t.Fatalf("runMCPTools failed: %v", err)
	}
	if !strings.Contains(out, "Watchdog MCP Tools (23 Registered)") {
		t.Errorf("expected 23 tools header, got: %s", out)
	}
	if !strings.Contains(out, "list_nodes") || !strings.Contains(out, "get_node_health") {
		t.Errorf("expected tool names in output, got: %s", out)
	}

	// 2. JSON mode
	mcpJSON = true
	outJSON, err := captureStdout(func() error {
		return runMCPTools(mcpToolsCmd, nil)
	})
	if err != nil {
		t.Fatalf("runMCPTools JSON failed: %v", err)
	}
	var tools []mcp.Tool
	if err := json.Unmarshal([]byte(outJSON), &tools); err != nil {
		t.Fatalf("failed to unmarshal tools JSON: %v", err)
	}
	if len(tools) != 23 {
		t.Errorf("expected 23 tools, got %d", len(tools))
	}
}

func TestCmd_MCP_Resources(t *testing.T) {
	// 1. Text mode
	mcpJSON = false
	out, err := captureStdout(func() error {
		return runMCPResources(mcpResourcesCmd, nil)
	})
	if err != nil {
		t.Fatalf("runMCPResources failed: %v", err)
	}
	if !strings.Contains(out, "Watchdog MCP Resources (12 Registered)") {
		t.Errorf("expected 12 resources header, got: %s", out)
	}
	if !strings.Contains(out, "watchdog://fleet") {
		t.Errorf("expected watchdog://fleet in output, got: %s", out)
	}

	// 2. JSON mode
	mcpJSON = true
	outJSON, err := captureStdout(func() error {
		return runMCPResources(mcpResourcesCmd, nil)
	})
	if err != nil {
		t.Fatalf("runMCPResources JSON failed: %v", err)
	}
	var resources []mcp.Resource
	if err := json.Unmarshal([]byte(outJSON), &resources); err != nil {
		t.Fatalf("failed to unmarshal resources JSON: %v", err)
	}
	if len(resources) != 12 {
		t.Errorf("expected 12 resources, got %d", len(resources))
	}
}

func TestCmd_MCP_Prompts(t *testing.T) {
	// 1. Text mode
	mcpJSON = false
	out, err := captureStdout(func() error {
		return runMCPPrompts(mcpPromptsCmd, nil)
	})
	if err != nil {
		t.Fatalf("runMCPPrompts failed: %v", err)
	}
	if !strings.Contains(out, "Watchdog MCP Prompt Templates (12 Registered)") {
		t.Errorf("expected 12 prompts header, got: %s", out)
	}
	if !strings.Contains(out, "system_health_audit") || !strings.Contains(out, "diagnose_node") {
		t.Errorf("expected prompt names in output, got: %s", out)
	}

	// 2. JSON mode
	mcpJSON = true
	outJSON, err := captureStdout(func() error {
		return runMCPPrompts(mcpPromptsCmd, nil)
	})
	if err != nil {
		t.Fatalf("runMCPPrompts JSON failed: %v", err)
	}
	var prompts []mcp.Prompt
	if err := json.Unmarshal([]byte(outJSON), &prompts); err != nil {
		t.Fatalf("failed to unmarshal prompts JSON: %v", err)
	}
	if len(prompts) != 12 {
		t.Errorf("expected 12 prompts, got %d", len(prompts))
	}
}

func TestCmd_MCP_Serve_Validation(t *testing.T) {
	globalCfg = config.DefaultConfig()

	// Reset flags
	mcpTransport = "http"
	mcpPort = 8444
	mcpHost = "0.0.0.0"
	mcpToken = ""
	mcpTLSCert = ""
	mcpTLSKey = ""

	err := runMCPServe(mcpServeCmd, nil)
	if err == nil {
		t.Fatalf("expected error for non-loopback without token and TLS")
	}
	if !strings.Contains(err.Error(), "without authentication") {
		t.Errorf("expected authentication error, got: %v", err)
	}

	// Incomplete TLS
	mcpToken = "test-token"
	mcpTLSCert = "cert-file"
	mcpTLSKey = ""
	errTLS := runMCPServe(mcpServeCmd, nil)
	if errTLS == nil {
		t.Fatalf("expected error for incomplete TLS")
	}
	if !strings.Contains(errTLS.Error(), "incomplete MCP TLS") {
		t.Errorf("expected incomplete TLS error, got: %v", errTLS)
	}

	// Unsupported transport
	mcpTransport = "invalid-transport"
	mcpHost = "127.0.0.1"
	errTransport := runMCPServe(mcpServeCmd, nil)
	if errTransport == nil {
		t.Fatalf("expected error for unsupported transport")
	}
	if !strings.Contains(errTransport.Error(), "unsupported MCP transport") {
		t.Errorf("expected unsupported transport error, got: %v", errTransport)
	}
}

func TestCmd_MCP_RootExecution(t *testing.T) {
	tests := [][]string{
		{"mcp", "status", "--json"},
		{"mcp", "tools", "--json"},
		{"mcp", "resources", "--json"},
		{"mcp", "prompts", "--json"},
	}

	for _, args := range tests {
		RootCmd.SetArgs(args)
		out, err := captureStdout(func() error {
			return RootCmd.Execute()
		})
		if err != nil {
			t.Fatalf("RootCmd.Execute(%v) failed: %v", args, err)
		}
		if len(out) == 0 {
			t.Errorf("expected non-empty output for RootCmd.Execute(%v)", args)
		}
	}
}
