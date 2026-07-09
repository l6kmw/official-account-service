package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadReadsYAMLConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	err := os.WriteFile(path, []byte(`app:
  env: staging
http:
  addr: ":9090"
log:
  level: debug
database:
  dsn: "postgres://user:pass@localhost:5432/db?sslmode=disable"
redis:
  addr: "localhost:6379"
security:
  admin_api_key: "admin-key"
  admin_username: "admin"
  admin_password_hash: "$2y$04$oHbnp5MZebtRqoPbLed2eeWLCwk4yDqvN3daGy/3F.1aF1EZSNC5y"
  admin_session_secret: "session-secret-with-at-least-32-chars"
wechat:
  component_app_id: "component-app-id"
  component_app_secret: "component-secret"
  api_base_url: "https://example.com/cgi-bin"
  component_verify_token: "verify-token"
  component_encoding_aes_key: "encoding-key"
  refresh_token_encryption_key: "refresh-key"
`), 0o600)
	require.NoError(t, err)

	cfg, err := Load(path)
	require.NoError(t, err)
	require.Equal(t, "staging", cfg.AppEnv)
	require.Equal(t, ":9090", cfg.HTTPAddr)
	require.Equal(t, "debug", cfg.LogLevel)
	require.Equal(t, "postgres://user:pass@localhost:5432/db?sslmode=disable", cfg.DBDSN)
	require.Equal(t, "localhost:6379", cfg.RedisAddr)
	require.Equal(t, "admin-key", cfg.AdminAPIKey)
	require.Equal(t, "admin", cfg.AdminUsername)
	require.Equal(t, "$2y$04$oHbnp5MZebtRqoPbLed2eeWLCwk4yDqvN3daGy/3F.1aF1EZSNC5y", cfg.AdminPasswordHash)
	require.Equal(t, "session-secret-with-at-least-32-chars", cfg.AdminSessionSecret)
	require.Equal(t, "component-app-id", cfg.WeChatComponentAppID)
	require.Equal(t, "component-secret", cfg.WeChatComponentAppSecret)
	require.Equal(t, "https://example.com/cgi-bin", cfg.WeChatAPIBaseURL)
	require.Equal(t, "verify-token", cfg.WeChatComponentToken)
	require.Equal(t, "encoding-key", cfg.WeChatComponentAESKey)
	require.Equal(t, "refresh-key", cfg.WeChatRefreshTokenKey)
}

func TestLoadAppliesYAMLDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	err := os.WriteFile(path, []byte("{}\n"), 0o600)
	require.NoError(t, err)

	cfg, err := Load(path)
	require.NoError(t, err)
	require.Equal(t, "dev", cfg.AppEnv)
	require.Equal(t, ":8080", cfg.HTTPAddr)
	require.Equal(t, "info", cfg.LogLevel)
	require.Empty(t, cfg.DBDSN)
	require.Empty(t, cfg.RedisAddr)
	require.Empty(t, cfg.AdminAPIKey)
	require.Empty(t, cfg.AdminUsername)
	require.Empty(t, cfg.AdminPasswordHash)
	require.Empty(t, cfg.AdminSessionSecret)
}

func TestLoadRequiresAdminAPIKeyInProd(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	err := os.WriteFile(path, []byte(`app:
  env: prod
`), 0o600)
	require.NoError(t, err)

	_, err = Load(path)
	require.Error(t, err)
	require.ErrorContains(t, err, "security.admin_api_key or administrator login")
}

func TestLoadAcceptsAdminLoginInProd(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	err := os.WriteFile(path, []byte(`app:
  env: prod
security:
  admin_username: "admin"
  admin_password_hash: "$2y$04$oHbnp5MZebtRqoPbLed2eeWLCwk4yDqvN3daGy/3F.1aF1EZSNC5y"
  admin_session_secret: "session-secret-with-at-least-32-chars"
`), 0o600)
	require.NoError(t, err)

	cfg, err := Load(path)
	require.NoError(t, err)
	require.Equal(t, "admin", cfg.AdminUsername)
}

func TestLoadRejectsPartialAdminLoginConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	err := os.WriteFile(path, []byte(`security:
  admin_username: "admin"
`), 0o600)
	require.NoError(t, err)

	_, err = Load(path)
	require.Error(t, err)
	require.ErrorContains(t, err, "must be configured together")
}

func TestLoadRejectsInvalidAdminLoginSecurityValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	err := os.WriteFile(path, []byte(`security:
  admin_username: "admin"
  admin_password_hash: "not-bcrypt"
  admin_session_secret: "short"
`), 0o600)
	require.NoError(t, err)

	_, err = Load(path)
	require.Error(t, err)
	require.ErrorContains(t, err, "security.admin_password_hash")
}

func TestLoadReturnsErrorForMissingConfig(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "missing.yaml"))
	require.Error(t, err)
	require.ErrorContains(t, err, "read config file")
}
