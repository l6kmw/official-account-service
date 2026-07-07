package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"official-account-service/internal/domain/authorization"
)

func TestAuthorizerAccessTokenCacheStoresToken(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 6, 20, 0, 0, 0, time.UTC)
	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	cache := NewAuthorizerAccessTokenCache(client, func() time.Time { return now })

	err := cache.SetAuthorizerAccessToken(ctx, "tenant-1:1:wx-component", authorization.CachedAuthorizerAccessToken{
		AccountID: 1, AppID: "wx-authorizer", AccessToken: "authorizer-token", ExpiresAt: now.Add(time.Hour),
	})
	require.NoError(t, err)
	token, ok, err := cache.GetAuthorizerAccessToken(ctx, "tenant-1:1:wx-component")
	require.NoError(t, err)

	require.True(t, ok)
	require.Equal(t, int64(1), token.AccountID)
	require.Equal(t, "wx-authorizer", token.AppID)
	require.Equal(t, "authorizer-token", token.AccessToken)
	require.Equal(t, now.Add(time.Hour), token.ExpiresAt)
}

func TestAuthorizerAccessTokenCacheMiss(t *testing.T) {
	ctx := context.Background()
	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	cache := NewAuthorizerAccessTokenCache(client, time.Now)

	_, ok, err := cache.GetAuthorizerAccessToken(ctx, "missing")
	require.NoError(t, err)
	require.False(t, ok)
}

func TestAuthorizerAccessTokenCacheValidatesInput(t *testing.T) {
	cache := NewAuthorizerAccessTokenCache(nil, time.Now)
	_, _, err := cache.GetAuthorizerAccessToken(context.Background(), "key")
	require.Error(t, err)

	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	cache = NewAuthorizerAccessTokenCache(client, time.Now)
	err = cache.SetAuthorizerAccessToken(context.Background(), "key", authorization.CachedAuthorizerAccessToken{})
	require.Error(t, err)
}
