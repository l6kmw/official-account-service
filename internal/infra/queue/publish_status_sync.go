package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hibiken/asynq"

	"official-account-service/internal/domain/publish"
)

const (
	// PublishQueueName is the asynq queue for publish-related background work.
	PublishQueueName = "publish"
	// TypeSyncPublishStatus is the asynq task type for WeChat publish status polling.
	TypeSyncPublishStatus = "publish:sync_status"
)

const (
	defaultPublishStatusSyncDelay     = time.Minute
	defaultPublishStatusSyncTimeout   = 30 * time.Second
	defaultPublishStatusSyncMaxRetry  = 10
	defaultPublishStatusSyncUniqueFor = 10 * time.Minute
)

// PublishStatusSyncQueueConfig configures publish status sync task enqueueing.
type PublishStatusSyncQueueConfig struct {
	RedisAddr string
	Delay     time.Duration
	Timeout   time.Duration
	MaxRetry  int
	UniqueFor time.Duration
}

// PublishStatusSyncQueue enqueues publish status sync tasks using asynq.
type PublishStatusSyncQueue struct {
	client    *asynq.Client
	delay     time.Duration
	timeout   time.Duration
	maxRetry  int
	uniqueFor time.Duration
}

// NewPublishStatusSyncQueue constructs an asynq-backed publish status sync queue.
func NewPublishStatusSyncQueue(cfg PublishStatusSyncQueueConfig) (*PublishStatusSyncQueue, error) {
	if strings.TrimSpace(cfg.RedisAddr) == "" {
		return nil, fmt.Errorf("validate publish status sync redis addr: %w", publish.ErrTaskQueueUnavailable)
	}
	if cfg.Delay == 0 {
		cfg.Delay = defaultPublishStatusSyncDelay
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = defaultPublishStatusSyncTimeout
	}
	if cfg.MaxRetry == 0 {
		cfg.MaxRetry = defaultPublishStatusSyncMaxRetry
	}
	if cfg.UniqueFor == 0 {
		cfg.UniqueFor = defaultPublishStatusSyncUniqueFor
	}
	if cfg.Delay < 0 || cfg.Timeout < 0 || cfg.MaxRetry < 0 || cfg.UniqueFor < 0 {
		return nil, fmt.Errorf("validate publish status sync queue config: %w", publish.ErrTaskQueueUnavailable)
	}
	return &PublishStatusSyncQueue{
		client:    asynq.NewClient(asynq.RedisClientOpt{Addr: cfg.RedisAddr}),
		delay:     cfg.Delay,
		timeout:   cfg.Timeout,
		maxRetry:  cfg.MaxRetry,
		uniqueFor: cfg.UniqueFor,
	}, nil
}

// EnqueueStatusSync enqueues one publish status sync task.
func (q *PublishStatusSyncQueue) EnqueueStatusSync(ctx context.Context, task publish.StatusSyncTask) error {
	if q == nil || q.client == nil {
		return fmt.Errorf("validate publish status sync queue: %w", publish.ErrTaskQueueUnavailable)
	}
	payload, err := encodeStatusSyncTask(task)
	if err != nil {
		return err
	}
	opts := []asynq.Option{
		asynq.Queue(PublishQueueName),
		asynq.MaxRetry(q.maxRetry),
	}
	if q.delay > 0 {
		opts = append(opts, asynq.ProcessIn(q.delay))
	}
	if q.timeout > 0 {
		opts = append(opts, asynq.Timeout(q.timeout))
	}
	if q.uniqueFor > 0 {
		opts = append(opts, asynq.Unique(q.uniqueFor))
	}
	_, err = q.client.EnqueueContext(ctx, asynq.NewTask(TypeSyncPublishStatus, payload), opts...)
	if err != nil && errors.Is(err, asynq.ErrDuplicateTask) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("enqueue publish status sync task: %w", err)
	}
	return nil
}

// Close closes the underlying asynq client.
func (q *PublishStatusSyncQueue) Close() error {
	if q == nil || q.client == nil {
		return nil
	}
	if err := q.client.Close(); err != nil {
		return fmt.Errorf("close publish status sync queue: %w", err)
	}
	return nil
}

// PublishStatusSyncHandler processes publish status sync tasks using an application handler.
type PublishStatusSyncHandler struct {
	handler publish.StatusSyncHandler
}

// NewPublishStatusSyncHandler constructs a PublishStatusSyncHandler.
func NewPublishStatusSyncHandler(handler publish.StatusSyncHandler) *PublishStatusSyncHandler {
	return &PublishStatusSyncHandler{handler: handler}
}

// ProcessTask handles one asynq publish status sync task.
func (h *PublishStatusSyncHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	if h == nil || h.handler == nil {
		return fmt.Errorf("validate publish status sync handler: %w", publish.ErrTaskQueueUnavailable)
	}
	if task == nil || task.Type() != TypeSyncPublishStatus {
		return fmt.Errorf("validate publish status sync task type: %w", publish.ErrTaskQueueUnavailable)
	}
	payload, err := decodeStatusSyncTask(task.Payload())
	if err != nil {
		return err
	}
	if err := h.handler.HandleStatusSyncTask(ctx, payload); err != nil {
		return fmt.Errorf("handle publish status sync task: %w", err)
	}
	return nil
}

// RegisterPublishStatusSyncHandler registers publish status sync handling on an asynq mux.
func RegisterPublishStatusSyncHandler(mux *asynq.ServeMux, handler publish.StatusSyncHandler) error {
	if mux == nil || handler == nil {
		return fmt.Errorf("validate publish status sync mux: %w", publish.ErrTaskQueueUnavailable)
	}
	mux.Handle(TypeSyncPublishStatus, NewPublishStatusSyncHandler(handler))
	return nil
}

type statusSyncPayload struct {
	TenantID        string `json:"tenant_id"`
	PublishRecordID int64  `json:"publish_record_id"`
}

func encodeStatusSyncTask(task publish.StatusSyncTask) ([]byte, error) {
	if strings.TrimSpace(task.TenantID) == "" || task.PublishRecordID <= 0 {
		return nil, fmt.Errorf("validate publish status sync payload: %w", publish.ErrTaskQueueUnavailable)
	}
	payload, err := json.Marshal(statusSyncPayload{
		TenantID: task.TenantID, PublishRecordID: task.PublishRecordID,
	})
	if err != nil {
		return nil, fmt.Errorf("encode publish status sync payload: %w", err)
	}
	return payload, nil
}

func decodeStatusSyncTask(raw []byte) (publish.StatusSyncTask, error) {
	var payload statusSyncPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return publish.StatusSyncTask{}, fmt.Errorf("decode publish status sync payload: %w", err)
	}
	if strings.TrimSpace(payload.TenantID) == "" || payload.PublishRecordID <= 0 {
		return publish.StatusSyncTask{}, fmt.Errorf("validate publish status sync payload: %w", publish.ErrTaskQueueUnavailable)
	}
	return publish.StatusSyncTask{
		TenantID: payload.TenantID, PublishRecordID: payload.PublishRecordID,
	}, nil
}
