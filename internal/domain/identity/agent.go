package identity

import (
	"context"
	"time"
)

// Agent is one independently authenticated actor inside a user's data space.
type Agent struct {
	ID                string
	UserID            string
	AgentID           string
	Name              string
	Purpose           string
	Status            Status
	APITokenHash      string
	APITokenHint      string
	APITokenCreatedAt *time.Time
	LastUsedAt        *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// AgentRepository persists user-owned agents and their independent credentials.
type AgentRepository interface {
	CreateAgent(ctx context.Context, agent Agent) (Agent, error)
	GetAgent(ctx context.Context, userID string, id string) (Agent, error)
	GetAgentByAPITokenHash(ctx context.Context, tokenHash string) (Agent, error)
	ListAgents(ctx context.Context, userID string) ([]Agent, error)
	UpdateAgent(ctx context.Context, userID string, agent Agent) (Agent, error)
	SaveAgentAPIToken(ctx context.Context, userID string, id string, tokenHash string, tokenHint string) (Agent, error)
	RevokeAgentAPIToken(ctx context.Context, userID string, id string) (Agent, error)
	MarkAgentUsed(ctx context.Context, userID string, id string, usedAt time.Time) (Agent, error)
}
