package intelligence

import (
	"fmt"
	"math"
	"time"

	"github.com/DocHoax/watchdog/internal/storage"
)

// CorrelationEngine identifies non-causal statistical and temporal co-occurrences between signals.
type CorrelationEngine struct {
	window time.Duration
}

// NewCorrelationEngine creates a correlation analyzer.
func NewCorrelationEngine(window time.Duration) *CorrelationEngine {
	if window <= 0 {
		window = 1 * time.Hour
	}
	return &CorrelationEngine{window: window}
}

// CorrelateSignals evaluates temporal co-occurrence and Pearson correlation between two aligned time series.
func (ce *CorrelationEngine) CorrelateSignals(primaryName string, primaryPts []storage.MetricPoint,
	secondaryName string, secondaryPts []storage.MetricPoint) *Correlation {

	if len(primaryPts) < 5 || len(secondaryPts) < 5 {
		return nil
	}

	// Align time series into buckets of 10 seconds
	bucketSize := 10 * time.Second
	primaryBuckets := make(map[int64]float64)
	for _, p := range primaryPts {
		if !math.IsNaN(p.Value) && !math.IsInf(p.Value, 0) {
			b := p.Timestamp.Truncate(bucketSize).Unix()
			primaryBuckets[b] = p.Value
		}
	}

	var xVals, yVals []float64
	coOccurrenceCount := 0

	for _, p := range secondaryPts {
		if !math.IsNaN(p.Value) && !math.IsInf(p.Value, 0) {
			b := p.Timestamp.Truncate(bucketSize).Unix()
			if pVal, exists := primaryBuckets[b]; exists {
				xVals = append(xVals, pVal)
				yVals = append(yVals, p.Value)
				coOccurrenceCount++
			}
		}
	}

	if len(xVals) < 5 {
		return nil
	}

	coeff := calculatePearson(xVals, yVals)
	if math.IsNaN(coeff) || math.IsInf(coeff, 0) {
		coeff = 0.0
	}

	// Only surface correlations with meaningful magnitude (|r| >= 0.5)
	if math.Abs(coeff) < 0.5 {
		return nil
	}

	confidence := CorrelationConfidenceLow
	absCoeff := math.Abs(coeff)
	if absCoeff >= 0.8 && coOccurrenceCount >= 15 {
		confidence = CorrelationConfidenceHigh
	} else if absCoeff >= 0.6 && coOccurrenceCount >= 8 {
		confidence = CorrelationConfidenceMedium
	}

	relationship := "co-occurring with"
	if coeff < 0 {
		relationship = "inversely associated with"
	}

	description := fmt.Sprintf("Signal '%s' is temporally %s '%s' (Pearson r = %+.2f across %d aligned samples)",
		primaryName, relationship, secondaryName, coeff, coOccurrenceCount)

	return &Correlation{
		PrimarySignal:     primaryName,
		SecondarySignal:   secondaryName,
		Coefficient:       math.Round(coeff*100) / 100,
		TimeOffsetSeconds: 0,
		CoOccurrenceCount: coOccurrenceCount,
		Confidence:        confidence,
		Description:       description,
	}
}

// calculatePearson computes the Pearson product-moment correlation coefficient.
func calculatePearson(x, y []float64) float64 {
	n := float64(len(x))
	if n < 2 {
		return 0.0
	}

	var sumX, sumY, sumXY, sumXX, sumYY float64
	for i := 0; i < len(x); i++ {
		sumX += x[i]
		sumY += y[i]
		sumXY += x[i] * y[i]
		sumXX += x[i] * x[i]
		sumYY += y[i] * y[i]
	}

	numerator := (n * sumXY) - (sumX * sumY)
	denomX := (n * sumXX) - (sumX * sumX)
	denomY := (n * sumYY) - (sumY * sumY)

	if denomX <= 0 || denomY <= 0 {
		return 0.0
	}

	return numerator / (math.Sqrt(denomX) * math.Sqrt(denomY))
}
