package application

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"official-account-service/internal/domain/identity"
)

// ActorType identifies how one authenticated request entered the service.
type ActorType string

const (
	// ActorTypeAgentToken identifies a user-owned Agent bearer token.
	ActorTypeAgentToken ActorType = "agent_token"
	// ActorTypeLegacyUserToken identifies a pre-migration user bearer token.
	ActorTypeLegacyUserToken ActorType = "legacy_user_token"
	// ActorTypeBrowserSession identifies a signed browser login session.
	ActorTypeBrowserSession ActorType = "browser_session"
	// ActorTypeAdminAPIKey identifies the configured legacy administrator key.
	ActorTypeAdminAPIKey ActorType = "admin_api_key"
)

// Principal contains the server-resolved user and optional Agent identity.
type Principal struct {
	User      identity.User
	ActorType ActorType
	Agent     *identity.Agent
}

// GenerateAPIToken rotates the compatibility default Agent credential.
func (s *IdentityService) GenerateAPIToken(ctx context.Context, userID string) (GeneratedAPIToken, error) {
	if s == nil || s.users == nil {
		return GeneratedAPIToken{}, fmt.Errorf("validate identity service: %w", ErrNotImplemented)
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return GeneratedAPIToken{}, fmt.Errorf("validate api token user: %w", ErrInvalidInput)
	}
	user, err := s.getActiveAgentUser(ctx, userID)
	if err != nil {
		return GeneratedAPIToken{}, err
	}
	if s.agents == nil {
		return s.generateLegacyUserAPIToken(ctx, user)
	}
	defaultAgent, found, err := s.defaultAgent(ctx, userID)
	if err != nil {
		return GeneratedAPIToken{}, err
	}
	if !found {
		generated, err := s.CreateAgent(ctx, CreateAgentInput{
			UserID: userID, AgentID: defaultAgentExternalID, Name: defaultAgentName, Purpose: defaultAgentPurpose,
		})
		if err != nil {
			return GeneratedAPIToken{}, err
		}
		agent := generated.Agent
		user = userWithAgentToken(user, agent)
		return GeneratedAPIToken{Token: generated.Token, User: user, Agent: &agent}, nil
	}
	if defaultAgent.Status != identity.StatusActive {
		return GeneratedAPIToken{}, fmt.Errorf("validate default agent status: %w", ErrConflict)
	}
	generated, err := s.RotateAgentAPIToken(ctx, userID, defaultAgent.ID)
	if err != nil {
		return GeneratedAPIToken{}, err
	}
	agent := generated.Agent
	user = userWithAgentToken(user, agent)
	return GeneratedAPIToken{Token: generated.Token, User: user, Agent: &agent}, nil
}

// RevokeAPIToken revokes the compatibility default Agent credential.
func (s *IdentityService) RevokeAPIToken(ctx context.Context, userID string) (identity.User, error) {
	if s == nil || s.users == nil {
		return identity.User{}, fmt.Errorf("validate identity service: %w", ErrNotImplemented)
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return identity.User{}, fmt.Errorf("validate api token user: %w", ErrInvalidInput)
	}
	user, err := s.getAgentUser(ctx, userID)
	if err != nil {
		return identity.User{}, err
	}
	if s.agents == nil {
		user, err = s.users.RevokeUserAPIToken(ctx, userID)
		if err != nil {
			return identity.User{}, mapAgentServiceError("revoke legacy api token", err)
		}
		return user, nil
	}
	defaultAgent, found, err := s.defaultAgent(ctx, userID)
	if err != nil {
		return identity.User{}, err
	}
	if !found {
		user, err = s.users.RevokeUserAPIToken(ctx, userID)
		if err != nil {
			return identity.User{}, mapAgentServiceError("revoke legacy api token", err)
		}
		return user, nil
	}
	agent, err := s.RevokeAgentAPIToken(ctx, userID, defaultAgent.ID)
	if err != nil {
		return identity.User{}, err
	}
	return userWithAgentToken(user, agent), nil
}

// AuthenticatePrincipal resolves an active user and Agent from a high-entropy API/MCP token.
func (s *IdentityService) AuthenticatePrincipal(ctx context.Context, token string) (Principal, error) {
	if s == nil || s.users == nil {
		return Principal{}, fmt.Errorf("validate identity service: %w", ErrNotImplemented)
	}
	token = strings.TrimSpace(token)
	if !strings.HasPrefix(token, "oat_") || len(token) < 40 {
		return Principal{}, ErrInvalidCredentials
	}
	tokenHash := apiTokenHash(token)
	if s.agents != nil {
		agent, err := s.agents.GetAgentByAPITokenHash(ctx, tokenHash)
		if err == nil {
			return s.authenticateAgentPrincipal(ctx, agent)
		}
		if !errors.Is(err, identity.ErrNotFound) {
			return Principal{}, fmt.Errorf("get api token agent: %w", err)
		}
	}
	user, err := s.users.GetUserByAPITokenHash(ctx, tokenHash)
	if errors.Is(err, identity.ErrNotFound) {
		return Principal{}, ErrInvalidCredentials
	}
	if err != nil {
		return Principal{}, fmt.Errorf("get api token user: %w", err)
	}
	if user.Status != identity.StatusActive {
		return Principal{}, ErrInvalidCredentials
	}
	if s.agents != nil {
		_, found, err := s.defaultAgent(ctx, user.ID)
		if err != nil {
			return Principal{}, err
		}
		if found {
			return Principal{}, ErrInvalidCredentials
		}
	}
	return Principal{User: user, ActorType: ActorTypeLegacyUserToken}, nil
}

// AuthenticateAPIToken preserves the legacy user-only authentication contract.
func (s *IdentityService) AuthenticateAPIToken(ctx context.Context, token string) (identity.User, error) {
	principal, err := s.AuthenticatePrincipal(ctx, token)
	if err != nil {
		return identity.User{}, err
	}
	return principal.User, nil
}

func (s *IdentityService) authenticateAgentPrincipal(ctx context.Context, agent identity.Agent) (Principal, error) {
	if agent.Status != identity.StatusActive {
		return Principal{}, ErrInvalidCredentials
	}
	user, err := s.users.GetUser(ctx, agent.UserID)
	if errors.Is(err, identity.ErrNotFound) {
		return Principal{}, ErrInvalidCredentials
	}
	if err != nil {
		return Principal{}, fmt.Errorf("get api token agent user: %w", err)
	}
	if user.Status != identity.StatusActive {
		return Principal{}, ErrInvalidCredentials
	}
	if s.now == nil {
		return Principal{}, fmt.Errorf("validate api token clock: %w", ErrNotImplemented)
	}
	agent, err = s.agents.MarkAgentUsed(ctx, user.ID, agent.ID, s.now())
	if err != nil {
		return Principal{}, fmt.Errorf("mark api token agent used: %w", err)
	}
	return Principal{User: user, ActorType: ActorTypeAgentToken, Agent: &agent}, nil
}

func (s *IdentityService) generateLegacyUserAPIToken(ctx context.Context, user identity.User) (GeneratedAPIToken, error) {
	raw, err := randomAPIToken()
	if err != nil {
		return GeneratedAPIToken{}, fmt.Errorf("generate api token: %w", err)
	}
	user, err = s.users.SaveUserAPIToken(ctx, user.ID, apiTokenHash(raw), apiTokenHint(raw))
	if err != nil {
		return GeneratedAPIToken{}, fmt.Errorf("save api token: %w", err)
	}
	return GeneratedAPIToken{Token: raw, User: user}, nil
}

func userWithAgentToken(user identity.User, agent identity.Agent) identity.User {
	user.APITokenHash = agent.APITokenHash
	user.APITokenHint = agent.APITokenHint
	user.APITokenCreatedAt = agent.APITokenCreatedAt
	return user
}
