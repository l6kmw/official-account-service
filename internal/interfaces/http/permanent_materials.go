package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"official-account-service/internal/application"
)

func registerPermanentMaterialRoutes(r gin.IRouter, service *application.PermanentMaterialService) {
	r.GET("/accounts/:id/permanent-materials", func(c *gin.Context) {
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
		result, err := service.ListPermanentMaterials(c.Request.Context(), application.ListPermanentMaterialsInput{
			TenantID: tenant, AuthorizerID: id, Offset: offset, Count: count,
		})
		if !writeServiceError(c, err) {
			return
		}
		c.JSON(http.StatusOK, result)
	})
	r.DELETE("/accounts/:id/permanent-materials/:media_id", func(c *gin.Context) {
		if service == nil {
			writeServiceError(c, application.ErrNotImplemented)
			return
		}
		tenant, id, ok := bindTenantAndID(c)
		if !ok {
			return
		}
		mediaID := c.Param("media_id")
		err := service.DeletePermanentMaterial(c.Request.Context(), application.DeletePermanentMaterialInput{
			TenantID: tenant, AuthorizerID: id, MediaID: mediaID,
		})
		if !writeServiceError(c, err) {
			return
		}
		c.JSON(http.StatusOK, gin.H{"deleted": true, "media_id": mediaID})
	})
}
