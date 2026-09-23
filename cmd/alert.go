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
	Use:     "alert",
	Aliases: []string{"alerts"},
	Short:   "Manage and inspect threshold alerts and firing events",
	Long:    `Query active firing alerts, inspect historical alert events, or test threshold rules.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAlertList(cmd, args)
	},
}

var alertListCmd = &cobra.Command{
	Use:   "list",
	Short: "List currently active and firing alerts",
	RunE:  runAlertList,
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
			return err
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
		fmt.Printf("[%d] Rule: %s | Severity: %s | Status: %s\n", i+1, a.RuleName, a.Severity, a.Status)
		fmt.Printf("    Metric : %s = %.2f (Threshold: %.2f)\n", a.Metric, a.MetricValue, a.Threshold)
		fmt.Printf("    Message: %s\n", a.Message)
		fmt.Printf("    Since  : %s (%s ago)\n\n", a.TriggeredAt.Format("2006-01-02 15:04:05"), time.Since(a.TriggeredAt).Truncate(time.Second))
	}

	return nil
}

var alertHistoryCmd = &cobra.Command{
	Use:   "history",
	Short: "Query past alert event history from SQLite storage",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := globalCfg
		if !cfg.Storage.Enabled {
			return fmt.Errorf("storage is disabled in configuration")
		}

		store, err := storage.NewSQLiteStorage(storage.Config{Path: cfg.Storage.DBPath})
		if err != nil {
			return fmt.Errorf("failed to open storage: %w", err)
		}
		defer store.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if alertLimit <= 0 {
			alertLimit = 50
		}

		history, err := store.GetAlertHistory(ctx, alertLimit, 0)
		if err != nil {
			return fmt.Errorf("failed to read alert history: %w", err)
		}

		if alertJSON {
			data, err := json.MarshalIndent(history, "", "  ")
			if err != nil {
				return err
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
				h.TriggeredAt.Format("2006-01-02 15:04:05"),
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
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("🔔 Dispatching synthetic test alert...")
		testAlert := model.AlertEvent{
			ID:           "test-" + time.Now().Format("20060102150405"),
			RuleName:     "TestAlertRule",
			Category:     "System",
			Severity:     model.SeverityWarning,
			Status:       model.AlertStatusFiring,
			Metric:       "cpu_usage_pct",
			MetricValue:  99.9,
			Threshold:    90.0,
			Message:      "Synthetic test alert generated via 'watchdog alert test'",
			TriggeredAt:  time.Now(),
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
