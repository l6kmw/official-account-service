package application

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"official-account-service/internal/domain/authorization"
)

func (s *TokenService) cacheKey(input RefreshAuthorizerAccessTokenInput) string {
	return fmt.Sprintf("%s:%d:%s", input.TenantID, input.AccountID, input.ComponentAppID)
}

func (s *TokenService) cachedAuthorizerToken(key string) (AuthorizerAccessToken, bool) {
	s.cacheMu.RLock()
	defer s.cacheMu.RUnlock()
	token, ok := s.cache[key]
	return token, ok
}

func (s *TokenService) cacheAuthorizerToken(key string, token AuthorizerAccessToken) {
	if strings.TrimSpace(token.AccessToken) == "" || token.ExpiresAt.IsZero() {
		return
	}
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	s.cache[key] = token
}

func (s *TokenService) sharedAuthorizerToken(ctx context.Context, key string) (AuthorizerAccessToken, bool, error) {
	if s.sharedCache == nil {
		return AuthorizerAccessToken{}, false, nil
	}
	cached, ok, err := s.sharedCache.GetAuthorizerAccessToken(ctx, key)
	if err != nil {
		return AuthorizerAccessToken{}, false, wrapAuthorizerAccessTokenCacheError("get shared authorizer access token", err)
	}
	if !ok {
		return AuthorizerAccessToken{}, false, nil
	}
	return AuthorizerAccessToken{
		AccountID:   cached.AccountID,
		AppID:       cached.AppID,
		AccessToken: cached.AccessToken,
		ExpiresAt:   cached.ExpiresAt,
	}, true, nil
}

func (s *TokenService) storeSharedAuthorizerToken(ctx context.Context, key string, token AuthorizerAccessToken) error {
	if s.sharedCache == nil {
		return nil
	}
	return wrapAuthorizerAccessTokenCacheError("set shared authorizer access token", s.sharedCache.SetAuthorizerAccessToken(ctx, key, authorization.CachedAuthorizerAccessToken{
		AccountID:   token.AccountID,
		AppID:       token.AppID,
		AccessToken: token.AccessToken,
		ExpiresAt:   token.ExpiresAt,
	}))
}

func (s *TokenService) needsRefresh(token AuthorizerAccessToken) bool {
	return strings.TrimSpace(token.AccessToken) == "" || !token.ExpiresAt.After(s.now().Add(s.refreshBefore))
}

func (s *TokenService) scheduleNextTokenRefresh(ctx context.Context, input RefreshAuthorizerAccessTokenInput, token AuthorizerAccessToken) error {
	if s.scheduler == nil {
		return nil
	}
	if token.ExpiresAt.IsZero() {
		return fmt.Errorf("validate token refresh schedule expiry: %w", ErrInvalidInput)
	}
	delay := token.ExpiresAt.Sub(s.now().Add(s.refreshBefore))
	if delay < 0 {
		delay = 0
	}
	if err := s.scheduler.ScheduleTokenRefresh(ctx, authorization.TokenRefreshTask{
		TenantID: input.TenantID, AccountID: input.AccountID, ComponentAppID: input.ComponentAppID,
	}, delay); err != nil {
		return wrapTokenRefreshTaskQueueError("schedule next authorizer token refresh", err)
	}
	return nil
}

func maxInt64(a int64, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func wrapTokenAccountError(action string, err error) error {
	if errors.Is(err, authorization.ErrNotFound) {
		return fmt.Errorf("%s: %w", action, ErrNotFound)
	}
	return fmt.Errorf("%s: %w", action, err)
}

func wrapTokenRefreshLockError(action string, err error) error {
	if errors.Is(err, authorization.ErrTokenRefreshLockUnavailable) {
		return fmt.Errorf("%s: %w", action, ErrNotImplemented)
	}
	return fmt.Errorf("%s: %w", action, err)
}

func wrapAuthorizerAccessTokenCacheError(action string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, authorization.ErrAuthorizerAccessTokenCacheUnavailable) {
		return fmt.Errorf("%s: %w", action, ErrNotImplemented)
	}
	return fmt.Errorf("%s: %w", action, err)
}

func wrapTokenRefreshTaskQueueError(action string, err error) error {
	if errors.Is(err, authorization.ErrTokenRefreshTaskQueueUnavailable) {
		return fmt.Errorf("%s: %w", action, ErrNotImplemented)
	}
	return fmt.Errorf("%s: %w", action, err)
}
