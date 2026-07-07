package application

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"official-account-service/internal/domain/material"
)

// MaterialService manages tenant-scoped article materials.
type MaterialService struct {
	materials      material.Repository
	uploader       material.Uploader
	tokens         MaterialTokenProvider
	componentAppID string
}

// MaterialTokenProvider provides authorizer access tokens for material uploads.
type MaterialTokenProvider interface {
	GetAuthorizerAccessToken(ctx context.Context, input RefreshAuthorizerAccessTokenInput) (AuthorizerAccessToken, error)
}

// NewMaterialService constructs a MaterialService.
func NewMaterialService(materials material.Repository, uploader material.Uploader) *MaterialService {
	return &MaterialService{materials: materials, uploader: uploader}
}

// NewMaterialServiceWithTokenProvider constructs a MaterialService with authorizer token lookup.
func NewMaterialServiceWithTokenProvider(materials material.Repository, uploader material.Uploader, tokens MaterialTokenProvider, componentAppID string) *MaterialService {
	return &MaterialService{materials: materials, uploader: uploader, tokens: tokens, componentAppID: componentAppID}
}

// UploadMaterialInput contains material upload metadata and content.
type UploadMaterialInput struct {
	TenantID     string
	AuthorizerID int64
	ArticleID    int64
	Filename     string
	Content      io.Reader
}

// UploadInlineImage uploads an article body image and stores the returned WeChat URL.
func (s *MaterialService) UploadInlineImage(ctx context.Context, input UploadMaterialInput) (material.Asset, error) {
	if err := s.validateUploadInput(input); err != nil {
		return material.Asset{}, err
	}
	accessToken, err := s.accessTokenForUpload(ctx, input)
	if err != nil {
		return material.Asset{}, err
	}
	result, err := s.uploader.UploadInlineImage(ctx, accessToken, input.Filename, input.Content)
	if err != nil {
		return material.Asset{}, wrapUploaderError("upload inline image", err)
	}
	if strings.TrimSpace(result.WeChatURL) == "" {
		return material.Asset{}, fmt.Errorf("validate inline image upload result: %w", ErrInvalidInput)
	}
	asset, err := s.materials.CreateMaterial(ctx, input.TenantID, material.Asset{
		TenantID: input.TenantID, AuthorizerID: input.AuthorizerID, ArticleID: input.ArticleID,
		Usage: material.UsageInlineImage, LocalURL: input.Filename, WeChatURL: result.WeChatURL,
	})
	if err != nil {
		return material.Asset{}, fmt.Errorf("create inline image material: %w", err)
	}
	return asset, nil
}

// UploadCover uploads an article cover image and stores the returned WeChat media id.
func (s *MaterialService) UploadCover(ctx context.Context, input UploadMaterialInput) (material.Asset, error) {
	if err := s.validateUploadInput(input); err != nil {
		return material.Asset{}, err
	}
	accessToken, err := s.accessTokenForUpload(ctx, input)
	if err != nil {
		return material.Asset{}, err
	}
	result, err := s.uploader.UploadCover(ctx, accessToken, input.Filename, input.Content)
	if err != nil {
		return material.Asset{}, wrapUploaderError("upload cover", err)
	}
	if strings.TrimSpace(result.MediaID) == "" {
		return material.Asset{}, fmt.Errorf("validate cover upload result: %w", ErrInvalidInput)
	}
	asset, err := s.materials.CreateMaterial(ctx, input.TenantID, material.Asset{
		TenantID: input.TenantID, AuthorizerID: input.AuthorizerID, ArticleID: input.ArticleID,
		Usage: material.UsageCover, LocalURL: input.Filename, MediaID: result.MediaID,
	})
	if err != nil {
		return material.Asset{}, fmt.Errorf("create cover material: %w", err)
	}
	return asset, nil
}

// GetMaterial returns one tenant-scoped material asset.
func (s *MaterialService) GetMaterial(ctx context.Context, tenantID string, id int64) (material.Asset, error) {
	if err := s.validateMaterialID(tenantID, id); err != nil {
		return material.Asset{}, err
	}
	asset, err := s.materials.GetMaterial(ctx, tenantID, id)
	if err != nil {
		return material.Asset{}, wrapMaterialReadError("get material", err)
	}
	return asset, nil
}

// ListMaterialsByArticle returns tenant-scoped material assets for an article.
func (s *MaterialService) ListMaterialsByArticle(ctx context.Context, tenantID string, articleID int64) ([]material.Asset, error) {
	if err := s.validateMaterialID(tenantID, articleID); err != nil {
		return nil, err
	}
	assets, err := s.materials.ListMaterialByArticle(ctx, tenantID, articleID)
	if err != nil {
		return nil, fmt.Errorf("list material by article: %w", err)
	}
	return assets, nil
}

func (s *MaterialService) validateUploadInput(input UploadMaterialInput) error {
	if err := s.validateReady(); err != nil {
		return err
	}
	if strings.TrimSpace(input.TenantID) == "" {
		return fmt.Errorf("validate material tenant id: %w", ErrInvalidInput)
	}
	if input.AuthorizerID <= 0 {
		return fmt.Errorf("validate material authorizer id: %w", ErrInvalidInput)
	}
	if input.ArticleID <= 0 {
		return fmt.Errorf("validate material article id: %w", ErrInvalidInput)
	}
	if strings.TrimSpace(input.Filename) == "" {
		return fmt.Errorf("validate material filename: %w", ErrInvalidInput)
	}
	if input.Content == nil {
		return fmt.Errorf("validate material content: %w", ErrInvalidInput)
	}
	return nil
}

func (s *MaterialService) validateReady() error {
	if s == nil || s.materials == nil || s.uploader == nil {
		return fmt.Errorf("validate material service dependencies: %w", ErrInvalidInput)
	}
	return nil
}

func (s *MaterialService) accessTokenForUpload(ctx context.Context, input UploadMaterialInput) (string, error) {
	if s.tokens == nil {
		return "", nil
	}
	if strings.TrimSpace(s.componentAppID) == "" {
		return "", fmt.Errorf("validate material component app id: %w", ErrNotImplemented)
	}
	token, err := s.tokens.GetAuthorizerAccessToken(ctx, RefreshAuthorizerAccessTokenInput{
		TenantID:       input.TenantID,
		AccountID:      input.AuthorizerID,
		ComponentAppID: s.componentAppID,
	})
	if err != nil {
		return "", fmt.Errorf("get authorizer access token for material upload: %w", err)
	}
	return token.AccessToken, nil
}

func (s *MaterialService) validateMaterialID(tenantID string, id int64) error {
	if err := s.validateReady(); err != nil {
		return err
	}
	if strings.TrimSpace(tenantID) == "" {
		return fmt.Errorf("validate material tenant id: %w", ErrInvalidInput)
	}
	if id <= 0 {
		return fmt.Errorf("validate material id: %w", ErrInvalidInput)
	}
	return nil
}

func wrapUploaderError(action string, err error) error {
	if errors.Is(err, material.ErrUploaderUnavailable) {
		return fmt.Errorf("%s: %w", action, ErrNotImplemented)
	}
	return fmt.Errorf("%s: %w", action, err)
}

func wrapMaterialReadError(action string, err error) error {
	if errors.Is(err, material.ErrNotFound) {
		return fmt.Errorf("%s: %w", action, ErrNotFound)
	}
	return fmt.Errorf("%s: %w", action, err)
}
