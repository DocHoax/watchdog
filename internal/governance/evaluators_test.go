package governance

import (
	"context"
	"testing"
	"time"

	"github.com/DocHoax/watchdog/internal/incidents"
	"github.com/DocHoax/watchdog/pkg/model"
)

func TestResourceThresholdEvaluator(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	clock := NewMockClock(now)

	evaluator := &ResourceThresholdEvaluator{}

	t.Run("Normal orientation - critical breach", func(t *testing.T) {
		lastTel := now.Add(-30 * time.Second)
		evalCtx := &EvaluationContext{
			Node: &model.FleetNode{
				LastTelemetry: &lastTel,
				Summary: &model.NodeSummary{
					CPUUsagePercent: 95.0,
				},
			},
			Clock:           clock,
			FreshnessConfig: DefaultFreshnessConfig(),
		}

		effective := EffectiveRule{
			Rule: model.PolicyRule{
				ID:      "rule-cpu-crit",
				Name:    "High CPU",
				Type:    model.RuleTypeResourceThreshold,
				Enabled: true,
				ResourceThreshold: &model.ResourceThresholdRuleConfig{
					Metric:            "cpu_usage_pct",
					WarningThreshold:  80.0,
					CriticalThreshold: 90.0,
					Unit:              "%",
				},
			},
			Category:        model.PolicyCategoryResourceThresholds,
			SourcePolicyID:  "pol-cpu",
			SourceRevision:  1,
			EnforcementMode: model.EnforcementModeEnforce,
		}

		res, err := evaluator.Evaluate(ctx, evalCtx, effective)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Status != model.EvaluationStatusNonCompliant {
			t.Errorf("expected NonCompliant status, got %s", res.Status)
		}
		if res.Severity != model.SeverityCritical {
			t.Errorf("expected Critical severity, got %s", res.Severity)
		}
		if res.DataFreshness != model.DataFreshnessFresh {
			t.Errorf("expected Fresh data, got %s", res.DataFreshness)
		}
	})

	t.Run("Normal orientation - warning breach", func(t *testing.T) {
		lastTel := now.Add(-30 * time.Second)
		evalCtx := &EvaluationContext{
			Node: &model.FleetNode{
				LastTelemetry: &lastTel,
				Summary: &model.NodeSummary{
					CPUUsagePercent: 85.0,
				},
			},
			Clock:           clock,
			FreshnessConfig: DefaultFreshnessConfig(),
		}

		effective := EffectiveRule{
			Rule: model.PolicyRule{
				ID:      "rule-cpu-warn",
				Name:    "Moderate CPU",
				Type:    model.RuleTypeResourceThreshold,
				Enabled: true,
				ResourceThreshold: &model.ResourceThresholdRuleConfig{
					Metric:            "cpu_usage_pct",
					WarningThreshold:  80.0,
					CriticalThreshold: 90.0,
				},
			},
			Category:        model.PolicyCategoryResourceThresholds,
			SourcePolicyID:  "pol-cpu",
			SourceRevision:  1,
			EnforcementMode: model.EnforcementModeEnforce,
		}

		res, err := evaluator.Evaluate(ctx, evalCtx, effective)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Status != model.EvaluationStatusWarning {
			t.Errorf("expected Warning status, got %s", res.Status)
		}
	})

	t.Run("Inverted orientation - available disk critical breach", func(t *testing.T) {
		evalCtx := &EvaluationContext{
			LatestTelemetry: &model.TelemetrySubmission{
				Timestamp: now.Add(-10 * time.Second),
				Metrics: map[string]float64{
					"disk_available_pct": 5.0,
				},
			},
			Clock:           clock,
			FreshnessConfig: DefaultFreshnessConfig(),
		}

		effective := EffectiveRule{
			Rule: model.PolicyRule{
				ID:      "rule-disk-avail",
				Type:    model.RuleTypeResourceThreshold,
				Enabled: true,
				ResourceThreshold: &model.ResourceThresholdRuleConfig{
					Metric:            "disk_available_pct",
					WarningThreshold:  20.0,
					CriticalThreshold: 10.0,
				},
			},
			Category:       model.PolicyCategoryResourceThresholds,
			SourcePolicyID: "pol-disk",
			SourceRevision: 1,
		}

		res, err := evaluator.Evaluate(ctx, evalCtx, effective)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Status != model.EvaluationStatusNonCompliant {
			t.Errorf("expected NonCompliant on low available disk, got %s", res.Status)
		}
	})

	t.Run("Metric missing - insufficient data", func(t *testing.T) {
		evalCtx := &EvaluationContext{
			Clock:           clock,
			FreshnessConfig: DefaultFreshnessConfig(),
		}

		effective := EffectiveRule{
			Rule: model.PolicyRule{
				ID:      "rule-unknown",
				Type:    model.RuleTypeResourceThreshold,
				Enabled: true,
				ResourceThreshold: &model.ResourceThresholdRuleConfig{
					Metric:            "non_existent_metric",
					WarningThreshold:  50.0,
					CriticalThreshold: 80.0,
				},
			},
			Category:       model.PolicyCategoryResourceThresholds,
			SourcePolicyID: "pol-res",
		}

		res, err := evaluator.Evaluate(ctx, evalCtx, effective)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Status != model.EvaluationStatusInsufficientData {
			t.Errorf("expected InsufficientData status, got %s", res.Status)
		}
	})
}

func TestAnomalyDetectionEvaluator(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 10, 1, 14, 0, 0, 0, time.UTC) // 14:00 UTC
	clock := NewMockClock(now)

	evaluator := &AnomalyDetectionEvaluator{}

	t.Run("Anomaly detected on spike", func(t *testing.T) {
		evalCtx := &EvaluationContext{
			LatestTelemetry: &model.TelemetrySubmission{
				Timestamp: now,
				Metrics:   map[string]float64{"request_latency_ms": 350.0},
			},
			TelemetryHistory: []model.TelemetrySubmission{
				{Timestamp: now.Add(-30 * time.Minute), Metrics: map[string]float64{"request_latency_ms": 50.0}},
				{Timestamp: now.Add(-20 * time.Minute), Metrics: map[string]float64{"request_latency_ms": 52.0}},
				{Timestamp: now.Add(-10 * time.Minute), Metrics: map[string]float64{"request_latency_ms": 48.0}},
				{Timestamp: now.Add(-5 * time.Minute), Metrics: map[string]float64{"request_latency_ms": 51.0}},
				{Timestamp: now, Metrics: map[string]float64{"request_latency_ms": 350.0}},
			},
			Clock:           clock,
			FreshnessConfig: DefaultFreshnessConfig(),
		}

		effective := EffectiveRule{
			Rule: model.PolicyRule{
				ID:      "rule-latency-anomaly",
				Type:    model.RuleTypeAnomalyDetection,
				Enabled: true,
				AnomalyDetection: &model.AnomalyDetectionRuleConfig{
					Metric:          "request_latency_ms",
					ZScoreThreshold: 2.0,
				},
			},
			Category:       model.PolicyCategoryAnomalyDetection,
			SourcePolicyID: "pol-anomaly",
			SourceRevision: 1,
		}

		res, err := evaluator.Evaluate(ctx, evalCtx, effective)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Status != model.EvaluationStatusNonCompliant && res.Status != model.EvaluationStatusWarning {
			t.Errorf("expected anomaly detection breach, got %s", res.Status)
		}
	})

	t.Run("Excluded maintenance hour skipped", func(t *testing.T) {
		evalCtx := &EvaluationContext{
			Clock:           clock, // 14:00 UTC
			FreshnessConfig: DefaultFreshnessConfig(),
		}

		effective := EffectiveRule{
			Rule: model.PolicyRule{
				ID:      "rule-offpeak-anomaly",
				Type:    model.RuleTypeAnomalyDetection,
				Enabled: true,
				AnomalyDetection: &model.AnomalyDetectionRuleConfig{
					Metric:        "request_latency_ms",
					ExcludedHours: []int{14, 15}, // Exclude current hour
				},
			},
			Category:       model.PolicyCategoryAnomalyDetection,
			SourcePolicyID: "pol-anomaly",
		}

		res, err := evaluator.Evaluate(ctx, evalCtx, effective)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Status != model.EvaluationStatusNotApplicable {
			t.Errorf("expected NotApplicable status during excluded hour, got %s", res.Status)
		}
	})
}

func TestCapacityPlanningEvaluator(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	clock := NewMockClock(now)

	evaluator := &CapacityPlanningEvaluator{}

	t.Run("Rapid disk exhaustion warning", func(t *testing.T) {
		// Disk growing 5% every 2 days -> 2.5% per day.
		// Current: 85%. Headroom = 15%. Days to 100% = 15 / 2.5 = 6 days.
		evalCtx := &EvaluationContext{
			TelemetryHistory: []model.TelemetrySubmission{
				{
					Timestamp: now.Add(-6 * 24 * time.Hour),
					Snapshot:  &model.SystemSnapshot{Disk: &model.DiskInfo{UsedPercent: 70.0}},
				},
				{
					Timestamp: now.Add(-4 * 24 * time.Hour),
					Snapshot:  &model.SystemSnapshot{Disk: &model.DiskInfo{UsedPercent: 75.0}},
				},
				{
					Timestamp: now.Add(-2 * 24 * time.Hour),
					Snapshot:  &model.SystemSnapshot{Disk: &model.DiskInfo{UsedPercent: 80.0}},
				},
				{
					Timestamp: now,
					Snapshot:  &model.SystemSnapshot{Disk: &model.DiskInfo{UsedPercent: 85.0}},
				},
			},
			Clock:           clock,
			FreshnessConfig: DefaultFreshnessConfig(),
		}

		effective := EffectiveRule{
			Rule: model.PolicyRule{
				ID:      "rule-cap-disk",
				Type:    model.RuleTypeCapacityPlanning,
				Enabled: true,
				CapacityPlanning: &model.CapacityPlanningRuleConfig{
					Metric:                   "disk_used_pct",
					HorizonDays:              90,
					WarningDaysToExhaustion:  14,
					CriticalDaysToExhaustion: 7,
				},
			},
			Category:       model.PolicyCategoryCapacityPlanning,
			SourcePolicyID: "pol-cap",
			SourceRevision: 1,
		}

		res, err := evaluator.Evaluate(ctx, evalCtx, effective)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Status != model.EvaluationStatusNonCompliant {
			t.Errorf("expected critical runway breach (6 days <= 7 days), got %s", res.Status)
		}
	})
}

func TestIncidentSeverityEvaluator(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	clock := NewMockClock(now)

	evaluator := &IncidentSeverityEvaluator{}

	t.Run("Active critical incident breach", func(t *testing.T) {
		evalCtx := &EvaluationContext{
			ActiveIncidents: []incidents.Incident{
				{
					ID:        "inc-100",
					Title:     "Kernel Panic Warning",
					Severity:  model.SeverityCritical,
					Status:    incidents.IncidentStatusInvestigating,
					StartTime: now.Add(-1 * time.Hour),
				},
			},
			Clock:           clock,
			FreshnessConfig: DefaultFreshnessConfig(),
		}

		effective := EffectiveRule{
			Rule: model.PolicyRule{
				ID:      "rule-inc-crit",
				Type:    model.RuleTypeIncidentSeverity,
				Enabled: true,
				IncidentSeverity: &model.IncidentSeverityRuleConfig{
					Condition: "severity == CRITICAL",
				},
			},
			Category:       model.PolicyCategoryIncidentSeverity,
			SourcePolicyID: "pol-inc",
			SourceRevision: 1,
		}

		res, err := evaluator.Evaluate(ctx, evalCtx, effective)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Status != model.EvaluationStatusNonCompliant {
			t.Errorf("expected NonCompliant on active critical incident, got %s", res.Status)
		}
	})
}

func TestOperationalComplianceEvaluator(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	clock := NewMockClock(now)

	evaluator := &OperationalComplianceEvaluator{}

	t.Run("Heartbeat freshness check pass", func(t *testing.T) {
		evalCtx := &EvaluationContext{
			Node: &model.FleetNode{
				LastHeartbeat: now.Add(-45 * time.Second),
			},
			Clock:           clock,
			FreshnessConfig: DefaultFreshnessConfig(),
		}

		effective := EffectiveRule{
			Rule: model.PolicyRule{
				ID:      "rule-heartbeat",
				Type:    model.RuleTypeOperationalCompliance,
				Enabled: true,
				OperationalCompliance: &model.OperationalComplianceRuleConfig{
					CheckType:     "heartbeat_freshness",
					MaxAgeSeconds: 120,
				},
			},
			Category:       model.PolicyCategoryOperationalCompliance,
			SourcePolicyID: "pol-sec",
		}

		res, err := evaluator.Evaluate(ctx, evalCtx, effective)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Status != model.EvaluationStatusCompliant {
			t.Errorf("expected Compliant for recent heartbeat, got %s", res.Status)
		}
	})

	t.Run("Mandatory tags check missing tag", func(t *testing.T) {
		evalCtx := &EvaluationContext{
			Node: &model.FleetNode{
				Identity: model.NodeIdentity{
					Tags: map[string]string{
						"env": "production",
					},
				},
			},
			Clock:           clock,
			FreshnessConfig: DefaultFreshnessConfig(),
		}

		effective := EffectiveRule{
			Rule: model.PolicyRule{
				ID:      "rule-tags",
				Type:    model.RuleTypeOperationalCompliance,
				Enabled: true,
				OperationalCompliance: &model.OperationalComplianceRuleConfig{
					CheckType:      "mandatory_tags",
					ExpectedValues: []string{"env", "owner", "tier"},
				},
			},
			Category:       model.PolicyCategoryOperationalCompliance,
			SourcePolicyID: "pol-gov",
		}

		res, err := evaluator.Evaluate(ctx, evalCtx, effective)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Status != model.EvaluationStatusNonCompliant {
			t.Errorf("expected NonCompliant on missing mandatory tags, got %s", res.Status)
		}
	})

	t.Run("Collector version constraint", func(t *testing.T) {
		evalCtx := &EvaluationContext{
			Node: &model.FleetNode{
				Identity: model.NodeIdentity{
					Version: "1.4.2",
				},
			},
			Clock:           clock,
			FreshnessConfig: DefaultFreshnessConfig(),
		}

		effective := EffectiveRule{
			Rule: model.PolicyRule{
				ID:      "rule-version",
				Type:    model.RuleTypeOperationalCompliance,
				Enabled: true,
				OperationalCompliance: &model.OperationalComplianceRuleConfig{
					CheckType:     "collector_version",
					ExpectedValue: ">= 1.4.0",
				},
			},
			Category:       model.PolicyCategoryOperationalCompliance,
			SourcePolicyID: "pol-sec",
		}

		res, err := evaluator.Evaluate(ctx, evalCtx, effective)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Status != model.EvaluationStatusCompliant {
			t.Errorf("expected Compliant for version 1.4.2 >= 1.4.0, got %s", res.Status)
		}
	})
}
