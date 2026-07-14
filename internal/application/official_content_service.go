package application

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"official-account-service/internal/domain/officialcontent"
)

const defaultPublishedArticlePageSize = 20

// OfficialContentService reads live content owned by an authorized account.
type OfficialContentService struct {
	reader         officialcontent.Reader
	metricsReader  officialcontent.MetricsReader
	commentReader  officialcontent.CommentReader
	tokens         PublishTokenProvider
	componentAppID string
	now            func() time.Time
}

// NewOfficialContentService constructs an OfficialContentService.
func NewOfficialContentService(reader officialcontent.Reader, tokens PublishTokenProvider, componentAppID string) *OfficialContentService {
	metricsReader, _ := reader.(officialcontent.MetricsReader)
	commentReader, _ := reader.(officialcontent.CommentReader)
	return &OfficialContentService{
		reader: reader, metricsReader: metricsReader, commentReader: commentReader, tokens: tokens,
		componentAppID: strings.TrimSpace(componentAppID), now: time.Now,
	}
}

// ListArticleCommentsInput selects one WeChat article comment page.
type ListArticleCommentsInput struct {
	TenantID     string
	AuthorizerID int64
	MsgID        string
	Begin        int
	Count        int
	Type         int
}

// ListArticleComments reads current WeChat comments without returning OpenIDs.
func (s *OfficialContentService) ListArticleComments(ctx context.Context, input ListArticleCommentsInput) (officialcontent.ArticleCommentList, error) {
	msgID := strings.TrimSpace(input.MsgID)
	msgDataID, articleIndex, err := parseMetricsMsgID(msgID)
	if err != nil || strings.TrimSpace(input.TenantID) == "" || input.AuthorizerID <= 0 || input.Begin < 0 || input.Count < 0 || input.Count >= 50 || input.Type < 0 || input.Type > 2 {
		return officialcontent.ArticleCommentList{}, fmt.Errorf("validate article comments input: %w", ErrInvalidInput)
	}
	if s == nil || s.commentReader == nil || s.tokens == nil || s.componentAppID == "" {
		return officialcontent.ArticleCommentList{}, fmt.Errorf("read article comments: %w", ErrNotImplemented)
	}
	count := input.Count
	if count == 0 {
		count = 20
	}
	token, err := s.tokens.GetAuthorizerAccessToken(ctx, RefreshAuthorizerAccessTokenInput{
		TenantID: input.TenantID, AccountID: input.AuthorizerID, ComponentAppID: s.componentAppID,
	})
	if err != nil {
		return officialcontent.ArticleCommentList{}, fmt.Errorf("get authorizer access token for article comments: %w", err)
	}
	batch, err := s.commentReader.ListArticleComments(ctx, token.AccessToken, msgDataID, articleIndex, input.Begin, count, input.Type)
	if err != nil {
		return officialcontent.ArticleCommentList{}, wrapPublisherError("list wechat article comments", err)
	}
	nextBegin := input.Begin + len(batch.Items)
	return officialcontent.ArticleCommentList{
		MsgID: msgID, MsgDataID: msgDataID, ArticleIndex: articleIndex,
		Total: batch.Total, ReturnedCount: len(batch.Items), NextBegin: nextBegin,
		HasMore: nextBegin < batch.Total, Items: batch.Items,
	}, nil
}

func parseMetricsMsgID(msgID string) (int64, int, error) {
	separator := strings.LastIndex(msgID, "_")
	if separator <= 0 || separator == len(msgID)-1 {
		return 0, 0, ErrInvalidInput
	}
	msgDataID, err := strconv.ParseInt(msgID[:separator], 10, 64)
	if err != nil || msgDataID <= 0 {
		return 0, 0, ErrInvalidInput
	}
	ordinal, err := strconv.Atoi(msgID[separator+1:])
	if err != nil || ordinal <= 0 {
		return 0, 0, ErrInvalidInput
	}
	return msgDataID, ordinal - 1, nil
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
	if strings.TrimSpace(input.TenantID) == "" || input.AuthorizerID <= 0 {
		return officialcontent.ArticleMetricsResult{}, fmt.Errorf("validate article metrics input: %w", ErrInvalidInput)
	}
	now := time.Now
	if s != nil && s.now != nil {
		now = s.now
	}
	current := now()
	location := current.Location()
	date, err := time.ParseInLocation("2006-01-02", dateString, location)
	if err != nil {
		return officialcontent.ArticleMetricsResult{}, fmt.Errorf("validate article metrics input: %w", ErrInvalidInput)
	}
	minimumDate := time.Date(2025, 11, 1, 0, 0, 0, 0, location)
	today := time.Date(current.Year(), current.Month(), current.Day(), 0, 0, 0, 0, location)
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
