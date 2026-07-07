package authorization

import (
	"context"
	"errors"
)

// ErrComponentCallbackCryptoUnavailable indicates encrypted callback handling is not configured.
var ErrComponentCallbackCryptoUnavailable = errors.New("component callback crypto unavailable")

// ErrComponentCallbackSignatureInvalid indicates the callback signature or payload is invalid.
var ErrComponentCallbackSignatureInvalid = errors.New("component callback signature invalid")

// ComponentCallbackDecryptInput contains encrypted callback fields from WeChat.
type ComponentCallbackDecryptInput struct {
	Signature  string
	Timestamp  string
	Nonce      string
	Ciphertext string
}

// ComponentCallbackDecryptor decrypts WeChat encrypted component callbacks.
type ComponentCallbackDecryptor interface {
	DecryptComponentCallback(ctx context.Context, input ComponentCallbackDecryptInput) ([]byte, error)
}
