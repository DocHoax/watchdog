package governance

import (
	"context"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/DocHoax/watchdog/pkg/model"
)

// RuleEvaluator defines the evaluation strategy interface for a specific policy rule type.
type RuleEvaluator interface {
	RuleType() model.PolicyRuleType
	Evaluate(ctx context.Context, evalCtx *EvaluationContext, effective EffectiveRule) (model.EvaluationResult, error)
}

// EvaluatorRegistry maintains the collection of available RuleEvaluators indexed by RuleType.
type EvaluatorRegistry struct {
	mu         sync.RWMutex
	evaluators map[model.PolicyRuleType]RuleEvaluator
}

// NewEvaluatorRegistry instantiates a registry preloaded with standard rule evaluators.
func NewEvaluatorRegistry() *EvaluatorRegistry {
	r := &EvaluatorRegistry{
		evaluators: make(map[model.PolicyRuleType]RuleEvaluator),
	}
	r.Register(&ResourceThresholdEvaluator{})
	r.Register(&AnomalyDetectionEvaluator{})
	r.Register(&CapacityPlanningEvaluator{})
	r.Register(&IncidentSeverityEvaluator{})
	r.Register(&OperationalComplianceEvaluator{})
	return r
}

// Register adds or replaces an evaluator for a rule type.
func (r *EvaluatorRegistry) Register(evaluator RuleEvaluator) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.evaluators[evaluator.RuleType()] = evaluator
}

// Get retrieves an evaluator for the requested rule type.
func (r *EvaluatorRegistry) Get(ruleType model.PolicyRuleType) (RuleEvaluator, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	eval, ok := r.evaluators[ruleType]
	return eval, ok
}

// EvaluateContext executes all effective rules on the evaluation context and produces evaluation results.
func (r *EvaluatorRegistry) EvaluateContext(ctx context.Context, evalCtx *EvaluationContext) ([]model.EvaluationResult, error) {
	if evalCtx == nil {
		return nil, fmt.Errorf("nil evaluation context")
	}
	if evalCtx.ResolvedPolicySet == nil {
		return nil, nil
	}

	var results []model.EvaluationResult
	now := evalCtx.Clock.Now()

	for _, effective := range evalCtx.ResolvedPolicySet.EffectiveRules {
		eval, ok := r.Get(effective.Rule.Type)
		if !ok {
			res := initBaseResult(effective, now)
			res.Status = model.EvaluationStatusError
			res.Message = fmt.Sprintf("no evaluator registered for rule type: %s", effective.Rule.Type)
			res.DataFreshness = model.DataFreshnessUnsupported
			results = append(results, res)
			continue
		}

		res, err := eval.Evaluate(ctx, evalCtx, effective)
		if err != nil {
			res = initBaseResult(effective, now)
			res.Status = model.EvaluationStatusError
			res.Message = fmt.Sprintf("evaluator error: %v", err)
		}
		results = append(results, res)
	}

	return results, nil
}

func initBaseResult(effective EffectiveRule, now time.Time) model.EvaluationResult {
	sev := effective.Rule.Severity
	if sev == "" {
		sev = model.SeverityWarning
	}
	return model.EvaluationResult{
		RuleID:          effective.Rule.ID,
		RuleName:        effective.Rule.Name,
		PolicyID:        effective.SourcePolicyID,
		PolicyRevision:  effective.SourceRevision,
		Category:        effective.Category,
		RuleType:        effective.Rule.Type,
		Status:          model.EvaluationStatusCompliant,
		Severity:        sev,
		EnforcementMode: effective.EnforcementMode,
		EvaluatedAt:     now,
		DataFreshness:   model.DataFreshnessFresh,
		Details:         make(map[string]string),
	}
}

// -------------------------------------------------------------------------
// 1. ResourceThresholdEvaluator
// -------------------------------------------------------------------------

// ResourceThresholdEvaluator evaluates telemetry metrics against scalar warning/critical thresholds.
type ResourceThresholdEvaluator struct{}

// RuleType returns RuleTypeResourceThreshold.
func (e *ResourceThresholdEvaluator) RuleType() model.PolicyRuleType {
	return model.RuleTypeResourceThreshold
}

// Evaluate performs threshold comparison on latest metric readings.
func (e *ResourceThresholdEvaluator) Evaluate(
	_ context.Context,
	evalCtx *EvaluationContext,
	effective EffectiveRule,
) (model.EvaluationResult, error) {
	now := evalCtx.Clock.Now()
	res := initBaseResult(effective, now)

	if !effective.Rule.Enabled {
		res.Status = model.EvaluationStatusNotApplicable
		res.Message = "rule is disabled"
		return res, nil
	}

	cfg := effective.Rule.ResourceThreshold
	if cfg == nil {
		res.Status = model.EvaluationStatusError
		res.Message = "missing resource_threshold configuration"
		return res, nil
	}

	val, freshness, found := evalCtx.GetMetricValue(cfg.Metric)
	res.DataFreshness = freshness

	if !found {
		res.Status = model.EvaluationStatusInsufficientData
		res.ObservedValue = "N/A"
		res.ExpectedValue = formatResourceThresholdExpectation(cfg)
		res.Message = fmt.Sprintf("metric '%s' not found in latest telemetry or node summary", cfg.Metric)
		return res, nil
	}

	if freshness == model.DataFreshnessInvalid {
		res.Status = model.EvaluationStatusInsufficientData
		res.ObservedValue = fmt.Sprintf("%.2f%s", val, cfg.Unit)
		res.ExpectedValue = formatResourceThresholdExpectation(cfg)
		res.Message = fmt.Sprintf("telemetry metric '%s' has invalid freshness (clock skew)", cfg.Metric)
		return res, nil
	}

	unit := cfg.Unit
	if unit == "" {
		unit = "%"
	}

	res.ObservedValue = fmt.Sprintf("%.2f%s", val, unit)
	res.ExpectedValue = formatResourceThresholdExpectation(cfg)

	// Threshold comparison
	// Normal orientation: Warning < Critical (e.g. CPU > 80% warning, > 90% critical)
	if cfg.CriticalThreshold >= cfg.WarningThreshold {
		if cfg.CriticalThreshold > 0 && val >= cfg.CriticalThreshold {
			res.Status = model.EvaluationStatusNonCompliant
			res.Severity = model.SeverityCritical
			res.Message = fmt.Sprintf("metric '%s' (%.2f%s) breaches critical threshold (%.2f%s)", cfg.Metric, val, unit, cfg.CriticalThreshold, unit)
		} else if cfg.WarningThreshold > 0 && val >= cfg.WarningThreshold {
			res.Status = model.EvaluationStatusWarning
			res.Severity = model.SeverityWarning
			res.Message = fmt.Sprintf("metric '%s' (%.2f%s) breaches warning threshold (%.2f%s)", cfg.Metric, val, unit, cfg.WarningThreshold, unit)
		} else {
			res.Status = model.EvaluationStatusCompliant
			res.Message = fmt.Sprintf("metric '%s' (%.2f%s) is within acceptable threshold (<= %.2f%s)", cfg.Metric, val, unit, cfg.WarningThreshold, unit)
		}
	} else {
		// Inverted orientation: Critical < Warning (e.g. Disk available < 20% warning, < 10% critical)
		if cfg.CriticalThreshold > 0 && val <= cfg.CriticalThreshold {
			res.Status = model.EvaluationStatusNonCompliant
			res.Severity = model.SeverityCritical
			res.Message = fmt.Sprintf("metric '%s' (%.2f%s) breaches critical lower threshold (<= %.2f%s)", cfg.Metric, val, unit, cfg.CriticalThreshold, unit)
		} else if cfg.WarningThreshold > 0 && val <= cfg.WarningThreshold {
			res.Status = model.EvaluationStatusWarning
			res.Severity = model.SeverityWarning
			res.Message = fmt.Sprintf("metric '%s' (%.2f%s) breaches warning lower threshold (<= %.2f%s)", cfg.Metric, val, unit, cfg.WarningThreshold, unit)
		} else {
			res.Status = model.EvaluationStatusCompliant
			res.Message = fmt.Sprintf("metric '%s' (%.2f%s) is within acceptable threshold (>= %.2f%s)", cfg.Metric, val, unit, cfg.WarningThreshold, unit)
		}
	}

	return res, nil
}

func formatResourceThresholdExpectation(cfg *model.ResourceThresholdRuleConfig) string {
	unit := cfg.Unit
	if unit == "" {
		unit = "%"
	}
	if cfg.CriticalThreshold >= cfg.WarningThreshold {
		if cfg.CriticalThreshold > 0 && cfg.WarningThreshold > 0 {
			return fmt.Sprintf("warn < %.2f%s, crit < %.2f%s", cfg.WarningThreshold, unit, cfg.CriticalThreshold, unit)
		} else if cfg.CriticalThreshold > 0 {
			return fmt.Sprintf("< %.2f%s", cfg.CriticalThreshold, unit)
		}
		return fmt.Sprintf("< %.2f%s", cfg.WarningThreshold, unit)
	}
	return fmt.Sprintf("warn > %.2f%s, crit > %.2f%s", cfg.WarningThreshold, unit, cfg.CriticalThreshold, unit)
}

// -------------------------------------------------------------------------
// 2. AnomalyDetectionEvaluator
// -------------------------------------------------------------------------

// AnomalyDetectionEvaluator performs statistical z-score evaluation against historical baseline telemetry.
type AnomalyDetectionEvaluator struct{}

// RuleType returns RuleTypeAnomalyDetection.
func (e *AnomalyDetectionEvaluator) RuleType() model.PolicyRuleType {
	return model.RuleTypeAnomalyDetection
}

// Evaluate computes the z-score of the latest metric value and tests for statistical deviance.
func (e *AnomalyDetectionEvaluator) Evaluate(
	_ context.Context,
	evalCtx *EvaluationContext,
	effective EffectiveRule,
) (model.EvaluationResult, error) {
	now := evalCtx.Clock.Now()
	res := initBaseResult(effective, now)

	if !effective.Rule.Enabled {
		res.Status = model.EvaluationStatusNotApplicable
		res.Message = "rule is disabled"
		return res, nil
	}

	cfg := effective.Rule.AnomalyDetection
	if cfg == nil {
		res.Status = model.EvaluationStatusError
		res.Message = "missing anomaly_detection configuration"
		return res, nil
	}

	// 1. Check excluded maintenance hours
	currHour := now.UTC().Hour()
	for _, h := range cfg.ExcludedHours {
		if h == currHour {
			res.Status = model.EvaluationStatusNotApplicable
			res.Message = fmt.Sprintf("anomaly evaluation skipped during excluded hour %d:00 UTC", currHour)
			return res, nil
		}
	}

	// 2. Lookup metric
	val, freshness, found := evalCtx.GetMetricValue(cfg.Metric)
	res.DataFreshness = freshness

	if !found {
		res.Status = model.EvaluationStatusInsufficientData
		res.ObservedValue = "N/A"
		res.Message = fmt.Sprintf("metric '%s' not found for anomaly evaluation", cfg.Metric)
		return res, nil
	}

	// 3. Compute or retrieve statistical baseline
	var mean, stdDev float64
	history := evalCtx.GetMetricHistory(cfg.Metric)
	agg, hasAgg := evalCtx.GetMetricAggregate(cfg.Metric)

	if hasAgg && agg != nil && agg.StdDev > 0 {
		mean = agg.Avg
		stdDev = agg.StdDev
	} else if len(history) >= 3 {
		var baselineValues []float64
		for _, pt := range history {
			// Exclude latest observation from baseline if we have sufficient prior history
			if evalCtx.LatestTelemetry != nil && pt.Timestamp.Equal(evalCtx.LatestTelemetry.Timestamp) && len(history) > 3 {
				continue
			}
			baselineValues = append(baselineValues, pt.Value)
		}
		if len(baselineValues) < 3 {
			baselineValues = nil
			for _, pt := range history {
				baselineValues = append(baselineValues, pt.Value)
			}
		}

		var sum float64
		for _, v := range baselineValues {
			sum += v
		}
		mean = sum / float64(len(baselineValues))
		var varianceSum float64
		for _, v := range baselineValues {
			diff := v - mean
			varianceSum += diff * diff
		}
		stdDev = math.Sqrt(varianceSum / float64(len(baselineValues)))
	} else {
		res.Status = model.EvaluationStatusInsufficientData
		res.ObservedValue = fmt.Sprintf("%.2f", val)
		res.Message = fmt.Sprintf("insufficient historical data points (%d points, minimum 3 required) to compute baseline for '%s'", len(history), cfg.Metric)
		return res, nil
	}

	// 4. Calculate Z-score
	threshold := cfg.ZScoreThreshold
	if threshold <= 0 {
		switch strings.ToLower(cfg.Sensitivity) {
		case "high":
			threshold = 2.0
		case "low":
			threshold = 3.5
		default:
			threshold = 2.5
		}
	}

	var zScore float64
	if stdDev > 0.0001 {
		zScore = math.Abs(val-mean) / stdDev
	} else if math.Abs(val-mean) > 0.0001 {
		zScore = 100.0 // Definite anomaly if stdDev is 0 but value shifted
	}

	res.ObservedValue = fmt.Sprintf("z_score=%.2f (val=%.2f, mean=%.2f, std=%.2f)", zScore, val, mean, stdDev)
	res.ExpectedValue = fmt.Sprintf("z_score <= %.2f", threshold)

	if zScore > threshold {
		res.Status = model.EvaluationStatusNonCompliant
		res.Severity = model.SeverityCritical
		if zScore <= threshold*1.25 {
			res.Status = model.EvaluationStatusWarning
			res.Severity = model.SeverityWarning
		}
		res.Message = fmt.Sprintf("statistical anomaly detected on metric '%s': z-score %.2f exceeds threshold %.2f (value %.2f vs baseline mean %.2f)", cfg.Metric, zScore, threshold, val, mean)
	} else {
		res.Status = model.EvaluationStatusCompliant
		res.Message = fmt.Sprintf("metric '%s' behaving normally (z-score %.2f <= %.2f)", cfg.Metric, zScore, threshold)
	}

	return res, nil
}

// -------------------------------------------------------------------------
// 3. CapacityPlanningEvaluator
// -------------------------------------------------------------------------

// CapacityPlanningEvaluator performs linear runway projection to detect imminent resource exhaustion.
type CapacityPlanningEvaluator struct{}

// RuleType returns RuleTypeCapacityPlanning.
func (e *CapacityPlanningEvaluator) RuleType() model.PolicyRuleType {
	return model.RuleTypeCapacityPlanning
}

// Evaluate computes linear trend and days-to-exhaustion on historical metrics.
func (e *CapacityPlanningEvaluator) Evaluate(
	_ context.Context,
	evalCtx *EvaluationContext,
	effective EffectiveRule,
) (model.EvaluationResult, error) {
	now := evalCtx.Clock.Now()
	res := initBaseResult(effective, now)

	if !effective.Rule.Enabled {
		res.Status = model.EvaluationStatusNotApplicable
		res.Message = "rule is disabled"
		return res, nil
	}

	cfg := effective.Rule.CapacityPlanning
	if cfg == nil {
		res.Status = model.EvaluationStatusError
		res.Message = "missing capacity_planning configuration"
		return res, nil
	}

	history := evalCtx.GetMetricHistory(cfg.Metric)
	minPoints := cfg.MinDataPoints
	if minPoints < 2 {
		minPoints = 2
	}

	if len(history) < minPoints {
		res.Status = model.EvaluationStatusInsufficientData
		res.ObservedValue = "N/A"
		res.ExpectedValue = fmt.Sprintf("runway > %d days", cfg.WarningDaysToExhaustion)
		res.Message = fmt.Sprintf("insufficient historical data points (%d/%d) for capacity projection on '%s'", len(history), minPoints, cfg.Metric)
		return res, nil
	}

	// Compute linear trend: slope m = delta_y / delta_t (in days)
	firstPt := history[0]
	lastPt := history[len(history)-1]
	durationHours := lastPt.Timestamp.Sub(firstPt.Timestamp).Hours()

	var slopePerDay float64
	if durationHours > 0.001 {
		slopePerDay = (lastPt.Value - firstPt.Value) / (durationHours / 24.0)
	}

	currVal := lastPt.Value
	ceiling := 100.0 // Default for percentage/utilization metrics

	res.ExpectedValue = fmt.Sprintf("runway > %d days", cfg.WarningDaysToExhaustion)

	if slopePerDay > 0.0001 && currVal < ceiling {
		headroom := ceiling - currVal
		daysToExhaustion := headroom / slopePerDay

		res.ObservedValue = fmt.Sprintf("runway=%.1f days (growth=%.2f%%/day)", daysToExhaustion, slopePerDay)

		if daysToExhaustion <= float64(cfg.CriticalDaysToExhaustion) {
			res.Status = model.EvaluationStatusNonCompliant
			res.Severity = model.SeverityCritical
			res.Message = fmt.Sprintf("metric '%s' projected to exhaust capacity in %.1f days (critical limit: %d days, rate: +%.2f%%/day)", cfg.Metric, daysToExhaustion, cfg.CriticalDaysToExhaustion, slopePerDay)
		} else if daysToExhaustion <= float64(cfg.WarningDaysToExhaustion) {
			res.Status = model.EvaluationStatusWarning
			res.Severity = model.SeverityWarning
			res.Message = fmt.Sprintf("metric '%s' projected to exhaust capacity in %.1f days (warning limit: %d days, rate: +%.2f%%/day)", cfg.Metric, daysToExhaustion, cfg.WarningDaysToExhaustion, slopePerDay)
		} else {
			res.Status = model.EvaluationStatusCompliant
			res.Message = fmt.Sprintf("metric '%s' has %.1f days of runway remaining (healthy)", cfg.Metric, daysToExhaustion)
		}
	} else {
		res.Status = model.EvaluationStatusCompliant
		res.ObservedValue = fmt.Sprintf("runway=stable (trend=%.2f/day)", slopePerDay)
		res.Message = fmt.Sprintf("metric '%s' consumption is stable or declining (trend: %.2f/day)", cfg.Metric, slopePerDay)
	}

	return res, nil
}

// -------------------------------------------------------------------------
// 4. IncidentSeverityEvaluator
// -------------------------------------------------------------------------

// IncidentSeverityEvaluator audits active incidents on the target node against policy escalation criteria.
type IncidentSeverityEvaluator struct{}

// RuleType returns RuleTypeIncidentSeverity.
func (e *IncidentSeverityEvaluator) RuleType() model.PolicyRuleType {
	return model.RuleTypeIncidentSeverity
}

// Evaluate checks active incidents against duration, severity, and count triggers.
func (e *IncidentSeverityEvaluator) Evaluate(
	_ context.Context,
	evalCtx *EvaluationContext,
	effective EffectiveRule,
) (model.EvaluationResult, error) {
	now := evalCtx.Clock.Now()
	res := initBaseResult(effective, now)

	if !effective.Rule.Enabled {
		res.Status = model.EvaluationStatusNotApplicable
		res.Message = "rule is disabled"
		return res, nil
	}

	cfg := effective.Rule.IncidentSeverity
	if cfg == nil {
		res.Status = model.EvaluationStatusError
		res.Message = "missing incident_severity configuration"
		return res, nil
	}

	res.ExpectedValue = cfg.Condition
	activeCount := len(evalCtx.ActiveIncidents)
	res.ObservedValue = fmt.Sprintf("%d active incident(s)", activeCount)

	cond := strings.ToLower(strings.TrimSpace(cfg.Condition))

	// 1. Duration check (e.g. "duration > 15m" or "duration > 1h")
	if strings.HasPrefix(cond, "duration >") || strings.HasPrefix(cond, "duration >=") {
		parts := strings.Split(cond, ">")
		if len(parts) >= 2 {
			durStr := strings.TrimSpace(strings.TrimPrefix(parts[1], "="))
			limitDur, err := time.ParseDuration(durStr)
			if err == nil {
				for _, inc := range evalCtx.ActiveIncidents {
					age := evalCtx.Clock.Since(inc.StartTime)
					if age > limitDur {
						res.Status = model.EvaluationStatusNonCompliant
						res.Severity = model.SeverityCritical
						if cfg.EscalateToSeverity.IsValid() {
							res.Severity = cfg.EscalateToSeverity
						}
						res.Message = fmt.Sprintf("incident '%s' has been open for %s, breaching limit of %s", inc.ID, age.Round(time.Minute), limitDur)
						return res, nil
					}
				}
			}
		}
	}

	// 2. Severity check (e.g. "severity == critical" or "severity == warning")
	if strings.Contains(cond, "severity ==") || strings.Contains(cond, "severity=") {
		targetSev := model.SeverityCritical
		if strings.Contains(cond, "warning") {
			targetSev = model.SeverityWarning
		}
		for _, inc := range evalCtx.ActiveIncidents {
			if inc.Severity == targetSev {
				res.Status = model.EvaluationStatusNonCompliant
				res.Severity = targetSev
				res.Message = fmt.Sprintf("node has active %s incident '%s' (%s)", targetSev, inc.ID, inc.Title)
				return res, nil
			}
		}
	}

	// 3. Count check (e.g. "count > 0", "count >= 3", "open > 0")
	if strings.Contains(cond, "count >") || strings.Contains(cond, "count >=") || strings.Contains(cond, "open >") {
		var thresholdCount int
		if strings.Contains(cond, "3") {
			thresholdCount = 3
		} else if strings.Contains(cond, "2") {
			thresholdCount = 2
		} else if strings.Contains(cond, "1") {
			thresholdCount = 1
		} else {
			thresholdCount = 0
		}

		if activeCount > thresholdCount {
			res.Status = model.EvaluationStatusNonCompliant
			res.Message = fmt.Sprintf("node has %d active incidents, exceeding limit of %d", activeCount, thresholdCount)
			return res, nil
		}
	}

	// 4. Default check if no condition matched: any critical incident breaches
	for _, inc := range evalCtx.ActiveIncidents {
		if inc.Severity == model.SeverityCritical {
			res.Status = model.EvaluationStatusNonCompliant
			res.Severity = model.SeverityCritical
			res.Message = fmt.Sprintf("node impacted by active critical incident '%s'", inc.ID)
			return res, nil
		}
	}

	res.Status = model.EvaluationStatusCompliant
	res.Message = fmt.Sprintf("no incidents breach condition '%s'", cfg.Condition)
	return res, nil
}

// -------------------------------------------------------------------------
// 5. OperationalComplianceEvaluator
// -------------------------------------------------------------------------

// OperationalComplianceEvaluator audits heartbeat freshness, collector semver, mandatory tags, and platform governance.
type OperationalComplianceEvaluator struct{}

// RuleType returns RuleTypeOperationalCompliance.
func (e *OperationalComplianceEvaluator) RuleType() model.PolicyRuleType {
	return model.RuleTypeOperationalCompliance
}

// Evaluate conducts compliance inspections on node posture and configuration.
func (e *OperationalComplianceEvaluator) Evaluate(
	_ context.Context,
	evalCtx *EvaluationContext,
	effective EffectiveRule,
) (model.EvaluationResult, error) {
	now := evalCtx.Clock.Now()
	res := initBaseResult(effective, now)

	if !effective.Rule.Enabled {
		res.Status = model.EvaluationStatusNotApplicable
		res.Message = "rule is disabled"
		return res, nil
	}

	cfg := effective.Rule.OperationalCompliance
	if cfg == nil {
		res.Status = model.EvaluationStatusError
		res.Message = "missing operational_compliance configuration"
		return res, nil
	}

	checkType := strings.ToLower(strings.TrimSpace(cfg.CheckType))
	res.Severity = cfg.ViolationSeverity
	if res.Severity == "" {
		res.Severity = model.SeverityWarning
	}

	switch checkType {
	case "heartbeat_freshness", "heartbeat":
		age, freshness := evalCtx.GetHeartbeatFreshness()
		res.DataFreshness = freshness

		maxAge := time.Duration(cfg.MaxAgeSeconds) * time.Second
		if maxAge <= 0 {
			maxAge = evalCtx.FreshnessConfig.HeartbeatTolerance
		}

		res.ExpectedValue = fmt.Sprintf("heartbeat <= %s", maxAge)
		res.ObservedValue = fmt.Sprintf("age=%s", age.Round(time.Second))

		if freshness == model.DataFreshnessMissing {
			res.Status = model.EvaluationStatusNonCompliant
			res.Message = "node has never submitted a heartbeat"
		} else if age > maxAge {
			res.Status = model.EvaluationStatusNonCompliant
			res.Message = fmt.Sprintf("heartbeat age %s exceeds maximum allowed %s", age.Round(time.Second), maxAge)
		} else {
			res.Status = model.EvaluationStatusCompliant
			res.Message = fmt.Sprintf("heartbeat is fresh (age %s <= %s)", age.Round(time.Second), maxAge)
		}

	case "collector_version", "agent_version", "min_version":
		nodeVer := evalCtx.Node.Identity.Version
		res.ObservedValue = nodeVer
		res.ExpectedValue = cfg.ExpectedValue

		if nodeVer == "" {
			res.Status = model.EvaluationStatusNonCompliant
			res.Message = "node has no reported agent version"
			return res, nil
		}

		if satisfiesVersionConstraint(nodeVer, cfg.ExpectedValue) {
			res.Status = model.EvaluationStatusCompliant
			res.Message = fmt.Sprintf("collector version %s satisfies requirement '%s'", nodeVer, cfg.ExpectedValue)
		} else {
			res.Status = model.EvaluationStatusNonCompliant
			res.Message = fmt.Sprintf("collector version %s does not satisfy requirement '%s'", nodeVer, cfg.ExpectedValue)
		}

	case "mandatory_tags", "required_tags":
		expected := cfg.ExpectedValues
		if len(expected) == 0 && cfg.ExpectedValue != "" {
			expected = strings.Split(cfg.ExpectedValue, ",")
		}
		var cleanExpected []string
		for _, t := range expected {
			if trimmed := strings.TrimSpace(t); trimmed != "" {
				cleanExpected = append(cleanExpected, trimmed)
			}
		}

		res.ExpectedValue = strings.Join(cleanExpected, ", ")
		var missing []string
		for _, tagKey := range cleanExpected {
			if _, ok := evalCtx.Node.Identity.Tags[tagKey]; !ok {
				missing = append(missing, tagKey)
			}
		}

		if len(missing) > 0 {
			res.Status = model.EvaluationStatusNonCompliant
			res.ObservedValue = fmt.Sprintf("missing: %s", strings.Join(missing, ", "))
			res.Message = fmt.Sprintf("node is missing %d mandatory tag(s): %s", len(missing), strings.Join(missing, ", "))
		} else {
			res.Status = model.EvaluationStatusCompliant
			res.ObservedValue = "all tags present"
			res.Message = "all mandatory tags are present"
		}

	case "approved_regions", "allowed_regions":
		region := ""
		if evalCtx.Ownership != nil && evalCtx.Ownership.Region != "" {
			region = evalCtx.Ownership.Region
		} else if val, ok := evalCtx.Node.Identity.Tags["region"]; ok {
			region = val
		}

		res.ObservedValue = region
		res.ExpectedValue = strings.Join(cfg.ExpectedValues, ", ")

		if region == "" {
			res.Status = model.EvaluationStatusNonCompliant
			res.Message = "node has no assigned region metadata or tag"
		} else if slices.Contains(cfg.ExpectedValues, region) || (cfg.ExpectedValue != "" && region == cfg.ExpectedValue) {
			res.Status = model.EvaluationStatusCompliant
			res.Message = fmt.Sprintf("node region '%s' is approved", region)
		} else {
			res.Status = model.EvaluationStatusNonCompliant
			res.Message = fmt.Sprintf("node region '%s' is not in approved regions list", region)
		}

	case "approved_platforms", "allowed_platforms", "allowed_os":
		plat := evalCtx.Node.Identity.Platform
		if plat == "" {
			plat = evalCtx.Node.Identity.OS
		}
		res.ObservedValue = plat
		res.ExpectedValue = strings.Join(cfg.ExpectedValues, ", ")

		if slices.Contains(cfg.ExpectedValues, plat) || (cfg.ExpectedValue != "" && plat == cfg.ExpectedValue) {
			res.Status = model.EvaluationStatusCompliant
			res.Message = fmt.Sprintf("platform '%s' is approved", plat)
		} else {
			res.Status = model.EvaluationStatusNonCompliant
			res.Message = fmt.Sprintf("platform '%s' is not in approved platforms list", plat)
		}

	case "diagnostics_pass", "diagnostic_status":
		report, freshness := evalCtx.GetDiagnosticReport()
		res.DataFreshness = freshness

		if report == nil {
			res.Status = model.EvaluationStatusInsufficientData
			res.ObservedValue = "N/A"
			res.ExpectedValue = "PASS"
			res.Message = "no diagnostic report available for node"
		} else if report.OverallStatus == model.StatusFail {
			res.Status = model.EvaluationStatusNonCompliant
			res.ObservedValue = string(report.OverallStatus)
			res.ExpectedValue = "PASS"
			res.Message = fmt.Sprintf("latest diagnostic report failed (%d critical checks)", report.CriticalChecks)
		} else if report.OverallStatus == model.StatusWarning {
			res.Status = model.EvaluationStatusWarning
			res.ObservedValue = string(report.OverallStatus)
			res.ExpectedValue = "PASS"
			res.Message = fmt.Sprintf("latest diagnostic report has warnings (%d warning checks)", report.WarningChecks)
		} else {
			res.Status = model.EvaluationStatusCompliant
			res.ObservedValue = string(report.OverallStatus)
			res.ExpectedValue = "PASS"
			res.Message = "diagnostic checks passed"
		}

	default:
		res.Status = model.EvaluationStatusError
		res.Message = fmt.Sprintf("unsupported operational compliance check_type '%s'", cfg.CheckType)
	}

	return res, nil
}

// satisfiesVersionConstraint tests if a semver string meets an expected constraint (e.g., ">= 1.4.0", "1.4.2").
func satisfiesVersionConstraint(actual, constraint string) bool {
	actual = strings.TrimSpace(strings.TrimPrefix(actual, "v"))
	constraint = strings.TrimSpace(strings.TrimPrefix(constraint, "v"))

	if constraint == "" || constraint == "*" {
		return true
	}

	op := "=="
	target := constraint
	if strings.HasPrefix(constraint, ">=") {
		op = ">="
		target = strings.TrimSpace(strings.TrimPrefix(constraint, ">="))
	} else if strings.HasPrefix(constraint, "<=") {
		op = "<="
		target = strings.TrimSpace(strings.TrimPrefix(constraint, "<="))
	} else if strings.HasPrefix(constraint, ">") {
		op = ">"
		target = strings.TrimSpace(strings.TrimPrefix(constraint, ">"))
	} else if strings.HasPrefix(constraint, "<") {
		op = "<"
		target = strings.TrimSpace(strings.TrimPrefix(constraint, "<"))
	} else if strings.HasPrefix(constraint, "==") {
		op = "=="
		target = strings.TrimSpace(strings.TrimPrefix(constraint, "=="))
	}

	cmp := compareSemver(actual, target)
	switch op {
	case ">=":
		return cmp >= 0
	case "<=":
		return cmp <= 0
	case ">":
		return cmp > 0
	case "<":
		return cmp < 0
	default:
		return cmp == 0
	}
}

func compareSemver(v1, v2 string) int {
	parts1 := parseVersionParts(v1)
	parts2 := parseVersionParts(v2)

	for i := 0; i < 3; i++ {
		if parts1[i] < parts2[i] {
			return -1
		}
		if parts1[i] > parts2[i] {
			return 1
		}
	}
	return 0
}

func parseVersionParts(v string) [3]int {
	var parts [3]int
	segments := strings.Split(v, ".")
	for i := 0; i < len(segments) && i < 3; i++ {
		// Strip any prerelease suffix like -beta
		seg := strings.Split(segments[i], "-")[0]
		num, _ := strconv.Atoi(seg)
		parts[i] = num
	}
	return parts
}
