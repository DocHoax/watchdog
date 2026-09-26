package audit

import (
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/DocHoax/watchdog/pkg/model"
)

// SensitiveKeySubstrings contains lowercase substrings that indicate a sensitive metadata field.
var SensitiveKeySubstrings = []string{
	"token",
	"password",
	"secret",
	"authorization",
	"auth_header",
	"private_key",
	"tls_key",
	"cookie",
	"credential",
	"api_key",
	"apikey",
	"dsn",
	"database_url",
	"cert_key",
	"bearer",
	"passphrase",
}

var (
	bearerPattern = regexp.MustCompile(`(?i)\b(bearer|basic)\s+[A-Za-z0-9._~+/-]+=*`)
)

// GenerateEventID creates a cryptographically random RFC 4122 v4 UUID string.
func GenerateEventID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// Fallback to timestamp-based pseudo-random if crypto/rand fails
		return fmt.Sprintf("evt-%d", time.Now().UnixNano())
	}
	b[6] = (b[6] & 0x0f) | 0x40 // Version 4
	b[8] = (b[8] & 0x3f) | 0x80 // Variant RFC 4122
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// MaskToken generates a non-reversible SHA-256 truncated identifier (e.g. token:sha256:a1b2c3d4)
// to enable safe correlation of token usage in audit logs without exposing raw tokens.
func MaskToken(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	// Strip Bearer prefix if present
	if strings.HasPrefix(strings.ToLower(raw), "bearer ") {
		raw = strings.TrimSpace(raw[7:])
	}
	if raw == "" {
		return ""
	}
	hash := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("token:sha256:%x", hash[:4])
}

// IsSensitiveKey returns true if the key name indicates sensitive credential content.
func IsSensitiveKey(key string) bool {
	lower := strings.ToLower(strings.TrimSpace(key))
	for _, sub := range SensitiveKeySubstrings {
		if strings.Contains(lower, sub) {
			return true
		}
	}
	return false
}

// SanitizeValue checks if a string value appears to contain raw credentials or sensitive data.
func SanitizeValue(val string) string {
	trimmed := strings.TrimSpace(val)
	lower := strings.ToLower(trimmed)

	// Check if value starts with Bearer or Basic authentication prefixes
	if strings.HasPrefix(lower, "bearer ") || strings.HasPrefix(lower, "basic ") {
		return "[REDACTED]"
	}

	// Check for PEM private key headers
	if strings.Contains(lower, "private key") || strings.Contains(lower, "begin rsa") || strings.Contains(lower, "begin ec") {
		return "[REDACTED]"
	}

	// Check for potential JWT tokens (three dot-separated base64 segments starting with eyJ)
	if strings.HasPrefix(trimmed, "eyJ") && strings.Count(trimmed, ".") == 2 {
		return "[REDACTED]"
	}

	// Redact embedded bearer/basic authorization patterns
	if bearerPattern.MatchString(val) {
		val = bearerPattern.ReplaceAllString(val, "$1 [REDACTED]")
	}

	return val
}

// SanitizeMetadata produces a deep sanitized copy of metadata key-values.
func SanitizeMetadata(meta map[string]string) map[string]string {
	if meta == nil {
		return nil
	}

	sanitized := make(map[string]string, len(meta))
	for k, v := range meta {
		if IsSensitiveKey(k) {
			sanitized[k] = "[REDACTED]"
		} else {
			sanitized[k] = SanitizeValue(v)
		}
	}
	return sanitized
}

// SanitizeEvent validates and sanitizes all fields of an AuditEvent before recording or storage.
func SanitizeEvent(event model.AuditEvent) model.AuditEvent {
	sanitized := event

	if sanitized.ID == "" {
		sanitized.ID = GenerateEventID()
	}

	if sanitized.Timestamp.IsZero() {
		sanitized.Timestamp = time.Now().UTC()
	} else {
		sanitized.Timestamp = sanitized.Timestamp.UTC()
	}

	// Ensure Actor Identity never contains tokens, keys, or bearer tokens
	if IsSensitiveKey(sanitized.Actor.Identity) || strings.HasPrefix(strings.ToLower(sanitized.Actor.Identity), "bearer ") {
		sanitized.Actor.Identity = "[REDACTED]"
	} else {
		sanitized.Actor.Identity = SanitizeValue(sanitized.Actor.Identity)
	}

	// Sanitize Metadata
	sanitized.Metadata = SanitizeMetadata(sanitized.Metadata)

	// Sanitize Message
	sanitized.Message = SanitizeValue(sanitized.Message)

	return sanitized
}
