package storage

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/DocHoax/watchdog/internal/incidents"
	"github.com/DocHoax/watchdog/pkg/model"
)

func TestSQLiteStorage_Incidents_CRUD(t *testing.T) {
	store, err := NewSQLiteStorage(Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("failed to create sqlite storage: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	now := time.Now().UTC()

	inc := &incidents.Incident{
		ID:              "inc-storage-01",
		Title:           "Memory Cascade on Worker Nodes",
		Status:          incidents.IncidentStatusDetected,
		Severity:        model.SeverityCritical,
		Scope:           incidents.IncidentScopeMultiNode,
		Confidence:      "high",
		StartTime:       now.Add(-30 * time.Minute),
		UpdatedAt:       now.Add(-10 * time.Minute),
		PrimarySymptoms: []string{"Memory pressure", "OOM killer active"},
		AffectedNodes:   []string{"node-1", "node-2"},
		RootSignals: []incidents.IncidentSignal{
			{
				ID:          "sig-1",
				Type:        incidents.SignalTypeAlert,
				Source:      "HighMemoryUsage",
				NodeID:      "node-1",
				Severity:    model.SeverityCritical,
				Description: "Memory reached 95%",
				Timestamp:   now.Add(-25 * time.Minute),
			},
		},
		Impact: incidents.ImpactScope{
			FleetPercentage: 20.0,
			TotalFleetNodes: 10,
			AffectedNodeIDs: []string{"node-1", "node-2"},
			Subsystems:      []string{"memory"},
		},
		SeverityExplanation: incidents.SeverityExplanation{
			BaseScore:          85,
			CalculatedSeverity: model.SeverityCritical,
			Confidence:         "high",
			Factors: []incidents.SeverityFactorContribution{
				{
					Name:        "AlertSeverity",
					Category:    "alert",
					Weight:      1.0,
					Points:      35,
					Description: "Critical active alert",
				},
			},
			Reasoning: []string{"Critical active alert on worker nodes"},
		},
		Findings: []incidents.IntelligenceFinding{
			{
				ID:                     "find-1",
				Category:               incidents.FindingCategoryResourceExhaustion,
				Severity:               model.SeverityCritical,
				Confidence:             incidents.FindingConfidenceHigh,
				Title:                  "Memory exhaustion imminent",
				Description:            "Node 1 approaching physical memory limit",
				NonInvasiveSuggestions: []string{"Investigate leaking worker processes"},
				DetectedAt:             now,
			},
		},
		Metadata: map[string]string{
			"env": "production",
		},
		Tags: map[string]string{
			"cluster": "k8s-prod",
		},
		Summary: "Severe memory exhaustion affecting multiple nodes",
	}

	// 1. Save incident
	if err := store.SaveIncident(ctx, inc); err != nil {
		t.Fatalf("failed to save incident: %v", err)
	}

	// 2. Get incident
	fetched, err := store.GetIncident(ctx, "inc-storage-01")
	if err != nil {
		t.Fatalf("failed to get incident: %v", err)
	}
	if fetched == nil {
		t.Fatalf("expected non-nil incident")
	}
	if fetched.ID != inc.ID || fetched.Title != inc.Title || fetched.Status != incidents.IncidentStatusDetected {
		t.Errorf("mismatch in fetched incident fields: %+v", fetched)
	}
	if len(fetched.AffectedNodes) != 2 || fetched.AffectedNodes[0] != "node-1" {
		t.Errorf("mismatch in affected nodes: %v", fetched.AffectedNodes)
	}
	if len(fetched.PrimarySymptoms) != 2 {
		t.Errorf("mismatch in primary symptoms: %v", fetched.PrimarySymptoms)
	}
	if len(fetched.RootSignals) != 1 || fetched.RootSignals[0].Source != "HighMemoryUsage" {
		t.Errorf("mismatch in root signals: %+v", fetched.RootSignals)
	}
	if fetched.Impact.FleetPercentage != 20.0 {
		t.Errorf("mismatch in impact: %+v", fetched.Impact)
	}
	if fetched.SeverityExplanation.BaseScore != 85 {
		t.Errorf("mismatch in severity explanation: %+v", fetched.SeverityExplanation)
	}
	if len(fetched.Findings) != 1 || fetched.Findings[0].ID != "find-1" {
		t.Errorf("mismatch in findings: %+v", fetched.Findings)
	}
	if fetched.Metadata["env"] != "production" || fetched.Tags["cluster"] != "k8s-prod" || fetched.Summary != inc.Summary {
		t.Errorf("mismatch in metadata/tags/summary: meta=%v, tags=%v, summary=%s", fetched.Metadata, fetched.Tags, fetched.Summary)
	}

	// 3. Upsert incident update
	inc.Title = "Memory Cascade on Worker Nodes (Escalated)"
	inc.Confidence = "critical"
	if err := store.SaveIncident(ctx, inc); err != nil {
		t.Fatalf("failed to update incident via upsert: %v", err)
	}
	updated, err := store.GetIncident(ctx, "inc-storage-01")
	if err != nil || updated.Title != "Memory Cascade on Worker Nodes (Escalated)" {
		t.Fatalf("upsert did not update fields properly: %+v, err: %v", updated, err)
	}

	// 4. Update status transition
	resolvedAt := now.Add(5 * time.Minute)
	if err := store.UpdateIncidentStatus(ctx, "inc-storage-01", incidents.IncidentStatusResolved, "Resolved by node restart", &resolvedAt); err != nil {
		t.Fatalf("failed to update incident status: %v", err)
	}
	resolved, err := store.GetIncident(ctx, "inc-storage-01")
	if err != nil || resolved.Status != incidents.IncidentStatusResolved || resolved.ResolvedAt == nil {
		t.Fatalf("status update failed: %+v, err: %v", resolved, err)
	}

	// 5. Non-existent incident handling
	missing, err := store.GetIncident(ctx, "non-existent-id")
	if err != nil || missing != nil {
		t.Fatalf("expected nil and no error for missing incident, got: %v, err: %v", missing, err)
	}
	err = store.UpdateIncidentStatus(ctx, "non-existent-id", incidents.IncidentStatusResolved, "", nil)
	if err == nil {
		t.Fatalf("expected ErrIncidentNotFound for updating missing incident")
	}
}

func TestSQLiteStorage_Incidents_ListAndFilter(t *testing.T) {
	store, err := NewSQLiteStorage(Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("failed to create sqlite storage: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	now := time.Now().UTC()

	// Insert 4 incidents with different characteristics
	incs := []*incidents.Incident{
		{
			ID:            "inc-list-1",
			Title:         "High CPU Utilization",
			Status:        incidents.IncidentStatusDetected,
			Severity:      model.SeverityCritical,
			Scope:         incidents.IncidentScopeNode,
			StartTime:     now.Add(-4 * time.Hour),
			AffectedNodes: []string{"node-cpu-1"},
		},
		{
			ID:            "inc-list-2",
			Title:         "Database I/O Latency Spikes",
			Status:        incidents.IncidentStatusInvestigating,
			Severity:      model.SeverityWarning,
			Scope:         incidents.IncidentScopeNode,
			StartTime:     now.Add(-3 * time.Hour),
			AffectedNodes: []string{"node-db-1"},
		},
		{
			ID:            "inc-list-3",
			Title:         "Fleet-wide Network Drops",
			Status:        incidents.IncidentStatusResolved,
			Severity:      model.SeverityCritical,
			Scope:         incidents.IncidentScopeFleet,
			StartTime:     now.Add(-2 * time.Hour),
			AffectedNodes: []string{"node-cpu-1", "node-db-1", "node-net-1"},
		},
		{
			ID:            "inc-list-4",
			Title:         "Low Disk Space",
			Status:        incidents.IncidentStatusSuppressed,
			Severity:      model.SeverityInfo,
			Scope:         incidents.IncidentScopeNode,
			StartTime:     now.Add(-1 * time.Hour),
			AffectedNodes: []string{"node-storage-1"},
		},
	}

	for _, inc := range incs {
		if err := store.SaveIncident(ctx, inc); err != nil {
			t.Fatalf("failed to save incident %s: %v", inc.ID, err)
		}
	}

	// 1. Filter by Status
	list, total, err := store.ListIncidents(ctx, incidents.IncidentFilter{
		Status: []incidents.IncidentStatus{incidents.IncidentStatusDetected, incidents.IncidentStatusInvestigating},
	})
	if err != nil || total != 2 || len(list) != 2 {
		t.Fatalf("status filter failed: total=%d, len=%d, err=%v", total, len(list), err)
	}

	// 2. Filter by Severity
	list, total, err = store.ListIncidents(ctx, incidents.IncidentFilter{
		Severity: []model.Severity{model.SeverityCritical},
	})
	if err != nil || total != 2 || len(list) != 2 {
		t.Fatalf("severity filter failed: total=%d, len=%d, err=%v", total, len(list), err)
	}

	// 3. Filter by Scope
	list, total, err = store.ListIncidents(ctx, incidents.IncidentFilter{
		Scope: []incidents.IncidentScope{incidents.IncidentScopeFleet},
	})
	if err != nil || total != 1 || len(list) != 1 || list[0].ID != "inc-list-3" {
		t.Fatalf("scope filter failed: total=%d, len=%d, err=%v", total, len(list), err)
	}

	// 4. Filter by NodeID
	list, total, err = store.ListIncidents(ctx, incidents.IncidentFilter{
		NodeID: "node-cpu-1",
	})
	if err != nil || total != 2 || len(list) != 2 {
		t.Fatalf("nodeID filter failed: total=%d, len=%d, err=%v", total, len(list), err)
	}

	// 5. Filter by Search Query
	list, total, err = store.ListIncidents(ctx, incidents.IncidentFilter{
		Search: "Network",
	})
	if err != nil || total != 1 || len(list) != 1 || list[0].ID != "inc-list-3" {
		t.Fatalf("search filter failed: total=%d, len=%d, err=%v", total, len(list), err)
	}

	// 6. Filter by Time Window
	list, total, err = store.ListIncidents(ctx, incidents.IncidentFilter{
		StartTime: now.Add(-3*time.Hour - 10*time.Minute),
		EndTime:   now.Add(-1*time.Hour - 30*time.Minute),
	})
	if err != nil || total != 2 || len(list) != 2 {
		t.Fatalf("time window filter failed: total=%d, len=%d, err=%v", total, len(list), err)
	}

	// 7. Pagination and Sorting
	list, total, err = store.ListIncidents(ctx, incidents.IncidentFilter{
		Limit:     2,
		Offset:    1,
		SortBy:    "start_time",
		SortOrder: "ASC",
	})
	if err != nil || total != 4 || len(list) != 2 {
		t.Fatalf("pagination failed: total=%d, len=%d, err=%v", total, len(list), err)
	}
	if list[0].ID != "inc-list-2" || list[1].ID != "inc-list-3" {
		t.Fatalf("sorting/pagination mismatch: got [%s, %s]", list[0].ID, list[1].ID)
	}
}

func TestSQLiteStorage_Timeline_SaveAndGet(t *testing.T) {
	store, err := NewSQLiteStorage(Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("failed to create sqlite storage: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	now := time.Now().UTC()

	inc := &incidents.Incident{
		ID:        "inc-tl-01",
		Title:     "Cascading Failure",
		Status:    incidents.IncidentStatusDetected,
		Severity:  model.SeverityCritical,
		StartTime: now.Add(-1 * time.Hour),
	}
	if err := store.SaveIncident(ctx, inc); err != nil {
		t.Fatalf("failed to save base incident: %v", err)
	}

	entries := []incidents.IncidentTimelineEntry{
		{
			ID:          "tl-1",
			IncidentID:  "inc-tl-01",
			Timestamp:   now.Add(-40 * time.Minute),
			EventType:   incidents.TimelineEventAlertFired,
			Source:      "HighCPU",
			NodeID:      "node-1",
			Severity:    model.SeverityWarning,
			Title:       "CPU usage > 80%",
			Description: "Initial spike detected",
			Payload:     map[string]any{"metric": "cpu", "val": 85.0},
		},
		{
			ID:          "tl-2",
			IncidentID:  "inc-tl-01",
			Timestamp:   now.Add(-30 * time.Minute),
			EventType:   incidents.TimelineEventAlertFired,
			Source:      "HighCPU",
			NodeID:      "node-2",
			Severity:    model.SeverityCritical,
			Title:       "CPU usage > 95%",
			Description: "Cascading spike on secondary node",
		},
		{
			ID:          "tl-3",
			IncidentID:  "inc-tl-01",
			Timestamp:   now.Add(-20 * time.Minute),
			EventType:   incidents.TimelineEventStatusChanged,
			Source:      "system",
			NodeID:      "",
			Severity:    model.SeverityInfo,
			Title:       "Incident state changed to Investigating",
			Description: "Operator triage in progress",
		},
	}

	// 1. Batch save timeline entries
	if err := store.SaveTimelineEntries(ctx, entries); err != nil {
		t.Fatalf("failed to save timeline entries: %v", err)
	}

	// 2. Retrieve full timeline
	tl, err := store.GetTimeline(ctx, "inc-tl-01", incidents.TimelineFilter{})
	if err != nil || len(tl) != 3 {
		t.Fatalf("failed to get full timeline: len=%d, err=%v", len(tl), err)
	}
	// Verify ascending time order
	if tl[0].ID != "tl-1" || tl[1].ID != "tl-2" || tl[2].ID != "tl-3" {
		t.Errorf("timeline ordering incorrect: %+v", tl)
	}
	if tl[0].Payload["metric"] != "cpu" {
		t.Errorf("timeline payload not deserialized properly: %+v", tl[0].Payload)
	}

	// 3. Filter by node
	tlNode, err := store.GetTimeline(ctx, "inc-tl-01", incidents.TimelineFilter{
		NodeID: "node-1",
	})
	if err != nil || len(tlNode) != 1 || tlNode[0].ID != "tl-1" {
		t.Fatalf("node timeline filter failed: len=%d, err=%v", len(tlNode), err)
	}

	// 4. Filter by time window
	tlTime, err := store.GetTimeline(ctx, "inc-tl-01", incidents.TimelineFilter{
		StartTime: now.Add(-35 * time.Minute),
		EndTime:   now.Add(-15 * time.Minute),
	})
	if err != nil || len(tlTime) != 2 {
		t.Fatalf("time timeline filter failed: len=%d, err=%v", len(tlTime), err)
	}

	// 5. Verification on GetIncident loading timeline
	incWithTL, err := store.GetIncident(ctx, "inc-tl-01")
	if err != nil || incWithTL == nil || len(incWithTL.Timeline) != 3 {
		t.Fatalf("GetIncident failed to load embedded timeline: %+v", incWithTL)
	}
}

func TestSQLiteStorage_GetIncidentHistory(t *testing.T) {
	store, err := NewSQLiteStorage(Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("failed to create sqlite storage: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	now := time.Now().UTC()

	incs := []*incidents.Incident{
		{
			ID:        "inc-hist-1",
			Title:     "Recent Incident",
			Status:    incidents.IncidentStatusResolved,
			Severity:  model.SeverityWarning,
			StartTime: now.Add(-2 * time.Hour),
		},
		{
			ID:        "inc-hist-2",
			Title:     "Old Incident",
			Status:    incidents.IncidentStatusResolved,
			Severity:  model.SeverityCritical,
			StartTime: now.Add(-48 * time.Hour),
		},
	}

	for _, inc := range incs {
		if err := store.SaveIncident(ctx, inc); err != nil {
			t.Fatalf("failed to save incident: %v", err)
		}
	}

	// Lookback 24 hours should only return inc-hist-1
	history, err := store.GetIncidentHistory(ctx, 24*time.Hour)
	if err != nil || len(history) != 1 || history[0].ID != "inc-hist-1" {
		t.Fatalf("GetIncidentHistory failed: len=%d, err=%v", len(history), err)
	}
}

func TestSQLiteStorage_Incidents_Concurrency(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "concurrency_test.db")
	store, err := NewSQLiteStorage(Config{Path: dbPath, MaxConns: 5})
	if err != nil {
		t.Fatalf("failed to create sqlite storage: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	now := time.Now().UTC()

	var wg sync.WaitGroup
	numWorkers := 10
	numOpsPerWorker := 10

	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for i := 0; i < numOpsPerWorker; i++ {
				incID := fmt.Sprintf("inc-conc-%d-%d", workerID, i)
				inc := &incidents.Incident{
					ID:            incID,
					Title:         fmt.Sprintf("Concurrent Incident %d-%d", workerID, i),
					Status:        incidents.IncidentStatusDetected,
					Severity:      model.SeverityWarning,
					StartTime:     now,
					AffectedNodes: []string{fmt.Sprintf("node-%d", workerID)},
				}

				if err := store.SaveIncident(ctx, inc); err != nil {
					t.Errorf("worker %d failed to save incident: %v", workerID, err)
					return
				}

				entry := incidents.IncidentTimelineEntry{
					ID:         fmt.Sprintf("tl-%d-%d", workerID, i),
					IncidentID: incID,
					Timestamp:  now,
					EventType:  incidents.TimelineEventAlertFired,
					Source:     "worker",
					Title:      "Alert fired",
				}
				if err := store.SaveTimelineEntries(ctx, []incidents.IncidentTimelineEntry{entry}); err != nil {
					t.Errorf("worker %d failed to save timeline: %v", workerID, err)
					return
				}

				fetched, err := store.GetIncident(ctx, incID)
				if err != nil || fetched == nil {
					t.Errorf("worker %d failed to get incident: %v", workerID, err)
					return
				}
			}
		}(w)
	}

	wg.Wait()

	list, total, err := store.ListIncidents(ctx, incidents.IncidentFilter{Limit: 200})
	if err != nil || total != numWorkers*numOpsPerWorker || len(list) != numWorkers*numOpsPerWorker {
		t.Fatalf("concurrent test failed verification: total=%d, len=%d, err=%v", total, len(list), err)
	}
}
