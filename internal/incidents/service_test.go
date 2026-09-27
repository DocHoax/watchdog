package incidents

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/DocHoax/watchdog/pkg/model"
)

type mockIncidentStore struct {
	mu        sync.RWMutex
	incidents map[string]Incident
	timelines map[string][]IncidentTimelineEntry
}

func newMockIncidentStore() *mockIncidentStore {
	return &mockIncidentStore{
		incidents: make(map[string]Incident),
		timelines: make(map[string][]IncidentTimelineEntry),
	}
}

func (m *mockIncidentStore) SaveIncident(ctx context.Context, inc *Incident) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if inc == nil {
		return errors.New("nil incident")
	}
	m.incidents[inc.ID] = *inc
	return nil
}

func (m *mockIncidentStore) GetIncident(ctx context.Context, id string) (*Incident, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	inc, ok := m.incidents[id]
	if !ok {
		return nil, nil
	}
	cpy := inc
	return &cpy, nil
}

func (m *mockIncidentStore) ListIncidents(ctx context.Context, filter IncidentFilter) ([]Incident, int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var list []Incident
	for _, inc := range m.incidents {
		if len(filter.Status) > 0 && !slices.Contains(filter.Status, inc.Status) {
			continue
		}
		if len(filter.Severity) > 0 && !slices.Contains(filter.Severity, inc.Severity) {
			continue
		}
		if len(filter.Scope) > 0 && !slices.Contains(filter.Scope, inc.Scope) {
			continue
		}
		list = append(list, inc)
	}
	total := len(list)
	if filter.Limit > 0 && len(list) > filter.Limit {
		list = list[:filter.Limit]
	}
	return list, total, nil
}

func (m *mockIncidentStore) UpdateIncidentStatus(ctx context.Context, id string, status IncidentStatus, reason string, resolvedAt *time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	inc, ok := m.incidents[id]
	if !ok {
		return fmt.Errorf("%w: %s", ErrIncidentNotFound, id)
	}
	inc.Status = status
	if resolvedAt != nil {
		inc.ResolvedAt = resolvedAt
	}
	m.incidents[id] = inc
	return nil
}

func (m *mockIncidentStore) SaveTimelineEntries(ctx context.Context, entries []IncidentTimelineEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, e := range entries {
		m.timelines[e.IncidentID] = append(m.timelines[e.IncidentID], e)
	}
	return nil
}

func (m *mockIncidentStore) GetTimeline(ctx context.Context, incidentID string, filter TimelineFilter) ([]IncidentTimelineEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	entries := m.timelines[incidentID]
	return FilterTimeline(entries, filter.NodeID, filter.MinSeverity, filter.StartTime, filter.EndTime), nil
}

func (m *mockIncidentStore) GetIncidentHistory(ctx context.Context, lookback time.Duration) ([]Incident, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var list []Incident
	for _, inc := range m.incidents {
		list = append(list, inc)
	}
	return list, nil
}

func TestDefaultService_Operations(t *testing.T) {
	ctx := context.Background()
	store := newMockIncidentStore()
	svc := NewService(store, 15*time.Minute)

	now := time.Now().UTC()
	inc := &Incident{
		ID:              "inc-svc-1",
		Title:           "High Memory Exhaustion",
		Scope:           IncidentScopeMultiNode,
		Severity:        model.SeverityCritical,
		Status:          IncidentStatusDetected,
		Confidence:      "high",
		AffectedNodes:   []string{"node-1", "node-2"},
		PrimarySymptoms: []string{"Memory pressure"},
		RootSignals: []IncidentSignal{
			{
				ID:          "sig-mem-1",
				Type:        SignalTypeAlert,
				Source:      "HighMemory",
				NodeID:      "node-1",
				Severity:    model.SeverityCritical,
				Description: "Memory usage 95%",
				Timestamp:   now,
			},
		},
		Impact: ImpactScope{
			FleetPercentage:  20.0,
			TotalFleetNodes:  10,
			AffectedNodeIDs:  []string{"node-1", "node-2"},
			Subsystems:       []string{"memory"},
		},
		StartTime: now,
	}

	// 1. Save and Get
	if err := store.SaveIncident(ctx, inc); err != nil {
		t.Fatalf("failed to save incident: %v", err)
	}

	fetched, err := svc.GetIncident(ctx, "inc-svc-1")
	if err != nil {
		t.Fatalf("failed to get incident: %v", err)
	}
	if fetched.ID != "inc-svc-1" || fetched.Status != IncidentStatusDetected {
		t.Errorf("unexpected incident data: %+v", fetched)
	}

	// 2. List Incidents
	list, total, err := svc.ListIncidents(ctx, IncidentFilter{Limit: 10})
	if err != nil || total != 1 || len(list) != 1 {
		t.Errorf("unexpected list output: list=%d, total=%d, err=%v", len(list), total, err)
	}

	// 3. Status Transition: Detected -> Investigating
	updated, err := svc.UpdateStatus(ctx, "inc-svc-1", IncidentStatusInvestigating, "Operator triage initiated")
	if err != nil {
		t.Fatalf("failed to update status: %v", err)
	}
	if updated.Status != IncidentStatusInvestigating {
		t.Errorf("expected status investigating, got %s", updated.Status)
	}

	// 4. Invalid Status Transition: Investigating -> Detected (rejected)
	_, err = svc.UpdateStatus(ctx, "inc-svc-1", IncidentStatusDetected, "Invalid step back")
	if err == nil {
		t.Errorf("expected error for illegal status transition")
	}

	// 5. Transition to Resolved
	resolved, err := svc.UpdateStatus(ctx, "inc-svc-1", IncidentStatusResolved, "Memory leak patched")
	if err != nil {
		t.Fatalf("failed to resolve incident: %v", err)
	}
	if resolved.Status != IncidentStatusResolved || resolved.ResolvedAt == nil {
		t.Errorf("expected resolved status with resolved timestamp, got %+v", resolved)
	}

	// 6. Get Impact
	impact, err := svc.GetImpact(ctx, "inc-svc-1")
	if err != nil || impact == nil {
		t.Fatalf("failed to get impact: %v", err)
	}
	if impact.Scope != IncidentScopeMultiNode {
		t.Errorf("expected multi-node impact scope, got %v", impact.Scope)
	}

	// 7. Get Findings
	findings, err := svc.GetFindings(ctx, "inc-svc-1")
	if err != nil || len(findings) == 0 {
		t.Fatalf("failed to get findings: %v, findings: %d", err, len(findings))
	}

	// 8. Investigate
	report, err := svc.Investigate(ctx, "inc-svc-1")
	if err != nil || report == nil {
		t.Fatalf("failed to investigate: %v", err)
	}
	if report.Incident.ID != "inc-svc-1" || report.Recurrence == nil {
		t.Errorf("investigation report missing components: %+v", report)
	}

	// 9. Get Summary
	summary, err := svc.GetSummary(ctx)
	if err != nil || summary == nil {
		t.Fatalf("failed to get summary: %v", err)
	}
	if summary.TotalCount != 1 || summary.ResolvedCount != 1 || summary.CriticalCount != 1 {
		t.Errorf("unexpected summary counts: %+v", summary)
	}
}

func TestDefaultService_EvaluateFleetSignals(t *testing.T) {
	ctx := context.Background()
	store := newMockIncidentStore()
	svc := NewService(store, 15*time.Minute)

	now := time.Now().UTC()
	bundles := []NodeSignalsBundle{
		{
			NodeID:   "node-1",
			Hostname: "srv-01",
			Tags:     map[string]string{"env": "prod"},
			Alerts: []model.AlertEvent{
				{
					ID:       "alt-1",
					RuleName: "HighCPU",
					Severity: model.SeverityCritical,
					FiredAt:  now,
					IsActive: true,
					Message:  "CPU > 90%",
				},
			},
		},
		{
			NodeID:   "node-2",
			Hostname: "srv-02",
			Tags:     map[string]string{"env": "prod"},
			Alerts: []model.AlertEvent{
				{
					ID:       "alt-2",
					RuleName: "HighCPU",
					Severity: model.SeverityCritical,
					FiredAt:  now,
					IsActive: true,
					Message:  "CPU > 95%",
				},
			},
		},
	}

	detected, err := svc.EvaluateFleetSignals(ctx, bundles, 2)
	if err != nil {
		t.Fatalf("failed to evaluate fleet signals: %v", err)
	}
	if len(detected) == 0 {
		t.Fatalf("expected at least 1 incident detected")
	}

	// Verify persistence in store
	list, total, err := store.ListIncidents(ctx, IncidentFilter{})
	if err != nil || total == 0 || len(list) == 0 {
		t.Errorf("expected incidents to be saved in store, got %d, total %d", len(list), total)
	}
}
