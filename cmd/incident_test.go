package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DocHoax/watchdog/internal/config"
	"github.com/DocHoax/watchdog/internal/incidents"
	"github.com/DocHoax/watchdog/pkg/model"
	"gopkg.in/yaml.v3"
)

func resetIncidentFlags() {
	incServerURL = ""
	incToken = ""
	incTokenFile = ""
	incTokenEnv = ""
	incInsecureTLS = false
	incTimeout = 10 * time.Second
	incFormat = "text"
	incStatuses = nil
	incSeverities = nil
	incScopes = nil
	incNodeID = ""
	incSearch = ""
	incStartTime = ""
	incEndTime = ""
	incLimit = 50
	incOffset = 0
	incSortBy = "start_time"
	incSortOrder = "desc"
	incMinSimilarity = 0.3
	incMinSeverity = ""
	incReason = ""
	globalCfg = config.DefaultConfig()
}

func setupIncidentMockServer(t *testing.T) *httptest.Server {
	t.Helper()
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	resolvedTime := now.Add(30 * time.Minute)

	sampleIncident := incidents.Incident{
		ID:         "inc-test-01",
		Title:      "Correlated High CPU and Memory Pressure",
		Summary:    "Fleet nodes experiencing critical memory exhaustion and CPU throttling",
		Status:     incidents.IncidentStatusInvestigating,
		Severity:   model.SeverityCritical,
		Scope:      incidents.IncidentScopeMultiNode,
		Confidence: "high",
		StartTime:  now,
		ResolvedAt: &resolvedTime,
		AffectedNodes: []string{
			"node-01",
			"node-02",
		},
		PrimarySymptoms: []string{
			"high_cpu_utilization",
			"memory_saturation",
		},
		RootSignals: []incidents.IncidentSignal{
			{
				ID:          "sig-01",
				NodeID:      "node-01",
				Type:        incidents.SignalTypeAlert,
				Source:      "alert_cpu_throttling",
				Severity:    model.SeverityCritical,
				Timestamp:   now,
				Value:       96.5,
				Threshold:   90.0,
				Description: "CPU utilization exceeded 90% threshold",
			},
			{
				ID:          "sig-02",
				NodeID:      "node-02",
				Type:        incidents.SignalTypeAnomaly,
				Source:      "anomaly_memory_usage",
				Severity:    model.SeverityWarning,
				Timestamp:   now.Add(2 * time.Minute),
				Value:       88.0,
				Threshold:   80.0,
				Description: "Memory consumption standard deviation anomaly",
			},
		},
		SeverityScore: 88.5,
		SeverityExplanation: incidents.SeverityExplanation{
			CalculatedSeverity: model.SeverityCritical,
			BaseScore:          88.5,
			Confidence:         "high",
			Factors: []incidents.SeverityFactorContribution{
				{
					Name:        "Active Critical Alerts",
					Category:    "alerts",
					Weight:      40.0,
					Points:      35.0,
					Description: "1 critical alert active on node-01",
				},
				{
					Name:        "Fleet Blast Radius",
					Category:    "blast_radius",
					Weight:      30.0,
					Points:      25.0,
					Description: "Affects 2 nodes (40% of cluster)",
				},
			},
			Reasoning: []string{
				"Multi-node critical incident driven by active CPU alerts and memory pressure",
			},
		},
		Impact: incidents.ImpactScope{
			AffectedNodeIDs:   []string{"node-01", "node-02"},
			AffectedHostnames: []string{"srv-01", "srv-02"},
			TotalFleetNodes:   5,
			FleetPercentage:   40.0,
			Subsystems:        []string{"compute", "memory"},
			Resources:          []string{"cpu", "ram"},
		},
		Tags: map[string]string{
			"cluster": "prod-us-east",
			"service": "database",
		},
		Metadata: map[string]string{
			"investigator": "operator-01",
		},
		CreatedAt: now,
		UpdatedAt: now.Add(15 * time.Minute),
	}

	sampleTimeline := []incidents.IncidentTimelineEntry{
		{
			ID:          "tl-001",
			IncidentID:  "inc-test-01",
			Timestamp:   now.Add(-5 * time.Minute),
			EventType:   incidents.TimelineEventAnomalyDetected,
			Source:      "anomaly_detector",
			NodeID:      "node-01",
			Severity:    model.SeverityWarning,
			Title:       "Memory Anomaly Detected",
			Description: "Elevated memory consumption pattern recognized",
			Payload: map[string]any{
				"value":     85.2,
				"threshold": 80.0,
			},
		},
		{
			ID:          "tl-002",
			IncidentID:  "inc-test-01",
			Timestamp:   now,
			EventType:   incidents.TimelineEventAlertFired,
			Source:      "alert_cpu_throttling",
			NodeID:      "node-01",
			Severity:    model.SeverityCritical,
			Title:       "CPU Throttling Alert Fired",
			Description: "CPU utilization reached 96.5%",
			Payload: map[string]any{
				"value":     96.5,
				"threshold": 90.0,
			},
		},
	}

	sampleRecurrence := &incidents.RecurrenceAnalysis{
		PatternKey:             "Correlated High CPU and Memory Pressure",
		OccurrenceCount:        4,
		FirstOccurrence:        now.Add(-72 * time.Hour),
		MostRecentOccurrence:   now,
		AverageInterval:        24 * time.Hour,
		MedianInterval:         24 * time.Hour,
		StandardDeviation:      2 * time.Hour,
		CoefficientOfVariation: 0.08,
		Periodicity:            incidents.PeriodicityPeriodic,
		IsFlapping:             false,
		Confidence:             "high",
		HistoricalIncidentIDs:  []string{"inc-001", "inc-002", "inc-003", "inc-test-01"},
		Summary:                "Periodic recurrence observed every ~24 hours matching daily cron peak",
	}

	sampleSimilar := []incidents.SimilarIncidentResult{
		{
			Incident: incidents.Incident{
				ID:        "inc-hist-01",
				Title:     "Historical Database CPU Throttling",
				Status:    incidents.IncidentStatusResolved,
				Severity:  model.SeverityCritical,
				StartTime: now.Add(-24 * time.Hour),
			},
			SimilarityScore: 0.85,
			Breakdown: incidents.SimilarityScoreBreakdown{
				SymptomSimilarity:   0.90,
				SubsystemSimilarity: 0.85,
				NodeSimilarity:      0.80,
				SeveritySimilarity:  0.85,
				Weights: map[string]float64{
					"symptoms":   0.40,
					"subsystems": 0.30,
					"nodes":      0.20,
					"severity":   0.10,
				},
			},
			Explanation: "High symptom similarity (90%) and subsystem overlap (85%)",
		},
	}

	sampleFindings := []incidents.IntelligenceFinding{
		{
			ID:          "find-001",
			Title:       "Correlated Resource Exhaustion",
			Category:    incidents.FindingCategoryResourceExhaustion,
			Severity:    model.SeverityCritical,
			Confidence:  incidents.FindingConfidenceHigh,
			Description: "Concurrent CPU and memory saturation detected across multiple database nodes",
			SupportingEvidence: []string{
				"node-01 CPU at 96.5%",
				"node-02 memory at 88.0%",
			},
			NonInvasiveSuggestions: []string{
				"Check query execution plans for runaway database tasks",
				"Verify no background compaction batch is running simultaneously",
			},
			AffectedNodes: []string{"node-01", "node-02"},
			DetectedAt:    now,
		},
	}

	sampleImpact := incidents.ImpactAnalysis{
		IncidentID:           "inc-test-01",
		Scope:                incidents.IncidentScopeMultiNode,
		EstimatedBlastRadius: "40% fleet impact across 2 nodes",
		Impact:               sampleIncident.Impact,
		CriticalNodesCount:   1,
		WarningNodesCount:    1,
		Summary:              "Multi-node impact targeting database tier nodes node-01 and node-02",
		AnalyzedAt:           now,
	}

	sampleReport := incidents.IncidentInvestigationReport{
		Incident:           sampleIncident,
		Recurrence:         sampleRecurrence,
		SimilarIncidents:   sampleSimilar,
		TimelineHighlights: sampleTimeline,
		Impact:             sampleImpact,
		Findings:           sampleFindings,
		Summary:            "Critical multi-node incident with 4 prior periodic recurrences",
		GeneratedAt:        now,
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/api/v1/incidents" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(incidents.IncidentListResponse{
				Incidents: []incidents.Incident{sampleIncident},
				Total:     1,
				Limit:     50,
				Offset:    0,
				Timestamp: now.Format(time.RFC3339),
			})

		case r.URL.Path == "/api/v1/incidents/summary" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(incidents.IncidentSummary{
				TotalCount:            10,
				DetectedCount:         1,
				AcknowledgedCount:     0,
				InvestigatingCount:    2,
				ResolvedCount:         6,
				SuppressedCount:       1,
				ReopenedCount:         0,
				CriticalCount:         2,
				WarningCount:          5,
				InfoCount:             3,
				ScopeDistribution: map[string]int{
					"fleet":      1,
					"multi_node": 3,
					"node":       6,
				},
				AverageResolutionTime: 45 * time.Minute,
				RecentIncidents:       []incidents.Incident{sampleIncident},
				TopAffectedNodes: []incidents.NodeIncidentCount{
					{NodeID: "node-01", Hostname: "srv-01", IncidentCount: 4, CriticalCount: 2, WarningCount: 2},
					{NodeID: "node-02", Hostname: "srv-02", IncidentCount: 2, CriticalCount: 1, WarningCount: 1},
				},
				EvaluatedAt: now,
			})

		case r.URL.Path == "/api/v1/incidents/similar" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(incidents.SimilarIncidentsResponse{
				IncidentID:       "inc-test-01",
				SimilarIncidents: sampleSimilar,
				Count:            len(sampleSimilar),
				MinSimilarityCut: 0.3,
				Timestamp:        now.Format(time.RFC3339),
			})

		case r.URL.Path == "/api/v1/incidents/inc-test-01" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(sampleIncident)

		case r.URL.Path == "/api/v1/incidents/inc-test-01/timeline" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(incidents.TimelineResponse{
				IncidentID: "inc-test-01",
				Timeline:   sampleTimeline,
				Count:      len(sampleTimeline),
				Timestamp:  now.Format(time.RFC3339),
			})

		case r.URL.Path == "/api/v1/incidents/inc-test-01/related" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(incidents.RelatedIncidentsResponse{
				IncidentID:       "inc-test-01",
				Signals:          sampleIncident.RootSignals,
				Recurrence:       sampleRecurrence,
				SimilarIncidents: sampleSimilar,
				Timestamp:        now.Format(time.RFC3339),
			})

		case r.URL.Path == "/api/v1/incidents/inc-test-01/impact" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(sampleImpact)

		case r.URL.Path == "/api/v1/incidents/inc-test-01/findings" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(incidents.FindingsResponse{
				IncidentID: "inc-test-01",
				Findings:   sampleFindings,
				Count:      len(sampleFindings),
				Timestamp:  now.Format(time.RFC3339),
			})

		case r.URL.Path == "/api/v1/incidents/inc-test-01/investigate" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(sampleReport)

		case r.URL.Path == "/api/v1/incidents/inc-test-01/status" && r.Method == http.MethodPost:
			var req incidents.UpdateIncidentStatusPayload
			_ = json.NewDecoder(r.Body).Decode(&req)
			updatedInc := sampleIncident
			updatedInc.Status = req.Status
			updatedInc.UpdatedAt = now.Add(20 * time.Minute)
			_ = json.NewEncoder(w).Encode(updatedInc)

		case r.URL.Path == "/api/v1/incidents/inc-nonexistent":
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(model.APIErrorResponse{
				Error: model.APIErrorDetail{
					Code:    "INCIDENT_NOT_FOUND",
					Message: "Incident 'inc-nonexistent' not found",
				},
			})

		default:
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(model.APIErrorResponse{
				Error: model.APIErrorDetail{
					Code:    "NOT_FOUND",
					Message: "Endpoint not found: " + r.URL.Path,
				},
			})
		}
	}))

	return ts
}

func TestCmd_Incident_RootDefaultToList(t *testing.T) {
	resetIncidentFlags()
	ts := setupIncidentMockServer(t)
	defer ts.Close()

	incServerURL = ts.URL
	out, err := captureStdout(func() error {
		return incidentCmd.RunE(incidentCmd, nil)
	})
	if err != nil {
		t.Fatalf("incident root command failed: %v", err)
	}

	if !strings.Contains(out, "inc-test-01") {
		t.Errorf("expected output to contain incident ID, got: %s", out)
	}
	if !strings.Contains(out, "Correlated High CPU and Memory Pressure") {
		t.Errorf("expected output to contain incident title, got: %s", out)
	}
}

func TestCmd_Incident_List_HumanReadable(t *testing.T) {
	resetIncidentFlags()
	ts := setupIncidentMockServer(t)
	defer ts.Close()

	incServerURL = ts.URL
	out, err := captureStdout(func() error {
		return runIncidentList(incidentListCmd, nil)
	})
	if err != nil {
		t.Fatalf("runIncidentList failed: %v", err)
	}

	if !strings.Contains(out, "Watchdog Correlated Cluster Incidents") {
		t.Errorf("expected header in output, got: %s", out)
	}
	if !strings.Contains(out, "inc-test-01") {
		t.Errorf("expected incident ID 'inc-test-01', got: %s", out)
	}
	if !strings.Contains(out, "CRITICAL") {
		t.Errorf("expected severity CRITICAL, got: %s", out)
	}
	if !strings.Contains(out, "investigating") {
		t.Errorf("expected status investigating, got: %s", out)
	}
	if !strings.Contains(out, "Showing 1 of 1 total incidents") {
		t.Errorf("expected total count, got: %s", out)
	}
}

func TestCmd_Incident_List_JSON(t *testing.T) {
	resetIncidentFlags()
	ts := setupIncidentMockServer(t)
	defer ts.Close()

	incServerURL = ts.URL
	incFormat = "json"

	out, err := captureStdout(func() error {
		return runIncidentList(incidentListCmd, nil)
	})
	if err != nil {
		t.Fatalf("runIncidentList JSON failed: %v", err)
	}

	var resp incidents.IncidentListResponse
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("failed to unmarshal JSON response: %v\nOutput was: %s", err, out)
	}

	if resp.Total != 1 || len(resp.Incidents) != 1 {
		t.Errorf("expected 1 incident, got total=%d, len=%d", resp.Total, len(resp.Incidents))
	}
	if resp.Incidents[0].ID != "inc-test-01" {
		t.Errorf("expected incident ID inc-test-01, got %s", resp.Incidents[0].ID)
	}
}

func TestCmd_Incident_List_YAML(t *testing.T) {
	resetIncidentFlags()
	ts := setupIncidentMockServer(t)
	defer ts.Close()

	incServerURL = ts.URL
	incFormat = "yaml"

	out, err := captureStdout(func() error {
		return runIncidentList(incidentListCmd, nil)
	})
	if err != nil {
		t.Fatalf("runIncidentList YAML failed: %v", err)
	}

	var resp incidents.IncidentListResponse
	if err := yaml.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("failed to unmarshal YAML response: %v\nOutput was: %s", err, out)
	}

	if resp.Total != 1 || len(resp.Incidents) != 1 {
		t.Errorf("expected 1 incident from YAML, got total=%d, len=%d", resp.Total, len(resp.Incidents))
	}
	if resp.Incidents[0].ID != "inc-test-01" {
		t.Errorf("expected incident ID inc-test-01, got %s", resp.Incidents[0].ID)
	}
}

func TestCmd_Incident_List_Filters(t *testing.T) {
	resetIncidentFlags()
	ts := setupIncidentMockServer(t)
	defer ts.Close()

	incServerURL = ts.URL
	incStatuses = []string{"investigating", "detected"}
	incSeverities = []string{"critical", "warning"}
	incScopes = []string{"multi_node"}
	incNodeID = "node-01"
	incSearch = "CPU"
	incLimit = 10
	incOffset = 0
	incSortBy = "start_time"
	incSortOrder = "desc"

	out, err := captureStdout(func() error {
		return runIncidentList(incidentListCmd, nil)
	})
	if err != nil {
		t.Fatalf("runIncidentList with filters failed: %v", err)
	}

	if !strings.Contains(out, "inc-test-01") {
		t.Errorf("expected filtered output to contain inc-test-01, got: %s", out)
	}
}

func TestCmd_Incident_Get_HumanReadable(t *testing.T) {
	resetIncidentFlags()
	ts := setupIncidentMockServer(t)
	defer ts.Close()

	incServerURL = ts.URL
	out, err := captureStdout(func() error {
		return runIncidentGet(incidentGetCmd, []string{"inc-test-01"})
	})
	if err != nil {
		t.Fatalf("runIncidentGet failed: %v", err)
	}

	if !strings.Contains(out, "Incident Dossier: inc-test-01") {
		t.Errorf("expected incident dossier header, got: %s", out)
	}
	if !strings.Contains(out, "Correlated High CPU and Memory Pressure") {
		t.Errorf("expected incident title, got: %s", out)
	}
	if !strings.Contains(out, "88.5/100.0") {
		t.Errorf("expected composite severity score 88.5, got: %s", out)
	}
	if !strings.Contains(out, "high_cpu_utilization") {
		t.Errorf("expected primary symptom high_cpu_utilization, got: %s", out)
	}
	if !strings.Contains(out, "Active Critical Alerts") {
		t.Errorf("expected severity factor breakdown, got: %s", out)
	}
	if !strings.Contains(out, "alert_cpu_throttling") {
		t.Errorf("expected root signals in table, got: %s", out)
	}
	if !strings.Contains(out, "cluster=prod-us-east") {
		t.Errorf("expected tags in output, got: %s", out)
	}
}

func TestCmd_Incident_Get_JSON_YAML(t *testing.T) {
	resetIncidentFlags()
	ts := setupIncidentMockServer(t)
	defer ts.Close()

	incServerURL = ts.URL

	// JSON format
	incFormat = "json"
	outJSON, err := captureStdout(func() error {
		return runIncidentGet(incidentGetCmd, []string{"inc-test-01"})
	})
	if err != nil {
		t.Fatalf("runIncidentGet JSON failed: %v", err)
	}
	var incJSON incidents.Incident
	if err := json.Unmarshal([]byte(outJSON), &incJSON); err != nil {
		t.Fatalf("failed to decode JSON get incident: %v", err)
	}
	if incJSON.ID != "inc-test-01" || incJSON.SeverityScore != 88.5 {
		t.Errorf("unexpected incident decoded from JSON: %+v", incJSON)
	}

	// YAML format
	incFormat = "yaml"
	outYAML, err := captureStdout(func() error {
		return runIncidentGet(incidentGetCmd, []string{"inc-test-01"})
	})
	if err != nil {
		t.Fatalf("runIncidentGet YAML failed: %v", err)
	}
	var incYAML incidents.Incident
	if err := yaml.Unmarshal([]byte(outYAML), &incYAML); err != nil {
		t.Fatalf("failed to decode YAML get incident: %v", err)
	}
	if incYAML.ID != "inc-test-01" {
		t.Errorf("unexpected incident decoded from YAML: %+v", incYAML)
	}
}

func TestCmd_Incident_Get_NotFound(t *testing.T) {
	resetIncidentFlags()
	ts := setupIncidentMockServer(t)
	defer ts.Close()

	incServerURL = ts.URL
	_, err := captureStdout(func() error {
		return runIncidentGet(incidentGetCmd, []string{"inc-nonexistent"})
	})
	if err == nil {
		t.Fatalf("expected error when fetching nonexistent incident, got nil")
	}
}

func TestCmd_Incident_Summary(t *testing.T) {
	resetIncidentFlags()
	ts := setupIncidentMockServer(t)
	defer ts.Close()

	incServerURL = ts.URL

	// Text format
	out, err := captureStdout(func() error {
		return runIncidentSummary(incidentSummaryCmd, nil)
	})
	if err != nil {
		t.Fatalf("runIncidentSummary failed: %v", err)
	}

	if !strings.Contains(out, "Watchdog Cluster Incident Summary") {
		t.Errorf("expected summary header, got: %s", out)
	}
	if !strings.Contains(out, "Total Incidents Recorded: 10") {
		t.Errorf("expected total incidents count 10, got: %s", out)
	}
	if !strings.Contains(out, "Critical : 2") {
		t.Errorf("expected severity distribution Critical : 2, got: %s", out)
	}
	if !strings.Contains(out, "node-01") {
		t.Errorf("expected top affected node node-01, got: %s", out)
	}

	// JSON format
	incFormat = "json"
	outJSON, err := captureStdout(func() error {
		return runIncidentSummary(incidentSummaryCmd, nil)
	})
	if err != nil {
		t.Fatalf("runIncidentSummary JSON failed: %v", err)
	}
	var sum incidents.IncidentSummary
	if err := json.Unmarshal([]byte(outJSON), &sum); err != nil {
		t.Fatalf("failed to decode JSON summary: %v", err)
	}
	if sum.TotalCount != 10 || sum.CriticalCount != 2 {
		t.Errorf("unexpected summary decoded: %+v", sum)
	}
}

func TestCmd_Incident_Timeline(t *testing.T) {
	resetIncidentFlags()
	ts := setupIncidentMockServer(t)
	defer ts.Close()

	incServerURL = ts.URL

	// Text format
	out, err := captureStdout(func() error {
		return runIncidentTimeline(incidentTimelineCmd, []string{"inc-test-01"})
	})
	if err != nil {
		t.Fatalf("runIncidentTimeline failed: %v", err)
	}

	if !strings.Contains(out, "Chronological Event Timeline for Incident: inc-test-01") {
		t.Errorf("expected timeline header, got: %s", out)
	}
	if !strings.Contains(out, "CPU Throttling Alert Fired") {
		t.Errorf("expected timeline event title, got: %s", out)
	}
	if !strings.Contains(out, "Total Timeline Events: 2") {
		t.Errorf("expected total events count 2, got: %s", out)
	}

	// JSON format
	incFormat = "json"
	outJSON, err := captureStdout(func() error {
		return runIncidentTimeline(incidentTimelineCmd, []string{"inc-test-01"})
	})
	if err != nil {
		t.Fatalf("runIncidentTimeline JSON failed: %v", err)
	}
	var resp incidents.TimelineResponse
	if err := json.Unmarshal([]byte(outJSON), &resp); err != nil {
		t.Fatalf("failed to decode timeline JSON: %v", err)
	}
	if resp.Count != 2 || len(resp.Timeline) != 2 {
		t.Errorf("unexpected timeline response decoded: %+v", resp)
	}
}

func TestCmd_Incident_Related(t *testing.T) {
	resetIncidentFlags()
	ts := setupIncidentMockServer(t)
	defer ts.Close()

	incServerURL = ts.URL

	// Text format
	out, err := captureStdout(func() error {
		return runIncidentRelated(incidentRelatedCmd, []string{"inc-test-01"})
	})
	if err != nil {
		t.Fatalf("runIncidentRelated failed: %v", err)
	}

	if !strings.Contains(out, "Correlated Context & Signals for Incident: inc-test-01") {
		t.Errorf("expected related header, got: %s", out)
	}
	if !strings.Contains(out, "Statistical Recurrence Pattern:") {
		t.Errorf("expected recurrence pattern section, got: %s", out)
	}
	if !strings.Contains(out, "Periodic recurrence observed every ~24 hours") {
		t.Errorf("expected recurrence pattern summary, got: %s", out)
	}
	if !strings.Contains(out, "Similar Historical Incidents (1):") {
		t.Errorf("expected similar incidents section, got: %s", out)
	}

	// JSON format
	incFormat = "json"
	outJSON, err := captureStdout(func() error {
		return runIncidentRelated(incidentRelatedCmd, []string{"inc-test-01"})
	})
	if err != nil {
		t.Fatalf("runIncidentRelated JSON failed: %v", err)
	}
	var resp incidents.RelatedIncidentsResponse
	if err := json.Unmarshal([]byte(outJSON), &resp); err != nil {
		t.Fatalf("failed to decode related JSON: %v", err)
	}
	if len(resp.Signals) != 2 || resp.Recurrence == nil {
		t.Errorf("unexpected related response: %+v", resp)
	}
}

func TestCmd_Incident_Impact(t *testing.T) {
	resetIncidentFlags()
	ts := setupIncidentMockServer(t)
	defer ts.Close()

	incServerURL = ts.URL

	// Text format
	out, err := captureStdout(func() error {
		return runIncidentImpact(incidentImpactCmd, []string{"inc-test-01"})
	})
	if err != nil {
		t.Fatalf("runIncidentImpact failed: %v", err)
	}

	if !strings.Contains(out, "Blast Radius & Impact Analysis for Incident: inc-test-01") {
		t.Errorf("expected impact header, got: %s", out)
	}
	if !strings.Contains(out, "40.0% Fleet Coverage") {
		t.Errorf("expected fleet coverage 40.0%%, got: %s", out)
	}
	if !strings.Contains(out, "Affected Nodes:") {
		t.Errorf("expected affected nodes table, got: %s", out)
	}
	if !strings.Contains(out, "Impacted Subsystems:") {
		t.Errorf("expected impacted subsystems, got: %s", out)
	}

	// JSON format
	incFormat = "json"
	outJSON, err := captureStdout(func() error {
		return runIncidentImpact(incidentImpactCmd, []string{"inc-test-01"})
	})
	if err != nil {
		t.Fatalf("runIncidentImpact JSON failed: %v", err)
	}
	var impact incidents.ImpactAnalysis
	if err := json.Unmarshal([]byte(outJSON), &impact); err != nil {
		t.Fatalf("failed to decode impact JSON: %v", err)
	}
	if impact.IncidentID != "inc-test-01" || impact.Impact.FleetPercentage != 40.0 {
		t.Errorf("unexpected impact analysis: %+v", impact)
	}
}

func TestCmd_Incident_Findings(t *testing.T) {
	resetIncidentFlags()
	ts := setupIncidentMockServer(t)
	defer ts.Close()

	incServerURL = ts.URL

	// Text format
	out, err := captureStdout(func() error {
		return runIncidentFindings(incidentFindingsCmd, []string{"inc-test-01"})
	})
	if err != nil {
		t.Fatalf("runIncidentFindings failed: %v", err)
	}

	if !strings.Contains(out, "Intelligence Findings & Advisory for Incident: inc-test-01") {
		t.Errorf("expected findings header, got: %s", out)
	}
	if !strings.Contains(out, "Correlated Resource Exhaustion") {
		t.Errorf("expected finding title, got: %s", out)
	}
	if !strings.Contains(out, "Advisory:") {
		t.Errorf("expected advisory recommendations, got: %s", out)
	}

	// JSON format
	incFormat = "json"
	outJSON, err := captureStdout(func() error {
		return runIncidentFindings(incidentFindingsCmd, []string{"inc-test-01"})
	})
	if err != nil {
		t.Fatalf("runIncidentFindings JSON failed: %v", err)
	}
	var resp incidents.FindingsResponse
	if err := json.Unmarshal([]byte(outJSON), &resp); err != nil {
		t.Fatalf("failed to decode findings JSON: %v", err)
	}
	if resp.Count != 1 || len(resp.Findings) != 1 {
		t.Errorf("unexpected findings response: %+v", resp)
	}
}

func TestCmd_Incident_Investigate(t *testing.T) {
	resetIncidentFlags()
	ts := setupIncidentMockServer(t)
	defer ts.Close()

	incServerURL = ts.URL

	// Text format
	out, err := captureStdout(func() error {
		return runIncidentInvestigate(incidentInvestigateCmd, []string{"inc-test-01"})
	})
	if err != nil {
		t.Fatalf("runIncidentInvestigate failed: %v", err)
	}

	if !strings.Contains(out, "Watchdog Comprehensive Incident Investigation Dossier") {
		t.Errorf("expected dossier header, got: %s", out)
	}
	if !strings.Contains(out, "Composite Score: 88.5/100.0") {
		t.Errorf("expected severity score, got: %s", out)
	}
	if !strings.Contains(out, "Blast Radius & Infrastructure Impact:") {
		t.Errorf("expected blast radius section, got: %s", out)
	}
	if !strings.Contains(out, "Statistical Recurrence Analysis:") {
		t.Errorf("expected statistical recurrence section, got: %s", out)
	}
	if !strings.Contains(out, "Historical Similar Incidents (1 matches):") {
		t.Errorf("expected similar incidents matches, got: %s", out)
	}
	if !strings.Contains(out, "Chronological Timeline Highlights (2 events):") {
		t.Errorf("expected timeline highlights, got: %s", out)
	}
	if !strings.Contains(out, "Intelligence Findings & Action Items (1):") {
		t.Errorf("expected findings and action items, got: %s", out)
	}

	// JSON format
	incFormat = "json"
	outJSON, err := captureStdout(func() error {
		return runIncidentInvestigate(incidentInvestigateCmd, []string{"inc-test-01"})
	})
	if err != nil {
		t.Fatalf("runIncidentInvestigate JSON failed: %v", err)
	}
	var report incidents.IncidentInvestigationReport
	if err := json.Unmarshal([]byte(outJSON), &report); err != nil {
		t.Fatalf("failed to decode investigation dossier JSON: %v", err)
	}
	if report.Incident.ID != "inc-test-01" || report.Recurrence == nil {
		t.Errorf("unexpected dossier decoded: %+v", report)
	}
}

func TestCmd_Incident_Status(t *testing.T) {
	resetIncidentFlags()
	ts := setupIncidentMockServer(t)
	defer ts.Close()

	incServerURL = ts.URL

	// Valid status transition
	incReason = "Engineers identified root cause"
	out, err := captureStdout(func() error {
		return runIncidentStatus(incidentStatusCmd, []string{"inc-test-01", "resolved"})
	})
	if err != nil {
		t.Fatalf("runIncidentStatus failed: %v", err)
	}

	if !strings.Contains(out, "Successfully transitioned incident 'inc-test-01' status to 'resolved'") {
		t.Errorf("expected status transition confirmation, got: %s", out)
	}
	if !strings.Contains(out, "Reason: Engineers identified root cause") {
		t.Errorf("expected reason in output, got: %s", out)
	}

	// Invalid status
	_, err = captureStdout(func() error {
		return runIncidentStatus(incidentStatusCmd, []string{"inc-test-01", "invalid_status_xyz"})
	})
	if err == nil {
		t.Fatalf("expected error on invalid status transition, got nil")
	}
}

func TestCmd_Incident_Similar(t *testing.T) {
	resetIncidentFlags()
	ts := setupIncidentMockServer(t)
	defer ts.Close()

	incServerURL = ts.URL
	incMinSimilarity = 0.35
	incLimit = 5

	// Text format
	out, err := captureStdout(func() error {
		return runIncidentSimilar(incidentSimilarCmd, []string{"inc-test-01"})
	})
	if err != nil {
		t.Fatalf("runIncidentSimilar failed: %v", err)
	}

	if !strings.Contains(out, "Multi-Factor Jaccard Similarity Analysis for Incident: inc-test-01") {
		t.Errorf("expected similar header, got: %s", out)
	}
	if !strings.Contains(out, "inc-hist-01") {
		t.Errorf("expected matching incident inc-hist-01, got: %s", out)
	}
	if !strings.Contains(out, "85.0%") {
		t.Errorf("expected similarity score 85.0%%, got: %s", out)
	}
	if !strings.Contains(out, "Total Matching Similar Incidents: 1") {
		t.Errorf("expected count 1, got: %s", out)
	}

	// JSON format
	incFormat = "json"
	outJSON, err := captureStdout(func() error {
		return runIncidentSimilar(incidentSimilarCmd, []string{"inc-test-01"})
	})
	if err != nil {
		t.Fatalf("runIncidentSimilar JSON failed: %v", err)
	}
	var resp incidents.SimilarIncidentsResponse
	if err := json.Unmarshal([]byte(outJSON), &resp); err != nil {
		t.Fatalf("failed to decode similar JSON: %v", err)
	}
	if resp.Count != 1 || len(resp.SimilarIncidents) != 1 {
		t.Errorf("unexpected similar response: %+v", resp)
	}
}

func TestCmd_Incident_MissingServerURL(t *testing.T) {
	resetIncidentFlags()
	globalCfg = nil

	_, err := getIncidentClient()
	if err == nil {
		t.Fatalf("expected error when server URL is not configured, got nil")
	}
	if !strings.Contains(err.Error(), "fleet server URL is required") {
		t.Errorf("expected error message about fleet server URL, got: %v", err)
	}
}

func TestCmd_Incident_ParseTimeOrDurationFlag(t *testing.T) {
	// Empty string
	t0, err := parseTimeOrDurationFlag("")
	if err != nil || !t0.IsZero() {
		t.Errorf("expected zero time for empty string, got: %v, err: %v", t0, err)
	}

	// Relative duration
	tDur, err := parseTimeOrDurationFlag("1h")
	if err != nil {
		t.Errorf("expected duration to parse, got err: %v", err)
	}
	if time.Since(tDur) < 59*time.Minute || time.Since(tDur) > 61*time.Minute {
		t.Errorf("expected ~1h ago, got %v", tDur)
	}

	// RFC3339 timestamp
	tRFC, err := parseTimeOrDurationFlag("2026-09-27T12:00:00Z")
	if err != nil {
		t.Errorf("expected RFC3339 to parse, got err: %v", err)
	}
	if tRFC.Year() != 2026 || tRFC.Month() != 9 || tRFC.Day() != 27 {
		t.Errorf("expected parsed date 2026-09-27, got %v", tRFC)
	}

	// Invalid string
	_, err = parseTimeOrDurationFlag("not-a-date-or-duration")
	if err == nil {
		t.Errorf("expected error parsing invalid string, got nil")
	}
}
