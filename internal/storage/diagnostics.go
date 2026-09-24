package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/watchdog-cli/watchdog/pkg/model"
)

// SaveDiagnosticReport writes a diagnostic report to the database.
func (s *SQLiteStorage) SaveDiagnosticReport(ctx context.Context, report *model.DiagnosticReport) error {
	if report == nil {
		return nil
	}

	reportID := uuid.New().String()
	t := report.GeneratedAt
	if t.IsZero() {
		t = time.Now()
	}

	checksJSON, err := json.Marshal(report.Results)
	if err != nil {
		return fmt.Errorf("failed to marshal diagnostic results: %w", err)
	}

	summaryData := map[string]any{
		"total":    report.TotalChecks,
		"passed":   report.PassedChecks,
		"warnings": report.WarningChecks,
		"critical": report.CriticalChecks,
	}
	summaryJSON, _ := json.Marshal(summaryData)

	query := `
		INSERT INTO diagnostics_history (id, overall_status, checks_json, summary_json, timestamp)
		VALUES (?, ?, ?, ?, ?)
	`

	_, err = s.db.ExecContext(ctx, query,
		reportID,
		string(report.OverallStatus),
		string(checksJSON),
		string(summaryJSON),
		t.UnixMilli(),
	)
	return err
}

// GetLatestDiagnosticReport retrieves the most recent diagnostic run.
func (s *SQLiteStorage) GetLatestDiagnosticReport(ctx context.Context) (*model.DiagnosticReport, error) {
	reports, err := s.GetDiagnosticHistory(ctx, 1)
	if err != nil {
		return nil, err
	}
	if len(reports) == 0 {
		return nil, nil
	}
	return &reports[0], nil
}

// GetDiagnosticHistory fetches historical diagnostic runs.
func (s *SQLiteStorage) GetDiagnosticHistory(ctx context.Context, limit int) ([]model.DiagnosticReport, error) {
	if limit <= 0 {
		limit = 20
	}

	query := `
		SELECT id, overall_status, checks_json, summary_json, timestamp
		FROM diagnostics_history
		ORDER BY timestamp DESC
		LIMIT ?
	`

	rows, err := s.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reports []model.DiagnosticReport
	for rows.Next() {
		var id, overallStatus, checksJSON, summaryJSON string
		var ts int64

		if err := rows.Scan(&id, &overallStatus, &checksJSON, &summaryJSON, &ts); err != nil {
			return nil, err
		}

		var results []model.DiagnosticResult
		_ = json.Unmarshal([]byte(checksJSON), &results)

		var summaryData struct {
			Total    int `json:"total"`
			Passed   int `json:"passed"`
			Warnings int `json:"warnings"`
			Critical int `json:"critical"`
		}
		_ = json.Unmarshal([]byte(summaryJSON), &summaryData)

		reports = append(reports, model.DiagnosticReport{
			OverallStatus:  model.DiagnosticStatus(overallStatus),
			TotalChecks:    summaryData.Total,
			PassedChecks:   summaryData.Passed,
			WarningChecks:  summaryData.Warnings,
			CriticalChecks: summaryData.Critical,
			Results:        results,
			GeneratedAt:    time.UnixMilli(ts),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return reports, nil
}
