package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/DocHoax/watchdog/pkg/model"
)

// SaveAuditEvent persists a single audit event into SQLite.
func (s *SQLiteStorage) SaveAuditEvent(ctx context.Context, event model.AuditEvent) error {
	return s.SaveAuditEvents(ctx, []model.AuditEvent{event})
}

// SaveAuditEvents persists a slice of audit events into SQLite within a single transaction.
func (s *SQLiteStorage) SaveAuditEvents(ctx context.Context, events []model.AuditEvent) error {
	if len(events) == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin audit transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO audit_events (
			id, timestamp, event_type, severity, outcome,
			actor_type, actor_identity, source_address, transport,
			protocol, user_agent, endpoint, method, request_id,
			resource, action, message, metadata_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare audit insert statement: %w", err)
	}
	defer stmt.Close()

	for _, event := range events {
		var metaJSON sql.NullString
		if len(event.Metadata) > 0 {
			if data, err := json.Marshal(event.Metadata); err == nil {
				metaJSON = sql.NullString{String: string(data), Valid: true}
			}
		}

		ts := event.Timestamp.UTC().UnixMilli()
		if event.Timestamp.IsZero() {
			ts = time.Now().UTC().UnixMilli()
		}

		_, err = stmt.ExecContext(ctx,
			event.ID,
			ts,
			event.EventType,
			event.Severity,
			event.Outcome,
			event.Actor.Type,
			event.Actor.Identity,
			event.Source.Address,
			event.Source.Transport,
			event.Source.Protocol,
			event.Source.UserAgent,
			event.Source.Endpoint,
			event.Source.Method,
			event.Source.RequestID,
			event.Resource,
			event.Action,
			event.Message,
			metaJSON,
		)
		if err != nil {
			return fmt.Errorf("failed to insert audit event %s: %w", event.ID, err)
		}
	}

	return tx.Commit()
}

// QueryAuditEvents retrieves audit events matching the specified filter criteria.
func (s *SQLiteStorage) QueryAuditEvents(ctx context.Context, filter AuditFilter) ([]model.AuditEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query, args := buildAuditQuery(filter, false)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query audit events: %w", err)
	}
	defer rows.Close()

	var events []model.AuditEvent
	for rows.Next() {
		var (
			id            string
			tsMillis      int64
			eventType     string
			severity      string
			outcome       string
			actorType     sql.NullString
			actorIdentity sql.NullString
			sourceAddress sql.NullString
			transport     sql.NullString
			protocol      sql.NullString
			userAgent     sql.NullString
			endpoint      sql.NullString
			method        sql.NullString
			requestID     sql.NullString
			resource      sql.NullString
			action        sql.NullString
			message       string
			metaJSON      sql.NullString
		)

		err := rows.Scan(
			&id,
			&tsMillis,
			&eventType,
			&severity,
			&outcome,
			&actorType,
			&actorIdentity,
			&sourceAddress,
			&transport,
			&protocol,
			&userAgent,
			&endpoint,
			&method,
			&requestID,
			&resource,
			&action,
			&message,
			&metaJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan audit event row: %w", err)
		}

		event := model.AuditEvent{
			ID:        id,
			Timestamp: time.UnixMilli(tsMillis).UTC(),
			EventType: eventType,
			Severity:  severity,
			Outcome:   outcome,
			Actor: model.AuditActor{
				Type:     actorType.String,
				Identity: actorIdentity.String,
			},
			Source: model.AuditSource{
				Address:   sourceAddress.String,
				Transport: transport.String,
				Protocol:  protocol.String,
				UserAgent: userAgent.String,
				Endpoint:  endpoint.String,
				Method:    method.String,
				RequestID: requestID.String,
			},
			Resource: resource.String,
			Action:   action.String,
			Message:  message,
		}

		if metaJSON.Valid && metaJSON.String != "" {
			var meta map[string]string
			if err := json.Unmarshal([]byte(metaJSON.String), &meta); err == nil {
				event.Metadata = meta
			}
		}

		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating audit rows: %w", err)
	}

	return events, nil
}

// CountAuditEvents returns the total number of audit events matching the filter criteria.
func (s *SQLiteStorage) CountAuditEvents(ctx context.Context, filter AuditFilter) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query, args := buildAuditQuery(filter, true)
	var count int64
	err := s.db.QueryRowContext(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count audit events: %w", err)
	}
	return count, nil
}

// PruneAuditEvents deletes audit events older than the specified retention duration.
func (s *SQLiteStorage) PruneAuditEvents(ctx context.Context, retention time.Duration) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().UTC().Add(-retention).UnixMilli()
	res, err := s.db.ExecContext(ctx, `DELETE FROM audit_events WHERE timestamp < ?`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("failed to prune audit events: %w", err)
	}
	return res.RowsAffected()
}

// PurgeAuditEvents permanently deletes audit events before a specified cutoff timestamp.
func (s *SQLiteStorage) PurgeAuditEvents(ctx context.Context, before time.Time) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := before.UTC().UnixMilli()
	res, err := s.db.ExecContext(ctx, `DELETE FROM audit_events WHERE timestamp < ?`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("failed to purge audit events: %w", err)
	}
	return res.RowsAffected()
}

func buildAuditQuery(filter AuditFilter, isCount bool) (string, []any) {
	var query string
	if isCount {
		query = "SELECT COUNT(*) FROM audit_events WHERE 1=1"
	} else {
		query = `SELECT id, timestamp, event_type, severity, outcome, actor_type, actor_identity,
		         source_address, transport, protocol, user_agent, endpoint, method, request_id,
		         resource, action, message, metadata_json
		         FROM audit_events WHERE 1=1`
	}

	var args []any

	if !filter.StartTime.IsZero() {
		query += " AND timestamp >= ?"
		args = append(args, filter.StartTime.UTC().UnixMilli())
	}
	if !filter.EndTime.IsZero() {
		query += " AND timestamp <= ?"
		args = append(args, filter.EndTime.UTC().UnixMilli())
	}
	if filter.EventType != "" {
		query += " AND event_type = ?"
		args = append(args, filter.EventType)
	}
	if filter.Severity != "" {
		query += " AND severity = ?"
		args = append(args, filter.Severity)
	}
	if filter.Outcome != "" {
		query += " AND outcome = ?"
		args = append(args, filter.Outcome)
	}
	if filter.ActorType != "" {
		query += " AND actor_type = ?"
		args = append(args, filter.ActorType)
	}
	if filter.ActorIdentity != "" {
		query += " AND actor_identity = ?"
		args = append(args, filter.ActorIdentity)
	}
	if filter.SourceAddress != "" {
		query += " AND source_address = ?"
		args = append(args, filter.SourceAddress)
	}
	if filter.RequestID != "" {
		query += " AND request_id = ?"
		args = append(args, filter.RequestID)
	}

	if !isCount {
		query += " ORDER BY timestamp DESC"
		limit := filter.Limit
		if limit <= 0 {
			limit = 100
		} else if limit > 10000 {
			limit = 10000
		}
		query += " LIMIT ?"
		args = append(args, limit)

		if filter.Offset > 0 {
			query += " OFFSET ?"
			args = append(args, filter.Offset)
		}
	}

	return query, args
}
