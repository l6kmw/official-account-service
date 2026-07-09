package agentapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"official-account-service/internal/application"
	"official-account-service/internal/domain/authorization"
	"official-account-service/internal/domain/material"
	"official-account-service/internal/domain/publish"
	"official-account-service/internal/infra/persistence/memory"
)

func TestWeChatAPIAuthorize(t *testing.T) {
	store := memory.NewStore(fixedAgentTime)
	api := NewWeChatAPI(Dependencies{
		Authorization: application.NewAuthorizationServiceWithPreAuthCodeCreator(store, fakePreAuthCodeCreator{}, fixedAgentTime),
	})

	out, err := api.Authorize(t.Context(), AuthorizeInput{
		ComponentAppID: "wx-component",
		RedirectURI:    "https://example.com/callback",
		AuthType:       application.AuthorizationAuthTypeOfficialAccount,
	})
	require.NoError(t, err)

	require.Contains(t, out.AuthorizationURL, "component_appid=wx-component")
	require.Contains(t, out.AuthorizationURL, "pre_auth_code=pre-auth-code")
	require.Equal(t, 600, out.PreAuthCodeExpiresInSec)
}

func TestWeChatAPIUploadImage(t *testing.T) {
	store := memory.NewStore(fixedAgentTime)
	api := NewWeChatAPI(Dependencies{
		Materials: application.NewMaterialService(store, fakeMaterialUploader{}),
	})

	inline, err := api.UploadImage(t.Context(), UploadImageInput{
		TenantID: "tenant-1", AuthorizerID: 1, ArticleID: 2, Usage: ImageUsageInline,
		Filename: "body.png", Content: bytes.NewBufferString("image"),
	})
	require.NoError(t, err)
	require.Equal(t, "inline_image", inline.Asset.Usage)
	require.Equal(t, "https://wechat.example/body.png", inline.Asset.WeChatURL)
	require.Empty(t, inline.Asset.MediaID)

	cover, err := api.UploadImage(t.Context(), UploadImageInput{
		TenantID: "tenant-1", AuthorizerID: 1, ArticleID: 2, Usage: ImageUsageCover,
		Filename: "cover.png", Content: bytes.NewBufferString("image"),
	})
	require.NoError(t, err)
	require.Equal(t, "cover", cover.Asset.Usage)
	require.Equal(t, "media-cover.png", cover.Asset.MediaID)
	require.Empty(t, cover.Asset.WeChatURL)

	_, err = api.UploadImage(t.Context(), UploadImageInput{
		TenantID: "tenant-1", AuthorizerID: 1, ArticleID: 2, Usage: ImageUsage("bad"),
		Filename: "bad.png", Content: bytes.NewBufferString("image"),
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, application.ErrInvalidInput))
}

func TestWeChatAPIListsAccountsAndArticlesSafely(t *testing.T) {
	store := memory.NewStore(fixedAgentTime)
	accounts := application.NewAccountService(store)
	articles := application.NewArticleService(store)
	_, err := accounts.SaveAccount(t.Context(), application.SaveAccountInput{
		TenantID: "tenant-1", AppID: "wx-account", Name: "Account",
		Status:                          authorization.AccountStatusActive,
		EncryptedAuthorizerRefreshToken: "encrypted-refresh",
	})
	require.NoError(t, err)
	_, err = accounts.SaveAccount(t.Context(), application.SaveAccountInput{
		TenantID: "tenant-2", AppID: "wx-other", Name: "Other", Status: authorization.AccountStatusActive,
	})
	require.NoError(t, err)
	_, err = articles.CreateArticle(t.Context(), application.CreateArticleInput{
		TenantID: "tenant-1", AuthorizerID: 1, Title: "hello", ContentHTML: "<p>body</p>",
	})
	require.NoError(t, err)
	api := NewWeChatAPI(Dependencies{Accounts: accounts, Articles: articles})

	accountOut, err := api.ListAccounts(t.Context(), ListAccountsInput{TenantID: "tenant-1"})
	require.NoError(t, err)
	require.Len(t, accountOut.Items, 1)
	require.Equal(t, "wx-account", accountOut.Items[0].AppID)
	encoded, err := json.Marshal(accountOut)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "refresh")
	require.NotContains(t, string(encoded), "encrypted")

	articleOut, err := api.ListArticles(t.Context(), ListArticlesInput{TenantID: "tenant-1"})
	require.NoError(t, err)
	require.Len(t, articleOut.Items, 1)
	require.Equal(t, "hello", articleOut.Items[0].Title)
}

func TestWeChatAPIPublishArticle(t *testing.T) {
	ctx := context.Background()
	now := fixedAgentTime()
	store := memory.NewStore(func() time.Time { return now })
	articles := application.NewArticleService(store)
	draft, err := articles.CreateArticle(ctx, application.CreateArticleInput{
		TenantID: "tenant-1", AuthorizerID: 1, Title: "hello", ContentHTML: "<p>body</p>",
	})
	require.NoError(t, err)
	cover, err := store.CreateMaterial(ctx, "tenant-1", material.Asset{
		AuthorizerID: 1, ArticleID: draft.ID, Usage: material.UsageCover, MediaID: "thumb-media",
	})
	require.NoError(t, err)
	_, err = articles.UpdateArticle(ctx, application.UpdateArticleInput{
		TenantID: "tenant-1", ID: draft.ID, Title: "hello", ContentHTML: "<p>body</p>", CoverMediaAssetID: cover.ID,
	})
	require.NoError(t, err)
	publishes := application.NewPublishServiceWithPublisher(
		store, store, store, fakePublisher{}, fakePublishTokenProvider{}, "wx-component", func() time.Time { return now },
	)
	api := NewWeChatAPI(Dependencies{Publishes: publishes})

	out, err := api.PublishArticle(ctx, PublishArticleInput{TenantID: "tenant-1", ArticleID: draft.ID})
	require.NoError(t, err)

	require.Equal(t, "publishing", out.Record.Status)
	require.Equal(t, "publish-1", out.Record.WeChatPublishID)
	encoded, err := json.Marshal(out)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "authorizer-token")
}

func TestWeChatAPIDependencyValidation(t *testing.T) {
	_, err := (*WeChatAPI)(nil).Authorize(t.Context(), AuthorizeInput{})
	require.Error(t, err)
	require.True(t, errors.Is(err, application.ErrNotImplemented))

	api := NewWeChatAPI(Dependencies{})
	_, err = api.ListArticles(t.Context(), ListArticlesInput{TenantID: "tenant-1"})
	require.Error(t, err)
	require.True(t, errors.Is(err, application.ErrNotImplemented))
}

type fakePreAuthCodeCreator struct{}

func (fakePreAuthCodeCreator) CreatePreAuthCode(_ context.Context, _ string) (authorization.PreAuthCode, error) {
	return authorization.PreAuthCode{Code: "pre-auth-code", ExpiresInSeconds: 600}, nil
}

type fakeMaterialUploader struct{}

func (fakeMaterialUploader) UploadInlineImage(_ context.Context, _ string, filename string, _ io.Reader) (material.InlineImageUpload, error) {
	return material.InlineImageUpload{WeChatURL: "https://wechat.example/" + filename}, nil
}

func (fakeMaterialUploader) UploadCover(_ context.Context, _ string, filename string, _ io.Reader) (material.CoverUpload, error) {
	return material.CoverUpload{MediaID: "media-" + filename}, nil
}

type fakePublisher struct{}

func (fakePublisher) AddDraft(_ context.Context, _ string, draft publish.ArticleDraft) (publish.DraftResult, error) {
	if draft.ThumbMediaID == "" {
		return publish.DraftResult{}, publish.ErrPublishFailed
	}
	return publish.DraftResult{MediaID: "draft-media"}, nil
}

func (fakePublisher) SubmitFreePublish(_ context.Context, _ string, _ string) (publish.SubmitResult, error) {
	return publish.SubmitResult{PublishID: "publish-1"}, nil
}

func (fakePublisher) GetFreePublishStatus(_ context.Context, _ string, _ string) (publish.StatusResult, error) {
	return publish.StatusResult{Status: publish.StatusPublished, WeChatArticleID: "article-1"}, nil
}

func (fakePublisher) DeleteFreePublish(_ context.Context, _ string, _ string, _ int) error {
	return nil
}

type fakePublishTokenProvider struct{}

func (fakePublishTokenProvider) GetAuthorizerAccessToken(_ context.Context, _ application.RefreshAuthorizerAccessTokenInput) (application.AuthorizerAccessToken, error) {
	return application.AuthorizerAccessToken{AccessToken: "authorizer-token"}, nil
}

func fixedAgentTime() time.Time {
	return time.Date(2026, 7, 6, 8, 0, 0, 0, time.UTC)
}
