package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"official-account-service/internal/application"
)

type dashboardStatsResponse struct {
	AccountTotal              int `json:"account_total"`
	ActiveAccountTotal        int `json:"active_account_total"`
	RevokedAccountTotal       int `json:"revoked_account_total"`
	RefreshFailedAccountTotal int `json:"refresh_failed_account_total"`
	ArticleTotal              int `json:"article_total"`
	DraftArticleTotal         int `json:"draft_article_total"`
	PublishingArticleTotal    int `json:"publishing_article_total"`
	PublishedArticleTotal     int `json:"published_article_total"`
	FailedArticleTotal        int `json:"failed_article_total"`
	PublishTotal              int `json:"publish_total"`
	PublishingPublishTotal    int `json:"publishing_publish_total"`
	PublishedPublishTotal     int `json:"published_publish_total"`
	FailedPublishTotal        int `json:"failed_publish_total"`
}

func registerDashboardRoutes(r gin.IRouter, service *application.DashboardService) {
	r.GET("/dashboard/stats", func(c *gin.Context) {
		tenant, ok := bindTenant(c)
		if !ok {
			return
		}
		stats, err := service.GetStats(c.Request.Context(), tenant)
		if !writeServiceError(c, err) {
			return
		}
		c.JSON(http.StatusOK, toDashboardStatsResponse(stats))
	})
}

func toDashboardStatsResponse(stats application.DashboardStats) dashboardStatsResponse {
	return dashboardStatsResponse{
		AccountTotal: stats.AccountTotal, ActiveAccountTotal: stats.ActiveAccountTotal,
		RevokedAccountTotal: stats.RevokedAccountTotal, RefreshFailedAccountTotal: stats.RefreshFailedAccountTotal,
		ArticleTotal: stats.ArticleTotal, DraftArticleTotal: stats.DraftArticleTotal,
		PublishingArticleTotal: stats.PublishingArticleTotal, PublishedArticleTotal: stats.PublishedArticleTotal,
		FailedArticleTotal: stats.FailedArticleTotal, PublishTotal: stats.PublishTotal,
		PublishingPublishTotal: stats.PublishingPublishTotal, PublishedPublishTotal: stats.PublishedPublishTotal,
		FailedPublishTotal: stats.FailedPublishTotal,
	}
}
