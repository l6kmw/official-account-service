package wechat

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"official-account-service/internal/domain/authorization"
	"official-account-service/internal/infra/persistence/memory"
)

func TestComponentClientCreatesPreAuthCode(t *testing.T) {
	ctx := context.Background()
	store := memory.NewStore(time.Now)
	_, err := store.SaveComponentVerifyTicket(ctx, authorization.ComponentVerifyTicket{
		ComponentAppID: "wx-component",
		Ticket:         "ticket-value",
		ReceivedAt:     time.Now(),
	})
	require.NoError(t, err)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/component/api_component_token":
			require.Equal(t, http.MethodPost, r.Method)
			var body componentTokenRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			require.Equal(t, "wx-component", body.ComponentAppID)
			require.Equal(t, "secret-value", body.ComponentAppSecret)
			require.Equal(t, "ticket-value", body.ComponentVerifyTicket)
			_, err := w.Write([]byte(`{"component_access_token":"component-token","expires_in":7200}`))
			require.NoError(t, err)
		case "/component/api_create_preauthcode":
			require.Equal(t, "component-token", r.URL.Query().Get("component_access_token"))
			var body preAuthCodeRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			require.Equal(t, "wx-component", body.ComponentAppID)
			_, err := w.Write([]byte(`{"pre_auth_code":"pre-auth-code","expires_in":600}`))
			require.NoError(t, err)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := NewComponentClient(store, ComponentClientConfig{
		ComponentAppSecret: "secret-value",
		BaseURL:            server.URL,
		RetryBackoff:       time.Millisecond,
	})
	require.NoError(t, err)

	code, err := client.CreatePreAuthCode(ctx, "wx-component")
	require.NoError(t, err)
	require.Equal(t, "pre-auth-code", code.Code)
	require.Equal(t, 600, code.ExpiresInSeconds)
}

func TestComponentClientQueriesAuthorizerAuthorizationAndProfile(t *testing.T) {
	ctx := context.Background()
	store := memory.NewStore(time.Now)
	_, err := store.SaveComponentVerifyTicket(ctx, authorization.ComponentVerifyTicket{
		ComponentAppID: "wx-component",
		Ticket:         "ticket-value",
		ReceivedAt:     time.Now(),
	})
	require.NoError(t, err)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/component/api_component_token":
			_, writeErr := w.Write([]byte(`{"component_access_token":"component-token","expires_in":7200}`))
			require.NoError(t, writeErr)
		case "/component/api_query_auth":
			require.Equal(t, "component-token", r.URL.Query().Get("component_access_token"))
			var body queryAuthorizerAuthRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			require.Equal(t, "wx-component", body.ComponentAppID)
			require.Equal(t, "auth-code", body.AuthorizationCode)
			_, writeErr := w.Write([]byte(`{"authorization_info":{"authorizer_appid":"wx-authorizer","authorizer_refresh_token":"refresh-token"}}`))
			require.NoError(t, writeErr)
		case "/component/api_get_authorizer_info":
			require.Equal(t, "component-token", r.URL.Query().Get("component_access_token"))
			var body authorizerInfoRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			require.Equal(t, "wx-component", body.ComponentAppID)
			require.Equal(t, "wx-authorizer", body.AuthorizerAppID)
			_, writeErr := w.Write([]byte(`{"authorizer_info":{"nick_name":"Account","head_img":"https://example.com/avatar.png"}}`))
			require.NoError(t, writeErr)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client, err := NewComponentClient(store, ComponentClientConfig{
		ComponentAppSecret: "secret-value",
		BaseURL:            server.URL,
		RetryBackoff:       time.Millisecond,
	})
	require.NoError(t, err)

	authorized, err := client.QueryAuthorizerAuthorization(ctx, "wx-component", "auth-code")
	require.NoError(t, err)
	require.Equal(t, "wx-authorizer", authorized.AppID)
	require.Equal(t, "refresh-token", authorized.RefreshToken)

	profile, err := client.GetAuthorizerProfile(ctx, "wx-component", "wx-authorizer")
	require.NoError(t, err)
	require.Equal(t, "Account", profile.Name)
	require.Equal(t, "https://example.com/avatar.png", profile.AvatarURL)
}

func TestComponentClientRefreshesAuthorizerAccessToken(t *testing.T) {
	ctx := context.Background()
	store := memory.NewStore(time.Now)
	_, err := store.SaveComponentVerifyTicket(ctx, authorization.ComponentVerifyTicket{
		ComponentAppID: "wx-component",
		Ticket:         "ticket-value",
		ReceivedAt:     time.Now(),
	})
	require.NoError(t, err)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/component/api_component_token":
			_, writeErr := w.Write([]byte(`{"component_access_token":"component-token","expires_in":7200}`))
			require.NoError(t, writeErr)
		case "/component/api_authorizer_token":
			require.Equal(t, "component-token", r.URL.Query().Get("component_access_token"))
			var body authorizerTokenRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			require.Equal(t, "wx-component", body.ComponentAppID)
			require.Equal(t, "wx-authorizer", body.AuthorizerAppID)
			require.Equal(t, "refresh-token", body.AuthorizerRefreshToken)
			_, writeErr := w.Write([]byte(`{"authorizer_access_token":"authorizer-token","authorizer_refresh_token":"refresh-token-2","expires_in":7200}`))
			require.NoError(t, writeErr)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client, err := NewComponentClient(store, ComponentClientConfig{
		ComponentAppSecret: "secret-value",
		BaseURL:            server.URL,
		RetryBackoff:       time.Millisecond,
	})
	require.NoError(t, err)

	token, err := client.RefreshAuthorizerAccessToken(ctx, "wx-component", "wx-authorizer", "refresh-token")
	require.NoError(t, err)
	require.Equal(t, "authorizer-token", token.AccessToken)
	require.Equal(t, "refresh-token-2", token.RefreshToken)
	require.Equal(t, 7200, token.ExpiresInSeconds)
}

func TestComponentClientRetriesTransientHTTPError(t *testing.T) {
	ctx := context.Background()
	store := memory.NewStore(time.Now)
	_, err := store.SaveComponentVerifyTicket(ctx, authorization.ComponentVerifyTicket{
		ComponentAppID: "wx-component",
		Ticket:         "ticket-value",
		ReceivedAt:     time.Now(),
	})
	require.NoError(t, err)
	var tokenAttempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/component/api_component_token":
			if atomic.AddInt32(&tokenAttempts, 1) == 1 {
				w.WriteHeader(http.StatusBadGateway)
				_, writeErr := w.Write([]byte(`bad gateway`))
				require.NoError(t, writeErr)
				return
			}
			_, writeErr := w.Write([]byte(`{"component_access_token":"component-token","expires_in":7200}`))
			require.NoError(t, writeErr)
		case "/component/api_create_preauthcode":
			_, writeErr := w.Write([]byte(`{"pre_auth_code":"pre-auth-code","expires_in":600}`))
			require.NoError(t, writeErr)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client, err := NewComponentClient(store, ComponentClientConfig{
		ComponentAppSecret: "secret-value",
		BaseURL:            server.URL,
		RetryBackoff:       time.Millisecond,
	})
	require.NoError(t, err)

	code, err := client.CreatePreAuthCode(ctx, "wx-component")
	require.NoError(t, err)
	require.Equal(t, "pre-auth-code", code.Code)
	require.Equal(t, int32(2), atomic.LoadInt32(&tokenAttempts))
}

func TestComponentClientMapsAuthorizerErrorWithoutLeakingRefreshToken(t *testing.T) {
	ctx := context.Background()
	store := memory.NewStore(time.Now)
	_, err := store.SaveComponentVerifyTicket(ctx, authorization.ComponentVerifyTicket{
		ComponentAppID: "wx-component",
		Ticket:         "ticket-value",
		ReceivedAt:     time.Now(),
	})
	require.NoError(t, err)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/component/api_component_token":
			_, writeErr := w.Write([]byte(`{"component_access_token":"component-token","expires_in":7200}`))
			require.NoError(t, writeErr)
		case "/component/api_query_auth":
			_, writeErr := w.Write([]byte(`{"errcode":61004,"errmsg":"auth code expired"}`))
			require.NoError(t, writeErr)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client, err := NewComponentClient(store, ComponentClientConfig{
		ComponentAppSecret: "secret-value",
		BaseURL:            server.URL,
		RetryBackoff:       time.Millisecond,
	})
	require.NoError(t, err)

	_, err = client.QueryAuthorizerAuthorization(ctx, "wx-component", "auth-code")
	require.Error(t, err)
	require.True(t, errors.Is(err, authorization.ErrAuthorizerClientUnavailable))
	require.Contains(t, err.Error(), "61004")
	require.NotContains(t, err.Error(), "secret-value")
	require.NotContains(t, err.Error(), "refresh-token")
}

func TestComponentClientMapsWeChatErrorWithoutLeakingSecretsOrTokens(t *testing.T) {
	ctx := context.Background()
	store := memory.NewStore(time.Now)
	_, err := store.SaveComponentVerifyTicket(ctx, authorization.ComponentVerifyTicket{
		ComponentAppID: "wx-component",
		Ticket:         "ticket-value",
		ReceivedAt:     time.Now(),
	})
	require.NoError(t, err)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/component/api_component_token":
			_, writeErr := w.Write([]byte(`{"component_access_token":"component-token","expires_in":7200}`))
			require.NoError(t, writeErr)
		case "/component/api_create_preauthcode":
			_, writeErr := w.Write([]byte(`{"errcode":61003,"errmsg":"component access token invalid"}`))
			require.NoError(t, writeErr)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client, err := NewComponentClient(store, ComponentClientConfig{
		ComponentAppSecret: "secret-value",
		BaseURL:            server.URL,
		RetryBackoff:       time.Millisecond,
	})
	require.NoError(t, err)

	_, err = client.CreatePreAuthCode(ctx, "wx-component")
	require.Error(t, err)
	require.True(t, errors.Is(err, authorization.ErrPreAuthCodeUnavailable))
	require.Contains(t, err.Error(), "61003")
	require.NotContains(t, err.Error(), "secret-value")
	require.NotContains(t, err.Error(), "component-token")
}

func TestComponentClientCircuitBreakerOpensAfterFailures(t *testing.T) {
	ctx := context.Background()
	store := memory.NewStore(time.Now)
	_, err := store.SaveComponentVerifyTicket(ctx, authorization.ComponentVerifyTicket{
		ComponentAppID: "wx-component",
		Ticket:         "ticket-value",
		ReceivedAt:     time.Now(),
	})
	require.NoError(t, err)
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()
	client, err := NewComponentClient(store, ComponentClientConfig{
		ComponentAppSecret: "secret-value",
		BaseURL:            server.URL,
		MaxRetries:         0,
		RetryBackoff:       time.Millisecond,
		BreakerThreshold:   1,
		BreakerOpenTimeout: time.Minute,
	})
	require.NoError(t, err)

	_, err = client.CreatePreAuthCode(ctx, "wx-component")
	require.Error(t, err)
	_, err = client.CreatePreAuthCode(ctx, "wx-component")
	require.Error(t, err)
	require.Contains(t, strings.ToLower(err.Error()), "circuit breaker open")
	require.Equal(t, int32(2), atomic.LoadInt32(&calls))
}
