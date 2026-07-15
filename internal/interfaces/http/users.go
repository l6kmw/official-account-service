package http

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"official-account-service/internal/application"
	"official-account-service/internal/domain/identity"
)

type createUserRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type userResponse struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Role      string    `json:"role"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func registerUserRoutes(r gin.IRouter, service *application.IdentityService) {
	r.GET("/admin/users", func(c *gin.Context) {
		if !requireAdminRole(c, service) {
			return
		}
		users, err := service.ListUsers(c.Request.Context())
		if !writeServiceError(c, err) {
			return
		}
		items := make([]userResponse, 0, len(users))
		for _, user := range users {
			items = append(items, toUserResponse(user))
		}
		c.JSON(http.StatusOK, gin.H{"items": items})
	})
	r.POST("/admin/users", func(c *gin.Context) {
		if !requireAdminRole(c, service) {
			return
		}
		var body createUserRequest
		if err := c.ShouldBindJSON(&body); err != nil {
			writeRequestReadError(c, err)
			return
		}
		user, err := service.CreateUser(c.Request.Context(), application.CreateUserInput{Username: body.Username, Password: body.Password})
		if !writeServiceError(c, err) {
			return
		}
		c.JSON(http.StatusCreated, toUserResponse(user))
	})
}

func requireAdminRole(c *gin.Context, service *application.IdentityService) bool {
	if service == nil {
		writeServiceError(c, application.ErrNotImplemented)
		return false
	}
	if currentUserRole(c) != string(identity.RoleAdmin) {
		writeError(c, http.StatusForbidden, "forbidden")
		return false
	}
	return true
}

func toUserResponse(user identity.User) userResponse {
	return userResponse{
		ID: user.ID, Username: user.Username, Role: string(user.Role), Status: string(user.Status),
		CreatedAt: user.CreatedAt, UpdatedAt: user.UpdatedAt,
	}
}
