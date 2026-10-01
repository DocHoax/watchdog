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

	CREATE TABLE IF NOT EXISTS audit_events (
		id TEXT PRIMARY KEY,
		timestamp INTEGER NOT NULL,
		event_type TEXT NOT NULL,
		severity TEXT NOT NULL,
		outcome TEXT NOT NULL,
		actor_type TEXT,
		actor_identity TEXT,
		source_address TEXT,
		transport TEXT,
		protocol TEXT,
		user_agent TEXT,
		endpoint TEXT,
		method TEXT,
		request_id TEXT,
		resource TEXT,
		action TEXT,
		message TEXT NOT NULL,
		metadata_json TEXT
	);

	CREATE INDEX IF NOT EXISTS idx_audit_timestamp ON audit_events(timestamp DESC);
	CREATE INDEX IF NOT EXISTS idx_audit_event_type ON audit_events(event_type);
	CREATE INDEX IF NOT EXISTS idx_audit_severity ON audit_events(severity);
	CREATE INDEX IF NOT EXISTS idx_audit_outcome ON audit_events(outcome);
	CREATE INDEX IF NOT EXISTS idx_audit_request_id ON audit_events(request_id);

	CREATE TABLE IF NOT EXISTS fleet_nodes (
		node_id TEXT PRIMARY KEY,
		hostname TEXT NOT NULL,
		os TEXT NOT NULL,
		platform TEXT NOT NULL,
		platform_version TEXT,
		arch TEXT NOT NULL,
		kernel_version TEXT,
		version TEXT NOT NULL,
		cpu_cores INTEGER NOT NULL,
		total_memory INTEGER NOT NULL,
		ip_addresses_json TEXT,
		mac_addresses_json TEXT,
		tags_json TEXT,
		status TEXT NOT NULL,
		status_message TEXT,
		registered_at INTEGER NOT NULL,
		last_heartbeat INTEGER NOT NULL,
		last_telemetry INTEGER,
		summary_json TEXT,
		metadata_json TEXT
	);

	CREATE INDEX IF NOT EXISTS idx_fleet_nodes_status ON fleet_nodes(status);
	CREATE INDEX IF NOT EXISTS idx_fleet_nodes_heartbeat ON fleet_nodes(last_heartbeat DESC);
	CREATE INDEX IF NOT EXISTS idx_fleet_nodes_hostname ON fleet_nodes(hostname);

	CREATE TABLE IF NOT EXISTS fleet_telemetry (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		node_id TEXT NOT NULL,
		timestamp INTEGER NOT NULL,
		sequence INTEGER NOT NULL,
		snapshot_json TEXT,
		diagnostics_json TEXT,
		alerts_json TEXT,
		metrics_json TEXT,
		tags_json TEXT
	);

	CREATE INDEX IF NOT EXISTS idx_fleet_telemetry_node_time ON fleet_telemetry(node_id, timestamp DESC);
	CREATE INDEX IF NOT EXISTS idx_fleet_telemetry_time ON fleet_telemetry(timestamp DESC);

	CREATE TABLE IF NOT EXISTS incidents (
		id TEXT PRIMARY KEY,
		title TEXT NOT NULL,
		status TEXT NOT NULL,
		severity TEXT NOT NULL,
		scope TEXT NOT NULL,
		confidence TEXT NOT NULL,
		start_time INTEGER NOT NULL,
		updated_at INTEGER NOT NULL,
		resolved_at INTEGER,
		primary_symptoms_json TEXT,
		affected_nodes_json TEXT,
		root_signals_json TEXT,
		impact_json TEXT,
		explanation_json TEXT,
		findings_json TEXT,
		metadata_json TEXT
	);

	CREATE INDEX IF NOT EXISTS idx_incidents_status ON incidents(status);
	CREATE INDEX IF NOT EXISTS idx_incidents_severity ON incidents(severity);
	CREATE INDEX IF NOT EXISTS idx_incidents_scope ON incidents(scope);
	CREATE INDEX IF NOT EXISTS idx_incidents_start_time ON incidents(start_time DESC);
	CREATE INDEX IF NOT EXISTS idx_incidents_updated_at ON incidents(updated_at DESC);

	CREATE TABLE IF NOT EXISTS incident_timeline (
		id TEXT PRIMARY KEY,
		incident_id TEXT NOT NULL,
		timestamp INTEGER NOT NULL,
		event_type TEXT NOT NULL,
		source TEXT NOT NULL,
		node_id TEXT,
		severity TEXT,
		title TEXT NOT NULL,
		description TEXT,
		payload_json TEXT
	);

	CREATE INDEX IF NOT EXISTS idx_incident_timeline_inc_time ON incident_timeline(incident_id, timestamp ASC);
	CREATE INDEX IF NOT EXISTS idx_incident_timeline_node ON incident_timeline(node_id);
	CREATE INDEX IF NOT EXISTS idx_incident_timeline_event_type ON incident_timeline(event_type);

	CREATE TABLE IF NOT EXISTS incident_nodes (
		incident_id TEXT NOT NULL,
		node_id TEXT NOT NULL,
		PRIMARY KEY (incident_id, node_id)
	);

	CREATE INDEX IF NOT EXISTS idx_incident_nodes_node ON incident_nodes(node_id);
	CREATE INDEX IF NOT EXISTS idx_incident_nodes_inc ON incident_nodes(incident_id);

	CREATE TABLE IF NOT EXISTS organizations (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		display_name TEXT,
		description TEXT,
		status TEXT NOT NULL,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL,
		metadata_json TEXT
	);

	CREATE INDEX IF NOT EXISTS idx_organizations_status ON organizations(status);

	CREATE TABLE IF NOT EXISTS fleet_groups (
		id TEXT PRIMARY KEY,
		org_id TEXT NOT NULL,
		parent_group_id TEXT,
		name TEXT NOT NULL,
		display_name TEXT,
		description TEXT,
		group_type TEXT NOT NULL,
		path TEXT,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL,
		metadata_json TEXT
	);

	CREATE INDEX IF NOT EXISTS idx_fleet_groups_org ON fleet_groups(org_id);
	CREATE INDEX IF NOT EXISTS idx_fleet_groups_parent ON fleet_groups(parent_group_id);

	CREATE TABLE IF NOT EXISTS fleet_group_members (
		group_id TEXT NOT NULL,
		node_id TEXT NOT NULL,
		added_at INTEGER NOT NULL,
		added_by TEXT,
		role TEXT NOT NULL,
		metadata_json TEXT,
		PRIMARY KEY (group_id, node_id)
	);

	CREATE INDEX IF NOT EXISTS idx_fleet_group_members_node ON fleet_group_members(node_id);
	CREATE INDEX IF NOT EXISTS idx_fleet_group_members_group ON fleet_group_members(group_id);

	CREATE TABLE IF NOT EXISTS policies (
		id TEXT PRIMARY KEY,
		org_id TEXT NOT NULL,
		name TEXT NOT NULL,
		display_name TEXT,
		description TEXT,
		category TEXT NOT NULL,
		status TEXT NOT NULL,
		active_revision INTEGER NOT NULL DEFAULT 0,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL,
		metadata_json TEXT
	);

	CREATE INDEX IF NOT EXISTS idx_policies_org ON policies(org_id);
	CREATE INDEX IF NOT EXISTS idx_policies_category ON policies(category);
	CREATE INDEX IF NOT EXISTS idx_policies_status ON policies(status);

	CREATE TABLE IF NOT EXISTS policy_revisions (
		policy_id TEXT NOT NULL,
		revision INTEGER NOT NULL,
		created_at INTEGER NOT NULL,
		created_by TEXT NOT NULL,
		change_summary TEXT,
		target_selector TEXT,
		priority INTEGER NOT NULL,
		inheritance_mode TEXT NOT NULL,
		enforcement_mode TEXT NOT NULL,
		rules_json TEXT NOT NULL,
		metadata_json TEXT,
		content_digest TEXT NOT NULL,
		PRIMARY KEY (policy_id, revision)
	);

	CREATE INDEX IF NOT EXISTS idx_policy_revisions_created_at ON policy_revisions(created_at DESC);
	CREATE INDEX IF NOT EXISTS idx_policy_revisions_digest ON policy_revisions(content_digest);

	CREATE TABLE IF NOT EXISTS policy_assignments (
		id TEXT PRIMARY KEY,
		policy_id TEXT NOT NULL,
		org_id TEXT NOT NULL,
		target_type TEXT NOT NULL,
		target_id TEXT NOT NULL,
		assigned_at INTEGER NOT NULL,
		assigned_by TEXT,
		enabled INTEGER NOT NULL,
		metadata_json TEXT
	);

	CREATE INDEX IF NOT EXISTS idx_policy_assignments_policy ON policy_assignments(policy_id);
	CREATE INDEX IF NOT EXISTS idx_policy_assignments_org ON policy_assignments(org_id);
	CREATE INDEX IF NOT EXISTS idx_policy_assignments_target ON policy_assignments(target_type, target_id);

	CREATE TABLE IF NOT EXISTS policy_evaluations (
		id TEXT PRIMARY KEY,
		org_id TEXT NOT NULL,
		target_node_id TEXT NOT NULL,
		trigger_type TEXT NOT NULL,
		evaluated_at INTEGER NOT NULL,
		duration_ns INTEGER NOT NULL,
		status TEXT NOT NULL,
		results_json TEXT NOT NULL,
		summary_json TEXT NOT NULL,
		metadata_json TEXT
	);

	CREATE INDEX IF NOT EXISTS idx_policy_evaluations_org_time ON policy_evaluations(org_id, evaluated_at DESC);
	CREATE INDEX IF NOT EXISTS idx_policy_evaluations_node_time ON policy_evaluations(target_node_id, evaluated_at DESC);
	CREATE INDEX IF NOT EXISTS idx_policy_evaluations_status ON policy_evaluations(status);

	CREATE TABLE IF NOT EXISTS compliance_findings (
		id TEXT PRIMARY KEY,
		org_id TEXT NOT NULL,
		target_node_id TEXT NOT NULL,
		policy_id TEXT NOT NULL,
		policy_revision INTEGER NOT NULL,
		rule_id TEXT NOT NULL,
		rule_name TEXT,
		category TEXT NOT NULL,
		severity TEXT NOT NULL,
		enforcement_mode TEXT NOT NULL,
		status TEXT NOT NULL,
		first_seen_at INTEGER NOT NULL,
		last_seen_at INTEGER NOT NULL,
		resolved_at INTEGER,
		occurrence_count INTEGER NOT NULL DEFAULT 1,
		message TEXT,
		observed_value TEXT,
		expected_value TEXT,
		context_data_json TEXT
	);

	CREATE INDEX IF NOT EXISTS idx_compliance_findings_org_status ON compliance_findings(org_id, status);
	CREATE INDEX IF NOT EXISTS idx_compliance_findings_node ON compliance_findings(target_node_id);
	CREATE INDEX IF NOT EXISTS idx_compliance_findings_policy_rule ON compliance_findings(policy_id, rule_id);
	CREATE INDEX IF NOT EXISTS idx_compliance_findings_last_seen ON compliance_findings(last_seen_at DESC);

	CREATE TABLE IF NOT EXISTS governance_maintenance_windows (
		id TEXT PRIMARY KEY,
		org_id TEXT NOT NULL,
		name TEXT NOT NULL,
		description TEXT,
		status TEXT NOT NULL,
		target_scope TEXT NOT NULL,
		target_id TEXT,
		target_selector TEXT,
		category_restrictions_json TEXT,
		severity_threshold TEXT,
		start_time INTEGER NOT NULL,
		end_time INTEGER NOT NULL,
		time_zone TEXT,
		recurrence_json TEXT,
		suppress_alerts INTEGER NOT NULL,
		suppress_findings INTEGER NOT NULL,
		allow_critical_alerts INTEGER NOT NULL,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL,
		created_by TEXT,
		metadata_json TEXT
	);

	CREATE INDEX IF NOT EXISTS idx_maint_windows_org_status ON governance_maintenance_windows(org_id, status);
	CREATE INDEX IF NOT EXISTS idx_maint_windows_time ON governance_maintenance_windows(start_time, end_time);
	CREATE INDEX IF NOT EXISTS idx_maint_windows_target ON governance_maintenance_windows(target_scope, target_id);

	CREATE TABLE IF NOT EXISTS governance_escalation_policies (
		id TEXT PRIMARY KEY,
		org_id TEXT NOT NULL,
		name TEXT NOT NULL,
		description TEXT,
		enabled INTEGER NOT NULL,
		severity_levels_json TEXT NOT NULL,
		stages_json TEXT NOT NULL,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL,
		metadata_json TEXT
	);

	CREATE INDEX IF NOT EXISTS idx_escalation_policies_org ON governance_escalation_policies(org_id);
	CREATE INDEX IF NOT EXISTS idx_escalation_policies_enabled ON governance_escalation_policies(org_id, enabled);

	CREATE TABLE IF NOT EXISTS governance_suppression_decisions (
		id TEXT PRIMARY KEY,
		org_id TEXT NOT NULL,
		alert_id TEXT,
		incident_id TEXT,
		node_id TEXT,
		window_id TEXT,
		rule_id TEXT,
		rule_name TEXT,
		category TEXT,
		severity TEXT NOT NULL,
		outcome TEXT NOT NULL,
		reason TEXT NOT NULL,
		message TEXT,
		evaluated_at INTEGER NOT NULL,
		details_json TEXT
	);

	CREATE INDEX IF NOT EXISTS idx_suppression_decisions_org_time ON governance_suppression_decisions(org_id, evaluated_at DESC);
	CREATE INDEX IF NOT EXISTS idx_suppression_decisions_node ON governance_suppression_decisions(node_id);
	CREATE INDEX IF NOT EXISTS idx_suppression_decisions_window ON governance_suppression_decisions(window_id);
	CREATE INDEX IF NOT EXISTS idx_suppression_decisions_alert ON governance_suppression_decisions(alert_id);
	CREATE INDEX IF NOT EXISTS idx_suppression_decisions_outcome ON governance_suppression_decisions(outcome);

	INSERT OR IGNORE INTO organizations (id, name, display_name, description, status, created_at, updated_at, metadata_json)
	VALUES ('default', 'Default Organization', 'Default Organization', 'Built-in single-tenant organization boundary', 'active', 0, 0, '{}');
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
