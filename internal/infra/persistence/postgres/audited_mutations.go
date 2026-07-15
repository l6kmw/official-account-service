package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"

	"official-account-service/internal/domain/agentaudit"
	"official-account-service/internal/domain/article"
	"official-account-service/internal/domain/publish"
)

var _ article.AuditedMutationRepository = (*Store)(nil)
var _ publish.AuditedCreateRepository = (*Store)(nil)

// CreateWithAudit creates one article and audit entry in a single database transaction.
func (s *Store) CreateWithAudit(ctx context.Context, tenantID string, draft article.Article, audit agentaudit.Entry) (article.Article, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return article.Article{}, fmt.Errorf("begin create article audit transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	row := tx.QueryRowContext(ctx, `
		INSERT INTO wechat_article (
			tenant_id, authorizer_id, title, author, digest, content_html, cover_media_asset_id, status,
			created_by_agent_id, updated_by_agent_id, version
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NULLIF($9, ''), NULLIF($10, ''), 1)
		RETURNING id, tenant_id, authorizer_id, title, author, digest, content_html, cover_media_asset_id, status,
		          COALESCE(created_by_agent_id, ''), COALESCE(updated_by_agent_id, ''), version, created_at, updated_at`,
		tenantID, draft.AuthorizerID, draft.Title, draft.Author, draft.Digest, draft.ContentHTML, draft.CoverMediaAssetID, draft.Status,
		draft.CreatedByAgentID, draft.UpdatedByAgentID)
	created, err := scanArticleRow(row)
	if err != nil {
		return article.Article{}, fmt.Errorf("create article in audit transaction: %w", err)
	}
	audit.ResourceID = strconv.FormatInt(created.ID, 10)
	if _, err := appendAgentAuditTx(ctx, tx, audit); err != nil {
		return article.Article{}, fmt.Errorf("append create article audit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return article.Article{}, fmt.Errorf("commit create article audit transaction: %w", err)
	}
	return created, nil
}

// UpdateWithAudit updates one article and appends its audit entry in a single database transaction.
func (s *Store) UpdateWithAudit(ctx context.Context, tenantID string, draft article.Article, audit agentaudit.Entry) (article.Article, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return article.Article{}, fmt.Errorf("begin update article audit transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	row := tx.QueryRowContext(ctx, `
		UPDATE wechat_article
		SET title = $3, author = $4, digest = $5, content_html = $6, cover_media_asset_id = $7, status = $8,
		    updated_by_agent_id = NULLIF($9, ''), version = version + 1, updated_at = NOW()
		WHERE tenant_id = $1 AND id = $2 AND version = $10
		RETURNING id, tenant_id, authorizer_id, title, author, digest, content_html, cover_media_asset_id, status,
		          COALESCE(created_by_agent_id, ''), COALESCE(updated_by_agent_id, ''), version, created_at, updated_at`,
		tenantID, draft.ID, draft.Title, draft.Author, draft.Digest, draft.ContentHTML, draft.CoverMediaAssetID, draft.Status,
		draft.UpdatedByAgentID, draft.Version)
	updated, err := scanArticleRow(row)
	if err != nil {
		return article.Article{}, mapArticleUpdateTxError(ctx, tx, tenantID, draft.ID, err)
	}
	audit.ResourceID = strconv.FormatInt(updated.ID, 10)
	if _, err := appendAgentAuditTx(ctx, tx, audit); err != nil {
		return article.Article{}, fmt.Errorf("append update article audit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return article.Article{}, fmt.Errorf("commit update article audit transaction: %w", err)
	}
	return updated, nil
}

// DeleteWithAudit deletes one article and appends its audit entry in a single database transaction.
func (s *Store) DeleteWithAudit(ctx context.Context, tenantID string, id int64, audit agentaudit.Entry) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin delete article audit transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	result, err := tx.ExecContext(ctx, `DELETE FROM wechat_article WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	if err != nil {
		return fmt.Errorf("delete article in audit transaction: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete article audit rows affected: %w", err)
	}
	if count == 0 {
		return fmt.Errorf("delete article audit lookup: %w", article.ErrNotFound)
	}
	audit.ResourceID = strconv.FormatInt(id, 10)
	if _, err := appendAgentAuditTx(ctx, tx, audit); err != nil {
		return fmt.Errorf("append delete article audit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit delete article audit transaction: %w", err)
	}
	return nil
}

// CreatePublishRecordWithAudit creates one publish intent and audit entry in a single database transaction.
func (s *Store) CreatePublishRecordWithAudit(ctx context.Context, tenantID string, record publish.Record, audit agentaudit.Entry) (publish.Record, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return publish.Record{}, fmt.Errorf("begin publish audit transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	row := tx.QueryRowContext(ctx, `
		INSERT INTO wechat_publish_record (
			tenant_id, authorizer_id, article_id, wechat_publish_id, wechat_article_id,
			status, error_code, error_message, submitted_at, finished_at, article_created_by_agent_id
		)
		VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
			(SELECT created_by_agent_id FROM wechat_article WHERE tenant_id = $1 AND id = $3)
		)
		RETURNING id, tenant_id, authorizer_id, article_id, wechat_publish_id, wechat_article_id, status, error_code, error_message,
		          submitted_at, finished_at, COALESCE(article_created_by_agent_id, ''), created_at, updated_at`,
		tenantID, record.AuthorizerID, record.ArticleID, record.WeChatPublishID, record.WeChatArticleID, record.Status,
		record.ErrorCode, record.ErrorMessage, nullableTime(record.SubmittedAt), nullableTime(record.FinishedAt))
	created, err := scanPublishRecordRow(row)
	if err != nil {
		if isUniqueViolation(err) {
			return publish.Record{}, fmt.Errorf("create publish record in progress: %w", publish.ErrPublishInProgress)
		}
		return publish.Record{}, fmt.Errorf("create publish record in audit transaction: %w", err)
	}
	audit.ResourceID = strconv.FormatInt(record.ArticleID, 10)
	if _, err := appendAgentAuditTx(ctx, tx, audit); err != nil {
		return publish.Record{}, fmt.Errorf("append publish audit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return publish.Record{}, fmt.Errorf("commit publish audit transaction: %w", err)
	}
	return created, nil
}

func appendAgentAuditTx(ctx context.Context, tx *sql.Tx, entry agentaudit.Entry) (agentaudit.Entry, error) {
	row := tx.QueryRowContext(ctx, `
		INSERT INTO app_agent_audit_log (user_id, agent_record_id, action, resource_type, resource_id)
		SELECT $1, NULLIF($2, ''), $3, $4, $5
		WHERE NULLIF($2, '') IS NULL
		   OR EXISTS (
			SELECT 1 FROM app_agent WHERE id = $2 AND user_id = $1
		   )
		RETURNING id, user_id, COALESCE(agent_record_id, ''), action, resource_type, resource_id, created_at`,
		entry.UserID, entry.AgentRecordID, entry.Action, entry.ResourceType, entry.ResourceID)
	created, err := scanAgentAudit(row)
	if err != nil {
		return agentaudit.Entry{}, fmt.Errorf("append agent audit: %w", err)
	}
	return created, nil
}

func mapArticleUpdateTxError(ctx context.Context, tx *sql.Tx, tenantID string, id int64, err error) error {
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("update article: %w", err)
	}
	var exists bool
	if queryErr := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM wechat_article WHERE tenant_id = $1 AND id = $2)`, tenantID, id).Scan(&exists); queryErr != nil {
		return fmt.Errorf("resolve article update conflict: %w", errors.Join(err, queryErr))
	}
	if exists {
		return fmt.Errorf("update article version: %w", article.ErrVersionConflict)
	}
	return fmt.Errorf("update article lookup: %w", article.ErrNotFound)
}
