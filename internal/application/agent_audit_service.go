package application

import (
	"context"
	"fmt"
	"strings"

	"official-account-service/internal/domain/agentaudit"
)

const (
	defaultAuditLimit = 100
	maxAuditLimit     = 500
)

// AgentAuditService reads safe, tenant-scoped mutation metadata.
type AgentAuditService struct {
	audits agentaudit.Repository
}

// NewAgentAuditService constructs an AgentAuditService.
func NewAgentAuditService(audits agentaudit.Repository) *AgentAuditService {
	return &AgentAuditService{audits: audits}
}

// ListAgentAuditInput contains optional safe audit filters.
type ListAgentAuditInput struct {
	UserID        string
	AgentRecordID string
	Action        agentaudit.Action
	ResourceType  agentaudit.ResourceType
	Limit         int
}

// ListAgentAudit returns newest-first append-only audit metadata.
func (s *AgentAuditService) ListAgentAudit(ctx context.Context, input ListAgentAuditInput) ([]agentaudit.Entry, error) {
	if s == nil || s.audits == nil {
		return nil, fmt.Errorf("validate agent audit service: %w", ErrNotImplemented)
	}
	input.UserID = strings.TrimSpace(input.UserID)
	input.AgentRecordID = strings.TrimSpace(input.AgentRecordID)
	if input.UserID == "" || input.Limit < 0 || input.Limit > maxAuditLimit {
		return nil, fmt.Errorf("validate agent audit filter: %w", ErrInvalidInput)
	}
	if input.Limit == 0 {
		input.Limit = defaultAuditLimit
	}
	entries, err := s.audits.List(ctx, input.UserID, agentaudit.Filter{
		AgentRecordID: input.AgentRecordID, Action: input.Action, ResourceType: input.ResourceType, Limit: input.Limit,
	})
	if err != nil {
		return nil, fmt.Errorf("list agent audit: %w", err)
	}
	return entries, nil
}
