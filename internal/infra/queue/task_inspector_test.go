package queue

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/require"

	"official-account-service/internal/domain/taskqueue"
)

func TestTaskInspectorListsArchivedTasksWithSafePayload(t *testing.T) {
	redis := miniredis.RunT(t)
	client := asynq.NewClient(asynq.RedisClientOpt{Addr: redis.Addr()})
	defer func() { require.NoError(t, client.Close()) }()
	asynqInspector := asynq.NewInspector(asynq.RedisClientOpt{Addr: redis.Addr()})
	defer func() { require.NoError(t, asynqInspector.Close()) }()
	inspector, err := NewTaskInspector(redis.Addr())
	require.NoError(t, err)
	defer func() { require.NoError(t, inspector.Close()) }()
	payload, err := json.Marshal(map[string]any{
		"tenant_id":                "tenant-1",
		"publish_record_id":        12,
		"access_token":             "secret-token",
		"authorizer_refresh_token": "secret-refresh",
		"raw_payload":              "<xml>secret</xml>",
	})
	require.NoError(t, err)
	info, err := client.EnqueueContext(t.Context(), asynq.NewTask(TypeSyncPublishStatus, payload), asynq.Queue(PublishQueueName), asynq.MaxRetry(5))
	require.NoError(t, err)
	require.NoError(t, asynqInspector.ArchiveTask(PublishQueueName, info.ID))

	items, err := inspector.ListArchivedTasks(context.Background(), PublishQueueName, 10)
	require.NoError(t, err)

	require.Len(t, items, 1)
	require.Equal(t, info.ID, items[0].ID)
	require.Equal(t, PublishQueueName, items[0].Queue)
	require.Equal(t, TypeSyncPublishStatus, items[0].Type)
	require.Equal(t, 5, items[0].MaxRetry)
	require.Equal(t, map[string]string{"tenant_id": "tenant-1", "publish_record_id": "12"}, items[0].Payload)
	require.NotContains(t, items[0].Payload, "access_token")
	require.NotContains(t, items[0].Payload, "authorizer_refresh_token")
	require.NotContains(t, items[0].Payload, "raw_payload")
}

func TestTaskInspectorRetriesArchivedTask(t *testing.T) {
	redis := miniredis.RunT(t)
	client := asynq.NewClient(asynq.RedisClientOpt{Addr: redis.Addr()})
	defer func() { require.NoError(t, client.Close()) }()
	asynqInspector := asynq.NewInspector(asynq.RedisClientOpt{Addr: redis.Addr()})
	defer func() { require.NoError(t, asynqInspector.Close()) }()
	inspector, err := NewTaskInspector(redis.Addr())
	require.NoError(t, err)
	defer func() { require.NoError(t, inspector.Close()) }()
	info, err := client.EnqueueContext(t.Context(), asynq.NewTask(TypeRefreshAuthorizerAccessToken, []byte(`{"tenant_id":"tenant-1","account_id":7,"component_app_id":"wx-component"}`)), asynq.Queue(TokenQueueName))
	require.NoError(t, err)
	require.NoError(t, asynqInspector.ArchiveTask(TokenQueueName, info.ID))

	err = inspector.RetryArchivedTask(context.Background(), TokenQueueName, info.ID)
	require.NoError(t, err)

	archived, err := asynqInspector.ListArchivedTasks(TokenQueueName)
	require.NoError(t, err)
	require.Empty(t, archived)
	pending, err := asynqInspector.ListPendingTasks(TokenQueueName)
	require.NoError(t, err)
	require.Len(t, pending, 1)
	require.Equal(t, info.ID, pending[0].ID)
}

func TestTaskInspectorRequiresRedisAddr(t *testing.T) {
	inspector, err := NewTaskInspector("")
	require.Nil(t, inspector)
	require.Error(t, err)
	require.True(t, errors.Is(err, taskqueue.ErrUnavailable))
}
