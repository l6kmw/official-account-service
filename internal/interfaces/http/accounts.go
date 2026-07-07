package http

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"official-account-service/internal/application"
)

type accountResponse struct {
	ID           int64     `json:"id"`
	TenantID     string    `json:"tenant_id"`
	AppID        string    `json:"app_id"`
	Name         string    `json:"name"`
	AvatarURL    string    `json:"avatar_url"`
	Status       string    `json:"status"`
	LastSyncedAt time.Time `json:"last_synced_at"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func registerAccountRoutes(r gin.IRouter, service *application.AccountService) {
	r.GET("/accounts", func(c *gin.Context) {
		tenant, ok := bindTenant(c)
		if !ok {
			return
		}
		items, err := service.ListAccounts(c.Request.Context(), tenant)
		if !writeServiceError(c, err) {
			return
		}
		out := make([]accountResponse, 0, len(items))
		for _, item := range items {
			out = append(out, accountResponse{
				ID: item.ID, TenantID: item.TenantID, AppID: item.AppID, Name: item.Name,
				AvatarURL: item.AvatarURL, Status: string(item.Status), LastSyncedAt: item.LastSyncedAt,
				CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
			})
		}
		c.JSON(http.StatusOK, gin.H{"items": out})
	})
	r.GET("/accounts/:id", func(c *gin.Context) {
		tenant, id, ok := bindTenantAndID(c)
		if !ok {
			return
		}
		item, err := service.GetAccount(c.Request.Context(), tenant, id)
		if !writeServiceError(c, err) {
			return
		}
		c.JSON(http.StatusOK, accountResponse{
			ID: item.ID, TenantID: item.TenantID, AppID: item.AppID, Name: item.Name,
			AvatarURL: item.AvatarURL, Status: string(item.Status), LastSyncedAt: item.LastSyncedAt,
			CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
		})
	})
}
