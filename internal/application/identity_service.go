package application

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
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

// GeneratedAPIToken is returned once when an administrator rotates a user's API/MCP credential.
type GeneratedAPIToken struct {
	Token string
	User  identity.User
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

// GenerateAPIToken replaces a user's API/MCP credential and returns the plaintext once.
func (s *IdentityService) GenerateAPIToken(ctx context.Context, userID string) (GeneratedAPIToken, error) {
	if s == nil || s.users == nil {
		return GeneratedAPIToken{}, fmt.Errorf("validate identity service: %w", ErrNotImplemented)
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return GeneratedAPIToken{}, fmt.Errorf("validate api token user: %w", ErrInvalidInput)
	}
	user, err := s.users.GetUser(ctx, userID)
	if errors.Is(err, identity.ErrNotFound) {
		return GeneratedAPIToken{}, fmt.Errorf("get api token user: %w", ErrNotFound)
	}
	if err != nil {
		return GeneratedAPIToken{}, fmt.Errorf("get api token user: %w", err)
	}
	if user.Status != identity.StatusActive {
		return GeneratedAPIToken{}, fmt.Errorf("validate api token user status: %w", ErrConflict)
	}
	raw, err := randomAPIToken()
	if err != nil {
		return GeneratedAPIToken{}, fmt.Errorf("generate api token: %w", err)
	}
	user, err = s.users.SaveUserAPIToken(ctx, userID, apiTokenHash(raw), apiTokenHint(raw))
	if err != nil {
		return GeneratedAPIToken{}, fmt.Errorf("save api token: %w", err)
	}
	return GeneratedAPIToken{Token: raw, User: user}, nil
}

// RevokeAPIToken immediately invalidates a user's API/MCP credential.
func (s *IdentityService) RevokeAPIToken(ctx context.Context, userID string) (identity.User, error) {
	if s == nil || s.users == nil {
		return identity.User{}, fmt.Errorf("validate identity service: %w", ErrNotImplemented)
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return identity.User{}, fmt.Errorf("validate api token user: %w", ErrInvalidInput)
	}
	user, err := s.users.RevokeUserAPIToken(ctx, userID)
	if errors.Is(err, identity.ErrNotFound) {
		return identity.User{}, fmt.Errorf("revoke api token: %w", ErrNotFound)
	}
	if err != nil {
		return identity.User{}, fmt.Errorf("revoke api token: %w", err)
	}
	return user, nil
}

// AuthenticateAPIToken resolves an active user from a high-entropy API/MCP token.
func (s *IdentityService) AuthenticateAPIToken(ctx context.Context, token string) (identity.User, error) {
	if s == nil || s.users == nil {
		return identity.User{}, fmt.Errorf("validate identity service: %w", ErrNotImplemented)
	}
	token = strings.TrimSpace(token)
	if !strings.HasPrefix(token, "oat_") || len(token) < 40 {
		return identity.User{}, ErrInvalidCredentials
	}
	user, err := s.users.GetUserByAPITokenHash(ctx, apiTokenHash(token))
	if errors.Is(err, identity.ErrNotFound) {
		return identity.User{}, ErrInvalidCredentials
	}
	if err != nil {
		return identity.User{}, fmt.Errorf("get api token user: %w", err)
	}
	if user.Status != identity.StatusActive {
		return identity.User{}, ErrInvalidCredentials
	}
	return user, nil
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

func randomAPIToken() (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return "oat_" + base64.RawURLEncoding.EncodeToString(raw[:]), nil
}

func apiTokenHash(token string) string {
	digest := sha256.Sum256([]byte(token))
	return hex.EncodeToString(digest[:])
}

func apiTokenHint(token string) string {
	if len(token) <= 10 {
		return token
	}
	return token[:4] + "..." + token[len(token)-6:]
}
