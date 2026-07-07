package application

import (
	"context"
	"fmt"
	"strings"

	"official-account-service/internal/domain/article"
	"official-account-service/internal/domain/authorization"
	"official-account-service/internal/domain/publish"
)

// DashboardService builds tenant-scoped dashboard summaries.
type DashboardService struct {
	accounts  *AccountService
	articles  *ArticleService
	publishes *PublishService
}

// NewDashboardService constructs a DashboardService.
func NewDashboardService(accounts *AccountService, articles *ArticleService, publishes *PublishService) *DashboardService {
	return &DashboardService{accounts: accounts, articles: articles, publishes: publishes}
}

// DashboardStats contains tenant-scoped dashboard counts.
type DashboardStats struct {
	AccountTotal              int
	ActiveAccountTotal        int
	RevokedAccountTotal       int
	RefreshFailedAccountTotal int
	ArticleTotal              int
	DraftArticleTotal         int
	PublishingArticleTotal    int
	PublishedArticleTotal     int
	FailedArticleTotal        int
	PublishTotal              int
	PublishingPublishTotal    int
	PublishedPublishTotal     int
	FailedPublishTotal        int
}

// GetStats returns tenant-scoped dashboard counts without exposing sensitive fields.
func (s *DashboardService) GetStats(ctx context.Context, tenantID string) (DashboardStats, error) {
	if err := s.validateReady(); err != nil {
		return DashboardStats{}, err
	}
	if strings.TrimSpace(tenantID) == "" {
		return DashboardStats{}, fmt.Errorf("validate dashboard tenant id: %w", ErrInvalidInput)
	}
	accounts, err := s.accounts.ListAccounts(ctx, tenantID)
	if err != nil {
		return DashboardStats{}, fmt.Errorf("list dashboard accounts: %w", err)
	}
	articles, err := s.articles.ListArticles(ctx, tenantID)
	if err != nil {
		return DashboardStats{}, fmt.Errorf("list dashboard articles: %w", err)
	}
	stats := DashboardStats{}
	countDashboardAccounts(&stats, accounts)
	if err := s.countArticlesAndPublishes(ctx, tenantID, articles, &stats); err != nil {
		return DashboardStats{}, err
	}
	return stats, nil
}

func (s *DashboardService) validateReady() error {
	if s == nil || s.accounts == nil || s.articles == nil || s.publishes == nil {
		return fmt.Errorf("validate dashboard service dependencies: %w", ErrNotImplemented)
	}
	return nil
}

func (s *DashboardService) countArticlesAndPublishes(ctx context.Context, tenantID string, articles []article.Article, stats *DashboardStats) error {
	for _, item := range articles {
		countDashboardArticle(stats, item.Status)
		records, err := s.publishes.ListPublishRecordsByArticle(ctx, tenantID, item.ID)
		if err != nil {
			return fmt.Errorf("list dashboard publish records for article %d: %w", item.ID, err)
		}
		for _, record := range records {
			countDashboardPublish(stats, record.Status)
		}
	}
	return nil
}

func countDashboardAccounts(stats *DashboardStats, accounts []authorization.Account) {
	for _, account := range accounts {
		stats.AccountTotal++
		switch account.Status {
		case authorization.AccountStatusActive:
			stats.ActiveAccountTotal++
		case authorization.AccountStatusRevoked:
			stats.RevokedAccountTotal++
		case authorization.AccountStatusRefreshFailed:
			stats.RefreshFailedAccountTotal++
		}
	}
}

func countDashboardArticle(stats *DashboardStats, status article.Status) {
	stats.ArticleTotal++
	switch status {
	case article.StatusDraft:
		stats.DraftArticleTotal++
	case article.StatusPublishing:
		stats.PublishingArticleTotal++
	case article.StatusPublished:
		stats.PublishedArticleTotal++
	case article.StatusFailed:
		stats.FailedArticleTotal++
	}
}

func countDashboardPublish(stats *DashboardStats, status publish.Status) {
	stats.PublishTotal++
	switch status {
	case publish.StatusPublishing:
		stats.PublishingPublishTotal++
	case publish.StatusPublished:
		stats.PublishedPublishTotal++
	case publish.StatusFailed:
		stats.FailedPublishTotal++
	}
}
