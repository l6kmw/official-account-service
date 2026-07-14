package agentapi

import (
	"context"
	"fmt"
	"io"
	"time"

	"official-account-service/internal/application"
	"official-account-service/internal/domain/article"
	"official-account-service/internal/domain/authorization"
	"official-account-service/internal/domain/material"
	"official-account-service/internal/domain/publish"
)

// WeChatAPI exposes stable WeChat functions for AI Agent callers.
type WeChatAPI struct {
	authorization *application.AuthorizationService
	publishes     *application.PublishService
	materials     *application.MaterialService
	accounts      *application.AccountService
	articles      *application.ArticleService
}

// Dependencies contains application services used by WeChatAPI.
type Dependencies struct {
	Authorization *application.AuthorizationService
	Publishes     *application.PublishService
	Materials     *application.MaterialService
	Accounts      *application.AccountService
	Articles      *application.ArticleService
}

// NewWeChatAPI constructs a WeChatAPI.
func NewWeChatAPI(deps Dependencies) *WeChatAPI {
	return &WeChatAPI{
		authorization: deps.Authorization,
		publishes:     deps.Publishes,
		materials:     deps.Materials,
		accounts:      deps.Accounts,
		articles:      deps.Articles,
	}
}

// ImageUsage describes which WeChat image upload path to use.
type ImageUsage string

const (
	// ImageUsageInline uploads a body image and returns a WeChat URL.
	ImageUsageInline ImageUsage = "inline_image"
	// ImageUsageCover uploads a cover image and returns a media id.
	ImageUsageCover ImageUsage = "cover"
)

// AuthorizeInput contains parameters for wechat.authorize().
type AuthorizeInput struct {
	TenantID       string `json:"tenant_id"`
	ComponentAppID string `json:"component_appid"`
	RedirectURI    string `json:"redirect_uri"`
	AuthType       int    `json:"auth_type"`
	BizAppID       string `json:"biz_appid"`
}

// AuthorizeOutput contains the generated WeChat authorization URL.
type AuthorizeOutput struct {
	AuthorizationURL        string `json:"authorization_url"`
	PreAuthCodeExpiresInSec int    `json:"pre_auth_code_expires_in_sec"`
}

// PublishArticleInput contains parameters for wechat.publishArticle().
type PublishArticleInput struct {
	TenantID  string `json:"tenant_id"`
	ArticleID int64  `json:"article_id"`
}

// PublishArticleOutput contains the created or existing publish record.
type PublishArticleOutput struct {
	Record PublishRecord `json:"record"`
}

// UploadImageInput contains parameters for wechat.uploadImage().
type UploadImageInput struct {
	TenantID     string     `json:"tenant_id"`
	AuthorizerID int64      `json:"authorizer_id"`
	ArticleID    int64      `json:"article_id"`
	Usage        ImageUsage `json:"usage"`
	Filename     string     `json:"filename"`
	Content      io.Reader  `json:"-"`
}

// UploadImageOutput contains the uploaded image asset.
type UploadImageOutput struct {
	Asset MaterialAsset `json:"asset"`
}

// ListAccountsInput contains parameters for wechat.listAccounts().
type ListAccountsInput struct {
	TenantID string `json:"tenant_id"`
}

// ListAccountsOutput contains safe account summaries.
type ListAccountsOutput struct {
	Items []Account `json:"items"`
}

// ListArticlesInput contains parameters for wechat.listArticles().
type ListArticlesInput struct {
	TenantID string `json:"tenant_id"`
}

// ListArticlesOutput contains tenant-scoped articles.
type ListArticlesOutput struct {
	Items []Article `json:"items"`
}

// Account is a safe official-account DTO for agent callers.
type Account struct {
	ID           int64     `json:"id"`
	TenantID     string    `json:"tenant_id"`
	AppID        string    `json:"app_id"`
	Name         string    `json:"name"`
	AvatarURL    string    `json:"avatar_url"`
	Status       string    `json:"status"`
	LastSyncedAt time.Time `json:"last_synced_at"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Article is a platform article DTO for agent callers.
type Article struct {
	ID                int64     `json:"id"`
	TenantID          string    `json:"tenant_id"`
	AuthorizerID      int64     `json:"authorizer_id"`
	Title             string    `json:"title"`
	Author            string    `json:"author"`
	Digest            string    `json:"digest"`
	ContentHTML       string    `json:"content_html"`
	CoverMediaAssetID int64     `json:"cover_media_asset_id"`
	Status            string    `json:"status"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// MaterialAsset is a safe material DTO for agent callers.
type MaterialAsset struct {
	ID           int64     `json:"id"`
	TenantID     string    `json:"tenant_id"`
	AuthorizerID int64     `json:"authorizer_id"`
	ArticleID    int64     `json:"article_id"`
	Usage        string    `json:"usage"`
	LocalURL     string    `json:"local_url"`
	WeChatURL    string    `json:"wechat_url"`
	MediaID      string    `json:"media_id"`
	CreatedAt    time.Time `json:"created_at"`
}

// PublishRecord is a publish record DTO for agent callers.
type PublishRecord struct {
	ID              int64     `json:"id"`
	TenantID        string    `json:"tenant_id"`
	AuthorizerID    int64     `json:"authorizer_id"`
	ArticleID       int64     `json:"article_id"`
	WeChatPublishID string    `json:"wechat_publish_id"`
	WeChatArticleID string    `json:"wechat_article_id"`
	Status          string    `json:"status"`
	ErrorCode       string    `json:"error_code"`
	ErrorMessage    string    `json:"error_message"`
	SubmittedAt     time.Time `json:"submitted_at"`
	FinishedAt      time.Time `json:"finished_at"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// Authorize implements wechat.authorize().
func (api *WeChatAPI) Authorize(ctx context.Context, input AuthorizeInput) (AuthorizeOutput, error) {
	if api == nil || api.authorization == nil {
		return AuthorizeOutput{}, fmt.Errorf("validate wechat.authorize dependencies: %w", application.ErrNotImplemented)
	}
	result, err := api.authorization.GenerateAuthorizationURL(ctx, application.GenerateAuthorizationURLInput{
		TenantID:       input.TenantID,
		ComponentAppID: input.ComponentAppID,
		RedirectURI:    input.RedirectURI,
		AuthType:       input.AuthType,
		BizAppID:       input.BizAppID,
	})
	if err != nil {
		return AuthorizeOutput{}, fmt.Errorf("generate authorization url: %w", err)
	}
	return AuthorizeOutput{AuthorizationURL: result.URL, PreAuthCodeExpiresInSec: result.PreAuthCodeExpiresInSec}, nil
}

// PublishArticle implements wechat.publishArticle().
func (api *WeChatAPI) PublishArticle(ctx context.Context, input PublishArticleInput) (PublishArticleOutput, error) {
	if api == nil || api.publishes == nil {
		return PublishArticleOutput{}, fmt.Errorf("validate wechat.publishArticle dependencies: %w", application.ErrNotImplemented)
	}
	record, err := api.publishes.PublishArticle(ctx, application.PublishArticleInput{
		TenantID: input.TenantID, ArticleID: input.ArticleID,
	})
	if err != nil {
		return PublishArticleOutput{}, fmt.Errorf("publish article: %w", err)
	}
	return PublishArticleOutput{Record: toPublishRecord(record)}, nil
}

// UploadImage implements wechat.uploadImage().
func (api *WeChatAPI) UploadImage(ctx context.Context, input UploadImageInput) (UploadImageOutput, error) {
	if api == nil || api.materials == nil {
		return UploadImageOutput{}, fmt.Errorf("validate wechat.uploadImage dependencies: %w", application.ErrNotImplemented)
	}
	asset, err := api.uploadImage(ctx, input)
	if err != nil {
		return UploadImageOutput{}, err
	}
	return UploadImageOutput{Asset: toMaterialAsset(asset)}, nil
}

// ListAccounts implements wechat.listAccounts().
func (api *WeChatAPI) ListAccounts(ctx context.Context, input ListAccountsInput) (ListAccountsOutput, error) {
	if api == nil || api.accounts == nil {
		return ListAccountsOutput{}, fmt.Errorf("validate wechat.listAccounts dependencies: %w", application.ErrNotImplemented)
	}
	items, err := api.accounts.ListAccounts(ctx, input.TenantID)
	if err != nil {
		return ListAccountsOutput{}, fmt.Errorf("list accounts: %w", err)
	}
	out := make([]Account, 0, len(items))
	for _, item := range items {
		out = append(out, toAccount(item))
	}
	return ListAccountsOutput{Items: out}, nil
}

// ListArticles implements wechat.listArticles().
func (api *WeChatAPI) ListArticles(ctx context.Context, input ListArticlesInput) (ListArticlesOutput, error) {
	if api == nil || api.articles == nil {
		return ListArticlesOutput{}, fmt.Errorf("validate wechat.listArticles dependencies: %w", application.ErrNotImplemented)
	}
	items, err := api.articles.ListArticles(ctx, input.TenantID)
	if err != nil {
		return ListArticlesOutput{}, fmt.Errorf("list articles: %w", err)
	}
	out := make([]Article, 0, len(items))
	for _, item := range items {
		out = append(out, toArticle(item))
	}
	return ListArticlesOutput{Items: out}, nil
}

func (api *WeChatAPI) uploadImage(ctx context.Context, input UploadImageInput) (material.Asset, error) {
	upload := application.UploadMaterialInput{
		TenantID: input.TenantID, AuthorizerID: input.AuthorizerID, ArticleID: input.ArticleID,
		Filename: input.Filename, Content: input.Content,
	}
	switch input.Usage {
	case ImageUsageInline:
		asset, err := api.materials.UploadInlineImage(ctx, upload)
		if err != nil {
			return material.Asset{}, fmt.Errorf("upload inline image: %w", err)
		}
		return asset, nil
	case ImageUsageCover:
		asset, err := api.materials.UploadCover(ctx, upload)
		if err != nil {
			return material.Asset{}, fmt.Errorf("upload cover image: %w", err)
		}
		return asset, nil
	default:
		return material.Asset{}, fmt.Errorf("validate image usage: %w", application.ErrInvalidInput)
	}
}

func toAccount(account authorization.Account) Account {
	return Account{
		ID: account.ID, TenantID: account.TenantID, AppID: account.AppID, Name: account.Name,
		AvatarURL: account.AvatarURL, Status: string(account.Status), LastSyncedAt: account.LastSyncedAt,
		CreatedAt: account.CreatedAt, UpdatedAt: account.UpdatedAt,
	}
}

func toArticle(item article.Article) Article {
	return Article{
		ID: item.ID, TenantID: item.TenantID, AuthorizerID: item.AuthorizerID,
		Title: item.Title, Author: item.Author, Digest: item.Digest, ContentHTML: item.ContentHTML,
		CoverMediaAssetID: item.CoverMediaAssetID, Status: string(item.Status),
		CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
}

func toMaterialAsset(asset material.Asset) MaterialAsset {
	return MaterialAsset{
		ID: asset.ID, TenantID: asset.TenantID, AuthorizerID: asset.AuthorizerID, ArticleID: asset.ArticleID,
		Usage: string(asset.Usage), LocalURL: asset.LocalURL, WeChatURL: asset.WeChatURL, MediaID: asset.MediaID,
		CreatedAt: asset.CreatedAt,
	}
}

func toPublishRecord(record publish.Record) PublishRecord {
	return PublishRecord{
		ID: record.ID, TenantID: record.TenantID, AuthorizerID: record.AuthorizerID, ArticleID: record.ArticleID,
		WeChatPublishID: record.WeChatPublishID, WeChatArticleID: record.WeChatArticleID, Status: string(record.Status),
		ErrorCode: record.ErrorCode, ErrorMessage: record.ErrorMessage, SubmittedAt: record.SubmittedAt,
		FinishedAt: record.FinishedAt, CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt,
	}
}
