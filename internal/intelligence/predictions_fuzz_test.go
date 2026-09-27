package intelligence

import (
	"math"
	"testing"
	"time"

	"github.com/DocHoax/watchdog/internal/storage"
	"github.com/DocHoax/watchdog/pkg/model"
)

// FuzzThresholdForecast fuzzes linear regression and threshold extrapolation logic against arbitrary inputs.
func FuzzThresholdForecast(f *testing.F) {
	f.Add(50.0, 80.0, 0.5, 20)
	f.Add(10.0, 90.0, -1.0, 15)
	f.Add(100.0, 80.0, 0.0, 30)
	f.Add(0.0, 0.0, 0.0, 0)
	f.Add(math.NaN(), 80.0, 1.0, 12)
	f.Add(math.Inf(1), 50.0, 0.5, 10)
	f.Add(-1000.0, 1000.0, 100.0, 50)

	f.Fuzz(func(t *testing.T, startVal float64, threshold float64, step float64, count int) {
		if count < 0 || count > 100 {
			return
		}

		now := time.Now().UTC()
		var points []storage.MetricPoint
		for i := 0; i < count; i++ {
			val := startVal + (float64(i) * step)
			points = append(points, storage.MetricPoint{
				Timestamp: now.Add(time.Duration(i) * time.Minute),
				Value:     val,
			})
		}

		pred := CalculatePrediction("node-fuzz", "cpu.usage_percent", points, threshold, 24*time.Hour, 1*time.Hour, nil)

		// Invariant checks:
		// 1. Never NaN or Inf in numeric fields
		if math.IsNaN(pred.CurrentValue) || math.IsInf(pred.CurrentValue, 0) {
			t.Errorf("CurrentValue must be finite, got: %f", pred.CurrentValue)
		}
		if math.IsNaN(pred.SlopePerMinute) || math.IsInf(pred.SlopePerMinute, 0) {
			t.Errorf("SlopePerMinute must be finite, got: %f", pred.SlopePerMinute)
		}
		if math.IsNaN(pred.RSquared) || math.IsInf(pred.RSquared, 0) {
			t.Errorf("RSquared must be finite, got: %f", pred.RSquared)
		}
		if math.IsNaN(pred.Variance) || math.IsInf(pred.Variance, 0) {
			t.Errorf("Variance must be finite, got: %f", pred.Variance)
		}

		// 2. Confidence must be one of the known enums
		switch pred.Confidence {
		case PredictionConfidenceHigh, PredictionConfidenceMedium, PredictionConfidenceLow, PredictionConfidenceInsufficientData:
			// valid
		default:
			t.Errorf("unknown confidence enum: %s", pred.Confidence)
		}

		// 3. Direction must be one of the known enums
		switch pred.Direction {
		case PredictionDirectionApproaching, PredictionDirectionReceding, PredictionDirectionStable, PredictionDirectionAlreadyExceeded, PredictionDirectionUnknown:
			// valid
		default:
			t.Errorf("unknown direction enum: %s", pred.Direction)
		}
	})
}

// FuzzPredictionConfidence fuzzes confidence tier evaluation against extreme statistical variance and sample distributions.
func FuzzPredictionConfidence(f *testing.F) {
	f.Add(25, 0.95, 0.1, 1000.0, 3600.0, true)
	f.Add(5, 0.99, 0.01, 100.0, 3600.0, true)
	f.Add(15, 0.50, 0.5, 5000.0, 3600.0, false)
	f.Add(30, -0.5, 10.0, 100.0, 3600.0, true)

	f.Fuzz(func(t *testing.T, sampleCount int, rSquared float64, cv float64, timeSec float64, bounded bool) {
		cfg := DefaultForecastConfig()
		conf := evaluateConfidence(rSquared, sampleCount, &cfg, bounded)

		switch conf {
		case PredictionConfidenceHigh, PredictionConfidenceMedium, PredictionConfidenceLow, PredictionConfidenceInsufficientData:
			// valid
		default:
			t.Errorf("unexpected confidence: %s", conf)
		}
	})
}

// FuzzRecurrenceDetection fuzzes recurrence pattern interval statistics against arbitrary incident streams.
func FuzzRecurrenceDetection(f *testing.F) {
	f.Add(4, 3600.0, 60.0)
	f.Add(1, 100.0, 0.0)
	f.Add(10, 600.0, 300.0)
	f.Add(0, 0.0, 0.0)

	f.Fuzz(func(t *testing.T, count int, baseIntervalSec float64, jitterSec float64) {
		if count < 0 || count > 50 {
			return
		}
		if baseIntervalSec <= 0 || baseIntervalSec > 86400 {
			return
		}

		now := time.Now().UTC()
		var incidents []Incident
		currTime := now.Add(-time.Duration(count) * time.Duration(baseIntervalSec*float64(time.Second)))

		for i := 0; i < count; i++ {
			inc := Incident{
				ID:              generateIncidentID([]string{"fuzz-node"}, "fuzz_event", currTime),
				Title:           "Fuzz Incident",
				AffectedNodes:   []string{"fuzz-node"},
				PrimarySymptoms: []string{"fuzz_event"},
				Severity:        model.SeverityWarning,
				StartTime:       currTime,
			}
			incidents = append(incidents, inc)
			currTime = currTime.Add(time.Duration((baseIntervalSec + jitterSec) * float64(time.Second)))
		}

		patterns := DetectRecurrencePatterns(incidents, 7*24*time.Hour, nil)
		for _, p := range patterns {
			if math.IsNaN(p.CoefficientOfVariation) || math.IsInf(p.CoefficientOfVariation, 0) {
				t.Errorf("CV must be finite, got: %f", p.CoefficientOfVariation)
			}
			if p.OccurrenceCount < 3 {
				t.Errorf("patterns must have at least 3 occurrences, got: %d", p.OccurrenceCount)
			}
		}
	})
}
