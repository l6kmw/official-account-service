package application

import (
	"context"
	"fmt"
	"strings"
	"time"

	"official-account-service/internal/domain/officialcontent"
)

const defaultPublishedArticlePageSize = 20

// OfficialContentService reads live content owned by an authorized account.
type OfficialContentService struct {
	reader         officialcontent.Reader
	metricsReader  officialcontent.MetricsReader
	tokens         PublishTokenProvider
	componentAppID string
	now            func() time.Time
}

// NewOfficialContentService constructs an OfficialContentService.
func NewOfficialContentService(reader officialcontent.Reader, tokens PublishTokenProvider, componentAppID string) *OfficialContentService {
	metricsReader, _ := reader.(officialcontent.MetricsReader)
	return &OfficialContentService{
		reader: reader, metricsReader: metricsReader, tokens: tokens,
		componentAppID: strings.TrimSpace(componentAppID), now: time.Now,
	}
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

// GetArticleMetricsInput selects metrics for articles published on one date.
type GetArticleMetricsInput struct {
	TenantID     string
	AuthorizerID int64
	Date         string
}

// GetArticleMetrics reads current WeChat article metrics for one publish date.
func (s *OfficialContentService) GetArticleMetrics(ctx context.Context, input GetArticleMetricsInput) (officialcontent.ArticleMetricsResult, error) {
	dateString := strings.TrimSpace(input.Date)
	date, err := time.Parse("2006-01-02", dateString)
	if err != nil || strings.TrimSpace(input.TenantID) == "" || input.AuthorizerID <= 0 {
		return officialcontent.ArticleMetricsResult{}, fmt.Errorf("validate article metrics input: %w", ErrInvalidInput)
	}
	now := time.Now
	if s != nil && s.now != nil {
		now = s.now
	}
	minimumDate := time.Date(2025, 11, 1, 0, 0, 0, 0, time.UTC)
	today := now().In(time.Local)
	today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.Local)
	if date.Before(minimumDate) || !date.Before(today) {
		return officialcontent.ArticleMetricsResult{}, fmt.Errorf("validate article metrics date: %w", ErrInvalidInput)
	}
	if s == nil || s.metricsReader == nil || s.tokens == nil || s.componentAppID == "" {
		return officialcontent.ArticleMetricsResult{}, fmt.Errorf("read article metrics: %w", ErrNotImplemented)
	}
	token, err := s.tokens.GetAuthorizerAccessToken(ctx, RefreshAuthorizerAccessTokenInput{
		TenantID: input.TenantID, AccountID: input.AuthorizerID, ComponentAppID: s.componentAppID,
	})
	if err != nil {
		return officialcontent.ArticleMetricsResult{}, fmt.Errorf("get authorizer access token for article metrics: %w", err)
	}
	result, err := s.metricsReader.GetArticleMetrics(ctx, token.AccessToken, dateString)
	if err != nil {
		return officialcontent.ArticleMetricsResult{}, wrapPublisherError("get wechat article metrics", err)
	}
	return result, nil
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
