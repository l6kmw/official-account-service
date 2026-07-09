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
)

const (
	maxRequestBodyBytes        int64 = 10 << 20
	loggerContextKey                 = "logger"
	maxLoggedErrorMessageRunes       = 1000
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
	AdminAPIKey        string
	AdminUsername      string
	AdminPasswordHash  string
	AdminSessionSecret string
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
	registerAuthorizationURLRoutes(v1, deps.Authorization)
	adminSessions := newAdminSessionManager(adminSessionConfig{
		Username:     deps.AdminUsername,
		PasswordHash: deps.AdminPasswordHash,
		Secret:       deps.AdminSessionSecret,
	}, logger)
	registerAdminSessionRoutes(v1, adminSessions, deps.AdminAPIKey)
	adminV1 := v1.Group("")
	adminV1.Use(requireAdminAuth(deps.AdminAPIKey, adminSessions))
	registerAccountRoutes(adminV1, deps.Accounts)
	registerArticleRoutes(adminV1, deps.Articles)
	registerMaterialRoutes(adminV1, deps.Materials)
	registerPublishRoutes(adminV1, deps.Publishes)
	registerTokenRoutes(adminV1, deps.Tokens)
	registerTaskQueueRoutes(adminV1, deps.TaskQueues)
	registerDashboardRoutes(adminV1, deps.Dashboard)
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

func requireAdminAuth(expectedAPIKey string, sessions *adminSessionManager) gin.HandlerFunc {
	expectedAPIKey = strings.TrimSpace(expectedAPIKey)
	authEnabled := expectedAPIKey != "" || (sessions != nil && sessions.enabled())
	if !authEnabled {
		return func(c *gin.Context) {
			c.Next()
		}
	}
	return func(c *gin.Context) {
		if expectedAPIKey != "" && isValidAdminAPIKey(c, expectedAPIKey) {
			c.Next()
			return
		}
		if sessions != nil {
			session, ok := sessions.sessionFromRequest(c)
			if ok && sessions.validCSRF(c, session) {
				c.Next()
				return
			}
		}
		writeError(c, http.StatusUnauthorized, "unauthorized")
		c.Abort()
	}
}

func isValidAdminAPIKey(c *gin.Context, expected string) bool {
	expected = strings.TrimSpace(expected)
	if expected == "" {
		return false
	}
	actual := strings.TrimSpace(c.GetHeader("X-Admin-API-Key"))
	if actual == "" {
		actual = bearerToken(c.GetHeader("Authorization"))
	}
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
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

func registerArticleRoutes(r gin.IRouter, service *application.ArticleService) {
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
			Author: body.Author, Digest: body.Digest, ContentHTML: body.ContentHTML,
		})
		if !writeServiceError(c, err) {
			return
		}
		c.JSON(http.StatusCreated, articleResponse{
			ID: created.ID, TenantID: created.TenantID, AuthorizerID: created.AuthorizerID,
			Title: created.Title, Author: created.Author, Digest: created.Digest, ContentHTML: created.ContentHTML,
			CoverMediaAssetID: created.CoverMediaAssetID, Status: string(created.Status),
			CreatedAt: created.CreatedAt, UpdatedAt: created.UpdatedAt,
		})
	})
	r.GET("/articles", func(c *gin.Context) {
		tenant, ok := bindTenant(c)
		if !ok {
			return
		}
		items, err := service.ListArticles(c.Request.Context(), tenant)
		if !writeServiceError(c, err) {
			return
		}
		out := make([]articleResponse, 0, len(items))
		for _, item := range items {
			out = append(out, articleResponse{
				ID: item.ID, TenantID: item.TenantID, AuthorizerID: item.AuthorizerID,
				Title: item.Title, Author: item.Author, Digest: item.Digest, ContentHTML: item.ContentHTML,
				CoverMediaAssetID: item.CoverMediaAssetID, Status: string(item.Status),
				CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
			})
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
		c.JSON(http.StatusOK, articleResponse{
			ID: item.ID, TenantID: item.TenantID, AuthorizerID: item.AuthorizerID,
			Title: item.Title, Author: item.Author, Digest: item.Digest, ContentHTML: item.ContentHTML,
			CoverMediaAssetID: item.CoverMediaAssetID, Status: string(item.Status),
			CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
		})
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
		})
		if !writeServiceError(c, err) {
			return
		}
		c.JSON(http.StatusOK, articleResponse{
			ID: updated.ID, TenantID: updated.TenantID, AuthorizerID: updated.AuthorizerID,
			Title: updated.Title, Author: updated.Author, Digest: updated.Digest, ContentHTML: updated.ContentHTML,
			CoverMediaAssetID: updated.CoverMediaAssetID, Status: string(updated.Status),
			CreatedAt: updated.CreatedAt, UpdatedAt: updated.UpdatedAt,
		})
	})
	r.DELETE("/articles/:id", func(c *gin.Context) {
		tenant, id, ok := bindTenantAndID(c)
		if !ok {
			return
		}
		if !writeServiceError(c, service.DeleteArticle(c.Request.Context(), tenant, id)) {
			return
		}
		c.Status(http.StatusNoContent)
	})
}

func bindTenant(c *gin.Context) (string, bool) {
	var header tenantHeader
	if err := c.ShouldBindHeader(&header); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request")
		return "", false
	}
	return header.TenantID, true
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
		zap.String("tenant_id", strings.TrimSpace(c.GetHeader("X-Tenant-ID"))),
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
