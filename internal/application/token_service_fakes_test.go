package application

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"official-account-service/internal/domain/authorization"
)

type fakeTokenAuthorizerClient struct {
	tokens []authorization.AuthorizerToken
	token  authorization.AuthorizerToken
	err    error
	wait   chan struct{}
	calls  int32
}

func (c *fakeTokenAuthorizerClient) QueryAuthorizerAuthorization(_ context.Context, _ string, _ string) (authorization.AuthorizerAuthorization, error) {
	return authorization.AuthorizerAuthorization{}, nil
}

func (c *fakeTokenAuthorizerClient) GetAuthorizerProfile(_ context.Context, _ string, _ string) (authorization.AuthorizerProfile, error) {
	return authorization.AuthorizerProfile{}, nil
}

func (c *fakeTokenAuthorizerClient) RefreshAuthorizerAccessToken(_ context.Context, _ string, _ string, _ string) (authorization.AuthorizerToken, error) {
	call := atomic.AddInt32(&c.calls, 1)
	if c.wait != nil {
		<-c.wait
	}
	if c.err != nil {
		return authorization.AuthorizerToken{}, c.err
	}
	if len(c.tokens) > 0 {
		index := int(call - 1)
		if index < len(c.tokens) {
			return c.tokens[index], nil
		}
	}
	return c.token, nil
}

type fakeRefreshTokenCodec struct {
	plaintext     string
	ciphertext    string
	err           error
	lastDecrypted string
	lastEncrypted string
}

func (c *fakeRefreshTokenCodec) EncryptAuthorizerRefreshToken(_ context.Context, plaintext string) (string, error) {
	c.lastEncrypted = plaintext
	if c.err != nil {
		return "", c.err
	}
	return c.ciphertext, nil
}

func (c *fakeRefreshTokenCodec) DecryptAuthorizerRefreshToken(_ context.Context, ciphertext string) (string, error) {
	c.lastDecrypted = c.plaintext
	if c.err != nil {
		return "", c.err
	}
	return c.plaintext, nil
}

type fakeTokenRefreshLocker struct {
	err      error
	lastKey  string
	lastTTL  time.Duration
	acquires int32
	releases int32
}

func (l *fakeTokenRefreshLocker) AcquireTokenRefreshLock(_ context.Context, key string, ttl time.Duration) (authorization.TokenRefreshLock, error) {
	atomic.AddInt32(&l.acquires, 1)
	l.lastKey = key
	l.lastTTL = ttl
	if l.err != nil {
		return nil, l.err
	}
	return fakeTokenRefreshLock{releases: &l.releases}, nil
}

type fakeTokenRefreshLock struct {
	releases *int32
}

func (l fakeTokenRefreshLock) Release(_ context.Context) error {
	atomic.AddInt32(l.releases, 1)
	return nil
}

type fakeAuthorizerAccessTokenCache struct {
	mu     sync.Mutex
	tokens map[string]authorization.CachedAuthorizerAccessToken
	err    error
}

func newFakeAuthorizerAccessTokenCache() *fakeAuthorizerAccessTokenCache {
	return &fakeAuthorizerAccessTokenCache{tokens: make(map[string]authorization.CachedAuthorizerAccessToken)}
}

func (c *fakeAuthorizerAccessTokenCache) GetAuthorizerAccessToken(_ context.Context, key string) (authorization.CachedAuthorizerAccessToken, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.err != nil {
		return authorization.CachedAuthorizerAccessToken{}, false, c.err
	}
	token, ok := c.tokens[key]
	return token, ok, nil
}

func (c *fakeAuthorizerAccessTokenCache) SetAuthorizerAccessToken(_ context.Context, key string, token authorization.CachedAuthorizerAccessToken) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.err != nil {
		return c.err
	}
	c.tokens[key] = token
	return nil
}

func (c *fakeAuthorizerAccessTokenCache) set(key string, token authorization.CachedAuthorizerAccessToken) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.tokens[key] = token
}

type fakeTokenRefreshScheduler struct {
	lastTask  authorization.TokenRefreshTask
	lastDelay time.Duration
	err       error
}

func (s *fakeTokenRefreshScheduler) ScheduleTokenRefresh(_ context.Context, task authorization.TokenRefreshTask, delay time.Duration) error {
	s.lastTask = task
	s.lastDelay = delay
	return s.err
}
