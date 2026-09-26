package cmd

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/DocHoax/watchdog/internal/audit"
	"github.com/DocHoax/watchdog/internal/storage"
	"github.com/DocHoax/watchdog/pkg/model"
	"github.com/spf13/cobra"
)

var (
	auditSince         string
	auditUntil         string
	auditEventType     string
	auditSeverity      string
	auditOutcome       string
	auditSource        string
	auditActor         string
	auditRequestID     string
	auditLimit         int
	auditOffset        int
	auditJSON          bool
	auditCSV           bool
	auditExportOutput  string
	auditExportFormat  string
	auditPurgeOlder    string
	auditRetentionDays int
	auditPurgeForce    bool
)

var auditCmd = &cobra.Command{
	Use:     "audit [command]",
	Aliases: []string{"audits"},
	Short:   "Inspect, query, export, and manage security audit logs",
	Long: `Provides administrative inspection, querying, exporting, and maintenance
for Watchdog structured security audit events stored in SQLite.`,
	Example: `  # List audit events recorded in the last 24 hours
  watchdog audit list

  # List authentication failure events in JSON format
  watchdog audit list --event-type auth.failure --json

  # Export all audit records for the past 7 days to a CSV file
  watchdog audit export --since 7d --format csv --output ./audit_log.csv

  # Purge audit logs older than 90 days
  watchdog audit purge --retention-days 90 --force`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAuditList(cmd, args)
	},
}

var auditListCmd = &cobra.Command{
	Use:   "list [flags]",
	Short: "List and filter recorded security audit events",
	Long:  `Queries and displays recorded security audit events with flexible filtering.`,
	Example: `  # List the latest 50 audit events
  watchdog audit list --limit 50

  # Filter by severity and output as JSON
  watchdog audit list --severity warning --json

  # Filter by event type and time range
  watchdog audit list --event-type auth.success --since 2h`,
	RunE: runAuditList,
}

var auditExportCmd = &cobra.Command{
	Use:   "export [flags]",
	Short: "Export audit logs to JSON or CSV format",
	Long: `Exports recorded security audit events matching filter criteria to a JSON or CSV file.
CSV outputs include automated protection against formula injection attacks.`,
	Example: `  # Export audit logs to JSON file
  watchdog audit export --output /var/log/watchdog/audit_export.json

  # Export audit logs for the last 30 days to CSV
  watchdog audit export --since 30d --format csv --output ./audit.csv`,
	RunE: runAuditExport,
}

var auditPurgeCmd = &cobra.Command{
	Use:   "purge [flags]",
	Short: "Purge historical audit events older than retention cutoff",
	Long:  `Permanently removes historical audit records from local SQLite storage to reclaim disk space.`,
	Example: `  # Purge audit logs older than 90 days
  watchdog audit purge --retention-days 90 --force

  # Purge audit logs older than 720 hours (30 days)
  watchdog audit purge --older-than 720h --force`,
	RunE: runAuditPurge,
}

func runAuditList(cmd *cobra.Command, args []string) error {
	cfg := globalCfg
	if !cfg.Storage.Enabled {
		return NewExitError(ExitConfigError, "storage is disabled; audit logging requires storage.enabled: true")
	}

	store, err := storage.NewSQLiteStorage(storage.Config{Path: cfg.Storage.DBPath})
	if err != nil {
		return NewExitError(ExitGeneralError, "failed to open storage database: %w", err)
	}
	defer store.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	filter, err := buildAuditFilterFromFlags()
	if err != nil {
		return NewExitError(ExitUsageError, "invalid filter parameters: %v", err)
	}

	events, err := store.QueryAuditEvents(ctx, filter)
	if err != nil {
		return NewExitError(ExitGeneralError, "failed to query audit events: %w", err)
	}
	if events == nil {
		events = []model.AuditEvent{}
	}

	total, err := store.CountAuditEvents(ctx, filter)
	if err != nil {
		return NewExitError(ExitGeneralError, "failed to count audit events: %w", err)
	}

	if auditJSON {
		resp := map[string]any{
			"total":     total,
			"count":     len(events),
			"limit":     filter.Limit,
			"offset":    filter.Offset,
			"events":    events,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}
		data, err := json.MarshalIndent(resp, "", "  ")
		if err != nil {
			return NewExitError(ExitGeneralError, "failed to marshal JSON: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}

	if auditCSV {
		csvData, err := renderAuditEventsCSV(events)
		if err != nil {
			return NewExitError(ExitGeneralError, "failed to generate CSV: %w", err)
		}
		fmt.Print(csvData)
		return nil
	}

	// Render ANSI table
	renderAuditTable(events, total, filter.Offset)
	return nil
}

func runAuditExport(cmd *cobra.Command, args []string) error {
	cfg := globalCfg
	if !cfg.Storage.Enabled {
		return NewExitError(ExitConfigError, "storage is disabled; audit export requires storage.enabled: true")
	}

	store, err := storage.NewSQLiteStorage(storage.Config{Path: cfg.Storage.DBPath})
	if err != nil {
		return NewExitError(ExitGeneralError, "failed to open storage database: %w", err)
	}
	defer store.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	filter, err := buildAuditFilterFromFlags()
	if err != nil {
		return NewExitError(ExitUsageError, "invalid filter parameters: %v", err)
	}

	events, err := store.QueryAuditEvents(ctx, filter)
	if err != nil {
		return NewExitError(ExitGeneralError, "failed to query audit events: %w", err)
	}
	if events == nil {
		events = []model.AuditEvent{}
	}

	format := strings.ToLower(auditExportFormat)
	var outputBytes []byte

	switch format {
	case "json":
		outputBytes, err = json.MarshalIndent(events, "", "  ")
		if err != nil {
			return NewExitError(ExitGeneralError, "failed to encode JSON: %w", err)
		}
	case "csv":
		csvStr, err := renderAuditEventsCSV(events)
		if err != nil {
			return NewExitError(ExitGeneralError, "failed to encode CSV: %w", err)
		}
		outputBytes = []byte(csvStr)
	default:
		return NewExitError(ExitUsageError, "unsupported export format %q: choose 'json' or 'csv'", auditExportFormat)
	}

	// Audit the administrative export action
	_ = store.SaveAuditEvent(ctx, model.AuditEvent{
		ID:        audit.GenerateEventID(),
		Timestamp: time.Now().UTC(),
		EventType: model.EventAdminExport,
		Severity:  model.AuditSeverityInfo,
		Outcome:   model.AuditOutcomeSuccess,
		Actor: model.AuditActor{
			Type:     model.ActorTypeCLI,
			Identity: "cli-user",
		},
		Resource: "audit_events",
		Action:   "export",
		Message:  fmt.Sprintf("Exported %d audit events (format: %s)", len(events), format),
	})

	if auditExportOutput != "" {
		if err := os.WriteFile(auditExportOutput, outputBytes, 0600); err != nil {
			return NewExitError(ExitGeneralError, "failed to write export file: %w", err)
		}
		fmt.Printf("✓ Successfully exported %d audit events to %s\n", len(events), auditExportOutput)
	} else {
		fmt.Print(string(outputBytes))
	}

	return nil
}

func runAuditPurge(cmd *cobra.Command, args []string) error {
	cfg := globalCfg
	if !cfg.Storage.Enabled {
		return NewExitError(ExitConfigError, "storage is disabled; audit purge requires storage.enabled: true")
	}

	if !auditPurgeForce {
		return NewExitError(ExitUsageError, "audit purge is permanent and requires --force flag to execute")
	}

	var cutoff time.Time
	var cutoffDesc string

	if auditRetentionDays > 0 {
		cutoff = time.Now().UTC().Add(-time.Duration(auditRetentionDays) * 24 * time.Hour)
		cutoffDesc = fmt.Sprintf("%d days ago (%s)", auditRetentionDays, cutoff.Format(time.RFC3339))
	} else if auditPurgeOlder != "" {
		t, err := parseTimeOrDuration(auditPurgeOlder)
		if err != nil {
			return NewExitError(ExitUsageError, "invalid --older-than parameter: %v", err)
		}
		cutoff = t
		cutoffDesc = fmt.Sprintf("%s (%s)", auditPurgeOlder, cutoff.Format(time.RFC3339))
	} else {
		return NewExitError(ExitUsageError, "either --retention-days or --older-than must be specified")
	}

	store, err := storage.NewSQLiteStorage(storage.Config{Path: cfg.Storage.DBPath})
	if err != nil {
		return NewExitError(ExitGeneralError, "failed to open storage database: %w", err)
	}
	defer store.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	purged, err := store.PurgeAuditEvents(ctx, cutoff)
	if err != nil {
		return NewExitError(ExitGeneralError, "failed to purge audit events: %w", err)
	}

	// Audit the purge action
	_ = store.SaveAuditEvent(ctx, model.AuditEvent{
		ID:        audit.GenerateEventID(),
		Timestamp: time.Now().UTC(),
		EventType: model.EventAdminAuditPurge,
		Severity:  model.AuditSeverityWarning,
		Outcome:   model.AuditOutcomeSuccess,
		Actor: model.AuditActor{
			Type:     model.ActorTypeCLI,
			Identity: "cli-user",
		},
		Resource: "audit_events",
		Action:   "purge",
		Message:  fmt.Sprintf("Purged %d historical audit events older than %s", purged, cutoffDesc),
	})

	fmt.Printf("✓ Successfully purged %d audit events older than %s\n", purged, cutoffDesc)
	return nil
}

func buildAuditFilterFromFlags() (storage.AuditFilter, error) {
	var filter storage.AuditFilter

	if auditSince != "" {
		t, err := parseTimeOrDuration(auditSince)
		if err != nil {
			return filter, fmt.Errorf("invalid --since: %w", err)
		}
		filter.StartTime = t
	}

	if auditUntil != "" {
		t, err := parseTimeOrDuration(auditUntil)
		if err != nil {
			return filter, fmt.Errorf("invalid --until: %w", err)
		}
		filter.EndTime = t
	}

	filter.EventType = strings.TrimSpace(auditEventType)
	filter.Severity = strings.TrimSpace(auditSeverity)
	filter.Outcome = strings.TrimSpace(auditOutcome)
	filter.SourceAddress = strings.TrimSpace(auditSource)
	filter.ActorIdentity = strings.TrimSpace(auditActor)
	filter.RequestID = strings.TrimSpace(auditRequestID)
	filter.Limit = auditLimit
	filter.Offset = auditOffset

	if filter.Limit <= 0 {
		filter.Limit = 100
	}

	return filter, nil
}

// sanitizeCSVCell protects spreadsheet viewers against formula injection attacks
// by escaping cells starting with '=', '+', '-', '@', '\t', '\r'.
func sanitizeCSVCell(val string) string {
	if len(val) == 0 {
		return val
	}
	first := val[0]
	if first == '=' || first == '+' || first == '-' || first == '@' || first == '\t' || first == '\r' {
		return "'" + val
	}
	return val
}

func renderAuditEventsCSV(events []model.AuditEvent) (string, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	header := []string{
		"id", "timestamp", "event_type", "severity", "outcome",
		"actor_type", "actor_identity", "source_address", "endpoint",
		"method", "request_id", "message", "metadata",
	}
	if err := writer.Write(header); err != nil {
		return "", err
	}

	for _, e := range events {
		metaBytes, _ := json.Marshal(e.Metadata)
		record := []string{
			sanitizeCSVCell(e.ID),
			sanitizeCSVCell(e.Timestamp.UTC().Format(time.RFC3339)),
			sanitizeCSVCell(e.EventType),
			sanitizeCSVCell(e.Severity),
			sanitizeCSVCell(e.Outcome),
			sanitizeCSVCell(e.Actor.Type),
			sanitizeCSVCell(e.Actor.Identity),
			sanitizeCSVCell(e.Source.Address),
			sanitizeCSVCell(e.Source.Endpoint),
			sanitizeCSVCell(e.Source.Method),
			sanitizeCSVCell(e.Source.RequestID),
			sanitizeCSVCell(e.Message),
			sanitizeCSVCell(string(metaBytes)),
		}
		if err := writer.Write(record); err != nil {
			return "", err
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func renderAuditTable(events []model.AuditEvent, total int64, offset int) {
	if len(events) == 0 {
		fmt.Println("No audit events found matching the query criteria.")
		return
	}

	fmt.Println()
	fmt.Printf("🔍 Security Audit Events (%d of %d total records):\n", len(events), total)
	fmt.Println(strings.Repeat("─", 120))
	fmt.Printf("%-20s  %-24s  %-8s  %-8s  %-15s  %-16s  %s\n",
		"TIMESTAMP", "EVENT TYPE", "SEVERITY", "OUTCOME", "ACTOR", "SOURCE IP", "MESSAGE")
	fmt.Println(strings.Repeat("─", 120))

	for _, e := range events {
		ts := e.Timestamp.Local().Format("2006-01-02 15:04:05")
		sevColor := colorSeverity(e.Severity)
		outcomeColor := colorOutcome(e.Outcome)

		actor := e.Actor.Identity
		if actor == "" {
			actor = e.Actor.Type
		}
		if len(actor) > 15 {
			actor = actor[:14] + "…"
		}

		source := e.Source.Address
		if len(source) > 16 {
			source = source[:15] + "…"
		}

		msg := e.Message
		if len(msg) > 40 {
			msg = msg[:39] + "…"
		}

		fmt.Printf("%-20s  %-24s  %-8s  %-8s  %-15s  %-16s  %s\n",
			ts,
			e.EventType,
			sevColor,
			outcomeColor,
			actor,
			source,
			msg,
		)
	}
	fmt.Println(strings.Repeat("─", 120))
	if total > int64(offset+len(events)) {
		fmt.Printf("ℹ️  Showing records %d-%d of %d. Use --offset %d to view next page.\n",
			offset+1, offset+len(events), total, offset+len(events))
	}
	fmt.Println()
}

func colorSeverity(sev string) string {
	switch strings.ToLower(sev) {
	case "critical", "error":
		return fmt.Sprintf("\033[31m%-8s\033[0m", sev) // Red
	case "warning":
		return fmt.Sprintf("\033[33m%-8s\033[0m", sev) // Yellow
	case "notice":
		return fmt.Sprintf("\033[36m%-8s\033[0m", sev) // Cyan
	case "info":
		return fmt.Sprintf("\033[32m%-8s\033[0m", sev) // Green
	default:
		return fmt.Sprintf("%-8s", sev)
	}
}

func colorOutcome(outcome string) string {
	switch strings.ToLower(outcome) {
	case "success":
		return fmt.Sprintf("\033[32m%-8s\033[0m", outcome) // Green
	case "failure", "denied":
		return fmt.Sprintf("\033[31m%-8s\033[0m", outcome) // Red
	default:
		return fmt.Sprintf("%-8s", outcome)
	}
}

func parseTimeOrDuration(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC(), nil
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t.UTC(), nil
	}
	if strings.HasSuffix(s, "d") || strings.HasSuffix(s, "D") {
		daysStr := s[:len(s)-1]
		if days, err := strconv.Atoi(daysStr); err == nil && days >= 0 {
			return time.Now().UTC().Add(-time.Duration(days) * 24 * time.Hour), nil
		}
	}
	if dur, err := time.ParseDuration(s); err == nil {
		return time.Now().UTC().Add(-dur), nil
	}
	return time.Time{}, fmt.Errorf("cannot parse '%s' as timestamp or duration", s)
}

func init() {
	// audit list flags
	auditListCmd.Flags().StringVar(&auditSince, "since", "24h", "filter events created since duration or timestamp")
	auditListCmd.Flags().StringVar(&auditUntil, "until", "", "filter events created until duration or timestamp")
	auditListCmd.Flags().StringVarP(&auditEventType, "event-type", "t", "", "filter by audit event type (e.g. auth.failure)")
	auditListCmd.Flags().StringVarP(&auditSeverity, "severity", "s", "", "filter by severity (info, warning, error, critical)")
	auditListCmd.Flags().StringVar(&auditOutcome, "outcome", "", "filter by outcome (success, failure, denied)")
	auditListCmd.Flags().StringVar(&auditSource, "source", "", "filter by source address")
	auditListCmd.Flags().StringVar(&auditActor, "actor", "", "filter by actor identity")
	auditListCmd.Flags().StringVar(&auditRequestID, "request-id", "", "filter by correlation request ID")
	auditListCmd.Flags().IntVarP(&auditLimit, "limit", "l", 100, "maximum number of events to return")
	auditListCmd.Flags().IntVar(&auditOffset, "offset", 0, "offset for pagination")
	auditListCmd.Flags().BoolVar(&auditJSON, "json", false, "output results as JSON")
	auditListCmd.Flags().BoolVar(&auditCSV, "csv", false, "output results as CSV")

	// audit export flags
	auditExportCmd.Flags().StringVarP(&auditExportOutput, "output", "o", "", "file path to write exported records (default: stdout)")
	auditExportCmd.Flags().StringVarP(&auditExportFormat, "format", "f", "json", "export format: json or csv")
	auditExportCmd.Flags().StringVar(&auditSince, "since", "", "filter events created since duration or timestamp")
	auditExportCmd.Flags().StringVar(&auditUntil, "until", "", "filter events created until duration or timestamp")
	auditExportCmd.Flags().StringVarP(&auditEventType, "event-type", "t", "", "filter by audit event type")
	auditExportCmd.Flags().StringVarP(&auditSeverity, "severity", "s", "", "filter by severity")
	auditExportCmd.Flags().StringVar(&auditOutcome, "outcome", "", "filter by outcome")
	auditExportCmd.Flags().StringVar(&auditSource, "source", "", "filter by source address")
	auditExportCmd.Flags().StringVar(&auditActor, "actor", "", "filter by actor identity")
	auditExportCmd.Flags().IntVarP(&auditLimit, "limit", "l", 1000, "maximum number of events to export")

	// audit purge flags
	auditPurgeCmd.Flags().StringVar(&auditPurgeOlder, "older-than", "", "purge records older than duration or timestamp (e.g. 90d, 2160h)")
	auditPurgeCmd.Flags().IntVar(&auditRetentionDays, "retention-days", 0, "purge records older than N days")
	auditPurgeCmd.Flags().BoolVarP(&auditPurgeForce, "force", "f", false, "confirm purge execution without interactive prompt")

	// Base auditCmd flags (inherit list flags for default list behavior)
	auditCmd.Flags().StringVar(&auditSince, "since", "24h", "filter events created since duration or timestamp")
	auditCmd.Flags().StringVar(&auditUntil, "until", "", "filter events created until duration or timestamp")
	auditCmd.Flags().StringVarP(&auditEventType, "event-type", "t", "", "filter by audit event type")
	auditCmd.Flags().StringVarP(&auditSeverity, "severity", "s", "", "filter by severity")
	auditCmd.Flags().StringVar(&auditOutcome, "outcome", "", "filter by outcome")
	auditCmd.Flags().StringVar(&auditSource, "source", "", "filter by source address")
	auditCmd.Flags().StringVar(&auditActor, "actor", "", "filter by actor identity")
	auditCmd.Flags().StringVar(&auditRequestID, "request-id", "", "filter by correlation request ID")
	auditCmd.Flags().IntVarP(&auditLimit, "limit", "l", 100, "maximum number of events to return")
	auditCmd.Flags().IntVar(&auditOffset, "offset", 0, "offset for pagination")
	auditCmd.Flags().BoolVar(&auditJSON, "json", false, "output results as JSON")
	auditCmd.Flags().BoolVar(&auditCSV, "csv", false, "output results as CSV")

	auditCmd.AddCommand(auditListCmd)
	auditCmd.AddCommand(auditExportCmd)
	auditCmd.AddCommand(auditPurgeCmd)

	RootCmd.AddCommand(auditCmd)
}
