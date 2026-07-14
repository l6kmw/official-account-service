package postgres

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"official-account-service/internal/domain/article"
	"official-account-service/internal/domain/authorization"
	"official-account-service/internal/domain/material"
	"official-account-service/internal/domain/publish"
	"official-account-service/internal/domain/wechatcallback"
)

func TestStoreArticleAndAccountIntegration(t *testing.T) {
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("set POSTGRES_TEST_DSN to run postgres integration test")
	}
	ctx := context.Background()
	store, err := Open(ctx, dsn)
	require.NoError(t, err)
	defer func() { require.NoError(t, store.Close()) }()
	runMigrations(t, store)

	tenantID := fmt.Sprintf("tenant-%d", time.Now().UnixNano())
	account, err := store.SaveAccount(ctx, tenantID, authorization.Account{
		AppID: "wx123", Name: "account", Status: authorization.AccountStatusActive,
		EncryptedAuthorizerRefreshToken: "encrypted-refresh-1",
	})
	require.NoError(t, err)
	require.Equal(t, "encrypted-refresh-1", account.EncryptedAuthorizerRefreshToken)

	receivedAt := time.Date(2026, 7, 6, 17, 0, 0, 0, time.UTC)
	componentTicket, err := store.SaveComponentVerifyTicket(ctx, authorization.ComponentVerifyTicket{
		ComponentAppID: "wx-component",
		Ticket:         "ticket-1",
		ReceivedAt:     receivedAt,
	})
	require.NoError(t, err)
	require.True(t, receivedAt.Equal(componentTicket.ReceivedAt))
	_, err = store.SaveComponentVerifyTicket(ctx, authorization.ComponentVerifyTicket{
		ComponentAppID: "wx-component",
		Ticket:         "ticket-2",
		ReceivedAt:     receivedAt.Add(time.Minute),
	})
	require.NoError(t, err)
	foundTicket, err := store.GetComponentVerifyTicket(ctx, "wx-component")
	require.NoError(t, err)
	require.Equal(t, "ticket-2", foundTicket.Ticket)

	saved, err := store.SaveAccount(ctx, tenantID, authorization.Account{
		AppID: "wx123", Name: "renamed", Status: authorization.AccountStatusActive,
		EncryptedAuthorizerRefreshToken: "encrypted-refresh-2",
	})
	require.NoError(t, err)
	require.Equal(t, account.ID, saved.ID)
	require.Equal(t, "renamed", saved.Name)
	require.Equal(t, "encrypted-refresh-2", saved.EncryptedAuthorizerRefreshToken)

	_, err = store.SaveAuthorizerTenantBinding(ctx, authorization.AuthorizerTenantBinding{
		ComponentAppID: "wx-component", AuthorizerAppID: "wx123", TenantID: tenantID,
	})
	require.NoError(t, err)
	binding, err := store.GetAuthorizerTenantBinding(ctx, "wx-component", "wx123")
	require.NoError(t, err)
	require.Equal(t, tenantID, binding.TenantID)
	authorizationState, err := store.SaveAuthorizationState(ctx, authorization.AuthorizationState{
		Digest: "state-digest", TenantID: tenantID, ComponentAppID: "wx-component", ExpiresAt: time.Now().Add(10 * time.Minute),
	})
	require.NoError(t, err)
	consumedState, err := store.ConsumeAuthorizationState(ctx, authorizationState.Digest, time.Now())
	require.NoError(t, err)
	require.Equal(t, tenantID, consumedState.TenantID)
	_, err = store.ConsumeAuthorizationState(ctx, authorizationState.Digest, time.Now())
	require.Error(t, err)
	require.True(t, errors.Is(err, authorization.ErrAuthorizationStateNotFound))

	revoked, err := store.RevokeAccountByAppID(ctx, tenantID, "wx123")
	require.NoError(t, err)
	require.Equal(t, authorization.AccountStatusRevoked, revoked.Status)
	require.Empty(t, revoked.EncryptedAuthorizerRefreshToken)

	accounts, err := store.ListAccounts(ctx, tenantID)
	require.NoError(t, err)
	require.Len(t, accounts, 1)
	require.Equal(t, account.ID, accounts[0].ID)

	created, err := store.Create(ctx, tenantID, article.Article{AuthorizerID: account.ID, Title: "hello", Status: article.StatusDraft})
	require.NoError(t, err)

	updated, err := store.Update(ctx, tenantID, article.Article{ID: created.ID, AuthorizerID: account.ID, Title: "updated", Status: article.StatusDraft})
	require.NoError(t, err)
	require.Equal(t, "updated", updated.Title)

	items, err := store.List(ctx, tenantID)
	require.NoError(t, err)
	require.Len(t, items, 1)

	asset, err := store.CreateMaterial(ctx, tenantID, material.Asset{AuthorizerID: account.ID, ArticleID: created.ID, Usage: material.UsageInlineImage, LocalURL: "body.png", WeChatURL: "https://wechat.example/body.png"})
	require.NoError(t, err)
	materials, err := store.ListMaterialByArticle(ctx, tenantID, created.ID)
	require.NoError(t, err)
	require.Len(t, materials, 1)
	require.Equal(t, asset.ID, materials[0].ID)

	publishID := fmt.Sprintf("pub-%d", time.Now().UnixNano())
	record, err := store.CreatePublishRecord(ctx, tenantID, publish.Record{AuthorizerID: account.ID, ArticleID: created.ID, WeChatPublishID: publishID, Status: publish.StatusPublishing, SubmittedAt: time.Now()})
	require.NoError(t, err)
	record.Status = publish.StatusPublished
	record.WeChatArticleID = "article-1"
	record.FinishedAt = time.Now()
	updatedRecord, err := store.UpdatePublishRecordStatus(ctx, tenantID, record)
	require.NoError(t, err)
	require.Equal(t, publish.StatusPublished, updatedRecord.Status)
	records, err := store.ListPublishRecordsByArticle(ctx, tenantID, created.ID)
	require.NoError(t, err)
	require.Len(t, records, 1)
	allRecords, err := store.ListPublishRecords(ctx, tenantID)
	require.NoError(t, err)
	require.Len(t, allRecords, 1)
	require.Equal(t, updatedRecord.ID, allRecords[0].ID)
	otherTenantRecords, err := store.ListPublishRecords(ctx, tenantID+"-other")
	require.NoError(t, err)
	require.Empty(t, otherTenantRecords)
	byPublishID, err := store.GetPublishRecordByPublishID(ctx, tenantID, publishID)
	require.NoError(t, err)
	require.Equal(t, updatedRecord.ID, byPublishID.ID)

	callbackEvent, err := store.SaveCallbackEvent(ctx, wechatcallback.Event{
		TenantID: tenantID, ComponentAppID: "wx-component", AuthorizerAppID: "wx123",
		EventType: wechatcallback.EventTypePublishResult, EventKey: publishID, RawBody: "<xml></xml>",
		ReceivedAt: time.Now(), RetainUntil: time.Now().Add(30 * 24 * time.Hour),
	})
	require.NoError(t, err)
	require.NotZero(t, callbackEvent.ID)
	_, err = store.GetCallbackEventByKey(ctx, tenantID, wechatcallback.EventTypePublishResult, publishID)
	require.NoError(t, err)
	_, err = store.SaveCallbackEvent(ctx, wechatcallback.Event{
		TenantID: tenantID, ComponentAppID: "wx-component", AuthorizerAppID: "wx123",
		EventType: wechatcallback.EventTypePublishResult, EventKey: publishID, RawBody: "<xml></xml>",
		ReceivedAt: time.Now(), RetainUntil: time.Now().Add(30 * 24 * time.Hour),
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, wechatcallback.ErrDuplicate))

	err = store.Delete(ctx, tenantID, created.ID)
	require.NoError(t, err)

	_, err = store.Get(ctx, tenantID, created.ID)
	require.Error(t, err)
	require.True(t, errors.Is(err, article.ErrNotFound))
}

func runMigrations(t *testing.T, store *Store) {
	t.Helper()
	for _, path := range []string{
		"../../../../migrations/001_initial_schema.sql",
		"../../../../migrations/002_wechat_authorization_account.sql",
		"../../../../migrations/003_wechat_component_verify_ticket.sql",
		"../../../../migrations/004_authorizer_refresh_token.sql",
		"../../../../migrations/005_authorizer_tenant_binding.sql",
		"../../../../migrations/006_wechat_callback_event.sql",
		"../../../../migrations/007_media_asset_article_fk.sql",
		"../../../../migrations/008_authorization_state.sql",
	} {
		sqlBytes, err := os.ReadFile(path)
		require.NoError(t, err)
		_, err = store.db.Exec(string(sqlBytes))
		require.NoError(t, err)
	}
}
