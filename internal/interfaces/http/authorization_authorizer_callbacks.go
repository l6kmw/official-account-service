package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"official-account-service/internal/application"
)

type authorizationCallbackRequest struct {
	State    string `form:"state" binding:"required"`
	AuthCode string `form:"auth_code" binding:"required"`
}

func registerAuthorizationCallbackRoutes(r gin.IRouter, service *application.AuthorizationService) {
	r.GET("/wechat/authorization-callback", func(c *gin.Context) {
		var query authorizationCallbackRequest
		if err := c.ShouldBindQuery(&query); err != nil {
			writeError(c, http.StatusBadRequest, "invalid_request")
			return
		}
		account, err := service.HandleAuthorizationCallback(c.Request.Context(), application.HandleAuthorizationCallbackInput{
			State:    query.State,
			AuthCode: query.AuthCode,
		})
		if !writeServiceError(c, err) {
			return
		}
		c.JSON(http.StatusOK, accountResponse{
			ID: account.ID, TenantID: account.TenantID, AppID: account.AppID, Name: account.Name,
			AvatarURL: account.AvatarURL, Status: string(account.Status), LastSyncedAt: account.LastSyncedAt,
			CreatedAt: account.CreatedAt, UpdatedAt: account.UpdatedAt,
		})
	})
}
