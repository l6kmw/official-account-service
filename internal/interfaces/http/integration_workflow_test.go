package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"official-account-service/internal/application"
	"official-account-service/internal/domain/authorization"
	"official-account-service/internal/domain/material"
	"official-account-service/internal/domain/publish"
	"official-account-service/internal/infra/persistence/memory"
)

func TestHTTPIntegrationTenantPublishingWorkflow(t *testing.T) {
	router, uploader, publisher := newIntegrationWorkflowRouter()

	account := authorizeIntegrationAccount(t, router, "tenant-1", "auth-code-tenant-1")
	require.Equal(t, int64(1), account.ID)
	require.Equal(t, "tenant-1", account.TenantID)
	require.Equal(t, "wx-authorizer-1", account.AppID)

	authorizeIntegrationAccount(t, router, "tenant-2", "auth-code-tenant-2")
	assertIntegrationAccountListsAreTenantScoped(t, router)
	assertIntegrationTokenStatus(t, router, account.ID, false)

	article := createIntegrationArticle(t, router, account.ID)
	cover := uploadIntegrationCover(t, router, account.ID, article.ID)
	require.Equal(t, "authorizer-token", uploader.coverToken)

	updateIntegrationArticleCover(t, router, article.ID, cover.ID)
	assertIntegrationTokenStatus(t, router, account.ID, true)

	record := publishIntegrationArticle(t, router, article.ID)
	require.Equal(t, publish.StatusPublishing, publish.Status(record.Status))
	require.Equal(t, "authorizer-token", publisher.addDraftToken)
	require.Equal(t, "authorizer-token", publisher.submitToken)

	synced := syncIntegrationPublishStatus(t, router, record.ID)
	require.Equal(t, publish.StatusPublished, publish.Status(synced.Status))
	require.Equal(t, "wechat-article-1", synced.WeChatArticleID)
	require.Equal(t, "authorizer-token", publisher.statusToken)

	assertIntegrationPublishAdminViews(t, router, synced.ID)
	assertIntegrationDashboardStats(t, router)
	assertIntegrationTenantIsolation(t, router, article.ID, synced.ID)
}

func newIntegrationWorkflowRouter() (stdhttp.Handler, *integrationUploader, *integrationPublisher) {
	store := memory.NewStore(fixedRouteTime)
	authorizers := integrationAuthorizerClient{
		authorizations: map[string]authorization.AuthorizerAuthorization{
			"auth-code-tenant-1": {AppID: "wx-authorizer-1", RefreshToken: "refresh-token-1"},
			"auth-code-tenant-2": {AppID: "wx-authorizer-2", RefreshToken: "refresh-token-2"},
		},
		profiles: map[string]authorization.AuthorizerProfile{
			"wx-authorizer-1": {Name: "Tenant One Account", AvatarURL: "https://example.com/tenant-one.png"},
			"wx-authorizer-2": {Name: "Tenant Two Account", AvatarURL: "https://example.com/tenant-two.png"},
		},
		token: authorization.AuthorizerToken{AccessToken: "authorizer-token", ExpiresInSeconds: 7200},
	}
	codec := integrationRefreshTokenCodec{}
	tokens := application.NewTokenService(store, authorizers, codec, fixedRouteTime)
	uploader := &integrationUploader{}
	publisher := &integrationPublisher{}
	accounts := application.NewAccountService(store)
	articles := application.NewArticleService(store)
	publishes := application.NewPublishServiceWithPublisher(store, store, store, publisher, tokens, "wx-component", fixedRouteTime)

	router := NewRouter(Dependencies{
		Logger: zap.NewNop(),
		Authorization: application.NewAuthorizationServiceWithAuthorizationFlow(
			store, store, nil, nil, authorizers, codec, fixedRouteTime,
		),
		Accounts:  accounts,
		Articles:  articles,
		Materials: application.NewMaterialServiceWithTokenProvider(store, store, uploader, tokens, "wx-component"),
		Publishes: publishes,
		Tokens:    tokens,
		Dashboard: application.NewDashboardService(accounts, articles, publishes),
	})
	return router, uploader, publisher
}

func authorizeIntegrationAccount(t *testing.T, router stdhttp.Handler, tenantID string, authCode string) accountResponse {
	t.Helper()
	path := fmt.Sprintf("/api/v1/wechat/authorization-callback?tenant_id=%s&component_appid=wx-component&auth_code=%s", tenantID, authCode)
	recorder := doJSON(t, router, stdhttp.MethodGet, path, "", "")
	require.Equal(t, stdhttp.StatusOK, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "refresh")
	require.NotContains(t, recorder.Body.String(), "token")
	require.NotContains(t, recorder.Body.String(), "secret")
	require.NotContains(t, recorder.Body.String(), "encrypted")

	var body accountResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	return body
}

func assertIntegrationAccountListsAreTenantScoped(t *testing.T, router stdhttp.Handler) {
	t.Helper()
	tenantOne := doJSON(t, router, stdhttp.MethodGet, "/api/v1/accounts", "", "tenant-1")
	require.Equal(t, stdhttp.StatusOK, tenantOne.Code)
	require.Contains(t, tenantOne.Body.String(), "wx-authorizer-1")
	require.NotContains(t, tenantOne.Body.String(), "wx-authorizer-2")
	require.NotContains(t, tenantOne.Body.String(), "refresh")
	require.NotContains(t, tenantOne.Body.String(), "token")

	tenantTwo := doJSON(t, router, stdhttp.MethodGet, "/api/v1/accounts", "", "tenant-2")
	require.Equal(t, stdhttp.StatusOK, tenantTwo.Code)
	require.Contains(t, tenantTwo.Body.String(), "wx-authorizer-2")
	require.NotContains(t, tenantTwo.Body.String(), "wx-authorizer-1")
}

func assertIntegrationTokenStatus(t *testing.T, router stdhttp.Handler, accountID int64, wantCached bool) {
	t.Helper()
	path := fmt.Sprintf("/api/v1/accounts/%d/token-status?component_appid=wx-component", accountID)
	recorder := doJSON(t, router, stdhttp.MethodGet, path, "", "tenant-1")
	require.Equal(t, stdhttp.StatusOK, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "authorizer-token")
	require.NotContains(t, recorder.Body.String(), "refresh-token")
	require.NotContains(t, recorder.Body.String(), "encrypted")

	var body tokenStatusResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Equal(t, accountID, body.AccountID)
	require.Equal(t, wantCached, body.Cached)
	if wantCached {
		require.False(t, body.NeedsRefresh)
		require.Equal(t, int64(7200), body.ExpiresInSeconds)
	} else {
		require.True(t, body.NeedsRefresh)
	}
}

func createIntegrationArticle(t *testing.T, router stdhttp.Handler, authorizerID int64) articleResponse {
	t.Helper()
	body := fmt.Sprintf(`{"authorizer_id":%d,"title":"Integration Article","author":"AI","digest":"summary","content_html":"<p>body</p>"}`, authorizerID)
	recorder := doJSON(t, router, stdhttp.MethodPost, "/api/v1/articles", body, "tenant-1")
	require.Equal(t, stdhttp.StatusCreated, recorder.Code)

	var article articleResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &article))
	require.Equal(t, authorizerID, article.AuthorizerID)
	require.Equal(t, "draft", article.Status)
	return article
}

func uploadIntegrationCover(t *testing.T, router stdhttp.Handler, authorizerID int64, articleID int64) materialResponse {
	t.Helper()
	recorder := doIntegrationMultipart(t, router, "/api/v1/materials/covers", "tenant-1", authorizerID, articleID)
	require.Equal(t, stdhttp.StatusCreated, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "authorizer-token")
	require.NotContains(t, recorder.Body.String(), "refresh-token")

	var cover materialResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &cover))
	require.Equal(t, "cover", cover.Usage)
	require.Equal(t, "cover-media-id", cover.MediaID)
	return cover
}

func updateIntegrationArticleCover(t *testing.T, router stdhttp.Handler, articleID int64, coverID int64) {
	t.Helper()
	body := fmt.Sprintf(`{"title":"Integration Article","author":"AI","digest":"summary","content_html":"<p>body</p>","cover_media_asset_id":%d}`, coverID)
	path := fmt.Sprintf("/api/v1/articles/%d", articleID)
	recorder := doJSON(t, router, stdhttp.MethodPut, path, body, "tenant-1")
	require.Equal(t, stdhttp.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), fmt.Sprintf(`"cover_media_asset_id":%d`, coverID))
}

func publishIntegrationArticle(t *testing.T, router stdhttp.Handler, articleID int64) publishRecordResponse {
	t.Helper()
	path := fmt.Sprintf("/api/v1/articles/%d/publish", articleID)
	recorder := doJSON(t, router, stdhttp.MethodPost, path, "", "tenant-1")
	require.Equal(t, stdhttp.StatusCreated, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "authorizer-token")
	require.NotContains(t, recorder.Body.String(), "refresh-token")

	var record publishRecordResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &record))
	require.Equal(t, "wechat-publish-1", record.WeChatPublishID)
	return record
}

func syncIntegrationPublishStatus(t *testing.T, router stdhttp.Handler, recordID int64) publishRecordResponse {
	t.Helper()
	path := fmt.Sprintf("/api/v1/publish-records/%d/sync-status", recordID)
	recorder := doJSON(t, router, stdhttp.MethodPost, path, "", "tenant-1")
	require.Equal(t, stdhttp.StatusOK, recorder.Code)

	var record publishRecordResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &record))
	return record
}

func assertIntegrationPublishAdminViews(t *testing.T, router stdhttp.Handler, recordID int64) {
	t.Helper()
	list := doJSON(t, router, stdhttp.MethodGet, "/api/v1/publish-records", "", "tenant-1")
	require.Equal(t, stdhttp.StatusOK, list.Code)
	var listBody struct {
		Items []publishRecordResponse `json:"items"`
	}
	require.NoError(t, json.Unmarshal(list.Body.Bytes(), &listBody))
	require.Len(t, listBody.Items, 1)
	require.Equal(t, recordID, listBody.Items[0].ID)
	require.Equal(t, "published", listBody.Items[0].Status)

	detailPath := fmt.Sprintf("/api/v1/publish-records/%d", recordID)
	detail := doJSON(t, router, stdhttp.MethodGet, detailPath, "", "tenant-1")
	require.Equal(t, stdhttp.StatusOK, detail.Code)
	require.Contains(t, detail.Body.String(), `"wechat_article_id":"wechat-article-1"`)
}

func assertIntegrationDashboardStats(t *testing.T, router stdhttp.Handler) {
	t.Helper()
	recorder := doJSON(t, router, stdhttp.MethodGet, "/api/v1/dashboard/stats", "", "tenant-1")
	require.Equal(t, stdhttp.StatusOK, recorder.Code)
	var stats dashboardStatsResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &stats))
	require.Equal(t, 1, stats.AccountTotal)
	require.Equal(t, 1, stats.ActiveAccountTotal)
	require.Equal(t, 1, stats.ArticleTotal)
	require.Equal(t, 1, stats.PublishedArticleTotal)
	require.Equal(t, 1, stats.PublishTotal)
	require.Equal(t, 1, stats.PublishedPublishTotal)
	require.Equal(t, 0, stats.DraftArticleTotal)
	require.Equal(t, 0, stats.FailedPublishTotal)
	require.NotContains(t, recorder.Body.String(), "token")
	require.NotContains(t, recorder.Body.String(), "secret")
}

func assertIntegrationTenantIsolation(t *testing.T, router stdhttp.Handler, articleID int64, recordID int64) {
	t.Helper()
	articlePath := fmt.Sprintf("/api/v1/articles/%d", articleID)
	article := doJSON(t, router, stdhttp.MethodGet, articlePath, "", "tenant-2")
	require.Equal(t, stdhttp.StatusNotFound, article.Code)

	recordPath := fmt.Sprintf("/api/v1/publish-records/%d", recordID)
	record := doJSON(t, router, stdhttp.MethodGet, recordPath, "", "tenant-2")
	require.Equal(t, stdhttp.StatusNotFound, record.Code)

	list := doJSON(t, router, stdhttp.MethodGet, "/api/v1/publish-records", "", "tenant-2")
	require.Equal(t, stdhttp.StatusOK, list.Code)
	require.JSONEq(t, `{"items":[]}`, list.Body.String())

	statsResponse := doJSON(t, router, stdhttp.MethodGet, "/api/v1/dashboard/stats", "", "tenant-2")
	require.Equal(t, stdhttp.StatusOK, statsResponse.Code)
	var stats dashboardStatsResponse
	require.NoError(t, json.Unmarshal(statsResponse.Body.Bytes(), &stats))
	require.Equal(t, 1, stats.AccountTotal)
	require.Equal(t, 0, stats.ArticleTotal)
	require.Equal(t, 0, stats.PublishTotal)
}

func doIntegrationMultipart(t *testing.T, router stdhttp.Handler, path string, tenantID string, authorizerID int64, articleID int64) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("authorizer_id", fmt.Sprintf("%d", authorizerID)))
	require.NoError(t, writer.WriteField("article_id", fmt.Sprintf("%d", articleID)))
	part, err := writer.CreateFormFile("file", "cover.png")
	require.NoError(t, err)
	_, err = part.Write([]byte("image"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(stdhttp.MethodPost, path, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-Tenant-ID", tenantID)
	router.ServeHTTP(recorder, req)
	return recorder
}

type integrationAuthorizerClient struct {
	authorizations map[string]authorization.AuthorizerAuthorization
	profiles       map[string]authorization.AuthorizerProfile
	token          authorization.AuthorizerToken
}

func (c integrationAuthorizerClient) QueryAuthorizerAuthorization(_ context.Context, _ string, authCode string) (authorization.AuthorizerAuthorization, error) {
	authorized, ok := c.authorizations[authCode]
	if !ok {
		return authorization.AuthorizerAuthorization{}, fmt.Errorf("query integration authorization for %s: %w", authCode, authorization.ErrAuthorizerClientUnavailable)
	}
	return authorized, nil
}

func (c integrationAuthorizerClient) GetAuthorizerProfile(_ context.Context, _ string, authorizerAppID string) (authorization.AuthorizerProfile, error) {
	profile, ok := c.profiles[authorizerAppID]
	if !ok {
		return authorization.AuthorizerProfile{}, fmt.Errorf("get integration profile for %s: %w", authorizerAppID, authorization.ErrAuthorizerClientUnavailable)
	}
	return profile, nil
}

func (c integrationAuthorizerClient) RefreshAuthorizerAccessToken(_ context.Context, _ string, _ string, _ string) (authorization.AuthorizerToken, error) {
	return c.token, nil
}

type integrationRefreshTokenCodec struct{}

func (integrationRefreshTokenCodec) EncryptAuthorizerRefreshToken(_ context.Context, plaintext string) (string, error) {
	return "encrypted-" + plaintext, nil
}

func (integrationRefreshTokenCodec) DecryptAuthorizerRefreshToken(_ context.Context, ciphertext string) (string, error) {
	refreshToken := strings.TrimPrefix(ciphertext, "encrypted-")
	if refreshToken == ciphertext || refreshToken == "" {
		return "", fmt.Errorf("decrypt integration refresh token: %w", authorization.ErrRefreshTokenEncryptorUnavailable)
	}
	return refreshToken, nil
}

type integrationUploader struct {
	coverToken string
}

func (u *integrationUploader) UploadInlineImage(_ context.Context, _ string, _ string, _ io.Reader) (material.InlineImageUpload, error) {
	return material.InlineImageUpload{WeChatURL: "https://wechat.example/inline.png"}, nil
}

func (u *integrationUploader) UploadCover(_ context.Context, token string, _ string, _ io.Reader) (material.CoverUpload, error) {
	u.coverToken = token
	return material.CoverUpload{MediaID: "cover-media-id"}, nil
}

type integrationPublisher struct {
	addDraftToken string
	submitToken   string
	statusToken   string
}

func (p *integrationPublisher) AddDraft(_ context.Context, token string, _ publish.ArticleDraft) (publish.DraftResult, error) {
	p.addDraftToken = token
	return publish.DraftResult{MediaID: "draft-media-id"}, nil
}

func (p *integrationPublisher) SubmitFreePublish(_ context.Context, token string, _ string) (publish.SubmitResult, error) {
	p.submitToken = token
	return publish.SubmitResult{PublishID: "wechat-publish-1"}, nil
}

func (p *integrationPublisher) GetFreePublishStatus(_ context.Context, token string, _ string) (publish.StatusResult, error) {
	p.statusToken = token
	return publish.StatusResult{Status: publish.StatusPublished, WeChatArticleID: "wechat-article-1"}, nil
}

func (p *integrationPublisher) DeleteFreePublish(_ context.Context, _ string, _ string, _ int) error {
	return nil
}
