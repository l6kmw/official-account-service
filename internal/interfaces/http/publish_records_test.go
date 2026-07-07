package http

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPublishRecordRoutes(t *testing.T) {
	router := testRouter()

	article := doJSON(t, router, http.MethodPost, "/api/v1/articles", `{"authorizer_id":1,"title":"hello"}`, "tenant-1")
	require.Equal(t, http.StatusCreated, article.Code)

	created := doJSON(t, router, http.MethodPost, "/api/v1/publish-records", `{"article_id":1,"wechat_publish_id":"pub-1"}`, "tenant-1")
	require.Equal(t, http.StatusCreated, created.Code)
	require.Contains(t, created.Body.String(), `"status":"publishing"`)

	list := doJSON(t, router, http.MethodGet, "/api/v1/articles/1/publish-records", ``, "tenant-1")
	require.Equal(t, http.StatusOK, list.Code)
	require.Contains(t, list.Body.String(), `"wechat_publish_id":"pub-1"`)

	adminList := doJSON(t, router, http.MethodGet, "/api/v1/publish-records", ``, "tenant-1")
	require.Equal(t, http.StatusOK, adminList.Code)
	require.Contains(t, adminList.Body.String(), `"wechat_publish_id":"pub-1"`)

	got := doJSON(t, router, http.MethodGet, "/api/v1/publish-records/1", ``, "tenant-1")
	require.Equal(t, http.StatusOK, got.Code)
	require.Contains(t, got.Body.String(), `"article_id":1`)

	updated := doJSON(t, router, http.MethodPut, "/api/v1/publish-records/1/status", `{"status":"published","wechat_article_id":"article-1"}`, "tenant-1")
	require.Equal(t, http.StatusOK, updated.Code)
	require.Contains(t, updated.Body.String(), `"status":"published"`)

	gotArticle := doJSON(t, router, http.MethodGet, "/api/v1/articles/1", ``, "tenant-1")
	require.Equal(t, http.StatusOK, gotArticle.Code)
	require.Contains(t, gotArticle.Body.String(), `"status":"published"`)

	otherTenant := doJSON(t, router, http.MethodGet, "/api/v1/articles/1/publish-records", ``, "tenant-2")
	require.Equal(t, http.StatusOK, otherTenant.Code)
	require.Contains(t, otherTenant.Body.String(), `"items":[]`)
	otherTenantList := doJSON(t, router, http.MethodGet, "/api/v1/publish-records", ``, "tenant-2")
	require.Equal(t, http.StatusOK, otherTenantList.Code)
	require.Contains(t, otherTenantList.Body.String(), `"items":[]`)
	otherTenantGet := doJSON(t, router, http.MethodGet, "/api/v1/publish-records/1", ``, "tenant-2")
	require.Equal(t, http.StatusNotFound, otherTenantGet.Code)
	missingTenant := doJSON(t, router, http.MethodGet, "/api/v1/publish-records", ``, "")
	require.Equal(t, http.StatusBadRequest, missingTenant.Code)
}
