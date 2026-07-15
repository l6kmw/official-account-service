package memory

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"official-account-service/internal/domain/agentaudit"
	"official-account-service/internal/domain/article"
	"official-account-service/internal/domain/authorization"
	"official-account-service/internal/domain/material"
	"official-account-service/internal/domain/publish"
	"official-account-service/internal/domain/wechatcallback"
)

func TestStoreManagesTenantScopedArticlesMaterialAndPublishRecords(t *testing.T) {
	ctx := context.Background()
	store := NewStore(func() time.Time { return time.Date(2026, 7, 6, 16, 30, 0, 0, time.UTC) })

	article, err := store.Create(ctx, "tenant-1", articleFixture())
	require.NoError(t, err)
	asset, err := store.CreateMaterial(ctx, "tenant-1", material.Asset{ArticleID: article.ID, Usage: material.UsageCover, MediaID: "media-id"})
	require.NoError(t, err)
	record, err := store.CreatePublishRecord(ctx, "tenant-1", publish.Record{ArticleID: article.ID, WeChatPublishID: "publish-1", Status: publish.StatusPublishing})
	require.NoError(t, err)

	otherTenantAssets, err := store.ListMaterialByArticle(ctx, "tenant-2", article.ID)
	require.NoError(t, err)
	require.Empty(t, otherTenantAssets)
	otherTenantRecords, err := store.ListPublishRecordsByArticle(ctx, "tenant-2", article.ID)
	require.NoError(t, err)
	require.Empty(t, otherTenantRecords)
	otherTenantAllRecords, err := store.ListPublishRecords(ctx, "tenant-2")
	require.NoError(t, err)
	require.Empty(t, otherTenantAllRecords)

	foundAsset, err := store.GetMaterial(ctx, "tenant-1", asset.ID)
	require.NoError(t, err)
	require.Equal(t, material.UsageCover, foundAsset.Usage)
	updatedRecord, err := store.UpdatePublishRecordStatus(ctx, "tenant-1", publish.Record{ID: record.ID, ArticleID: article.ID, Status: publish.StatusPublished})
	require.NoError(t, err)
	require.Equal(t, publish.StatusPublished, updatedRecord.Status)
	byPublishID, err := store.GetPublishRecordByPublishID(ctx, "tenant-1", "publish-1")
	require.NoError(t, err)
	require.Equal(t, record.ID, byPublishID.ID)
	records, err := store.ListPublishRecords(ctx, "tenant-1")
	require.NoError(t, err)
	require.Len(t, records, 1)
	require.Equal(t, record.ID, records[0].ID)
	callbackEvent, err := store.SaveCallbackEvent(ctx, wechatcallback.Event{
		TenantID: "tenant-1", ComponentAppID: "wx-component", AuthorizerAppID: "wx-app",
		EventType: wechatcallback.EventTypePublishResult, EventKey: "publish-1", RawBody: "<xml></xml>",
		RetainUntil: time.Date(2026, 8, 5, 16, 30, 0, 0, time.UTC),
	})
	require.NoError(t, err)
	require.NotZero(t, callbackEvent.ID)
	_, err = store.GetCallbackEventByKey(ctx, "tenant-1", wechatcallback.EventTypePublishResult, "publish-1")
	require.NoError(t, err)
	_, err = store.SaveCallbackEvent(ctx, wechatcallback.Event{
		TenantID: "tenant-1", ComponentAppID: "wx-component", AuthorizerAppID: "wx-app",
		EventType: wechatcallback.EventTypePublishResult, EventKey: "publish-1", RawBody: "<xml></xml>",
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, wechatcallback.ErrDuplicate))
}

func articleFixture() article.Article {
	return article.Article{AuthorizerID: 1, Title: "title", Status: article.StatusDraft}
}

func TestStoreArticleOptimisticLockAllowsOneVersionWinner(t *testing.T) {
	ctx := context.Background()
	store := NewStore(time.Now)
	created, err := store.Create(ctx, "user-1", articleFixture())
	require.NoError(t, err)
	require.Equal(t, int64(1), created.Version)

	start := make(chan struct{})
	results := make(chan error, 2)
	for _, title := range []string{"agent-a", "agent-b"} {
		draft := created
		draft.Title = title
		go func() {
			<-start
			_, updateErr := store.Update(ctx, "user-1", draft)
			results <- updateErr
		}()
	}
	close(start)

	succeeded := 0
	conflicted := 0
	for range 2 {
		updateErr := <-results
		switch {
		case updateErr == nil:
			succeeded++
		case errors.Is(updateErr, article.ErrVersionConflict):
			conflicted++
		default:
			require.NoError(t, updateErr)
		}
	}
	require.Equal(t, 1, succeeded)
	require.Equal(t, 1, conflicted)
	latest, err := store.Get(ctx, "user-1", created.ID)
	require.NoError(t, err)
	require.Equal(t, int64(2), latest.Version)
}

func TestStoreFiltersArticlesPublishesAndAuditByAgent(t *testing.T) {
	ctx := context.Background()
	store := NewStore(func() time.Time { return time.Date(2026, 7, 15, 13, 0, 0, 0, time.UTC) })

	draftA, err := store.Create(ctx, "user-1", article.Article{
		AuthorizerID: 1, Title: "article-a", Status: article.StatusDraft, CreatedByAgentID: "agent-a",
	})
	require.NoError(t, err)
	draftB, err := store.Create(ctx, "user-1", article.Article{
		AuthorizerID: 1, Title: "article-b", Status: article.StatusDraft, CreatedByAgentID: "agent-b",
	})
	require.NoError(t, err)
	_, err = store.Create(ctx, "user-2", article.Article{
		AuthorizerID: 2, Title: "other-user", Status: article.StatusDraft, CreatedByAgentID: "agent-a",
	})
	require.NoError(t, err)

	recordA, err := store.CreatePublishRecord(ctx, "user-1", publish.Record{ArticleID: draftA.ID, Status: publish.StatusPublishing})
	require.NoError(t, err)
	_, err = store.CreatePublishRecord(ctx, "user-1", publish.Record{ArticleID: draftB.ID, Status: publish.StatusPublishing})
	require.NoError(t, err)

	articles, err := store.ListByCreatedByAgent(ctx, "user-1", "agent-a")
	require.NoError(t, err)
	require.Len(t, articles, 1)
	require.Equal(t, draftA.ID, articles[0].ID)
	records, err := store.ListPublishRecordsByAgent(ctx, "user-1", "agent-a")
	require.NoError(t, err)
	require.Len(t, records, 1)
	require.Equal(t, recordA.ID, records[0].ID)
	require.Equal(t, "agent-a", records[0].ArticleCreatedByAgentID)
	require.NoError(t, store.Delete(ctx, "user-1", draftA.ID))
	records, err = store.ListPublishRecordsByAgent(ctx, "user-1", "agent-a")
	require.NoError(t, err)
	require.Len(t, records, 1)
	require.Equal(t, recordA.ID, records[0].ID)
	require.Equal(t, "agent-a", records[0].ArticleCreatedByAgentID)

	_, err = store.Append(ctx, agentaudit.Entry{
		UserID: "user-1", AgentRecordID: "agent-a", Action: agentaudit.ActionCreateArticle,
		ResourceType: agentaudit.ResourceArticle, ResourceID: "1",
	})
	require.NoError(t, err)
	_, err = store.Append(ctx, agentaudit.Entry{
		UserID: "user-1", AgentRecordID: "agent-b", Action: agentaudit.ActionUpdateArticle,
		ResourceType: agentaudit.ResourceArticle, ResourceID: "2",
	})
	require.NoError(t, err)
	_, err = store.Append(ctx, agentaudit.Entry{
		UserID: "user-2", AgentRecordID: "agent-a", Action: agentaudit.ActionDeleteArticle,
		ResourceType: agentaudit.ResourceArticle, ResourceID: "3",
	})
	require.NoError(t, err)

	auditEntries, err := store.ListAuditEntries(ctx, "user-1", agentaudit.Filter{AgentRecordID: "agent-a", Limit: 10})
	require.NoError(t, err)
	require.Len(t, auditEntries, 1)
	require.Equal(t, "1", auditEntries[0].ResourceID)
	otherUserAudit, err := store.ListAuditEntries(ctx, "user-2", agentaudit.Filter{Limit: 10})
	require.NoError(t, err)
	require.Len(t, otherUserAudit, 1)
	require.Equal(t, "3", otherUserAudit[0].ResourceID)
}

func TestStoreManagesTenantScopedAccounts(t *testing.T) {
	ctx := context.Background()
	store := NewStore(func() time.Time { return time.Date(2026, 7, 6, 16, 30, 0, 0, time.UTC) })

	created, err := store.CreateAccount(ctx, "tenant-1", authorization.Account{
		AppID: "wx-app", Name: "account", Status: authorization.AccountStatusActive,
		EncryptedAuthorizerRefreshToken: "encrypted-refresh",
	})
	require.NoError(t, err)

	found, err := store.GetAccount(ctx, "tenant-1", created.ID)
	require.NoError(t, err)
	require.Equal(t, "wx-app", found.AppID)
	require.Equal(t, "encrypted-refresh", found.EncryptedAuthorizerRefreshToken)

	otherTenantItems, err := store.ListAccounts(ctx, "tenant-2")
	require.NoError(t, err)
	require.Empty(t, otherTenantItems)

	updated, err := store.UpdateAccountStatus(ctx, "tenant-1", created.ID, authorization.AccountStatusRevoked)
	require.NoError(t, err)
	require.Equal(t, authorization.AccountStatusRevoked, updated.Status)

	_, err = store.SaveAuthorizerTenantBinding(ctx, authorization.AuthorizerTenantBinding{
		ComponentAppID: "wx-component", AuthorizerAppID: "wx-app", TenantID: "tenant-1",
	})
	require.NoError(t, err)
	binding, err := store.GetAuthorizerTenantBinding(ctx, "wx-component", "wx-app")
	require.NoError(t, err)
	require.Equal(t, "tenant-1", binding.TenantID)

	revoked, err := store.RevokeAccountByAppID(ctx, "tenant-1", "wx-app")
	require.NoError(t, err)
	require.Equal(t, authorization.AccountStatusRevoked, revoked.Status)
	require.Empty(t, revoked.EncryptedAuthorizerRefreshToken)
}

func TestStoreManagesComponentVerifyTickets(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 6, 16, 30, 0, 0, time.UTC)
	store := NewStore(func() time.Time { return now })

	_, err := store.GetComponentVerifyTicket(ctx, "wx-component")
	require.Error(t, err)
	require.True(t, errors.Is(err, authorization.ErrComponentVerifyTicketNotFound))

	receivedAt := time.Date(2026, 7, 6, 16, 20, 0, 0, time.UTC)
	saved, err := store.SaveComponentVerifyTicket(ctx, authorization.ComponentVerifyTicket{
		ComponentAppID: "wx-component",
		Ticket:         "ticket-1",
		ReceivedAt:     receivedAt,
	})
	require.NoError(t, err)
	require.Equal(t, now, saved.UpdatedAt)

	_, err = store.SaveComponentVerifyTicket(ctx, authorization.ComponentVerifyTicket{
		ComponentAppID: "wx-component",
		Ticket:         "ticket-2",
		ReceivedAt:     receivedAt,
	})
	require.NoError(t, err)

	got, err := store.GetComponentVerifyTicket(ctx, "wx-component")
	require.NoError(t, err)
	require.Equal(t, "ticket-2", got.Ticket)
}
