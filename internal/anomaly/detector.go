package anomaly

import (
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/watchdog-cli/watchdog/internal/config"
	"github.com/watchdog-cli/watchdog/pkg/model"
)

// MetricStream holds statistical estimators for a single time series metric.
type MetricStream struct {
	window *RollingWindow
	ewma   *EWMATracker
}

// Detector tracks multiple metric streams and identifies statistical anomalies using Z-score and EWMA.
type Detector struct {
	mu              sync.RWMutex
	cfg             *config.AnomalyConfig
	streams         map[string]*MetricStream
	windowSize      int
	minSampleCount  int
	zScoreThreshold float64
	alpha           float64
}

// NewDetector creates a statistical anomaly detector.
func NewDetector(cfg *config.AnomalyConfig) *Detector {
	windowSize := 60
	zScoreThresh := 3.0
	alpha := 0.2

	if cfg != nil {
		if cfg.WindowSize > 0 {
			windowSize = cfg.WindowSize
		}
		if cfg.ZScoreThreshold > 0 {
			zScoreThresh = cfg.ZScoreThreshold
		}
		if cfg.Alpha > 0 {
			alpha = cfg.Alpha
		}
	}

	return &Detector{
		cfg:             cfg,
		streams:         make(map[string]*MetricStream),
		windowSize:      windowSize,
		minSampleCount:  10, // Require minimum 10 samples before flagging anomalies
		zScoreThreshold: zScoreThresh,
		alpha:           alpha,
	}
}

// getOrCreateStream returns or initializes the stream for a metric.
func (d *Detector) getOrCreateStream(metric string) *MetricStream {
	d.mu.Lock()
	defer d.mu.Unlock()

	stream, exists := d.streams[metric]
	if !exists {
		stream = &MetricStream{
			window: NewRollingWindow(d.windowSize),
			ewma:   NewEWMATracker(d.alpha),
		}
		d.streams[metric] = stream
	}
	return stream
}

// Feed ingests a single metric point, updates baseline stats, and evaluates anomaly score.
func (d *Detector) Feed(metric string, value float64, ts time.Time) model.AnomalyScore {
	stream := d.getOrCreateStream(metric)

	// Calculate prior statistics before adding new point (to prevent self-skewing baseline)
	mean := stream.window.Mean()
	stdDev := stream.window.StdDev()
	count := stream.window.Count()
	ewmaVal := stream.ewma.Update(value)
	stream.window.Add(value)

	if ts.IsZero() {
		ts = time.Now()
	}

	score := model.AnomalyScore{
		MetricName:   metric,
		CurrentValue: value,
		Mean:         mean,
		StdDev:       stdDev,
		EWMA:         ewmaVal,
		DetectedAt:   ts,
	}

	// If insufficient samples, consider baseline forming (not an anomaly)
	if count < d.minSampleCount {
		score.Explanation = fmt.Sprintf("Baseline establishing (%d/%d samples)", count+1, d.minSampleCount)
		return score
	}

	zScore := CalculateZScore(value, mean, stdDev)
	devPct := CalculateDeviationPct(value, mean)

	score.ZScore = zScore
	score.DeviationPct = devPct

	absZ := math.Abs(zScore)
	if absZ >= d.zScoreThreshold && !math.IsInf(absZ, 1) {
		score.IsAnomaly = true
		if absZ >= d.zScoreThreshold+2.0 {
			score.Severity = model.SeverityCritical
		} else {
			score.Severity = model.SeverityWarning
		}

		direction := "Spike"
		if value < mean {
			direction = "Drop"
		}

		score.Explanation = fmt.Sprintf("%s detected: %.2f (baseline mean: %.2f ± %.2f, Z: %+.2f, dev: %+.1f%%)",
			direction, value, mean, stdDev, zScore, devPct)
	} else {
		score.Severity = model.SeverityInfo
		score.Explanation = fmt.Sprintf("Normal variation (mean: %.2f, Z: %+.2f)", mean, zScore)
	}

	return score
}

// FeedSnapshot extracts all standard metrics from a snapshot and evaluates anomalies.
func (d *Detector) FeedSnapshot(snapshot *model.SystemSnapshot) *model.AnomalyReport {
	if snapshot == nil {
		return &model.AnomalyReport{GeneratedAt: time.Now()}
	}

	ts := snapshot.Timestamp
	if ts.IsZero() {
		ts = time.Now()
	}

	var scores []model.AnomalyScore

	// CPU
	if snapshot.CPU != nil {
		scores = append(scores, d.Feed("cpu_usage_pct", snapshot.CPU.OverallUsage, ts))
		if snapshot.CPU.LogicalCores > 0 {
			loadRatio := snapshot.CPU.LoadAverage.Load1 / float64(snapshot.CPU.LogicalCores)
			scores = append(scores, d.Feed("cpu_load_ratio", loadRatio, ts))
		}
	}

	// Memory
	if snapshot.Memory != nil && snapshot.Memory.TotalBytes > 0 {
		scores = append(scores, d.Feed("memory_used_pct", snapshot.Memory.UsedPercent, ts))
		if snapshot.Memory.SwapTotalBytes > 0 {
			scores = append(scores, d.Feed("swap_used_pct", snapshot.Memory.SwapUsedPercent, ts))
		}
	}

	// Disk
	if snapshot.Disk != nil {
		scores = append(scores, d.Feed("disk_used_pct", snapshot.Disk.UsedPercent, ts))
		var totalReadRate, totalWriteRate float64
		for _, io := range snapshot.Disk.IOCounters {
			totalReadRate += io.ReadRate
			totalWriteRate += io.WriteRate
		}
		scores = append(scores, d.Feed("disk_read_rate", totalReadRate, ts))
		scores = append(scores, d.Feed("disk_write_rate", totalWriteRate, ts))
	}

	// Network
	if snapshot.Network != nil {
		scores = append(scores, d.Feed("net_rx_rate", snapshot.Network.TotalRxRate, ts))
		scores = append(scores, d.Feed("net_tx_rate", snapshot.Network.TotalTxRate, ts))
	}

	// Processes
	if snapshot.Processes != nil {
		scores = append(scores, d.Feed("process_count", float64(snapshot.Processes.TotalCount), ts))
		scores = append(scores, d.Feed("zombie_count", float64(snapshot.Processes.ZombieCount), ts))
	}

	anomaliesCount := 0
	for _, s := range scores {
		if s.IsAnomaly {
			anomaliesCount++
		}
	}

	return &model.AnomalyReport{
		TotalChecked:   len(scores),
		AnomaliesCount: anomaliesCount,
		Scores:         scores,
		GeneratedAt:    ts,
	}
}

// Reset clears all baseline metrics.
func (d *Detector) Reset() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.streams = make(map[string]*MetricStream)
}
