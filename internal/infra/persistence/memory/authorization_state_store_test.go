package memory

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"official-account-service/internal/domain/authorization"
)

func TestStoreConsumesAuthorizationStateOnce(t *testing.T) {
	now := time.Date(2026, 7, 14, 8, 0, 0, 0, time.UTC)
	store := NewStore(func() time.Time { return now })
	state, err := store.SaveAuthorizationState(context.Background(), authorization.AuthorizationState{
		Digest: "digest-1", TenantID: "tenant-1", ComponentAppID: "wx-component", ExpiresAt: now.Add(10 * time.Minute),
	})
	require.NoError(t, err)
	require.Equal(t, now, state.CreatedAt)

	consumed, err := store.ConsumeAuthorizationState(context.Background(), state.Digest, now.Add(time.Minute))
	require.NoError(t, err)
	require.Equal(t, "tenant-1", consumed.TenantID)
	require.Equal(t, now.Add(time.Minute), consumed.ConsumedAt)

	_, err = store.ConsumeAuthorizationState(context.Background(), state.Digest, now.Add(2*time.Minute))
	require.Error(t, err)
	require.True(t, errors.Is(err, authorization.ErrAuthorizationStateNotFound))
}

func TestStoreRejectsExpiredAuthorizationState(t *testing.T) {
	now := time.Date(2026, 7, 14, 8, 0, 0, 0, time.UTC)
	store := NewStore(func() time.Time { return now })
	_, err := store.SaveAuthorizationState(context.Background(), authorization.AuthorizationState{
		Digest: "digest-1", TenantID: "tenant-1", ComponentAppID: "wx-component", ExpiresAt: now,
	})
	require.NoError(t, err)

	_, err = store.ConsumeAuthorizationState(context.Background(), "digest-1", now)
	require.Error(t, err)
	require.True(t, errors.Is(err, authorization.ErrAuthorizationStateNotFound))
}
