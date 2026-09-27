package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/DocHoax/watchdog/internal/alerts"
	"github.com/DocHoax/watchdog/internal/audit"
	"github.com/DocHoax/watchdog/internal/collector"
	"github.com/DocHoax/watchdog/internal/config"
	"github.com/DocHoax/watchdog/internal/diagnostics"
	"github.com/DocHoax/watchdog/internal/fleet"
	"github.com/DocHoax/watchdog/internal/logger"
	"github.com/DocHoax/watchdog/internal/mcp"
	"github.com/DocHoax/watchdog/internal/storage"
	"github.com/DocHoax/watchdog/pkg/model"
	"github.com/spf13/cobra"
)

var (
	mcpJSON            bool
	mcpTransport       string
	mcpPort            int
	mcpHost            string
	mcpToken           string
	mcpTokenFile       string
	mcpTokenEnv        string
	mcpTLSCert         string
	mcpTLSCertFile     string
	mcpTLSCertEnv      string
	mcpTLSKey          string
	mcpTLSKeyFile      string
	mcpTLSKeyEnv       string
	mcpRateLimit       float64
	mcpBurst           int
	mcpMaxBodyBytes    int64
	mcpReadTimeoutSec  int
	mcpWriteTimeoutSec int
)

var mcpCmd = &cobra.Command{
	Use:     "mcp [command]",
	Aliases: []string{"ai"},
	Short:   "Model Context Protocol (MCP) server & AI assistant integration",
	Long: `Provides Model Context Protocol (MCP) server endpoints and discovery tools
for integrating Watchdog with AI assistants (Claude Desktop, IDE extensions, autonomous agents).

Watchdog MCP exposes read-only system telemetry, fleet diagnostics, alerts, and statistical
anomaly scores via standard JSON-RPC 2.0 tools, resources, and prompt templates.

Transport Modes:
  - stdio : Direct communication over standard I/O pipes (ideal for local Claude Desktop)
  - http  : Authenticated JSON-RPC 2.0 POST endpoint (/mcp)
  - sse   : Authenticated Server-Sent Events stream (/sse and /mcp/message)`,
	Example: `  # Start MCP server in standard I/O mode for Claude Desktop
  watchdog mcp serve --transport stdio

  # Start authenticated HTTP/SSE MCP server on port 8444
  watchdog mcp serve --transport http --port 8444 --token secret-token

  # Inspect MCP status and security configuration
  watchdog mcp status

  # List registered MCP tools and JSON schemas
  watchdog mcp tools

  # List readable MCP resources
  watchdog mcp resources

  # List available MCP prompt templates
  watchdog mcp prompts`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runMCPStatus(cmd, args)
	},
}

var mcpServeCmd = &cobra.Command{
	Use:     "serve [flags]",
	Aliases: []string{"start", "run"},
	Short:   "Start the Model Context Protocol (MCP) server daemon",
	Long: `Starts the Watchdog MCP server in the configured transport mode.

In 'stdio' mode, all protocol messages are exchanged via stdin/stdout framing,
and all logging output is strictly redirected to stderr to prevent protocol corruption.

In 'http' and 'sse' modes, network listeners are established on the configured bind address
and port with mandatory token and TLS requirements for non-loopback exposure.`,
	Example: `  # Run in stdio mode (default)
  watchdog mcp serve

  # Run over HTTP with Bearer token authentication
  watchdog mcp serve --transport http --port 8444 --token <token>

  # Run over SSE with TLS encryption
  watchdog mcp serve --transport sse --port 8444 --tls-cert /path/to/cert.pem --tls-key /path/to/key.pem --token-file /etc/watchdog/token`,
	RunE: runMCPServe,
}

var mcpStatusCmd = &cobra.Command{
	Use:     "status [flags]",
	Aliases: []string{"info"},
	Short:   "Display MCP server configuration, capabilities, and security posture",
	Long:    `Displays the current MCP server configuration, security posture, and capability counts.`,
	Example: `  # Display status summary
  watchdog mcp status

  # Output status as JSON
  watchdog mcp status --json`,
	RunE: runMCPStatus,
}

var mcpToolsCmd = &cobra.Command{
	Use:     "tools [flags]",
	Aliases: []string{"list-tools"},
	Short:   "List registered MCP tool definitions and parameter schemas",
	Long:    `Displays all available MCP tools callable by AI agents, including input parameter schemas and requirements.`,
	Example: `  # List all tools
  watchdog mcp tools

  # List tools in full JSON schema format
  watchdog mcp tools --json`,
	RunE: runMCPTools,
}

var mcpResourcesCmd = &cobra.Command{
	Use:     "resources [flags]",
	Aliases: []string{"list-resources"},
	Short:   "List registered MCP resource URIs and templates",
	Long:    `Displays all available MCP readable resources and URI templates accessible by AI agents.`,
	Example: `  # List all readable resources
  watchdog mcp resources

  # Output resources as JSON
  watchdog mcp resources --json`,
	RunE: runMCPResources,
}

var mcpPromptsCmd = &cobra.Command{
	Use:     "prompts [flags]",
	Aliases: []string{"list-prompts"},
	Short:   "List registered MCP prompt templates and arguments",
	Long:    `Displays all available MCP prompt templates and workflow recipes for AI assistants.`,
	Example: `  # List all prompt templates
  watchdog mcp prompts

  # Output prompt templates as JSON
  watchdog mcp prompts --json`,
	RunE: runMCPPrompts,
}

func runMCPServe(cmd *cobra.Command, args []string) error {
	cfg := globalCfg
	if cfg == nil {
		cfg = config.DefaultConfig()
	}

	// Apply CLI flag overrides to MCP config
	if mcpTransport != "" {
		cfg.MCP.Transport = mcpTransport
	}
	if mcpPort > 0 {
		cfg.MCP.Port = mcpPort
	}
	if mcpHost != "" {
		cfg.MCP.BindAddress = mcpHost
	}
	if mcpToken != "" {
		cfg.MCP.Token = mcpToken
	}
	if mcpTokenFile != "" {
		cfg.MCP.TokenFile = mcpTokenFile
	}
	if mcpTokenEnv != "" {
		cfg.MCP.TokenEnv = mcpTokenEnv
	}
	if mcpTLSCert != "" {
		cfg.MCP.TLSCert = mcpTLSCert
	}
	if mcpTLSCertFile != "" {
		cfg.MCP.TLSCertFile = mcpTLSCertFile
	}
	if mcpTLSCertEnv != "" {
		cfg.MCP.TLSCertEnv = mcpTLSCertEnv
	}
	if mcpTLSKey != "" {
		cfg.MCP.TLSKey = mcpTLSKey
	}
	if mcpTLSKeyFile != "" {
		cfg.MCP.TLSKeyFile = mcpTLSKeyFile
	}
	if mcpTLSKeyEnv != "" {
		cfg.MCP.TLSKeyEnv = mcpTLSKeyEnv
	}
	if mcpRateLimit > 0 {
		cfg.MCP.RateLimitRate = mcpRateLimit / 60.0
	}
	if mcpBurst > 0 {
		cfg.MCP.RateLimitBurst = mcpBurst
	}
	if mcpMaxBodyBytes > 0 {
		cfg.MCP.MaxRequestBodyBytes = mcpMaxBodyBytes
	}
	if mcpReadTimeoutSec > 0 {
		cfg.MCP.ReadTimeout = time.Duration(mcpReadTimeoutSec) * time.Second
	}
	if mcpWriteTimeoutSec > 0 {
		cfg.MCP.WriteTimeout = time.Duration(mcpWriteTimeoutSec) * time.Second
	}

	// Resolve secrets (files, environment variables)
	if err := cfg.ResolveSecrets(); err != nil {
		return NewExitError(ExitConfigError, "failed to resolve MCP secrets: %w", err)
	}

	// Pre-flight security validation — fail fast before allocating resources
	if err := mcp.ValidateMCPSecurity(&cfg.MCP); err != nil {
		return NewExitError(ExitConfigError, "%v", err)
	}

	// Storage initialization
	var store storage.Storage
	if cfg.Storage.Enabled {
		var err error
		store, err = storage.NewSQLiteStorage(storage.Config{Path: cfg.Storage.DBPath})
		if err != nil {
			logger.Warnf("Storage initialization warning: %v; running without persistent storage", err)
		} else {
			defer store.Close()
		}
	}

	// Subsystem engines
	col := collector.NewDefaultManager(cfg)
	diagEng := diagnostics.NewEngine(cfg)
	alertEng := alerts.NewEngine(cfg, store)

	// Discover local node identity
	localID := model.NodeIdentity{
		NodeID:    "local-node",
		Hostname:  "localhost",
		CreatedAt: time.Now(),
	}
	if cfg.Fleet.NodeID != "" {
		localID.NodeID = cfg.Fleet.NodeID
	} else if cfg.Fleet.NodeIDFile != "" {
		if id, err := fleet.GetOrGenerateNodeID(cfg.Fleet.NodeIDFile); err == nil && id != "" {
			localID.NodeID = id
		}
	}
	if discID, err := fleet.DiscoverNodeIdentity(context.Background(), localID.NodeID, "1.0.0", cfg.Fleet.Tags); err == nil && discID != nil {
		localID = *discID
	}

	// Fleet service
	var fleetSvc fleet.FleetService
	if store != nil {
		fleetSvc = fleet.NewFleetService(store, cfg.Fleet)
		// Register local node
		_, _ = fleetSvc.RegisterNode(context.Background(), &model.NodeRegistrationRequest{
			Identity: localID,
		})
	}

	// Audit logging
	auditLog := audit.New(cfg.Audit, store, logger.GetDefault())
	defer auditLog.Close()

	// Create MCP server
	srv := mcp.NewServer(cfg.MCP, store, col, diagEng, alertEng, fleetSvc, auditLog, localID)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle OS shutdown signals gracefully
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		if cfg.MCP.Transport != "stdio" {
			logger.Infof("Received termination signal %v, initiating MCP shutdown...", sig)
		}
		cancel()
	}()

	transport := strings.ToLower(cfg.MCP.Transport)
	if transport == "" || transport == "stdio" {
		// In stdio mode, standard I/O is used strictly for JSON-RPC 2.0 communication.
		// All logging must go to stderr to prevent protocol stream corruption.
		return srv.ServeStdio(ctx, os.Stdin, os.Stdout)
	}

	// Network transport: print startup banner to stdout
	bindDisplay := cfg.MCP.BindAddress
	if bindDisplay == "" {
		bindDisplay = "127.0.0.1"
	}
	scheme := "http"
	if cfg.MCP.TLSCert != "" {
		scheme = "https"
	}

	fmt.Printf("🐺 Watchdog Model Context Protocol (MCP) Server\n")
	fmt.Printf("  Transport:                  %s\n", strings.ToUpper(transport))
	fmt.Printf("  Listen Address:             %s:%d\n", bindDisplay, cfg.MCP.Port)
	fmt.Printf("  Protocol Version:           %s\n", mcp.ProtocolVersion)
	fmt.Printf("  JSON-RPC Endpoint:          %s://%s:%d/mcp\n", scheme, bindDisplay, cfg.MCP.Port)
	if transport == "sse" {
		fmt.Printf("  SSE Stream Endpoint:        %s://%s:%d/sse\n", scheme, bindDisplay, cfg.MCP.Port)
		fmt.Printf("  SSE Message Endpoint:       %s://%s:%d/mcp/message\n", scheme, bindDisplay, cfg.MCP.Port)
	}
	fmt.Printf("  Health Probe:               %s://%s:%d/health\n", scheme, bindDisplay, cfg.MCP.Port)
	fmt.Printf("  Prometheus Metrics:         %s://%s:%d/metrics\n", scheme, bindDisplay, cfg.MCP.Port)
	if cfg.MCP.Token != "" {
		fmt.Println("  Authentication:             🔒 Bearer Token Enforced")
	} else {
		fmt.Println("  Authentication:             ℹ️  None (Loopback-only access)")
	}
	if cfg.MCP.TLSCert != "" {
		fmt.Println("  TLS Encryption:             🔒 Enabled (HTTPS)")
	} else {
		fmt.Println("  TLS Encryption:             Disabled (Plain HTTP)")
	}
	fmt.Println()

	if err := srv.StartHTTP(ctx); err != nil {
		return NewExitError(ExitNetworkError, "MCP server error: %w", err)
	}

	return nil
}

func runMCPStatus(cmd *cobra.Command, args []string) error {
	cfg := globalCfg
	if cfg == nil {
		cfg = config.DefaultConfig()
	}

	tools := mcp.ToolDefinitions()
	promptReg := mcp.NewPromptRegistry()
	prompts := promptReg.ListPrompts()
	resReg := mcp.NewResourceRegistry(nil, nil, nil, nil, nil, model.NodeIdentity{NodeID: "local-node", Hostname: "localhost"})
	resources, _ := resReg.ListResources(context.Background())

	bindAddr := cfg.MCP.BindAddress
	if bindAddr == "" {
		bindAddr = "127.0.0.1"
	}
	port := cfg.MCP.Port
	if port == 0 {
		port = 8444
	}
	transport := cfg.MCP.Transport
	if transport == "" {
		transport = "stdio"
	}

	isLoopback := mcp.IsLoopback(bindAddr)
	hasAuth := cfg.MCP.Token != "" || cfg.MCP.TokenFile != "" || cfg.MCP.TokenEnv != ""
	hasTLS := (cfg.MCP.TLSCert != "" || cfg.MCP.TLSCertFile != "" || cfg.MCP.TLSCertEnv != "") &&
		(cfg.MCP.TLSKey != "" || cfg.MCP.TLSKeyFile != "" || cfg.MCP.TLSKeyEnv != "")

	if mcpJSON {
		statusData := map[string]any{
			"protocol_version": mcp.ProtocolVersion,
			"server_name":      mcp.ServerName,
			"transport":        transport,
			"bind_address":     bindAddr,
			"port":             port,
			"is_loopback":      isLoopback,
			"has_auth":         hasAuth,
			"has_tls":          hasTLS,
			"tools_count":      len(tools),
			"resources_count":  len(resources),
			"prompts_count":    len(prompts),
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(statusData)
	}

	fmt.Println("🐺 Watchdog Model Context Protocol (MCP) Server Status")
	fmt.Printf("  Protocol Version:           %s\n", mcp.ProtocolVersion)
	fmt.Printf("  Server Name:                %s\n", mcp.ServerName)
	fmt.Printf("  Default Transport:          %s\n", transport)
	fmt.Printf("  Bind Address:               %s:%d\n", bindAddr, port)
	if isLoopback {
		fmt.Printf("  Network Exposure:           Localhost Only (Loopback)\n")
	} else {
		fmt.Printf("  Network Exposure:           🌐 External Network Interface\n")
	}
	if hasAuth {
		fmt.Printf("  Authentication:             🔒 Token Configured\n")
	} else {
		fmt.Printf("  Authentication:             ℹ️  None (Permitted for loopback)\n")
	}
	if hasTLS {
		fmt.Printf("  TLS Encryption:             🔒 Enabled\n")
	} else {
		fmt.Printf("  TLS Encryption:             Disabled\n")
	}
	fmt.Printf("  Registered Tools:           %d\n", len(tools))
	fmt.Printf("  Registered Resources:       %d\n", len(resources))
	fmt.Printf("  Registered Prompts:         %d\n", len(prompts))
	return nil
}

func runMCPTools(cmd *cobra.Command, args []string) error {
	tools := mcp.ToolDefinitions()

	if mcpJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(tools)
	}

	fmt.Printf("🛠️  Watchdog MCP Tools (%d Registered)\n\n", len(tools))
	for i, t := range tools {
		fmt.Printf("%d. %s\n", i+1, t.Name)
		fmt.Printf("   Description: %s\n", t.Description)
		if len(t.InputSchema.Properties) > 0 {
			fmt.Printf("   Parameters:\n")
			for propName, propSchema := range t.InputSchema.Properties {
				reqStr := "optional"
				for _, req := range t.InputSchema.Required {
					if req == propName {
						reqStr = "required"
						break
					}
				}
				fmt.Printf("     - %s (%s, %s): %s\n", propName, propSchema.Type, reqStr, propSchema.Description)
			}
		} else {
			fmt.Printf("   Parameters:  None\n")
		}
		fmt.Println()
	}
	return nil
}

func runMCPResources(cmd *cobra.Command, args []string) error {
	resReg := mcp.NewResourceRegistry(nil, nil, nil, nil, nil, model.NodeIdentity{NodeID: "local-node", Hostname: "localhost"})
	resources, err := resReg.ListResources(context.Background())
	if err != nil {
		return NewExitError(ExitGeneralError, "failed to list MCP resources: %w", err)
	}

	if mcpJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(resources)
	}

	fmt.Printf("📦 Watchdog MCP Resources (%d Registered)\n\n", len(resources))
	for i, r := range resources {
		fmt.Printf("%d. %s (%s)\n", i+1, r.Name, r.URI)
		fmt.Printf("   MIME Type:   %s\n", r.MIMEType)
		if r.Description != "" {
			fmt.Printf("   Description: %s\n", r.Description)
		}
		fmt.Println()
	}
	return nil
}

func runMCPPrompts(cmd *cobra.Command, args []string) error {
	promptReg := mcp.NewPromptRegistry()
	prompts := promptReg.ListPrompts()

	if mcpJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(prompts)
	}

	fmt.Printf("💡 Watchdog MCP Prompt Templates (%d Registered)\n\n", len(prompts))
	for i, p := range prompts {
		fmt.Printf("%d. %s\n", i+1, p.Name)
		fmt.Printf("   Description: %s\n", p.Description)
		if len(p.Arguments) > 0 {
			fmt.Printf("   Arguments:\n")
			for _, arg := range p.Arguments {
				reqStr := "optional"
				if arg.Required {
					reqStr = "required"
				}
				fmt.Printf("     - %s (%s): %s\n", arg.Name, reqStr, arg.Description)
			}
		} else {
			fmt.Printf("   Arguments:   None\n")
		}
		fmt.Println()
	}
	return nil
}

func init() {
	mcpCmd.PersistentFlags().BoolVar(&mcpJSON, "json", false, "output response in structured JSON format")

	// Serve flags
	mcpServeCmd.Flags().StringVar(&mcpTransport, "transport", "stdio", "transport protocol ('stdio', 'http', 'sse')")
	mcpServeCmd.Flags().IntVarP(&mcpPort, "port", "p", 8444, "listen port for HTTP/SSE transport")
	mcpServeCmd.Flags().StringVarP(&mcpHost, "host", "H", "127.0.0.1", "bind IP interface address")
	mcpServeCmd.Flags().StringVarP(&mcpToken, "token", "t", "", "authentication bearer token")
	mcpServeCmd.Flags().StringVar(&mcpTokenFile, "token-file", "", "path to file containing bearer authentication token")
	mcpServeCmd.Flags().StringVar(&mcpTokenEnv, "token-env", "", "environment variable name containing bearer authentication token")
	mcpServeCmd.Flags().StringVar(&mcpTLSCert, "tls-cert", "", "path to TLS certificate file")
	mcpServeCmd.Flags().StringVar(&mcpTLSCertFile, "tls-cert-file", "", "path to TLS certificate file")
	mcpServeCmd.Flags().StringVar(&mcpTLSCertEnv, "tls-cert-env", "", "environment variable name containing TLS certificate path")
	mcpServeCmd.Flags().StringVar(&mcpTLSKey, "tls-key", "", "path to TLS private key file")
	mcpServeCmd.Flags().StringVar(&mcpTLSKeyFile, "tls-key-file", "", "path to TLS private key file")
	mcpServeCmd.Flags().StringVar(&mcpTLSKeyEnv, "tls-key-env", "", "environment variable name containing TLS private key path")
	mcpServeCmd.Flags().Float64Var(&mcpRateLimit, "rate-limit", 120.0, "maximum requests per minute rate limit")
	mcpServeCmd.Flags().IntVar(&mcpBurst, "burst", 20, "burst capacity for rate limiter")
	mcpServeCmd.Flags().Int64Var(&mcpMaxBodyBytes, "max-body-size", 1048576, "maximum HTTP request body size in bytes (default: 1MB)")
	mcpServeCmd.Flags().IntVar(&mcpReadTimeoutSec, "read-timeout", 10, "HTTP read timeout in seconds")
	mcpServeCmd.Flags().IntVar(&mcpWriteTimeoutSec, "write-timeout", 10, "HTTP write timeout in seconds")

	// Subcommands
	mcpCmd.AddCommand(mcpServeCmd)
	mcpCmd.AddCommand(mcpStatusCmd)
	mcpCmd.AddCommand(mcpToolsCmd)
	mcpCmd.AddCommand(mcpResourcesCmd)
	mcpCmd.AddCommand(mcpPromptsCmd)

	RootCmd.AddCommand(mcpCmd)
}
