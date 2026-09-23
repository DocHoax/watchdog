package anomaly

import (
	"math"
	"sync"
)

// RollingWindow maintains a fixed-size ring buffer for statistical calculations.
type RollingWindow struct {
	mu       sync.RWMutex
	capacity int
	values   []float64
	index    int
	count    int
	sum      float64
}

// NewRollingWindow creates a rolling window buffer with maximum capacity.
func NewRollingWindow(capacity int) *RollingWindow {
	if capacity <= 0 {
		capacity = 60
	}
	return &RollingWindow{
		capacity: capacity,
		values:   make([]float64, capacity),
	}
}

// Add appends a new data point into the rolling window.
func (rw *RollingWindow) Add(val float64) {
	rw.mu.Lock()
	defer rw.mu.Unlock()

	if rw.count < rw.capacity {
		rw.values[rw.count] = val
		rw.sum += val
		rw.count++
		rw.index = (rw.index + 1) % rw.capacity
	} else {
		oldVal := rw.values[rw.index]
		rw.sum = rw.sum - oldVal + val
		rw.values[rw.index] = val
		rw.index = (rw.index + 1) % rw.capacity
	}
}

// Count returns the current number of samples in the window.
func (rw *RollingWindow) Count() int {
	rw.mu.RLock()
	defer rw.mu.RUnlock()
	return rw.count
}

// Mean returns the arithmetic mean of values in the window.
func (rw *RollingWindow) Mean() float64 {
	rw.mu.RLock()
	defer rw.mu.RUnlock()
	if rw.count == 0 {
		return 0
	}
	return rw.sum / float64(rw.count)
}

// StdDev computes the sample standard deviation.
func (rw *RollingWindow) StdDev() float64 {
	rw.mu.RLock()
	defer rw.mu.RUnlock()

	if rw.count < 2 {
		return 0
	}

	mean := rw.sum / float64(rw.count)
	var varianceSum float64
	for i := 0; i < rw.count; i++ {
		diff := rw.values[i] - mean
		varianceSum += diff * diff
	}

	variance := varianceSum / float64(rw.count-1)
	if variance <= 0 {
		return 0
	}
	return math.Sqrt(variance)
}

// MinMax returns minimum and maximum values in the window.
func (rw *RollingWindow) MinMax() (float64, float64) {
	rw.mu.RLock()
	defer rw.mu.RUnlock()

	if rw.count == 0 {
		return 0, 0
	}

	minVal := rw.values[0]
	maxVal := rw.values[0]
	for i := 1; i < rw.count; i++ {
		if rw.values[i] < minVal {
			minVal = rw.values[i]
		}
		if rw.values[i] > maxVal {
			maxVal = rw.values[i]
		}
	}
	return minVal, maxVal
}

// Values returns a copy of current elements in the window.
func (rw *RollingWindow) Values() []float64 {
	rw.mu.RLock()
	defer rw.mu.RUnlock()

	result := make([]float64, rw.count)
	copy(result, rw.values[:rw.count])
	return result
}

// EWMATracker computes Exponentially Weighted Moving Average.
type EWMATracker struct {
	mu          sync.RWMutex
	alpha       float64
	currentEWMA float64
	initialized bool
}

// NewEWMATracker creates an EWMA calculator with smoothing factor alpha (0.0 < alpha <= 1.0).
func NewEWMATracker(alpha float64) *EWMATracker {
	if alpha <= 0 || alpha > 1 {
		alpha = 0.2 // default alpha
	}
	return &EWMATracker{
		alpha: alpha,
	}
}

// Update incorporates a new sample and returns the new EWMA.
func (e *EWMATracker) Update(val float64) float64 {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.initialized {
		e.currentEWMA = val
		e.initialized = true
		return val
	}

	e.currentEWMA = (e.alpha * val) + ((1.0 - e.alpha) * e.currentEWMA)
	return e.currentEWMA
}

// Value returns the current EWMA.
func (e *EWMATracker) Value() float64 {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.currentEWMA
}

// CalculateZScore computes Z-score: (x - mean) / stddev.
func CalculateZScore(val, mean, stddev float64) float64 {
	if math.IsNaN(val) || math.IsInf(val, 0) || math.IsNaN(mean) || math.IsInf(mean, 0) {
		return 0
	}
	diff := val - mean
	if math.Abs(diff) < 1e-6 {
		return 0
	}
	if stddev <= 1e-6 {
		effectiveStd := 0.05 * math.Abs(mean)
		if effectiveStd < 1e-3 {
			effectiveStd = 1.0
		}
		return diff / effectiveStd
	}
	return diff / stddev
}

// CalculateDeviationPct computes relative percentage change from baseline.
func CalculateDeviationPct(val, baseline float64) float64 {
	absBase := math.Abs(baseline)
	if absBase < 1e-6 {
		if val > 0 {
			return 100.0
		} else if val < 0 {
			return -100.0
		}
		return 0
	}
	return ((val - baseline) / absBase) * 100.0
}
