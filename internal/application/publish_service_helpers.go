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
