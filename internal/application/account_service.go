package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"official-account-service/internal/domain/authorization"
)

// AccountService manages tenant-scoped official accounts.
type AccountService struct {
	accounts authorization.Repository
}

// NewAccountService constructs an AccountService.
func NewAccountService(accounts authorization.Repository) *AccountService {
	return &AccountService{accounts: accounts}
}

// SaveAccountInput contains authorized official account fields from an authorization flow.
type SaveAccountInput struct {
	TenantID                        string
	AppID                           string
	Name                            string
	AvatarURL                       string
	Status                          authorization.AccountStatus
	EncryptedAuthorizerRefreshToken string
	LastSyncedAt                    time.Time
}

// SaveAccount creates or updates one tenant-scoped official account by app id.
func (s *AccountService) SaveAccount(ctx context.Context, input SaveAccountInput) (authorization.Account, error) {
	if err := s.validateReady(); err != nil {
		return authorization.Account{}, err
	}
	if strings.TrimSpace(input.TenantID) == "" {
		return authorization.Account{}, fmt.Errorf("validate account tenant id: %w", ErrInvalidInput)
	}
	if strings.TrimSpace(input.AppID) == "" {
		return authorization.Account{}, fmt.Errorf("validate account app id: %w", ErrInvalidInput)
	}
	if err := validateAccountStatus(input.Status); err != nil {
		return authorization.Account{}, err
	}
	account, err := s.accounts.SaveAccount(ctx, input.TenantID, authorization.Account{
		TenantID: input.TenantID, AppID: input.AppID, Name: input.Name, AvatarURL: input.AvatarURL,
		Status: input.Status, EncryptedAuthorizerRefreshToken: input.EncryptedAuthorizerRefreshToken,
		LastSyncedAt: input.LastSyncedAt,
	})
	if errors.Is(err, authorization.ErrConflict) {
		return authorization.Account{}, fmt.Errorf("save account: %w", ErrConflict)
	}
	if err != nil {
		return authorization.Account{}, fmt.Errorf("save account: %w", err)
	}
	return account, nil
}

// GetAccount returns one tenant-scoped official account.
func (s *AccountService) GetAccount(ctx context.Context, tenantID string, id int64) (authorization.Account, error) {
	if err := s.validateAccountID(tenantID, id); err != nil {
		return authorization.Account{}, err
	}
	account, err := s.accounts.GetAccount(ctx, tenantID, id)
	if err != nil {
		return authorization.Account{}, wrapAccountReadError("get account", err)
	}
	return account, nil
}

// ListAccounts returns tenant-scoped official accounts.
func (s *AccountService) ListAccounts(ctx context.Context, tenantID string) ([]authorization.Account, error) {
	if err := s.validateReady(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(tenantID) == "" {
		return nil, fmt.Errorf("validate account tenant id: %w", ErrInvalidInput)
	}
	accounts, err := s.accounts.ListAccounts(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}
	return accounts, nil
}

// UpdateAccountStatus updates one tenant-scoped official account status.
func (s *AccountService) UpdateAccountStatus(ctx context.Context, tenantID string, id int64, status authorization.AccountStatus) (authorization.Account, error) {
	if err := s.validateAccountID(tenantID, id); err != nil {
		return authorization.Account{}, err
	}
	if err := validateAccountStatus(status); err != nil {
		return authorization.Account{}, err
	}
	account, err := s.accounts.UpdateAccountStatus(ctx, tenantID, id, status)
	if err != nil {
		return authorization.Account{}, wrapAccountReadError("update account status", err)
	}
	return account, nil
}

func (s *AccountService) validateReady() error {
	if s == nil || s.accounts == nil {
		return fmt.Errorf("validate account service repository: %w", ErrInvalidInput)
	}
	return nil
}

func (s *AccountService) validateAccountID(tenantID string, id int64) error {
	if err := s.validateReady(); err != nil {
		return err
	}
	if strings.TrimSpace(tenantID) == "" {
		return fmt.Errorf("validate account tenant id: %w", ErrInvalidInput)
	}
	if id <= 0 {
		return fmt.Errorf("validate account id: %w", ErrInvalidInput)
	}
	return nil
}

func validateAccountStatus(status authorization.AccountStatus) error {
	switch status {
	case authorization.AccountStatusActive, authorization.AccountStatusRevoked, authorization.AccountStatusRefreshFailed:
		return nil
	default:
		return fmt.Errorf("validate account status: %w", ErrInvalidInput)
	}
}

func wrapAccountReadError(action string, err error) error {
	if errors.Is(err, authorization.ErrNotFound) {
		return fmt.Errorf("%s: %w", action, ErrNotFound)
	}
	return fmt.Errorf("%s: %w", action, err)
}
