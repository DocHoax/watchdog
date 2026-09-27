package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/DocHoax/watchdog/internal/incidents"
	"github.com/DocHoax/watchdog/pkg/model"
)

// SaveIncident inserts or updates a full incident record in SQLite storage.
func (s *SQLiteStorage) SaveIncident(ctx context.Context, inc *incidents.Incident) error {
	if inc == nil || inc.ID == "" {
		return fmt.Errorf("invalid incident: missing incident ID")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	symptomsJSON, _ := json.Marshal(inc.PrimarySymptoms)
	nodesJSON, _ := json.Marshal(inc.AffectedNodes)
	signalsJSON, _ := json.Marshal(inc.RootSignals)
	impactJSON, _ := json.Marshal(inc.Impact)
	explanationJSON, _ := json.Marshal(inc.SeverityExplanation)
	findingsJSON, _ := json.Marshal(inc.Findings)

	metaCopy := make(map[string]string)
	for k, v := range inc.Metadata {
		metaCopy[k] = v
	}
	for k, v := range inc.Tags {
		metaCopy["tag:"+k] = v
	}
	if inc.Summary != "" {
		metaCopy["summary"] = inc.Summary
	}
	metaJSON, _ := json.Marshal(metaCopy)

	var resolvedTs sql.NullInt64
	if inc.ResolvedAt != nil && !inc.ResolvedAt.IsZero() {
		resolvedTs = sql.NullInt64{Int64: inc.ResolvedAt.UTC().UnixMilli(), Valid: true}
	}

	startMs := inc.StartTime.UTC().UnixMilli()
	if inc.StartTime.IsZero() {
		startMs = time.Now().UTC().UnixMilli()
	}

	updMs := inc.UpdatedAt.UTC().UnixMilli()
	if inc.UpdatedAt.IsZero() {
		updMs = startMs
	}

	conf := inc.Confidence
	if conf == "" {
		conf = "medium"
	}

	query := `
		INSERT INTO incidents (
			id, title, status, severity, scope, confidence,
			start_time, updated_at, resolved_at,
			primary_symptoms_json, affected_nodes_json, root_signals_json,
			impact_json, explanation_json, findings_json, metadata_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			title = excluded.title,
			status = excluded.status,
			severity = excluded.severity,
			scope = excluded.scope,
			confidence = excluded.confidence,
			updated_at = excluded.updated_at,
			resolved_at = excluded.resolved_at,
			primary_symptoms_json = excluded.primary_symptoms_json,
			affected_nodes_json = excluded.affected_nodes_json,
			root_signals_json = excluded.root_signals_json,
			impact_json = excluded.impact_json,
			explanation_json = excluded.explanation_json,
			findings_json = excluded.findings_json,
			metadata_json = excluded.metadata_json
	`

	_, err := s.db.ExecContext(ctx, query,
		inc.ID,
		inc.Title,
		string(inc.Status),
		string(inc.Severity),
		string(inc.Scope),
		conf,
		startMs,
		updMs,
		resolvedTs,
		string(symptomsJSON),
		string(nodesJSON),
		string(signalsJSON),
		string(impactJSON),
		string(explanationJSON),
		string(findingsJSON),
		string(metaJSON),
	)
	if err != nil {
		return fmt.Errorf("failed to save incident %s: %w", inc.ID, err)
	}

	// Update incident_nodes mapping
	for _, nodeID := range inc.AffectedNodes {
		if nodeID == "" {
			continue
		}
		_, _ = s.db.ExecContext(ctx,
			"INSERT OR IGNORE INTO incident_nodes (incident_id, node_id) VALUES (?, ?)",
			inc.ID, nodeID,
		)
	}

	return nil
}

// GetIncident retrieves a single incident by ID.
func (s *SQLiteStorage) GetIncident(ctx context.Context, id string) (*incidents.Incident, error) {
	query := `
		SELECT id, title, status, severity, scope, confidence,
		       start_time, updated_at, resolved_at,
		       primary_symptoms_json, affected_nodes_json, root_signals_json,
		       impact_json, explanation_json, findings_json, metadata_json
		FROM incidents
		WHERE id = ?
	`

	var inc incidents.Incident
	var (
		statusStr, sevStr, scopeStr, confStr string
		startMs, updMs                       int64
		resolvedMs                           sql.NullInt64
		symptomsJSON, nodesJSON, signalsJSON sql.NullString
		impactJSON, explJSON, findJSON       sql.NullString
		metaJSON                             sql.NullString
	)

	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&inc.ID,
		&inc.Title,
		&statusStr,
		&sevStr,
		&scopeStr,
		&confStr,
		&startMs,
		&updMs,
		&resolvedMs,
		&symptomsJSON,
		&nodesJSON,
		&signalsJSON,
		&impactJSON,
		&explJSON,
		&findJSON,
		&metaJSON,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get incident %s: %w", id, err)
	}

	inc.Status = incidents.IncidentStatus(statusStr)
	inc.Severity = model.Severity(sevStr)
	inc.Scope = incidents.IncidentScope(scopeStr)
	inc.Confidence = confStr
	inc.StartTime = time.UnixMilli(startMs).UTC()
	inc.UpdatedAt = time.UnixMilli(updMs).UTC()
	inc.CreatedAt = inc.StartTime

	if resolvedMs.Valid {
		t := time.UnixMilli(resolvedMs.Int64).UTC()
		inc.ResolvedAt = &t
	}

	if symptomsJSON.Valid && symptomsJSON.String != "" {
		_ = json.Unmarshal([]byte(symptomsJSON.String), &inc.PrimarySymptoms)
	}
	if nodesJSON.Valid && nodesJSON.String != "" {
		_ = json.Unmarshal([]byte(nodesJSON.String), &inc.AffectedNodes)
	}
	if signalsJSON.Valid && signalsJSON.String != "" {
		_ = json.Unmarshal([]byte(signalsJSON.String), &inc.RootSignals)
	}
	if impactJSON.Valid && impactJSON.String != "" {
		_ = json.Unmarshal([]byte(impactJSON.String), &inc.Impact)
	}
	if explJSON.Valid && explJSON.String != "" {
		_ = json.Unmarshal([]byte(explJSON.String), &inc.SeverityExplanation)
		inc.SeverityScore = inc.SeverityExplanation.BaseScore
	}
	if findJSON.Valid && findJSON.String != "" {
		_ = json.Unmarshal([]byte(findJSON.String), &inc.Findings)
	}
	if metaJSON.Valid && metaJSON.String != "" {
		var meta map[string]string
		if err := json.Unmarshal([]byte(metaJSON.String), &meta); err == nil {
			inc.Metadata = make(map[string]string)
			inc.Tags = make(map[string]string)
			for k, v := range meta {
				if strings.HasPrefix(k, "tag:") {
					inc.Tags[strings.TrimPrefix(k, "tag:")] = v
				} else if k == "summary" {
					inc.Summary = v
				} else {
					inc.Metadata[k] = v
				}
			}
		}
	}

	// Fetch timeline entries
	tl, err := s.GetTimeline(ctx, id, incidents.TimelineFilter{})
	if err == nil {
		inc.Timeline = tl
	}

	return &inc, nil
}

// ListIncidents queries and returns a paginated list of incidents matching the filter.
func (s *SQLiteStorage) ListIncidents(ctx context.Context, filter incidents.IncidentFilter) ([]incidents.Incident, int, error) {
	var whereClauses []string
	var args []any

	if len(filter.Status) > 0 {
		placeholders := make([]string, len(filter.Status))
		for i, st := range filter.Status {
			placeholders[i] = "?"
			args = append(args, string(st))
		}
		whereClauses = append(whereClauses, fmt.Sprintf("status IN (%s)", strings.Join(placeholders, ",")))
	}

	if len(filter.Severity) > 0 {
		placeholders := make([]string, len(filter.Severity))
		for i, sv := range filter.Severity {
			placeholders[i] = "UPPER(?)"
			args = append(args, strings.ToUpper(string(sv)))
		}
		whereClauses = append(whereClauses, fmt.Sprintf("UPPER(severity) IN (%s)", strings.Join(placeholders, ",")))
	}

	if len(filter.Scope) > 0 {
		placeholders := make([]string, len(filter.Scope))
		for i, sc := range filter.Scope {
			placeholders[i] = "?"
			args = append(args, string(sc))
		}
		whereClauses = append(whereClauses, fmt.Sprintf("scope IN (%s)", strings.Join(placeholders, ",")))
	}

	if filter.NodeID != "" {
		whereClauses = append(whereClauses, "id IN (SELECT incident_id FROM incident_nodes WHERE node_id = ?)")
		args = append(args, filter.NodeID)
	}

	if filter.Search != "" {
		whereClauses = append(whereClauses, "(title LIKE ? OR id LIKE ?)")
		searchTerm := "%" + filter.Search + "%"
		args = append(args, searchTerm, searchTerm)
	}

	if !filter.StartTime.IsZero() {
		whereClauses = append(whereClauses, "start_time >= ?")
		args = append(args, filter.StartTime.UTC().UnixMilli())
	}

	if !filter.EndTime.IsZero() {
		whereClauses = append(whereClauses, "start_time <= ?")
		args = append(args, filter.EndTime.UTC().UnixMilli())
	}

	whereSQL := ""
	if len(whereClauses) > 0 {
		whereSQL = "WHERE " + strings.Join(whereClauses, " AND ")
	}

	// Count total matching
	countSQL := fmt.Sprintf("SELECT COUNT(*) FROM incidents %s", whereSQL)
	var totalCount int
	if err := s.db.QueryRowContext(ctx, countSQL, args...).Scan(&totalCount); err != nil {
		return nil, 0, fmt.Errorf("failed to count incidents: %w", err)
	}

	// Whitelist order by
	sortColumn := "start_time"
	switch strings.ToLower(filter.SortBy) {
	case "updated_at", "updated":
		sortColumn = "updated_at"
	case "severity":
		sortColumn = "severity"
	case "status":
		sortColumn = "status"
	case "title":
		sortColumn = "title"
	}

	sortOrder := "DESC"
	if strings.ToUpper(filter.SortOrder) == "ASC" {
		sortOrder = "ASC"
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	querySQL := fmt.Sprintf(`
		SELECT id, title, status, severity, scope, confidence,
		       start_time, updated_at, resolved_at,
		       primary_symptoms_json, affected_nodes_json, root_signals_json,
		       impact_json, explanation_json, findings_json, metadata_json
		FROM incidents
		%s
		ORDER BY %s %s
		LIMIT ? OFFSET ?
	`, whereSQL, sortColumn, sortOrder)

	queryArgs := append(args, limit, offset)
	rows, err := s.db.QueryContext(ctx, querySQL, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list incidents: %w", err)
	}
	defer rows.Close()

	var result []incidents.Incident
	for rows.Next() {
		var inc incidents.Incident
		var (
			statusStr, sevStr, scopeStr, confStr string
			startMs, updMs                       int64
			resolvedMs                           sql.NullInt64
			symptomsJSON, nodesJSON, signalsJSON sql.NullString
			impactJSON, explJSON, findJSON       sql.NullString
			metaJSON                             sql.NullString
		)

		if err := rows.Scan(
			&inc.ID,
			&inc.Title,
			&statusStr,
			&sevStr,
			&scopeStr,
			&confStr,
			&startMs,
			&updMs,
			&resolvedMs,
			&symptomsJSON,
			&nodesJSON,
			&signalsJSON,
			&impactJSON,
			&explJSON,
			&findJSON,
			&metaJSON,
		); err != nil {
			return nil, 0, fmt.Errorf("failed to scan incident row: %w", err)
		}

		inc.Status = incidents.IncidentStatus(statusStr)
		inc.Severity = model.Severity(sevStr)
		inc.Scope = incidents.IncidentScope(scopeStr)
		inc.Confidence = confStr
		inc.StartTime = time.UnixMilli(startMs).UTC()
		inc.UpdatedAt = time.UnixMilli(updMs).UTC()
		inc.CreatedAt = inc.StartTime

		if resolvedMs.Valid {
			t := time.UnixMilli(resolvedMs.Int64).UTC()
			inc.ResolvedAt = &t
		}

		if symptomsJSON.Valid && symptomsJSON.String != "" {
			_ = json.Unmarshal([]byte(symptomsJSON.String), &inc.PrimarySymptoms)
		}
		if nodesJSON.Valid && nodesJSON.String != "" {
			_ = json.Unmarshal([]byte(nodesJSON.String), &inc.AffectedNodes)
		}
		if signalsJSON.Valid && signalsJSON.String != "" {
			_ = json.Unmarshal([]byte(signalsJSON.String), &inc.RootSignals)
		}
		if impactJSON.Valid && impactJSON.String != "" {
			_ = json.Unmarshal([]byte(impactJSON.String), &inc.Impact)
		}
		if explJSON.Valid && explJSON.String != "" {
			_ = json.Unmarshal([]byte(explJSON.String), &inc.SeverityExplanation)
			inc.SeverityScore = inc.SeverityExplanation.BaseScore
		}
		if findJSON.Valid && findJSON.String != "" {
			_ = json.Unmarshal([]byte(findJSON.String), &inc.Findings)
		}
		if metaJSON.Valid && metaJSON.String != "" {
			var meta map[string]string
			if err := json.Unmarshal([]byte(metaJSON.String), &meta); err == nil {
				inc.Metadata = make(map[string]string)
				inc.Tags = make(map[string]string)
				for k, v := range meta {
					if strings.HasPrefix(k, "tag:") {
						inc.Tags[strings.TrimPrefix(k, "tag:")] = v
					} else if k == "summary" {
						inc.Summary = v
					} else {
						inc.Metadata[k] = v
					}
				}
			}
		}

		result = append(result, inc)
	}

	return result, totalCount, nil
}

// UpdateIncidentStatus updates status, resolution timestamp, and metadata.
func (s *SQLiteStorage) UpdateIncidentStatus(ctx context.Context, id string, status incidents.IncidentStatus, reason string, resolvedAt *time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	nowMs := time.Now().UTC().UnixMilli()
	var resolvedTs sql.NullInt64
	if resolvedAt != nil && !resolvedAt.IsZero() {
		resolvedTs = sql.NullInt64{Int64: resolvedAt.UTC().UnixMilli(), Valid: true}
	}

	query := `
		UPDATE incidents
		SET status = ?, updated_at = ?, resolved_at = ?
		WHERE id = ?
	`
	res, err := s.db.ExecContext(ctx, query, string(status), nowMs, resolvedTs, id)
	if err != nil {
		return fmt.Errorf("failed to update incident %s status: %w", id, err)
	}
	rowsAff, _ := res.RowsAffected()
	if rowsAff == 0 {
		return incidents.ErrIncidentNotFound
	}
	return nil
}

// SaveTimelineEntries inserts batch timeline entries into SQLite.
func (s *SQLiteStorage) SaveTimelineEntries(ctx context.Context, entries []incidents.IncidentTimelineEntry) error {
	if len(entries) == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO incident_timeline (
			id, incident_id, timestamp, event_type, source, node_id, severity, title, description, payload_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			title = excluded.title,
			description = excluded.description,
			payload_json = excluded.payload_json
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, e := range entries {
		var payloadJSON sql.NullString
		if len(e.Payload) > 0 {
			if data, err := json.Marshal(e.Payload); err == nil {
				payloadJSON = sql.NullString{String: string(data), Valid: true}
			}
		}

		tMs := e.Timestamp.UTC().UnixMilli()
		if e.Timestamp.IsZero() {
			tMs = time.Now().UTC().UnixMilli()
		}

		_, err := stmt.ExecContext(ctx,
			e.ID,
			e.IncidentID,
			tMs,
			string(e.EventType),
			e.Source,
			e.NodeID,
			string(e.Severity),
			e.Title,
			e.Description,
			payloadJSON,
		)
		if err != nil {
			return fmt.Errorf("failed to insert timeline entry %s: %w", e.ID, err)
		}
	}

	return tx.Commit()
}

// GetTimeline retrieves chronological timeline entries for an incident.
func (s *SQLiteStorage) GetTimeline(ctx context.Context, incidentID string, filter incidents.TimelineFilter) ([]incidents.IncidentTimelineEntry, error) {
	var whereClauses []string
	var args []any

	whereClauses = append(whereClauses, "incident_id = ?")
	args = append(args, incidentID)

	if filter.NodeID != "" {
		whereClauses = append(whereClauses, "node_id = ?")
		args = append(args, filter.NodeID)
	}

	if !filter.StartTime.IsZero() {
		whereClauses = append(whereClauses, "timestamp >= ?")
		args = append(args, filter.StartTime.UTC().UnixMilli())
	}

	if !filter.EndTime.IsZero() {
		whereClauses = append(whereClauses, "timestamp <= ?")
		args = append(args, filter.EndTime.UTC().UnixMilli())
	}

	whereSQL := strings.Join(whereClauses, " AND ")
	querySQL := fmt.Sprintf(`
		SELECT id, incident_id, timestamp, event_type, source, node_id, severity, title, description, payload_json
		FROM incident_timeline
		WHERE %s
		ORDER BY timestamp ASC
	`, whereSQL)

	if filter.Limit > 0 {
		querySQL += fmt.Sprintf(" LIMIT %d", filter.Limit)
	}

	rows, err := s.db.QueryContext(ctx, querySQL, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query timeline: %w", err)
	}
	defer rows.Close()

	var entries []incidents.IncidentTimelineEntry
	for rows.Next() {
		var e incidents.IncidentTimelineEntry
		var (
			tMs                       int64
			evType, sevStr, nodeStr   string
			descStr, payloadJSON      sql.NullString
		)

		if err := rows.Scan(
			&e.ID,
			&e.IncidentID,
			&tMs,
			&evType,
			&e.Source,
			&nodeStr,
			&sevStr,
			&e.Title,
			&descStr,
			&payloadJSON,
		); err != nil {
			return nil, fmt.Errorf("failed to scan timeline entry: %w", err)
		}

		e.Timestamp = time.UnixMilli(tMs).UTC()
		e.EventType = incidents.TimelineEventType(evType)
		e.NodeID = nodeStr
		e.Severity = model.Severity(sevStr)
		if descStr.Valid {
			e.Description = descStr.String
		}
		if payloadJSON.Valid && payloadJSON.String != "" {
			_ = json.Unmarshal([]byte(payloadJSON.String), &e.Payload)
		}

		entries = append(entries, e)
	}

	return entries, nil
}

// GetIncidentHistory retrieves historical incidents within the lookback window.
func (s *SQLiteStorage) GetIncidentHistory(ctx context.Context, lookback time.Duration) ([]incidents.Incident, error) {
	if lookback <= 0 {
		lookback = 30 * 24 * time.Hour
	}
	startTime := time.Now().UTC().Add(-lookback)

	filter := incidents.IncidentFilter{
		StartTime: startTime,
		Limit:     500,
	}

	res, _, err := s.ListIncidents(ctx, filter)
	return res, err
}
