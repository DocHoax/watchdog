package intelligence

import (
	"math"
	"time"

	"github.com/DocHoax/watchdog/internal/storage"
)

// TrendConfig defines thresholds and parameters for trend detection.
type TrendConfig struct {
	MinPointsRequired int     // Minimum points required for calculation (default 3)
	StabilityEpsilon  float64 // Minimum hourly rate of change to classify as increasing/decreasing (default 0.5)
}

// DefaultTrendConfig returns default configuration for trend analysis.
func DefaultTrendConfig() TrendConfig {
	return TrendConfig{
		MinPointsRequired: 3,
		StabilityEpsilon:  0.5,
	}
}

// CalculateTrend performs ordinary least squares linear regression over time-series metric points.
func CalculateTrend(metric string, points []storage.MetricPoint, window time.Duration, unit string, cfg *TrendConfig) HealthTrend {
	if cfg == nil {
		c := DefaultTrendConfig()
		cfg = &c
	}

	res := HealthTrend{
		Metric:     metric,
		Direction:  TrendDirectionInsufficientData,
		Unit:       unit,
		Window:     window,
		Confidence: 0.0,
	}

	// Filter out non-finite metric values
	var valid []storage.MetricPoint
	for _, p := range points {
		if !math.IsNaN(p.Value) && !math.IsInf(p.Value, 0) {
			valid = append(valid, p)
		}
	}

	if len(valid) < cfg.MinPointsRequired {
		if len(valid) > 0 {
			res.StartValue = valid[0].Value
			res.EndValue = valid[len(valid)-1].Value
		}
		return res
	}

	res.StartValue = valid[0].Value
	res.EndValue = valid[len(valid)-1].Value

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
		// All timestamps identical or zero variance in time
		res.Direction = TrendDirectionStable
		res.RateOfChange = 0.0
		res.Confidence = 1.0
		return res
	}

	slope := ((n * sumXY) - (sumX * sumY)) / denom
	intercept := (sumY - (slope * sumX)) / n

	// Hourly rate of change: slope is change per second, multiply by 3600
	hourlyRate := slope * 3600.0
	res.RateOfChange = math.Round(hourlyRate*1000) / 1000

	// Calculate R-squared (coefficient of determination)
	yMean := sumY / n
	var ssTot, ssRes float64
	for _, p := range valid {
		x := float64(p.Timestamp.Unix() - firstTime)
		y := p.Value
		predicted := (slope * x) + intercept
		ssTot += (y - yMean) * (y - yMean)
		ssRes += (y - predicted) * (y - predicted)
	}

	rSquared := 0.0
	if ssTot > 0 {
		rSquared = math.Max(0.0, math.Min(1.0, 1.0-(ssRes/ssTot)))
	} else {
		// Flatline (zero total variance in Y)
		rSquared = 1.0
	}

	// Weight confidence by sample size up to 20 samples
	sampleWeight := math.Min(1.0, float64(len(valid))/20.0)
	res.Confidence = math.Round((rSquared*0.7+sampleWeight*0.3)*100) / 100

	// Direction classification
	if math.Abs(hourlyRate) < cfg.StabilityEpsilon {
		res.Direction = TrendDirectionStable
	} else if hourlyRate > 0 {
		res.Direction = TrendDirectionIncreasing
	} else {
		res.Direction = TrendDirectionDecreasing
	}

	return res
}
