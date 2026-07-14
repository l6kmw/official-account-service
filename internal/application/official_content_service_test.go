package application

import (
	"context"
	"errors"
	"testing"

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

type fakeOfficialContentReader struct {
	batch          officialcontent.PublishedArticleBatch
	accessToken    string
	offset         int
	count          int
	includeContent bool
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
