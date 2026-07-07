package http

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"official-account-service/internal/application"
)

const maxComponentCallbackBodyBytes = 1 << 20

type componentCallbackQuery struct {
	EncryptType  string `form:"encrypt_type" binding:"omitempty,oneof=aes"`
	MsgSignature string `form:"msg_signature"`
	Timestamp    string `form:"timestamp"`
	Nonce        string `form:"nonce"`
}

func registerAuthorizationRoutes(r gin.IRouter, service *application.AuthorizationService) {
	r.POST("/wechat/component/callback", func(c *gin.Context) {
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
		err = service.HandleComponentCallback(c.Request.Context(), application.HandleComponentCallbackInput{
			RawBody:      body,
			EncryptType:  query.EncryptType,
			MsgSignature: query.MsgSignature,
			Timestamp:    query.Timestamp,
			Nonce:        query.Nonce,
		})
		if !writeServiceError(c, err) {
			return
		}
		c.String(http.StatusOK, "success")
	})
}
