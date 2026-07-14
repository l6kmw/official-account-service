package authorization

import (
	"context"
	"errors"
	"time"
)

// ErrAuthorizationStateNotFound indicates that an authorization state is missing, expired, or already consumed.
var ErrAuthorizationStateNotFound = errors.New("authorization state not found")

// ErrAuthorizationStateExists indicates an authorization state digest collision.
var ErrAuthorizationStateExists = errors.New("authorization state already exists")

// AuthorizationState binds one authorization attempt to a tenant and component app.
type AuthorizationState struct {
	Digest         string
	TenantID       string
	ComponentAppID string
	ExpiresAt      time.Time
	ConsumedAt     time.Time
	CreatedAt      time.Time
}

// AuthorizationStateRepository persists and atomically consumes one-time authorization states.
type AuthorizationStateRepository interface {
	SaveAuthorizationState(ctx context.Context, state AuthorizationState) (AuthorizationState, error)
	ConsumeAuthorizationState(ctx context.Context, digest string, consumedAt time.Time) (AuthorizationState, error)
}
