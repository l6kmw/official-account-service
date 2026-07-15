package application

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"official-account-service/internal/domain/identity"
)

const invalidPasswordHash = "$2y$12$QnmwaSNnA6XLj0psgijIF.TFmph4HHbplvdjh/ddcWpGzjEz6FyK."

// IdentityService authenticates users who own isolated data spaces.
type IdentityService struct {
	users identity.Repository
}

// NewIdentityService constructs an identity service.
func NewIdentityService(users identity.Repository) *IdentityService {
	return &IdentityService{users: users}
}

// BootstrapUserInput activates the existing configured administrator without changing its data-space id.
type BootstrapUserInput struct {
	UserID       string
	Username     string
	PasswordHash string
}

// EnsureBootstrapUser creates or activates the configured platform administrator.
func (s *IdentityService) EnsureBootstrapUser(ctx context.Context, input BootstrapUserInput) (identity.User, error) {
	if s == nil || s.users == nil {
		return identity.User{}, fmt.Errorf("validate identity service: %w", ErrNotImplemented)
	}
	input.UserID = strings.TrimSpace(input.UserID)
	input.Username = strings.TrimSpace(input.Username)
	input.PasswordHash = strings.TrimSpace(input.PasswordHash)
	if input.UserID == "" || input.Username == "" || input.PasswordHash == "" {
		return identity.User{}, fmt.Errorf("validate bootstrap user: %w", ErrInvalidInput)
	}
	if _, err := bcrypt.Cost([]byte(input.PasswordHash)); err != nil {
		return identity.User{}, fmt.Errorf("validate bootstrap password hash: %w", ErrInvalidInput)
	}
	user, err := s.users.SaveUser(ctx, identity.User{
		ID: input.UserID, Username: input.Username, PasswordHash: input.PasswordHash,
		Role: identity.RoleAdmin, Status: identity.StatusActive,
	})
	if err != nil {
		return identity.User{}, fmt.Errorf("save bootstrap user: %w", err)
	}
	return user, nil
}

// Authenticate validates one active database user.
func (s *IdentityService) Authenticate(ctx context.Context, username string, password string) (identity.User, error) {
	if s == nil || s.users == nil {
		return identity.User{}, fmt.Errorf("validate identity service: %w", ErrNotImplemented)
	}
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return identity.User{}, ErrInvalidCredentials
	}
	user, err := s.users.GetUserByUsername(ctx, username)
	if errors.Is(err, identity.ErrNotFound) {
		_ = bcrypt.CompareHashAndPassword([]byte(invalidPasswordHash), []byte(password))
		return identity.User{}, ErrInvalidCredentials
	}
	if err != nil {
		return identity.User{}, fmt.Errorf("get login user: %w", err)
	}
	if user.Status != identity.StatusActive || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		return identity.User{}, ErrInvalidCredentials
	}
	return user, nil
}

// GetActiveUser validates a session subject against current database state.
func (s *IdentityService) GetActiveUser(ctx context.Context, userID string) (identity.User, error) {
	if s == nil || s.users == nil {
		return identity.User{}, fmt.Errorf("validate identity service: %w", ErrNotImplemented)
	}
	user, err := s.users.GetUser(ctx, strings.TrimSpace(userID))
	if errors.Is(err, identity.ErrNotFound) {
		return identity.User{}, ErrInvalidCredentials
	}
	if err != nil {
		return identity.User{}, fmt.Errorf("get active user: %w", err)
	}
	if user.Status != identity.StatusActive {
		return identity.User{}, ErrInvalidCredentials
	}
	return user, nil
}
