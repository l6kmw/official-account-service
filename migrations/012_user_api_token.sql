ALTER TABLE app_user
  ADD COLUMN IF NOT EXISTS api_token_hash TEXT,
  ADD COLUMN IF NOT EXISTS api_token_hint TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS api_token_created_at TIMESTAMPTZ;

CREATE UNIQUE INDEX IF NOT EXISTS idx_app_user_api_token_hash
  ON app_user (api_token_hash)
  WHERE api_token_hash IS NOT NULL;

ALTER TABLE app_user
  DROP CONSTRAINT IF EXISTS app_user_api_token_hash_length;

ALTER TABLE app_user
  ADD CONSTRAINT app_user_api_token_hash_length
  CHECK (api_token_hash IS NULL OR LENGTH(api_token_hash) = 64);
