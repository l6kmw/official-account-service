package wechatcallback

import (
	"context"
	"errors"
	"time"
)

// ErrNotFound indicates a callback event was not found.
var ErrNotFound = errors.New("wechat callback event not found")

// ErrDuplicate indicates a callback event was already processed.
var ErrDuplicate = errors.New("wechat callback event duplicate")

const (
	// EventTypePublishResult is the callback event type for WeChat free publish results.
	EventTypePublishResult = "publish_result"
)

// Event stores callback raw payload and dedupe metadata.
type Event struct {
	ID              int64
	TenantID        string
	ComponentAppID  string
	AuthorizerAppID string
	EventType       string
	EventKey        string
	RawBody         string
	ReceivedAt      time.Time
	RetainUntil     time.Time
	CreatedAt       time.Time
}

// Repository persists callback audit events and dedupe keys.
type Repository interface {
	SaveCallbackEvent(ctx context.Context, event Event) (Event, error)
	GetCallbackEventByKey(ctx context.Context, tenantID string, eventType string, eventKey string) (Event, error)
}
