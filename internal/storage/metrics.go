package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/watchdog-cli/watchdog/pkg/model"
)

// SaveSnapshot unpacks standard metrics from a system snapshot and commits them to the database.
func (s *SQLiteStorage) SaveSnapshot(ctx context.Context, snapshot *model.SystemSnapshot) error {
	if snapshot == nil {
		return nil
	}

	var pts []MetricPoint
	t := snapshot.Timestamp
	if t.IsZero() {
		t = time.Now()
	}

	host := ""
	if snapshot.System != nil {
		host = snapshot.System.Hostname
	}
	if host == "" {
		host = s.hostname
	}

	// CPU Metrics
	if snapshot.CPU != nil {
		pts = append(pts, MetricPoint{
			Timestamp: t,
			Metric:    "cpu_usage_pct",
			Value:     snapshot.CPU.OverallUsage,
			Hostname:  host,
		})
		pts = append(pts, MetricPoint{
			Timestamp: t,
			Metric:    "load1",
			Value:     snapshot.CPU.LoadAverage.Load1,
			Hostname:  host,
		})
		pts = append(pts, MetricPoint{
			Timestamp: t,
			Metric:    "load5",
			Value:     snapshot.CPU.LoadAverage.Load5,
			Hostname:  host,
		})
		pts = append(pts, MetricPoint{
			Timestamp: t,
			Metric:    "load15",
			Value:     snapshot.CPU.LoadAverage.Load15,
			Hostname:  host,
		})
	}

	// Memory Metrics
	if snapshot.Memory != nil {
		pts = append(pts, MetricPoint{
			Timestamp: t,
			Metric:    "memory_used_pct",
			Value:     snapshot.Memory.UsedPercent,
			Hostname:  host,
		})
		pts = append(pts, MetricPoint{
			Timestamp: t,
			Metric:    "memory_used_bytes",
			Value:     float64(snapshot.Memory.UsedBytes),
			Hostname:  host,
		})
		pts = append(pts, MetricPoint{
			Timestamp: t,
			Metric:    "swap_used_pct",
			Value:     snapshot.Memory.SwapUsedPercent,
			Hostname:  host,
		})
	}

	// Disk Metrics
	if snapshot.Disk != nil {
		pts = append(pts, MetricPoint{
			Timestamp: t,
			Metric:    "disk_used_pct",
			Value:     snapshot.Disk.UsedPercent,
			Hostname:  host,
		})

		var totalReadRate, totalWriteRate float64
		for _, io := range snapshot.Disk.IOCounters {
			totalReadRate += io.ReadRate
			totalWriteRate += io.WriteRate
		}
		pts = append(pts, MetricPoint{
			Timestamp: t,
			Metric:    "disk_read_bytes_sec",
			Value:     totalReadRate,
			Hostname:  host,
		})
		pts = append(pts, MetricPoint{
			Timestamp: t,
			Metric:    "disk_write_bytes_sec",
			Value:     totalWriteRate,
			Hostname:  host,
		})
	}

	// Network Metrics
	if snapshot.Network != nil {
		pts = append(pts, MetricPoint{
			Timestamp: t,
			Metric:    "net_rx_bytes_sec",
			Value:     snapshot.Network.TotalRxRate,
			Hostname:  host,
		})
		pts = append(pts, MetricPoint{
			Timestamp: t,
			Metric:    "net_tx_bytes_sec",
			Value:     snapshot.Network.TotalTxRate,
			Hostname:  host,
		})
	}

	// Process Count
	if snapshot.Processes != nil {
		pts = append(pts, MetricPoint{
			Timestamp: t,
			Metric:    "process_count",
			Value:     float64(snapshot.Processes.TotalCount),
			Hostname:  host,
		})
	}

	return s.SaveMetricPoints(ctx, pts)
}

// SaveMetricPoint inserts a single metric point.
func (s *SQLiteStorage) SaveMetricPoint(ctx context.Context, pt MetricPoint) error {
	return s.SaveMetricPoints(ctx, []MetricPoint{pt})
}

// SaveMetricPoints inserts multiple metric points within a single transaction.
func (s *SQLiteStorage) SaveMetricPoints(ctx context.Context, pts []MetricPoint) error {
	if len(pts) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO metrics_history (timestamp, metric, value, hostname, tags_json)
		VALUES (?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, pt := range pts {
		ts := pt.Timestamp.UnixMilli()
		var tagsJSON sql.NullString
		if len(pt.Tags) > 0 {
			if b, err := json.Marshal(pt.Tags); err == nil {
				tagsJSON.String = string(b)
				tagsJSON.Valid = true
			}
		}

		if _, err := stmt.ExecContext(ctx, ts, pt.Metric, pt.Value, pt.Hostname, tagsJSON); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// QueryMetrics queries metrics in a time range with optional step decimation.
func (s *SQLiteStorage) QueryMetrics(ctx context.Context, q TimeRangeQuery) ([]MetricPoint, error) {
	startMs := q.StartTime.UnixMilli()
	endMs := q.EndTime.UnixMilli()

	query := `
		SELECT timestamp, metric, value, hostname, tags_json
		FROM metrics_history
		WHERE metric = ? AND timestamp >= ? AND timestamp <= ?
		ORDER BY timestamp ASC
	`
	if q.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", q.Limit)
	}

	rows, err := s.db.QueryContext(ctx, query, q.Metric, startMs, endMs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var points []MetricPoint
	for rows.Next() {
		var ts int64
		var metric string
		var val float64
		var hostname sql.NullString
		var tagsJSON sql.NullString

		if err := rows.Scan(&ts, &metric, &val, &hostname, &tagsJSON); err != nil {
			return nil, err
		}

		var tags map[string]string
		if tagsJSON.Valid && tagsJSON.String != "" {
			_ = json.Unmarshal([]byte(tagsJSON.String), &tags)
		}

		points = append(points, MetricPoint{
			Timestamp: time.UnixMilli(ts),
			Metric:    metric,
			Value:     val,
			Hostname:  hostname.String,
			Tags:      tags,
		})
	}

	// Apply downsampling if step > 0
	if q.Step > 0 && len(points) > 0 {
		return downsamplePoints(points, q.Step), nil
	}

	return points, nil
}

func downsamplePoints(points []MetricPoint, step time.Duration) []MetricPoint {
	if len(points) <= 1 {
		return points
	}

	var downsampled []MetricPoint
	currentBucketStart := points[0].Timestamp.Truncate(step)
	var bucketVals []float64
	metricName := points[0].Metric
	hostname := points[0].Hostname

	for _, pt := range points {
		bucket := pt.Timestamp.Truncate(step)
		if bucket.Equal(currentBucketStart) {
			bucketVals = append(bucketVals, pt.Value)
		} else {
			if len(bucketVals) > 0 {
				var sum float64
				for _, v := range bucketVals {
					sum += v
				}
				avg := sum / float64(len(bucketVals))
				downsampled = append(downsampled, MetricPoint{
					Timestamp: currentBucketStart,
					Metric:    metricName,
					Value:     avg,
					Hostname:  hostname,
				})
			}
			currentBucketStart = bucket
			bucketVals = []float64{pt.Value}
		}
	}

	if len(bucketVals) > 0 {
		var sum float64
		for _, v := range bucketVals {
			sum += v
		}
		avg := sum / float64(len(bucketVals))
		downsampled = append(downsampled, MetricPoint{
			Timestamp: currentBucketStart,
			Metric:    metricName,
			Value:     avg,
			Hostname:  hostname,
		})
	}

	return downsampled
}

// GetMetricAggregate calculates summary statistics for a metric over a time window.
func (s *SQLiteStorage) GetMetricAggregate(ctx context.Context, metric string, start, end time.Time) (*MetricAggregate, error) {
	pts, err := s.QueryMetrics(ctx, TimeRangeQuery{
		Metric:    metric,
		StartTime: start,
		EndTime:   end,
	})
	if err != nil {
		return nil, err
	}

	if len(pts) == 0 {
		return &MetricAggregate{
			Metric:    metric,
			StartTime: start,
			EndTime:   end,
		}, nil
	}

	values := make([]float64, len(pts))
	var sum float64
	minVal := math.MaxFloat64
	maxVal := -math.MaxFloat64

	for i, pt := range pts {
		v := pt.Value
		values[i] = v
		sum += v
		if v < minVal {
			minVal = v
		}
		if v > maxVal {
			maxVal = v
		}
	}

	count := int64(len(values))
	avg := sum / float64(count)

	// StdDev
	var varianceSum float64
	for _, v := range values {
		varianceSum += (v - avg) * (v - avg)
	}
	stdDev := math.Sqrt(varianceSum / float64(count))

	// Percentiles
	sort.Float64s(values)
	p50 := percentile(values, 50)
	p90 := percentile(values, 90)
	p99 := percentile(values, 99)

	return &MetricAggregate{
		Metric:    metric,
		StartTime: start,
		EndTime:   end,
		Count:     count,
		Min:       minVal,
		Max:       maxVal,
		Avg:       avg,
		Sum:       sum,
		P50:       p50,
		P90:       p90,
		P99:       p99,
		StdDev:    stdDev,
	}, nil
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if len(sorted) == 1 {
		return sorted[0]
	}
	index := (p / 100.0) * float64(len(sorted)-1)
	lower := int(index)
	upper := lower + 1
	if upper >= len(sorted) {
		return sorted[len(sorted)-1]
	}
	weight := index - float64(lower)
	return sorted[lower]*(1-weight) + sorted[upper]*weight
}

// GetAvailableMetrics returns a list of all distinct metric names recorded in storage.
func (s *SQLiteStorage) GetAvailableMetrics(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT metric FROM metrics_history ORDER BY metric ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var metrics []string
	for rows.Next() {
		var m string
		if err := rows.Scan(&m); err == nil {
			metrics = append(metrics, m)
		}
	}
	return metrics, nil
}
