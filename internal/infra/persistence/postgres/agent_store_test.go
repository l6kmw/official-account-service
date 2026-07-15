package postgres

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"official-account-service/internal/domain/identity"
)

func TestAgentStoreKeepsUserAgentTokensIndependent(t *testing.T) {
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("set POSTGRES_TEST_DSN to run postgres integration test")
	}
	ctx := context.Background()
	store, err := Open(ctx, dsn)
	require.NoError(t, err)
	defer func() { require.NoError(t, store.Close()) }()
	runMigrations(t, store)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	userA := "agent-user-a-" + suffix
	userB := "agent-user-b-" + suffix
	for _, userID := range []string{userA, userB} {
		_, err = store.SaveUser(ctx, identity.User{
			ID: userID, Username: userID, PasswordHash: "hash", Role: identity.RoleUser, Status: identity.StatusActive,
		})
		require.NoError(t, err)
	}

	hashA := fmt.Sprintf("%064x", time.Now().UnixNano())
	hashB := fmt.Sprintf("%064x", time.Now().UnixNano()+1)
	agentA, err := store.CreateAgent(ctx, identity.Agent{
		ID: "agent-record-a-" + suffix, UserID: userA, AgentID: "writer-a", Name: "Writer A",
		Status: identity.StatusActive, APITokenHash: hashA, APITokenHint: "oat_...aaaaaa",
	})
	require.NoError(t, err)
	agentB, err := store.CreateAgent(ctx, identity.Agent{
		ID: "agent-record-b-" + suffix, UserID: userA, AgentID: "writer-b", Name: "Writer B",
		Status: identity.StatusActive, APITokenHash: hashB, APITokenHint: "oat_...bbbbbb",
	})
	require.NoError(t, err)

	items, err := store.ListAgents(ctx, userA)
	require.NoError(t, err)
	require.Len(t, items, 2)
	byTokenA, err := store.GetAgentByAPITokenHash(ctx, hashA)
	require.NoError(t, err)
	require.Equal(t, agentA.ID, byTokenA.ID)
	_, err = store.GetAgent(ctx, userB, agentA.ID)
	require.ErrorIs(t, err, identity.ErrNotFound)

	rotatedHashB := fmt.Sprintf("%064x", time.Now().UnixNano()+2)
	_, err = store.SaveAgentAPIToken(ctx, userA, agentB.ID, rotatedHashB, "oat_...cccccc")
	require.NoError(t, err)
	_, err = store.GetAgentByAPITokenHash(ctx, hashB)
	require.ErrorIs(t, err, identity.ErrNotFound)
	stillActiveA, err := store.GetAgentByAPITokenHash(ctx, hashA)
	require.NoError(t, err)
	require.Equal(t, agentA.ID, stillActiveA.ID)

	usedAt := time.Now().UTC().Truncate(time.Microsecond)
	usedA, err := store.MarkAgentUsed(ctx, userA, agentA.ID, usedAt)
	require.NoError(t, err)
	require.NotNil(t, usedA.LastUsedAt)
	require.WithinDuration(t, usedAt, *usedA.LastUsedAt, time.Microsecond)
	usedA, err = store.MarkAgentUsed(ctx, userA, agentA.ID, usedAt.Add(-time.Minute))
	require.NoError(t, err)
	require.WithinDuration(t, usedAt, *usedA.LastUsedAt, time.Microsecond)

	_, err = store.CreateAgent(ctx, identity.Agent{
		ID: "agent-record-duplicate-" + suffix, UserID: userA, AgentID: agentA.AgentID, Name: "Duplicate",
		Status: identity.StatusActive,
	})
	require.ErrorIs(t, err, identity.ErrConflict)
	_, err = store.CreateAgent(ctx, identity.Agent{
		ID: "agent-record-duplicate-token-" + suffix, UserID: userB, AgentID: "writer-c", Name: "Writer C",
		Status: identity.StatusActive, APITokenHash: hashA, APITokenHint: "oat_...aaaaaa",
	})
	require.ErrorIs(t, err, identity.ErrConflict)
	_, err = store.CreateAgent(ctx, identity.Agent{
		ID: "agent-record-missing-" + suffix, UserID: "missing-" + suffix, AgentID: "writer", Name: "Missing",
		Status: identity.StatusActive,
	})
	require.ErrorIs(t, err, identity.ErrNotFound)

	revokedB, err := store.RevokeAgentAPIToken(ctx, userA, agentB.ID)
	require.NoError(t, err)
	require.Empty(t, revokedB.APITokenHash)
	require.Nil(t, revokedB.APITokenCreatedAt)
}

func TestMultiAgentMigrationBackfillsLegacyUserToken(t *testing.T) {
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("set POSTGRES_TEST_DSN to run postgres integration test")
	}
	ctx := context.Background()
	store, err := Open(ctx, dsn)
	require.NoError(t, err)
	defer func() { require.NoError(t, store.Close()) }()
	runMigrations(t, store)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	userID := "legacy-agent-user-" + suffix
	_, err = store.SaveUser(ctx, identity.User{
		ID: userID, Username: userID, PasswordHash: "hash", Role: identity.RoleUser, Status: identity.StatusActive,
	})
	require.NoError(t, err)
	legacyHash := strings.Repeat("d", 48) + fmt.Sprintf("%016x", time.Now().UnixNano())
	_, err = store.SaveUserAPIToken(ctx, userID, legacyHash, "oat_...legacy")
	require.NoError(t, err)

	migration, err := os.ReadFile("../../../../migrations/013_multi_agent_token.sql")
	require.NoError(t, err)
	_, err = store.db.Exec(string(migration))
	require.NoError(t, err)

	backfilled, err := store.GetAgentByAPITokenHash(ctx, legacyHash)
	require.NoError(t, err)
	require.Equal(t, userID, backfilled.UserID)
	require.Equal(t, "default", backfilled.AgentID)
	require.Equal(t, "Default Agent", backfilled.Name)
	require.Equal(t, identity.StatusActive, backfilled.Status)
	legacyUser, err := store.GetUserByAPITokenHash(ctx, legacyHash)
	require.NoError(t, err)
	require.Equal(t, userID, legacyUser.ID)

	_, err = store.db.Exec(string(migration))
	require.NoError(t, err)
	agents, err := store.ListAgents(ctx, userID)
	require.NoError(t, err)
	require.Len(t, agents, 1)
}

func TestAgentStoreRejectsConcurrentDuplicateAgentID(t *testing.T) {
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("set POSTGRES_TEST_DSN to run postgres integration test")
	}
	ctx := context.Background()
	store, err := Open(ctx, dsn)
	require.NoError(t, err)
	defer func() { require.NoError(t, store.Close()) }()
	runMigrations(t, store)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	userID := "concurrent-agent-user-" + suffix
	_, err = store.SaveUser(ctx, identity.User{
		ID: userID, Username: userID, PasswordHash: "hash", Role: identity.RoleUser, Status: identity.StatusActive,
	})
	require.NoError(t, err)

	const attempts = 8
	errs := make(chan error, attempts)
	var wg sync.WaitGroup
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			_, createErr := store.CreateAgent(ctx, identity.Agent{
				ID: fmt.Sprintf("concurrent-agent-%s-%d", suffix, index), UserID: userID, AgentID: "shared-agent",
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
