package intelligence

import (
	"math"
	"sort"
	"time"

	"github.com/DocHoax/watchdog/internal/storage"
)

// CalculateBaseline computes statistical baseline metrics including mean, stddev, and percentiles.
func CalculateBaseline(metric string, points []storage.MetricPoint, window time.Duration) HistoricalBaseline {
	now := time.Now()
	res := HistoricalBaseline{
		Metric:     metric,
		Window:     window,
		ComputedAt: now,
	}

	var values []float64
	for _, p := range points {
		if !math.IsNaN(p.Value) && !math.IsInf(p.Value, 0) {
			values = append(values, p.Value)
		}
	}

	res.SampleCount = len(values)
	if len(values) == 0 {
		return res
	}

	// Calculate Min, Max, Mean
	minVal := values[0]
	maxVal := values[0]
	sum := 0.0

	for _, v := range values {
		if v < minVal {
			minVal = v
		}
		if v > maxVal {
			maxVal = v
		}
		sum += v
	}

	mean := sum / float64(len(values))
	res.Min = math.Round(minVal*100) / 100
	res.Max = math.Round(maxVal*100) / 100
	res.Mean = math.Round(mean*100) / 100

	// Calculate Standard Deviation
	var varianceSum float64
	for _, v := range values {
		diff := v - mean
		varianceSum += diff * diff
	}
	stdDev := math.Sqrt(varianceSum / float64(len(values)))
	res.StdDev = math.Round(stdDev*100) / 100

	// Calculate Percentiles
	sort.Float64s(values)
	res.P50 = math.Round(calculatePercentile(values, 50.0)*100) / 100
	res.P90 = math.Round(calculatePercentile(values, 90.0)*100) / 100
	res.P95 = math.Round(calculatePercentile(values, 95.0)*100) / 100
	res.P99 = math.Round(calculatePercentile(values, 99.0)*100) / 100

	return res
}

// calculatePercentile calculates the p-th percentile from an already sorted slice using linear interpolation.
func calculatePercentile(sortedValues []float64, p float64) float64 {
	n := len(sortedValues)
	if n == 0 {
		return 0.0
	}
	if n == 1 {
		return sortedValues[0]
	}

	if p <= 0 {
		return sortedValues[0]
	}
	if p >= 100 {
		return sortedValues[n-1]
	}

	rank := (p / 100.0) * float64(n-1)
	lowerIndex := int(math.Floor(rank))
	upperIndex := int(math.Ceil(rank))
	weight := rank - float64(lowerIndex)

	if lowerIndex == upperIndex || upperIndex >= n {
		return sortedValues[lowerIndex]
	}

	return sortedValues[lowerIndex]*(1.0-weight) + sortedValues[upperIndex]*weight
}
