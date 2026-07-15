package application

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"official-account-service/internal/domain/identity"
)

const invalidPasswordHash = "$2y$12$QnmwaSNnA6XLj0psgijIF.TFmph4HHbplvdjh/ddcWpGzjEz6FyK."

// IdentityService authenticates users who own isolated data spaces.
type IdentityService struct {
	users     identity.Repository
	newUserID func() (string, error)
}

// NewIdentityService constructs an identity service.
func NewIdentityService(users identity.Repository) *IdentityService {
	return &IdentityService{users: users, newUserID: randomUserID}
}

// CreateUserInput contains administrator-provided credentials for a new isolated user.
type CreateUserInput struct {
	Username string
	Password string
}

// CreateUser creates an active non-admin user with an independent data-space id.
func (s *IdentityService) CreateUser(ctx context.Context, input CreateUserInput) (identity.User, error) {
	if s == nil || s.users == nil || s.newUserID == nil {
		return identity.User{}, fmt.Errorf("validate identity service: %w", ErrNotImplemented)
	}
	username := strings.TrimSpace(input.Username)
	if len([]rune(username)) < 3 || len([]rune(username)) > 64 || len(input.Password) < 12 || len([]byte(input.Password)) > 72 {
		return identity.User{}, fmt.Errorf("validate new user: %w", ErrInvalidInput)
	}
	userID, err := s.newUserID()
	if err != nil {
		return identity.User{}, fmt.Errorf("generate user id: %w", err)
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(input.Password), 12)
	if err != nil {
		return identity.User{}, fmt.Errorf("hash user password: %w", err)
	}
	user, err := s.users.SaveUser(ctx, identity.User{
		ID: userID, Username: username, PasswordHash: string(passwordHash), Role: identity.RoleUser, Status: identity.StatusActive,
	})
	if errors.Is(err, identity.ErrConflict) {
		return identity.User{}, fmt.Errorf("create user: %w", ErrConflict)
	}
	if err != nil {
		return identity.User{}, fmt.Errorf("create user: %w", err)
	}
	return user, nil
}

// ListUsers returns platform users for administrator management.
func (s *IdentityService) ListUsers(ctx context.Context) ([]identity.User, error) {
	if s == nil || s.users == nil {
		return nil, fmt.Errorf("validate identity service: %w", ErrNotImplemented)
	}
	users, err := s.users.ListUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	return users, nil
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

func randomUserID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return "usr_" + base64.RawURLEncoding.EncodeToString(raw[:]), nil
}
