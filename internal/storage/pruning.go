package storage

import (
	"context"
	"os"
	"time"
)

// PruneOlderThan removes metrics, alerts, and diagnostics older than the specified retention duration.
func (s *SQLiteStorage) PruneOlderThan(ctx context.Context, retention time.Duration) (int64, error) {
	cutoff := time.Now().Add(-retention).UnixMilli()

	res, err := s.db.ExecContext(ctx, `DELETE FROM metrics_history WHERE timestamp < ?`, cutoff)
	if err != nil {
		return 0, err
	}

	rowsDeleted, _ := res.RowsAffected()

	// Also prune old diagnostics and resolved alerts older than 2x retention
	alertCutoff := time.Now().Add(-2 * retention).UnixMilli()
	_, _ = s.db.ExecContext(ctx, `DELETE FROM alerts_history WHERE triggered_at < ? AND status = 'RESOLVED'`, alertCutoff)
	_, _ = s.db.ExecContext(ctx, `DELETE FROM diagnostics_history WHERE timestamp < ?`, alertCutoff)

	return rowsDeleted, nil
}

// GetDatabaseSize returns the current database file size in bytes.
func (s *SQLiteStorage) GetDatabaseSize() (int64, error) {
	if s.dbPath == ":memory:" || s.dbPath == "" {
		return 0, nil
	}

	info, err := os.Stat(s.dbPath)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// Vacuum reorganizes and defragments the database storage file.
func (s *SQLiteStorage) Vacuum(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `VACUUM;`)
	return err
}
