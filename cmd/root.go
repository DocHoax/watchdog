package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/watchdog-cli/watchdog/internal/config"
	"github.com/watchdog-cli/watchdog/internal/logger"
)

var (
	cfgFile   string
	verbose   bool
	jsonLogs  bool
	noColor   bool
	globalCfg *config.Config
	loadedPath string
)

// RootCmd is the base command for Watchdog.
var RootCmd = &cobra.Command{
	Use:           "watchdog",
	Short:         "🐺 Watchdog: Enterprise-Grade System Monitoring & Diagnostics CLI",
	SilenceUsage:  true,
	SilenceErrors: true,
	Long: `Watchdog is a high-performance, cross-platform system monitoring,
automated diagnostics, statistical anomaly detection, and interactive TUI suite.

Run 'watchdog' or 'watchdog dash' to launch the real-time interactive terminal UI.
Run 'watchdog diagnose' to execute instant health checks with automated remediation.
Run 'watchdog report' to generate rich standalone HTML/JSON/CSV diagnostic reports.
Run 'watchdog server' to start the Prometheus exporter and remote API server.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Setup logging
		lvl := logger.LevelInfo
		if verbose {
			lvl = logger.LevelDebug
		}
		log := logger.New(os.Stderr, lvl, jsonLogs, noColor)
		logger.SetDefault(log)

		// Load configuration
		cfg, path, err := config.Load(cfgFile)
		if err != nil {
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
		os.Exit(1)
	}
}

func init() {
	RootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file path (default is $HOME/.watchdog/config.yaml)")
	RootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose / debug logging output")
	RootCmd.PersistentFlags().BoolVar(&jsonLogs, "json-logs", false, "output logs in structured JSON format")
	RootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "disable ANSI color formatting in output")
}
