package governance

import (
	"fmt"
	"math"
	"strings"

	"github.com/DocHoax/watchdog/pkg/model"
)

// ConflictSeverity represents the severity classification of a policy or rule conflict.
type ConflictSeverity string

const (
	// ConflictSeverityDefinite indicates direct, irreconcilable contradiction (e.g., conflicting thresholds at same precedence).
	ConflictSeverityDefinite ConflictSeverity = "definite"
	// ConflictSeverityPotential indicates overlapping scopes with divergent parameters resolved via precedence.
	ConflictSeverityPotential ConflictSeverity = "potential"
	// ConflictSeverityCompatible indicates overlapping rules that are complementary.
	ConflictSeverityCompatible ConflictSeverity = "compatible"
	// ConflictSeverityIndeterminate indicates rules whose selector expressions cannot be statically determined disjoint.
	ConflictSeverityIndeterminate ConflictSeverity = "indeterminate"
)

// PolicyConflict captures the details of a detected rule or policy conflict.
type PolicyConflict struct {
	Severity  ConflictSeverity     `json:"severity"`
	Category  model.PolicyCategory `json:"category"`
	Metric    string               `json:"metric,omitempty"`
	PolicyA   string               `json:"policy_a"`
	RevisionA int                  `json:"revision_a"`
	RuleA     string               `json:"rule_a"`
	PolicyB   string               `json:"policy_b"`
	RevisionB int                  `json:"revision_b"`
	RuleB     string               `json:"rule_b"`
	Reason    string               `json:"reason"`
}

// DetectEffectiveRuleConflicts analyzes effective resolved rules to detect any internal inconsistencies or warnings.
func DetectEffectiveRuleConflicts(effectiveRules []EffectiveRule) []PolicyConflict {
	var conflicts []PolicyConflict
	if len(effectiveRules) < 2 {
		return conflicts
	}

	for i := 0; i < len(effectiveRules); i++ {
		rA := effectiveRules[i]
		for j := i + 1; j < len(effectiveRules); j++ {
			rB := effectiveRules[j]

			// Check resource thresholds on the same metric
			if rA.Rule.Type == model.RuleTypeResourceThreshold && rB.Rule.Type == model.RuleTypeResourceThreshold {
				cfgA := rA.Rule.ResourceThreshold
				cfgB := rB.Rule.ResourceThreshold
				if cfgA != nil && cfgB != nil && strings.EqualFold(cfgA.Metric, cfgB.Metric) {
					if rA.Priority == rB.Priority && rA.HierarchyLevel == rB.HierarchyLevel {
						// Same precedence level but different thresholds -> Definite Conflict
						if cfgA.WarningThreshold != cfgB.WarningThreshold || cfgA.CriticalThreshold != cfgB.CriticalThreshold {
							conflicts = append(conflicts, PolicyConflict{
								Severity:  ConflictSeverityDefinite,
								Category:  rA.Category,
								Metric:    cfgA.Metric,
								PolicyA:   rA.SourcePolicyID,
								RevisionA: rA.SourceRevision,
								RuleA:     rA.Rule.ID,
								PolicyB:   rB.SourcePolicyID,
								RevisionB: rB.SourceRevision,
								RuleB:     rB.Rule.ID,
								Reason: fmt.Sprintf("Contradictory resource thresholds on metric %q at identical precedence (warn: %.1f vs %.1f, crit: %.1f vs %.1f)",
									cfgA.Metric, cfgA.WarningThreshold, cfgB.WarningThreshold, cfgA.CriticalThreshold, cfgB.CriticalThreshold),
							})
						}
					}
				}
			}

			// Check anomaly detection on the same metric
			if rA.Rule.Type == model.RuleTypeAnomalyDetection && rB.Rule.Type == model.RuleTypeAnomalyDetection {
				cfgA := rA.Rule.AnomalyDetection
				cfgB := rB.Rule.AnomalyDetection
				if cfgA != nil && cfgB != nil && strings.EqualFold(cfgA.Metric, cfgB.Metric) {
					if rA.Priority == rB.Priority && rA.HierarchyLevel == rB.HierarchyLevel {
						if math.Abs(cfgA.ZScoreThreshold-cfgB.ZScoreThreshold) > 0.001 {
							conflicts = append(conflicts, PolicyConflict{
								Severity:  ConflictSeverityDefinite,
								Category:  rA.Category,
								Metric:    cfgA.Metric,
								PolicyA:   rA.SourcePolicyID,
								RevisionA: rA.SourceRevision,
								RuleA:     rA.Rule.ID,
								PolicyB:   rB.SourcePolicyID,
								RevisionB: rB.SourceRevision,
								RuleB:     rB.Rule.ID,
								Reason: fmt.Sprintf("Contradictory anomaly z-score thresholds on metric %q at identical precedence (z: %.2f vs %.2f)",
									cfgA.Metric, cfgA.ZScoreThreshold, cfgB.ZScoreThreshold),
							})
						}
					}
				}
			}

			// Check operational compliance checks of the same type
			if rA.Rule.Type == model.RuleTypeOperationalCompliance && rB.Rule.Type == model.RuleTypeOperationalCompliance {
				cfgA := rA.Rule.OperationalCompliance
				cfgB := rB.Rule.OperationalCompliance
				if cfgA != nil && cfgB != nil && strings.EqualFold(cfgA.CheckType, cfgB.CheckType) {
					if rA.Priority == rB.Priority && rA.HierarchyLevel == rB.HierarchyLevel {
						if (cfgA.ExpectedValue != "" && cfgB.ExpectedValue != "" && cfgA.ExpectedValue != cfgB.ExpectedValue) ||
							(cfgA.MaxAgeSeconds > 0 && cfgB.MaxAgeSeconds > 0 && cfgA.MaxAgeSeconds != cfgB.MaxAgeSeconds) {
							conflicts = append(conflicts, PolicyConflict{
								Severity:  ConflictSeverityDefinite,
								Category:  rA.Category,
								PolicyA:   rA.SourcePolicyID,
								RevisionA: rA.SourceRevision,
								RuleA:     rA.Rule.ID,
								PolicyB:   rB.SourcePolicyID,
								RevisionB: rB.SourceRevision,
								RuleB:     rB.Rule.ID,
								Reason: fmt.Sprintf("Contradictory compliance check expectations for %q at identical precedence (%q vs %q)",
									cfgA.CheckType, cfgA.ExpectedValue, cfgB.ExpectedValue),
							})
						}
					}
				}
			}
		}
	}

	return conflicts
}

// DetectPolicyPairConflicts inspects two policy revisions to detect potential overlaps or conflicts before assignment.
func DetectPolicyPairConflicts(revA *model.PolicyRevision, revB *model.PolicyRevision) []PolicyConflict {
	var conflicts []PolicyConflict
	if revA == nil || revB == nil {
		return conflicts
	}

	for _, ruleA := range revA.Rules {
		if !ruleA.Enabled {
			continue
		}
		for _, ruleB := range revB.Rules {
			if !ruleB.Enabled {
				continue
			}

			if ruleA.Type == model.RuleTypeResourceThreshold && ruleB.Type == model.RuleTypeResourceThreshold {
				cfgA := ruleA.ResourceThreshold
				cfgB := ruleB.ResourceThreshold
				if cfgA != nil && cfgB != nil && strings.EqualFold(cfgA.Metric, cfgB.Metric) {
					if cfgA.WarningThreshold != cfgB.WarningThreshold || cfgA.CriticalThreshold != cfgB.CriticalThreshold {
						conflicts = append(conflicts, PolicyConflict{
							Severity:  ConflictSeverityPotential,
							Category:  model.PolicyCategoryResourceThresholds,
							Metric:    cfgA.Metric,
							PolicyA:   revA.PolicyID,
							RevisionA: revA.Revision,
							RuleA:     ruleA.ID,
							PolicyB:   revB.PolicyID,
							RevisionB: revB.Revision,
							RuleB:     ruleB.ID,
							Reason: fmt.Sprintf("Divergent threshold configurations for metric %q (Policy A: warn=%.1f/crit=%.1f vs Policy B: warn=%.1f/crit=%.1f)",
								cfgA.Metric, cfgA.WarningThreshold, cfgA.CriticalThreshold, cfgB.WarningThreshold, cfgB.CriticalThreshold),
						})
					}
				}
			}

			if ruleA.Type == model.RuleTypeAnomalyDetection && ruleB.Type == model.RuleTypeAnomalyDetection {
				cfgA := ruleA.AnomalyDetection
				cfgB := ruleB.AnomalyDetection
				if cfgA != nil && cfgB != nil && strings.EqualFold(cfgA.Metric, cfgB.Metric) {
					if math.Abs(cfgA.ZScoreThreshold-cfgB.ZScoreThreshold) > 0.001 {
						conflicts = append(conflicts, PolicyConflict{
							Severity:  ConflictSeverityPotential,
							Category:  model.PolicyCategoryAnomalyDetection,
							Metric:    cfgA.Metric,
							PolicyA:   revA.PolicyID,
							RevisionA: revA.Revision,
							RuleA:     ruleA.ID,
							PolicyB:   revB.PolicyID,
							RevisionB: revB.Revision,
							RuleB:     ruleB.ID,
							Reason: fmt.Sprintf("Divergent anomaly z-score thresholds for metric %q (Policy A: %.2f vs Policy B: %.2f)",
								cfgA.Metric, cfgA.ZScoreThreshold, cfgB.ZScoreThreshold),
						})
					}
				}
			}

			if ruleA.Type == model.RuleTypeOperationalCompliance && ruleB.Type == model.RuleTypeOperationalCompliance {
				cfgA := ruleA.OperationalCompliance
				cfgB := ruleB.OperationalCompliance
				if cfgA != nil && cfgB != nil && strings.EqualFold(cfgA.CheckType, cfgB.CheckType) {
					if (cfgA.ExpectedValue != "" && cfgB.ExpectedValue != "" && cfgA.ExpectedValue != cfgB.ExpectedValue) ||
						(cfgA.MaxAgeSeconds > 0 && cfgB.MaxAgeSeconds > 0 && cfgA.MaxAgeSeconds != cfgB.MaxAgeSeconds) {
						conflicts = append(conflicts, PolicyConflict{
							Severity:  ConflictSeverityPotential,
							Category:  model.PolicyCategoryOperationalCompliance,
							PolicyA:   revA.PolicyID,
							RevisionA: revA.Revision,
							RuleA:     ruleA.ID,
							PolicyB:   revB.PolicyID,
							RevisionB: revB.Revision,
							RuleB:     ruleB.ID,
							Reason:    fmt.Sprintf("Divergent compliance expectations for check %q", cfgA.CheckType),
						})
					}
				}
			}
		}
	}

	return conflicts
}
