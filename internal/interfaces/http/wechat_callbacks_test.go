package http

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"official-account-service/internal/application"
	"official-account-service/internal/domain/authorization"
	"official-account-service/internal/domain/publish"
	"official-account-service/internal/infra/persistence/memory"
)

func TestAuthorizerPublishCallbackRoute(t *testing.T) {
	store := memory.NewStore(func() time.Time { return time.Date(2026, 7, 6, 8, 0, 0, 0, time.UTC) })
	articles := application.NewArticleService(store)
	draft, err := articles.CreateArticle(t.Context(), application.CreateArticleInput{TenantID: "tenant-1", AuthorizerID: 1, Title: "hello"})
	require.NoError(t, err)
	publishes := application.NewPublishService(store, store, time.Now)
	_, err = publishes.CreatePublishRecord(t.Context(), application.CreatePublishRecordInput{
		TenantID: "tenant-1", ArticleID: draft.ID, WeChatPublishID: "publish-1",
	})
	require.NoError(t, err)
	_, err = store.SaveAuthorizerTenantBinding(t.Context(), authorization.AuthorizerTenantBinding{
		ComponentAppID: "wx-component", AuthorizerAppID: "wx-authorizer", TenantID: "tenant-1",
	})
	require.NoError(t, err)
	decryptor := &routeFakeComponentCallbackDecryptor{plaintext: []byte(`<xml><Event>PUBLISHJOBFINISH</Event><PublishEventInfo><publish_id>publish-1</publish_id><publish_status>0</publish_status><article_id>article-1</article_id></PublishEventInfo></xml>`)}
	callbacks := application.NewCallbackService(store, store, publishes, decryptor, "wx-component", time.Now)
	router := NewRouter(Dependencies{Logger: zap.NewNop(), Callbacks: callbacks})

	recorder := doXML(t, router, http.MethodPost, "/wechat/authorizer/wx-authorizer/callback?encrypt_type=aes&msg_signature=signature&timestamp=1783334400&nonce=nonce", `<xml><Encrypt>ciphertext</Encrypt></xml>`)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "success", recorder.Body.String())
	require.NotContains(t, recorder.Body.String(), "publish-1")

	record, err := store.GetPublishRecordByPublishID(t.Context(), "tenant-1", "publish-1")
	require.NoError(t, err)
	require.Equal(t, publish.StatusPublished, record.Status)
	require.Equal(t, "article-1", record.WeChatArticleID)
}

func TestAuthorizerPublishCallbackRouteUnavailable(t *testing.T) {
	recorder := doXML(t, testRouter(), http.MethodPost, "/wechat/authorizer/wx-authorizer/callback", `<xml></xml>`)
	require.Equal(t, http.StatusNotImplemented, recorder.Code)
}
