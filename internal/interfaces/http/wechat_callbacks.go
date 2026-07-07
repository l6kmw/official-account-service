package http

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"official-account-service/internal/application"
)

type authorizerCallbackURI struct {
	AppID string `uri:"app_id" binding:"required"`
}

func registerWechatCallbackRoutes(r gin.IRouter, service *application.CallbackService) {
	r.POST("/wechat/authorizer/:app_id/callback", func(c *gin.Context) {
		var uri authorizerCallbackURI
		if err := c.ShouldBindUri(&uri); err != nil {
			writeError(c, http.StatusBadRequest, "invalid_request")
			return
		}
		var query componentCallbackQuery
		if err := c.ShouldBindQuery(&query); err != nil {
			writeError(c, http.StatusBadRequest, "invalid_request")
			return
		}
		body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxComponentCallbackBodyBytes))
		if err != nil || len(body) == 0 {
			writeError(c, http.StatusBadRequest, "invalid_request")
			return
		}
		err = service.HandleAuthorizerCallback(c.Request.Context(), application.HandleAuthorizerCallbackInput{
			AuthorizerAppID: uri.AppID,
			RawBody:         body,
			EncryptType:     query.EncryptType,
			MsgSignature:    query.MsgSignature,
			Timestamp:       query.Timestamp,
			Nonce:           query.Nonce,
		})
		if !writeServiceError(c, err) {
			return
		}
		c.String(http.StatusOK, "success")
	})
}
