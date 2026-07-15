package memory

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"official-account-service/internal/domain/identity"
)

func TestStorePersistsUsersByStableDataSpaceID(t *testing.T) {
	store := NewStore(func() time.Time { return time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC) })
	ctx := context.Background()

	created, err := store.SaveUser(ctx, identity.User{
		ID: "user-1", Username: "Alice", PasswordHash: "hash", Role: identity.RoleUser, Status: identity.StatusActive,
	})
	require.NoError(t, err)
	require.Equal(t, "user-1", created.ID)
	require.False(t, created.CreatedAt.IsZero())

	byID, err := store.GetUser(ctx, "user-1")
	require.NoError(t, err)
	require.Equal(t, "Alice", byID.Username)

	byUsername, err := store.GetUserByUsername(ctx, "alice")
	require.NoError(t, err)
	require.Equal(t, "user-1", byUsername.ID)

	users, err := store.ListUsers(ctx)
	require.NoError(t, err)
	require.Len(t, users, 1)
}

func TestStoreRejectsUsernameOwnedByAnotherUser(t *testing.T) {
	store := NewStore(time.Now)
	ctx := context.Background()
	_, err := store.SaveUser(ctx, identity.User{ID: "user-1", Username: "alice", Role: identity.RoleUser, Status: identity.StatusActive})
	require.NoError(t, err)

	_, err = store.SaveUser(ctx, identity.User{ID: "user-2", Username: "ALICE", Role: identity.RoleUser, Status: identity.StatusActive})

	require.Error(t, err)
	require.True(t, errors.Is(err, identity.ErrConflict))
}
