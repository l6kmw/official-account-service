package application

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"official-account-service/internal/domain/authorization"
)

const weChatComponentLoginURL = "https://mp.weixin.qq.com/cgi-bin/componentloginpage"

const (
	// AuthorizationAuthTypeOfficialAccount limits authorization to official accounts.
	AuthorizationAuthTypeOfficialAccount = 1
	// AuthorizationAuthTypeMiniProgram limits authorization to mini programs.
	AuthorizationAuthTypeMiniProgram = 2
	// AuthorizationAuthTypeAll allows either official accounts or mini programs.
	AuthorizationAuthTypeAll = 3
)

// AuthorizationService manages WeChat third-party platform authorization state.
type AuthorizationService struct {
	tickets           authorization.ComponentVerifyTicketRepository
	accounts          authorization.Repository
	bindings          authorization.AuthorizerTenantBindingRepository
	preAuthCodes      authorization.PreAuthCodeCreator
	callbackDecryptor authorization.ComponentCallbackDecryptor
	authorizers       authorization.AuthorizerClient
	refreshTokens     authorization.RefreshTokenEncryptor
	now               func() time.Time
}

// NewAuthorizationService constructs an AuthorizationService.
func NewAuthorizationService(tickets authorization.ComponentVerifyTicketRepository, now func() time.Time) *AuthorizationService {
	return NewAuthorizationServiceWithPreAuthCodeCreator(tickets, nil, now)
}

// NewAuthorizationServiceWithPreAuthCodeCreator constructs an AuthorizationService with a pre-auth-code creator.
func NewAuthorizationServiceWithPreAuthCodeCreator(tickets authorization.ComponentVerifyTicketRepository, preAuthCodes authorization.PreAuthCodeCreator, now func() time.Time) *AuthorizationService {
	return NewAuthorizationServiceWithDependencies(tickets, preAuthCodes, nil, now)
}

// NewAuthorizationServiceWithDependencies constructs an AuthorizationService with optional dependencies.
func NewAuthorizationServiceWithDependencies(tickets authorization.ComponentVerifyTicketRepository, preAuthCodes authorization.PreAuthCodeCreator, callbackDecryptor authorization.ComponentCallbackDecryptor, now func() time.Time) *AuthorizationService {
	return NewAuthorizationServiceWithAuthorizationFlow(tickets, repositoryFromComponentTickets(tickets), preAuthCodes, callbackDecryptor, nil, nil, now)
}

// NewAuthorizationServiceWithAuthorizationFlow constructs an AuthorizationService with authorization callback dependencies.
func NewAuthorizationServiceWithAuthorizationFlow(tickets authorization.ComponentVerifyTicketRepository, accounts authorization.Repository, preAuthCodes authorization.PreAuthCodeCreator, callbackDecryptor authorization.ComponentCallbackDecryptor, authorizers authorization.AuthorizerClient, refreshTokens authorization.RefreshTokenEncryptor, now func() time.Time) *AuthorizationService {
	if now == nil {
		now = time.Now
	}
	return &AuthorizationService{
		tickets: tickets, accounts: accounts, bindings: bindingRepositoryFromDependencies(tickets, accounts),
		preAuthCodes: preAuthCodes, callbackDecryptor: callbackDecryptor,
		authorizers: authorizers, refreshTokens: refreshTokens, now: now,
	}
}

// SaveComponentVerifyTicketInput contains a WeChat component verify ticket callback payload.
type SaveComponentVerifyTicketInput struct {
	ComponentAppID string
	Ticket         string
	ReceivedAt     time.Time
}

// HandleComponentCallbackInput contains a raw WeChat component callback request.
type HandleComponentCallbackInput struct {
	RawBody      []byte
	EncryptType  string
	MsgSignature string
	Timestamp    string
	Nonce        string
}

// HandleAuthorizationCallbackInput contains a WeChat authorization redirect callback.
type HandleAuthorizationCallbackInput struct {
	TenantID       string
	ComponentAppID string
	AuthCode       string
	ReceivedAt     time.Time
}

// GenerateAuthorizationURLInput contains fields for creating a WeChat component authorization URL.
type GenerateAuthorizationURLInput struct {
	ComponentAppID string
	RedirectURI    string
	AuthType       int
	BizAppID       string
}

// AuthorizationURL contains a generated WeChat component authorization URL.
type AuthorizationURL struct {
	URL                     string
	PreAuthCodeExpiresInSec int
}

// HandleAuthorizationCallback exchanges a WeChat auth code and saves the authorized account.
func (s *AuthorizationService) HandleAuthorizationCallback(ctx context.Context, input HandleAuthorizationCallbackInput) (authorization.Account, error) {
	if err := s.validateAuthorizationFlowReady(); err != nil {
		return authorization.Account{}, err
	}
	if strings.TrimSpace(input.TenantID) == "" {
		return authorization.Account{}, fmt.Errorf("validate authorization callback tenant id: %w", ErrInvalidInput)
	}
	if strings.TrimSpace(input.ComponentAppID) == "" {
		return authorization.Account{}, fmt.Errorf("validate authorization callback component app id: %w", ErrInvalidInput)
	}
	if strings.TrimSpace(input.AuthCode) == "" {
		return authorization.Account{}, fmt.Errorf("validate authorization callback auth code: %w", ErrInvalidInput)
	}
	authorized, err := s.authorizers.QueryAuthorizerAuthorization(ctx, input.ComponentAppID, input.AuthCode)
	if err != nil {
		return authorization.Account{}, wrapAuthorizerClientError("query authorizer authorization", err)
	}
	if strings.TrimSpace(authorized.AppID) == "" {
		return authorization.Account{}, fmt.Errorf("validate authorizer app id: %w", ErrInvalidInput)
	}
	if strings.TrimSpace(authorized.RefreshToken) == "" {
		return authorization.Account{}, fmt.Errorf("validate authorizer refresh token: %w", ErrInvalidInput)
	}
	profile, err := s.authorizers.GetAuthorizerProfile(ctx, input.ComponentAppID, authorized.AppID)
	if err != nil {
		return authorization.Account{}, wrapAuthorizerClientError("get authorizer profile", err)
	}
	encryptedRefreshToken, err := s.refreshTokens.EncryptAuthorizerRefreshToken(ctx, authorized.RefreshToken)
	if err != nil {
		return authorization.Account{}, wrapRefreshTokenEncryptorError("encrypt authorizer refresh token", err)
	}
	if strings.TrimSpace(encryptedRefreshToken) == "" {
		return authorization.Account{}, fmt.Errorf("validate encrypted authorizer refresh token: %w", ErrInvalidInput)
	}
	receivedAt := input.ReceivedAt
	if receivedAt.IsZero() {
		receivedAt = s.now()
	}
	account, err := s.accounts.SaveAccount(ctx, input.TenantID, authorization.Account{
		TenantID: input.TenantID, AppID: authorized.AppID, Name: profile.Name, AvatarURL: profile.AvatarURL,
		Status: authorization.AccountStatusActive, EncryptedAuthorizerRefreshToken: encryptedRefreshToken,
		LastSyncedAt: receivedAt,
	})
	if err != nil {
		return authorization.Account{}, fmt.Errorf("save authorized account: %w", err)
	}
	if s.bindings != nil {
		_, err = s.bindings.SaveAuthorizerTenantBinding(ctx, authorization.AuthorizerTenantBinding{
			ComponentAppID:  input.ComponentAppID,
			AuthorizerAppID: authorized.AppID,
			TenantID:        input.TenantID,
		})
		if err != nil {
			return authorization.Account{}, fmt.Errorf("save authorizer tenant binding: %w", err)
		}
	}
	return account, nil
}

// SaveComponentVerifyTicket saves the latest component verify ticket.
func (s *AuthorizationService) SaveComponentVerifyTicket(ctx context.Context, input SaveComponentVerifyTicketInput) (authorization.ComponentVerifyTicket, error) {
	if err := s.validateReady(); err != nil {
		return authorization.ComponentVerifyTicket{}, err
	}
	if strings.TrimSpace(input.ComponentAppID) == "" {
		return authorization.ComponentVerifyTicket{}, fmt.Errorf("validate component app id: %w", ErrInvalidInput)
	}
	if strings.TrimSpace(input.Ticket) == "" {
		return authorization.ComponentVerifyTicket{}, fmt.Errorf("validate component verify ticket: %w", ErrInvalidInput)
	}
	receivedAt := input.ReceivedAt
	if receivedAt.IsZero() {
		receivedAt = s.now()
	}
	ticket, err := s.tickets.SaveComponentVerifyTicket(ctx, authorization.ComponentVerifyTicket{
		ComponentAppID: input.ComponentAppID,
		Ticket:         input.Ticket,
		ReceivedAt:     receivedAt,
	})
	if err != nil {
		return authorization.ComponentVerifyTicket{}, fmt.Errorf("save component verify ticket: %w", err)
	}
	return ticket, nil
}

// GenerateAuthorizationURL creates a WeChat third-party platform authorization URL.
func (s *AuthorizationService) GenerateAuthorizationURL(ctx context.Context, input GenerateAuthorizationURLInput) (AuthorizationURL, error) {
	if err := s.validatePreAuthCodeReady(); err != nil {
		return AuthorizationURL{}, err
	}
	if err := validateAuthorizationURLInput(input); err != nil {
		return AuthorizationURL{}, err
	}
	preAuthCode, err := s.preAuthCodes.CreatePreAuthCode(ctx, input.ComponentAppID)
	if err != nil {
		return AuthorizationURL{}, wrapPreAuthCodeError("create pre auth code", err)
	}
	if strings.TrimSpace(preAuthCode.Code) == "" {
		return AuthorizationURL{}, fmt.Errorf("validate pre auth code result: %w", ErrInvalidInput)
	}
	authType := input.AuthType
	if authType == 0 {
		authType = AuthorizationAuthTypeOfficialAccount
	}
	values := url.Values{}
	values.Set("component_appid", input.ComponentAppID)
	values.Set("pre_auth_code", preAuthCode.Code)
	values.Set("redirect_uri", input.RedirectURI)
	values.Set("auth_type", fmt.Sprintf("%d", authType))
	if strings.TrimSpace(input.BizAppID) != "" {
		values.Set("biz_appid", input.BizAppID)
	}
	return AuthorizationURL{
		URL:                     weChatComponentLoginURL + "?" + values.Encode(),
		PreAuthCodeExpiresInSec: preAuthCode.ExpiresInSeconds,
	}, nil
}

// GetComponentVerifyTicket returns the latest component verify ticket for a component app id.
func (s *AuthorizationService) GetComponentVerifyTicket(ctx context.Context, componentAppID string) (authorization.ComponentVerifyTicket, error) {
	if err := s.validateReady(); err != nil {
		return authorization.ComponentVerifyTicket{}, err
	}
	if strings.TrimSpace(componentAppID) == "" {
		return authorization.ComponentVerifyTicket{}, fmt.Errorf("validate component app id: %w", ErrInvalidInput)
	}
	ticket, err := s.tickets.GetComponentVerifyTicket(ctx, componentAppID)
	if err != nil {
		return authorization.ComponentVerifyTicket{}, wrapComponentVerifyTicketReadError("get component verify ticket", err)
	}
	return ticket, nil
}

func (s *AuthorizationService) validateReady() error {
	if s == nil || s.tickets == nil {
		return fmt.Errorf("validate authorization service repository: %w", ErrInvalidInput)
	}
	return nil
}

func (s *AuthorizationService) validateAuthorizationFlowReady() error {
	if s == nil || s.accounts == nil {
		return fmt.Errorf("validate authorization account repository: %w", ErrInvalidInput)
	}
	if s.authorizers == nil {
		return fmt.Errorf("validate authorizer client: %w", ErrNotImplemented)
	}
	if s.refreshTokens == nil {
		return fmt.Errorf("validate refresh token encryptor: %w", ErrNotImplemented)
	}
	return nil
}

func (s *AuthorizationService) validateUnauthorizedCallbackReady() error {
	if s == nil || s.accounts == nil {
		return fmt.Errorf("validate authorization account repository: %w", ErrInvalidInput)
	}
	if s.bindings == nil {
		return fmt.Errorf("validate authorizer tenant binding repository: %w", ErrNotImplemented)
	}
	return nil
}

func (s *AuthorizationService) validatePreAuthCodeReady() error {
	if s == nil || s.preAuthCodes == nil {
		return fmt.Errorf("validate pre auth code creator: %w", ErrNotImplemented)
	}
	return nil
}

func validateAuthorizationURLInput(input GenerateAuthorizationURLInput) error {
	if strings.TrimSpace(input.ComponentAppID) == "" {
		return fmt.Errorf("validate component app id: %w", ErrInvalidInput)
	}
	if strings.TrimSpace(input.RedirectURI) == "" {
		return fmt.Errorf("validate authorization redirect uri: %w", ErrInvalidInput)
	}
	parsed, err := url.Parse(input.RedirectURI)
	if err != nil {
		return fmt.Errorf("parse authorization redirect uri: %w", ErrInvalidInput)
	}
	if parsed.Scheme != "https" || parsed.Host == "" {
		return fmt.Errorf("validate authorization redirect uri: %w", ErrInvalidInput)
	}
	if input.AuthType != 0 && input.AuthType != AuthorizationAuthTypeOfficialAccount && input.AuthType != AuthorizationAuthTypeMiniProgram && input.AuthType != AuthorizationAuthTypeAll {
		return fmt.Errorf("validate authorization auth type: %w", ErrInvalidInput)
	}
	return nil
}

func wrapComponentVerifyTicketReadError(action string, err error) error {
	if errors.Is(err, authorization.ErrComponentVerifyTicketNotFound) {
		return fmt.Errorf("%s: %w", action, ErrNotFound)
	}
	return fmt.Errorf("%s: %w", action, err)
}

func wrapPreAuthCodeError(action string, err error) error {
	if errors.Is(err, authorization.ErrComponentVerifyTicketNotFound) {
		return fmt.Errorf("%s: %w", action, ErrNotFound)
	}
	if errors.Is(err, authorization.ErrPreAuthCodeUnavailable) {
		return fmt.Errorf("%s: %w", action, ErrNotImplemented)
	}
	return fmt.Errorf("%s: %w", action, err)
}

func wrapComponentCallbackDecryptError(action string, err error) error {
	if errors.Is(err, authorization.ErrComponentCallbackCryptoUnavailable) {
		return fmt.Errorf("%s: %w", action, ErrNotImplemented)
	}
	if errors.Is(err, authorization.ErrComponentCallbackSignatureInvalid) {
		return fmt.Errorf("%s: %w", action, ErrInvalidInput)
	}
	return fmt.Errorf("%s: %w", action, err)
}

func wrapAuthorizerClientError(action string, err error) error {
	if errors.Is(err, authorization.ErrComponentVerifyTicketNotFound) {
		return fmt.Errorf("%s: %w", action, ErrNotFound)
	}
	if errors.Is(err, authorization.ErrAuthorizerClientUnavailable) || errors.Is(err, authorization.ErrPreAuthCodeUnavailable) {
		return fmt.Errorf("%s: %w", action, ErrNotImplemented)
	}
	return fmt.Errorf("%s: %w", action, err)
}

func wrapRefreshTokenEncryptorError(action string, err error) error {
	if errors.Is(err, authorization.ErrRefreshTokenEncryptorUnavailable) {
		return fmt.Errorf("%s: %w", action, ErrNotImplemented)
	}
	return fmt.Errorf("%s: %w", action, err)
}

func wrapAuthorizerTenantBindingError(action string, err error) error {
	if errors.Is(err, authorization.ErrAuthorizerTenantBindingNotFound) {
		return fmt.Errorf("%s: %w", action, ErrNotFound)
	}
	return fmt.Errorf("%s: %w", action, err)
}

func wrapAccountWriteError(action string, err error) error {
	if errors.Is(err, authorization.ErrNotFound) {
		return fmt.Errorf("%s: %w", action, ErrNotFound)
	}
	return fmt.Errorf("%s: %w", action, err)
}

func repositoryFromComponentTickets(tickets authorization.ComponentVerifyTicketRepository) authorization.Repository {
	accounts, _ := tickets.(authorization.Repository)
	return accounts
}

func bindingRepositoryFromDependencies(tickets authorization.ComponentVerifyTicketRepository, accounts authorization.Repository) authorization.AuthorizerTenantBindingRepository {
	if bindings, ok := tickets.(authorization.AuthorizerTenantBindingRepository); ok {
		return bindings
	}
	bindings, _ := accounts.(authorization.AuthorizerTenantBindingRepository)
	return bindings
}
