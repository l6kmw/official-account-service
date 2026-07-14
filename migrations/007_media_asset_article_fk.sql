DELETE FROM wechat_media_asset AS material
WHERE NOT EXISTS (
  SELECT 1
  FROM wechat_article AS article
  WHERE article.id = material.article_id
);

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conname = 'fk_wechat_media_asset_article'
  ) THEN
    ALTER TABLE wechat_media_asset
      ADD CONSTRAINT fk_wechat_media_asset_article
      FOREIGN KEY (article_id)
      REFERENCES wechat_article (id)
      ON DELETE CASCADE;
  END IF;
END $$;
