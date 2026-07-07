package authorization

import (
	"context"
	"errors"
	"time"
)

// ErrAuthorizerClientUnavailable indicates the WeChat authorizer client is not configured.
var ErrAuthorizerClientUnavailable = errors.New("authorizer client unavailable")

// ErrRefreshTokenEncryptorUnavailable indicates refresh-token encryption is not configured.
var ErrRefreshTokenEncryptorUnavailable = errors.New("refresh token encryptor unavailable")

// ErrTokenRefreshLockUnavailable indicates the token refresh lock backend is unavailable.
var ErrTokenRefreshLockUnavailable = errors.New("token refresh lock unavailable")

// ErrAuthorizerAccessTokenCacheUnavailable indicates the shared token cache is unavailable.
var ErrAuthorizerAccessTokenCacheUnavailable = errors.New("authorizer access token cache unavailable")

// ErrTokenRefreshTaskQueueUnavailable indicates token refresh task queueing is unavailable.
var ErrTokenRefreshTaskQueueUnavailable = errors.New("token refresh task queue unavailable")

// AuthorizerAuthorization contains WeChat authorization data returned for an auth code.
type AuthorizerAuthorization struct {
	AppID        string
	RefreshToken string
}

// AuthorizerProfile contains WeChat authorizer account profile data.
type AuthorizerProfile struct {
	Name      string
	AvatarURL string
}

// AuthorizerToken contains a refreshed authorizer access token.
type AuthorizerToken struct {
	AccessToken      string
	RefreshToken     string
	ExpiresInSeconds int
}

// CachedAuthorizerAccessToken contains a cacheable authorizer access token.
type CachedAuthorizerAccessToken struct {
	AccountID   int64
	AppID       string
	AccessToken string
	ExpiresAt   time.Time
}

// TokenRefreshTask identifies one tenant-scoped account token refresh job.
type TokenRefreshTask struct {
	TenantID       string
	AccountID      int64
	ComponentAppID string
}

// AuthorizerClient fetches authorizer authorization and profile data from WeChat.
type AuthorizerClient interface {
	QueryAuthorizerAuthorization(ctx context.Context, componentAppID string, authCode string) (AuthorizerAuthorization, error)
	GetAuthorizerProfile(ctx context.Context, componentAppID string, authorizerAppID string) (AuthorizerProfile, error)
	RefreshAuthorizerAccessToken(ctx context.Context, componentAppID string, authorizerAppID string, refreshToken string) (AuthorizerToken, error)
}

// RefreshTokenEncryptor encrypts authorizer refresh tokens before persistence.
type RefreshTokenEncryptor interface {
	EncryptAuthorizerRefreshToken(ctx context.Context, plaintext string) (string, error)
}

// RefreshTokenDecryptor decrypts authorizer refresh tokens for token refresh.
type RefreshTokenDecryptor interface {
	DecryptAuthorizerRefreshToken(ctx context.Context, ciphertext string) (string, error)
}

// RefreshTokenCodec encrypts and decrypts authorizer refresh tokens.
type RefreshTokenCodec interface {
	RefreshTokenEncryptor
	RefreshTokenDecryptor
}

// TokenRefreshLock is a held distributed lock for one token refresh key.
type TokenRefreshLock interface {
	Release(ctx context.Context) error
}

// TokenRefreshLocker coordinates token refresh across service instances.
type TokenRefreshLocker interface {
	AcquireTokenRefreshLock(ctx context.Context, key string, ttl time.Duration) (TokenRefreshLock, error)
}

// AuthorizerAccessTokenCache shares authorizer access tokens across service instances.
type AuthorizerAccessTokenCache interface {
	GetAuthorizerAccessToken(ctx context.Context, key string) (CachedAuthorizerAccessToken, bool, error)
	SetAuthorizerAccessToken(ctx context.Context, key string, token CachedAuthorizerAccessToken) error
}

// TokenRefreshScheduler schedules future authorizer access token refresh jobs.
type TokenRefreshScheduler interface {
	ScheduleTokenRefresh(ctx context.Context, task TokenRefreshTask, delay time.Duration) error
}

// TokenRefreshHandler handles authorizer access token refresh jobs.
type TokenRefreshHandler interface {
	HandleTokenRefreshTask(ctx context.Context, task TokenRefreshTask) error
}
