package crypto

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"official-account-service/internal/domain/authorization"
)

func TestRefreshTokenEncryptorEncryptsRefreshToken(t *testing.T) {
	key := base64.StdEncoding.EncodeToString([]byte("abcdefghijklmnopqrstuvwxyz012345"))
	encryptor, err := NewRefreshTokenEncryptor(key)
	require.NoError(t, err)

	first, err := encryptor.EncryptAuthorizerRefreshToken(context.Background(), "refresh-token")
	require.NoError(t, err)
	second, err := encryptor.EncryptAuthorizerRefreshToken(context.Background(), "refresh-token")
	require.NoError(t, err)

	require.NotEqual(t, "refresh-token", first)
	require.NotEqual(t, first, second)
	require.NotContains(t, first, "refresh-token")

	decrypted, err := encryptor.DecryptAuthorizerRefreshToken(context.Background(), first)
	require.NoError(t, err)
	require.Equal(t, "refresh-token", decrypted)
}

func TestRefreshTokenEncryptorValidatesConfig(t *testing.T) {
	_, err := NewRefreshTokenEncryptor("")
	require.Error(t, err)
	require.True(t, errors.Is(err, authorization.ErrRefreshTokenEncryptorUnavailable))

	_, err = NewRefreshTokenEncryptor(base64.StdEncoding.EncodeToString([]byte("short")))
	require.Error(t, err)
	require.True(t, errors.Is(err, authorization.ErrRefreshTokenEncryptorUnavailable))
}

func TestRefreshTokenEncryptorDoesNotLeakPlaintextOnInputError(t *testing.T) {
	key := base64.StdEncoding.EncodeToString([]byte("abcdefghijklmnopqrstuvwxyz012345"))
	encryptor, err := NewRefreshTokenEncryptor(key)
	require.NoError(t, err)

	_, err = encryptor.EncryptAuthorizerRefreshToken(context.Background(), "")
	require.Error(t, err)
	require.False(t, strings.Contains(err.Error(), "refresh-token"))

	_, err = encryptor.DecryptAuthorizerRefreshToken(context.Background(), "not-base64")
	require.Error(t, err)
	require.False(t, strings.Contains(err.Error(), "refresh-token"))
}
