package agentmcp

import (
	"context"
	"encoding/json"
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
	toolsByName := make(map[string]*mcp.Tool)
	for _, tool := range tools.Tools {
		toolsByName[tool.Name] = tool
	}
	require.Contains(t, toolsByName, "official_account_list_accounts")
	require.Contains(t, toolsByName, "official_account_publish_article")
	require.Contains(t, toolsByName, "official_account_delete_article")
	require.Contains(t, toolsByName, "official_account_list_published_articles")
	require.Contains(t, toolsByName, "official_account_list_permanent_materials")
	require.Contains(t, toolsByName, "official_account_delete_permanent_material")
	require.Contains(t, toolsByName, "official_account_get_article_metrics")
	require.Contains(t, toolsByName, "official_account_list_article_comments")
	require.Contains(t, toolsByName, "official_account_get_authorization_entry")
	require.ElementsMatch(t, []string{"authorizer_id", "title"}, requiredToolFields(t, toolsByName["official_account_create_article"]))
	require.ElementsMatch(t, []string{"article_id", "title", "author", "digest", "content_html", "cover_media_asset_id"}, requiredToolFields(t, toolsByName["official_account_update_article"]))
	require.ElementsMatch(t, []string{"authorizer_id", "article_id", "usage"}, requiredToolFields(t, toolsByName["official_account_upload_image"]))
	require.ElementsMatch(t, []string{"authorizer_id"}, requiredToolFields(t, toolsByName["official_account_list_permanent_materials"]))
	require.ElementsMatch(t, []string{"authorizer_id", "media_id", "confirm_delete"}, requiredToolFields(t, toolsByName["official_account_delete_permanent_material"]))
	require.ElementsMatch(t, []string{"article_id", "confirm_publish"}, requiredToolFields(t, toolsByName["official_account_publish_article"]))
	require.Contains(t, toolsByName["official_account_upload_image"].Description, "asset.id")
	require.Contains(t, toolsByName["official_account_upload_image"].Description, "image_url")
	require.Contains(t, toolsByName["official_account_publish_article"].Description, "content_html")

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "official_account_list_accounts",
		Arguments: map[string]any{},
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.NotNil(t, result.StructuredContent)
}

func requiredToolFields(t *testing.T, tool *mcp.Tool) []string {
	t.Helper()
	require.NotNil(t, tool)
	raw, err := json.Marshal(tool.InputSchema)
	require.NoError(t, err)
	var schema struct {
		Required []string `json:"required"`
	}
	require.NoError(t, json.Unmarshal(raw, &schema))
	return schema.Required
}

func TestMCPServerDeletesPublishedArticleCompletely(t *testing.T) {
	var deleted bool
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "tenant-test", r.Header.Get("X-Tenant-ID"))
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/articles/12":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":12,"tenant_id":"tenant-test","authorizer_id":1,"title":"published","status":"published"}`))
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

func TestMCPServerRequiresConfirmationBeforeDeletingPermanentMaterial(t *testing.T) {
	var deleteRequests int
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodDelete, r.Method)
		require.Equal(t, "/api/v1/accounts/7/permanent-materials/media-1", r.URL.Path)
		deleteRequests++
		_, _ = w.Write([]byte(`{"deleted":true,"media_id":"media-1"}`))
	}))
	defer apiServer.Close()

	httpClient, err := NewClient(Config{BaseURL: apiServer.URL, TenantID: "tenant-test"})
	require.NoError(t, err)
	server := NewServer(httpClient, ServerConfig{})
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go func() { _ = server.Run(ctx, serverTransport) }()
	mcpClient := mcp.NewClient(&mcp.Implementation{Name: "official-account-test", Version: "0.1.0"}, nil)
	session, err := mcpClient.Connect(ctx, clientTransport, nil)
	require.NoError(t, err)
	defer session.Close()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "official_account_delete_permanent_material",
		Arguments: map[string]any{
			"authorizer_id": 7,
			"media_id":      "media-1",
		},
	})
	require.True(t, err != nil || result.IsError)
	require.Zero(t, deleteRequests)

	result, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name: "official_account_delete_permanent_material",
		Arguments: map[string]any{
			"authorizer_id":  7,
			"media_id":       "media-1",
			"confirm_delete": "DELETE",
		},
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.Equal(t, 1, deleteRequests)
}
