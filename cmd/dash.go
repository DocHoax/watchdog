package cmd

import (
	"context"
	"time"

	"github.com/DocHoax/watchdog/internal/alerts"
	"github.com/DocHoax/watchdog/internal/anomaly"
	"github.com/DocHoax/watchdog/internal/collector"
	"github.com/DocHoax/watchdog/internal/diagnostics"
	"github.com/DocHoax/watchdog/internal/logger"
	"github.com/DocHoax/watchdog/internal/storage"
	"github.com/DocHoax/watchdog/internal/tui"
	"github.com/spf13/cobra"
)

var (
	dashInterval  time.Duration
	dashTheme     string
	dashRemote    string
	dashToken     string
	dashTokenFile string
	dashTokenEnv  string
	dashInsecure  bool
)

var dashCmd = &cobra.Command{
	Use:     "dash [flags]",
	Aliases: []string{"dashboard", "tui", "top"},
	Short:   "Launch the interactive real-time Terminal UI",
	Long: `Starts the full-screen interactive terminal monitoring dashboard.

Features:
  - 6 dedicated subsystem tabs: Dashboard Overview, Processes, CPU & Cores, Memory & Disks, Network & Ports, Docker & Containers.
  - Interactive process table with live sorting, search filtering, signal sending (SIGTERM/SIGKILL), and thread inspection.
  - Live ASCII/Unicode sparklines and real-time history charts.
  - Real-time diagnostic evaluation and statistical anomaly notifications.
  - Optional connection to a remote Watchdog agent server over HTTP/HTTPS.`,
	Example: `  # Launch terminal UI with default 1-second refresh
  watchdog dash

  # Launch with 500ms rapid refresh rate and dark theme
  watchdog dash --interval 500ms --theme dark

  # Connect to a remote server instance with token authentication
  watchdog dash --remote https://node-01.internal:8443 --token <token>

  # Connect to a remote server instance with token from secret file
  watchdog dash --remote https://node-01.internal:8443 --token-file /etc/watchdog/token

  # Fast alias directly from root
  watchdog`,
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

	// Resolve remote token if configured
	if dashRemote != "" {
		remoteAgent := config.AgentConfig{
			Token:     dashToken,
			TokenFile: dashTokenFile,
			TokenEnv:  dashTokenEnv,
		}
		if token, err := remoteAgent.ResolveToken(); err == nil && token != "" {
			dashToken = token
		}
	}

	var store storage.Storage
	if cfg.Storage.Enabled && dashRemote == "" {
		var err error
		store, err = storage.NewSQLiteStorage(storage.Config{Path: cfg.Storage.DBPath})
		if err != nil {
			logger.Warnf("Unable to initialize SQLite storage (%s): %v; proceeding with in-memory mode", cfg.Storage.DBPath, err)
		} else {
			defer store.Close()
		}
	}

	col := collector.NewDefaultManager(cfg)
	diagEng := diagnostics.NewEngine(cfg)
	alertEng := alerts.NewEngine(cfg, store)
	anomDet := anomaly.NewDetector(&cfg.Anomaly)

	// Pre-seed anomaly detector from history if storage is available
	if store != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		pts, err := store.QueryMetrics(ctx, storage.TimeRangeQuery{
			Metric:    "cpu_usage_pct",
			StartTime: time.Now().Add(-2 * time.Hour),
			EndTime:   time.Now(),
			Limit:     100,
		})
		cancel()
		if err == nil {
			for _, p := range pts {
				anomDet.Feed(p.Metric, p.Value, p.Timestamp)
			}
		}
	}

	if err := tui.Run(cfg, col, store, diagEng, alertEng, anomDet); err != nil {
		return WrapExitError(ExitGeneralError, err)
	}
	return nil
}

func init() {
	dashCmd.Flags().DurationVarP(&dashInterval, "interval", "i", 0, "dashboard refresh interval (e.g. 500ms, 1s, 2s)")
	dashCmd.Flags().StringVar(&dashTheme, "theme", "", "color theme palette (default, dark, light)")
	dashCmd.Flags().StringVar(&dashRemote, "remote", "", "connect to remote Watchdog agent URL (e.g. https://10.0.0.5:8443)")
	dashCmd.Flags().StringVar(&dashToken, "token", "", "authentication bearer token for remote agent")
	dashCmd.Flags().StringVar(&dashTokenFile, "token-file", "", "path to file containing authentication token for remote agent")
	dashCmd.Flags().StringVar(&dashTokenEnv, "token-env", "", "environment variable name containing authentication token for remote agent")
	dashCmd.Flags().BoolVar(&dashInsecure, "insecure", false, "skip TLS certificate verification for remote agent")

	RootCmd.AddCommand(dashCmd)
}
