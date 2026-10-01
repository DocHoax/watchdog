package governance

import (
	"context"
	"testing"
	"time"

	"github.com/DocHoax/watchdog/internal/storage"
	"github.com/DocHoax/watchdog/pkg/model"
)

func TestAuditSanitization(t *testing.T) {
	rawMeta := map[string]string{
		"org_id":          "org-acme",
		"auth_token":      "secret-token-123456",
		"api_key":         "key-abcdef1234567890",
		"user_password":   "SuperSecretPass!",
		"client_secret":   "oauth-client-secret-value",
		"safe_property":   "value-123",
		"freeform_notes":  "User connected with Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9 and apiKey: secretapikey12345",
	}

	sanitized := SanitizeMetadata(rawMeta)

	if sanitized["org_id"] != "org-acme" {
		t.Errorf("expected org_id to be preserved, got %q", sanitized["org_id"])
	}
	if sanitized["safe_property"] != "value-123" {
		t.Errorf("expected safe_property to be preserved, got %q", sanitized["safe_property"])
	}
	if sanitized["auth_token"] != RedactedPlaceholder {
		t.Errorf("expected auth_token to be redacted, got %q", sanitized["auth_token"])
	}
	if sanitized["api_key"] != RedactedPlaceholder {
		t.Errorf("expected api_key to be redacted, got %q", sanitized["api_key"])
	}
	if sanitized["user_password"] != RedactedPlaceholder {
		t.Errorf("expected user_password to be redacted, got %q", sanitized["user_password"])
	}
	if sanitized["client_secret"] != RedactedPlaceholder {
		t.Errorf("expected client_secret to be redacted, got %q", sanitized["client_secret"])
	}

	// Verify Bearer and apiKey in free-form text
	notes := sanitized["freeform_notes"]
	if !stringsContains(notes, "Bearer "+RedactedPlaceholder) {
		t.Errorf("expected bearer token to be scrubbed in notes, got %q", notes)
	}
	if !stringsContains(notes, "apiKey: "+RedactedPlaceholder) && !stringsContains(notes, "apiKey:"+RedactedPlaceholder) {
		t.Errorf("expected apiKey to be scrubbed in notes, got %q", notes)
	}
}

func TestAuditRecorder_Recording(t *testing.T) {
	ctx := context.Background()
	store, err := storage.NewSQLiteStorage(storage.Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer store.Close()

	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	recorder := NewAuditRecorder(store, NewMockClock(now))

	err = recorder.RecordEvent(
		ctx,
		model.EventGovernanceMaintenanceCreated,
		model.AuditSeverityNotice,
		model.AuditOutcomeSuccess,
		"mw-123",
		"create",
		"Created maintenance window with api_key=secret-api-key-12345",
		model.AuditActor{Type: model.ActorTypeCLI, Identity: "admin-user"},
		map[string]string{
			"org_id":     "org-acme",
			"window_id":  "mw-123",
			"auth_token": "token-xyz",
		},
	)
	if err != nil {
		t.Fatalf("failed to record event: %v", err)
	}

	events, err := store.QueryAuditEvents(ctx, storage.AuditFilter{
		EventType: model.EventGovernanceMaintenanceCreated,
	})
	if err != nil {
		t.Fatalf("failed to query audit events: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(events))
	}

	ev := events[0]
	if ev.EventType != model.EventGovernanceMaintenanceCreated {
		t.Errorf("expected event type %s, got %s", model.EventGovernanceMaintenanceCreated, ev.EventType)
	}
	if !ev.Timestamp.Equal(now) {
		t.Errorf("expected timestamp %v, got %v", now, ev.Timestamp)
	}
	if ev.Metadata["auth_token"] != RedactedPlaceholder {
		t.Errorf("expected auth_token to be redacted in store, got %q", ev.Metadata["auth_token"])
	}
	if !stringsContains(ev.Message, RedactedPlaceholder) {
		t.Errorf("expected message to have credentials redacted, got %q", ev.Message)
	}
}

func stringsContains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || indexOfString(s, substr) >= 0)
}

func indexOfString(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
