package memory

import (
	"context"
	"fmt"
	"sort"

	"official-account-service/internal/domain/publish"
)

// CreatePublishRecord stores a tenant-scoped publish record.
func (s *Store) CreatePublishRecord(_ context.Context, tenantID string, record publish.Record) (publish.Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextRecordID++
	now := s.now()
	record.ID = s.nextRecordID
	record.TenantID = tenantID
	record.CreatedAt = now
	record.UpdatedAt = now
	s.publishRecords[record.ID] = record
	return record, nil
}

// GetPublishRecord returns one tenant-scoped publish record.
func (s *Store) GetPublishRecord(_ context.Context, tenantID string, id int64) (publish.Record, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	record, ok := s.publishRecords[id]
	if !ok || record.TenantID != tenantID {
		return publish.Record{}, fmt.Errorf("get publish record lookup: %w", publish.ErrNotFound)
	}
	return record, nil
}

// GetPublishRecordByPublishID returns one tenant-scoped publish record by WeChat publish id.
func (s *Store) GetPublishRecordByPublishID(_ context.Context, tenantID string, publishID string) (publish.Record, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, record := range s.publishRecords {
		if record.TenantID == tenantID && record.WeChatPublishID == publishID {
			return record, nil
		}
	}
	return publish.Record{}, fmt.Errorf("get publish record by publish id lookup: %w", publish.ErrNotFound)
}

// ListPublishRecords returns tenant-scoped publish records.
func (s *Store) ListPublishRecords(_ context.Context, tenantID string) ([]publish.Record, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]publish.Record, 0)
	for _, record := range s.publishRecords {
		if record.TenantID == tenantID {
			items = append(items, record)
		}
	}
	sortPublishRecords(items)
	return items, nil
}

// ListPublishRecordsByArticle returns publish records for an article.
func (s *Store) ListPublishRecordsByArticle(_ context.Context, tenantID string, articleID int64) ([]publish.Record, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]publish.Record, 0)
	for _, record := range s.publishRecords {
		if record.TenantID == tenantID && record.ArticleID == articleID {
			items = append(items, record)
		}
	}
	sortPublishRecords(items)
	return items, nil
}

// UpdatePublishRecordStatus updates a tenant-scoped publish record status.
func (s *Store) UpdatePublishRecordStatus(_ context.Context, tenantID string, record publish.Record) (publish.Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.publishRecords[record.ID]
	if !ok || current.TenantID != tenantID {
		return publish.Record{}, fmt.Errorf("update publish record lookup: %w", publish.ErrNotFound)
	}
	if current.ArticleID != record.ArticleID {
		return publish.Record{}, fmt.Errorf("validate publish record article id: %w", publish.ErrNotFound)
	}
	current.WeChatArticleID = record.WeChatArticleID
	current.Status = record.Status
	current.ErrorCode = record.ErrorCode
	current.ErrorMessage = record.ErrorMessage
	current.FinishedAt = record.FinishedAt
	current.UpdatedAt = s.now()
	s.publishRecords[current.ID] = current
	return current, nil
}

func sortPublishRecords(items []publish.Record) {
	sort.Slice(items, func(i, j int) bool {
		return items[i].ID < items[j].ID
	})
}
