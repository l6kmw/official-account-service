package http

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"official-account-service/internal/application"
	"official-account-service/internal/domain/authorization"
	"official-account-service/internal/domain/publish"
)

func TestDashboardStatsRoute(t *testing.T) {
	store, router := testRouterWithStore()
	accounts := application.NewAccountService(store)
	articles := application.NewArticleService(store)
	publishes := application.NewPublishService(store, store, fixedRouteTime)
	_, err := accounts.SaveAccount(t.Context(), application.SaveAccountInput{
		TenantID: "tenant-1", AppID: "wx-active", Name: "active", Status: authorization.AccountStatusActive,
	})
	require.NoError(t, err)
	_, err = accounts.SaveAccount(t.Context(), application.SaveAccountInput{
		TenantID: "tenant-2", AppID: "wx-other", Name: "other", Status: authorization.AccountStatusActive,
	})
	require.NoError(t, err)
	draft, err := articles.CreateArticle(t.Context(), application.CreateArticleInput{TenantID: "tenant-1", AuthorizerID: 1, Title: "draft"})
	require.NoError(t, err)
	published, err := articles.CreateArticle(t.Context(), application.CreateArticleInput{TenantID: "tenant-1", AuthorizerID: 1, Title: "published"})
	require.NoError(t, err)
	record, err := publishes.CreatePublishRecord(t.Context(), application.CreatePublishRecordInput{
		TenantID: "tenant-1", ArticleID: published.ID, WeChatPublishID: "publish-1",
	})
	require.NoError(t, err)
	_, err = publishes.UpdatePublishStatus(t.Context(), application.UpdatePublishStatusInput{
		TenantID: "tenant-1", ID: record.ID, Status: publish.StatusPublished,
	})
	require.NoError(t, err)
	require.NotZero(t, draft.ID)

	recorder := doJSON(t, router, http.MethodGet, "/api/v1/dashboard/stats", ``, "tenant-1")
	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{
		"account_total":1,
		"active_account_total":1,
		"revoked_account_total":0,
		"refresh_failed_account_total":0,
		"article_total":2,
		"draft_article_total":1,
		"publishing_article_total":0,
		"published_article_total":1,
		"failed_article_total":0,
		"publish_total":1,
		"publishing_publish_total":0,
		"published_publish_total":1,
		"failed_publish_total":0
	}`, recorder.Body.String())
	require.NotContains(t, recorder.Body.String(), "token")
	require.NotContains(t, recorder.Body.String(), "secret")
}

func TestDashboardStatsRouteValidatesTenantAndDependency(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:    zap.NewNop(),
		Dashboard: application.NewDashboardService(nil, nil, nil),
	})
	missingTenant := doJSON(t, router, http.MethodGet, "/api/v1/dashboard/stats", ``, "")
	require.Equal(t, http.StatusBadRequest, missingTenant.Code)

	unavailable := doJSON(t, NewRouter(Dependencies{Logger: zap.NewNop()}), http.MethodGet, "/api/v1/dashboard/stats", ``, "tenant-1")
	require.Equal(t, http.StatusNotImplemented, unavailable.Code)
}
