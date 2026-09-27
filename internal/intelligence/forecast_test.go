package intelligence

import (
	"math"
	"testing"
	"time"

	"github.com/DocHoax/watchdog/internal/storage"
)

func TestCalculatePrediction_InsufficientSamples(t *testing.T) {
	nodeID := "node-test-01"
	metric := "cpu.usage_percent"

	// Fewer than 10 samples
	var points []storage.MetricPoint
	now := time.Now().UTC()
	for i := 0; i < 5; i++ {
		points = append(points, storage.MetricPoint{
			Timestamp: now.Add(time.Duration(i) * time.Minute),
			Value:     float64(50 + i),
		})
	}

	pred := CalculatePrediction(nodeID, metric, points, 80.0, 24*time.Hour, 1*time.Hour, nil)

	if pred.Confidence != PredictionConfidenceInsufficientData {
		t.Errorf("expected confidence insufficient_data, got: %s", pred.Confidence)
	}
	if pred.SampleCount != 5 {
		t.Errorf("expected 5 samples, got: %d", pred.SampleCount)
	}
	if pred.EstimatedTimeToThreshold != nil {
		t.Errorf("expected nil EstimatedTimeToThreshold for insufficient data")
	}
}

func TestCalculatePrediction_PositiveSlope_Approaching(t *testing.T) {
	nodeID := "node-test-01"
	metric := "memory.usage_percent"

	// Create 20 linearly increasing points: 50.0 to 60.0 over 20 minutes (slope = 0.5 per minute)
	now := time.Now().UTC()
	var points []storage.MetricPoint
	for i := 0; i < 20; i++ {
		points = append(points, storage.MetricPoint{
			Timestamp: now.Add(time.Duration(i) * time.Minute),
			Value:     50.0 + (float64(i) * 0.5) + 0.5, // 50.5 to 60.0
		})
	}

	targetThreshold := 80.0
	horizon := 24 * time.Hour
	pred := CalculatePrediction(nodeID, metric, points, targetThreshold, horizon, 1*time.Hour, nil)

	if pred.Direction != PredictionDirectionApproaching {
		t.Errorf("expected direction approaching, got: %s", pred.Direction)
	}
	if pred.SlopePerMinute < 0.49 || pred.SlopePerMinute > 0.51 {
		t.Errorf("expected slope per min ~0.5, got: %f", pred.SlopePerMinute)
	}
	if pred.RSquared < 0.99 {
		t.Errorf("expected R² close to 1.0, got: %f", pred.RSquared)
	}
	if pred.Confidence != PredictionConfidenceHigh {
		t.Errorf("expected high confidence, got: %s", pred.Confidence)
	}
	if pred.EstimatedTimeToThreshold == nil {
		t.Fatalf("expected non-nil EstimatedTimeToThreshold")
	}

	// Current is 60.0, target is 80.0, delta = 20.0, slope = 0.5/min -> 40 minutes (2400s)
	expectedDur := 40 * time.Minute
	diff := math.Abs(pred.EstimatedTimeToThreshold.Seconds() - expectedDur.Seconds())
	if diff > 10.0 {
		t.Errorf("expected time to threshold ~40m (2400s), got: %v", pred.EstimatedTimeToThreshold)
	}
}

func TestCalculatePrediction_AlreadyExceeded(t *testing.T) {
	nodeID := "node-test-01"
	metric := "disk.usage_percent"

	now := time.Now().UTC()
	var points []storage.MetricPoint
	for i := 0; i < 15; i++ {
		points = append(points, storage.MetricPoint{
			Timestamp: now.Add(time.Duration(i) * time.Minute),
			Value:     90.0 + float64(i),
		})
	}

	targetThreshold := 85.0 // Current is > 90.0, so already exceeded
	pred := CalculatePrediction(nodeID, metric, points, targetThreshold, 24*time.Hour, 1*time.Hour, nil)

	if pred.Direction != PredictionDirectionAlreadyExceeded {
		t.Errorf("expected already_exceeded direction, got: %s", pred.Direction)
	}
	if pred.EstimatedTimeToThreshold == nil || *pred.EstimatedTimeToThreshold != 0 {
		t.Errorf("expected time to threshold 0, got: %v", pred.EstimatedTimeToThreshold)
	}
	if pred.Confidence != PredictionConfidenceHigh {
		t.Errorf("expected high confidence for already exceeded, got: %s", pred.Confidence)
	}
}

func TestCalculatePrediction_NegativeSlope_Receding(t *testing.T) {
	nodeID := "node-test-01"
	metric := "cpu.usage_percent"

	now := time.Now().UTC()
	var points []storage.MetricPoint
	for i := 0; i < 15; i++ {
		points = append(points, storage.MetricPoint{
			Timestamp: now.Add(time.Duration(i) * time.Minute),
			Value:     80.0 - float64(i)*1.5, // Decreasing
		})
	}

	targetThreshold := 90.0
	pred := CalculatePrediction(nodeID, metric, points, targetThreshold, 24*time.Hour, 1*time.Hour, nil)

	if pred.Direction != PredictionDirectionReceding {
		t.Errorf("expected receding direction, got: %s", pred.Direction)
	}
	if pred.EstimatedTimeToThreshold != nil {
		t.Errorf("expected nil time to threshold for receding metric")
	}
}

func TestCalculatePrediction_Flatline_Stable(t *testing.T) {
	nodeID := "node-test-01"
	metric := "memory.usage_percent"

	now := time.Now().UTC()
	var points []storage.MetricPoint
	for i := 0; i < 15; i++ {
		points = append(points, storage.MetricPoint{
			Timestamp: now.Add(time.Duration(i) * time.Minute),
			Value:     45.0, // Constant
		})
	}

	targetThreshold := 85.0
	pred := CalculatePrediction(nodeID, metric, points, targetThreshold, 24*time.Hour, 1*time.Hour, nil)

	if pred.Direction != PredictionDirectionStable {
		t.Errorf("expected stable direction, got: %s", pred.Direction)
	}
	if pred.EstimatedTimeToThreshold != nil {
		t.Errorf("expected nil time to threshold for stable metric")
	}
}

func TestCalculatePrediction_BeyondHorizon(t *testing.T) {
	nodeID := "node-test-01"
	metric := "disk.usage_percent"

	// Slope is very gentle: +0.01 per minute. Current: 50.0, Target: 90.0 (delta = 40.0 -> 4000 min = ~66.6 hours)
	now := time.Now().UTC()
	var points []storage.MetricPoint
	for i := 0; i < 20; i++ {
		points = append(points, storage.MetricPoint{
			Timestamp: now.Add(time.Duration(i) * time.Minute),
			Value:     50.0 + (float64(i) * 0.01),
		})
	}

	targetThreshold := 90.0
	horizon := 1 * time.Hour // 1 hour horizon, crossing is at 66 hours
	pred := CalculatePrediction(nodeID, metric, points, targetThreshold, horizon, 1*time.Hour, nil)

	if pred.Direction != PredictionDirectionApproaching {
		t.Errorf("expected approaching direction, got: %s", pred.Direction)
	}
	if pred.EstimatedTimeToThreshold != nil {
		t.Errorf("expected nil time to threshold when crossing exceeds horizon")
	}
	if pred.Confidence != PredictionConfidenceLow {
		t.Errorf("expected low confidence when crossing exceeds horizon, got: %s", pred.Confidence)
	}
}

func TestCalculatePrediction_NonFiniteFiltering(t *testing.T) {
	nodeID := "node-test-01"
	metric := "cpu.usage_percent"

	now := time.Now().UTC()
	var points []storage.MetricPoint
	for i := 0; i < 15; i++ {
		val := 30.0 + float64(i)
		if i == 5 {
			val = math.NaN()
		} else if i == 10 {
			val = math.Inf(1)
		}
		points = append(points, storage.MetricPoint{
			Timestamp: now.Add(time.Duration(i) * time.Minute),
			Value:     val,
		})
	}

	pred := CalculatePrediction(nodeID, metric, points, 90.0, 24*time.Hour, 1*time.Hour, nil)

	// 15 total minus 2 invalid = 13 valid samples >= 10 min samples
	if pred.SampleCount != 13 {
		t.Errorf("expected 13 valid samples, got: %d", pred.SampleCount)
	}
	if math.IsNaN(pred.SlopePerMinute) || math.IsInf(pred.SlopePerMinute, 0) {
		t.Errorf("slope must be finite, got: %f", pred.SlopePerMinute)
	}
	if math.IsNaN(pred.RSquared) || math.IsInf(pred.RSquared, 0) {
		t.Errorf("R² must be finite, got: %f", pred.RSquared)
	}
}

func TestCalculatePrediction_ZeroTimestampVariance(t *testing.T) {
	nodeID := "node-test-01"
	metric := "memory.usage_percent"

	sameTime := time.Now().UTC()
	var points []storage.MetricPoint
	for i := 0; i < 12; i++ {
		points = append(points, storage.MetricPoint{
			Timestamp: sameTime, // identical timestamps
			Value:     50.0 + float64(i),
		})
	}

	pred := CalculatePrediction(nodeID, metric, points, 80.0, 24*time.Hour, 1*time.Hour, nil)

	if pred.Direction != PredictionDirectionStable {
		t.Errorf("expected stable direction for zero timestamp variance, got: %s", pred.Direction)
	}
	if pred.SlopePerMinute != 0.0 {
		t.Errorf("expected 0 slope, got: %f", pred.SlopePerMinute)
	}
}

func TestSanitizeFloat(t *testing.T) {
	tests := []struct {
		name     string
		input    float64
		expected float64
	}{
		{"NaN", math.NaN(), 0.0},
		{"PosInf", math.Inf(1), 0.0},
		{"NegInf", math.Inf(-1), 0.0},
		{"ValidNormal", 42.123456, 42.1235},
		{"Zero", 0.0, 0.0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := sanitizeFloat(tc.input)
			if got != tc.expected {
				t.Errorf("sanitizeFloat(%v) = %v; expected %v", tc.input, got, tc.expected)
			}
		})
	}
}
