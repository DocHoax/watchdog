package intelligence

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"time"

	"github.com/DocHoax/watchdog/internal/storage"
)

// ForecastConfig defines parameters for deterministic threshold forecasting.
type ForecastConfig struct {
	MinSamples                int           // Minimum samples required (default: 10)
	MaxHorizon                time.Duration // Maximum extrapolation horizon (default: 7*24h)
	SlopeEpsilon              float64       // Minimum slope per minute to classify as approaching/receding (default: 1e-6)
	HighConfidenceRSquared    float64       // Minimum R² for high confidence (default: 0.70)
	MediumConfidenceRSquared  float64       // Minimum R² for medium confidence (default: 0.40)
	HighConfidenceSampleCount int           // Minimum sample count for high confidence (default: 20)
}

// DefaultForecastConfig returns standard production defaults for linear threshold forecasting.
func DefaultForecastConfig() ForecastConfig {
	return ForecastConfig{
		MinSamples:                10,
		MaxHorizon:                7 * 24 * time.Hour,
		SlopeEpsilon:              1e-6,
		HighConfidenceRSquared:    0.70,
		MediumConfidenceRSquared:  0.40,
		HighConfidenceSampleCount: 20,
	}
}

// CalculatePrediction performs deterministic closed-form linear threshold forecasting.
// It computes slope per minute, R² goodness-of-fit, variance, and estimates time-to-threshold.
func CalculatePrediction(
	nodeID string,
	metric string,
	points []storage.MetricPoint,
	targetThreshold float64,
	horizon time.Duration,
	observationWindow time.Duration,
	cfg *ForecastConfig,
) Prediction {
	if cfg == nil {
		c := DefaultForecastConfig()
		cfg = &c
	}
	if horizon <= 0 {
		horizon = 24 * time.Hour
	}
	if horizon > cfg.MaxHorizon {
		horizon = cfg.MaxHorizon
	}

	now := time.Now().UTC()
	predID := generatePredictionID(nodeID, metric, now)

	pred := Prediction{
		ID:                predID,
		NodeID:            nodeID,
		Metric:            metric,
		TargetThreshold:   sanitizeFloat(targetThreshold),
		Direction:         PredictionDirectionUnknown,
		Confidence:        PredictionConfidenceInsufficientData,
		Horizon:           horizon,
		ObservationWindow: observationWindow,
		Method:            "ordinary_least_squares_linear_projection",
		GeneratedAt:       now,
		Evidence:          make([]string, 0),
	}

	// Filter non-finite points (NaN, Inf)
	var valid []storage.MetricPoint
	for _, p := range points {
		if !math.IsNaN(p.Value) && !math.IsInf(p.Value, 0) {
			valid = append(valid, p)
		}
	}
	pred.SampleCount = len(valid)

	if len(valid) < cfg.MinSamples {
		if len(valid) > 0 {
			pred.CurrentValue = sanitizeFloat(valid[len(valid)-1].Value)
		}
		pred.Evidence = append(pred.Evidence,
			fmt.Sprintf("Insufficient data: %d valid samples collected, minimum %d required for threshold projection", len(valid), cfg.MinSamples),
		)
		return pred
	}

	currentVal := valid[len(valid)-1].Value
	pred.CurrentValue = sanitizeFloat(currentVal)

	firstTime := valid[0].Timestamp.Unix()
	var n float64
	var sumX, sumY, sumXY, sumXX float64

	for _, p := range valid {
		x := float64(p.Timestamp.Unix() - firstTime)
		y := p.Value
		n++
		sumX += x
		sumY += y
		sumXY += x * y
		sumXX += x * x
	}

	denom := (n * sumXX) - (sumX * sumX)
	if denom == 0 {
		// All timestamps identical
		pred.Direction = PredictionDirectionStable
		pred.SlopePerMinute = 0.0
		pred.RSquared = 1.0
		pred.Variance = 0.0
		pred.Confidence = PredictionConfidenceMedium
		pred.Evidence = append(pred.Evidence, "Metric is stable: zero timestamp variance across samples")
		return pred
	}

	slopePerSec := ((n * sumXY) - (sumX * sumY)) / denom
	intercept := (sumY - (slopePerSec * sumX)) / n
	slopePerMin := slopePerSec * 60.0

	pred.SlopePerMinute = sanitizeFloat(slopePerMin)

	// Compute R-squared (coefficient of determination) and sample variance
	yMean := sumY / n
	var ssTot, ssRes float64
	for _, p := range valid {
		x := float64(p.Timestamp.Unix() - firstTime)
		y := p.Value
		predicted := (slopePerSec * x) + intercept
		ssTot += (y - yMean) * (y - yMean)
		ssRes += (y - predicted) * (y - predicted)
	}

	rSquared := 0.0
	if ssTot > 0 {
		rSquared = math.Max(0.0, math.Min(1.0, 1.0-(ssRes/ssTot)))
	} else {
		// Flatline with 0 variance in Y
		rSquared = 1.0
	}
	pred.RSquared = sanitizeFloat(rSquared)

	variance := 0.0
	if n > 1 {
		variance = ssTot / (n - 1)
	}
	pred.Variance = sanitizeFloat(variance)

	// Threshold evaluation & projection
	if currentVal >= targetThreshold {
		pred.Direction = PredictionDirectionAlreadyExceeded
		zeroDur := time.Duration(0)
		pred.EstimatedTimeToThreshold = &zeroDur
		crossingTime := now
		pred.PredictedCrossingTime = &crossingTime
		pred.Confidence = PredictionConfidenceHigh
		pred.Evidence = append(pred.Evidence,
			fmt.Sprintf("Target threshold %.2f already exceeded (current: %.2f)", targetThreshold, currentVal),
		)
		return pred
	}

	if math.Abs(slopePerMin) < cfg.SlopeEpsilon {
		pred.Direction = PredictionDirectionStable
		pred.Confidence = evaluateConfidence(pred.RSquared, len(valid), cfg, false)
		pred.Evidence = append(pred.Evidence,
			fmt.Sprintf("Metric is stable (slope: %.6f/min, current: %.2f, threshold: %.2f, R²: %.2f)",
				slopePerMin, currentVal, targetThreshold, rSquared),
		)
		return pred
	}

	if slopePerMin < 0 {
		pred.Direction = PredictionDirectionReceding
		pred.Confidence = evaluateConfidence(pred.RSquared, len(valid), cfg, false)
		pred.Evidence = append(pred.Evidence,
			fmt.Sprintf("Metric is receding away from threshold (slope: %.4f/min, current: %.2f, threshold: %.2f, R²: %.2f)",
				slopePerMin, currentVal, targetThreshold, rSquared),
		)
		return pred
	}

	// Approaching threshold with positive slope
	pred.Direction = PredictionDirectionApproaching
	deltaNeeded := targetThreshold - currentVal
	timeSec := deltaNeeded / slopePerSec

	if timeSec > 0 && !math.IsNaN(timeSec) && !math.IsInf(timeSec, 0) {
		timeDur := time.Duration(timeSec * float64(time.Second))
		if timeDur <= horizon && timeDur <= cfg.MaxHorizon {
			pred.EstimatedTimeToThreshold = &timeDur
			crossingTime := now.Add(timeDur)
			pred.PredictedCrossingTime = &crossingTime
			pred.Confidence = evaluateConfidence(pred.RSquared, len(valid), cfg, true)
			pred.Evidence = append(pred.Evidence,
				fmt.Sprintf("Approaching threshold %.2f in %v (current: %.2f, slope: +%.4f/min, R²: %.2f, N: %d)",
					targetThreshold, timeDur.Round(time.Second), currentVal, slopePerMin, rSquared, len(valid)),
			)
		} else {
			// Projected crossing is beyond the requested horizon
			pred.Confidence = PredictionConfidenceLow
			pred.Evidence = append(pred.Evidence,
				fmt.Sprintf("Approaching threshold %.2f but projected crossing (%v) exceeds forecast horizon (%v)",
					targetThreshold, time.Duration(timeSec*float64(time.Second)).Round(time.Minute), horizon),
			)
		}
	} else {
		pred.Confidence = PredictionConfidenceLow
		pred.Evidence = append(pred.Evidence, "Unable to extrapolate reliable crossing time due to invalid time calculation")
	}

	return pred
}

// evaluateConfidence determines confidence level from R², sample count, and horizon criteria.
func evaluateConfidence(rSquared float64, sampleCount int, cfg *ForecastConfig, boundedWithinHorizon bool) PredictionConfidence {
	if sampleCount < cfg.MinSamples {
		return PredictionConfidenceInsufficientData
	}
	if rSquared >= cfg.HighConfidenceRSquared && sampleCount >= cfg.HighConfidenceSampleCount && boundedWithinHorizon {
		return PredictionConfidenceHigh
	}
	if (rSquared >= cfg.MediumConfidenceRSquared || sampleCount >= cfg.MinSamples) && boundedWithinHorizon {
		return PredictionConfidenceMedium
	}
	if rSquared >= cfg.MediumConfidenceRSquared && !boundedWithinHorizon {
		return PredictionConfidenceMedium
	}
	return PredictionConfidenceLow
}

// sanitizeFloat guarantees valid non-NaN, non-Inf finite float64 values.
func sanitizeFloat(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0.0
	}
	return math.Round(v*10000) / 10000
}

// generatePredictionID creates a deterministic identifier for a prediction.
func generatePredictionID(nodeID, metric string, t time.Time) string {
	raw := fmt.Sprintf("%s:%s:%d", nodeID, metric, t.UnixNano())
	hash := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("pred-%s-%s-%s", nodeID, metric, hex.EncodeToString(hash[:4]))
}
