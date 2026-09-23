package storage

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/watchdog-cli/watchdog/pkg/model"
)

// Scenario 1: WAL mode verification: check PRAGMA journal_mode
func TestStorage_WALMode(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "wal_test.db")

	store, err := NewSQLiteStorage(Config{
		Path:     dbPath,
		Hostname: "wal-host",
	})
	require.NoError(t, err)
	defer store.Close()

	var journalMode string
	err = store.db.QueryRow("PRAGMA journal_mode;").Scan(&journalMode)
	require.NoError(t, err)
	assert.Equal(t, "wal", journalMode, "disk database must enable WAL journal mode")

	var synchronous int
	err = store.db.QueryRow("PRAGMA synchronous;").Scan(&synchronous)
	require.NoError(t, err)
	assert.Equal(t, 1, synchronous, "synchronous pragma should be NORMAL (1)")
}

// Scenario 2: Transaction integrity: commit/rollback under error
func TestStorage_TransactionIntegrity(t *testing.T) {
	ctx := context.Background()
	store, err := NewSQLiteStorage(Config{
		Path:     ":memory:",
		Hostname: "tx-host",
	})
	require.NoError(t, err)
	defer store.Close()

	now := time.Now()

	// 1. Rollback test: insert and rollback -> verify no rows persist
	tx, err := store.db.BeginTx(ctx, nil)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `INSERT INTO metrics_history (timestamp, metric, value, hostname) VALUES (?, ?, ?, ?)`,
		now.UnixMilli(), "test_metric", 123.45, "tx-host")
	require.NoError(t, err)
	err = tx.Rollback()
	require.NoError(t, err)

	pts, err := store.QueryMetrics(ctx, TimeRangeQuery{
		Metric:    "test_metric",
		StartTime: now.Add(-time.Hour),
		EndTime:   now.Add(time.Hour),
	})
	require.NoError(t, err)
	assert.Empty(t, pts, "rolled back transaction must not persist rows")

	// 2. Commit test: SaveMetricPoints commits atomic batch
	batch := []MetricPoint{
		{Timestamp: now, Metric: "batch_m1", Value: 10.0, Hostname: "tx-host"},
		{Timestamp: now.Add(time.Second), Metric: "batch_m1", Value: 20.0, Hostname: "tx-host"},
		{Timestamp: now.Add(2 * time.Second), Metric: "batch_m1", Value: 30.0, Hostname: "tx-host"},
	}
	err = store.SaveMetricPoints(ctx, batch)
	require.NoError(t, err)

	pts, err = store.QueryMetrics(ctx, TimeRangeQuery{
		Metric:    "batch_m1",
		StartTime: now.Add(-time.Second),
		EndTime:   now.Add(5 * time.Second),
	})
	require.NoError(t, err)
	assert.Len(t, pts, 3, "committed batch should persist exactly 3 points")
}

// Scenario 3: Schema migration: verify all tables created, indexes present
func TestStorage_SchemaMigration(t *testing.T) {
	store, err := NewSQLiteStorage(Config{
		Path:     ":memory:",
		Hostname: "schema-host",
	})
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()

	// Verify tables
	expectedTables := []string{"metrics_history", "alerts_history", "diagnostics_history"}
	for _, tbl := range expectedTables {
		var count int
		err := store.db.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?`, tbl).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count, "table '%s' must exist", tbl)
	}

	// Verify indexes
	expectedIndexes := []string{
		"idx_metrics_metric_time",
		"idx_metrics_time",
		"idx_alerts_triggered_at",
		"idx_alerts_status",
		"idx_diagnostics_time",
	}
	for _, idx := range expectedIndexes {
		var count int
		err := store.db.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_master WHERE type='index' AND name=?`, idx).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count, "index '%s' must exist", idx)
	}
}

// Scenario 4: Retention policy enforcement: prune metrics older than retention window, verify space reclaimed
func TestStorage_RetentionPolicyEnforcement(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "retention_test.db")

	store, err := NewSQLiteStorage(Config{
		Path:     dbPath,
		Hostname: "retention-host",
	})
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()
	now := time.Now()

	var points []MetricPoint
	// 500 old points (48 hours ago)
	for i := range 500 {
		points = append(points, MetricPoint{
			Timestamp: now.Add(-48 * time.Hour).Add(time.Duration(i) * time.Minute),
			Metric:    "cpu_usage_pct",
			Value:     float64(i % 100),
			Hostname:  "retention-host",
		})
	}
	// 500 recent points (within last 6 hours)
	for i := range 500 {
		points = append(points, MetricPoint{
			Timestamp: now.Add(-6 * time.Hour).Add(time.Duration(i) * time.Minute),
			Metric:    "cpu_usage_pct",
			Value:     float64(i % 100),
			Hostname:  "retention-host",
		})
	}

	err = store.SaveMetricPoints(ctx, points)
	require.NoError(t, err)

	// Prune metrics older than 24h
	pruned, err := store.PruneOlderThan(ctx, 24*time.Hour)
	require.NoError(t, err)
	assert.Equal(t, int64(500), pruned, "should prune exactly 500 records older than 24 hours")

	// Verify remaining points count
	var remainingCount int
	err = store.db.QueryRowContext(ctx, `SELECT count(*) FROM metrics_history`).Scan(&remainingCount)
	require.NoError(t, err)
	assert.Equal(t, 500, remainingCount, "remaining count should be exactly 500")

	// Vacuum to defragment space
	err = store.Vacuum(ctx)
	require.NoError(t, err)

	size, err := store.GetDatabaseSize()
	require.NoError(t, err)
	assert.Greater(t, size, int64(0), "database size must be valid positive byte count")
}

// Scenario 5: Concurrent read/write stress: 10 concurrent goroutines writing metrics while 5 read
func TestStorage_ConcurrentReadWriteStress(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "stress_test.db")

	store, err := NewSQLiteStorage(Config{
		Path:     dbPath,
		Hostname: "stress-host",
		MaxConns: 20,
	})
	require.NoError(t, err)
	defer store.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	errCh := make(chan error, 50)

	numWriters := 10
	numReaders := 5
	writesPerWorker := 30

	// Launch Writers
	for w := range numWriters {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			r := rand.New(rand.NewSource(int64(workerID * 1000)))
			for i := 0; i < writesPerWorker; i++ {
				select {
				case <-ctx.Done():
					return
				default:
				}

				now := time.Now()
				snap := &model.SystemSnapshot{
					Timestamp: now,
					System:    &model.SystemInfo{Hostname: fmt.Sprintf("host-%d", workerID)},
					CPU:       &model.CPUInfo{OverallUsage: 10.0 + r.Float64()*80.0},
					Memory:    &model.MemoryInfo{TotalBytes: 16000000, UsedPercent: 20.0 + r.Float64()*70.0},
					Disk: &model.DiskInfo{
						Partitions: []model.PartitionInfo{
							{Mountpoint: "/", TotalBytes: 1000000, UsedPercent: 50.0},
						},
					},
				}

				if err := store.SaveSnapshot(ctx, snap); err != nil {
					errCh <- fmt.Errorf("writer %d iter %d error: %w", workerID, i, err)
					return
				}
				time.Sleep(2 * time.Millisecond)
			}
		}(w)
	}

	// Launch Readers
	for rID := range numReaders {
		wg.Add(1)
		go func(readerID int) {
			defer wg.Done()
			for i := 0; i < writesPerWorker; i++ {
				select {
				case <-ctx.Done():
					return
				default:
				}

				now := time.Now()
				_, err := store.QueryMetrics(ctx, TimeRangeQuery{
					Metric:    "cpu_usage_pct",
					StartTime: now.Add(-5 * time.Minute),
					EndTime:   now.Add(5 * time.Minute),
				})
				if err != nil && err != sql.ErrNoRows {
					errCh <- fmt.Errorf("reader %d iter %d error: %w", readerID, i, err)
					return
				}

				_, _ = store.GetMetricAggregate(ctx, "cpu_usage_pct", now.Add(-5*time.Minute), now.Add(5*time.Minute))
				time.Sleep(3 * time.Millisecond)
			}
		}(rID)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("concurrent stress error: %v", err)
	}

	// Verify database is intact and queryable
	metrics, err := store.GetAvailableMetrics(ctx)
	require.NoError(t, err)
	assert.Contains(t, metrics, "cpu_usage_pct")
}
