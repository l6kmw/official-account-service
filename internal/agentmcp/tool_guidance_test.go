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

func TestMCPToolGuidesCoverEveryTool(t *testing.T) {
	client, err := NewClient(Config{BaseURL: "https://unused.example", TenantID: "tenant-test"})
	require.NoError(t, err)
	ctx, session := newToolGuidanceSession(t, client)

	result, err := session.ListTools(ctx, &mcp.ListToolsParams{})
	require.NoError(t, err)
	require.Len(t, result.Tools, len(officialAccountToolGuides))
	for _, tool := range result.Tools {
		require.Contains(t, officialAccountToolGuides, tool.Name)
		require.Contains(t, tool.Description, "调用前：")
		require.Contains(t, tool.Description, "输入：")
		require.Contains(t, tool.Description, "返回：")
		require.Contains(t, tool.Description, "后续：")
		require.Contains(t, tool.Description, "不要猜测")
	}
}

func TestMCPToolGuidanceReportsAllMissingFields(t *testing.T) {
	client, err := NewClient(Config{BaseURL: "https://unused.example", TenantID: "tenant-test"})
	require.NoError(t, err)
	ctx, session := newToolGuidanceSession(t, client)

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "official_account_create_draft",
		Arguments: map[string]any{},
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
	text := toolGuidanceResultText(t, result)
	require.Contains(t, text, "共 2 项")
	require.Contains(t, text, "`authorizer_id`")
	require.Contains(t, text, "official_account_list_accounts")
	require.Contains(t, text, "`title`")
	require.Contains(t, text, "未提供")
	requireToolGuidanceErrorCode(t, result, "invalid_tool_arguments")
}

func TestMCPToolGuidanceReportsUnknownTypesAndRangesTogether(t *testing.T) {
	client, err := NewClient(Config{BaseURL: "https://unused.example", TenantID: "tenant-test"})
	require.NoError(t, err)
	ctx, session := newToolGuidanceSession(t, client)

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "official_account_list_published_articles",
		Arguments: map[string]any{
			"authorizer_id":   -1,
			"offset":          -2,
			"count":           30,
			"include_content": "yes",
			"tenant_id":       "other-user",
		},
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
	text := toolGuidanceResultText(t, result)
	for _, field := range []string{"authorizer_id", "offset", "count", "include_content", "tenant_id"} {
		require.Contains(t, text, "`"+field+"`")
	}
	require.Contains(t, text, "字段名不受支持")
	require.Contains(t, text, "1-20")
	require.Contains(t, text, "布尔值 true 或 false")
	require.NotContains(t, text, "other-user")
}

func TestMCPToolGuidanceRedactsImageInputsAndExplainsMutualExclusion(t *testing.T) {
	client, err := NewClient(Config{BaseURL: "https://unused.example", TenantID: "tenant-test"})
	require.NoError(t, err)
	ctx, session := newToolGuidanceSession(t, client)

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "official_account_upload_image",
		Arguments: map[string]any{
			"authorizer_id":  1,
			"article_id":     2,
			"usage":          "body",
			"content_base64": 12345,
			"image_url":      "http://127.0.0.1/private.png?token=secret",
		},
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
	text := toolGuidanceResultText(t, result)
	require.Contains(t, text, "`usage`")
	require.Contains(t, text, "inline_image")
	require.Contains(t, text, "`file_path|content_base64|image_url`")
	require.Contains(t, text, "必须且只能提供一个")
	require.Contains(t, text, "内容已隐藏")
	require.NotContains(t, text, "127.0.0.1")
	require.NotContains(t, text, "token=secret")
	require.NotContains(t, text, "12345")
}

func TestMCPToolGuidanceRequiresExplicitPublishConfirmation(t *testing.T) {
	client, err := NewClient(Config{BaseURL: "https://unused.example", TenantID: "tenant-test"})
	require.NoError(t, err)
	ctx, session := newToolGuidanceSession(t, client)

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "official_account_publish_draft",
		Arguments: map[string]any{
			"draft_id":        12,
			"confirm_publish": false,
		},
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
	text := toolGuidanceResultText(t, result)
	require.Contains(t, text, "`confirm_publish`")
	require.Contains(t, text, "布尔值 true")
	require.Contains(t, text, "不要替用户自动确认")
}

func TestMCPToolGuidanceEnhancesBackendErrors(t *testing.T) {
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/articles/99", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not_found"}`))
	}))
	defer apiServer.Close()

	client, err := NewClient(Config{BaseURL: apiServer.URL, TenantID: "tenant-test"})
	require.NoError(t, err)
	ctx, session := newToolGuidanceSession(t, client)
	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "official_account_get_draft",
		Arguments: map[string]any{"draft_id": 99},
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
	text := toolGuidanceResultText(t, result)
	require.Contains(t, text, "资源不存在、已被删除")
	require.Contains(t, text, "official_account_list_drafts")
	requireToolGuidanceErrorCode(t, result, "resource_not_found")
}

func TestMCPToolGuidanceValidatesMetricsDateInChinaTimezone(t *testing.T) {
	now := time.Date(2026, 7, 15, 17, 0, 0, 0, time.UTC) // 2026-07-16 01:00 in China.
	require.True(t, validMetricsDate("2026-07-15", now))
	require.False(t, validMetricsDate("2026-07-16", now))
	require.False(t, validMetricsDate("2025-10-31", now))
	require.False(t, validMetricsDate("2026/07/15", now))
}

func newToolGuidanceSession(t *testing.T, client *Client) (context.Context, *mcp.ClientSession) {
	t.Helper()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	server := NewServer(client, ServerConfig{})
	go func() { _ = server.Run(ctx, serverTransport) }()
	mcpClient := mcp.NewClient(&mcp.Implementation{Name: "tool-guidance-test", Version: "0.1.0"}, nil)
	session, err := mcpClient.Connect(ctx, clientTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = session.Close()
		cancel()
	})
	return ctx, session
}

func toolGuidanceResultText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	require.NotEmpty(t, result.Content)
	text, ok := result.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	return text.Text
}

func requireToolGuidanceErrorCode(t *testing.T, result *mcp.CallToolResult, expected string) {
	t.Helper()
	raw, err := json.Marshal(result.StructuredContent)
	require.NoError(t, err)
	require.Contains(t, string(raw), `"code":"`+expected+`"`)
}
