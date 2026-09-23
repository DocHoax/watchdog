package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/watchdog-cli/watchdog/internal/alerts"
	"github.com/watchdog-cli/watchdog/internal/collector"
	"github.com/watchdog-cli/watchdog/internal/logger"
	"github.com/watchdog-cli/watchdog/internal/storage"
	"github.com/watchdog-cli/watchdog/pkg/model"
)

var (
	alertJSON     bool
	alertLimit    int
	alertDuration time.Duration
)

var alertCmd = &cobra.Command{
	Use:     "alert [command]",
	Aliases: []string{"alerts"},
	Short:   "Manage and inspect threshold alerts and firing events",
	Long:    `Query active firing alerts, inspect historical alert events, or dispatch synthetic test alerts.`,
	Example: `  # List all currently active and firing alerts
  watchdog alert list

  # Query past 20 historical alert events from SQLite storage
  watchdog alert history --limit 20

  # Trigger a synthetic test alert to verify notification channels
  watchdog alert test`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAlertList(cmd, args)
	},
}

var alertListCmd = &cobra.Command{
	Use:   "list [flags]",
	Short: "List currently active and firing alerts",
	Long:  `Evaluates all active threshold rules against current system state and lists firing alerts.`,
	Example: `  # List firing alerts in terminal format
  watchdog alert list

  # List firing alerts in JSON format for automated monitoring scripts
  watchdog alert list --json`,
	RunE: runAlertList,
}

func runAlertList(cmd *cobra.Command, args []string) error {
	cfg := globalCfg
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

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

	col := collector.NewDefaultManager(cfg)
	snap, err := col.CollectAll(ctx)
	if err != nil {
		logger.Warnf("Partial metrics collection: %v", err)
	}

	alertEng := alerts.NewEngine(cfg, store)
	_, _, _ = alertEng.Evaluate(ctx, snap)
	active := alertEng.GetActiveAlerts()

	if alertJSON {
		data, err := json.MarshalIndent(active, "", "  ")
		if err != nil {
			return NewExitError(ExitGeneralError, "failed to marshal JSON: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}

	if len(active) == 0 {
		fmt.Println("✓ No active alerts firing. All system thresholds within normal bounds.")
		return nil
	}

	fmt.Printf("\n🚨 %d ACTIVE ALERT(S) FIRING:\n", len(active))
	fmt.Println(strings.Repeat("-", 80))
	for i, a := range active {
		statusStr := "ACTIVE"
		if !a.IsActive {
			statusStr = "RESOLVED"
		}
		fmt.Printf("[%d] Rule: %s | Severity: %s | Status: %s\n", i+1, a.RuleName, a.Severity, statusStr)
		fmt.Printf("    Metric : %s = %.2f (Threshold: %.2f)\n", a.MetricName, a.ActualValue, a.Threshold)
		fmt.Printf("    Message: %s\n", a.Message)
		fmt.Printf("    Since  : %s (%s ago)\n\n", a.FiredAt.Format("2006-01-02 15:04:05"), time.Since(a.FiredAt).Truncate(time.Second))
	}

	return nil
}

var alertHistoryCmd = &cobra.Command{
	Use:   "history [flags]",
	Short: "Query past alert event history from SQLite storage",
	Long:  `Retrieves historical alert records, state transitions, and resolution timestamps from local database.`,
	Example: `  # Show latest 50 alert records
  watchdog alert history

  # Show latest 10 alert records in JSON format
  watchdog alert history --limit 10 --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := globalCfg
		if !cfg.Storage.Enabled {
			return NewExitError(ExitConfigError, "storage is disabled in configuration")
		}

		store, err := storage.NewSQLiteStorage(storage.Config{Path: cfg.Storage.DBPath})
		if err != nil {
			return NewExitError(ExitGeneralError, "failed to open storage: %w", err)
		}
		defer store.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if alertLimit <= 0 {
			alertLimit = 50
		}

		history, err := store.GetAlertHistory(ctx, alertLimit, 0)
		if err != nil {
			return NewExitError(ExitGeneralError, "failed to read alert history: %w", err)
		}

		if alertJSON {
			data, err := json.MarshalIndent(history, "", "  ")
			if err != nil {
				return NewExitError(ExitGeneralError, "failed to marshal JSON: %w", err)
			}
			fmt.Println(string(data))
			return nil
		}

		if len(history) == 0 {
			fmt.Println("No historical alert records found in storage database.")
			return nil
		}

		fmt.Printf("\n📋 Alert Event History (Last %d events):\n", len(history))
		fmt.Println(strings.Repeat("-", 85))
		for _, h := range history {
			resolvedStr := "Active"
			if h.ResolvedAt != nil {
				resolvedStr = fmt.Sprintf("Resolved (%s)", h.ResolvedAt.Format("15:04:05"))
			}
			fmt.Printf("%s | %-12s | %-8s | %-20s | %s\n",
				h.FiredAt.Format("2006-01-02 15:04:05"),
				h.RuleName,
				h.Severity,
				resolvedStr,
				h.Message,
			)
		}
		fmt.Println()
		return nil
	},
}

var alertTestCmd = &cobra.Command{
	Use:   "test",
	Short: "Trigger a synthetic test alert to verify notifications and engine",
	Long:  `Dispatches an in-memory synthetic alert event and writes it to storage to test alerting pipelines.`,
	Example: `  # Fire a synthetic test alert
  watchdog alert test`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("🔔 Dispatching synthetic test alert...")
		testAlert := model.AlertEvent{
			ID:          "test-" + time.Now().Format("20060102150405"),
			RuleID:      "test-rule",
			RuleName:    "TestAlertRule",
			Severity:    model.SeverityWarning,
			MetricName:  "cpu_usage_pct",
			ActualValue: 99.9,
			Threshold:   90.0,
			Message:     "Synthetic test alert generated via 'watchdog alert test'",
			FiredAt:     time.Now(),
			IsActive:    true,
		}

		cfg := globalCfg
		if cfg.Storage.Enabled {
			store, err := storage.NewSQLiteStorage(storage.Config{Path: cfg.Storage.DBPath})
			if err == nil {
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				_ = store.SaveAlertEvent(ctx, testAlert)
				store.Close()
				cancel()
			}
		}

		fmt.Printf("✓ Test alert triggered: [%s] %s (%s)\n", testAlert.Severity, testAlert.RuleName, testAlert.Message)
		return nil
	},
}

func init() {
	alertCmd.PersistentFlags().BoolVar(&alertJSON, "json", false, "format output as JSON")
	alertHistoryCmd.Flags().IntVarP(&alertLimit, "limit", "n", 50, "maximum number of historical events to return")
	alertCmd.PersistentFlags().DurationVarP(&alertDuration, "duration", "d", 30*time.Minute, "duration for silence operations")

	alertCmd.AddCommand(alertListCmd)
	alertCmd.AddCommand(alertHistoryCmd)
	alertCmd.AddCommand(alertTestCmd)

	RootCmd.AddCommand(alertCmd)
}
