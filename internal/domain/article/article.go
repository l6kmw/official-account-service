package article

import (
	"context"
	"errors"
	"time"
)

// ErrNotFound indicates a tenant-scoped article was not found.
var ErrNotFound = errors.New("article not found")

// Status describes a platform article lifecycle state.
type Status string

const (
	// StatusDraft means the article is editable and not submitted to WeChat.
	StatusDraft Status = "draft"
	// StatusPublishing means the article has been submitted and awaits final result.
	StatusPublishing Status = "publishing"
	// StatusPublished means WeChat confirmed the article is published.
	StatusPublished Status = "published"
	// StatusFailed means publishing failed.
	StatusFailed Status = "failed"
)

// Article is a platform-managed official account article draft.
type Article struct {
	ID                int64
	TenantID          string
	AuthorizerID      int64
	Title             string
	Author            string
	Digest            string
	ContentHTML       string
	CoverMediaAssetID int64
	Status            Status
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// Repository persists tenant-scoped articles.
type Repository interface {
	Create(ctx context.Context, tenantID string, article Article) (Article, error)
	Get(ctx context.Context, tenantID string, id int64) (Article, error)
	List(ctx context.Context, tenantID string) ([]Article, error)
	Update(ctx context.Context, tenantID string, article Article) (Article, error)
	Delete(ctx context.Context, tenantID string, id int64) error
}
