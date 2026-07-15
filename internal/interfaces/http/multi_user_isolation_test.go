package http

import (
	"context"
	"encoding/json"
	"fmt"
	stdhttp "net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	"official-account-service/internal/application"
	"official-account-service/internal/domain/article"
	"official-account-service/internal/domain/authorization"
	"official-account-service/internal/domain/identity"
	"official-account-service/internal/domain/material"
	"official-account-service/internal/domain/publish"
	"official-account-service/internal/infra/persistence/memory"
)

func TestMultiUserIsolationAcrossAuthenticatedWorkflows(t *testing.T) {
	ctx := context.Background()
	store := memory.NewStore(fixedRouteTime)
	identities := application.NewIdentityService(store)
	createIsolationUser(t, store, "user-a", "alice", "alice-password-123")
	createIsolationUser(t, store, "user-b", "bob", "bob-password-123")
	tokenA, err := identities.GenerateAPIToken(ctx, "user-a")
	require.NoError(t, err)
	tokenB, err := identities.GenerateAPIToken(ctx, "user-b")
	require.NoError(t, err)

	authorizers := integrationAuthorizerClient{
		authorizations: map[string]authorization.AuthorizerAuthorization{
			"auth-code-a": {AppID: "wx-shared", RefreshToken: "refresh-a"},
			"auth-code-b": {AppID: "wx-shared", RefreshToken: "refresh-b"},
		},
		profiles: map[string]authorization.AuthorizerProfile{"wx-shared": {Name: "Alice Account"}},
	}
	codec := integrationRefreshTokenCodec{}
	accounts := application.NewAccountService(store)
	articles := application.NewArticleServiceWithAuthorizerRepository(store, store)
	materials := application.NewMaterialService(store, store, routeFakeUploader{})
	publishes := application.NewPublishService(store, store, fixedRouteTime)
	router := NewRouter(Dependencies{
		Logger: zap.NewNop(), Identity: identities, AdminSessionSecret: "test-isolation-session-secret-123456",
		Authorization: application.NewAuthorizationServiceWithSecureAuthorizationFlow(
			store, store, store, routeFakePreAuthCodeCreator{}, nil, authorizers, codec, fixedRouteTime,
		),
		Accounts: accounts, Articles: articles, Materials: materials, Publishes: publishes,
	})

	loginA := loginIsolationUser(t, router, "alice", "alice-password-123")
	loginB := loginIsolationUser(t, router, "bob", "bob-password-123")
	require.NotEqual(t, loginA.Value, loginB.Value)

	accountA := authorizeIsolationAccount(t, router, tokenA.Token, "auth-code-a", stdhttp.StatusOK)
	require.Equal(t, "user-a", accountA.TenantID)
	authorizeIsolationAccount(t, router, tokenB.Token, "auth-code-b", stdhttp.StatusConflict)
	accountB, err := store.SaveAccount(ctx, "user-b", authorization.Account{AppID: "wx-bob", Name: "Bob Account", Status: authorization.AccountStatusActive})
	require.NoError(t, err)

	articleA, err := store.Create(ctx, "user-a", article.Article{AuthorizerID: accountA.ID, Title: "Alice Draft", Status: article.StatusDraft})
	require.NoError(t, err)
	articleB, err := store.Create(ctx, "user-b", article.Article{AuthorizerID: accountB.ID, Title: "Bob Draft", Status: article.StatusDraft})
	require.NoError(t, err)
	assetA, err := store.CreateMaterial(ctx, "user-a", material.Asset{AuthorizerID: accountA.ID, ArticleID: articleA.ID, Usage: material.UsageCover, MediaID: "alice-cover"})
	require.NoError(t, err)
	recordA, err := store.CreatePublishRecord(ctx, "user-a", publish.Record{AuthorizerID: accountA.ID, ArticleID: articleA.ID, WeChatPublishID: "publish-a", Status: publish.StatusPublishing, SubmittedAt: fixedRouteTime()})
	require.NoError(t, err)

	accountsA := doJSONWithBearer(t, router, stdhttp.MethodGet, "/api/v1/accounts", ``, "user-b", tokenA.Token)
	require.Equal(t, stdhttp.StatusOK, accountsA.Code)
	require.Contains(t, accountsA.Body.String(), "wx-shared")
	require.NotContains(t, accountsA.Body.String(), "wx-bob")
	accountsB := doJSONWithBearer(t, router, stdhttp.MethodGet, "/api/v1/accounts", ``, "user-a", tokenB.Token)
	require.Equal(t, stdhttp.StatusOK, accountsB.Code)
	require.Contains(t, accountsB.Body.String(), "wx-bob")
	require.NotContains(t, accountsB.Body.String(), "wx-shared")

	crossArticle := doJSONWithBearer(t, router, stdhttp.MethodGet, fmt.Sprintf("/api/v1/articles/%d", articleA.ID), ``, "user-a", tokenB.Token)
	require.Equal(t, stdhttp.StatusNotFound, crossArticle.Code)
	ownedArticle := doJSONWithBearer(t, router, stdhttp.MethodGet, fmt.Sprintf("/api/v1/articles/%d", articleB.ID), ``, "user-a", tokenB.Token)
	require.Equal(t, stdhttp.StatusOK, ownedArticle.Code)
	crossMaterials := doJSONWithBearer(t, router, stdhttp.MethodGet, fmt.Sprintf("/api/v1/materials?article_id=%d", articleA.ID), ``, "user-a", tokenB.Token)
	require.Equal(t, stdhttp.StatusNotFound, crossMaterials.Code)
	_, err = materials.GetMaterial(ctx, "user-b", assetA.ID)
	require.ErrorIs(t, err, application.ErrNotFound)
	crossPublish := doJSONWithBearer(t, router, stdhttp.MethodGet, fmt.Sprintf("/api/v1/publish-records/%d", recordA.ID), ``, "user-a", tokenB.Token)
	require.Equal(t, stdhttp.StatusNotFound, crossPublish.Code)

	userA, err := store.GetUser(ctx, "user-a")
	require.NoError(t, err)
	userA.Status = identity.StatusDisabled
	_, err = store.SaveUser(ctx, userA)
	require.NoError(t, err)
	disabledSession := doJSONWithCookiesAndHeaders(t, router, stdhttp.MethodGet, "/api/v1/accounts", ``, "user-a", []*stdhttp.Cookie{loginA}, nil)
	require.Equal(t, stdhttp.StatusUnauthorized, disabledSession.Code)
	disabledToken := doJSONWithBearer(t, router, stdhttp.MethodGet, "/api/v1/accounts", ``, "user-a", tokenA.Token)
	require.Equal(t, stdhttp.StatusUnauthorized, disabledToken.Code)
}

func createIsolationUser(t *testing.T, store *memory.Store, id string, username string, password string) {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	require.NoError(t, err)
	_, err = store.SaveUser(context.Background(), identity.User{
		ID: id, Username: username, PasswordHash: string(hash), Role: identity.RoleUser, Status: identity.StatusActive,
	})
	require.NoError(t, err)
}

func loginIsolationUser(t *testing.T, router stdhttp.Handler, username string, password string) *stdhttp.Cookie {
	t.Helper()
	response := doJSON(t, router, stdhttp.MethodPost, "/api/v1/admin/session", fmt.Sprintf(`{"username":%q,"password":%q}`, username, password), "")
	require.Equal(t, stdhttp.StatusOK, response.Code)
	return firstCookie(t, response, adminSessionCookieName)
}

func authorizeIsolationAccount(t *testing.T, router stdhttp.Handler, token string, authCode string, wantStatus int) accountResponse {
	t.Helper()
	generated := doJSONWithBearer(t, router, stdhttp.MethodGet, "/api/v1/wechat/authorization-url?component_appid=wx-component&redirect_uri=https%3A%2F%2Fexample.com%2Fapi%2Fv1%2Fwechat%2Fauthorization-callback", ``, "forged-user", token)
	require.Equal(t, stdhttp.StatusOK, generated.Code)
	var body authorizationURLResponse
	require.NoError(t, json.Unmarshal(generated.Body.Bytes(), &body))
	authorizationURL, err := url.Parse(body.AuthorizationURL)
	require.NoError(t, err)
	callbackURL, err := url.Parse(authorizationURL.Query().Get("redirect_uri"))
	require.NoError(t, err)
	callback := doJSON(t, router, stdhttp.MethodGet, fmt.Sprintf("/api/v1/wechat/authorization-callback?state=%s&auth_code=%s", url.QueryEscape(callbackURL.Query().Get("state")), url.QueryEscape(authCode)), ``, "")
	require.Equal(t, wantStatus, callback.Code)
	if wantStatus != stdhttp.StatusOK {
		return accountResponse{}
	}
	var account accountResponse
	require.NoError(t, json.Unmarshal(callback.Body.Bytes(), &account))
	return account
}
