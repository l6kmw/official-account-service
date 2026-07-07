package authorization

import (
	"context"
	"errors"
)

// ErrPreAuthCodeUnavailable indicates the WeChat pre-auth-code creator is not configured.
var ErrPreAuthCodeUnavailable = errors.New("pre auth code creator unavailable")

// PreAuthCode is a WeChat third-party platform pre-authorization code.
type PreAuthCode struct {
	Code             string
	ExpiresInSeconds int
}

// PreAuthCodeCreator creates WeChat third-party platform pre-authorization codes.
type PreAuthCodeCreator interface {
	CreatePreAuthCode(ctx context.Context, componentAppID string) (PreAuthCode, error)
}
