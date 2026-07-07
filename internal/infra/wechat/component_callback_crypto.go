package wechat

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"official-account-service/internal/domain/authorization"
)

const (
	encodingAESKeyLength     = 43
	wechatPKCS7PaddingBlocks = 32
)

// ComponentCallbackCryptoConfig contains WeChat component callback crypto settings.
type ComponentCallbackCryptoConfig struct {
	Token          string
	EncodingAESKey string
	ComponentAppID string
}

// ComponentCallbackCrypto decrypts WeChat encrypted component callbacks.
type ComponentCallbackCrypto struct {
	token          string
	aesKey         []byte
	componentAppID string
}

// NewComponentCallbackCrypto constructs a ComponentCallbackCrypto.
func NewComponentCallbackCrypto(cfg ComponentCallbackCryptoConfig) (*ComponentCallbackCrypto, error) {
	if strings.TrimSpace(cfg.Token) == "" {
		return nil, fmt.Errorf("validate component callback token: %w", authorization.ErrComponentCallbackCryptoUnavailable)
	}
	if len(strings.TrimSpace(cfg.EncodingAESKey)) != encodingAESKeyLength {
		return nil, fmt.Errorf("validate component callback aes key: %w", authorization.ErrComponentCallbackCryptoUnavailable)
	}
	aesKey, err := base64.StdEncoding.DecodeString(strings.TrimSpace(cfg.EncodingAESKey) + "=")
	if err != nil {
		return nil, fmt.Errorf("decode component callback aes key: %w", authorization.ErrComponentCallbackCryptoUnavailable)
	}
	if len(aesKey) != 32 {
		return nil, fmt.Errorf("validate component callback aes key length: %w", authorization.ErrComponentCallbackCryptoUnavailable)
	}
	return &ComponentCallbackCrypto{
		token:          cfg.Token,
		aesKey:         aesKey,
		componentAppID: cfg.ComponentAppID,
	}, nil
}

// DecryptComponentCallback verifies and decrypts a WeChat encrypted component callback.
func (c *ComponentCallbackCrypto) DecryptComponentCallback(_ context.Context, input authorization.ComponentCallbackDecryptInput) ([]byte, error) {
	if c == nil {
		return nil, fmt.Errorf("validate component callback crypto: %w", authorization.ErrComponentCallbackCryptoUnavailable)
	}
	if strings.TrimSpace(input.Signature) == "" || strings.TrimSpace(input.Timestamp) == "" || strings.TrimSpace(input.Nonce) == "" || strings.TrimSpace(input.Ciphertext) == "" {
		return nil, fmt.Errorf("validate encrypted component callback input: %w", authorization.ErrComponentCallbackSignatureInvalid)
	}
	if !secureEqual(input.Signature, componentCallbackSignature(c.token, input.Timestamp, input.Nonce, input.Ciphertext)) {
		return nil, fmt.Errorf("validate component callback signature: %w", authorization.ErrComponentCallbackSignatureInvalid)
	}
	ciphertext, err := base64.StdEncoding.DecodeString(input.Ciphertext)
	if err != nil {
		return nil, fmt.Errorf("decode component callback ciphertext: %w", authorization.ErrComponentCallbackSignatureInvalid)
	}
	if len(ciphertext) == 0 || len(ciphertext)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("validate component callback ciphertext length: %w", authorization.ErrComponentCallbackSignatureInvalid)
	}
	block, err := aes.NewCipher(c.aesKey)
	if err != nil {
		return nil, fmt.Errorf("init component callback cipher: %w", authorization.ErrComponentCallbackCryptoUnavailable)
	}
	plaintext := make([]byte, len(ciphertext))
	cipher.NewCBCDecrypter(block, c.aesKey[:aes.BlockSize]).CryptBlocks(plaintext, ciphertext)
	unpadded, err := pkcs7Unpad(plaintext, wechatPKCS7PaddingBlocks)
	if err != nil {
		return nil, fmt.Errorf("unpad component callback plaintext: %w", authorization.ErrComponentCallbackSignatureInvalid)
	}
	message, receiveID, err := splitComponentCallbackPlaintext(unpadded)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(c.componentAppID) != "" && receiveID != c.componentAppID {
		return nil, fmt.Errorf("validate component callback receive id: %w", authorization.ErrComponentCallbackSignatureInvalid)
	}
	return message, nil
}

func splitComponentCallbackPlaintext(plaintext []byte) ([]byte, string, error) {
	if len(plaintext) < 20 {
		return nil, "", fmt.Errorf("validate component callback plaintext length: %w", authorization.ErrComponentCallbackSignatureInvalid)
	}
	messageLength := int(binary.BigEndian.Uint32(plaintext[16:20]))
	messageStart := 20
	messageEnd := messageStart + messageLength
	if messageLength <= 0 || messageEnd > len(plaintext) {
		return nil, "", fmt.Errorf("validate component callback message length: %w", authorization.ErrComponentCallbackSignatureInvalid)
	}
	message := plaintext[messageStart:messageEnd]
	receiveID := string(plaintext[messageEnd:])
	if strings.TrimSpace(receiveID) == "" {
		return nil, "", fmt.Errorf("validate component callback receive id: %w", authorization.ErrComponentCallbackSignatureInvalid)
	}
	return message, receiveID, nil
}

func componentCallbackSignature(token string, timestamp string, nonce string, ciphertext string) string {
	parts := []string{token, timestamp, nonce, ciphertext}
	sort.Strings(parts)
	hash := sha1.Sum([]byte(strings.Join(parts, "")))
	return hex.EncodeToString(hash[:])
}

func pkcs7Unpad(data []byte, blockSize int) ([]byte, error) {
	if len(data) == 0 || len(data)%blockSize != 0 {
		return nil, fmt.Errorf("validate pkcs7 data length: %w", authorization.ErrComponentCallbackSignatureInvalid)
	}
	padding := int(data[len(data)-1])
	if padding == 0 || padding > blockSize || padding > len(data) {
		return nil, fmt.Errorf("validate pkcs7 padding length: %w", authorization.ErrComponentCallbackSignatureInvalid)
	}
	if !bytes.Equal(data[len(data)-padding:], bytes.Repeat([]byte{byte(padding)}, padding)) {
		return nil, fmt.Errorf("validate pkcs7 padding bytes: %w", authorization.ErrComponentCallbackSignatureInvalid)
	}
	return data[:len(data)-padding], nil
}

func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	return append(data, bytes.Repeat([]byte{byte(padding)}, padding)...)
}

func secureEqual(a string, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var result byte
	for i := 0; i < len(a); i++ {
		result |= a[i] ^ b[i]
	}
	return result == 0
}
