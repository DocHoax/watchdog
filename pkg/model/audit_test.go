package model

import (
	"encoding/json"
	"testing"
	"time"
)

func TestAuditEventSerialization(t *testing.T) {
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	evt := AuditEvent{
		ID:        "test-uuid-1234",
		Timestamp: now,
		EventType: EventAuthSuccess,
		Severity:  AuditSeverityInfo,
		Outcome:   AuditOutcomeSuccess,
		Actor: AuditActor{
			Type:     ActorTypeAuthenticatedClient,
			Identity: "api-client",
		},
		Source: AuditSource{
			Address:   "127.0.0.1",
			Transport: "tcp",
			Protocol:  "HTTP/1.1",
			UserAgent: "watchdog-cli/1.0",
			Endpoint:  "/api/v1/snapshot",
			Method:    "GET",
			RequestID: "req-abc-999",
		},
		Resource: "/api/v1/snapshot",
		Action:   "read",
		Message:  "Client authenticated successfully",
		Metadata: map[string]string{
			"auth_method": "bearer_token",
		},
	}

	data, err := json.Marshal(evt)
	if err != nil {
		t.Fatalf("failed to marshal AuditEvent: %v", err)
	}

	var decoded AuditEvent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal AuditEvent: %v", err)
	}

	if decoded.ID != evt.ID {
		t.Errorf("expected ID %q, got %q", evt.ID, decoded.ID)
	}
	if !decoded.Timestamp.Equal(evt.Timestamp) {
		t.Errorf("expected Timestamp %v, got %v", evt.Timestamp, decoded.Timestamp)
	}
	if decoded.EventType != EventAuthSuccess {
		t.Errorf("expected EventType %q, got %q", EventAuthSuccess, decoded.EventType)
	}
	if decoded.Severity != AuditSeverityInfo {
		t.Errorf("expected Severity %q, got %q", AuditSeverityInfo, decoded.Severity)
	}
	if decoded.Outcome != AuditOutcomeSuccess {
		t.Errorf("expected Outcome %q, got %q", AuditOutcomeSuccess, decoded.Outcome)
	}
	if decoded.Actor.Type != ActorTypeAuthenticatedClient || decoded.Actor.Identity != "api-client" {
		t.Errorf("unexpected Actor: %+v", decoded.Actor)
	}
	if decoded.Source.RequestID != "req-abc-999" || decoded.Source.Address != "127.0.0.1" {
		t.Errorf("unexpected Source: %+v", decoded.Source)
	}
	if decoded.Metadata["auth_method"] != "bearer_token" {
		t.Errorf("unexpected Metadata: %+v", decoded.Metadata)
	}
}

func TestAuditConstants(t *testing.T) {
	// Verify event type prefixes and categories
	eventTypes := []string{
		EventAuthSuccess, EventAuthFailure, EventAuthMissingCredentials, EventAuthInvalidCredentials,
		EventServerStart, EventServerStop, EventServerStartFailure, EventServerShutdown,
		EventTLSEnabled, EventTLSDisabled, EventTLSConfigFailure, EventTLSCertFailure, EventTLSKeyFailure,
		EventConfigLoaded, EventConfigValidationFailure, EventConfigChanged, EventConfigSave, EventConfigSaveFailure,
		EventSecurityAuthEnabled, EventSecurityAuthFailure, EventSecurityTLSFailure, EventSecuritySecretFailure,
		EventAdminConfigChange, EventAdminServerStart, EventAdminServerStop, EventAdminExport, EventAdminAuditPurge,
	}

	for _, et := range eventTypes {
		if et == "" {
			t.Errorf("empty event type constant detected")
		}
	}

	severities := []string{
		AuditSeverityInfo, AuditSeverityNotice, AuditSeverityWarning, AuditSeverityError, AuditSeverityCritical,
	}
	for _, sev := range severities {
		if sev == "" {
			t.Errorf("empty severity constant detected")
		}
	}

	outcomes := []string{
		AuditOutcomeSuccess, AuditOutcomeFailure, AuditOutcomeDenied,
	}
	for _, out := range outcomes {
		if out == "" {
			t.Errorf("empty outcome constant detected")
		}
	}
}
