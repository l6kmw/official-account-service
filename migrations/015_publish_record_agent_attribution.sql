ALTER TABLE wechat_publish_record
  ADD COLUMN IF NOT EXISTS article_created_by_agent_id TEXT;

UPDATE wechat_publish_record AS record
SET article_created_by_agent_id = article.created_by_agent_id
FROM wechat_article AS article
WHERE record.tenant_id = article.tenant_id
  AND record.article_id = article.id
  AND record.article_created_by_agent_id IS NULL
  AND article.created_by_agent_id IS NOT NULL;

WITH article_creators AS (
  SELECT DISTINCT ON (audit.user_id, audit.resource_id)
    audit.user_id,
    audit.resource_id,
    audit.agent_record_id
  FROM app_agent_audit_log AS audit
  JOIN app_agent AS agent
    ON agent.id = audit.agent_record_id
   AND agent.user_id = audit.user_id
  WHERE audit.action = 'create_article'
    AND audit.resource_type = 'article'
    AND audit.agent_record_id IS NOT NULL
  ORDER BY audit.user_id, audit.resource_id, audit.id
)
UPDATE wechat_publish_record AS record
SET article_created_by_agent_id = creator.agent_record_id
FROM article_creators AS creator
WHERE record.tenant_id = creator.user_id
  AND record.article_id::TEXT = creator.resource_id
  AND record.article_created_by_agent_id IS NULL;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_wechat_publish_record_created_by_agent') THEN
    ALTER TABLE wechat_publish_record
      ADD CONSTRAINT fk_wechat_publish_record_created_by_agent
      FOREIGN KEY (article_created_by_agent_id) REFERENCES app_agent (id) ON DELETE SET NULL;
  END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_wechat_publish_record_tenant_created_agent
  ON wechat_publish_record (tenant_id, article_created_by_agent_id);
