package intelligence

import (
	"math"
	"time"
)

// ScorePoint represents a historical health score measurement at a point in time.
type ScorePoint struct {
	Score     float64   `json:"score"`
	Timestamp time.Time `json:"timestamp"`
}

// TrajectoryConfig specifies the thresholds for trajectory classification.
type TrajectoryConfig struct {
	SlopeThreshold    float64 // Minimum point delta to classify as improving/degrading (default 5.0)
	VolatilityStdDev  float64 // StdDev threshold above which scores are classified as volatile (default 8.0)
	MinPointsRequired int     // Minimum samples needed for trajectory computation (default 2)
}

// DefaultTrajectoryConfig returns the default trajectory thresholds.
func DefaultTrajectoryConfig() TrajectoryConfig {
	return TrajectoryConfig{
		SlopeThreshold:    5.0,
		VolatilityStdDev:  8.0,
		MinPointsRequired: 2,
	}
}

// CalculateTrajectory determines the directional health trajectory from historical score points.
func CalculateTrajectory(points []ScorePoint, cfg *TrajectoryConfig) Trajectory {
	if cfg == nil {
		c := DefaultTrajectoryConfig()
		cfg = &c
	}

	if len(points) < cfg.MinPointsRequired {
		return TrajectoryUnknown
	}

	// Filter out invalid/NaN points
	var validPoints []ScorePoint
	for _, p := range points {
		if !math.IsNaN(p.Score) && !math.IsInf(p.Score, 0) {
			validPoints = append(validPoints, p)
		}
	}

	if len(validPoints) < cfg.MinPointsRequired {
		return TrajectoryUnknown
	}

	// 1. Calculate Standard Deviation to test for Volatility
	mean := 0.0
	for _, p := range validPoints {
		mean += p.Score
	}
	mean /= float64(len(validPoints))

	variance := 0.0
	for _, p := range validPoints {
		diff := p.Score - mean
		variance += diff * diff
	}
	variance /= float64(len(validPoints))
	stdDev := math.Sqrt(variance)

	// Count sign reversals / oscillations
	reversals := 0
	if len(validPoints) >= 4 {
		prevDelta := 0.0
		for i := 1; i < len(validPoints); i++ {
			delta := validPoints[i].Score - validPoints[i-1].Score
			if math.Abs(delta) > 2.0 {
				if (prevDelta > 0 && delta < 0) || (prevDelta < 0 && delta > 0) {
					reversals++
				}
				prevDelta = delta
			}
		}
	}

	if stdDev >= cfg.VolatilityStdDev && reversals >= 2 {
		return TrajectoryVolatile
	}

	// 2. Linear Regression Slope
	firstTime := validPoints[0].Timestamp.Unix()
	var n float64
	var sumX, sumY, sumXY, sumXX float64

	for _, p := range validPoints {
		x := float64(p.Timestamp.Unix() - firstTime)
		y := p.Score
		n++
		sumX += x
		sumY += y
		sumXY += x * y
		sumXX += x * x
	}

	denom := (n * sumXX) - (sumX * sumX)
	var overallDelta float64

	if denom != 0 && n > 2 {
		slope := ((n * sumXY) - (sumX * sumY)) / denom
		totalSeconds := float64(validPoints[len(validPoints)-1].Timestamp.Unix() - firstTime)
		if totalSeconds > 0 {
			overallDelta = slope * totalSeconds
		} else {
			overallDelta = validPoints[len(validPoints)-1].Score - validPoints[0].Score
		}
	} else {
		// Fallback to simple endpoint delta
		overallDelta = validPoints[len(validPoints)-1].Score - validPoints[0].Score
	}

	if overallDelta >= cfg.SlopeThreshold {
		return TrajectoryImproving
	} else if overallDelta <= -cfg.SlopeThreshold {
		return TrajectoryDegrading
	}

	if stdDev >= cfg.VolatilityStdDev {
		return TrajectoryVolatile
	}

	return TrajectoryStable
}
