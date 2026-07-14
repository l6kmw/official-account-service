CREATE UNIQUE INDEX IF NOT EXISTS idx_wechat_publish_record_one_publishing
  ON wechat_publish_record (tenant_id, article_id)
  WHERE status = 'publishing';
