package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"official-account-service/internal/domain/agentaudit"
	"official-account-service/internal/domain/article"
	"official-account-service/internal/domain/authorization"
	"official-account-service/internal/infra/persistence/memory"
)

func TestArticleServiceCreateListGetUpdateDeleteArticles(t *testing.T) {
	ctx := context.Background()
	store := memory.NewStore(func() time.Time { return time.Date(2026, 7, 6, 16, 0, 0, 0, time.UTC) })
	service := NewArticleService(store)

	created, err := service.CreateArticle(ctx, CreateArticleInput{TenantID: "tenant-1", AuthorizerID: 1, Title: "hello", ContentHTML: "<p>body</p>"})
	require.NoError(t, err)
	require.Equal(t, article.StatusDraft, created.Status)

	items, err := service.ListArticles(ctx, "tenant-1")
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "hello", items[0].Title)

	got, err := service.GetArticle(ctx, "tenant-1", created.ID)
	require.NoError(t, err)
	require.Equal(t, created.ID, got.ID)

	updated, err := service.UpdateArticle(ctx, UpdateArticleInput{TenantID: "tenant-1", ID: created.ID, Title: "updated", Author: "me", Digest: "sum", ContentHTML: "<p>new</p>", CoverMediaAssetID: 3})
	require.NoError(t, err)
	require.Equal(t, "updated", updated.Title)
	require.Equal(t, int64(3), updated.CoverMediaAssetID)

	otherTenantItems, err := service.ListArticles(ctx, "tenant-2")
	require.NoError(t, err)
	require.Empty(t, otherTenantItems)

	err = service.DeleteArticle(ctx, "tenant-1", created.ID)
	require.NoError(t, err)

	_, err = service.GetArticle(ctx, "tenant-1", created.ID)
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrNotFound))
}

func TestArticleServiceValidatesRequiredFields(t *testing.T) {
	ctx := context.Background()
	service := NewArticleService(memory.NewStore(time.Now))

	_, err := service.CreateArticle(ctx, CreateArticleInput{TenantID: "", AuthorizerID: 1, Title: "hello"})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))

	_, err = service.CreateArticle(ctx, CreateArticleInput{TenantID: "tenant", AuthorizerID: 0, Title: "hello"})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))

	_, err = service.CreateArticle(ctx, CreateArticleInput{TenantID: "tenant", AuthorizerID: 1, Title: ""})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))

	_, err = service.GetArticle(ctx, "tenant", 0)
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))

	_, err = service.UpdateArticle(ctx, UpdateArticleInput{TenantID: "tenant", ID: 1, Title: ""})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))
}

func TestArticleServiceValidatesAuthorizerOwnership(t *testing.T) {
	ctx := context.Background()
	store := memory.NewStore(func() time.Time { return time.Date(2026, 7, 6, 16, 0, 0, 0, time.UTC) })
	service := NewArticleServiceWithAuthorizerRepository(store, store)
	active, err := store.SaveAccount(ctx, "tenant-1", authorization.Account{
		AppID: "wx-active", Status: authorization.AccountStatusActive,
	})
	require.NoError(t, err)
	revoked, err := store.SaveAccount(ctx, "tenant-1", authorization.Account{
		AppID: "wx-revoked", Status: authorization.AccountStatusRevoked,
	})
	require.NoError(t, err)

	_, err = service.CreateArticle(ctx, CreateArticleInput{TenantID: "tenant-1", AuthorizerID: active.ID, Title: "hello"})
	require.NoError(t, err)

	_, err = service.CreateArticle(ctx, CreateArticleInput{TenantID: "tenant-2", AuthorizerID: active.ID, Title: "hello"})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))

	_, err = service.CreateArticle(ctx, CreateArticleInput{TenantID: "tenant-1", AuthorizerID: revoked.ID, Title: "hello"})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))

	_, err = service.CreateArticle(ctx, CreateArticleInput{TenantID: "tenant-1", AuthorizerID: 999, Title: "hello"})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))
}

func TestArticleServiceAttributesAndAuditsAgentMutations(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	store := memory.NewStore(func() time.Time { return now })
	audits := &recordingAuditRepository{now: now}
	service := NewArticleServiceWithRepositories(store, nil, audits)
	creator := Actor{UserID: "user-1", ActorType: ActorTypeAgentToken, AgentRecordID: "agent-a"}
	editor := Actor{UserID: "user-1", ActorType: ActorTypeAgentToken, AgentRecordID: "agent-b"}

	created, err := service.CreateArticle(ctx, CreateArticleInput{
		TenantID: "user-1", AuthorizerID: 1, Title: "hello", ContentHTML: "<p>sensitive-body</p>", Actor: creator,
	})
	require.NoError(t, err)
	require.Equal(t, "agent-a", created.CreatedByAgentID)
	require.Equal(t, "agent-a", created.UpdatedByAgentID)

	updated, err := service.UpdateArticle(ctx, UpdateArticleInput{
		TenantID: "user-1", ID: created.ID, Title: "updated", ContentHTML: "<p>new-sensitive-body</p>",
		Version: created.Version, Actor: editor,
	})
	require.NoError(t, err)
	require.Equal(t, "agent-a", updated.CreatedByAgentID)
	require.Equal(t, "agent-b", updated.UpdatedByAgentID)

	require.NoError(t, service.DeleteArticleWithActor(ctx, DeleteArticleInput{TenantID: "user-1", ID: created.ID, Actor: editor}))
	require.Equal(t, []agentaudit.Action{
		agentaudit.ActionCreateArticle, agentaudit.ActionUpdateArticle, agentaudit.ActionDeleteArticle,
	}, auditActions(audits.entries))
	require.Equal(t, "agent-a", audits.entries[0].AgentRecordID)
	require.Equal(t, "agent-b", audits.entries[1].AgentRecordID)
	require.Equal(t, fmt.Sprint(created.ID), audits.entries[2].ResourceID)
	raw, err := json.Marshal(audits.entries)
	require.NoError(t, err)
	for _, forbidden := range []string{"sensitive-body", "new-sensitive-body", "api_token", "image_url", "access_token", "refresh_token"} {
		require.NotContains(t, string(raw), forbidden)
	}
}

func TestArticleServiceRejectsForgedActorAndMapsVersionConflict(t *testing.T) {
	ctx := context.Background()
	store := memory.NewStore(time.Now)
	service := NewArticleServiceWithRepositories(store, nil, &recordingAuditRepository{now: time.Now()})

	_, err := service.CreateArticle(ctx, CreateArticleInput{
		TenantID: "user-1", AuthorizerID: 1, Title: "wrong user",
		Actor: Actor{UserID: "user-2", ActorType: ActorTypeAgentToken, AgentRecordID: "agent-a"},
	})
	require.ErrorIs(t, err, ErrInvalidInput)

	created, err := service.CreateArticle(ctx, CreateArticleInput{
		TenantID: "user-1", AuthorizerID: 1, Title: "browser",
		Actor: Actor{UserID: "user-1", ActorType: ActorTypeBrowserSession, AgentRecordID: "forged-agent"},
	})
	require.NoError(t, err)
	require.Empty(t, created.CreatedByAgentID)

	conflicting := NewArticleService(conflictingArticleRepository{Repository: store})
	_, err = conflicting.UpdateArticle(ctx, UpdateArticleInput{
		TenantID: "user-1", ID: created.ID, Title: "stale", Version: created.Version,
	})
	require.ErrorIs(t, err, ErrConflict)
}

func TestAgentAuditServiceFiltersSafeMetadata(t *testing.T) {
	ctx := context.Background()
	repository := &recordingAuditRepository{now: time.Now()}
	_, _ = repository.Append(ctx, agentaudit.Entry{UserID: "user-1", AgentRecordID: "agent-a", Action: agentaudit.ActionCreateArticle, ResourceType: agentaudit.ResourceArticle, ResourceID: "1"})
	_, _ = repository.Append(ctx, agentaudit.Entry{UserID: "user-1", AgentRecordID: "agent-b", Action: agentaudit.ActionUpdateArticle, ResourceType: agentaudit.ResourceArticle, ResourceID: "2"})
	_, _ = repository.Append(ctx, agentaudit.Entry{UserID: "user-2", AgentRecordID: "agent-a", Action: agentaudit.ActionCreateArticle, ResourceType: agentaudit.ResourceArticle, ResourceID: "3"})

	entries, err := NewAgentAuditService(repository).ListAgentAudit(ctx, ListAgentAuditInput{UserID: "user-1", AgentRecordID: "agent-a"})
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "1", entries[0].ResourceID)
	_, err = NewAgentAuditService(repository).ListAgentAudit(ctx, ListAgentAuditInput{UserID: "user-1", Limit: maxAuditLimit + 1})
	require.ErrorIs(t, err, ErrInvalidInput)
}

type conflictingArticleRepository struct {
	article.Repository
}

func (r conflictingArticleRepository) Update(context.Context, string, article.Article) (article.Article, error) {
	return article.Article{}, article.ErrVersionConflict
}

type recordingAuditRepository struct {
	entries []agentaudit.Entry
	now     time.Time
}

func (r *recordingAuditRepository) Append(_ context.Context, entry agentaudit.Entry) (agentaudit.Entry, error) {
	entry.ID = int64(len(r.entries) + 1)
	entry.CreatedAt = r.now
	r.entries = append(r.entries, entry)
	return entry, nil
}

func (r *recordingAuditRepository) List(_ context.Context, userID string, filter agentaudit.Filter) ([]agentaudit.Entry, error) {
	items := make([]agentaudit.Entry, 0)
	for index := len(r.entries) - 1; index >= 0; index-- {
		entry := r.entries[index]
		if entry.UserID != userID || filter.AgentRecordID != "" && entry.AgentRecordID != filter.AgentRecordID || filter.Action != "" && entry.Action != filter.Action || filter.ResourceType != "" && entry.ResourceType != filter.ResourceType {
			continue
		}
		items = append(items, entry)
		if filter.Limit > 0 && len(items) == filter.Limit {
			break
		}
	}
	return items, nil
}

func auditActions(entries []agentaudit.Entry) []agentaudit.Action {
	actions := make([]agentaudit.Action, 0, len(entries))
	for _, entry := range entries {
		actions = append(actions, entry.Action)
	}
	return actions
}
