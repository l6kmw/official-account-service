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

type routeFakeOfficialContentReader struct {
	batch officialcontent.PublishedArticleBatch
}

func (f routeFakeOfficialContentReader) ListPublishedArticles(context.Context, string, int, int, bool) (officialcontent.PublishedArticleBatch, error) {
	return f.batch, nil
}

type routeFakeOfficialContentTokenProvider struct{}

func (routeFakeOfficialContentTokenProvider) GetAuthorizerAccessToken(context.Context, application.RefreshAuthorizerAccessTokenInput) (application.AuthorizerAccessToken, error) {
	return application.AuthorizerAccessToken{AccessToken: "token"}, nil
}
