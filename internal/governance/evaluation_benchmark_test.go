package governance

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/DocHoax/watchdog/internal/storage"
	"github.com/DocHoax/watchdog/pkg/model"
)

func BenchmarkEvaluationEngine_1000Rules(b *testing.B) {
	ctx := context.Background()
	clock := NewMockClock(time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC))
	registry := NewEvaluatorRegistry()

	// Construct an EvaluationContext with 1,000 rules
	rules := make([]EffectiveRule, 1000)
	for i := 0; i < 1000; i++ {
		metricName := fmt.Sprintf("metric_%d", i%10)
		rules[i] = EffectiveRule{
			Rule: model.PolicyRule{
				ID:       fmt.Sprintf("rule-%04d", i),
				Name:     fmt.Sprintf("Rule %04d", i),
				Type:     model.RuleTypeResourceThreshold,
				Severity: model.SeverityWarning,
				Enabled:  true,
				ResourceThreshold: &model.ResourceThresholdRuleConfig{
					Metric:            metricName,
					WarningThreshold:  80.0,
					CriticalThreshold: 90.0,
				},
			},
			Category:        model.PolicyCategoryResourceThresholds,
			SourcePolicyID:  fmt.Sprintf("pol-%d", i/50),
			SourceRevision:  1,
			Priority:        100,
			HierarchyLevel:  0,
		}
	}

	metrics := make(map[string]float64)
	for i := 0; i < 10; i++ {
		metrics[fmt.Sprintf("metric_%d", i)] = 75.0 + float64(i)
	}

	evalCtx := &EvaluationContext{
		OrgID: "org-bench",
		Node: &model.FleetNode{
			Identity: model.NodeIdentity{
				NodeID:   "node-bench-01",
				Hostname: "bench-01.corp",
			},
			Status:        model.NodeStatusHealthy,
			LastHeartbeat: clock.Now(),
		},
		ResolvedPolicySet: &ResolvedPolicySet{
			NodeID:         "node-bench-01",
			OrgID:          "org-bench",
			ResolvedAt:     clock.Now(),
			EffectiveRules: rules,
		},
		LatestTelemetry: &model.TelemetrySubmission{
			NodeID:    "node-bench-01",
			Timestamp: clock.Now(),
			Metrics:   metrics,
		},
		Clock:           clock,
		FreshnessConfig: DefaultFreshnessConfig(),
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		results, err := registry.EvaluateContext(ctx, evalCtx)
		if err != nil {
			b.Fatalf("evaluation failed: %v", err)
		}
		if len(results) != 1000 {
			b.Fatalf("expected 1000 results, got %d", len(results))
		}
	}
}

func BenchmarkSimulationEngine_100Nodes(b *testing.B) {
	ctx := context.Background()
	store, err := storage.NewSQLiteStorage(storage.Config{Path: ":memory:"})
	if err != nil {
		b.Fatalf("failed to create sqlite store: %v", err)
	}
	defer store.Close()

	clock := NewMockClock(time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC))
	registry := NewEvaluatorRegistry()
	engine := NewSimulationEngine(store, registry, clock)

	orgID := "org-sim-bench"

	// Create 100 nodes with telemetry
	for i := 0; i < 100; i++ {
		nodeID := fmt.Sprintf("node-sim-%03d", i)
		node := &model.FleetNode{
			Identity: model.NodeIdentity{
				NodeID:   nodeID,
				Hostname: fmt.Sprintf("sim-%03d.internal", i),
				Platform: "linux",
			},
			Status:        model.NodeStatusHealthy,
			LastHeartbeat: clock.Now(),
			Metadata: map[string]string{
				"org_id": orgID,
			},
		}
		if err := store.SaveFleetNode(ctx, node); err != nil {
			b.Fatalf("failed to save node: %v", err)
		}

		sub := &model.TelemetrySubmission{
			NodeID:    nodeID,
			Timestamp: clock.Now(),
			Metrics: map[string]float64{
				"cpu_usage_pct":    70.0 + float64(i%25),
				"memory_usage_pct": 50.0 + float64(i%40),
			},
		}
		if err := store.SaveTelemetrySubmission(ctx, sub); err != nil {
			b.Fatalf("failed to save telemetry: %v", err)
		}
	}

	// Base policy
	basePolicy := &model.Policy{
		ID:             "pol-baseline",
		OrgID:          orgID,
		Name:           "Baseline Policy",
		Category:       model.PolicyCategoryResourceThresholds,
		Status:         model.PolicyStatusActive,
		ActiveRevision: 1,
	}
	if err := store.SavePolicy(ctx, basePolicy); err != nil {
		b.Fatalf("failed to save policy: %v", err)
	}

	baseRev := &model.PolicyRevision{
		PolicyID:        "pol-baseline",
		Revision:        1,
		Priority:        100,
		InheritanceMode: model.InheritanceModeInheritAndOverride,
		EnforcementMode: model.EnforcementModeEnforce,
		Rules: []model.PolicyRule{
			{
				ID:       "rule-cpu",
				Name:     "CPU Threshold",
				Type:     model.RuleTypeResourceThreshold,
				Severity: model.SeverityCritical,
				Enabled:  true,
				ResourceThreshold: &model.ResourceThresholdRuleConfig{
					Metric:            "cpu_usage_pct",
					WarningThreshold:  75,
					CriticalThreshold: 85,
				},
			},
		},
	}
	if err := store.SavePolicyRevision(ctx, baseRev); err != nil {
		b.Fatalf("failed to save revision: %v", err)
	}

	baseAsgn := &model.PolicyAssignment{
		ID:         "asgn-baseline",
		PolicyID:   "pol-baseline",
		OrgID:      orgID,
		TargetType: model.TargetTypeOrganization,
		TargetID:   orgID,
		Enabled:    true,
	}
	if err := store.SavePolicyAssignment(ctx, baseAsgn); err != nil {
		b.Fatalf("failed to save assignment: %v", err)
	}

	// Simulation Request proposing relaxed thresholds
	simReq := &model.SimulationRequest{
		OrgID: orgID,
		ProposedPolicies: []model.Policy{
			{
				ID:             "pol-baseline",
				OrgID:          orgID,
				Name:           "Baseline Policy",
				Category:       model.PolicyCategoryResourceThresholds,
				Status:         model.PolicyStatusActive,
				ActiveRevision: 2,
			},
		},
		ProposedRevisions: []model.PolicyRevision{
			{
				PolicyID:        "pol-baseline",
				Revision:        2,
				Priority:        100,
				InheritanceMode: model.InheritanceModeInheritAndOverride,
				EnforcementMode: model.EnforcementModeEnforce,
				Rules: []model.PolicyRule{
					{
						ID:       "rule-cpu",
						Name:     "CPU Threshold",
						Type:     model.RuleTypeResourceThreshold,
						Severity: model.SeverityCritical,
						Enabled:  true,
						ResourceThreshold: &model.ResourceThresholdRuleConfig{
							Metric:            "cpu_usage_pct",
							WarningThreshold:  85,
							CriticalThreshold: 95,
						},
					},
				},
			},
		},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		res, err := engine.SimulatePolicyChanges(ctx, simReq)
		if err != nil {
			b.Fatalf("simulation failed: %v", err)
		}
		if res.TotalNodes != 100 {
			b.Fatalf("expected 100 nodes, got %d", res.TotalNodes)
		}
	}
}
