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
	auditFormat = "terminal"
	auditJSON = false

	err := runAuditList(auditListCmd, []string{})
	if err != nil {
		t.Fatalf("runAuditList failed: %v", err)
	}

	// 2. Test JSON listing via --format json
	auditFormat = "json"
	auditJSON = false
	err = runAuditList(auditListCmd, []string{})
	if err != nil {
		t.Fatalf("runAuditList with --format json failed: %v", err)
	}

	// 3. Test JSON listing via --json flag
	auditFormat = "terminal"
	auditJSON = true
	err = runAuditList(auditListCmd, []string{})
	if err != nil {
		t.Fatalf("runAuditList with --json failed: %v", err)
	}

	// 4. Test unsupported format
	auditFormat = "unsupported_fmt"
	auditJSON = false
	err = runAuditList(auditListCmd, []string{})
	if err == nil {
		t.Fatal("expected error for unsupported format, got nil")
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
	auditRequestID = ""
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
