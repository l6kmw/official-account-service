package application

import (
	"context"
	"encoding/xml"
	"fmt"
	"strings"
	"time"

	"official-account-service/internal/domain/authorization"
)

// HandleComponentCallback handles a WeChat third-party platform callback.
func (s *AuthorizationService) HandleComponentCallback(ctx context.Context, input HandleComponentCallbackInput) error {
	if err := s.validateReady(); err != nil {
		return err
	}
	if len(input.RawBody) == 0 {
		return fmt.Errorf("validate component callback body: %w", ErrInvalidInput)
	}
	plaintext, err := s.componentCallbackPlaintext(ctx, input)
	if err != nil {
		return err
	}
	var body componentCallbackPayload
	if err := xml.Unmarshal(plaintext, &body); err != nil {
		return fmt.Errorf("decode component callback xml: %w", ErrInvalidInput)
	}
	switch body.InfoType {
	case "component_verify_ticket":
		return s.handleComponentVerifyTicketCallback(ctx, body)
	case "unauthorized":
		return s.handleUnauthorizedCallback(ctx, body)
	default:
		return fmt.Errorf("validate component callback info type: %w", ErrInvalidInput)
	}
}

func (s *AuthorizationService) handleComponentVerifyTicketCallback(ctx context.Context, body componentCallbackPayload) error {
	if body.CreateTime <= 0 {
		return fmt.Errorf("validate component callback create time: %w", ErrInvalidInput)
	}
	_, err := s.SaveComponentVerifyTicket(ctx, SaveComponentVerifyTicketInput{
		ComponentAppID: body.AppID,
		Ticket:         body.ComponentVerifyTicket,
		ReceivedAt:     time.Unix(body.CreateTime, 0).UTC(),
	})
	if err != nil {
		return fmt.Errorf("save component verify ticket from callback: %w", err)
	}
	return nil
}

func (s *AuthorizationService) handleUnauthorizedCallback(ctx context.Context, body componentCallbackPayload) error {
	if err := s.validateUnauthorizedCallbackReady(); err != nil {
		return err
	}
	if body.CreateTime <= 0 {
		return fmt.Errorf("validate unauthorized callback create time: %w", ErrInvalidInput)
	}
	if strings.TrimSpace(body.AppID) == "" {
		return fmt.Errorf("validate unauthorized callback component app id: %w", ErrInvalidInput)
	}
	if strings.TrimSpace(body.AuthorizerAppID) == "" {
		return fmt.Errorf("validate unauthorized callback authorizer app id: %w", ErrInvalidInput)
	}
	binding, err := s.bindings.GetAuthorizerTenantBinding(ctx, body.AppID, body.AuthorizerAppID)
	if err != nil {
		return wrapAuthorizerTenantBindingError("get authorizer tenant binding", err)
	}
	_, err = s.accounts.RevokeAccountByAppID(ctx, binding.TenantID, body.AuthorizerAppID)
	if err != nil {
		return wrapAccountWriteError("revoke unauthorized account", err)
	}
	return nil
}

func (s *AuthorizationService) componentCallbackPlaintext(ctx context.Context, input HandleComponentCallbackInput) ([]byte, error) {
	encryptType := strings.TrimSpace(input.EncryptType)
	if encryptType == "" && strings.TrimSpace(input.MsgSignature) == "" {
		return input.RawBody, nil
	}
	if encryptType != "" && encryptType != "aes" {
		return nil, fmt.Errorf("validate component callback encrypt type: %w", ErrInvalidInput)
	}
	if strings.TrimSpace(input.MsgSignature) == "" || strings.TrimSpace(input.Timestamp) == "" || strings.TrimSpace(input.Nonce) == "" {
		return nil, fmt.Errorf("validate encrypted component callback query: %w", ErrInvalidInput)
	}
	if s.callbackDecryptor == nil {
		return nil, fmt.Errorf("validate component callback decryptor: %w", ErrNotImplemented)
	}
	var envelope encryptedComponentCallbackEnvelope
	if err := xml.Unmarshal(input.RawBody, &envelope); err != nil {
		return nil, fmt.Errorf("decode encrypted component callback xml: %w", ErrInvalidInput)
	}
	if strings.TrimSpace(envelope.Encrypt) == "" {
		return nil, fmt.Errorf("validate encrypted component callback payload: %w", ErrInvalidInput)
	}
	plaintext, err := s.callbackDecryptor.DecryptComponentCallback(ctx, authorization.ComponentCallbackDecryptInput{
		Signature:  input.MsgSignature,
		Timestamp:  input.Timestamp,
		Nonce:      input.Nonce,
		Ciphertext: envelope.Encrypt,
	})
	if err != nil {
		return nil, wrapComponentCallbackDecryptError("decrypt component callback", err)
	}
	return plaintext, nil
}

type componentCallbackPayload struct {
	AppID                 string `xml:"AppId"`
	AuthorizerAppID       string `xml:"AuthorizerAppid"`
	CreateTime            int64  `xml:"CreateTime"`
	InfoType              string `xml:"InfoType"`
	ComponentVerifyTicket string `xml:"ComponentVerifyTicket"`
}

type encryptedComponentCallbackEnvelope struct {
	Encrypt string `xml:"Encrypt"`
}
