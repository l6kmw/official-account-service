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
	Transport      string `json:"transport"`
	Path           string `json:"path"`
	HeaderName     string `json:"header_name"`
	Token          string `json:"token"`
	TokenHint      string `json:"token_hint,omitempty"`
	Configured     bool   `json:"configured"`
	Revealable     bool   `json:"revealable"`
	AgentID        string `json:"agent_id,omitempty"`
	AgentName      string `json:"agent_name,omitempty"`
	AgentPurpose   string `json:"agent_purpose,omitempty"`
	ManagementPath string `json:"management_path"`
}

func registerMCPConfigRoutes(r gin.IRouter, cfg mcpConfig, identities *application.IdentityService, adminUserID string) {
	r.GET("/admin/mcp-config", func(c *gin.Context) {
		userID := currentUserID(c)
		token := ""
		tokenHint := ""
		configured := false
		agentID := ""
		agentName := ""
		agentPurpose := ""
		actorType := currentActorType(c)
		canRevealLegacy := actorType == string(application.ActorTypeAdminAPIKey) || actorType == string(application.ActorTypeBrowserSession)
		if canRevealLegacy && userID != "" && userID == strings.TrimSpace(adminUserID) {
			token = strings.TrimSpace(cfg.Token)
			configured = token != ""
		}
		if token == "" && identities != nil && currentAgentRecordID(c) != "" {
			agent, err := identities.GetAgent(c.Request.Context(), userID, currentAgentRecordID(c))
			if !writeServiceError(c, err) {
				return
			}
			configured = agent.APITokenHash != ""
			tokenHint = agent.APITokenHint
			agentID = agent.AgentID
			agentName = agent.Name
			agentPurpose = agent.Purpose
		} else if token == "" && identities != nil && actorType == string(application.ActorTypeLegacyUserToken) {
			user, err := identities.GetActiveUser(c.Request.Context(), userID)
			if !writeServiceError(c, err) {
				return
			}
			configured = user.APITokenHash != ""
			tokenHint = user.APITokenHint
		} else if token == "" && identities != nil && actorType == string(application.ActorTypeBrowserSession) {
			agents, err := identities.ListAgents(c.Request.Context(), userID)
			if !writeServiceError(c, err) {
				return
			}
			for _, agent := range agents {
				configured = configured || agent.APITokenHash != ""
			}
		}
		c.JSON(http.StatusOK, mcpConfigResponse{
			Transport: "streamable-http", Path: normalizeMCPConfigPath(cfg.Path), HeaderName: "Authorization",
			Token: token, TokenHint: tokenHint, Configured: configured, Revealable: token != "",
			AgentID: agentID, AgentName: agentName, AgentPurpose: agentPurpose, ManagementPath: "#/users",
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
