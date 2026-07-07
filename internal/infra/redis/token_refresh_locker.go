package redis

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"official-account-service/internal/domain/authorization"
)

const tokenRefreshLockPrefix = "official-account-service:token-refresh:"
const defaultTokenRefreshLockRetryDelay = 50 * time.Millisecond

// TokenRefreshLocker coordinates authorizer token refresh with Redis SET NX locks.
type TokenRefreshLocker struct {
	client     goredis.Cmdable
	retryDelay time.Duration
}

// OpenClient creates and verifies a Redis client.
func OpenClient(ctx context.Context, addr string) (*goredis.Client, error) {
	if strings.TrimSpace(addr) == "" {
		return nil, fmt.Errorf("validate redis addr: %w", authorization.ErrTokenRefreshLockUnavailable)
	}
	client := goredis.NewClient(&goredis.Options{Addr: addr})
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ping redis: %w", authorization.ErrTokenRefreshLockUnavailable)
	}
	return client, nil
}

// NewTokenRefreshLocker constructs a Redis-backed token refresh locker.
func NewTokenRefreshLocker(client goredis.Cmdable) *TokenRefreshLocker {
	return &TokenRefreshLocker{client: client, retryDelay: defaultTokenRefreshLockRetryDelay}
}

// AcquireTokenRefreshLock waits until a lock is acquired or ctx is canceled.
func (l *TokenRefreshLocker) AcquireTokenRefreshLock(ctx context.Context, key string, ttl time.Duration) (authorization.TokenRefreshLock, error) {
	if l == nil || l.client == nil {
		return nil, fmt.Errorf("validate redis token refresh locker: %w", authorization.ErrTokenRefreshLockUnavailable)
	}
	if strings.TrimSpace(key) == "" || ttl <= 0 {
		return nil, fmt.Errorf("validate token refresh lock input: %w", authorization.ErrTokenRefreshLockUnavailable)
	}
	value, err := randomLockValue()
	if err != nil {
		return nil, fmt.Errorf("generate token refresh lock value: %w", authorization.ErrTokenRefreshLockUnavailable)
	}
	lockKey := tokenRefreshLockPrefix + key
	for {
		acquired, err := l.client.SetNX(ctx, lockKey, value, ttl).Result()
		if err != nil {
			return nil, fmt.Errorf("acquire token refresh lock: %w", authorization.ErrTokenRefreshLockUnavailable)
		}
		if acquired {
			return tokenRefreshLock{client: l.client, key: lockKey, value: value}, nil
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("wait token refresh lock: %w", ctx.Err())
		case <-time.After(l.retryDelay):
		}
	}
}

type tokenRefreshLock struct {
	client goredis.Cmdable
	key    string
	value  string
}

// Release deletes the lock only when it is still owned by this instance.
func (l tokenRefreshLock) Release(ctx context.Context) error {
	if l.client == nil || strings.TrimSpace(l.key) == "" || strings.TrimSpace(l.value) == "" {
		return fmt.Errorf("validate token refresh lock: %w", authorization.ErrTokenRefreshLockUnavailable)
	}
	const releaseScript = `if redis.call("get", KEYS[1]) == ARGV[1] then return redis.call("del", KEYS[1]) end return 0`
	if err := l.client.Eval(ctx, releaseScript, []string{l.key}, l.value).Err(); err != nil {
		return fmt.Errorf("release token refresh lock: %w", authorization.ErrTokenRefreshLockUnavailable)
	}
	return nil
}

func randomLockValue() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
