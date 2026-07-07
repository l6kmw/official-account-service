CREATE TABLE IF NOT EXISTS wechat_callback_event (
  id BIGSERIAL PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  component_app_id TEXT NOT NULL,
  authorizer_app_id TEXT NOT NULL,
  event_type TEXT NOT NULL,
  event_key TEXT NOT NULL,
  raw_body TEXT NOT NULL,
  received_at TIMESTAMPTZ NOT NULL,
  retain_until TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_wechat_callback_event_dedupe
  ON wechat_callback_event (tenant_id, event_type, event_key);

CREATE INDEX IF NOT EXISTS idx_wechat_callback_event_retain_until
  ON wechat_callback_event (retain_until);
