package http

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"official-account-service/internal/application"
	"official-account-service/internal/domain/publish"
)

type createPublishRecordRequest struct {
	ArticleID       int64  `json:"article_id" binding:"required,gt=0"`
	WeChatPublishID string `json:"wechat_publish_id"`
}

type updatePublishStatusRequest struct {
	Status          string `json:"status" binding:"required,oneof=publishing published failed"`
	WeChatArticleID string `json:"wechat_article_id"`
	ErrorCode       string `json:"error_code"`
	ErrorMessage    string `json:"error_message"`
}

type publishRecordResponse struct {
	ID                      int64     `json:"id"`
	TenantID                string    `json:"tenant_id"`
	AuthorizerID            int64     `json:"authorizer_id"`
	ArticleID               int64     `json:"article_id"`
	WeChatPublishID         string    `json:"wechat_publish_id"`
	WeChatArticleID         string    `json:"wechat_article_id"`
	Status                  string    `json:"status"`
	ErrorCode               string    `json:"error_code"`
	ErrorMessage            string    `json:"error_message"`
	ArticleCreatedByAgentID string    `json:"article_created_by_agent_id"`
	SubmittedAt             time.Time `json:"submitted_at"`
	FinishedAt              time.Time `json:"finished_at"`
	CreatedAt               time.Time `json:"created_at"`
	UpdatedAt               time.Time `json:"updated_at"`
}

func registerPublishRoutes(r gin.IRouter, service *application.PublishService) {
	r.POST("/articles/:id/publish", func(c *gin.Context) {
		tenant, articleID, ok := bindTenantAndID(c)
		if !ok {
			return
		}
		record, err := service.PublishArticle(c.Request.Context(), application.PublishArticleInput{
			TenantID: tenant, ArticleID: articleID, Actor: currentActor(c),
		})
		if !writeServiceError(c, err) {
			return
		}
		c.JSON(http.StatusCreated, toPublishRecordResponse(record))
	})
	r.POST("/publish-records", func(c *gin.Context) {
		tenant, ok := bindTenant(c)
		if !ok {
			return
		}
		var body createPublishRecordRequest
		if err := c.ShouldBindJSON(&body); err != nil {
			writeRequestReadError(c, err)
			return
		}
		record, err := service.CreatePublishRecord(c.Request.Context(), application.CreatePublishRecordInput{
			TenantID: tenant, ArticleID: body.ArticleID, WeChatPublishID: body.WeChatPublishID, Actor: currentActor(c),
		})
		if !writeServiceError(c, err) {
			return
		}
		c.JSON(http.StatusCreated, toPublishRecordResponse(record))
	})
	r.GET("/publish-records", func(c *gin.Context) {
		tenant, ok := bindTenant(c)
		if !ok {
			return
		}
		var records []publish.Record
		var err error
		if agentRecordID := strings.TrimSpace(c.Query("agent_record_id")); agentRecordID != "" {
			records, err = service.ListPublishRecordsByAgent(c.Request.Context(), tenant, agentRecordID)
		} else {
			records, err = service.ListPublishRecords(c.Request.Context(), tenant)
		}
		if !writeServiceError(c, err) {
			return
		}
		out := make([]publishRecordResponse, 0, len(records))
		for _, record := range records {
			out = append(out, toPublishRecordResponse(record))
		}
		c.JSON(http.StatusOK, gin.H{"items": out})
	})
	r.GET("/articles/:id/publish-records", func(c *gin.Context) {
		tenant, articleID, ok := bindTenantAndID(c)
		if !ok {
			return
		}
		records, err := service.ListPublishRecordsByArticle(c.Request.Context(), tenant, articleID)
		if !writeServiceError(c, err) {
			return
		}
		out := make([]publishRecordResponse, 0, len(records))
		for _, record := range records {
			out = append(out, toPublishRecordResponse(record))
		}
		c.JSON(http.StatusOK, gin.H{"items": out})
	})
	r.GET("/publish-records/:id", func(c *gin.Context) {
		tenant, id, ok := bindTenantAndID(c)
		if !ok {
			return
		}
		record, err := service.GetPublishRecord(c.Request.Context(), tenant, id)
		if !writeServiceError(c, err) {
			return
		}
		c.JSON(http.StatusOK, toPublishRecordResponse(record))
	})
	r.PUT("/publish-records/:id/status", func(c *gin.Context) {
		tenant, id, ok := bindTenantAndID(c)
		if !ok {
			return
		}
		var body updatePublishStatusRequest
		if err := c.ShouldBindJSON(&body); err != nil {
			writeRequestReadError(c, err)
			return
		}
		record, err := service.UpdatePublishStatus(c.Request.Context(), application.UpdatePublishStatusInput{
			TenantID: tenant, ID: id, Status: publish.Status(body.Status), WeChatArticleID: body.WeChatArticleID,
			ErrorCode: body.ErrorCode, ErrorMessage: body.ErrorMessage,
		})
		if !writeServiceError(c, err) {
			return
		}
		c.JSON(http.StatusOK, toPublishRecordResponse(record))
	})
	r.POST("/publish-records/:id/sync-status", func(c *gin.Context) {
		tenant, id, ok := bindTenantAndID(c)
		if !ok {
			return
		}
		record, err := service.SyncPublishStatus(c.Request.Context(), application.SyncPublishStatusInput{
			TenantID: tenant, ID: id,
		})
		if !writeServiceError(c, err) {
			return
		}
		c.JSON(http.StatusOK, toPublishRecordResponse(record))
	})
	r.POST("/publish-records/:id/delete-published", func(c *gin.Context) {
		tenant, id, ok := bindTenantAndID(c)
		if !ok {
			return
		}
		record, err := service.DeletePublishedRecord(c.Request.Context(), application.DeletePublishedRecordInput{
			TenantID: tenant, ID: id,
		})
		if !writeServiceError(c, err) {
			return
		}
		c.JSON(http.StatusOK, toPublishRecordResponse(record))
	})
}

func toPublishRecordResponse(record publish.Record) publishRecordResponse {
	return publishRecordResponse{
		ID: record.ID, TenantID: record.TenantID, AuthorizerID: record.AuthorizerID, ArticleID: record.ArticleID,
		WeChatPublishID: record.WeChatPublishID, WeChatArticleID: record.WeChatArticleID, Status: string(record.Status),
		ErrorCode: record.ErrorCode, ErrorMessage: record.ErrorMessage, ArticleCreatedByAgentID: record.ArticleCreatedByAgentID,
		SubmittedAt: record.SubmittedAt, FinishedAt: record.FinishedAt,
		CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt,
	}
}
