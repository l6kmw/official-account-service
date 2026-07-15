package http

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"official-account-service/internal/application"
)

const defaultMCPConfigPath = "/mcp"

type mcpConfig struct {
	Token string
	Path  string
}

type mcpConfigResponse struct {
	Transport  string `json:"transport"`
	Path       string `json:"path"`
	HeaderName string `json:"header_name"`
	Token      string `json:"token"`
	TokenHint  string `json:"token_hint,omitempty"`
	Configured bool   `json:"configured"`
	Revealable bool   `json:"revealable"`
}

func registerMCPConfigRoutes(r gin.IRouter, cfg mcpConfig, identities *application.IdentityService, adminUserID string) {
	r.GET("/admin/mcp-config", func(c *gin.Context) {
		userID := currentUserID(c)
		token := ""
		tokenHint := ""
		configured := false
		if userID != "" && userID == strings.TrimSpace(adminUserID) {
			token = strings.TrimSpace(cfg.Token)
			configured = token != ""
		}
		if token == "" && identities != nil {
			user, err := identities.GetActiveUser(c.Request.Context(), userID)
			if !writeServiceError(c, err) {
				return
			}
			configured = user.APITokenHash != ""
			tokenHint = user.APITokenHint
		}
		c.JSON(http.StatusOK, mcpConfigResponse{
			Transport:  "streamable-http",
			Path:       normalizeMCPConfigPath(cfg.Path),
			HeaderName: "Authorization",
			Token:      token,
			TokenHint:  tokenHint,
			Configured: configured,
			Revealable: token != "",
		})
	})
}

func normalizeMCPConfigPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return defaultMCPConfigPath
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if path != "/" {
		path = strings.TrimRight(path, "/")
	}
	return path
}
