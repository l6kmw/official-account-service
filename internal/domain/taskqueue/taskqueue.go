package taskqueue

import (
	"context"
	"errors"
	"time"
)

// ErrNotFound indicates a task queue or task was not found.
var ErrNotFound = errors.New("task queue item not found")

// ErrUnavailable indicates the task queue backend is unavailable.
var ErrUnavailable = errors.New("task queue unavailable")

const (
	// QueuePublish is the queue for publish-related jobs.
	QueuePublish = "publish"
	// QueueToken is the queue for token-related jobs.
	QueueToken = "token"
)

// ArchivedTask describes one archived task without exposing raw payloads.
type ArchivedTask struct {
	ID           string
	Queue        string
	Type         string
	Payload      map[string]string
	Retried      int
	MaxRetry     int
	LastError    string
	LastFailedAt time.Time
}

// ArchivedTaskRepository manages archived background tasks.
type ArchivedTaskRepository interface {
	ListArchivedTasks(ctx context.Context, queue string, limit int) ([]ArchivedTask, error)
	GetArchivedTask(ctx context.Context, queue string, id string) (ArchivedTask, error)
	RetryArchivedTask(ctx context.Context, queue string, id string) error
}
