package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/hibiken/asynq"

	"official-account-service/internal/domain/authorization"
)

const (
	// TokenQueueName is the asynq queue for token-related background work.
	TokenQueueName = "token"
	// TypeRefreshAuthorizerAccessToken is the asynq task type for authorizer token refresh.
	TypeRefreshAuthorizerAccessToken = "token:refresh_authorizer_access_token"
)

const (
	defaultTokenRefreshTaskTimeout  = 30 * time.Second
	defaultTokenRefreshTaskMaxRetry = 10
)

// TokenRefreshQueueConfig configures token refresh task enqueueing.
type TokenRefreshQueueConfig struct {
	RedisAddr string
	Timeout   time.Duration
	MaxRetry  int
}

// TokenRefreshQueue enqueues token refresh tasks using asynq.
type TokenRefreshQueue struct {
	client   *asynq.Client
	timeout  time.Duration
	maxRetry int
}

// NewTokenRefreshQueue constructs an asynq-backed token refresh queue.
func NewTokenRefreshQueue(cfg TokenRefreshQueueConfig) (*TokenRefreshQueue, error) {
	if strings.TrimSpace(cfg.RedisAddr) == "" {
		return nil, fmt.Errorf("validate token refresh redis addr: %w", authorization.ErrTokenRefreshTaskQueueUnavailable)
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = defaultTokenRefreshTaskTimeout
	}
	if cfg.MaxRetry == 0 {
		cfg.MaxRetry = defaultTokenRefreshTaskMaxRetry
	}
	if cfg.Timeout < 0 || cfg.MaxRetry < 0 {
		return nil, fmt.Errorf("validate token refresh queue config: %w", authorization.ErrTokenRefreshTaskQueueUnavailable)
	}
	return &TokenRefreshQueue{
		client: asynq.NewClient(asynq.RedisClientOpt{Addr: cfg.RedisAddr}), timeout: cfg.Timeout,
		maxRetry: cfg.MaxRetry,
	}, nil
}

// ScheduleTokenRefresh schedules one token refresh task.
func (q *TokenRefreshQueue) ScheduleTokenRefresh(ctx context.Context, task authorization.TokenRefreshTask, delay time.Duration) error {
	if q == nil || q.client == nil {
		return fmt.Errorf("validate token refresh queue: %w", authorization.ErrTokenRefreshTaskQueueUnavailable)
	}
	if delay < 0 {
		return fmt.Errorf("validate token refresh delay: %w", authorization.ErrTokenRefreshTaskQueueUnavailable)
	}
	payload, err := encodeTokenRefreshTask(task)
	if err != nil {
		return err
	}
	opts := []asynq.Option{
		asynq.Queue(TokenQueueName),
		asynq.MaxRetry(q.maxRetry),
	}
	if delay > 0 {
		opts = append(opts, asynq.ProcessIn(delay))
	}
	if q.timeout > 0 {
		opts = append(opts, asynq.Timeout(q.timeout))
	}
	_, err = q.client.EnqueueContext(ctx, asynq.NewTask(TypeRefreshAuthorizerAccessToken, payload), opts...)
	if err != nil {
		return fmt.Errorf("enqueue token refresh task: %w", err)
	}
	return nil
}

// Close closes the underlying asynq client.
func (q *TokenRefreshQueue) Close() error {
	if q == nil || q.client == nil {
		return nil
	}
	if err := q.client.Close(); err != nil {
		return fmt.Errorf("close token refresh queue: %w", err)
	}
	return nil
}

// TokenRefreshHandler processes token refresh tasks using an application handler.
type TokenRefreshHandler struct {
	handler authorization.TokenRefreshHandler
}

// NewTokenRefreshHandler constructs a TokenRefreshHandler.
func NewTokenRefreshHandler(handler authorization.TokenRefreshHandler) *TokenRefreshHandler {
	return &TokenRefreshHandler{handler: handler}
}

// ProcessTask handles one asynq token refresh task.
func (h *TokenRefreshHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	if h == nil || h.handler == nil {
		return fmt.Errorf("validate token refresh handler: %w", authorization.ErrTokenRefreshTaskQueueUnavailable)
	}
	if task == nil || task.Type() != TypeRefreshAuthorizerAccessToken {
		return fmt.Errorf("validate token refresh task type: %w", authorization.ErrTokenRefreshTaskQueueUnavailable)
	}
	payload, err := decodeTokenRefreshTask(task.Payload())
	if err != nil {
		return err
	}
	if err := h.handler.HandleTokenRefreshTask(ctx, payload); err != nil {
		return fmt.Errorf("handle token refresh task: %w", err)
	}
	return nil
}

// RegisterTokenRefreshHandler registers token refresh handling on an asynq mux.
func RegisterTokenRefreshHandler(mux *asynq.ServeMux, handler authorization.TokenRefreshHandler) error {
	if mux == nil || handler == nil {
		return fmt.Errorf("validate token refresh mux: %w", authorization.ErrTokenRefreshTaskQueueUnavailable)
	}
	mux.Handle(TypeRefreshAuthorizerAccessToken, NewTokenRefreshHandler(handler))
	return nil
}

type tokenRefreshPayload struct {
	TenantID       string `json:"tenant_id"`
	AccountID      int64  `json:"account_id"`
	ComponentAppID string `json:"component_app_id"`
}

func encodeTokenRefreshTask(task authorization.TokenRefreshTask) ([]byte, error) {
	if strings.TrimSpace(task.TenantID) == "" || task.AccountID <= 0 || strings.TrimSpace(task.ComponentAppID) == "" {
		return nil, fmt.Errorf("validate token refresh payload: %w", authorization.ErrTokenRefreshTaskQueueUnavailable)
	}
	payload, err := json.Marshal(tokenRefreshPayload{
		TenantID: task.TenantID, AccountID: task.AccountID, ComponentAppID: task.ComponentAppID,
	})
	if err != nil {
		return nil, fmt.Errorf("encode token refresh payload: %w", err)
	}
	return payload, nil
}

func decodeTokenRefreshTask(raw []byte) (authorization.TokenRefreshTask, error) {
	var payload tokenRefreshPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return authorization.TokenRefreshTask{}, fmt.Errorf("decode token refresh payload: %w", err)
	}
	if strings.TrimSpace(payload.TenantID) == "" || payload.AccountID <= 0 || strings.TrimSpace(payload.ComponentAppID) == "" {
		return authorization.TokenRefreshTask{}, fmt.Errorf("validate token refresh payload: %w", authorization.ErrTokenRefreshTaskQueueUnavailable)
	}
	return authorization.TokenRefreshTask{
		TenantID: payload.TenantID, AccountID: payload.AccountID, ComponentAppID: payload.ComponentAppID,
	}, nil
}
