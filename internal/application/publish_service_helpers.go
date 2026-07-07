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

func validateArticleReadyForPublish(draft article.Article) error {
	if draft.Status == article.StatusPublished {
		return fmt.Errorf("validate article publish status: %w", ErrInvalidInput)
	}
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

func validatePublishTransition(from publish.Status, to publish.Status) error {
	if from == to {
		return nil
	}
	if from == publish.StatusPublished || from == publish.StatusFailed {
		return fmt.Errorf("validate publish final transition: %w", ErrInvalidInput)
	}
	return nil
}

func samePublishUpdate(current publish.Record, input UpdatePublishStatusInput) bool {
	return current.Status == input.Status && current.WeChatArticleID == input.WeChatArticleID && current.ErrorCode == input.ErrorCode && current.ErrorMessage == input.ErrorMessage
}

func isPublishStatus(status publish.Status) bool {
	return status == publish.StatusPublishing || status == publish.StatusPublished || status == publish.StatusFailed
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
