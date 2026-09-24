package anomaly

import (
	"math"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/DocHoax/watchdog/internal/config"
	"github.com/DocHoax/watchdog/pkg/model"
)

// Scenario 1: Constant series: mean=X, stddev=0, no false anomalies
func TestAnomaly_Scenario1_ConstantSeries(t *testing.T) {
	cfg := &config.AnomalyConfig{
		Enabled:         true,
		WindowSize:      60,
		ZScoreThreshold: 3.0,
		Alpha:           0.2,
	}
	detector := NewDetector(cfg)
	metric := "cpu_constant"
	val := 42.0

	for i := range 50 {
		score := detector.Feed(metric, val, time.Now())
		if i >= 10 { // After baseline establishment
			assert.False(t, score.IsAnomaly, "constant series should never trigger anomaly at iteration %d", i)
			assert.InDelta(t, val, score.Mean, 1e-6)
			assert.InDelta(t, 0.0, score.StdDev, 1e-6)
			assert.InDelta(t, 0.0, score.ZScore, 1e-6)
		}
	}
}

// Scenario 2: Linear ramp: gradual increase, verify threshold crossing
func TestAnomaly_Scenario2_LinearRamp(t *testing.T) {
	cfg := &config.AnomalyConfig{
		Enabled:         true,
		WindowSize:      30,
		ZScoreThreshold: 3.0,
		Alpha:           0.1,
	}
	detector := NewDetector(cfg)
	metric := "memory_ramp"

	// Establish baseline around 100.0 with minor variance
	for i := range 20 {
		detector.Feed(metric, 100.0+float64(i%3)*0.5, time.Now())
	}

	// Gradual increase ramp
	var anomalyDetected bool
	for val := 105.0; val <= 250.0; val += 5.0 {
		score := detector.Feed(metric, val, time.Now())
		if score.IsAnomaly {
			anomalyDetected = true
			assert.True(t, score.ZScore >= 3.0)
			break
		}
	}

	assert.True(t, anomalyDetected, "linear ramp should eventually trigger threshold crossing")
}

// Scenario 3: Step jump: sudden level shift, verify immediate detection
func TestAnomaly_Scenario3_StepJump(t *testing.T) {
	cfg := &config.AnomalyConfig{
		Enabled:         true,
		WindowSize:      50,
		ZScoreThreshold: 3.0,
		Alpha:           0.2,
	}
	detector := NewDetector(cfg)
	metric := "disk_jump"

	// Baseline at 50
	for i := range 30 {
		v := 50.0 + float64(i%2)*0.2
		detector.Feed(metric, v, time.Now())
	}

	// Sudden step jump to 500
	jumpScore := detector.Feed(metric, 500.0, time.Now())
	assert.True(t, jumpScore.IsAnomaly, "sudden jump should be detected as anomaly")
	assert.Equal(t, model.SeverityCritical, jumpScore.Severity)
	assert.Greater(t, jumpScore.ZScore, 3.0)
}

// Scenario 4: Single spike: one outlier, verify spike detected then recovery
func TestAnomaly_Scenario4_SingleSpikeAndRecovery(t *testing.T) {
	cfg := &config.AnomalyConfig{
		Enabled:         true,
		WindowSize:      40,
		ZScoreThreshold: 3.0,
		Alpha:           0.2,
	}
	detector := NewDetector(cfg)
	metric := "network_spike"

	// Baseline 10.0 ± 0.5
	for i := range 25 {
		detector.Feed(metric, 10.0+float64(i%3)*0.5, time.Now())
	}

	// Outlier spike
	spike := detector.Feed(metric, 100.0, time.Now())
	assert.True(t, spike.IsAnomaly, "spike must be detected as anomaly")

	// Return to baseline values
	for i := range 15 {
		detector.Feed(metric, 10.0+float64(i%3)*0.5, time.Now())
	}

	// Recovery check
	normalScore := detector.Feed(metric, 10.5, time.Now())
	assert.False(t, normalScore.IsAnomaly, "metric should recover to normal status")
}

// Scenario 5: Periodic wave: sinusoidal input, verify baseline tracking
func TestAnomaly_Scenario5_PeriodicWave(t *testing.T) {
	cfg := &config.AnomalyConfig{
		Enabled:         true,
		WindowSize:      60,
		ZScoreThreshold: 3.0,
		Alpha:           0.1,
	}
	detector := NewDetector(cfg)
	metric := "sine_wave"

	// Sinusoidal wave between 40 and 60 (mean = 50, amplitude = 10)
	for i := range 120 {
		val := 50.0 + 10.0*math.Sin(float64(i)*2.0*math.Pi/20.0)
		score := detector.Feed(metric, val, time.Now())
		if i >= 40 {
			// In a regular sine wave within established variance, no false alarms
			assert.False(t, score.IsAnomaly, "regular sinusoidal wave should not trigger false alarms (i=%d, val=%.2f, Z=%.2f)", i, val, score.ZScore)
		}
	}
}

// Scenario 6: Zero variance: identical inputs, verify no division by zero
func TestAnomaly_Scenario6_ZeroVariance(t *testing.T) {
	cfg := &config.AnomalyConfig{
		Enabled:         true,
		WindowSize:      30,
		ZScoreThreshold: 3.0,
		Alpha:           0.2,
	}
	detector := NewDetector(cfg)
	metric := "zero_var"

	// All zeros
	for range 20 {
		score := detector.Feed(metric, 0.0, time.Now())
		assert.False(t, math.IsNaN(score.Mean))
		assert.False(t, math.IsNaN(score.StdDev))
		assert.False(t, math.IsNaN(score.ZScore))
		assert.False(t, math.IsInf(score.ZScore, 0))
	}
}

// Scenario 7: Negative values: valid negative inputs handled correctly
func TestAnomaly_Scenario7_NegativeValues(t *testing.T) {
	cfg := &config.AnomalyConfig{
		Enabled:         true,
		WindowSize:      30,
		ZScoreThreshold: 3.0,
		Alpha:           0.2,
	}
	detector := NewDetector(cfg)
	metric := "temp_negative"

	// Baseline around -20.0
	for i := range 20 {
		v := -20.0 + float64(i%3)*0.5
		detector.Feed(metric, v, time.Now())
	}

	// Normal negative value
	normalScore := detector.Feed(metric, -19.5, time.Now())
	assert.False(t, normalScore.IsAnomaly)
	assert.True(t, normalScore.Mean < 0)

	// Anomaly jump into positive
	spikeScore := detector.Feed(metric, 30.0, time.Now())
	assert.True(t, spikeScore.IsAnomaly)
	assert.Greater(t, spikeScore.ZScore, 3.0)
}

// Scenario 8: High variance noise: random noise within bounds, no false alerts
func TestAnomaly_Scenario8_HighVarianceNoise(t *testing.T) {
	cfg := &config.AnomalyConfig{
		Enabled:         true,
		WindowSize:      60,
		ZScoreThreshold: 3.5, // 3.5 sigma
		Alpha:           0.1,
	}
	detector := NewDetector(cfg)
	metric := "noisy_metric"

	r := rand.New(rand.NewSource(42))
	var falsePositives int

	for i := range 150 {
		// Normal distribution N(100, 15^2)
		val := 100.0 + r.NormFloat64()*15.0
		score := detector.Feed(metric, val, time.Now())
		if i >= 30 && score.IsAnomaly {
			falsePositives++
		}
	}

	// For 120 samples from normal distribution, at 3.5 sigma false positive rate is < 1%
	assert.LessOrEqual(t, falsePositives, 2, "high variance Gaussian noise should have minimal false positives")
}

// Scenario 9: EWMA response: verify EWMA adjusts to new baseline at rate alpha
func TestAnomaly_Scenario9_EWMAResponse(t *testing.T) {
	alpha := 0.2
	ewma := NewEWMATracker(alpha)

	// Initialize at 0
	val1 := ewma.Update(100.0)
	assert.InDelta(t, 100.0, val1, 1e-6)

	// Step to 200: next step = 0.2*200 + 0.8*100 = 40 + 80 = 120
	val2 := ewma.Update(200.0)
	assert.InDelta(t, 120.0, val2, 1e-6)

	// Step to 200: next step = 0.2*200 + 0.8*120 = 40 + 96 = 136
	val3 := ewma.Update(200.0)
	assert.InDelta(t, 136.0, val3, 1e-6)

	// Continuous updates should asymptotically converge to 200
	for range 50 {
		ewma.Update(200.0)
	}
	assert.InDelta(t, 200.0, ewma.Value(), 0.01)
}

// Scenario 10: NaN/Inf handling: verify graceful rejection of non-finite floats
func TestAnomaly_Scenario10_NaNAndInfHandling(t *testing.T) {
	cfg := &config.AnomalyConfig{
		Enabled:         true,
		WindowSize:      30,
		ZScoreThreshold: 3.0,
		Alpha:           0.2,
	}
	detector := NewDetector(cfg)
	metric := "nan_test"

	// Feed valid points
	for i := range 15 {
		detector.Feed(metric, 50.0+float64(i), time.Now())
	}

	// Feed NaN
	nanScore := detector.Feed(metric, math.NaN(), time.Now())
	assert.False(t, nanScore.IsAnomaly)
	assert.Contains(t, nanScore.Explanation, "Ignored non-finite")

	// Feed +Inf
	infScore := detector.Feed(metric, math.Inf(1), time.Now())
	assert.False(t, infScore.IsAnomaly)
	assert.Contains(t, infScore.Explanation, "Ignored non-finite")

	// Feed -Inf
	negInfScore := detector.Feed(metric, math.Inf(-1), time.Now())
	assert.False(t, negInfScore.IsAnomaly)
	assert.Contains(t, negInfScore.Explanation, "Ignored non-finite")

	// Verify detector state was not corrupted by subsequent valid points
	validScore := detector.Feed(metric, 58.0, time.Now())
	assert.False(t, math.IsNaN(validScore.Mean))
	assert.False(t, math.IsNaN(validScore.StdDev))
	assert.False(t, math.IsNaN(validScore.EWMA))
	require.False(t, math.IsNaN(validScore.ZScore))
}
