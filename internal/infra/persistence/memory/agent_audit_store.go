package memory

import (
	"context"
	"fmt"
	"strings"

	"official-account-service/internal/domain/agentaudit"
)

var _ agentaudit.Repository = (*Store)(nil)

// Append stores one immutable safe audit entry.
func (s *Store) Append(_ context.Context, entry agentaudit.Entry) (agentaudit.Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.appendAuditLocked(entry)
}

func (s *Store) appendAuditLocked(entry agentaudit.Entry) (agentaudit.Entry, error) {
	if strings.TrimSpace(entry.UserID) == "" || entry.Action == "" || entry.ResourceType == "" || strings.TrimSpace(entry.ResourceID) == "" {
		return agentaudit.Entry{}, fmt.Errorf("validate agent audit entry")
	}
	s.nextAuditID++
	entry.ID = s.nextAuditID
	entry.CreatedAt = s.now()
	s.auditEntries = append(s.auditEntries, entry)
	return entry, nil
}

// ListAuditEntries returns newest-first audit metadata scoped to one user.
func (s *Store) ListAuditEntries(_ context.Context, userID string, filter agentaudit.Filter) ([]agentaudit.Entry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]agentaudit.Entry, 0)
	for index := len(s.auditEntries) - 1; index >= 0; index-- {
		entry := s.auditEntries[index]
		if entry.UserID != userID || filter.AgentRecordID != "" && entry.AgentRecordID != filter.AgentRecordID || filter.Action != "" && entry.Action != filter.Action || filter.ResourceType != "" && entry.ResourceType != filter.ResourceType {
			continue
		}
		items = append(items, entry)
		if filter.Limit > 0 && len(items) == filter.Limit {
			break
		}
	}
	return items, nil
}
