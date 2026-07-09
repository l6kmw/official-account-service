package application

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"official-account-service/internal/domain/taskqueue"
)

const (
	defaultArchivedTaskLimit = 30
	maxArchivedTaskLimit     = 100
)

// TaskQueueService manages background task queues for operators.
type TaskQueueService struct {
	tasks taskqueue.ArchivedTaskRepository
}

// NewTaskQueueService constructs a TaskQueueService.
func NewTaskQueueService(tasks taskqueue.ArchivedTaskRepository) *TaskQueueService {
	return &TaskQueueService{tasks: tasks}
}

// ListArchivedTasksInput contains archived task listing filters.
type ListArchivedTasksInput struct {
	TenantID string
	Queue    string
	Limit    int
}

// RetryArchivedTaskInput identifies one archived task to replay.
type RetryArchivedTaskInput struct {
	TenantID string
	Queue    string
	TaskID   string
}

// ListArchivedTasks returns archived tasks for an operator queue.
func (s *TaskQueueService) ListArchivedTasks(ctx context.Context, input ListArchivedTasksInput) ([]taskqueue.ArchivedTask, error) {
	if err := s.validateReady(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(input.TenantID) == "" {
		return nil, fmt.Errorf("validate archived task tenant id: %w", ErrInvalidInput)
	}
	queue, err := validateTaskQueue(input.Queue)
	if err != nil {
		return nil, err
	}
	limit := input.Limit
	if limit == 0 {
		limit = defaultArchivedTaskLimit
	}
	if limit < 0 || limit > maxArchivedTaskLimit {
		return nil, fmt.Errorf("validate archived task limit: %w", ErrInvalidInput)
	}
	tasks, err := s.tasks.ListArchivedTasks(ctx, queue, limit)
	if err != nil {
		return nil, wrapTaskQueueError("list archived tasks", err)
	}
	return filterArchivedTasksByTenant(tasks, input.TenantID), nil
}

// RetryArchivedTask replays one archived task by moving it back to pending.
func (s *TaskQueueService) RetryArchivedTask(ctx context.Context, input RetryArchivedTaskInput) error {
	if err := s.validateReady(); err != nil {
		return err
	}
	if strings.TrimSpace(input.TenantID) == "" {
		return fmt.Errorf("validate archived task tenant id: %w", ErrInvalidInput)
	}
	queue, err := validateTaskQueue(input.Queue)
	if err != nil {
		return err
	}
	if strings.TrimSpace(input.TaskID) == "" {
		return fmt.Errorf("validate archived task id: %w", ErrInvalidInput)
	}
	task, err := s.tasks.GetArchivedTask(ctx, queue, input.TaskID)
	if err != nil {
		return wrapTaskQueueError("get archived task", err)
	}
	if task.Payload["tenant_id"] != strings.TrimSpace(input.TenantID) {
		return fmt.Errorf("validate archived task tenant: %w", ErrNotFound)
	}
	if err := s.tasks.RetryArchivedTask(ctx, queue, input.TaskID); err != nil {
		return wrapTaskQueueError("retry archived task", err)
	}
	return nil
}

func filterArchivedTasksByTenant(tasks []taskqueue.ArchivedTask, tenantID string) []taskqueue.ArchivedTask {
	tenantID = strings.TrimSpace(tenantID)
	out := make([]taskqueue.ArchivedTask, 0, len(tasks))
	for _, task := range tasks {
		if task.Payload["tenant_id"] == tenantID {
			out = append(out, task)
		}
	}
	return out
}

func (s *TaskQueueService) validateReady() error {
	if s == nil || s.tasks == nil {
		return fmt.Errorf("validate task queue service dependencies: %w", ErrNotImplemented)
	}
	return nil
}

func validateTaskQueue(queue string) (string, error) {
	queue = strings.TrimSpace(queue)
	if queue != taskqueue.QueuePublish && queue != taskqueue.QueueToken {
		return "", fmt.Errorf("validate task queue: %w", ErrInvalidInput)
	}
	return queue, nil
}

func wrapTaskQueueError(action string, err error) error {
	if errors.Is(err, taskqueue.ErrNotFound) {
		return fmt.Errorf("%s: %w", action, ErrNotFound)
	}
	if errors.Is(err, taskqueue.ErrUnavailable) {
		return fmt.Errorf("%s: %w", action, ErrNotImplemented)
	}
	return fmt.Errorf("%s: %w", action, err)
}
