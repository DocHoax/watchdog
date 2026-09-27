package intelligence

import (
	"math"
	"testing"
	"time"

	"github.com/DocHoax/watchdog/internal/storage"
)

func TestCalculateTrend(t *testing.T) {
	now := time.Now()

	t.Run("Insufficient Data", func(t *testing.T) {
		points := []storage.MetricPoint{
			{Timestamp: now.Add(-10 * time.Minute), Value: 20.0},
			{Timestamp: now, Value: 25.0},
		}
		trend := CalculateTrend("cpu_usage", points, 1*time.Hour, "%", nil)
		if trend.Direction != TrendDirectionInsufficientData {
			t.Errorf("expected %s, got %s", TrendDirectionInsufficientData, trend.Direction)
		}
		if trend.StartValue != 20.0 || trend.EndValue != 25.0 {
			t.Errorf("unexpected start/end values: start=%.1f end=%.1f", trend.StartValue, trend.EndValue)
		}
	})

	t.Run("Linear Increasing Trend", func(t *testing.T) {
		// Value increases by 1.0 every 60 seconds (rate = 60.0 units/hour)
		points := make([]storage.MetricPoint, 10)
		for i := 0; i < 10; i++ {
			points[i] = storage.MetricPoint{
				Timestamp: now.Add(time.Duration(i*60) * time.Second),
				Value:     float64(10 + i),
			}
		}

		trend := CalculateTrend("memory_used_percent", points, 10*time.Minute, "%", nil)
		if trend.Direction != TrendDirectionIncreasing {
			t.Errorf("expected %s, got %s", TrendDirectionIncreasing, trend.Direction)
		}
		if math.Abs(trend.RateOfChange-60.0) > 0.5 {
			t.Errorf("expected rate of change ~60.0/hr, got %.2f", trend.RateOfChange)
		}
		if trend.Confidence < 0.5 {
			t.Errorf("expected high confidence for perfect line, got %.2f", trend.Confidence)
		}
	})

	t.Run("Linear Decreasing Trend", func(t *testing.T) {
		// Value decreases by 0.5 every 60 seconds (rate = -30.0 units/hour)
		points := make([]storage.MetricPoint, 10)
		for i := 0; i < 10; i++ {
			points[i] = storage.MetricPoint{
				Timestamp: now.Add(time.Duration(i*60) * time.Second),
				Value:     float64(50) - float64(i)*0.5,
			}
		}

		trend := CalculateTrend("disk_free_gb", points, 10*time.Minute, "GB", nil)
		if trend.Direction != TrendDirectionDecreasing {
			t.Errorf("expected %s, got %s", TrendDirectionDecreasing, trend.Direction)
		}
		if math.Abs(trend.RateOfChange-(-30.0)) > 0.5 {
			t.Errorf("expected rate of change ~ -30.0/hr, got %.2f", trend.RateOfChange)
		}
	})

	t.Run("Flatline Stable Trend", func(t *testing.T) {
		points := make([]storage.MetricPoint, 10)
		for i := 0; i < 10; i++ {
			points[i] = storage.MetricPoint{
				Timestamp: now.Add(time.Duration(i*60) * time.Second),
				Value:     42.0,
			}
		}

		trend := CalculateTrend("cpu_usage", points, 10*time.Minute, "%", nil)
		if trend.Direction != TrendDirectionStable {
			t.Errorf("expected %s, got %s", TrendDirectionStable, trend.Direction)
		}
		if trend.RateOfChange != 0.0 {
			t.Errorf("expected rate of change 0.0, got %.2f", trend.RateOfChange)
		}
	})

	t.Run("Filters NaN and Inf values", func(t *testing.T) {
		points := []storage.MetricPoint{
			{Timestamp: now.Add(-3 * time.Minute), Value: 10.0},
			{Timestamp: now.Add(-2 * time.Minute), Value: math.NaN()},
			{Timestamp: now.Add(-1 * time.Minute), Value: math.Inf(1)},
			{Timestamp: now, Value: 20.0},
		}

		trend := CalculateTrend("cpu_usage", points, 3*time.Minute, "%", nil)
		// Only 2 valid points, less than required 3
		if trend.Direction != TrendDirectionInsufficientData {
			t.Errorf("expected %s due to NaN/Inf filtering, got %s", TrendDirectionInsufficientData, trend.Direction)
		}
	})
}

func TestCalculateBaseline(t *testing.T) {
	now := time.Now()

	t.Run("Empty points", func(t *testing.T) {
		b := CalculateBaseline("cpu_usage", nil, 1*time.Hour)
		if b.SampleCount != 0 {
			t.Errorf("expected sample count 0, got %d", b.SampleCount)
		}
	})

	t.Run("Verified statistical distribution", func(t *testing.T) {
		// Values: 10, 20, 30, 40, 50, 60, 70, 80, 90, 100
		points := make([]storage.MetricPoint, 10)
		for i := 0; i < 10; i++ {
			points[i] = storage.MetricPoint{
				Timestamp: now.Add(time.Duration(i*60) * time.Second),
				Value:     float64((i + 1) * 10),
			}
		}

		b := CalculateBaseline("cpu_usage", points, 10*time.Minute)
		if b.SampleCount != 10 {
			t.Errorf("expected sample count 10, got %d", b.SampleCount)
		}
		if b.Min != 10.0 {
			t.Errorf("expected min 10.0, got %.2f", b.Min)
		}
		if b.Max != 100.0 {
			t.Errorf("expected max 100.0, got %.2f", b.Max)
		}
		if b.Mean != 55.0 {
			t.Errorf("expected mean 55.0, got %.2f", b.Mean)
		}
		// Population/sample standard deviation of [10..100] is ~28.72
		if math.Abs(b.StdDev-28.72) > 0.1 {
			t.Errorf("expected stddev ~28.72, got %.2f", b.StdDev)
		}
		// P50 should be 55.0
		if b.P50 != 55.0 {
			t.Errorf("expected P50 55.0, got %.2f", b.P50)
		}
		// P90 should be 91.0
		if math.Abs(b.P90-91.0) > 1.0 {
			t.Errorf("expected P90 ~91.0, got %.2f", b.P90)
		}
	})
}
