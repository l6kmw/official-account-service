package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"official-account-service/internal/application"
)

type authorizationURLRequest struct {
	ComponentAppID string `form:"component_appid" binding:"required"`
	RedirectURI    string `form:"redirect_uri" binding:"required"`
	AuthType       int    `form:"auth_type" binding:"omitempty,oneof=1 2 3"`
	BizAppID       string `form:"biz_appid"`
}

type authorizationURLResponse struct {
	AuthorizationURL        string `json:"authorization_url"`
	PreAuthCodeExpiresInSec int    `json:"pre_auth_code_expires_in_sec"`
}

func registerAuthorizationURLRoutes(r gin.IRouter, service *application.AuthorizationService) {
	r.GET("/wechat/authorization-url", func(c *gin.Context) {
		var query authorizationURLRequest
		if err := c.ShouldBindQuery(&query); err != nil {
			writeError(c, http.StatusBadRequest, "invalid_request")
			return
		}
		result, err := service.GenerateAuthorizationURL(c.Request.Context(), application.GenerateAuthorizationURLInput{
			ComponentAppID: query.ComponentAppID,
			RedirectURI:    query.RedirectURI,
			AuthType:       query.AuthType,
			BizAppID:       query.BizAppID,
		})
		if !writeServiceError(c, err) {
			return
		}
		c.JSON(http.StatusOK, authorizationURLResponse{
			AuthorizationURL:        result.URL,
			PreAuthCodeExpiresInSec: result.PreAuthCodeExpiresInSec,
		})
	})
}
