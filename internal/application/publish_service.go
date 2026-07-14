package application

import (
	"context"
	"fmt"
	"strings"
	"time"

	"official-account-service/internal/domain/article"
	"official-account-service/internal/domain/material"
	"official-account-service/internal/domain/publish"
)

// PublishService manages local publish records and article status sync.
type PublishService struct {
	records        publish.Repository
	articles       article.Repository
	materials      material.Repository
	publisher      publish.Publisher
	tokens         PublishTokenProvider
	componentAppID string
	statusSync     publish.StatusSyncScheduler
	now            func() time.Time
}

// PublishTokenProvider provides authorizer access tokens for WeChat publish calls.
type PublishTokenProvider interface {
	GetAuthorizerAccessToken(ctx context.Context, input RefreshAuthorizerAccessTokenInput) (AuthorizerAccessToken, error)
}

// NewPublishService constructs a PublishService.
func NewPublishService(records publish.Repository, articles article.Repository, now func() time.Time) *PublishService {
	if now == nil {
		now = time.Now
	}
	return &PublishService{records: records, articles: articles, now: now}
}

// NewPublishServiceWithPublisher constructs a PublishService with WeChat publish support.
func NewPublishServiceWithPublisher(records publish.Repository, articles article.Repository, materials material.Repository, publisher publish.Publisher, tokens PublishTokenProvider, componentAppID string, now func() time.Time) *PublishService {
	service := NewPublishService(records, articles, now)
	service.materials = materials
	service.publisher = publisher
	service.tokens = tokens
	service.componentAppID = componentAppID
	return service
}

// NewPublishServiceWithPublisherAndStatusSync constructs a PublishService with publish status fallback polling.
func NewPublishServiceWithPublisherAndStatusSync(records publish.Repository, articles article.Repository, materials material.Repository, publisher publish.Publisher, tokens PublishTokenProvider, componentAppID string, statusSync publish.StatusSyncScheduler, now func() time.Time) *PublishService {
	service := NewPublishServiceWithPublisher(records, articles, materials, publisher, tokens, componentAppID, now)
	service.statusSync = statusSync
	return service
}

// CreatePublishRecordInput contains fields for creating a local publish record.
type CreatePublishRecordInput struct {
	TenantID        string
	ArticleID       int64
	WeChatPublishID string
}

// UpdatePublishStatusInput contains fields for updating a publish record status.
type UpdatePublishStatusInput struct {
	TenantID        string
	ID              int64
	Status          publish.Status
	WeChatArticleID string
	ErrorCode       string
	ErrorMessage    string
}

// PublishArticleInput contains fields for submitting one article to WeChat.
type PublishArticleInput struct {
	TenantID  string
	ArticleID int64
}

// SyncPublishStatusInput contains fields for polling one WeChat publish job.
type SyncPublishStatusInput struct {
	TenantID string
	ID       int64
}

// DeletePublishedArticleInput identifies one article whose WeChat publishes should be deleted.
type DeletePublishedArticleInput struct {
	TenantID  string
	ArticleID int64
}

// DeletePublishedRecordInput identifies one publish record whose WeChat article should be deleted.
type DeletePublishedRecordInput struct {
	TenantID string
	ID       int64
}

// HandlePublishResultInput contains a WeChat publish result callback payload.
type HandlePublishResultInput struct {
	TenantID        string
	WeChatPublishID string
	Status          publish.Status
	WeChatArticleID string
	ErrorCode       string
	ErrorMessage    string
}

// PublishArticle creates a WeChat draft, submits freepublish, and stores a publishing record.
func (s *PublishService) PublishArticle(ctx context.Context, input PublishArticleInput) (publish.Record, error) {
	if err := s.validatePublishRecordID(input.TenantID, input.ArticleID); err != nil {
		return publish.Record{}, err
	}
	if err := s.validatePublisherReady(); err != nil {
		return publish.Record{}, err
	}
	draft, err := s.articles.Get(ctx, input.TenantID, input.ArticleID)
	if err != nil {
		return publish.Record{}, wrapPublishArticleError("get article for publish", err)
	}
	if pending, ok, err := s.pendingPublishRecord(ctx, input.TenantID, input.ArticleID); err != nil {
		return publish.Record{}, err
	} else if ok {
		return pending, nil
	}
	if err := validateArticleReadyForPublish(draft); err != nil {
		return publish.Record{}, err
	}
	cover, err := s.materials.GetMaterial(ctx, input.TenantID, draft.CoverMediaAssetID)
	if err != nil {
		return publish.Record{}, wrapPublishMaterialError("get cover material for publish", err)
	}
	if cover.ArticleID != draft.ID || cover.AuthorizerID != draft.AuthorizerID || cover.Usage != material.UsageCover || strings.TrimSpace(cover.MediaID) == "" {
		return publish.Record{}, fmt.Errorf("validate cover material for publish: %w", ErrInvalidInput)
	}
	accessToken, err := s.accessTokenForPublish(ctx, input.TenantID, draft.AuthorizerID)
	if err != nil {
		return publish.Record{}, err
	}
	wechatDraft, err := s.publisher.AddDraft(ctx, accessToken, publish.ArticleDraft{
		Title: draft.Title, Author: draft.Author, Digest: draft.Digest, ContentHTML: draft.ContentHTML,
		ThumbMediaID: cover.MediaID,
	})
	if err != nil {
		return publish.Record{}, wrapPublisherError("add wechat draft", err)
	}
	submitted, err := s.publisher.SubmitFreePublish(ctx, accessToken, wechatDraft.MediaID)
	if err != nil {
		return publish.Record{}, wrapPublisherError("submit wechat free publish", err)
	}
	record, err := s.CreatePublishRecord(ctx, CreatePublishRecordInput{
		TenantID: input.TenantID, ArticleID: input.ArticleID, WeChatPublishID: submitted.PublishID,
	})
	if err != nil {
		return publish.Record{}, err
	}
	if err := s.enqueueStatusSync(ctx, record); err != nil {
		return publish.Record{}, err
	}
	return record, nil
}

// SyncPublishStatus polls WeChat publish status and updates the local record.
func (s *PublishService) SyncPublishStatus(ctx context.Context, input SyncPublishStatusInput) (publish.Record, error) {
	if err := s.validatePublishRecordID(input.TenantID, input.ID); err != nil {
		return publish.Record{}, err
	}
	if err := s.validatePublisherReady(); err != nil {
		return publish.Record{}, err
	}
	record, err := s.records.GetPublishRecord(ctx, input.TenantID, input.ID)
	if err != nil {
		return publish.Record{}, wrapPublishReadError("get publish record for status sync", err)
	}
	if record.Status == publish.StatusPublished {
		if err := s.deletePreviousPublishedRecords(ctx, input.TenantID, record); err != nil {
			return publish.Record{}, err
		}
		return record, nil
	}
	if record.Status == publish.StatusFailed || record.Status == publish.StatusDeleted {
		return record, nil
	}
	if strings.TrimSpace(record.WeChatPublishID) == "" {
		return publish.Record{}, fmt.Errorf("validate publish id for status sync: %w", ErrInvalidInput)
	}
	accessToken, err := s.accessTokenForPublish(ctx, input.TenantID, record.AuthorizerID)
	if err != nil {
		return publish.Record{}, err
	}
	status, err := s.publisher.GetFreePublishStatus(ctx, accessToken, record.WeChatPublishID)
	if err != nil {
		return publish.Record{}, wrapPublisherError("get wechat publish status", err)
	}
	return s.UpdatePublishStatus(ctx, UpdatePublishStatusInput{
		TenantID: input.TenantID, ID: record.ID, Status: status.Status, WeChatArticleID: status.WeChatArticleID,
		ErrorCode: status.ErrorCode, ErrorMessage: status.ErrorMessage,
	})
}

// DeletePublishedArticle removes published WeChat copies for one local article.
func (s *PublishService) DeletePublishedArticle(ctx context.Context, input DeletePublishedArticleInput) error {
	if err := s.validatePublishRecordID(input.TenantID, input.ArticleID); err != nil {
		return err
	}
	if err := s.validateReady(); err != nil {
		return err
	}
	if _, err := s.articles.Get(ctx, input.TenantID, input.ArticleID); err != nil {
		return wrapPublishArticleError("get article for delete", err)
	}
	records, err := s.records.ListPublishRecordsByArticle(ctx, input.TenantID, input.ArticleID)
	if err != nil {
		return fmt.Errorf("list publish records for delete: %w", err)
	}
	targets := make([]publish.Record, 0)
	for _, record := range records {
		if record.Status == publish.StatusPublishing {
			return fmt.Errorf("validate publishing article delete: %w", ErrInvalidInput)
		}
		if record.Status == publish.StatusPublished && strings.TrimSpace(record.WeChatArticleID) != "" {
			targets = append(targets, record)
		}
	}
	if len(targets) == 0 {
		return nil
	}
	if err := s.validateDeletePublisherReady(); err != nil {
		return err
	}
	tokens := make(map[int64]string)
	for _, record := range targets {
		if _, err := s.deletePublishRecordFromWeChat(ctx, input.TenantID, record, tokens); err != nil {
			return err
		}
	}
	return nil
}

// DeletePublishedRecord deletes the WeChat article referenced by one publish record.
func (s *PublishService) DeletePublishedRecord(ctx context.Context, input DeletePublishedRecordInput) (publish.Record, error) {
	if err := s.validatePublishRecordID(input.TenantID, input.ID); err != nil {
		return publish.Record{}, err
	}
	record, err := s.records.GetPublishRecord(ctx, input.TenantID, input.ID)
	if err != nil {
		return publish.Record{}, wrapPublishReadError("get publish record for delete", err)
	}
	if record.Status == publish.StatusDeleted {
		return record, nil
	}
	if record.Status == publish.StatusPublishing || record.Status != publish.StatusPublished || strings.TrimSpace(record.WeChatArticleID) == "" {
		return publish.Record{}, fmt.Errorf("validate publish record delete: %w", ErrInvalidInput)
	}
	if err := s.validateDeletePublisherReady(); err != nil {
		return publish.Record{}, err
	}
	return s.deletePublishRecordFromWeChat(ctx, input.TenantID, record, nil)
}

// HandlePublishResult updates one publish record from a WeChat publish result callback.
func (s *PublishService) HandlePublishResult(ctx context.Context, input HandlePublishResultInput) (publish.Record, error) {
	if err := s.validatePublishResultInput(input); err != nil {
		return publish.Record{}, err
	}
	record, err := s.records.GetPublishRecordByPublishID(ctx, input.TenantID, input.WeChatPublishID)
	if err != nil {
		return publish.Record{}, wrapPublishReadError("get publish record by publish id", err)
	}
	return s.UpdatePublishStatus(ctx, UpdatePublishStatusInput{
		TenantID: input.TenantID, ID: record.ID, Status: input.Status, WeChatArticleID: input.WeChatArticleID,
		ErrorCode: input.ErrorCode, ErrorMessage: input.ErrorMessage,
	})
}

// CreatePublishRecord creates a publishing record and marks the article as publishing.
func (s *PublishService) CreatePublishRecord(ctx context.Context, input CreatePublishRecordInput) (publish.Record, error) {
	if err := s.validatePublishRecordID(input.TenantID, input.ArticleID); err != nil {
		return publish.Record{}, err
	}
	draft, err := s.articles.Get(ctx, input.TenantID, input.ArticleID)
	if err != nil {
		return publish.Record{}, wrapPublishArticleError("get article for publish record", err)
	}
	now := s.now()
	record, err := s.records.CreatePublishRecord(ctx, input.TenantID, publish.Record{
		TenantID: input.TenantID, AuthorizerID: draft.AuthorizerID, ArticleID: draft.ID,
		WeChatPublishID: input.WeChatPublishID, Status: publish.StatusPublishing, SubmittedAt: now,
	})
	if err != nil {
		return publish.Record{}, fmt.Errorf("create publish record: %w", err)
	}
	if err := s.syncArticleStatus(ctx, input.TenantID, draft, article.StatusPublishing); err != nil {
		return publish.Record{}, err
	}
	return record, nil
}

// GetPublishRecord returns one tenant-scoped publish record.
func (s *PublishService) GetPublishRecord(ctx context.Context, tenantID string, id int64) (publish.Record, error) {
	if err := s.validatePublishRecordID(tenantID, id); err != nil {
		return publish.Record{}, err
	}
	record, err := s.records.GetPublishRecord(ctx, tenantID, id)
	if err != nil {
		return publish.Record{}, wrapPublishReadError("get publish record", err)
	}
	return record, nil
}

// ListPublishRecords returns tenant-scoped publish records.
func (s *PublishService) ListPublishRecords(ctx context.Context, tenantID string) ([]publish.Record, error) {
	if err := s.validateTenantForPublishList(tenantID); err != nil {
		return nil, err
	}
	records, err := s.records.ListPublishRecords(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list publish records: %w", err)
	}
	return records, nil
}

// ListPublishRecordsByArticle returns tenant-scoped publish records for an article.
func (s *PublishService) ListPublishRecordsByArticle(ctx context.Context, tenantID string, articleID int64) ([]publish.Record, error) {
	if err := s.validatePublishRecordID(tenantID, articleID); err != nil {
		return nil, err
	}
	records, err := s.records.ListPublishRecordsByArticle(ctx, tenantID, articleID)
	if err != nil {
		return nil, fmt.Errorf("list publish records by article: %w", err)
	}
	return records, nil
}

// UpdatePublishStatus updates one publish record and syncs the article status.
func (s *PublishService) UpdatePublishStatus(ctx context.Context, input UpdatePublishStatusInput) (publish.Record, error) {
	if err := s.validateUpdateInput(input); err != nil {
		return publish.Record{}, err
	}
	current, err := s.records.GetPublishRecord(ctx, input.TenantID, input.ID)
	if err != nil {
		return publish.Record{}, wrapPublishReadError("get publish record", err)
	}
	if err := validatePublishTransition(current.Status, input.Status); err != nil {
		return publish.Record{}, err
	}
	if samePublishUpdate(current, input) {
		draft, err := s.articles.Get(ctx, input.TenantID, current.ArticleID)
		if err != nil {
			return publish.Record{}, wrapPublishArticleError("get article for idempotent status sync", err)
		}
		if err := s.syncArticleStatus(ctx, input.TenantID, draft, mapPublishStatus(input.Status)); err != nil {
			return publish.Record{}, err
		}
		if err := s.deletePreviousPublishedRecords(ctx, input.TenantID, current); err != nil {
			return publish.Record{}, err
		}
		return current, nil
	}
	updated := current
	updated.Status = input.Status
	updated.WeChatArticleID = input.WeChatArticleID
	updated.ErrorCode = input.ErrorCode
	updated.ErrorMessage = input.ErrorMessage
	if input.Status == publish.StatusPublished || input.Status == publish.StatusFailed || input.Status == publish.StatusDeleted {
		updated.FinishedAt = s.now()
	}
	updated, err = s.records.UpdatePublishRecordStatus(ctx, input.TenantID, updated)
	if err != nil {
		return publish.Record{}, wrapPublishReadError("update publish record", err)
	}
	draft, err := s.articles.Get(ctx, input.TenantID, updated.ArticleID)
	if err != nil {
		return publish.Record{}, wrapPublishArticleError("get article for status sync", err)
	}
	if err := s.syncArticleStatus(ctx, input.TenantID, draft, mapPublishStatus(input.Status)); err != nil {
		return publish.Record{}, err
	}
	if err := s.deletePreviousPublishedRecords(ctx, input.TenantID, updated); err != nil {
		return publish.Record{}, err
	}
	return updated, nil
}

func (s *PublishService) validateReady() error {
	if s == nil || s.records == nil || s.articles == nil {
		return fmt.Errorf("validate publish service dependencies: %w", ErrInvalidInput)
	}
	return nil
}

func (s *PublishService) validatePublisherReady() error {
	if err := s.validateReady(); err != nil {
		return err
	}
	if s.materials == nil || s.publisher == nil || s.tokens == nil || strings.TrimSpace(s.componentAppID) == "" {
		return fmt.Errorf("validate publish service publisher dependencies: %w", ErrNotImplemented)
	}
	return nil
}

func (s *PublishService) validateDeletePublisherReady() error {
	if err := s.validateReady(); err != nil {
		return err
	}
	if s.publisher == nil || s.tokens == nil || strings.TrimSpace(s.componentAppID) == "" {
		return fmt.Errorf("validate publish delete dependencies: %w", ErrNotImplemented)
	}
	return nil
}

func (s *PublishService) validatePublishRecordID(tenantID string, id int64) error {
	if err := s.validateReady(); err != nil {
		return err
	}
	if strings.TrimSpace(tenantID) == "" {
		return fmt.Errorf("validate publish tenant id: %w", ErrInvalidInput)
	}
	if id <= 0 {
		return fmt.Errorf("validate publish id: %w", ErrInvalidInput)
	}
	return nil
}

func (s *PublishService) validateTenantForPublishList(tenantID string) error {
	if err := s.validateReady(); err != nil {
		return err
	}
	if strings.TrimSpace(tenantID) == "" {
		return fmt.Errorf("validate publish tenant id: %w", ErrInvalidInput)
	}
	return nil
}

func (s *PublishService) validateUpdateInput(input UpdatePublishStatusInput) error {
	if err := s.validatePublishRecordID(input.TenantID, input.ID); err != nil {
		return err
	}
	if !isPublishStatus(input.Status) {
		return fmt.Errorf("validate publish status: %w", ErrInvalidInput)
	}
	return nil
}

func (s *PublishService) validatePublishResultInput(input HandlePublishResultInput) error {
	if err := s.validateReady(); err != nil {
		return err
	}
	if strings.TrimSpace(input.TenantID) == "" {
		return fmt.Errorf("validate publish result tenant id: %w", ErrInvalidInput)
	}
	if strings.TrimSpace(input.WeChatPublishID) == "" {
		return fmt.Errorf("validate publish result publish id: %w", ErrInvalidInput)
	}
	if !isPublishStatus(input.Status) {
		return fmt.Errorf("validate publish result status: %w", ErrInvalidInput)
	}
	return nil
}
