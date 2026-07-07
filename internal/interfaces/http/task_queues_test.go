package http

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"official-account-service/internal/application"
	"official-account-service/internal/domain/taskqueue"
)

func TestTaskQueueRoutesListArchivedTasks(t *testing.T) {
	repo := &taskQueueRouteFakeRepository{tasks: []taskqueue.ArchivedTask{{
		ID: "task-1", Queue: taskqueue.QueuePublish, Type: "publish:sync_status",
		Payload: map[string]string{"tenant_id": "tenant-1", "publish_record_id": "12"},
		Retried: 2, MaxRetry: 3, LastError: "access_token secret leaked from dependency",
		LastFailedAt: time.Date(2026, 7, 6, 8, 0, 0, 0, time.UTC),
	}}}
	router := NewRouter(Dependencies{
		Logger:     zap.NewNop(),
		TaskQueues: application.NewTaskQueueService(repo),
	})

	recorder := doJSON(t, router, http.MethodGet, "/api/v1/task-queues/publish/archived-tasks?limit=5", ``, "")
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, taskqueue.QueuePublish, repo.listQueue)
	require.Equal(t, 5, repo.listLimit)
	require.Contains(t, recorder.Body.String(), `"id":"task-1"`)
	require.Contains(t, recorder.Body.String(), `"tenant_id":"tenant-1"`)
	require.Contains(t, recorder.Body.String(), `"last_error":"[REDACTED]"`)
	require.NotContains(t, recorder.Body.String(), "access_token")
	require.NotContains(t, recorder.Body.String(), "refresh_token")
	require.NotContains(t, recorder.Body.String(), "secret leaked")
}

func TestTaskQueueRoutesValidateArchivedTaskList(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:     zap.NewNop(),
		TaskQueues: application.NewTaskQueueService(&taskQueueRouteFakeRepository{}),
	})

	badQueue := doJSON(t, router, http.MethodGet, "/api/v1/task-queues/default/archived-tasks", ``, "")
	require.Equal(t, http.StatusBadRequest, badQueue.Code)

	badLimit := doJSON(t, router, http.MethodGet, "/api/v1/task-queues/publish/archived-tasks?limit=101", ``, "")
	require.Equal(t, http.StatusBadRequest, badLimit.Code)

	unavailable := doJSON(t, NewRouter(Dependencies{Logger: zap.NewNop()}), http.MethodGet, "/api/v1/task-queues/publish/archived-tasks", ``, "")
	require.Equal(t, http.StatusNotImplemented, unavailable.Code)
}

func TestTaskQueueRoutesRetryArchivedTask(t *testing.T) {
	repo := &taskQueueRouteFakeRepository{}
	router := NewRouter(Dependencies{
		Logger:     zap.NewNop(),
		TaskQueues: application.NewTaskQueueService(repo),
	})

	recorder := doJSON(t, router, http.MethodPost, "/api/v1/task-queues/token/archived-tasks/task-1/retry", ``, "")
	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"status":"queued"}`, recorder.Body.String())
	require.Equal(t, taskqueue.QueueToken, repo.retryQueue)
	require.Equal(t, "task-1", repo.retryID)

	missingRouter := NewRouter(Dependencies{
		Logger:     zap.NewNop(),
		TaskQueues: application.NewTaskQueueService(&taskQueueRouteFakeRepository{err: taskqueue.ErrNotFound}),
	})
	missing := doJSON(t, missingRouter, http.MethodPost, "/api/v1/task-queues/token/archived-tasks/missing/retry", ``, "")
	require.Equal(t, http.StatusNotFound, missing.Code)
}

type taskQueueRouteFakeRepository struct {
	tasks      []taskqueue.ArchivedTask
	err        error
	listQueue  string
	listLimit  int
	retryQueue string
	retryID    string
}

func (r *taskQueueRouteFakeRepository) ListArchivedTasks(_ context.Context, queue string, limit int) ([]taskqueue.ArchivedTask, error) {
	r.listQueue = queue
	r.listLimit = limit
	if r.err != nil {
		return nil, r.err
	}
	return r.tasks, nil
}

func (r *taskQueueRouteFakeRepository) RetryArchivedTask(_ context.Context, queue string, id string) error {
	r.retryQueue = queue
	r.retryID = id
	if r.err != nil {
		return r.err
	}
	return nil
}
