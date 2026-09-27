package intelligence

import (
	"math"
	"testing"
	"time"

	"github.com/DocHoax/watchdog/internal/storage"
)

func TestCorrelateSignals(t *testing.T) {
	now := time.Now().Truncate(10 * time.Second)
	ce := NewCorrelationEngine(1 * time.Hour)

	t.Run("Insufficient Data Points", func(t *testing.T) {
		ptsA := []storage.MetricPoint{
			{Timestamp: now, Value: 10.0},
			{Timestamp: now.Add(10 * time.Second), Value: 20.0},
		}
		ptsB := []storage.MetricPoint{
			{Timestamp: now, Value: 100.0},
			{Timestamp: now.Add(10 * time.Second), Value: 200.0},
		}
		corr := ce.CorrelateSignals("cpu", ptsA, "memory", ptsB)
		if corr != nil {
			t.Errorf("expected nil correlation for insufficient data, got %+v", corr)
		}
	})

	t.Run("Strong Positive Correlation (High Confidence)", func(t *testing.T) {
		ptsA := make([]storage.MetricPoint, 20)
		ptsB := make([]storage.MetricPoint, 20)

		for i := 0; i < 20; i++ {
			ts := now.Add(time.Duration(i*10) * time.Second)
			ptsA[i] = storage.MetricPoint{Timestamp: ts, Value: float64(10 + i*2)}
			ptsB[i] = storage.MetricPoint{Timestamp: ts, Value: float64(50 + i*4)}
		}

		corr := ce.CorrelateSignals("cpu_usage", ptsA, "load_avg", ptsB)
		if corr == nil {
			t.Fatalf("expected correlation result, got nil")
		}
		if corr.Coefficient < 0.95 {
			t.Errorf("expected Pearson r >= 0.95, got %.2f", corr.Coefficient)
		}
		if corr.Confidence != CorrelationConfidenceHigh {
			t.Errorf("expected %s, got %s", CorrelationConfidenceHigh, corr.Confidence)
		}
		if corr.CoOccurrenceCount != 20 {
			t.Errorf("expected 20 co-occurrences, got %d", corr.CoOccurrenceCount)
		}
	})

	t.Run("Strong Negative Correlation", func(t *testing.T) {
		ptsA := make([]storage.MetricPoint, 20)
		ptsB := make([]storage.MetricPoint, 20)

		for i := 0; i < 20; i++ {
			ts := now.Add(time.Duration(i*10) * time.Second)
			ptsA[i] = storage.MetricPoint{Timestamp: ts, Value: float64(10 + i)}
			ptsB[i] = storage.MetricPoint{Timestamp: ts, Value: float64(100 - i)}
		}

		corr := ce.CorrelateSignals("disk_used", ptsA, "disk_free", ptsB)
		if corr == nil {
			t.Fatalf("expected correlation result, got nil")
		}
		if corr.Coefficient > -0.95 {
			t.Errorf("expected Pearson r <= -0.95, got %.2f", corr.Coefficient)
		}
		if corr.Confidence != CorrelationConfidenceHigh {
			t.Errorf("expected %s, got %s", CorrelationConfidenceHigh, corr.Confidence)
		}
	})

	t.Run("Uncorrelated Signals Filtered Out", func(t *testing.T) {
		ptsA := make([]storage.MetricPoint, 20)
		ptsB := make([]storage.MetricPoint, 20)

		for i := 0; i < 20; i++ {
			ts := now.Add(time.Duration(i*10) * time.Second)
			ptsA[i] = storage.MetricPoint{Timestamp: ts, Value: float64(i)}
			// Alternating values with zero correlation to linear trend
			if i%2 == 0 {
				ptsB[i] = storage.MetricPoint{Timestamp: ts, Value: 10.0}
			} else {
				ptsB[i] = storage.MetricPoint{Timestamp: ts, Value: 100.0}
			}
		}

		corr := ce.CorrelateSignals("signal_a", ptsA, "signal_b", ptsB)
		if corr != nil && math.Abs(corr.Coefficient) >= 0.5 {
			t.Errorf("expected nil or low correlation, got %+v", corr)
		}
	})
}
