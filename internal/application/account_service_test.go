package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"official-account-service/internal/domain/authorization"
	"official-account-service/internal/infra/persistence/memory"
)

func TestAccountServiceSaveListGetAndUpdateStatus(t *testing.T) {
	ctx := context.Background()
	store := memory.NewStore(func() time.Time { return time.Date(2026, 7, 6, 16, 0, 0, 0, time.UTC) })
	service := NewAccountService(store)

	created, err := service.SaveAccount(ctx, SaveAccountInput{TenantID: "tenant-1", AppID: "wx123", Name: "account", Status: authorization.AccountStatusActive})
	require.NoError(t, err)
	require.Equal(t, "account", created.Name)

	saved, err := service.SaveAccount(ctx, SaveAccountInput{TenantID: "tenant-1", AppID: "wx123", Name: "renamed", Status: authorization.AccountStatusActive})
	require.NoError(t, err)
	require.Equal(t, created.ID, saved.ID)
	require.Equal(t, "renamed", saved.Name)

	_, err = service.SaveAccount(ctx, SaveAccountInput{TenantID: "tenant-2", AppID: "wx456", Name: "other", Status: authorization.AccountStatusActive})
	require.NoError(t, err)

	items, err := service.ListAccounts(ctx, "tenant-1")
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "renamed", items[0].Name)

	got, err := service.GetAccount(ctx, "tenant-1", created.ID)
	require.NoError(t, err)
	require.Equal(t, "wx123", got.AppID)

	updated, err := service.UpdateAccountStatus(ctx, "tenant-1", created.ID, authorization.AccountStatusRevoked)
	require.NoError(t, err)
	require.Equal(t, authorization.AccountStatusRevoked, updated.Status)

	_, err = service.GetAccount(ctx, "tenant-2", created.ID)
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrNotFound))
}

func TestAccountServiceValidatesInput(t *testing.T) {
	ctx := context.Background()
	service := NewAccountService(memory.NewStore(time.Now))

	_, err := service.SaveAccount(ctx, SaveAccountInput{TenantID: "", AppID: "wx123", Status: authorization.AccountStatusActive})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))

	_, err = service.SaveAccount(ctx, SaveAccountInput{TenantID: "tenant", AppID: "", Status: authorization.AccountStatusActive})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))

	_, err = service.SaveAccount(ctx, SaveAccountInput{TenantID: "tenant", AppID: "wx123", Status: "bad"})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))

	_, err = service.ListAccounts(ctx, "")
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))

	_, err = service.GetAccount(ctx, "tenant", 0)
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))

	_, err = service.UpdateAccountStatus(ctx, "tenant", 1, "bad")
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))
}
