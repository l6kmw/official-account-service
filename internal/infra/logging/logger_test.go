package logging

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"
)

func TestRedactedString(t *testing.T) {
	field := RedactedString("access_token")
	require.Equal(t, "access_token", field.Key)
	require.Equal(t, zapcore.StringType, field.Type)
	require.Equal(t, "[REDACTED]", field.String)
}
