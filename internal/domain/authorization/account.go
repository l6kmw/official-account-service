package authorization

import (
	"context"
	"errors"
	"time"
)

// ErrNotFound indicates a tenant-scoped authorization account was not found.
var ErrNotFound = errors.New("authorization account not found")

// AccountStatus describes an authorized official account status.
type AccountStatus string

const (
	// AccountStatusActive means the account can be used for publishing.
	AccountStatusActive AccountStatus = "active"
	// AccountStatusRevoked means the user revoked authorization.
	AccountStatusRevoked AccountStatus = "revoked"
	// AccountStatusRefreshFailed means token refresh failed and requires re-authorization.
	AccountStatusRefreshFailed AccountStatus = "refresh_failed"
)

// Account represents an authorized WeChat official account.
type Account struct {
	ID                              int64
	TenantID                        string
	AppID                           string
	Name                            string
	AvatarURL                       string
	Status                          AccountStatus
	EncryptedAuthorizerRefreshToken string
	LastSyncedAt                    time.Time
	CreatedAt                       time.Time
	UpdatedAt                       time.Time
}

// Repository persists tenant-scoped authorized official accounts.
type Repository interface {
	CreateAccount(ctx context.Context, tenantID string, account Account) (Account, error)
	SaveAccount(ctx context.Context, tenantID string, account Account) (Account, error)
	GetAccount(ctx context.Context, tenantID string, id int64) (Account, error)
	ListAccounts(ctx context.Context, tenantID string) ([]Account, error)
	RevokeAccountByAppID(ctx context.Context, tenantID string, appID string) (Account, error)
	UpdateAccountStatus(ctx context.Context, tenantID string, id int64, status AccountStatus) (Account, error)
}
