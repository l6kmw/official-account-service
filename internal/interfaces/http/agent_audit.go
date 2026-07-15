package http

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"official-account-service/internal/application"
	"official-account-service/internal/domain/agentaudit"
)

type listAgentAuditQuery struct {
	AgentRecordID string `form:"agent_record_id"`
	Action        string `form:"action" binding:"omitempty,oneof=create_article update_article publish_article delete_article"`
	ResourceType  string `form:"resource_type" binding:"omitempty,oneof=article"`
	Limit         int    `form:"limit" binding:"omitempty,gte=1,lte=500"`
}

type agentAuditResponse struct {
	ID            int64     `json:"id"`
	UserID        string    `json:"user_id"`
	AgentRecordID string    `json:"agent_record_id"`
	Action        string    `json:"action"`
	ResourceType  string    `json:"resource_type"`
	ResourceID    string    `json:"resource_id"`
	CreatedAt     time.Time `json:"created_at"`
}

func registerAgentAuditRoutes(r gin.IRouter, service *application.AgentAuditService) {
	r.GET("/audit-logs", func(c *gin.Context) {
		userID, ok := bindTenant(c)
		if !ok {
			return
		}
		var query listAgentAuditQuery
		if err := c.ShouldBindQuery(&query); err != nil {
			writeError(c, http.StatusBadRequest, "invalid_request")
			return
		}
		entries, err := service.ListAgentAudit(c.Request.Context(), application.ListAgentAuditInput{
			UserID: userID, AgentRecordID: query.AgentRecordID, Action: agentaudit.Action(query.Action),
			ResourceType: agentaudit.ResourceType(query.ResourceType), Limit: query.Limit,
		})
		if !writeServiceError(c, err) {
			return
		}
		items := make([]agentAuditResponse, 0, len(entries))
		for _, entry := range entries {
			items = append(items, agentAuditResponse{
				ID: entry.ID, UserID: entry.UserID, AgentRecordID: entry.AgentRecordID,
				Action: string(entry.Action), ResourceType: string(entry.ResourceType), ResourceID: entry.ResourceID,
				CreatedAt: entry.CreatedAt,
			})
		}
		c.JSON(http.StatusOK, gin.H{"items": items})
	})
}
