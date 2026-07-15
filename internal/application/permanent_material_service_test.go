package application

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"official-account-service/internal/domain/material"
)

func TestPermanentMaterialServiceListsLiveImages(t *testing.T) {
	manager := &fakePermanentMaterialManager{batch: material.PermanentImageBatch{
		TotalCount: 5, ItemCount: 2,
		Items: []material.PermanentImage{{MediaID: "media-1", Name: "cover.png"}},
	}}
	tokens := &fakeMaterialTokenProvider{token: AuthorizerAccessToken{AccessToken: "authorizer-token"}}
	service := NewPermanentMaterialService(manager, tokens, "wx-component")

	result, err := service.ListPermanentMaterials(context.Background(), ListPermanentMaterialsInput{
		TenantID: "tenant-1", AuthorizerID: 7, Offset: 1, Count: 2,
	})

	require.NoError(t, err)
	require.Equal(t, "tenant-1", tokens.lastInput.TenantID)
	require.Equal(t, int64(7), tokens.lastInput.AccountID)
	require.Equal(t, "wx-component", tokens.lastInput.ComponentAppID)
	require.Equal(t, "authorizer-token", manager.accessToken)
	require.Equal(t, 1, manager.offset)
	require.Equal(t, 2, manager.count)
	require.Equal(t, 3, result.NextOffset)
	require.True(t, result.HasMore)
	require.Equal(t, "media-1", result.Items[0].MediaID)
}

func TestPermanentMaterialServiceValidatesPagination(t *testing.T) {
	service := NewPermanentMaterialService(&fakePermanentMaterialManager{}, &fakeMaterialTokenProvider{}, "wx-component")

	_, err := service.ListPermanentMaterials(context.Background(), ListPermanentMaterialsInput{
		TenantID: "tenant-1", AuthorizerID: 1, Count: 21,
	})

	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))
}

func TestPermanentMaterialServiceDeletesWithAuthorizerToken(t *testing.T) {
	manager := &fakePermanentMaterialManager{}
	tokens := &fakeMaterialTokenProvider{token: AuthorizerAccessToken{AccessToken: "authorizer-token"}}
	service := NewPermanentMaterialService(manager, tokens, "wx-component")

	err := service.DeletePermanentMaterial(context.Background(), DeletePermanentMaterialInput{
		TenantID: "tenant-1", AuthorizerID: 7, MediaID: "media-1",
	})

	require.NoError(t, err)
	require.Equal(t, "authorizer-token", manager.deleteAccessToken)
	require.Equal(t, "media-1", manager.deletedMediaID)
	require.Equal(t, int64(7), tokens.lastInput.AccountID)
}

type fakePermanentMaterialManager struct {
	batch             material.PermanentImageBatch
	accessToken       string
	offset            int
	count             int
	deleteAccessToken string
	deletedMediaID    string
}

func (f *fakePermanentMaterialManager) DeletePermanentMaterial(_ context.Context, accessToken string, mediaID string) error {
	f.deleteAccessToken = accessToken
	f.deletedMediaID = mediaID
	return nil
}

func (f *fakePermanentMaterialManager) ListPermanentImages(_ context.Context, accessToken string, offset int, count int) (material.PermanentImageBatch, error) {
	f.accessToken = accessToken
	f.offset = offset
	f.count = count
	return f.batch, nil
}
