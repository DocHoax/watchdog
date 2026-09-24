package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/DocHoax/watchdog/internal/collector"
	"github.com/DocHoax/watchdog/internal/reporting"
	"github.com/DocHoax/watchdog/internal/storage"
	"github.com/DocHoax/watchdog/pkg/model"
)

var (
	exportFormat string
	exportMetric string
	exportOutput string
	exportSince  time.Duration
	exportLimit  int
)

var exportCmd = &cobra.Command{
	Use:   "export [flags]",
	Short: "Export system metrics or history in JSON or CSV",
	Long: `Exports live snapshot data or historical time-series metric series from local storage.
Supports JSON formatted object arrays and tabular CSV output.`,
	Example: `  # Export a real-time full system snapshot to JSON
  watchdog export --format json

  # Export historical CPU usage over the past 2 hours as CSV
  watchdog export --metric cpu_usage_pct --since 2h --format csv --output ./cpu.csv

  # Export the latest 500 records of memory usage to a JSON file
  watchdog export --metric memory_used_pct --limit 500 --output /tmp/memory.json`,
	RunE: runExport,
}

func runExport(cmd *cobra.Command, args []string) error {
	cfg := globalCfg
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var outputBytes []byte
	format := strings.ToLower(exportFormat)

	if exportMetric != "" && cfg.Storage.Enabled {
		// Query specific metric from storage
		store, err := storage.NewSQLiteStorage(storage.Config{Path: cfg.Storage.DBPath})
		if err != nil {
			return NewExitError(ExitGeneralError, "failed to open storage: %w", err)
		}
		defer store.Close()

		startTime := time.Now().Add(-exportSince)
		if exportSince <= 0 {
			startTime = time.Now().Add(-1 * time.Hour)
		}

		pts, err := store.QueryMetrics(ctx, storage.TimeRangeQuery{
			Metric:    exportMetric,
			StartTime: startTime,
			EndTime:   time.Now(),
			Limit:     exportLimit,
		})
		if err != nil {
			return NewExitError(ExitGeneralError, "failed to query metric %s: %w", exportMetric, err)
		}

		switch format {
		case "json":
			outputBytes, err = json.MarshalIndent(pts, "", "  ")
			if err != nil {
				return NewExitError(ExitGeneralError, "failed to marshal JSON: %w", err)
			}
		case "csv":
			var sb strings.Builder
			sb.WriteString("timestamp,metric,value,hostname\n")
			for _, p := range pts {
				sb.WriteString(fmt.Sprintf("%s,%s,%f,%s\n",
					p.Timestamp.Format(time.RFC3339),
					p.Metric,
					p.Value,
					p.Hostname,
				))
			}
			outputBytes = []byte(sb.String())
		default:
			return NewExitError(ExitUsageError, "unsupported format %q for metric export (use json or csv)", exportFormat)
		}
	} else {
		// Live single snapshot export
		col := collector.NewDefaultManager(cfg)
		snap, err := col.CollectAll(ctx)
		if err != nil {
			return NewExitError(ExitGeneralError, "failed to collect snapshot: %w", err)
		}

		switch format {
		case "json":
			outputBytes, err = json.MarshalIndent(snap, "", "  ")
			if err != nil {
				return NewExitError(ExitGeneralError, "failed to marshal JSON: %w", err)
			}
		case "csv":
			outputBytes, err = reporting.GenerateSnapshotsCSV([]*model.SystemSnapshot{snap})
			if err != nil {
				return NewExitError(ExitGeneralError, "failed to generate CSV: %w", err)
			}
		default:
			return NewExitError(ExitUsageError, "unsupported format %q; use json or csv", exportFormat)
		}
	}

	if exportOutput == "" || exportOutput == "-" {
		fmt.Println(string(outputBytes))
	} else {
		if dir := filepath.Dir(exportOutput); dir != "." && dir != "" {
			_ = os.MkdirAll(dir, 0755)
		}
		if err := os.WriteFile(exportOutput, outputBytes, 0644); err != nil {
			return NewExitError(ExitGeneralError, "failed to write output to %s: %w", exportOutput, err)
		}
		fmt.Printf("✓ Exported %d bytes to %s\n", len(outputBytes), exportOutput)
	}

	return nil
}

func init() {
	exportCmd.Flags().StringVarP(&exportFormat, "format", "f", "json", "export format (json, csv)")
	exportCmd.Flags().StringVarP(&exportMetric, "metric", "m", "", "specific time-series metric name to export (e.g. cpu_usage_pct, memory_used_pct)")
	exportCmd.Flags().StringVarP(&exportOutput, "output", "o", "", "file path to save exported data (default is stdout)")
	exportCmd.Flags().DurationVar(&exportSince, "since", 1*time.Hour, "historical time duration when exporting metric history")
	exportCmd.Flags().IntVarP(&exportLimit, "limit", "n", 1000, "maximum records to export")

	RootCmd.AddCommand(exportCmd)
}
