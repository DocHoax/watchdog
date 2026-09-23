package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/watchdog-cli/watchdog/internal/alerts"
	"github.com/watchdog-cli/watchdog/internal/anomaly"
	"github.com/watchdog-cli/watchdog/internal/collector"
	"github.com/watchdog-cli/watchdog/internal/diagnostics"
	"github.com/watchdog-cli/watchdog/internal/logger"
	"github.com/watchdog-cli/watchdog/internal/reporting"
	"github.com/watchdog-cli/watchdog/internal/storage"
	"github.com/watchdog-cli/watchdog/pkg/model"
)

var (
	reportFormat     string
	reportOutput     string
	reportHistoryDur time.Duration
	reportTitle      string
	reportCharts     bool
	reportRawJSON    bool
)

var reportCmd = &cobra.Command{
	Use:     "report [flags]",
	Aliases: []string{"generate-report", "export-report"},
	Short:   "Generate rich standalone diagnostic reports (HTML, JSON, CSV, Terminal)",
	Long: `Generates comprehensive, multi-format system health and diagnostic reports.

Supported Formats:
  - html     : Standalone self-contained HTML report with dark theme and inline SVG sparklines
  - json     : Structured full snapshot + diagnostics + alerts + anomalies JSON payload
  - csv      : Time-series tabular snapshot metrics in CSV format
  - terminal : Formatted ANSI terminal summary with system breakdown tables`,
	Example: `  # Generate a standalone HTML report with charts and save to file
  watchdog report --format html --output /tmp/watchdog-report.html

  # Generate a raw JSON report and pipe directly to stdout
  watchdog report --format json --output -

  # Generate CSV metrics from the last 2 hours of historical data
  watchdog report --format csv --history 2h --output ./metrics.csv

  # Generate a terminal text summary with a custom title
  watchdog report --format terminal --title "Production Web-01 Health Audit"`,
	RunE: runReport,
}

func runReport(cmd *cobra.Command, args []string) error {
	cfg := globalCfg
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	logger.Infof("Generating system report...")

	var store storage.Storage
	if cfg.Storage.Enabled {
		var err error
		store, err = storage.NewSQLiteStorage(storage.Config{Path: cfg.Storage.DBPath})
		if err != nil {
			logger.Warnf("Storage unavailable: %v", err)
		} else {
			defer store.Close()
		}
	}

	// 1. Metrics Snapshot
	col := collector.NewDefaultManager(cfg)
	snap, err := col.CollectAll(ctx)
	if err != nil {
		logger.Warnf("Partial metrics collected: %v", err)
	}

	// Save snapshot if store available
	if store != nil && snap != nil {
		_ = store.SaveSnapshot(ctx, snap)
	}

	// 2. Diagnostics
	diagEngine := diagnostics.NewEngine(cfg)
	diagReport, _ := diagEngine.Run(ctx, snap)

	// 3. Alerts
	alertEngine := alerts.NewEngine(cfg, store)
	_, _, _ = alertEngine.Evaluate(ctx, snap)
	activeAlerts := alertEngine.GetActiveAlerts()

	// 4. Anomalies
	anomDetector := anomaly.NewDetector(&cfg.Anomaly)
	anomReport := anomDetector.FeedSnapshot(snap)

	var historySnaps []*model.SystemSnapshot
	if snap != nil {
		historySnaps = append(historySnaps, snap)
	}

	// Build unified ReportData
	title := reportTitle
	if title == "" {
		hostName := "Host"
		if snap != nil && snap.System != nil && snap.System.Hostname != "" {
			hostName = snap.System.Hostname
		}
		title = fmt.Sprintf("Watchdog Health Report — %s", hostName)
	}

	reportData := reporting.BuildReportData(title, snap, diagReport, activeAlerts, anomReport)

	var outputBytes []byte
	format := strings.ToLower(reportFormat)

	switch format {
	case "html":
		opts := reporting.HTMLReportOptions{
			Title:          title,
			History:        historySnaps,
			IncludeCharts:  reportCharts,
			IncludeRawJSON: reportRawJSON,
		}
		outputBytes, err = reporting.GenerateHTML(reportData, opts)
		if err != nil {
			return NewExitError(ExitGeneralError, "failed to generate HTML report: %w", err)
		}
		if reportOutput == "" {
			reportOutput = "watchdog-report.html"
		}

	case "json":
		outputBytes, err = reporting.GenerateJSON(reportData)
		if err != nil {
			return NewExitError(ExitGeneralError, "failed to generate JSON report: %w", err)
		}

	case "csv":
		snapsToExport := historySnaps
		if len(snapsToExport) == 0 && snap != nil {
			snapsToExport = []*model.SystemSnapshot{snap}
		}
		outputBytes, err = reporting.GenerateSnapshotsCSV(snapsToExport)
		if err != nil {
			return NewExitError(ExitGeneralError, "failed to generate CSV report: %w", err)
		}
		if reportOutput == "" {
			reportOutput = "watchdog-metrics.csv"
		}

	case "terminal", "text", "ansi":
		outputStr := reporting.GenerateTerminal(reportData)
		outputBytes = []byte(outputStr)

	default:
		return NewExitError(ExitUsageError, "unsupported report format %q; use html, json, csv, or terminal", reportFormat)
	}

	// Output target
	if reportOutput == "" || reportOutput == "-" {
		fmt.Print(string(outputBytes))
	} else {
		// Ensure parent directory exists
		if dir := filepath.Dir(reportOutput); dir != "." && dir != "" {
			_ = os.MkdirAll(dir, 0755)
		}
		if err := os.WriteFile(reportOutput, outputBytes, 0644); err != nil {
			return NewExitError(ExitGeneralError, "failed to write report to %s: %w", reportOutput, err)
		}
		logger.Infof("Report successfully written to: %s (%d bytes)", reportOutput, len(outputBytes))
		fmt.Printf("✓ Report saved to %s\n", reportOutput)
	}

	return nil
}

func init() {
	reportCmd.Flags().StringVarP(&reportFormat, "format", "f", "html", "output format: html, json, csv, terminal")
	reportCmd.Flags().StringVarP(&reportOutput, "output", "o", "", "output file destination path (or '-' for stdout)")
	reportCmd.Flags().DurationVarP(&reportHistoryDur, "history", "H", 1*time.Hour, "historical time-window for trend sparklines (e.g. 30m, 1h, 24h)")
	reportCmd.Flags().StringVarP(&reportTitle, "title", "t", "", "custom report header title")
	reportCmd.Flags().BoolVar(&reportCharts, "charts", true, "include inline SVG trend charts in HTML output")
	reportCmd.Flags().BoolVar(&reportRawJSON, "raw-json", false, "embed raw snapshot payload in HTML report")

	RootCmd.AddCommand(reportCmd)
}
