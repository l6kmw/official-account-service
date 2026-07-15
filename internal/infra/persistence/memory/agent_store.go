package memory

import (
	"context"
	"fmt"
	"sort"
	"time"

	"official-account-service/internal/domain/identity"
)

var _ identity.AgentRepository = (*Store)(nil)

// CreateAgent stores one independently authenticated agent under an existing user.
func (s *Store) CreateAgent(_ context.Context, agent identity.Agent) (identity.Agent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users[agent.UserID]; !ok {
		return identity.Agent{}, fmt.Errorf("create agent user lookup: %w", identity.ErrNotFound)
	}
	if _, ok := s.agents[agent.ID]; ok {
		return identity.Agent{}, fmt.Errorf("create agent id: %w", identity.ErrConflict)
	}
	for _, current := range s.agents {
		if current.UserID == agent.UserID && current.AgentID == agent.AgentID {
			return identity.Agent{}, fmt.Errorf("create agent external id: %w", identity.ErrConflict)
		}
		if agent.APITokenHash != "" && current.APITokenHash == agent.APITokenHash {
			return identity.Agent{}, fmt.Errorf("create agent api token hash: %w", identity.ErrConflict)
		}
	}
	now := s.now()
	agent.CreatedAt = now
	agent.UpdatedAt = now
	if agent.APITokenHash != "" {
		if agent.APITokenCreatedAt == nil {
			agent.APITokenCreatedAt = &now
		}
	} else {
		agent.APITokenHint = ""
		agent.APITokenCreatedAt = nil
	}
	s.agents[agent.ID] = agent
	return agent, nil
}

// GetAgent returns one user-scoped agent by its internal id.
func (s *Store) GetAgent(_ context.Context, userID string, id string) (identity.Agent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	agent, ok := s.agents[id]
	if !ok || agent.UserID != userID {
		return identity.Agent{}, fmt.Errorf("get agent lookup: %w", identity.ErrNotFound)
	}
	return agent, nil
}

// GetAgentByAPITokenHash returns the agent owning one non-reversible token digest.
func (s *Store) GetAgentByAPITokenHash(_ context.Context, tokenHash string) (identity.Agent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, agent := range s.agents {
		if tokenHash != "" && agent.APITokenHash == tokenHash {
			return agent, nil
		}
	}
	return identity.Agent{}, fmt.Errorf("get agent by api token lookup: %w", identity.ErrNotFound)
}

// ListAgents returns all agents owned by one user.
func (s *Store) ListAgents(_ context.Context, userID string) ([]identity.Agent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	agents := make([]identity.Agent, 0)
	for _, agent := range s.agents {
		if agent.UserID == userID {
			agents = append(agents, agent)
		}
	}
	sort.Slice(agents, func(i, j int) bool {
		if agents[i].Name == agents[j].Name {
			return agents[i].ID < agents[j].ID
		}
		return agents[i].Name < agents[j].Name
	})
	return agents, nil
}

// UpdateAgent updates mutable metadata while preserving ownership and credentials.
func (s *Store) UpdateAgent(_ context.Context, userID string, agent identity.Agent) (identity.Agent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.agents[agent.ID]
	if !ok || current.UserID != userID {
		return identity.Agent{}, fmt.Errorf("update agent lookup: %w", identity.ErrNotFound)
	}
	current.Name = agent.Name
	current.Purpose = agent.Purpose
	current.Status = agent.Status
	current.UpdatedAt = s.now()
	s.agents[current.ID] = current
	return current, nil
}

// SaveAgentAPIToken rotates one agent credential without affecting sibling agents.
func (s *Store) SaveAgentAPIToken(_ context.Context, userID string, id string, tokenHash string, tokenHint string) (identity.Agent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	agent, ok := s.agents[id]
	if !ok || agent.UserID != userID {
		return identity.Agent{}, fmt.Errorf("save agent api token lookup: %w", identity.ErrNotFound)
	}
	for currentID, current := range s.agents {
		if currentID != id && tokenHash != "" && current.APITokenHash == tokenHash {
			return identity.Agent{}, fmt.Errorf("save agent api token hash: %w", identity.ErrConflict)
		}
	}
	now := s.now()
	agent.APITokenHash = tokenHash
	if tokenHash == "" {
		agent.APITokenHint = ""
		agent.APITokenCreatedAt = nil
	} else {
		agent.APITokenHint = tokenHint
		agent.APITokenCreatedAt = &now
	}
	agent.UpdatedAt = now
	s.agents[id] = agent
	return agent, nil
}

// RevokeAgentAPIToken invalidates one agent credential and preserves the agent record.
func (s *Store) RevokeAgentAPIToken(_ context.Context, userID string, id string) (identity.Agent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	agent, ok := s.agents[id]
	if !ok || agent.UserID != userID {
		return identity.Agent{}, fmt.Errorf("revoke agent api token lookup: %w", identity.ErrNotFound)
	}
	agent.APITokenHash = ""
	agent.APITokenHint = ""
	agent.APITokenCreatedAt = nil
	agent.UpdatedAt = s.now()
	s.agents[id] = agent
	return agent, nil
}

// MarkAgentUsed stores the latest successful authentication time for one agent.
func (s *Store) MarkAgentUsed(_ context.Context, userID string, id string, usedAt time.Time) (identity.Agent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	agent, ok := s.agents[id]
	if !ok || agent.UserID != userID {
		return identity.Agent{}, fmt.Errorf("mark agent used lookup: %w", identity.ErrNotFound)
	}
	if agent.LastUsedAt == nil || usedAt.After(*agent.LastUsedAt) {
		agent.LastUsedAt = &usedAt
		agent.UpdatedAt = s.now()
	}
	s.agents[id] = agent
	return agent, nil
}
