package wechat

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"official-account-service/internal/domain/authorization"
)

func TestComponentCallbackCryptoDecryptsCallback(t *testing.T) {
	key := testEncodingAESKey()
	plaintext := []byte(`<xml><AppId>wx-component</AppId><CreateTime>1783334400</CreateTime><InfoType>component_verify_ticket</InfoType><ComponentVerifyTicket>ticket-1</ComponentVerifyTicket></xml>`)
	ciphertext := encryptComponentCallbackForTest(t, key, "wx-component", plaintext)
	signature := componentCallbackSignature("token-value", "1783334400", "nonce-value", ciphertext)
	crypto, err := NewComponentCallbackCrypto(ComponentCallbackCryptoConfig{
		Token:          "token-value",
		EncodingAESKey: key,
		ComponentAppID: "wx-component",
	})
	require.NoError(t, err)

	decrypted, err := crypto.DecryptComponentCallback(context.Background(), authorization.ComponentCallbackDecryptInput{
		Signature:  signature,
		Timestamp:  "1783334400",
		Nonce:      "nonce-value",
		Ciphertext: ciphertext,
	})
	require.NoError(t, err)
	require.Equal(t, plaintext, decrypted)
}

func TestComponentCallbackCryptoRejectsInvalidSignature(t *testing.T) {
	key := testEncodingAESKey()
	ciphertext := encryptComponentCallbackForTest(t, key, "wx-component", []byte(`<xml></xml>`))
	crypto, err := NewComponentCallbackCrypto(ComponentCallbackCryptoConfig{
		Token:          "token-value",
		EncodingAESKey: key,
		ComponentAppID: "wx-component",
	})
	require.NoError(t, err)

	_, err = crypto.DecryptComponentCallback(context.Background(), authorization.ComponentCallbackDecryptInput{
		Signature:  "bad-signature",
		Timestamp:  "1783334400",
		Nonce:      "nonce-value",
		Ciphertext: ciphertext,
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, authorization.ErrComponentCallbackSignatureInvalid))
}

func TestComponentCallbackCryptoAcceptsWeChatPKCS7PaddingAboveAESBlockSize(t *testing.T) {
	key := testEncodingAESKey()
	plaintext := []byte(`<xml><AppId>wx-component</AppId><CreateTime>1783334400</CreateTime><InfoType>component_verify_ticket</InfoType><ComponentVerifyTicket>ticket-with-padding-length-over-sixteen</ComponentVerifyTicket></xml>`)
	ciphertext := encryptComponentCallbackForTest(t, key, "wx-component", plaintext)
	signature := componentCallbackSignature("token-value", "1783334400", "nonce-value", ciphertext)
	crypto, err := NewComponentCallbackCrypto(ComponentCallbackCryptoConfig{
		Token:          "token-value",
		EncodingAESKey: key,
		ComponentAppID: "wx-component",
	})
	require.NoError(t, err)

	decrypted, err := crypto.DecryptComponentCallback(context.Background(), authorization.ComponentCallbackDecryptInput{
		Signature:  signature,
		Timestamp:  "1783334400",
		Nonce:      "nonce-value",
		Ciphertext: ciphertext,
	})
	require.NoError(t, err)
	require.Equal(t, plaintext, decrypted)
}

func TestComponentCallbackCryptoRejectsReceiveIDMismatch(t *testing.T) {
	key := testEncodingAESKey()
	ciphertext := encryptComponentCallbackForTest(t, key, "wx-other", []byte(`<xml></xml>`))
	crypto, err := NewComponentCallbackCrypto(ComponentCallbackCryptoConfig{
		Token:          "token-value",
		EncodingAESKey: key,
		ComponentAppID: "wx-component",
	})
	require.NoError(t, err)

	_, err = crypto.DecryptComponentCallback(context.Background(), authorization.ComponentCallbackDecryptInput{
		Signature:  componentCallbackSignature("token-value", "1783334400", "nonce-value", ciphertext),
		Timestamp:  "1783334400",
		Nonce:      "nonce-value",
		Ciphertext: ciphertext,
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, authorization.ErrComponentCallbackSignatureInvalid))
}

func testEncodingAESKey() string {
	keyBytes := []byte("abcdefghijklmnopqrstuvwxyz012345")
	return base64.StdEncoding.EncodeToString(keyBytes)[:encodingAESKeyLength]
}

func encryptComponentCallbackForTest(t *testing.T, encodingAESKey string, receiveID string, message []byte) string {
	t.Helper()
	aesKey, err := base64.StdEncoding.DecodeString(encodingAESKey + "=")
	require.NoError(t, err)
	require.Len(t, aesKey, 32)
	var length [4]byte
	binary.BigEndian.PutUint32(length[:], uint32(len(message)))
	plaintext := bytes.Join([][]byte{
		[]byte("1234567890123456"),
		length[:],
		message,
		[]byte(receiveID),
	}, nil)
	block, err := aes.NewCipher(aesKey)
	require.NoError(t, err)
	padded := pkcs7Pad(plaintext, wechatPKCS7PaddingBlocks)
	ciphertext := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, aesKey[:aes.BlockSize]).CryptBlocks(ciphertext, padded)
	return base64.StdEncoding.EncodeToString(ciphertext)
}
