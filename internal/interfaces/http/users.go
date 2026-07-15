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
	ID                 string     `json:"id"`
	Username           string     `json:"username"`
	Role               string     `json:"role"`
	Status             string     `json:"status"`
	APITokenConfigured bool       `json:"api_token_configured"`
	APITokenHint       string     `json:"api_token_hint,omitempty"`
	APITokenCreatedAt  *time.Time `json:"api_token_created_at,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

type generatedAPITokenResponse struct {
	Token string       `json:"token"`
	User  userResponse `json:"user"`
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
	r.POST("/admin/users/:user_id/api-token", func(c *gin.Context) {
		if !requireAdminRole(c, service) {
			return
		}
		generated, err := service.GenerateAPIToken(c.Request.Context(), c.Param("user_id"))
		if !writeServiceError(c, err) {
			return
		}
		c.JSON(http.StatusCreated, generatedAPITokenResponse{Token: generated.Token, User: toUserResponse(generated.User)})
	})
	r.DELETE("/admin/users/:user_id/api-token", func(c *gin.Context) {
		if !requireAdminRole(c, service) {
			return
		}
		user, err := service.RevokeAPIToken(c.Request.Context(), c.Param("user_id"))
		if !writeServiceError(c, err) {
			return
		}
		c.JSON(http.StatusOK, toUserResponse(user))
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
		APITokenConfigured: user.APITokenHash != "", APITokenHint: user.APITokenHint, APITokenCreatedAt: user.APITokenCreatedAt,
		CreatedAt: user.CreatedAt, UpdatedAt: user.UpdatedAt,
	}
}
