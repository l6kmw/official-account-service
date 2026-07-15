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
	require.Contains(t, toolsByName, "official_account_get_identity")
	require.Contains(t, toolsByName, "official_account_publish_article")
	require.Contains(t, toolsByName, "official_account_delete_article")
	require.Contains(t, toolsByName, "official_account_list_published_articles")
	require.Contains(t, toolsByName, "official_account_list_permanent_materials")
	require.Contains(t, toolsByName, "official_account_delete_permanent_material")
	require.Contains(t, toolsByName, "official_account_get_article_metrics")
	require.Contains(t, toolsByName, "official_account_list_article_comments")
	require.Contains(t, toolsByName, "official_account_get_authorization_entry")
	require.Contains(t, toolsByName, "official_account_list_drafts")
	require.Contains(t, toolsByName, "official_account_get_draft")
	require.Contains(t, toolsByName, "official_account_create_draft")
	require.Contains(t, toolsByName, "official_account_update_draft")
	require.Contains(t, toolsByName, "official_account_delete_draft")
	require.Contains(t, toolsByName, "official_account_publish_draft")
	require.Empty(t, requiredToolFields(t, toolsByName["official_account_get_identity"]))
	require.ElementsMatch(t, []string{"draft_id"}, requiredToolFields(t, toolsByName["official_account_get_draft"]))
	require.ElementsMatch(t, []string{"authorizer_id", "title"}, requiredToolFields(t, toolsByName["official_account_create_draft"]))
	require.ElementsMatch(t, []string{"draft_id", "version", "title", "author", "digest", "content_html", "cover_media_asset_id"}, requiredToolFields(t, toolsByName["official_account_update_draft"]))
	require.ElementsMatch(t, []string{"draft_id", "confirm_delete"}, requiredToolFields(t, toolsByName["official_account_delete_draft"]))
	require.ElementsMatch(t, []string{"draft_id", "confirm_publish"}, requiredToolFields(t, toolsByName["official_account_publish_draft"]))
	require.ElementsMatch(t, []string{"authorizer_id", "title"}, requiredToolFields(t, toolsByName["official_account_create_article"]))
	require.ElementsMatch(t, []string{"article_id", "version", "title", "author", "digest", "content_html", "cover_media_asset_id"}, requiredToolFields(t, toolsByName["official_account_update_article"]))
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

func TestMCPServerReturnsSafeAuthenticatedIdentity(t *testing.T) {
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/api/v1/admin/session", r.URL.Path)
		require.Equal(t, "admin-key", r.Header.Get("X-Admin-API-Key"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"authenticated":true,
			"username":"writer",
			"user_id":"user-1",
			"role":"user",
			"actor_type":"agent_token",
			"agent_record_id":"agent-record-a",
			"agent_id":"writer-a",
			"agent_name":"Writer A",
			"agent_purpose":"industry analysis",
			"api_token":"must-not-leak",
			"api_token_hint":"must-not-leak",
			"api_token_hash":"must-not-leak"
		}`))
	}))
	defer apiServer.Close()

	httpClient, err := NewClient(Config{BaseURL: apiServer.URL, TenantID: "tenant-test", AdminAPIKey: "admin-key"})
	require.NoError(t, err)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go func() { _ = NewServer(httpClient, ServerConfig{}).Run(ctx, serverTransport) }()
	session, err := mcp.NewClient(&mcp.Implementation{Name: "identity-test", Version: "0.1.0"}, nil).Connect(ctx, clientTransport, nil)
	require.NoError(t, err)
	defer session.Close()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "official_account_get_identity", Arguments: map[string]any{}})
	require.NoError(t, err)
	require.False(t, result.IsError)
	raw, err := json.Marshal(result.StructuredContent)
	require.NoError(t, err)
	text := string(raw)
	for _, expected := range []string{"writer", "user-1", "agent_token", "agent-record-a", "writer-a", "Writer A", "industry analysis"} {
		require.Contains(t, text, expected)
	}
	for _, forbidden := range []string{"api_token", "api_token_hint", "api_token_hash", "must-not-leak"} {
		require.NotContains(t, text, forbidden)
	}
}

func TestMCPServerManagesLocalDraftLifecycle(t *testing.T) {
	var created, updated, deleted, published bool
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "tenant-test", r.Header.Get("X-Tenant-ID"))
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/articles":
			_, _ = w.Write([]byte(`{"items":[{"id":12,"title":"draft","status":"draft"},{"id":14,"title":"retry","status":"failed"},{"id":15,"title":"live","status":"published"}]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/articles/12":
			_, _ = w.Write([]byte(`{"id":12,"authorizer_id":7,"title":"draft","content_html":"<p>body</p>","cover_media_asset_id":8,"status":"draft","version":3}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/articles/14":
			_, _ = w.Write([]byte(`{"id":14,"authorizer_id":7,"title":"retry","status":"failed"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/articles/15":
			_, _ = w.Write([]byte(`{"id":15,"authorizer_id":7,"title":"live","status":"published"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/articles":
			var body CreateArticleInput
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			require.Equal(t, int64(7), body.AuthorizerID)
			require.Equal(t, "new draft", body.Title)
			created = true
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":16,"authorizer_id":7,"title":"new draft","status":"draft"}`))
		case r.Method == http.MethodPut && r.URL.Path == "/api/v1/articles/12":
			var body UpdateArticleInput
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			require.Equal(t, "updated draft", body.Title)
			require.Equal(t, int64(8), body.CoverMediaAssetID)
			require.Equal(t, int64(3), body.Version)
			updated = true
			_, _ = w.Write([]byte(`{"id":12,"authorizer_id":7,"title":"updated draft","content_html":"<p>updated</p>","cover_media_asset_id":8,"status":"draft","version":4}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/articles/12/publish":
			published = true
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":21,"article_id":12,"status":"publishing"}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/articles/14":
			deleted = true
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer apiServer.Close()

	httpClient, err := NewClient(Config{BaseURL: apiServer.URL, TenantID: "tenant-test"})
	require.NoError(t, err)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go func() { _ = NewServer(httpClient, ServerConfig{}).Run(ctx, serverTransport) }()
	session, err := mcp.NewClient(&mcp.Implementation{Name: "draft-test", Version: "0.1.0"}, nil).Connect(ctx, clientTransport, nil)
	require.NoError(t, err)
	defer session.Close()

	list, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "official_account_list_drafts", Arguments: map[string]any{}})
	require.NoError(t, err)
	require.False(t, list.IsError)
	listJSON, err := json.Marshal(list.StructuredContent)
	require.NoError(t, err)
	require.Contains(t, string(listJSON), `"id":12`)
	require.Contains(t, string(listJSON), `"id":14`)
	require.NotContains(t, string(listJSON), `"id":15`)

	detail, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "official_account_get_draft", Arguments: map[string]any{"draft_id": 12}})
	require.NoError(t, err)
	require.False(t, detail.IsError)
	require.Contains(t, detail.StructuredContent, "draft")

	rejected, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "official_account_get_draft", Arguments: map[string]any{"draft_id": 15}})
	require.True(t, err != nil || rejected.IsError)

	createdResult, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "official_account_create_draft", Arguments: map[string]any{"authorizer_id": 7, "title": "new draft"}})
	require.NoError(t, err)
	require.False(t, createdResult.IsError)

	updatedResult, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "official_account_update_draft", Arguments: map[string]any{
		"draft_id": 12, "version": 3, "title": "updated draft", "author": "", "digest": "", "content_html": "<p>updated</p>", "cover_media_asset_id": 8,
	}})
	require.NoError(t, err)
	require.False(t, updatedResult.IsError)

	unconfirmedPublish, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "official_account_publish_draft", Arguments: map[string]any{"draft_id": 12, "confirm_publish": false}})
	require.True(t, err != nil || unconfirmedPublish.IsError)
	require.False(t, published)
	publishedResult, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "official_account_publish_draft", Arguments: map[string]any{"draft_id": 12, "confirm_publish": true}})
	require.NoError(t, err)
	require.False(t, publishedResult.IsError)

	unconfirmedDelete, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "official_account_delete_draft", Arguments: map[string]any{"draft_id": 14, "confirm_delete": ""}})
	require.True(t, err != nil || unconfirmedDelete.IsError)
	require.False(t, deleted)
	deletedResult, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "official_account_delete_draft", Arguments: map[string]any{"draft_id": 14, "confirm_delete": "DELETE"}})
	require.NoError(t, err)
	require.False(t, deletedResult.IsError)

	require.True(t, created)
	require.True(t, updated)
	require.True(t, published)
	require.True(t, deleted)
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
