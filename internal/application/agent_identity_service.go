package application

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"official-account-service/internal/domain/identity"
)

const (
	defaultAgentExternalID = "default"
	defaultAgentName       = "Default Agent"
	defaultAgentPurpose    = "Legacy user-level MCP token"
)

// CreateAgentInput contains administrator-provided metadata for one Agent.
type CreateAgentInput struct {
	UserID  string
	AgentID string
	Name    string
	Purpose string
}

// UpdateAgentInput contains mutable Agent metadata.
type UpdateAgentInput struct {
	UserID  string
	ID      string
	Name    string
	Purpose string
	Status  identity.Status
}

// GeneratedAgentAPIToken returns one Agent credential in plaintext once.
type GeneratedAgentAPIToken struct {
	Token string
	Agent identity.Agent
}

// CreateAgent creates an active user-owned Agent with its first credential.
func (s *IdentityService) CreateAgent(ctx context.Context, input CreateAgentInput) (GeneratedAgentAPIToken, error) {
	if err := s.validateAgentDependencies(); err != nil {
		return GeneratedAgentAPIToken{}, err
	}
	input.UserID = strings.TrimSpace(input.UserID)
	input.AgentID = strings.TrimSpace(input.AgentID)
	input.Name = strings.TrimSpace(input.Name)
	input.Purpose = strings.TrimSpace(input.Purpose)
	if !validAgentMetadata(input.UserID, input.AgentID, input.Name, input.Purpose) {
		return GeneratedAgentAPIToken{}, fmt.Errorf("validate new agent: %w", ErrInvalidInput)
	}
	if _, err := s.getActiveAgentUser(ctx, input.UserID); err != nil {
		return GeneratedAgentAPIToken{}, err
	}
	recordID, err := s.newAgentID()
	if err != nil {
		return GeneratedAgentAPIToken{}, fmt.Errorf("generate agent id: %w", err)
	}
	raw, err := randomAPIToken()
	if err != nil {
		return GeneratedAgentAPIToken{}, fmt.Errorf("generate agent api token: %w", err)
	}
	agent, err := s.agents.CreateAgent(ctx, identity.Agent{
		ID: recordID, UserID: input.UserID, AgentID: input.AgentID, Name: input.Name, Purpose: input.Purpose,
		Status: identity.StatusActive, APITokenHash: apiTokenHash(raw), APITokenHint: apiTokenHint(raw),
	})
	if err != nil {
		return GeneratedAgentAPIToken{}, mapAgentServiceError("create agent", err)
	}
	return GeneratedAgentAPIToken{Token: raw, Agent: agent}, nil
}

// ListAgents returns all Agents owned by one user.
func (s *IdentityService) ListAgents(ctx context.Context, userID string) ([]identity.Agent, error) {
	if err := s.validateAgentDependencies(); err != nil {
		return nil, err
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, fmt.Errorf("validate list agents user: %w", ErrInvalidInput)
	}
	if _, err := s.getAgentUser(ctx, userID); err != nil {
		return nil, err
	}
	agents, err := s.agents.ListAgents(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list agents: %w", err)
	}
	return agents, nil
}

// GetAgent returns one user-scoped Agent.
func (s *IdentityService) GetAgent(ctx context.Context, userID string, id string) (identity.Agent, error) {
	if err := s.validateAgentDependencies(); err != nil {
		return identity.Agent{}, err
	}
	userID = strings.TrimSpace(userID)
	id = strings.TrimSpace(id)
	if userID == "" || id == "" {
		return identity.Agent{}, fmt.Errorf("validate get agent: %w", ErrInvalidInput)
	}
	agent, err := s.agents.GetAgent(ctx, userID, id)
	if err != nil {
		return identity.Agent{}, mapAgentServiceError("get agent", err)
	}
	return agent, nil
}

// UpdateAgent replaces mutable metadata without changing ownership or Agent ID.
func (s *IdentityService) UpdateAgent(ctx context.Context, input UpdateAgentInput) (identity.Agent, error) {
	if err := s.validateAgentDependencies(); err != nil {
		return identity.Agent{}, err
	}
	input.UserID = strings.TrimSpace(input.UserID)
	input.ID = strings.TrimSpace(input.ID)
	input.Name = strings.TrimSpace(input.Name)
	input.Purpose = strings.TrimSpace(input.Purpose)
	if input.UserID == "" || input.ID == "" || !validAgentNameAndPurpose(input.Name, input.Purpose) || !validIdentityStatus(input.Status) {
		return identity.Agent{}, fmt.Errorf("validate update agent: %w", ErrInvalidInput)
	}
	agent, err := s.agents.UpdateAgent(ctx, input.UserID, identity.Agent{
		ID: input.ID, Name: input.Name, Purpose: input.Purpose, Status: input.Status,
	})
	if err != nil {
		return identity.Agent{}, mapAgentServiceError("update agent", err)
	}
	return agent, nil
}

// RotateAgentAPIToken replaces only the selected active Agent credential.
func (s *IdentityService) RotateAgentAPIToken(ctx context.Context, userID string, id string) (GeneratedAgentAPIToken, error) {
	if err := s.validateAgentDependencies(); err != nil {
		return GeneratedAgentAPIToken{}, err
	}
	userID = strings.TrimSpace(userID)
	id = strings.TrimSpace(id)
	if userID == "" || id == "" {
		return GeneratedAgentAPIToken{}, fmt.Errorf("validate rotate agent api token: %w", ErrInvalidInput)
	}
	if _, err := s.getActiveAgentUser(ctx, userID); err != nil {
		return GeneratedAgentAPIToken{}, err
	}
	agent, err := s.GetAgent(ctx, userID, id)
	if err != nil {
		return GeneratedAgentAPIToken{}, err
	}
	if agent.Status != identity.StatusActive {
		return GeneratedAgentAPIToken{}, fmt.Errorf("validate rotate agent status: %w", ErrConflict)
	}
	raw, err := randomAPIToken()
	if err != nil {
		return GeneratedAgentAPIToken{}, fmt.Errorf("generate agent api token: %w", err)
	}
	agent, err = s.agents.SaveAgentAPIToken(ctx, userID, id, apiTokenHash(raw), apiTokenHint(raw))
	if err != nil {
		return GeneratedAgentAPIToken{}, mapAgentServiceError("rotate agent api token", err)
	}
	return GeneratedAgentAPIToken{Token: raw, Agent: agent}, nil
}

// RevokeAgentAPIToken invalidates only the selected Agent credential.
func (s *IdentityService) RevokeAgentAPIToken(ctx context.Context, userID string, id string) (identity.Agent, error) {
	if err := s.validateAgentDependencies(); err != nil {
		return identity.Agent{}, err
	}
	userID = strings.TrimSpace(userID)
	id = strings.TrimSpace(id)
	if userID == "" || id == "" {
		return identity.Agent{}, fmt.Errorf("validate revoke agent api token: %w", ErrInvalidInput)
	}
	agent, err := s.agents.RevokeAgentAPIToken(ctx, userID, id)
	if err != nil {
		return identity.Agent{}, mapAgentServiceError("revoke agent api token", err)
	}
	return agent, nil
}

func (s *IdentityService) validateAgentDependencies() error {
	if s == nil || s.users == nil || s.agents == nil || s.newAgentID == nil {
		return fmt.Errorf("validate agent identity service: %w", ErrNotImplemented)
	}
	return nil
}

func (s *IdentityService) getActiveAgentUser(ctx context.Context, userID string) (identity.User, error) {
	user, err := s.getAgentUser(ctx, userID)
	if err != nil {
		return identity.User{}, err
	}
	if user.Status != identity.StatusActive {
		return identity.User{}, fmt.Errorf("validate agent user status: %w", ErrConflict)
	}
	return user, nil
}

func (s *IdentityService) getAgentUser(ctx context.Context, userID string) (identity.User, error) {
	user, err := s.users.GetUser(ctx, userID)
	if errors.Is(err, identity.ErrNotFound) {
		return identity.User{}, fmt.Errorf("get agent user: %w", ErrNotFound)
	}
	if err != nil {
		return identity.User{}, fmt.Errorf("get agent user: %w", err)
	}
	return user, nil
}

func (s *IdentityService) defaultAgent(ctx context.Context, userID string) (identity.Agent, bool, error) {
	if s == nil || s.agents == nil {
		return identity.Agent{}, false, nil
	}
	agents, err := s.agents.ListAgents(ctx, userID)
	if err != nil {
		return identity.Agent{}, false, fmt.Errorf("list default agent: %w", err)
	}
	for _, agent := range agents {
		if agent.AgentID == defaultAgentExternalID {
			return agent, true, nil
		}
	}
	return identity.Agent{}, false, nil
}

func (s *IdentityService) userWithDefaultAgentToken(ctx context.Context, user identity.User) (identity.User, error) {
	agent, found, err := s.defaultAgent(ctx, user.ID)
	if err != nil {
		return identity.User{}, err
	}
	if !found {
		return user, nil
	}
	user.APITokenHash = agent.APITokenHash
	user.APITokenHint = agent.APITokenHint
	user.APITokenCreatedAt = agent.APITokenCreatedAt
	return user, nil
}

func validAgentMetadata(userID string, agentID string, name string, purpose string) bool {
	return userID != "" && len([]rune(agentID)) >= 1 && len([]rune(agentID)) <= 128 && validAgentNameAndPurpose(name, purpose)
}

func validAgentNameAndPurpose(name string, purpose string) bool {
	return len([]rune(name)) >= 1 && len([]rune(name)) <= 64 && len([]rune(purpose)) <= 200
}

func validIdentityStatus(status identity.Status) bool {
	return status == identity.StatusActive || status == identity.StatusDisabled
}

func mapAgentServiceError(action string, err error) error {
	if errors.Is(err, identity.ErrNotFound) {
		return fmt.Errorf("%s: %w", action, ErrNotFound)
	}
	if errors.Is(err, identity.ErrConflict) {
		return fmt.Errorf("%s: %w", action, ErrConflict)
	}
	return fmt.Errorf("%s: %w", action, err)
}
