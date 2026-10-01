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

// SavePolicy inserts or updates a policy record.
func (s *SQLiteStorage) SavePolicy(ctx context.Context, policy *model.Policy) error {
	if policy == nil {
		return fmt.Errorf("nil policy")
	}
	if err := policy.Validate(); err != nil {
		return fmt.Errorf("invalid policy: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	metaJSON, _ := json.Marshal(policy.Metadata)

	now := time.Now().UTC()
	createdMs := policy.CreatedAt.UTC().UnixMilli()
	if policy.CreatedAt.IsZero() {
		createdMs = now.UnixMilli()
	}
	updatedMs := policy.UpdatedAt.UTC().UnixMilli()
	if policy.UpdatedAt.IsZero() {
		updatedMs = now.UnixMilli()
	}

	query := `
		INSERT INTO policies (id, org_id, name, display_name, description, category, status, active_revision, created_at, updated_at, metadata_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			org_id = excluded.org_id,
			name = excluded.name,
			display_name = excluded.display_name,
			description = excluded.description,
			category = excluded.category,
			status = excluded.status,
			active_revision = excluded.active_revision,
			updated_at = excluded.updated_at,
			metadata_json = excluded.metadata_json
	`

	_, err := s.db.ExecContext(ctx, query,
		policy.ID,
		policy.OrgID,
		policy.Name,
		policy.DisplayName,
		policy.Description,
		string(policy.Category),
		string(policy.Status),
		policy.ActiveRevision,
		createdMs,
		updatedMs,
		string(metaJSON),
	)
	return err
}

// GetPolicy retrieves a single policy by ID.
func (s *SQLiteStorage) GetPolicy(ctx context.Context, id string) (*model.Policy, error) {
	if id == "" {
		return nil, fmt.Errorf("empty policy id")
	}

	query := `
		SELECT id, org_id, name, display_name, description, category, status, active_revision, created_at, updated_at, metadata_json
		FROM policies
		WHERE id = ?
	`

	row := s.db.QueryRowContext(ctx, query, id)

	var p model.Policy
	var catStr, statusStr, metaStr string
	var createdMs, updatedMs int64

	err := row.Scan(
		&p.ID,
		&p.OrgID,
		&p.Name,
		&p.DisplayName,
		&p.Description,
		&catStr,
		&statusStr,
		&p.ActiveRevision,
		&createdMs,
		&updatedMs,
		&metaStr,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("policy %q not found", id)
		}
		return nil, fmt.Errorf("failed to get policy %q: %w", id, err)
	}

	p.Category = model.PolicyCategory(catStr)
	p.Status = model.PolicyStatus(statusStr)
	if createdMs > 0 {
		p.CreatedAt = time.UnixMilli(createdMs).UTC()
	}
	if updatedMs > 0 {
		p.UpdatedAt = time.UnixMilli(updatedMs).UTC()
	}
	if metaStr != "" {
		_ = json.Unmarshal([]byte(metaStr), &p.Metadata)
	}

	return &p, nil
}

// ListPolicies lists policies matching the filter.
func (s *SQLiteStorage) ListPolicies(ctx context.Context, filter model.PolicyFilter) ([]model.Policy, error) {
	var conditions []string
	var args []interface{}

	if filter.OrgID != "" {
		conditions = append(conditions, "org_id = ?")
		args = append(args, filter.OrgID)
	}
	if filter.Category != "" {
		conditions = append(conditions, "category = ?")
		args = append(args, string(filter.Category))
	}
	if filter.Status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, string(filter.Status))
	}
	if filter.Search != "" {
		conditions = append(conditions, "(name LIKE ? OR description LIKE ?)")
		pattern := "%" + filter.Search + "%"
		args = append(args, pattern, pattern)
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	limitClause := ""
	if filter.Limit > 0 {
		limitClause = fmt.Sprintf("LIMIT %d", filter.Limit)
		if filter.Offset > 0 {
			limitClause += fmt.Sprintf(" OFFSET %d", filter.Offset)
		}
	}

	query := fmt.Sprintf(`
		SELECT id, org_id, name, display_name, description, category, status, active_revision, created_at, updated_at, metadata_json
		FROM policies
		%s
		ORDER BY created_at ASC
		%s
	`, whereClause, limitClause)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list policies: %w", err)
	}
	defer rows.Close()

	var policies []model.Policy
	for rows.Next() {
		var p model.Policy
		var catStr, statusStr, metaStr string
		var createdMs, updatedMs int64

		if err := rows.Scan(
			&p.ID,
			&p.OrgID,
			&p.Name,
			&p.DisplayName,
			&p.Description,
			&catStr,
			&statusStr,
			&p.ActiveRevision,
			&createdMs,
			&updatedMs,
			&metaStr,
		); err != nil {
			return nil, fmt.Errorf("failed to scan policy row: %w", err)
		}

		p.Category = model.PolicyCategory(catStr)
		p.Status = model.PolicyStatus(statusStr)
		if createdMs > 0 {
			p.CreatedAt = time.UnixMilli(createdMs).UTC()
		}
		if updatedMs > 0 {
			p.UpdatedAt = time.UnixMilli(updatedMs).UTC()
		}
		if metaStr != "" {
			_ = json.Unmarshal([]byte(metaStr), &p.Metadata)
		}

		policies = append(policies, p)
	}

	return policies, rows.Err()
}

// DeletePolicy removes a policy and cascades to its revisions and assignments.
func (s *SQLiteStorage) DeletePolicy(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("empty policy id")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, "DELETE FROM policy_assignments WHERE policy_id = ?", id); err != nil {
		return fmt.Errorf("failed to delete policy assignments for %q: %w", id, err)
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM policy_revisions WHERE policy_id = ?", id); err != nil {
		return fmt.Errorf("failed to delete policy revisions for %q: %w", id, err)
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM policies WHERE id = ?", id); err != nil {
		return fmt.Errorf("failed to delete policy %q: %w", id, err)
	}

	return tx.Commit()
}

// SavePolicyRevision inserts an immutable revision of a policy.
func (s *SQLiteStorage) SavePolicyRevision(ctx context.Context, rev *model.PolicyRevision) error {
	if rev == nil {
		return fmt.Errorf("nil policy revision")
	}
	if err := rev.Validate(); err != nil {
		return fmt.Errorf("invalid policy revision: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	rulesJSON, err := json.Marshal(rev.Rules)
	if err != nil {
		return fmt.Errorf("failed to marshal rules: %w", err)
	}

	metaJSON, _ := json.Marshal(rev.Metadata)

	now := time.Now().UTC()
	createdMs := rev.CreatedAt.UTC().UnixMilli()
	if rev.CreatedAt.IsZero() {
		createdMs = now.UnixMilli()
	}

	query := `
		INSERT INTO policy_revisions (
			policy_id, revision, created_at, created_by, change_summary,
			target_selector, priority, inheritance_mode, enforcement_mode,
			rules_json, metadata_json, content_digest
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(policy_id, revision) DO UPDATE SET
			change_summary = excluded.change_summary,
			target_selector = excluded.target_selector,
			priority = excluded.priority,
			inheritance_mode = excluded.inheritance_mode,
			enforcement_mode = excluded.enforcement_mode,
			rules_json = excluded.rules_json,
			metadata_json = excluded.metadata_json,
			content_digest = excluded.content_digest
	`

	_, err = s.db.ExecContext(ctx, query,
		rev.PolicyID,
		rev.Revision,
		createdMs,
		rev.CreatedBy,
		rev.ChangeSummary,
		rev.Selector,
		rev.Priority,
		string(rev.InheritanceMode),
		string(rev.EnforcementMode),
		string(rulesJSON),
		string(metaJSON),
		rev.ContentDigest,
	)
	return err
}

// GetPolicyRevision retrieves a specific revision for a given policy.
func (s *SQLiteStorage) GetPolicyRevision(ctx context.Context, policyID string, revision int) (*model.PolicyRevision, error) {
	if policyID == "" {
		return nil, fmt.Errorf("empty policy id")
	}
	if revision < 1 {
		return nil, fmt.Errorf("invalid revision number: %d", revision)
	}

	query := `
		SELECT policy_id, revision, created_at, created_by, change_summary,
		       target_selector, priority, inheritance_mode, enforcement_mode,
		       rules_json, metadata_json, content_digest
		FROM policy_revisions
		WHERE policy_id = ? AND revision = ?
	`

	row := s.db.QueryRowContext(ctx, query, policyID, revision)

	var rev model.PolicyRevision
	var inhStr, enfStr, rulesStr, metaStr string
	var createdMs int64

	err := row.Scan(
		&rev.PolicyID,
		&rev.Revision,
		&createdMs,
		&rev.CreatedBy,
		&rev.ChangeSummary,
		&rev.Selector,
		&rev.Priority,
		&inhStr,
		&enfStr,
		&rulesStr,
		&metaStr,
		&rev.ContentDigest,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("policy revision %s@v%d not found", policyID, revision)
		}
		return nil, fmt.Errorf("failed to get policy revision %s@v%d: %w", policyID, revision, err)
	}

	rev.InheritanceMode = model.InheritanceMode(inhStr)
	rev.EnforcementMode = model.EnforcementMode(enfStr)
	if createdMs > 0 {
		rev.CreatedAt = time.UnixMilli(createdMs).UTC()
	}
	if rulesStr != "" {
		_ = json.Unmarshal([]byte(rulesStr), &rev.Rules)
	}
	if metaStr != "" {
		_ = json.Unmarshal([]byte(metaStr), &rev.Metadata)
	}

	return &rev, nil
}

// ListPolicyRevisions lists all revisions for a given policy in ascending revision order.
func (s *SQLiteStorage) ListPolicyRevisions(ctx context.Context, policyID string) ([]model.PolicyRevision, error) {
	if policyID == "" {
		return nil, fmt.Errorf("empty policy id")
	}

	query := `
		SELECT policy_id, revision, created_at, created_by, change_summary,
		       target_selector, priority, inheritance_mode, enforcement_mode,
		       rules_json, metadata_json, content_digest
		FROM policy_revisions
		WHERE policy_id = ?
		ORDER BY revision ASC
	`

	rows, err := s.db.QueryContext(ctx, query, policyID)
	if err != nil {
		return nil, fmt.Errorf("failed to list policy revisions for %q: %w", policyID, err)
	}
	defer rows.Close()

	var revisions []model.PolicyRevision
	for rows.Next() {
		var rev model.PolicyRevision
		var inhStr, enfStr, rulesStr, metaStr string
		var createdMs int64

		if err := rows.Scan(
			&rev.PolicyID,
			&rev.Revision,
			&createdMs,
			&rev.CreatedBy,
			&rev.ChangeSummary,
			&rev.Selector,
			&rev.Priority,
			&inhStr,
			&enfStr,
			&rulesStr,
			&metaStr,
			&rev.ContentDigest,
		); err != nil {
			return nil, fmt.Errorf("failed to scan policy revision row: %w", err)
		}

		rev.InheritanceMode = model.InheritanceMode(inhStr)
		rev.EnforcementMode = model.EnforcementMode(enfStr)
		if createdMs > 0 {
			rev.CreatedAt = time.UnixMilli(createdMs).UTC()
		}
		if rulesStr != "" {
			_ = json.Unmarshal([]byte(rulesStr), &rev.Rules)
		}
		if metaStr != "" {
			_ = json.Unmarshal([]byte(metaStr), &rev.Metadata)
		}

		revisions = append(revisions, rev)
	}

	return revisions, rows.Err()
}

// SavePolicyAssignment creates or updates a policy assignment mapping.
func (s *SQLiteStorage) SavePolicyAssignment(ctx context.Context, asgn *model.PolicyAssignment) error {
	if asgn == nil {
		return fmt.Errorf("nil policy assignment")
	}
	if err := asgn.Validate(); err != nil {
		return fmt.Errorf("invalid policy assignment: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	metaJSON, _ := json.Marshal(asgn.Metadata)

	now := time.Now().UTC()
	assignedMs := asgn.AssignedAt.UTC().UnixMilli()
	if asgn.AssignedAt.IsZero() {
		assignedMs = now.UnixMilli()
	}

	enabledInt := 0
	if asgn.Enabled {
		enabledInt = 1
	}

	query := `
		INSERT INTO policy_assignments (
			id, policy_id, org_id, target_type, target_id,
			assigned_at, assigned_by, enabled, metadata_json
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			policy_id = excluded.policy_id,
			org_id = excluded.org_id,
			target_type = excluded.target_type,
			target_id = excluded.target_id,
			assigned_at = excluded.assigned_at,
			assigned_by = excluded.assigned_by,
			enabled = excluded.enabled,
			metadata_json = excluded.metadata_json
	`

	_, err := s.db.ExecContext(ctx, query,
		asgn.ID,
		asgn.PolicyID,
		asgn.OrgID,
		string(asgn.TargetType),
		asgn.TargetID,
		assignedMs,
		asgn.AssignedBy,
		enabledInt,
		string(metaJSON),
	)
	return err
}

// GetPolicyAssignment retrieves an assignment by its ID.
func (s *SQLiteStorage) GetPolicyAssignment(ctx context.Context, id string) (*model.PolicyAssignment, error) {
	if id == "" {
		return nil, fmt.Errorf("empty assignment id")
	}

	query := `
		SELECT id, policy_id, org_id, target_type, target_id, assigned_at, assigned_by, enabled, metadata_json
		FROM policy_assignments
		WHERE id = ?
	`

	row := s.db.QueryRowContext(ctx, query, id)

	var asgn model.PolicyAssignment
	var targetTypeStr, metaStr string
	var assignedMs int64
	var enabledInt int

	err := row.Scan(
		&asgn.ID,
		&asgn.PolicyID,
		&asgn.OrgID,
		&targetTypeStr,
		&asgn.TargetID,
		&assignedMs,
		&asgn.AssignedBy,
		&enabledInt,
		&metaStr,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("policy assignment %q not found", id)
		}
		return nil, fmt.Errorf("failed to get policy assignment %q: %w", id, err)
	}

	asgn.TargetType = model.PolicyTargetType(targetTypeStr)
	asgn.Enabled = enabledInt == 1
	if assignedMs > 0 {
		asgn.AssignedAt = time.UnixMilli(assignedMs).UTC()
	}
	if metaStr != "" {
		_ = json.Unmarshal([]byte(metaStr), &asgn.Metadata)
	}

	return &asgn, nil
}

// ListPolicyAssignments retrieves policy assignments matching the provided filter.
func (s *SQLiteStorage) ListPolicyAssignments(ctx context.Context, filter model.PolicyAssignmentFilter) ([]model.PolicyAssignment, error) {
	var conditions []string
	var args []interface{}

	if filter.OrgID != "" {
		conditions = append(conditions, "org_id = ?")
		args = append(args, filter.OrgID)
	}
	if filter.PolicyID != "" {
		conditions = append(conditions, "policy_id = ?")
		args = append(args, filter.PolicyID)
	}
	if filter.TargetType != "" {
		conditions = append(conditions, "target_type = ?")
		args = append(args, string(filter.TargetType))
	}
	if filter.TargetID != "" {
		conditions = append(conditions, "target_id = ?")
		args = append(args, filter.TargetID)
	}
	if filter.EnabledOnly {
		conditions = append(conditions, "enabled = 1")
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	query := fmt.Sprintf(`
		SELECT id, policy_id, org_id, target_type, target_id, assigned_at, assigned_by, enabled, metadata_json
		FROM policy_assignments
		%s
		ORDER BY assigned_at ASC
	`, whereClause)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list policy assignments: %w", err)
	}
	defer rows.Close()

	var assignments []model.PolicyAssignment
	for rows.Next() {
		var asgn model.PolicyAssignment
		var targetTypeStr, metaStr string
		var assignedMs int64
		var enabledInt int

		if err := rows.Scan(
			&asgn.ID,
			&asgn.PolicyID,
			&asgn.OrgID,
			&targetTypeStr,
			&asgn.TargetID,
			&assignedMs,
			&asgn.AssignedBy,
			&enabledInt,
			&metaStr,
		); err != nil {
			return nil, fmt.Errorf("failed to scan policy assignment row: %w", err)
		}

		asgn.TargetType = model.PolicyTargetType(targetTypeStr)
		asgn.Enabled = enabledInt == 1
		if assignedMs > 0 {
			asgn.AssignedAt = time.UnixMilli(assignedMs).UTC()
		}
		if metaStr != "" {
			_ = json.Unmarshal([]byte(metaStr), &asgn.Metadata)
		}

		assignments = append(assignments, asgn)
	}

	return assignments, rows.Err()
}

// GetAssignmentsForTargets retrieves all enabled assignments for a set of target IDs within an organization.
func (s *SQLiteStorage) GetAssignmentsForTargets(ctx context.Context, orgID string, targetType model.PolicyTargetType, targetIDs []string) ([]model.PolicyAssignment, error) {
	if len(targetIDs) == 0 {
		return nil, nil
	}

	placeholders := make([]string, len(targetIDs))
	args := make([]interface{}, 0, len(targetIDs)+2)

	if orgID != "" {
		args = append(args, orgID)
	}
	args = append(args, string(targetType))

	for i, id := range targetIDs {
		placeholders[i] = "?"
		args = append(args, id)
	}

	orgCondition := ""
	if orgID != "" {
		orgCondition = "org_id = ? AND "
	}

	query := fmt.Sprintf(`
		SELECT id, policy_id, org_id, target_type, target_id, assigned_at, assigned_by, enabled, metadata_json
		FROM policy_assignments
		WHERE %starget_type = ? AND target_id IN (%s) AND enabled = 1
		ORDER BY assigned_at ASC
	`, orgCondition, strings.Join(placeholders, ", "))

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get assignments for targets: %w", err)
	}
	defer rows.Close()

	var assignments []model.PolicyAssignment
	for rows.Next() {
		var asgn model.PolicyAssignment
		var targetTypeStr, metaStr string
		var assignedMs int64
		var enabledInt int

		if err := rows.Scan(
			&asgn.ID,
			&asgn.PolicyID,
			&asgn.OrgID,
			&targetTypeStr,
			&asgn.TargetID,
			&assignedMs,
			&asgn.AssignedBy,
			&enabledInt,
			&metaStr,
		); err != nil {
			return nil, fmt.Errorf("failed to scan policy assignment row: %w", err)
		}

		asgn.TargetType = model.PolicyTargetType(targetTypeStr)
		asgn.Enabled = enabledInt == 1
		if assignedMs > 0 {
			asgn.AssignedAt = time.UnixMilli(assignedMs).UTC()
		}
		if metaStr != "" {
			_ = json.Unmarshal([]byte(metaStr), &asgn.Metadata)
		}

		assignments = append(assignments, asgn)
	}

	return assignments, rows.Err()
}

// DeletePolicyAssignment deletes an assignment by ID.
func (s *SQLiteStorage) DeletePolicyAssignment(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("empty assignment id")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	res, err := s.db.ExecContext(ctx, "DELETE FROM policy_assignments WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete policy assignment %q: %w", id, err)
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("policy assignment %q not found", id)
	}
	return nil
}
