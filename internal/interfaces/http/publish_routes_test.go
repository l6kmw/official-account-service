package http

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"official-account-service/internal/application"
	"official-account-service/internal/domain/material"
	"official-account-service/internal/domain/publish"
	"official-account-service/internal/infra/persistence/memory"
)

func TestRealPublishRoutes(t *testing.T) {
	now := time.Date(2026, 7, 6, 8, 0, 0, 0, time.UTC)
	store := memory.NewStore(func() time.Time { return now })
	articles := application.NewArticleService(store)
	draft, err := articles.CreateArticle(t.Context(), application.CreateArticleInput{
		TenantID: "tenant-1", AuthorizerID: 1, Title: "hello", ContentHTML: "<p>body</p>",
	})
	require.NoError(t, err)
	cover, err := store.CreateMaterial(t.Context(), "tenant-1", material.Asset{
		AuthorizerID: 1, ArticleID: draft.ID, Usage: material.UsageCover, MediaID: "thumb-media",
	})
	require.NoError(t, err)
	_, err = articles.UpdateArticle(t.Context(), application.UpdateArticleInput{
		TenantID: "tenant-1", ID: draft.ID, Title: "hello", ContentHTML: "<p>body</p>", CoverMediaAssetID: cover.ID,
	})
	require.NoError(t, err)
	publisher := &routeFakePublisher{draftMediaID: "draft-media", publishID: "publish-1", status: publish.StatusResult{
		Status: publish.StatusPublished, WeChatArticleID: "article-1",
	}}
	router := NewRouter(Dependencies{
		Logger: zap.NewNop(),
		Publishes: application.NewPublishServiceWithPublisher(
			store, store, store, publisher, routeFakePublishTokenProvider{}, "wx-component", func() time.Time { return now },
		),
	})

	published := doJSON(t, router, http.MethodPost, "/api/v1/articles/1/publish", ``, "tenant-1")
	require.Equal(t, http.StatusCreated, published.Code)
	require.Contains(t, published.Body.String(), `"wechat_publish_id":"publish-1"`)
	require.NotContains(t, published.Body.String(), "authorizer-token")

	synced := doJSON(t, router, http.MethodPost, "/api/v1/publish-records/1/sync-status", ``, "tenant-1")
	require.Equal(t, http.StatusOK, synced.Code)
	require.Contains(t, synced.Body.String(), `"status":"published"`)
	require.Contains(t, synced.Body.String(), `"wechat_article_id":"article-1"`)
}

func TestRealPublishRouteUnavailable(t *testing.T) {
	router := testRouter()
	recorder := doJSON(t, router, http.MethodPost, "/api/v1/articles/1/publish", ``, "tenant-1")
	require.Equal(t, http.StatusNotImplemented, recorder.Code)
}

type routeFakePublisher struct {
	draftMediaID string
	publishID    string
	status       publish.StatusResult
}

func (p *routeFakePublisher) AddDraft(_ context.Context, _ string, _ publish.ArticleDraft) (publish.DraftResult, error) {
	return publish.DraftResult{MediaID: p.draftMediaID}, nil
}

func (p *routeFakePublisher) SubmitFreePublish(_ context.Context, _ string, _ string) (publish.SubmitResult, error) {
	return publish.SubmitResult{PublishID: p.publishID}, nil
}

func (p *routeFakePublisher) GetFreePublishStatus(_ context.Context, _ string, _ string) (publish.StatusResult, error) {
	return p.status, nil
}

type routeFakePublishTokenProvider struct{}

func (routeFakePublishTokenProvider) GetAuthorizerAccessToken(_ context.Context, _ application.RefreshAuthorizerAccessTokenInput) (application.AuthorizerAccessToken, error) {
	return application.AuthorizerAccessToken{AccessToken: "authorizer-token"}, nil
}
