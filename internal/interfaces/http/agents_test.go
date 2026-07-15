package http

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"official-account-service/internal/application"
	"official-account-service/internal/domain/authorization"
	"official-account-service/internal/domain/identity"
	"official-account-service/internal/infra/persistence/memory"
)

func TestAdminManagesIndependentAgentTokens(t *testing.T) {
	store := memory.NewStore(fixedRouteTime)
	ctx := context.Background()
	for _, userID := range []string{"user-1", "user-2"} {
		_, err := store.SaveUser(ctx, identity.User{
			ID: userID, Username: userID, PasswordHash: "hash", Role: identity.RoleUser, Status: identity.StatusActive,
		})
		require.NoError(t, err)
	}
	_, err := store.SaveUser(ctx, identity.User{
		ID: "tenant-1", Username: "admin", PasswordHash: "hash", Role: identity.RoleAdmin, Status: identity.StatusActive,
	})
	require.NoError(t, err)
	_, err = store.SaveAccount(ctx, "user-1", authorization.Account{AppID: "wx-user-1", Name: "User 1", Status: authorization.AccountStatusActive})
	require.NoError(t, err)
	_, err = store.SaveAccount(ctx, "user-2", authorization.Account{AppID: "wx-user-2", Name: "User 2", Status: authorization.AccountStatusActive})
	require.NoError(t, err)
	identities := application.NewIdentityService(store)
	router := NewRouter(Dependencies{
		Logger: zap.NewNop(), AdminAPIKey: "admin-key", AdminUserID: "tenant-1",
		Identity: identities, Accounts: application.NewAccountService(store), MCPToken: "global-mcp-token", MCPPath: "/mcp",
	})

	agentA := createAgentThroughAPI(t, router, "user-1", `{"agent_id":"writer-a","name":"Writer A","purpose":"industry"}`)
	agentB := createAgentThroughAPI(t, router, "user-1", `{"agent_id":"writer-b","name":"Writer B","purpose":"product"}`)
	require.NotEqual(t, agentA.Token, agentB.Token)

	listed := doJSONWithAdminKey(t, router, http.MethodGet, "/api/v1/admin/users/user-1/agents", ``, "forged-user", "admin-key")
	require.Equal(t, http.StatusOK, listed.Code)
	require.NotContains(t, listed.Body.String(), "api_token_hash")
	require.NotContains(t, listed.Body.String(), agentA.Token)
	var listBody struct {
		Items []agentResponse `json:"items"`
	}
	require.NoError(t, json.Unmarshal(listed.Body.Bytes(), &listBody))
	require.Len(t, listBody.Items, 2)

	spoofedSession := doJSONWithCookiesAndHeaders(t, router, http.MethodGet, "/api/v1/admin/session", ``, "user-2", nil, map[string]string{
		"Authorization": "Bearer " + agentA.Token,
		"X-Agent-ID":    "writer-b",
	})
	require.Equal(t, http.StatusOK, spoofedSession.Code)
	var session adminSessionResponse
	require.NoError(t, json.Unmarshal(spoofedSession.Body.Bytes(), &session))
	require.True(t, session.Authenticated)
	require.Equal(t, "user-1", session.UserID)
	require.Equal(t, "writer-a", session.AgentID)
	require.Equal(t, string(application.ActorTypeAgentToken), session.ActorType)

	accountsA := doJSONWithBearer(t, router, http.MethodGet, "/api/v1/accounts", ``, "user-2", agentA.Token)
	require.Equal(t, http.StatusOK, accountsA.Code)
	require.Contains(t, accountsA.Body.String(), "wx-user-1")
	require.NotContains(t, accountsA.Body.String(), "wx-user-2")

	forbidden := doJSONWithBearer(t, router, http.MethodPost, "/api/v1/admin/users/user-1/agents", `{"agent_id":"forbidden","name":"Forbidden"}`, "user-1", agentA.Token)
	require.Equal(t, http.StatusForbidden, forbidden.Code)
	wrongOwner := doJSONWithAdminKey(t, router, http.MethodPatch, "/api/v1/admin/users/user-2/agents/"+agentA.Agent.ID, `{"name":"Wrong","purpose":"","status":"active"}`, "", "admin-key")
	require.Equal(t, http.StatusNotFound, wrongOwner.Code)

	rotatedB := doJSONWithAdminKey(t, router, http.MethodPost, "/api/v1/admin/users/user-1/agents/"+agentB.Agent.ID+"/api-token", ``, "", "admin-key")
	require.Equal(t, http.StatusCreated, rotatedB.Code)
	var rotated generatedAgentAPITokenResponse
	require.NoError(t, json.Unmarshal(rotatedB.Body.Bytes(), &rotated))
	require.NotEqual(t, agentB.Token, rotated.Token)
	require.Equal(t, http.StatusUnauthorized, doJSONWithBearer(t, router, http.MethodGet, "/api/v1/accounts", ``, "", agentB.Token).Code)
	require.Equal(t, http.StatusOK, doJSONWithBearer(t, router, http.MethodGet, "/api/v1/accounts", ``, "", agentA.Token).Code)

	revokedB := doJSONWithAdminKey(t, router, http.MethodDelete, "/api/v1/admin/users/user-1/agents/"+agentB.Agent.ID+"/api-token", ``, "", "admin-key")
	require.Equal(t, http.StatusOK, revokedB.Code)
	require.Equal(t, http.StatusUnauthorized, doJSONWithBearer(t, router, http.MethodGet, "/api/v1/accounts", ``, "", rotated.Token).Code)
	require.Equal(t, http.StatusOK, doJSONWithBearer(t, router, http.MethodGet, "/api/v1/accounts", ``, "", agentA.Token).Code)

	disabledA := doJSONWithAdminKey(t, router, http.MethodPatch, "/api/v1/admin/users/user-1/agents/"+agentA.Agent.ID, `{"name":"Writer A","purpose":"industry","status":"disabled"}`, "", "admin-key")
	require.Equal(t, http.StatusOK, disabledA.Code)
	require.Equal(t, http.StatusUnauthorized, doJSONWithBearer(t, router, http.MethodGet, "/api/v1/accounts", ``, "", agentA.Token).Code)

	adminAgent := createAgentThroughAPI(t, router, "tenant-1", `{"agent_id":"admin-agent","name":"Admin Agent","purpose":"operations"}`)
	adminMCPConfig := doJSONWithBearer(t, router, http.MethodGet, "/api/v1/admin/mcp-config", ``, "", adminAgent.Token)
	require.Equal(t, http.StatusOK, adminMCPConfig.Code)
	var config mcpConfigResponse
	require.NoError(t, json.Unmarshal(adminMCPConfig.Body.Bytes(), &config))
	require.Empty(t, config.Token)
	require.False(t, config.Revealable)
	require.Equal(t, "admin-agent", config.AgentID)
	require.NotContains(t, adminMCPConfig.Body.String(), "global-mcp-token")
}

func createAgentThroughAPI(t *testing.T, router http.Handler, userID string, body string) generatedAgentAPITokenResponse {
	t.Helper()
	created := doJSONWithAdminKey(t, router, http.MethodPost, "/api/v1/admin/users/"+userID+"/agents", body, "forged-user", "admin-key")
	require.Equal(t, http.StatusCreated, created.Code)
	require.Equal(t, "no-store", created.Header().Get("Cache-Control"))
	require.Equal(t, "no-cache", created.Header().Get("Pragma"))
	require.NotContains(t, created.Body.String(), "api_token_hash")
	var response generatedAgentAPITokenResponse
	require.NoError(t, json.Unmarshal(created.Body.Bytes(), &response))
	require.Contains(t, response.Token, "oat_")
	require.NotEmpty(t, response.Agent.ID)
	return response
}
