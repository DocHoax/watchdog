package audit

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/DocHoax/watchdog/internal/config"
	"github.com/DocHoax/watchdog/internal/logger"
	"github.com/DocHoax/watchdog/pkg/model"
)

// AuditStorer defines the persistence interface required for saving audit events.
type AuditStorer interface {
	SaveAuditEvent(ctx context.Context, event model.AuditEvent) error
}

// AuditLogger records security-relevant events across Watchdog subsystems.
type AuditLogger interface {
	Record(ctx context.Context, event model.AuditEvent) error
	Close() error
}

// Logger implements AuditLogger by recording to standard structured logs and SQLite storage.
type Logger struct {
	mu           sync.RWMutex
	cfg          config.AuditConfig
	storer       AuditStorer
	floodLimiter *FloodLimiter
	log          *logger.Logger
}

// New creates a new AuditLogger with the given configuration, storage sink, and logger.
func New(cfg config.AuditConfig, storer AuditStorer, l *logger.Logger) *Logger {
	if l == nil {
		l = logger.GetDefault()
	}
	return &Logger{
		cfg:          cfg,
		storer:       storer,
		floodLimiter: NewFloodLimiter(50, 10*time.Second),
		log:          l,
	}
}

// Record sanitizes, logs, and persists an AuditEvent.
func (a *Logger) Record(ctx context.Context, event model.AuditEvent) error {
	a.mu.RLock()
	enabled := a.cfg.Enabled
	a.mu.RUnlock()

	if !enabled {
		return nil
	}

	sanitized := SanitizeEvent(event)

	// Check rate limiting for failure/denied events to prevent storage exhaustion
	if sanitized.Outcome == model.AuditOutcomeFailure || sanitized.Outcome == model.AuditOutcomeDenied {
		key := fmt.Sprintf("%s:%s", sanitized.Source.Address, sanitized.EventType)
		allowed, suppressed := a.floodLimiter.Allow(key)
		if !allowed {
			if suppressed == 1 || suppressed%50 == 0 {
				a.log.Warn("Audit failure event rate limit exceeded; throttling storage writes",
					"audit", true,
					"source_ip", sanitized.Source.Address,
					"event_type", sanitized.EventType,
					"suppressed_count", suppressed,
				)
			}
			// Emit log entry but skip database write to protect SQLite
			a.emitLog(sanitized)
			return nil
		}
	}

	// 1. Emit structured operational log
	a.emitLog(sanitized)

	// 2. Persist to storage engine if configured
	if a.storer != nil {
		if err := a.storer.SaveAuditEvent(ctx, sanitized); err != nil {
			a.log.Warn("Failed to persist audit event to storage engine",
				"audit", true,
				"event_id", sanitized.ID,
				"event_type", sanitized.EventType,
				"error", err.Error(),
			)
			return fmt.Errorf("failed to save audit event: %w", err)
		}
	}

	return nil
}

// emitLog outputs structured log record matching event severity.
func (a *Logger) emitLog(event model.AuditEvent) {
	keyvals := []any{
		"audit", true,
		"event_id", event.ID,
		"event_type", event.EventType,
		"severity", event.Severity,
		"outcome", event.Outcome,
		"actor_type", event.Actor.Type,
	}
	if event.Actor.Identity != "" {
		keyvals = append(keyvals, "actor_identity", event.Actor.Identity)
	}
	if event.Source.Address != "" {
		keyvals = append(keyvals, "source_ip", event.Source.Address)
	}
	if event.Source.Endpoint != "" {
		keyvals = append(keyvals, "endpoint", event.Source.Endpoint)
	}
	if event.Source.Method != "" {
		keyvals = append(keyvals, "method", event.Source.Method)
	}
	if event.Source.RequestID != "" {
		keyvals = append(keyvals, "request_id", event.Source.RequestID)
	}

	msg := event.Message
	if msg == "" {
		msg = fmt.Sprintf("Audit event: %s", event.EventType)
	}

	switch event.Severity {
	case model.AuditSeverityCritical, model.AuditSeverityError:
		a.log.Error(msg, keyvals...)
	case model.AuditSeverityWarning:
		a.log.Warn(msg, keyvals...)
	default:
		a.log.Info(msg, keyvals...)
	}
}

// Close closes the audit logger.
func (a *Logger) Close() error {
	return nil
}

// NopAuditLogger is a no-op implementation of AuditLogger.
type NopAuditLogger struct{}

// NewNopAuditLogger creates a no-op AuditLogger.
func NewNopAuditLogger() *NopAuditLogger {
	return &NopAuditLogger{}
}

// Record does nothing.
func (n *NopAuditLogger) Record(ctx context.Context, event model.AuditEvent) error {
	return nil
}

// Close does nothing.
func (n *NopAuditLogger) Close() error {
	return nil
}
