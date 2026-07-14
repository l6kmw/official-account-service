package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"official-account-service/internal/domain/officialcontent"
)

func TestOfficialContentServiceListsAndFiltersPublishedArticles(t *testing.T) {
	reader := &fakeOfficialContentReader{batch: officialcontent.PublishedArticleBatch{
		TotalMessageCount:   5,
		FetchedMessageCount: 2,
		Items: []officialcontent.PublishedArticle{
			{ArticleID: "article-1", Index: 0, Title: "visible"},
			{ArticleID: "article-1", Index: 1, Title: "deleted", Deleted: true},
		},
	}}
	tokens := &fakeOfficialContentTokenProvider{token: AuthorizerAccessToken{AccessToken: "secret-token"}}
	service := NewOfficialContentService(reader, tokens, "wx-component")

	result, err := service.ListPublishedArticles(context.Background(), ListPublishedArticlesInput{
		TenantID: "tenant-1", AuthorizerID: 7, Offset: 1, Count: 2,
	})
	require.NoError(t, err)

	require.Equal(t, "tenant-1", tokens.input.TenantID)
	require.Equal(t, int64(7), tokens.input.AccountID)
	require.Equal(t, "wx-component", tokens.input.ComponentAppID)
	require.Equal(t, "secret-token", reader.accessToken)
	require.Equal(t, 1, reader.offset)
	require.Equal(t, 2, reader.count)
	require.False(t, reader.includeContent)
	require.Equal(t, 3, result.NextOffset)
	require.True(t, result.HasMore)
	require.Equal(t, 1, result.ReturnedArticleCount)
	require.Equal(t, "visible", result.Items[0].Title)
}

func TestOfficialContentServiceValidatesPublishedArticlePagination(t *testing.T) {
	service := NewOfficialContentService(&fakeOfficialContentReader{}, &fakeOfficialContentTokenProvider{}, "wx-component")

	_, err := service.ListPublishedArticles(context.Background(), ListPublishedArticlesInput{
		TenantID: "tenant-1", AuthorizerID: 1, Count: 21,
	})

	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))
}

func TestOfficialContentServiceGetsArticleMetrics(t *testing.T) {
	reader := &fakeOfficialContentReader{metrics: officialcontent.ArticleMetricsResult{
		Date: "2026-07-10", Items: []officialcontent.ArticleMetrics{{MsgID: "100_1", Title: "article"}},
	}}
	tokens := &fakeOfficialContentTokenProvider{token: AuthorizerAccessToken{AccessToken: "secret-token"}}
	service := NewOfficialContentService(reader, tokens, "wx-component")
	service.now = func() time.Time { return time.Date(2026, 7, 14, 12, 0, 0, 0, time.Local) }

	result, err := service.GetArticleMetrics(context.Background(), GetArticleMetricsInput{
		TenantID: "tenant-1", AuthorizerID: 7, Date: "2026-07-10",
	})

	require.NoError(t, err)
	require.Equal(t, "secret-token", reader.metricsAccessToken)
	require.Equal(t, "2026-07-10", reader.metricsDate)
	require.Equal(t, "100_1", result.Items[0].MsgID)
}

func TestOfficialContentServiceRejectsTodayMetrics(t *testing.T) {
	service := NewOfficialContentService(&fakeOfficialContentReader{}, &fakeOfficialContentTokenProvider{}, "wx-component")
	west := time.FixedZone("test-west", -8*60*60)
	service.now = func() time.Time { return time.Date(2026, 7, 14, 12, 0, 0, 0, west) }

	_, err := service.GetArticleMetrics(context.Background(), GetArticleMetricsInput{
		TenantID: "tenant-1", AuthorizerID: 1, Date: "2026-07-14",
	})

	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))
}

func TestOfficialContentServiceListsArticleCommentsAndConvertsIndex(t *testing.T) {
	reader := &fakeOfficialContentReader{comments: officialcontent.ArticleCommentBatch{
		Total: 3, Items: []officialcontent.ArticleComment{{UserCommentID: 1, Content: "hello"}},
	}}
	tokens := &fakeOfficialContentTokenProvider{token: AuthorizerAccessToken{AccessToken: "secret-token"}}
	service := NewOfficialContentService(reader, tokens, "wx-component")

	result, err := service.ListArticleComments(context.Background(), ListArticleCommentsInput{
		TenantID: "tenant-1", AuthorizerID: 7, MsgID: "2247490098_2", Begin: 1, Count: 10, Type: 2,
	})

	require.NoError(t, err)
	require.Equal(t, int64(2247490098), reader.commentsMsgDataID)
	require.Equal(t, 1, reader.commentsIndex)
	require.Equal(t, 2, reader.commentsType)
	require.Equal(t, 2, result.NextBegin)
	require.True(t, result.HasMore)
}

func TestOfficialContentServiceRejectsInvalidCommentMsgID(t *testing.T) {
	service := NewOfficialContentService(&fakeOfficialContentReader{}, &fakeOfficialContentTokenProvider{}, "wx-component")

	_, err := service.ListArticleComments(context.Background(), ListArticleCommentsInput{
		TenantID: "tenant-1", AuthorizerID: 1, MsgID: "not-a-msgid", Count: 20,
	})

	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))
}

type fakeOfficialContentReader struct {
	batch              officialcontent.PublishedArticleBatch
	metrics            officialcontent.ArticleMetricsResult
	comments           officialcontent.ArticleCommentBatch
	accessToken        string
	offset             int
	count              int
	includeContent     bool
	metricsAccessToken string
	metricsDate        string
	commentsMsgDataID  int64
	commentsIndex      int
	commentsType       int
}

func (f *fakeOfficialContentReader) GetArticleMetrics(_ context.Context, accessToken, date string) (officialcontent.ArticleMetricsResult, error) {
	f.metricsAccessToken = accessToken
	f.metricsDate = date
	return f.metrics, nil
}

func (f *fakeOfficialContentReader) ListArticleComments(_ context.Context, _ string, msgDataID int64, articleIndex, _, _, commentType int) (officialcontent.ArticleCommentBatch, error) {
	f.commentsMsgDataID = msgDataID
	f.commentsIndex = articleIndex
	f.commentsType = commentType
	return f.comments, nil
}

func (f *fakeOfficialContentReader) ListPublishedArticles(_ context.Context, accessToken string, offset, count int, includeContent bool) (officialcontent.PublishedArticleBatch, error) {
	f.accessToken = accessToken
	f.offset = offset
	f.count = count
	f.includeContent = includeContent
	return f.batch, nil
}

type fakeOfficialContentTokenProvider struct {
	token AuthorizerAccessToken
	input RefreshAuthorizerAccessTokenInput
}

func (f *fakeOfficialContentTokenProvider) GetAuthorizerAccessToken(_ context.Context, input RefreshAuthorizerAccessTokenInput) (AuthorizerAccessToken, error) {
	f.input = input
	return f.token, nil
}
