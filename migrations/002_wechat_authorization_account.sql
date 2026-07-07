CREATE TABLE IF NOT EXISTS wechat_authorization_account (
  id BIGSERIAL PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  app_id TEXT NOT NULL,
  name TEXT NOT NULL DEFAULT '',
  avatar_url TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL,
  last_synced_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_wechat_authorization_account_tenant_id ON wechat_authorization_account (tenant_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_wechat_authorization_account_tenant_app ON wechat_authorization_account (tenant_id, app_id);
