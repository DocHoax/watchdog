package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/DocHoax/watchdog/internal/config"
	"github.com/DocHoax/watchdog/internal/logger"
)

var (
	cfgFile    string
	verbose    bool
	quiet      bool
	jsonLogs   bool
	noColor    bool
	globalCfg  *config.Config
	loadedPath string
)

// RootCmd is the base command for Watchdog.
var RootCmd = &cobra.Command{
	Use:           "watchdog [command] [flags]",
	Short:         "🐺 Watchdog: Enterprise-Grade System Monitoring & Diagnostics CLI",
	SilenceUsage:  true,
	SilenceErrors: true,
	Long: `Watchdog is a high-performance, cross-platform system monitoring,
automated diagnostics, statistical anomaly detection, SQLite storage engine, and interactive TUI suite.

Key Capabilities:
  - Interactive Terminal Dashboard: Multi-tab real-time CPU/RAM/Disk/Net/Process/Docker TUI.
  - Automated Health Diagnostics: Concurrent heuristic rules with remediation advice.
  - Statistical Anomaly Detection: Real-time Z-score & EWMA anomaly scoring without ML dependencies.
  - Standalone Multi-Format Reports: Rich self-contained HTML (with inline charts), JSON, and CSV.
  - Headless Daemon & Prometheus: Production-ready HTTP REST API and Prometheus metrics exporter.`,
	Example: `  # Launch the interactive real-time terminal UI
  watchdog

  # Run automated health diagnostics and print issues
  watchdog diagnose

  # Generate a standalone HTML health report with charts
  watchdog report -f html -o /tmp/report.html

  # Start the background Prometheus metrics and REST server on port 8443
  watchdog server --port 8443 --token s3cr3t-t0k3n

  # Run with custom configuration and verbose debug logging
  watchdog --config /etc/watchdog.yaml --verbose`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Setup logging level
		lvl := logger.LevelInfo
		if quiet {
			lvl = logger.LevelError
		} else if verbose {
			lvl = logger.LevelDebug
		}
		log := logger.New(os.Stderr, lvl, jsonLogs, noColor)
		logger.SetDefault(log)

		// Load configuration
		cfg, path, err := config.Load(cfgFile)
		if err != nil {
			if cfgFile != "" {
				return NewExitError(ExitConfigError, "failed to load specified configuration file %q: %w", cfgFile, err)
			}
			logger.Warnf("Warning loading config: %v; using default settings", err)
			cfg = config.DefaultConfig()
		}
		globalCfg = cfg
		loadedPath = path
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		// Default action when invoked with no args: launch the dashboard TUI
		return runDashboard(cmd, args)
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		code := GetExitCode(err)
		os.Exit(code)
	}
}

func init() {
	RootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "configuration file path (default: $HOME/.watchdog/config.yaml)")
	RootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose / debug logging output")
	RootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "suppress informational log output (errors only)")
	RootCmd.PersistentFlags().BoolVar(&jsonLogs, "json-logs", false, "output logs in structured JSON format")
	RootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "disable ANSI color formatting in output")
}
