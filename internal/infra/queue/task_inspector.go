package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/hibiken/asynq"

	"official-account-service/internal/domain/taskqueue"
)

var safePayloadFields = map[string]struct{}{
	"tenant_id":         {},
	"publish_record_id": {},
	"account_id":        {},
	"component_app_id":  {},
}

// TaskInspector manages archived tasks using asynq Inspector.
type TaskInspector struct {
	inspector *asynq.Inspector
}

// NewTaskInspector constructs an asynq-backed task inspector.
func NewTaskInspector(redisAddr string) (*TaskInspector, error) {
	if strings.TrimSpace(redisAddr) == "" {
		return nil, fmt.Errorf("validate task inspector redis addr: %w", taskqueue.ErrUnavailable)
	}
	return &TaskInspector{inspector: asynq.NewInspector(asynq.RedisClientOpt{Addr: redisAddr})}, nil
}

// ListArchivedTasks returns archived tasks from one asynq queue.
func (i *TaskInspector) ListArchivedTasks(_ context.Context, queue string, limit int) ([]taskqueue.ArchivedTask, error) {
	if i == nil || i.inspector == nil {
		return nil, fmt.Errorf("validate task inspector: %w", taskqueue.ErrUnavailable)
	}
	items, err := i.inspector.ListArchivedTasks(queue, asynq.PageSize(limit))
	if err != nil {
		return nil, mapInspectorError("list archived tasks", err)
	}
	out := make([]taskqueue.ArchivedTask, 0, len(items))
	for _, item := range items {
		out = append(out, archivedTaskFromTaskInfo(item))
	}
	return out, nil
}

// GetArchivedTask returns one archived task summary from one asynq queue.
func (i *TaskInspector) GetArchivedTask(_ context.Context, queue string, id string) (taskqueue.ArchivedTask, error) {
	if i == nil || i.inspector == nil {
		return taskqueue.ArchivedTask{}, fmt.Errorf("validate task inspector: %w", taskqueue.ErrUnavailable)
	}
	item, err := i.inspector.GetTaskInfo(queue, id)
	if err != nil {
		return taskqueue.ArchivedTask{}, mapInspectorError("get archived task", err)
	}
	return archivedTaskFromTaskInfo(item), nil
}

// RetryArchivedTask replays one archived task by moving it back to pending.
func (i *TaskInspector) RetryArchivedTask(_ context.Context, queue string, id string) error {
	if i == nil || i.inspector == nil {
		return fmt.Errorf("validate task inspector: %w", taskqueue.ErrUnavailable)
	}
	if err := i.inspector.RunTask(queue, id); err != nil {
		return mapInspectorError("retry archived task", err)
	}
	return nil
}

func archivedTaskFromTaskInfo(item *asynq.TaskInfo) taskqueue.ArchivedTask {
	return taskqueue.ArchivedTask{
		ID: item.ID, Queue: item.Queue, Type: item.Type, Payload: payloadSummary(item.Payload),
		Retried: item.Retried, MaxRetry: item.MaxRetry, LastError: item.LastErr, LastFailedAt: item.LastFailedAt,
	}
}

// Close closes the underlying asynq inspector.
func (i *TaskInspector) Close() error {
	if i == nil || i.inspector == nil {
		return nil
	}
	if err := i.inspector.Close(); err != nil {
		return fmt.Errorf("close task inspector: %w", err)
	}
	return nil
}

func payloadSummary(raw []byte) map[string]string {
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return map[string]string{}
	}
	keys := make([]string, 0, len(payload))
	for key := range payload {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make(map[string]string)
	for _, key := range keys {
		if _, ok := safePayloadFields[key]; !ok {
			continue
		}
		switch value := payload[key].(type) {
		case string:
			out[key] = value
		case float64:
			out[key] = fmt.Sprintf("%.0f", value)
		case bool:
			out[key] = fmt.Sprintf("%t", value)
		}
	}
	return out
}

func mapInspectorError(action string, err error) error {
	if errors.Is(err, asynq.ErrQueueNotFound) || errors.Is(err, asynq.ErrTaskNotFound) {
		return fmt.Errorf("%s: %w", action, taskqueue.ErrNotFound)
	}
	return fmt.Errorf("%s: %w", action, err)
}
