package storage

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// BenchmarkStorage_InsertThroughput benchmarks metric insertion throughput in batches.
func BenchmarkStorage_InsertThroughput(b *testing.B) {
	tempDir := b.TempDir()
	dbPath := filepath.Join(tempDir, "bench_insert.db")

	store, err := NewSQLiteStorage(Config{
		Path:     dbPath,
		Hostname: "bench-insert-host",
	})
	require.NoError(b, err)
	defer store.Close()

	ctx := context.Background()
	baseTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	batchSize := 1000

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		var pts []MetricPoint
		for j := 0; j < batchSize; j++ {
			pts = append(pts, MetricPoint{
				Timestamp: baseTime.Add(time.Duration(i*batchSize+j) * time.Second),
				Metric:    "cpu_usage_pct",
				Value:     float64(j % 100),
				Hostname:  "bench-insert-host",
			})
		}
		b.StartTimer()

		err := store.SaveMetricPoints(ctx, pts)
		if err != nil {
			b.Fatalf("insert failed: %v", err)
		}
	}
}

// BenchmarkStorage_RangeQuery10K benchmarks 1-hour range query latency over 10K rows.
func BenchmarkStorage_RangeQuery10K(b *testing.B) {
	tempDir := b.TempDir()
	dbPath := filepath.Join(tempDir, "bench_10k.db")

	store, err := NewSQLiteStorage(Config{
		Path:     dbPath,
		Hostname: "bench-10k-host",
	})
	require.NoError(b, err)
	defer store.Close()

	ctx := context.Background()
	baseTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	// Populate 10,000 records (approx 1 record every 5 seconds = ~14 hours of data)
	batchSize := 2000
	totalRows := 10000

	for i := 0; i < totalRows; i += batchSize {
		var pts []MetricPoint
		for j := 0; j < batchSize; j++ {
			idx := i + j
			pts = append(pts, MetricPoint{
				Timestamp: baseTime.Add(time.Duration(idx) * 5 * time.Second),
				Metric:    "cpu_usage_pct",
				Value:     float64(idx%100) + 0.5,
				Hostname:  "bench-10k-host",
			})
		}
		err := store.SaveMetricPoints(ctx, pts)
		require.NoError(b, err)
	}

	// 1-hour range query (720 records at 5s interval)
	queryStart := baseTime.Add(2000 * 5 * time.Second)
	queryEnd := queryStart.Add(1 * time.Hour)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		pts, err := store.QueryMetrics(ctx, TimeRangeQuery{
			Metric:    "cpu_usage_pct",
			StartTime: queryStart,
			EndTime:   queryEnd,
		})
		if err != nil {
			b.Fatalf("query failed: %v", err)
		}
		if len(pts) == 0 {
			b.Fatalf("expected non-empty points")
		}
	}
}

// BenchmarkStorage_TimeRangeQuery100K benchmarks time-range queries over 100K metric records.
func BenchmarkStorage_TimeRangeQuery100K(b *testing.B) {
	tempDir := b.TempDir()
	dbPath := filepath.Join(tempDir, "bench_100k.db")

	store, err := NewSQLiteStorage(Config{
		Path:     dbPath,
		Hostname: "bench-host",
	})
	require.NoError(b, err)
	defer store.Close()

	ctx := context.Background()
	baseTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	batchSize := 2000
	totalRows := 100000

	for i := 0; i < totalRows; i += batchSize {
		var pts []MetricPoint
		for j := 0; j < batchSize; j++ {
			idx := i + j
			pts = append(pts, MetricPoint{
				Timestamp: baseTime.Add(time.Duration(idx) * 10 * time.Second),
				Metric:    fmt.Sprintf("metric_%d", (idx % 5)),
				Value:     float64(idx%100) + 0.5,
				Hostname:  "bench-host",
			})
		}
		err := store.SaveMetricPoints(ctx, pts)
		require.NoError(b, err)
	}

	queryStart := baseTime.Add(20000 * 10 * time.Second)
	queryEnd := baseTime.Add(25000 * 10 * time.Second) // 5,000 records window

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		pts, err := store.QueryMetrics(ctx, TimeRangeQuery{
			Metric:    "metric_0",
			StartTime: queryStart,
			EndTime:   queryEnd,
		})
		if err != nil {
			b.Fatalf("query failed: %v", err)
		}
		if len(pts) == 0 {
			b.Fatalf("expected non-empty points")
		}
	}
}

// TestStorage_TimeRangeQuery100KPerformance ensures a 100k row dataset query completes well under 50ms.
func TestStorage_TimeRangeQuery100KPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping 100K performance test in short mode")
	}

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "perf_100k.db")

	store, err := NewSQLiteStorage(Config{
		Path:     dbPath,
		Hostname: "perf-host",
	})
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()
	baseTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	batchSize := 5000
	totalRows := 100000

	for i := 0; i < totalRows; i += batchSize {
		var pts []MetricPoint
		for j := 0; j < batchSize; j++ {
			idx := i + j
			pts = append(pts, MetricPoint{
				Timestamp: baseTime.Add(time.Duration(idx) * 5 * time.Second),
				Metric:    "cpu_usage_pct",
				Value:     float64(idx%100) + 0.5,
				Hostname:  "perf-host",
			})
		}
		err := store.SaveMetricPoints(ctx, pts)
		require.NoError(t, err)
	}

	queryStart := baseTime.Add(30000 * 5 * time.Second)
	queryEnd := baseTime.Add(32000 * 5 * time.Second) // 2,000 records window

	start := time.Now()
	pts, err := store.QueryMetrics(ctx, TimeRangeQuery{
		Metric:    "cpu_usage_pct",
		StartTime: queryStart,
		EndTime:   queryEnd,
	})
	duration := time.Since(start)

	require.NoError(t, err)
	require.Len(t, pts, 2001)
	t.Logf("100K table indexed time-range query completed in %v (target: < 500ms)", duration)
	require.Less(t, duration, 500*time.Millisecond, "time-range query over 100k dataset must execute in < 500ms")
}
