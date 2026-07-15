package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"official-account-service/internal/domain/identity"
)

var _ identity.AgentRepository = (*Store)(nil)

// CreateAgent stores one independently authenticated agent under an existing user.
func (s *Store) CreateAgent(ctx context.Context, agent identity.Agent) (identity.Agent, error) {
	row := s.db.QueryRowContext(ctx, `
		INSERT INTO app_agent (
			id, user_id, agent_id, name, purpose, status,
			api_token_hash, api_token_hint, api_token_created_at, last_used_at
		)
		VALUES (
			$1, $2, $3, $4, $5, $6,
			NULLIF($7, ''), CASE WHEN NULLIF($7, '') IS NULL THEN '' ELSE $8 END,
			CASE WHEN NULLIF($7, '') IS NULL THEN NULL ELSE COALESCE($9, NOW()) END,
			$10
		)
		RETURNING id, user_id, agent_id, name, purpose, status, COALESCE(api_token_hash, ''),
		          api_token_hint, api_token_created_at, last_used_at, created_at, updated_at`,
		agent.ID, agent.UserID, agent.AgentID, agent.Name, agent.Purpose, agent.Status,
		agent.APITokenHash, agent.APITokenHint, agent.APITokenCreatedAt, agent.LastUsedAt)
	created, err := scanAgent(row)
	if err != nil {
		return identity.Agent{}, mapAgentError("create agent", err)
	}
	return created, nil
}

// GetAgent returns one user-scoped agent by its internal id.
func (s *Store) GetAgent(ctx context.Context, userID string, id string) (identity.Agent, error) {
	agent, err := scanAgent(s.db.QueryRowContext(ctx, agentSelectSQL+` WHERE user_id = $1 AND id = $2`, userID, id))
	if err != nil {
		return identity.Agent{}, mapAgentError("get agent", err)
	}
	return agent, nil
}

// GetAgentByAPITokenHash returns the agent owning one non-reversible token digest.
func (s *Store) GetAgentByAPITokenHash(ctx context.Context, tokenHash string) (identity.Agent, error) {
	agent, err := scanAgent(s.db.QueryRowContext(ctx, agentSelectSQL+` WHERE api_token_hash = NULLIF($1, '')`, tokenHash))
	if err != nil {
		return identity.Agent{}, mapAgentError("get agent by api token", err)
	}
	return agent, nil
}

// ListAgents returns all agents owned by one user.
func (s *Store) ListAgents(ctx context.Context, userID string) ([]identity.Agent, error) {
	rows, err := s.db.QueryContext(ctx, agentSelectSQL+` WHERE user_id = $1 ORDER BY name, id`, userID)
	if err != nil {
		return nil, fmt.Errorf("list agents query: %w", err)
	}
	defer rows.Close()
	agents := make([]identity.Agent, 0)
	for rows.Next() {
		agent, err := scanAgent(rows)
		if err != nil {
			return nil, fmt.Errorf("list agents scan: %w", err)
		}
		agents = append(agents, agent)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list agents rows: %w", err)
	}
	return agents, nil
}

// UpdateAgent updates mutable metadata while preserving ownership and credentials.
func (s *Store) UpdateAgent(ctx context.Context, userID string, agent identity.Agent) (identity.Agent, error) {
	row := s.db.QueryRowContext(ctx, `
		UPDATE app_agent
		SET name = $3, purpose = $4, status = $5, updated_at = NOW()
		WHERE user_id = $1 AND id = $2
		RETURNING id, user_id, agent_id, name, purpose, status, COALESCE(api_token_hash, ''),
		          api_token_hint, api_token_created_at, last_used_at, created_at, updated_at`,
		userID, agent.ID, agent.Name, agent.Purpose, agent.Status)
	updated, err := scanAgent(row)
	if err != nil {
		return identity.Agent{}, mapAgentError("update agent", err)
	}
	return updated, nil
}

// SaveAgentAPIToken rotates one agent credential without affecting sibling agents.
func (s *Store) SaveAgentAPIToken(ctx context.Context, userID string, id string, tokenHash string, tokenHint string) (identity.Agent, error) {
	row := s.db.QueryRowContext(ctx, `
		UPDATE app_agent
		SET api_token_hash = NULLIF($3, ''),
		    api_token_hint = CASE WHEN NULLIF($3, '') IS NULL THEN '' ELSE $4 END,
		    api_token_created_at = CASE WHEN NULLIF($3, '') IS NULL THEN NULL ELSE NOW() END,
		    updated_at = NOW()
		WHERE user_id = $1 AND id = $2
		RETURNING id, user_id, agent_id, name, purpose, status, COALESCE(api_token_hash, ''),
		          api_token_hint, api_token_created_at, last_used_at, created_at, updated_at`,
		userID, id, tokenHash, tokenHint)
	updated, err := scanAgent(row)
	if err != nil {
		return identity.Agent{}, mapAgentError("save agent api token", err)
	}
	return updated, nil
}

// RevokeAgentAPIToken invalidates one agent credential and preserves the agent record.
func (s *Store) RevokeAgentAPIToken(ctx context.Context, userID string, id string) (identity.Agent, error) {
	row := s.db.QueryRowContext(ctx, `
		UPDATE app_agent
		SET api_token_hash = NULL, api_token_hint = '', api_token_created_at = NULL, updated_at = NOW()
		WHERE user_id = $1 AND id = $2
		RETURNING id, user_id, agent_id, name, purpose, status, COALESCE(api_token_hash, ''),
		          api_token_hint, api_token_created_at, last_used_at, created_at, updated_at`,
		userID, id)
	updated, err := scanAgent(row)
	if err != nil {
		return identity.Agent{}, mapAgentError("revoke agent api token", err)
	}
	return updated, nil
}

// MarkAgentUsed stores the latest successful authentication time for one agent.
func (s *Store) MarkAgentUsed(ctx context.Context, userID string, id string, usedAt time.Time) (identity.Agent, error) {
	row := s.db.QueryRowContext(ctx, `
		UPDATE app_agent
		SET last_used_at = GREATEST(COALESCE(last_used_at, $3), $3),
		    updated_at = CASE WHEN last_used_at IS NULL OR last_used_at < $3 THEN NOW() ELSE updated_at END
		WHERE user_id = $1 AND id = $2
		RETURNING id, user_id, agent_id, name, purpose, status, COALESCE(api_token_hash, ''),
		          api_token_hint, api_token_created_at, last_used_at, created_at, updated_at`,
		userID, id, usedAt)
	updated, err := scanAgent(row)
	if err != nil {
		return identity.Agent{}, mapAgentError("mark agent used", err)
	}
	return updated, nil
}

const agentSelectSQL = `
	SELECT id, user_id, agent_id, name, purpose, status, COALESCE(api_token_hash, ''),
	       api_token_hint, api_token_created_at, last_used_at, created_at, updated_at
	FROM app_agent`

type agentScanner interface {
	Scan(dest ...any) error
}

func scanAgent(scanner agentScanner) (identity.Agent, error) {
	var agent identity.Agent
	if err := scanner.Scan(
		&agent.ID, &agent.UserID, &agent.AgentID, &agent.Name, &agent.Purpose, &agent.Status,
		&agent.APITokenHash, &agent.APITokenHint, &agent.APITokenCreatedAt, &agent.LastUsedAt,
		&agent.CreatedAt, &agent.UpdatedAt,
	); err != nil {
		return identity.Agent{}, err
	}
	return agent, nil
}

func mapAgentError(action string, err error) error {
	if errors.Is(err, sql.ErrNoRows) || isAgentForeignKeyViolation(err) {
		return fmt.Errorf("%s: %w", action, identity.ErrNotFound)
	}
	if isUniqueViolation(err) {
		return fmt.Errorf("%s: %w", action, identity.ErrConflict)
	}
	return fmt.Errorf("%s: %w", action, err)
}

func isAgentForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}
