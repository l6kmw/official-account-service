package application

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"official-account-service/internal/domain/authorization"
	"official-account-service/internal/domain/publish"
	"official-account-service/internal/domain/wechatcallback"
	"official-account-service/internal/infra/persistence/memory"
)

func TestCallbackServiceHandlesPublishResultCallback(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 6, 16, 0, 0, 0, time.UTC)
	store := memory.NewStore(func() time.Time { return now })
	publishService, record := preparePublishingRecord(t, ctx, store)
	_, err := store.SaveAuthorizerTenantBinding(ctx, authorization.AuthorizerTenantBinding{
		ComponentAppID: "wx-component", AuthorizerAppID: "wx-authorizer", TenantID: "tenant-1",
	})
	require.NoError(t, err)
	raw := []byte(`<xml><Event><![CDATA[PUBLISHJOBFINISH]]></Event><PublishEventInfo><publish_id><![CDATA[publish-1]]></publish_id><publish_status>0</publish_status><article_id><![CDATA[article-1]]></article_id></PublishEventInfo></xml>`)
	service := NewCallbackService(store, store, publishService, &fakeAuthorizerCallbackDecryptor{plaintext: raw}, "wx-component", func() time.Time { return now })

	err = service.HandleAuthorizerCallback(ctx, encryptedAuthorizerCallbackInput())
	require.NoError(t, err)

	updated, err := store.GetPublishRecord(ctx, "tenant-1", record.ID)
	require.NoError(t, err)
	require.Equal(t, publish.StatusPublished, updated.Status)
	require.Equal(t, "article-1", updated.WeChatArticleID)
	event, err := store.GetCallbackEventByKey(ctx, "tenant-1", wechatcallback.EventTypePublishResult, "publish-1")
	require.NoError(t, err)
	require.Equal(t, string(raw), event.RawBody)
	require.Equal(t, now.Add(defaultCallbackRetention), event.RetainUntil)
}

func TestCallbackServiceDeduplicatesPublishResultCallback(t *testing.T) {
	ctx := context.Background()
	store := memory.NewStore(time.Now)
	publishService, record := preparePublishingRecord(t, ctx, store)
	_, err := store.SaveAuthorizerTenantBinding(ctx, authorization.AuthorizerTenantBinding{
		ComponentAppID: "wx-component", AuthorizerAppID: "wx-authorizer", TenantID: "tenant-1",
	})
	require.NoError(t, err)
	first := []byte(`<xml><Event><![CDATA[PUBLISHJOBFINISH]]></Event><PublishEventInfo><publish_id>publish-1</publish_id><publish_status>0</publish_status><article_id>article-1</article_id></PublishEventInfo></xml>`)
	second := []byte(`<xml><Event><![CDATA[PUBLISHJOBFINISH]]></Event><PublishEventInfo><publish_id>publish-1</publish_id><publish_status>3</publish_status></PublishEventInfo></xml>`)
	decryptor := &fakeAuthorizerCallbackDecryptor{plaintext: first}
	service := NewCallbackService(store, store, publishService, decryptor, "wx-component", time.Now)
	require.NoError(t, service.HandleAuthorizerCallback(ctx, encryptedAuthorizerCallbackInput()))

	decryptor.plaintext = second
	err = service.HandleAuthorizerCallback(ctx, encryptedAuthorizerCallbackInput())
	require.NoError(t, err)

	updated, err := store.GetPublishRecord(ctx, "tenant-1", record.ID)
	require.NoError(t, err)
	require.Equal(t, publish.StatusPublished, updated.Status)
	require.Empty(t, updated.ErrorCode)
}

func TestCallbackServiceHandlesEncryptedPublishCallback(t *testing.T) {
	ctx := context.Background()
	store := memory.NewStore(time.Now)
	publishService, record := preparePublishingRecord(t, ctx, store)
	_, err := store.SaveAuthorizerTenantBinding(ctx, authorization.AuthorizerTenantBinding{
		ComponentAppID: "wx-component", AuthorizerAppID: "wx-authorizer", TenantID: "tenant-1",
	})
	require.NoError(t, err)
	decryptor := &fakeAuthorizerCallbackDecryptor{plaintext: []byte(`<xml><Event>PUBLISHJOBFINISH</Event><PublishEventInfo><publish_id>publish-1</publish_id><publish_status>3</publish_status></PublishEventInfo></xml>`)}
	service := NewCallbackService(store, store, publishService, decryptor, "wx-component", time.Now)

	err = service.HandleAuthorizerCallback(ctx, HandleAuthorizerCallbackInput{
		AuthorizerAppID: "wx-authorizer",
		RawBody:         []byte(`<xml><Encrypt>ciphertext</Encrypt></xml>`),
		EncryptType:     "aes",
		MsgSignature:    "signature",
		Timestamp:       "1783334400",
		Nonce:           "nonce",
	})
	require.NoError(t, err)

	require.Equal(t, "ciphertext", decryptor.lastInput.Ciphertext)
	updated, err := store.GetPublishRecord(ctx, "tenant-1", record.ID)
	require.NoError(t, err)
	require.Equal(t, publish.StatusFailed, updated.Status)
	require.Equal(t, "publish_status_3", updated.ErrorCode)
	event, err := store.GetCallbackEventByKey(ctx, "tenant-1", wechatcallback.EventTypePublishResult, "publish-1")
	require.NoError(t, err)
	require.Equal(t, string(decryptor.plaintext), event.RawBody)
	require.NotContains(t, event.RawBody, "ciphertext")
}

func TestCallbackServiceRejectsPlainAuthorizerCallback(t *testing.T) {
	store := memory.NewStore(time.Now)
	service := NewCallbackService(store, store, NewPublishService(store, store, time.Now), nil, "wx-component", time.Now)

	err := service.HandleAuthorizerCallback(context.Background(), HandleAuthorizerCallbackInput{
		AuthorizerAppID: "wx-authorizer",
		RawBody:         []byte(`<xml><Event>subscribe</Event></xml>`),
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))
}

func TestCallbackServiceHandlesConcurrentDuplicatePublishCallbacks(t *testing.T) {
	ctx := context.Background()
	store := memory.NewStore(time.Now)
	publishService, record := preparePublishingRecord(t, ctx, store)
	_, err := store.SaveAuthorizerTenantBinding(ctx, authorization.AuthorizerTenantBinding{
		ComponentAppID: "wx-component", AuthorizerAppID: "wx-authorizer", TenantID: "tenant-1",
	})
	require.NoError(t, err)
	raw := []byte(`<xml><Event>PUBLISHJOBFINISH</Event><PublishEventInfo><publish_id>publish-1</publish_id><publish_status>0</publish_status><article_id>article-1</article_id></PublishEventInfo></xml>`)
	service := NewCallbackService(store, store, publishService, &fakeAuthorizerCallbackDecryptor{plaintext: raw}, "wx-component", time.Now)

	errs := make(chan error, 20)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- service.HandleAuthorizerCallback(ctx, encryptedAuthorizerCallbackInput())
		}()
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}
	updated, err := store.GetPublishRecord(ctx, "tenant-1", record.ID)
	require.NoError(t, err)
	require.Equal(t, publish.StatusPublished, updated.Status)
	event, err := store.GetCallbackEventByKey(ctx, "tenant-1", wechatcallback.EventTypePublishResult, "publish-1")
	require.NoError(t, err)
	require.Equal(t, string(raw), event.RawBody)
}

func TestCallbackServiceIgnoresUnsupportedAuthorizerCallback(t *testing.T) {
	ctx := context.Background()
	store := memory.NewStore(time.Now)
	decryptor := &fakeAuthorizerCallbackDecryptor{plaintext: []byte(`<xml><Event>subscribe</Event></xml>`)}
	service := NewCallbackService(store, store, NewPublishService(store, store, time.Now), decryptor, "wx-component", time.Now)

	err := service.HandleAuthorizerCallback(ctx, encryptedAuthorizerCallbackInput())
	require.NoError(t, err)
}

func TestCallbackServiceValidatesPublishCallback(t *testing.T) {
	ctx := context.Background()
	store := memory.NewStore(time.Now)
	decryptor := &fakeAuthorizerCallbackDecryptor{plaintext: []byte(`<xml><Event>PUBLISHJOBFINISH</Event><PublishEventInfo><publish_status>0</publish_status></PublishEventInfo></xml>`)}
	service := NewCallbackService(store, store, NewPublishService(store, store, time.Now), decryptor, "wx-component", time.Now)

	err := service.HandleAuthorizerCallback(ctx, encryptedAuthorizerCallbackInput())
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))
}

func preparePublishingRecord(t *testing.T, ctx context.Context, store *memory.Store) (*PublishService, publish.Record) {
	t.Helper()
	articles := NewArticleService(store)
	draft, err := articles.CreateArticle(ctx, CreateArticleInput{TenantID: "tenant-1", AuthorizerID: 1, Title: "hello"})
	require.NoError(t, err)
	service := NewPublishService(store, store, time.Now)
	record, err := service.CreatePublishRecord(ctx, CreatePublishRecordInput{TenantID: "tenant-1", ArticleID: draft.ID, WeChatPublishID: "publish-1"})
	require.NoError(t, err)
	return service, record
}

type fakeAuthorizerCallbackDecryptor struct {
	plaintext []byte
	lastInput authorization.ComponentCallbackDecryptInput
}

func encryptedAuthorizerCallbackInput() HandleAuthorizerCallbackInput {
	return HandleAuthorizerCallbackInput{
		AuthorizerAppID: "wx-authorizer",
		RawBody:         []byte(`<xml><Encrypt>ciphertext</Encrypt></xml>`),
		EncryptType:     "aes",
		MsgSignature:    "signature",
		Timestamp:       "1783334400",
		Nonce:           "nonce",
	}
}

func (d *fakeAuthorizerCallbackDecryptor) DecryptComponentCallback(_ context.Context, input authorization.ComponentCallbackDecryptInput) ([]byte, error) {
	d.lastInput = input
	return d.plaintext, nil
}
