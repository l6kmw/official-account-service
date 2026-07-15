CREATE TABLE IF NOT EXISTS app_agent (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES app_user(id) ON DELETE RESTRICT,
  agent_id TEXT NOT NULL,
  name TEXT NOT NULL,
  purpose TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
  api_token_hash TEXT,
  api_token_hint TEXT NOT NULL DEFAULT '',
  api_token_created_at TIMESTAMPTZ,
  last_used_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT app_agent_id_not_empty CHECK (LENGTH(id) > 0),
  CONSTRAINT app_agent_external_id_length CHECK (LENGTH(agent_id) BETWEEN 1 AND 128),
  CONSTRAINT app_agent_name_length CHECK (LENGTH(name) BETWEEN 1 AND 64),
  CONSTRAINT app_agent_purpose_length CHECK (LENGTH(purpose) <= 200),
  CONSTRAINT app_agent_api_token_hash_length CHECK (api_token_hash IS NULL OR LENGTH(api_token_hash) = 64),
  CONSTRAINT app_agent_api_token_metadata CHECK (
    (api_token_hash IS NULL AND api_token_created_at IS NULL)
    OR (api_token_hash IS NOT NULL AND api_token_created_at IS NOT NULL)
  )
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_app_agent_user_agent_id
  ON app_agent (user_id, agent_id);

CREATE UNIQUE INDEX IF NOT EXISTS idx_app_agent_api_token_hash
  ON app_agent (api_token_hash)
  WHERE api_token_hash IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_app_agent_user_id
  ON app_agent (user_id);

INSERT INTO app_agent (
  id, user_id, agent_id, name, purpose, status,
  api_token_hash, api_token_hint, api_token_created_at,
  created_at, updated_at
)
SELECT
  'agt_' || SUBSTRING(MD5(id || ':default'), 1, 32),
  id,
  'default',
  'Default Agent',
  'Migrated user-level MCP token',
  'active',
  api_token_hash,
  api_token_hint,
  COALESCE(api_token_created_at, updated_at, NOW()),
  COALESCE(api_token_created_at, created_at),
  updated_at
FROM app_user
WHERE api_token_hash IS NOT NULL
ON CONFLICT DO NOTHING;
