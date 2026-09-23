package cmd

import (
	"context"
	"time"

	"github.com/spf13/cobra"
	"github.com/watchdog-cli/watchdog/internal/alerts"
	"github.com/watchdog-cli/watchdog/internal/anomaly"
	"github.com/watchdog-cli/watchdog/internal/collector"
	"github.com/watchdog-cli/watchdog/internal/diagnostics"
	"github.com/watchdog-cli/watchdog/internal/logger"
	"github.com/watchdog-cli/watchdog/internal/storage"
	"github.com/watchdog-cli/watchdog/internal/tui"
)

var (
	dashInterval time.Duration
	dashTheme    string
	dashRemote   string
	dashToken    string
	dashInsecure bool
)

var dashCmd = &cobra.Command{
	Use:     "dash",
	Aliases: []string{"dashboard", "tui", "top"},
	Short:   "Launch the interactive real-time Terminal UI",
	Long: `Starts the full-screen, interactive terminal monitoring dashboard.
Features 6 live subsystem tabs, CPU/Memory/Disk/Network sparklines, process tree,
interactive filtering & process termination, health diagnostics, and anomaly detection.`,
	RunE: runDashboard,
}

func runDashboard(cmd *cobra.Command, args []string) error {
	cfg := globalCfg
	if dashInterval > 0 {
		cfg.RefreshInterval = dashInterval
	}
	if dashTheme != "" {
		cfg.Dashboard.Theme = dashTheme
	}

	var store storage.Storage
	if cfg.Storage.Enabled && dashRemote == "" {
		var err error
		store, err = storage.NewSQLiteStorage(cfg.Storage.DBPath)
		if err != nil {
			logger.Warnf("Unable to initialize SQLite storage (%s): %v; proceeding with in-memory mode", cfg.Storage.DBPath, err)
		} else {
			defer store.Close()
		}
	}

	col := collector.NewManager(cfg)
	diagEng := diagnostics.NewEngine(cfg)
	alertEng := alerts.NewEngine(cfg, store)
	anomDet := anomaly.NewDetector(cfg)

	// Pre-seed anomaly detector from history if storage is available
	if store != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		snaps, err := store.GetSnapshots(ctx, time.Now().Add(-2*time.Hour), time.Now(), 100)
		cancel()
		if err == nil {
			for _, s := range snaps {
				anomDet.FeedSnapshot(s)
			}
		}
	}

	return tui.Run(cfg, col, store, diagEng, alertEng, anomDet)
}

func init() {
	dashCmd.Flags().DurationVarP(&dashInterval, "interval", "i", 0, "dashboard refresh interval (e.g. 500ms, 1s, 2s)")
	dashCmd.Flags().StringVar(&dashTheme, "theme", "", "color theme (default, dark, light)")
	dashCmd.Flags().StringVar(&dashRemote, "remote", "", "connect to remote Watchdog agent URL (e.g. http://10.0.0.5:8443)")
	dashCmd.Flags().StringVar(&dashToken, "token", "", "authentication token for remote agent")
	dashCmd.Flags().BoolVar(&dashInsecure, "insecure", false, "skip TLS certificate verification for remote agent")

	RootCmd.AddCommand(dashCmd)
}
