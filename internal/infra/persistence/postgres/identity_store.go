package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"official-account-service/internal/domain/identity"
)

var _ identity.Repository = (*Store)(nil)

// SaveUser creates or updates one platform user by stable data-space id.
func (s *Store) SaveUser(ctx context.Context, user identity.User) (identity.User, error) {
	row := s.db.QueryRowContext(ctx, `
		INSERT INTO app_user (id, username, password_hash, role, status)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (id) DO UPDATE SET
			username = EXCLUDED.username,
			password_hash = EXCLUDED.password_hash,
			role = EXCLUDED.role,
			status = EXCLUDED.status,
			updated_at = NOW()
		RETURNING id, username, password_hash, role, status, created_at, updated_at`,
		user.ID, user.Username, user.PasswordHash, user.Role, user.Status)
	saved, err := scanUser(row)
	if err != nil {
		return identity.User{}, mapUserError("save user", err)
	}
	return saved, nil
}

// GetUser returns one platform user by id.
func (s *Store) GetUser(ctx context.Context, id string) (identity.User, error) {
	user, err := scanUser(s.db.QueryRowContext(ctx, userSelectSQL+` WHERE id = $1`, id))
	if err != nil {
		return identity.User{}, mapUserError("get user", err)
	}
	return user, nil
}

// GetUserByUsername returns one platform user by case-insensitive username.
func (s *Store) GetUserByUsername(ctx context.Context, username string) (identity.User, error) {
	user, err := scanUser(s.db.QueryRowContext(ctx, userSelectSQL+` WHERE LOWER(username) = LOWER($1)`, username))
	if err != nil {
		return identity.User{}, mapUserError("get user by username", err)
	}
	return user, nil
}

// ListUsers returns platform users for administration. Business resources remain user-scoped.
func (s *Store) ListUsers(ctx context.Context) ([]identity.User, error) {
	rows, err := s.db.QueryContext(ctx, userSelectSQL+` ORDER BY username`)
	if err != nil {
		return nil, fmt.Errorf("list users query: %w", err)
	}
	defer rows.Close()
	users := make([]identity.User, 0)
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, fmt.Errorf("list users scan: %w", err)
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list users rows: %w", err)
	}
	return users, nil
}

const userSelectSQL = `SELECT id, username, password_hash, role, status, created_at, updated_at FROM app_user`

type userScanner interface {
	Scan(dest ...any) error
}

func scanUser(scanner userScanner) (identity.User, error) {
	var user identity.User
	if err := scanner.Scan(&user.ID, &user.Username, &user.PasswordHash, &user.Role, &user.Status, &user.CreatedAt, &user.UpdatedAt); err != nil {
		return identity.User{}, err
	}
	return user, nil
}

func mapUserError(action string, err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%s: %w", action, identity.ErrNotFound)
	}
	if isUniqueViolation(err) {
		return fmt.Errorf("%s: %w", action, identity.ErrConflict)
	}
	return fmt.Errorf("%s: %w", action, err)
}
