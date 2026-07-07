package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"official-account-service/internal/domain/taskqueue"
)

func TestTaskQueueServiceListsArchivedTasks(t *testing.T) {
	repo := &fakeArchivedTaskRepository{tasks: []taskqueue.ArchivedTask{{
		ID: "task-1", Queue: taskqueue.QueuePublish, Type: "publish:sync_status",
		Payload: map[string]string{"tenant_id": "tenant-1"}, Retried: 2, MaxRetry: 3,
		LastError: "publish still processing", LastFailedAt: time.Date(2026, 7, 6, 8, 0, 0, 0, time.UTC),
	}}}
	service := NewTaskQueueService(repo)

	items, err := service.ListArchivedTasks(t.Context(), ListArchivedTasksInput{Queue: taskqueue.QueuePublish, Limit: 10})
	require.NoError(t, err)

	require.Equal(t, taskqueue.QueuePublish, repo.listQueue)
	require.Equal(t, 10, repo.listLimit)
	require.Equal(t, repo.tasks, items)
}

func TestTaskQueueServiceDefaultsAndValidatesListInput(t *testing.T) {
	repo := &fakeArchivedTaskRepository{}
	service := NewTaskQueueService(repo)

	_, err := service.ListArchivedTasks(t.Context(), ListArchivedTasksInput{Queue: taskqueue.QueueToken})
	require.NoError(t, err)
	require.Equal(t, defaultArchivedTaskLimit, repo.listLimit)

	_, err = service.ListArchivedTasks(t.Context(), ListArchivedTasksInput{Queue: "default"})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))

	_, err = service.ListArchivedTasks(t.Context(), ListArchivedTasksInput{Queue: taskqueue.QueuePublish, Limit: 101})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))

	_, err = (*TaskQueueService)(nil).ListArchivedTasks(t.Context(), ListArchivedTasksInput{Queue: taskqueue.QueuePublish})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrNotImplemented))
}

func TestTaskQueueServiceRetriesArchivedTask(t *testing.T) {
	repo := &fakeArchivedTaskRepository{}
	service := NewTaskQueueService(repo)

	err := service.RetryArchivedTask(t.Context(), RetryArchivedTaskInput{Queue: taskqueue.QueueToken, TaskID: "task-1"})
	require.NoError(t, err)

	require.Equal(t, taskqueue.QueueToken, repo.retryQueue)
	require.Equal(t, "task-1", repo.retryID)
}

func TestTaskQueueServiceMapsRepositoryErrors(t *testing.T) {
	service := NewTaskQueueService(&fakeArchivedTaskRepository{err: taskqueue.ErrNotFound})
	_, err := service.ListArchivedTasks(t.Context(), ListArchivedTasksInput{Queue: taskqueue.QueuePublish})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrNotFound))

	service = NewTaskQueueService(&fakeArchivedTaskRepository{err: taskqueue.ErrUnavailable})
	err = service.RetryArchivedTask(t.Context(), RetryArchivedTaskInput{Queue: taskqueue.QueuePublish, TaskID: "task-1"})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrNotImplemented))
}

type fakeArchivedTaskRepository struct {
	tasks      []taskqueue.ArchivedTask
	err        error
	listQueue  string
	listLimit  int
	retryQueue string
	retryID    string
}

func (r *fakeArchivedTaskRepository) ListArchivedTasks(_ context.Context, queue string, limit int) ([]taskqueue.ArchivedTask, error) {
	r.listQueue = queue
	r.listLimit = limit
	if r.err != nil {
		return nil, r.err
	}
	return r.tasks, nil
}

func (r *fakeArchivedTaskRepository) RetryArchivedTask(_ context.Context, queue string, id string) error {
	r.retryQueue = queue
	r.retryID = id
	return r.err
}
