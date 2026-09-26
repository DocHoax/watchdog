package storage

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/DocHoax/watchdog/pkg/model"
)

func TestSQLiteStorage_AuditEvents_CRUD(t *testing.T) {
	store, err := NewSQLiteStorage(Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("failed to create sqlite storage: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	now := time.Now().UTC()

	event1 := model.AuditEvent{
		ID:        "evt-001",
		Timestamp: now.Add(-10 * time.Minute),
		EventType: model.EventAuthSuccess,
		Severity:  model.AuditSeverityInfo,
		Outcome:   model.AuditOutcomeSuccess,
		Actor: model.AuditActor{
			Type:     model.ActorTypeAuthenticatedClient,
			Identity: "api-client",
		},
		Source: model.AuditSource{
			Address:   "192.168.1.50",
			Endpoint:  "/api/v1/snapshot",
			Method:    "GET",
			RequestID: "req-abc-1",
		},
		Message: "Authentication succeeded",
		Metadata: map[string]string{
			"env": "production",
		},
	}

	event2 := model.AuditEvent{
		ID:        "evt-002",
		Timestamp: now.Add(-5 * time.Minute),
		EventType: model.EventAuthFailure,
		Severity:  model.AuditSeverityWarning,
		Outcome:   model.AuditOutcomeDenied,
		Actor: model.AuditActor{
			Type:     model.ActorTypeAnonymousClient,
			Identity: "unknown",
		},
		Source: model.AuditSource{
			Address:   "10.0.0.99",
			Endpoint:  "/api/v1/config",
			Method:    "POST",
			RequestID: "req-abc-2",
		},
		Message: "Missing credentials",
	}

	event3 := model.AuditEvent{
		ID:        "evt-003",
		Timestamp: now,
		EventType: model.EventServerStart,
		Severity:  model.AuditSeverityInfo,
		Outcome:   model.AuditOutcomeSuccess,
		Actor: model.AuditActor{
			Type:     model.ActorTypeSystem,
			Identity: "watchdog-daemon",
		},
		Source: model.AuditSource{
			Address: "127.0.0.1",
		},
		Message: "Watchdog server started",
	}

	// 1. Test single save
	if err := store.SaveAuditEvent(ctx, event1); err != nil {
		t.Fatalf("failed to save single audit event: %v", err)
	}

	// 2. Test batch save
	if err := store.SaveAuditEvents(ctx, []model.AuditEvent{event2, event3}); err != nil {
		t.Fatalf("failed to save batch audit events: %v", err)
	}

	// 3. Test Count without filter
	total, err := store.CountAuditEvents(ctx, AuditFilter{})
	if err != nil {
		t.Fatalf("failed to count audit events: %v", err)
	}
	if total != 3 {
		t.Fatalf("expected 3 total events, got %d", total)
	}

	// 4. Test Query with EventType filter
	events, err := store.QueryAuditEvents(ctx, AuditFilter{EventType: model.EventAuthSuccess})
	if err != nil {
		t.Fatalf("failed to query by event_type: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event for auth.success, got %d", len(events))
	}
	if events[0].ID != "evt-001" {
		t.Errorf("expected evt-001, got %s", events[0].ID)
	}
	if events[0].Metadata["env"] != "production" {
		t.Errorf("expected metadata env=production, got %v", events[0].Metadata)
	}

	// 5. Test Query with Outcome filter
	deniedEvents, err := store.QueryAuditEvents(ctx, AuditFilter{Outcome: model.AuditOutcomeDenied})
	if err != nil {
		t.Fatalf("failed to query by outcome: %v", err)
	}
	if len(deniedEvents) != 1 || deniedEvents[0].ID != "evt-002" {
		t.Fatalf("expected 1 denied event with id evt-002, got %v", deniedEvents)
	}

	// 6. Test Query with Time range
	recentEvents, err := store.QueryAuditEvents(ctx, AuditFilter{
		StartTime: now.Add(-6 * time.Minute),
		EndTime:   now.Add(1 * time.Minute),
	})
	if err != nil {
		t.Fatalf("failed to query by time range: %v", err)
	}
	if len(recentEvents) != 2 {
		t.Fatalf("expected 2 recent events, got %d", len(recentEvents))
	}

	// 7. Test Pagination (Limit & Offset)
	pagedEvents, err := store.QueryAuditEvents(ctx, AuditFilter{
		Limit:  1,
		Offset: 1,
	})
	if err != nil {
		t.Fatalf("failed to query paged events: %v", err)
	}
	if len(pagedEvents) != 1 {
		t.Fatalf("expected 1 event for paged query, got %d", len(pagedEvents))
	}
	// Ordered by timestamp DESC: evt-003, evt-002, evt-001. Offset 1 should be evt-002.
	if pagedEvents[0].ID != "evt-002" {
		t.Errorf("expected offset 1 to return evt-002, got %s", pagedEvents[0].ID)
	}
}

func TestSQLiteStorage_AuditEvents_PruneAndPurge(t *testing.T) {
	store, err := NewSQLiteStorage(Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("failed to create sqlite storage: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	now := time.Now().UTC()

	// Old event: 100 days ago
	oldEvt := model.AuditEvent{
		ID:        "evt-old",
		Timestamp: now.Add(-100 * 24 * time.Hour),
		EventType: model.EventAuthSuccess,
		Severity:  model.AuditSeverityInfo,
		Outcome:   model.AuditOutcomeSuccess,
		Message:   "Old event",
	}

	// Mid event: 45 days ago
	midEvt := model.AuditEvent{
		ID:        "evt-mid",
		Timestamp: now.Add(-45 * 24 * time.Hour),
		EventType: model.EventAuthSuccess,
		Severity:  model.AuditSeverityInfo,
		Outcome:   model.AuditOutcomeSuccess,
		Message:   "Mid event",
	}

	// Recent event: 1 hour ago
	recentEvt := model.AuditEvent{
		ID:        "evt-recent",
		Timestamp: now.Add(-1 * time.Hour),
		EventType: model.EventAuthSuccess,
		Severity:  model.AuditSeverityInfo,
		Outcome:   model.AuditOutcomeSuccess,
		Message:   "Recent event",
	}

	if err := store.SaveAuditEvents(ctx, []model.AuditEvent{oldEvt, midEvt, recentEvt}); err != nil {
		t.Fatalf("failed to save events: %v", err)
	}

	// Prune older than 90 days (should delete oldEvt)
	pruned, err := store.PruneAuditEvents(ctx, 90*24*time.Hour)
	if err != nil {
		t.Fatalf("failed to prune audit events: %v", err)
	}
	if pruned != 1 {
		t.Errorf("expected 1 pruned event, got %d", pruned)
	}

	count, err := store.CountAuditEvents(ctx, AuditFilter{})
	if err != nil {
		t.Fatalf("failed to count: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 remaining events, got %d", count)
	}

	// Purge before 30 days ago (should delete midEvt)
	purged, err := store.PurgeAuditEvents(ctx, now.Add(-30*24*time.Hour))
	if err != nil {
		t.Fatalf("failed to purge audit events: %v", err)
	}
	if purged != 1 {
		t.Errorf("expected 1 purged event, got %d", purged)
	}

	count, err = store.CountAuditEvents(ctx, AuditFilter{})
	if err != nil {
		t.Fatalf("failed to count: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 remaining event (evt-recent), got %d", count)
	}

	remaining, err := store.QueryAuditEvents(ctx, AuditFilter{})
	if err != nil {
		t.Fatalf("failed to query remaining: %v", err)
	}
	if len(remaining) != 1 || remaining[0].ID != "evt-recent" {
		t.Errorf("expected evt-recent remaining, got %v", remaining)
	}
}

func TestSQLiteStorage_AuditEvents_ConcurrentWrites(t *testing.T) {
	store, err := NewSQLiteStorage(Config{Path: ":memory:", MaxConns: 5})
	if err != nil {
		t.Fatalf("failed to create sqlite storage: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	const numGoroutines = 20
	const eventsPerGoroutine = 10

	var wg sync.WaitGroup
	errCh := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(routineID int) {
			defer wg.Done()
			for j := 0; j < eventsPerGoroutine; j++ {
				evt := model.AuditEvent{
					ID:        fmt.Sprintf("evt-r%d-e%d", routineID, j),
					Timestamp: time.Now().UTC(),
					EventType: model.EventAuthSuccess,
					Severity:  model.AuditSeverityInfo,
					Outcome:   model.AuditOutcomeSuccess,
					Actor: model.AuditActor{
						Type:     model.ActorTypeAuthenticatedClient,
						Identity: "api-client",
					},
					Source: model.AuditSource{
						Address: "127.0.0.1",
					},
					Message: fmt.Sprintf("Routine %d event %d", routineID, j),
				}
				if err := store.SaveAuditEvent(ctx, evt); err != nil {
					errCh <- fmt.Errorf("routine %d failed write %d: %w", routineID, j, err)
					return
				}
			}
		}(i)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("concurrent write error: %v", err)
	}

	count, err := store.CountAuditEvents(ctx, AuditFilter{})
	if err != nil {
		t.Fatalf("failed to count audit events after concurrent writes: %v", err)
	}

	expectedCount := int64(numGoroutines * eventsPerGoroutine)
	if count != expectedCount {
		t.Errorf("expected %d events, got %d", expectedCount, count)
	}
}
