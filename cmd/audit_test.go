package cmd

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DocHoax/watchdog/internal/config"
	"github.com/DocHoax/watchdog/internal/storage"
	"github.com/DocHoax/watchdog/pkg/model"
)

func seedTestAuditData(t *testing.T, dbPath string) {
	t.Helper()
	store, err := storage.NewSQLiteStorage(storage.Config{Path: dbPath})
	if err != nil {
		t.Fatalf("failed to open storage for seeding: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	now := time.Now().UTC()

	events := []model.AuditEvent{
		{
			ID:        "evt-cli-001",
			Timestamp: now.Add(-2 * time.Hour),
			EventType: model.EventAuthSuccess,
			Severity:  model.AuditSeverityInfo,
			Outcome:   model.AuditOutcomeSuccess,
			Actor: model.AuditActor{
				Type:     model.ActorTypeAuthenticatedClient,
				Identity: "admin-user",
			},
			Source: model.AuditSource{
				Address:   "127.0.0.1",
				Endpoint:  "/api/v1/snapshot",
				Method:    "GET",
				RequestID: "req-cli-1",
			},
			Message: "Admin authenticated successfully",
		},
		{
			ID:        "evt-cli-002",
			Timestamp: now.Add(-1 * time.Hour),
			EventType: model.EventAuthInvalidCredentials,
			Severity:  model.AuditSeverityWarning,
			Outcome:   model.AuditOutcomeDenied,
			Actor: model.AuditActor{
				Type:     model.ActorTypeAnonymousClient,
				Identity: "token:sha256:abcd1234",
			},
			Source: model.AuditSource{
				Address:   "10.0.0.99",
				Endpoint:  "/api/v1/snapshot",
				Method:    "GET",
				RequestID: "req-cli-2",
			},
			Message: "=SUM(1+1) formula test in message",
		},
		{
			ID:        "evt-cli-003",
			Timestamp: now.Add(-100 * 24 * time.Hour), // 100 days old
			EventType: model.EventServerStart,
			Severity:  model.AuditSeverityInfo,
			Outcome:   model.AuditOutcomeSuccess,
			Actor: model.AuditActor{
				Type:     model.ActorTypeSystem,
				Identity: "watchdog-daemon",
			},
			Source: model.AuditSource{
				Address: "127.0.0.1:8443",
			},
			Message: "+cmd|' /C calc'!A0 injection test in message",
		},
	}

	if err := store.SaveAuditEvents(ctx, events); err != nil {
		t.Fatalf("failed to seed audit events: %v", err)
	}
}

func TestSanitizeCSVCell(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"normal text", "normal text"},
		{"", ""},
		{"=1+1", "'=1+1"},
		{"+SUM(A1:A10)", "'+SUM(A1:A10)"},
		{"-1234", "'-1234"},
		{"@SUM(A1)", "'@SUM(A1)"},
		{"\tmalicious tab", "'\tmalicious tab"},
		{"\rmalicious return", "'\rmalicious return"},
		{"safe_user_id_123", "safe_user_id_123"},
	}

	for _, tt := range tests {
		got := sanitizeCSVCell(tt.input)
		if got != tt.expected {
			t.Errorf("sanitizeCSVCell(%q) = %q, expected %q", tt.input, got, tt.expected)
		}
	}
}

func TestRunAuditList(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "audit_test.db")
	seedTestAuditData(t, dbPath)

	oldCfg := globalCfg
	defer func() { globalCfg = oldCfg }()

	globalCfg = config.DefaultConfig()
	globalCfg.Storage.Enabled = true
	globalCfg.Storage.DBPath = dbPath

	// 1. Test basic table listing
	auditSince = "24h"
	auditUntil = ""
	auditEventType = ""
	auditSeverity = ""
	auditOutcome = ""
	auditSource = ""
	auditActor = ""
	auditRequestID = ""
	auditLimit = 100
	auditOffset = 0
	auditJSON = false
	auditCSV = false

	err := runAuditList(auditListCmd, []string{})
	if err != nil {
		t.Fatalf("runAuditList failed: %v", err)
	}

	// 2. Test JSON listing
	auditJSON = true
	auditCSV = false
	err = runAuditList(auditListCmd, []string{})
	if err != nil {
		t.Fatalf("runAuditList with --json failed: %v", err)
	}

	// 3. Test CSV listing
	auditJSON = false
	auditCSV = true
	err = runAuditList(auditListCmd, []string{})
	if err != nil {
		t.Fatalf("runAuditList with --csv failed: %v", err)
	}
}

func TestRunAuditExport(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "audit_export_test.db")
	seedTestAuditData(t, dbPath)

	oldCfg := globalCfg
	defer func() { globalCfg = oldCfg }()

	globalCfg = config.DefaultConfig()
	globalCfg.Storage.Enabled = true
	globalCfg.Storage.DBPath = dbPath

	// Reset flags
	auditSince = "365d"
	auditUntil = ""
	auditEventType = ""
	auditSeverity = ""
	auditOutcome = ""
	auditSource = ""
	auditActor = ""
	auditLimit = 1000

	// 1. Export JSON to file
	jsonFile := filepath.Join(tmpDir, "export.json")
	auditExportFormat = "json"
	auditExportOutput = jsonFile

	if err := runAuditExport(auditExportCmd, []string{}); err != nil {
		t.Fatalf("runAuditExport (json) failed: %v", err)
	}

	jsonBytes, err := os.ReadFile(jsonFile)
	if err != nil {
		t.Fatalf("failed to read exported JSON file: %v", err)
	}
	var exportedEvents []model.AuditEvent
	if err := json.Unmarshal(jsonBytes, &exportedEvents); err != nil {
		t.Fatalf("failed to unmarshal exported JSON: %v", err)
	}
	if len(exportedEvents) != 3 {
		t.Errorf("expected 3 exported events, got %d", len(exportedEvents))
	}

	// 2. Export CSV to file and verify formula injection neutralization
	csvFile := filepath.Join(tmpDir, "export.csv")
	auditExportFormat = "csv"
	auditExportOutput = csvFile

	if err := runAuditExport(auditExportCmd, []string{}); err != nil {
		t.Fatalf("runAuditExport (csv) failed: %v", err)
	}

	csvBytes, err := os.ReadFile(csvFile)
	if err != nil {
		t.Fatalf("failed to read exported CSV file: %v", err)
	}
	csvContent := string(csvBytes)

	// Check CSV headers
	if !strings.Contains(csvContent, "id,timestamp,event_type,severity,outcome") {
		t.Errorf("CSV missing expected header: %s", csvContent)
	}

	// Verify formula injection prefixes were escaped with single quote
	if !strings.Contains(csvContent, "'=SUM(1+1)") {
		t.Errorf("expected escaped formula '=SUM(1+1) in CSV, got: %s", csvContent)
	}
	if !strings.Contains(csvContent, "'+cmd|") {
		t.Errorf("expected escaped formula '+cmd| in CSV, got: %s", csvContent)
	}

	// 3. Test unsupported export format
	auditExportFormat = "xml"
	if err := runAuditExport(auditExportCmd, []string{}); err == nil {
		t.Fatal("expected error for unsupported export format 'xml', got nil")
	}
}

func TestRunAuditPurge(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "audit_purge_test.db")
	seedTestAuditData(t, dbPath)

	oldCfg := globalCfg
	defer func() { globalCfg = oldCfg }()

	globalCfg = config.DefaultConfig()
	globalCfg.Storage.Enabled = true
	globalCfg.Storage.DBPath = dbPath

	// 1. Purge without --force should fail
	auditRetentionDays = 90
	auditPurgeOlder = ""
	auditPurgeForce = false

	if err := runAuditPurge(auditPurgeCmd, []string{}); err == nil {
		t.Fatal("expected error when running audit purge without --force, got nil")
	}

	// 2. Purge with --force and --retention-days
	auditPurgeForce = true
	auditRetentionDays = 90

	if err := runAuditPurge(auditPurgeCmd, []string{}); err != nil {
		t.Fatalf("runAuditPurge failed: %v", err)
	}

	// Verify 100-day old event was deleted, and admin.audit.purge event was recorded
	store, err := storage.NewSQLiteStorage(storage.Config{Path: dbPath})
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	events, err := store.QueryAuditEvents(ctx, storage.AuditFilter{Limit: 100})
	if err != nil {
		t.Fatalf("failed to query audit events: %v", err)
	}

	for _, e := range events {
		if e.ID == "evt-cli-003" {
			t.Errorf("expected evt-cli-003 to be purged, but found: %+v", e)
		}
	}

	// Verify purge audit event exists
	foundPurgeAudit := false
	for _, e := range events {
		if e.EventType == model.EventAdminAuditPurge {
			foundPurgeAudit = true
			if e.Severity != model.AuditSeverityWarning {
				t.Errorf("expected severity warning, got %s", e.Severity)
			}
			if e.Actor.Type != model.ActorTypeCLI {
				t.Errorf("expected actor type cli, got %s", e.Actor.Type)
			}
		}
	}
	if !foundPurgeAudit {
		t.Error("expected to find admin.audit.purge audit event recorded in storage")
	}
}

func TestRenderAuditTable(t *testing.T) {
	// Should not panic on empty or non-empty events
	renderAuditTable([]model.AuditEvent{}, 0, 0)

	events := []model.AuditEvent{
		{
			ID:        "evt-tbl-1",
			Timestamp: time.Now().UTC(),
			EventType: model.EventAuthSuccess,
			Severity:  model.AuditSeverityInfo,
			Outcome:   model.AuditOutcomeSuccess,
			Actor: model.AuditActor{
				Type:     model.ActorTypeAuthenticatedClient,
				Identity: "long-actor-name-exceeding-limit",
			},
			Source: model.AuditSource{
				Address: "192.168.1.100:54321",
			},
			Message: "A very long message description that exceeds forty characters for formatting truncation test",
		},
	}

	renderAuditTable(events, 1, 0)
}

func TestRenderAuditEventsCSV(t *testing.T) {
	events := []model.AuditEvent{
		{
			ID:        "evt-csv-1",
			Timestamp: time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC),
			EventType: model.EventAuthSuccess,
			Severity:  model.AuditSeverityInfo,
			Outcome:   model.AuditOutcomeSuccess,
			Actor: model.AuditActor{
				Type:     model.ActorTypeAuthenticatedClient,
				Identity: "user1",
			},
			Source: model.AuditSource{
				Address:   "127.0.0.1",
				Endpoint:  "/api/v1/snapshot",
				Method:    "GET",
				RequestID: "req-1",
			},
			Message: "Normal message",
			Metadata: map[string]string{
				"key": "value",
			},
		},
	}

	csvStr, err := renderAuditEventsCSV(events)
	if err != nil {
		t.Fatalf("renderAuditEventsCSV failed: %v", err)
	}

	if !strings.Contains(csvStr, "evt-csv-1") {
		t.Errorf("expected event id in CSV, got: %s", csvStr)
	}
}

func TestParseTimeOrDuration(t *testing.T) {
	// RFC3339
	t1, err := parseTimeOrDuration("2026-09-26T12:00:00Z")
	if err != nil || t1.Year() != 2026 {
		t.Errorf("failed to parse RFC3339: %v, got %v", err, t1)
	}

	// Date format
	t2, err := parseTimeOrDuration("2026-09-26")
	if err != nil || t2.Day() != 26 {
		t.Errorf("failed to parse date: %v, got %v", err, t2)
	}

	// Days duration
	t3, err := parseTimeOrDuration("7d")
	if err != nil || time.Since(t3) < 6*24*time.Hour {
		t.Errorf("failed to parse 7d: %v, got %v", err, t3)
	}

	// Go duration
	t4, err := parseTimeOrDuration("24h")
	if err != nil || time.Since(t4) < 23*time.Hour {
		t.Errorf("failed to parse 24h: %v, got %v", err, t4)
	}

	// Invalid
	_, err = parseTimeOrDuration("invalid-time-format-xyz")
	if err == nil {
		t.Error("expected error for invalid time format, got nil")
	}
}
