package application

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"official-account-service/internal/domain/authorization"
	"official-account-service/internal/infra/persistence/memory"
)

func TestTokenServiceRefreshesAuthorizerAccessToken(t *testing.T) {
	ctx := context.Background()
	store := memory.NewStore(func() time.Time { return time.Date(2026, 7, 6, 20, 0, 0, 0, time.UTC) })
	account, err := store.SaveAccount(ctx, "tenant-1", authorization.Account{
		AppID: "wx-authorizer", Status: authorization.AccountStatusActive,
		EncryptedAuthorizerRefreshToken: "encrypted-refresh",
	})
	require.NoError(t, err)
	codec := &fakeRefreshTokenCodec{plaintext: "refresh-token", ciphertext: "encrypted-refresh-2"}
	service := NewTokenService(store, &fakeTokenAuthorizerClient{
		token: authorization.AuthorizerToken{AccessToken: "authorizer-token", RefreshToken: "refresh-token-2", ExpiresInSeconds: 7200},
	}, codec, func() time.Time { return time.Date(2026, 7, 6, 20, 0, 0, 0, time.UTC) })

	token, err := service.RefreshAuthorizerAccessToken(ctx, RefreshAuthorizerAccessTokenInput{
		TenantID: "tenant-1", AccountID: account.ID, ComponentAppID: "wx-component",
	})
	require.NoError(t, err)
	require.Equal(t, "authorizer-token", token.AccessToken)
	require.Equal(t, time.Date(2026, 7, 6, 22, 0, 0, 0, time.UTC), token.ExpiresAt)
	require.Equal(t, "refresh-token", codec.lastDecrypted)
	require.Equal(t, "refresh-token-2", codec.lastEncrypted)

	updated, err := store.GetAccount(ctx, "tenant-1", account.ID)
	require.NoError(t, err)
	require.Equal(t, "encrypted-refresh-2", updated.EncryptedAuthorizerRefreshToken)
	require.Equal(t, authorization.AccountStatusActive, updated.Status)
}

func TestTokenServiceReturnsCachedAuthorizerAccessToken(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 6, 20, 0, 0, 0, time.UTC)
	store := memory.NewStore(func() time.Time { return now })
	account, err := store.SaveAccount(ctx, "tenant-1", authorization.Account{
		AppID: "wx-authorizer", Status: authorization.AccountStatusActive,
		EncryptedAuthorizerRefreshToken: "encrypted-refresh",
	})
	require.NoError(t, err)
	client := &fakeTokenAuthorizerClient{
		token: authorization.AuthorizerToken{AccessToken: "authorizer-token", ExpiresInSeconds: 7200},
	}
	service := NewTokenService(store, client, &fakeRefreshTokenCodec{plaintext: "refresh-token"}, func() time.Time { return now })

	first, err := service.GetAuthorizerAccessToken(ctx, RefreshAuthorizerAccessTokenInput{
		TenantID: "tenant-1", AccountID: account.ID, ComponentAppID: "wx-component",
	})
	require.NoError(t, err)
	second, err := service.GetAuthorizerAccessToken(ctx, RefreshAuthorizerAccessTokenInput{
		TenantID: "tenant-1", AccountID: account.ID, ComponentAppID: "wx-component",
	})
	require.NoError(t, err)

	require.Equal(t, "authorizer-token", first.AccessToken)
	require.Equal(t, first, second)
	require.Equal(t, int32(1), atomic.LoadInt32(&client.calls))
}

func TestTokenServiceRefreshesCachedAuthorizerAccessTokenNearExpiry(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 6, 20, 0, 0, 0, time.UTC)
	store := memory.NewStore(func() time.Time { return now })
	account, err := store.SaveAccount(ctx, "tenant-1", authorization.Account{
		AppID: "wx-authorizer", Status: authorization.AccountStatusActive,
		EncryptedAuthorizerRefreshToken: "encrypted-refresh",
	})
	require.NoError(t, err)
	client := &fakeTokenAuthorizerClient{
		tokens: []authorization.AuthorizerToken{
			{AccessToken: "authorizer-token-1", ExpiresInSeconds: 600},
			{AccessToken: "authorizer-token-2", ExpiresInSeconds: 7200},
		},
	}
	service := NewTokenService(store, client, &fakeRefreshTokenCodec{plaintext: "refresh-token"}, func() time.Time { return now })

	first, err := service.GetAuthorizerAccessToken(ctx, RefreshAuthorizerAccessTokenInput{
		TenantID: "tenant-1", AccountID: account.ID, ComponentAppID: "wx-component",
	})
	require.NoError(t, err)
	now = now.Add(301 * time.Second)
	second, err := service.GetAuthorizerAccessToken(ctx, RefreshAuthorizerAccessTokenInput{
		TenantID: "tenant-1", AccountID: account.ID, ComponentAppID: "wx-component",
	})
	require.NoError(t, err)

	require.Equal(t, "authorizer-token-1", first.AccessToken)
	require.Equal(t, "authorizer-token-2", second.AccessToken)
	require.Equal(t, int32(2), atomic.LoadInt32(&client.calls))
}

func TestTokenServiceReturnsSharedCachedAuthorizerAccessToken(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 6, 20, 0, 0, 0, time.UTC)
	store := memory.NewStore(func() time.Time { return now })
	account, err := store.SaveAccount(ctx, "tenant-1", authorization.Account{
		AppID: "wx-authorizer", Status: authorization.AccountStatusActive,
		EncryptedAuthorizerRefreshToken: "encrypted-refresh",
	})
	require.NoError(t, err)
	client := &fakeTokenAuthorizerClient{
		token: authorization.AuthorizerToken{AccessToken: "authorizer-token", ExpiresInSeconds: 7200},
	}
	cache := newFakeAuthorizerAccessTokenCache()
	cache.set("tenant-1:1:wx-component", authorization.CachedAuthorizerAccessToken{
		AccountID: account.ID, AppID: "wx-authorizer", AccessToken: "shared-authorizer-token", ExpiresAt: now.Add(time.Hour),
	})
	service := NewTokenServiceWithLockerAndCache(store, client, &fakeRefreshTokenCodec{plaintext: "refresh-token"}, nil, cache, func() time.Time { return now })

	token, err := service.GetAuthorizerAccessToken(ctx, RefreshAuthorizerAccessTokenInput{
		TenantID: "tenant-1", AccountID: account.ID, ComponentAppID: "wx-component",
	})
	require.NoError(t, err)

	require.Equal(t, "shared-authorizer-token", token.AccessToken)
	require.Equal(t, int32(0), atomic.LoadInt32(&client.calls))
}

func TestTokenServiceStoresSharedAuthorizerAccessToken(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 6, 20, 0, 0, 0, time.UTC)
	store := memory.NewStore(func() time.Time { return now })
	account, err := store.SaveAccount(ctx, "tenant-1", authorization.Account{
		AppID: "wx-authorizer", Status: authorization.AccountStatusActive,
		EncryptedAuthorizerRefreshToken: "encrypted-refresh",
	})
	require.NoError(t, err)
	cache := newFakeAuthorizerAccessTokenCache()
	service := NewTokenServiceWithLockerAndCache(store, &fakeTokenAuthorizerClient{
		token: authorization.AuthorizerToken{AccessToken: "authorizer-token", ExpiresInSeconds: 7200},
	}, &fakeRefreshTokenCodec{plaintext: "refresh-token"}, nil, cache, func() time.Time { return now })

	_, err = service.RefreshAuthorizerAccessToken(ctx, RefreshAuthorizerAccessTokenInput{
		TenantID: "tenant-1", AccountID: account.ID, ComponentAppID: "wx-component",
	})
	require.NoError(t, err)
	cached, ok, err := cache.GetAuthorizerAccessToken(ctx, "tenant-1:1:wx-component")
	require.NoError(t, err)

	require.True(t, ok)
	require.Equal(t, "authorizer-token", cached.AccessToken)
	require.Equal(t, now.Add(2*time.Hour), cached.ExpiresAt)
}

func TestTokenServiceSchedulesNextRefreshAfterSuccessfulRefresh(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 6, 20, 0, 0, 0, time.UTC)
	store := memory.NewStore(func() time.Time { return now })
	account, err := store.SaveAccount(ctx, "tenant-1", authorization.Account{
		AppID: "wx-authorizer", Status: authorization.AccountStatusActive,
		EncryptedAuthorizerRefreshToken: "encrypted-refresh",
	})
	require.NoError(t, err)
	scheduler := &fakeTokenRefreshScheduler{}
	service := NewTokenServiceWithLockerCacheAndScheduler(store, &fakeTokenAuthorizerClient{
		token: authorization.AuthorizerToken{AccessToken: "authorizer-token", ExpiresInSeconds: 7200},
	}, &fakeRefreshTokenCodec{plaintext: "refresh-token"}, nil, nil, scheduler, func() time.Time { return now })

	_, err = service.RefreshAuthorizerAccessToken(ctx, RefreshAuthorizerAccessTokenInput{
		TenantID: "tenant-1", AccountID: account.ID, ComponentAppID: "wx-component",
	})
	require.NoError(t, err)

	require.Equal(t, authorization.TokenRefreshTask{
		TenantID: "tenant-1", AccountID: account.ID, ComponentAppID: "wx-component",
	}, scheduler.lastTask)
	require.Equal(t, 115*time.Minute, scheduler.lastDelay)
}

func TestTokenServiceGetsAuthorizerTokenStatus(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 6, 20, 0, 0, 0, time.UTC)
	store := memory.NewStore(func() time.Time { return now })
	account, err := store.SaveAccount(ctx, "tenant-1", authorization.Account{
		AppID: "wx-authorizer", Status: authorization.AccountStatusActive,
		EncryptedAuthorizerRefreshToken: "encrypted-refresh",
	})
	require.NoError(t, err)
	service := NewTokenService(store, &fakeTokenAuthorizerClient{
		token: authorization.AuthorizerToken{AccessToken: "authorizer-token", ExpiresInSeconds: 7200},
	}, &fakeRefreshTokenCodec{plaintext: "refresh-token"}, func() time.Time { return now })

	missing, err := service.GetAuthorizerTokenStatus(ctx, RefreshAuthorizerAccessTokenInput{
		TenantID: "tenant-1", AccountID: account.ID, ComponentAppID: "wx-component",
	})
	require.NoError(t, err)
	require.False(t, missing.Cached)
	require.True(t, missing.NeedsRefresh)

	_, err = service.GetAuthorizerAccessToken(ctx, RefreshAuthorizerAccessTokenInput{
		TenantID: "tenant-1", AccountID: account.ID, ComponentAppID: "wx-component",
	})
	require.NoError(t, err)
	cached, err := service.GetAuthorizerTokenStatus(ctx, RefreshAuthorizerAccessTokenInput{
		TenantID: "tenant-1", AccountID: account.ID, ComponentAppID: "wx-component",
	})
	require.NoError(t, err)

	require.True(t, cached.Cached)
	require.False(t, cached.NeedsRefresh)
	require.Equal(t, account.ID, cached.AccountID)
	require.Equal(t, "wx-authorizer", cached.AppID)
	require.Equal(t, authorization.AccountStatusActive, cached.AccountStatus)
	require.Equal(t, int64(7200), cached.ExpiresInSeconds)
	require.Equal(t, now.Add(2*time.Hour), cached.ExpiresAt)
}

func TestTokenServiceMarksAccountRefreshFailed(t *testing.T) {
	ctx := context.Background()
	store := memory.NewStore(time.Now)
	account, err := store.SaveAccount(ctx, "tenant-1", authorization.Account{
		AppID: "wx-authorizer", Status: authorization.AccountStatusActive,
		EncryptedAuthorizerRefreshToken: "encrypted-refresh",
	})
	require.NoError(t, err)
	service := NewTokenService(store, &fakeTokenAuthorizerClient{err: authorization.ErrAuthorizerClientUnavailable}, &fakeRefreshTokenCodec{plaintext: "refresh-token"}, time.Now)

	_, err = service.RefreshAuthorizerAccessToken(ctx, RefreshAuthorizerAccessTokenInput{
		TenantID: "tenant-1", AccountID: account.ID, ComponentAppID: "wx-component",
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrNotImplemented))

	updated, err := store.GetAccount(ctx, "tenant-1", account.ID)
	require.NoError(t, err)
	require.Equal(t, authorization.AccountStatusRefreshFailed, updated.Status)
}

func TestTokenServiceUsesDistributedRefreshLock(t *testing.T) {
	ctx := context.Background()
	store := memory.NewStore(time.Now)
	account, err := store.SaveAccount(ctx, "tenant-1", authorization.Account{
		AppID: "wx-authorizer", Status: authorization.AccountStatusActive,
		EncryptedAuthorizerRefreshToken: "encrypted-refresh",
	})
	require.NoError(t, err)
	locker := &fakeTokenRefreshLocker{}
	service := NewTokenServiceWithLocker(store, &fakeTokenAuthorizerClient{
		token: authorization.AuthorizerToken{AccessToken: "authorizer-token", ExpiresInSeconds: 7200},
	}, &fakeRefreshTokenCodec{plaintext: "refresh-token"}, locker, time.Now)

	_, err = service.RefreshAuthorizerAccessToken(ctx, RefreshAuthorizerAccessTokenInput{
		TenantID: "tenant-1", AccountID: account.ID, ComponentAppID: "wx-component",
	})
	require.NoError(t, err)

	require.Equal(t, "tenant-1:1:wx-component", locker.lastKey)
	require.Equal(t, 30*time.Second, locker.lastTTL)
	require.Equal(t, int32(1), atomic.LoadInt32(&locker.acquires))
	require.Equal(t, int32(1), atomic.LoadInt32(&locker.releases))
}

func TestTokenServiceDoesNotMarkRefreshFailedWhenLockUnavailable(t *testing.T) {
	ctx := context.Background()
	store := memory.NewStore(time.Now)
	account, err := store.SaveAccount(ctx, "tenant-1", authorization.Account{
		AppID: "wx-authorizer", Status: authorization.AccountStatusActive,
		EncryptedAuthorizerRefreshToken: "encrypted-refresh",
	})
	require.NoError(t, err)
	client := &fakeTokenAuthorizerClient{
		token: authorization.AuthorizerToken{AccessToken: "authorizer-token", ExpiresInSeconds: 7200},
	}
	service := NewTokenServiceWithLocker(store, client, &fakeRefreshTokenCodec{plaintext: "refresh-token"}, &fakeTokenRefreshLocker{err: authorization.ErrTokenRefreshLockUnavailable}, time.Now)

	_, err = service.RefreshAuthorizerAccessToken(ctx, RefreshAuthorizerAccessTokenInput{
		TenantID: "tenant-1", AccountID: account.ID, ComponentAppID: "wx-component",
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrNotImplemented))

	updated, err := store.GetAccount(ctx, "tenant-1", account.ID)
	require.NoError(t, err)
	require.Equal(t, authorization.AccountStatusActive, updated.Status)
	require.Equal(t, int32(0), atomic.LoadInt32(&client.calls))
}

func TestTokenServiceValidatesAccountBeforeReturningSharedTokenDuringRefresh(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 6, 20, 0, 0, 0, time.UTC)
	store := memory.NewStore(func() time.Time { return now })
	account, err := store.SaveAccount(ctx, "tenant-1", authorization.Account{
		AppID: "wx-authorizer", Status: authorization.AccountStatusRevoked,
		EncryptedAuthorizerRefreshToken: "encrypted-refresh",
	})
	require.NoError(t, err)
	client := &fakeTokenAuthorizerClient{
		token: authorization.AuthorizerToken{AccessToken: "authorizer-token", ExpiresInSeconds: 7200},
	}
	cache := newFakeAuthorizerAccessTokenCache()
	cache.set("tenant-1:1:wx-component", authorization.CachedAuthorizerAccessToken{
		AccountID: account.ID, AppID: "wx-authorizer", AccessToken: "shared-authorizer-token", ExpiresAt: now.Add(time.Hour),
	})
	service := NewTokenServiceWithLockerAndCache(store, client, &fakeRefreshTokenCodec{plaintext: "refresh-token"}, &fakeTokenRefreshLocker{}, cache, func() time.Time { return now })

	_, err = service.RefreshAuthorizerAccessToken(ctx, RefreshAuthorizerAccessTokenInput{
		TenantID: "tenant-1", AccountID: account.ID, ComponentAppID: "wx-component",
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))
	require.Equal(t, int32(0), atomic.LoadInt32(&client.calls))
}

func TestTokenServiceDeduplicatesConcurrentRefreshes(t *testing.T) {
	ctx := context.Background()
	store := memory.NewStore(time.Now)
	account, err := store.SaveAccount(ctx, "tenant-1", authorization.Account{
		AppID: "wx-authorizer", Status: authorization.AccountStatusActive,
		EncryptedAuthorizerRefreshToken: "encrypted-refresh",
	})
	require.NoError(t, err)
	client := &fakeTokenAuthorizerClient{
		token: authorization.AuthorizerToken{AccessToken: "authorizer-token", ExpiresInSeconds: 7200},
		wait:  make(chan struct{}),
	}
	service := NewTokenService(store, client, &fakeRefreshTokenCodec{plaintext: "refresh-token"}, time.Now)

	const workers = 20
	var ready sync.WaitGroup
	var wg sync.WaitGroup
	start := make(chan struct{})
	results := make(chan error, workers)
	for i := 0; i < workers; i++ {
		ready.Add(1)
		wg.Add(1)
		go func() {
			defer wg.Done()
			ready.Done()
			<-start
			_, err := service.RefreshAuthorizerAccessToken(ctx, RefreshAuthorizerAccessTokenInput{
				TenantID: "tenant-1", AccountID: account.ID, ComponentAppID: "wx-component",
			})
			results <- err
		}()
	}
	ready.Wait()
	close(start)
	for atomic.LoadInt32(&client.calls) == 0 {
		time.Sleep(time.Millisecond)
	}
	close(client.wait)
	wg.Wait()
	close(results)
	for err := range results {
		require.NoError(t, err)
	}
	require.Equal(t, int32(1), atomic.LoadInt32(&client.calls))
}
