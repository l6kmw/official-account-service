package http

import (
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"official-account-service/internal/application"
	"official-account-service/internal/domain/article"
)

const (
	maxRequestBodyBytes            int64 = 10 << 20
	loggerContextKey                     = "logger"
	currentUserIDContextKey              = "current_user_id"
	currentUserRoleContextKey            = "current_user_role"
	currentActorTypeContextKey           = "current_actor_type"
	currentAgentRecordIDContextKey       = "current_agent_record_id"
	currentAgentIDContextKey             = "current_agent_id"
	legacyTenantHeaderContextKey         = "legacy_tenant_header_allowed"
	maxLoggedErrorMessageRunes           = 1000
)

var sensitiveLogValuePattern = regexp.MustCompile(`(?i)("?(?:access[_-]?token|component[_-]?access[_-]?token|authorizer[_-]?access[_-]?token|refresh[_-]?token|admin[_-]?api[_-]?key|authorization|app[_-]?secret|appsecret|secret|password)"?\s*[:=]\s*"?)([^\s,"'&}]+)("?)`)

// Dependencies contains HTTP adapter dependencies.
type Dependencies struct {
	Logger             *zap.Logger
	Authorization      *application.AuthorizationService
	Accounts           *application.AccountService
	Articles           *application.ArticleService
	Materials          *application.MaterialService
	Publishes          *application.PublishService
	Tokens             *application.TokenService
	Callbacks          *application.CallbackService
	TaskQueues         *application.TaskQueueService
	Dashboard          *application.DashboardService
	OfficialContent    *application.OfficialContentService
	PermanentMaterials *application.PermanentMaterialService
	Identity           *application.IdentityService
	AgentAudit         *application.AgentAuditService
	AdminAPIKey        string
	AdminUserID        string
	AdminSessionSecret string
	MCPToken           string
	MCPPath            string
}

// NewRouter constructs the HTTP router.
func NewRouter(deps Dependencies) http.Handler {
	gin.SetMode(gin.ReleaseMode)
	logger := deps.Logger
	if logger == nil {
		logger = zap.NewNop()
	}
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(attachLogger(logger))
	r.Use(limitRequestBody(maxRequestBodyBytes))
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	v1 := r.Group("/api/v1")
	v1.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	registerAuthorizationRoutes(r, deps.Authorization)
	registerWechatCallbackRoutes(r, deps.Callbacks)
	registerAuthorizationCallbackRoutes(v1, deps.Authorization)
	adminSessions := newAdminSessionManager(adminSessionConfig{Secret: deps.AdminSessionSecret}, deps.Identity, logger)
	registerAdminSessionRoutes(v1, adminSessions, deps.Identity, deps.AdminAPIKey, deps.AdminUserID)
	adminV1 := v1.Group("")
	adminV1.Use(requireAdminAuth(deps.AdminAPIKey, deps.AdminUserID, deps.Identity, adminSessions))
	registerAuthorizationURLRoutes(adminV1, deps.Authorization)
	registerUserRoutes(adminV1, deps.Identity)
	registerAgentRoutes(adminV1, deps.Identity)
	registerAgentAuditRoutes(adminV1, deps.AgentAudit)
	registerAccountRoutes(adminV1, deps.Accounts)
	registerArticleRoutes(adminV1, deps.Articles, deps.Publishes)
	registerMaterialRoutes(adminV1, deps.Materials)
	registerPublishRoutes(adminV1, deps.Publishes)
	registerTokenRoutes(adminV1, deps.Tokens)
	registerTaskQueueRoutes(adminV1, deps.TaskQueues)
	registerDashboardRoutes(adminV1, deps.Dashboard)
	registerOfficialContentRoutes(adminV1, deps.OfficialContent)
	registerPermanentMaterialRoutes(adminV1, deps.PermanentMaterials)
	registerMCPConfigRoutes(adminV1, mcpConfig{
		Token: deps.MCPToken,
		Path:  deps.MCPPath,
	}, deps.Identity, deps.AdminUserID)
	return r
}

func limitRequestBody(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		}
		c.Next()
	}
}

func requireAdminAuth(expectedAPIKey string, apiUserID string, identities *application.IdentityService, sessions *adminSessionManager) gin.HandlerFunc {
	expectedAPIKey = strings.TrimSpace(expectedAPIKey)
	apiUserID = strings.TrimSpace(apiUserID)
	authEnabled := expectedAPIKey != "" || (sessions != nil && sessions.enabled())
	if !authEnabled {
		return func(c *gin.Context) {
			c.Set(legacyTenantHeaderContextKey, true)
			c.Next()
		}
	}
	return func(c *gin.Context) {
		if expectedAPIKey != "" && isValidAdminAPIKey(c, expectedAPIKey) {
			if apiUserID == "" {
				writeError(c, http.StatusUnauthorized, "unauthorized")
				c.Abort()
				return
			}
			c.Set(currentUserIDContextKey, apiUserID)
			c.Set(currentUserRoleContextKey, "admin")
			c.Set(currentActorTypeContextKey, string(application.ActorTypeAdminAPIKey))
			c.Next()
			return
		}
		if identities != nil {
			if token := requestAPIToken(c); token != "" {
				principal, err := identities.AuthenticatePrincipal(c.Request.Context(), token)
				if err == nil {
					setPrincipalContext(c, principal)
					c.Next()
					return
				}
				if !errors.Is(err, application.ErrInvalidCredentials) {
					loggerFromContext(c).Error("authenticate api token",
						zap.String("error_code", "internal_error"),
						zap.String("method", requestMethod(c)),
						zap.String("path", requestPath(c)),
						zap.String("error", safeLogError(err)),
					)
				}
			}
		}
		if sessions != nil {
			session, ok := sessions.sessionFromRequest(c)
			if ok && sessions.validCSRF(c, session) {
				c.Set(currentUserIDContextKey, session.UserID)
				c.Set(currentUserRoleContextKey, session.Role)
				c.Set(currentActorTypeContextKey, string(application.ActorTypeBrowserSession))
				c.Next()
				return
			}
		}
		writeError(c, http.StatusUnauthorized, "unauthorized")
		c.Abort()
	}
}

func setPrincipalContext(c *gin.Context, principal application.Principal) {
	c.Set(currentUserIDContextKey, principal.User.ID)
	c.Set(currentUserRoleContextKey, string(principal.User.Role))
	c.Set(currentActorTypeContextKey, string(principal.ActorType))
	if principal.Agent != nil {
		c.Set(currentAgentRecordIDContextKey, principal.Agent.ID)
		c.Set(currentAgentIDContextKey, principal.Agent.AgentID)
	}
}

func requestAPIToken(c *gin.Context) string {
	actual := strings.TrimSpace(c.GetHeader("X-Admin-API-Key"))
	if actual == "" {
		actual = bearerToken(c.GetHeader("Authorization"))
	}
	return actual
}

func isValidAdminAPIKey(c *gin.Context, expected string) bool {
	expected = strings.TrimSpace(expected)
	if expected == "" {
		return false
	}
	actual := requestAPIToken(c)
	return constantTimeStringEqual(actual, expected)
}

func bearerToken(header string) string {
	const prefix = "Bearer "
	header = strings.TrimSpace(header)
	if !strings.HasPrefix(header, prefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(header, prefix))
}

func constantTimeStringEqual(actual string, expected string) bool {
	if actual == "" || expected == "" {
		return false
	}
	actualHash := sha256.Sum256([]byte(actual))
	expectedHash := sha256.Sum256([]byte(expected))
	return subtle.ConstantTimeCompare(actualHash[:], expectedHash[:]) == 1
}

type tenantHeader struct {
	TenantID string `header:"X-Tenant-ID" binding:"required"`
}

type resourceURI struct {
	ID int64 `uri:"id" binding:"required,gt=0"`
}

type createArticleRequest struct {
	AuthorizerID int64  `json:"authorizer_id" binding:"required,gt=0"`
	Title        string `json:"title" binding:"required"`
	Author       string `json:"author"`
	Digest       string `json:"digest"`
	ContentHTML  string `json:"content_html"`
}

type updateArticleRequest struct {
	Title             string `json:"title" binding:"required"`
	Author            string `json:"author"`
	Digest            string `json:"digest"`
	ContentHTML       string `json:"content_html"`
	CoverMediaAssetID int64  `json:"cover_media_asset_id" binding:"gte=0"`
	Version           int64  `json:"version" binding:"required,gt=0"`
}

type articleResponse struct {
	ID                int64     `json:"id"`
	TenantID          string    `json:"tenant_id"`
	AuthorizerID      int64     `json:"authorizer_id"`
	Title             string    `json:"title"`
	Author            string    `json:"author"`
	Digest            string    `json:"digest"`
	ContentHTML       string    `json:"content_html"`
	CoverMediaAssetID int64     `json:"cover_media_asset_id"`
	Status            string    `json:"status"`
	CreatedByAgentID  string    `json:"created_by_agent_id"`
	UpdatedByAgentID  string    `json:"updated_by_agent_id"`
	Version           int64     `json:"version"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

func registerArticleRoutes(r gin.IRouter, service *application.ArticleService, publishes *application.PublishService) {
	r.POST("/articles", func(c *gin.Context) {
		tenant, ok := bindTenant(c)
		if !ok {
			return
		}
		var body createArticleRequest
		if !bindCreateJSON(c, &body) {
			return
		}
		created, err := service.CreateArticle(c.Request.Context(), application.CreateArticleInput{
			TenantID: tenant, AuthorizerID: body.AuthorizerID, Title: body.Title,
			Author: body.Author, Digest: body.Digest, ContentHTML: body.ContentHTML, Actor: currentActor(c),
		})
		if !writeServiceError(c, err) {
			return
		}
		c.JSON(http.StatusCreated, toArticleResponse(created))
	})
	r.GET("/articles", func(c *gin.Context) {
		tenant, ok := bindTenant(c)
		if !ok {
			return
		}
		var items []article.Article
		var err error
		if agentRecordID := strings.TrimSpace(c.Query("agent_record_id")); agentRecordID != "" {
			items, err = service.ListArticlesByAgent(c.Request.Context(), tenant, agentRecordID)
		} else {
			items, err = service.ListArticles(c.Request.Context(), tenant)
		}
		if !writeServiceError(c, err) {
			return
		}
		out := make([]articleResponse, 0, len(items))
		for _, item := range items {
			out = append(out, toArticleResponse(item))
		}
		c.JSON(http.StatusOK, gin.H{"items": out})
	})
	r.GET("/articles/:id", func(c *gin.Context) {
		tenant, id, ok := bindTenantAndID(c)
		if !ok {
			return
		}
		item, err := service.GetArticle(c.Request.Context(), tenant, id)
		if !writeServiceError(c, err) {
			return
		}
		c.JSON(http.StatusOK, toArticleResponse(item))
	})
	r.PUT("/articles/:id", func(c *gin.Context) {
		tenant, id, ok := bindTenantAndID(c)
		if !ok {
			return
		}
		var body updateArticleRequest
		if !bindUpdateJSON(c, &body) {
			return
		}
		updated, err := service.UpdateArticle(c.Request.Context(), application.UpdateArticleInput{
			TenantID: tenant, ID: id, Title: body.Title, Author: body.Author, Digest: body.Digest,
			ContentHTML: body.ContentHTML, CoverMediaAssetID: body.CoverMediaAssetID,
			Version: body.Version, Actor: currentActor(c),
		})
		if errors.Is(err, application.ErrConflict) {
			c.JSON(http.StatusConflict, gin.H{
				"error":   "article_version_conflict",
				"message": "article changed after it was loaded; fetch the latest version, merge changes, and retry",
			})
			return
		}
		if !writeServiceError(c, err) {
			return
		}
		c.JSON(http.StatusOK, toArticleResponse(updated))
	})
	r.DELETE("/articles/:id", func(c *gin.Context) {
		tenant, id, ok := bindTenantAndID(c)
		if !ok {
			return
		}
		if publishes != nil {
			if !writeServiceError(c, publishes.DeletePublishedArticle(c.Request.Context(), application.DeletePublishedArticleInput{
				TenantID: tenant, ArticleID: id,
			})) {
				return
			}
		}
		if !writeServiceError(c, service.DeleteArticleWithActor(c.Request.Context(), application.DeleteArticleInput{
			TenantID: tenant, ID: id, Actor: currentActor(c),
		})) {
			return
		}
		c.Status(http.StatusNoContent)
	})
}

func toArticleResponse(item article.Article) articleResponse {
	return articleResponse{
		ID: item.ID, TenantID: item.TenantID, AuthorizerID: item.AuthorizerID,
		Title: item.Title, Author: item.Author, Digest: item.Digest, ContentHTML: item.ContentHTML,
		CoverMediaAssetID: item.CoverMediaAssetID, Status: string(item.Status),
		CreatedByAgentID: item.CreatedByAgentID, UpdatedByAgentID: item.UpdatedByAgentID, Version: item.Version,
		CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
}

func bindTenant(c *gin.Context) (string, bool) {
	if currentUserID, ok := c.Get(currentUserIDContextKey); ok {
		if userID, ok := currentUserID.(string); ok && strings.TrimSpace(userID) != "" {
			return strings.TrimSpace(userID), true
		}
	}
	if allowed, ok := c.Get(legacyTenantHeaderContextKey); !ok || allowed != true {
		writeError(c, http.StatusUnauthorized, "unauthorized")
		return "", false
	}
	var header tenantHeader
	if err := c.ShouldBindHeader(&header); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request")
		return "", false
	}
	return header.TenantID, true
}

func currentUserID(c *gin.Context) string {
	value, ok := c.Get(currentUserIDContextKey)
	if !ok {
		return ""
	}
	userID, _ := value.(string)
	return strings.TrimSpace(userID)
}

func currentUserRole(c *gin.Context) string {
	value, ok := c.Get(currentUserRoleContextKey)
	if !ok {
		return ""
	}
	role, _ := value.(string)
	return strings.TrimSpace(role)
}

func currentActorType(c *gin.Context) string {
	value, ok := c.Get(currentActorTypeContextKey)
	if !ok {
		return ""
	}
	actorType, _ := value.(string)
	return strings.TrimSpace(actorType)
}

func currentAgentRecordID(c *gin.Context) string {
	value, ok := c.Get(currentAgentRecordIDContextKey)
	if !ok {
		return ""
	}
	id, _ := value.(string)
	return strings.TrimSpace(id)
}

func currentAgentID(c *gin.Context) string {
	value, ok := c.Get(currentAgentIDContextKey)
	if !ok {
		return ""
	}
	id, _ := value.(string)
	return strings.TrimSpace(id)
}

func currentActor(c *gin.Context) application.Actor {
	return application.Actor{
		UserID: currentUserID(c), ActorType: application.ActorType(currentActorType(c)), AgentRecordID: currentAgentRecordID(c),
	}
}

func bindTenantAndID(c *gin.Context) (string, int64, bool) {
	tenant, ok := bindTenant(c)
	if !ok {
		return "", 0, false
	}
	var uri resourceURI
	if err := c.ShouldBindUri(&uri); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request")
		return "", 0, false
	}
	return tenant, uri.ID, true
}

func bindCreateJSON(c *gin.Context, out *createArticleRequest) bool {
	if err := c.ShouldBindJSON(out); err != nil {
		writeRequestReadError(c, err)
		return false
	}
	return true
}

func bindUpdateJSON(c *gin.Context, out *updateArticleRequest) bool {
	if err := c.ShouldBindJSON(out); err != nil {
		writeRequestReadError(c, err)
		return false
	}
	return true
}

func writeRequestReadError(c *gin.Context, err error) {
	var maxBytesError *http.MaxBytesError
	if errors.As(err, &maxBytesError) {
		writeError(c, http.StatusRequestEntityTooLarge, "request_too_large")
		return
	}
	writeError(c, http.StatusBadRequest, "invalid_request")
}

func writeServiceError(c *gin.Context, err error) bool {
	if err == nil {
		return true
	}
	if errors.Is(err, application.ErrInvalidInput) {
		writeError(c, http.StatusBadRequest, "invalid_request")
		return false
	}
	if errors.Is(err, application.ErrNotFound) {
		writeError(c, http.StatusNotFound, "not_found")
		return false
	}
	if errors.Is(err, application.ErrConflict) {
		writeError(c, http.StatusConflict, "conflict")
		return false
	}
	if errors.Is(err, application.ErrNotImplemented) {
		writeError(c, http.StatusNotImplemented, "not_implemented")
		return false
	}
	logInternalServiceError(c, err)
	writeError(c, http.StatusInternalServerError, "internal_error")
	return false
}

func writeError(c *gin.Context, status int, code string) {
	c.JSON(status, gin.H{"error": code})
}

func attachLogger(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(loggerContextKey, logger)
		c.Next()
	}
}

func logInternalServiceError(c *gin.Context, err error) {
	loggerFromContext(c).Error("service internal error",
		zap.String("error_code", "internal_error"),
		zap.String("method", requestMethod(c)),
		zap.String("route", requestRoute(c)),
		zap.String("path", requestPath(c)),
		zap.String("user_id", currentUserID(c)),
		zap.String("agent_id", currentAgentID(c)),
		zap.String("error", safeLogError(err)),
	)
}

func loggerFromContext(c *gin.Context) *zap.Logger {
	if value, ok := c.Get(loggerContextKey); ok {
		if logger, ok := value.(*zap.Logger); ok && logger != nil {
			return logger
		}
	}
	return zap.NewNop()
}

func requestMethod(c *gin.Context) string {
	if c == nil || c.Request == nil {
		return ""
	}
	return c.Request.Method
}

func requestRoute(c *gin.Context) string {
	if c == nil {
		return ""
	}
	return c.FullPath()
}

func requestPath(c *gin.Context) string {
	if c == nil || c.Request == nil || c.Request.URL == nil {
		return ""
	}
	return c.Request.URL.Path
}

func safeLogError(err error) string {
	if err == nil {
		return ""
	}
	message := sensitiveLogValuePattern.ReplaceAllString(err.Error(), "${1}[REDACTED]${3}")
	return truncateLogMessage(message, maxLoggedErrorMessageRunes)
}

func truncateLogMessage(value string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return string(runes[:maxRunes]) + "...[truncated]"
}
