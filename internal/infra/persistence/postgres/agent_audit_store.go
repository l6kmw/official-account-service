package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"official-account-service/internal/domain/agentaudit"
)

var _ agentaudit.Repository = (*Store)(nil)

// Append stores one immutable audit entry without request payloads or credentials.
func (s *Store) Append(ctx context.Context, entry agentaudit.Entry) (agentaudit.Entry, error) {
	row := s.db.QueryRowContext(ctx, `
		INSERT INTO app_agent_audit_log (user_id, agent_record_id, action, resource_type, resource_id)
		SELECT $1, NULLIF($2, ''), $3, $4, $5
		WHERE NULLIF($2, '') IS NULL
		   OR EXISTS (
			SELECT 1 FROM app_agent WHERE id = $2 AND user_id = $1
		   )
		RETURNING id, user_id, COALESCE(agent_record_id, ''), action, resource_type, resource_id, created_at`,
		entry.UserID, entry.AgentRecordID, entry.Action, entry.ResourceType, entry.ResourceID)
	created, err := scanAgentAudit(row)
	if err != nil {
		return agentaudit.Entry{}, fmt.Errorf("append agent audit: %w", err)
	}
	return created, nil
}

// ListAuditEntries returns newest-first safe audit metadata scoped to one user.
func (s *Store) ListAuditEntries(ctx context.Context, userID string, filter agentaudit.Filter) ([]agentaudit.Entry, error) {
	query := `
		SELECT id, user_id, COALESCE(agent_record_id, ''), action, resource_type, resource_id, created_at
		FROM app_agent_audit_log
		WHERE user_id = $1`
	args := []any{userID}
	if filter.AgentRecordID != "" {
		args = append(args, filter.AgentRecordID)
		query += fmt.Sprintf(" AND agent_record_id = $%d", len(args))
	}
	if filter.Action != "" {
		args = append(args, filter.Action)
		query += fmt.Sprintf(" AND action = $%d", len(args))
	}
	if filter.ResourceType != "" {
		args = append(args, filter.ResourceType)
		query += fmt.Sprintf(" AND resource_type = $%d", len(args))
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	args = append(args, limit)
	query += fmt.Sprintf(" ORDER BY created_at DESC, id DESC LIMIT $%d", len(args))

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list agent audit query: %w", err)
	}
	defer rows.Close()
	entries := make([]agentaudit.Entry, 0)
	for rows.Next() {
		entry, err := scanAgentAudit(rows)
		if err != nil {
			return nil, fmt.Errorf("list agent audit scan: %w", err)
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list agent audit rows: %w", err)
	}
	return entries, nil
}

type agentAuditScanner interface {
	Scan(dest ...any) error
}

func scanAgentAudit(scanner agentAuditScanner) (agentaudit.Entry, error) {
	var entry agentaudit.Entry
	if err := scanner.Scan(
		&entry.ID, &entry.UserID, &entry.AgentRecordID, &entry.Action,
		&entry.ResourceType, &entry.ResourceID, &entry.CreatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return agentaudit.Entry{}, fmt.Errorf("audit actor does not belong to user: %w", err)
		}
		return agentaudit.Entry{}, err
	}
	return entry, nil
}
