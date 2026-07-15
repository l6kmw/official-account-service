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

	"official-account-service/internal/domain/material"
)

func TestMaterialUploaderUploadsInlineImage(t *testing.T) {
	var path string
	var token string
	var filename string
	var content string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		token = r.URL.Query().Get("access_token")
		file, header, err := r.FormFile("media")
		require.NoError(t, err)
		defer file.Close()
		filename = header.Filename
		body, err := io.ReadAll(file)
		require.NoError(t, err)
		content = string(body)
		_, err = w.Write([]byte(`{"url":"https://mmbiz.qpic.cn/body.png"}`))
		require.NoError(t, err)
	}))
	defer server.Close()
	uploader, err := NewMaterialUploader(MaterialUploaderConfig{BaseURL: server.URL, MaxRetries: -1})
	require.NoError(t, err)

	result, err := uploader.UploadInlineImage(context.Background(), "authorizer-token", "body.png", strings.NewReader("image"))
	require.NoError(t, err)

	require.Equal(t, "/media/uploadimg", path)
	require.Equal(t, "authorizer-token", token)
	require.Equal(t, "body.png", filename)
	require.Equal(t, "image", content)
	require.Equal(t, "https://mmbiz.qpic.cn/body.png", result.WeChatURL)
}

func TestMaterialUploaderUploadsCover(t *testing.T) {
	var path string
	var mediaType string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		mediaType = r.URL.Query().Get("type")
		_, _, err := r.FormFile("media")
		require.NoError(t, err)
		require.NoError(t, json.NewEncoder(w).Encode(map[string]string{"media_id": "media-cover"}))
	}))
	defer server.Close()
	uploader, err := NewMaterialUploader(MaterialUploaderConfig{BaseURL: server.URL, MaxRetries: -1})
	require.NoError(t, err)

	result, err := uploader.UploadCover(context.Background(), "authorizer-token", "cover.png", strings.NewReader("image"))
	require.NoError(t, err)

	require.Equal(t, "/material/add_material", path)
	require.Equal(t, "image", mediaType)
	require.Equal(t, "media-cover", result.MediaID)
}

func TestMaterialUploaderListsPermanentImages(t *testing.T) {
	var body permanentMaterialListRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/material/batchget_material", r.URL.Path)
		require.Equal(t, "authorizer-token", r.URL.Query().Get("access_token"))
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		_, err := w.Write([]byte(`{"total_count":3,"item_count":1,"item":[{"media_id":"media-1","name":"cover.png","update_time":1784000000,"url":"https://img/cover.png"}]}`))
		require.NoError(t, err)
	}))
	defer server.Close()
	uploader, err := NewMaterialUploader(MaterialUploaderConfig{BaseURL: server.URL, MaxRetries: -1})
	require.NoError(t, err)

	result, err := uploader.ListPermanentImages(context.Background(), "authorizer-token", 1, 2)

	require.NoError(t, err)
	require.Equal(t, permanentMaterialListRequest{Type: "image", Offset: 1, Count: 2}, body)
	require.Equal(t, 3, result.TotalCount)
	require.Equal(t, 1, result.ItemCount)
	require.Equal(t, "media-1", result.Items[0].MediaID)
}

func TestMaterialUploaderDeletesPermanentMaterial(t *testing.T) {
	var body permanentMaterialDeleteRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/material/del_material", r.URL.Path)
		require.Equal(t, "authorizer-token", r.URL.Query().Get("access_token"))
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		_, err := w.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
		require.NoError(t, err)
	}))
	defer server.Close()
	uploader, err := NewMaterialUploader(MaterialUploaderConfig{BaseURL: server.URL, MaxRetries: -1})
	require.NoError(t, err)

	err = uploader.DeletePermanentMaterial(context.Background(), "authorizer-token", "media-1")

	require.NoError(t, err)
	require.Equal(t, permanentMaterialDeleteRequest{MediaID: "media-1"}, body)
}

func TestMaterialUploaderHandlesWeChatErrCodeWithoutLeakingToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, err := w.Write([]byte(`{"errcode":40001,"errmsg":"invalid credential"}`))
		require.NoError(t, err)
	}))
	defer server.Close()
	uploader, err := NewMaterialUploader(MaterialUploaderConfig{BaseURL: server.URL, MaxRetries: -1})
	require.NoError(t, err)

	_, err = uploader.UploadInlineImage(context.Background(), "authorizer-token", "body.png", strings.NewReader("image"))
	require.Error(t, err)

	require.True(t, errors.Is(err, material.ErrUploadFailed))
	require.NotContains(t, err.Error(), "authorizer-token")
}

func TestMaterialUploaderRetriesServerErrors(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		_, err := w.Write([]byte(`{"url":"https://mmbiz.qpic.cn/body.png"}`))
		require.NoError(t, err)
	}))
	defer server.Close()
	uploader, err := NewMaterialUploader(MaterialUploaderConfig{BaseURL: server.URL, MaxRetries: 1, RetryBackoff: time.Millisecond})
	require.NoError(t, err)

	result, err := uploader.UploadInlineImage(context.Background(), "authorizer-token", "body.png", strings.NewReader("image"))
	require.NoError(t, err)

	require.Equal(t, int32(2), atomic.LoadInt32(&calls))
	require.Equal(t, "https://mmbiz.qpic.cn/body.png", result.WeChatURL)
}

func TestMaterialUploaderValidatesInput(t *testing.T) {
	uploader, err := NewMaterialUploader(MaterialUploaderConfig{})
	require.NoError(t, err)

	_, err = uploader.UploadCover(context.Background(), "", "cover.png", strings.NewReader("image"))
	require.Error(t, err)
	require.True(t, errors.Is(err, material.ErrUploaderUnavailable))
}
