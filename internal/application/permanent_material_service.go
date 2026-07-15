package application

import (
	"context"
	"fmt"
	"strings"

	"official-account-service/internal/domain/material"
)

// PermanentMaterialService manages the live permanent material library in WeChat.
type PermanentMaterialService struct {
	manager        material.PermanentManager
	tokens         MaterialTokenProvider
	componentAppID string
}

// NewPermanentMaterialService constructs a permanent material service.
func NewPermanentMaterialService(manager material.PermanentManager, tokens MaterialTokenProvider, componentAppID string) *PermanentMaterialService {
	return &PermanentMaterialService{manager: manager, tokens: tokens, componentAppID: componentAppID}
}

// ListPermanentMaterialsInput identifies one authorized account and page.
type ListPermanentMaterialsInput struct {
	TenantID     string
	AuthorizerID int64
	Offset       int
	Count        int
}

// PermanentMaterialPage is a live WeChat permanent image page.
type PermanentMaterialPage struct {
	TotalCount int                       `json:"total_count"`
	ItemCount  int                       `json:"item_count"`
	NextOffset int                       `json:"next_offset"`
	HasMore    bool                      `json:"has_more"`
	Items      []material.PermanentImage `json:"items"`
}

// ListPermanentMaterials reads permanent images directly from WeChat.
func (s *PermanentMaterialService) ListPermanentMaterials(ctx context.Context, input ListPermanentMaterialsInput) (PermanentMaterialPage, error) {
	if err := s.validateListInput(input); err != nil {
		return PermanentMaterialPage{}, err
	}
	accessToken, err := s.accessToken(ctx, input.TenantID, input.AuthorizerID)
	if err != nil {
		return PermanentMaterialPage{}, err
	}
	batch, err := s.manager.ListPermanentImages(ctx, accessToken, input.Offset, input.Count)
	if err != nil {
		return PermanentMaterialPage{}, fmt.Errorf("list permanent materials: %w", err)
	}
	nextOffset := input.Offset + batch.ItemCount
	return PermanentMaterialPage{
		TotalCount: batch.TotalCount,
		ItemCount:  batch.ItemCount,
		NextOffset: nextOffset,
		HasMore:    nextOffset < batch.TotalCount,
		Items:      batch.Items,
	}, nil
}

// DeletePermanentMaterialInput identifies one permanent WeChat material.
type DeletePermanentMaterialInput struct {
	TenantID     string
	AuthorizerID int64
	MediaID      string
}

// DeletePermanentMaterial permanently removes one material from WeChat.
func (s *PermanentMaterialService) DeletePermanentMaterial(ctx context.Context, input DeletePermanentMaterialInput) error {
	if err := s.validateReady(); err != nil {
		return err
	}
	if strings.TrimSpace(input.TenantID) == "" || input.AuthorizerID <= 0 || strings.TrimSpace(input.MediaID) == "" {
		return fmt.Errorf("validate permanent material delete input: %w", ErrInvalidInput)
	}
	accessToken, err := s.accessToken(ctx, input.TenantID, input.AuthorizerID)
	if err != nil {
		return err
	}
	if err := s.manager.DeletePermanentMaterial(ctx, accessToken, input.MediaID); err != nil {
		return fmt.Errorf("delete permanent material: %w", err)
	}
	return nil
}

func (s *PermanentMaterialService) validateListInput(input ListPermanentMaterialsInput) error {
	if err := s.validateReady(); err != nil {
		return err
	}
	if strings.TrimSpace(input.TenantID) == "" || input.AuthorizerID <= 0 || input.Offset < 0 || input.Count < 1 || input.Count > 20 {
		return fmt.Errorf("validate permanent material list input: %w", ErrInvalidInput)
	}
	return nil
}

func (s *PermanentMaterialService) validateReady() error {
	if s == nil || s.manager == nil || s.tokens == nil || strings.TrimSpace(s.componentAppID) == "" {
		return fmt.Errorf("validate permanent material service: %w", ErrNotImplemented)
	}
	return nil
}

func (s *PermanentMaterialService) accessToken(ctx context.Context, tenantID string, authorizerID int64) (string, error) {
	token, err := s.tokens.GetAuthorizerAccessToken(ctx, RefreshAuthorizerAccessTokenInput{
		TenantID: tenantID, AccountID: authorizerID, ComponentAppID: s.componentAppID,
	})
	if err != nil {
		return "", fmt.Errorf("get authorizer access token for permanent materials: %w", err)
	}
	return token.AccessToken, nil
}
