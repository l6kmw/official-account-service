package http

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"official-account-service/internal/application"
	"official-account-service/internal/infra/persistence/memory"
)

func TestAdminCreatesUserWithoutExposingPasswordHash(t *testing.T) {
	store := memory.NewStore(fixedRouteTime)
	identities := application.NewIdentityService(store)
	router := NewRouter(Dependencies{
		Logger: zap.NewNop(), AdminAPIKey: "admin-key", AdminUserID: "tenant-1",
		AdminSessionSecret: "test-session-secret", Identity: identities,
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
}
