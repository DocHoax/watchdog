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

// SaveEvaluationExecution persists an evaluation run and its results.
func (s *SQLiteStorage) SaveEvaluationExecution(ctx context.Context, exec *model.EvaluationExecution) error {
	if exec == nil {
		return fmt.Errorf("nil evaluation execution")
	}
	if err := exec.Validate(); err != nil {
		return fmt.Errorf("invalid evaluation execution: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	resultsJSON, err := json.Marshal(exec.Results)
	if err != nil {
		return fmt.Errorf("failed to marshal results: %w", err)
	}

	summaryJSON, err := json.Marshal(exec.Summary)
	if err != nil {
		return fmt.Errorf("failed to marshal summary: %w", err)
	}

	metaJSON, _ := json.Marshal(exec.Metadata)

	evalMs := exec.EvaluatedAt.UTC().UnixMilli()
	if exec.EvaluatedAt.IsZero() {
		evalMs = time.Now().UTC().UnixMilli()
	}

	query := `
		INSERT INTO policy_evaluations (
			id, org_id, target_node_id, trigger_type, evaluated_at,
			duration_ns, status, results_json, summary_json, metadata_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			org_id = excluded.org_id,
			target_node_id = excluded.target_node_id,
			trigger_type = excluded.trigger_type,
			evaluated_at = excluded.evaluated_at,
			duration_ns = excluded.duration_ns,
			status = excluded.status,
			results_json = excluded.results_json,
			summary_json = excluded.summary_json,
			metadata_json = excluded.metadata_json
	`

	_, err = s.db.ExecContext(ctx, query,
		exec.ID,
		exec.OrgID,
		exec.TargetNodeID,
		string(exec.TriggerType),
		evalMs,
		exec.DurationNs,
		string(exec.Status),
		string(resultsJSON),
		string(summaryJSON),
		string(metaJSON),
	)
	return err
}

// GetEvaluationExecution retrieves a single evaluation run by ID.
func (s *SQLiteStorage) GetEvaluationExecution(ctx context.Context, id string) (*model.EvaluationExecution, error) {
	if id == "" {
		return nil, fmt.Errorf("empty evaluation execution id")
	}

	query := `
		SELECT id, org_id, target_node_id, trigger_type, evaluated_at,
		       duration_ns, status, results_json, summary_json, metadata_json
		FROM policy_evaluations
		WHERE id = ?
	`

	row := s.db.QueryRowContext(ctx, query, id)

	var exec model.EvaluationExecution
	var triggerStr, statusStr string
	var evalMs int64
	var resultsStr, summaryStr, metaStr string

	err := row.Scan(
		&exec.ID,
		&exec.OrgID,
		&exec.TargetNodeID,
		&triggerStr,
		&evalMs,
		&exec.DurationNs,
		&statusStr,
		&resultsStr,
		&summaryStr,
		&metaStr,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("evaluation execution %q not found", id)
		}
		return nil, fmt.Errorf("failed to get evaluation execution %q: %w", id, err)
	}

	exec.TriggerType = model.EvaluationTriggerType(triggerStr)
	exec.Status = model.EvaluationStatus(statusStr)
	if evalMs > 0 {
		exec.EvaluatedAt = time.UnixMilli(evalMs).UTC()
	}
	if resultsStr != "" {
		_ = json.Unmarshal([]byte(resultsStr), &exec.Results)
	}
	if summaryStr != "" {
		_ = json.Unmarshal([]byte(summaryStr), &exec.Summary)
	}
	if metaStr != "" {
		_ = json.Unmarshal([]byte(metaStr), &exec.Metadata)
	}

	return &exec, nil
}

// GetLatestNodeEvaluation retrieves the most recent evaluation execution for a specific node in an organization.
func (s *SQLiteStorage) GetLatestNodeEvaluation(ctx context.Context, orgID, targetNodeID string) (*model.EvaluationExecution, error) {
	if targetNodeID == "" {
		return nil, fmt.Errorf("empty target node id")
	}

	query := `
		SELECT id, org_id, target_node_id, trigger_type, evaluated_at,
		       duration_ns, status, results_json, summary_json, metadata_json
		FROM policy_evaluations
		WHERE target_node_id = ?
	`
	args := []interface{}{targetNodeID}

	if orgID != "" {
		query += " AND org_id = ?"
		args = append(args, orgID)
	}

	query += " ORDER BY evaluated_at DESC LIMIT 1"

	row := s.db.QueryRowContext(ctx, query, args...)

	var exec model.EvaluationExecution
	var triggerStr, statusStr string
	var evalMs int64
	var resultsStr, summaryStr, metaStr string

	err := row.Scan(
		&exec.ID,
		&exec.OrgID,
		&exec.TargetNodeID,
		&triggerStr,
		&evalMs,
		&exec.DurationNs,
		&statusStr,
		&resultsStr,
		&summaryStr,
		&metaStr,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("no evaluation found for node %q", targetNodeID)
		}
		return nil, fmt.Errorf("failed to get latest evaluation for node %q: %w", targetNodeID, err)
	}

	exec.TriggerType = model.EvaluationTriggerType(triggerStr)
	exec.Status = model.EvaluationStatus(statusStr)
	if evalMs > 0 {
		exec.EvaluatedAt = time.UnixMilli(evalMs).UTC()
	}
	if resultsStr != "" {
		_ = json.Unmarshal([]byte(resultsStr), &exec.Results)
	}
	if summaryStr != "" {
		_ = json.Unmarshal([]byte(summaryStr), &exec.Summary)
	}
	if metaStr != "" {
		_ = json.Unmarshal([]byte(metaStr), &exec.Metadata)
	}

	return &exec, nil
}

// ListEvaluationExecutions lists evaluation executions matching the filter.
func (s *SQLiteStorage) ListEvaluationExecutions(ctx context.Context, filter model.EvaluationFilter) ([]model.EvaluationExecution, error) {
	var conditions []string
	var args []interface{}

	if filter.OrgID != "" {
		conditions = append(conditions, "org_id = ?")
		args = append(args, filter.OrgID)
	}
	if filter.TargetNodeID != "" {
		conditions = append(conditions, "target_node_id = ?")
		args = append(args, filter.TargetNodeID)
	}
	if filter.TriggerType != "" {
		conditions = append(conditions, "trigger_type = ?")
		args = append(args, string(filter.TriggerType))
	}
	if filter.Status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, string(filter.Status))
	}
	if !filter.Since.IsZero() {
		conditions = append(conditions, "evaluated_at >= ?")
		args = append(args, filter.Since.UTC().UnixMilli())
	}
	if !filter.Until.IsZero() {
		conditions = append(conditions, "evaluated_at <= ?")
		args = append(args, filter.Until.UTC().UnixMilli())
	}

	query := `
		SELECT id, org_id, target_node_id, trigger_type, evaluated_at,
		       duration_ns, status, results_json, summary_json, metadata_json
		FROM policy_evaluations
	`
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY evaluated_at DESC"

	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
		if filter.Offset > 0 {
			query += " OFFSET ?"
			args = append(args, filter.Offset)
		}
	} else if filter.Offset > 0 {
		query += " LIMIT -1 OFFSET ?"
		args = append(args, filter.Offset)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query policy evaluations: %w", err)
	}
	defer rows.Close()

	var list []model.EvaluationExecution
	for rows.Next() {
		var exec model.EvaluationExecution
		var triggerStr, statusStr string
		var evalMs int64
		var resultsStr, summaryStr, metaStr string

		if err := rows.Scan(
			&exec.ID,
			&exec.OrgID,
			&exec.TargetNodeID,
			&triggerStr,
			&evalMs,
			&exec.DurationNs,
			&statusStr,
			&resultsStr,
			&summaryStr,
			&metaStr,
		); err != nil {
			return nil, fmt.Errorf("failed to scan policy evaluation: %w", err)
		}

		exec.TriggerType = model.EvaluationTriggerType(triggerStr)
		exec.Status = model.EvaluationStatus(statusStr)
		if evalMs > 0 {
			exec.EvaluatedAt = time.UnixMilli(evalMs).UTC()
		}
		if resultsStr != "" {
			_ = json.Unmarshal([]byte(resultsStr), &exec.Results)
		}
		if summaryStr != "" {
			_ = json.Unmarshal([]byte(summaryStr), &exec.Summary)
		}
		if metaStr != "" {
			_ = json.Unmarshal([]byte(metaStr), &exec.Metadata)
		}

		list = append(list, exec)
	}

	return list, rows.Err()
}

// SaveComplianceFinding inserts or updates a single compliance finding.
func (s *SQLiteStorage) SaveComplianceFinding(ctx context.Context, finding *model.ComplianceFinding) error {
	if finding == nil {
		return fmt.Errorf("nil compliance finding")
	}
	if err := finding.Validate(); err != nil {
		return fmt.Errorf("invalid compliance finding: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	return s.saveComplianceFindingLocked(ctx, finding)
}

func (s *SQLiteStorage) saveComplianceFindingLocked(ctx context.Context, finding *model.ComplianceFinding) error {
	ctxJSON, _ := json.Marshal(finding.ContextData)

	firstSeenMs := finding.FirstSeenAt.UTC().UnixMilli()
	if finding.FirstSeenAt.IsZero() {
		firstSeenMs = time.Now().UTC().UnixMilli()
	}
	lastSeenMs := finding.LastSeenAt.UTC().UnixMilli()
	if finding.LastSeenAt.IsZero() {
		lastSeenMs = firstSeenMs
	}

	var resolvedMs *int64
	if finding.ResolvedAt != nil && !finding.ResolvedAt.IsZero() {
		v := finding.ResolvedAt.UTC().UnixMilli()
		resolvedMs = &v
	}

	query := `
		INSERT INTO compliance_findings (
			id, org_id, target_node_id, policy_id, policy_revision, rule_id, rule_name,
			category, severity, enforcement_mode, status, first_seen_at, last_seen_at,
			resolved_at, occurrence_count, message, observed_value, expected_value, context_data_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			org_id = excluded.org_id,
			target_node_id = excluded.target_node_id,
			policy_id = excluded.policy_id,
			policy_revision = excluded.policy_revision,
			rule_id = excluded.rule_id,
			rule_name = excluded.rule_name,
			category = excluded.category,
			severity = excluded.severity,
			enforcement_mode = excluded.enforcement_mode,
			status = excluded.status,
			first_seen_at = excluded.first_seen_at,
			last_seen_at = excluded.last_seen_at,
			resolved_at = excluded.resolved_at,
			occurrence_count = excluded.occurrence_count,
			message = excluded.message,
			observed_value = excluded.observed_value,
			expected_value = excluded.expected_value,
			context_data_json = excluded.context_data_json
	`

	_, err := s.db.ExecContext(ctx, query,
		finding.ID,
		finding.OrgID,
		finding.TargetNodeID,
		finding.PolicyID,
		finding.PolicyRevision,
		finding.RuleID,
		finding.RuleName,
		string(finding.Category),
		string(finding.Severity),
		string(finding.EnforcementMode),
		string(finding.Status),
		firstSeenMs,
		lastSeenMs,
		resolvedMs,
		finding.OccurrenceCount,
		finding.Message,
		finding.ObservedValue,
		finding.ExpectedValue,
		string(ctxJSON),
	)
	return err
}

// SaveComplianceFindings batch-persists multiple compliance findings within a single transaction.
func (s *SQLiteStorage) SaveComplianceFindings(ctx context.Context, findings []model.ComplianceFinding) error {
	if len(findings) == 0 {
		return nil
	}

	for i := range findings {
		if err := findings[i].Validate(); err != nil {
			return fmt.Errorf("finding[%d] invalid: %w", i, err)
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
		INSERT INTO compliance_findings (
			id, org_id, target_node_id, policy_id, policy_revision, rule_id, rule_name,
			category, severity, enforcement_mode, status, first_seen_at, last_seen_at,
			resolved_at, occurrence_count, message, observed_value, expected_value, context_data_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			org_id = excluded.org_id,
			target_node_id = excluded.target_node_id,
			policy_id = excluded.policy_id,
			policy_revision = excluded.policy_revision,
			rule_id = excluded.rule_id,
			rule_name = excluded.rule_name,
			category = excluded.category,
			severity = excluded.severity,
			enforcement_mode = excluded.enforcement_mode,
			status = excluded.status,
			first_seen_at = excluded.first_seen_at,
			last_seen_at = excluded.last_seen_at,
			resolved_at = excluded.resolved_at,
			occurrence_count = excluded.occurrence_count,
			message = excluded.message,
			observed_value = excluded.observed_value,
			expected_value = excluded.expected_value,
			context_data_json = excluded.context_data_json
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, finding := range findings {
		ctxJSON, _ := json.Marshal(finding.ContextData)

		firstSeenMs := finding.FirstSeenAt.UTC().UnixMilli()
		if finding.FirstSeenAt.IsZero() {
			firstSeenMs = time.Now().UTC().UnixMilli()
		}
		lastSeenMs := finding.LastSeenAt.UTC().UnixMilli()
		if finding.LastSeenAt.IsZero() {
			lastSeenMs = firstSeenMs
		}

		var resolvedMs *int64
		if finding.ResolvedAt != nil && !finding.ResolvedAt.IsZero() {
			v := finding.ResolvedAt.UTC().UnixMilli()
			resolvedMs = &v
		}

		_, err := stmt.ExecContext(ctx,
			finding.ID,
			finding.OrgID,
			finding.TargetNodeID,
			finding.PolicyID,
			finding.PolicyRevision,
			finding.RuleID,
			finding.RuleName,
			string(finding.Category),
			string(finding.Severity),
			string(finding.EnforcementMode),
			string(finding.Status),
			firstSeenMs,
			lastSeenMs,
			resolvedMs,
			finding.OccurrenceCount,
			finding.Message,
			finding.ObservedValue,
			finding.ExpectedValue,
			string(ctxJSON),
		)
		if err != nil {
			return fmt.Errorf("failed to insert finding %s: %w", finding.ID, err)
		}
	}

	return tx.Commit()
}

// GetComplianceFinding retrieves a single compliance finding by ID.
func (s *SQLiteStorage) GetComplianceFinding(ctx context.Context, id string) (*model.ComplianceFinding, error) {
	if id == "" {
		return nil, fmt.Errorf("empty finding id")
	}

	query := `
		SELECT id, org_id, target_node_id, policy_id, policy_revision, rule_id, rule_name,
		       category, severity, enforcement_mode, status, first_seen_at, last_seen_at,
		       resolved_at, occurrence_count, message, observed_value, expected_value, context_data_json
		FROM compliance_findings
		WHERE id = ?
	`

	row := s.db.QueryRowContext(ctx, query, id)

	var f model.ComplianceFinding
	var catStr, sevStr, enfStr, statusStr string
	var firstSeenMs, lastSeenMs int64
	var resolvedMs *int64
	var ctxStr string

	err := row.Scan(
		&f.ID,
		&f.OrgID,
		&f.TargetNodeID,
		&f.PolicyID,
		&f.PolicyRevision,
		&f.RuleID,
		&f.RuleName,
		&catStr,
		&sevStr,
		&enfStr,
		&statusStr,
		&firstSeenMs,
		&lastSeenMs,
		&resolvedMs,
		&f.OccurrenceCount,
		&f.Message,
		&f.ObservedValue,
		&f.ExpectedValue,
		&ctxStr,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("compliance finding %q not found", id)
		}
		return nil, fmt.Errorf("failed to get compliance finding %q: %w", id, err)
	}

	f.Category = model.PolicyCategory(catStr)
	f.Severity = model.Severity(sevStr)
	f.EnforcementMode = model.EnforcementMode(enfStr)
	f.Status = model.FindingStatus(statusStr)
	if firstSeenMs > 0 {
		f.FirstSeenAt = time.UnixMilli(firstSeenMs).UTC()
	}
	if lastSeenMs > 0 {
		f.LastSeenAt = time.UnixMilli(lastSeenMs).UTC()
	}
	if resolvedMs != nil && *resolvedMs > 0 {
		t := time.UnixMilli(*resolvedMs).UTC()
		f.ResolvedAt = &t
	}
	if ctxStr != "" {
		_ = json.Unmarshal([]byte(ctxStr), &f.ContextData)
	}

	return &f, nil
}

// ListComplianceFindings lists compliance findings matching the filter.
func (s *SQLiteStorage) ListComplianceFindings(ctx context.Context, filter model.FindingFilter) ([]model.ComplianceFinding, error) {
	var conditions []string
	var args []interface{}

	if filter.OrgID != "" {
		conditions = append(conditions, "org_id = ?")
		args = append(args, filter.OrgID)
	}
	if filter.TargetNodeID != "" {
		conditions = append(conditions, "target_node_id = ?")
		args = append(args, filter.TargetNodeID)
	}
	if filter.PolicyID != "" {
		conditions = append(conditions, "policy_id = ?")
		args = append(args, filter.PolicyID)
	}
	if filter.RuleID != "" {
		conditions = append(conditions, "rule_id = ?")
		args = append(args, filter.RuleID)
	}
	if filter.Category != "" {
		conditions = append(conditions, "category = ?")
		args = append(args, string(filter.Category))
	}
	if filter.Severity != "" {
		conditions = append(conditions, "severity = ?")
		args = append(args, string(filter.Severity))
	}
	if filter.Status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, string(filter.Status))
	}
	if !filter.Since.IsZero() {
		conditions = append(conditions, "last_seen_at >= ?")
		args = append(args, filter.Since.UTC().UnixMilli())
	}
	if !filter.Until.IsZero() {
		conditions = append(conditions, "last_seen_at <= ?")
		args = append(args, filter.Until.UTC().UnixMilli())
	}

	query := `
		SELECT id, org_id, target_node_id, policy_id, policy_revision, rule_id, rule_name,
		       category, severity, enforcement_mode, status, first_seen_at, last_seen_at,
		       resolved_at, occurrence_count, message, observed_value, expected_value, context_data_json
		FROM compliance_findings
	`
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY last_seen_at DESC"

	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
		if filter.Offset > 0 {
			query += " OFFSET ?"
			args = append(args, filter.Offset)
		}
	} else if filter.Offset > 0 {
		query += " LIMIT -1 OFFSET ?"
		args = append(args, filter.Offset)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query compliance findings: %w", err)
	}
	defer rows.Close()

	var list []model.ComplianceFinding
	for rows.Next() {
		var f model.ComplianceFinding
		var catStr, sevStr, enfStr, statusStr string
		var firstSeenMs, lastSeenMs int64
		var resolvedMs *int64
		var ctxStr string

		if err := rows.Scan(
			&f.ID,
			&f.OrgID,
			&f.TargetNodeID,
			&f.PolicyID,
			&f.PolicyRevision,
			&f.RuleID,
			&f.RuleName,
			&catStr,
			&sevStr,
			&enfStr,
			&statusStr,
			&firstSeenMs,
			&lastSeenMs,
			&resolvedMs,
			&f.OccurrenceCount,
			&f.Message,
			&f.ObservedValue,
			&f.ExpectedValue,
			&ctxStr,
		); err != nil {
			return nil, fmt.Errorf("failed to scan compliance finding: %w", err)
		}

		f.Category = model.PolicyCategory(catStr)
		f.Severity = model.Severity(sevStr)
		f.EnforcementMode = model.EnforcementMode(enfStr)
		f.Status = model.FindingStatus(statusStr)
		if firstSeenMs > 0 {
			f.FirstSeenAt = time.UnixMilli(firstSeenMs).UTC()
		}
		if lastSeenMs > 0 {
			f.LastSeenAt = time.UnixMilli(lastSeenMs).UTC()
		}
		if resolvedMs != nil && *resolvedMs > 0 {
			t := time.UnixMilli(*resolvedMs).UTC()
			f.ResolvedAt = &t
		}
		if ctxStr != "" {
			_ = json.Unmarshal([]byte(ctxStr), &f.ContextData)
		}

		list = append(list, f)
	}

	return list, rows.Err()
}

// DeleteComplianceFinding removes a compliance finding by ID.
func (s *SQLiteStorage) DeleteComplianceFinding(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("empty finding id")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	result, err := s.db.ExecContext(ctx, "DELETE FROM compliance_findings WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete compliance finding %q: %w", id, err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("compliance finding %q not found", id)
	}

	return nil
}

// PruneEvaluationExecutions deletes evaluation executions older than the specified retention duration.
func (s *SQLiteStorage) PruneEvaluationExecutions(ctx context.Context, retention time.Duration) (int64, error) {
	if retention <= 0 {
		return 0, nil
	}

	cutoff := time.Now().UTC().Add(-retention).UnixMilli()

	s.mu.Lock()
	defer s.mu.Unlock()

	result, err := s.db.ExecContext(ctx, "DELETE FROM policy_evaluations WHERE evaluated_at < ?", cutoff)
	if err != nil {
		return 0, fmt.Errorf("failed to prune policy evaluations: %w", err)
	}

	return result.RowsAffected()
}
