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

// SaveFleetNode inserts or updates a fleet node record in SQLite.
func (s *SQLiteStorage) SaveFleetNode(ctx context.Context, node *model.FleetNode) error {
	if node == nil || node.Identity.NodeID == "" {
		return fmt.Errorf("invalid fleet node: missing node_id")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	ipJSON, _ := json.Marshal(node.Identity.IPAddresses)
	macJSON, _ := json.Marshal(node.Identity.MACAddresses)
	tagsJSON, _ := json.Marshal(node.Identity.Tags)
	metaJSON, _ := json.Marshal(node.Metadata)

	var summaryJSON sql.NullString
	if node.Summary != nil {
		if data, err := json.Marshal(node.Summary); err == nil {
			summaryJSON = sql.NullString{String: string(data), Valid: true}
		}
	}

	var lastTelemMs sql.NullInt64
	if node.LastTelemetry != nil && !node.LastTelemetry.IsZero() {
		lastTelemMs = sql.NullInt64{Int64: node.LastTelemetry.UTC().UnixMilli(), Valid: true}
	}

	regMs := node.RegisteredAt.UTC().UnixMilli()
	if node.RegisteredAt.IsZero() {
		regMs = time.Now().UTC().UnixMilli()
	}

	hbMs := node.LastHeartbeat.UTC().UnixMilli()
	if node.LastHeartbeat.IsZero() {
		hbMs = regMs
	}

	query := `
		INSERT INTO fleet_nodes (
			node_id, hostname, os, platform, platform_version, arch,
			kernel_version, version, cpu_cores, total_memory,
			ip_addresses_json, mac_addresses_json, tags_json,
			status, status_message, registered_at, last_heartbeat,
			last_telemetry, summary_json, metadata_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(node_id) DO UPDATE SET
			hostname = excluded.hostname,
			os = excluded.os,
			platform = excluded.platform,
			platform_version = excluded.platform_version,
			arch = excluded.arch,
			kernel_version = excluded.kernel_version,
			version = excluded.version,
			cpu_cores = excluded.cpu_cores,
			total_memory = excluded.total_memory,
			ip_addresses_json = excluded.ip_addresses_json,
			mac_addresses_json = excluded.mac_addresses_json,
			tags_json = excluded.tags_json,
			status = excluded.status,
			status_message = excluded.status_message,
			last_heartbeat = excluded.last_heartbeat,
			last_telemetry = COALESCE(excluded.last_telemetry, fleet_nodes.last_telemetry),
			summary_json = COALESCE(excluded.summary_json, fleet_nodes.summary_json),
			metadata_json = excluded.metadata_json
	`

	_, err := s.db.ExecContext(ctx, query,
		node.Identity.NodeID,
		node.Identity.Hostname,
		node.Identity.OS,
		node.Identity.Platform,
		node.Identity.PlatformVer,
		node.Identity.Arch,
		node.Identity.KernelVer,
		node.Identity.Version,
		node.Identity.CPUCores,
		int64(node.Identity.TotalMemory),
		string(ipJSON),
		string(macJSON),
		string(tagsJSON),
		string(node.Status),
		node.StatusMessage,
		regMs,
		hbMs,
		lastTelemMs,
		summaryJSON,
		string(metaJSON),
	)
	return err
}

// GetFleetNode retrieves a fleet node by its node_id.
func (s *SQLiteStorage) GetFleetNode(ctx context.Context, nodeID string) (*model.FleetNode, error) {
	if nodeID == "" {
		return nil, fmt.Errorf("empty node_id")
	}

	query := `
		SELECT node_id, hostname, os, platform, platform_version, arch,
		       kernel_version, version, cpu_cores, total_memory,
		       ip_addresses_json, mac_addresses_json, tags_json,
		       status, status_message, registered_at, last_heartbeat,
		       last_telemetry, summary_json, metadata_json
		FROM fleet_nodes
		WHERE node_id = ?
	`

	row := s.db.QueryRowContext(ctx, query, nodeID)
	return scanFleetNode(row)
}

// ListFleetNodes returns a filtered list of nodes and total matching count.
func (s *SQLiteStorage) ListFleetNodes(ctx context.Context, filter model.FleetFilter) ([]model.FleetNode, int, error) {
	var whereClauses []string
	var args []any

	if filter.Status != "" {
		whereClauses = append(whereClauses, "status = ?")
		args = append(args, string(filter.Status))
	}

	if filter.Search != "" {
		searchPattern := "%" + strings.ToLower(filter.Search) + "%"
		whereClauses = append(whereClauses, "(LOWER(node_id) LIKE ? OR LOWER(hostname) LIKE ? OR LOWER(ip_addresses_json) LIKE ?)")
		args = append(args, searchPattern, searchPattern, searchPattern)
	}

	if !filter.Since.IsZero() {
		whereClauses = append(whereClauses, "last_heartbeat >= ?")
		args = append(args, filter.Since.UTC().UnixMilli())
	}

	whereSQL := ""
	if len(whereClauses) > 0 {
		whereSQL = " WHERE " + strings.Join(whereClauses, " AND ")
	}

	// 1. Get total count
	countQuery := "SELECT COUNT(*) FROM fleet_nodes" + whereSQL
	var totalCount int
	if err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&totalCount); err != nil {
		return nil, 0, fmt.Errorf("failed to count fleet nodes: %w", err)
	}

	// 2. Determine sort order
	sortCol := "last_heartbeat"
	switch strings.ToLower(filter.SortBy) {
	case "hostname":
		sortCol = "hostname"
	case "status":
		sortCol = "status"
	case "registered_at":
		sortCol = "registered_at"
	case "last_heartbeat":
		sortCol = "last_heartbeat"
	}

	direction := "DESC"
	if strings.EqualFold(filter.SortDirection, "asc") {
		direction = "ASC"
	}

	limit := filter.Limit
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	query := fmt.Sprintf(`
		SELECT node_id, hostname, os, platform, platform_version, arch,
		       kernel_version, version, cpu_cores, total_memory,
		       ip_addresses_json, mac_addresses_json, tags_json,
		       status, status_message, registered_at, last_heartbeat,
		       last_telemetry, summary_json, metadata_json
		FROM fleet_nodes
		%s
		ORDER BY %s %s
		LIMIT ? OFFSET ?
	`, whereSQL, sortCol, direction)

	queryArgs := append(args, limit, offset)
	rows, err := s.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query fleet nodes: %w", err)
	}
	defer rows.Close()

	var nodes []model.FleetNode
	for rows.Next() {
		node, err := scanFleetNode(rows)
		if err != nil {
			return nil, 0, err
		}

		// Filter by tags in-memory if tags filter specified
		if len(filter.Tags) > 0 {
			match := true
			for k, v := range filter.Tags {
				if node.Identity.Tags == nil || node.Identity.Tags[k] != v {
					match = false
					break
				}
			}
			if !match {
				continue
			}
		}

		nodes = append(nodes, *node)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return nodes, totalCount, nil
}

// DeleteFleetNode removes a fleet node and its associated telemetry.
func (s *SQLiteStorage) DeleteFleetNode(ctx context.Context, nodeID string) error {
	if nodeID == "" {
		return fmt.Errorf("empty node_id")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	_, _ = s.db.ExecContext(ctx, "DELETE FROM fleet_telemetry WHERE node_id = ?", nodeID)
	_, err := s.db.ExecContext(ctx, "DELETE FROM fleet_nodes WHERE node_id = ?", nodeID)
	return err
}

// SaveTelemetrySubmission persists a batch of telemetry and updates the node record.
func (s *SQLiteStorage) SaveTelemetrySubmission(ctx context.Context, sub *model.TelemetrySubmission) error {
	if sub == nil || sub.NodeID == "" {
		return fmt.Errorf("invalid telemetry submission: missing node_id")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	tsMs := sub.Timestamp.UTC().UnixMilli()
	if sub.Timestamp.IsZero() {
		tsMs = time.Now().UTC().UnixMilli()
	}

	var snapJSON, diagJSON, alertsJSON, metricsJSON, tagsJSON sql.NullString
	if sub.Snapshot != nil {
		if data, err := json.Marshal(sub.Snapshot); err == nil {
			snapJSON = sql.NullString{String: string(data), Valid: true}
		}
	}
	if sub.Diagnostics != nil {
		if data, err := json.Marshal(sub.Diagnostics); err == nil {
			diagJSON = sql.NullString{String: string(data), Valid: true}
		}
	}
	if len(sub.ActiveAlerts) > 0 {
		if data, err := json.Marshal(sub.ActiveAlerts); err == nil {
			alertsJSON = sql.NullString{String: string(data), Valid: true}
		}
	}
	if len(sub.Metrics) > 0 {
		if data, err := json.Marshal(sub.Metrics); err == nil {
			metricsJSON = sql.NullString{String: string(data), Valid: true}
		}
	}
	if len(sub.Tags) > 0 {
		if data, err := json.Marshal(sub.Tags); err == nil {
			tagsJSON = sql.NullString{String: string(data), Valid: true}
		}
	}

	// 1. Insert into fleet_telemetry
	insertQuery := `
		INSERT INTO fleet_telemetry (
			node_id, timestamp, sequence, snapshot_json,
			diagnostics_json, alerts_json, metrics_json, tags_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := s.db.ExecContext(ctx, insertQuery,
		sub.NodeID,
		tsMs,
		sub.Sequence,
		snapJSON,
		diagJSON,
		alertsJSON,
		metricsJSON,
		tagsJSON,
	)
	if err != nil {
		return fmt.Errorf("failed to insert fleet telemetry: %w", err)
	}

	// 2. Update node summary and last_telemetry in fleet_nodes
	var summary *model.NodeSummary
	if sub.Snapshot != nil {
		var load1 float64
		if sub.Snapshot.Load != nil {
			load1 = sub.Snapshot.Load.Load1
		}
		var diagStatus string
		if sub.Diagnostics != nil {
			diagStatus = string(sub.Diagnostics.OverallStatus)
		}
		var memPct float64
		if sub.Snapshot.Memory != nil {
			memPct = sub.Snapshot.Memory.UsedPercent
		}
		var diskPct float64
		if len(sub.Snapshot.Disks) > 0 {
			diskPct = sub.Snapshot.Disks[0].UsedPercent
		}
		var cpuPct float64
		if sub.Snapshot.CPU != nil {
			cpuPct = sub.Snapshot.CPU.TotalUsage
		}

		tNow := time.UnixMilli(tsMs).UTC()
		summary = &model.NodeSummary{
			NodeID:             sub.NodeID,
			Hostname:           sub.Snapshot.Host.Hostname,
			Status:             model.NodeStatusHealthy,
			CPUUsagePercent:    cpuPct,
			MemoryUsagePercent: memPct,
			DiskUsagePercent:   diskPct,
			Load1:              load1,
			ActiveAlertsCount:  len(sub.ActiveAlerts),
			DiagnosticStatus:   diagStatus,
			LastHeartbeat:      tNow,
			LastTelemetry:      &tNow,
			Version:            sub.Snapshot.Host.PlatformFamily,
			Tags:               sub.Tags,
		}
	}

	if summary != nil {
		sumData, _ := json.Marshal(summary)
		updateQuery := `
			UPDATE fleet_nodes
			SET last_telemetry = ?, summary_json = ?
			WHERE node_id = ?
		`
		_, _ = s.db.ExecContext(ctx, updateQuery, tsMs, string(sumData), sub.NodeID)
	} else {
		updateQuery := `
			UPDATE fleet_nodes
			SET last_telemetry = ?
			WHERE node_id = ?
		`
		_, _ = s.db.ExecContext(ctx, updateQuery, tsMs, sub.NodeID)
	}

	return nil
}

// GetNodeTelemetrySubmissions retrieves recent telemetry submissions for a node.
func (s *SQLiteStorage) GetNodeTelemetrySubmissions(ctx context.Context, nodeID string, since time.Time, limit int) ([]model.TelemetrySubmission, error) {
	if nodeID == "" {
		return nil, fmt.Errorf("empty node_id")
	}

	if limit <= 0 || limit > 500 {
		limit = 50
	}

	var sinceMs int64
	if !since.IsZero() {
		sinceMs = since.UTC().UnixMilli()
	}

	query := `
		SELECT node_id, timestamp, sequence, snapshot_json,
		       diagnostics_json, alerts_json, metrics_json, tags_json
		FROM fleet_telemetry
		WHERE node_id = ? AND timestamp >= ?
		ORDER BY timestamp DESC
		LIMIT ?
	`

	rows, err := s.db.QueryContext(ctx, query, nodeID, sinceMs, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query fleet telemetry: %w", err)
	}
	defer rows.Close()

	var subs []model.TelemetrySubmission
	for rows.Next() {
		var sub model.TelemetrySubmission
		var tsMs int64
		var snapJSON, diagJSON, alertsJSON, metricsJSON, tagsJSON sql.NullString

		if err := rows.Scan(
			&sub.NodeID,
			&tsMs,
			&sub.Sequence,
			&snapJSON,
			&diagJSON,
			&alertsJSON,
			&metricsJSON,
			&tagsJSON,
		); err != nil {
			return nil, fmt.Errorf("failed to scan telemetry submission: %w", err)
		}

		sub.Timestamp = time.UnixMilli(tsMs).UTC()

		if snapJSON.Valid && snapJSON.String != "" {
			var snap model.SystemSnapshot
			if err := json.Unmarshal([]byte(snapJSON.String), &snap); err == nil {
				sub.Snapshot = &snap
			}
		}
		if diagJSON.Valid && diagJSON.String != "" {
			var diag model.DiagnosticReport
			if err := json.Unmarshal([]byte(diagJSON.String), &diag); err == nil {
				sub.Diagnostics = &diag
			}
		}
		if alertsJSON.Valid && alertsJSON.String != "" {
			var alerts []model.AlertEvent
			if err := json.Unmarshal([]byte(alertsJSON.String), &alerts); err == nil {
				sub.ActiveAlerts = alerts
			}
		}
		if metricsJSON.Valid && metricsJSON.String != "" {
			var metrics map[string]float64
			if err := json.Unmarshal([]byte(metricsJSON.String), &metrics); err == nil {
				sub.Metrics = metrics
			}
		}
		if tagsJSON.Valid && tagsJSON.String != "" {
			var tags map[string]string
			if err := json.Unmarshal([]byte(tagsJSON.String), &tags); err == nil {
				sub.Tags = tags
			}
		}

		subs = append(subs, sub)
	}

	return subs, rows.Err()
}

// PruneFleetTelemetry deletes telemetry records older than retention duration.
func (s *SQLiteStorage) PruneFleetTelemetry(ctx context.Context, retention time.Duration) (int64, error) {
	if retention <= 0 {
		return 0, nil
	}

	cutoff := time.Now().Add(-retention).UTC().UnixMilli()

	s.mu.Lock()
	defer s.mu.Unlock()

	result, err := s.db.ExecContext(ctx, "DELETE FROM fleet_telemetry WHERE timestamp < ?", cutoff)
	if err != nil {
		return 0, fmt.Errorf("failed to prune fleet telemetry: %w", err)
	}
	return result.RowsAffected()
}

type scannable interface {
	Scan(dest ...any) error
}

func scanFleetNode(s scannable) (*model.FleetNode, error) {
	var node model.FleetNode
	var regMs, hbMs int64
	var lastTelemMs sql.NullInt64
	var ipJSON, macJSON, tagsJSON, summaryJSON, metaJSON sql.NullString
	var statusStr, platVer, kernVer, statMsg string

	err := s.Scan(
		&node.Identity.NodeID,
		&node.Identity.Hostname,
		&node.Identity.OS,
		&node.Identity.Platform,
		&platVer,
		&node.Identity.Arch,
		&kernVer,
		&node.Identity.Version,
		&node.Identity.CPUCores,
		&node.Identity.TotalMemory,
		&ipJSON,
		&macJSON,
		&tagsJSON,
		&statusStr,
		&statMsg,
		&regMs,
		&hbMs,
		&lastTelemMs,
		&summaryJSON,
		&metaJSON,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to scan fleet node: %w", err)
	}

	node.Identity.PlatformVer = platVer
	node.Identity.KernelVer = kernVer
	node.Status = model.NodeStatus(statusStr)
	node.StatusMessage = statMsg
	node.RegisteredAt = time.UnixMilli(regMs).UTC()
	node.Identity.CreatedAt = node.RegisteredAt
	node.LastHeartbeat = time.UnixMilli(hbMs).UTC()

	if lastTelemMs.Valid && lastTelemMs.Int64 > 0 {
		t := time.UnixMilli(lastTelemMs.Int64).UTC()
		node.LastTelemetry = &t
	}

	if ipJSON.Valid && ipJSON.String != "" {
		_ = json.Unmarshal([]byte(ipJSON.String), &node.Identity.IPAddresses)
	}
	if macJSON.Valid && macJSON.String != "" {
		_ = json.Unmarshal([]byte(macJSON.String), &node.Identity.MACAddresses)
	}
	if tagsJSON.Valid && tagsJSON.String != "" {
		_ = json.Unmarshal([]byte(tagsJSON.String), &node.Identity.Tags)
	}
	if metaJSON.Valid && metaJSON.String != "" {
		_ = json.Unmarshal([]byte(metaJSON.String), &node.Metadata)
	}
	if summaryJSON.Valid && summaryJSON.String != "" {
		var summary model.NodeSummary
		if err := json.Unmarshal([]byte(summaryJSON.String), &summary); err == nil {
			node.Summary = &summary
		}
	}

	return &node, nil
}
