package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"official-account-service/internal/domain/authorization"
)

const authorizerAccessTokenCachePrefix = "official-account-service:authorizer-access-token:"

// AuthorizerAccessTokenCache stores authorizer access tokens in Redis.
type AuthorizerAccessTokenCache struct {
	client goredis.Cmdable
	now    func() time.Time
}

// NewAuthorizerAccessTokenCache constructs a Redis-backed authorizer access token cache.
func NewAuthorizerAccessTokenCache(client goredis.Cmdable, now func() time.Time) *AuthorizerAccessTokenCache {
	if now == nil {
		now = time.Now
	}
	return &AuthorizerAccessTokenCache{client: client, now: now}
}

// GetAuthorizerAccessToken gets a cached token by cache key.
func (c *AuthorizerAccessTokenCache) GetAuthorizerAccessToken(ctx context.Context, key string) (authorization.CachedAuthorizerAccessToken, bool, error) {
	if c == nil || c.client == nil {
		return authorization.CachedAuthorizerAccessToken{}, false, fmt.Errorf("validate authorizer token cache: %w", authorization.ErrAuthorizerAccessTokenCacheUnavailable)
	}
	if strings.TrimSpace(key) == "" {
		return authorization.CachedAuthorizerAccessToken{}, false, fmt.Errorf("validate authorizer token cache key: %w", authorization.ErrAuthorizerAccessTokenCacheUnavailable)
	}
	raw, err := c.client.Get(ctx, authorizerAccessTokenCachePrefix+key).Bytes()
	if err != nil {
		if err == goredis.Nil {
			return authorization.CachedAuthorizerAccessToken{}, false, nil
		}
		return authorization.CachedAuthorizerAccessToken{}, false, fmt.Errorf("get authorizer token cache: %w", authorization.ErrAuthorizerAccessTokenCacheUnavailable)
	}
	var token authorization.CachedAuthorizerAccessToken
	if err := json.Unmarshal(raw, &token); err != nil {
		return authorization.CachedAuthorizerAccessToken{}, false, fmt.Errorf("decode authorizer token cache: %w", authorization.ErrAuthorizerAccessTokenCacheUnavailable)
	}
	if strings.TrimSpace(token.AccessToken) == "" || token.ExpiresAt.IsZero() {
		return authorization.CachedAuthorizerAccessToken{}, false, fmt.Errorf("validate authorizer token cache payload: %w", authorization.ErrAuthorizerAccessTokenCacheUnavailable)
	}
	return token, true, nil
}

// SetAuthorizerAccessToken sets a cached token until its expiry time.
func (c *AuthorizerAccessTokenCache) SetAuthorizerAccessToken(ctx context.Context, key string, token authorization.CachedAuthorizerAccessToken) error {
	if c == nil || c.client == nil {
		return fmt.Errorf("validate authorizer token cache: %w", authorization.ErrAuthorizerAccessTokenCacheUnavailable)
	}
	if strings.TrimSpace(key) == "" || strings.TrimSpace(token.AccessToken) == "" || token.ExpiresAt.IsZero() {
		return fmt.Errorf("validate authorizer token cache input: %w", authorization.ErrAuthorizerAccessTokenCacheUnavailable)
	}
	ttl := time.Until(token.ExpiresAt)
	if c.now != nil {
		ttl = token.ExpiresAt.Sub(c.now())
	}
	if ttl <= 0 {
		return fmt.Errorf("validate authorizer token cache ttl: %w", authorization.ErrAuthorizerAccessTokenCacheUnavailable)
	}
	raw, err := json.Marshal(token)
	if err != nil {
		return fmt.Errorf("encode authorizer token cache: %w", authorization.ErrAuthorizerAccessTokenCacheUnavailable)
	}
	if err := c.client.Set(ctx, authorizerAccessTokenCachePrefix+key, raw, ttl).Err(); err != nil {
		return fmt.Errorf("set authorizer token cache: %w", authorization.ErrAuthorizerAccessTokenCacheUnavailable)
	}
	return nil
}
