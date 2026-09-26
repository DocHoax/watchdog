package audit

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DocHoax/watchdog/internal/config"
	"github.com/DocHoax/watchdog/internal/logger"
	"github.com/DocHoax/watchdog/pkg/model"
)

type mockAuditStorer struct {
	events    []model.AuditEvent
	returnErr error
}

func (m *mockAuditStorer) SaveAuditEvent(ctx context.Context, event model.AuditEvent) error {
	if m.returnErr != nil {
		return m.returnErr
	}
	m.events = append(m.events, event)
	return nil
}

func TestAuditLogger_Record_Success(t *testing.T) {
	buf := new(bytes.Buffer)
	log := logger.New(buf, logger.LevelInfo, false, false)
	storer := &mockAuditStorer{}

	cfg := config.AuditConfig{
		Enabled:       true,
		RetentionDays: 90,
		MaxQueryLimit: 1000,
	}

	auditLog := New(cfg, storer, log)

	evt := model.AuditEvent{
		EventType: model.EventAuthSuccess,
		Severity:  model.AuditSeverityInfo,
		Outcome:   model.AuditOutcomeSuccess,
		Actor: model.AuditActor{
			Type:     model.ActorTypeAuthenticatedClient,
			Identity: "api-client",
		},
		Source: model.AuditSource{
			Address:   "192.168.1.100",
			Endpoint:  "/api/v1/snapshot",
			Method:    "GET",
			RequestID: "req-12345",
		},
		Message: "Authentication succeeded",
	}

	err := auditLog.Record(context.Background(), evt)
	if err != nil {
		t.Fatalf("unexpected record error: %v", err)
	}

	if len(storer.events) != 1 {
		t.Fatalf("expected 1 event in storage, got %d", len(storer.events))
	}

	saved := storer.events[0]
	if saved.EventType != model.EventAuthSuccess {
		t.Errorf("expected EventType %q, got %q", model.EventAuthSuccess, saved.EventType)
	}
	if saved.ID == "" {
		t.Errorf("expected generated ID in saved event")
	}

	logOut := buf.String()
	if !strings.Contains(logOut, "auth.success") {
		t.Errorf("expected log output to contain auth.success, got: %s", logOut)
	}
	if !strings.Contains(logOut, "audit=true") {
		t.Errorf("expected log output to contain audit=true, got: %s", logOut)
	}
}

func TestAuditLogger_Disabled(t *testing.T) {
	buf := new(bytes.Buffer)
	log := logger.New(buf, logger.LevelInfo, false, false)
	storer := &mockAuditStorer{}

	cfg := config.AuditConfig{
		Enabled: false,
	}

	auditLog := New(cfg, storer, log)
	evt := model.AuditEvent{EventType: model.EventAuthSuccess}
	if err := auditLog.Record(context.Background(), evt); err != nil {
		t.Fatalf("unexpected error when disabled: %v", err)
	}

	if len(storer.events) != 0 {
		t.Errorf("expected 0 events saved when disabled, got %d", len(storer.events))
	}
	if buf.Len() != 0 {
		t.Errorf("expected no log output when disabled, got: %s", buf.String())
	}
}

func TestAuditLogger_StorageError_Graceful(t *testing.T) {
	buf := new(bytes.Buffer)
	log := logger.New(buf, logger.LevelInfo, false, false)
	storer := &mockAuditStorer{
		returnErr: errors.New("sqlite disk I/O error"),
	}

	cfg := config.AuditConfig{
		Enabled: true,
	}

	auditLog := New(cfg, storer, log)
	evt := model.AuditEvent{
		EventType: model.EventAuthSuccess,
		Severity:  model.AuditSeverityInfo,
	}

	err := auditLog.Record(context.Background(), evt)
	if err == nil {
		t.Fatalf("expected error from Record when storage fails")
	}

	logOut := buf.String()
	if !strings.Contains(logOut, "Failed to persist audit event") {
		t.Errorf("expected warning in log output regarding storage failure, got: %s", logOut)
	}
}

func TestFloodLimiter(t *testing.T) {
	fl := NewFloodLimiter(3, 100*time.Millisecond)
	key := "127.0.0.1:auth.failure"

	// First 3 should be allowed
	for i := 1; i <= 3; i++ {
		allowed, supp := fl.Allow(key)
		if !allowed || supp != 0 {
			t.Errorf("event %d should be allowed, got allowed=%v, supp=%d", i, allowed, supp)
		}
	}

	// 4th and 5th should be throttled
	allowed, supp := fl.Allow(key)
	if allowed || supp != 1 {
		t.Errorf("event 4 should be throttled, got allowed=%v, supp=%d", allowed, supp)
	}

	allowed, supp = fl.Allow(key)
	if allowed || supp != 2 {
		t.Errorf("event 5 should be throttled, got allowed=%v, supp=%d", allowed, supp)
	}

	// Wait for window to expire
	time.Sleep(150 * time.Millisecond)

	// Should be allowed again
	allowed, supp = fl.Allow(key)
	if !allowed || supp != 0 {
		t.Errorf("event after window should be allowed, got allowed=%v, supp=%d", allowed, supp)
	}
}
