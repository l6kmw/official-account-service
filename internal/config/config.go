package config

import (
	"fmt"

	"github.com/go-playground/validator/v10"
	"github.com/spf13/viper"
)

// Config contains service configuration.
type Config struct {
	AppEnv                   string `mapstructure:"app_env" validate:"required,oneof=dev test staging prod"`
	HTTPAddr                 string `mapstructure:"http_addr" validate:"required"`
	LogLevel                 string `mapstructure:"log_level" validate:"required"`
	DBDSN                    string `mapstructure:"db_dsn"`
	RedisAddr                string `mapstructure:"redis_addr"`
	WeChatComponentAppSecret string `mapstructure:"wechat_component_app_secret"`
	WeChatAPIBaseURL         string `mapstructure:"wechat_api_base_url"`
	WeChatComponentAppID     string `mapstructure:"wechat_component_app_id"`
	WeChatComponentToken     string `mapstructure:"wechat_component_verify_token"`
	WeChatComponentAESKey    string `mapstructure:"wechat_component_encoding_aes_key"`
	WeChatRefreshTokenKey    string `mapstructure:"wechat_refresh_token_encryption_key"`
}

// Load reads configuration from environment variables and .env when present.
func Load() (Config, error) {
	v := viper.New()
	v.SetConfigFile(".env")
	v.SetConfigType("env")
	v.AutomaticEnv()
	v.SetDefault("APP_ENV", "dev")
	v.SetDefault("HTTP_ADDR", ":8080")
	v.SetDefault("LOG_LEVEL", "info")
	v.SetDefault("DB_DSN", "")
	v.SetDefault("REDIS_ADDR", "")
	v.SetDefault("WECHAT_COMPONENT_APP_SECRET", "")
	v.SetDefault("WECHAT_API_BASE_URL", "")
	v.SetDefault("WECHAT_COMPONENT_APP_ID", "")
	v.SetDefault("WECHAT_COMPONENT_VERIFY_TOKEN", "")
	v.SetDefault("WECHAT_COMPONENT_ENCODING_AES_KEY", "")
	v.SetDefault("WECHAT_REFRESH_TOKEN_ENCRYPTION_KEY", "")
	_ = v.ReadInConfig()

	cfg := Config{
		AppEnv:                   v.GetString("APP_ENV"),
		HTTPAddr:                 v.GetString("HTTP_ADDR"),
		LogLevel:                 v.GetString("LOG_LEVEL"),
		DBDSN:                    v.GetString("DB_DSN"),
		RedisAddr:                v.GetString("REDIS_ADDR"),
		WeChatComponentAppSecret: v.GetString("WECHAT_COMPONENT_APP_SECRET"),
		WeChatAPIBaseURL:         v.GetString("WECHAT_API_BASE_URL"),
		WeChatComponentAppID:     v.GetString("WECHAT_COMPONENT_APP_ID"),
		WeChatComponentToken:     v.GetString("WECHAT_COMPONENT_VERIFY_TOKEN"),
		WeChatComponentAESKey:    v.GetString("WECHAT_COMPONENT_ENCODING_AES_KEY"),
		WeChatRefreshTokenKey:    v.GetString("WECHAT_REFRESH_TOKEN_ENCRYPTION_KEY"),
	}
	if err := validator.New().Struct(cfg); err != nil {
		return Config{}, fmt.Errorf("validate config: %w", err)
	}
	return cfg, nil
}
