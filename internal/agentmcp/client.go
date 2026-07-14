package agentmcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	defaultBaseURL = "http://localhost:8080"
	defaultTenant  = "tenant-1"
)

// Config contains the HTTP settings for the Official Account MCP adapter.
type Config struct {
	BaseURL        string
	PublicBaseURL  string
	ComponentAppID string
	TenantID       string
	AdminAPIKey    string
	HTTPClient     *http.Client
}

// ConfigFromEnv reads MCP adapter settings from environment variables.
func ConfigFromEnv() Config {
	return Config{
		BaseURL:        os.Getenv("OFFICIAL_ACCOUNT_BASE_URL"),
		PublicBaseURL:  os.Getenv("OFFICIAL_ACCOUNT_PUBLIC_BASE_URL"),
		ComponentAppID: os.Getenv("OFFICIAL_ACCOUNT_COMPONENT_APP_ID"),
		TenantID:       os.Getenv("OFFICIAL_ACCOUNT_TENANT_ID"),
		AdminAPIKey:    os.Getenv("OFFICIAL_ACCOUNT_ADMIN_API_KEY"),
	}
}

// Client calls the existing Official Account Service HTTP API.
type Client struct {
	baseURL        string
	publicBaseURL  string
	componentAppID string
	tenantID       string
	adminAPIKey    string
	httpClient     *http.Client
}

// NewClient constructs a tenant-scoped HTTP API client.
func NewClient(cfg Config) (*Client, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("validate official account base url: %w", ErrInvalidConfig)
	}
	publicBaseURL := strings.TrimRight(strings.TrimSpace(cfg.PublicBaseURL), "/")
	if publicBaseURL == "" {
		publicBaseURL = baseURL
	}
	parsedPublic, err := url.Parse(publicBaseURL)
	if err != nil || parsedPublic.Scheme == "" || parsedPublic.Host == "" {
		return nil, fmt.Errorf("validate official account public base url: %w", ErrInvalidConfig)
	}
	tenantID := strings.TrimSpace(cfg.TenantID)
	if tenantID == "" {
		tenantID = defaultTenant
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 20 * time.Second}
	}
	return &Client{
		baseURL:        baseURL,
		publicBaseURL:  publicBaseURL,
		componentAppID: strings.TrimSpace(cfg.ComponentAppID),
		tenantID:       tenantID,
		adminAPIKey:    strings.TrimSpace(cfg.AdminAPIKey),
		httpClient:     httpClient,
	}, nil
}

// ErrInvalidConfig indicates invalid MCP adapter configuration.
var ErrInvalidConfig = errors.New("invalid mcp adapter config")

type Account struct {
	ID           int64     `json:"id"`
	TenantID     string    `json:"tenant_id"`
	AppID        string    `json:"app_id"`
	Name         string    `json:"name"`
	AvatarURL    string    `json:"avatar_url"`
	Status       string    `json:"status"`
	LastSyncedAt time.Time `json:"last_synced_at"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Article struct {
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

type PublishedArticle struct {
	ArticleID          string `json:"article_id"`
	Index              int    `json:"index"`
	UpdateTime         int64  `json:"update_time"`
	Title              string `json:"title"`
	Author             string `json:"author"`
	Digest             string `json:"digest"`
	ContentHTML        string `json:"content_html,omitempty"`
	ContentSourceURL   string `json:"content_source_url"`
	ThumbMediaID       string `json:"thumb_media_id"`
	ThumbURL           string `json:"thumb_url"`
	URL                string `json:"url"`
	NeedOpenComment    bool   `json:"need_open_comment"`
	OnlyFansCanComment bool   `json:"only_fans_can_comment"`
	Deleted            bool   `json:"deleted"`
}

type PublishedArticleList struct {
	TotalMessageCount    int                `json:"total_message_count"`
	FetchedMessageCount  int                `json:"fetched_message_count"`
	ReturnedArticleCount int                `json:"returned_article_count"`
	NextOffset           int                `json:"next_offset"`
	HasMore              bool               `json:"has_more"`
	Items                []PublishedArticle `json:"items"`
}

type MaterialAsset struct {
	ID           int64     `json:"id"`
	TenantID     string    `json:"tenant_id"`
	AuthorizerID int64     `json:"authorizer_id"`
	ArticleID    int64     `json:"article_id"`
	Usage        string    `json:"usage"`
	LocalURL     string    `json:"local_url"`
	WeChatURL    string    `json:"wechat_url"`
	MediaID      string    `json:"media_id"`
	CreatedAt    time.Time `json:"created_at"`
}

type PublishRecord struct {
	ID              int64     `json:"id"`
	TenantID        string    `json:"tenant_id"`
	AuthorizerID    int64     `json:"authorizer_id"`
	ArticleID       int64     `json:"article_id"`
	WeChatPublishID string    `json:"wechat_publish_id"`
	WeChatArticleID string    `json:"wechat_article_id"`
	Status          string    `json:"status"`
	ErrorCode       string    `json:"error_code"`
	ErrorMessage    string    `json:"error_message"`
	SubmittedAt     time.Time `json:"submitted_at"`
	FinishedAt      time.Time `json:"finished_at"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type AuthorizationURL struct {
	AuthorizationURL        string `json:"authorization_url"`
	PreAuthCodeExpiresInSec int    `json:"pre_auth_code_expires_in_sec"`
}

type AuthorizationEntry struct {
	AuthorizationEntryURL string `json:"authorization_entry_url"`
	QRCodePayloadURL      string `json:"qr_code_payload_url"`
	TenantID              string `json:"tenant_id"`
	ComponentAppID        string `json:"component_appid"`
}

type CreateArticleInput struct {
	AuthorizerID int64  `json:"authorizer_id"`
	Title        string `json:"title"`
	Author       string `json:"author"`
	Digest       string `json:"digest"`
	ContentHTML  string `json:"content_html"`
}

type UpdateArticleInput struct {
	Title             string `json:"title"`
	Author            string `json:"author"`
	Digest            string `json:"digest"`
	ContentHTML       string `json:"content_html"`
	CoverMediaAssetID int64  `json:"cover_media_asset_id"`
}

type UploadImageInput struct {
	AuthorizerID int64
	ArticleID    int64
	Usage        string
	Filename     string
	Content      io.Reader
}

// ListAccounts returns authorized official-account summaries.
func (c *Client) ListAccounts(ctx context.Context) ([]Account, error) {
	var out struct {
		Items []Account `json:"items"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/api/v1/accounts", nil, &out, http.StatusOK); err != nil {
		return nil, err
	}
	return out.Items, nil
}

// ListArticles returns tenant-scoped articles.
func (c *Client) ListArticles(ctx context.Context) ([]Article, error) {
	var out struct {
		Items []Article `json:"items"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/api/v1/articles", nil, &out, http.StatusOK); err != nil {
		return nil, err
	}
	return out.Items, nil
}

// ListPublishedArticles returns a live page from the authorized WeChat account.
func (c *Client) ListPublishedArticles(ctx context.Context, authorizerID int64, offset, count int, includeContent, includeDeleted bool) (PublishedArticleList, error) {
	query := url.Values{}
	query.Set("offset", fmt.Sprintf("%d", offset))
	query.Set("count", fmt.Sprintf("%d", count))
	query.Set("include_content", fmt.Sprintf("%t", includeContent))
	query.Set("include_deleted", fmt.Sprintf("%t", includeDeleted))
	var out PublishedArticleList
	path := fmt.Sprintf("/api/v1/accounts/%d/published-articles?%s", authorizerID, query.Encode())
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &out, http.StatusOK); err != nil {
		return PublishedArticleList{}, err
	}
	return out, nil
}

// GetArticle returns one tenant-scoped article.
func (c *Client) GetArticle(ctx context.Context, id int64) (Article, error) {
	var out Article
	if err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/api/v1/articles/%d", id), nil, &out, http.StatusOK); err != nil {
		return Article{}, err
	}
	return out, nil
}

// CreateArticle creates a draft article.
func (c *Client) CreateArticle(ctx context.Context, input CreateArticleInput) (Article, error) {
	var out Article
	if err := c.doJSON(ctx, http.MethodPost, "/api/v1/articles", input, &out, http.StatusCreated); err != nil {
		return Article{}, err
	}
	return out, nil
}

// UpdateArticle updates article content and cover binding.
func (c *Client) UpdateArticle(ctx context.Context, id int64, input UpdateArticleInput) (Article, error) {
	var out Article
	if err := c.doJSON(ctx, http.MethodPut, fmt.Sprintf("/api/v1/articles/%d", id), input, &out, http.StatusOK); err != nil {
		return Article{}, err
	}
	return out, nil
}

// DeleteArticle deletes one article and lets the backend remove any published WeChat copies first.
func (c *Client) DeleteArticle(ctx context.Context, id int64) (Article, error) {
	article, err := c.GetArticle(ctx, id)
	if err != nil {
		return Article{}, err
	}
	if err := c.doJSON(ctx, http.MethodDelete, fmt.Sprintf("/api/v1/articles/%d", id), nil, nil, http.StatusNoContent); err != nil {
		return Article{}, err
	}
	return article, nil
}

// UploadImage uploads a body image or cover image.
func (c *Client) UploadImage(ctx context.Context, input UploadImageInput) (MaterialAsset, error) {
	if input.Content == nil {
		return MaterialAsset{}, fmt.Errorf("validate image content: %w", ErrInvalidConfig)
	}
	endpoint := "/api/v1/materials/inline-images"
	if input.Usage == "cover" {
		endpoint = "/api/v1/materials/covers"
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("authorizer_id", fmt.Sprintf("%d", input.AuthorizerID)); err != nil {
		return MaterialAsset{}, err
	}
	if err := writer.WriteField("article_id", fmt.Sprintf("%d", input.ArticleID)); err != nil {
		return MaterialAsset{}, err
	}
	filename := strings.TrimSpace(input.Filename)
	if filename == "" {
		filename = "image.png"
	}
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return MaterialAsset{}, fmt.Errorf("create upload field: %w", err)
	}
	if _, err := io.Copy(part, input.Content); err != nil {
		return MaterialAsset{}, fmt.Errorf("copy upload content: %w", err)
	}
	if err := writer.Close(); err != nil {
		return MaterialAsset{}, fmt.Errorf("close multipart upload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url(endpoint), &body)
	if err != nil {
		return MaterialAsset{}, fmt.Errorf("create upload request: %w", err)
	}
	c.addHeaders(req)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	var out MaterialAsset
	if err := c.do(req, &out, http.StatusCreated); err != nil {
		return MaterialAsset{}, err
	}
	return out, nil
}

// PublishArticle submits one article to WeChat free publish.
func (c *Client) PublishArticle(ctx context.Context, articleID int64) (PublishRecord, error) {
	var out PublishRecord
	if err := c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/api/v1/articles/%d/publish", articleID), nil, &out, http.StatusCreated); err != nil {
		return PublishRecord{}, err
	}
	return out, nil
}

// ListPublishRecords returns tenant publish records.
func (c *Client) ListPublishRecords(ctx context.Context) ([]PublishRecord, error) {
	var out struct {
		Items []PublishRecord `json:"items"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/api/v1/publish-records", nil, &out, http.StatusOK); err != nil {
		return nil, err
	}
	return out.Items, nil
}

// SyncPublishStatus polls WeChat and updates one publish record.
func (c *Client) SyncPublishStatus(ctx context.Context, recordID int64) (PublishRecord, error) {
	var out PublishRecord
	if err := c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/api/v1/publish-records/%d/sync-status", recordID), nil, &out, http.StatusOK); err != nil {
		return PublishRecord{}, err
	}
	return out, nil
}

// DeletePublishedRecord deletes one published WeChat copy by publish record.
func (c *Client) DeletePublishedRecord(ctx context.Context, recordID int64) (PublishRecord, error) {
	var out PublishRecord
	if err := c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/api/v1/publish-records/%d/delete-published", recordID), nil, &out, http.StatusOK); err != nil {
		return PublishRecord{}, err
	}
	return out, nil
}

// GenerateAuthorizationURL returns a WeChat component authorization URL.
func (c *Client) GenerateAuthorizationURL(ctx context.Context, componentAppID, redirectURI string, authType int, bizAppID string) (AuthorizationURL, error) {
	componentAppID = c.resolveComponentAppID(componentAppID)
	if componentAppID == "" {
		return AuthorizationURL{}, fmt.Errorf("validate component appid: %w", ErrInvalidConfig)
	}
	redirectURI = strings.TrimSpace(redirectURI)
	if redirectURI == "" {
		redirectURI = c.authorizationCallbackURL()
	}
	query := url.Values{}
	query.Set("tenant_id", c.tenantID)
	query.Set("component_appid", componentAppID)
	query.Set("redirect_uri", redirectURI)
	if authType > 0 {
		query.Set("auth_type", fmt.Sprintf("%d", authType))
	}
	if strings.TrimSpace(bizAppID) != "" {
		query.Set("biz_appid", bizAppID)
	}
	var out AuthorizationURL
	if err := c.doJSON(ctx, http.MethodGet, "/api/v1/wechat/authorization-url?"+query.Encode(), nil, &out, http.StatusOK); err != nil {
		return AuthorizationURL{}, err
	}
	return out, nil
}

// AuthorizationEntry returns the public authorization entry link suitable for a QR code.
func (c *Client) AuthorizationEntry(componentAppID string) (AuthorizationEntry, error) {
	componentAppID = c.resolveComponentAppID(componentAppID)
	if componentAppID == "" {
		return AuthorizationEntry{}, fmt.Errorf("validate component appid: %w", ErrInvalidConfig)
	}
	entryURL, err := url.Parse(c.publicBaseURL + "/wechat-authorize.html")
	if err != nil {
		return AuthorizationEntry{}, fmt.Errorf("build authorization entry url: %w", err)
	}
	query := entryURL.Query()
	query.Set("tenant_id", c.tenantID)
	query.Set("component_appid", componentAppID)
	entryURL.RawQuery = query.Encode()
	return AuthorizationEntry{
		AuthorizationEntryURL: entryURL.String(),
		QRCodePayloadURL:      entryURL.String(),
		TenantID:              c.tenantID,
		ComponentAppID:        componentAppID,
	}, nil
}

func (c *Client) resolveComponentAppID(componentAppID string) string {
	componentAppID = strings.TrimSpace(componentAppID)
	if componentAppID != "" {
		return componentAppID
	}
	return c.componentAppID
}

func (c *Client) authorizationCallbackURL() string {
	callbackURL, err := url.Parse(c.publicBaseURL + "/api/v1/wechat/authorization-callback")
	if err != nil {
		return ""
	}
	return callbackURL.String()
}

func (c *Client) doJSON(ctx context.Context, method, path string, body any, out any, wantStatus int) error {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode request body: %w", err)
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.url(path), reader)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	c.addHeaders(req)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.do(req, out, wantStatus)
}

func (c *Client) do(req *http.Request, out any, wantStatus int) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != wantStatus {
		return c.responseError(resp)
	}
	if out == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func (c *Client) addHeaders(req *http.Request) {
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Tenant-ID", c.tenantID)
	if c.adminAPIKey != "" {
		req.Header.Set("X-Admin-API-Key", c.adminAPIKey)
	}
}

func (c *Client) url(path string) string {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	return c.baseURL + path
}

func (c *Client) responseError(resp *http.Response) error {
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 32<<10))
	var apiErr struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(raw, &apiErr); err == nil && apiErr.Error != "" {
		return fmt.Errorf("official account api %s %s returned %d: %s", resp.Request.Method, resp.Request.URL.Path, resp.StatusCode, apiErr.Error)
	}
	body := strings.TrimSpace(string(raw))
	if body == "" {
		body = resp.Status
	}
	return fmt.Errorf("official account api %s %s returned %d: %s", resp.Request.Method, resp.Request.URL.Path, resp.StatusCode, body)
}
