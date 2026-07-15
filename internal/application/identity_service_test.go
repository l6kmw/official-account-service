package application

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"official-account-service/internal/domain/identity"
	"official-account-service/internal/infra/persistence/memory"
)

func TestIdentityServiceBootstrapsAndAuthenticatesUser(t *testing.T) {
	store := memory.NewStore(nil)
	service := NewIdentityService(store)
	hash, err := bcrypt.GenerateFromPassword([]byte("secret-password"), bcrypt.MinCost)
	require.NoError(t, err)

	bootstrapped, err := service.EnsureBootstrapUser(context.Background(), BootstrapUserInput{
		UserID: "tenant-1", Username: "admin", PasswordHash: string(hash),
	})
	require.NoError(t, err)
	require.Equal(t, identity.RoleAdmin, bootstrapped.Role)
	require.Equal(t, identity.StatusActive, bootstrapped.Status)

	authenticated, err := service.Authenticate(context.Background(), "ADMIN", "secret-password")
	require.NoError(t, err)
	require.Equal(t, "tenant-1", authenticated.ID)

	_, err = service.Authenticate(context.Background(), "admin", "wrong")
	require.ErrorIs(t, err, ErrInvalidCredentials)
}

func TestIdentityServiceRejectsDisabledSessionUser(t *testing.T) {
	store := memory.NewStore(nil)
	_, err := store.SaveUser(context.Background(), identity.User{
		ID: "user-1", Username: "disabled", PasswordHash: "hash", Role: identity.RoleUser, Status: identity.StatusDisabled,
	})
	require.NoError(t, err)
	service := NewIdentityService(store)

	_, err = service.GetActiveUser(context.Background(), "user-1")

	require.True(t, errors.Is(err, ErrInvalidCredentials))
}

func TestIdentityServiceCreatesIndependentUser(t *testing.T) {
	store := memory.NewStore(nil)
	service := NewIdentityService(store)
	service.newUserID = func() (string, error) { return "user-2", nil }

	created, err := service.CreateUser(context.Background(), CreateUserInput{Username: "writer", Password: "strong-password-123"})

	require.NoError(t, err)
	require.Equal(t, "user-2", created.ID)
	require.Equal(t, identity.RoleUser, created.Role)
	require.NotEqual(t, "strong-password-123", created.PasswordHash)
	authenticated, err := service.Authenticate(context.Background(), "writer", "strong-password-123")
	require.NoError(t, err)
	require.Equal(t, "user-2", authenticated.ID)
}

func TestIdentityServiceGeneratesAuthenticatesAndRevokesAPIToken(t *testing.T) {
	store := memory.NewStore(nil)
	_, err := store.SaveUser(context.Background(), identity.User{
		ID: "user-1", Username: "writer", PasswordHash: "hash", Role: identity.RoleUser, Status: identity.StatusActive,
	})
	require.NoError(t, err)
	service := NewIdentityService(store)

	generated, err := service.GenerateAPIToken(context.Background(), "user-1")
	require.NoError(t, err)
	require.Contains(t, generated.Token, "oat_")
	require.NotEmpty(t, generated.User.APITokenHash)
	require.NotContains(t, generated.User.APITokenHint, generated.Token)
	require.NotNil(t, generated.User.APITokenCreatedAt)

	authenticated, err := service.AuthenticateAPIToken(context.Background(), generated.Token)
	require.NoError(t, err)
	require.Equal(t, "user-1", authenticated.ID)
	_, err = service.AuthenticateAPIToken(context.Background(), "oat_invalid-token-value-that-is-long-enough")
	require.ErrorIs(t, err, ErrInvalidCredentials)

	_, err = service.RevokeAPIToken(context.Background(), "user-1")
	require.NoError(t, err)
	_, err = service.AuthenticateAPIToken(context.Background(), generated.Token)
	require.ErrorIs(t, err, ErrInvalidCredentials)
}

func TestIdentityServiceRejectsAPITokenForDisabledUser(t *testing.T) {
	store := memory.NewStore(nil)
	_, err := store.SaveUser(context.Background(), identity.User{
		ID: "user-1", Username: "writer", PasswordHash: "hash", Role: identity.RoleUser, Status: identity.StatusActive,
	})
	require.NoError(t, err)
	service := NewIdentityService(store)
	generated, err := service.GenerateAPIToken(context.Background(), "user-1")
	require.NoError(t, err)

	_, err = store.SaveUser(context.Background(), identity.User{
		ID: "user-1", Username: "writer", PasswordHash: "hash", Role: identity.RoleUser, Status: identity.StatusDisabled,
	})
	require.NoError(t, err)
	_, err = service.AuthenticateAPIToken(context.Background(), generated.Token)
	require.ErrorIs(t, err, ErrInvalidCredentials)
}
