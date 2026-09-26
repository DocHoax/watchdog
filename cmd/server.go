package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/DocHoax/watchdog/internal/alerts"
	"github.com/DocHoax/watchdog/internal/anomaly"
	"github.com/DocHoax/watchdog/internal/collector"
	"github.com/DocHoax/watchdog/internal/diagnostics"
	"github.com/DocHoax/watchdog/internal/logger"
	"github.com/DocHoax/watchdog/internal/server"
	"github.com/DocHoax/watchdog/internal/storage"
	"github.com/spf13/cobra"
)

var (
	serverPort        int
	serverHost        string
	serverToken       string
	serverTokenFile   string
	serverTokenEnv    string
	serverTLSCert     string
	serverTLSCertFile string
	serverTLSCertEnv  string
	serverTLSKey      string
	serverTLSKeyFile  string
	serverTLSKeyEnv   string
)

var serverCmd = &cobra.Command{
	Use:     "server [flags]",
	Aliases: []string{"serve", "daemon"},
	Short:   "Start the Watchdog HTTP REST API & Prometheus metrics server",
	Long: `Launches the Watchdog headless background server daemon.

Exposed Endpoints:
  - GET /health               : Unauthenticated uptime & health status probe
  - GET /metrics              : Prometheus text-format scrape exporter
  - GET /api/v1/snapshot      : Authenticated real-time full system snapshot JSON
  - GET /api/v1/diagnostics   : Authenticated on-demand automated diagnostic report JSON
  - GET /api/v1/alerts        : Authenticated active and historical alert events JSON
  - GET /api/v1/anomalies     : Authenticated statistical anomaly detection scores JSON
  - GET /debug/pprof/*        : Runtime CPU/memory diagnostic profiling endpoints`,
	Example: `  # Start the server listening on localhost:8443
  watchdog server --port 8443 --host 127.0.0.1

  # Start with Bearer token authentication required for API routes
  watchdog server --port 8443 --token <token>

  # Start with Bearer token read from a secure secret file
  watchdog server --port 8443 --token-file /etc/watchdog/token

  # Run with TLS encryption using HTTPS certificates
  watchdog server --port 8443 --tls-cert /etc/ssl/cert.pem --tls-key /etc/ssl/key.pem

  # Run in background with JSON structured logging
  watchdog server --json-logs --port 9090`,
	RunE: runServer,
}

func runServer(cmd *cobra.Command, args []string) error {
	cfg := globalCfg
	if serverPort > 0 {
		cfg.Agent.Port = serverPort
	}
	if serverHost != "" {
		cfg.Agent.BindAddress = serverHost
	}
	if serverToken != "" {
		cfg.Agent.Token = serverToken
	}
	if serverTokenFile != "" {
		cfg.Agent.TokenFile = serverTokenFile
	}
	if serverTokenEnv != "" {
		cfg.Agent.TokenEnv = serverTokenEnv
	}
	if serverTLSCert != "" {
		cfg.Agent.TLSCert = serverTLSCert
	}
	if serverTLSCertFile != "" {
		cfg.Agent.TLSCertFile = serverTLSCertFile
	}
	if serverTLSCertEnv != "" {
		cfg.Agent.TLSCertEnv = serverTLSCertEnv
	}
	if serverTLSKey != "" {
		cfg.Agent.TLSKey = serverTLSKey
	}
	if serverTLSKeyFile != "" {
		cfg.Agent.TLSKeyFile = serverTLSKeyFile
	}
	if serverTLSKeyEnv != "" {
		cfg.Agent.TLSKeyEnv = serverTLSKeyEnv
	}

	// Resolve secrets (files, environment variables, precedence)
	if err := cfg.ResolveSecrets(); err != nil {
		return NewExitError(ExitConfigError, "failed to resolve server secrets: %w", err)
	}

	// Pre-flight security validation — fail fast before allocating resources.
	if err := server.ValidateServerSecurity(&cfg.Agent); err != nil {
		return NewExitError(ExitConfigError, "%v", err)
	}

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

	col := collector.NewDefaultManager(cfg)
	diagEng := diagnostics.NewEngine(cfg)
	alertEng := alerts.NewEngine(cfg, store)
	anomDet := anomaly.NewDetector(&cfg.Anomaly)

	srv := server.NewServer(cfg, col, store, diagEng, alertEng, anomDet)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle OS shutdown signals gracefully
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		logger.Infof("Received termination signal %v, initiating shutdown...", sig)
		cancel()
	}()

	bindDisplay := cfg.Agent.BindAddress
	if bindDisplay == "" {
		bindDisplay = "127.0.0.1"
	}
	scheme := "http"
	if cfg.Agent.TLSCert != "" {
		scheme = "https"
	}

	fmt.Printf("🐺 Watchdog Server starting on %s:%d\n", bindDisplay, cfg.Agent.Port)
	if cfg.Agent.Token != "" {
		fmt.Println("🔒 Bearer Token authentication enabled")
	} else {
		if server.IsLoopback(cfg.Agent.BindAddress) {
			fmt.Println("ℹ️  No authentication token configured (localhost-only access)")
		} else {
			// Should not reach here due to validation above, but defensive.
			fmt.Println("⚠️  Warning: No authentication token configured (open API)")
		}
	}
	fmt.Printf("📊 Prometheus metrics available at: %s://%s:%d/metrics\n", scheme, bindDisplay, cfg.Agent.Port)

	if err := srv.Start(ctx); err != nil {
		return NewExitError(ExitNetworkError, "server error: %w", err)
	}

	logger.Infof("Server stopped cleanly.")
	return nil
}

func init() {
	serverCmd.Flags().IntVarP(&serverPort, "port", "p", 8443, "HTTP/HTTPS listen port")
	serverCmd.Flags().StringVarP(&serverHost, "host", "H", "127.0.0.1", "bind IP interface address (default: 127.0.0.1)")
	serverCmd.Flags().StringVarP(&serverToken, "token", "t", "", "authentication token required for API endpoints")
	serverCmd.Flags().StringVar(&serverTokenFile, "token-file", "", "path to file containing authentication token")
	serverCmd.Flags().StringVar(&serverTokenEnv, "token-env", "", "environment variable name containing authentication token")
	serverCmd.Flags().StringVar(&serverTLSCert, "tls-cert", "", "path to TLS certificate file")
	serverCmd.Flags().StringVar(&serverTLSCertFile, "tls-cert-file", "", "path to TLS certificate file")
	serverCmd.Flags().StringVar(&serverTLSCertEnv, "tls-cert-env", "", "environment variable name containing TLS certificate path")
	serverCmd.Flags().StringVar(&serverTLSKey, "tls-key", "", "path to TLS private key file")
	serverCmd.Flags().StringVar(&serverTLSKeyFile, "tls-key-file", "", "path to TLS private key file")
	serverCmd.Flags().StringVar(&serverTLSKeyEnv, "tls-key-env", "", "environment variable name containing TLS private key path")

	RootCmd.AddCommand(serverCmd)
}
