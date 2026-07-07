package crypto

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"strings"

	"official-account-service/internal/domain/authorization"
)

// RefreshTokenEncryptor encrypts authorizer refresh tokens with AES-GCM.
type RefreshTokenEncryptor struct {
	aead cipher.AEAD
}

// NewRefreshTokenEncryptor constructs a RefreshTokenEncryptor from a base64 32-byte key.
func NewRefreshTokenEncryptor(encodedKey string) (*RefreshTokenEncryptor, error) {
	if strings.TrimSpace(encodedKey) == "" {
		return nil, fmt.Errorf("validate refresh token encryption key: %w", authorization.ErrRefreshTokenEncryptorUnavailable)
	}
	key, err := base64.StdEncoding.DecodeString(encodedKey)
	if err != nil {
		return nil, fmt.Errorf("decode refresh token encryption key: %w", authorization.ErrRefreshTokenEncryptorUnavailable)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("validate refresh token encryption key length: %w", authorization.ErrRefreshTokenEncryptorUnavailable)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("init refresh token cipher: %w", authorization.ErrRefreshTokenEncryptorUnavailable)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("init refresh token gcm: %w", authorization.ErrRefreshTokenEncryptorUnavailable)
	}
	return &RefreshTokenEncryptor{aead: aead}, nil
}

// EncryptAuthorizerRefreshToken encrypts an authorizer refresh token.
func (e *RefreshTokenEncryptor) EncryptAuthorizerRefreshToken(_ context.Context, plaintext string) (string, error) {
	if e == nil {
		return "", fmt.Errorf("validate refresh token encryptor: %w", authorization.ErrRefreshTokenEncryptorUnavailable)
	}
	if strings.TrimSpace(plaintext) == "" {
		return "", fmt.Errorf("validate authorizer refresh token: %w", authorization.ErrRefreshTokenEncryptorUnavailable)
	}
	nonce := make([]byte, e.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate refresh token nonce: %w", err)
	}
	ciphertext := e.aead.Seal(nil, nonce, []byte(plaintext), nil)
	sealed := make([]byte, 0, len(nonce)+len(ciphertext))
	sealed = append(sealed, nonce...)
	sealed = append(sealed, ciphertext...)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// DecryptAuthorizerRefreshToken decrypts an authorizer refresh token.
func (e *RefreshTokenEncryptor) DecryptAuthorizerRefreshToken(_ context.Context, ciphertext string) (string, error) {
	if e == nil {
		return "", fmt.Errorf("validate refresh token encryptor: %w", authorization.ErrRefreshTokenEncryptorUnavailable)
	}
	if strings.TrimSpace(ciphertext) == "" {
		return "", fmt.Errorf("validate encrypted authorizer refresh token: %w", authorization.ErrRefreshTokenEncryptorUnavailable)
	}
	sealed, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("decode encrypted authorizer refresh token: %w", authorization.ErrRefreshTokenEncryptorUnavailable)
	}
	nonceSize := e.aead.NonceSize()
	if len(sealed) <= nonceSize {
		return "", fmt.Errorf("validate encrypted authorizer refresh token length: %w", authorization.ErrRefreshTokenEncryptorUnavailable)
	}
	plaintext, err := e.aead.Open(nil, sealed[:nonceSize], sealed[nonceSize:], nil)
	if err != nil {
		return "", fmt.Errorf("decrypt authorizer refresh token: %w", authorization.ErrRefreshTokenEncryptorUnavailable)
	}
	return string(plaintext), nil
}
