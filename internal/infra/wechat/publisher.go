package wechat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"official-account-service/internal/domain/publish"
)

// PublisherConfig contains WeChat free publish client settings.
type PublisherConfig struct {
	BaseURL      string
	HTTPClient   *http.Client
	Timeout      time.Duration
	MaxRetries   int
	RetryBackoff time.Duration
}

// Publisher calls WeChat draft and freepublish APIs.
type Publisher struct {
	baseURL      string
	httpClient   *http.Client
	maxRetries   int
	retryBackoff time.Duration
}

// NewPublisher constructs a WeChat free publish client.
func NewPublisher(cfg PublisherConfig) (*Publisher, error) {
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
	return &Publisher{baseURL: baseURL, httpClient: httpClient, maxRetries: maxRetries, retryBackoff: retryBackoff}, nil
}

// AddDraft creates a WeChat article draft.
func (p *Publisher) AddDraft(ctx context.Context, authorizerAccessToken string, draft publish.ArticleDraft) (publish.DraftResult, error) {
	if strings.TrimSpace(draft.Title) == "" || strings.TrimSpace(draft.ContentHTML) == "" || strings.TrimSpace(draft.ThumbMediaID) == "" {
		return publish.DraftResult{}, fmt.Errorf("validate draft payload: %w", publish.ErrPublisherUnavailable)
	}
	var response draftAddResponse
	err := p.postJSON(ctx, "draft_add", "/draft/add", authorizerAccessToken, draftAddRequest{Articles: []draftArticle{{
		Title:        draft.Title,
		Author:       draft.Author,
		Digest:       draft.Digest,
		Content:      draft.ContentHTML,
		ThumbMediaID: draft.ThumbMediaID,
	}}}, &response)
	if err != nil {
		return publish.DraftResult{}, err
	}
	if strings.TrimSpace(response.MediaID) == "" {
		return publish.DraftResult{}, fmt.Errorf("validate draft add response: %w", publish.ErrPublishFailed)
	}
	return publish.DraftResult{MediaID: response.MediaID}, nil
}

// SubmitFreePublish submits a WeChat draft for asynchronous publishing.
func (p *Publisher) SubmitFreePublish(ctx context.Context, authorizerAccessToken string, mediaID string) (publish.SubmitResult, error) {
	if strings.TrimSpace(mediaID) == "" {
		return publish.SubmitResult{}, fmt.Errorf("validate free publish media id: %w", publish.ErrPublisherUnavailable)
	}
	var response freePublishSubmitResponse
	if err := p.postJSON(ctx, "freepublish_submit", "/freepublish/submit", authorizerAccessToken, freePublishSubmitRequest{MediaID: mediaID}, &response); err != nil {
		return publish.SubmitResult{}, err
	}
	publishID := response.PublishID.String()
	if strings.TrimSpace(publishID) == "" {
		return publish.SubmitResult{}, fmt.Errorf("validate free publish response: %w", publish.ErrPublishFailed)
	}
	return publish.SubmitResult{PublishID: publishID}, nil
}

// GetFreePublishStatus gets one WeChat asynchronous publish status.
func (p *Publisher) GetFreePublishStatus(ctx context.Context, authorizerAccessToken string, publishID string) (publish.StatusResult, error) {
	if strings.TrimSpace(publishID) == "" {
		return publish.StatusResult{}, fmt.Errorf("validate free publish id: %w", publish.ErrPublisherUnavailable)
	}
	var response freePublishStatusResponse
	if err := p.postJSON(ctx, "freepublish_get", "/freepublish/get", authorizerAccessToken, freePublishStatusRequest{PublishID: wechatPublishID(publishID)}, &response); err != nil {
		return publish.StatusResult{}, err
	}
	return response.statusResult(), nil
}

// DeleteFreePublish deletes one successfully published WeChat article.
func (p *Publisher) DeleteFreePublish(ctx context.Context, authorizerAccessToken string, articleID string, index int) error {
	if strings.TrimSpace(articleID) == "" || index < 0 {
		return fmt.Errorf("validate free publish delete input: %w", publish.ErrPublisherUnavailable)
	}
	var response freePublishDeleteResponse
	return p.postJSON(ctx, "freepublish_delete", "/freepublish/delete", authorizerAccessToken, freePublishDeleteRequest{
		ArticleID: articleID,
		Index:     index,
	}, &response)
}

func (p *Publisher) postJSON(ctx context.Context, operation string, path string, accessToken string, body any, out wechatPublishResponse) error {
	if p == nil || p.httpClient == nil {
		return fmt.Errorf("validate publisher: %w", publish.ErrPublisherUnavailable)
	}
	if strings.TrimSpace(accessToken) == "" {
		return fmt.Errorf("validate authorizer access token: %w", publish.ErrPublisherUnavailable)
	}
	var lastErr error
	for attempt := 0; attempt <= p.maxRetries; attempt++ {
		if attempt > 0 {
			if err := sleepContext(ctx, p.retryBackoff); err != nil {
				return fmt.Errorf("wait before retry %s: %w", operation, err)
			}
		}
		err := p.postJSONOnce(ctx, operation, path, accessToken, body, out)
		if err == nil {
			return nil
		}
		lastErr = err
		if !isRetryable(err) {
			break
		}
	}
	return lastErr
}

func (p *Publisher) postJSONOnce(ctx context.Context, operation string, path string, accessToken string, body any, out wechatPublishResponse) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal %s request: %w", operation, err)
	}
	endpoint, err := p.endpoint(path, accessToken)
	if err != nil {
		return fmt.Errorf("build %s endpoint: %w", operation, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create %s request: %w", operation, err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return retryableError{err: fmt.Errorf("send %s request: %w", operation, publish.ErrPublishFailed)}
	}
	defer resp.Body.Close()
	responseBytes, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return fmt.Errorf("read %s response: %w", operation, publish.ErrPublishFailed)
	}
	if resp.StatusCode >= http.StatusInternalServerError {
		return retryableError{err: fmt.Errorf("%s response status %d: %w", operation, resp.StatusCode, publish.ErrPublishFailed)}
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s response status %d: %w", operation, resp.StatusCode, publish.ErrPublishFailed)
	}
	if err := json.Unmarshal(responseBytes, out); err != nil {
		return fmt.Errorf("decode %s response: %w", operation, publish.ErrPublishFailed)
	}
	return out.wechatError(operation)
}

func (p *Publisher) endpoint(path string, accessToken string) (string, error) {
	parsed, err := url.Parse(p.baseURL + path)
	if err != nil {
		return "", fmt.Errorf("parse endpoint: %w", err)
	}
	query := parsed.Query()
	query.Set("access_token", accessToken)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

type wechatPublishResponse interface {
	wechatError(operation string) error
}

type draftAddRequest struct {
	Articles []draftArticle `json:"articles"`
}

type draftArticle struct {
	Title        string `json:"title"`
	Author       string `json:"author,omitempty"`
	Digest       string `json:"digest,omitempty"`
	Content      string `json:"content"`
	ThumbMediaID string `json:"thumb_media_id"`
}

type draftAddResponse struct {
	MediaID string `json:"media_id"`
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

func (r draftAddResponse) wechatError(operation string) error {
	return wechatPublishError(operation, r.ErrCode, r.ErrMsg)
}

type freePublishSubmitRequest struct {
	MediaID string `json:"media_id"`
}

type freePublishSubmitResponse struct {
	PublishID wechatPublishID `json:"publish_id"`
	ErrCode   int             `json:"errcode"`
	ErrMsg    string          `json:"errmsg"`
}

func (r freePublishSubmitResponse) wechatError(operation string) error {
	return wechatPublishError(operation, r.ErrCode, r.ErrMsg)
}

type freePublishStatusRequest struct {
	PublishID wechatPublishID `json:"publish_id"`
}

type freePublishStatusResponse struct {
	PublishID        wechatPublishID `json:"publish_id"`
	PublishStatus    int             `json:"publish_status"`
	ArticleID        string          `json:"article_id"`
	FailIndexes      []int           `json:"fail_idx"`
	ErrCode          int             `json:"errcode"`
	ErrMsg           string          `json:"errmsg"`
	ArticleDetailRaw struct{}        `json:"article_detail"`
}

type freePublishDeleteRequest struct {
	ArticleID string `json:"article_id"`
	Index     int    `json:"index"`
}

type freePublishDeleteResponse struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

func (r freePublishDeleteResponse) wechatError(operation string) error {
	return wechatPublishError(operation, r.ErrCode, r.ErrMsg)
}

type wechatPublishID string

func (id wechatPublishID) String() string {
	return string(id)
}

func (id wechatPublishID) MarshalJSON() ([]byte, error) {
	value := strings.TrimSpace(string(id))
	if isDecimalDigits(value) {
		return []byte(value), nil
	}
	return json.Marshal(value)
}

func (id *wechatPublishID) UnmarshalJSON(data []byte) error {
	value := strings.TrimSpace(string(data))
	if value == "" || value == "null" {
		*id = ""
		return nil
	}
	if strings.HasPrefix(value, "\"") {
		var decoded string
		if err := json.Unmarshal(data, &decoded); err != nil {
			return err
		}
		*id = wechatPublishID(decoded)
		return nil
	}
	var decoded json.Number
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*id = wechatPublishID(decoded.String())
	return nil
}

func isDecimalDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func (r freePublishStatusResponse) wechatError(operation string) error {
	return wechatPublishError(operation, r.ErrCode, r.ErrMsg)
}

func (r freePublishStatusResponse) statusResult() publish.StatusResult {
	if r.PublishStatus == 0 {
		return publish.StatusResult{Status: publish.StatusPublished, WeChatArticleID: r.ArticleID}
	}
	if r.PublishStatus == 1 {
		return publish.StatusResult{Status: publish.StatusPublishing}
	}
	return publish.StatusResult{
		Status:       publish.StatusFailed,
		ErrorCode:    fmt.Sprintf("publish_status_%d", r.PublishStatus),
		ErrorMessage: fmt.Sprintf("wechat publish failed with status %d", r.PublishStatus),
	}
}

func wechatPublishError(operation string, errCode int, errMsg string) error {
	if errCode == 0 {
		return nil
	}
	return fmt.Errorf("%s errcode %d errmsg %q: %w", operation, errCode, errMsg, publish.ErrPublishFailed)
}
