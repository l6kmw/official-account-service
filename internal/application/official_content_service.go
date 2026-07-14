package application

import (
	"context"
	"fmt"
	"strings"

	"official-account-service/internal/domain/officialcontent"
)

const defaultPublishedArticlePageSize = 20

// OfficialContentService reads live content owned by an authorized account.
type OfficialContentService struct {
	reader         officialcontent.Reader
	tokens         PublishTokenProvider
	componentAppID string
}

// NewOfficialContentService constructs an OfficialContentService.
func NewOfficialContentService(reader officialcontent.Reader, tokens PublishTokenProvider, componentAppID string) *OfficialContentService {
	return &OfficialContentService{reader: reader, tokens: tokens, componentAppID: strings.TrimSpace(componentAppID)}
}

// ListPublishedArticlesInput selects one WeChat-side published-message page.
type ListPublishedArticlesInput struct {
	TenantID       string
	AuthorizerID   int64
	Offset         int
	Count          int
	IncludeContent bool
	IncludeDeleted bool
}

// ListPublishedArticles reads current published content directly from WeChat.
func (s *OfficialContentService) ListPublishedArticles(ctx context.Context, input ListPublishedArticlesInput) (officialcontent.PublishedArticleList, error) {
	if strings.TrimSpace(input.TenantID) == "" || input.AuthorizerID <= 0 || input.Offset < 0 || input.Count < 0 || input.Count > defaultPublishedArticlePageSize {
		return officialcontent.PublishedArticleList{}, fmt.Errorf("validate published article list input: %w", ErrInvalidInput)
	}
	if s == nil || s.reader == nil || s.tokens == nil || s.componentAppID == "" {
		return officialcontent.PublishedArticleList{}, fmt.Errorf("read published articles: %w", ErrNotImplemented)
	}
	count := input.Count
	if count == 0 {
		count = defaultPublishedArticlePageSize
	}
	token, err := s.tokens.GetAuthorizerAccessToken(ctx, RefreshAuthorizerAccessTokenInput{
		TenantID: input.TenantID, AccountID: input.AuthorizerID, ComponentAppID: s.componentAppID,
	})
	if err != nil {
		return officialcontent.PublishedArticleList{}, fmt.Errorf("get authorizer access token for published articles: %w", err)
	}
	batch, err := s.reader.ListPublishedArticles(ctx, token.AccessToken, input.Offset, count, input.IncludeContent)
	if err != nil {
		return officialcontent.PublishedArticleList{}, wrapPublisherError("list wechat published articles", err)
	}
	items := batch.Items
	if !input.IncludeDeleted {
		items = make([]officialcontent.PublishedArticle, 0, len(batch.Items))
		for _, item := range batch.Items {
			if !item.Deleted {
				items = append(items, item)
			}
		}
	}
	nextOffset := input.Offset + batch.FetchedMessageCount
	return officialcontent.PublishedArticleList{
		TotalMessageCount:    batch.TotalMessageCount,
		FetchedMessageCount:  batch.FetchedMessageCount,
		ReturnedArticleCount: len(items),
		NextOffset:           nextOffset,
		HasMore:              nextOffset < batch.TotalMessageCount,
		Items:                items,
	}, nil
}
