package application

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"official-account-service/internal/domain/authorization"
	"official-account-service/internal/infra/persistence/memory"
)

func TestTokenRefreshTaskServiceHandlesRefreshTask(t *testing.T) {
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
	scheduler := &fakeTokenRefreshScheduler{}
	tokens := NewTokenServiceWithLockerCacheAndScheduler(store, client, &fakeRefreshTokenCodec{plaintext: "refresh-token"}, nil, nil, scheduler, func() time.Time { return now })
	service := NewTokenRefreshTaskService(tokens)

	err = service.HandleTokenRefreshTask(ctx, authorization.TokenRefreshTask{
		TenantID: "tenant-1", AccountID: account.ID, ComponentAppID: "wx-component",
	})
	require.NoError(t, err)

	require.Equal(t, int32(1), atomic.LoadInt32(&client.calls))
	require.Equal(t, authorization.TokenRefreshTask{
		TenantID: "tenant-1", AccountID: account.ID, ComponentAppID: "wx-component",
	}, scheduler.lastTask)
}

func TestTokenRefreshTaskServiceValidatesInput(t *testing.T) {
	service := NewTokenRefreshTaskService(NewTokenService(memory.NewStore(time.Now), &fakeTokenAuthorizerClient{}, &fakeRefreshTokenCodec{}, time.Now))

	err := service.HandleTokenRefreshTask(context.Background(), authorization.TokenRefreshTask{TenantID: "", AccountID: 1, ComponentAppID: "wx-component"})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))

	err = service.HandleTokenRefreshTask(context.Background(), authorization.TokenRefreshTask{TenantID: "tenant-1", AccountID: 0, ComponentAppID: "wx-component"})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))

	err = service.HandleTokenRefreshTask(context.Background(), authorization.TokenRefreshTask{TenantID: "tenant-1", AccountID: 1, ComponentAppID: ""})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))
}
