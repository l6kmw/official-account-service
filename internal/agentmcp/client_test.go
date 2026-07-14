package agentmcp

import (
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClientCreatesArticleWithTenantAndAdminHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/api/v1/articles", r.URL.Path)
		require.Equal(t, "tenant-test", r.Header.Get("X-Tenant-ID"))
		require.Equal(t, "admin-key", r.Header.Get("X-Admin-API-Key"))
		var body CreateArticleInput
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		require.Equal(t, int64(7), body.AuthorizerID)
		require.Equal(t, "hello", body.Title)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":12,"tenant_id":"tenant-test","authorizer_id":7,"title":"hello","status":"draft"}`))
	}))
	defer server.Close()
	client, err := NewClient(Config{BaseURL: server.URL, TenantID: "tenant-test", AdminAPIKey: "admin-key"})
	require.NoError(t, err)

	article, err := client.CreateArticle(context.Background(), CreateArticleInput{AuthorizerID: 7, Title: "hello"})
	require.NoError(t, err)
	require.Equal(t, int64(12), article.ID)
	require.Equal(t, "draft", article.Status)
}

func TestClientListsLivePublishedArticles(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/accounts/7/published-articles", r.URL.Path)
		require.Equal(t, "2", r.URL.Query().Get("offset"))
		require.Equal(t, "10", r.URL.Query().Get("count"))
		require.Equal(t, "true", r.URL.Query().Get("include_content"))
		require.Equal(t, "false", r.URL.Query().Get("include_deleted"))
		_, _ = w.Write([]byte(`{"total_message_count":3,"fetched_message_count":1,"returned_article_count":1,"next_offset":3,"has_more":false,"items":[{"article_id":"wx-1","msgid":"100_1","title":"live"}]}`))
	}))
	defer server.Close()
	client, err := NewClient(Config{BaseURL: server.URL, TenantID: "tenant-test"})
	require.NoError(t, err)

	result, err := client.ListPublishedArticles(context.Background(), 7, 2, 10, true, false)
	require.NoError(t, err)
	require.Equal(t, "wx-1", result.Items[0].ArticleID)
	require.Equal(t, "100_1", result.Items[0].MsgID)
	require.Equal(t, 3, result.NextOffset)
}

func TestClientGetsArticleMetrics(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/accounts/7/article-metrics", r.URL.Path)
		require.Equal(t, "2026-07-10", r.URL.Query().Get("date"))
		_, _ = w.Write([]byte(`{"date":"2026-07-10","delayed":false,"items":[{"msgid":"100_1","title":"live","detail_list":[{"read_user":42,"like_user":5}]}]}`))
	}))
	defer server.Close()
	client, err := NewClient(Config{BaseURL: server.URL, TenantID: "tenant-test"})
	require.NoError(t, err)

	result, err := client.GetArticleMetrics(context.Background(), 7, "2026-07-10")
	require.NoError(t, err)
	require.Equal(t, int64(42), result.Items[0].Details[0].ReadUser)
	require.Equal(t, int64(5), result.Items[0].Details[0].LikeUser)
}

func TestClientListsArticleComments(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/accounts/7/article-comments", r.URL.Path)
		require.Equal(t, "100_2", r.URL.Query().Get("msgid"))
		require.Equal(t, "1", r.URL.Query().Get("begin"))
		require.Equal(t, "10", r.URL.Query().Get("count"))
		require.Equal(t, "2", r.URL.Query().Get("type"))
		_, _ = w.Write([]byte(`{"msgid":"100_2","msg_data_id":100,"article_index":1,"total":1,"returned_count":1,"next_begin":2,"has_more":false,"items":[{"user_comment_id":9,"content":"hello","selected":true}]}`))
	}))
	defer server.Close()
	client, err := NewClient(Config{BaseURL: server.URL, TenantID: "tenant-test"})
	require.NoError(t, err)

	result, err := client.ListArticleComments(context.Background(), 7, "100_2", 1, 10, 2)
	require.NoError(t, err)
	require.Equal(t, "hello", result.Items[0].Content)
	require.True(t, result.Items[0].Selected)
}

func TestClientDeletesDraftArticle(t *testing.T) {
	var deleted bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "tenant-test", r.Header.Get("X-Tenant-ID"))
		require.Equal(t, "admin-key", r.Header.Get("X-Admin-API-Key"))
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/articles/12":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":12,"tenant_id":"tenant-test","authorizer_id":7,"title":"hello","status":"draft"}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/articles/12":
			deleted = true
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()
	client, err := NewClient(Config{BaseURL: server.URL, TenantID: "tenant-test", AdminAPIKey: "admin-key"})
	require.NoError(t, err)

	article, err := client.DeleteArticle(context.Background(), 12)

	require.NoError(t, err)
	require.True(t, deleted)
	require.Equal(t, int64(12), article.ID)
	require.Equal(t, "draft", article.Status)
}

func TestClientDeletesPublishedArticleViaBackend(t *testing.T) {
	var deleteRequests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "tenant-test", r.Header.Get("X-Tenant-ID"))
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/articles/12":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":12,"tenant_id":"tenant-test","authorizer_id":7,"title":"hello","status":"published"}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/articles/12":
			deleteRequests++
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()
	client, err := NewClient(Config{BaseURL: server.URL, TenantID: "tenant-test"})
	require.NoError(t, err)

	article, err := client.DeleteArticle(context.Background(), 12)

	require.NoError(t, err)
	require.Equal(t, "published", article.Status)
	require.Equal(t, 1, deleteRequests)
}

func TestClientUploadsImageAsMultipart(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/api/v1/materials/covers", r.URL.Path)
		reader, err := r.MultipartReader()
		require.NoError(t, err)
		fields := readMultipartFields(t, reader)
		require.Equal(t, "7", fields["authorizer_id"])
		require.Equal(t, "12", fields["article_id"])
		require.Equal(t, "image-bytes", fields["file"])
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":33,"tenant_id":"tenant-test","authorizer_id":7,"article_id":12,"usage":"cover","media_id":"media-1"}`))
	}))
	defer server.Close()
	client, err := NewClient(Config{BaseURL: server.URL, TenantID: "tenant-test"})
	require.NoError(t, err)

	asset, err := client.UploadImage(context.Background(), UploadImageInput{
		AuthorizerID: 7,
		ArticleID:    12,
		Usage:        "cover",
		Filename:     "cover.png",
		Content:      strings.NewReader("image-bytes"),
	})
	require.NoError(t, err)
	require.Equal(t, "media-1", asset.MediaID)
}

func TestOpenUploadContentHonorsAllowedRoot(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "cover.png")
	require.NoError(t, os.WriteFile(inside, []byte("image"), 0o600))
	content, filename, err := openUploadContent(context.Background(), uploadImageToolInput{
		Usage:    "cover",
		FilePath: inside,
	}, root, nil)
	require.NoError(t, err)
	defer content.Close()
	require.Equal(t, "cover.png", filename)

	_, _, err = openUploadContent(context.Background(), uploadImageToolInput{
		Usage:    "cover",
		FilePath: filepath.Join(t.TempDir(), "outside.png"),
	}, root, nil)
	require.ErrorContains(t, err, "OFFICIAL_ACCOUNT_MCP_ALLOWED_ROOT")
}

func TestAuthorizationEntryUsesConfiguredPublicURL(t *testing.T) {
	client, err := NewClient(Config{
		BaseURL:        "https://mp.example.com",
		PublicBaseURL:  "https://public.example.com/",
		TenantID:       "tenant-test",
		ComponentAppID: "wx-component",
	})
	require.NoError(t, err)

	entry, err := client.AuthorizationEntry("")
	require.NoError(t, err)
	require.Equal(t, "tenant-test", entry.TenantID)
	require.Equal(t, "wx-component", entry.ComponentAppID)
	require.Equal(t, "https://public.example.com/wechat-authorize.html?component_appid=wx-component&tenant_id=tenant-test", entry.AuthorizationEntryURL)
	require.Equal(t, entry.AuthorizationEntryURL, entry.QRCodePayloadURL)
}

func TestGenerateAuthorizationURLUsesConfiguredDefaults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/wechat/authorization-url", r.URL.Path)
		require.Equal(t, "tenant-test", r.URL.Query().Get("tenant_id"))
		require.Equal(t, "wx-component", r.URL.Query().Get("component_appid"))
		require.Equal(t, "https://public.example.com/api/v1/wechat/authorization-callback", r.URL.Query().Get("redirect_uri"))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"authorization_url":"https://mp.weixin.qq.com/authorize","pre_auth_code_expires_in_sec":600}`))
	}))
	defer server.Close()
	client, err := NewClient(Config{
		BaseURL:        server.URL,
		PublicBaseURL:  "https://public.example.com",
		TenantID:       "tenant-test",
		ComponentAppID: "wx-component",
	})
	require.NoError(t, err)

	result, err := client.GenerateAuthorizationURL(context.Background(), "", "", 0, "")
	require.NoError(t, err)
	require.Equal(t, "https://mp.weixin.qq.com/authorize", result.AuthorizationURL)
	require.Equal(t, 600, result.PreAuthCodeExpiresInSec)
}

func TestOpenUploadContentDecodesBase64(t *testing.T) {
	content, filename, err := openUploadContent(context.Background(), uploadImageToolInput{
		Usage:         "inline_image",
		Filename:      "body.png",
		ContentBase64: "aW1hZ2U=",
	}, "", nil)
	require.NoError(t, err)
	defer content.Close()
	raw, err := io.ReadAll(content)
	require.NoError(t, err)
	require.Equal(t, "image", string(raw))
	require.Equal(t, "body.png", filename)
}

func readMultipartFields(t *testing.T, reader *multipart.Reader) map[string]string {
	t.Helper()
	fields := make(map[string]string)
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			return fields
		}
		require.NoError(t, err)
		raw, err := io.ReadAll(part)
		require.NoError(t, err)
		fields[part.FormName()] = string(raw)
	}
}
