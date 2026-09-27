package mcp

import (
	"fmt"
	"net"
	"strings"

	"github.com/DocHoax/watchdog/internal/config"
)

// IsLoopback reports whether the specified IP or hostname resolves exclusively to a loopback interface.
func IsLoopback(addr string) bool {
	if addr == "" || addr == "localhost" || addr == "127.0.0.1" || addr == "::1" {
		return true
	}
	ip := net.ParseIP(addr)
	if ip != nil && ip.IsLoopback() {
		return true
	}
	// Check standard localhost prefixes
	if strings.HasPrefix(addr, "127.") {
		return true
	}
	return false
}

// ValidateMCPSecurity performs pre-flight security validation on the MCP configuration.
// It enforces that non-loopback network transports mandate both token authentication and TLS encryption.
func ValidateMCPSecurity(cfg *config.MCPConfig) error {
	if cfg == nil {
		return fmt.Errorf("mcp config is nil")
	}

	transport := strings.ToLower(cfg.Transport)
	if transport == "" || transport == "stdio" {
		// Stdio transport operates strictly over local standard I/O pipes.
		return nil
	}

	if transport != "http" && transport != "sse" {
		return fmt.Errorf("unsupported MCP transport %q: valid transports are 'stdio', 'sse', and 'http'", cfg.Transport)
	}

	if cfg.Port <= 0 || cfg.Port > 65535 {
		return fmt.Errorf("invalid MCP port %d: must be between 1 and 65535", cfg.Port)
	}

	hasCert := cfg.TLSCert != ""
	hasKey := cfg.TLSKey != ""
	if hasCert != hasKey {
		if hasCert {
			return fmt.Errorf("incomplete MCP TLS configuration: tls_cert is set but tls_key is missing; provide both or neither")
		}
		return fmt.Errorf("incomplete MCP TLS configuration: tls_key is set but tls_cert is missing; provide both or neither")
	}

	bindAddr := cfg.BindAddress
	if IsLoopback(bindAddr) {
		// Loopback network interface is permitted without mandatory token/TLS
		return nil
	}

	// Non-loopback binding strictly mandates authentication
	if cfg.Token == "" {
		return fmt.Errorf(
			"MCP server refuses to bind to %s without authentication: "+
				"externally exposed MCP API requires a token; configure mcp.token "+
				"or bind to localhost (127.0.0.1)",
			bindAddr,
		)
	}

	// Non-loopback binding strictly mandates TLS encryption
	if !hasCert {
		return fmt.Errorf(
			"MCP server refuses to bind to %s without TLS: "+
				"externally exposed MCP API requires encryption; configure mcp.tls_cert "+
				"and mcp.tls_key or bind to localhost (127.0.0.1)",
			bindAddr,
		)
	}

	return nil
}
