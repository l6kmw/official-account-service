package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"official-account-service/internal/domain/authorization"
)

// SaveAuthorizerTenantBinding stores callback routing metadata for an authorizer.
func (s *Store) SaveAuthorizerTenantBinding(ctx context.Context, binding authorization.AuthorizerTenantBinding) (authorization.AuthorizerTenantBinding, error) {
	row := s.db.QueryRowContext(ctx, `
		INSERT INTO wechat_authorizer_tenant_binding (component_app_id, authorizer_app_id, tenant_id, updated_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (component_app_id, authorizer_app_id) DO UPDATE SET
			tenant_id = EXCLUDED.tenant_id,
			updated_at = NOW()
		RETURNING component_app_id, authorizer_app_id, tenant_id, updated_at`,
		binding.ComponentAppID, binding.AuthorizerAppID, binding.TenantID)
	saved, err := scanAuthorizerTenantBindingRow(row)
	if err != nil {
		return authorization.AuthorizerTenantBinding{}, fmt.Errorf("save authorizer tenant binding: %w", err)
	}
	return saved, nil
}

// GetAuthorizerTenantBinding returns callback routing metadata for an authorizer.
func (s *Store) GetAuthorizerTenantBinding(ctx context.Context, componentAppID string, authorizerAppID string) (authorization.AuthorizerTenantBinding, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT component_app_id, authorizer_app_id, tenant_id, updated_at
		FROM wechat_authorizer_tenant_binding
		WHERE component_app_id = $1 AND authorizer_app_id = $2`, componentAppID, authorizerAppID)
	binding, err := scanAuthorizerTenantBindingRow(row)
	if err != nil {
		return authorization.AuthorizerTenantBinding{}, mapAuthorizerTenantBindingError("get authorizer tenant binding", err)
	}
	return binding, nil
}

func scanAuthorizerTenantBindingRow(row *sql.Row) (authorization.AuthorizerTenantBinding, error) {
	var binding authorization.AuthorizerTenantBinding
	if err := row.Scan(&binding.ComponentAppID, &binding.AuthorizerAppID, &binding.TenantID, &binding.UpdatedAt); err != nil {
		return authorization.AuthorizerTenantBinding{}, fmt.Errorf("scan authorizer tenant binding: %w", err)
	}
	return binding, nil
}

func mapAuthorizerTenantBindingError(action string, err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%s: %w", action, authorization.ErrAuthorizerTenantBindingNotFound)
	}
	return fmt.Errorf("%s: %w", action, err)
}
