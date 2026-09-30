package intelligence

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DocHoax/watchdog/pkg/model"
)

func TestIntelligenceClient_AllMethods(t *testing.T) {
	var receivedAuth string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/api/v1/intelligence/fleet" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(FleetHealthSummary{
				AverageScore:  88.5,
				TotalNodes:    10,
				HealthyCount:  9,
				WarningCount:  1,
			})

		case r.URL.Path == "/api/v1/intelligence/nodes/node-1" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(NodeHealthSummary{
				NodeID: "node-1",
				HealthScore: HealthScore{
					Score:            92.0,
					NormalizedStatus: model.NodeStatusHealthy,
					Trajectory:       TrajectoryStable,
				},
				Status: model.NodeStatusHealthy,
			})

		case r.URL.Path == "/api/v1/intelligence/nodes/node-1/trends" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode([]HealthTrend{
				{
					Metric:       "cpu",
					RateOfChange: 0.05,
					Direction:    TrendDirectionIncreasing,
				},
			})

		case r.URL.Path == "/api/v1/intelligence/nodes/node-1/baselines" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode([]HistoricalBaseline{
				{
					Metric: "cpu",
					Mean:   45.2,
					StdDev: 5.1,
					P95:    55.0,
					P99:    62.0,
				},
			})

		case r.URL.Path == "/api/v1/intelligence/incidents" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode([]Incident{
				{
					ID:       "inc-001",
					Title:    "Memory Leak Detected",
					Severity: model.SeverityWarning,
					Status:   IncidentStatusOpen,
				},
			})

		case r.URL.Path == "/api/v1/intelligence/incidents/inc-001" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(Incident{
				ID:       "inc-001",
				Title:    "Memory Leak Detected",
				Severity: model.SeverityWarning,
				Status:   IncidentStatusOpen,
			})

		case r.URL.Path == "/api/v1/intelligence/correlations" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode([]Correlation{
				{
					PrimarySignal:   "cpu",
					SecondarySignal: "temperature",
					Coefficient:     0.89,
				},
			})

		case r.URL.Path == "/api/v1/intelligence/findings" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode([]IntelligenceFinding{
				{
					ID:       "find-1",
					Category: FindingCategoryAnomalyCluster,
					Severity: model.SeverityWarning,
					Title:    "Unusual Memory Growth",
				},
			})

		case r.URL.Path == "/api/v1/intelligence/nodes/node-1/predictions" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode([]Prediction{
				{
					ID:              "pred-1",
					NodeID:          "node-1",
					Metric:          "memory",
					TargetThreshold: 90.0,
					Confidence:      PredictionConfidenceHigh,
				},
			})

		case r.URL.Path == "/api/v1/intelligence/nodes/node-1/capacity" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(NodeCapacityReport{
				NodeID: "node-1",
				Forecasts: []CapacityForecast{
					{
						Resource:          CapacityResourceMemory,
						CriticalThreshold: 100.0,
						Confidence:        PredictionConfidenceHigh,
					},
				},
			})

		case r.URL.Path == "/api/v1/intelligence/fleet/predictions" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(FleetCapacitySummary{
				TotalNodes:               10,
				NodesApproachingCritical: 1,
			})

		case r.URL.Path == "/api/v1/intelligence/recurrence" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode([]RecurrencePattern{
				{
					ID:              "pat-1",
					EventType:       "disk_pressure",
					OccurrenceCount: 4,
				},
			})

		case r.URL.Path == "/api/v1/intelligence/predictions/pred-1" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(Prediction{
				ID:              "pred-1",
				NodeID:          "node-1",
				Metric:          "memory",
				TargetThreshold: 90.0,
			})

		case r.URL.Path == "/api/v1/intelligence/root-cause/inc-001" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(RootCauseReport{
				IncidentID: "inc-001",
				PrimaryRootCause: &RootCauseCandidate{
					NodeID:     "node-1",
					TotalScore: 0.95,
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
	defer ts.Close()

	client := NewClient(ClientConfig{
		Endpoint: ts.URL,
		Token:    "intel-secret-token",
		Timeout:  3 * time.Second,
	})
	ctx := context.Background()

	if client.Endpoint() != ts.URL {
		t.Errorf("expected endpoint %s, got %s", ts.URL, client.Endpoint())
	}

	// 1. GetFleetHealth
	fleetHealth, err := client.GetFleetHealth(ctx)
	if err != nil {
		t.Fatalf("GetFleetHealth failed: %v", err)
	}
	if fleetHealth.AverageScore != 88.5 || fleetHealth.TotalNodes != 10 {
		t.Errorf("unexpected GetFleetHealth response: %+v", fleetHealth)
	}
	if receivedAuth != "Bearer intel-secret-token" {
		t.Errorf("expected auth Bearer intel-secret-token, got %s", receivedAuth)
	}

	// 2. GetNodeHealth
	nodeHealth, err := client.GetNodeHealth(ctx, "node-1")
	if err != nil {
		t.Fatalf("GetNodeHealth failed: %v", err)
	}
	if nodeHealth.NodeID != "node-1" || nodeHealth.HealthScore.Score != 92.0 {
		t.Errorf("unexpected GetNodeHealth response: %+v", nodeHealth)
	}

	// 3. GetNodeTrends
	trends, err := client.GetNodeTrends(ctx, "node-1", 1*time.Hour)
	if err != nil {
		t.Fatalf("GetNodeTrends failed: %v", err)
	}
	if len(trends) != 1 || trends[0].Metric != "cpu" {
		t.Errorf("unexpected GetNodeTrends response: %+v", trends)
	}

	// 4. GetNodeBaselines
	baselines, err := client.GetNodeBaselines(ctx, "node-1", 24*time.Hour)
	if err != nil {
		t.Fatalf("GetNodeBaselines failed: %v", err)
	}
	if len(baselines) != 1 || baselines[0].Mean != 45.2 {
		t.Errorf("unexpected GetNodeBaselines response: %+v", baselines)
	}

	// 5. GetActiveIncidents
	activeInc, err := client.GetActiveIncidents(ctx)
	if err != nil {
		t.Fatalf("GetActiveIncidents failed: %v", err)
	}
	if len(activeInc) != 1 || activeInc[0].ID != "inc-001" {
		t.Errorf("unexpected GetActiveIncidents response: %+v", activeInc)
	}

	// 6. GetIncident
	inc, err := client.GetIncident(ctx, "inc-001")
	if err != nil {
		t.Fatalf("GetIncident failed: %v", err)
	}
	if inc.ID != "inc-001" {
		t.Errorf("expected incident ID inc-001, got %s", inc.ID)
	}

	// 7. GetCorrelations
	corrs, err := client.GetCorrelations(ctx, 6*time.Hour)
	if err != nil {
		t.Fatalf("GetCorrelations failed: %v", err)
	}
	if len(corrs) != 1 || corrs[0].Coefficient != 0.89 {
		t.Errorf("unexpected GetCorrelations response: %+v", corrs)
	}

	// 8. GetFindings
	findings, err := client.GetFindings(ctx, FindingCategoryAnomalyCluster, model.SeverityWarning)
	if err != nil {
		t.Fatalf("GetFindings failed: %v", err)
	}
	if len(findings) != 1 || findings[0].ID != "find-1" {
		t.Errorf("unexpected GetFindings response: %+v", findings)
	}

	// 9. GetNodePredictions
	preds, err := client.GetNodePredictions(ctx, "node-1", 12*time.Hour)
	if err != nil {
		t.Fatalf("GetNodePredictions failed: %v", err)
	}
	if len(preds) != 1 || preds[0].ID != "pred-1" {
		t.Errorf("unexpected GetNodePredictions response: %+v", preds)
	}

	// 10. GetNodeCapacityForecast
	capacity, err := client.GetNodeCapacityForecast(ctx, "node-1", 24*time.Hour)
	if err != nil {
		t.Fatalf("GetNodeCapacityForecast failed: %v", err)
	}
	if capacity.NodeID != "node-1" || len(capacity.Forecasts) != 1 {
		t.Errorf("unexpected GetNodeCapacityForecast response: %+v", capacity)
	}

	// 11. GetFleetPredictions
	fleetPreds, err := client.GetFleetPredictions(ctx, 24*time.Hour)
	if err != nil {
		t.Fatalf("GetFleetPredictions failed: %v", err)
	}
	if fleetPreds.TotalNodes != 10 || fleetPreds.NodesApproachingCritical != 1 {
		t.Errorf("unexpected GetFleetPredictions response: %+v", fleetPreds)
	}

	// 12. GetRecurringIncidents
	recPatterns, err := client.GetRecurringIncidents(ctx, 7*24*time.Hour)
	if err != nil {
		t.Fatalf("GetRecurringIncidents failed: %v", err)
	}
	if len(recPatterns) != 1 || recPatterns[0].ID != "pat-1" {
		t.Errorf("unexpected GetRecurringIncidents response: %+v", recPatterns)
	}

	// 13. GetPrediction
	singlePred, err := client.GetPrediction(ctx, "pred-1")
	if err != nil {
		t.Fatalf("GetPrediction failed: %v", err)
	}
	if singlePred.ID != "pred-1" {
		t.Errorf("expected prediction ID pred-1, got %s", singlePred.ID)
	}

	// 14. GetRootCauseAnalysis
	rca, err := client.GetRootCauseAnalysis(ctx, "inc-001")
	if err != nil {
		t.Fatalf("GetRootCauseAnalysis failed: %v", err)
	}
	if rca.IncidentID != "inc-001" || rca.PrimaryRootCause == nil {
		t.Errorf("unexpected GetRootCauseAnalysis response: %+v", rca)
	}

	// Validation error checks
	if _, err := client.GetNodeHealth(ctx, ""); err == nil {
		t.Errorf("expected error on empty node ID in GetNodeHealth")
	}
	if _, err := client.GetNodeTrends(ctx, "", 0); err == nil {
		t.Errorf("expected error on empty node ID in GetNodeTrends")
	}
	if _, err := client.GetNodeBaselines(ctx, "", 0); err == nil {
		t.Errorf("expected error on empty node ID in GetNodeBaselines")
	}
	if _, err := client.GetIncident(ctx, ""); err == nil {
		t.Errorf("expected error on empty incident ID in GetIncident")
	}
	if _, err := client.GetNodePredictions(ctx, "", 0); err == nil {
		t.Errorf("expected error on empty node ID in GetNodePredictions")
	}
	if _, err := client.GetNodeCapacityForecast(ctx, "", 0); err == nil {
		t.Errorf("expected error on empty node ID in GetNodeCapacityForecast")
	}
	if _, err := client.GetPrediction(ctx, ""); err == nil {
		t.Errorf("expected error on empty prediction ID in GetPrediction")
	}
	if _, err := client.GetRootCauseAnalysis(ctx, ""); err == nil {
		t.Errorf("expected error on empty incident ID in GetRootCauseAnalysis")
	}

	// 404 error check
	if _, err := client.GetNodeHealth(ctx, "unknown-node"); err == nil {
		t.Errorf("expected error for unknown node")
	}
}
