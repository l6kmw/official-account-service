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
	token, err := s.tokens.GetAuthorizerAccessToken(ctx, RefreshAuthorizerAccessTokenInput{
		TenantID: input.TenantID, AccountID: input.AuthorizerID, ComponentAppID: s.componentAppID,
	})
	if err != nil {
		return PermanentMaterialPage{}, fmt.Errorf("get authorizer access token for permanent materials: %w", err)
	}
	batch, err := s.manager.ListPermanentImages(ctx, token.AccessToken, input.Offset, input.Count)
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

func (s *PermanentMaterialService) validateListInput(input ListPermanentMaterialsInput) error {
	if s == nil || s.manager == nil || s.tokens == nil || strings.TrimSpace(s.componentAppID) == "" {
		return fmt.Errorf("validate permanent material service: %w", ErrNotImplemented)
	}
	if strings.TrimSpace(input.TenantID) == "" || input.AuthorizerID <= 0 || input.Offset < 0 || input.Count < 1 || input.Count > 20 {
		return fmt.Errorf("validate permanent material list input: %w", ErrInvalidInput)
	}
	return nil
}
