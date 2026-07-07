package application

import (
	"context"
	"fmt"
	"strings"

	"official-account-service/internal/domain/authorization"
)

// TokenRefreshTaskService handles asynchronous authorizer token refresh tasks.
type TokenRefreshTaskService struct {
	tokens *TokenService
}

// NewTokenRefreshTaskService constructs a TokenRefreshTaskService.
func NewTokenRefreshTaskService(tokens *TokenService) *TokenRefreshTaskService {
	return &TokenRefreshTaskService{tokens: tokens}
}

// HandleTokenRefreshTask refreshes one tenant-scoped account token.
func (s *TokenRefreshTaskService) HandleTokenRefreshTask(ctx context.Context, task authorization.TokenRefreshTask) error {
	if s == nil || s.tokens == nil {
		return fmt.Errorf("validate token refresh task service: %w", ErrNotImplemented)
	}
	if strings.TrimSpace(task.TenantID) == "" {
		return fmt.Errorf("validate token refresh task tenant id: %w", ErrInvalidInput)
	}
	if task.AccountID <= 0 {
		return fmt.Errorf("validate token refresh task account id: %w", ErrInvalidInput)
	}
	if strings.TrimSpace(task.ComponentAppID) == "" {
		return fmt.Errorf("validate token refresh task component app id: %w", ErrInvalidInput)
	}
	if _, err := s.tokens.RefreshAuthorizerAccessToken(ctx, RefreshAuthorizerAccessTokenInput{
		TenantID: task.TenantID, AccountID: task.AccountID, ComponentAppID: task.ComponentAppID,
	}); err != nil {
		return fmt.Errorf("refresh authorizer token task: %w", err)
	}
	return nil
}
