package wechat

import (
	"context"
	"fmt"

	"official-account-service/internal/domain/authorization"
)

// DisabledPreAuthCodeCreator is a placeholder until the real WeChat auth client is implemented.
type DisabledPreAuthCodeCreator struct{}

// CreatePreAuthCode returns an unavailable error for pre-auth-code creation.
func (DisabledPreAuthCodeCreator) CreatePreAuthCode(_ context.Context, _ string) (authorization.PreAuthCode, error) {
	return authorization.PreAuthCode{}, fmt.Errorf("wechat pre auth code creator: %w", authorization.ErrPreAuthCodeUnavailable)
}
