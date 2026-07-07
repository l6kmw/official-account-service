package wechat

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"official-account-service/internal/domain/authorization"
)

const maxResponseBytes = 1 << 20

func (c *ComponentClient) postJSON(ctx context.Context, operation string, path string, query url.Values, body any, out any) error {
	if !c.breaker.allow() {
		return fmt.Errorf("%s circuit breaker open: %w", operation, authorization.ErrPreAuthCodeUnavailable)
	}
	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			if err := sleepContext(ctx, c.retryBackoff); err != nil {
				return fmt.Errorf("wait before retry %s: %w", operation, err)
			}
		}
		err := c.postJSONOnce(ctx, operation, path, query, body, out)
		if err == nil {
			c.breaker.recordSuccess()
			return nil
		}
		lastErr = err
		if !isRetryable(err) {
			break
		}
	}
	c.breaker.recordFailure()
	return lastErr
}

func (c *ComponentClient) postJSONOnce(ctx context.Context, operation string, path string, query url.Values, body any, out any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal %s request: %w", operation, err)
	}
	endpoint, err := c.endpoint(path, query)
	if err != nil {
		return fmt.Errorf("build %s endpoint: %w", operation, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create %s request: %w", operation, err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return retryableError{err: fmt.Errorf("send %s request: %w", operation, err)}
	}
	defer resp.Body.Close()
	responseBytes, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return fmt.Errorf("read %s response: %w", operation, err)
	}
	if resp.StatusCode >= http.StatusInternalServerError {
		return retryableError{err: fmt.Errorf("%s response status %d: %w", operation, resp.StatusCode, authorization.ErrPreAuthCodeUnavailable)}
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s response status %d: %w", operation, resp.StatusCode, authorization.ErrPreAuthCodeUnavailable)
	}
	if err := json.Unmarshal(responseBytes, out); err != nil {
		return fmt.Errorf("decode %s response: %w", operation, err)
	}
	return nil
}

func (c *ComponentClient) endpoint(path string, query url.Values) (string, error) {
	parsed, err := url.Parse(c.baseURL + path)
	if err != nil {
		return "", fmt.Errorf("parse endpoint: %w", err)
	}
	if query != nil {
		parsed.RawQuery = query.Encode()
	}
	return parsed.String(), nil
}

type retryableError struct {
	err error
}

func (e retryableError) Error() string {
	return e.err.Error()
}

func (e retryableError) Unwrap() error {
	return e.err
}

func isRetryable(err error) bool {
	var retryable retryableError
	return errors.As(err, &retryable)
}

func sleepContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return fmt.Errorf("context done: %w", ctx.Err())
	case <-timer.C:
		return nil
	}
}

type circuitBreaker struct {
	mu          sync.Mutex
	failures    int
	threshold   int
	openedAt    time.Time
	openTimeout time.Duration
	now         func() time.Time
}

func newCircuitBreaker(threshold int, openTimeout time.Duration, now func() time.Time) *circuitBreaker {
	if now == nil {
		now = time.Now
	}
	return &circuitBreaker{threshold: threshold, openTimeout: openTimeout, now: now}
}

func (b *circuitBreaker) allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.failures < b.threshold {
		return true
	}
	if b.now().Sub(b.openedAt) >= b.openTimeout {
		b.failures = 0
		b.openedAt = time.Time{}
		return true
	}
	return false
}

func (b *circuitBreaker) recordSuccess() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures = 0
	b.openedAt = time.Time{}
}

func (b *circuitBreaker) recordFailure() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures++
	if b.failures >= b.threshold {
		b.openedAt = b.now()
	}
}
