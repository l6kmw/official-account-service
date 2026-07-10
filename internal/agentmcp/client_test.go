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
	content, filename, err := openUploadContent(uploadImageToolInput{
		Usage:    "cover",
		FilePath: inside,
	}, root)
	require.NoError(t, err)
	defer content.Close()
	require.Equal(t, "cover.png", filename)

	_, _, err = openUploadContent(uploadImageToolInput{
		Usage:    "cover",
		FilePath: filepath.Join(t.TempDir(), "outside.png"),
	}, root)
	require.ErrorContains(t, err, "OFFICIAL_ACCOUNT_MCP_ALLOWED_ROOT")
}

func TestOpenUploadContentDecodesBase64(t *testing.T) {
	content, filename, err := openUploadContent(uploadImageToolInput{
		Usage:         "inline_image",
		Filename:      "body.png",
		ContentBase64: "aW1hZ2U=",
	}, "")
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
