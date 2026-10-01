package governance

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"time"

	"github.com/DocHoax/watchdog/internal/storage"
	"github.com/DocHoax/watchdog/pkg/model"
)

// RedactedPlaceholder is the standard string used to replace sensitive credentials in audit records.
const RedactedPlaceholder = "[REDACTED]"

var (
	sensitiveKeyPattern = regexp.MustCompile(`(?i)(token|secret|password|passwd|pwd|api_?key|auth|credential|private_?key|bearer|cert|access_?key)`)
	bearerPattern       = regexp.MustCompile(`(?i)\b(bearer\s+)[a-zA-Z0-9_\-\.]{10,}`)
	apiKeyPattern       = regexp.MustCompile(`(?i)\b(api_?key[\s:=]+)[a-zA-Z0-9_\-\.]{10,}`)
	passwordPattern     = regexp.MustCompile(`(?i)\b(password[\s:=]+)[^\s,;&]+`)
	secretPattern       = regexp.MustCompile(`(?i)\b(secret[\s:=]+)[^\s,;&]+`)
)

// AuditRecorder provides structured, security-conscious recording of governance audit events.
type AuditRecorder struct {
	store storage.Storage
	clock Clock
}

// NewAuditRecorder constructs an AuditRecorder with storage and clock.
func NewAuditRecorder(store storage.Storage, clock Clock) *AuditRecorder {
	if clock == nil {
		clock = RealClock{}
	}
	return &AuditRecorder{
		store: store,
		clock: clock,
	}
}

// Record sanitizes and persists a single audit event.
func (a *AuditRecorder) Record(ctx context.Context, event model.AuditEvent) error {
	if a.store == nil {
		return nil
	}

	sanitized := SanitizeAuditEvent(event)
	if sanitized.ID == "" {
		sanitized.ID = GenerateAuditID()
	}
	if sanitized.Timestamp.IsZero() {
		sanitized.Timestamp = a.clock.Now().UTC()
	} else {
		sanitized.Timestamp = sanitized.Timestamp.UTC()
	}

	return a.store.SaveAuditEvent(ctx, sanitized)
}

// RecordEvent constructs, sanitizes, and persists an audit event.
func (a *AuditRecorder) RecordEvent(
	ctx context.Context,
	eventType string,
	severity string,
	outcome string,
	resource string,
	action string,
	message string,
	actor model.AuditActor,
	meta map[string]string,
) error {
	event := model.AuditEvent{
		ID:        GenerateAuditID(),
		Timestamp: a.clock.Now().UTC(),
		EventType: eventType,
		Severity:  severity,
		Outcome:   outcome,
		Actor:     actor,
		Resource:  resource,
		Action:    action,
		Message:   message,
		Metadata:  meta,
	}
	return a.Record(ctx, event)
}

// SanitizeAuditEvent scrubs sensitive secrets, tokens, and credentials from all audit event fields.
func SanitizeAuditEvent(event model.AuditEvent) model.AuditEvent {
	sanitized := event
	sanitized.Actor.Identity = SanitizeString(event.Actor.Identity)
	sanitized.Message = SanitizeString(event.Message)
	sanitized.Resource = SanitizeString(event.Resource)
	sanitized.Action = SanitizeString(event.Action)
	sanitized.Metadata = SanitizeMetadata(event.Metadata)
	return sanitized
}

// SanitizeMetadata scrubs sensitive keys and values from a metadata map.
func SanitizeMetadata(meta map[string]string) map[string]string {
	if meta == nil {
		return nil
	}
	cleaned := make(map[string]string, len(meta))
	for k, v := range meta {
		if sensitiveKeyPattern.MatchString(k) {
			cleaned[k] = RedactedPlaceholder
		} else {
			cleaned[k] = SanitizeString(v)
		}
	}
	return cleaned
}

// SanitizeString scrubs bearer tokens, password strings, and api keys from free-form text.
func SanitizeString(s string) string {
	if s == "" {
		return s
	}
	s = bearerPattern.ReplaceAllString(s, "${1}"+RedactedPlaceholder)
	s = apiKeyPattern.ReplaceAllString(s, "${1}"+RedactedPlaceholder)
	s = passwordPattern.ReplaceAllString(s, "${1}"+RedactedPlaceholder)
	s = secretPattern.ReplaceAllString(s, "${1}"+RedactedPlaceholder)
	return s
}

// GenerateAuditID generates a secure random identifier for audit records.
func GenerateAuditID() string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("audit-%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("audit-%s", hex.EncodeToString(b[:]))
}
