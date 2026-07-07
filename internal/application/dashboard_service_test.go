package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"official-account-service/internal/domain/authorization"
	"official-account-service/internal/domain/publish"
	"official-account-service/internal/infra/persistence/memory"
)

func TestDashboardServiceReturnsTenantScopedStats(t *testing.T) {
	ctx := context.Background()
	now := func() time.Time { return time.Date(2026, 7, 6, 8, 0, 0, 0, time.UTC) }
	store := memory.NewStore(now)
	accounts := NewAccountService(store)
	articles := NewArticleService(store)
	publishes := NewPublishService(store, store, now)
	service := NewDashboardService(accounts, articles, publishes)
	require.NoError(t, seedDashboardAccount(ctx, accounts, "tenant-1", "wx-active", authorization.AccountStatusActive))
	require.NoError(t, seedDashboardAccount(ctx, accounts, "tenant-1", "wx-revoked", authorization.AccountStatusRevoked))
	require.NoError(t, seedDashboardAccount(ctx, accounts, "tenant-1", "wx-refresh-failed", authorization.AccountStatusRefreshFailed))
	require.NoError(t, seedDashboardAccount(ctx, accounts, "tenant-2", "wx-other", authorization.AccountStatusActive))
	draft, err := articles.CreateArticle(ctx, CreateArticleInput{TenantID: "tenant-1", AuthorizerID: 1, Title: "draft"})
	require.NoError(t, err)
	publishing, err := articles.CreateArticle(ctx, CreateArticleInput{TenantID: "tenant-1", AuthorizerID: 1, Title: "publishing"})
	require.NoError(t, err)
	published, err := articles.CreateArticle(ctx, CreateArticleInput{TenantID: "tenant-1", AuthorizerID: 1, Title: "published"})
	require.NoError(t, err)
	failed, err := articles.CreateArticle(ctx, CreateArticleInput{TenantID: "tenant-1", AuthorizerID: 1, Title: "failed"})
	require.NoError(t, err)
	other, err := articles.CreateArticle(ctx, CreateArticleInput{TenantID: "tenant-2", AuthorizerID: 9, Title: "other"})
	require.NoError(t, err)
	require.NotZero(t, draft.ID)
	require.NoError(t, seedDashboardPublish(ctx, publishes, "tenant-1", publishing.ID, "publish-1", publish.StatusPublishing))
	require.NoError(t, seedDashboardPublish(ctx, publishes, "tenant-1", published.ID, "publish-2", publish.StatusPublished))
	require.NoError(t, seedDashboardPublish(ctx, publishes, "tenant-1", failed.ID, "publish-3", publish.StatusFailed))
	require.NoError(t, seedDashboardPublish(ctx, publishes, "tenant-2", other.ID, "publish-other", publish.StatusPublished))

	stats, err := service.GetStats(ctx, "tenant-1")
	require.NoError(t, err)

	require.Equal(t, DashboardStats{
		AccountTotal:              3,
		ActiveAccountTotal:        1,
		RevokedAccountTotal:       1,
		RefreshFailedAccountTotal: 1,
		ArticleTotal:              4,
		DraftArticleTotal:         1,
		PublishingArticleTotal:    1,
		PublishedArticleTotal:     1,
		FailedArticleTotal:        1,
		PublishTotal:              3,
		PublishingPublishTotal:    1,
		PublishedPublishTotal:     1,
		FailedPublishTotal:        1,
	}, stats)
}

func TestDashboardServiceValidatesInputAndDependencies(t *testing.T) {
	_, err := (*DashboardService)(nil).GetStats(t.Context(), "tenant-1")
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrNotImplemented))

	store := memory.NewStore(time.Now)
	service := NewDashboardService(NewAccountService(store), NewArticleService(store), NewPublishService(store, store, time.Now))
	_, err = service.GetStats(t.Context(), "")
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))
}

func seedDashboardAccount(ctx context.Context, accounts *AccountService, tenantID string, appID string, status authorization.AccountStatus) error {
	_, err := accounts.SaveAccount(ctx, SaveAccountInput{
		TenantID: tenantID, AppID: appID, Name: appID, Status: status,
	})
	return err
}

func seedDashboardPublish(ctx context.Context, publishes *PublishService, tenantID string, articleID int64, publishID string, status publish.Status) error {
	record, err := publishes.CreatePublishRecord(ctx, CreatePublishRecordInput{
		TenantID: tenantID, ArticleID: articleID, WeChatPublishID: publishID,
	})
	if err != nil {
		return err
	}
	if status == publish.StatusPublishing {
		return nil
	}
	_, err = publishes.UpdatePublishStatus(ctx, UpdatePublishStatusInput{
		TenantID: tenantID, ID: record.ID, Status: status,
	})
	return err
}
