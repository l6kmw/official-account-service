package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"official-account-service/internal/agentmcp"
)

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

func TestStreamableHTTPClientListsToolsAndCallsOfficialAccountTools(t *testing.T) {
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/accounts", r.URL.Path)
		require.Equal(t, "tenant-test", r.Header.Get("X-Tenant-ID"))
		require.Equal(t, "admin-key", r.Header.Get("X-Admin-API-Key"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[{"id":1,"tenant_id":"tenant-test","app_id":"wx-account","name":"Account","status":"active"}]}`))
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

	accounts, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "official_account_list_accounts",
		Arguments: map[string]any{},
	})
	require.NoError(t, err)
	require.False(t, accounts.IsError)
	require.NotNil(t, accounts.StructuredContent)
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
