package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"official-account-service/internal/domain/authorization"
)

// SaveAuthorizationState stores one authorization state digest.
func (s *Store) SaveAuthorizationState(ctx context.Context, state authorization.AuthorizationState) (authorization.AuthorizationState, error) {
	row := s.db.QueryRowContext(ctx, `
		INSERT INTO wechat_authorization_state (state_digest, tenant_id, component_app_id, expires_at)
		VALUES ($1, $2, $3, $4)
		RETURNING state_digest, tenant_id, component_app_id, expires_at, consumed_at, created_at`,
		state.Digest, state.TenantID, state.ComponentAppID, state.ExpiresAt)
	saved, err := scanAuthorizationStateRow(row)
	if err != nil {
		if isUniqueViolation(err) {
			return authorization.AuthorizationState{}, fmt.Errorf("save authorization state duplicate: %w", authorization.ErrAuthorizationStateExists)
		}
		return authorization.AuthorizationState{}, fmt.Errorf("save authorization state: %w", err)
	}
	return saved, nil
}

// ConsumeAuthorizationState atomically consumes one valid authorization state digest.
func (s *Store) ConsumeAuthorizationState(ctx context.Context, digest string, consumedAt time.Time) (authorization.AuthorizationState, error) {
	row := s.db.QueryRowContext(ctx, `
		UPDATE wechat_authorization_state
		SET consumed_at = $2
		WHERE state_digest = $1 AND consumed_at IS NULL AND expires_at > $2
		RETURNING state_digest, tenant_id, component_app_id, expires_at, consumed_at, created_at`, digest, consumedAt)
	state, err := scanAuthorizationStateRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return authorization.AuthorizationState{}, fmt.Errorf("consume authorization state lookup: %w", authorization.ErrAuthorizationStateNotFound)
		}
		return authorization.AuthorizationState{}, fmt.Errorf("consume authorization state: %w", err)
	}
	return state, nil
}

func scanAuthorizationStateRow(row *sql.Row) (authorization.AuthorizationState, error) {
	var state authorization.AuthorizationState
	var consumedAt sql.NullTime
	if err := row.Scan(&state.Digest, &state.TenantID, &state.ComponentAppID, &state.ExpiresAt, &consumedAt, &state.CreatedAt); err != nil {
		return authorization.AuthorizationState{}, err
	}
	state.ConsumedAt = nullTimeValue(consumedAt)
	return state, nil
}
