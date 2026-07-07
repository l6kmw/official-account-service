ALTER TABLE wechat_authorization_account
  ADD COLUMN IF NOT EXISTS encrypted_authorizer_refresh_token TEXT NOT NULL DEFAULT '';
