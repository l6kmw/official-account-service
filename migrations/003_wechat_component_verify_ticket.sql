CREATE TABLE IF NOT EXISTS wechat_component_verify_ticket (
  component_app_id TEXT PRIMARY KEY,
  verify_ticket TEXT NOT NULL,
  received_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
