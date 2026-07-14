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
	service := NewAuthorizationServiceWithSecureAuthorizationFlow(
		store, store, store, nil, nil, authorizers, fakeRefreshTokenEncryptor{ciphertext: "encrypted-refresh"}, time.Now,
	)
	saveAuthorizationStateForTest(t, store, "state-1", "tenant-1", "wx-component")

	account, err := service.HandleAuthorizationCallback(ctx, HandleAuthorizationCallbackInput{
		State: "state-1", AuthCode: "auth-code",
		ReceivedAt: time.Date(2026, 7, 6, 18, 10, 0, 0, time.UTC),
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

	_, err = service.HandleAuthorizationCallback(ctx, HandleAuthorizationCallbackInput{State: "state-1", AuthCode: "auth-code"})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))
}

func TestAuthorizationServiceValidatesAuthorizationCallback(t *testing.T) {
	ctx := context.Background()
	store := memory.NewStore(time.Now)
	service := NewAuthorizationServiceWithSecureAuthorizationFlow(
		store, store, store, nil, nil,
		fakeAuthorizerClient{authorization: authorization.AuthorizerAuthorization{AppID: "wx-authorizer", RefreshToken: "refresh-token"}},
		fakeRefreshTokenEncryptor{ciphertext: "encrypted-refresh"},
		time.Now,
	)

	_, err := service.HandleAuthorizationCallback(ctx, HandleAuthorizationCallbackInput{AuthCode: "auth-code"})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))

	_, err = service.HandleAuthorizationCallback(ctx, HandleAuthorizationCallbackInput{State: "missing", AuthCode: "auth-code"})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))

	_, err = service.HandleAuthorizationCallback(ctx, HandleAuthorizationCallbackInput{State: "state"})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))
}

func TestAuthorizationServiceRequiresAuthorizationCallbackDependencies(t *testing.T) {
	ctx := context.Background()
	store := memory.NewStore(time.Now)
	service := NewAuthorizationService(store, time.Now)

	_, err := service.HandleAuthorizationCallback(ctx, HandleAuthorizationCallbackInput{
		State: "state", AuthCode: "auth-code",
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrNotImplemented))

	service = NewAuthorizationServiceWithAuthorizationFlow(store, store, nil, nil, fakeAuthorizerClient{
		authorization: authorization.AuthorizerAuthorization{AppID: "wx-authorizer", RefreshToken: "refresh-token"},
	}, nil, time.Now)
	_, err = service.HandleAuthorizationCallback(ctx, HandleAuthorizationCallbackInput{
		State: "state", AuthCode: "auth-code",
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrNotImplemented))
}

func TestAuthorizationServiceMapsAuthorizationCallbackMissingTicket(t *testing.T) {
	ctx := context.Background()
	store := memory.NewStore(time.Now)
	service := NewAuthorizationServiceWithSecureAuthorizationFlow(
		store, store, store, nil, nil,
		fakeAuthorizerClient{err: authorization.ErrComponentVerifyTicketNotFound},
		fakeRefreshTokenEncryptor{ciphertext: "encrypted-refresh"},
		time.Now,
	)
	saveAuthorizationStateForTest(t, store, "state-1", "tenant", "wx-component")

	_, err := service.HandleAuthorizationCallback(ctx, HandleAuthorizationCallbackInput{
		State: "state-1", AuthCode: "auth-code",
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrNotFound))
}

func saveAuthorizationStateForTest(t *testing.T, store *memory.Store, token string, tenantID string, componentAppID string) {
	t.Helper()
	_, err := store.SaveAuthorizationState(context.Background(), authorization.AuthorizationState{
		Digest: authorizationStateDigest(token), TenantID: tenantID, ComponentAppID: componentAppID,
		ExpiresAt: time.Now().Add(time.Hour),
	})
	require.NoError(t, err)
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
