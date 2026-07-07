package material

import (
	"context"
	"errors"
	"io"
	"time"
)

// ErrNotFound indicates a tenant-scoped material asset was not found.
var ErrNotFound = errors.New("material asset not found")

// ErrUploaderUnavailable indicates the WeChat material uploader is not configured.
var ErrUploaderUnavailable = errors.New("material uploader unavailable")

// ErrUploadFailed indicates WeChat rejected or failed a material upload.
var ErrUploadFailed = errors.New("material upload failed")

// Usage describes which WeChat material upload path an asset uses.
type Usage string

const (
	// UsageInlineImage is an article body image uploaded through WeChat uploadimg.
	UsageInlineImage Usage = "inline_image"
	// UsageCover is an article cover uploaded as permanent material.
	UsageCover Usage = "cover"
)

// Asset represents an uploaded image or cover material.
type Asset struct {
	ID           int64
	TenantID     string
	AuthorizerID int64
	ArticleID    int64
	Usage        Usage
	LocalURL     string
	WeChatURL    string
	MediaID      string
	CreatedAt    time.Time
}

// InlineImageUpload is the result of a WeChat inline image upload.
type InlineImageUpload struct {
	WeChatURL string
}

// CoverUpload is the result of a WeChat permanent cover upload.
type CoverUpload struct {
	MediaID string
}

// Uploader uploads material bytes to WeChat using the correct endpoint for each usage.
type Uploader interface {
	UploadInlineImage(ctx context.Context, authorizerAccessToken string, filename string, content io.Reader) (InlineImageUpload, error)
	UploadCover(ctx context.Context, authorizerAccessToken string, filename string, content io.Reader) (CoverUpload, error)
}

// Repository persists tenant-scoped material assets.
type Repository interface {
	CreateMaterial(ctx context.Context, tenantID string, asset Asset) (Asset, error)
	GetMaterial(ctx context.Context, tenantID string, id int64) (Asset, error)
	ListMaterialByArticle(ctx context.Context, tenantID string, articleID int64) ([]Asset, error)
}
