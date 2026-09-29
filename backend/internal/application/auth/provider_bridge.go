package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	domainuser "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/user"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/requestmeta"
	"go.uber.org/zap"
)

const (
	// ProviderAuthBridgeProtocolVersion 标识 OAuth 交接契约版本。
	ProviderAuthBridgeProtocolVersion = 1
	// ProviderAuthWebClientID 标识提供方交接请求中的 Web 客户端。
	ProviderAuthWebClientID = "deeix-web"
	// ProviderAuthNativeClientID 标识提供方交接请求中的原生客户端。
	ProviderAuthNativeClientID = "com.deeix.chat.native"
	// ProviderAuthDesktopClientID 标识提供方交接请求中的桌面客户端。
	ProviderAuthDesktopClientID = "com.deeix.chat.desktop"

	providerAuthBridgeAudience = "provider_auth_bridge_v1"
	providerAuthTransactionTTL = 10 * time.Minute
	providerAuthGrantTTL       = 90 * time.Second
	providerAuthNativeRedirect = "com.deeix.chat:/oauth/callback"
)

type providerAuthBridgeState struct {
	Audience      string `json:"audience"`
	Provider      string `json:"provider"`
	TransactionID string `json:"transactionID"`
	ExpiresAt     int64  `json:"expiresAt"`
}

// ProviderAuthBridgeStartInput 包含客户端发起 OAuth 交接的请求。
type ProviderAuthBridgeStartInput struct {
	ClientID      string
	RedirectURI   string
	CodeChallenge string
	ClientState   string
	Intent        string
	Next          string
	// UserID 为发起绑定的已登录用户；登录/注册时为 0。
	UserID uint
}

// ProviderAuthBridgeStartResult 包含提供方授权 URL 及过期时间。
type ProviderAuthBridgeStartResult struct {
	AuthorizationURL string
	ExpiresAt        time.Time
}

// ProviderAuthBridgeCallbackInput 包含提供方回调参数。
type ProviderAuthBridgeCallbackInput struct {
	Code          string
	State         string
	ProviderError string
}

// ProviderAuthBridgeCallbackResult 包含回调生成的客户端重定向。
type ProviderAuthBridgeCallbackResult struct {
	RedirectURI string
}

// ProviderAuthBridgeExchangeInput 包含客户端兑换授权凭证的请求。
type ProviderAuthBridgeExchangeInput struct {
	ClientID     string
	Grant        string
	CodeVerifier string
}

// GetProviderAuthBridgeOptions 返回已配置的 OAuth 交接能力。
func (s *Service) GetProviderAuthBridgeOptions() ProviderAuthBridgeOptions {
	baseURL, err := s.providerAuthBridgeCallbackBaseURL()
	enabled := s != nil && s.providerAuthBridge != nil && err == nil
	if !enabled {
		baseURL = ""
	}
	return ProviderAuthBridgeOptions{
		Enabled:         enabled,
		ProtocolVersion: ProviderAuthBridgeProtocolVersion,
		CallbackBaseURL: baseURL,
	}
}

// StartProviderAuthBridge 启动短时效的提供方授权事务。
func (s *Service) StartProviderAuthBridge(
	ctx context.Context,
	slug string,
	input ProviderAuthBridgeStartInput,
) (*ProviderAuthBridgeStartResult, error) {
	if s == nil || s.providerAuthBridge == nil {
		return nil, ErrProviderAuthBridgeUnavailable
	}
	if !s.cfg.Snapshot().ThirdPartyLoginEnabled {
		return nil, ErrThirdPartyLoginDisabled
	}
	if err := s.validateProviderAuthClientRedirect(slug, input.ClientID, input.RedirectURI); err != nil {
		return nil, err
	}
	if err := validateProviderCodeChallenge(input.CodeChallenge); err != nil {
		return nil, err
	}
	if err := validateProviderClientState(input.ClientState); err != nil {
		return nil, err
	}
	provider, err := s.repo.GetIdentityProviderBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	normalizedIntent := normalizeProviderIntent(input.Intent)
	if err = validateProviderBridgeIntent(*provider, normalizedIntent, input.UserID); err != nil {
		return nil, err
	}
	authURL, _, _, err := s.resolveProviderEndpoints(ctx, *provider)
	if err != nil {
		return nil, err
	}
	callbackURL, err := s.providerAuthBridgeCallbackURL(slug)
	if err != nil {
		return nil, err
	}
	providerVerifier, err := randomProviderAuthToken(48)
	if err != nil {
		return nil, err
	}
	transactionID, err := randomProviderAuthToken(32)
	if err != nil {
		return nil, err
	}
	expiresAt := time.Now().Add(providerAuthTransactionTTL)
	transaction := repository.ProviderAuthTransaction{
		ProviderSlug:         slug,
		ClientID:             strings.TrimSpace(input.ClientID),
		ClientRedirectURI:    strings.TrimSpace(input.RedirectURI),
		ClientState:          strings.TrimSpace(input.ClientState),
		ClientCodeChallenge:  strings.TrimSpace(input.CodeChallenge),
		ProviderCodeVerifier: providerVerifier,
		Intent:               normalizedIntent,
		UserID:               input.UserID,
		Next:                 normalizeProviderNextPath(input.Next),
		ExpiresAt:            expiresAt,
	}
	if err = s.providerAuthBridge.PutProviderAuthTransaction(ctx, transactionID, transaction, providerAuthTransactionTTL); err != nil {
		return nil, err
	}
	state, err := s.signProviderAuthBridgeState(providerAuthBridgeState{
		Audience:      providerAuthBridgeAudience,
		Provider:      slug,
		TransactionID: transactionID,
		ExpiresAt:     expiresAt.Unix(),
	})
	if err != nil {
		return nil, err
	}
	target, err := buildProviderAuthURL(*provider, authURL, callbackURL, state, providerCodeChallenge(providerVerifier))
	if err != nil {
		return nil, err
	}
	return &ProviderAuthBridgeStartResult{AuthorizationURL: target, ExpiresAt: expiresAt}, nil
}

// CompleteProviderAuthBridgeCallback 消费提供方回调并创建授权凭证。
func (s *Service) CompleteProviderAuthBridgeCallback(
	ctx context.Context,
	slug string,
	input ProviderAuthBridgeCallbackInput,
) (*ProviderAuthBridgeCallbackResult, error) {
	if s == nil || s.providerAuthBridge == nil {
		return nil, ErrProviderAuthBridgeUnavailable
	}
	state, err := s.verifyProviderAuthBridgeState(slug, input.State)
	if err != nil {
		return nil, err
	}
	transaction, err := s.providerAuthBridge.ConsumeProviderAuthTransaction(ctx, state.TransactionID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrProviderTransactionExpired
		}
		return nil, err
	}
	if transaction.ProviderSlug != slug || time.Now().After(transaction.ExpiresAt) {
		return nil, ErrProviderTransactionMismatch
	}

	grant := repository.ProviderAuthGrant{
		ProviderSlug: slug,
		ClientID:     transaction.ClientID,
		Intent:       transaction.Intent,
		UserID:       transaction.UserID,
		ExpiresAt:    time.Now().Add(providerAuthGrantTTL),
	}
	if strings.TrimSpace(input.ProviderError) != "" {
		grant.ErrorCode = "auth.provider_authorization_denied"
		grant.ErrorMessage = "provider authorization was denied"
	} else if strings.TrimSpace(input.Code) == "" {
		grant.ErrorCode = "auth.provider_callback_invalid"
		grant.ErrorMessage = "provider callback did not include an authorization code"
	} else if providerErr := s.resolveProviderAuthGrant(ctx, slug, *transaction, strings.TrimSpace(input.Code), &grant); providerErr != nil {
		s.populateProviderAuthGrantError(&grant, providerErr)
	}

	rawGrant, err := randomProviderAuthToken(32)
	if err != nil {
		return nil, err
	}
	grantKey := providerAuthGrantKey(rawGrant, transaction.ClientCodeChallenge)
	if err = s.providerAuthBridge.PutProviderAuthGrant(ctx, grantKey, grant, providerAuthGrantTTL); err != nil {
		return nil, err
	}
	redirectURI, err := buildProviderAuthClientRedirect(*transaction, rawGrant)
	if err != nil {
		return nil, err
	}
	return &ProviderAuthBridgeCallbackResult{RedirectURI: redirectURI}, nil
}

// ExchangeProviderAuthBridgeGrant 将有效的一次性授权凭证兑换为应用会话。
func (s *Service) ExchangeProviderAuthBridgeGrant(
	ctx context.Context,
	slug string,
	input ProviderAuthBridgeExchangeInput,
	requestID string,
	auditCtx requestmeta.SessionAuditContext,
) (*LoginResult, error) {
	grant, err := s.consumeProviderAuthGrant(ctx, slug, input)
	if err != nil {
		return nil, err
	}
	if grant.Intent == providerIntentBind {
		return nil, ErrProviderGrantMismatch
	}
	userItem, err := s.repo.GetByID(ctx, grant.UserID)
	if err != nil {
		return nil, err
	}
	return s.completeProviderLoginForUser(ctx, userItem, grant.ProviderSlug, grant.Subject, requestID, auditCtx)
}

// ExchangeProviderAuthBridgeBindGrant 兑换绑定用的一次性授权凭证，把回调阶段取回的身份绑到当前登录用户。
func (s *Service) ExchangeProviderAuthBridgeBindGrant(
	ctx context.Context,
	userID uint,
	slug string,
	input ProviderAuthBridgeExchangeInput,
	requestID string,
	auditCtx requestmeta.SessionAuditContext,
) (*UserIdentityView, error) {
	if userID == 0 {
		return nil, ErrUnauthorized
	}
	grant, err := s.consumeProviderAuthGrant(ctx, slug, input)
	if err != nil {
		return nil, err
	}
	if grant.Intent != providerIntentBind || grant.UserID != userID || grant.Profile == nil {
		return nil, ErrProviderGrantMismatch
	}
	provider, err := s.repo.GetIdentityProviderBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	return s.bindProviderIdentity(ctx, userID, *provider, providerProfileClaims{
		Subject:       grant.Subject,
		Email:         grant.Profile.Email,
		DisplayName:   grant.Profile.DisplayName,
		EmailVerified: grant.Profile.EmailVerified,
		ProfileJSON:   grant.Profile.ProfileJSON,
	}, requestID, auditCtx)
}

// consumeProviderAuthGrant 校验 PKCE 并原子消费一次性授权凭证；回调阶段记录的错误在这里抛出。
func (s *Service) consumeProviderAuthGrant(ctx context.Context, slug string, input ProviderAuthBridgeExchangeInput) (*repository.ProviderAuthGrant, error) {
	if s == nil || s.providerAuthBridge == nil {
		return nil, ErrProviderAuthBridgeUnavailable
	}
	trimmedVerifier := strings.TrimSpace(input.CodeVerifier)
	if !providerPKCEPattern.MatchString(trimmedVerifier) {
		return nil, ErrPKCEVerifierRequired
	}
	trimmedGrant := strings.TrimSpace(input.Grant)
	if !providerPKCEPattern.MatchString(trimmedGrant) {
		return nil, ErrProviderGrantRequired
	}
	challenge := providerCodeChallenge(trimmedVerifier)
	grant, err := s.providerAuthBridge.ConsumeProviderAuthGrant(ctx, providerAuthGrantKey(trimmedGrant, challenge))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrProviderGrantExpired
		}
		return nil, err
	}
	if grant.ProviderSlug != slug || grant.ClientID != strings.TrimSpace(input.ClientID) || time.Now().After(grant.ExpiresAt) {
		return nil, ErrProviderGrantMismatch
	}
	if grant.ErrorCode != "" {
		return nil, providerAuthGrantError(*grant)
	}
	return grant, nil
}

// resolveProviderAuthGrant 用身份源回调的授权码填充一次性授权凭证：登录/注册在此解析或创建用户，绑定只保存资料等待已登录用户兑换。
func (s *Service) resolveProviderAuthGrant(
	ctx context.Context,
	slug string,
	transaction repository.ProviderAuthTransaction,
	code string,
	grant *repository.ProviderAuthGrant,
) error {
	provider, err := s.repo.GetIdentityProviderBySlug(ctx, slug)
	if err != nil {
		return err
	}
	if err = validateProviderBridgeIntent(*provider, transaction.Intent, transaction.UserID); err != nil {
		return err
	}
	callbackURL, err := s.providerAuthBridgeCallbackURL(slug)
	if err != nil {
		return err
	}
	if transaction.Intent == providerIntentBind {
		claims, claimsErr := s.fetchProviderProfileClaims(ctx, *provider, code, callbackURL, transaction.ProviderCodeVerifier)
		if claimsErr != nil {
			return claimsErr
		}
		grant.Subject = claims.Subject
		grant.Profile = &repository.ProviderAuthGrantProfile{
			DisplayName:   claims.DisplayName,
			Email:         claims.Email,
			EmailVerified: claims.EmailVerified,
			ProfileJSON:   claims.ProfileJSON,
		}
		return nil
	}
	userItem, subject, err := s.resolveProviderLoginCode(ctx, *provider, code, callbackURL, transaction.ProviderCodeVerifier)
	if err != nil {
		return err
	}
	grant.UserID = userItem.ID
	grant.Subject = subject
	return nil
}

// validateProviderBridgeIntent 校验意图与身份源开关、登录状态是否匹配；绑定必须由已登录用户发起。
func validateProviderBridgeIntent(provider domainuser.IdentityProvider, intent string, userID uint) error {
	if intent == providerIntentBind {
		if userID == 0 {
			return ErrUnauthorized
		}
		if !provider.LoginEnabled {
			return ErrProviderLoginDisabled
		}
		return nil
	}
	return validateProviderLoginIntent(provider, intent)
}

func validateProviderLoginIntent(provider domainuser.IdentityProvider, intent string) error {
	if intent == providerIntentLogin && !provider.LoginEnabled {
		return ErrProviderLoginDisabled
	}
	if intent == providerIntentRegister && (!provider.LoginEnabled || !provider.RegistrationEnabled) {
		return ErrProviderRegistrationDisabled
	}
	return nil
}

func validateProviderClientState(value string) error {
	if !providerPKCEPattern.MatchString(strings.TrimSpace(value)) {
		return ErrProviderClientStateInvalid
	}
	return nil
}

func (s *Service) validateProviderAuthClientRedirect(slug string, clientID string, redirectURI string) error {
	trimmedClientID := strings.TrimSpace(clientID)
	trimmedRedirectURI := strings.TrimSpace(redirectURI)
	switch trimmedClientID {
	case ProviderAuthWebClientID:
		return s.validateProviderRedirectURI(slug, trimmedRedirectURI)
	case ProviderAuthNativeClientID:
		if trimmedRedirectURI != providerAuthNativeRedirect {
			return ErrProviderNativeRedirectInvalid
		}
		return nil
	case ProviderAuthDesktopClientID:
		parsed, err := url.Parse(trimmedRedirectURI)
		if err != nil || parsed.Scheme != "http" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
			return ErrProviderDesktopRedirectInvalid
		}
		if !isLoopbackHost(parsed.Hostname()) || parsed.Port() == "" || parsed.Path != "/oauth/callback" {
			return ErrProviderDesktopRedirectInvalid
		}
		if port, portErr := strconv.Atoi(parsed.Port()); portErr != nil || port < 1 || port > 65535 {
			return ErrProviderDesktopRedirectInvalid
		}
		return nil
	default:
		return ErrProviderAuthClientUnsupported
	}
}

func (s *Service) providerAuthBridgeCallbackBaseURL() (string, error) {
	if s == nil || s.cfg == nil {
		return "", ErrProviderCallbackMisconfigured
	}
	raw := strings.TrimRight(strings.TrimSpace(s.cfg.Snapshot().PublicAPIBaseURL), "/")
	parsed, err := url.Parse(raw)
	if err != nil || parsed == nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", ErrProviderCallbackBaseInvalid
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", ErrProviderCallbackBaseDirty
	}
	return raw + "/api/v1/auth/providers", nil
}

func (s *Service) providerAuthBridgeCallbackURL(slug string) (string, error) {
	baseURL, err := s.providerAuthBridgeCallbackBaseURL()
	if err != nil {
		return "", err
	}
	return baseURL + "/" + url.PathEscape(slug) + "/callback", nil
}

func (s *Service) signProviderAuthBridgeState(state providerAuthBridgeState) (string, error) {
	payload, err := json.Marshal(state)
	if err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	return encoded + "." + providerStateSignature(s.cfg.Snapshot().JWTSecret, encoded), nil
}

func (s *Service) verifyProviderAuthBridgeState(slug string, raw string) (*providerAuthBridgeState, error) {
	parts := strings.Split(strings.TrimSpace(raw), ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil, ErrProviderBridgeStateInvalid
	}
	if !secureStringEqual(providerStateSignature(s.cfg.Snapshot().JWTSecret, parts[0]), parts[1]) {
		return nil, ErrProviderBridgeStateInvalid
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, ErrProviderBridgeStateInvalid
	}
	var state providerAuthBridgeState
	if err = json.Unmarshal(payload, &state); err != nil {
		return nil, ErrProviderBridgeStateInvalid
	}
	if state.Audience != providerAuthBridgeAudience || state.Provider != slug || state.TransactionID == "" {
		return nil, ErrProviderBridgeStateMismatch
	}
	if time.Now().Unix() > state.ExpiresAt {
		return nil, ErrProviderBridgeStateExpired
	}
	return &state, nil
}

func secureStringEqual(left string, right string) bool {
	return hmac.Equal([]byte(left), []byte(right))
}

func randomProviderAuthToken(size int) (string, error) {
	if size < 32 {
		size = 32
	}
	raw := make([]byte, size)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate provider auth token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func providerAuthGrantKey(grant string, codeChallenge string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(grant) + ":" + strings.TrimSpace(codeChallenge)))
	return hex.EncodeToString(sum[:])
}

func buildProviderAuthClientRedirect(transaction repository.ProviderAuthTransaction, grant string) (string, error) {
	parsed, err := url.Parse(transaction.ClientRedirectURI)
	if err != nil {
		return "", err
	}
	query := parsed.Query()
	query.Set("provider", transaction.ProviderSlug)
	query.Set("grant", grant)
	query.Set("state", transaction.ClientState)
	query.Set("intent", transaction.Intent)
	query.Set("next", transaction.Next)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func (s *Service) populateProviderAuthGrantError(grant *repository.ProviderAuthGrant, err error) {
	if grant == nil || err == nil {
		return
	}
	var conflict *ProviderEmailConflictError
	if errors.As(err, &conflict) {
		grant.ErrorCode = "auth.provider_email_conflict"
		grant.ErrorMessage = conflict.Error()
		details, _ := json.Marshal(map[string]string{
			"providerSlug": conflict.ProviderSlug,
			"email":        conflict.Email,
			"action":       conflict.Action,
		})
		grant.ErrorDetails = string(details)
		return
	}
	grant.ErrorCode = "auth.provider_authentication_failed"
	grant.ErrorMessage = "provider authentication failed"
	s.warn("provider_auth_bridge_callback_failed", zap.String("provider", grant.ProviderSlug), zap.Error(err))
}

func providerAuthGrantError(grant repository.ProviderAuthGrant) error {
	switch strings.TrimSpace(grant.ErrorCode) {
	case "auth.provider_email_conflict":
		var details struct {
			ProviderSlug string `json:"providerSlug"`
			Email        string `json:"email"`
			Action       string `json:"action"`
		}
		if json.Unmarshal([]byte(grant.ErrorDetails), &details) == nil {
			return &ProviderEmailConflictError{
				ProviderSlug: details.ProviderSlug,
				Email:        details.Email,
				Action:       details.Action,
			}
		}
	case "auth.provider_authorization_denied":
		return ErrProviderAuthorizationDenied
	case "auth.provider_callback_invalid":
		return ErrProviderCallbackInvalid
	case "auth.provider_authentication_failed", "auth.provider_invalid":
		return ErrProviderAuthenticationFailed
	}
	return ErrProviderAuthenticationFailed
}
