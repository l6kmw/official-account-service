package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"official-account-service/internal/domain/article"
	"official-account-service/internal/domain/publish"
	"official-account-service/internal/infra/persistence/memory"
)

func TestPublishStatusTaskServiceHandlesSyncTask(t *testing.T) {
	ctx := context.Background()
	store := memory.NewStore(time.Now)
	articles := NewArticleService(store)
	draft, err := articles.CreateArticle(ctx, CreateArticleInput{TenantID: "tenant-1", AuthorizerID: 1, Title: "hello"})
	require.NoError(t, err)
	record, err := NewPublishService(store, store, time.Now).CreatePublishRecord(ctx, CreatePublishRecordInput{
		TenantID: "tenant-1", ArticleID: draft.ID, WeChatPublishID: "publish-1",
	})
	require.NoError(t, err)
	publisher := &fakePublishPublisher{status: publish.StatusResult{Status: publish.StatusPublished, WeChatArticleID: "article-1"}}
	publishes := NewPublishServiceWithPublisher(store, store, store, publisher, &fakePublishTokenProvider{token: AuthorizerAccessToken{AccessToken: "authorizer-token"}}, "wx-component", time.Now)
	service := NewPublishStatusTaskService(publishes)

	err = service.HandleStatusSyncTask(ctx, publish.StatusSyncTask{TenantID: "tenant-1", PublishRecordID: record.ID})
	require.NoError(t, err)

	updated, err := store.GetPublishRecord(ctx, "tenant-1", record.ID)
	require.NoError(t, err)
	require.Equal(t, publish.StatusPublished, updated.Status)
	require.Equal(t, "article-1", updated.WeChatArticleID)
	current, err := articles.GetArticle(ctx, "tenant-1", draft.ID)
	require.NoError(t, err)
	require.Equal(t, article.StatusPublished, current.Status)
}

func TestPublishStatusTaskServiceRetriesWhenPublishStillProcessing(t *testing.T) {
	ctx := context.Background()
	store := memory.NewStore(time.Now)
	articles := NewArticleService(store)
	draft, err := articles.CreateArticle(ctx, CreateArticleInput{TenantID: "tenant-1", AuthorizerID: 1, Title: "hello"})
	require.NoError(t, err)
	record, err := NewPublishService(store, store, time.Now).CreatePublishRecord(ctx, CreatePublishRecordInput{
		TenantID: "tenant-1", ArticleID: draft.ID, WeChatPublishID: "publish-1",
	})
	require.NoError(t, err)
	publisher := &fakePublishPublisher{status: publish.StatusResult{Status: publish.StatusPublishing}}
	publishes := NewPublishServiceWithPublisher(store, store, store, publisher, &fakePublishTokenProvider{token: AuthorizerAccessToken{AccessToken: "authorizer-token"}}, "wx-component", time.Now)
	service := NewPublishStatusTaskService(publishes)

	err = service.HandleStatusSyncTask(ctx, publish.StatusSyncTask{TenantID: "tenant-1", PublishRecordID: record.ID})
	require.Error(t, err)
	require.True(t, errors.Is(err, publish.ErrPublishStillProcessing))
}

func TestPublishStatusTaskServiceValidatesInput(t *testing.T) {
	service := NewPublishStatusTaskService(NewPublishService(memory.NewStore(time.Now), memory.NewStore(time.Now), time.Now))

	err := service.HandleStatusSyncTask(context.Background(), publish.StatusSyncTask{TenantID: "", PublishRecordID: 1})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))

	err = service.HandleStatusSyncTask(context.Background(), publish.StatusSyncTask{TenantID: "tenant-1", PublishRecordID: 0})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))
}
