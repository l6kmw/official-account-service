package wechat

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"official-account-service/internal/domain/publish"
)

func TestPublisherAddsDraftAndSubmitsFreePublish(t *testing.T) {
	var draftPath string
	var submitPath string
	var token string
	var draftBody draftAddRequest
	var submitBody freePublishSubmitRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token = r.URL.Query().Get("access_token")
		switch r.URL.Path {
		case "/draft/add":
			draftPath = r.URL.Path
			require.NoError(t, json.NewDecoder(r.Body).Decode(&draftBody))
			require.NoError(t, json.NewEncoder(w).Encode(map[string]string{"media_id": "draft-media"}))
		case "/freepublish/submit":
			submitPath = r.URL.Path
			require.NoError(t, json.NewDecoder(r.Body).Decode(&submitBody))
			_, err := w.Write([]byte(`{"publish_id":2247483657}`))
			require.NoError(t, err)
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()
	publisher, err := NewPublisher(PublisherConfig{BaseURL: server.URL, MaxRetries: -1})
	require.NoError(t, err)

	draft, err := publisher.AddDraft(context.Background(), "authorizer-token", publish.ArticleDraft{
		Title: "title", Author: "me", Digest: "summary", ContentHTML: "<p>body</p>", ThumbMediaID: "thumb-media",
	})
	require.NoError(t, err)
	submitted, err := publisher.SubmitFreePublish(context.Background(), "authorizer-token", draft.MediaID)
	require.NoError(t, err)

	require.Equal(t, "authorizer-token", token)
	require.Equal(t, "/draft/add", draftPath)
	require.Equal(t, "/freepublish/submit", submitPath)
	require.Len(t, draftBody.Articles, 1)
	require.Equal(t, "thumb-media", draftBody.Articles[0].ThumbMediaID)
	require.Equal(t, "<p>body</p>", draftBody.Articles[0].Content)
	require.Equal(t, "draft-media", submitBody.MediaID)
	require.Equal(t, "2247483657", submitted.PublishID)
}

func TestPublisherGetsFreePublishStatus(t *testing.T) {
	tests := []struct {
		name      string
		response  string
		status    publish.Status
		articleID string
		errorCode string
	}{
		{name: "published", response: `{"publish_status":0,"article_id":"article-1"}`, status: publish.StatusPublished, articleID: "article-1"},
		{name: "publishing", response: `{"publish_status":1}`, status: publish.StatusPublishing},
		{name: "failed", response: `{"publish_status":3}`, status: publish.StatusFailed, errorCode: "publish_status_3"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body freePublishStatusRequest
			var rawBody string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "/freepublish/get", r.URL.Path)
				var err error
				raw, err := io.ReadAll(r.Body)
				require.NoError(t, err)
				rawBody = string(raw)
				require.NoError(t, json.Unmarshal(raw, &body))
				_, err = w.Write([]byte(tt.response))
				require.NoError(t, err)
			}))
			defer server.Close()
			publisher, err := NewPublisher(PublisherConfig{BaseURL: server.URL, MaxRetries: -1})
			require.NoError(t, err)

			result, err := publisher.GetFreePublishStatus(context.Background(), "authorizer-token", "publish-1")
			require.NoError(t, err)

			require.Equal(t, "publish-1", body.PublishID.String())
			require.Equal(t, `{"publish_id":"publish-1"}`, rawBody)
			require.Equal(t, tt.status, result.Status)
			require.Equal(t, tt.articleID, result.WeChatArticleID)
			require.Equal(t, tt.errorCode, result.ErrorCode)
		})
	}
}

func TestPublisherGetsFreePublishStatusWithNumericPublishID(t *testing.T) {
	var rawBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/freepublish/get", r.URL.Path)
		raw, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		rawBody = string(raw)
		_, err = w.Write([]byte(`{"publish_id":2247483657,"publish_status":1}`))
		require.NoError(t, err)
	}))
	defer server.Close()
	publisher, err := NewPublisher(PublisherConfig{BaseURL: server.URL, MaxRetries: -1})
	require.NoError(t, err)

	result, err := publisher.GetFreePublishStatus(context.Background(), "authorizer-token", "2247483657")
	require.NoError(t, err)

	require.Equal(t, `{"publish_id":2247483657}`, rawBody)
	require.Equal(t, publish.StatusPublishing, result.Status)
}

func TestPublisherHandlesWeChatErrCodeWithoutLeakingToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, err := w.Write([]byte(`{"errcode":40001,"errmsg":"invalid credential"}`))
		require.NoError(t, err)
	}))
	defer server.Close()
	publisher, err := NewPublisher(PublisherConfig{BaseURL: server.URL, MaxRetries: -1})
	require.NoError(t, err)

	_, err = publisher.SubmitFreePublish(context.Background(), "authorizer-token", "draft-media")
	require.Error(t, err)

	require.True(t, errors.Is(err, publish.ErrPublishFailed))
	require.NotContains(t, err.Error(), "authorizer-token")
}

func TestPublisherRetriesServerErrors(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		require.NoError(t, json.NewEncoder(w).Encode(map[string]string{"publish_id": "publish-1"}))
	}))
	defer server.Close()
	publisher, err := NewPublisher(PublisherConfig{BaseURL: server.URL, MaxRetries: 1, RetryBackoff: time.Millisecond})
	require.NoError(t, err)

	result, err := publisher.SubmitFreePublish(context.Background(), "authorizer-token", "draft-media")
	require.NoError(t, err)

	require.Equal(t, int32(2), atomic.LoadInt32(&calls))
	require.Equal(t, "publish-1", result.PublishID)
}
