package queue

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/require"

	"official-account-service/internal/domain/publish"
)

func TestPublishStatusSyncQueueEnqueuesTask(t *testing.T) {
	redis := miniredis.RunT(t)
	queue, err := NewPublishStatusSyncQueue(PublishStatusSyncQueueConfig{
		RedisAddr: redis.Addr(),
		Delay:     time.Millisecond,
		Timeout:   time.Second,
		MaxRetry:  3,
		UniqueFor: time.Second,
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, queue.Close()) }()

	err = queue.EnqueueStatusSync(context.Background(), publish.StatusSyncTask{
		TenantID: "tenant-1", PublishRecordID: 12,
	})
	require.NoError(t, err)

	inspector := asynq.NewInspector(asynq.RedisClientOpt{Addr: redis.Addr()})
	defer func() { require.NoError(t, inspector.Close()) }()
	items, err := inspector.ListScheduledTasks(PublishQueueName)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, TypeSyncPublishStatus, items[0].Type)
	require.Equal(t, 3, items[0].MaxRetry)

	task, err := decodeStatusSyncTask(items[0].Payload)
	require.NoError(t, err)
	require.Equal(t, "tenant-1", task.TenantID)
	require.Equal(t, int64(12), task.PublishRecordID)
}

func TestPublishStatusSyncQueueDeduplicatesTasks(t *testing.T) {
	redis := miniredis.RunT(t)
	queue, err := NewPublishStatusSyncQueue(PublishStatusSyncQueueConfig{
		RedisAddr: redis.Addr(),
		Delay:     time.Second,
		Timeout:   time.Second,
		MaxRetry:  3,
		UniqueFor: time.Minute,
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, queue.Close()) }()
	task := publish.StatusSyncTask{TenantID: "tenant-1", PublishRecordID: 12}

	require.NoError(t, queue.EnqueueStatusSync(context.Background(), task))
	require.NoError(t, queue.EnqueueStatusSync(context.Background(), task))

	inspector := asynq.NewInspector(asynq.RedisClientOpt{Addr: redis.Addr()})
	defer func() { require.NoError(t, inspector.Close()) }()
	items, err := inspector.ListScheduledTasks(PublishQueueName)
	require.NoError(t, err)
	require.Len(t, items, 1)
}

func TestPublishStatusSyncHandlerProcessesTask(t *testing.T) {
	handler := &fakeStatusSyncHandler{}
	payload, err := encodeStatusSyncTask(publish.StatusSyncTask{TenantID: "tenant-1", PublishRecordID: 12})
	require.NoError(t, err)

	err = NewPublishStatusSyncHandler(handler).ProcessTask(context.Background(), asynq.NewTask(TypeSyncPublishStatus, payload))
	require.NoError(t, err)

	require.Equal(t, publish.StatusSyncTask{TenantID: "tenant-1", PublishRecordID: 12}, handler.lastTask)
}

func TestPublishStatusSyncHandlerReturnsRetryableErrors(t *testing.T) {
	handler := &fakeStatusSyncHandler{err: publish.ErrNotFound}
	payload, err := encodeStatusSyncTask(publish.StatusSyncTask{TenantID: "tenant-1", PublishRecordID: 12})
	require.NoError(t, err)

	err = NewPublishStatusSyncHandler(handler).ProcessTask(context.Background(), asynq.NewTask(TypeSyncPublishStatus, payload))
	require.Error(t, err)
	require.True(t, errors.Is(err, publish.ErrNotFound))
}

type fakeStatusSyncHandler struct {
	lastTask publish.StatusSyncTask
	err      error
}

func (h *fakeStatusSyncHandler) HandleStatusSyncTask(_ context.Context, task publish.StatusSyncTask) error {
	h.lastTask = task
	return h.err
}
