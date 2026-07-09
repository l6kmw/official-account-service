package http

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"official-account-service/internal/application"
)

type materialResponse struct {
	ID           int64     `json:"id"`
	TenantID     string    `json:"tenant_id"`
	AuthorizerID int64     `json:"authorizer_id"`
	ArticleID    int64     `json:"article_id"`
	Usage        string    `json:"usage"`
	LocalURL     string    `json:"local_url"`
	WeChatURL    string    `json:"wechat_url"`
	MediaID      string    `json:"media_id"`
	CreatedAt    time.Time `json:"created_at"`
}

func registerMaterialRoutes(r gin.IRouter, service *application.MaterialService) {
	r.POST("/materials/inline-images", func(c *gin.Context) {
		input, closeFile, ok := bindMaterialUpload(c)
		if !ok {
			return
		}
		defer closeFile()
		asset, err := service.UploadInlineImage(c.Request.Context(), input)
		if !writeServiceError(c, err) {
			return
		}
		c.JSON(http.StatusCreated, materialResponse{
			ID: asset.ID, TenantID: asset.TenantID, AuthorizerID: asset.AuthorizerID, ArticleID: asset.ArticleID,
			Usage: string(asset.Usage), LocalURL: asset.LocalURL, WeChatURL: asset.WeChatURL, MediaID: asset.MediaID,
			CreatedAt: asset.CreatedAt,
		})
	})
	r.POST("/materials/covers", func(c *gin.Context) {
		input, closeFile, ok := bindMaterialUpload(c)
		if !ok {
			return
		}
		defer closeFile()
		asset, err := service.UploadCover(c.Request.Context(), input)
		if !writeServiceError(c, err) {
			return
		}
		c.JSON(http.StatusCreated, materialResponse{
			ID: asset.ID, TenantID: asset.TenantID, AuthorizerID: asset.AuthorizerID, ArticleID: asset.ArticleID,
			Usage: string(asset.Usage), LocalURL: asset.LocalURL, WeChatURL: asset.WeChatURL, MediaID: asset.MediaID,
			CreatedAt: asset.CreatedAt,
		})
	})
}

func bindMaterialUpload(c *gin.Context) (application.UploadMaterialInput, func(), bool) {
	tenant, ok := bindTenant(c)
	if !ok {
		return application.UploadMaterialInput{}, func() {}, false
	}
	if err := c.Request.ParseMultipartForm(maxRequestBodyBytes); err != nil {
		writeRequestReadError(c, err)
		return application.UploadMaterialInput{}, func() {}, false
	}
	authorizerID, ok := parsePositiveFormInt(c, "authorizer_id")
	if !ok {
		return application.UploadMaterialInput{}, func() {}, false
	}
	articleID, ok := parsePositiveFormInt(c, "article_id")
	if !ok {
		return application.UploadMaterialInput{}, func() {}, false
	}
	file, err := c.FormFile("file")
	if err != nil {
		writeRequestReadError(c, err)
		return application.UploadMaterialInput{}, func() {}, false
	}
	opened, err := file.Open()
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request")
		return application.UploadMaterialInput{}, func() {}, false
	}
	input := application.UploadMaterialInput{TenantID: tenant, AuthorizerID: authorizerID, ArticleID: articleID, Filename: file.Filename, Content: opened}
	return input, func() { _ = opened.Close() }, true
}

func parsePositiveFormInt(c *gin.Context, name string) (int64, bool) {
	value, err := strconv.ParseInt(c.PostForm(name), 10, 64)
	if err != nil || value <= 0 {
		writeError(c, http.StatusBadRequest, "invalid_request")
		return 0, false
	}
	return value, true
}
