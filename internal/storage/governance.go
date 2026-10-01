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

// SaveOrganization inserts or updates an organization record.
func (s *SQLiteStorage) SaveOrganization(ctx context.Context, org *model.Organization) error {
	if org == nil {
		return fmt.Errorf("nil organization")
	}
	if err := org.Validate(); err != nil {
		return fmt.Errorf("invalid organization: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	metaJSON, _ := json.Marshal(org.Metadata)

	now := time.Now().UTC()
	createdMs := org.CreatedAt.UTC().UnixMilli()
	if org.CreatedAt.IsZero() {
		createdMs = now.UnixMilli()
	}
	updatedMs := org.UpdatedAt.UTC().UnixMilli()
	if org.UpdatedAt.IsZero() {
		updatedMs = now.UnixMilli()
	}

	query := `
		INSERT INTO organizations (id, name, display_name, description, status, created_at, updated_at, metadata_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			display_name = excluded.display_name,
			description = excluded.description,
			status = excluded.status,
			updated_at = excluded.updated_at,
			metadata_json = excluded.metadata_json
	`

	_, err := s.db.ExecContext(ctx, query,
		org.ID,
		org.Name,
		org.DisplayName,
		org.Description,
		string(org.Status),
		createdMs,
		updatedMs,
		string(metaJSON),
	)
	return err
}

// GetOrganization retrieves an organization by its ID.
func (s *SQLiteStorage) GetOrganization(ctx context.Context, id string) (*model.Organization, error) {
	if id == "" {
		return nil, fmt.Errorf("empty organization id")
	}

	query := `
		SELECT id, name, display_name, description, status, created_at, updated_at, metadata_json
		FROM organizations
		WHERE id = ?
	`

	row := s.db.QueryRowContext(ctx, query, id)

	var org model.Organization
	var statusStr, metaStr string
	var createdMs, updatedMs int64

	err := row.Scan(
		&org.ID,
		&org.Name,
		&org.DisplayName,
		&org.Description,
		&statusStr,
		&createdMs,
		&updatedMs,
		&metaStr,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("organization %q not found", id)
		}
		return nil, fmt.Errorf("failed to get organization %q: %w", id, err)
	}

	org.Status = model.OrganizationStatus(statusStr)
	if createdMs > 0 {
		org.CreatedAt = time.UnixMilli(createdMs).UTC()
	}
	if updatedMs > 0 {
		org.UpdatedAt = time.UnixMilli(updatedMs).UTC()
	}
	if metaStr != "" {
		_ = json.Unmarshal([]byte(metaStr), &org.Metadata)
	}

	return &org, nil
}

// ListOrganizations lists all organizations ordered by name.
func (s *SQLiteStorage) ListOrganizations(ctx context.Context) ([]model.Organization, error) {
	query := `
		SELECT id, name, display_name, description, status, created_at, updated_at, metadata_json
		FROM organizations
		ORDER BY name ASC
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query organizations: %w", err)
	}
	defer rows.Close()

	var result []model.Organization
	for rows.Next() {
		var org model.Organization
		var statusStr, metaStr string
		var createdMs, updatedMs int64

		if err := rows.Scan(
			&org.ID,
			&org.Name,
			&org.DisplayName,
			&org.Description,
			&statusStr,
			&createdMs,
			&updatedMs,
			&metaStr,
		); err != nil {
			return nil, fmt.Errorf("failed to scan organization: %w", err)
		}

		org.Status = model.OrganizationStatus(statusStr)
		if createdMs > 0 {
			org.CreatedAt = time.UnixMilli(createdMs).UTC()
		}
		if updatedMs > 0 {
			org.UpdatedAt = time.UnixMilli(updatedMs).UTC()
		}
		if metaStr != "" {
			_ = json.Unmarshal([]byte(metaStr), &org.Metadata)
		}
		result = append(result, org)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

// DeleteOrganization deletes an organization by ID, preventing deletion of the default organization.
func (s *SQLiteStorage) DeleteOrganization(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("empty organization id")
	}
	if id == model.DefaultOrganizationID {
		return fmt.Errorf("cannot delete default organization")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	query := `DELETE FROM organizations WHERE id = ?`
	res, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete organization %q: %w", id, err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("organization %q not found", id)
	}
	return nil
}

// SaveFleetGroup inserts or updates a fleet group record.
func (s *SQLiteStorage) SaveFleetGroup(ctx context.Context, group *model.FleetGroup) error {
	if group == nil {
		return fmt.Errorf("nil fleet group")
	}
	if err := group.Validate(); err != nil {
		return fmt.Errorf("invalid fleet group: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	metaJSON, _ := json.Marshal(group.Metadata)

	now := time.Now().UTC()
	createdMs := group.CreatedAt.UTC().UnixMilli()
	if group.CreatedAt.IsZero() {
		createdMs = now.UnixMilli()
	}
	updatedMs := group.UpdatedAt.UTC().UnixMilli()
	if group.UpdatedAt.IsZero() {
		updatedMs = now.UnixMilli()
	}

	var parentID sql.NullString
	if group.ParentGroupID != "" {
		parentID = sql.NullString{String: group.ParentGroupID, Valid: true}
	}

	query := `
		INSERT INTO fleet_groups (
			id, org_id, parent_group_id, name, display_name, description, group_type, path, created_at, updated_at, metadata_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			org_id = excluded.org_id,
			parent_group_id = excluded.parent_group_id,
			name = excluded.name,
			display_name = excluded.display_name,
			description = excluded.description,
			group_type = excluded.group_type,
			path = excluded.path,
			updated_at = excluded.updated_at,
			metadata_json = excluded.metadata_json
	`

	_, err := s.db.ExecContext(ctx, query,
		group.ID,
		group.OrgID,
		parentID,
		group.Name,
		group.DisplayName,
		group.Description,
		string(group.Type),
		group.Path,
		createdMs,
		updatedMs,
		string(metaJSON),
	)
	return err
}

// GetFleetGroup retrieves a fleet group by ID.
func (s *SQLiteStorage) GetFleetGroup(ctx context.Context, id string) (*model.FleetGroup, error) {
	if id == "" {
		return nil, fmt.Errorf("empty fleet group id")
	}

	query := `
		SELECT id, org_id, parent_group_id, name, display_name, description, group_type, path, created_at, updated_at, metadata_json
		FROM fleet_groups
		WHERE id = ?
	`

	row := s.db.QueryRowContext(ctx, query, id)

	var group model.FleetGroup
	var parentID sql.NullString
	var typeStr, metaStr string
	var createdMs, updatedMs int64

	err := row.Scan(
		&group.ID,
		&group.OrgID,
		&parentID,
		&group.Name,
		&group.DisplayName,
		&group.Description,
		&typeStr,
		&group.Path,
		&createdMs,
		&updatedMs,
		&metaStr,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("fleet group %q not found", id)
		}
		return nil, fmt.Errorf("failed to get fleet group %q: %w", id, err)
	}

	if parentID.Valid {
		group.ParentGroupID = parentID.String
	}
	group.Type = model.GroupType(typeStr)
	if createdMs > 0 {
		group.CreatedAt = time.UnixMilli(createdMs).UTC()
	}
	if updatedMs > 0 {
		group.UpdatedAt = time.UnixMilli(updatedMs).UTC()
	}
	if metaStr != "" {
		_ = json.Unmarshal([]byte(metaStr), &group.Metadata)
	}

	return &group, nil
}

// ListFleetGroups lists fleet groups, optionally filtered by orgID.
func (s *SQLiteStorage) ListFleetGroups(ctx context.Context, orgID string) ([]model.FleetGroup, error) {
	var query string
	var args []any

	if orgID != "" {
		query = `
			SELECT id, org_id, parent_group_id, name, display_name, description, group_type, path, created_at, updated_at, metadata_json
			FROM fleet_groups
			WHERE org_id = ?
			ORDER BY name ASC
		`
		args = append(args, orgID)
	} else {
		query = `
			SELECT id, org_id, parent_group_id, name, display_name, description, group_type, path, created_at, updated_at, metadata_json
			FROM fleet_groups
			ORDER BY name ASC
		`
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query fleet groups: %w", err)
	}
	defer rows.Close()

	var result []model.FleetGroup
	for rows.Next() {
		var group model.FleetGroup
		var parentID sql.NullString
		var typeStr, metaStr string
		var createdMs, updatedMs int64

		if err := rows.Scan(
			&group.ID,
			&group.OrgID,
			&parentID,
			&group.Name,
			&group.DisplayName,
			&group.Description,
			&typeStr,
			&group.Path,
			&createdMs,
			&updatedMs,
			&metaStr,
		); err != nil {
			return nil, fmt.Errorf("failed to scan fleet group: %w", err)
		}

		if parentID.Valid {
			group.ParentGroupID = parentID.String
		}
		group.Type = model.GroupType(typeStr)
		if createdMs > 0 {
			group.CreatedAt = time.UnixMilli(createdMs).UTC()
		}
		if updatedMs > 0 {
			group.UpdatedAt = time.UnixMilli(updatedMs).UTC()
		}
		if metaStr != "" {
			_ = json.Unmarshal([]byte(metaStr), &group.Metadata)
		}
		result = append(result, group)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

// DeleteFleetGroup removes a fleet group and associated memberships.
func (s *SQLiteStorage) DeleteFleetGroup(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("empty fleet group id")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Clean up members
	if _, err := s.db.ExecContext(ctx, `DELETE FROM fleet_group_members WHERE group_id = ?`, id); err != nil {
		return fmt.Errorf("failed to clean up members for group %q: %w", id, err)
	}

	// Update any children whose parent was this group
	if _, err := s.db.ExecContext(ctx, `UPDATE fleet_groups SET parent_group_id = NULL WHERE parent_group_id = ?`, id); err != nil {
		return fmt.Errorf("failed to detach child groups of %q: %w", id, err)
	}

	res, err := s.db.ExecContext(ctx, `DELETE FROM fleet_groups WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to delete fleet group %q: %w", id, err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("fleet group %q not found", id)
	}
	return nil
}

// AddGroupMember adds a node to a fleet group.
func (s *SQLiteStorage) AddGroupMember(ctx context.Context, member *model.FleetGroupMember) error {
	if member == nil {
		return fmt.Errorf("nil group member")
	}
	if err := member.Validate(); err != nil {
		return fmt.Errorf("invalid group member: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	metaJSON, _ := json.Marshal(member.Metadata)

	addedMs := member.AddedAt.UTC().UnixMilli()
	if member.AddedAt.IsZero() {
		addedMs = time.Now().UTC().UnixMilli()
	}

	query := `
		INSERT INTO fleet_group_members (group_id, node_id, added_at, added_by, role, metadata_json)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(group_id, node_id) DO UPDATE SET
			role = excluded.role,
			added_by = excluded.added_by,
			metadata_json = excluded.metadata_json
	`

	_, err := s.db.ExecContext(ctx, query,
		member.GroupID,
		member.NodeID,
		addedMs,
		member.AddedBy,
		string(member.Role),
		string(metaJSON),
	)
	return err
}

// RemoveGroupMember removes a node from a fleet group.
func (s *SQLiteStorage) RemoveGroupMember(ctx context.Context, groupID string, nodeID string) error {
	if groupID == "" || nodeID == "" {
		return fmt.Errorf("empty group_id or node_id")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	query := `DELETE FROM fleet_group_members WHERE group_id = ? AND node_id = ?`
	_, err := s.db.ExecContext(ctx, query, groupID, nodeID)
	return err
}

// SetGroupMembers replaces all memberships for a group in a transaction.
func (s *SQLiteStorage) SetGroupMembers(ctx context.Context, groupID string, nodeIDs []string) error {
	if groupID == "" {
		return fmt.Errorf("empty group_id")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Delete existing members
	if _, err := tx.ExecContext(ctx, `DELETE FROM fleet_group_members WHERE group_id = ?`, groupID); err != nil {
		return fmt.Errorf("failed to clear existing group members: %w", err)
	}

	nowMs := time.Now().UTC().UnixMilli()
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO fleet_group_members (group_id, node_id, added_at, added_by, role, metadata_json)
		VALUES (?, ?, ?, ?, ?, '{}')
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare insert statement: %w", err)
	}
	defer stmt.Close()

	for _, nodeID := range nodeIDs {
		cleanNodeID := strings.TrimSpace(nodeID)
		if cleanNodeID == "" {
			continue
		}
		if _, err := stmt.ExecContext(ctx, groupID, cleanNodeID, nowMs, "api", string(model.MembershipRolePrimary)); err != nil {
			return fmt.Errorf("failed to insert group member %q: %w", cleanNodeID, err)
		}
	}

	return tx.Commit()
}

// GetGroupMembers retrieves all member nodes of a fleet group.
func (s *SQLiteStorage) GetGroupMembers(ctx context.Context, groupID string) ([]model.FleetGroupMember, error) {
	if groupID == "" {
		return nil, fmt.Errorf("empty group_id")
	}

	query := `
		SELECT group_id, node_id, added_at, added_by, role, metadata_json
		FROM fleet_group_members
		WHERE group_id = ?
		ORDER BY node_id ASC
	`

	rows, err := s.db.QueryContext(ctx, query, groupID)
	if err != nil {
		return nil, fmt.Errorf("failed to query group members: %w", err)
	}
	defer rows.Close()

	var result []model.FleetGroupMember
	for rows.Next() {
		var m model.FleetGroupMember
		var roleStr, metaStr string
		var addedMs int64

		if err := rows.Scan(
			&m.GroupID,
			&m.NodeID,
			&addedMs,
			&m.AddedBy,
			&roleStr,
			&metaStr,
		); err != nil {
			return nil, fmt.Errorf("failed to scan group member: %w", err)
		}

		m.Role = model.MembershipRole(roleStr)
		if addedMs > 0 {
			m.AddedAt = time.UnixMilli(addedMs).UTC()
		}
		if metaStr != "" {
			_ = json.Unmarshal([]byte(metaStr), &m.Metadata)
		}
		result = append(result, m)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

// GetNodeGroups retrieves all fleet groups that a node belongs to.
func (s *SQLiteStorage) GetNodeGroups(ctx context.Context, nodeID string) ([]model.FleetGroup, error) {
	if nodeID == "" {
		return nil, fmt.Errorf("empty node_id")
	}

	query := `
		SELECT g.id, g.org_id, g.parent_group_id, g.name, g.display_name, g.description, g.group_type, g.path, g.created_at, g.updated_at, g.metadata_json
		FROM fleet_groups g
		INNER JOIN fleet_group_members m ON g.id = m.group_id
		WHERE m.node_id = ?
		ORDER BY g.name ASC
	`

	rows, err := s.db.QueryContext(ctx, query, nodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to query node groups: %w", err)
	}
	defer rows.Close()

	var result []model.FleetGroup
	for rows.Next() {
		var group model.FleetGroup
		var parentID sql.NullString
		var typeStr, metaStr string
		var createdMs, updatedMs int64

		if err := rows.Scan(
			&group.ID,
			&group.OrgID,
			&parentID,
			&group.Name,
			&group.DisplayName,
			&group.Description,
			&typeStr,
			&group.Path,
			&createdMs,
			&updatedMs,
			&metaStr,
		); err != nil {
			return nil, fmt.Errorf("failed to scan node group: %w", err)
		}

		if parentID.Valid {
			group.ParentGroupID = parentID.String
		}
		group.Type = model.GroupType(typeStr)
		if createdMs > 0 {
			group.CreatedAt = time.UnixMilli(createdMs).UTC()
		}
		if updatedMs > 0 {
			group.UpdatedAt = time.UnixMilli(updatedMs).UTC()
		}
		if metaStr != "" {
			_ = json.Unmarshal([]byte(metaStr), &group.Metadata)
		}
		result = append(result, group)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}
