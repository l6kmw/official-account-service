package http

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"official-account-service/internal/application"
	"official-account-service/internal/domain/identity"
)

type createAgentRequest struct {
	AgentID string `json:"agent_id" binding:"required"`
	Name    string `json:"name" binding:"required"`
	Purpose string `json:"purpose"`
}

type updateAgentRequest struct {
	Name    string `json:"name" binding:"required"`
	Purpose string `json:"purpose"`
	Status  string `json:"status" binding:"required,oneof=active disabled"`
}

type agentResponse struct {
	ID                 string     `json:"id"`
	UserID             string     `json:"user_id"`
	AgentID            string     `json:"agent_id"`
	Name               string     `json:"name"`
	Purpose            string     `json:"purpose"`
	Status             string     `json:"status"`
	APITokenConfigured bool       `json:"api_token_configured"`
	APITokenHint       string     `json:"api_token_hint,omitempty"`
	APITokenCreatedAt  *time.Time `json:"api_token_created_at,omitempty"`
	LastUsedAt         *time.Time `json:"last_used_at,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

type generatedAgentAPITokenResponse struct {
	Token string        `json:"token"`
	Agent agentResponse `json:"agent"`
}

type agentSummaryResponse struct {
	ID      string `json:"id"`
	AgentID string `json:"agent_id"`
	Name    string `json:"name"`
	Purpose string `json:"purpose"`
	Status  string `json:"status"`
}

func registerAgentRoutes(r gin.IRouter, service *application.IdentityService) {
	r.GET("/agents", func(c *gin.Context) {
		userID, ok := bindTenant(c)
		if !ok {
			return
		}
		agents, err := service.ListAgents(c.Request.Context(), userID)
		if !writeServiceError(c, err) {
			return
		}
		items := make([]agentSummaryResponse, 0, len(agents))
		for _, agent := range agents {
			items = append(items, agentSummaryResponse{
				ID: agent.ID, AgentID: agent.AgentID, Name: agent.Name, Purpose: agent.Purpose, Status: string(agent.Status),
			})
		}
		c.JSON(http.StatusOK, gin.H{"items": items})
	})

	r.GET("/admin/users/:user_id/agents", func(c *gin.Context) {
		if !requireAdminRole(c, service) {
			return
		}
		agents, err := service.ListAgents(c.Request.Context(), c.Param("user_id"))
		if !writeServiceError(c, err) {
			return
		}
		items := make([]agentResponse, 0, len(agents))
		for _, agent := range agents {
			items = append(items, toAgentResponse(agent))
		}
		c.JSON(http.StatusOK, gin.H{"items": items})
	})

	r.POST("/admin/users/:user_id/agents", func(c *gin.Context) {
		if !requireAdminRole(c, service) {
			return
		}
		var body createAgentRequest
		if err := c.ShouldBindJSON(&body); err != nil {
			writeRequestReadError(c, err)
			return
		}
		generated, err := service.CreateAgent(c.Request.Context(), application.CreateAgentInput{
			UserID: c.Param("user_id"), AgentID: body.AgentID, Name: body.Name, Purpose: body.Purpose,
		})
		if !writeServiceError(c, err) {
			return
		}
		preventCredentialCaching(c)
		c.JSON(http.StatusCreated, generatedAgentAPITokenResponse{Token: generated.Token, Agent: toAgentResponse(generated.Agent)})
	})

	r.POST("/admin/users/:user_id/agents/:id/api-token", func(c *gin.Context) {
		if !requireAdminRole(c, service) {
			return
		}
		generated, err := service.RotateAgentAPIToken(c.Request.Context(), c.Param("user_id"), c.Param("id"))
		if !writeServiceError(c, err) {
			return
		}
		preventCredentialCaching(c)
		c.JSON(http.StatusCreated, generatedAgentAPITokenResponse{Token: generated.Token, Agent: toAgentResponse(generated.Agent)})
	})

	r.DELETE("/admin/users/:user_id/agents/:id/api-token", func(c *gin.Context) {
		if !requireAdminRole(c, service) {
			return
		}
		agent, err := service.RevokeAgentAPIToken(c.Request.Context(), c.Param("user_id"), c.Param("id"))
		if !writeServiceError(c, err) {
			return
		}
		c.JSON(http.StatusOK, toAgentResponse(agent))
	})

	r.PATCH("/admin/users/:user_id/agents/:id", func(c *gin.Context) {
		if !requireAdminRole(c, service) {
			return
		}
		var body updateAgentRequest
		if err := c.ShouldBindJSON(&body); err != nil {
			writeRequestReadError(c, err)
			return
		}
		agent, err := service.UpdateAgent(c.Request.Context(), application.UpdateAgentInput{
			UserID: c.Param("user_id"), ID: c.Param("id"), Name: body.Name, Purpose: body.Purpose, Status: identity.Status(body.Status),
		})
		if !writeServiceError(c, err) {
			return
		}
		c.JSON(http.StatusOK, toAgentResponse(agent))
	})
}

func preventCredentialCaching(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
}

func toAgentResponse(agent identity.Agent) agentResponse {
	return agentResponse{
		ID: agent.ID, UserID: agent.UserID, AgentID: agent.AgentID, Name: agent.Name, Purpose: agent.Purpose,
		Status: string(agent.Status), APITokenConfigured: agent.APITokenHash != "", APITokenHint: agent.APITokenHint,
		APITokenCreatedAt: agent.APITokenCreatedAt, LastUsedAt: agent.LastUsedAt,
		CreatedAt: agent.CreatedAt, UpdatedAt: agent.UpdatedAt,
	}
}
