package application

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"official-account-service/internal/domain/authorization"
	"official-account-service/internal/domain/publish"
	"official-account-service/internal/domain/wechatcallback"
)

const defaultCallbackRetention = 30 * 24 * time.Hour

// CallbackService handles WeChat authorizer callbacks.
type CallbackService struct {
	events         wechatcallback.Repository
	bindings       authorization.AuthorizerTenantBindingRepository
	publishes      *PublishService
	decryptor      authorization.ComponentCallbackDecryptor
	componentAppID string
	now            func() time.Time
	retention      time.Duration
}

// NewCallbackService constructs a CallbackService.
func NewCallbackService(events wechatcallback.Repository, bindings authorization.AuthorizerTenantBindingRepository, publishes *PublishService, decryptor authorization.ComponentCallbackDecryptor, componentAppID string, now func() time.Time) *CallbackService {
	if now == nil {
		now = time.Now
	}
	return &CallbackService{
		events: events, bindings: bindings, publishes: publishes, decryptor: decryptor, componentAppID: componentAppID,
		now: now, retention: defaultCallbackRetention,
	}
}

// HandleAuthorizerCallbackInput contains one WeChat authorizer callback request.
type HandleAuthorizerCallbackInput struct {
	AuthorizerAppID string
	RawBody         []byte
	EncryptType     string
	MsgSignature    string
	Timestamp       string
	Nonce           string
}

// HandleAuthorizerCallback handles authorizer events such as publish results.
func (s *CallbackService) HandleAuthorizerCallback(ctx context.Context, input HandleAuthorizerCallbackInput) error {
	if err := s.validateReady(); err != nil {
		return err
	}
	if strings.TrimSpace(input.AuthorizerAppID) == "" {
		return fmt.Errorf("validate authorizer callback app id: %w", ErrInvalidInput)
	}
	if len(input.RawBody) == 0 {
		return fmt.Errorf("validate authorizer callback body: %w", ErrInvalidInput)
	}
	plaintext, err := s.callbackPlaintext(ctx, input)
	if err != nil {
		return err
	}
	event, ok, err := decodePublishResultCallback(plaintext)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	binding, err := s.bindings.GetAuthorizerTenantBinding(ctx, s.componentAppID, input.AuthorizerAppID)
	if err != nil {
		return wrapAuthorizerTenantBindingError("get callback tenant binding", err)
	}
	if _, err := s.events.GetCallbackEventByKey(ctx, binding.TenantID, wechatcallback.EventTypePublishResult, event.PublishID); err == nil {
		return nil
	} else if !errors.Is(err, wechatcallback.ErrNotFound) {
		return fmt.Errorf("get callback event for dedupe: %w", err)
	}
	_, err = s.publishes.HandlePublishResult(ctx, HandlePublishResultInput{
		TenantID: binding.TenantID, WeChatPublishID: event.PublishID, Status: event.Status,
		WeChatArticleID: event.ArticleID, ErrorCode: event.ErrorCode, ErrorMessage: event.ErrorMessage,
	})
	if err != nil {
		return fmt.Errorf("handle publish callback result: %w", err)
	}
	receivedAt := s.now()
	_, err = s.events.SaveCallbackEvent(ctx, wechatcallback.Event{
		TenantID: binding.TenantID, ComponentAppID: s.componentAppID, AuthorizerAppID: input.AuthorizerAppID,
		EventType: wechatcallback.EventTypePublishResult, EventKey: event.PublishID, RawBody: string(plaintext),
		ReceivedAt: receivedAt, RetainUntil: receivedAt.Add(s.retention),
	})
	if err != nil && errors.Is(err, wechatcallback.ErrDuplicate) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("save callback event audit: %w", err)
	}
	return nil
}

func (s *CallbackService) validateReady() error {
	if s == nil || s.events == nil || s.bindings == nil || s.publishes == nil {
		return fmt.Errorf("validate callback service dependencies: %w", ErrNotImplemented)
	}
	if strings.TrimSpace(s.componentAppID) == "" {
		return fmt.Errorf("validate callback component app id: %w", ErrNotImplemented)
	}
	return nil
}

func (s *CallbackService) callbackPlaintext(ctx context.Context, input HandleAuthorizerCallbackInput) ([]byte, error) {
	encryptType := strings.TrimSpace(input.EncryptType)
	if encryptType == "" && strings.TrimSpace(input.MsgSignature) == "" {
		return input.RawBody, nil
	}
	if encryptType != "" && encryptType != "aes" {
		return nil, fmt.Errorf("validate authorizer callback encrypt type: %w", ErrInvalidInput)
	}
	if strings.TrimSpace(input.MsgSignature) == "" || strings.TrimSpace(input.Timestamp) == "" || strings.TrimSpace(input.Nonce) == "" {
		return nil, fmt.Errorf("validate encrypted authorizer callback query: %w", ErrInvalidInput)
	}
	if s.decryptor == nil {
		return nil, fmt.Errorf("validate authorizer callback decryptor: %w", ErrNotImplemented)
	}
	var envelope encryptedComponentCallbackEnvelope
	if err := xml.Unmarshal(input.RawBody, &envelope); err != nil {
		return nil, fmt.Errorf("decode encrypted authorizer callback xml: %w", ErrInvalidInput)
	}
	if strings.TrimSpace(envelope.Encrypt) == "" {
		return nil, fmt.Errorf("validate encrypted authorizer callback payload: %w", ErrInvalidInput)
	}
	plaintext, err := s.decryptor.DecryptComponentCallback(ctx, authorization.ComponentCallbackDecryptInput{
		Signature: input.MsgSignature, Timestamp: input.Timestamp, Nonce: input.Nonce, Ciphertext: envelope.Encrypt,
	})
	if err != nil {
		return nil, wrapComponentCallbackDecryptError("decrypt authorizer callback", err)
	}
	return plaintext, nil
}

type publishResultCallback struct {
	PublishID    string
	Status       publish.Status
	ArticleID    string
	ErrorCode    string
	ErrorMessage string
}

func decodePublishResultCallback(plaintext []byte) (publishResultCallback, bool, error) {
	var body publishResultCallbackXML
	if err := xml.Unmarshal(plaintext, &body); err != nil {
		return publishResultCallback{}, false, fmt.Errorf("decode publish callback xml: %w", ErrInvalidInput)
	}
	if !strings.EqualFold(strings.TrimSpace(body.Event), "PUBLISHJOBFINISH") {
		return publishResultCallback{}, false, nil
	}
	publishID := firstNonEmpty(body.PublishID, body.PublishIDAlt, body.PublishIDFlat)
	if strings.TrimSpace(publishID) == "" {
		return publishResultCallback{}, false, fmt.Errorf("validate publish callback publish id: %w", ErrInvalidInput)
	}
	statusValue, err := strconv.Atoi(firstNonEmpty(body.PublishStatus, body.PublishStatusAlt, body.PublishStatusFlat))
	if err != nil {
		return publishResultCallback{}, false, fmt.Errorf("validate publish callback status: %w", ErrInvalidInput)
	}
	status := publish.StatusPublishing
	errorCode := ""
	errorMessage := ""
	if statusValue == 0 {
		status = publish.StatusPublished
	} else if statusValue != 1 {
		status = publish.StatusFailed
		errorCode = fmt.Sprintf("publish_status_%d", statusValue)
		errorMessage = fmt.Sprintf("wechat publish callback failed with status %d", statusValue)
	}
	return publishResultCallback{
		PublishID: publishID, Status: status, ArticleID: firstNonEmpty(body.ArticleID, body.ArticleIDAlt, body.ArticleIDFlat),
		ErrorCode: errorCode, ErrorMessage: errorMessage,
	}, true, nil
}

type publishResultCallbackXML struct {
	Event             string `xml:"Event"`
	PublishID         string `xml:"PublishEventInfo>publish_id"`
	PublishIDAlt      string `xml:"PublishEventInfo>PublishId"`
	PublishIDFlat     string `xml:"publish_id"`
	PublishStatus     string `xml:"PublishEventInfo>publish_status"`
	PublishStatusAlt  string `xml:"PublishEventInfo>PublishStatus"`
	PublishStatusFlat string `xml:"publish_status"`
	ArticleID         string `xml:"PublishEventInfo>article_id"`
	ArticleIDAlt      string `xml:"PublishEventInfo>ArticleId"`
	ArticleIDFlat     string `xml:"article_id"`
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
