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
