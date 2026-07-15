package memory

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"official-account-service/internal/domain/identity"
)

func TestAgentStoreKeepsUserAgentTokensIndependent(t *testing.T) {
	now := time.Date(2026, 7, 15, 14, 0, 0, 0, time.UTC)
	store := NewStore(func() time.Time { return now })
	ctx := context.Background()
	for _, userID := range []string{"user-1", "user-2"} {
		_, err := store.SaveUser(ctx, identity.User{
			ID: userID, Username: userID, Role: identity.RoleUser, Status: identity.StatusActive,
		})
		require.NoError(t, err)
	}

	hashA := strings.Repeat("a", 64)
	hashB := strings.Repeat("b", 64)
	agentA, err := store.CreateAgent(ctx, identity.Agent{
		ID: "agent-record-a", UserID: "user-1", AgentID: "writer-a", Name: "Writer A",
		Status: identity.StatusActive, APITokenHash: hashA, APITokenHint: "oat_...aaaaaa",
	})
	require.NoError(t, err)
	agentB, err := store.CreateAgent(ctx, identity.Agent{
		ID: "agent-record-b", UserID: "user-1", AgentID: "writer-b", Name: "Writer B",
		Status: identity.StatusActive, APITokenHash: hashB, APITokenHint: "oat_...bbbbbb",
	})
	require.NoError(t, err)

	items, err := store.ListAgents(ctx, "user-1")
	require.NoError(t, err)
	require.Len(t, items, 2)
	byTokenA, err := store.GetAgentByAPITokenHash(ctx, hashA)
	require.NoError(t, err)
	require.Equal(t, agentA.ID, byTokenA.ID)
	byTokenB, err := store.GetAgentByAPITokenHash(ctx, hashB)
	require.NoError(t, err)
	require.Equal(t, agentB.ID, byTokenB.ID)

	_, err = store.GetAgent(ctx, "user-2", agentA.ID)
	require.ErrorIs(t, err, identity.ErrNotFound)

	rotatedHashB := strings.Repeat("c", 64)
	rotatedB, err := store.SaveAgentAPIToken(ctx, "user-1", agentB.ID, rotatedHashB, "oat_...cccccc")
	require.NoError(t, err)
	require.Equal(t, rotatedHashB, rotatedB.APITokenHash)
	_, err = store.GetAgentByAPITokenHash(ctx, hashB)
	require.ErrorIs(t, err, identity.ErrNotFound)
	stillActiveA, err := store.GetAgentByAPITokenHash(ctx, hashA)
	require.NoError(t, err)
	require.Equal(t, agentA.ID, stillActiveA.ID)

	now = now.Add(time.Minute)
	usedA, err := store.MarkAgentUsed(ctx, "user-1", agentA.ID, now)
	require.NoError(t, err)
	require.NotNil(t, usedA.LastUsedAt)
	require.Equal(t, now, *usedA.LastUsedAt)
	usedA, err = store.MarkAgentUsed(ctx, "user-1", agentA.ID, now.Add(-time.Minute))
	require.NoError(t, err)
	require.Equal(t, now, *usedA.LastUsedAt)

	_, err = store.CreateAgent(ctx, identity.Agent{
		ID: "agent-record-duplicate", UserID: "user-1", AgentID: agentA.AgentID, Name: "Duplicate",
		Status: identity.StatusActive,
	})
	require.ErrorIs(t, err, identity.ErrConflict)
	_, err = store.CreateAgent(ctx, identity.Agent{
		ID: "agent-record-duplicate-token", UserID: "user-2", AgentID: "writer-c", Name: "Writer C",
		Status: identity.StatusActive, APITokenHash: hashA, APITokenHint: "oat_...aaaaaa",
	})
	require.ErrorIs(t, err, identity.ErrConflict)
	_, err = store.CreateAgent(ctx, identity.Agent{
		ID: "agent-record-missing-user", UserID: "missing", AgentID: "writer", Name: "Missing",
		Status: identity.StatusActive,
	})
	require.ErrorIs(t, err, identity.ErrNotFound)

	revokedB, err := store.RevokeAgentAPIToken(ctx, "user-1", agentB.ID)
	require.NoError(t, err)
	require.Empty(t, revokedB.APITokenHash)
	require.Nil(t, revokedB.APITokenCreatedAt)
	_, err = store.GetAgentByAPITokenHash(ctx, hashA)
	require.NoError(t, err)

	_, err = store.GetAgentByAPITokenHash(ctx, "")
	require.True(t, errors.Is(err, identity.ErrNotFound))
}

func TestAgentStoreRejectsConcurrentDuplicateAgentID(t *testing.T) {
	store := NewStore(time.Now)
	ctx := context.Background()
	_, err := store.SaveUser(ctx, identity.User{
		ID: "user-1", Username: "user-1", Role: identity.RoleUser, Status: identity.StatusActive,
	})
	require.NoError(t, err)

	const attempts = 12
	errs := make(chan error, attempts)
	var wg sync.WaitGroup
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			_, createErr := store.CreateAgent(ctx, identity.Agent{
				ID: fmt.Sprintf("agent-record-%d", index), UserID: "user-1", AgentID: "shared-agent",
				Name: "Shared Agent", Status: identity.StatusActive,
			})
			errs <- createErr
		}(i)
	}
	wg.Wait()
	close(errs)

	successes := 0
	conflicts := 0
	for createErr := range errs {
		if createErr == nil {
			successes++
			continue
		}
		require.ErrorIs(t, createErr, identity.ErrConflict)
		conflicts++
	}
	require.Equal(t, 1, successes)
	require.Equal(t, attempts-1, conflicts)
}
