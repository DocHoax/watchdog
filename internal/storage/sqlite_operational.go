package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/DocHoax/watchdog/pkg/model"
)

// ============================================================================
// Maintenance Windows
// ============================================================================

// SaveMaintenanceWindow persists or updates a maintenance window.
func (s *SQLiteStorage) SaveMaintenanceWindow(ctx context.Context, window *model.MaintenanceWindow) error {
	if window == nil {
		return fmt.Errorf("nil maintenance window")
	}
	if err := window.Validate(); err != nil {
		return fmt.Errorf("invalid maintenance window: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	if window.CreatedAt.IsZero() {
		window.CreatedAt = now
	}
	window.UpdatedAt = now

	catJSON, err := json.Marshal(window.CategoryRestrictions)
	if err != nil {
		return fmt.Errorf("failed to marshal category restrictions: %w", err)
	}

	var recJSON []byte
	if window.Schedule.Recurrence != nil {
		recJSON, err = json.Marshal(window.Schedule.Recurrence)
		if err != nil {
			return fmt.Errorf("failed to marshal recurrence: %w", err)
		}
	}

	metaJSON, _ := json.Marshal(window.Metadata)

	query := `
		INSERT INTO governance_maintenance_windows (
			id, org_id, name, description, status, target_scope, target_id,
			target_selector, category_restrictions_json, severity_threshold,
			start_time, end_time, time_zone, recurrence_json, suppress_alerts,
			suppress_findings, allow_critical_alerts, created_at, updated_at,
			created_by, metadata_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			org_id = excluded.org_id,
			name = excluded.name,
			description = excluded.description,
			status = excluded.status,
			target_scope = excluded.target_scope,
			target_id = excluded.target_id,
			target_selector = excluded.target_selector,
			category_restrictions_json = excluded.category_restrictions_json,
			severity_threshold = excluded.severity_threshold,
			start_time = excluded.start_time,
			end_time = excluded.end_time,
			time_zone = excluded.time_zone,
			recurrence_json = excluded.recurrence_json,
			suppress_alerts = excluded.suppress_alerts,
			suppress_findings = excluded.suppress_findings,
			allow_critical_alerts = excluded.allow_critical_alerts,
			updated_at = excluded.updated_at,
			created_by = excluded.created_by,
			metadata_json = excluded.metadata_json
	`

	suppressAlertsInt := 0
	if window.SuppressAlerts {
		suppressAlertsInt = 1
	}
	suppressFindingsInt := 0
	if window.SuppressFindings {
		suppressFindingsInt = 1
	}
	allowCriticalInt := 0
	if window.AllowCriticalAlerts {
		allowCriticalInt = 1
	}

	var recJSONStr sql.NullString
	if len(recJSON) > 0 {
		recJSONStr = sql.NullString{String: string(recJSON), Valid: true}
	}

	_, err = s.db.ExecContext(ctx, query,
		window.ID,
		window.OrgID,
		window.Name,
		window.Description,
		string(window.Status),
		string(window.TargetScope),
		window.TargetID,
		window.TargetSelector,
		string(catJSON),
		string(window.SeverityThreshold),
		window.Schedule.StartTime.UTC().UnixMilli(),
		window.Schedule.EndTime.UTC().UnixMilli(),
		window.Schedule.TimeZone,
		recJSONStr,
		suppressAlertsInt,
		suppressFindingsInt,
		allowCriticalInt,
		window.CreatedAt.UTC().UnixMilli(),
		window.UpdatedAt.UTC().UnixMilli(),
		window.CreatedBy,
		string(metaJSON),
	)
	if err != nil {
		return fmt.Errorf("failed to save maintenance window: %w", err)
	}

	return nil
}

// GetMaintenanceWindow retrieves a maintenance window by ID.
func (s *SQLiteStorage) GetMaintenanceWindow(ctx context.Context, id string) (*model.MaintenanceWindow, error) {
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("empty maintenance window id")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `
		SELECT id, org_id, name, description, status, target_scope, target_id,
		       target_selector, category_restrictions_json, severity_threshold,
		       start_time, end_time, time_zone, recurrence_json, suppress_alerts,
		       suppress_findings, allow_critical_alerts, created_at, updated_at,
		       created_by, metadata_json
		FROM governance_maintenance_windows
		WHERE id = ?
	`

	row := s.db.QueryRowContext(ctx, query, id)
	return scanMaintenanceWindow(row)
}

// ListMaintenanceWindows lists maintenance windows matching the filter.
func (s *SQLiteStorage) ListMaintenanceWindows(ctx context.Context, filter model.MaintenanceWindowFilter) ([]model.MaintenanceWindow, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var conditions []string
	var args []interface{}

	if filter.OrgID != "" {
		conditions = append(conditions, "org_id = ?")
		args = append(args, filter.OrgID)
	}

	if filter.Status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, string(filter.Status))
	}

	if filter.TargetScope != "" {
		conditions = append(conditions, "target_scope = ?")
		args = append(args, string(filter.TargetScope))
	}

	if filter.TargetID != "" {
		conditions = append(conditions, "target_id = ?")
		args = append(args, filter.TargetID)
	}

	if filter.ActiveAt != nil {
		ms := filter.ActiveAt.UTC().UnixMilli()
		conditions = append(conditions, "(start_time <= ? AND end_time >= ?)")
		args = append(args, ms, ms)
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	limit := filter.Limit
	if limit <= 0 || limit > 1000 {
		limit = 100
	}

	query := fmt.Sprintf(`
		SELECT id, org_id, name, description, status, target_scope, target_id,
		       target_selector, category_restrictions_json, severity_threshold,
		       start_time, end_time, time_zone, recurrence_json, suppress_alerts,
		       suppress_findings, allow_critical_alerts, created_at, updated_at,
		       created_by, metadata_json
		FROM governance_maintenance_windows
		%s
		ORDER BY start_time ASC
		LIMIT ? OFFSET ?
	`, whereClause)

	args = append(args, limit, filter.Offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query maintenance windows: %w", err)
	}
	defer rows.Close()

	var results []model.MaintenanceWindow
	for rows.Next() {
		win, err := scanMaintenanceWindow(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, *win)
	}

	return results, rows.Err()
}

// DeleteMaintenanceWindow removes a maintenance window by ID.
func (s *SQLiteStorage) DeleteMaintenanceWindow(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("empty maintenance window id")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	res, err := s.db.ExecContext(ctx, `DELETE FROM governance_maintenance_windows WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to delete maintenance window: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("maintenance window %q not found", id)
	}

	return nil
}

func scanMaintenanceWindow(s scannable) (*model.MaintenanceWindow, error) {
	var win model.MaintenanceWindow
	var statusStr, scopeStr, sevStr string
	var catJSON, recJSON, metaJSON sql.NullString
	var startMs, endMs, createdMs, updatedMs int64
	var suppressAlertsInt, suppressFindingsInt, allowCriticalInt int

	err := s.Scan(
		&win.ID,
		&win.OrgID,
		&win.Name,
		&win.Description,
		&statusStr,
		&scopeStr,
		&win.TargetID,
		&win.TargetSelector,
		&catJSON,
		&sevStr,
		&startMs,
		&endMs,
		&win.Schedule.TimeZone,
		&recJSON,
		&suppressAlertsInt,
		&suppressFindingsInt,
		&allowCriticalInt,
		&createdMs,
		&updatedMs,
		&win.CreatedBy,
		&metaJSON,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("maintenance window not found")
		}
		return nil, fmt.Errorf("failed to scan maintenance window: %w", err)
	}

	win.Status = model.MaintenanceStatus(statusStr)
	win.TargetScope = model.MaintenanceTargetScope(scopeStr)
	win.SeverityThreshold = model.Severity(sevStr)
	win.Schedule.StartTime = time.UnixMilli(startMs).UTC()
	win.Schedule.EndTime = time.UnixMilli(endMs).UTC()
	win.SuppressAlerts = (suppressAlertsInt == 1)
	win.SuppressFindings = (suppressFindingsInt == 1)
	win.AllowCriticalAlerts = (allowCriticalInt == 1)
	win.CreatedAt = time.UnixMilli(createdMs).UTC()
	win.UpdatedAt = time.UnixMilli(updatedMs).UTC()

	if catJSON.Valid && catJSON.String != "" {
		_ = json.Unmarshal([]byte(catJSON.String), &win.CategoryRestrictions)
	}

	if recJSON.Valid && recJSON.String != "" {
		var rec model.RecurrenceSchedule
		if err := json.Unmarshal([]byte(recJSON.String), &rec); err == nil {
			win.Schedule.Recurrence = &rec
		}
	}

	if metaJSON.Valid && metaJSON.String != "" {
		_ = json.Unmarshal([]byte(metaJSON.String), &win.Metadata)
	}

	return &win, nil
}

// ============================================================================
// Escalation Policies
// ============================================================================

// SaveEscalationPolicy persists or updates an escalation policy.
func (s *SQLiteStorage) SaveEscalationPolicy(ctx context.Context, policy *model.EscalationPolicy) error {
	if policy == nil {
		return fmt.Errorf("nil escalation policy")
	}
	if err := policy.Validate(); err != nil {
		return fmt.Errorf("invalid escalation policy: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	if policy.CreatedAt.IsZero() {
		policy.CreatedAt = now
	}
	policy.UpdatedAt = now

	sevJSON, err := json.Marshal(policy.SeverityLevels)
	if err != nil {
		return fmt.Errorf("failed to marshal severity levels: %w", err)
	}

	stagesJSON, err := json.Marshal(policy.Stages)
	if err != nil {
		return fmt.Errorf("failed to marshal stages: %w", err)
	}

	metaJSON, _ := json.Marshal(policy.Metadata)

	query := `
		INSERT INTO governance_escalation_policies (
			id, org_id, name, description, enabled, severity_levels_json,
			stages_json, created_at, updated_at, metadata_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			org_id = excluded.org_id,
			name = excluded.name,
			description = excluded.description,
			enabled = excluded.enabled,
			severity_levels_json = excluded.severity_levels_json,
			stages_json = excluded.stages_json,
			updated_at = excluded.updated_at,
			metadata_json = excluded.metadata_json
	`

	enabledInt := 0
	if policy.Enabled {
		enabledInt = 1
	}

	_, err = s.db.ExecContext(ctx, query,
		policy.ID,
		policy.OrgID,
		policy.Name,
		policy.Description,
		enabledInt,
		string(sevJSON),
		string(stagesJSON),
		policy.CreatedAt.UTC().UnixMilli(),
		policy.UpdatedAt.UTC().UnixMilli(),
		string(metaJSON),
	)
	if err != nil {
		return fmt.Errorf("failed to save escalation policy: %w", err)
	}

	return nil
}

// GetEscalationPolicy retrieves an escalation policy by ID.
func (s *SQLiteStorage) GetEscalationPolicy(ctx context.Context, id string) (*model.EscalationPolicy, error) {
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("empty escalation policy id")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `
		SELECT id, org_id, name, description, enabled, severity_levels_json,
		       stages_json, created_at, updated_at, metadata_json
		FROM governance_escalation_policies
		WHERE id = ?
	`

	row := s.db.QueryRowContext(ctx, query, id)
	return scanEscalationPolicy(row)
}

// ListEscalationPolicies lists escalation policies matching the filter.
func (s *SQLiteStorage) ListEscalationPolicies(ctx context.Context, filter model.EscalationPolicyFilter) ([]model.EscalationPolicy, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var conditions []string
	var args []interface{}

	if filter.OrgID != "" {
		conditions = append(conditions, "org_id = ?")
		args = append(args, filter.OrgID)
	}

	if filter.Enabled != nil {
		if *filter.Enabled {
			conditions = append(conditions, "enabled = 1")
		} else {
			conditions = append(conditions, "enabled = 0")
		}
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	limit := filter.Limit
	if limit <= 0 || limit > 1000 {
		limit = 100
	}

	query := fmt.Sprintf(`
		SELECT id, org_id, name, description, enabled, severity_levels_json,
		       stages_json, created_at, updated_at, metadata_json
		FROM governance_escalation_policies
		%s
		ORDER BY created_at ASC
		LIMIT ? OFFSET ?
	`, whereClause)

	args = append(args, limit, filter.Offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query escalation policies: %w", err)
	}
	defer rows.Close()

	var results []model.EscalationPolicy
	for rows.Next() {
		p, err := scanEscalationPolicy(rows)
		if err != nil {
			return nil, err
		}

		// Optional in-memory filter for severity level if specified
		if filter.Severity != "" {
			matched := false
			for _, s := range p.SeverityLevels {
				if s == filter.Severity {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}

		results = append(results, *p)
	}

	return results, rows.Err()
}

// DeleteEscalationPolicy removes an escalation policy by ID.
func (s *SQLiteStorage) DeleteEscalationPolicy(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("empty escalation policy id")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	res, err := s.db.ExecContext(ctx, `DELETE FROM governance_escalation_policies WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to delete escalation policy: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("escalation policy %q not found", id)
	}

	return nil
}

func scanEscalationPolicy(s scannable) (*model.EscalationPolicy, error) {
	var policy model.EscalationPolicy
	var enabledInt int
	var sevJSON, stagesJSON, metaJSON sql.NullString
	var createdMs, updatedMs int64

	err := s.Scan(
		&policy.ID,
		&policy.OrgID,
		&policy.Name,
		&policy.Description,
		&enabledInt,
		&sevJSON,
		&stagesJSON,
		&createdMs,
		&updatedMs,
		&metaJSON,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("escalation policy not found")
		}
		return nil, fmt.Errorf("failed to scan escalation policy: %w", err)
	}

	policy.Enabled = (enabledInt == 1)
	policy.CreatedAt = time.UnixMilli(createdMs).UTC()
	policy.UpdatedAt = time.UnixMilli(updatedMs).UTC()

	if sevJSON.Valid && sevJSON.String != "" {
		_ = json.Unmarshal([]byte(sevJSON.String), &policy.SeverityLevels)
	}

	if stagesJSON.Valid && stagesJSON.String != "" {
		_ = json.Unmarshal([]byte(stagesJSON.String), &policy.Stages)
	}

	if metaJSON.Valid && metaJSON.String != "" {
		_ = json.Unmarshal([]byte(metaJSON.String), &policy.Metadata)
	}

	return &policy, nil
}

// ============================================================================
// Suppression Decisions
// ============================================================================

// SaveSuppressionDecision stores an alert or incident suppression evaluation decision.
func (s *SQLiteStorage) SaveSuppressionDecision(ctx context.Context, decision *model.SuppressionDecision) error {
	if decision == nil {
		return fmt.Errorf("nil suppression decision")
	}
	if err := decision.Validate(); err != nil {
		return fmt.Errorf("invalid suppression decision: %w", err)
	}

	return s.SaveSuppressionDecisions(ctx, []model.SuppressionDecision{*decision})
}

// SaveSuppressionDecisions batch stores suppression decisions within a single transaction.
func (s *SQLiteStorage) SaveSuppressionDecisions(ctx context.Context, decisions []model.SuppressionDecision) error {
	if len(decisions) == 0 {
		return nil
	}

	for i, d := range decisions {
		if err := d.Validate(); err != nil {
			return fmt.Errorf("invalid suppression decision at index %d: %w", i, err)
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO governance_suppression_decisions (
			id, org_id, alert_id, incident_id, node_id, window_id, rule_id,
			rule_name, category, severity, outcome, reason, message,
			evaluated_at, details_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			org_id = excluded.org_id,
			alert_id = excluded.alert_id,
			incident_id = excluded.incident_id,
			node_id = excluded.node_id,
			window_id = excluded.window_id,
			rule_id = excluded.rule_id,
			rule_name = excluded.rule_name,
			category = excluded.category,
			severity = excluded.severity,
			outcome = excluded.outcome,
			reason = excluded.reason,
			message = excluded.message,
			evaluated_at = excluded.evaluated_at,
			details_json = excluded.details_json
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, d := range decisions {
		evalMs := d.EvaluatedAt.UTC().UnixMilli()
		if d.EvaluatedAt.IsZero() {
			evalMs = time.Now().UTC().UnixMilli()
		}

		var detailsJSON []byte
		if len(d.Details) > 0 {
			detailsJSON, _ = json.Marshal(d.Details)
		}

		_, err := stmt.ExecContext(ctx,
			d.ID,
			d.OrgID,
			d.AlertID,
			d.IncidentID,
			d.NodeID,
			d.WindowID,
			d.RuleID,
			d.RuleName,
			d.Category,
			string(d.Severity),
			string(d.Outcome),
			string(d.Reason),
			d.Message,
			evalMs,
			string(detailsJSON),
		)
		if err != nil {
			return fmt.Errorf("failed to insert suppression decision %s: %w", d.ID, err)
		}
	}

	return tx.Commit()
}

// GetSuppressionDecision retrieves a suppression decision by ID.
func (s *SQLiteStorage) GetSuppressionDecision(ctx context.Context, id string) (*model.SuppressionDecision, error) {
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("empty suppression decision id")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `
		SELECT id, org_id, alert_id, incident_id, node_id, window_id, rule_id,
		       rule_name, category, severity, outcome, reason, message,
		       evaluated_at, details_json
		FROM governance_suppression_decisions
		WHERE id = ?
	`

	row := s.db.QueryRowContext(ctx, query, id)
	return scanSuppressionDecision(row)
}

// ListSuppressionDecisions queries suppression decisions with filtering.
func (s *SQLiteStorage) ListSuppressionDecisions(ctx context.Context, filter model.SuppressionFilter) ([]model.SuppressionDecision, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var conditions []string
	var args []interface{}

	if filter.OrgID != "" {
		conditions = append(conditions, "org_id = ?")
		args = append(args, filter.OrgID)
	}

	if filter.AlertID != "" {
		conditions = append(conditions, "alert_id = ?")
		args = append(args, filter.AlertID)
	}

	if filter.IncidentID != "" {
		conditions = append(conditions, "incident_id = ?")
		args = append(args, filter.IncidentID)
	}

	if filter.NodeID != "" {
		conditions = append(conditions, "node_id = ?")
		args = append(args, filter.NodeID)
	}

	if filter.WindowID != "" {
		conditions = append(conditions, "window_id = ?")
		args = append(args, filter.WindowID)
	}

	if filter.Outcome != "" {
		conditions = append(conditions, "outcome = ?")
		args = append(args, string(filter.Outcome))
	}

	if filter.Reason != "" {
		conditions = append(conditions, "reason = ?")
		args = append(args, string(filter.Reason))
	}

	if filter.Severity != "" {
		conditions = append(conditions, "severity = ?")
		args = append(args, string(filter.Severity))
	}

	if filter.Since != nil {
		conditions = append(conditions, "evaluated_at >= ?")
		args = append(args, filter.Since.UTC().UnixMilli())
	}

	if filter.Until != nil {
		conditions = append(conditions, "evaluated_at <= ?")
		args = append(args, filter.Until.UTC().UnixMilli())
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	limit := filter.Limit
	if limit <= 0 || limit > 1000 {
		limit = 100
	}

	query := fmt.Sprintf(`
		SELECT id, org_id, alert_id, incident_id, node_id, window_id, rule_id,
		       rule_name, category, severity, outcome, reason, message,
		       evaluated_at, details_json
		FROM governance_suppression_decisions
		%s
		ORDER BY evaluated_at DESC
		LIMIT ? OFFSET ?
	`, whereClause)

	args = append(args, limit, filter.Offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query suppression decisions: %w", err)
	}
	defer rows.Close()

	var results []model.SuppressionDecision
	for rows.Next() {
		dec, err := scanSuppressionDecision(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, *dec)
	}

	return results, rows.Err()
}

// PruneSuppressionDecisions removes decisions older than the given retention duration.
func (s *SQLiteStorage) PruneSuppressionDecisions(ctx context.Context, retention time.Duration) (int64, error) {
	if retention <= 0 {
		return 0, nil
	}

	cutoff := time.Now().UTC().Add(-retention).UnixMilli()

	s.mu.Lock()
	defer s.mu.Unlock()

	res, err := s.db.ExecContext(ctx, `DELETE FROM governance_suppression_decisions WHERE evaluated_at < ?`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("failed to prune suppression decisions: %w", err)
	}

	return res.RowsAffected()
}

func scanSuppressionDecision(s scannable) (*model.SuppressionDecision, error) {
	var dec model.SuppressionDecision
	var sevStr, outcomeStr, reasonStr string
	var evalMs int64
	var detailsJSON sql.NullString

	err := s.Scan(
		&dec.ID,
		&dec.OrgID,
		&dec.AlertID,
		&dec.IncidentID,
		&dec.NodeID,
		&dec.WindowID,
		&dec.RuleID,
		&dec.RuleName,
		&dec.Category,
		&sevStr,
		&outcomeStr,
		&reasonStr,
		&dec.Message,
		&evalMs,
		&detailsJSON,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("suppression decision not found")
		}
		return nil, fmt.Errorf("failed to scan suppression decision: %w", err)
	}

	dec.Severity = model.Severity(sevStr)
	dec.Outcome = model.SuppressionOutcome(outcomeStr)
	dec.Reason = model.SuppressionReason(reasonStr)
	dec.EvaluatedAt = time.UnixMilli(evalMs).UTC()

	if detailsJSON.Valid && detailsJSON.String != "" {
		_ = json.Unmarshal([]byte(detailsJSON.String), &dec.Details)
	}

	return &dec, nil
}
