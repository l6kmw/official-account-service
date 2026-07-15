CREATE TABLE IF NOT EXISTS app_user (
  id TEXT PRIMARY KEY,
  username TEXT NOT NULL,
  password_hash TEXT NOT NULL DEFAULT '',
  role TEXT NOT NULL DEFAULT 'user' CHECK (role IN ('admin', 'user')),
  status TEXT NOT NULL DEFAULT 'disabled' CHECK (status IN ('active', 'disabled')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_app_user_username_lower
  ON app_user (LOWER(username));

WITH existing_spaces AS (
  SELECT tenant_id FROM wechat_authorization_account
  UNION SELECT tenant_id FROM wechat_article
  UNION SELECT tenant_id FROM wechat_media_asset
  UNION SELECT tenant_id FROM wechat_publish_record
  UNION SELECT tenant_id FROM wechat_authorizer_tenant_binding
  UNION SELECT tenant_id FROM wechat_callback_event
  UNION SELECT tenant_id FROM wechat_authorization_state
)
INSERT INTO app_user (id, username, role, status)
SELECT tenant_id, 'legacy-' || SUBSTRING(MD5(tenant_id), 1, 16), 'user', 'disabled'
FROM existing_spaces
WHERE tenant_id <> ''
ON CONFLICT (id) DO NOTHING;
