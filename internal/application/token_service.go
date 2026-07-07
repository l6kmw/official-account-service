package application

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"official-account-service/internal/domain/authorization"
)

// TokenService manages authorizer access tokens.
type TokenService struct {
	accounts      authorization.Repository
	authorizers   authorization.AuthorizerClient
	refreshTokens authorization.RefreshTokenCodec
	locker        authorization.TokenRefreshLocker
	sharedCache   authorization.AuthorizerAccessTokenCache
	scheduler     authorization.TokenRefreshScheduler
	now           func() time.Time
	refreshGroup  singleflight.Group
	cacheMu       sync.RWMutex
	cache         map[string]AuthorizerAccessToken
	refreshBefore time.Duration
	lockTTL       time.Duration
}

const defaultAuthorizerAccessTokenRefreshBefore = 5 * time.Minute
const defaultAuthorizerTokenRefreshLockTTL = 30 * time.Second

// NewTokenService constructs a TokenService.
func NewTokenService(accounts authorization.Repository, authorizers authorization.AuthorizerClient, refreshTokens authorization.RefreshTokenCodec, now func() time.Time) *TokenService {
	if now == nil {
		now = time.Now
	}
	return &TokenService{
		accounts:      accounts,
		authorizers:   authorizers,
		refreshTokens: refreshTokens,
		now:           now,
		cache:         make(map[string]AuthorizerAccessToken),
		refreshBefore: defaultAuthorizerAccessTokenRefreshBefore,
		lockTTL:       defaultAuthorizerTokenRefreshLockTTL,
	}
}

// NewTokenServiceWithLocker constructs a TokenService with distributed refresh locking.
func NewTokenServiceWithLocker(accounts authorization.Repository, authorizers authorization.AuthorizerClient, refreshTokens authorization.RefreshTokenCodec, locker authorization.TokenRefreshLocker, now func() time.Time) *TokenService {
	return NewTokenServiceWithLockerAndCache(accounts, authorizers, refreshTokens, locker, nil, now)
}

// NewTokenServiceWithLockerAndCache constructs a TokenService with distributed refresh locking and shared token cache.
func NewTokenServiceWithLockerAndCache(accounts authorization.Repository, authorizers authorization.AuthorizerClient, refreshTokens authorization.RefreshTokenCodec, locker authorization.TokenRefreshLocker, sharedCache authorization.AuthorizerAccessTokenCache, now func() time.Time) *TokenService {
	return NewTokenServiceWithLockerCacheAndScheduler(accounts, authorizers, refreshTokens, locker, sharedCache, nil, now)
}

// NewTokenServiceWithLockerCacheAndScheduler constructs a TokenService with refresh locking, shared cache, and scheduled refresh.
func NewTokenServiceWithLockerCacheAndScheduler(accounts authorization.Repository, authorizers authorization.AuthorizerClient, refreshTokens authorization.RefreshTokenCodec, locker authorization.TokenRefreshLocker, sharedCache authorization.AuthorizerAccessTokenCache, scheduler authorization.TokenRefreshScheduler, now func() time.Time) *TokenService {
	service := NewTokenService(accounts, authorizers, refreshTokens, now)
	service.locker = locker
	service.sharedCache = sharedCache
	service.scheduler = scheduler
	return service
}

// RefreshAuthorizerAccessTokenInput contains fields for refreshing an authorizer token.
type RefreshAuthorizerAccessTokenInput struct {
	TenantID       string
	AccountID      int64
	ComponentAppID string
}

// AuthorizerAccessToken contains a refreshed authorizer access token.
type AuthorizerAccessToken struct {
	AccountID   int64
	AppID       string
	AccessToken string
	ExpiresAt   time.Time
}

// AuthorizerTokenStatus describes cached token state without exposing token values.
type AuthorizerTokenStatus struct {
	AccountID        int64
	AppID            string
	AccountStatus    authorization.AccountStatus
	Cached           bool
	ExpiresAt        time.Time
	ExpiresInSeconds int64
	NeedsRefresh     bool
}

// GetAuthorizerAccessToken returns a cached authorizer access token or refreshes it when missing/near expiry.
func (s *TokenService) GetAuthorizerAccessToken(ctx context.Context, input RefreshAuthorizerAccessTokenInput) (AuthorizerAccessToken, error) {
	if err := s.validateRefreshInput(input); err != nil {
		return AuthorizerAccessToken{}, err
	}
	account, err := s.accountForToken(ctx, input)
	if err != nil {
		return AuthorizerAccessToken{}, err
	}
	key := s.cacheKey(input)
	cached, ok := s.cachedAuthorizerToken(key)
	if ok && !s.needsRefresh(cached) {
		cached.AccountID = account.ID
		cached.AppID = account.AppID
		return cached, nil
	}
	shared, ok, err := s.sharedAuthorizerToken(ctx, key)
	if err != nil {
		return AuthorizerAccessToken{}, err
	}
	if ok && shared.AccountID == account.ID && shared.AppID == account.AppID && !s.needsRefresh(shared) {
		s.cacheAuthorizerToken(key, shared)
		return shared, nil
	}
	return s.RefreshAuthorizerAccessToken(ctx, input)
}

// RefreshAuthorizerAccessToken refreshes one tenant-scoped authorizer access token.
func (s *TokenService) RefreshAuthorizerAccessToken(ctx context.Context, input RefreshAuthorizerAccessTokenInput) (AuthorizerAccessToken, error) {
	if err := s.validateRefreshInput(input); err != nil {
		return AuthorizerAccessToken{}, err
	}
	key := s.cacheKey(input)
	value, err, _ := s.refreshGroup.Do(key, func() (any, error) {
		return s.refreshWithLock(ctx, key, input)
	})
	if err != nil {
		return AuthorizerAccessToken{}, err
	}
	token, ok := value.(AuthorizerAccessToken)
	if !ok {
		return AuthorizerAccessToken{}, fmt.Errorf("validate authorizer access token result: %w", ErrInvalidInput)
	}
	s.cacheAuthorizerToken(key, token)
	if err := s.scheduleNextTokenRefresh(ctx, input, token); err != nil {
		return AuthorizerAccessToken{}, err
	}
	return token, nil
}

func (s *TokenService) refreshWithLock(ctx context.Context, key string, input RefreshAuthorizerAccessTokenInput) (AuthorizerAccessToken, error) {
	if s.locker == nil {
		token, err := s.refreshAuthorizerAccessToken(ctx, input)
		if err != nil {
			return AuthorizerAccessToken{}, err
		}
		if err := s.storeSharedAuthorizerToken(ctx, key, token); err != nil {
			return AuthorizerAccessToken{}, err
		}
		return token, nil
	}
	lock, err := s.locker.AcquireTokenRefreshLock(ctx, key, s.lockTTL)
	if err != nil {
		return AuthorizerAccessToken{}, wrapTokenRefreshLockError("acquire authorizer token refresh lock", err)
	}
	defer func() {
		_ = lock.Release(context.WithoutCancel(ctx))
	}()
	account, err := s.accountForToken(ctx, input)
	if err != nil {
		return AuthorizerAccessToken{}, err
	}
	shared, ok, err := s.sharedAuthorizerToken(ctx, key)
	if err != nil {
		return AuthorizerAccessToken{}, err
	}
	if ok && shared.AccountID == account.ID && shared.AppID == account.AppID && !s.needsRefresh(shared) {
		return shared, nil
	}
	token, err := s.refreshAuthorizerAccessToken(ctx, input)
	if err != nil {
		return AuthorizerAccessToken{}, err
	}
	if err := s.storeSharedAuthorizerToken(ctx, key, token); err != nil {
		return AuthorizerAccessToken{}, err
	}
	return token, nil
}

// GetAuthorizerTokenStatus returns token cache state without exposing access or refresh tokens.
func (s *TokenService) GetAuthorizerTokenStatus(ctx context.Context, input RefreshAuthorizerAccessTokenInput) (AuthorizerTokenStatus, error) {
	if err := s.validateRefreshInput(input); err != nil {
		return AuthorizerTokenStatus{}, err
	}
	account, err := s.accounts.GetAccount(ctx, input.TenantID, input.AccountID)
	if err != nil {
		return AuthorizerTokenStatus{}, wrapTokenAccountError("get account for token status", err)
	}
	status := AuthorizerTokenStatus{
		AccountID:     account.ID,
		AppID:         account.AppID,
		AccountStatus: authorization.AccountStatus(account.Status),
	}
	cached, ok := s.cachedAuthorizerToken(s.cacheKey(input))
	status.Cached = ok
	status.NeedsRefresh = account.Status == authorization.AccountStatusActive
	if ok {
		status.ExpiresAt = cached.ExpiresAt
		status.ExpiresInSeconds = maxInt64(0, int64(cached.ExpiresAt.Sub(s.now()).Seconds()))
		status.NeedsRefresh = account.Status == authorization.AccountStatusActive && s.needsRefresh(cached)
		if !status.NeedsRefresh {
			return status, nil
		}
	}
	shared, ok, err := s.sharedAuthorizerToken(ctx, s.cacheKey(input))
	if err != nil {
		return AuthorizerTokenStatus{}, err
	}
	if ok {
		status.Cached = true
		status.ExpiresAt = shared.ExpiresAt
		status.ExpiresInSeconds = maxInt64(0, int64(shared.ExpiresAt.Sub(s.now()).Seconds()))
		status.NeedsRefresh = account.Status == authorization.AccountStatusActive && s.needsRefresh(shared)
	}
	return status, nil
}

func (s *TokenService) refreshAuthorizerAccessToken(ctx context.Context, input RefreshAuthorizerAccessTokenInput) (AuthorizerAccessToken, error) {
	account, err := s.accountForToken(ctx, input)
	if err != nil {
		return AuthorizerAccessToken{}, err
	}
	refreshToken, err := s.refreshTokens.DecryptAuthorizerRefreshToken(ctx, account.EncryptedAuthorizerRefreshToken)
	if err != nil {
		return AuthorizerAccessToken{}, s.markRefreshFailed(ctx, input, wrapRefreshTokenEncryptorError("decrypt authorizer refresh token", err))
	}
	token, err := s.authorizers.RefreshAuthorizerAccessToken(ctx, input.ComponentAppID, account.AppID, refreshToken)
	if err != nil {
		return AuthorizerAccessToken{}, s.markRefreshFailed(ctx, input, wrapAuthorizerClientError("refresh authorizer access token", err))
	}
	if strings.TrimSpace(token.AccessToken) == "" || token.ExpiresInSeconds <= 0 {
		return AuthorizerAccessToken{}, s.markRefreshFailed(ctx, input, fmt.Errorf("validate authorizer access token response: %w", ErrInvalidInput))
	}
	if strings.TrimSpace(token.RefreshToken) != "" && token.RefreshToken != refreshToken {
		encrypted, err := s.refreshTokens.EncryptAuthorizerRefreshToken(ctx, token.RefreshToken)
		if err != nil {
			return AuthorizerAccessToken{}, s.markRefreshFailed(ctx, input, wrapRefreshTokenEncryptorError("encrypt refreshed authorizer refresh token", err))
		}
		account.EncryptedAuthorizerRefreshToken = encrypted
		account.Status = authorization.AccountStatusActive
		if _, err := s.accounts.SaveAccount(ctx, input.TenantID, account); err != nil {
			return AuthorizerAccessToken{}, fmt.Errorf("save refreshed authorizer refresh token: %w", err)
		}
	}
	return AuthorizerAccessToken{
		AccountID:   account.ID,
		AppID:       account.AppID,
		AccessToken: token.AccessToken,
		ExpiresAt:   s.now().Add(time.Duration(token.ExpiresInSeconds) * time.Second),
	}, nil
}

func (s *TokenService) accountForToken(ctx context.Context, input RefreshAuthorizerAccessTokenInput) (authorization.Account, error) {
	account, err := s.accounts.GetAccount(ctx, input.TenantID, input.AccountID)
	if err != nil {
		return authorization.Account{}, wrapTokenAccountError("get account for token refresh", err)
	}
	if account.Status != authorization.AccountStatusActive {
		return authorization.Account{}, fmt.Errorf("validate account status for token refresh: %w", ErrInvalidInput)
	}
	if strings.TrimSpace(account.EncryptedAuthorizerRefreshToken) == "" {
		return authorization.Account{}, fmt.Errorf("validate encrypted authorizer refresh token: %w", ErrInvalidInput)
	}
	return account, nil
}

func (s *TokenService) validateRefreshInput(input RefreshAuthorizerAccessTokenInput) error {
	if s == nil || s.accounts == nil || s.authorizers == nil || s.refreshTokens == nil {
		return fmt.Errorf("validate token service dependencies: %w", ErrNotImplemented)
	}
	if strings.TrimSpace(input.TenantID) == "" {
		return fmt.Errorf("validate token tenant id: %w", ErrInvalidInput)
	}
	if input.AccountID <= 0 {
		return fmt.Errorf("validate token account id: %w", ErrInvalidInput)
	}
	if strings.TrimSpace(input.ComponentAppID) == "" {
		return fmt.Errorf("validate token component app id: %w", ErrInvalidInput)
	}
	return nil
}

func (s *TokenService) markRefreshFailed(ctx context.Context, input RefreshAuthorizerAccessTokenInput, err error) error {
	if _, statusErr := s.accounts.UpdateAccountStatus(ctx, input.TenantID, input.AccountID, authorization.AccountStatusRefreshFailed); statusErr != nil {
		return fmt.Errorf("mark account refresh failed: %w", statusErr)
	}
	return err
}
