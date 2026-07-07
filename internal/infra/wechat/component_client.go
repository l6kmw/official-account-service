package wechat

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"official-account-service/internal/domain/authorization"
)

const (
	defaultAPIBaseURL         = "https://api.weixin.qq.com/cgi-bin"
	defaultHTTPTimeout        = 5 * time.Second
	defaultMaxRetries         = 1
	defaultRetryBackoff       = 100 * time.Millisecond
	defaultBreakerThreshold   = 3
	defaultBreakerOpenTimeout = 30 * time.Second
)

// ComponentClientConfig contains WeChat third-party platform client settings.
type ComponentClientConfig struct {
	ComponentAppSecret string
	BaseURL            string
	HTTPClient         *http.Client
	Timeout            time.Duration
	MaxRetries         int
	RetryBackoff       time.Duration
	BreakerThreshold   int
	BreakerOpenTimeout time.Duration
}

// ComponentClient calls WeChat third-party platform APIs.
type ComponentClient struct {
	componentAppSecret string
	baseURL            string
	httpClient         *http.Client
	maxRetries         int
	retryBackoff       time.Duration
	breaker            *circuitBreaker
	tickets            authorization.ComponentVerifyTicketRepository
}

// NewComponentClient constructs a ComponentClient.
func NewComponentClient(tickets authorization.ComponentVerifyTicketRepository, cfg ComponentClientConfig) (*ComponentClient, error) {
	if tickets == nil {
		return nil, fmt.Errorf("validate component client ticket repository: %w", authorization.ErrPreAuthCodeUnavailable)
	}
	if strings.TrimSpace(cfg.ComponentAppSecret) == "" {
		return nil, fmt.Errorf("validate component client credentials: %w", authorization.ErrPreAuthCodeUnavailable)
	}
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		baseURL = defaultAPIBaseURL
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultHTTPTimeout
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: timeout}
	}
	maxRetries := cfg.MaxRetries
	if maxRetries < 0 {
		maxRetries = 0
	}
	if maxRetries == 0 {
		maxRetries = defaultMaxRetries
	}
	retryBackoff := cfg.RetryBackoff
	if retryBackoff <= 0 {
		retryBackoff = defaultRetryBackoff
	}
	breakerThreshold := cfg.BreakerThreshold
	if breakerThreshold <= 0 {
		breakerThreshold = defaultBreakerThreshold
	}
	breakerOpenTimeout := cfg.BreakerOpenTimeout
	if breakerOpenTimeout <= 0 {
		breakerOpenTimeout = defaultBreakerOpenTimeout
	}
	return &ComponentClient{
		componentAppSecret: cfg.ComponentAppSecret,
		baseURL:            baseURL,
		httpClient:         httpClient,
		maxRetries:         maxRetries,
		retryBackoff:       retryBackoff,
		breaker:            newCircuitBreaker(breakerThreshold, breakerOpenTimeout, time.Now),
		tickets:            tickets,
	}, nil
}

// CreatePreAuthCode creates a WeChat pre-authorization code.
func (c *ComponentClient) CreatePreAuthCode(ctx context.Context, componentAppID string) (authorization.PreAuthCode, error) {
	if strings.TrimSpace(componentAppID) == "" {
		return authorization.PreAuthCode{}, fmt.Errorf("validate component app id: %w", authorization.ErrPreAuthCodeUnavailable)
	}
	ticket, err := c.tickets.GetComponentVerifyTicket(ctx, componentAppID)
	if err != nil {
		return authorization.PreAuthCode{}, fmt.Errorf("get component verify ticket for pre auth code: %w", err)
	}
	componentToken, err := c.createComponentAccessToken(ctx, componentAppID, ticket.Ticket)
	if err != nil {
		return authorization.PreAuthCode{}, err
	}
	preAuthCode, err := c.createPreAuthCode(ctx, componentAppID, componentToken)
	if err != nil {
		return authorization.PreAuthCode{}, err
	}
	return preAuthCode, nil
}

// QueryAuthorizerAuthorization exchanges an auth code for authorizer authorization data.
func (c *ComponentClient) QueryAuthorizerAuthorization(ctx context.Context, componentAppID string, authCode string) (authorization.AuthorizerAuthorization, error) {
	if strings.TrimSpace(componentAppID) == "" || strings.TrimSpace(authCode) == "" {
		return authorization.AuthorizerAuthorization{}, fmt.Errorf("validate query authorizer authorization input: %w", authorization.ErrAuthorizerClientUnavailable)
	}
	componentToken, err := c.componentAccessTokenForApp(ctx, componentAppID)
	if err != nil {
		return authorization.AuthorizerAuthorization{}, fmt.Errorf("get component access token for query auth: %w", err)
	}
	query := url.Values{}
	query.Set("component_access_token", componentToken)
	var response queryAuthorizerAuthResponse
	err = c.postJSON(ctx, "query_authorizer_auth", "/component/api_query_auth", query, queryAuthorizerAuthRequest{
		ComponentAppID:    componentAppID,
		AuthorizationCode: authCode,
	}, &response)
	if err != nil {
		return authorization.AuthorizerAuthorization{}, fmt.Errorf("query authorizer authorization: %w", err)
	}
	if err := response.wechatError("query_authorizer_auth"); err != nil {
		return authorization.AuthorizerAuthorization{}, fmt.Errorf("query authorizer authorization: %w", err)
	}
	if strings.TrimSpace(response.AuthorizationInfo.AuthorizerAppID) == "" || strings.TrimSpace(response.AuthorizationInfo.AuthorizerRefreshToken) == "" {
		return authorization.AuthorizerAuthorization{}, fmt.Errorf("validate query authorizer authorization response: %w", authorization.ErrAuthorizerClientUnavailable)
	}
	return authorization.AuthorizerAuthorization{
		AppID:        response.AuthorizationInfo.AuthorizerAppID,
		RefreshToken: response.AuthorizationInfo.AuthorizerRefreshToken,
	}, nil
}

// GetAuthorizerProfile returns authorizer profile data.
func (c *ComponentClient) GetAuthorizerProfile(ctx context.Context, componentAppID string, authorizerAppID string) (authorization.AuthorizerProfile, error) {
	if strings.TrimSpace(componentAppID) == "" || strings.TrimSpace(authorizerAppID) == "" {
		return authorization.AuthorizerProfile{}, fmt.Errorf("validate get authorizer profile input: %w", authorization.ErrAuthorizerClientUnavailable)
	}
	componentToken, err := c.componentAccessTokenForApp(ctx, componentAppID)
	if err != nil {
		return authorization.AuthorizerProfile{}, fmt.Errorf("get component access token for authorizer info: %w", err)
	}
	query := url.Values{}
	query.Set("component_access_token", componentToken)
	var response authorizerInfoResponse
	err = c.postJSON(ctx, "get_authorizer_info", "/component/api_get_authorizer_info", query, authorizerInfoRequest{
		ComponentAppID:  componentAppID,
		AuthorizerAppID: authorizerAppID,
	}, &response)
	if err != nil {
		return authorization.AuthorizerProfile{}, fmt.Errorf("get authorizer profile: %w", err)
	}
	if err := response.wechatError("get_authorizer_info"); err != nil {
		return authorization.AuthorizerProfile{}, fmt.Errorf("get authorizer profile: %w", err)
	}
	return authorization.AuthorizerProfile{
		Name:      response.AuthorizerInfo.NickName,
		AvatarURL: response.AuthorizerInfo.HeadImage,
	}, nil
}

// RefreshAuthorizerAccessToken refreshes an authorizer access token.
func (c *ComponentClient) RefreshAuthorizerAccessToken(ctx context.Context, componentAppID string, authorizerAppID string, refreshToken string) (authorization.AuthorizerToken, error) {
	if strings.TrimSpace(componentAppID) == "" || strings.TrimSpace(authorizerAppID) == "" || strings.TrimSpace(refreshToken) == "" {
		return authorization.AuthorizerToken{}, fmt.Errorf("validate refresh authorizer token input: %w", authorization.ErrAuthorizerClientUnavailable)
	}
	componentToken, err := c.componentAccessTokenForApp(ctx, componentAppID)
	if err != nil {
		return authorization.AuthorizerToken{}, fmt.Errorf("get component access token for authorizer token: %w", err)
	}
	query := url.Values{}
	query.Set("component_access_token", componentToken)
	var response authorizerTokenResponse
	err = c.postJSON(ctx, "refresh_authorizer_token", "/component/api_authorizer_token", query, authorizerTokenRequest{
		ComponentAppID:         componentAppID,
		AuthorizerAppID:        authorizerAppID,
		AuthorizerRefreshToken: refreshToken,
	}, &response)
	if err != nil {
		return authorization.AuthorizerToken{}, fmt.Errorf("refresh authorizer access token: %w", err)
	}
	if err := response.wechatError("refresh_authorizer_token"); err != nil {
		return authorization.AuthorizerToken{}, fmt.Errorf("refresh authorizer access token: %w", err)
	}
	if strings.TrimSpace(response.AuthorizerAccessToken) == "" {
		return authorization.AuthorizerToken{}, fmt.Errorf("validate authorizer access token response: %w", authorization.ErrAuthorizerClientUnavailable)
	}
	return authorization.AuthorizerToken{
		AccessToken:      response.AuthorizerAccessToken,
		RefreshToken:     response.AuthorizerRefreshToken,
		ExpiresInSeconds: response.ExpiresIn,
	}, nil
}

func (c *ComponentClient) createComponentAccessToken(ctx context.Context, componentAppID string, ticket string) (string, error) {
	var response componentTokenResponse
	err := c.postJSON(ctx, "component_access_token", "/component/api_component_token", nil, componentTokenRequest{
		ComponentAppID:        componentAppID,
		ComponentAppSecret:    c.componentAppSecret,
		ComponentVerifyTicket: ticket,
	}, &response)
	if err != nil {
		return "", fmt.Errorf("create component access token: %w", err)
	}
	if err := response.wechatError("component_access_token"); err != nil {
		return "", fmt.Errorf("create component access token: %w", err)
	}
	if strings.TrimSpace(response.ComponentAccessToken) == "" {
		return "", fmt.Errorf("validate component access token response: %w", authorization.ErrPreAuthCodeUnavailable)
	}
	return response.ComponentAccessToken, nil
}

func (c *ComponentClient) componentAccessTokenForApp(ctx context.Context, componentAppID string) (string, error) {
	ticket, err := c.tickets.GetComponentVerifyTicket(ctx, componentAppID)
	if err != nil {
		return "", fmt.Errorf("get component verify ticket: %w", err)
	}
	componentToken, err := c.createComponentAccessToken(ctx, componentAppID, ticket.Ticket)
	if err != nil {
		return "", err
	}
	return componentToken, nil
}

func (c *ComponentClient) createPreAuthCode(ctx context.Context, componentAppID string, componentToken string) (authorization.PreAuthCode, error) {
	query := url.Values{}
	query.Set("component_access_token", componentToken)
	var response preAuthCodeResponse
	err := c.postJSON(ctx, "pre_auth_code", "/component/api_create_preauthcode", query, preAuthCodeRequest{
		ComponentAppID: componentAppID,
	}, &response)
	if err != nil {
		return authorization.PreAuthCode{}, fmt.Errorf("create pre auth code: %w", err)
	}
	if err := response.wechatError("pre_auth_code"); err != nil {
		return authorization.PreAuthCode{}, fmt.Errorf("create pre auth code: %w", err)
	}
	if strings.TrimSpace(response.PreAuthCode) == "" {
		return authorization.PreAuthCode{}, fmt.Errorf("validate pre auth code response: %w", authorization.ErrPreAuthCodeUnavailable)
	}
	return authorization.PreAuthCode{Code: response.PreAuthCode, ExpiresInSeconds: response.ExpiresIn}, nil
}

type componentTokenRequest struct {
	ComponentAppID        string `json:"component_appid"`
	ComponentAppSecret    string `json:"component_appsecret"`
	ComponentVerifyTicket string `json:"component_verify_ticket"`
}

type componentTokenResponse struct {
	ComponentAccessToken string `json:"component_access_token"`
	ExpiresIn            int    `json:"expires_in"`
	ErrCode              int    `json:"errcode"`
	ErrMsg               string `json:"errmsg"`
}

func (r componentTokenResponse) wechatError(operation string) error {
	if r.ErrCode == 0 {
		return nil
	}
	return fmt.Errorf("%s errcode %d errmsg %q: %w", operation, r.ErrCode, r.ErrMsg, authorization.ErrPreAuthCodeUnavailable)
}

type preAuthCodeRequest struct {
	ComponentAppID string `json:"component_appid"`
}

type preAuthCodeResponse struct {
	PreAuthCode string `json:"pre_auth_code"`
	ExpiresIn   int    `json:"expires_in"`
	ErrCode     int    `json:"errcode"`
	ErrMsg      string `json:"errmsg"`
}

func (r preAuthCodeResponse) wechatError(operation string) error {
	if r.ErrCode == 0 {
		return nil
	}
	return fmt.Errorf("%s errcode %d errmsg %q: %w", operation, r.ErrCode, r.ErrMsg, authorization.ErrPreAuthCodeUnavailable)
}

type queryAuthorizerAuthRequest struct {
	ComponentAppID    string `json:"component_appid"`
	AuthorizationCode string `json:"authorization_code"`
}

type queryAuthorizerAuthResponse struct {
	AuthorizationInfo struct {
		AuthorizerAppID        string `json:"authorizer_appid"`
		AuthorizerRefreshToken string `json:"authorizer_refresh_token"`
	} `json:"authorization_info"`
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

func (r queryAuthorizerAuthResponse) wechatError(operation string) error {
	if r.ErrCode == 0 {
		return nil
	}
	return fmt.Errorf("%s errcode %d errmsg %q: %w", operation, r.ErrCode, r.ErrMsg, authorization.ErrAuthorizerClientUnavailable)
}

type authorizerInfoRequest struct {
	ComponentAppID  string `json:"component_appid"`
	AuthorizerAppID string `json:"authorizer_appid"`
}

type authorizerInfoResponse struct {
	AuthorizerInfo struct {
		NickName  string `json:"nick_name"`
		HeadImage string `json:"head_img"`
	} `json:"authorizer_info"`
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

func (r authorizerInfoResponse) wechatError(operation string) error {
	if r.ErrCode == 0 {
		return nil
	}
	return fmt.Errorf("%s errcode %d errmsg %q: %w", operation, r.ErrCode, r.ErrMsg, authorization.ErrAuthorizerClientUnavailable)
}

type authorizerTokenRequest struct {
	ComponentAppID         string `json:"component_appid"`
	AuthorizerAppID        string `json:"authorizer_appid"`
	AuthorizerRefreshToken string `json:"authorizer_refresh_token"`
}

type authorizerTokenResponse struct {
	AuthorizerAccessToken  string `json:"authorizer_access_token"`
	AuthorizerRefreshToken string `json:"authorizer_refresh_token"`
	ExpiresIn              int    `json:"expires_in"`
	ErrCode                int    `json:"errcode"`
	ErrMsg                 string `json:"errmsg"`
}

func (r authorizerTokenResponse) wechatError(operation string) error {
	if r.ErrCode == 0 {
		return nil
	}
	return fmt.Errorf("%s errcode %d errmsg %q: %w", operation, r.ErrCode, r.ErrMsg, authorization.ErrAuthorizerClientUnavailable)
}
