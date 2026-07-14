package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	"golang.org/x/crypto/bcrypt"

	"official-account-service/internal/application"
	"official-account-service/internal/domain/article"
	"official-account-service/internal/domain/authorization"
	"official-account-service/internal/domain/material"
	"official-account-service/internal/infra/persistence/memory"
)

func TestHealthz(t *testing.T) {
	router := NewRouter(Dependencies{Logger: zap.NewNop()})
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)

	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"status":"ok"}`, recorder.Body.String())
}

func TestAccountRoutes(t *testing.T) {
	store, router := testRouterWithStore()
	_, err := store.CreateAccount(t.Context(), "tenant-1", authorization.Account{AppID: "wx123", Name: "account", Status: authorization.AccountStatusActive})
	require.NoError(t, err)

	list := doJSON(t, router, http.MethodGet, "/api/v1/accounts", ``, "tenant-1")
	require.Equal(t, http.StatusOK, list.Code)
	require.Contains(t, list.Body.String(), `"app_id":"wx123"`)
	require.NotContains(t, list.Body.String(), "token")

	got := doJSON(t, router, http.MethodGet, "/api/v1/accounts/1", ``, "tenant-1")
	require.Equal(t, http.StatusOK, got.Code)
	require.Contains(t, got.Body.String(), `"name":"account"`)

	otherTenant := doJSON(t, router, http.MethodGet, "/api/v1/accounts/1", ``, "tenant-2")
	require.Equal(t, http.StatusNotFound, otherTenant.Code)
}

func TestComponentVerifyTicketCallbackRejectsPlainXML(t *testing.T) {
	store, router := testRouterWithStore()
	body := `<xml><AppId>wx-component</AppId><CreateTime>1783334400</CreateTime><InfoType>component_verify_ticket</InfoType><ComponentVerifyTicket>ticket-1</ComponentVerifyTicket></xml>`

	recorder := doXML(t, router, http.MethodPost, "/wechat/component/callback", body)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
	_, err := store.GetComponentVerifyTicket(t.Context(), "wx-component")
	require.Error(t, err)
}

func TestEncryptedComponentVerifyTicketCallback(t *testing.T) {
	store := memory.NewStore(func() time.Time { return time.Date(2026, 7, 6, 8, 0, 0, 0, time.UTC) })
	decryptor := &routeFakeComponentCallbackDecryptor{
		plaintext: []byte(`<xml><AppId>wx-component</AppId><CreateTime>1783334400</CreateTime><InfoType>component_verify_ticket</InfoType><ComponentVerifyTicket>ticket-1</ComponentVerifyTicket></xml>`),
	}
	router := NewRouter(Dependencies{
		Logger:        zap.NewNop(),
		Authorization: application.NewAuthorizationServiceWithDependencies(store, nil, decryptor, time.Now),
	})

	recorder := doXML(t, router, http.MethodPost, "/wechat/component/callback?encrypt_type=aes&msg_signature=signature&timestamp=1783334400&nonce=nonce", `<xml><Encrypt>ciphertext</Encrypt></xml>`)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "success", recorder.Body.String())
	require.Equal(t, "signature", decryptor.lastInput.Signature)
	require.Equal(t, "ciphertext", decryptor.lastInput.Ciphertext)

	ticket, err := store.GetComponentVerifyTicket(t.Context(), "wx-component")
	require.NoError(t, err)
	require.Equal(t, "ticket-1", ticket.Ticket)

	bad := doXML(t, router, http.MethodPost, "/wechat/component/callback?encrypt_type=aes", `<xml><Encrypt>ciphertext</Encrypt></xml>`)
	require.Equal(t, http.StatusBadRequest, bad.Code)
}

func TestEncryptedUnauthorizedComponentCallback(t *testing.T) {
	store := memory.NewStore(fixedRouteTime)
	account, err := store.SaveAccount(t.Context(), "tenant-1", authorization.Account{
		AppID: "wx-authorizer", Name: "Account", Status: authorization.AccountStatusActive,
		EncryptedAuthorizerRefreshToken: "encrypted-refresh",
	})
	require.NoError(t, err)
	_, err = store.SaveAuthorizerTenantBinding(t.Context(), authorization.AuthorizerTenantBinding{
		ComponentAppID: "wx-component", AuthorizerAppID: "wx-authorizer", TenantID: "tenant-1",
	})
	require.NoError(t, err)
	body := `<xml><AppId>wx-component</AppId><CreateTime>1783334400</CreateTime><InfoType>unauthorized</InfoType><AuthorizerAppid>wx-authorizer</AuthorizerAppid></xml>`
	decryptor := &routeFakeComponentCallbackDecryptor{plaintext: []byte(body)}
	router := NewRouter(Dependencies{
		Logger:        zap.NewNop(),
		Authorization: application.NewAuthorizationServiceWithDependencies(store, nil, decryptor, fixedRouteTime),
	})

	recorder := doXML(t, router, http.MethodPost, "/wechat/component/callback?encrypt_type=aes&msg_signature=signature&timestamp=1783334400&nonce=nonce", `<xml><Encrypt>ciphertext</Encrypt></xml>`)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "success", recorder.Body.String())

	revoked, err := store.GetAccount(t.Context(), "tenant-1", account.ID)
	require.NoError(t, err)
	require.Equal(t, authorization.AccountStatusRevoked, revoked.Status)
	require.Empty(t, revoked.EncryptedAuthorizerRefreshToken)
}

func TestAuthorizationURLRoute(t *testing.T) {
	store := memory.NewStore(func() time.Time { return time.Date(2026, 7, 6, 8, 0, 0, 0, time.UTC) })
	router := NewRouter(Dependencies{
		Logger:        zap.NewNop(),
		Authorization: application.NewAuthorizationServiceWithPreAuthCodeCreator(store, routeFakePreAuthCodeCreator{}, time.Now),
	})
	path := "/api/v1/wechat/authorization-url?component_appid=wx-component&redirect_uri=https%3A%2F%2Fexample.com%2Fcallback&auth_type=3&biz_appid=wx-authorizer"

	recorder := doJSON(t, router, http.MethodGet, path, ``, "")
	require.Equal(t, http.StatusOK, recorder.Code)
	var body authorizationURLResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Equal(t, 600, body.PreAuthCodeExpiresInSec)
	parsed, err := url.Parse(body.AuthorizationURL)
	require.NoError(t, err)
	require.Equal(t, "mp.weixin.qq.com", parsed.Host)
	require.Equal(t, "wx-component", parsed.Query().Get("component_appid"))
	require.Equal(t, "route-pre-auth-code", parsed.Query().Get("pre_auth_code"))
	require.Equal(t, "https://example.com/callback", parsed.Query().Get("redirect_uri"))
	require.Equal(t, "3", parsed.Query().Get("auth_type"))
	require.Equal(t, "wx-authorizer", parsed.Query().Get("biz_appid"))

	bad := doJSON(t, router, http.MethodGet, "/api/v1/wechat/authorization-url?component_appid=wx-component", ``, "")
	require.Equal(t, http.StatusBadRequest, bad.Code)

	unavailable := doJSON(t, testRouter(), http.MethodGet, "/api/v1/wechat/authorization-url?component_appid=wx-component&redirect_uri=https%3A%2F%2Fexample.com%2Fcallback", ``, "")
	require.Equal(t, http.StatusNotImplemented, unavailable.Code)
}

func TestAuthorizationCallbackRoute(t *testing.T) {
	store := memory.NewStore(func() time.Time { return time.Date(2026, 7, 6, 8, 0, 0, 0, time.UTC) })
	router := NewRouter(Dependencies{
		Logger: zap.NewNop(),
		Authorization: application.NewAuthorizationServiceWithAuthorizationFlow(
			store, store, nil, nil,
			routeFakeAuthorizerClient{
				authorization: authorization.AuthorizerAuthorization{AppID: "wx-authorizer", RefreshToken: "refresh-token"},
				profile:       authorization.AuthorizerProfile{Name: "Account", AvatarURL: "https://example.com/avatar.png"},
			},
			routeFakeRefreshTokenEncryptor{ciphertext: "encrypted-refresh"},
			func() time.Time { return time.Date(2026, 7, 6, 8, 0, 0, 0, time.UTC) },
		),
	})
	path := "/api/v1/wechat/authorization-callback?tenant_id=tenant-1&component_appid=wx-component&auth_code=auth-code"

	recorder := doJSON(t, router, http.MethodGet, path, ``, "")
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"app_id":"wx-authorizer"`)
	require.Contains(t, recorder.Body.String(), `"name":"Account"`)
	require.NotContains(t, recorder.Body.String(), "refresh")
	require.NotContains(t, recorder.Body.String(), "encrypted")

	items, err := store.ListAccounts(t.Context(), "tenant-1")
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "encrypted-refresh", items[0].EncryptedAuthorizerRefreshToken)

	bad := doJSON(t, router, http.MethodGet, "/api/v1/wechat/authorization-callback?tenant_id=tenant-1", ``, "")
	require.Equal(t, http.StatusBadRequest, bad.Code)

	unavailable := doJSON(t, testRouter(), http.MethodGet, path, ``, "")
	require.Equal(t, http.StatusNotImplemented, unavailable.Code)
}

func TestAdminAPIKeyProtectsManagementRoutes(t *testing.T) {
	store := memory.NewStore(fixedRouteTime)
	router := NewRouter(Dependencies{
		Logger:        zap.NewNop(),
		AdminAPIKey:   "admin-key",
		MCPToken:      "mcp-token",
		MCPPath:       "/mcp",
		Authorization: application.NewAuthorizationService(store, fixedRouteTime),
		Accounts:      application.NewAccountService(store),
	})

	health := doJSON(t, router, http.MethodGet, "/api/v1/healthz", ``, "")
	require.Equal(t, http.StatusOK, health.Code)

	publicAuthorization := doJSON(t, router, http.MethodGet, "/api/v1/wechat/authorization-url?component_appid=wx-component", ``, "")
	require.Equal(t, http.StatusBadRequest, publicAuthorization.Code)

	missingKey := doJSON(t, router, http.MethodGet, "/api/v1/accounts", ``, "tenant-1")
	require.Equal(t, http.StatusUnauthorized, missingKey.Code)
	require.JSONEq(t, `{"error":"unauthorized"}`, missingKey.Body.String())

	missingMCPKey := doJSON(t, router, http.MethodGet, "/api/v1/admin/mcp-config", ``, "")
	require.Equal(t, http.StatusUnauthorized, missingMCPKey.Code)

	wrongKey := doJSONWithAdminKey(t, router, http.MethodGet, "/api/v1/accounts", ``, "tenant-1", "wrong-key")
	require.Equal(t, http.StatusUnauthorized, wrongKey.Code)

	headerKey := doJSONWithAdminKey(t, router, http.MethodGet, "/api/v1/accounts", ``, "tenant-1", "admin-key")
	require.Equal(t, http.StatusOK, headerKey.Code)

	mcpConfig := doJSONWithAdminKey(t, router, http.MethodGet, "/api/v1/admin/mcp-config", ``, "", "admin-key")
	require.Equal(t, http.StatusOK, mcpConfig.Code)
	require.JSONEq(t, `{"transport":"streamable-http","path":"/mcp","header_name":"Authorization","token":"mcp-token","configured":true}`, mcpConfig.Body.String())

	bearer := doJSONWithBearer(t, router, http.MethodGet, "/api/v1/accounts", ``, "tenant-1", "admin-key")
	require.Equal(t, http.StatusOK, bearer.Code)
}

func TestAdminSessionLoginProtectsManagementRoutes(t *testing.T) {
	store := memory.NewStore(fixedRouteTime)
	hash, err := bcrypt.GenerateFromPassword([]byte("secret-password"), bcrypt.MinCost)
	require.NoError(t, err)
	router := NewRouter(Dependencies{
		Logger:             zap.NewNop(),
		AdminUsername:      "admin",
		AdminPasswordHash:  string(hash),
		AdminSessionSecret: "test-session-secret",
		Authorization:      application.NewAuthorizationService(store, fixedRouteTime),
		Accounts:           application.NewAccountService(store),
		Articles:           application.NewArticleService(store),
	})

	status := doJSON(t, router, http.MethodGet, "/api/v1/admin/session", ``, "")
	require.Equal(t, http.StatusOK, status.Code)
	require.JSONEq(t, `{"authenticated":false,"auth_enabled":true,"login_enabled":true}`, status.Body.String())

	missingSession := doJSON(t, router, http.MethodGet, "/api/v1/accounts", ``, "tenant-1")
	require.Equal(t, http.StatusUnauthorized, missingSession.Code)

	wrongPassword := doJSON(t, router, http.MethodPost, "/api/v1/admin/session", `{"username":"admin","password":"wrong"}`, "")
	require.Equal(t, http.StatusUnauthorized, wrongPassword.Code)

	login := doJSON(t, router, http.MethodPost, "/api/v1/admin/session", `{"username":"admin","password":"secret-password"}`, "")
	require.Equal(t, http.StatusOK, login.Code)
	var loginBody adminSessionResponse
	require.NoError(t, json.Unmarshal(login.Body.Bytes(), &loginBody))
	require.True(t, loginBody.Authenticated)
	require.Equal(t, "admin", loginBody.Username)
	require.NotEmpty(t, loginBody.CSRFToken)
	sessionCookie := firstCookie(t, login, adminSessionCookieName)
	require.True(t, sessionCookie.HttpOnly)
	require.Equal(t, http.SameSiteStrictMode, sessionCookie.SameSite)

	sessionStatus := doJSONWithCookiesAndHeaders(t, router, http.MethodGet, "/api/v1/admin/session", ``, "", []*http.Cookie{sessionCookie}, nil)
	require.Equal(t, http.StatusOK, sessionStatus.Code)
	require.Contains(t, sessionStatus.Body.String(), `"authenticated":true`)
	require.Contains(t, sessionStatus.Body.String(), `"csrf_token"`)

	accounts := doJSONWithCookiesAndHeaders(t, router, http.MethodGet, "/api/v1/accounts", ``, "tenant-1", []*http.Cookie{sessionCookie}, nil)
	require.Equal(t, http.StatusOK, accounts.Code)

	missingCSRF := doJSONWithCookiesAndHeaders(t, router, http.MethodPost, "/api/v1/articles", `{"authorizer_id":1,"title":"hello"}`, "tenant-1", []*http.Cookie{sessionCookie}, nil)
	require.Equal(t, http.StatusUnauthorized, missingCSRF.Code)

	created := doJSONWithCookiesAndHeaders(t, router, http.MethodPost, "/api/v1/articles", `{"authorizer_id":1,"title":"hello"}`, "tenant-1", []*http.Cookie{sessionCookie}, map[string]string{adminCSRFHeaderName: loginBody.CSRFToken})
	require.Equal(t, http.StatusCreated, created.Code)

	logout := doJSONWithCookiesAndHeaders(t, router, http.MethodDelete, "/api/v1/admin/session", ``, "", []*http.Cookie{sessionCookie}, map[string]string{adminCSRFHeaderName: loginBody.CSRFToken})
	require.Equal(t, http.StatusOK, logout.Code)
	clearedCookie := firstCookie(t, logout, adminSessionCookieName)
	require.Equal(t, -1, clearedCookie.MaxAge)
}

func TestArticleCRUDRoutes(t *testing.T) {
	router := testRouter()

	created := doJSON(t, router, http.MethodPost, "/api/v1/articles", `{"authorizer_id":1,"title":"hello","content_html":"<p>body</p>"}`, "tenant-1")
	require.Equal(t, http.StatusCreated, created.Code)
	require.JSONEq(t, `{"id":1,"tenant_id":"tenant-1","authorizer_id":1,"title":"hello","author":"","digest":"","content_html":"<p>body</p>","cover_media_asset_id":0,"status":"draft","created_at":"2026-07-06T08:00:00Z","updated_at":"2026-07-06T08:00:00Z"}`, created.Body.String())

	got := doJSON(t, router, http.MethodGet, "/api/v1/articles/1", ``, "tenant-1")
	require.Equal(t, http.StatusOK, got.Code)
	require.Contains(t, got.Body.String(), `"title":"hello"`)

	updated := doJSON(t, router, http.MethodPut, "/api/v1/articles/1", `{"title":"updated","author":"me","digest":"sum","content_html":"<p>new</p>","cover_media_asset_id":2}`, "tenant-1")
	require.Equal(t, http.StatusOK, updated.Code)
	require.Contains(t, updated.Body.String(), `"title":"updated"`)
	require.Contains(t, updated.Body.String(), `"cover_media_asset_id":2`)

	list := doJSON(t, router, http.MethodGet, "/api/v1/articles", ``, "tenant-1")
	require.Equal(t, http.StatusOK, list.Code)
	require.Contains(t, list.Body.String(), `"items":[`)
	require.Contains(t, list.Body.String(), `"title":"updated"`)

	deleted := doJSON(t, router, http.MethodDelete, "/api/v1/articles/1", ``, "tenant-1")
	require.Equal(t, http.StatusNoContent, deleted.Code)

	missing := doJSON(t, router, http.MethodGet, "/api/v1/articles/1", ``, "tenant-1")
	require.Equal(t, http.StatusNotFound, missing.Code)
	require.JSONEq(t, `{"error":"not_found"}`, missing.Body.String())
}

func TestMaterialUploadRoutes(t *testing.T) {
	store, router := testRouterWithStore()
	_, err := store.Create(t.Context(), "tenant-1", article.Article{TenantID: "tenant-1", AuthorizerID: 1, Title: "first", Status: article.StatusDraft})
	require.NoError(t, err)
	_, err = store.Create(t.Context(), "tenant-1", article.Article{TenantID: "tenant-1", AuthorizerID: 1, Title: "second", Status: article.StatusDraft})
	require.NoError(t, err)

	inline := doMultipart(t, router, "/api/v1/materials/inline-images", "tenant-1")
	require.Equal(t, http.StatusCreated, inline.Code)
	require.Contains(t, inline.Body.String(), `"usage":"inline_image"`)
	require.Contains(t, inline.Body.String(), `"wechat_url":"https://wechat.example/image.png"`)
	require.Contains(t, inline.Body.String(), `"media_id":""`)

	cover := doMultipart(t, router, "/api/v1/materials/covers", "tenant-1")
	require.Equal(t, http.StatusCreated, cover.Code)
	require.Contains(t, cover.Body.String(), `"usage":"cover"`)
	require.Contains(t, cover.Body.String(), `"media_id":"media-image.png"`)
	require.Contains(t, cover.Body.String(), `"wechat_url":""`)

	list := doJSON(t, router, http.MethodGet, "/api/v1/materials?article_id=2", ``, "tenant-1")
	require.Equal(t, http.StatusOK, list.Code)
	var listed struct {
		Items []materialResponse `json:"items"`
	}
	require.NoError(t, json.Unmarshal(list.Body.Bytes(), &listed))
	require.Len(t, listed.Items, 2)
	require.Positive(t, listed.Items[0].ID)
	require.Positive(t, listed.Items[1].ID)
	require.Equal(t, "inline_image", listed.Items[0].Usage)
	require.Equal(t, "cover", listed.Items[1].Usage)

	missingArticle := doJSON(t, router, http.MethodGet, "/api/v1/materials", ``, "tenant-1")
	require.Equal(t, http.StatusBadRequest, missingArticle.Code)

	otherTenant := doJSON(t, router, http.MethodGet, "/api/v1/materials?article_id=2", ``, "tenant-2")
	require.Equal(t, http.StatusNotFound, otherTenant.Code)

	bad := doMultipart(t, router, "/api/v1/materials/covers", "")
	require.Equal(t, http.StatusBadRequest, bad.Code)
}

func TestRoutesRejectOversizedBodies(t *testing.T) {
	router := testRouter()
	oversizedJSON := `{"authorizer_id":1,"title":"` + string(bytes.Repeat([]byte("x"), int(maxRequestBodyBytes))) + `"}`

	jsonRecorder := doJSON(t, router, http.MethodPost, "/api/v1/articles", oversizedJSON, "tenant-1")
	require.Equal(t, http.StatusRequestEntityTooLarge, jsonRecorder.Code)
	require.JSONEq(t, `{"error":"request_too_large"}`, jsonRecorder.Body.String())

	multipartRecorder := doOversizedMultipart(t, router, "/api/v1/materials/covers", "tenant-1")
	require.Equal(t, http.StatusRequestEntityTooLarge, multipartRecorder.Code)
	require.JSONEq(t, `{"error":"request_too_large"}`, multipartRecorder.Body.String())
}

func TestWriteServiceErrorLogsInternalErrorSafely(t *testing.T) {
	core, observed := observer.New(zap.ErrorLevel)
	logger := zap.New(core)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/articles/1/publish?access_token=query-secret", nil)
	c.Request.Header.Set("X-Tenant-ID", "tenant-1")
	c.Set(loggerContextKey, logger)

	ok := writeServiceError(c, errors.New("submit failed: access_token=token-secret admin_api_key=admin-secret password=pw"))

	require.False(t, ok)
	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	require.JSONEq(t, `{"error":"internal_error"}`, recorder.Body.String())
	entries := observed.FilterMessage("service internal error").All()
	require.Len(t, entries, 1)
	fields := entries[0].ContextMap()
	require.Equal(t, "internal_error", fields["error_code"])
	require.Equal(t, "POST", fields["method"])
	require.Equal(t, "/api/v1/articles/1/publish", fields["path"])
	require.Equal(t, "tenant-1", fields["tenant_id"])
	errorMessage, ok := fields["error"].(string)
	require.True(t, ok)
	require.Contains(t, errorMessage, "[REDACTED]")
	require.NotContains(t, errorMessage, "token-secret")
	require.NotContains(t, errorMessage, "admin-secret")
	require.NotContains(t, errorMessage, "pw")
	require.NotContains(t, fields["path"], "query-secret")
}

func TestArticleRoutesValidateInputAndTenantIsolation(t *testing.T) {
	router := testRouter()

	noTenant := doJSON(t, router, http.MethodPost, "/api/v1/articles", `{"authorizer_id":1,"title":"hello"}`, "")
	require.Equal(t, http.StatusBadRequest, noTenant.Code)

	badBody := doJSON(t, router, http.MethodPost, "/api/v1/articles", `{"authorizer_id":0,"title":""}`, "tenant-1")
	require.Equal(t, http.StatusBadRequest, badBody.Code)

	created := doJSON(t, router, http.MethodPost, "/api/v1/articles", `{"authorizer_id":1,"title":"hello"}`, "tenant-1")
	require.Equal(t, http.StatusCreated, created.Code)

	otherTenant := doJSON(t, router, http.MethodGet, "/api/v1/articles/1", ``, "tenant-2")
	require.Equal(t, http.StatusNotFound, otherTenant.Code)
}

func testRouter() http.Handler {
	_, router := testRouterWithStore()
	return router
}

func testRouterWithStore() (*memory.Store, http.Handler) {
	store := memory.NewStore(fixedRouteTime)
	return store, NewRouter(Dependencies{
		Logger:        zap.NewNop(),
		Authorization: application.NewAuthorizationService(store, fixedRouteTime),
		Accounts:      application.NewAccountService(store),
		Articles:      application.NewArticleService(store),
		Materials:     application.NewMaterialService(store, store, routeFakeUploader{}),
		Publishes:     application.NewPublishService(store, store, fixedRouteTime),
		Dashboard: application.NewDashboardService(
			application.NewAccountService(store),
			application.NewArticleService(store),
			application.NewPublishService(store, store, fixedRouteTime),
		),
	})
}

func fixedRouteTime() time.Time {
	return time.Date(2026, 7, 6, 8, 0, 0, 0, time.UTC)
}

func doMultipart(t *testing.T, router http.Handler, path string, tenantID string) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("authorizer_id", "1"))
	require.NoError(t, writer.WriteField("article_id", "2"))
	part, err := writer.CreateFormFile("file", "image.png")
	require.NoError(t, err)
	_, err = part.Write([]byte("image"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if tenantID != "" {
		req.Header.Set("X-Tenant-ID", tenantID)
	}
	router.ServeHTTP(recorder, req)
	return recorder
}

func doOversizedMultipart(t *testing.T, router http.Handler, path string, tenantID string) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("authorizer_id", "1"))
	require.NoError(t, writer.WriteField("article_id", "2"))
	part, err := writer.CreateFormFile("file", "large.png")
	require.NoError(t, err)
	_, err = part.Write(bytes.Repeat([]byte("x"), int(maxRequestBodyBytes)))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if tenantID != "" {
		req.Header.Set("X-Tenant-ID", tenantID)
	}
	router.ServeHTTP(recorder, req)
	return recorder
}

type routeFakeUploader struct{}

func (routeFakeUploader) UploadInlineImage(_ context.Context, _ string, filename string, _ io.Reader) (material.InlineImageUpload, error) {
	return material.InlineImageUpload{WeChatURL: "https://wechat.example/" + filename}, nil
}

func (routeFakeUploader) UploadCover(_ context.Context, _ string, filename string, _ io.Reader) (material.CoverUpload, error) {
	return material.CoverUpload{MediaID: "media-" + filename}, nil
}

type routeFakePreAuthCodeCreator struct{}

func (routeFakePreAuthCodeCreator) CreatePreAuthCode(_ context.Context, _ string) (authorization.PreAuthCode, error) {
	return authorization.PreAuthCode{Code: "route-pre-auth-code", ExpiresInSeconds: 600}, nil
}

type routeFakeComponentCallbackDecryptor struct {
	plaintext []byte
	lastInput authorization.ComponentCallbackDecryptInput
}

func (d *routeFakeComponentCallbackDecryptor) DecryptComponentCallback(_ context.Context, input authorization.ComponentCallbackDecryptInput) ([]byte, error) {
	d.lastInput = input
	return d.plaintext, nil
}

type routeFakeAuthorizerClient struct {
	authorization authorization.AuthorizerAuthorization
	profile       authorization.AuthorizerProfile
}

func (c routeFakeAuthorizerClient) QueryAuthorizerAuthorization(_ context.Context, _ string, _ string) (authorization.AuthorizerAuthorization, error) {
	return c.authorization, nil
}

func (c routeFakeAuthorizerClient) GetAuthorizerProfile(_ context.Context, _ string, _ string) (authorization.AuthorizerProfile, error) {
	return c.profile, nil
}

func (c routeFakeAuthorizerClient) RefreshAuthorizerAccessToken(_ context.Context, _ string, _ string, _ string) (authorization.AuthorizerToken, error) {
	return authorization.AuthorizerToken{}, nil
}

type routeFakeRefreshTokenEncryptor struct {
	ciphertext string
}

func (e routeFakeRefreshTokenEncryptor) EncryptAuthorizerRefreshToken(_ context.Context, _ string) (string, error) {
	return e.ciphertext, nil
}

func doJSON(t *testing.T, router http.Handler, method string, path string, body string, tenantID string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if tenantID != "" {
		req.Header.Set("X-Tenant-ID", tenantID)
	}
	router.ServeHTTP(recorder, req)
	return recorder
}

func doJSONWithAdminKey(t *testing.T, router http.Handler, method string, path string, body string, tenantID string, adminAPIKey string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if tenantID != "" {
		req.Header.Set("X-Tenant-ID", tenantID)
	}
	if adminAPIKey != "" {
		req.Header.Set("X-Admin-API-Key", adminAPIKey)
	}
	router.ServeHTTP(recorder, req)
	return recorder
}

func doJSONWithBearer(t *testing.T, router http.Handler, method string, path string, body string, tenantID string, token string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if tenantID != "" {
		req.Header.Set("X-Tenant-ID", tenantID)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	router.ServeHTTP(recorder, req)
	return recorder
}

func doJSONWithCookiesAndHeaders(t *testing.T, router http.Handler, method string, path string, body string, tenantID string, cookies []*http.Cookie, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if tenantID != "" {
		req.Header.Set("X-Tenant-ID", tenantID)
	}
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	router.ServeHTTP(recorder, req)
	return recorder
}

func firstCookie(t *testing.T, recorder *httptest.ResponseRecorder, name string) *http.Cookie {
	t.Helper()
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == name {
			return cookie
		}
	}
	t.Fatalf("cookie %q not found", name)
	return nil
}

func doXML(t *testing.T, router http.Handler, method string, path string, body string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/xml")
	router.ServeHTTP(recorder, req)
	return recorder
}
