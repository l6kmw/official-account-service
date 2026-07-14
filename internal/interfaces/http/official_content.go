package http

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"official-account-service/internal/application"
)

func registerOfficialContentRoutes(r gin.IRouter, service *application.OfficialContentService) {
	r.GET("/accounts/:id/published-articles", func(c *gin.Context) {
		if service == nil {
			writeServiceError(c, application.ErrNotImplemented)
			return
		}
		tenant, id, ok := bindTenantAndID(c)
		if !ok {
			return
		}
		offset, ok := bindOptionalIntQuery(c, "offset", 0)
		if !ok {
			return
		}
		count, ok := bindOptionalIntQuery(c, "count", 20)
		if !ok {
			return
		}
		includeContent, ok := bindOptionalBoolQuery(c, "include_content", false)
		if !ok {
			return
		}
		includeDeleted, ok := bindOptionalBoolQuery(c, "include_deleted", false)
		if !ok {
			return
		}
		result, err := service.ListPublishedArticles(c.Request.Context(), application.ListPublishedArticlesInput{
			TenantID: tenant, AuthorizerID: id, Offset: offset, Count: count,
			IncludeContent: includeContent, IncludeDeleted: includeDeleted,
		})
		if !writeServiceError(c, err) {
			return
		}
		c.JSON(http.StatusOK, result)
	})
	r.GET("/accounts/:id/article-metrics", func(c *gin.Context) {
		if service == nil {
			writeServiceError(c, application.ErrNotImplemented)
			return
		}
		tenant, id, ok := bindTenantAndID(c)
		if !ok {
			return
		}
		result, err := service.GetArticleMetrics(c.Request.Context(), application.GetArticleMetricsInput{
			TenantID: tenant, AuthorizerID: id, Date: c.Query("date"),
		})
		if !writeServiceError(c, err) {
			return
		}
		c.JSON(http.StatusOK, result)
	})
}

func bindOptionalIntQuery(c *gin.Context, name string, fallback int) (int, bool) {
	raw := c.Query(name)
	if raw == "" {
		return fallback, true
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request")
		return 0, false
	}
	return value, true
}

func bindOptionalBoolQuery(c *gin.Context, name string, fallback bool) (bool, bool) {
	raw := c.Query(name)
	if raw == "" {
		return fallback, true
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request")
		return false, false
	}
	return value, true
}
