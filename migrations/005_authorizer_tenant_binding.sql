CREATE TABLE IF NOT EXISTS wechat_authorizer_tenant_binding (
  component_app_id TEXT NOT NULL,
  authorizer_app_id TEXT NOT NULL,
  tenant_id TEXT NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (component_app_id, authorizer_app_id)
);

CREATE INDEX IF NOT EXISTS idx_wechat_authorizer_tenant_binding_tenant_id
  ON wechat_authorizer_tenant_binding (tenant_id);
