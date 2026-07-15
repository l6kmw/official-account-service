package publish

import (
	"context"
	"errors"
	"time"

	"official-account-service/internal/domain/agentaudit"
)

// ErrNotFound indicates a tenant-scoped publish record was not found.
var ErrNotFound = errors.New("publish record not found")

// ErrPublishInProgress indicates that an article already has an active publish attempt.
var ErrPublishInProgress = errors.New("publish already in progress")

// ErrPublisherUnavailable indicates the WeChat publisher is not configured.
var ErrPublisherUnavailable = errors.New("publish publisher unavailable")

// ErrPublishFailed indicates WeChat rejected or failed a publish operation.
var ErrPublishFailed = errors.New("publish failed")

// ErrPublishStillProcessing indicates WeChat has not finalized a publish job yet.
var ErrPublishStillProcessing = errors.New("publish still processing")

// ErrTaskQueueUnavailable indicates the publish task queue is not configured or reachable.
var ErrTaskQueueUnavailable = errors.New("publish task queue unavailable")

// Status describes a WeChat publish submission result state.
type Status string

const (
	// StatusPublishing means WeChat accepted the publish submission but has not finalized it.
	StatusPublishing Status = "publishing"
	// StatusPublished means WeChat reported publish success.
	StatusPublished Status = "published"
	// StatusFailed means WeChat reported publish failure.
	StatusFailed Status = "failed"
	// StatusDeleted means a published WeChat article was deleted.
	StatusDeleted Status = "deleted"
)

// Record stores one publish attempt and its final result.
type Record struct {
	ID                      int64
	TenantID                string
	AuthorizerID            int64
	ArticleID               int64
	WeChatPublishID         string
	WeChatArticleID         string
	Status                  Status
	ErrorCode               string
	ErrorMessage            string
	SubmittedAt             time.Time
	FinishedAt              time.Time
	ArticleCreatedByAgentID string
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

// ArticleDraft is the article payload submitted to WeChat draft.add.
type ArticleDraft struct {
	Title        string
	Author       string
	Digest       string
	ContentHTML  string
	ThumbMediaID string
}

// DraftResult contains the WeChat media id returned by draft.add.
type DraftResult struct {
	MediaID string
}

// SubmitResult contains the WeChat publish id returned by freepublish.submit.
type SubmitResult struct {
	PublishID string
}

// StatusResult contains the latest WeChat free publish status.
type StatusResult struct {
	Status          Status
	WeChatArticleID string
	ErrorCode       string
	ErrorMessage    string
}

// StatusSyncTask identifies one publish record that needs status polling.
type StatusSyncTask struct {
	TenantID        string
	PublishRecordID int64
}

// StatusSyncScheduler enqueues publish status polling tasks.
type StatusSyncScheduler interface {
	EnqueueStatusSync(ctx context.Context, task StatusSyncTask) error
}

// StatusSyncHandler handles publish status polling tasks.
type StatusSyncHandler interface {
	HandleStatusSyncTask(ctx context.Context, task StatusSyncTask) error
}

// Publisher submits and queries WeChat publish jobs.
type Publisher interface {
	AddDraft(ctx context.Context, authorizerAccessToken string, draft ArticleDraft) (DraftResult, error)
	SubmitFreePublish(ctx context.Context, authorizerAccessToken string, mediaID string) (SubmitResult, error)
	GetFreePublishStatus(ctx context.Context, authorizerAccessToken string, publishID string) (StatusResult, error)
	DeleteFreePublish(ctx context.Context, authorizerAccessToken string, articleID string, index int) error
}

// Repository persists tenant-scoped publish records.
type Repository interface {
	CreatePublishRecord(ctx context.Context, tenantID string, record Record) (Record, error)
	GetPublishRecord(ctx context.Context, tenantID string, id int64) (Record, error)
	GetPublishRecordByPublishID(ctx context.Context, tenantID string, publishID string) (Record, error)
	ListPublishRecords(ctx context.Context, tenantID string) ([]Record, error)
	ListPublishRecordsByArticle(ctx context.Context, tenantID string, articleID int64) ([]Record, error)
	UpdatePublishRecordSubmission(ctx context.Context, tenantID string, record Record) (Record, error)
	UpdatePublishRecordStatus(ctx context.Context, tenantID string, record Record) (Record, error)
}

// AgentFilteredRepository lists publish records through the creating Agent of their article.
type AgentFilteredRepository interface {
	ListPublishRecordsByAgent(ctx context.Context, tenantID string, agentRecordID string) ([]Record, error)
}

// AuditedCreateRepository persists one publish intent and its audit entry atomically.
type AuditedCreateRepository interface {
	CreatePublishRecordWithAudit(ctx context.Context, tenantID string, record Record, audit agentaudit.Entry) (Record, error)
}
