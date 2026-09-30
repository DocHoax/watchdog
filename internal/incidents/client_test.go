package incidents

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DocHoax/watchdog/pkg/model"
)

func TestIncidentsClient_AllMethods(t *testing.T) {
	var receivedAuth string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/api/v1/incidents" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(IncidentListResponse{
				Incidents: []Incident{
					{
						ID:       "inc-001",
						Title:    "High CPU Spikes across cluster",
						Severity: model.SeverityCritical,
						Status:   IncidentStatusDetected,
						Scope:    IncidentScopeMultiNode,
					},
				},
				Total:     1,
				Limit:     10,
				Offset:    0,
				Timestamp: time.Now().UTC().Format(time.RFC3339),
			})

		case r.URL.Path == "/api/v1/incidents/summary" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(IncidentSummary{
				TotalCount:         5,
				DetectedCount:      2,
				ResolvedCount:      3,
				CriticalCount:      1,
				ScopeDistribution:  map[string]int{"multi_node": 2, "node": 3},
			})

		case r.URL.Path == "/api/v1/incidents/similar" && r.Method == http.MethodGet:
			id := r.URL.Query().Get("id")
			if id == "inc-001" {
				_ = json.NewEncoder(w).Encode(SimilarIncidentsResponse{
					IncidentID: "inc-001",
					SimilarIncidents: []SimilarIncidentResult{
						{
							Incident: Incident{
								ID:       "inc-000",
								Title:    "Past CPU Spike",
								Severity: model.SeverityCritical,
							},
							SimilarityScore: 0.85,
							Explanation:     "High symptom and subsystem overlap",
						},
					},
					Count:            1,
					MinSimilarityCut: 0.7,
					Timestamp:        time.Now().UTC().Format(time.RFC3339),
				})
			} else {
				w.WriteHeader(http.StatusNotFound)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"error": map[string]any{
						"code":    "NOT_FOUND",
						"message": "incident not found",
					},
				})
			}

		case r.URL.Path == "/api/v1/incidents/inc-001" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(Incident{
				ID:       "inc-001",
				Title:    "High CPU Spikes across cluster",
				Severity: model.SeverityCritical,
				Status:   IncidentStatusDetected,
			})

		case r.URL.Path == "/api/v1/incidents/inc-001/timeline" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(TimelineResponse{
				IncidentID: "inc-001",
				Timeline: []IncidentTimelineEntry{
					{
						ID:          "tl-1",
						IncidentID:  "inc-001",
						Timestamp:   time.Now().UTC(),
						EventType:   TimelineEventAlertFired,
						NodeID:      "node-1",
						Title:       "CPU crossed 95%",
						Description: "CPU spike detected on host",
					},
				},
				Count:     1,
				Timestamp: time.Now().UTC().Format(time.RFC3339),
			})

		case r.URL.Path == "/api/v1/incidents/inc-001/related" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(RelatedIncidentsResponse{
				IncidentID: "inc-001",
				Signals: []IncidentSignal{
					{
						ID:          "sig-1",
						Source:      "cpu_collector",
						NodeID:      "node-1",
						Type:        SignalTypeAlert,
						Description: "cpu.usage threshold exceeded",
					},
				},
				Recurrence: &RecurrenceAnalysis{
					PatternKey:      "cpu_spike",
					OccurrenceCount: 3,
					Periodicity:     PeriodicityPeriodic,
				},
				SimilarIncidents: []SimilarIncidentResult{},
				Timestamp:        time.Now().UTC().Format(time.RFC3339),
			})

		case r.URL.Path == "/api/v1/incidents/inc-001/impact" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(ImpactAnalysis{
				IncidentID: "inc-001",
				Scope:      IncidentScopeMultiNode,
				Impact: ImpactScope{
					AffectedNodeIDs: []string{"node-1", "node-2"},
					Subsystems:      []string{"cpu", "api-gateway"},
				},
				CriticalNodesCount: 1,
				WarningNodesCount:  1,
				Summary:            "2 nodes affected",
			})

		case r.URL.Path == "/api/v1/incidents/inc-001/findings" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(FindingsResponse{
				IncidentID: "inc-001",
				Findings: []IntelligenceFinding{
					{
						ID:          "find-1",
						Category:    FindingCategoryAnomalyCluster,
						Severity:    model.SeverityCritical,
						Title:       "Abnormal CPU Activity",
						Description: "Detected continuous CPU threshold breach",
					},
				},
				Count:     1,
				Timestamp: time.Now().UTC().Format(time.RFC3339),
			})

		case r.URL.Path == "/api/v1/incidents/inc-001/investigate" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(IncidentInvestigationReport{
				Incident: Incident{
					ID:       "inc-001",
					Title:    "High CPU Spikes across cluster",
					Severity: model.SeverityCritical,
				},
				GeneratedAt: time.Now().UTC(),
				Summary:     "Investigation indicates container runaway",
			})

		case r.URL.Path == "/api/v1/incidents/inc-001/status" && r.Method == http.MethodPost:
			var req UpdateIncidentStatusPayload
			_ = json.NewDecoder(r.Body).Decode(&req)
			_ = json.NewEncoder(w).Encode(Incident{
				ID:       "inc-001",
				Title:    "High CPU Spikes across cluster",
				Severity: model.SeverityCritical,
				Status:   req.Status,
			})

		default:
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{
					"code":    "NOT_FOUND",
					"message": "resource not found",
				},
			})
		}
	}))
	defer ts.Close()

	client := NewClient(ClientConfig{
		Endpoint: ts.URL,
		Token:    "secret-incident-token",
		Timeout:  3 * time.Second,
	})
	ctx := context.Background()

	if client.Endpoint() != ts.URL {
		t.Errorf("expected endpoint %s, got %s", ts.URL, client.Endpoint())
	}

	// 1. ListIncidents
	listResp, err := client.ListIncidents(ctx, IncidentFilter{
		Status:    []IncidentStatus{IncidentStatusDetected},
		Severity:  []model.Severity{model.SeverityCritical},
		Scope:     []IncidentScope{IncidentScopeMultiNode},
		NodeID:    "node-1",
		Search:    "CPU",
		StartTime: time.Now().Add(-1 * time.Hour),
		EndTime:   time.Now(),
		Limit:     10,
		Offset:    0,
		SortBy:    "created_at",
		SortOrder: "desc",
	})
	if err != nil {
		t.Fatalf("ListIncidents failed: %v", err)
	}
	if listResp.Total != 1 || len(listResp.Incidents) != 1 || listResp.Incidents[0].ID != "inc-001" {
		t.Errorf("unexpected ListIncidents response: %+v", listResp)
	}
	if receivedAuth != "Bearer secret-incident-token" {
		t.Errorf("expected auth Bearer secret-incident-token, got %s", receivedAuth)
	}

	// 2. GetSummary
	summary, err := client.GetSummary(ctx)
	if err != nil {
		t.Fatalf("GetSummary failed: %v", err)
	}
	if summary.TotalCount != 5 || summary.DetectedCount != 2 {
		t.Errorf("unexpected GetSummary response: %+v", summary)
	}

	// 3. GetSimilar
	simResp, err := client.GetSimilar(ctx, "inc-001", 0.7, 5)
	if err != nil {
		t.Fatalf("GetSimilar failed: %v", err)
	}
	if simResp.IncidentID != "inc-001" || len(simResp.SimilarIncidents) != 1 {
		t.Errorf("unexpected GetSimilar response: %+v", simResp)
	}

	// 4. GetIncident
	inc, err := client.GetIncident(ctx, "inc-001")
	if err != nil {
		t.Fatalf("GetIncident failed: %v", err)
	}
	if inc.ID != "inc-001" {
		t.Errorf("expected incident ID inc-001, got %s", inc.ID)
	}

	// 5. GetTimeline
	timeline, err := client.GetTimeline(ctx, "inc-001", TimelineFilter{
		NodeID:      "node-1",
		MinSeverity: model.SeverityCritical,
		StartTime:   time.Now().Add(-1 * time.Hour),
		EndTime:     time.Now(),
		Limit:       10,
	})
	if err != nil {
		t.Fatalf("GetTimeline failed: %v", err)
	}
	if timeline.Count != 1 || len(timeline.Timeline) != 1 {
		t.Errorf("unexpected GetTimeline response: %+v", timeline)
	}

	// 6. GetRelated
	related, err := client.GetRelated(ctx, "inc-001")
	if err != nil {
		t.Fatalf("GetRelated failed: %v", err)
	}
	if related.IncidentID != "inc-001" || len(related.Signals) != 1 || related.Recurrence == nil {
		t.Errorf("unexpected GetRelated response: %+v", related)
	}

	// 7. GetImpact
	impact, err := client.GetImpact(ctx, "inc-001")
	if err != nil {
		t.Fatalf("GetImpact failed: %v", err)
	}
	if impact.IncidentID != "inc-001" || len(impact.Impact.AffectedNodeIDs) != 2 {
		t.Errorf("unexpected GetImpact response: %+v", impact)
	}

	// 8. GetFindings
	findings, err := client.GetFindings(ctx, "inc-001")
	if err != nil {
		t.Fatalf("GetFindings failed: %v", err)
	}
	if findings.Count != 1 || len(findings.Findings) != 1 {
		t.Errorf("unexpected GetFindings response: %+v", findings)
	}

	// 9. Investigate
	inv, err := client.Investigate(ctx, "inc-001")
	if err != nil {
		t.Fatalf("Investigate failed: %v", err)
	}
	if inv.Incident.ID != "inc-001" || inv.Summary == "" {
		t.Errorf("unexpected Investigate response: %+v", inv)
	}

	// 10. UpdateStatus
	updated, err := client.UpdateStatus(ctx, "inc-001", IncidentStatusResolved, "Applied rate limit")
	if err != nil {
		t.Fatalf("UpdateStatus failed: %v", err)
	}
	if updated.Status != IncidentStatusResolved {
		t.Errorf("expected status resolved, got %s", updated.Status)
	}

	// Validation error checks
	if _, err := client.GetSimilar(ctx, "", 0.5, 5); err == nil {
		t.Errorf("expected error on empty incident ID in GetSimilar")
	}
	if _, err := client.GetIncident(ctx, ""); err == nil {
		t.Errorf("expected error on empty incident ID in GetIncident")
	}
	if _, err := client.GetTimeline(ctx, "", TimelineFilter{}); err == nil {
		t.Errorf("expected error on empty incident ID in GetTimeline")
	}
	if _, err := client.GetRelated(ctx, ""); err == nil {
		t.Errorf("expected error on empty incident ID in GetRelated")
	}
	if _, err := client.GetImpact(ctx, ""); err == nil {
		t.Errorf("expected error on empty incident ID in GetImpact")
	}
	if _, err := client.GetFindings(ctx, ""); err == nil {
		t.Errorf("expected error on empty incident ID in GetFindings")
	}
	if _, err := client.Investigate(ctx, ""); err == nil {
		t.Errorf("expected error on empty incident ID in Investigate")
	}
	if _, err := client.UpdateStatus(ctx, "", IncidentStatusResolved, "done"); err == nil {
		t.Errorf("expected error on empty incident ID in UpdateStatus")
	}

	// 404 error check
	if _, err := client.GetIncident(ctx, "non-existent"); err == nil {
		t.Errorf("expected error for non-existent incident")
	}
}
