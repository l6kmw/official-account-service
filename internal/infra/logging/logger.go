package logging

import (
	"fmt"

	"go.uber.org/zap"
)

// Error creates a zap error field.
func Error(err error) zap.Field { return zap.Error(err) }

// String creates a zap string field.
func String(key string, value string) zap.Field { return zap.String(key, value) }

// RedactedString creates a zap string field with sensitive content removed.
func RedactedString(key string) zap.Field { return zap.String(key, "[REDACTED]") }

// New constructs a production zap logger.
func New(level string) (*zap.Logger, error) {
	cfg := zap.NewProductionConfig()
	if err := cfg.Level.UnmarshalText([]byte(level)); err != nil {
		return nil, fmt.Errorf("parse log level: %w", err)
	}
	logger, err := cfg.Build()
	if err != nil {
		return nil, fmt.Errorf("build zap logger: %w", err)
	}
	return logger, nil
}
