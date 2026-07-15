package postgres

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"official-account-service/internal/domain/agentaudit"
	"official-account-service/internal/domain/article"
	"official-account-service/internal/domain/authorization"
	"official-account-service/internal/domain/identity"
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
	appID := "wx-" + tenantID
	user, err := store.SaveUser(ctx, identity.User{
		ID: tenantID, Username: "user-" + tenantID, PasswordHash: "hash", Role: identity.RoleUser, Status: identity.StatusActive,
	})
	require.NoError(t, err)
	require.Equal(t, tenantID, user.ID)
	byUsername, err := store.GetUserByUsername(ctx, strings.ToUpper(user.Username))
	require.NoError(t, err)
	require.Equal(t, tenantID, byUsername.ID)
	tokenHash := fmt.Sprintf("%064x", time.Now().UnixNano())
	withToken, err := store.SaveUserAPIToken(ctx, tenantID, tokenHash, "oat_...abc123")
	require.NoError(t, err)
	require.Equal(t, tokenHash, withToken.APITokenHash)
	require.NotNil(t, withToken.APITokenCreatedAt)
	byToken, err := store.GetUserByAPITokenHash(ctx, tokenHash)
	require.NoError(t, err)
	require.Equal(t, tenantID, byToken.ID)
	revokedTokenUser, err := store.RevokeUserAPIToken(ctx, tenantID)
	require.NoError(t, err)
	require.Empty(t, revokedTokenUser.APITokenHash)
	account, err := store.SaveAccount(ctx, tenantID, authorization.Account{
		AppID: appID, Name: "account", Status: authorization.AccountStatusActive,
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
		AppID: appID, Name: "renamed", Status: authorization.AccountStatusActive,
		EncryptedAuthorizerRefreshToken: "encrypted-refresh-2",
	})
	require.NoError(t, err)
	require.Equal(t, account.ID, saved.ID)
	require.Equal(t, "renamed", saved.Name)
	require.Equal(t, "encrypted-refresh-2", saved.EncryptedAuthorizerRefreshToken)
	_, err = store.SaveAccount(ctx, tenantID+"-other", authorization.Account{
		AppID: appID, Name: "not-owner", Status: authorization.AccountStatusActive,
	})
	require.ErrorIs(t, err, authorization.ErrConflict)

	_, err = store.SaveAuthorizerTenantBinding(ctx, authorization.AuthorizerTenantBinding{
		ComponentAppID: "wx-component", AuthorizerAppID: appID, TenantID: tenantID,
	})
	require.NoError(t, err)
	binding, err := store.GetAuthorizerTenantBinding(ctx, "wx-component", appID)
	require.NoError(t, err)
	require.Equal(t, tenantID, binding.TenantID)
	stateDigest := "state-digest-" + tenantID
	authorizationState, err := store.SaveAuthorizationState(ctx, authorization.AuthorizationState{
		Digest: stateDigest, TenantID: tenantID, ComponentAppID: "wx-component", ExpiresAt: time.Now().Add(10 * time.Minute),
	})
	require.NoError(t, err)
	consumedState, err := store.ConsumeAuthorizationState(ctx, authorizationState.Digest, time.Now())
	require.NoError(t, err)
	require.Equal(t, tenantID, consumedState.TenantID)
	_, err = store.ConsumeAuthorizationState(ctx, authorizationState.Digest, time.Now())
	require.Error(t, err)
	require.True(t, errors.Is(err, authorization.ErrAuthorizationStateNotFound))

	revoked, err := store.RevokeAccountByAppID(ctx, tenantID, appID)
	require.NoError(t, err)
	require.Equal(t, authorization.AccountStatusRevoked, revoked.Status)
	require.Empty(t, revoked.EncryptedAuthorizerRefreshToken)

	accounts, err := store.ListAccounts(ctx, tenantID)
	require.NoError(t, err)
	require.Len(t, accounts, 1)
	require.Equal(t, account.ID, accounts[0].ID)

	agentA, err := store.CreateAgent(ctx, identity.Agent{
		ID: "agt-a-" + tenantID, UserID: tenantID, AgentID: "writer-a", Name: "Writer A", Status: identity.StatusActive,
	})
	require.NoError(t, err)
	agentB, err := store.CreateAgent(ctx, identity.Agent{
		ID: "agt-b-" + tenantID, UserID: tenantID, AgentID: "writer-b", Name: "Writer B", Status: identity.StatusActive,
	})
	require.NoError(t, err)

	created, err := store.Create(ctx, tenantID, article.Article{
		AuthorizerID: account.ID, Title: "hello", Status: article.StatusDraft,
		CreatedByAgentID: agentA.ID, UpdatedByAgentID: agentA.ID,
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), created.Version)
	require.Equal(t, agentA.ID, created.CreatedByAgentID)

	updated, err := store.Update(ctx, tenantID, article.Article{
		ID: created.ID, AuthorizerID: account.ID, Title: "updated", Status: article.StatusDraft,
		UpdatedByAgentID: agentB.ID, Version: created.Version,
	})
	require.NoError(t, err)
	require.Equal(t, "updated", updated.Title)
	require.Equal(t, int64(2), updated.Version)
	require.Equal(t, agentA.ID, updated.CreatedByAgentID)
	require.Equal(t, agentB.ID, updated.UpdatedByAgentID)

	_, err = store.Update(ctx, tenantID, article.Article{ID: created.ID, Title: "stale", Status: article.StatusDraft, Version: created.Version})
	require.ErrorIs(t, err, article.ErrVersionConflict)

	concurrentDraft, err := store.Create(ctx, tenantID, article.Article{
		AuthorizerID: account.ID, Title: "concurrent", Status: article.StatusDraft,
	})
	require.NoError(t, err)
	startUpdates := make(chan struct{})
	updateResults := make(chan error, 2)
	for _, title := range []string{"concurrent-a", "concurrent-b"} {
		draft := concurrentDraft
		draft.Title = title
		go func() {
			<-startUpdates
			_, updateErr := store.Update(ctx, tenantID, draft)
			updateResults <- updateErr
		}()
	}
	close(startUpdates)
	succeededUpdates := 0
	conflictedUpdates := 0
	for range 2 {
		updateErr := <-updateResults
		switch {
		case updateErr == nil:
			succeededUpdates++
		case errors.Is(updateErr, article.ErrVersionConflict):
			conflictedUpdates++
		default:
			require.NoError(t, updateErr)
		}
	}
	require.Equal(t, 1, succeededUpdates)
	require.Equal(t, 1, conflictedUpdates)

	items, err := store.List(ctx, tenantID)
	require.NoError(t, err)
	require.Len(t, items, 2)
	agentArticles, err := store.ListByCreatedByAgent(ctx, tenantID, agentA.ID)
	require.NoError(t, err)
	require.Len(t, agentArticles, 1)
	otherAgentArticles, err := store.ListByCreatedByAgent(ctx, tenantID, agentB.ID)
	require.NoError(t, err)
	require.Empty(t, otherAgentArticles)

	asset, err := store.CreateMaterial(ctx, tenantID, material.Asset{AuthorizerID: account.ID, ArticleID: created.ID, Usage: material.UsageInlineImage, LocalURL: "body.png", WeChatURL: "https://wechat.example/body.png"})
	require.NoError(t, err)
	materials, err := store.ListMaterialByArticle(ctx, tenantID, created.ID)
	require.NoError(t, err)
	require.Len(t, materials, 1)
	require.Equal(t, asset.ID, materials[0].ID)

	publishID := fmt.Sprintf("pub-%d", time.Now().UnixNano())
	record, err := store.CreatePublishRecord(ctx, tenantID, publish.Record{AuthorizerID: account.ID, ArticleID: created.ID, WeChatPublishID: publishID, Status: publish.StatusPublishing, SubmittedAt: time.Now()})
	require.NoError(t, err)
	require.Equal(t, agentA.ID, record.ArticleCreatedByAgentID)
	record.Status = publish.StatusPublished
	record.WeChatArticleID = "article-1"
	record.FinishedAt = time.Now()
	updatedRecord, err := store.UpdatePublishRecordStatus(ctx, tenantID, record)
	require.NoError(t, err)
	require.Equal(t, publish.StatusPublished, updatedRecord.Status)
	require.Equal(t, agentA.ID, updatedRecord.ArticleCreatedByAgentID)
	records, err := store.ListPublishRecordsByArticle(ctx, tenantID, created.ID)
	require.NoError(t, err)
	require.Len(t, records, 1)
	require.Equal(t, agentA.ID, records[0].ArticleCreatedByAgentID)
	agentRecords, err := store.ListPublishRecordsByAgent(ctx, tenantID, agentA.ID)
	require.NoError(t, err)
	require.Len(t, agentRecords, 1)
	otherAgentRecords, err := store.ListPublishRecordsByAgent(ctx, tenantID, agentB.ID)
	require.NoError(t, err)
	require.Empty(t, otherAgentRecords)
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

	_, err = store.Append(ctx, agentaudit.Entry{
		UserID: tenantID, AgentRecordID: agentA.ID, Action: agentaudit.ActionCreateArticle,
		ResourceType: agentaudit.ResourceArticle, ResourceID: fmt.Sprint(created.ID),
	})
	require.NoError(t, err)
	_, err = store.Append(ctx, agentaudit.Entry{
		UserID: tenantID, AgentRecordID: agentB.ID, Action: agentaudit.ActionUpdateArticle,
		ResourceType: agentaudit.ResourceArticle, ResourceID: fmt.Sprint(created.ID),
	})
	require.NoError(t, err)
	agentAudit, err := store.ListAuditEntries(ctx, tenantID, agentaudit.Filter{AgentRecordID: agentB.ID, Limit: 10})
	require.NoError(t, err)
	require.Len(t, agentAudit, 1)
	require.Equal(t, agentaudit.ActionUpdateArticle, agentAudit[0].Action)
	otherTenantAudit, err := store.ListAuditEntries(ctx, tenantID+"-other", agentaudit.Filter{Limit: 10})
	require.NoError(t, err)
	require.Empty(t, otherTenantAudit)
	_, err = store.Append(ctx, agentaudit.Entry{
		UserID: tenantID, AgentRecordID: "agt-not-owned", Action: agentaudit.ActionUpdateArticle,
		ResourceType: agentaudit.ResourceArticle, ResourceID: fmt.Sprint(created.ID),
	})
	require.Error(t, err)

	callbackEvent, err := store.SaveCallbackEvent(ctx, wechatcallback.Event{
		TenantID: tenantID, ComponentAppID: "wx-component", AuthorizerAppID: appID,
		EventType: wechatcallback.EventTypePublishResult, EventKey: publishID, RawBody: "<xml></xml>",
		ReceivedAt: time.Now(), RetainUntil: time.Now().Add(30 * 24 * time.Hour),
	})
	require.NoError(t, err)
	require.NotZero(t, callbackEvent.ID)
	_, err = store.GetCallbackEventByKey(ctx, tenantID, wechatcallback.EventTypePublishResult, publishID)
	require.NoError(t, err)
	_, err = store.SaveCallbackEvent(ctx, wechatcallback.Event{
		TenantID: tenantID, ComponentAppID: "wx-component", AuthorizerAppID: appID,
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
	agentRecords, err = store.ListPublishRecordsByAgent(ctx, tenantID, agentA.ID)
	require.NoError(t, err)
	require.Len(t, agentRecords, 1)
	require.Equal(t, record.ID, agentRecords[0].ID)
	require.Equal(t, agentA.ID, agentRecords[0].ArticleCreatedByAgentID)
}

func TestStoreAuditedMutationsRollbackOnAuditFailure(t *testing.T) {
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("set POSTGRES_TEST_DSN to run postgres integration test")
	}
	ctx := context.Background()
	store, err := Open(ctx, dsn)
	require.NoError(t, err)
	defer func() { require.NoError(t, store.Close()) }()
	runMigrations(t, store)

	tenantID := fmt.Sprintf("audit-tenant-%d", time.Now().UnixNano())
	_, err = store.SaveUser(ctx, identity.User{
		ID: tenantID, Username: tenantID, PasswordHash: "hash", Role: identity.RoleUser, Status: identity.StatusActive,
	})
	require.NoError(t, err)
	invalidAudit := agentaudit.Entry{UserID: tenantID, ResourceType: agentaudit.ResourceArticle}

	_, err = store.CreateWithAudit(ctx, tenantID, article.Article{
		AuthorizerID: 1, Title: "must roll back", Status: article.StatusDraft,
	}, invalidAudit)
	require.Error(t, err)
	articles, err := store.List(ctx, tenantID)
	require.NoError(t, err)
	require.Empty(t, articles)

	created, err := store.CreateWithAudit(ctx, tenantID, article.Article{
		AuthorizerID: 1, Title: "original", Status: article.StatusDraft,
	}, agentaudit.Entry{
		UserID: tenantID, Action: agentaudit.ActionCreateArticle, ResourceType: agentaudit.ResourceArticle,
	})
	require.NoError(t, err)

	changed := created
	changed.Title = "must roll back"
	_, err = store.UpdateWithAudit(ctx, tenantID, changed, invalidAudit)
	require.Error(t, err)
	current, err := store.Get(ctx, tenantID, created.ID)
	require.NoError(t, err)
	require.Equal(t, "original", current.Title)
	require.Equal(t, int64(1), current.Version)

	_, err = store.CreatePublishRecordWithAudit(ctx, tenantID, publish.Record{
		AuthorizerID: 1, ArticleID: created.ID, Status: publish.StatusPublishing,
	}, invalidAudit)
	require.Error(t, err)
	records, err := store.ListPublishRecordsByArticle(ctx, tenantID, created.ID)
	require.NoError(t, err)
	require.Empty(t, records)

	err = store.DeleteWithAudit(ctx, tenantID, created.ID, invalidAudit)
	require.Error(t, err)
	_, err = store.Get(ctx, tenantID, created.ID)
	require.NoError(t, err)

	audits, err := store.ListAuditEntries(ctx, tenantID, agentaudit.Filter{Limit: 10})
	require.NoError(t, err)
	require.Len(t, audits, 1)
	require.Equal(t, agentaudit.ActionCreateArticle, audits[0].Action)
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
		"../../../../migrations/009_publish_in_progress_unique.sql",
		"../../../../migrations/010_app_user.sql",
		"../../../../migrations/011_account_global_owner.sql",
		"../../../../migrations/012_user_api_token.sql",
		"../../../../migrations/013_multi_agent_token.sql",
		"../../../../migrations/014_article_agent_audit.sql",
		"../../../../migrations/015_publish_record_agent_attribution.sql",
	} {
		sqlBytes, err := os.ReadFile(path)
		require.NoError(t, err)
		_, err = store.db.Exec(string(sqlBytes))
		require.NoError(t, err)
	}
}
