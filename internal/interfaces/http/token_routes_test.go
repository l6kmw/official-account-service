package http

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"official-account-service/internal/application"
	"official-account-service/internal/domain/authorization"
	"official-account-service/internal/infra/persistence/memory"
)

func TestTokenStatusRoute(t *testing.T) {
	now := time.Date(2026, 7, 6, 8, 0, 0, 0, time.UTC)
	store := memory.NewStore(func() time.Time { return now })
	account, err := store.SaveAccount(t.Context(), "tenant-1", authorization.Account{
		AppID: "wx-authorizer", Status: authorization.AccountStatusActive,
		EncryptedAuthorizerRefreshToken: "encrypted-refresh",
	})
	require.NoError(t, err)
	tokens := application.NewTokenService(
		store,
		routeFakeTokenAuthorizerClient{token: authorization.AuthorizerToken{AccessToken: "authorizer-token", ExpiresInSeconds: 7200}},
		routeFakeRefreshTokenCodec{plaintext: "refresh-token"},
		func() time.Time { return now },
	)
	router := NewRouter(Dependencies{Logger: zap.NewNop(), Tokens: tokens})

	missing := doJSON(t, router, http.MethodGet, "/api/v1/accounts/1/token-status?component_appid=wx-component", ``, "tenant-1")
	require.Equal(t, http.StatusOK, missing.Code)
	require.Contains(t, missing.Body.String(), `"cached":false`)
	require.Contains(t, missing.Body.String(), `"needs_refresh":true`)
	require.NotContains(t, missing.Body.String(), "authorizer-token")
	require.NotContains(t, missing.Body.String(), "refresh-token")
	require.NotContains(t, missing.Body.String(), "encrypted-refresh")

	_, err = tokens.GetAuthorizerAccessToken(t.Context(), application.RefreshAuthorizerAccessTokenInput{
		TenantID: "tenant-1", AccountID: account.ID, ComponentAppID: "wx-component",
	})
	require.NoError(t, err)
	cached := doJSON(t, router, http.MethodGet, "/api/v1/accounts/1/token-status?component_appid=wx-component", ``, "tenant-1")
	require.Equal(t, http.StatusOK, cached.Code)
	require.Contains(t, cached.Body.String(), `"account_id":1`)
	require.Contains(t, cached.Body.String(), `"app_id":"wx-authorizer"`)
	require.Contains(t, cached.Body.String(), `"account_status":"active"`)
	require.Contains(t, cached.Body.String(), `"cached":true`)
	require.Contains(t, cached.Body.String(), `"expires_in_seconds":7200`)
	require.Contains(t, cached.Body.String(), `"needs_refresh":false`)
	require.NotContains(t, cached.Body.String(), "authorizer-token")
	require.NotContains(t, cached.Body.String(), "refresh-token")
	require.NotContains(t, cached.Body.String(), "encrypted-refresh")

	otherTenant := doJSON(t, router, http.MethodGet, "/api/v1/accounts/1/token-status?component_appid=wx-component", ``, "tenant-2")
	require.Equal(t, http.StatusNotFound, otherTenant.Code)
	bad := doJSON(t, router, http.MethodGet, "/api/v1/accounts/1/token-status", ``, "tenant-1")
	require.Equal(t, http.StatusBadRequest, bad.Code)
	unavailable := doJSON(t, testRouter(), http.MethodGet, "/api/v1/accounts/1/token-status?component_appid=wx-component", ``, "tenant-1")
	require.Equal(t, http.StatusNotImplemented, unavailable.Code)
}

type routeFakeTokenAuthorizerClient struct {
	token authorization.AuthorizerToken
}

func (c routeFakeTokenAuthorizerClient) QueryAuthorizerAuthorization(_ context.Context, _ string, _ string) (authorization.AuthorizerAuthorization, error) {
	return authorization.AuthorizerAuthorization{}, nil
}

func (c routeFakeTokenAuthorizerClient) GetAuthorizerProfile(_ context.Context, _ string, _ string) (authorization.AuthorizerProfile, error) {
	return authorization.AuthorizerProfile{}, nil
}

func (c routeFakeTokenAuthorizerClient) RefreshAuthorizerAccessToken(_ context.Context, _ string, _ string, _ string) (authorization.AuthorizerToken, error) {
	return c.token, nil
}

type routeFakeRefreshTokenCodec struct {
	plaintext string
}

func (c routeFakeRefreshTokenCodec) EncryptAuthorizerRefreshToken(_ context.Context, plaintext string) (string, error) {
	return plaintext, nil
}

func (c routeFakeRefreshTokenCodec) DecryptAuthorizerRefreshToken(_ context.Context, _ string) (string, error) {
	return c.plaintext, nil
}
