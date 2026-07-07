package queue

import "go.uber.org/zap"

// ZapLogger adapts zap to asynq's logger interface.
type ZapLogger struct {
	logger *zap.SugaredLogger
}

// NewZapLogger constructs a ZapLogger.
func NewZapLogger(logger *zap.Logger) ZapLogger {
	if logger == nil {
		logger = zap.NewNop()
	}
	return ZapLogger{logger: logger.Sugar()}
}

// Debug logs a debug message.
func (l ZapLogger) Debug(args ...interface{}) { l.logger.Debug(args...) }

// Info logs an info message.
func (l ZapLogger) Info(args ...interface{}) { l.logger.Info(args...) }

// Warn logs a warning message.
func (l ZapLogger) Warn(args ...interface{}) { l.logger.Warn(args...) }

// Error logs an error message.
func (l ZapLogger) Error(args ...interface{}) { l.logger.Error(args...) }

// Fatal logs a fatal message.
func (l ZapLogger) Fatal(args ...interface{}) { l.logger.Fatal(args...) }
