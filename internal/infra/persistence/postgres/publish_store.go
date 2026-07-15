package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"official-account-service/internal/domain/publish"
)

// CreatePublishRecord stores a tenant-scoped publish record.
func (s *Store) CreatePublishRecord(ctx context.Context, tenantID string, record publish.Record) (publish.Record, error) {
	row := s.db.QueryRowContext(ctx, `
		INSERT INTO wechat_publish_record (tenant_id, authorizer_id, article_id, wechat_publish_id, wechat_article_id, status, error_code, error_message, submitted_at, finished_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, tenant_id, authorizer_id, article_id, wechat_publish_id, wechat_article_id, status, error_code, error_message,
		          submitted_at, finished_at, created_at, updated_at`,
		tenantID, record.AuthorizerID, record.ArticleID, record.WeChatPublishID, record.WeChatArticleID, record.Status,
		record.ErrorCode, record.ErrorMessage, nullableTime(record.SubmittedAt), nullableTime(record.FinishedAt))
	created, err := scanPublishRecordRow(row)
	if err != nil {
		if isUniqueViolation(err) {
			return publish.Record{}, fmt.Errorf("create publish record in progress: %w", publish.ErrPublishInProgress)
		}
		return publish.Record{}, fmt.Errorf("create publish record: %w", err)
	}
	return s.attachArticleCreatedAgent(ctx, created)
}

// UpdatePublishRecordSubmission stores the WeChat publish id for an active publish record.
func (s *Store) UpdatePublishRecordSubmission(ctx context.Context, tenantID string, record publish.Record) (publish.Record, error) {
	row := s.db.QueryRowContext(ctx, `
		UPDATE wechat_publish_record
		SET wechat_publish_id = $4, updated_at = NOW()
		WHERE tenant_id = $1 AND id = $2 AND article_id = $3 AND status = 'publishing'
		RETURNING id, tenant_id, authorizer_id, article_id, wechat_publish_id, wechat_article_id, status, error_code, error_message,
		          submitted_at, finished_at, created_at, updated_at`,
		tenantID, record.ID, record.ArticleID, record.WeChatPublishID)
	updated, err := scanPublishRecordRow(row)
	if err != nil {
		return publish.Record{}, mapPublishError("update publish record submission", err)
	}
	return s.attachArticleCreatedAgent(ctx, updated)
}

// GetPublishRecord returns one tenant-scoped publish record.
func (s *Store) GetPublishRecord(ctx context.Context, tenantID string, id int64) (publish.Record, error) {
	row := s.db.QueryRowContext(ctx, publishRecordWithAgentSelectSQL+` WHERE record.tenant_id = $1 AND record.id = $2`, tenantID, id)
	record, err := scanPublishRecordWithAgentRow(row)
	if err != nil {
		return publish.Record{}, mapPublishError("get publish record", err)
	}
	return record, nil
}

// GetPublishRecordByPublishID returns one tenant-scoped publish record by WeChat publish id.
func (s *Store) GetPublishRecordByPublishID(ctx context.Context, tenantID string, publishID string) (publish.Record, error) {
	row := s.db.QueryRowContext(ctx, publishRecordWithAgentSelectSQL+` WHERE record.tenant_id = $1 AND record.wechat_publish_id = $2`, tenantID, publishID)
	record, err := scanPublishRecordWithAgentRow(row)
	if err != nil {
		return publish.Record{}, mapPublishError("get publish record by publish id", err)
	}
	return record, nil
}

// ListPublishRecords returns tenant-scoped publish records.
func (s *Store) ListPublishRecords(ctx context.Context, tenantID string) ([]publish.Record, error) {
	rows, err := s.db.QueryContext(ctx, publishRecordWithAgentSelectSQL+` WHERE record.tenant_id = $1 ORDER BY record.id`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list publish records query: %w", err)
	}
	defer rows.Close()
	items := make([]publish.Record, 0)
	for rows.Next() {
		record, err := scanPublishRecordWithAgentRows(rows)
		if err != nil {
			return nil, fmt.Errorf("list publish records scan: %w", err)
		}
		items = append(items, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list publish records rows: %w", err)
	}
	return items, nil
}

// ListPublishRecordsByArticle returns publish records for an article.
func (s *Store) ListPublishRecordsByArticle(ctx context.Context, tenantID string, articleID int64) ([]publish.Record, error) {
	rows, err := s.db.QueryContext(ctx, publishRecordWithAgentSelectSQL+` WHERE record.tenant_id = $1 AND record.article_id = $2 ORDER BY record.id`, tenantID, articleID)
	if err != nil {
		return nil, fmt.Errorf("list publish records query: %w", err)
	}
	defer rows.Close()
	items := make([]publish.Record, 0)
	for rows.Next() {
		record, err := scanPublishRecordWithAgentRows(rows)
		if err != nil {
			return nil, fmt.Errorf("list publish records scan: %w", err)
		}
		items = append(items, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list publish records rows: %w", err)
	}
	return items, nil
}

// ListPublishRecordsByAgent returns records whose article was created by the selected Agent.
func (s *Store) ListPublishRecordsByAgent(ctx context.Context, tenantID string, agentRecordID string) ([]publish.Record, error) {
	rows, err := s.db.QueryContext(ctx, publishRecordWithAgentSelectSQL+`
		WHERE record.tenant_id = $1 AND article.created_by_agent_id = $2 ORDER BY record.id`, tenantID, agentRecordID)
	if err != nil {
		return nil, fmt.Errorf("list publish records by agent query: %w", err)
	}
	defer rows.Close()
	items := make([]publish.Record, 0)
	for rows.Next() {
		record, err := scanPublishRecordWithAgentRows(rows)
		if err != nil {
			return nil, fmt.Errorf("list publish records by agent scan: %w", err)
		}
		items = append(items, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list publish records by agent rows: %w", err)
	}
	return items, nil
}

// UpdatePublishRecordStatus updates a tenant-scoped publish record status.
func (s *Store) UpdatePublishRecordStatus(ctx context.Context, tenantID string, record publish.Record) (publish.Record, error) {
	row := s.db.QueryRowContext(ctx, `
		UPDATE wechat_publish_record
		SET wechat_article_id = $4, status = $5, error_code = $6, error_message = $7, finished_at = $8, updated_at = NOW()
		WHERE tenant_id = $1 AND id = $2 AND article_id = $3
		RETURNING id, tenant_id, authorizer_id, article_id, wechat_publish_id, wechat_article_id, status, error_code, error_message,
		          submitted_at, finished_at, created_at, updated_at`,
		tenantID, record.ID, record.ArticleID, record.WeChatArticleID, record.Status, record.ErrorCode, record.ErrorMessage, nullableTime(record.FinishedAt))
	updated, err := scanPublishRecordRow(row)
	if err != nil {
		return publish.Record{}, mapPublishError("update publish record", err)
	}
	return s.attachArticleCreatedAgent(ctx, updated)
}

func (s *Store) attachArticleCreatedAgent(ctx context.Context, record publish.Record) (publish.Record, error) {
	err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(created_by_agent_id, '')
		FROM wechat_article
		WHERE tenant_id = $1 AND id = $2`, record.TenantID, record.ArticleID).Scan(&record.ArticleCreatedByAgentID)
	if errors.Is(err, sql.ErrNoRows) {
		return record, nil
	}
	if err != nil {
		return publish.Record{}, fmt.Errorf("attach publish article agent: %w", err)
	}
	return record, nil
}

const publishRecordWithAgentSelectSQL = `
	SELECT record.id, record.tenant_id, record.authorizer_id, record.article_id, record.wechat_publish_id,
	       record.wechat_article_id, record.status, record.error_code, record.error_message,
	       record.submitted_at, record.finished_at, COALESCE(article.created_by_agent_id, ''),
	       record.created_at, record.updated_at
	FROM wechat_publish_record AS record
	LEFT JOIN wechat_article AS article
	  ON article.tenant_id = record.tenant_id AND article.id = record.article_id`

func scanPublishRecordRow(row *sql.Row) (publish.Record, error) {
	var record publish.Record
	var submittedAt sql.NullTime
	var finishedAt sql.NullTime
	if err := row.Scan(&record.ID, &record.TenantID, &record.AuthorizerID, &record.ArticleID, &record.WeChatPublishID, &record.WeChatArticleID, &record.Status, &record.ErrorCode, &record.ErrorMessage, &submittedAt, &finishedAt, &record.CreatedAt, &record.UpdatedAt); err != nil {
		return publish.Record{}, fmt.Errorf("scan publish record: %w", err)
	}
	record.SubmittedAt = nullTimeValue(submittedAt)
	record.FinishedAt = nullTimeValue(finishedAt)
	return record, nil
}

func scanPublishRecordRows(rows *sql.Rows) (publish.Record, error) {
	var record publish.Record
	var submittedAt sql.NullTime
	var finishedAt sql.NullTime
	if err := rows.Scan(&record.ID, &record.TenantID, &record.AuthorizerID, &record.ArticleID, &record.WeChatPublishID, &record.WeChatArticleID, &record.Status, &record.ErrorCode, &record.ErrorMessage, &submittedAt, &finishedAt, &record.CreatedAt, &record.UpdatedAt); err != nil {
		return publish.Record{}, fmt.Errorf("scan publish record: %w", err)
	}
	record.SubmittedAt = nullTimeValue(submittedAt)
	record.FinishedAt = nullTimeValue(finishedAt)
	return record, nil
}

func scanPublishRecordWithAgentRow(row *sql.Row) (publish.Record, error) {
	var record publish.Record
	var submittedAt sql.NullTime
	var finishedAt sql.NullTime
	if err := row.Scan(
		&record.ID, &record.TenantID, &record.AuthorizerID, &record.ArticleID, &record.WeChatPublishID,
		&record.WeChatArticleID, &record.Status, &record.ErrorCode, &record.ErrorMessage, &submittedAt, &finishedAt,
		&record.ArticleCreatedByAgentID, &record.CreatedAt, &record.UpdatedAt,
	); err != nil {
		return publish.Record{}, fmt.Errorf("scan publish record with agent: %w", err)
	}
	record.SubmittedAt = nullTimeValue(submittedAt)
	record.FinishedAt = nullTimeValue(finishedAt)
	return record, nil
}

func scanPublishRecordWithAgentRows(rows *sql.Rows) (publish.Record, error) {
	var record publish.Record
	var submittedAt sql.NullTime
	var finishedAt sql.NullTime
	if err := rows.Scan(
		&record.ID, &record.TenantID, &record.AuthorizerID, &record.ArticleID, &record.WeChatPublishID,
		&record.WeChatArticleID, &record.Status, &record.ErrorCode, &record.ErrorMessage, &submittedAt, &finishedAt,
		&record.ArticleCreatedByAgentID, &record.CreatedAt, &record.UpdatedAt,
	); err != nil {
		return publish.Record{}, fmt.Errorf("scan publish record with agent: %w", err)
	}
	record.SubmittedAt = nullTimeValue(submittedAt)
	record.FinishedAt = nullTimeValue(finishedAt)
	return record, nil
}

func nullTimeValue(value sql.NullTime) time.Time {
	if value.Valid {
		return value.Time
	}
	return time.Time{}
}

func mapPublishError(action string, err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%s: %w", action, publish.ErrNotFound)
	}
	return fmt.Errorf("%s: %w", action, err)
}
