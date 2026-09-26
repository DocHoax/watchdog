package audit

import (
	"strings"
	"testing"
	"time"

	"github.com/DocHoax/watchdog/pkg/model"
)

func TestSanitizer_KeyDetection(t *testing.T) {
	sensitiveKeys := []string{
		"token", "Token", "TOKEN", "auth_token", "agent_token",
		"password", "secret", "authorization", "Authorization",
		"private_key", "tls_key", "cookie", "credential",
		"api_key", "apiKey", "dsn", "database_url", "cert_key",
		"bearer_token", "passphrase",
	}

	for _, k := range sensitiveKeys {
		if !IsSensitiveKey(k) {
			t.Errorf("expected key %q to be detected as sensitive", k)
		}
	}

	safeKeys := []string{
		"request_id", "endpoint", "method", "remote_ip", "hostname", "duration_ms", "status_code",
	}
	for _, k := range safeKeys {
		if IsSensitiveKey(k) {
			t.Errorf("expected key %q to be safe, but detected as sensitive", k)
		}
	}
}

func TestSanitizer_SanitizeValue(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Bearer secret-token-12345", "[REDACTED]"},
		{"bearer my-api-token", "[REDACTED]"},
		{"Basic dXNlcjpwYXNz", "[REDACTED]"},
		{"-----BEGIN RSA PRIVATE KEY-----\nMIIE...", "[REDACTED]"},
		{"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgN_pnyVvW_Q_g_m_4", "[REDACTED]"},
		{"normal text value", "normal text value"},
		{"GET", "GET"},
		{"127.0.0.1", "127.0.0.1"},
	}

	for _, tc := range tests {
		got := SanitizeValue(tc.input)
		if got != tc.expected {
			t.Errorf("SanitizeValue(%q) = %q, expected %q", tc.input, got, tc.expected)
		}
	}
}

func TestSanitizer_SanitizeMetadata(t *testing.T) {
	meta := map[string]string{
		"client_id":       "my-client",
		"auth_token":      "secret-token-value",
		"password":        "secret-pass",
		"Authorization":   "Bearer header-token",
		"tls_key":         "private-key-data",
		"request_id":      "req-123",
		"raw_auth_header": "Bearer raw-token-here",
	}

	sanitized := SanitizeMetadata(meta)

	if sanitized["client_id"] != "my-client" {
		t.Errorf("expected client_id to be preserved, got %q", sanitized["client_id"])
	}
	if sanitized["request_id"] != "req-123" {
		t.Errorf("expected request_id to be preserved, got %q", sanitized["request_id"])
	}
	if sanitized["auth_token"] != "[REDACTED]" {
		t.Errorf("expected auth_token to be [REDACTED], got %q", sanitized["auth_token"])
	}
	if sanitized["password"] != "[REDACTED]" {
		t.Errorf("expected password to be [REDACTED], got %q", sanitized["password"])
	}
	if sanitized["Authorization"] != "[REDACTED]" {
		t.Errorf("expected Authorization to be [REDACTED], got %q", sanitized["Authorization"])
	}
	if sanitized["tls_key"] != "[REDACTED]" {
		t.Errorf("expected tls_key to be [REDACTED], got %q", sanitized["tls_key"])
	}
	if sanitized["raw_auth_header"] != "[REDACTED]" {
		t.Errorf("expected raw_auth_header to be [REDACTED], got %q", sanitized["raw_auth_header"])
	}
}

func TestSanitizer_SanitizeEvent(t *testing.T) {
	evt := model.AuditEvent{
		Actor: model.AuditActor{
			Type:     model.ActorTypeAuthenticatedClient,
			Identity: "Bearer secret-token-attempt", // Token in identity must be sanitized
		},
		Message: "Authentication failed with token: Bearer secret-123",
		Metadata: map[string]string{
			"token": "secret-val",
		},
	}

	sanitized := SanitizeEvent(evt)

	if sanitized.ID == "" {
		t.Errorf("expected generated event ID")
	}
	if sanitized.Timestamp.IsZero() {
		t.Errorf("expected populated timestamp")
	}
	if sanitized.Timestamp.Location() != time.UTC {
		t.Errorf("expected UTC timestamp")
	}
	if sanitized.Actor.Identity != "[REDACTED]" {
		t.Errorf("expected actor identity to be [REDACTED], got %q", sanitized.Actor.Identity)
	}
	if sanitized.Metadata["token"] != "[REDACTED]" {
		t.Errorf("expected metadata token to be [REDACTED], got %q", sanitized.Metadata["token"])
	}
	if strings.Contains(sanitized.Message, "secret-123") {
		t.Errorf("message should not contain raw token: %q", sanitized.Message)
	}
}

func TestGenerateEventID(t *testing.T) {
	id1 := GenerateEventID()
	id2 := GenerateEventID()

	if id1 == "" || id2 == "" {
		t.Fatalf("generated empty ID")
	}
	if id1 == id2 {
		t.Fatalf("generated identical IDs: %s", id1)
	}
	// UUID v4 length is 36 chars (8-4-4-4-12)
	if len(id1) != 36 {
		t.Errorf("expected 36-character UUID, got len %d (%s)", len(id1), id1)
	}
}

func TestSanitizer_MaskToken(t *testing.T) {
	tok1 := "my-secret-token-12345"
	m1 := MaskToken(tok1)
	m2 := MaskToken(tok1)
	if m1 == "" {
		t.Fatalf("expected non-empty masked token")
	}
	if m1 != m2 {
		t.Errorf("expected deterministic masked token, got %s and %s", m1, m2)
	}
	if !strings.HasPrefix(m1, "token:sha256:") {
		t.Errorf("expected token:sha256: prefix, got %s", m1)
	}
	if strings.Contains(m1, tok1) {
		t.Errorf("masked token must not contain raw token: %s", m1)
	}

	// Bearer prefix should be stripped before hashing
	mBearer := MaskToken("Bearer " + tok1)
	if mBearer != m1 {
		t.Errorf("MaskToken with Bearer prefix should match raw token mask: got %s, expected %s", mBearer, m1)
	}

	if MaskToken("") != "" {
		t.Errorf("MaskToken on empty string should be empty")
	}
}
