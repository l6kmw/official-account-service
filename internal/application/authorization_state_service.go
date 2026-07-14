package application

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"official-account-service/internal/domain/authorization"
)

const maximumAuthorizationStateTTL = 10 * time.Minute

func (s *AuthorizationService) createAuthorizationState(ctx context.Context, input GenerateAuthorizationURLInput, expiresInSeconds int) (string, error) {
	if expiresInSeconds <= 0 {
		return "", fmt.Errorf("validate authorization state expiry: %w", ErrInvalidInput)
	}
	token, err := newAuthorizationStateToken()
	if err != nil {
		return "", err
	}
	ttl := time.Duration(expiresInSeconds) * time.Second
	if ttl > maximumAuthorizationStateTTL {
		ttl = maximumAuthorizationStateTTL
	}
	_, err = s.states.SaveAuthorizationState(ctx, authorization.AuthorizationState{
		Digest: authorizationStateDigest(token), TenantID: strings.TrimSpace(input.TenantID),
		ComponentAppID: strings.TrimSpace(input.ComponentAppID), ExpiresAt: s.now().Add(ttl),
	})
	if err != nil {
		return "", fmt.Errorf("save authorization state: %w", err)
	}
	redirectURI, err := url.Parse(input.RedirectURI)
	if err != nil {
		return "", fmt.Errorf("parse authorization state redirect uri: %w", ErrInvalidInput)
	}
	query := redirectURI.Query()
	query.Del("tenant_id")
	query.Del("component_appid")
	query.Set("state", token)
	redirectURI.RawQuery = query.Encode()
	return redirectURI.String(), nil
}

func (s *AuthorizationService) consumeAuthorizationState(ctx context.Context, token string) (authorization.AuthorizationState, error) {
	state, err := s.states.ConsumeAuthorizationState(ctx, authorizationStateDigest(strings.TrimSpace(token)), s.now())
	if err != nil {
		if errors.Is(err, authorization.ErrAuthorizationStateNotFound) {
			return authorization.AuthorizationState{}, fmt.Errorf("consume authorization state: %w", ErrInvalidInput)
		}
		return authorization.AuthorizationState{}, fmt.Errorf("consume authorization state: %w", err)
	}
	return state, nil
}

func newAuthorizationStateToken() (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate authorization state: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw[:]), nil
}

func authorizationStateDigest(token string) string {
	digest := sha256.Sum256([]byte(token))
	return hex.EncodeToString(digest[:])
}
