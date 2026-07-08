package config

import (
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/spf13/viper"
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
	WeChatComponentAppSecret string
	WeChatAPIBaseURL         string
	WeChatComponentAppID     string
	WeChatComponentToken     string
	WeChatComponentAESKey    string
	WeChatRefreshTokenKey    string
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
		WeChatComponentAppSecret: v.GetString("wechat.component_app_secret"),
		WeChatAPIBaseURL:         v.GetString("wechat.api_base_url"),
		WeChatComponentAppID:     v.GetString("wechat.component_app_id"),
		WeChatComponentToken:     v.GetString("wechat.component_verify_token"),
		WeChatComponentAESKey:    v.GetString("wechat.component_encoding_aes_key"),
		WeChatRefreshTokenKey:    v.GetString("wechat.refresh_token_encryption_key"),
	}
	if err := validator.New().Struct(cfg); err != nil {
		return Config{}, fmt.Errorf("validate config: %w", err)
	}
	return cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("app.env", "dev")
	v.SetDefault("http.addr", ":8080")
	v.SetDefault("log.level", "info")
	v.SetDefault("database.dsn", "")
	v.SetDefault("redis.addr", "")
	v.SetDefault("wechat.component_app_secret", "")
	v.SetDefault("wechat.api_base_url", "")
	v.SetDefault("wechat.component_app_id", "")
	v.SetDefault("wechat.component_verify_token", "")
	v.SetDefault("wechat.component_encoding_aes_key", "")
	v.SetDefault("wechat.refresh_token_encryption_key", "")
}
