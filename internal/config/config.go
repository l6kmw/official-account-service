package config

import (
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/spf13/viper"
	"golang.org/x/crypto/bcrypt"
)

// DefaultPath is the default YAML config path used by the server.
const DefaultPath = "config.yaml"

// Config contains service configuration.
type Config struct {
	AppEnv                   string `validate:"required,oneof=dev test staging prod"`
	HTTPAddr                 string `validate:"required"`
	LogLevel                 string `validate:"required"`
	DBDSN                    string
	RedisAddr                string
	AdminAPIKey              string
	AdminUsername            string
	AdminPasswordHash        string
	AdminSessionSecret       string
	WeChatComponentAppSecret string
	WeChatAPIBaseURL         string
	WeChatComponentAppID     string
	WeChatComponentToken     string
	WeChatComponentAESKey    string
	WeChatRefreshTokenKey    string
	MCPToken                 string
	MCPPath                  string
}

// Load reads service configuration from a YAML file.
func Load(path string) (Config, error) {
	if strings.TrimSpace(path) == "" {
		path = DefaultPath
	}
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")
	setDefaults(v)
	if err := v.ReadInConfig(); err != nil {
		return Config{}, fmt.Errorf("read config file %q: %w", path, err)
	}

	cfg := Config{
		AppEnv:                   v.GetString("app.env"),
		HTTPAddr:                 v.GetString("http.addr"),
		LogLevel:                 v.GetString("log.level"),
		DBDSN:                    v.GetString("database.dsn"),
		RedisAddr:                v.GetString("redis.addr"),
		AdminAPIKey:              v.GetString("security.admin_api_key"),
		AdminUsername:            v.GetString("security.admin_username"),
		AdminPasswordHash:        v.GetString("security.admin_password_hash"),
		AdminSessionSecret:       v.GetString("security.admin_session_secret"),
		WeChatComponentAppSecret: v.GetString("wechat.component_app_secret"),
		WeChatAPIBaseURL:         v.GetString("wechat.api_base_url"),
		WeChatComponentAppID:     v.GetString("wechat.component_app_id"),
		WeChatComponentToken:     v.GetString("wechat.component_verify_token"),
		WeChatComponentAESKey:    v.GetString("wechat.component_encoding_aes_key"),
		WeChatRefreshTokenKey:    v.GetString("wechat.refresh_token_encryption_key"),
		MCPToken:                 v.GetString("mcp.token"),
		MCPPath:                  v.GetString("mcp.path"),
	}
	if err := validator.New().Struct(cfg); err != nil {
		return Config{}, fmt.Errorf("validate config: %w", err)
	}
	if err := validateAdminAuth(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func validateAdminAuth(cfg Config) error {
	adminAPIKey := strings.TrimSpace(cfg.AdminAPIKey)
	adminUsername := strings.TrimSpace(cfg.AdminUsername)
	adminPasswordHash := strings.TrimSpace(cfg.AdminPasswordHash)
	adminSessionSecret := strings.TrimSpace(cfg.AdminSessionSecret)
	adminSessionValues := 0
	for _, value := range []string{adminUsername, adminPasswordHash, adminSessionSecret} {
		if value != "" {
			adminSessionValues++
		}
	}
	if adminSessionValues > 0 && adminSessionValues < 3 {
		return fmt.Errorf("validate config: security.admin_username, security.admin_password_hash and security.admin_session_secret must be configured together")
	}
	if adminPasswordHash != "" {
		if _, err := bcrypt.Cost([]byte(adminPasswordHash)); err != nil {
			return fmt.Errorf("validate config: security.admin_password_hash must be a bcrypt hash: %w", err)
		}
	}
	if adminSessionSecret != "" && len(adminSessionSecret) < 32 {
		return fmt.Errorf("validate config: security.admin_session_secret must be at least 32 characters")
	}
	if cfg.AppEnv == "prod" && adminAPIKey == "" && adminSessionValues == 0 {
		return fmt.Errorf("validate config: security.admin_api_key or administrator login must be configured in prod")
	}
	return nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("app.env", "dev")
	v.SetDefault("http.addr", ":8080")
	v.SetDefault("log.level", "info")
	v.SetDefault("database.dsn", "")
	v.SetDefault("redis.addr", "")
	v.SetDefault("security.admin_api_key", "")
	v.SetDefault("security.admin_username", "")
	v.SetDefault("security.admin_password_hash", "")
	v.SetDefault("security.admin_session_secret", "")
	v.SetDefault("wechat.component_app_secret", "")
	v.SetDefault("wechat.api_base_url", "")
	v.SetDefault("wechat.component_app_id", "")
	v.SetDefault("wechat.component_verify_token", "")
	v.SetDefault("wechat.component_encoding_aes_key", "")
	v.SetDefault("wechat.refresh_token_encryption_key", "")
	v.SetDefault("mcp.token", "")
	v.SetDefault("mcp.path", "/mcp")
}
