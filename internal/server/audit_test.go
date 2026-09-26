package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/DocHoax/watchdog/internal/config"
	"github.com/DocHoax/watchdog/internal/storage"
	"github.com/DocHoax/watchdog/pkg/model"
)

// mockAuditLogger captures recorded audit events in memory for test verification.
type mockAuditLogger struct {
	mu     sync.Mutex
	events []model.AuditEvent
}

func newMockAuditLogger() *mockAuditLogger {
	return &mockAuditLogger{events: make([]model.AuditEvent, 0)}
}

func (m *mockAuditLogger) Record(ctx context.Context, event model.AuditEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, event)
	return nil
}

func (m *mockAuditLogger) Close() error {
	return nil
}

func (m *mockAuditLogger) getEvents() []model.AuditEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	copied := make([]model.AuditEvent, len(m.events))
	copy(copied, m.events)
	return copied
}

func TestRequestIDMiddleware(t *testing.T) {
	// Case 1: Request with existing valid X-Request-ID
	customReqID := "test-req-id-12345"
	req := httptest.NewRequest("GET", "/health", nil)
	req.Header.Set("X-Request-ID", customReqID)
	rec := httptest.NewRecorder()

	var capturedCtxID string
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCtxID = GetRequestID(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := RequestIDMiddleware(testHandler)
	handler.ServeHTTP(rec, req)

	if capturedCtxID != customReqID {
		t.Errorf("expected context request ID %q, got %q", customReqID, capturedCtxID)
	}
	if rec.Header().Get("X-Request-ID") != customReqID {
		t.Errorf("expected response header X-Request-ID %q, got %q", customReqID, rec.Header().Get("X-Request-ID"))
	}

	// Case 2: Request without X-Request-ID -> auto-generated
	req2 := httptest.NewRequest("GET", "/health", nil)
	rec2 := httptest.NewRecorder()
	capturedCtxID = ""

	handler.ServeHTTP(rec2, req2)

	if capturedCtxID == "" {
		t.Fatal("expected auto-generated request ID in context, got empty string")
	}
	if rec2.Header().Get("X-Request-ID") != capturedCtxID {
		t.Errorf("expected response header to match context ID %q, got %q", capturedCtxID, rec2.Header().Get("X-Request-ID"))
	}
}

func TestAuthMiddleware_Auditing(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Agent.Token = "secret-secure-token-999"
	cfg.Audit.Enabled = true

	store, err := storage.NewSQLiteStorage(storage.Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("failed to create sqlite store: %v", err)
	}
	defer store.Close()

	mockAudit := newMockAuditLogger()
	srv := NewServer(cfg, nil, store, nil, nil, nil)
	srv.SetAuditLogger(mockAudit)

	protectedHandler := srv.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))

	handler := RequestIDMiddleware(protectedHandler)

	// 1. Missing credentials
	req1 := httptest.NewRequest("GET", "/api/v1/snapshot", nil)
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401 for missing credentials, got %d", rec1.Code)
	}

	events := mockAudit.getEvents()
	if len(events) != 1 {
		t.Fatalf("expected 1 audit event recorded, got %d", len(events))
	}
	if events[0].EventType != model.EventAuthMissingCredentials {
		t.Errorf("expected event type %s, got %s", model.EventAuthMissingCredentials, events[0].EventType)
	}
	if events[0].Outcome != model.AuditOutcomeDenied {
		t.Errorf("expected outcome denied, got %s", events[0].Outcome)
	}
	if events[0].Actor.Type != model.ActorTypeAnonymousClient {
		t.Errorf("expected actor type %s, got %s", model.ActorTypeAnonymousClient, events[0].Actor.Type)
	}
	if events[0].Source.RequestID == "" {
		t.Error("expected non-empty request ID in audit source")
	}

	// 2. Invalid credentials
	rawBadToken := "Bearer completely-wrong-token-abc123"
	req2 := httptest.NewRequest("GET", "/api/v1/snapshot", nil)
	req2.Header.Set("Authorization", rawBadToken)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401 for invalid credentials, got %d", rec2.Code)
	}

	events = mockAudit.getEvents()
	if len(events) != 2 {
		t.Fatalf("expected 2 audit events recorded, got %d", len(events))
	}
	invalidEvt := events[1]
	if invalidEvt.EventType != model.EventAuthInvalidCredentials {
		t.Errorf("expected event type %s, got %s", model.EventAuthInvalidCredentials, invalidEvt.EventType)
	}
	if invalidEvt.Outcome != model.AuditOutcomeDenied {
		t.Errorf("expected outcome denied, got %s", invalidEvt.Outcome)
	}
	// Verify raw bad token is NOT stored anywhere in the event
	if strings.Contains(invalidEvt.Message, "completely-wrong-token") ||
		strings.Contains(invalidEvt.Actor.Identity, "completely-wrong-token") {
		t.Errorf("raw token leaked in audit event: %+v", invalidEvt)
	}
	if !strings.HasPrefix(invalidEvt.Actor.Identity, "token:sha256:") {
		t.Errorf("expected masked actor identity, got %s", invalidEvt.Actor.Identity)
	}

	// 3. Valid credentials
	req3 := httptest.NewRequest("GET", "/api/v1/snapshot", nil)
	req3.Header.Set("Authorization", "Bearer secret-secure-token-999")
	rec3 := httptest.NewRecorder()
	handler.ServeHTTP(rec3, req3)

	if rec3.Code != http.StatusOK {
		t.Errorf("expected status 200 for valid credentials, got %d", rec3.Code)
	}

	events = mockAudit.getEvents()
	if len(events) != 3 {
		t.Fatalf("expected 3 audit events recorded, got %d", len(events))
	}
	successEvt := events[2]
	if successEvt.EventType != model.EventAuthSuccess {
		t.Errorf("expected event type %s, got %s", model.EventAuthSuccess, successEvt.EventType)
	}
	if successEvt.Outcome != model.AuditOutcomeSuccess {
		t.Errorf("expected outcome success, got %s", successEvt.Outcome)
	}
	if successEvt.Actor.Type != model.ActorTypeAuthenticatedClient {
		t.Errorf("expected actor type %s, got %s", model.ActorTypeAuthenticatedClient, successEvt.Actor.Type)
	}
}

func TestHandleAuditEvents_API(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Agent.Token = "test-token"
	cfg.Audit.Enabled = true
	cfg.Audit.MaxQueryLimit = 500

	store, err := storage.NewSQLiteStorage(storage.Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("failed to create sqlite store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	now := time.Now().UTC()

	// Seed audit events in storage
	evt1 := model.AuditEvent{
		ID:        "evt-api-001",
		Timestamp: now.Add(-30 * time.Minute),
		EventType: model.EventAuthSuccess,
		Severity:  model.AuditSeverityInfo,
		Outcome:   model.AuditOutcomeSuccess,
		Actor: model.AuditActor{
			Type:     model.ActorTypeAuthenticatedClient,
			Identity: "api-client",
		},
		Source: model.AuditSource{
			Address:   "192.168.1.10",
			Endpoint:  "/api/v1/snapshot",
			Method:    "GET",
			RequestID: "req-1",
		},
		Message: "Auth ok",
	}

	evt2 := model.AuditEvent{
		ID:        "evt-api-002",
		Timestamp: now.Add(-10 * time.Minute),
		EventType: model.EventAuthFailure,
		Severity:  model.AuditSeverityWarning,
		Outcome:   model.AuditOutcomeDenied,
		Actor: model.AuditActor{
			Type:     model.ActorTypeAnonymousClient,
			Identity: "token:sha256:1234abcd",
		},
		Source: model.AuditSource{
			Address:   "10.0.0.5",
			Endpoint:  "/api/v1/snapshot",
			Method:    "GET",
			RequestID: "req-2",
		},
		Message: "Invalid token",
	}

	if err := store.SaveAuditEvents(ctx, []model.AuditEvent{evt1, evt2}); err != nil {
		t.Fatalf("failed to seed audit events: %v", err)
	}

	srv := NewServer(cfg, nil, store, nil, nil, nil)

	mux := http.NewServeMux()
	mux.Handle("/api/v1/audit/events", srv.authMiddleware(http.HandlerFunc(srv.handleAuditEvents)))
	handler := RequestIDMiddleware(mux)

	// 1. Unauthenticated request -> 401
	unauthReq := httptest.NewRequest("GET", "/api/v1/audit/events", nil)
	unauthRec := httptest.NewRecorder()
	handler.ServeHTTP(unauthRec, unauthReq)
	if unauthRec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 unauthenticated, got %d", unauthRec.Code)
	}

	// 2. Authenticated request -> 200 with all events
	authReq := httptest.NewRequest("GET", "/api/v1/audit/events", nil)
	authReq.Header.Set("Authorization", "Bearer test-token")
	authRec := httptest.NewRecorder()
	handler.ServeHTTP(authRec, authReq)

	if authRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d; body: %s", authRec.Code, authRec.Body.String())
	}

	var resp struct {
		Events []model.AuditEvent `json:"events"`
		Count  int                `json:"count"`
		Total  int64              `json:"total"`
		Limit  int                `json:"limit"`
		Offset int                `json:"offset"`
	}
	if err := json.Unmarshal(authRec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode json response: %v", err)
	}

	// Note: authMiddleware records auth.success into the logger, but if logger is wired to store, it may also appear
	if resp.Total < 2 {
		t.Errorf("expected at least 2 events, got %d", resp.Total)
	}

	// 3. Filter by EventType
	filterReq := httptest.NewRequest("GET", "/api/v1/audit/events?event_type=auth.failure", nil)
	filterReq.Header.Set("Authorization", "Bearer test-token")
	filterRec := httptest.NewRecorder()
	handler.ServeHTTP(filterRec, filterReq)

	if filterRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", filterRec.Code)
	}

	var filterResp struct {
		Events []model.AuditEvent `json:"events"`
		Count  int                `json:"count"`
		Total  int64              `json:"total"`
	}
	if err := json.Unmarshal(filterRec.Body.Bytes(), &filterResp); err != nil {
		t.Fatalf("failed to decode json response: %v", err)
	}
	if filterResp.Count != 1 || filterResp.Events[0].ID != "evt-api-002" {
		t.Errorf("expected 1 event with id evt-api-002, got count=%d, events=%v", filterResp.Count, filterResp.Events)
	}

	// 4. Method not allowed (POST)
	postReq := httptest.NewRequest("POST", "/api/v1/audit/events", nil)
	postReq.Header.Set("Authorization", "Bearer test-token")
	postRec := httptest.NewRecorder()
	handler.ServeHTTP(postRec, postReq)
	if postRec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 Method Not Allowed, got %d", postRec.Code)
	}
}
