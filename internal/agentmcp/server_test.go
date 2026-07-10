package agentmcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

func TestMCPServerListsToolsAndCallsOfficialAccountAPI(t *testing.T) {
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/accounts", r.URL.Path)
		require.Equal(t, "tenant-test", r.Header.Get("X-Tenant-ID"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[{"id":1,"tenant_id":"tenant-test","app_id":"wx-account","name":"Account","status":"active"}]}`))
	}))
	defer apiServer.Close()

	httpClient, err := NewClient(Config{BaseURL: apiServer.URL, TenantID: "tenant-test"})
	require.NoError(t, err)
	server := NewServer(httpClient, ServerConfig{})
	serverTransport, clientTransport := mcp.NewInMemoryTransports()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go func() {
		_ = server.Run(ctx, serverTransport)
	}()

	mcpClient := mcp.NewClient(&mcp.Implementation{Name: "official-account-test", Version: "0.1.0"}, nil)
	session, err := mcpClient.Connect(ctx, clientTransport, nil)
	require.NoError(t, err)
	defer session.Close()

	tools, err := session.ListTools(ctx, &mcp.ListToolsParams{})
	require.NoError(t, err)
	names := make(map[string]bool)
	for _, tool := range tools.Tools {
		names[tool.Name] = true
	}
	require.True(t, names["official_account_list_accounts"])
	require.True(t, names["official_account_publish_article"])
	require.True(t, names["official_account_delete_article"])
	require.True(t, names["official_account_get_authorization_entry"])

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "official_account_list_accounts",
		Arguments: map[string]any{},
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.NotNil(t, result.StructuredContent)
}

func TestMCPServerDeletesLocalArticle(t *testing.T) {
	var deleted bool
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "tenant-test", r.Header.Get("X-Tenant-ID"))
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/articles/12":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":12,"tenant_id":"tenant-test","authorizer_id":1,"title":"draft","status":"draft"}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/articles/12":
			deleted = true
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer apiServer.Close()

	httpClient, err := NewClient(Config{BaseURL: apiServer.URL, TenantID: "tenant-test"})
	require.NoError(t, err)
	server := NewServer(httpClient, ServerConfig{})
	serverTransport, clientTransport := mcp.NewInMemoryTransports()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go func() {
		_ = server.Run(ctx, serverTransport)
	}()

	mcpClient := mcp.NewClient(&mcp.Implementation{Name: "official-account-test", Version: "0.1.0"}, nil)
	session, err := mcpClient.Connect(ctx, clientTransport, nil)
	require.NoError(t, err)
	defer session.Close()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "official_account_delete_article",
		Arguments: map[string]any{
			"article_id":     12,
			"confirm_delete": "DELETE",
		},
	})

	require.NoError(t, err)
	require.False(t, result.IsError)
	require.True(t, deleted)
	require.NotNil(t, result.StructuredContent)
}
