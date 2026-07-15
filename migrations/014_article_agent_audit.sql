ALTER TABLE wechat_article
  ADD COLUMN IF NOT EXISTS created_by_agent_id TEXT,
  ADD COLUMN IF NOT EXISTS updated_by_agent_id TEXT,
  ADD COLUMN IF NOT EXISTS version BIGINT NOT NULL DEFAULT 1;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_wechat_article_created_by_agent') THEN
    ALTER TABLE wechat_article
      ADD CONSTRAINT fk_wechat_article_created_by_agent
      FOREIGN KEY (created_by_agent_id) REFERENCES app_agent (id) ON DELETE SET NULL;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_wechat_article_updated_by_agent') THEN
    ALTER TABLE wechat_article
      ADD CONSTRAINT fk_wechat_article_updated_by_agent
      FOREIGN KEY (updated_by_agent_id) REFERENCES app_agent (id) ON DELETE SET NULL;
  END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_wechat_article_tenant_created_agent
  ON wechat_article (tenant_id, created_by_agent_id);

CREATE TABLE IF NOT EXISTS app_agent_audit_log (
  id BIGSERIAL PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES app_user (id) ON DELETE RESTRICT,
  agent_record_id TEXT REFERENCES app_agent (id) ON DELETE SET NULL,
  action TEXT NOT NULL,
  resource_type TEXT NOT NULL,
  resource_id TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT app_agent_audit_action_not_empty CHECK (LENGTH(action) > 0),
  CONSTRAINT app_agent_audit_resource_type_not_empty CHECK (LENGTH(resource_type) > 0),
  CONSTRAINT app_agent_audit_resource_id_not_empty CHECK (LENGTH(resource_id) > 0)
);

CREATE INDEX IF NOT EXISTS idx_app_agent_audit_user_created
  ON app_agent_audit_log (user_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_app_agent_audit_user_agent_created
  ON app_agent_audit_log (user_id, agent_record_id, created_at DESC, id DESC);
