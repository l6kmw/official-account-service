package application

import (
	"context"
	"errors"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"official-account-service/internal/domain/authorization"
	"official-account-service/internal/infra/persistence/memory"
)

func TestAuthorizationServiceSaveAndGetComponentVerifyTicket(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 6, 18, 0, 0, 0, time.UTC)
	service := NewAuthorizationService(memory.NewStore(func() time.Time { return now }), func() time.Time { return now })

	saved, err := service.SaveComponentVerifyTicket(ctx, SaveComponentVerifyTicketInput{
		ComponentAppID: "wx-component",
		Ticket:         "ticket-1",
	})
	require.NoError(t, err)
	require.Equal(t, "wx-component", saved.ComponentAppID)
	require.Equal(t, "ticket-1", saved.Ticket)
	require.Equal(t, now, saved.ReceivedAt)
	require.Equal(t, now, saved.UpdatedAt)

	receivedAt := time.Date(2026, 7, 6, 18, 5, 0, 0, time.UTC)
	updated, err := service.SaveComponentVerifyTicket(ctx, SaveComponentVerifyTicketInput{
		ComponentAppID: "wx-component",
		Ticket:         "ticket-2",
		ReceivedAt:     receivedAt,
	})
	require.NoError(t, err)
	require.Equal(t, "ticket-2", updated.Ticket)
	require.Equal(t, receivedAt, updated.ReceivedAt)

	got, err := service.GetComponentVerifyTicket(ctx, "wx-component")
	require.NoError(t, err)
	require.Equal(t, "ticket-2", got.Ticket)
}

func TestAuthorizationServiceValidatesComponentVerifyTicketInput(t *testing.T) {
	ctx := context.Background()
	service := NewAuthorizationService(memory.NewStore(time.Now), time.Now)

	_, err := service.SaveComponentVerifyTicket(ctx, SaveComponentVerifyTicketInput{ComponentAppID: "", Ticket: "ticket"})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))

	_, err = service.SaveComponentVerifyTicket(ctx, SaveComponentVerifyTicketInput{ComponentAppID: "wx-component", Ticket: ""})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))

	_, err = service.GetComponentVerifyTicket(ctx, "")
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))

	_, err = service.GetComponentVerifyTicket(ctx, "missing")
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrNotFound))
}

func TestAuthorizationServiceRejectsPlainComponentCallback(t *testing.T) {
	ctx := context.Background()
	store := memory.NewStore(func() time.Time { return time.Date(2026, 7, 6, 18, 0, 0, 0, time.UTC) })
	service := NewAuthorizationService(store, time.Now)
	rawBody := []byte(`<xml><AppId>wx-component</AppId><CreateTime>1783334400</CreateTime><InfoType>component_verify_ticket</InfoType><ComponentVerifyTicket>ticket-1</ComponentVerifyTicket></xml>`)

	err := service.HandleComponentCallback(ctx, HandleComponentCallbackInput{RawBody: rawBody})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))
}

func TestAuthorizationServiceHandlesUnauthorizedComponentCallback(t *testing.T) {
	ctx := context.Background()
	store := memory.NewStore(func() time.Time { return time.Date(2026, 7, 6, 18, 0, 0, 0, time.UTC) })
	account, err := store.SaveAccount(ctx, "tenant-1", authorization.Account{
		AppID: "wx-authorizer", Name: "Account", Status: authorization.AccountStatusActive,
		EncryptedAuthorizerRefreshToken: "encrypted-refresh",
	})
	require.NoError(t, err)
	_, err = store.SaveAuthorizerTenantBinding(ctx, authorization.AuthorizerTenantBinding{
		ComponentAppID: "wx-component", AuthorizerAppID: "wx-authorizer", TenantID: "tenant-1",
	})
	require.NoError(t, err)
	rawBody := []byte(`<xml><AppId>wx-component</AppId><CreateTime>1783334400</CreateTime><InfoType>unauthorized</InfoType><AuthorizerAppid>wx-authorizer</AuthorizerAppid></xml>`)
	decryptor := &fakeComponentCallbackDecryptor{plaintext: rawBody}
	service := NewAuthorizationServiceWithDependencies(store, nil, decryptor, time.Now)

	err = service.HandleComponentCallback(ctx, encryptedComponentCallbackInput())
	require.NoError(t, err)

	got, err := store.GetAccount(ctx, "tenant-1", account.ID)
	require.NoError(t, err)
	require.Equal(t, authorization.AccountStatusRevoked, got.Status)
	require.Empty(t, got.EncryptedAuthorizerRefreshToken)
}

func TestAuthorizationServiceValidatesUnauthorizedComponentCallback(t *testing.T) {
	ctx := context.Background()
	store := memory.NewStore(time.Now)
	decryptor := &fakeComponentCallbackDecryptor{}
	service := NewAuthorizationServiceWithDependencies(store, nil, decryptor, time.Now)

	decryptor.plaintext = []byte(`<xml><AppId>wx-component</AppId><CreateTime>1783334400</CreateTime><InfoType>unauthorized</InfoType><AuthorizerAppid>wx-missing</AuthorizerAppid></xml>`)
	err := service.HandleComponentCallback(ctx, encryptedComponentCallbackInput())
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrNotFound))

	decryptor.plaintext = []byte(`<xml><AppId>wx-component</AppId><CreateTime>1783334400</CreateTime><InfoType>unauthorized</InfoType></xml>`)
	err = service.HandleComponentCallback(ctx, encryptedComponentCallbackInput())
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))
}

func TestAuthorizationServiceHandlesEncryptedComponentCallback(t *testing.T) {
	ctx := context.Background()
	store := memory.NewStore(func() time.Time { return time.Date(2026, 7, 6, 18, 0, 0, 0, time.UTC) })
	plaintext := []byte(`<xml><AppId>wx-component</AppId><CreateTime>1783334400</CreateTime><InfoType>component_verify_ticket</InfoType><ComponentVerifyTicket>ticket-1</ComponentVerifyTicket></xml>`)
	decryptor := &fakeComponentCallbackDecryptor{plaintext: plaintext}
	service := NewAuthorizationServiceWithDependencies(store, nil, decryptor, time.Now)

	err := service.HandleComponentCallback(ctx, HandleComponentCallbackInput{
		RawBody:      []byte(`<xml><Encrypt>ciphertext</Encrypt></xml>`),
		EncryptType:  "aes",
		MsgSignature: "signature",
		Timestamp:    "1783334400",
		Nonce:        "nonce",
	})
	require.NoError(t, err)
	require.Equal(t, "signature", decryptor.lastInput.Signature)
	require.Equal(t, "ciphertext", decryptor.lastInput.Ciphertext)

	ticket, err := service.GetComponentVerifyTicket(ctx, "wx-component")
	require.NoError(t, err)
	require.Equal(t, "ticket-1", ticket.Ticket)
}

func TestAuthorizationServiceRequiresAESModeForComponentCallback(t *testing.T) {
	ctx := context.Background()
	store := memory.NewStore(func() time.Time { return time.Date(2026, 7, 6, 18, 0, 0, 0, time.UTC) })
	plaintext := []byte(`<xml><AppId>wx-component</AppId><CreateTime>1783334400</CreateTime><InfoType>component_verify_ticket</InfoType><ComponentVerifyTicket>ticket-1</ComponentVerifyTicket></xml>`)
	decryptor := &fakeComponentCallbackDecryptor{plaintext: plaintext}
	service := NewAuthorizationServiceWithDependencies(store, nil, decryptor, time.Now)

	err := service.HandleComponentCallback(ctx, HandleComponentCallbackInput{
		RawBody:      []byte(`<xml><Encrypt>ciphertext</Encrypt></xml>`),
		MsgSignature: "signature",
		Timestamp:    "1783334400",
		Nonce:        "nonce",
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))
	require.Empty(t, decryptor.lastInput.Ciphertext)
}

func TestAuthorizationServiceValidatesComponentCallback(t *testing.T) {
	ctx := context.Background()
	service := NewAuthorizationService(memory.NewStore(time.Now), time.Now)

	err := service.HandleComponentCallback(ctx, HandleComponentCallbackInput{RawBody: nil})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))

	err = service.HandleComponentCallback(ctx, HandleComponentCallbackInput{
		RawBody:     []byte(`<xml><Encrypt>ciphertext</Encrypt></xml>`),
		EncryptType: "aes",
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))

	err = service.HandleComponentCallback(ctx, HandleComponentCallbackInput{
		RawBody:      []byte(`<xml><Encrypt>ciphertext</Encrypt></xml>`),
		EncryptType:  "aes",
		MsgSignature: "signature",
		Timestamp:    "1783334400",
		Nonce:        "nonce",
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrNotImplemented))
}

func TestAuthorizationServiceGenerateAuthorizationURL(t *testing.T) {
	ctx := context.Background()
	service := NewAuthorizationServiceWithPreAuthCodeCreator(
		memory.NewStore(time.Now),
		fakePreAuthCodeCreator{code: "pre-auth-code", expiresInSeconds: 600},
		time.Now,
	)

	result, err := service.GenerateAuthorizationURL(ctx, GenerateAuthorizationURLInput{
		ComponentAppID: "wx-component",
		RedirectURI:    "https://example.com/wechat/auth/callback?tenant_id=tenant-1",
	})
	require.NoError(t, err)
	require.Equal(t, 600, result.PreAuthCodeExpiresInSec)

	parsed, err := url.Parse(result.URL)
	require.NoError(t, err)
	require.Equal(t, "https", parsed.Scheme)
	require.Equal(t, "mp.weixin.qq.com", parsed.Host)
	require.Equal(t, "/cgi-bin/componentloginpage", parsed.Path)
	require.Equal(t, "wx-component", parsed.Query().Get("component_appid"))
	require.Equal(t, "pre-auth-code", parsed.Query().Get("pre_auth_code"))
	require.Equal(t, "https://example.com/wechat/auth/callback?tenant_id=tenant-1", parsed.Query().Get("redirect_uri"))
	require.Equal(t, "1", parsed.Query().Get("auth_type"))
	require.Empty(t, parsed.Query().Get("biz_appid"))
}

func TestAuthorizationServiceGenerateAuthorizationURLWithOptionalFields(t *testing.T) {
	ctx := context.Background()
	service := NewAuthorizationServiceWithPreAuthCodeCreator(
		memory.NewStore(time.Now),
		fakePreAuthCodeCreator{code: "pre-auth-code", expiresInSeconds: 600},
		time.Now,
	)

	result, err := service.GenerateAuthorizationURL(ctx, GenerateAuthorizationURLInput{
		ComponentAppID: "wx-component",
		RedirectURI:    "https://example.com/wechat/auth/callback",
		AuthType:       AuthorizationAuthTypeAll,
		BizAppID:       "wx-authorizer",
	})
	require.NoError(t, err)

	parsed, err := url.Parse(result.URL)
	require.NoError(t, err)
	require.Equal(t, "3", parsed.Query().Get("auth_type"))
	require.Equal(t, "wx-authorizer", parsed.Query().Get("biz_appid"))
}

func TestAuthorizationServiceValidatesAuthorizationURLInput(t *testing.T) {
	ctx := context.Background()
	service := NewAuthorizationServiceWithPreAuthCodeCreator(
		memory.NewStore(time.Now),
		fakePreAuthCodeCreator{code: "pre-auth-code", expiresInSeconds: 600},
		time.Now,
	)

	_, err := service.GenerateAuthorizationURL(ctx, GenerateAuthorizationURLInput{RedirectURI: "https://example.com/callback"})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))

	_, err = service.GenerateAuthorizationURL(ctx, GenerateAuthorizationURLInput{ComponentAppID: "wx-component", RedirectURI: "http://example.com/callback"})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))

	_, err = service.GenerateAuthorizationURL(ctx, GenerateAuthorizationURLInput{ComponentAppID: "wx-component", RedirectURI: "https://example.com/callback", AuthType: 9})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))
}

func TestAuthorizationServiceMapsPreAuthCodeUnavailable(t *testing.T) {
	ctx := context.Background()
	service := NewAuthorizationServiceWithPreAuthCodeCreator(
		memory.NewStore(time.Now),
		fakePreAuthCodeCreator{err: authorization.ErrPreAuthCodeUnavailable},
		time.Now,
	)

	_, err := service.GenerateAuthorizationURL(ctx, GenerateAuthorizationURLInput{
		ComponentAppID: "wx-component",
		RedirectURI:    "https://example.com/callback",
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrNotImplemented))

	service = NewAuthorizationService(memory.NewStore(time.Now), time.Now)
	_, err = service.GenerateAuthorizationURL(ctx, GenerateAuthorizationURLInput{
		ComponentAppID: "wx-component",
		RedirectURI:    "https://example.com/callback",
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrNotImplemented))
}

func TestAuthorizationServiceMapsMissingComponentVerifyTicket(t *testing.T) {
	ctx := context.Background()
	service := NewAuthorizationServiceWithPreAuthCodeCreator(
		memory.NewStore(time.Now),
		fakePreAuthCodeCreator{err: authorization.ErrComponentVerifyTicketNotFound},
		time.Now,
	)

	_, err := service.GenerateAuthorizationURL(ctx, GenerateAuthorizationURLInput{
		ComponentAppID: "wx-component",
		RedirectURI:    "https://example.com/callback",
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrNotFound))
}

type fakePreAuthCodeCreator struct {
	code             string
	expiresInSeconds int
	err              error
}

func (c fakePreAuthCodeCreator) CreatePreAuthCode(_ context.Context, _ string) (authorization.PreAuthCode, error) {
	if c.err != nil {
		return authorization.PreAuthCode{}, c.err
	}
	return authorization.PreAuthCode{Code: c.code, ExpiresInSeconds: c.expiresInSeconds}, nil
}

type fakeComponentCallbackDecryptor struct {
	plaintext []byte
	err       error
	lastInput authorization.ComponentCallbackDecryptInput
}

func encryptedComponentCallbackInput() HandleComponentCallbackInput {
	return HandleComponentCallbackInput{
		RawBody:      []byte(`<xml><Encrypt>ciphertext</Encrypt></xml>`),
		EncryptType:  "aes",
		MsgSignature: "signature",
		Timestamp:    "1783334400",
		Nonce:        "nonce",
	}
}

func (d *fakeComponentCallbackDecryptor) DecryptComponentCallback(_ context.Context, input authorization.ComponentCallbackDecryptInput) ([]byte, error) {
	d.lastInput = input
	if d.err != nil {
		return nil, d.err
	}
	return d.plaintext, nil
}
