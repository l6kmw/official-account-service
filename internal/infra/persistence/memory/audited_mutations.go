package memory

import (
	"context"
	"fmt"
	"strconv"

	"official-account-service/internal/domain/agentaudit"
	"official-account-service/internal/domain/article"
	"official-account-service/internal/domain/publish"
)

var _ article.AuditedMutationRepository = (*Store)(nil)
var _ publish.AuditedCreateRepository = (*Store)(nil)

// CreateWithAudit creates an article and audit entry while holding one store lock.
func (s *Store) CreateWithAudit(_ context.Context, tenantID string, draft article.Article, audit agentaudit.Entry) (article.Article, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	previousArticleID := s.nextArticleID
	s.nextArticleID++
	now := s.now()
	draft.ID = s.nextArticleID
	draft.TenantID = tenantID
	draft.Version = 1
	draft.CreatedAt = now
	draft.UpdatedAt = now
	s.articles[draft.ID] = draft

	audit.ResourceID = strconv.FormatInt(draft.ID, 10)
	if _, err := s.appendAuditLocked(audit); err != nil {
		delete(s.articles, draft.ID)
		s.nextArticleID = previousArticleID
		return article.Article{}, fmt.Errorf("create article audit: %w", err)
	}
	return draft, nil
}

// UpdateWithAudit updates an article and audit entry while holding one store lock.
func (s *Store) UpdateWithAudit(_ context.Context, tenantID string, draft article.Article, audit agentaudit.Entry) (article.Article, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	current, ok := s.articles[draft.ID]
	if !ok || current.TenantID != tenantID {
		return article.Article{}, fmt.Errorf("update article lookup: %w", article.ErrNotFound)
	}
	if draft.Version != current.Version {
		return article.Article{}, fmt.Errorf("update article version: %w", article.ErrVersionConflict)
	}
	draft.TenantID = tenantID
	draft.CreatedAt = current.CreatedAt
	draft.CreatedByAgentID = current.CreatedByAgentID
	draft.Version = current.Version + 1
	draft.UpdatedAt = s.now()
	s.articles[draft.ID] = draft

	audit.ResourceID = strconv.FormatInt(draft.ID, 10)
	if _, err := s.appendAuditLocked(audit); err != nil {
		s.articles[draft.ID] = current
		return article.Article{}, fmt.Errorf("update article audit: %w", err)
	}
	return draft, nil
}

// DeleteWithAudit deletes an article and appends its audit entry under one store lock.
func (s *Store) DeleteWithAudit(_ context.Context, tenantID string, id int64, audit agentaudit.Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	draft, ok := s.articles[id]
	if !ok || draft.TenantID != tenantID {
		return fmt.Errorf("delete article lookup: %w", article.ErrNotFound)
	}
	delete(s.articles, id)
	audit.ResourceID = strconv.FormatInt(id, 10)
	if _, err := s.appendAuditLocked(audit); err != nil {
		s.articles[id] = draft
		return fmt.Errorf("delete article audit: %w", err)
	}
	return nil
}

// CreatePublishRecordWithAudit creates one publish intent and audit entry under one store lock.
func (s *Store) CreatePublishRecordWithAudit(_ context.Context, tenantID string, record publish.Record, audit agentaudit.Entry) (publish.Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if record.Status == publish.StatusPublishing {
		for _, current := range s.publishRecords {
			if current.TenantID == tenantID && current.ArticleID == record.ArticleID && current.Status == publish.StatusPublishing {
				return publish.Record{}, fmt.Errorf("create publish record in progress: %w", publish.ErrPublishInProgress)
			}
		}
	}
	previousRecordID := s.nextRecordID
	s.nextRecordID++
	now := s.now()
	record.ID = s.nextRecordID
	record.TenantID = tenantID
	if draft, ok := s.articles[record.ArticleID]; ok && draft.TenantID == tenantID {
		record.ArticleCreatedByAgentID = draft.CreatedByAgentID
	}
	record.CreatedAt = now
	record.UpdatedAt = now
	s.publishRecords[record.ID] = record

	audit.ResourceID = strconv.FormatInt(record.ArticleID, 10)
	if _, err := s.appendAuditLocked(audit); err != nil {
		delete(s.publishRecords, record.ID)
		s.nextRecordID = previousRecordID
		return publish.Record{}, fmt.Errorf("create publish record audit: %w", err)
	}
	return s.withArticleAgent(record), nil
}
