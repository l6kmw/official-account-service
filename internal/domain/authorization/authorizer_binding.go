package authorization

import (
	"context"
	"errors"
	"time"
)

// ErrAuthorizerTenantBindingNotFound indicates an authorizer tenant binding was not found.
var ErrAuthorizerTenantBindingNotFound = errors.New("authorizer tenant binding not found")

// AuthorizerTenantBinding maps a WeChat authorizer app id to its owning tenant.
type AuthorizerTenantBinding struct {
	ComponentAppID  string
	AuthorizerAppID string
	TenantID        string
	UpdatedAt       time.Time
}

// AuthorizerTenantBindingRepository persists authorizer-to-tenant callback routing metadata.
type AuthorizerTenantBindingRepository interface {
	SaveAuthorizerTenantBinding(ctx context.Context, binding AuthorizerTenantBinding) (AuthorizerTenantBinding, error)
	GetAuthorizerTenantBinding(ctx context.Context, componentAppID string, authorizerAppID string) (AuthorizerTenantBinding, error)
}
