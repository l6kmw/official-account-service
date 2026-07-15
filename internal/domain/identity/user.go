package identity

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrNotFound indicates that a user does not exist.
	ErrNotFound = errors.New("user not found")
	// ErrConflict indicates that a user id or username is already owned by another user.
	ErrConflict = errors.New("user conflict")
)

// Role controls platform-level user administration only. Business data remains user-scoped for every role.
type Role string

const (
	RoleAdmin Role = "admin"
	RoleUser  Role = "user"
)

// Status controls whether a user may authenticate.
type Status string

const (
	StatusActive   Status = "active"
	StatusDisabled Status = "disabled"
)

// User owns one isolated data space. ID is also used as the existing tenant_id value.
type User struct {
	ID                string
	Username          string
	PasswordHash      string
	Role              Role
	Status            Status
	APITokenHash      string
	APITokenHint      string
	APITokenCreatedAt *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// Repository persists platform users and authentication data.
type Repository interface {
	SaveUser(ctx context.Context, user User) (User, error)
	GetUser(ctx context.Context, id string) (User, error)
	GetUserByUsername(ctx context.Context, username string) (User, error)
	GetUserByAPITokenHash(ctx context.Context, tokenHash string) (User, error)
	ListUsers(ctx context.Context) ([]User, error)
	SaveUserAPIToken(ctx context.Context, userID string, tokenHash string, tokenHint string) (User, error)
	RevokeUserAPIToken(ctx context.Context, userID string) (User, error)
}
