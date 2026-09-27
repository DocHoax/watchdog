package mcp

import (
	"strings"
	"testing"

	"github.com/DocHoax/watchdog/internal/config"
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
