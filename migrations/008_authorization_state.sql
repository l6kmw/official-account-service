CREATE TABLE IF NOT EXISTS wechat_authorization_state (
  state_digest TEXT PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  component_app_id TEXT NOT NULL,
  expires_at TIMESTAMPTZ NOT NULL,
  consumed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_wechat_authorization_state_expires_at
  ON wechat_authorization_state (expires_at);
