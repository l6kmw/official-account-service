package http

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"official-account-service/internal/application"
	"official-account-service/internal/domain/identity"
)

const (
	adminSessionCookieName = "oa_admin_session"
	adminSessionTTL        = 12 * time.Hour
	adminCSRFHeaderName    = "X-CSRF-Token"
	maxAdminLoginFailures  = 5
	adminLoginLockout      = 5 * time.Minute
)

type adminSessionConfig struct {
	Secret string
}

type adminSessionManager struct {
	identities identityAuthenticator
	secret     []byte
	now        func() time.Time
	logger     *zap.Logger
	mu         sync.Mutex
	failures   map[string]adminLoginFailure
}

type identityAuthenticator interface {
	Authenticate(ctx context.Context, username string, password string) (identity.User, error)
	GetActiveUser(ctx context.Context, userID string) (identity.User, error)
}

type adminLoginFailure struct {
	Count       int
	LockedUntil time.Time
}

type adminSessionClaims struct {
	Username  string `json:"username"`
	UserID    string `json:"user_id"`
	Role      string `json:"role"`
	ExpiresAt int64  `json:"expires_at"`
	CSRFToken string `json:"csrf_token"`
}

type adminLoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type adminSessionResponse struct {
	Authenticated bool   `json:"authenticated"`
	AuthEnabled   bool   `json:"auth_enabled"`
	LoginEnabled  bool   `json:"login_enabled"`
	Username      string `json:"username,omitempty"`
	UserID        string `json:"user_id,omitempty"`
	Role          string `json:"role,omitempty"`
	CSRFToken     string `json:"csrf_token,omitempty"`
}

func newAdminSessionManager(cfg adminSessionConfig, identities identityAuthenticator, logger *zap.Logger) *adminSessionManager {
	secret := strings.TrimSpace(cfg.Secret)
	if identities == nil || secret == "" {
		return nil
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &adminSessionManager{
		identities: identities,
		secret:     []byte(secret),
		now:        time.Now,
		logger:     logger,
		failures:   make(map[string]adminLoginFailure),
	}
}

func registerAdminSessionRoutes(r gin.IRouter, sessions *adminSessionManager, identities *application.IdentityService, adminAPIKey string, apiUserID string) {
	r.GET("/admin/session", func(c *gin.Context) {
		authEnabled := strings.TrimSpace(adminAPIKey) != "" || (sessions != nil && sessions.enabled())
		response := adminSessionResponse{
			AuthEnabled:  authEnabled,
			LoginEnabled: sessions != nil && sessions.enabled(),
		}
		if strings.TrimSpace(adminAPIKey) != "" && isValidAdminAPIKey(c, adminAPIKey) {
			response.Authenticated = true
			response.Username = "api-key"
			response.UserID = strings.TrimSpace(apiUserID)
			response.Role = string(identity.RoleAdmin)
			c.JSON(http.StatusOK, response)
			return
		}
		if identities != nil {
			if user, err := identities.AuthenticateAPIToken(c.Request.Context(), requestAPIToken(c)); err == nil {
				response.Authenticated = true
				response.Username = user.Username
				response.UserID = user.ID
				response.Role = string(user.Role)
				c.JSON(http.StatusOK, response)
				return
			}
		}
		if sessions != nil {
			if session, ok := sessions.sessionFromRequest(c); ok {
				response.Authenticated = true
				response.Username = session.Username
				response.UserID = session.UserID
				response.Role = session.Role
				response.CSRFToken = session.CSRFToken
				c.JSON(http.StatusOK, response)
				return
			}
		}
		if !authEnabled {
			response.Authenticated = true
		}
		c.JSON(http.StatusOK, response)
	})

	r.POST("/admin/session", func(c *gin.Context) {
		if sessions == nil || !sessions.enabled() {
			writeError(c, http.StatusNotImplemented, "not_implemented")
			return
		}
		clientKey := adminLoginClientKey(c)
		if sessions.isLocked(clientKey) {
			writeError(c, http.StatusTooManyRequests, "rate_limited")
			return
		}
		var body adminLoginRequest
		if err := c.ShouldBindJSON(&body); err != nil {
			writeRequestReadError(c, err)
			return
		}
		session, ok := sessions.authenticate(c.Request.Context(), body.Username, body.Password)
		if !ok {
			sessions.recordFailure(clientKey)
			writeError(c, http.StatusUnauthorized, "unauthorized")
			return
		}
		sessions.clearFailures(clientKey)
		token, err := sessions.sign(session)
		if err != nil {
			sessions.logger.Error("sign admin session", zap.String("error_code", "internal_error"), zap.Error(err))
			writeError(c, http.StatusInternalServerError, "internal_error")
			return
		}
		sessions.setSessionCookie(c, token, session.ExpiresAt)
		c.JSON(http.StatusOK, adminSessionResponse{
			Authenticated: true,
			AuthEnabled:   true,
			LoginEnabled:  true,
			Username:      session.Username,
			UserID:        session.UserID,
			Role:          session.Role,
			CSRFToken:     session.CSRFToken,
		})
	})

	r.DELETE("/admin/session", func(c *gin.Context) {
		if sessions != nil {
			sessions.clearSessionCookie(c)
		}
		c.JSON(http.StatusOK, adminSessionResponse{
			Authenticated: false,
			AuthEnabled:   strings.TrimSpace(adminAPIKey) != "" || (sessions != nil && sessions.enabled()),
			LoginEnabled:  sessions != nil && sessions.enabled(),
		})
	})
}

func (m *adminSessionManager) enabled() bool {
	return m != nil && m.identities != nil && len(m.secret) > 0
}

func (m *adminSessionManager) authenticate(ctx context.Context, username string, password string) (adminSessionClaims, bool) {
	user, err := m.identities.Authenticate(ctx, username, password)
	if err != nil {
		if !errors.Is(err, application.ErrInvalidCredentials) {
			m.logger.Error("authenticate user", zap.String("error_code", "internal_error"), zap.Error(err))
		}
		return adminSessionClaims{}, false
	}
	csrfToken, err := randomToken()
	if err != nil {
		m.logger.Error("generate admin csrf token", zap.String("error_code", "internal_error"), zap.Error(err))
		return adminSessionClaims{}, false
	}
	return adminSessionClaims{
		Username:  user.Username,
		UserID:    user.ID,
		Role:      string(user.Role),
		ExpiresAt: m.now().Add(adminSessionTTL).Unix(),
		CSRFToken: csrfToken,
	}, true
}

func (m *adminSessionManager) sign(session adminSessionClaims) (string, error) {
	payload, err := json.Marshal(session)
	if err != nil {
		return "", err
	}
	encodedPayload := base64.RawURLEncoding.EncodeToString(payload)
	signature := m.signature(encodedPayload)
	return encodedPayload + "." + signature, nil
}

func (m *adminSessionManager) sessionFromRequest(c *gin.Context) (adminSessionClaims, bool) {
	cookie, err := c.Cookie(adminSessionCookieName)
	if err != nil || strings.TrimSpace(cookie) == "" {
		return adminSessionClaims{}, false
	}
	return m.verify(c.Request.Context(), cookie)
}

func (m *adminSessionManager) verify(ctx context.Context, token string) (adminSessionClaims, bool) {
	payload, signature, ok := strings.Cut(strings.TrimSpace(token), ".")
	if !ok || payload == "" || signature == "" {
		return adminSessionClaims{}, false
	}
	if !constantTimeStringEqual(signature, m.signature(payload)) {
		return adminSessionClaims{}, false
	}
	rawPayload, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return adminSessionClaims{}, false
	}
	var session adminSessionClaims
	if err := json.Unmarshal(rawPayload, &session); err != nil {
		return adminSessionClaims{}, false
	}
	if session.UserID == "" || session.Username == "" || session.CSRFToken == "" || session.ExpiresAt <= m.now().Unix() {
		return adminSessionClaims{}, false
	}
	user, err := m.identities.GetActiveUser(ctx, session.UserID)
	if err != nil || !constantTimeStringEqual(user.Username, session.Username) || string(user.Role) != session.Role {
		return adminSessionClaims{}, false
	}
	return session, true
}

func (m *adminSessionManager) signature(payload string) string {
	mac := hmac.New(sha256.New, m.secret)
	_, _ = mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (m *adminSessionManager) validCSRF(c *gin.Context, session adminSessionClaims) bool {
	if isSafeHTTPMethod(c.Request.Method) {
		return true
	}
	return constantTimeStringEqual(strings.TrimSpace(c.GetHeader(adminCSRFHeaderName)), session.CSRFToken)
}

func (m *adminSessionManager) setSessionCookie(c *gin.Context, token string, expiresAt int64) {
	expires := time.Unix(expiresAt, 0)
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     adminSessionCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expires,
		MaxAge:   int(time.Until(expires).Seconds()),
		HttpOnly: true,
		Secure:   secureAdminCookie(c),
		SameSite: http.SameSiteStrictMode,
	})
}

func (m *adminSessionManager) clearSessionCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     adminSessionCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secureAdminCookie(c),
		SameSite: http.SameSiteStrictMode,
	})
}

func (m *adminSessionManager) isLocked(clientKey string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	failure := m.failures[clientKey]
	if failure.LockedUntil.IsZero() {
		return false
	}
	if !failure.LockedUntil.After(m.now()) {
		delete(m.failures, clientKey)
		return false
	}
	return true
}

func (m *adminSessionManager) recordFailure(clientKey string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	failure := m.failures[clientKey]
	failure.Count++
	if failure.Count >= maxAdminLoginFailures {
		failure.LockedUntil = m.now().Add(adminLoginLockout)
	}
	m.failures[clientKey] = failure
}

func (m *adminSessionManager) clearFailures(clientKey string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.failures, clientKey)
}

func isSafeHTTPMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	default:
		return false
	}
}

func randomToken() (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw[:]), nil
}

func secureAdminCookie(c *gin.Context) bool {
	if c.Request != nil {
		if c.Request.TLS != nil {
			return true
		}
		if strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") {
			return true
		}
		host := c.Request.Host
		if hostWithoutPort, _, err := net.SplitHostPort(host); err == nil {
			host = hostWithoutPort
		}
		if host != "localhost" && host != "127.0.0.1" && host != "::1" {
			return true
		}
	}
	return false
}

func adminLoginClientKey(c *gin.Context) string {
	if forwardedFor := strings.TrimSpace(c.GetHeader("X-Forwarded-For")); forwardedFor != "" {
		first, _, _ := strings.Cut(forwardedFor, ",")
		return strings.TrimSpace(first)
	}
	return strings.TrimSpace(c.ClientIP())
}
