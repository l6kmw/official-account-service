package http

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

const (
	adminSessionCookieName = "oa_admin_session"
	adminSessionTTL        = 12 * time.Hour
	adminCSRFHeaderName    = "X-CSRF-Token"
	maxAdminLoginFailures  = 5
	adminLoginLockout      = 5 * time.Minute
)

type adminSessionConfig struct {
	Username     string
	PasswordHash string
	Secret       string
}

type adminSessionManager struct {
	username     string
	passwordHash []byte
	secret       []byte
	now          func() time.Time
	logger       *zap.Logger
	mu           sync.Mutex
	failures     map[string]adminLoginFailure
}

type adminLoginFailure struct {
	Count       int
	LockedUntil time.Time
}

type adminSessionClaims struct {
	Username  string `json:"username"`
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
	CSRFToken     string `json:"csrf_token,omitempty"`
}

func newAdminSessionManager(cfg adminSessionConfig, logger *zap.Logger) *adminSessionManager {
	username := strings.TrimSpace(cfg.Username)
	passwordHash := strings.TrimSpace(cfg.PasswordHash)
	secret := strings.TrimSpace(cfg.Secret)
	if username == "" || passwordHash == "" || secret == "" {
		return nil
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &adminSessionManager{
		username:     username,
		passwordHash: []byte(passwordHash),
		secret:       []byte(secret),
		now:          time.Now,
		logger:       logger,
		failures:     make(map[string]adminLoginFailure),
	}
}

func registerAdminSessionRoutes(r gin.IRouter, sessions *adminSessionManager, adminAPIKey string) {
	r.GET("/admin/session", func(c *gin.Context) {
		authEnabled := strings.TrimSpace(adminAPIKey) != "" || (sessions != nil && sessions.enabled())
		response := adminSessionResponse{
			AuthEnabled:  authEnabled,
			LoginEnabled: sessions != nil && sessions.enabled(),
		}
		if strings.TrimSpace(adminAPIKey) != "" && isValidAdminAPIKey(c, adminAPIKey) {
			response.Authenticated = true
			response.Username = "api-key"
			c.JSON(http.StatusOK, response)
			return
		}
		if sessions != nil {
			if session, ok := sessions.sessionFromRequest(c); ok {
				response.Authenticated = true
				response.Username = session.Username
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
		session, ok := sessions.authenticate(body.Username, body.Password)
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
	return m != nil && m.username != "" && len(m.passwordHash) > 0 && len(m.secret) > 0
}

func (m *adminSessionManager) authenticate(username string, password string) (adminSessionClaims, bool) {
	userOK := constantTimeStringEqual(strings.TrimSpace(username), m.username)
	passwordOK := bcrypt.CompareHashAndPassword(m.passwordHash, []byte(password)) == nil
	if !userOK || !passwordOK {
		return adminSessionClaims{}, false
	}
	csrfToken, err := randomToken()
	if err != nil {
		m.logger.Error("generate admin csrf token", zap.String("error_code", "internal_error"), zap.Error(err))
		return adminSessionClaims{}, false
	}
	return adminSessionClaims{
		Username:  m.username,
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
	return m.verify(cookie)
}

func (m *adminSessionManager) verify(token string) (adminSessionClaims, bool) {
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
	if !constantTimeStringEqual(session.Username, m.username) {
		return adminSessionClaims{}, false
	}
	if session.CSRFToken == "" || session.ExpiresAt <= m.now().Unix() {
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
