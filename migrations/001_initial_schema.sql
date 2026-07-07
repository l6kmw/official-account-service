CREATE TABLE IF NOT EXISTS wechat_article (
  id BIGSERIAL PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  authorizer_id BIGINT NOT NULL,
  title TEXT NOT NULL,
  author TEXT NOT NULL DEFAULT '',
  digest TEXT NOT NULL DEFAULT '',
  content_html TEXT NOT NULL DEFAULT '',
  cover_media_asset_id BIGINT NOT NULL DEFAULT 0,
  status TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_wechat_article_tenant_id ON wechat_article (tenant_id);
CREATE INDEX IF NOT EXISTS idx_wechat_article_tenant_authorizer ON wechat_article (tenant_id, authorizer_id);

CREATE TABLE IF NOT EXISTS wechat_media_asset (
  id BIGSERIAL PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  authorizer_id BIGINT NOT NULL,
  article_id BIGINT NOT NULL,
  usage TEXT NOT NULL,
  local_url TEXT NOT NULL DEFAULT '',
  wechat_url TEXT NOT NULL DEFAULT '',
  media_id TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_wechat_media_asset_tenant_article ON wechat_media_asset (tenant_id, article_id);

CREATE TABLE IF NOT EXISTS wechat_publish_record (
  id BIGSERIAL PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  authorizer_id BIGINT NOT NULL,
  article_id BIGINT NOT NULL,
  wechat_publish_id TEXT NOT NULL DEFAULT '',
  wechat_article_id TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL,
  error_code TEXT NOT NULL DEFAULT '',
  error_message TEXT NOT NULL DEFAULT '',
  submitted_at TIMESTAMPTZ,
  finished_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_wechat_publish_record_tenant_article ON wechat_publish_record (tenant_id, article_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_wechat_publish_record_publish_id ON wechat_publish_record (wechat_publish_id) WHERE wechat_publish_id <> '';
