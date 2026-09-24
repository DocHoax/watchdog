package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

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
	agentInterval time.Duration
	agentPort     int
	agentToken    string
)

var agentCmd = &cobra.Command{
	Use:   "agent [flags]",
	Short: "Run Watchdog as a lightweight background monitoring agent",
	Long: `Starts Watchdog in headless agent daemon mode for server nodes, containers, and Kubernetes pods.

Key Functions:
  - Periodically samples host health metrics at the configured interval.
  - Persists time-series history to local SQLite database in WAL mode.
  - Continuously evaluates threshold alert rules and sends notifications.
  - Continuously trains online statistical anomaly detection.
  - Exposes HTTP REST API and Prometheus scrape exporter.`,
	Example: `  # Start agent with default 2-second collection interval
  watchdog agent

  # Start agent with 5-second interval on port 9090
  watchdog agent --interval 5s --port 9090

  # Start agent secured with API bearer token
  watchdog agent --port 8443 --token s3cr3t-t0k3n

  # Run agent with structured JSON logging for container logs
  watchdog agent --json-logs --quiet`,
	RunE: runAgent,
}

func runAgent(cmd *cobra.Command, args []string) error {
	cfg := globalCfg
	if agentInterval > 0 {
		cfg.RefreshInterval = agentInterval
	}
	if agentPort > 0 {
		cfg.Agent.Port = agentPort
	}
	if agentToken != "" {
		cfg.Agent.Token = agentToken
	}

	var store storage.Storage
	if cfg.Storage.Enabled {
		var err error
		store, err = storage.NewSQLiteStorage(storage.Config{Path: cfg.Storage.DBPath})
		if err != nil {
			logger.Warnf("Storage unavailable: %v; running without persistent storage", err)
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

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		logger.Infof("Agent received stop signal (%v), exiting...", sig)
		cancel()
	}()

	fmt.Printf("🐺 Watchdog Agent active (interval: %v, port: %d)\n", cfg.RefreshInterval, cfg.Agent.Port)
	if err := srv.Start(ctx); err != nil {
		return NewExitError(ExitNetworkError, "agent server failed: %w", err)
	}
	return nil
}

func init() {
	agentCmd.Flags().DurationVarP(&agentInterval, "interval", "i", 2*time.Second, "metric collection frequency")
	agentCmd.Flags().IntVarP(&agentPort, "port", "p", 8443, "agent API / Prometheus port")
	agentCmd.Flags().StringVarP(&agentToken, "token", "t", "", "agent authentication token")

	RootCmd.AddCommand(agentCmd)
}
