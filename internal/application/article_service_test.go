package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

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
