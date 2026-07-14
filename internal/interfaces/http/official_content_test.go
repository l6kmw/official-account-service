package http

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"official-account-service/internal/application"
	"official-account-service/internal/domain/officialcontent"
)

func TestPublishedArticlesRouteReturnsLiveWechatPage(t *testing.T) {
	reader := routeFakeOfficialContentReader{batch: officialcontent.PublishedArticleBatch{
		TotalMessageCount: 1, FetchedMessageCount: 1,
		Items: []officialcontent.PublishedArticle{{ArticleID: "article-1", Title: "live"}},
	}}
	service := application.NewOfficialContentService(reader, routeFakeOfficialContentTokenProvider{}, "wx-component")
	router := NewRouter(Dependencies{Logger: zap.NewNop(), OfficialContent: service})

	recorder := doJSON(t, router, http.MethodGet, "/api/v1/accounts/7/published-articles?offset=0&count=20&include_content=false", ``, "tenant-1")

	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"total_message_count":1,"fetched_message_count":1,"returned_article_count":1,"next_offset":1,"has_more":false,"items":[{"article_id":"article-1","index":0,"update_time":0,"title":"live","author":"","digest":"","content_source_url":"","thumb_media_id":"","thumb_url":"","url":"","need_open_comment":false,"only_fans_can_comment":false,"deleted":false}]}`, recorder.Body.String())

	bad := doJSON(t, router, http.MethodGet, "/api/v1/accounts/7/published-articles?count=bad", ``, "tenant-1")
	require.Equal(t, http.StatusBadRequest, bad.Code)
}

func TestArticleMetricsRouteReturnsLiveWechatData(t *testing.T) {
	reader := routeFakeOfficialContentReader{metrics: officialcontent.ArticleMetricsResult{
		Date: "2026-07-10", Items: []officialcontent.ArticleMetrics{{MsgID: "100_1", Title: "live"}},
	}}
	service := application.NewOfficialContentService(reader, routeFakeOfficialContentTokenProvider{}, "wx-component")
	router := NewRouter(Dependencies{Logger: zap.NewNop(), OfficialContent: service})

	recorder := doJSON(t, router, http.MethodGet, "/api/v1/accounts/7/article-metrics?date=2026-07-10", ``, "tenant-1")

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"msgid":"100_1"`)
}

func TestArticleCommentsRouteReturnsDataWithoutOpenID(t *testing.T) {
	reader := routeFakeOfficialContentReader{comments: officialcontent.ArticleCommentBatch{
		Total: 1, Items: []officialcontent.ArticleComment{{UserCommentID: 10, Content: "hello"}},
	}}
	service := application.NewOfficialContentService(reader, routeFakeOfficialContentTokenProvider{}, "wx-component")
	router := NewRouter(Dependencies{Logger: zap.NewNop(), OfficialContent: service})

	recorder := doJSON(t, router, http.MethodGet, "/api/v1/accounts/7/article-comments?msgid=100_1&count=20&type=0", ``, "tenant-1")

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"content":"hello"`)
	require.NotContains(t, recorder.Body.String(), "openid")
}

type routeFakeOfficialContentReader struct {
	batch    officialcontent.PublishedArticleBatch
	metrics  officialcontent.ArticleMetricsResult
	comments officialcontent.ArticleCommentBatch
}

func (f routeFakeOfficialContentReader) GetArticleMetrics(context.Context, string, string) (officialcontent.ArticleMetricsResult, error) {
	return f.metrics, nil
}

func (f routeFakeOfficialContentReader) ListArticleComments(context.Context, string, int64, int, int, int, int) (officialcontent.ArticleCommentBatch, error) {
	return f.comments, nil
}

func (f routeFakeOfficialContentReader) ListPublishedArticles(context.Context, string, int, int, bool) (officialcontent.PublishedArticleBatch, error) {
	return f.batch, nil
}

type routeFakeOfficialContentTokenProvider struct{}

func (routeFakeOfficialContentTokenProvider) GetAuthorizerAccessToken(context.Context, application.RefreshAuthorizerAccessTokenInput) (application.AuthorizerAccessToken, error) {
	return application.AuthorizerAccessToken{AccessToken: "token"}, nil
}
