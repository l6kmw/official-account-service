package application

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"official-account-service/internal/domain/article"
	"official-account-service/internal/domain/material"
	"official-account-service/internal/domain/publish"
)

func (s *PublishService) syncArticleStatus(ctx context.Context, tenantID string, draft article.Article, status article.Status) error {
	if draft.Status == status {
		return nil
	}
	draft.Status = status
	if _, err := s.articles.Update(ctx, tenantID, draft); err != nil {
		return wrapPublishArticleError("sync article status", err)
	}
	return nil
}

func (s *PublishService) pendingPublishRecord(ctx context.Context, tenantID string, articleID int64) (publish.Record, bool, error) {
	records, err := s.records.ListPublishRecordsByArticle(ctx, tenantID, articleID)
	if err != nil {
		return publish.Record{}, false, fmt.Errorf("list publish records for idempotency: %w", err)
	}
	for _, record := range records {
		if record.Status == publish.StatusPublishing {
			return record, true, nil
		}
	}
	return publish.Record{}, false, nil
}

func (s *PublishService) createPublishIntent(ctx context.Context, tenantID string, draft article.Article) (publish.Record, bool, error) {
	now := s.now()
	record, err := s.records.CreatePublishRecord(ctx, tenantID, publish.Record{
		TenantID: tenantID, AuthorizerID: draft.AuthorizerID, ArticleID: draft.ID,
		Status: publish.StatusPublishing, SubmittedAt: now,
	})
	if errors.Is(err, publish.ErrPublishInProgress) {
		pending, ok, pendingErr := s.pendingPublishRecord(ctx, tenantID, draft.ID)
		if pendingErr != nil {
			return publish.Record{}, false, pendingErr
		}
		if !ok {
			return publish.Record{}, false, fmt.Errorf("get concurrent publish intent: %w", ErrNotFound)
		}
		return pending, false, nil
	}
	if err != nil {
		return publish.Record{}, false, fmt.Errorf("create publish intent: %w", err)
	}
	if err := s.syncArticleStatus(ctx, tenantID, draft, article.StatusPublishing); err != nil {
		return publish.Record{}, false, s.failPublishIntent(ctx, tenantID, draft, record, "local_state_failed", err)
	}
	return record, true, nil
}

func (s *PublishService) failPublishIntent(ctx context.Context, tenantID string, draft article.Article, record publish.Record, errorCode string, cause error) error {
	record.Status = publish.StatusFailed
	record.ErrorCode = errorCode
	record.ErrorMessage = "publish submission failed"
	record.FinishedAt = s.now()
	if _, err := s.records.UpdatePublishRecordStatus(ctx, tenantID, record); err != nil {
		return fmt.Errorf("mark publish intent failed: %w", errors.Join(cause, err))
	}
	if err := s.syncArticleStatus(ctx, tenantID, draft, article.StatusFailed); err != nil {
		return fmt.Errorf("mark publish article failed: %w", errors.Join(cause, err))
	}
	return cause
}

func (s *PublishService) enqueueExistingPublish(ctx context.Context, record publish.Record) error {
	if s.statusSync == nil {
		return nil
	}
	if err := s.enqueueStatusSync(ctx, record); err != nil {
		return s.recordPublishingError(ctx, record.TenantID, record, "status_sync_enqueue_failed", err)
	}
	if record.ErrorCode == "status_sync_enqueue_failed" {
		record.ErrorCode = ""
		record.ErrorMessage = ""
		if _, err := s.records.UpdatePublishRecordStatus(ctx, record.TenantID, record); err != nil {
			return wrapPublishReadError("clear publish enqueue error", err)
		}
	}
	return nil
}

func (s *PublishService) recordPublishingError(ctx context.Context, tenantID string, record publish.Record, errorCode string, cause error) error {
	record.ErrorCode = errorCode
	record.ErrorMessage = "publish status sync is waiting for retry"
	if _, err := s.records.UpdatePublishRecordStatus(ctx, tenantID, record); err != nil {
		return fmt.Errorf("store publish recovery state: %w", errors.Join(cause, err))
	}
	return cause
}

func validateArticleReadyForPublish(draft article.Article) error {
	if draft.Status == article.StatusPublishing {
		return fmt.Errorf("validate article publish status: %w", ErrInvalidInput)
	}
	if strings.TrimSpace(draft.Title) == "" || strings.TrimSpace(draft.ContentHTML) == "" {
		return fmt.Errorf("validate article publish content: %w", ErrInvalidInput)
	}
	if draft.CoverMediaAssetID <= 0 {
		return fmt.Errorf("validate article publish cover: %w", ErrInvalidInput)
	}
	return nil
}

func (s *PublishService) accessTokenForPublish(ctx context.Context, tenantID string, authorizerID int64) (string, error) {
	token, err := s.tokens.GetAuthorizerAccessToken(ctx, RefreshAuthorizerAccessTokenInput{
		TenantID: tenantID, AccountID: authorizerID, ComponentAppID: s.componentAppID,
	})
	if err != nil {
		return "", fmt.Errorf("get authorizer access token for publish: %w", err)
	}
	return token.AccessToken, nil
}

func (s *PublishService) deletePublishRecordFromWeChat(ctx context.Context, tenantID string, record publish.Record, tokens map[int64]string) (publish.Record, error) {
	accessToken := ""
	if tokens != nil {
		accessToken = tokens[record.AuthorizerID]
	}
	if strings.TrimSpace(accessToken) == "" {
		token, err := s.accessTokenForPublish(ctx, tenantID, record.AuthorizerID)
		if err != nil {
			return publish.Record{}, err
		}
		accessToken = token
		if tokens != nil {
			tokens[record.AuthorizerID] = token
		}
	}
	if err := s.publisher.DeleteFreePublish(ctx, accessToken, record.WeChatArticleID, 0); err != nil {
		return publish.Record{}, wrapPublisherError("delete wechat published article", err)
	}
	updated := record
	updated.Status = publish.StatusDeleted
	updated.FinishedAt = s.now()
	updated, err := s.records.UpdatePublishRecordStatus(ctx, tenantID, updated)
	if err != nil {
		return publish.Record{}, wrapPublishReadError("mark publish record deleted", err)
	}
	return updated, nil
}

func (s *PublishService) deletePreviousPublishedRecords(ctx context.Context, tenantID string, current publish.Record) error {
	if current.Status != publish.StatusPublished || strings.TrimSpace(current.WeChatArticleID) == "" {
		return nil
	}
	records, err := s.records.ListPublishRecordsByArticle(ctx, tenantID, current.ArticleID)
	if err != nil {
		return fmt.Errorf("list previous publish records for cleanup: %w", err)
	}
	targets := make([]publish.Record, 0)
	for _, record := range records {
		if record.ID == current.ID {
			continue
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
		if _, err := s.deletePublishRecordFromWeChat(ctx, tenantID, record, tokens); err != nil {
			return err
		}
	}
	return nil
}

func validatePublishTransition(from publish.Status, to publish.Status) error {
	if from == to {
		return nil
	}
	if from == publish.StatusPublished && to == publish.StatusDeleted {
		return nil
	}
	if from == publish.StatusPublished || from == publish.StatusFailed {
		return fmt.Errorf("validate publish final transition: %w", ErrInvalidInput)
	}
	if from == publish.StatusDeleted {
		return fmt.Errorf("validate publish deleted transition: %w", ErrInvalidInput)
	}
	return nil
}

func samePublishUpdate(current publish.Record, input UpdatePublishStatusInput) bool {
	return current.Status == input.Status && current.WeChatArticleID == input.WeChatArticleID && current.ErrorCode == input.ErrorCode && current.ErrorMessage == input.ErrorMessage
}

func isPublishStatus(status publish.Status) bool {
	return status == publish.StatusPublishing || status == publish.StatusPublished || status == publish.StatusFailed || status == publish.StatusDeleted
}

func mapPublishStatus(status publish.Status) article.Status {
	if status == publish.StatusPublished {
		return article.StatusPublished
	}
	if status == publish.StatusFailed {
		return article.StatusFailed
	}
	return article.StatusPublishing
}

func wrapPublishReadError(action string, err error) error {
	if errors.Is(err, publish.ErrNotFound) {
		return fmt.Errorf("%s: %w", action, ErrNotFound)
	}
	return fmt.Errorf("%s: %w", action, err)
}

func wrapPublishArticleError(action string, err error) error {
	if errors.Is(err, article.ErrNotFound) {
		return fmt.Errorf("%s: %w", action, ErrNotFound)
	}
	return fmt.Errorf("%s: %w", action, err)
}

func wrapPublishMaterialError(action string, err error) error {
	if errors.Is(err, material.ErrNotFound) {
		return fmt.Errorf("%s: %w", action, ErrInvalidInput)
	}
	return fmt.Errorf("%s: %w", action, err)
}

func wrapPublisherError(action string, err error) error {
	if errors.Is(err, publish.ErrPublisherUnavailable) {
		return fmt.Errorf("%s: %w", action, ErrNotImplemented)
	}
	return fmt.Errorf("%s: %w", action, err)
}
