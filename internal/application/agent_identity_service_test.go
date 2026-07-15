package application

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"official-account-service/internal/domain/identity"
	"official-account-service/internal/infra/persistence/memory"
)

func TestIdentityServiceAuthenticatesIndependentAgentTokens(t *testing.T) {
	store := memory.NewStore(nil)
	ctx := context.Background()
	for _, userID := range []string{"user-1", "user-2"} {
		_, err := store.SaveUser(ctx, identity.User{
			ID: userID, Username: userID, PasswordHash: "hash", Role: identity.RoleUser, Status: identity.StatusActive,
		})
		require.NoError(t, err)
	}
	service := NewIdentityService(store)
	ids := []string{"agent-record-a", "agent-record-b"}
	service.newAgentID = func() (string, error) {
		id := ids[0]
		ids = ids[1:]
		return id, nil
	}

	agentA, err := service.CreateAgent(ctx, CreateAgentInput{
		UserID: "user-1", AgentID: "writer-a", Name: "Writer A", Purpose: "industry",
	})
	require.NoError(t, err)
	agentB, err := service.CreateAgent(ctx, CreateAgentInput{
		UserID: "user-1", AgentID: "writer-b", Name: "Writer B", Purpose: "product",
	})
	require.NoError(t, err)
	require.NotEqual(t, agentA.Token, agentB.Token)

	principalA, err := service.AuthenticatePrincipal(ctx, agentA.Token)
	require.NoError(t, err)
	require.Equal(t, "user-1", principalA.User.ID)
	require.Equal(t, ActorTypeAgentToken, principalA.ActorType)
	require.NotNil(t, principalA.Agent)
	require.Equal(t, "writer-a", principalA.Agent.AgentID)
	require.NotNil(t, principalA.Agent.LastUsedAt)
	principalB, err := service.AuthenticatePrincipal(ctx, agentB.Token)
	require.NoError(t, err)
	require.Equal(t, "writer-b", principalB.Agent.AgentID)

	items, err := service.ListAgents(ctx, "user-1")
	require.NoError(t, err)
	require.Len(t, items, 2)
	_, err = service.GetAgent(ctx, "user-2", agentA.Agent.ID)
	require.ErrorIs(t, err, ErrNotFound)

	rotatedB, err := service.RotateAgentAPIToken(ctx, "user-1", agentB.Agent.ID)
	require.NoError(t, err)
	require.NotEqual(t, agentB.Token, rotatedB.Token)
	_, err = service.AuthenticatePrincipal(ctx, agentB.Token)
	require.ErrorIs(t, err, ErrInvalidCredentials)
	_, err = service.AuthenticatePrincipal(ctx, agentA.Token)
	require.NoError(t, err)
	_, err = service.AuthenticatePrincipal(ctx, rotatedB.Token)
	require.NoError(t, err)

	_, err = service.RevokeAgentAPIToken(ctx, "user-1", agentB.Agent.ID)
	require.NoError(t, err)
	_, err = service.AuthenticatePrincipal(ctx, rotatedB.Token)
	require.ErrorIs(t, err, ErrInvalidCredentials)
	_, err = service.AuthenticatePrincipal(ctx, agentA.Token)
	require.NoError(t, err)
}

func TestIdentityServiceLegacyTokenMovesToDefaultAgentWithoutFallbackBypass(t *testing.T) {
	store := memory.NewStore(nil)
	ctx := context.Background()
	_, err := store.SaveUser(ctx, identity.User{
		ID: "user-1", Username: "writer", PasswordHash: "hash", Role: identity.RoleUser, Status: identity.StatusActive,
	})
	require.NoError(t, err)
	legacyToken := "oat_" + strings.Repeat("l", 43)
	_, err = store.SaveUserAPIToken(ctx, "user-1", apiTokenHash(legacyToken), apiTokenHint(legacyToken))
	require.NoError(t, err)
	service := NewIdentityService(store)
	service.newAgentID = func() (string, error) { return "default-agent-record", nil }

	legacyPrincipal, err := service.AuthenticatePrincipal(ctx, legacyToken)
	require.NoError(t, err)
	require.Equal(t, ActorTypeLegacyUserToken, legacyPrincipal.ActorType)
	require.Nil(t, legacyPrincipal.Agent)

	generated, err := service.GenerateAPIToken(ctx, "user-1")
	require.NoError(t, err)
	require.NotNil(t, generated.Agent)
	require.Equal(t, "default", generated.Agent.AgentID)
	_, err = service.AuthenticatePrincipal(ctx, legacyToken)
	require.ErrorIs(t, err, ErrInvalidCredentials)
	defaultPrincipal, err := service.AuthenticatePrincipal(ctx, generated.Token)
	require.NoError(t, err)
	require.Equal(t, ActorTypeAgentToken, defaultPrincipal.ActorType)
	require.Equal(t, "default", defaultPrincipal.Agent.AgentID)

	users, err := service.ListUsers(ctx)
	require.NoError(t, err)
	require.Len(t, users, 1)
	require.Equal(t, generated.Agent.APITokenHash, users[0].APITokenHash)

	_, err = service.RevokeAPIToken(ctx, "user-1")
	require.NoError(t, err)
	_, err = service.AuthenticatePrincipal(ctx, generated.Token)
	require.ErrorIs(t, err, ErrInvalidCredentials)
	_, err = service.AuthenticatePrincipal(ctx, legacyToken)
	require.ErrorIs(t, err, ErrInvalidCredentials)
}

func TestIdentityServiceValidatesAgentOwnershipAndStatus(t *testing.T) {
	store := memory.NewStore(nil)
	ctx := context.Background()
	_, err := store.SaveUser(ctx, identity.User{
		ID: "user-1", Username: "writer", PasswordHash: "hash", Role: identity.RoleUser, Status: identity.StatusActive,
	})
	require.NoError(t, err)
	service := NewIdentityService(store)
	service.newAgentID = func() (string, error) { return "agent-record", nil }
	generated, err := service.CreateAgent(ctx, CreateAgentInput{UserID: "user-1", AgentID: "writer", Name: "Writer"})
	require.NoError(t, err)

	_, err = service.CreateAgent(ctx, CreateAgentInput{UserID: "user-1", AgentID: "writer", Name: "Duplicate"})
	require.ErrorIs(t, err, ErrConflict)
	_, err = service.UpdateAgent(ctx, UpdateAgentInput{
		UserID: "other-user", ID: generated.Agent.ID, Name: "Other", Status: identity.StatusActive,
	})
	require.ErrorIs(t, err, ErrNotFound)
	updated, err := service.UpdateAgent(ctx, UpdateAgentInput{
		UserID: "user-1", ID: generated.Agent.ID, Name: "Writer", Purpose: "news", Status: identity.StatusDisabled,
	})
	require.NoError(t, err)
	require.Equal(t, identity.StatusDisabled, updated.Status)
	_, err = service.AuthenticatePrincipal(ctx, generated.Token)
	require.ErrorIs(t, err, ErrInvalidCredentials)
	_, err = service.RotateAgentAPIToken(ctx, "user-1", generated.Agent.ID)
	require.ErrorIs(t, err, ErrConflict)

	_, err = service.CreateAgent(ctx, CreateAgentInput{UserID: "missing", AgentID: "writer", Name: "Writer"})
	require.ErrorIs(t, err, ErrNotFound)
	_, err = service.CreateAgent(ctx, CreateAgentInput{UserID: "user-1", AgentID: "", Name: ""})
	require.ErrorIs(t, err, ErrInvalidInput)
}
