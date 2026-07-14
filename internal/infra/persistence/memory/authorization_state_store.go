package memory

import (
	"context"
	"fmt"
	"time"

	"official-account-service/internal/domain/authorization"
)

// SaveAuthorizationState stores one authorization state digest.
func (s *Store) SaveAuthorizationState(_ context.Context, state authorization.AuthorizationState) (authorization.AuthorizationState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.authorizationStates[state.Digest]; exists {
		return authorization.AuthorizationState{}, fmt.Errorf("save authorization state duplicate: %w", authorization.ErrAuthorizationStateExists)
	}
	state.CreatedAt = s.now()
	s.authorizationStates[state.Digest] = state
	return state, nil
}

// ConsumeAuthorizationState atomically consumes one valid authorization state digest.
func (s *Store) ConsumeAuthorizationState(_ context.Context, digest string, consumedAt time.Time) (authorization.AuthorizationState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, exists := s.authorizationStates[digest]
	if !exists || !state.ConsumedAt.IsZero() || !state.ExpiresAt.After(consumedAt) {
		return authorization.AuthorizationState{}, fmt.Errorf("consume authorization state lookup: %w", authorization.ErrAuthorizationStateNotFound)
	}
	state.ConsumedAt = consumedAt
	s.authorizationStates[digest] = state
	return state, nil
}
