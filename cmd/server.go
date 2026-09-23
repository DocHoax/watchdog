package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/watchdog-cli/watchdog/internal/alerts"
	"github.com/watchdog-cli/watchdog/internal/anomaly"
	"github.com/watchdog-cli/watchdog/internal/collector"
	"github.com/watchdog-cli/watchdog/internal/diagnostics"
	"github.com/watchdog-cli/watchdog/internal/logger"
	"github.com/watchdog-cli/watchdog/internal/server"
	"github.com/watchdog-cli/watchdog/internal/storage"
)

var (
	serverPort    int
	serverHost    string
	serverToken   string
	serverTLSCert string
	serverTLSKey  string
)

var serverCmd = &cobra.Command{
	Use:     "server",
	Aliases: []string{"serve", "daemon"},
	Short:   "Start the Watchdog HTTP REST API & Prometheus metrics server",
	Long: `Launches the Watchdog headless background server daemon.
Exposes:
- GET /health               : Unauthenticated uptime & health status probe
- GET /metrics              : Prometheus text format metrics exporter
- GET /api/v1/snapshot      : Authenticated real-time full system snapshot JSON
- GET /api/v1/diagnostics   : Authenticated on-demand automated diagnostic report JSON
- GET /api/v1/alerts        : Authenticated active and historical alert events JSON
- GET /api/v1/anomalies     : Authenticated statistical anomaly detection scores JSON`,
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
	if serverTLSCert != "" {
		cfg.Agent.TLSCert = serverTLSCert
	}
	if serverTLSKey != "" {
		cfg.Agent.TLSKey = serverTLSKey
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

	fmt.Printf("🐺 Watchdog Server starting on %s:%d\n", cfg.Agent.BindAddress, cfg.Agent.Port)
	if cfg.Agent.Token != "" {
		fmt.Println("🔒 Bearer Token authentication enabled")
	} else {
		fmt.Println("⚠️  Warning: No authentication token configured (open API)")
	}
	fmt.Printf("📊 Prometheus metrics available at: http://%s:%d/metrics\n", cfg.Agent.BindAddress, cfg.Agent.Port)

	if err := srv.Start(ctx); err != nil {
		return fmt.Errorf("server error: %w", err)
	}

	logger.Infof("Server stopped cleanly.")
	return nil
}

func init() {
	serverCmd.Flags().IntVarP(&serverPort, "port", "p", 8443, "HTTP/HTTPS listen port")
	serverCmd.Flags().StringVarP(&serverHost, "host", "H", "127.0.0.1", "bind IP interface address (default: 127.0.0.1)")
	serverCmd.Flags().StringVarP(&serverToken, "token", "t", "", "authentication token required for API endpoints")
	serverCmd.Flags().StringVar(&serverTLSCert, "tls-cert", "", "path to TLS certificate file")
	serverCmd.Flags().StringVar(&serverTLSKey, "tls-key", "", "path to TLS private key file")

	RootCmd.AddCommand(serverCmd)
}
