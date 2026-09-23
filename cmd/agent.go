package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

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
	agentInterval time.Duration
	agentPort     int
	agentToken    string
)

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Run Watchdog as a lightweight background monitoring agent",
	Long: `Starts Watchdog in headless agent daemon mode for server nodes, containers, and Kubernetes pods.
Periodically samples host health metrics, persists time-series history to local SQLite,
evaluates temporal threshold alerts, trains online anomaly detection, and serves Prometheus metrics.`,
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
		store, err = storage.NewSQLiteStorage(cfg.Storage.DBPath)
		if err != nil {
			logger.Warnf("Storage unavailable: %v; running without persistent storage", err)
		} else {
			defer store.Close()
		}
	}

	col := collector.NewManager(cfg)
	diagEng := diagnostics.NewEngine(cfg)
	alertEng := alerts.NewEngine(cfg, store)
	anomDet := anomaly.NewDetector(cfg)

	srv := server.NewServer(cfg, col, store, diagEng, alertEng, anomDet)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		logger.Infof("Agent received stop signal, exiting...")
		cancel()
	}()

	fmt.Printf("🐺 Watchdog Agent active (interval: %v, port: %d)\n", cfg.RefreshInterval, cfg.Agent.Port)
	return srv.Start(ctx)
}

func init() {
	agentCmd.Flags().DurationVarP(&agentInterval, "interval", "i", 2*time.Second, "metric collection frequency")
	agentCmd.Flags().IntVarP(&agentPort, "port", "p", 8443, "agent API / Prometheus port")
	agentCmd.Flags().StringVarP(&agentToken, "token", "t", "", "agent authentication token")

	RootCmd.AddCommand(agentCmd)
}
