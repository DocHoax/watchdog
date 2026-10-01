package governance

import (
	"testing"

	"github.com/DocHoax/watchdog/pkg/model"
)

func TestDetectEffectiveRuleConflicts(t *testing.T) {
	t.Run("no conflicts with single rule or empty", func(t *testing.T) {
		conflicts := DetectEffectiveRuleConflicts(nil)
		if len(conflicts) != 0 {
			t.Errorf("expected 0 conflicts, got %d", len(conflicts))
		}

		single := []EffectiveRule{
			{
				SourcePolicyID: "pol-1",
				SourceRevision: 1,
				HierarchyLevel: 1,
				Priority:       100,
				Category:       model.PolicyCategoryResourceThresholds,
				Rule: model.PolicyRule{
					ID:   "rule-cpu",
					Type: model.RuleTypeResourceThreshold,
					ResourceThreshold: &model.ResourceThresholdRuleConfig{
						Metric:            "cpu_usage_pct",
						WarningThreshold:  70,
						CriticalThreshold: 85,
					},
				},
			},
		}
		conflicts = DetectEffectiveRuleConflicts(single)
		if len(conflicts) != 0 {
			t.Errorf("expected 0 conflicts for single rule, got %d", len(conflicts))
		}
	})

	t.Run("definite conflict on resource threshold at same precedence", func(t *testing.T) {
		rules := []EffectiveRule{
			{
				SourcePolicyID: "pol-1",
				SourceRevision: 1,
				HierarchyLevel: 1,
				Priority:       100,
				Category:       model.PolicyCategoryResourceThresholds,
				Rule: model.PolicyRule{
					ID:   "rule-cpu-a",
					Type: model.RuleTypeResourceThreshold,
					ResourceThreshold: &model.ResourceThresholdRuleConfig{
						Metric:            "cpu_usage_pct",
						WarningThreshold:  70,
						CriticalThreshold: 85,
					},
				},
			},
			{
				SourcePolicyID: "pol-2",
				SourceRevision: 1,
				HierarchyLevel: 1,
				Priority:       100,
				Category:       model.PolicyCategoryResourceThresholds,
				Rule: model.PolicyRule{
					ID:   "rule-cpu-b",
					Type: model.RuleTypeResourceThreshold,
					ResourceThreshold: &model.ResourceThresholdRuleConfig{
						Metric:            "cpu_usage_pct",
						WarningThreshold:  80,
						CriticalThreshold: 95,
					},
				},
			},
		}

		conflicts := DetectEffectiveRuleConflicts(rules)
		if len(conflicts) != 1 {
			t.Fatalf("expected 1 conflict, got %d", len(conflicts))
		}
		if conflicts[0].Severity != ConflictSeverityDefinite {
			t.Errorf("expected severity %s, got %s", ConflictSeverityDefinite, conflicts[0].Severity)
		}
		if conflicts[0].Metric != "cpu_usage_pct" {
			t.Errorf("expected metric cpu_usage_pct, got %s", conflicts[0].Metric)
		}
	})

	t.Run("no conflict if precedence levels differ", func(t *testing.T) {
		rules := []EffectiveRule{
			{
				SourcePolicyID: "pol-org",
				SourceRevision: 1,
				HierarchyLevel: 1,
				Priority:       100,
				Category:       model.PolicyCategoryResourceThresholds,
				Rule: model.PolicyRule{
					ID:   "rule-cpu-org",
					Type: model.RuleTypeResourceThreshold,
					ResourceThreshold: &model.ResourceThresholdRuleConfig{
						Metric:            "cpu_usage_pct",
						WarningThreshold:  70,
						CriticalThreshold: 85,
					},
				},
			},
			{
				SourcePolicyID: "pol-grp",
				SourceRevision: 1,
				HierarchyLevel: 2, // higher precedence
				Priority:       100,
				Category:       model.PolicyCategoryResourceThresholds,
				Rule: model.PolicyRule{
					ID:   "rule-cpu-grp",
					Type: model.RuleTypeResourceThreshold,
					ResourceThreshold: &model.ResourceThresholdRuleConfig{
						Metric:            "cpu_usage_pct",
						WarningThreshold:  80,
						CriticalThreshold: 95,
					},
				},
			},
		}

		conflicts := DetectEffectiveRuleConflicts(rules)
		if len(conflicts) != 0 {
			t.Errorf("expected 0 conflicts when hierarchy levels differ, got %d", len(conflicts))
		}
	})

	t.Run("definite conflict on anomaly detection at same precedence", func(t *testing.T) {
		rules := []EffectiveRule{
			{
				SourcePolicyID: "pol-1",
				SourceRevision: 1,
				HierarchyLevel: 10,
				Priority:       50,
				Category:       model.PolicyCategoryAnomalyDetection,
				Rule: model.PolicyRule{
					ID:   "rule-anom-a",
					Type: model.RuleTypeAnomalyDetection,
					AnomalyDetection: &model.AnomalyDetectionRuleConfig{
						Metric:          "disk_io_rate",
						ZScoreThreshold: 3.0,
					},
				},
			},
			{
				SourcePolicyID: "pol-2",
				SourceRevision: 1,
				HierarchyLevel: 10,
				Priority:       50,
				Category:       model.PolicyCategoryAnomalyDetection,
				Rule: model.PolicyRule{
					ID:   "rule-anom-b",
					Type: model.RuleTypeAnomalyDetection,
					AnomalyDetection: &model.AnomalyDetectionRuleConfig{
						Metric:          "disk_io_rate",
						ZScoreThreshold: 4.5,
					},
				},
			},
		}

		conflicts := DetectEffectiveRuleConflicts(rules)
		if len(conflicts) != 1 {
			t.Fatalf("expected 1 conflict, got %d", len(conflicts))
		}
		if conflicts[0].Severity != ConflictSeverityDefinite {
			t.Errorf("expected definite severity, got %s", conflicts[0].Severity)
		}
	})

	t.Run("definite conflict on compliance check at same precedence", func(t *testing.T) {
		rules := []EffectiveRule{
			{
				SourcePolicyID: "pol-1",
				SourceRevision: 1,
				HierarchyLevel: 5,
				Priority:       10,
				Category:       model.PolicyCategoryOperationalCompliance,
				Rule: model.PolicyRule{
					ID:   "rule-comp-a",
					Type: model.RuleTypeOperationalCompliance,
					OperationalCompliance: &model.OperationalComplianceRuleConfig{
						CheckType:     "collector_version",
						ExpectedValue: ">= 1.5.0",
					},
				},
			},
			{
				SourcePolicyID: "pol-2",
				SourceRevision: 1,
				HierarchyLevel: 5,
				Priority:       10,
				Category:       model.PolicyCategoryOperationalCompliance,
				Rule: model.PolicyRule{
					ID:   "rule-comp-b",
					Type: model.RuleTypeOperationalCompliance,
					OperationalCompliance: &model.OperationalComplianceRuleConfig{
						CheckType:     "collector_version",
						ExpectedValue: ">= 2.0.0",
					},
				},
			},
		}

		conflicts := DetectEffectiveRuleConflicts(rules)
		if len(conflicts) != 1 {
			t.Fatalf("expected 1 conflict, got %d", len(conflicts))
		}
		if conflicts[0].Severity != ConflictSeverityDefinite {
			t.Errorf("expected definite severity, got %s", conflicts[0].Severity)
		}
	})
}

func TestDetectPolicyPairConflicts(t *testing.T) {
	revA := &model.PolicyRevision{
		PolicyID: "pol-a",
		Revision: 1,
		Rules: []model.PolicyRule{
			{
				ID:      "r-cpu-a",
				Type:    model.RuleTypeResourceThreshold,
				Enabled: true,
				ResourceThreshold: &model.ResourceThresholdRuleConfig{
					Metric:            "cpu_usage_pct",
					WarningThreshold:  70,
					CriticalThreshold: 85,
				},
			},
			{
				ID:      "r-anom-a",
				Type:    model.RuleTypeAnomalyDetection,
				Enabled: true,
				AnomalyDetection: &model.AnomalyDetectionRuleConfig{
					Metric:          "network_rx",
					ZScoreThreshold: 2.5,
				},
			},
		},
	}

	revB := &model.PolicyRevision{
		PolicyID: "pol-b",
		Revision: 2,
		Rules: []model.PolicyRule{
			{
				ID:      "r-cpu-b",
				Type:    model.RuleTypeResourceThreshold,
				Enabled: true,
				ResourceThreshold: &model.ResourceThresholdRuleConfig{
					Metric:            "cpu_usage_pct",
					WarningThreshold:  75,
					CriticalThreshold: 90,
				},
			},
			{
				ID:      "r-anom-b",
				Type:    model.RuleTypeAnomalyDetection,
				Enabled: true,
				AnomalyDetection: &model.AnomalyDetectionRuleConfig{
					Metric:          "network_rx",
					ZScoreThreshold: 3.5,
				},
			},
		},
	}

	conflicts := DetectPolicyPairConflicts(revA, revB)
	if len(conflicts) != 2 {
		t.Fatalf("expected 2 potential conflicts, got %d", len(conflicts))
	}

	for _, c := range conflicts {
		if c.Severity != ConflictSeverityPotential {
			t.Errorf("expected severity %s, got %s", ConflictSeverityPotential, c.Severity)
		}
	}

	// Test nil handling
	if len(DetectPolicyPairConflicts(nil, revB)) != 0 {
		t.Errorf("expected 0 conflicts for nil revA")
	}
	if len(DetectPolicyPairConflicts(revA, nil)) != 0 {
		t.Errorf("expected 0 conflicts for nil revB")
	}
}
