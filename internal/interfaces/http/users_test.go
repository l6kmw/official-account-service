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
	"official-account-service/internal/infra/persistence/memory"
)

func TestAdminCreatesUserWithoutExposingPasswordHash(t *testing.T) {
	store := memory.NewStore(fixedRouteTime)
	identities := application.NewIdentityService(store)
	router := NewRouter(Dependencies{
		Logger: zap.NewNop(), AdminAPIKey: "admin-key", AdminUserID: "tenant-1",
		AdminSessionSecret: "test-session-secret", Identity: identities, Accounts: application.NewAccountService(store), MCPToken: "global-mcp-token",
	})

	created := doJSONWithAdminKey(t, router, http.MethodPost, "/api/v1/admin/users", `{"username":"writer","password":"strong-password-123"}`, "spoofed-user", "admin-key")

	require.Equal(t, http.StatusCreated, created.Code)
	require.NotContains(t, created.Body.String(), "password")
	require.NotContains(t, created.Body.String(), "hash")
	var user userResponse
	require.NoError(t, json.Unmarshal(created.Body.Bytes(), &user))
	require.NotEmpty(t, user.ID)
	require.Equal(t, "writer", user.Username)
	require.Equal(t, "user", user.Role)

	login := doJSON(t, router, http.MethodPost, "/api/v1/admin/session", `{"username":"writer","password":"strong-password-123"}`, "")
	require.Equal(t, http.StatusOK, login.Code)
	var session adminSessionResponse
	require.NoError(t, json.Unmarshal(login.Body.Bytes(), &session))
	require.Equal(t, user.ID, session.UserID)
	cookie := firstCookie(t, login, adminSessionCookieName)

	users := doJSONWithCookiesAndHeaders(t, router, http.MethodGet, "/api/v1/admin/users", ``, "tenant-1", []*http.Cookie{cookie}, nil)
	require.Equal(t, http.StatusForbidden, users.Code)

	generated := doJSONWithAdminKey(t, router, http.MethodPost, "/api/v1/admin/users/"+user.ID+"/api-token", ``, "spoofed-user", "admin-key")
	require.Equal(t, http.StatusCreated, generated.Code)
	require.NotContains(t, generated.Body.String(), "api_token_hash")
	var credential generatedAPITokenResponse
	require.NoError(t, json.Unmarshal(generated.Body.Bytes(), &credential))
	require.Contains(t, credential.Token, "oat_")
	require.True(t, credential.User.APITokenConfigured)
	_, err := store.CreateAccount(context.Background(), user.ID, authorization.Account{AppID: "wx-writer", Name: "writer account", Status: authorization.AccountStatusActive})
	require.NoError(t, err)

	tokenSession := doJSONWithBearer(t, router, http.MethodGet, "/api/v1/admin/session", ``, "tenant-1", credential.Token)
	require.Equal(t, http.StatusOK, tokenSession.Code)
	var tokenSessionBody adminSessionResponse
	require.NoError(t, json.Unmarshal(tokenSession.Body.Bytes(), &tokenSessionBody))
	require.True(t, tokenSessionBody.Authenticated)
	require.Equal(t, user.ID, tokenSessionBody.UserID)

	accounts := doJSONWithBearer(t, router, http.MethodGet, "/api/v1/accounts", ``, "tenant-1", credential.Token)
	require.Equal(t, http.StatusOK, accounts.Code)
	require.Contains(t, accounts.Body.String(), "wx-writer")
	require.NotContains(t, accounts.Body.String(), "tenant-1\"")

	mcpConfig := doJSONWithBearer(t, router, http.MethodGet, "/api/v1/admin/mcp-config", ``, "tenant-1", credential.Token)
	require.Equal(t, http.StatusOK, mcpConfig.Code)
	var userMCPConfig mcpConfigResponse
	require.NoError(t, json.Unmarshal(mcpConfig.Body.Bytes(), &userMCPConfig))
	require.True(t, userMCPConfig.Configured)
	require.False(t, userMCPConfig.Revealable)
	require.Empty(t, userMCPConfig.Token)
	require.NotEmpty(t, userMCPConfig.TokenHint)
	require.NotContains(t, mcpConfig.Body.String(), "global-mcp-token")

	revoked := doJSONWithAdminKey(t, router, http.MethodDelete, "/api/v1/admin/users/"+user.ID+"/api-token", ``, "spoofed-user", "admin-key")
	require.Equal(t, http.StatusOK, revoked.Code)
	var revokedUser userResponse
	require.NoError(t, json.Unmarshal(revoked.Body.Bytes(), &revokedUser))
	require.False(t, revokedUser.APITokenConfigured)

	afterRevoke := doJSONWithBearer(t, router, http.MethodGet, "/api/v1/accounts", ``, user.ID, credential.Token)
	require.Equal(t, http.StatusUnauthorized, afterRevoke.Code)
}
