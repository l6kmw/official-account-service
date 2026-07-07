package http

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"official-account-service/internal/application"
)

type tokenStatusRequest struct {
	ComponentAppID string `form:"component_appid" binding:"required"`
}

type tokenStatusResponse struct {
	AccountID        int64      `json:"account_id"`
	AppID            string     `json:"app_id"`
	AccountStatus    string     `json:"account_status"`
	Cached           bool       `json:"cached"`
	ExpiresAt        *time.Time `json:"expires_at,omitempty"`
	ExpiresInSeconds int64      `json:"expires_in_seconds"`
	NeedsRefresh     bool       `json:"needs_refresh"`
}

func registerTokenRoutes(r gin.IRouter, service *application.TokenService) {
	r.GET("/accounts/:id/token-status", func(c *gin.Context) {
		tenant, id, ok := bindTenantAndID(c)
		if !ok {
			return
		}
		var query tokenStatusRequest
		if err := c.ShouldBindQuery(&query); err != nil {
			writeError(c, http.StatusBadRequest, "invalid_request")
			return
		}
		status, err := service.GetAuthorizerTokenStatus(c.Request.Context(), application.RefreshAuthorizerAccessTokenInput{
			TenantID:       tenant,
			AccountID:      id,
			ComponentAppID: query.ComponentAppID,
		})
		if !writeServiceError(c, err) {
			return
		}
		var expiresAt *time.Time
		if !status.ExpiresAt.IsZero() {
			value := status.ExpiresAt
			expiresAt = &value
		}
		c.JSON(http.StatusOK, tokenStatusResponse{
			AccountID:        status.AccountID,
			AppID:            status.AppID,
			AccountStatus:    string(status.AccountStatus),
			Cached:           status.Cached,
			ExpiresAt:        expiresAt,
			ExpiresInSeconds: status.ExpiresInSeconds,
			NeedsRefresh:     status.NeedsRefresh,
		})
	})
}
