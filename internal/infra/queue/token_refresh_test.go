package queue

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/require"

	"official-account-service/internal/domain/authorization"
)

func TestTokenRefreshQueueSchedulesTask(t *testing.T) {
	redis := miniredis.RunT(t)
	queue, err := NewTokenRefreshQueue(TokenRefreshQueueConfig{
		RedisAddr: redis.Addr(),
		Timeout:   time.Second,
		MaxRetry:  4,
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, queue.Close()) }()

	err = queue.ScheduleTokenRefresh(context.Background(), authorization.TokenRefreshTask{
		TenantID: "tenant-1", AccountID: 12, ComponentAppID: "wx-component",
	}, time.Minute)
	require.NoError(t, err)

	inspector := asynq.NewInspector(asynq.RedisClientOpt{Addr: redis.Addr()})
	defer func() { require.NoError(t, inspector.Close()) }()
	items, err := inspector.ListScheduledTasks(TokenQueueName)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, TypeRefreshAuthorizerAccessToken, items[0].Type)
	require.Equal(t, 4, items[0].MaxRetry)

	task, err := decodeTokenRefreshTask(items[0].Payload)
	require.NoError(t, err)
	require.Equal(t, "tenant-1", task.TenantID)
	require.Equal(t, int64(12), task.AccountID)
	require.Equal(t, "wx-component", task.ComponentAppID)
}

func TestTokenRefreshQueueAllowsRepeatedSchedules(t *testing.T) {
	redis := miniredis.RunT(t)
	queue, err := NewTokenRefreshQueue(TokenRefreshQueueConfig{
		RedisAddr: redis.Addr(),
		Timeout:   time.Second,
		MaxRetry:  4,
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, queue.Close()) }()
	task := authorization.TokenRefreshTask{TenantID: "tenant-1", AccountID: 12, ComponentAppID: "wx-component"}

	require.NoError(t, queue.ScheduleTokenRefresh(context.Background(), task, time.Minute))
	require.NoError(t, queue.ScheduleTokenRefresh(context.Background(), task, time.Minute))

	inspector := asynq.NewInspector(asynq.RedisClientOpt{Addr: redis.Addr()})
	defer func() { require.NoError(t, inspector.Close()) }()
	items, err := inspector.ListScheduledTasks(TokenQueueName)
	require.NoError(t, err)
	require.Len(t, items, 2)
}

func TestTokenRefreshHandlerProcessesTask(t *testing.T) {
	handler := &fakeTokenRefreshHandler{}
	payload, err := encodeTokenRefreshTask(authorization.TokenRefreshTask{TenantID: "tenant-1", AccountID: 12, ComponentAppID: "wx-component"})
	require.NoError(t, err)

	err = NewTokenRefreshHandler(handler).ProcessTask(context.Background(), asynq.NewTask(TypeRefreshAuthorizerAccessToken, payload))
	require.NoError(t, err)

	require.Equal(t, authorization.TokenRefreshTask{TenantID: "tenant-1", AccountID: 12, ComponentAppID: "wx-component"}, handler.lastTask)
}

func TestTokenRefreshHandlerReturnsRetryableErrors(t *testing.T) {
	handler := &fakeTokenRefreshHandler{err: authorization.ErrAuthorizerClientUnavailable}
	payload, err := encodeTokenRefreshTask(authorization.TokenRefreshTask{TenantID: "tenant-1", AccountID: 12, ComponentAppID: "wx-component"})
	require.NoError(t, err)

	err = NewTokenRefreshHandler(handler).ProcessTask(context.Background(), asynq.NewTask(TypeRefreshAuthorizerAccessToken, payload))
	require.Error(t, err)
	require.True(t, errors.Is(err, authorization.ErrAuthorizerClientUnavailable))
}

type fakeTokenRefreshHandler struct {
	lastTask authorization.TokenRefreshTask
	err      error
}

func (h *fakeTokenRefreshHandler) HandleTokenRefreshTask(_ context.Context, task authorization.TokenRefreshTask) error {
	h.lastTask = task
	return h.err
}
