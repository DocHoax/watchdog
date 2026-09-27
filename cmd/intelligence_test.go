package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DocHoax/watchdog/internal/config"
	"github.com/DocHoax/watchdog/internal/intelligence"
	"github.com/DocHoax/watchdog/pkg/model"
)

func setupIntelligenceMockServer(t *testing.T) *httptest.Server {
	t.Helper()
	now := time.Now().UTC()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/api/v1/intelligence/fleet" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(intelligence.FleetHealthSummary{
				EvaluatedAt:   now,
				TotalNodes:    2,
				HealthyCount:  1,
				WarningCount:  1,
				CriticalCount: 0,
				AverageScore:  85.5,
				LowestScoringNodes: []intelligence.NodeHealthSummary{
					{
						NodeID:   "node-01",
						Hostname: "srv-01",
						Status:   model.NodeStatusHealthy,
						HealthScore: intelligence.HealthScore{
							Score:           92.0,
							Trajectory:      intelligence.TrajectoryStable,
							PrimaryConcerns: []string{"Low memory buffer"},
						},
					},
					{
						NodeID:   "node-02",
						Hostname: "srv-02",
						Status:   model.NodeStatusWarning,
						HealthScore: intelligence.HealthScore{
							Score:           78.0,
							Trajectory:      intelligence.TrajectoryDegrading,
							PrimaryConcerns: []string{"Elevated CPU usage"},
						},
					},
				},
				FleetFindings: []intelligence.IntelligenceFinding{
					{
						ID:            "find-001",
						Category:      intelligence.FindingCategoryFleetPattern,
						Severity:      model.SeverityWarning,
						Confidence:    intelligence.FindingConfidenceHigh,
						Title:         "Concurrent CPU Pressure",
						Description:   "Multiple nodes experiencing concurrent CPU spikes",
						AffectedNodes: []string{"node-01", "node-02"},
						DetectedAt:    now,
					},
				},
			})

		case r.URL.Path == "/api/v1/intelligence/nodes/node-01" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(intelligence.NodeHealthSummary{
				NodeID:   "node-01",
				Hostname: "srv-01",
				Status:   model.NodeStatusHealthy,
				HealthScore: intelligence.HealthScore{
					Score:      92.0,
					Trajectory: intelligence.TrajectoryStable,
					Breakdown: []intelligence.FactorContribution{
						{
							Name:        "CPU Aggregate Usage",
							Category:    "cpu",
							Weight:      25.0,
							Deduction:   5.0,
							Impact:      "negative",
							Explanation: "CPU usage at 65%",
						},
					},
					PrimaryConcerns: []string{"Elevated CPU load"},
				},
				ActiveIncidents: []intelligence.Incident{
					{
						ID:        "inc-01",
						Title:     "CPU and Memory Spikes",
						Severity:  model.SeverityWarning,
						Status:    intelligence.IncidentStatusOpen,
						StartTime: now.Add(-10 * time.Minute),
					},
				},
				Findings: []intelligence.IntelligenceFinding{
					{
						ID:                     "find-node-01",
						Title:                  "Elevated Process Thread Count",
						Severity:               model.SeverityWarning,
						Confidence:             intelligence.FindingConfidenceHigh,
						Description:            "Thread count growing steadily",
						NonInvasiveSuggestions: []string{"Check thread leaks"},
					},
				},
				EvaluatedAt: now,
			})

		case r.URL.Path == "/api/v1/intelligence/nodes/node-01/trends" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode([]intelligence.HealthTrend{
				{
					Metric:       "cpu_usage_pct",
					Direction:    intelligence.TrendDirectionIncreasing,
					RateOfChange: 2.5,
					Unit:         "%/min",
					StartValue:   20.0,
					EndValue:     45.0,
					Confidence:   0.95,
				},
			})

		case r.URL.Path == "/api/v1/intelligence/nodes/node-01/baselines" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode([]intelligence.HistoricalBaseline{
				{
					Metric:      "cpu_usage_pct",
					SampleCount: 100,
					Min:         10.0,
					Max:         90.0,
					Mean:        45.0,
					StdDev:      12.5,
					P50:         42.0,
					P90:         75.0,
					P95:         82.0,
					P99:         88.0,
				},
			})

		case r.URL.Path == "/api/v1/intelligence/incidents" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode([]intelligence.Incident{
				{
					ID:            "inc-01",
					Title:         "Fleet-Wide Memory Contention",
					Severity:      model.SeverityCritical,
					Status:        intelligence.IncidentStatusOpen,
					StartTime:     now.Add(-20 * time.Minute),
					AffectedNodes: []string{"node-01", "node-02"},
				},
			})

		case r.URL.Path == "/api/v1/intelligence/incidents/inc-01" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(intelligence.Incident{
				ID:              "inc-01",
				Title:           "Fleet-Wide Memory Contention",
				Severity:        model.SeverityCritical,
				Status:          intelligence.IncidentStatusOpen,
				StartTime:       now.Add(-20 * time.Minute),
				AffectedNodes:   []string{"node-01", "node-02"},
				PrimarySymptoms: []string{"High memory pressure", "Active swap paging"},
				Timeline: []intelligence.IncidentTimelineEvent{
					{
						Timestamp:   now.Add(-20 * time.Minute),
						NodeID:      "node-01",
						EventType:   "alert_fired",
						Severity:    model.SeverityCritical,
						Description: "Memory usage reached 96%",
					},
				},
			})

		case r.URL.Path == "/api/v1/intelligence/correlations" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode([]intelligence.Correlation{
				{
					PrimarySignal:     "cpu_usage_pct",
					SecondarySignal:   "network_tx_bytes",
					Coefficient:       0.92,
					TimeOffsetSeconds: 0,
					CoOccurrenceCount: 30,
					Confidence:        intelligence.CorrelationConfidenceHigh,
					Description:       "Strong positive correlation between CPU and Network TX",
				},
			})

		case r.URL.Path == "/api/v1/intelligence/findings" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode([]intelligence.IntelligenceFinding{
				{
					ID:                     "find-001",
					Category:               intelligence.FindingCategoryFleetPattern,
					Severity:               model.SeverityWarning,
					Confidence:             intelligence.FindingConfidenceHigh,
					Title:                  "Concurrent CPU Pressure",
					Description:            "Multiple nodes experiencing concurrent CPU spikes",
					AffectedNodes:          []string{"node-01", "node-02"},
					SupportingEvidence:     []string{"2 nodes with CPU deduction >= 8.0"},
					NonInvasiveSuggestions: []string{"Inspect batch jobs"},
					DetectedAt:             now,
				},
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

	return ts
}

func TestCmd_Intelligence_Fleet(t *testing.T) {
	ts := setupIntelligenceMockServer(t)
	defer ts.Close()

	intelServerURL = ts.URL
	intelToken = "test-token"
	intelTokenFile = ""
	intelTokenEnv = ""
	intelInsecureTLS = false
	intelTimeout = 5 * time.Second
	globalCfg = config.DefaultConfig()

	// 1. Human readable
	intelFormat = "text"
	out, err := captureStdout(func() error {
		return runIntelligenceFleet(intelligenceFleetCmd, nil)
	})
	if err != nil {
		t.Fatalf("runIntelligenceFleet returned error: %v", err)
	}
	if !strings.Contains(out, "Watchdog Fleet Intelligence Summary") || !strings.Contains(out, "Fleet Health Score: 85.5") {
		t.Errorf("Unexpected fleet intelligence output: %s", out)
	}
	if !strings.Contains(out, "node-01") || !strings.Contains(out, "Concurrent CPU Pressure") {
		t.Errorf("Expected node-01 and finding title in output: %s", out)
	}

	// 2. JSON format
	intelFormat = "json"
	outJSON, err := captureStdout(func() error {
		return runIntelligenceFleet(intelligenceFleetCmd, nil)
	})
	if err != nil {
		t.Fatalf("runIntelligenceFleet JSON returned error: %v", err)
	}
	var sum intelligence.FleetHealthSummary
	if err := json.Unmarshal([]byte(outJSON), &sum); err != nil {
		t.Fatalf("Failed to parse JSON fleet summary: %v", err)
	}
	if sum.TotalNodes != 2 || sum.AverageScore != 85.5 {
		t.Errorf("Unexpected parsed fleet summary: %+v", sum)
	}

	// 3. YAML format
	intelFormat = "yaml"
	outYAML, err := captureStdout(func() error {
		return runIntelligenceFleet(intelligenceFleetCmd, nil)
	})
	if err != nil {
		t.Fatalf("runIntelligenceFleet YAML returned error: %v", err)
	}
	if !strings.Contains(outYAML, "totalnodes: 2") && !strings.Contains(outYAML, "total_nodes: 2") {
		t.Errorf("Expected YAML output to contain node count: %s", outYAML)
	}
}

func TestCmd_Intelligence_Node(t *testing.T) {
	ts := setupIntelligenceMockServer(t)
	defer ts.Close()

	intelServerURL = ts.URL
	intelToken = "test-token"
	intelTimeout = 5 * time.Second
	globalCfg = config.DefaultConfig()

	// 1. Human readable
	intelFormat = "text"
	out, err := captureStdout(func() error {
		return runIntelligenceNode(intelligenceNodeCmd, []string{"node-01"})
	})
	if err != nil {
		t.Fatalf("runIntelligenceNode returned error: %v", err)
	}
	if !strings.Contains(out, "Node Health Summary: srv-01 (node-01)") || !strings.Contains(out, "Health Score:    92.0") {
		t.Errorf("Unexpected node output: %s", out)
	}
	if !strings.Contains(out, "CPU Aggregate Usage") || !strings.Contains(out, "Elevated Process Thread Count") {
		t.Errorf("Expected factor and finding in output: %s", out)
	}

	// 2. JSON format
	intelFormat = "json"
	outJSON, err := captureStdout(func() error {
		return runIntelligenceNode(intelligenceNodeCmd, []string{"node-01"})
	})
	if err != nil {
		t.Fatalf("runIntelligenceNode JSON returned error: %v", err)
	}
	var nodeSum intelligence.NodeHealthSummary
	if err := json.Unmarshal([]byte(outJSON), &nodeSum); err != nil {
		t.Fatalf("Failed to parse JSON node summary: %v", err)
	}
	if nodeSum.NodeID != "node-01" || nodeSum.HealthScore.Score != 92.0 {
		t.Errorf("Unexpected parsed node summary: %+v", nodeSum)
	}

	// 3. 404 Node Not Found
	intelFormat = "text"
	err404 := runIntelligenceNode(intelligenceNodeCmd, []string{"non-existent"})
	if err404 == nil {
		t.Errorf("Expected error for non-existent node")
	}
}

func TestCmd_Intelligence_Incidents(t *testing.T) {
	ts := setupIntelligenceMockServer(t)
	defer ts.Close()

	intelServerURL = ts.URL
	intelToken = "test-token"
	intelTimeout = 5 * time.Second
	globalCfg = config.DefaultConfig()

	// 1. List active incidents (human readable)
	intelFormat = "text"
	intelIncidentID = ""
	out, err := captureStdout(func() error {
		return runIntelligenceIncidents(intelligenceIncidentsCmd, nil)
	})
	if err != nil {
		t.Fatalf("runIntelligenceIncidents returned error: %v", err)
	}
	if !strings.Contains(out, "Active Fleet Incidents (1)") || !strings.Contains(out, "Fleet-Wide Memory Contention") {
		t.Errorf("Unexpected incidents output: %s", out)
	}

	// 2. List active incidents (JSON)
	intelFormat = "json"
	outJSON, err := captureStdout(func() error {
		return runIntelligenceIncidents(intelligenceIncidentsCmd, nil)
	})
	if err != nil {
		t.Fatalf("runIntelligenceIncidents JSON returned error: %v", err)
	}
	var incList []intelligence.Incident
	if err := json.Unmarshal([]byte(outJSON), &incList); err != nil {
		t.Fatalf("Failed to parse JSON incidents list: %v", err)
	}
	if len(incList) != 1 || incList[0].ID != "inc-01" {
		t.Errorf("Unexpected parsed incidents: %+v", incList)
	}

	// 3. Inspect single incident (human readable)
	intelFormat = "text"
	outSingle, err := captureStdout(func() error {
		return runIntelligenceIncidents(intelligenceIncidentsCmd, []string{"inc-01"})
	})
	if err != nil {
		t.Fatalf("runIntelligenceIncidents single returned error: %v", err)
	}
	if !strings.Contains(outSingle, "Incident: Fleet-Wide Memory Contention") || !strings.Contains(outSingle, "Memory usage reached 96%") {
		t.Errorf("Unexpected single incident output: %s", outSingle)
	}

	// 4. Inspect single incident (JSON)
	intelFormat = "json"
	outSingleJSON, err := captureStdout(func() error {
		return runIntelligenceIncidents(intelligenceIncidentsCmd, []string{"inc-01"})
	})
	if err != nil {
		t.Fatalf("runIntelligenceIncidents single JSON returned error: %v", err)
	}
	var inc intelligence.Incident
	if err := json.Unmarshal([]byte(outSingleJSON), &inc); err != nil {
		t.Fatalf("Failed to parse JSON single incident: %v", err)
	}
	if inc.ID != "inc-01" || len(inc.Timeline) != 1 {
		t.Errorf("Unexpected single incident parsed: %+v", inc)
	}
}

func TestCmd_Intelligence_Trends(t *testing.T) {
	ts := setupIntelligenceMockServer(t)
	defer ts.Close()

	intelServerURL = ts.URL
	intelToken = "test-token"
	intelTimeout = 5 * time.Second
	intelWindow = "1h"
	globalCfg = config.DefaultConfig()

	// 1. Human readable
	intelFormat = "text"
	out, err := captureStdout(func() error {
		return runIntelligenceTrends(intelligenceTrendsCmd, []string{"node-01"})
	})
	if err != nil {
		t.Fatalf("runIntelligenceTrends returned error: %v", err)
	}
	if !strings.Contains(out, "Metric Trends for Node \"node-01\"") || !strings.Contains(out, "cpu_usage_pct") {
		t.Errorf("Unexpected trends output: %s", out)
	}

	// 2. JSON format
	intelFormat = "json"
	outJSON, err := captureStdout(func() error {
		return runIntelligenceTrends(intelligenceTrendsCmd, []string{"node-01"})
	})
	if err != nil {
		t.Fatalf("runIntelligenceTrends JSON returned error: %v", err)
	}
	var trends []intelligence.HealthTrend
	if err := json.Unmarshal([]byte(outJSON), &trends); err != nil {
		t.Fatalf("Failed to parse JSON trends: %v", err)
	}
	if len(trends) != 1 || trends[0].Metric != "cpu_usage_pct" {
		t.Errorf("Unexpected trends parsed: %+v", trends)
	}
}

func TestCmd_Intelligence_Baselines(t *testing.T) {
	ts := setupIntelligenceMockServer(t)
	defer ts.Close()

	intelServerURL = ts.URL
	intelToken = "test-token"
	intelTimeout = 5 * time.Second
	intelWindow = "24h"
	globalCfg = config.DefaultConfig()

	// 1. Human readable
	intelFormat = "text"
	out, err := captureStdout(func() error {
		return runIntelligenceBaselines(intelligenceBaselinesCmd, []string{"node-01"})
	})
	if err != nil {
		t.Fatalf("runIntelligenceBaselines returned error: %v", err)
	}
	if !strings.Contains(out, "Historical Baselines for Node \"node-01\"") || !strings.Contains(out, "cpu_usage_pct") {
		t.Errorf("Unexpected baselines output: %s", out)
	}

	// 2. JSON format
	intelFormat = "json"
	outJSON, err := captureStdout(func() error {
		return runIntelligenceBaselines(intelligenceBaselinesCmd, []string{"node-01"})
	})
	if err != nil {
		t.Fatalf("runIntelligenceBaselines JSON returned error: %v", err)
	}
	var baselines []intelligence.HistoricalBaseline
	if err := json.Unmarshal([]byte(outJSON), &baselines); err != nil {
		t.Fatalf("Failed to parse JSON baselines: %v", err)
	}
	if len(baselines) != 1 || baselines[0].SampleCount != 100 {
		t.Errorf("Unexpected baselines parsed: %+v", baselines)
	}
}

func TestCmd_Intelligence_Correlations(t *testing.T) {
	ts := setupIntelligenceMockServer(t)
	defer ts.Close()

	intelServerURL = ts.URL
	intelToken = "test-token"
	intelTimeout = 5 * time.Second
	intelWindow = "1h"
	globalCfg = config.DefaultConfig()

	// 1. Human readable
	intelFormat = "text"
	out, err := captureStdout(func() error {
		return runIntelligenceCorrelations(intelligenceCorrelationsCmd, nil)
	})
	if err != nil {
		t.Fatalf("runIntelligenceCorrelations returned error: %v", err)
	}
	if !strings.Contains(out, "Signal Temporal Correlations") || !strings.Contains(out, "cpu_usage_pct") {
		t.Errorf("Unexpected correlations output: %s", out)
	}

	// 2. JSON format
	intelFormat = "json"
	outJSON, err := captureStdout(func() error {
		return runIntelligenceCorrelations(intelligenceCorrelationsCmd, nil)
	})
	if err != nil {
		t.Fatalf("runIntelligenceCorrelations JSON returned error: %v", err)
	}
	var correlations []intelligence.Correlation
	if err := json.Unmarshal([]byte(outJSON), &correlations); err != nil {
		t.Fatalf("Failed to parse JSON correlations: %v", err)
	}
	if len(correlations) != 1 || correlations[0].Coefficient != 0.92 {
		t.Errorf("Unexpected correlations parsed: %+v", correlations)
	}
}

func TestCmd_Intelligence_Findings(t *testing.T) {
	ts := setupIntelligenceMockServer(t)
	defer ts.Close()

	intelServerURL = ts.URL
	intelToken = "test-token"
	intelTimeout = 5 * time.Second
	intelCategory = ""
	intelMinSeverity = ""
	globalCfg = config.DefaultConfig()

	// 1. Human readable
	intelFormat = "text"
	out, err := captureStdout(func() error {
		return runIntelligenceFindings(intelligenceFindingsCmd, nil)
	})
	if err != nil {
		t.Fatalf("runIntelligenceFindings returned error: %v", err)
	}
	if !strings.Contains(out, "Intelligence Findings (1)") || !strings.Contains(out, "Concurrent CPU Pressure") {
		t.Errorf("Unexpected findings output: %s", out)
	}

	// 2. JSON format
	intelFormat = "json"
	outJSON, err := captureStdout(func() error {
		return runIntelligenceFindings(intelligenceFindingsCmd, nil)
	})
	if err != nil {
		t.Fatalf("runIntelligenceFindings JSON returned error: %v", err)
	}
	var findings []intelligence.IntelligenceFinding
	if err := json.Unmarshal([]byte(outJSON), &findings); err != nil {
		t.Fatalf("Failed to parse JSON findings: %v", err)
	}
	if len(findings) != 1 || findings[0].ID != "find-001" {
		t.Errorf("Unexpected findings parsed: %+v", findings)
	}
}

func TestCmd_Intelligence_MissingServerURL(t *testing.T) {
	intelServerURL = ""
	globalCfg = config.DefaultConfig()
	globalCfg.Fleet.ServerURL = ""

	err := runIntelligenceFleet(intelligenceFleetCmd, nil)
	if err == nil {
		t.Errorf("Expected error when intelligence fleet server URL is missing")
	}
	if GetExitCode(err) != ExitConfigError {
		t.Errorf("Expected ExitConfigError, got %d", GetExitCode(err))
	}
}
