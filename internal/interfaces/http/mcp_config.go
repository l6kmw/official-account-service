package http

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
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
	Configured bool   `json:"configured"`
}

func registerMCPConfigRoutes(r gin.IRouter, cfg mcpConfig) {
	r.GET("/admin/mcp-config", func(c *gin.Context) {
		token := strings.TrimSpace(cfg.Token)
		c.JSON(http.StatusOK, mcpConfigResponse{
			Transport:  "streamable-http",
			Path:       normalizeMCPConfigPath(cfg.Path),
			HeaderName: "Authorization",
			Token:      token,
			Configured: token != "",
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
