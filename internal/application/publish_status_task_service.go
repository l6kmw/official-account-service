package application

import (
	"context"
	"fmt"
	"strings"

	"official-account-service/internal/domain/publish"
)

// PublishStatusTaskService handles asynchronous publish status tasks.
type PublishStatusTaskService struct {
	publishes *PublishService
}

// NewPublishStatusTaskService constructs a PublishStatusTaskService.
func NewPublishStatusTaskService(publishes *PublishService) *PublishStatusTaskService {
	return &PublishStatusTaskService{publishes: publishes}
}

// HandleStatusSyncTask polls and syncs one publish record status.
func (s *PublishStatusTaskService) HandleStatusSyncTask(ctx context.Context, task publish.StatusSyncTask) error {
	if s == nil || s.publishes == nil {
		return fmt.Errorf("validate publish status task service: %w", ErrNotImplemented)
	}
	if strings.TrimSpace(task.TenantID) == "" {
		return fmt.Errorf("validate publish status task tenant id: %w", ErrInvalidInput)
	}
	if task.PublishRecordID <= 0 {
		return fmt.Errorf("validate publish status task record id: %w", ErrInvalidInput)
	}
	record, err := s.publishes.SyncPublishStatus(ctx, SyncPublishStatusInput{
		TenantID: task.TenantID, ID: task.PublishRecordID,
	})
	if err != nil {
		return fmt.Errorf("sync publish status task: %w", err)
	}
	if record.Status == publish.StatusPublishing {
		return fmt.Errorf("sync publish status task still processing: %w", publish.ErrPublishStillProcessing)
	}
	return nil
}

func (s *PublishService) enqueueStatusSync(ctx context.Context, record publish.Record) error {
	if s.statusSync == nil {
		return nil
	}
	if strings.TrimSpace(record.TenantID) == "" || record.ID <= 0 {
		return fmt.Errorf("validate publish status sync task: %w", ErrInvalidInput)
	}
	if err := s.statusSync.EnqueueStatusSync(ctx, publish.StatusSyncTask{
		TenantID: record.TenantID, PublishRecordID: record.ID,
	}); err != nil {
		return fmt.Errorf("enqueue publish status sync: %w", err)
	}
	return nil
}
