package application

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"official-account-service/internal/domain/material"
	"official-account-service/internal/infra/persistence/memory"
)

func TestMaterialServiceUploadsInlineImageAndCoverSeparately(t *testing.T) {
	ctx := context.Background()
	store := memory.NewStore(func() time.Time { return time.Date(2026, 7, 6, 16, 0, 0, 0, time.UTC) })
	service := NewMaterialService(store, &fakeUploader{})

	inline, err := service.UploadInlineImage(ctx, UploadMaterialInput{TenantID: "tenant-1", AuthorizerID: 1, ArticleID: 2, Filename: "body.png", Content: strings.NewReader("inline")})
	require.NoError(t, err)
	require.Equal(t, material.UsageInlineImage, inline.Usage)
	require.Equal(t, "https://wechat.example/body.png", inline.WeChatURL)
	require.Empty(t, inline.MediaID)

	cover, err := service.UploadCover(ctx, UploadMaterialInput{TenantID: "tenant-1", AuthorizerID: 1, ArticleID: 2, Filename: "cover.png", Content: strings.NewReader("cover")})
	require.NoError(t, err)
	require.Equal(t, material.UsageCover, cover.Usage)
	require.Equal(t, "media-cover", cover.MediaID)
	require.Empty(t, cover.WeChatURL)

	items, err := service.ListMaterialsByArticle(ctx, "tenant-1", 2)
	require.NoError(t, err)
	require.Len(t, items, 2)
}

func TestMaterialServiceUsesAuthorizerAccessToken(t *testing.T) {
	ctx := context.Background()
	store := memory.NewStore(func() time.Time { return time.Date(2026, 7, 6, 16, 0, 0, 0, time.UTC) })
	uploader := &fakeUploader{}
	tokens := &fakeMaterialTokenProvider{token: AuthorizerAccessToken{AccessToken: "authorizer-token"}}
	service := NewMaterialServiceWithTokenProvider(store, uploader, tokens, "wx-component")

	_, err := service.UploadInlineImage(ctx, UploadMaterialInput{TenantID: "tenant-1", AuthorizerID: 7, ArticleID: 2, Filename: "body.png", Content: strings.NewReader("inline")})
	require.NoError(t, err)

	require.Equal(t, "authorizer-token", uploader.lastToken)
	require.Equal(t, "tenant-1", tokens.lastInput.TenantID)
	require.Equal(t, int64(7), tokens.lastInput.AccountID)
	require.Equal(t, "wx-component", tokens.lastInput.ComponentAppID)
}

func TestMaterialServiceValidatesUploadInput(t *testing.T) {
	ctx := context.Background()
	service := NewMaterialService(memory.NewStore(time.Now), &fakeUploader{})

	_, err := service.UploadInlineImage(ctx, UploadMaterialInput{TenantID: "", AuthorizerID: 1, ArticleID: 2, Filename: "a.png", Content: strings.NewReader("x")})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))

	_, err = service.UploadInlineImage(ctx, UploadMaterialInput{TenantID: "tenant", AuthorizerID: 0, ArticleID: 2, Filename: "a.png", Content: strings.NewReader("x")})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))

	_, err = service.UploadCover(ctx, UploadMaterialInput{TenantID: "tenant", AuthorizerID: 1, ArticleID: 2, Filename: "a.png"})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))
}

type fakeUploader struct {
	lastToken string
}

func (u *fakeUploader) UploadInlineImage(_ context.Context, accessToken string, filename string, _ io.Reader) (material.InlineImageUpload, error) {
	u.lastToken = accessToken
	return material.InlineImageUpload{WeChatURL: "https://wechat.example/" + filename}, nil
}

func (u *fakeUploader) UploadCover(_ context.Context, accessToken string, _ string, _ io.Reader) (material.CoverUpload, error) {
	u.lastToken = accessToken
	return material.CoverUpload{MediaID: "media-cover"}, nil
}

type fakeMaterialTokenProvider struct {
	token     AuthorizerAccessToken
	lastInput RefreshAuthorizerAccessTokenInput
}

func (p *fakeMaterialTokenProvider) GetAuthorizerAccessToken(_ context.Context, input RefreshAuthorizerAccessTokenInput) (AuthorizerAccessToken, error) {
	p.lastInput = input
	return p.token, nil
}
