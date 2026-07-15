package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"official-account-service/internal/agentmcp"
)

func TestMCPStreamableHTTPConfigFallsBackToYAMLConfig(t *testing.T) {
	t.Setenv("OFFICIAL_ACCOUNT_MCP_TOKEN", "")
	t.Setenv("OFFICIAL_ACCOUNT_MCP_PATH", "")
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`mcp:
  token: yaml-token
  path: /wechat-mcp
`), 0o600))

	cfg, err := mcpStreamableHTTPConfig(path)

	require.NoError(t, err)
	require.Equal(t, "yaml-token", cfg.Token)
	require.Equal(t, "/wechat-mcp", cfg.Path)
}

func TestStreamableHTTPRequiresBearerToken(t *testing.T) {
	client, err := agentmcp.NewClient(agentmcp.Config{
		BaseURL:  "https://mp.example.com",
		TenantID: "tenant-test",
	})
	require.NoError(t, err)
	handler := newStreamableHTTPMux(agentmcp.NewServer(client, agentmcp.ServerConfig{}), streamableHTTPConfig{
		Path:  "/mcp",
		Token: "mcp-token",
	})

	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.Contains(t, rec.Body.String(), "no bearer token")
}

func TestStreamableHTTPAcceptsBearerAuthorizationHeader(t *testing.T) {
	client, err := agentmcp.NewClient(agentmcp.Config{
		BaseURL:  "https://mp.example.com",
		TenantID: "tenant-test",
	})
	require.NoError(t, err)
	handler := newStreamableHTTPMux(agentmcp.NewServer(client, agentmcp.ServerConfig{}), streamableHTTPConfig{
		Path:  "/mcp",
		Token: "mcp-token",
	})

	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"0.1.0"}}}`))
	req.Header.Set("Authorization", "Bearer mcp-token")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "official-account-service")
}

func TestStreamableHTTPAcceptsAPIKeyHeader(t *testing.T) {
	client, err := agentmcp.NewClient(agentmcp.Config{
		BaseURL:  "https://mp.example.com",
		TenantID: "tenant-test",
	})
	require.NoError(t, err)
	handler := newStreamableHTTPMux(agentmcp.NewServer(client, agentmcp.ServerConfig{}), streamableHTTPConfig{
		Path:  "/mcp",
		Token: "mcp-token",
	})

	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"0.1.0"}}}`))
	req.Header.Set("X-API-Key", "mcp-token")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "official-account-service")
}

func TestStreamableHTTPClientListsToolsAndCallsOfficialAccountTools(t *testing.T) {
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "tenant-test", r.Header.Get("X-Tenant-ID"))
		require.Equal(t, "admin-key", r.Header.Get("X-Admin-API-Key"))
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/accounts":
			_, _ = w.Write([]byte(`{"items":[{"id":1,"tenant_id":"tenant-test","app_id":"wx-account","name":"Account","status":"active"}]}`))
		case "/api/v1/wechat/authorization-url":
			_, _ = w.Write([]byte(`{"authorization_url":"https://mp.weixin.qq.com/cgi-bin/componentloginpage?state=user-bound","pre_auth_code_expires_in_sec":600}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer apiServer.Close()

	client, err := agentmcp.NewClient(agentmcp.Config{
		BaseURL:        apiServer.URL,
		PublicBaseURL:  "https://mp.example.com",
		ComponentAppID: "wx-component",
		TenantID:       "tenant-test",
		AdminAPIKey:    "admin-key",
	})
	require.NoError(t, err)
	mcpHTTPServer := httptest.NewServer(newStreamableHTTPMux(agentmcp.NewServer(client, agentmcp.ServerConfig{}), streamableHTTPConfig{
		Path:  "/mcp",
		Token: "mcp-token",
	}))
	defer mcpHTTPServer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	mcpClient := mcp.NewClient(&mcp.Implementation{Name: "official-account-test", Version: "0.1.0"}, nil)
	session, err := mcpClient.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint:             mcpHTTPServer.URL + "/mcp",
		DisableStandaloneSSE: true,
		OAuthHandler:         bearerTokenHandler{token: "mcp-token"},
		MaxRetries:           -1,
	}, nil)
	require.NoError(t, err)
	defer session.Close()

	tools, err := session.ListTools(ctx, &mcp.ListToolsParams{})
	require.NoError(t, err)
	names := make(map[string]bool)
	for _, tool := range tools.Tools {
		names[tool.Name] = true
	}
	require.True(t, names["official_account_get_authorization_entry"])
	require.True(t, names["official_account_publish_article"])

	authEntry, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "official_account_get_authorization_entry",
		Arguments: map[string]any{},
	})
	require.NoError(t, err)
	require.False(t, authEntry.IsError)
	require.NotNil(t, authEntry.StructuredContent)
	require.Contains(t, fmt.Sprint(authEntry.StructuredContent), "https://mp.example.com/wechat-authorize.html")
	require.Contains(t, fmt.Sprint(authEntry.StructuredContent), "state%3Duser-bound")
	advancedAuthEntry, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "official_account_generate_authorization_url",
		Arguments: map[string]any{"auth_type": 1},
	})
	require.NoError(t, err)
	require.False(t, advancedAuthEntry.IsError)
	require.Contains(t, fmt.Sprint(advancedAuthEntry.StructuredContent), "https://mp.example.com/wechat-authorize.html")
	require.Contains(t, fmt.Sprint(advancedAuthEntry.StructuredContent), "state%3Duser-bound")
	rejectedRedirect, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "official_account_generate_authorization_url",
		Arguments: map[string]any{"redirect_uri": "https://evil.example/callback"},
	})
	require.True(t, err != nil || rejectedRedirect.IsError)

	accounts, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "official_account_list_accounts",
		Arguments: map[string]any{},
	})
	require.NoError(t, err)
	require.False(t, accounts.IsError)
	require.NotNil(t, accounts.StructuredContent)
}

func TestStreamableHTTPUserTokenBindsToolCallsToAuthenticatedUser(t *testing.T) {
	const userToken = "oat_abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG"
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer "+userToken, r.Header.Get("Authorization"))
		require.Empty(t, r.Header.Get("X-Admin-API-Key"))
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/admin/session":
			_, _ = w.Write([]byte(`{"authenticated":true,"user_id":"user-2","username":"writer","role":"user"}`))
		case "/api/v1/accounts":
			_, _ = w.Write([]byte(`{"items":[{"id":2,"tenant_id":"user-2","app_id":"wx-user-2","name":"Writer","status":"active"}]}`))
		case "/api/v1/wechat/authorization-url":
			_, _ = w.Write([]byte(`{"authorization_url":"https://mp.weixin.qq.com/cgi-bin/componentloginpage?state=user-2-bound","pre_auth_code_expires_in_sec":600}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer apiServer.Close()

	client, err := agentmcp.NewClient(agentmcp.Config{
		BaseURL: apiServer.URL, TenantID: "tenant-1", AdminAPIKey: "admin-key", ComponentAppID: "wx-component",
	})
	require.NoError(t, err)
	mcpHTTPServer := httptest.NewServer(newStreamableHTTPMux(agentmcp.NewServer(client, agentmcp.ServerConfig{}), streamableHTTPConfig{
		Path: "/mcp", Token: "legacy-mcp-token", Verifier: userAwareTokenVerifier(client, "legacy-mcp-token"),
	}))
	defer mcpHTTPServer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	mcpClient := mcp.NewClient(&mcp.Implementation{Name: "user-token-test", Version: "0.1.0"}, nil)
	session, err := mcpClient.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint: mcpHTTPServer.URL + "/mcp", DisableStandaloneSSE: true,
		OAuthHandler: bearerTokenHandler{token: userToken}, MaxRetries: -1,
	}, nil)
	require.NoError(t, err)
	defer session.Close()

	accounts, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "official_account_list_accounts", Arguments: map[string]any{}})
	require.NoError(t, err)
	require.False(t, accounts.IsError)
	require.Contains(t, accounts.StructuredContent, "items")
	authorization, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "official_account_get_authorization_entry", Arguments: map[string]any{}})
	require.NoError(t, err)
	require.False(t, authorization.IsError)
	require.Contains(t, fmt.Sprint(authorization.StructuredContent), "wechat-authorize.html")
	require.Contains(t, fmt.Sprint(authorization.StructuredContent), "state%3Duser-2-bound")
	advancedAuthorization, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "official_account_generate_authorization_url", Arguments: map[string]any{"auth_type": 1}})
	require.NoError(t, err)
	require.False(t, advancedAuthorization.IsError)
	require.Contains(t, fmt.Sprint(advancedAuthorization.StructuredContent), "wechat-authorize.html")
	require.Contains(t, fmt.Sprint(advancedAuthorization.StructuredContent), "state%3Duser-2-bound")
}

type bearerTokenHandler struct {
	token string
}

func (h bearerTokenHandler) TokenSource(context.Context) (oauth2.TokenSource, error) {
	return oauth2.StaticTokenSource(&oauth2.Token{AccessToken: h.token}), nil
}

func (h bearerTokenHandler) Authorize(context.Context, *http.Request, *http.Response) error {
	return nil
}
