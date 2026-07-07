package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"official-account-service/internal/domain/authorization"
	"official-account-service/internal/infra/persistence/memory"
)

func TestAuthorizationServiceHandlesAuthorizationCallback(t *testing.T) {
	ctx := context.Background()
	store := memory.NewStore(func() time.Time { return time.Date(2026, 7, 6, 18, 0, 0, 0, time.UTC) })
	authorizers := fakeAuthorizerClient{
		authorization: authorization.AuthorizerAuthorization{AppID: "wx-authorizer", RefreshToken: "refresh-token"},
		profile:       authorization.AuthorizerProfile{Name: "Account", AvatarURL: "https://example.com/avatar.png"},
	}
	service := NewAuthorizationServiceWithAuthorizationFlow(
		store, store, nil, nil, authorizers, fakeRefreshTokenEncryptor{ciphertext: "encrypted-refresh"}, time.Now,
	)

	account, err := service.HandleAuthorizationCallback(ctx, HandleAuthorizationCallbackInput{
		TenantID:       "tenant-1",
		ComponentAppID: "wx-component",
		AuthCode:       "auth-code",
		ReceivedAt:     time.Date(2026, 7, 6, 18, 10, 0, 0, time.UTC),
	})
	require.NoError(t, err)
	require.Equal(t, "tenant-1", account.TenantID)
	require.Equal(t, "wx-authorizer", account.AppID)
	require.Equal(t, "Account", account.Name)
	require.Equal(t, "https://example.com/avatar.png", account.AvatarURL)
	require.Equal(t, authorization.AccountStatusActive, account.Status)
	require.Equal(t, "encrypted-refresh", account.EncryptedAuthorizerRefreshToken)
	require.NotContains(t, account.EncryptedAuthorizerRefreshToken, "refresh-token")

	got, err := store.GetAccount(ctx, "tenant-1", account.ID)
	require.NoError(t, err)
	require.Equal(t, "encrypted-refresh", got.EncryptedAuthorizerRefreshToken)

	binding, err := store.GetAuthorizerTenantBinding(ctx, "wx-component", "wx-authorizer")
	require.NoError(t, err)
	require.Equal(t, "tenant-1", binding.TenantID)
}

func TestAuthorizationServiceValidatesAuthorizationCallback(t *testing.T) {
	ctx := context.Background()
	store := memory.NewStore(time.Now)
	service := NewAuthorizationServiceWithAuthorizationFlow(
		store, store, nil, nil,
		fakeAuthorizerClient{authorization: authorization.AuthorizerAuthorization{AppID: "wx-authorizer", RefreshToken: "refresh-token"}},
		fakeRefreshTokenEncryptor{ciphertext: "encrypted-refresh"},
		time.Now,
	)

	_, err := service.HandleAuthorizationCallback(ctx, HandleAuthorizationCallbackInput{ComponentAppID: "wx-component", AuthCode: "auth-code"})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))

	_, err = service.HandleAuthorizationCallback(ctx, HandleAuthorizationCallbackInput{TenantID: "tenant", AuthCode: "auth-code"})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))

	_, err = service.HandleAuthorizationCallback(ctx, HandleAuthorizationCallbackInput{TenantID: "tenant", ComponentAppID: "wx-component"})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))
}

func TestAuthorizationServiceRequiresAuthorizationCallbackDependencies(t *testing.T) {
	ctx := context.Background()
	store := memory.NewStore(time.Now)
	service := NewAuthorizationService(store, time.Now)

	_, err := service.HandleAuthorizationCallback(ctx, HandleAuthorizationCallbackInput{
		TenantID: "tenant", ComponentAppID: "wx-component", AuthCode: "auth-code",
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrNotImplemented))

	service = NewAuthorizationServiceWithAuthorizationFlow(store, store, nil, nil, fakeAuthorizerClient{
		authorization: authorization.AuthorizerAuthorization{AppID: "wx-authorizer", RefreshToken: "refresh-token"},
	}, nil, time.Now)
	_, err = service.HandleAuthorizationCallback(ctx, HandleAuthorizationCallbackInput{
		TenantID: "tenant", ComponentAppID: "wx-component", AuthCode: "auth-code",
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrNotImplemented))
}

func TestAuthorizationServiceMapsAuthorizationCallbackMissingTicket(t *testing.T) {
	ctx := context.Background()
	store := memory.NewStore(time.Now)
	service := NewAuthorizationServiceWithAuthorizationFlow(
		store, store, nil, nil,
		fakeAuthorizerClient{err: authorization.ErrComponentVerifyTicketNotFound},
		fakeRefreshTokenEncryptor{ciphertext: "encrypted-refresh"},
		time.Now,
	)

	_, err := service.HandleAuthorizationCallback(ctx, HandleAuthorizationCallbackInput{
		TenantID: "tenant", ComponentAppID: "wx-component", AuthCode: "auth-code",
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrNotFound))
}

type fakeAuthorizerClient struct {
	authorization authorization.AuthorizerAuthorization
	profile       authorization.AuthorizerProfile
	err           error
}

func (c fakeAuthorizerClient) QueryAuthorizerAuthorization(_ context.Context, _ string, _ string) (authorization.AuthorizerAuthorization, error) {
	if c.err != nil {
		return authorization.AuthorizerAuthorization{}, c.err
	}
	return c.authorization, nil
}

func (c fakeAuthorizerClient) GetAuthorizerProfile(_ context.Context, _ string, _ string) (authorization.AuthorizerProfile, error) {
	if c.err != nil {
		return authorization.AuthorizerProfile{}, c.err
	}
	return c.profile, nil
}

func (c fakeAuthorizerClient) RefreshAuthorizerAccessToken(_ context.Context, _ string, _ string, _ string) (authorization.AuthorizerToken, error) {
	if c.err != nil {
		return authorization.AuthorizerToken{}, c.err
	}
	return authorization.AuthorizerToken{}, nil
}

type fakeRefreshTokenEncryptor struct {
	ciphertext string
	err        error
}

func (e fakeRefreshTokenEncryptor) EncryptAuthorizerRefreshToken(_ context.Context, _ string) (string, error) {
	if e.err != nil {
		return "", e.err
	}
	return e.ciphertext, nil
}
