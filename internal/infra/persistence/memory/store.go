package memory

import (
	"context"
	"fmt"
	"sync"
	"time"

	"official-account-service/internal/domain/article"
	"official-account-service/internal/domain/authorization"
	"official-account-service/internal/domain/material"
	"official-account-service/internal/domain/publish"
	"official-account-service/internal/domain/wechatcallback"
)

// Store is an in-memory repository implementation for local development and tests.
type Store struct {
	mu             sync.RWMutex
	nextAccountID  int64
	nextArticleID  int64
	nextAssetID    int64
	nextRecordID   int64
	accounts       map[int64]authorization.Account
	articles       map[int64]article.Article
	materialAssets map[int64]material.Asset
	publishRecords map[int64]publish.Record
	callbackEvents map[int64]wechatcallback.Event
	tickets        map[string]authorization.ComponentVerifyTicket
	bindings       map[string]authorization.AuthorizerTenantBinding
	now            func() time.Time
}

// NewStore constructs an in-memory Store.
func NewStore(now func() time.Time) *Store {
	if now == nil {
		now = time.Now
	}
	return &Store{
		accounts:       make(map[int64]authorization.Account),
		articles:       make(map[int64]article.Article),
		materialAssets: make(map[int64]material.Asset),
		publishRecords: make(map[int64]publish.Record),
		callbackEvents: make(map[int64]wechatcallback.Event),
		tickets:        make(map[string]authorization.ComponentVerifyTicket),
		bindings:       make(map[string]authorization.AuthorizerTenantBinding),
		now:            now,
	}
}

// SaveComponentVerifyTicket stores the latest component verify ticket.
func (s *Store) SaveComponentVerifyTicket(_ context.Context, ticket authorization.ComponentVerifyTicket) (authorization.ComponentVerifyTicket, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ticket.UpdatedAt = s.now()
	s.tickets[ticket.ComponentAppID] = ticket
	return ticket, nil
}

// GetComponentVerifyTicket returns the latest component verify ticket.
func (s *Store) GetComponentVerifyTicket(_ context.Context, componentAppID string) (authorization.ComponentVerifyTicket, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ticket, ok := s.tickets[componentAppID]
	if !ok {
		return authorization.ComponentVerifyTicket{}, fmt.Errorf("get component verify ticket lookup: %w", authorization.ErrComponentVerifyTicketNotFound)
	}
	return ticket, nil
}

// SaveAuthorizerTenantBinding stores callback routing metadata for an authorizer.
func (s *Store) SaveAuthorizerTenantBinding(_ context.Context, binding authorization.AuthorizerTenantBinding) (authorization.AuthorizerTenantBinding, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	binding.UpdatedAt = s.now()
	s.bindings[authorizerTenantBindingKey(binding.ComponentAppID, binding.AuthorizerAppID)] = binding
	return binding, nil
}

// GetAuthorizerTenantBinding returns callback routing metadata for an authorizer.
func (s *Store) GetAuthorizerTenantBinding(_ context.Context, componentAppID string, authorizerAppID string) (authorization.AuthorizerTenantBinding, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	binding, ok := s.bindings[authorizerTenantBindingKey(componentAppID, authorizerAppID)]
	if !ok {
		return authorization.AuthorizerTenantBinding{}, fmt.Errorf("get authorizer tenant binding lookup: %w", authorization.ErrAuthorizerTenantBindingNotFound)
	}
	return binding, nil
}

// CreateAccount stores a tenant-scoped official account.
func (s *Store) CreateAccount(_ context.Context, tenantID string, account authorization.Account) (authorization.Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextAccountID++
	now := s.now()
	account.ID = s.nextAccountID
	account.TenantID = tenantID
	account.CreatedAt = now
	account.UpdatedAt = now
	s.accounts[account.ID] = account
	return account, nil
}

// SaveAccount creates or updates a tenant-scoped official account by app id.
func (s *Store) SaveAccount(_ context.Context, tenantID string, account authorization.Account) (authorization.Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, current := range s.accounts {
		if current.TenantID == tenantID && current.AppID == account.AppID {
			account.ID = id
			account.TenantID = tenantID
			account.CreatedAt = current.CreatedAt
			account.UpdatedAt = s.now()
			s.accounts[id] = account
			return account, nil
		}
	}
	s.nextAccountID++
	now := s.now()
	account.ID = s.nextAccountID
	account.TenantID = tenantID
	account.CreatedAt = now
	account.UpdatedAt = now
	s.accounts[account.ID] = account
	return account, nil
}

// GetAccount returns one tenant-scoped official account.
func (s *Store) GetAccount(_ context.Context, tenantID string, id int64) (authorization.Account, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	account, ok := s.accounts[id]
	if !ok || account.TenantID != tenantID {
		return authorization.Account{}, fmt.Errorf("get account lookup: %w", authorization.ErrNotFound)
	}
	return account, nil
}

// ListAccounts returns tenant-scoped official accounts.
func (s *Store) ListAccounts(_ context.Context, tenantID string) ([]authorization.Account, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]authorization.Account, 0)
	for _, account := range s.accounts {
		if account.TenantID == tenantID {
			items = append(items, account)
		}
	}
	return items, nil
}

// RevokeAccountByAppID revokes a tenant-scoped official account by app id.
func (s *Store) RevokeAccountByAppID(_ context.Context, tenantID string, appID string) (authorization.Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, account := range s.accounts {
		if account.TenantID == tenantID && account.AppID == appID {
			account.Status = authorization.AccountStatusRevoked
			account.EncryptedAuthorizerRefreshToken = ""
			account.UpdatedAt = s.now()
			s.accounts[id] = account
			return account, nil
		}
	}
	return authorization.Account{}, fmt.Errorf("revoke account lookup: %w", authorization.ErrNotFound)
}

// UpdateAccountStatus updates one tenant-scoped official account status.
func (s *Store) UpdateAccountStatus(_ context.Context, tenantID string, id int64, status authorization.AccountStatus) (authorization.Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	account, ok := s.accounts[id]
	if !ok || account.TenantID != tenantID {
		return authorization.Account{}, fmt.Errorf("update account status lookup: %w", authorization.ErrNotFound)
	}
	account.Status = status
	account.UpdatedAt = s.now()
	s.accounts[id] = account
	return account, nil
}

// Create stores a tenant-scoped article.
func (s *Store) Create(_ context.Context, tenantID string, draft article.Article) (article.Article, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextArticleID++
	now := s.now()
	draft.ID = s.nextArticleID
	draft.TenantID = tenantID
	draft.CreatedAt = now
	draft.UpdatedAt = now
	s.articles[draft.ID] = draft
	return draft, nil
}

// Get returns one tenant-scoped article.
func (s *Store) Get(_ context.Context, tenantID string, id int64) (article.Article, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	draft, ok := s.articles[id]
	if !ok || draft.TenantID != tenantID {
		return article.Article{}, fmt.Errorf("get article lookup: %w", article.ErrNotFound)
	}
	return draft, nil
}

// List returns tenant-scoped articles.
func (s *Store) List(_ context.Context, tenantID string) ([]article.Article, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]article.Article, 0)
	for _, draft := range s.articles {
		if draft.TenantID == tenantID {
			items = append(items, draft)
		}
	}
	return items, nil
}

// Update replaces a tenant-scoped article.
func (s *Store) Update(_ context.Context, tenantID string, draft article.Article) (article.Article, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.articles[draft.ID]
	if !ok || current.TenantID != tenantID {
		return article.Article{}, fmt.Errorf("update article lookup: %w", article.ErrNotFound)
	}
	draft.TenantID = tenantID
	draft.CreatedAt = current.CreatedAt
	draft.UpdatedAt = s.now()
	s.articles[draft.ID] = draft
	return draft, nil
}

// Delete removes a tenant-scoped article.
func (s *Store) Delete(_ context.Context, tenantID string, id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	draft, ok := s.articles[id]
	if !ok || draft.TenantID != tenantID {
		return fmt.Errorf("delete article lookup: %w", article.ErrNotFound)
	}
	delete(s.articles, id)
	return nil
}

// CreateMaterial stores a tenant-scoped material asset.
func (s *Store) CreateMaterial(_ context.Context, tenantID string, asset material.Asset) (material.Asset, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextAssetID++
	asset.ID = s.nextAssetID
	asset.TenantID = tenantID
	asset.CreatedAt = s.now()
	s.materialAssets[asset.ID] = asset
	return asset, nil
}

// GetMaterial returns one tenant-scoped material asset.
func (s *Store) GetMaterial(_ context.Context, tenantID string, id int64) (material.Asset, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	asset, ok := s.materialAssets[id]
	if !ok || asset.TenantID != tenantID {
		return material.Asset{}, fmt.Errorf("get material lookup: %w", material.ErrNotFound)
	}
	return asset, nil
}

// ListMaterialByArticle returns material assets for an article.
func (s *Store) ListMaterialByArticle(_ context.Context, tenantID string, articleID int64) ([]material.Asset, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]material.Asset, 0)
	for _, asset := range s.materialAssets {
		if asset.TenantID == tenantID && asset.ArticleID == articleID {
			items = append(items, asset)
		}
	}
	return items, nil
}

// SaveCallbackEvent stores callback raw payload and dedupe metadata.
func (s *Store) SaveCallbackEvent(_ context.Context, event wechatcallback.Event) (wechatcallback.Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, current := range s.callbackEvents {
		if current.TenantID == event.TenantID && current.EventType == event.EventType && current.EventKey == event.EventKey {
			return wechatcallback.Event{}, fmt.Errorf("save callback event duplicate: %w", wechatcallback.ErrDuplicate)
		}
	}
	s.nextRecordID++
	event.ID = s.nextRecordID
	if event.ReceivedAt.IsZero() {
		event.ReceivedAt = s.now()
	}
	event.CreatedAt = s.now()
	s.callbackEvents[event.ID] = event
	return event, nil
}

// GetCallbackEventByKey returns one callback event by tenant-scoped dedupe key.
func (s *Store) GetCallbackEventByKey(_ context.Context, tenantID string, eventType string, eventKey string) (wechatcallback.Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, event := range s.callbackEvents {
		if event.TenantID == tenantID && event.EventType == eventType && event.EventKey == eventKey {
			return event, nil
		}
	}
	return wechatcallback.Event{}, fmt.Errorf("get callback event by key lookup: %w", wechatcallback.ErrNotFound)
}

func authorizerTenantBindingKey(componentAppID string, authorizerAppID string) string {
	return componentAppID + "\x00" + authorizerAppID
}
