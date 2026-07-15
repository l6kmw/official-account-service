package wechat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"

	"official-account-service/internal/domain/material"
)

// MaterialUploaderConfig contains WeChat material upload settings.
type MaterialUploaderConfig struct {
	BaseURL      string
	HTTPClient   *http.Client
	Timeout      time.Duration
	MaxRetries   int
	RetryBackoff time.Duration
}

// MaterialUploader uploads materials to WeChat official account APIs.
type MaterialUploader struct {
	baseURL      string
	httpClient   *http.Client
	maxRetries   int
	retryBackoff time.Duration
}

var _ material.PermanentManager = (*MaterialUploader)(nil)

// DisabledMaterialUploader is a placeholder until the real WeChat client is implemented.
type DisabledMaterialUploader struct{}

// NewMaterialUploader constructs a WeChat material uploader.
func NewMaterialUploader(cfg MaterialUploaderConfig) (*MaterialUploader, error) {
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
	return &MaterialUploader{baseURL: baseURL, httpClient: httpClient, maxRetries: maxRetries, retryBackoff: retryBackoff}, nil
}

// UploadInlineImage uploads an image for article body HTML and returns a WeChat URL.
func (u *MaterialUploader) UploadInlineImage(ctx context.Context, authorizerAccessToken string, filename string, content io.Reader) (material.InlineImageUpload, error) {
	var response materialUploadResponse
	if err := u.upload(ctx, "upload_inline_image", "/media/uploadimg", nil, authorizerAccessToken, filename, content, &response); err != nil {
		return material.InlineImageUpload{}, err
	}
	if strings.TrimSpace(response.URL) == "" {
		return material.InlineImageUpload{}, fmt.Errorf("validate inline image upload response: %w", material.ErrUploadFailed)
	}
	return material.InlineImageUpload{WeChatURL: response.URL}, nil
}

// UploadCover uploads a permanent image material and returns a media id for article covers.
func (u *MaterialUploader) UploadCover(ctx context.Context, authorizerAccessToken string, filename string, content io.Reader) (material.CoverUpload, error) {
	query := url.Values{}
	query.Set("type", "image")
	var response materialUploadResponse
	if err := u.upload(ctx, "upload_cover", "/material/add_material", query, authorizerAccessToken, filename, content, &response); err != nil {
		return material.CoverUpload{}, err
	}
	if strings.TrimSpace(response.MediaID) == "" {
		return material.CoverUpload{}, fmt.Errorf("validate cover upload response: %w", material.ErrUploadFailed)
	}
	return material.CoverUpload{MediaID: response.MediaID}, nil
}

// ListPermanentImages returns one live page from the authorized account's permanent image library.
func (u *MaterialUploader) ListPermanentImages(ctx context.Context, authorizerAccessToken string, offset int, count int) (material.PermanentImageBatch, error) {
	if u == nil || u.httpClient == nil || strings.TrimSpace(authorizerAccessToken) == "" || offset < 0 || count < 1 || count > 20 {
		return material.PermanentImageBatch{}, fmt.Errorf("validate permanent material list input: %w", material.ErrManagerUnavailable)
	}
	var response permanentMaterialListResponse
	err := u.postMaterialJSON(ctx, "list_permanent_materials", "/material/batchget_material", authorizerAccessToken, permanentMaterialListRequest{
		Type: "image", Offset: offset, Count: count,
	}, &response)
	if err != nil {
		return material.PermanentImageBatch{}, err
	}
	return material.PermanentImageBatch{TotalCount: response.TotalCount, ItemCount: response.ItemCount, Items: response.Items}, nil
}

// UploadInlineImage returns an unavailable error for inline image uploads.
func (DisabledMaterialUploader) UploadInlineImage(_ context.Context, _ string, _ string, _ io.Reader) (material.InlineImageUpload, error) {
	return material.InlineImageUpload{}, fmt.Errorf("wechat inline image uploader: %w", material.ErrUploaderUnavailable)
}

// UploadCover returns an unavailable error for cover uploads.
func (DisabledMaterialUploader) UploadCover(_ context.Context, _ string, _ string, _ io.Reader) (material.CoverUpload, error) {
	return material.CoverUpload{}, fmt.Errorf("wechat cover uploader: %w", material.ErrUploaderUnavailable)
}

func (u *MaterialUploader) upload(ctx context.Context, operation string, path string, query url.Values, accessToken string, filename string, content io.Reader, out *materialUploadResponse) error {
	if u == nil || u.httpClient == nil {
		return fmt.Errorf("validate material uploader: %w", material.ErrUploaderUnavailable)
	}
	if strings.TrimSpace(accessToken) == "" {
		return fmt.Errorf("validate authorizer access token: %w", material.ErrUploaderUnavailable)
	}
	if strings.TrimSpace(filename) == "" || content == nil {
		return fmt.Errorf("validate material upload input: %w", material.ErrUploaderUnavailable)
	}
	payload, err := io.ReadAll(content)
	if err != nil {
		return fmt.Errorf("read material upload content: %w", material.ErrUploadFailed)
	}
	var lastErr error
	for attempt := 0; attempt <= u.maxRetries; attempt++ {
		if attempt > 0 {
			if err := sleepContext(ctx, u.retryBackoff); err != nil {
				return fmt.Errorf("wait before retry %s: %w", operation, err)
			}
		}
		err := u.uploadOnce(ctx, operation, path, query, accessToken, filename, payload, out)
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

func (u *MaterialUploader) uploadOnce(ctx context.Context, operation string, path string, query url.Values, accessToken string, filename string, payload []byte, out *materialUploadResponse) error {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("media", filename)
	if err != nil {
		return fmt.Errorf("create %s multipart file: %w", operation, err)
	}
	if _, err := part.Write(payload); err != nil {
		return fmt.Errorf("write %s multipart file: %w", operation, err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("close %s multipart body: %w", operation, err)
	}
	endpoint, err := u.endpoint(path, query, accessToken)
	if err != nil {
		return fmt.Errorf("build %s endpoint: %w", operation, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, body)
	if err != nil {
		return fmt.Errorf("create %s request: %w", operation, err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := u.httpClient.Do(req)
	if err != nil {
		return retryableError{err: fmt.Errorf("send %s request: %w", operation, material.ErrUploadFailed)}
	}
	defer resp.Body.Close()
	responseBytes, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return fmt.Errorf("read %s response: %w", operation, material.ErrUploadFailed)
	}
	if resp.StatusCode >= http.StatusInternalServerError {
		return retryableError{err: fmt.Errorf("%s response status %d: %w", operation, resp.StatusCode, material.ErrUploadFailed)}
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s response status %d: %w", operation, resp.StatusCode, material.ErrUploadFailed)
	}
	if err := json.Unmarshal(responseBytes, out); err != nil {
		return fmt.Errorf("decode %s response: %w", operation, material.ErrUploadFailed)
	}
	if err := out.wechatError(operation); err != nil {
		return err
	}
	return nil
}

func (u *MaterialUploader) postMaterialJSON(ctx context.Context, operation string, path string, accessToken string, body any, out materialJSONResponse) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal %s request: %w", operation, material.ErrManagementFailed)
	}
	var lastErr error
	for attempt := 0; attempt <= u.maxRetries; attempt++ {
		if attempt > 0 {
			if err := sleepContext(ctx, u.retryBackoff); err != nil {
				return fmt.Errorf("wait before retry %s: %w", operation, err)
			}
		}
		lastErr = u.postMaterialJSONOnce(ctx, operation, path, accessToken, payload, out)
		if lastErr == nil || !isRetryable(lastErr) {
			return lastErr
		}
	}
	return lastErr
}

func (u *MaterialUploader) postMaterialJSONOnce(ctx context.Context, operation string, path string, accessToken string, payload []byte, out materialJSONResponse) error {
	endpoint, err := u.endpoint(path, nil, accessToken)
	if err != nil {
		return fmt.Errorf("build %s endpoint: %w", operation, material.ErrManagementFailed)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create %s request: %w", operation, material.ErrManagementFailed)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := u.httpClient.Do(req)
	if err != nil {
		return retryableError{err: fmt.Errorf("send %s request: %w", operation, material.ErrManagementFailed)}
	}
	defer resp.Body.Close()
	responseBytes, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return fmt.Errorf("read %s response: %w", operation, material.ErrManagementFailed)
	}
	if resp.StatusCode >= http.StatusInternalServerError {
		return retryableError{err: fmt.Errorf("%s response status %d: %w", operation, resp.StatusCode, material.ErrManagementFailed)}
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s response status %d: %w", operation, resp.StatusCode, material.ErrManagementFailed)
	}
	if err := json.Unmarshal(responseBytes, out); err != nil {
		return fmt.Errorf("decode %s response: %w", operation, material.ErrManagementFailed)
	}
	return out.wechatError(operation)
}

func (u *MaterialUploader) endpoint(path string, query url.Values, accessToken string) (string, error) {
	parsed, err := url.Parse(u.baseURL + path)
	if err != nil {
		return "", fmt.Errorf("parse endpoint: %w", err)
	}
	values := parsed.Query()
	if query != nil {
		for key, items := range query {
			for _, item := range items {
				values.Add(key, item)
			}
		}
	}
	values.Set("access_token", accessToken)
	parsed.RawQuery = values.Encode()
	return parsed.String(), nil
}

type materialUploadResponse struct {
	URL     string `json:"url"`
	MediaID string `json:"media_id"`
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

type materialJSONResponse interface {
	wechatError(operation string) error
}

type permanentMaterialListRequest struct {
	Type   string `json:"type"`
	Offset int    `json:"offset"`
	Count  int    `json:"count"`
}

type permanentMaterialListResponse struct {
	TotalCount int                       `json:"total_count"`
	ItemCount  int                       `json:"item_count"`
	Items      []material.PermanentImage `json:"item"`
	ErrCode    int                       `json:"errcode"`
	ErrMsg     string                    `json:"errmsg"`
}

func (r permanentMaterialListResponse) wechatError(operation string) error {
	if r.ErrCode == 0 {
		return nil
	}
	return fmt.Errorf("%s errcode %d errmsg %q: %w", operation, r.ErrCode, r.ErrMsg, material.ErrManagementFailed)
}

func (r materialUploadResponse) wechatError(operation string) error {
	if r.ErrCode == 0 {
		return nil
	}
	return fmt.Errorf("%s errcode %d errmsg %q: %w", operation, r.ErrCode, r.ErrMsg, material.ErrUploadFailed)
}
