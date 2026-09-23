package storage

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// SQLiteStorage implements Storage interface using pure-Go modernc.org/sqlite.
type SQLiteStorage struct {
	db       *sql.DB
	dbPath   string
	mu       sync.RWMutex
	hostname string
}

// Config holds configuration for SQLite storage.
type Config struct {
	Path     string
	WALMode  bool
	MaxConns int
	Hostname string
}

// NewSQLiteStorage initializes and migrates the SQLite storage database.
func NewSQLiteStorage(cfg Config) (*SQLiteStorage, error) {
	if cfg.Path == "" {
		home, _ := os.UserHomeDir()
		cfg.Path = filepath.Join(home, ".watchdog", "metrics.db")
	}

	if cfg.Path != ":memory:" {
		dir := filepath.Dir(cfg.Path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create storage directory %s: %w", dir, err)
		}
	}

	dsn := cfg.Path
	if cfg.Path != ":memory:" {
		dsn = fmt.Sprintf("%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)", cfg.Path)
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database %s: %w", cfg.Path, err)
	}

	maxConns := cfg.MaxConns
	if maxConns <= 0 {
		maxConns = 10
	}
	db.SetMaxOpenConns(maxConns)
	db.SetMaxIdleConns(maxConns)
	db.SetConnMaxLifetime(time.Hour)

	store := &SQLiteStorage{
		db:       db,
		dbPath:   cfg.Path,
		hostname: cfg.Hostname,
	}

	if err := store.migrate(context.Background()); err != nil {
		db.Close()
		return nil, fmt.Errorf("database migration failed: %w", err)
	}

	return store, nil
}

func (s *SQLiteStorage) migrate(ctx context.Context) error {
	schema := `
	CREATE TABLE IF NOT EXISTS metrics_history (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp INTEGER NOT NULL,
		metric TEXT NOT NULL,
		value REAL NOT NULL,
		hostname TEXT,
		tags_json TEXT
	);

	CREATE INDEX IF NOT EXISTS idx_metrics_metric_time ON metrics_history(metric, timestamp);
	CREATE INDEX IF NOT EXISTS idx_metrics_time ON metrics_history(timestamp);

	CREATE TABLE IF NOT EXISTS alerts_history (
		id TEXT PRIMARY KEY,
		rule_name TEXT NOT NULL,
		category TEXT NOT NULL,
		severity TEXT NOT NULL,
		status TEXT NOT NULL,
		metric TEXT NOT NULL,
		metric_value REAL NOT NULL,
		threshold REAL NOT NULL,
		message TEXT NOT NULL,
		triggered_at INTEGER NOT NULL,
		resolved_at INTEGER
	);

	CREATE INDEX IF NOT EXISTS idx_alerts_triggered_at ON alerts_history(triggered_at);
	CREATE INDEX IF NOT EXISTS idx_alerts_status ON alerts_history(status);

	CREATE TABLE IF NOT EXISTS diagnostics_history (
		id TEXT PRIMARY KEY,
		overall_status TEXT NOT NULL,
		checks_json TEXT NOT NULL,
		summary_json TEXT NOT NULL,
		timestamp INTEGER NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_diagnostics_time ON diagnostics_history(timestamp);
	`

	_, err := s.db.ExecContext(ctx, schema)
	return err
}

// Close closes the underlying SQLite database.
func (s *SQLiteStorage) Close() error {
	return s.db.Close()
}

// Ping verifies database responsiveness.
func (s *SQLiteStorage) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}
