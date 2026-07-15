package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"official-account-service/internal/domain/article"
	"official-account-service/internal/domain/authorization"
	"official-account-service/internal/domain/material"
)

// Store is a PostgreSQL repository implementation.
type Store struct {
	db *sql.DB
}

// Open connects to PostgreSQL and verifies the connection.
func Open(ctx context.Context, dsn string) (*Store, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return &Store{db: db}, nil
}

// Close releases database resources.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	if err := s.db.Close(); err != nil {
		return fmt.Errorf("close postgres: %w", err)
	}
	return nil
}

// CreateAccount stores a tenant-scoped official account.
func (s *Store) CreateAccount(ctx context.Context, tenantID string, account authorization.Account) (authorization.Account, error) {
	row := s.db.QueryRowContext(ctx, `
		INSERT INTO wechat_authorization_account (tenant_id, app_id, name, avatar_url, status, encrypted_authorizer_refresh_token, last_synced_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, tenant_id, app_id, name, avatar_url, status, encrypted_authorizer_refresh_token,
		          COALESCE(last_synced_at, '0001-01-01 00:00:00+00'::timestamptz), created_at, updated_at`,
		tenantID, account.AppID, account.Name, account.AvatarURL, account.Status, account.EncryptedAuthorizerRefreshToken, nullableTime(account.LastSyncedAt))
	created, err := scanAccountRow(row)
	if err != nil {
		return authorization.Account{}, mapAccountError("create account", err)
	}
	return created, nil
}

// SaveAccount creates or updates a tenant-scoped official account by app id.
func (s *Store) SaveAccount(ctx context.Context, tenantID string, account authorization.Account) (authorization.Account, error) {
	row := s.db.QueryRowContext(ctx, `
		INSERT INTO wechat_authorization_account (tenant_id, app_id, name, avatar_url, status, encrypted_authorizer_refresh_token, last_synced_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (tenant_id, app_id) DO UPDATE SET
			name = EXCLUDED.name,
			avatar_url = EXCLUDED.avatar_url,
			status = EXCLUDED.status,
			encrypted_authorizer_refresh_token = EXCLUDED.encrypted_authorizer_refresh_token,
			last_synced_at = EXCLUDED.last_synced_at,
			updated_at = NOW()
		RETURNING id, tenant_id, app_id, name, avatar_url, status, encrypted_authorizer_refresh_token,
		          COALESCE(last_synced_at, '0001-01-01 00:00:00+00'::timestamptz), created_at, updated_at`,
		tenantID, account.AppID, account.Name, account.AvatarURL, account.Status, account.EncryptedAuthorizerRefreshToken, nullableTime(account.LastSyncedAt))
	saved, err := scanAccountRow(row)
	if err != nil {
		return authorization.Account{}, mapAccountError("save account", err)
	}
	return saved, nil
}

// GetAccount returns one tenant-scoped official account.
func (s *Store) GetAccount(ctx context.Context, tenantID string, id int64) (authorization.Account, error) {
	row := s.db.QueryRowContext(ctx, accountSelectSQL+` WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	account, err := scanAccountRow(row)
	if err != nil {
		return authorization.Account{}, mapAccountError("get account", err)
	}
	return account, nil
}

// ListAccounts returns tenant-scoped official accounts.
func (s *Store) ListAccounts(ctx context.Context, tenantID string) ([]authorization.Account, error) {
	rows, err := s.db.QueryContext(ctx, accountSelectSQL+` WHERE tenant_id = $1 ORDER BY id`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list accounts query: %w", err)
	}
	defer rows.Close()
	items := make([]authorization.Account, 0)
	for rows.Next() {
		account, err := scanAccountRows(rows)
		if err != nil {
			return nil, fmt.Errorf("list accounts scan: %w", err)
		}
		items = append(items, account)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list accounts rows: %w", err)
	}
	return items, nil
}

// RevokeAccountByAppID revokes a tenant-scoped official account by app id.
func (s *Store) RevokeAccountByAppID(ctx context.Context, tenantID string, appID string) (authorization.Account, error) {
	row := s.db.QueryRowContext(ctx, `
		UPDATE wechat_authorization_account
		SET status = $3, encrypted_authorizer_refresh_token = '', updated_at = NOW()
		WHERE tenant_id = $1 AND app_id = $2
		RETURNING id, tenant_id, app_id, name, avatar_url, status, encrypted_authorizer_refresh_token,
		          COALESCE(last_synced_at, '0001-01-01 00:00:00+00'::timestamptz), created_at, updated_at`,
		tenantID, appID, authorization.AccountStatusRevoked)
	account, err := scanAccountRow(row)
	if err != nil {
		return authorization.Account{}, mapAccountError("revoke account by app id", err)
	}
	return account, nil
}

// UpdateAccountStatus updates one tenant-scoped official account status.
func (s *Store) UpdateAccountStatus(ctx context.Context, tenantID string, id int64, status authorization.AccountStatus) (authorization.Account, error) {
	row := s.db.QueryRowContext(ctx, `
		UPDATE wechat_authorization_account SET status = $3, updated_at = NOW()
		WHERE tenant_id = $1 AND id = $2
		RETURNING id, tenant_id, app_id, name, avatar_url, status, encrypted_authorizer_refresh_token,
		          COALESCE(last_synced_at, '0001-01-01 00:00:00+00'::timestamptz), created_at, updated_at`, tenantID, id, status)
	account, err := scanAccountRow(row)
	if err != nil {
		return authorization.Account{}, mapAccountError("update account status", err)
	}
	return account, nil
}

// Create stores a tenant-scoped article.
func (s *Store) Create(ctx context.Context, tenantID string, draft article.Article) (article.Article, error) {
	row := s.db.QueryRowContext(ctx, `
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
		return article.Article{}, fmt.Errorf("create article: %w", err)
	}
	return created, nil
}

// Get returns one tenant-scoped article.
func (s *Store) Get(ctx context.Context, tenantID string, id int64) (article.Article, error) {
	row := s.db.QueryRowContext(ctx, articleSelectSQL+` WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	draft, err := scanArticleRow(row)
	if err != nil {
		return article.Article{}, mapArticleError("get article", err)
	}
	return draft, nil
}

// List returns tenant-scoped articles.
func (s *Store) List(ctx context.Context, tenantID string) ([]article.Article, error) {
	rows, err := s.db.QueryContext(ctx, articleSelectSQL+` WHERE tenant_id = $1 ORDER BY id`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list articles query: %w", err)
	}
	defer rows.Close()
	items := make([]article.Article, 0)
	for rows.Next() {
		draft, err := scanArticleRows(rows)
		if err != nil {
			return nil, fmt.Errorf("list articles scan: %w", err)
		}
		items = append(items, draft)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list articles rows: %w", err)
	}
	return items, nil
}

// ListByCreatedByAgent returns tenant-scoped articles owned by one creating Agent.
func (s *Store) ListByCreatedByAgent(ctx context.Context, tenantID string, agentRecordID string) ([]article.Article, error) {
	rows, err := s.db.QueryContext(ctx, articleSelectSQL+` WHERE tenant_id = $1 AND created_by_agent_id = $2 ORDER BY id`, tenantID, agentRecordID)
	if err != nil {
		return nil, fmt.Errorf("list articles by agent query: %w", err)
	}
	defer rows.Close()
	items := make([]article.Article, 0)
	for rows.Next() {
		draft, err := scanArticleRows(rows)
		if err != nil {
			return nil, fmt.Errorf("list articles by agent scan: %w", err)
		}
		items = append(items, draft)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list articles by agent rows: %w", err)
	}
	return items, nil
}

// Update replaces a tenant-scoped article.
func (s *Store) Update(ctx context.Context, tenantID string, draft article.Article) (article.Article, error) {
	row := s.db.QueryRowContext(ctx, `
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
		return article.Article{}, s.mapArticleUpdateError(ctx, tenantID, draft.ID, err)
	}
	return updated, nil
}

// CreateMaterial stores a tenant-scoped material asset.
func (s *Store) CreateMaterial(ctx context.Context, tenantID string, asset material.Asset) (material.Asset, error) {
	row := s.db.QueryRowContext(ctx, `
		INSERT INTO wechat_media_asset (tenant_id, authorizer_id, article_id, usage, local_url, wechat_url, media_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, tenant_id, authorizer_id, article_id, usage, local_url, wechat_url, media_id, created_at`,
		tenantID, asset.AuthorizerID, asset.ArticleID, asset.Usage, asset.LocalURL, asset.WeChatURL, asset.MediaID)
	created, err := scanMaterialRow(row)
	if err != nil {
		return material.Asset{}, fmt.Errorf("create material: %w", err)
	}
	return created, nil
}

// GetMaterial returns one tenant-scoped material asset.
func (s *Store) GetMaterial(ctx context.Context, tenantID string, id int64) (material.Asset, error) {
	row := s.db.QueryRowContext(ctx, materialSelectSQL+` WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	asset, err := scanMaterialRow(row)
	if err != nil {
		return material.Asset{}, mapMaterialError("get material", err)
	}
	return asset, nil
}

// ListMaterialByArticle returns material assets for an article.
func (s *Store) ListMaterialByArticle(ctx context.Context, tenantID string, articleID int64) ([]material.Asset, error) {
	rows, err := s.db.QueryContext(ctx, materialSelectSQL+` WHERE tenant_id = $1 AND article_id = $2 ORDER BY id`, tenantID, articleID)
	if err != nil {
		return nil, fmt.Errorf("list materials query: %w", err)
	}
	defer rows.Close()
	items := make([]material.Asset, 0)
	for rows.Next() {
		asset, err := scanMaterialRows(rows)
		if err != nil {
			return nil, fmt.Errorf("list materials scan: %w", err)
		}
		items = append(items, asset)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list materials rows: %w", err)
	}
	return items, nil
}

// Delete removes a tenant-scoped article.
func (s *Store) Delete(ctx context.Context, tenantID string, id int64) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM wechat_article WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	if err != nil {
		return fmt.Errorf("delete article exec: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete article rows affected: %w", err)
	}
	if count == 0 {
		return fmt.Errorf("delete article lookup: %w", article.ErrNotFound)
	}
	return nil
}

const accountSelectSQL = `
	SELECT id, tenant_id, app_id, name, avatar_url, status, encrypted_authorizer_refresh_token,
	       COALESCE(last_synced_at, '0001-01-01 00:00:00+00'::timestamptz), created_at, updated_at
	FROM wechat_authorization_account`

const articleSelectSQL = `
	SELECT id, tenant_id, authorizer_id, title, author, digest, content_html, cover_media_asset_id, status,
	       COALESCE(created_by_agent_id, ''), COALESCE(updated_by_agent_id, ''), version, created_at, updated_at
	FROM wechat_article`

const materialSelectSQL = `
	SELECT id, tenant_id, authorizer_id, article_id, usage, local_url, wechat_url, media_id, created_at
	FROM wechat_media_asset`

func scanAccountRow(row *sql.Row) (authorization.Account, error) {
	var account authorization.Account
	if err := row.Scan(&account.ID, &account.TenantID, &account.AppID, &account.Name, &account.AvatarURL, &account.Status, &account.EncryptedAuthorizerRefreshToken, &account.LastSyncedAt, &account.CreatedAt, &account.UpdatedAt); err != nil {
		return authorization.Account{}, fmt.Errorf("scan account: %w", err)
	}
	return account, nil
}

func scanAccountRows(rows *sql.Rows) (authorization.Account, error) {
	var account authorization.Account
	if err := rows.Scan(&account.ID, &account.TenantID, &account.AppID, &account.Name, &account.AvatarURL, &account.Status, &account.EncryptedAuthorizerRefreshToken, &account.LastSyncedAt, &account.CreatedAt, &account.UpdatedAt); err != nil {
		return authorization.Account{}, fmt.Errorf("scan account: %w", err)
	}
	return account, nil
}

func scanArticleRow(row *sql.Row) (article.Article, error) {
	var draft article.Article
	if err := row.Scan(
		&draft.ID, &draft.TenantID, &draft.AuthorizerID, &draft.Title, &draft.Author, &draft.Digest, &draft.ContentHTML,
		&draft.CoverMediaAssetID, &draft.Status, &draft.CreatedByAgentID, &draft.UpdatedByAgentID, &draft.Version,
		&draft.CreatedAt, &draft.UpdatedAt,
	); err != nil {
		return article.Article{}, fmt.Errorf("scan article: %w", err)
	}
	return draft, nil
}

func scanArticleRows(rows *sql.Rows) (article.Article, error) {
	var draft article.Article
	if err := rows.Scan(
		&draft.ID, &draft.TenantID, &draft.AuthorizerID, &draft.Title, &draft.Author, &draft.Digest, &draft.ContentHTML,
		&draft.CoverMediaAssetID, &draft.Status, &draft.CreatedByAgentID, &draft.UpdatedByAgentID, &draft.Version,
		&draft.CreatedAt, &draft.UpdatedAt,
	); err != nil {
		return article.Article{}, fmt.Errorf("scan article: %w", err)
	}
	return draft, nil
}

func scanMaterialRow(row *sql.Row) (material.Asset, error) {
	var asset material.Asset
	if err := row.Scan(&asset.ID, &asset.TenantID, &asset.AuthorizerID, &asset.ArticleID, &asset.Usage, &asset.LocalURL, &asset.WeChatURL, &asset.MediaID, &asset.CreatedAt); err != nil {
		return material.Asset{}, fmt.Errorf("scan material: %w", err)
	}
	return asset, nil
}

func scanMaterialRows(rows *sql.Rows) (material.Asset, error) {
	var asset material.Asset
	if err := rows.Scan(&asset.ID, &asset.TenantID, &asset.AuthorizerID, &asset.ArticleID, &asset.Usage, &asset.LocalURL, &asset.WeChatURL, &asset.MediaID, &asset.CreatedAt); err != nil {
		return material.Asset{}, fmt.Errorf("scan material: %w", err)
	}
	return asset, nil
}

func mapAccountError(action string, err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%s: %w", action, authorization.ErrNotFound)
	}
	if isUniqueViolation(err) {
		return fmt.Errorf("%s: %w", action, authorization.ErrConflict)
	}
	return fmt.Errorf("%s: %w", action, err)
}

func mapArticleError(action string, err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%s: %w", action, article.ErrNotFound)
	}
	return fmt.Errorf("%s: %w", action, err)
}

func (s *Store) mapArticleUpdateError(ctx context.Context, tenantID string, id int64, err error) error {
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("update article: %w", err)
	}
	var exists bool
	if queryErr := s.db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM wechat_article WHERE tenant_id = $1 AND id = $2)`, tenantID, id).Scan(&exists); queryErr != nil {
		return fmt.Errorf("resolve article update conflict: %w", errors.Join(err, queryErr))
	}
	if exists {
		return fmt.Errorf("update article version: %w", article.ErrVersionConflict)
	}
	return fmt.Errorf("update article lookup: %w", article.ErrNotFound)
}

func mapMaterialError(action string, err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%s: %w", action, material.ErrNotFound)
	}
	return fmt.Errorf("%s: %w", action, err)
}

func nullableTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}
