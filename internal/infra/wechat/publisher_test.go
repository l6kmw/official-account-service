package wechat

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestPublisherDeletesFreePublishArticle(t *testing.T) {
	var token string
	var body freePublishDeleteRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/freepublish/delete", r.URL.Path)
		token = r.URL.Query().Get("access_token")
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		_, err := w.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
		require.NoError(t, err)
	}))
	defer server.Close()
	publisher, err := NewPublisher(PublisherConfig{BaseURL: server.URL, MaxRetries: -1})
	require.NoError(t, err)

	err = publisher.DeleteFreePublish(context.Background(), "authorizer-token", "article-1", 0)
	require.NoError(t, err)

	require.Equal(t, "authorizer-token", token)
	require.Equal(t, "article-1", body.ArticleID)
	require.Equal(t, 0, body.Index)
}

func TestPublisherListsLivePublishedArticles(t *testing.T) {
	var body freePublishBatchGetRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/freepublish/batchget", r.URL.Path)
		require.Equal(t, "authorizer-token", r.URL.Query().Get("access_token"))
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		_, err := w.Write([]byte(`{
			"total_count":3,"item_count":1,"item":[{
				"article_id":"article-1","update_time":1784000000,
				"content":{"news_item":[
					{"title":"first","author":"me","digest":"one","content":"<p>one</p>","thumb_media_id":"thumb-1","thumb_url":"https://img/1","url":"https://mp.weixin.qq.com/s?mid=2247483683&idx=1","need_open_comment":1},
					{"title":"second","is_deleted":true,"only_fans_can_comment":1}
				]}
			}]}`))
		require.NoError(t, err)
	}))
	defer server.Close()
	publisher, err := NewPublisher(PublisherConfig{BaseURL: server.URL, MaxRetries: -1})
	require.NoError(t, err)

	result, err := publisher.ListPublishedArticles(context.Background(), "authorizer-token", 1, 2, true)
	require.NoError(t, err)

	require.Equal(t, freePublishBatchGetRequest{Offset: 1, Count: 2, NoContent: 0}, body)
	require.Equal(t, 3, result.TotalMessageCount)
	require.Equal(t, 1, result.FetchedMessageCount)
	require.Len(t, result.Items, 2)
	require.Equal(t, "article-1", result.Items[1].ArticleID)
	require.Equal(t, "2247483683_1", result.Items[0].MsgID)
	require.Empty(t, result.Items[1].MsgID)
	require.Equal(t, 1, result.Items[1].Index)
	require.True(t, result.Items[0].NeedOpenComment)
	require.True(t, result.Items[1].OnlyFansCanComment)
	require.True(t, result.Items[1].Deleted)
}

func TestPublisherAllowsLargePublishedArticlePages(t *testing.T) {
	newsItems := make([]map[string]any, 0, 8)
	for index := 0; index < 8; index++ {
		newsItems = append(newsItems, map[string]any{
			"title": "article", "content": strings.Repeat("x", 8<<10),
		})
	}
	messages := make([]map[string]any, 0, 20)
	for index := 0; index < 20; index++ {
		messages = append(messages, map[string]any{
			"article_id": "message", "content": map[string]any{"news_item": newsItems},
		})
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"total_count": 20, "item_count": 20, "item": messages,
		}))
	}))
	defer server.Close()
	publisher, err := NewPublisher(PublisherConfig{BaseURL: server.URL, MaxRetries: -1})
	require.NoError(t, err)

	result, err := publisher.ListPublishedArticles(context.Background(), "authorizer-token", 0, 20, true)

	require.NoError(t, err)
	require.Len(t, result.Items, 160)
	require.Len(t, result.Items[0].ContentHTML, 8<<10)
}

func TestPublisherGetsArticleMetrics(t *testing.T) {
	var body articleMetricsRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/datacube/getarticletotaldetail", r.URL.Path)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		_, err := w.Write([]byte(`{
			"is_delay":false,
			"list":[{"ref_date":"2026-07-10","msgid":"2247490098_1","title":"article","content_url":"https://mp/article","detail_list":[{
				"stat_date":"2026-07-11","read_user":123,"share_user":12,"zaikan_user":8,"like_user":9,"comment_count":3,"collection_user":4,"praise_money":500,"read_subscribe_user":2,
				"read_delivery_rate":0.4,"read_finish_rate":0.6,"read_avg_activetime":1.2,
				"read_user_source":[{"user_count":100,"scene_desc":"公众号消息"}],"read_jump_position":[{"position":1,"rate":0.3}]
			}]}]}`))
		require.NoError(t, err)
	}))
	defer server.Close()
	publisher, err := NewPublisher(PublisherConfig{BaseURL: server.URL + "/cgi-bin", MaxRetries: -1})
	require.NoError(t, err)

	result, err := publisher.GetArticleMetrics(context.Background(), "authorizer-token", "2026-07-10")
	require.NoError(t, err)

	require.Equal(t, articleMetricsRequest{BeginDate: "2026-07-10", EndDate: "2026-07-10"}, body)
	require.False(t, result.Delayed)
	require.Equal(t, "2247490098_1", result.Items[0].MsgID)
	require.Equal(t, int64(123), result.Items[0].Details[0].ReadUser)
	require.Equal(t, int64(9), result.Items[0].Details[0].LikeUser)
	require.Equal(t, "公众号消息", result.Items[0].Details[0].ReadUserSources[0].SceneDesc)
}

func TestPublisherListsCommentsWithoutExposingOpenID(t *testing.T) {
	var body articleCommentsRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/cgi-bin/comment/list", r.URL.Path)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		_, err := w.Write([]byte(`{
			"errcode":0,"errmsg":"ok","total":2,"comment":[{
				"user_comment_id":99,"openid":"sensitive-openid","create_time":1784000000,
				"content":"useful","comment_type":1,"reply":{"content":"thanks","create_time":1784000100}
			}]
		}`))
		require.NoError(t, err)
	}))
	defer server.Close()
	publisher, err := NewPublisher(PublisherConfig{BaseURL: server.URL + "/cgi-bin", MaxRetries: -1})
	require.NoError(t, err)

	result, err := publisher.ListArticleComments(context.Background(), "authorizer-token", 2247490098, 1, 0, 20, 2)
	require.NoError(t, err)

	require.Equal(t, articleCommentsRequest{MsgDataID: 2247490098, Index: 1, Begin: 0, Count: 20, Type: 2}, body)
	require.Equal(t, 2, result.Total)
	require.True(t, result.Items[0].Selected)
	require.Equal(t, "thanks", result.Items[0].Reply.Content)
	raw, err := json.Marshal(result)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "sensitive-openid")
	require.NotContains(t, string(raw), "openid")
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
