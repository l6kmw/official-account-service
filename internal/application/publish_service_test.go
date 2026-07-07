package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"official-account-service/internal/domain/article"
	"official-account-service/internal/domain/material"
	"official-account-service/internal/domain/publish"
	"official-account-service/internal/infra/persistence/memory"
)

func TestPublishServiceCreatesUpdatesAndListsRecords(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 6, 16, 0, 0, 0, time.UTC)
	store := memory.NewStore(func() time.Time { return now })
	articles := NewArticleService(store)
	service := NewPublishService(store, store, func() time.Time { return now })

	draft, err := articles.CreateArticle(ctx, CreateArticleInput{TenantID: "tenant-1", AuthorizerID: 1, Title: "hello"})
	require.NoError(t, err)

	record, err := service.CreatePublishRecord(ctx, CreatePublishRecordInput{TenantID: "tenant-1", ArticleID: draft.ID, WeChatPublishID: "pub-1"})
	require.NoError(t, err)
	require.Equal(t, publish.StatusPublishing, record.Status)
	require.Equal(t, now, record.SubmittedAt)

	current, err := articles.GetArticle(ctx, "tenant-1", draft.ID)
	require.NoError(t, err)
	require.Equal(t, article.StatusPublishing, current.Status)

	updated, err := service.UpdatePublishStatus(ctx, UpdatePublishStatusInput{TenantID: "tenant-1", ID: record.ID, Status: publish.StatusPublished, WeChatArticleID: "article-1"})
	require.NoError(t, err)
	require.Equal(t, publish.StatusPublished, updated.Status)
	require.Equal(t, "article-1", updated.WeChatArticleID)
	require.Equal(t, now, updated.FinishedAt)

	again, err := service.UpdatePublishStatus(ctx, UpdatePublishStatusInput{TenantID: "tenant-1", ID: record.ID, Status: publish.StatusPublished, WeChatArticleID: "article-1"})
	require.NoError(t, err)
	require.Equal(t, updated.UpdatedAt, again.UpdatedAt)

	current, err = articles.GetArticle(ctx, "tenant-1", draft.ID)
	require.NoError(t, err)
	require.Equal(t, article.StatusPublished, current.Status)

	records, err := service.ListPublishRecordsByArticle(ctx, "tenant-1", draft.ID)
	require.NoError(t, err)
	require.Len(t, records, 1)
	allRecords, err := service.ListPublishRecords(ctx, "tenant-1")
	require.NoError(t, err)
	require.Len(t, allRecords, 1)
	require.Equal(t, record.ID, allRecords[0].ID)
	got, err := service.GetPublishRecord(ctx, "tenant-1", record.ID)
	require.NoError(t, err)
	require.Equal(t, record.ID, got.ID)
}

func TestPublishServiceValidatesAndProtectsTenantIsolation(t *testing.T) {
	ctx := context.Background()
	store := memory.NewStore(time.Now)
	articles := NewArticleService(store)
	service := NewPublishService(store, store, time.Now)

	draft, err := articles.CreateArticle(ctx, CreateArticleInput{TenantID: "tenant-1", AuthorizerID: 1, Title: "hello"})
	require.NoError(t, err)
	record, err := service.CreatePublishRecord(ctx, CreatePublishRecordInput{TenantID: "tenant-1", ArticleID: draft.ID})
	require.NoError(t, err)

	_, err = service.ListPublishRecordsByArticle(ctx, "", draft.ID)
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))

	_, err = service.ListPublishRecords(ctx, "")
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))

	_, err = service.GetPublishRecord(ctx, "tenant-2", record.ID)
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrNotFound))

	_, err = service.UpdatePublishStatus(ctx, UpdatePublishStatusInput{TenantID: "tenant-2", ID: record.ID, Status: publish.StatusPublished})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrNotFound))

	_, err = service.UpdatePublishStatus(ctx, UpdatePublishStatusInput{TenantID: "tenant-1", ID: record.ID, Status: publish.Status("bad")})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))
}

func TestPublishServiceRejectsFinalStatusTransition(t *testing.T) {
	ctx := context.Background()
	store := memory.NewStore(time.Now)
	articles := NewArticleService(store)
	service := NewPublishService(store, store, time.Now)

	draft, err := articles.CreateArticle(ctx, CreateArticleInput{TenantID: "tenant-1", AuthorizerID: 1, Title: "hello"})
	require.NoError(t, err)
	record, err := service.CreatePublishRecord(ctx, CreatePublishRecordInput{TenantID: "tenant-1", ArticleID: draft.ID})
	require.NoError(t, err)
	_, err = service.UpdatePublishStatus(ctx, UpdatePublishStatusInput{TenantID: "tenant-1", ID: record.ID, Status: publish.StatusFailed, ErrorCode: "40001", ErrorMessage: "failed"})
	require.NoError(t, err)

	_, err = service.UpdatePublishStatus(ctx, UpdatePublishStatusInput{TenantID: "tenant-1", ID: record.ID, Status: publish.StatusPublished})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))
}

func TestPublishServicePublishesArticleThroughWeChat(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 6, 16, 0, 0, 0, time.UTC)
	store := memory.NewStore(func() time.Time { return now })
	articles := NewArticleService(store)
	draft, err := articles.CreateArticle(ctx, CreateArticleInput{
		TenantID: "tenant-1", AuthorizerID: 1, Title: "hello", Author: "me", Digest: "summary", ContentHTML: "<p>body</p>",
	})
	require.NoError(t, err)
	cover, err := store.CreateMaterial(ctx, "tenant-1", material.Asset{
		AuthorizerID: 1, ArticleID: draft.ID, Usage: material.UsageCover, MediaID: "thumb-media",
	})
	require.NoError(t, err)
	_, err = articles.UpdateArticle(ctx, UpdateArticleInput{
		TenantID: "tenant-1", ID: draft.ID, Title: "hello", Author: "me", Digest: "summary",
		ContentHTML: "<p>body</p>", CoverMediaAssetID: cover.ID,
	})
	require.NoError(t, err)
	publisher := &fakePublishPublisher{draftMediaID: "draft-media", publishID: "publish-1"}
	tokens := &fakePublishTokenProvider{token: AuthorizerAccessToken{AccessToken: "authorizer-token"}}
	statusSync := &fakeStatusSyncScheduler{}
	service := NewPublishServiceWithPublisherAndStatusSync(store, store, store, publisher, tokens, "wx-component", statusSync, func() time.Time { return now })

	record, err := service.PublishArticle(ctx, PublishArticleInput{TenantID: "tenant-1", ArticleID: draft.ID})
	require.NoError(t, err)

	require.Equal(t, publish.StatusPublishing, record.Status)
	require.Equal(t, "publish-1", record.WeChatPublishID)
	require.Equal(t, "authorizer-token", publisher.lastToken)
	require.Equal(t, "thumb-media", publisher.lastDraft.ThumbMediaID)
	require.Equal(t, "<p>body</p>", publisher.lastDraft.ContentHTML)
	require.Equal(t, "wx-component", tokens.lastInput.ComponentAppID)
	require.Equal(t, "tenant-1", statusSync.lastTask.TenantID)
	require.Equal(t, record.ID, statusSync.lastTask.PublishRecordID)
	current, err := articles.GetArticle(ctx, "tenant-1", draft.ID)
	require.NoError(t, err)
	require.Equal(t, article.StatusPublishing, current.Status)
}

func TestPublishServiceDeduplicatesPublishingArticle(t *testing.T) {
	ctx := context.Background()
	store := memory.NewStore(time.Now)
	articles := NewArticleService(store)
	draft, err := articles.CreateArticle(ctx, CreateArticleInput{TenantID: "tenant-1", AuthorizerID: 1, Title: "hello", ContentHTML: "<p>body</p>"})
	require.NoError(t, err)
	existing, err := store.CreatePublishRecord(ctx, "tenant-1", publish.Record{ArticleID: draft.ID, AuthorizerID: 1, Status: publish.StatusPublishing, WeChatPublishID: "publish-1"})
	require.NoError(t, err)
	publisher := &fakePublishPublisher{draftMediaID: "draft-media", publishID: "publish-2"}
	service := NewPublishServiceWithPublisher(store, store, store, publisher, &fakePublishTokenProvider{}, "wx-component", time.Now)

	record, err := service.PublishArticle(ctx, PublishArticleInput{TenantID: "tenant-1", ArticleID: draft.ID})
	require.NoError(t, err)

	require.Equal(t, existing.ID, record.ID)
	require.Equal(t, int32(0), publisher.addDraftCalls)
}

func TestPublishServiceSyncsPublishStatus(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 6, 16, 0, 0, 0, time.UTC)
	store := memory.NewStore(func() time.Time { return now })
	articles := NewArticleService(store)
	draft, err := articles.CreateArticle(ctx, CreateArticleInput{TenantID: "tenant-1", AuthorizerID: 1, Title: "hello"})
	require.NoError(t, err)
	record, err := serviceWithStore(store, now).CreatePublishRecord(ctx, CreatePublishRecordInput{TenantID: "tenant-1", ArticleID: draft.ID, WeChatPublishID: "publish-1"})
	require.NoError(t, err)
	publisher := &fakePublishPublisher{status: publish.StatusResult{Status: publish.StatusPublished, WeChatArticleID: "article-1"}}
	service := NewPublishServiceWithPublisher(store, store, store, publisher, &fakePublishTokenProvider{token: AuthorizerAccessToken{AccessToken: "authorizer-token"}}, "wx-component", func() time.Time { return now })

	updated, err := service.SyncPublishStatus(ctx, SyncPublishStatusInput{TenantID: "tenant-1", ID: record.ID})
	require.NoError(t, err)

	require.Equal(t, publish.StatusPublished, updated.Status)
	require.Equal(t, "article-1", updated.WeChatArticleID)
	current, err := articles.GetArticle(ctx, "tenant-1", draft.ID)
	require.NoError(t, err)
	require.Equal(t, article.StatusPublished, current.Status)
}

func TestPublishServiceRecordsFailedPublishStatus(t *testing.T) {
	ctx := context.Background()
	store := memory.NewStore(time.Now)
	articles := NewArticleService(store)
	draft, err := articles.CreateArticle(ctx, CreateArticleInput{TenantID: "tenant-1", AuthorizerID: 1, Title: "hello"})
	require.NoError(t, err)
	record, err := NewPublishService(store, store, time.Now).CreatePublishRecord(ctx, CreatePublishRecordInput{TenantID: "tenant-1", ArticleID: draft.ID, WeChatPublishID: "publish-1"})
	require.NoError(t, err)
	publisher := &fakePublishPublisher{status: publish.StatusResult{Status: publish.StatusFailed, ErrorCode: "publish_status_3", ErrorMessage: "publish failed"}}
	service := NewPublishServiceWithPublisher(store, store, store, publisher, &fakePublishTokenProvider{token: AuthorizerAccessToken{AccessToken: "authorizer-token"}}, "wx-component", time.Now)

	updated, err := service.SyncPublishStatus(ctx, SyncPublishStatusInput{TenantID: "tenant-1", ID: record.ID})
	require.NoError(t, err)

	require.Equal(t, publish.StatusFailed, updated.Status)
	require.Equal(t, "publish_status_3", updated.ErrorCode)
	require.Equal(t, "publish failed", updated.ErrorMessage)
}

func TestPublishServiceRequiresPublisherForRealPublish(t *testing.T) {
	ctx := context.Background()
	store := memory.NewStore(time.Now)
	articles := NewArticleService(store)
	draft, err := articles.CreateArticle(ctx, CreateArticleInput{TenantID: "tenant-1", AuthorizerID: 1, Title: "hello"})
	require.NoError(t, err)
	service := NewPublishService(store, store, time.Now)

	_, err = service.PublishArticle(ctx, PublishArticleInput{TenantID: "tenant-1", ArticleID: draft.ID})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrNotImplemented))
}

func serviceWithStore(store *memory.Store, now time.Time) *PublishService {
	return NewPublishService(store, store, func() time.Time { return now })
}

type fakePublishPublisher struct {
	draftMediaID  string
	publishID     string
	status        publish.StatusResult
	lastToken     string
	lastDraft     publish.ArticleDraft
	addDraftCalls int32
}

func (p *fakePublishPublisher) AddDraft(_ context.Context, accessToken string, draft publish.ArticleDraft) (publish.DraftResult, error) {
	p.addDraftCalls++
	p.lastToken = accessToken
	p.lastDraft = draft
	return publish.DraftResult{MediaID: p.draftMediaID}, nil
}

func (p *fakePublishPublisher) SubmitFreePublish(_ context.Context, _ string, _ string) (publish.SubmitResult, error) {
	return publish.SubmitResult{PublishID: p.publishID}, nil
}

func (p *fakePublishPublisher) GetFreePublishStatus(_ context.Context, _ string, _ string) (publish.StatusResult, error) {
	return p.status, nil
}

type fakePublishTokenProvider struct {
	token     AuthorizerAccessToken
	lastInput RefreshAuthorizerAccessTokenInput
}

func (p *fakePublishTokenProvider) GetAuthorizerAccessToken(_ context.Context, input RefreshAuthorizerAccessTokenInput) (AuthorizerAccessToken, error) {
	p.lastInput = input
	return p.token, nil
}

type fakeStatusSyncScheduler struct {
	lastTask publish.StatusSyncTask
}

func (s *fakeStatusSyncScheduler) EnqueueStatusSync(_ context.Context, task publish.StatusSyncTask) error {
	s.lastTask = task
	return nil
}
