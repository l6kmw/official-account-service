package http

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"official-account-service/internal/application"
	"official-account-service/internal/domain/material"
)

func TestPermanentMaterialRouteReturnsLiveWechatPage(t *testing.T) {
	manager := &routeFakePermanentMaterialManager{batch: material.PermanentImageBatch{
		TotalCount: 3, ItemCount: 1,
		Items: []material.PermanentImage{{MediaID: "media-1", Name: "cover.png", UpdateTime: 1784000000, URL: "https://img/cover.png"}},
	}}
	service := application.NewPermanentMaterialService(manager, routeFakePermanentMaterialTokenProvider{}, "wx-component")
	router := NewRouter(Dependencies{Logger: zap.NewNop(), PermanentMaterials: service})

	recorder := doJSON(t, router, http.MethodGet, "/api/v1/accounts/7/permanent-materials?offset=1&count=2", ``, "tenant-1")

	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"total_count":3,"item_count":1,"next_offset":2,"has_more":true,"items":[{"media_id":"media-1","name":"cover.png","update_time":1784000000,"url":"https://img/cover.png"}]}`, recorder.Body.String())

	bad := doJSON(t, router, http.MethodGet, "/api/v1/accounts/7/permanent-materials?count=bad", ``, "tenant-1")
	require.Equal(t, http.StatusBadRequest, bad.Code)
}

func TestPermanentMaterialRouteDeletesWechatMaterial(t *testing.T) {
	manager := &routeFakePermanentMaterialManager{}
	service := application.NewPermanentMaterialService(manager, routeFakePermanentMaterialTokenProvider{}, "wx-component")
	router := NewRouter(Dependencies{Logger: zap.NewNop(), PermanentMaterials: service})

	recorder := doJSON(t, router, http.MethodDelete, "/api/v1/accounts/7/permanent-materials/media-1", ``, "tenant-1")

	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"deleted":true,"media_id":"media-1"}`, recorder.Body.String())
	require.Equal(t, "media-1", manager.deletedMediaID)
}

type routeFakePermanentMaterialManager struct {
	batch          material.PermanentImageBatch
	deletedMediaID string
}

func (f *routeFakePermanentMaterialManager) ListPermanentImages(context.Context, string, int, int) (material.PermanentImageBatch, error) {
	return f.batch, nil
}

func (f *routeFakePermanentMaterialManager) DeletePermanentMaterial(_ context.Context, _ string, mediaID string) error {
	f.deletedMediaID = mediaID
	return nil
}

type routeFakePermanentMaterialTokenProvider struct{}

func (routeFakePermanentMaterialTokenProvider) GetAuthorizerAccessToken(context.Context, application.RefreshAuthorizerAccessTokenInput) (application.AuthorizerAccessToken, error) {
	return application.AuthorizerAccessToken{AccessToken: "token"}, nil
}
