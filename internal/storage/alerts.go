package storage

import (
	"context"
	"database/sql"
	"time"

	"github.com/DocHoax/watchdog/pkg/model"
)

// SaveAlertEvent stores a new or updated alert event.
func (s *SQLiteStorage) SaveAlertEvent(ctx context.Context, alert model.AlertEvent) error {
	status := string(model.AlertStatusActive)
	if !alert.IsActive || alert.ResolvedAt != nil {
		status = string(model.AlertStatusResolved)
	}

	var resolvedTs sql.NullInt64
	if alert.ResolvedAt != nil {
		resolvedTs.Int64 = alert.ResolvedAt.UnixMilli()
		resolvedTs.Valid = true
	}

	query := `
		INSERT INTO alerts_history (
			id, rule_name, category, severity, status, metric, metric_value, threshold, message, triggered_at, resolved_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			status = excluded.status,
			metric_value = excluded.metric_value,
			resolved_at = excluded.resolved_at,
			message = excluded.message
	`

	_, err := s.db.ExecContext(ctx, query,
		alert.ID,
		alert.RuleName,
		alert.RuleID,
		string(alert.Severity),
		status,
		alert.MetricName,
		alert.ActualValue,
		alert.Threshold,
		alert.Message,
		alert.FiredAt.UnixMilli(),
		resolvedTs,
	)
	return err
}

// UpdateAlertStatus updates the status and optional resolved timestamp of an alert.
func (s *SQLiteStorage) UpdateAlertStatus(ctx context.Context, id string, status model.AlertStatus, resolvedAt *time.Time) error {
	var resolvedTs sql.NullInt64
	if resolvedAt != nil {
		resolvedTs.Int64 = resolvedAt.UnixMilli()
		resolvedTs.Valid = true
	}

	query := `UPDATE alerts_history SET status = ?, resolved_at = ? WHERE id = ?`
	_, err := s.db.ExecContext(ctx, query, string(status), resolvedTs, id)
	return err
}

// GetAlertHistory returns historical alerts sorted by trigger time descending.
func (s *SQLiteStorage) GetAlertHistory(ctx context.Context, limit int, offset int) ([]model.AlertEvent, error) {
	if limit <= 0 {
		limit = 100
	}

	query := `
		SELECT id, rule_name, category, severity, status, metric, metric_value, threshold, message, triggered_at, resolved_at
		FROM alerts_history
		ORDER BY triggered_at DESC
		LIMIT ? OFFSET ?
	`

	rows, err := s.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanAlertRows(rows)
}

// GetActiveAlerts returns all currently active alerts.
func (s *SQLiteStorage) GetActiveAlerts(ctx context.Context) ([]model.AlertEvent, error) {
	query := `
		SELECT id, rule_name, category, severity, status, metric, metric_value, threshold, message, triggered_at, resolved_at
		FROM alerts_history
		WHERE status = ?
		ORDER BY triggered_at DESC
	`

	rows, err := s.db.QueryContext(ctx, query, string(model.AlertStatusActive))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanAlertRows(rows)
}

func scanAlertRows(rows *sql.Rows) ([]model.AlertEvent, error) {
	var alerts []model.AlertEvent
	for rows.Next() {
		var id, ruleName, category, severity, status, metric, message string
		var metricVal, threshold float64
		var triggeredAt int64
		var resolvedAt sql.NullInt64

		if err := rows.Scan(&id, &ruleName, &category, &severity, &status, &metric, &metricVal, &threshold, &message, &triggeredAt, &resolvedAt); err != nil {
			return nil, err
		}

		var resTime *time.Time
		if resolvedAt.Valid {
			t := time.UnixMilli(resolvedAt.Int64)
			resTime = &t
		}

		alerts = append(alerts, model.AlertEvent{
			ID:          id,
			RuleID:      category,
			RuleName:    ruleName,
			Severity:    model.Severity(severity),
			Message:     message,
			MetricName:  metric,
			ActualValue: metricVal,
			Threshold:   threshold,
			FiredAt:     time.UnixMilli(triggeredAt),
			ResolvedAt:  resTime,
			IsActive:    status == string(model.AlertStatusActive),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return alerts, nil
}
