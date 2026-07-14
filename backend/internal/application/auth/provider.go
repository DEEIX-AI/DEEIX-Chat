package auth

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"
	"time"

	userapp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/user"
	domainuser "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/user"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/conv"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/secretbox"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/requestmeta"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type LoginOptions struct {
	UsernameEnabled              bool
	EmailEnabled                 bool
	EmailRegistrationEnabled     bool
	EmailVerificationEnabled     bool
	PasswordResetEnabled         bool
	TurnstileRegistrationEnabled bool
	TurnstileSiteKey             string
	Providers                    []IdentityProviderView
}

type IdentityProviderView struct {
	PublicID            string
	Type                string
	Name                string
	Slug                string
	LogoURL             string
	LoginEnabled        bool
	RegistrationEnabled bool
	ClientID            string
	IssuerURL           string
	DiscoveryURL        string
	AuthURL             string
	TokenURL            string
	UserInfoURL         string
	JWKSURL             string
	Scopes              string
	DefaultRole         string
	SubjectField        string
	EmailField          string
	EmailVerifiedField  string
	NameField           string
	AvatarField         string
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type UserIdentityView struct {
	ID                  uint
	ProviderID          uint
	ProviderType        string
	ProviderName        string
	ProviderSlug        string
	ProviderLogoURL     string
	ProviderDisplayName string
	Email               string
	EmailVerified       bool
	LinkedAt            time.Time
	LastLoginAt         *time.Time
}

type IdentityProviderLogoAsset struct {
	ContentType string
	Content     []byte
}

type UpsertIdentityProviderInput struct {
	ActorRole           string
	Type                string
	Name                string
	Slug                string
	LogoURL             string
	LoginEnabled        *bool
	RegistrationEnabled *bool
	ClientID            string
	ClientSecret        string
	IssuerURL           string
	DiscoveryURL        string
	AuthURL             string
	TokenURL            string
	UserInfoURL         string
	JWKSURL             string
	Scopes              string
	DefaultRole         string
	SubjectField        string
	EmailField          string
	EmailVerifiedField  string
	NameField           string
	AvatarField         string
}

type oauthTokenResponse struct {
	AccessToken string
	TokenType   string
	IDToken     string
}

type oidcDiscoveryDocument struct {
	AuthorizationEndpoint string
	TokenEndpoint         string
	UserInfoEndpoint      string
}

type githubEmailAddress struct {
	Email    string `json:"email"`
	Primary  bool   `json:"primary"`
	Verified bool   `json:"verified"`
}

type providerOAuthState struct {
	Provider      string `json:"provider"`
	RedirectURI   string `json:"redirectURI"`
	Next          string `json:"next"`
	Intent        string `json:"intent"`
	CodeChallenge string `json:"codeChallenge"`
	Nonce         string `json:"nonce"`
	ExpiresAt     int64  `json:"expiresAt"`
}

func (s *Service) GetLoginOptions(ctx context.Context) (*LoginOptions, error) {
	cfg := s.cfg.Snapshot()
	providerViews := []IdentityProviderView{}
	if cfg.ThirdPartyLoginEnabled {
		providers, err := s.repo.ListIdentityProviders(ctx, false)
		if err != nil {
			return nil, err
		}
		providerViews = toProviderViews(providers, false)
	}
	return &LoginOptions{
		UsernameEnabled:              cfg.UsernameLoginEnabled,
		EmailEnabled:                 cfg.EmailLoginEnabled,
		EmailRegistrationEnabled:     cfg.EmailRegistrationEnabled,
		EmailVerificationEnabled:     cfg.EmailVerificationEnabled,
		PasswordResetEnabled:         passwordResetEnabled(cfg),
		TurnstileRegistrationEnabled: cfg.TurnstileRegistrationEnabled,
		TurnstileSiteKey:             cfg.TurnstileSiteKey,
		Providers:                    providerViews,
	}, nil
}

func (s *Service) ListIdentityProviders(ctx context.Context) ([]IdentityProviderView, error) {
	providers, err := s.repo.ListIdentityProviders(ctx, true)
	if err != nil {
		return nil, err
	}
	return toProviderViews(providers, true), nil
}

func (s *Service) CreateIdentityProvider(ctx context.Context, input UpsertIdentityProviderInput) (*IdentityProviderView, error) {
	provider, err := s.normalizeProviderInput(input, nil)
	if err != nil {
		return nil, err
	}
	provider.PublicID = conv.NormalizePublicID(uuid.NewString())
	created, err := s.repo.CreateIdentityProvider(ctx, provider)
	if err != nil {
		return nil, err
	}
	view := toProviderView(*created, true)
	return &view, nil
}

func (s *Service) UpdateIdentityProvider(ctx context.Context, publicID string, input UpsertIdentityProviderInput) (*IdentityProviderView, error) {
	current, err := s.repo.GetIdentityProviderByPublicID(ctx, publicID)
	if err != nil {
		return nil, err
	}
	normalized, err := s.normalizeProviderInput(input, current)
	if err != nil {
		return nil, err
	}
	updates := providerUpdateInput(normalized)
	updated, err := s.repo.UpdateIdentityProvider(ctx, publicID, updates)
	if err != nil {
		return nil, err
	}
	view := toProviderView(*updated, true)
	return &view, nil
}

func (s *Service) DeleteIdentityProvider(ctx context.Context, publicID string, force bool) error {
	if err := s.repo.DeleteIdentityProvider(ctx, publicID, force); err != nil {
		var dependentErr *repository.IdentityProviderDeleteConflictError
		if errors.As(err, &dependentErr) {
			return &IdentityProviderDeleteConflictError{DependentUsers: dependentErr.DependentUsers}
		}
		if errors.Is(err, repository.ErrConflict) {
			return ErrIdentityProviderDeleteConflict
		}
		return err
	}
	return nil
}

func (s *Service) HasActiveSuperAdminIdentity(ctx context.Context) (bool, error) {
	return s.repo.HasActiveSuperAdminIdentity(ctx)
}

func (s *Service) ListCurrentUserIdentities(ctx context.Context, userID uint) ([]UserIdentityView, error) {
	identities, err := s.repo.ListUserIdentitiesByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	providers, err := s.repo.ListIdentityProviders(ctx, true)
	if err != nil {
		return nil, err
	}
	providerMap := make(map[uint]domainuser.IdentityProvider, len(providers))
	for _, provider := range providers {
		providerMap[provider.ID] = provider
	}
	results := make([]UserIdentityView, 0, len(identities))
	for _, identity := range identities {
		provider := providerMap[identity.ProviderID]
		results = append(results, UserIdentityView{
			ID:                  identity.ID,
			ProviderID:          identity.ProviderID,
			ProviderType:        identity.ProviderType,
			ProviderName:        provider.Name,
			ProviderSlug:        provider.Slug,
			ProviderLogoURL:     provider.LogoURL,
			ProviderDisplayName: identity.ProviderDisplayName,
			Email:               identity.Email,
			EmailVerified:       identity.EmailVerified,
			LinkedAt:            identity.LinkedAt,
			LastLoginAt:         identity.LastLoginAt,
		})
	}
	return results, nil
}

func (s *Service) UnlinkCurrentUserIdentity(ctx context.Context, userID uint, identityID uint) error {
	if err := s.ensureIdentityUnlinkAllowed(ctx, userID, identityID); err != nil {
		return err
	}
	err := s.repo.DeleteUserIdentity(ctx, userID, identityID)
	if errors.Is(err, repository.ErrConflict) {
		return ErrLastLoginMethodNotAllowed
	}
	if errors.Is(err, repository.ErrNotFound) {
		return ErrIdentityNotFound
	}
	return err
}

func (s *Service) ensureIdentityUnlinkAllowed(ctx context.Context, userID uint, identityID uint) error {
	credential, err := s.repo.GetCredentialByUserID(ctx, userID)
	if err != nil {
		return err
	}
	identities, err := s.repo.ListUserIdentitiesByUserID(ctx, userID)
	if err != nil {
		return err
	}
	targetFound := false
	for _, identity := range identities {
		if identity.ID == identityID {
			targetFound = true
			break
		}
	}
	if !targetFound {
		return ErrIdentityNotFound
	}
	if !credential.PasswordEnabled && len(identities) <= 1 {
		return ErrLastLoginMethodNotAllowed
	}
	return nil
}

func (s *Service) ReorderIdentityProviders(ctx context.Context, publicIDs []string) error {
	normalizedIDs := make([]string, 0, len(publicIDs))
	seen := make(map[string]struct{}, len(publicIDs))
	for _, publicID := range publicIDs {
		normalizedID := conv.NormalizePublicID(publicID)
		if normalizedID == "" {
			return fmt.Errorf("provider id is required")
		}
		if _, ok := seen[normalizedID]; ok {
			return fmt.Errorf("provider ids must be unique")
		}
		seen[normalizedID] = struct{}{}
		normalizedIDs = append(normalizedIDs, normalizedID)
	}
	return s.repo.UpdateIdentityProviderSortOrders(ctx, normalizedIDs)
}

func (s *Service) CompleteProviderLogin(
	ctx context.Context,
	slug string,
	code string,
	state string,
	redirectURI string,
	codeVerifier string,
	intent string,
	requestID string,
	auditCtx requestmeta.SessionAuditContext,
) (*LoginResult, error) {
	if !s.cfg.Snapshot().ThirdPartyLoginEnabled {
		return nil, fmt.Errorf("third-party login is disabled")
	}
	provider, err := s.repo.GetIdentityProviderBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	trimmedCode := strings.TrimSpace(code)
	if trimmedCode == "" {
		return nil, fmt.Errorf("authorization code is required")
	}
	verifiedState, err := s.verifyProviderState(slug, redirectURI, state)
	if err != nil {
		return nil, err
	}
	if verifiedState.Intent != normalizeProviderIntent(intent) {
		return nil, fmt.Errorf("oauth intent mismatch")
	}
	if verifiedState.Intent == providerIntentLogin && !provider.LoginEnabled {
		return nil, fmt.Errorf("provider login is disabled")
	}
	if verifiedState.Intent == providerIntentBind {
		return nil, fmt.Errorf("provider bind must use account binding endpoint")
	}
	if verifiedState.Intent == providerIntentRegister {
		if !provider.LoginEnabled || !provider.RegistrationEnabled {
			return nil, fmt.Errorf("provider registration is disabled")
		}
	}
	if err = validateProviderCodeVerifier(codeVerifier, verifiedState.CodeChallenge); err != nil {
		return nil, err
	}

	tokenResponse, err := s.exchangeProviderCode(ctx, *provider, trimmedCode, redirectURI, strings.TrimSpace(codeVerifier))
	if err != nil {
		return nil, err
	}
	profile, err := s.fetchProviderUserInfo(ctx, *provider, tokenResponse.AccessToken)
	if err != nil {
		return nil, err
	}
	profileJSON, _ := json.Marshal(profile)
	subject := claimString(profile, provider.SubjectField)
	if subject == "" {
		return nil, fmt.Errorf("provider subject is missing")
	}
	email, err := normalizeProviderEmail(claimString(profile, provider.EmailField))
	if err != nil {
		return nil, err
	}
	displayName := firstNonEmpty(claimString(profile, provider.NameField), email, subject)
	avatarURL := claimString(profile, provider.AvatarField)
	emailVerified := resolveProviderEmailVerified(profile, *provider)

	userItem, err := s.resolveProviderUser(ctx, *provider, subject, email, displayName, avatarURL, emailVerified, string(profileJSON), verifiedState.Intent)
	if err != nil {
		return nil, err
	}

	normalizedAuditCtx := s.resolveSessionAuditContext(ctx, auditCtx)
	requireTwoFactor, err := s.shouldRequireTwoFactor(ctx, userItem)
	if err != nil {
		return nil, err
	}
	if requireTwoFactor {
		result, challengeErr := s.buildTwoFactorChallenge(ctx, userItem)
		if challengeErr != nil {
			return nil, challengeErr
		}
		s.RecordAuthEvent(
			ctx,
			result.User.ID,
			requestID,
			"provider_login",
			"challenge",
			"two_factor_required",
			normalizedAuditCtx.ClientIP,
			normalizedAuditCtx.UserAgent,
			marshalAuthEventDetail(map[string]interface{}{
				"provider": provider.Slug,
				"subject":  subject,
			}),
		)
		return result, nil
	}
	result, err := s.issueLoginResult(ctx, userItem, normalizedAuditCtx, time.Now())
	if err != nil {
		return nil, err
	}
	s.RecordAuthEvent(
		ctx,
		result.User.ID,
		requestID,
		"provider_login",
		"success",
		"",
		normalizedAuditCtx.ClientIP,
		normalizedAuditCtx.UserAgent,
		marshalAuthEventDetail(map[string]interface{}{
			"provider":   provider.Slug,
			"subject":    subject,
			"session_id": result.SessionID,
		}),
	)
	return result, nil
}

func (s *Service) CompleteProviderBind(
	ctx context.Context,
	userID uint,
	slug string,
	code string,
	state string,
	redirectURI string,
	codeVerifier string,
	requestID string,
	auditCtx requestmeta.SessionAuditContext,
) (*UserIdentityView, error) {
	if userID == 0 {
		return nil, fmt.Errorf("unauthorized")
	}
	if !s.cfg.Snapshot().ThirdPartyLoginEnabled {
		return nil, fmt.Errorf("third-party login is disabled")
	}
	provider, err := s.repo.GetIdentityProviderBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	if !provider.LoginEnabled {
		return nil, fmt.Errorf("provider login is disabled")
	}
	trimmedCode := strings.TrimSpace(code)
	if trimmedCode == "" {
		return nil, fmt.Errorf("authorization code is required")
	}
	verifiedState, err := s.verifyProviderState(slug, redirectURI, state)
	if err != nil {
		return nil, err
	}
	if verifiedState.Intent != providerIntentBind {
		return nil, fmt.Errorf("oauth intent mismatch")
	}
	if err = validateProviderCodeVerifier(codeVerifier, verifiedState.CodeChallenge); err != nil {
		return nil, err
	}

	tokenResponse, err := s.exchangeProviderCode(ctx, *provider, trimmedCode, redirectURI, strings.TrimSpace(codeVerifier))
	if err != nil {
		return nil, err
	}
	profile, err := s.fetchProviderUserInfo(ctx, *provider, tokenResponse.AccessToken)
	if err != nil {
		return nil, err
	}
	profileJSON, _ := json.Marshal(profile)
	subject := claimString(profile, provider.SubjectField)
	if subject == "" {
		return nil, fmt.Errorf("provider subject is missing")
	}
	normalizedEmail, err := normalizeProviderEmail(claimString(profile, provider.EmailField))
	if err != nil {
		return nil, err
	}
	providerDisplayName := firstNonEmpty(claimString(profile, provider.NameField), normalizedEmail, subject)
	emailVerified := resolveProviderEmailVerified(profile, *provider)
	now := time.Now()

	existingIdentity, err := s.repo.GetUserIdentityByProviderSubject(ctx, provider.ID, subject)
	if err == nil {
		if existingIdentity.UserID != userID {
			return nil, fmt.Errorf("provider identity is already bound to another account")
		}
		if err = s.repo.UpdateUserIdentityLogin(ctx, existingIdentity.ID, string(profileJSON), providerDisplayName, normalizedEmail, emailVerified); err != nil {
			return nil, err
		}
		return &UserIdentityView{
			ID:                  existingIdentity.ID,
			ProviderID:          provider.ID,
			ProviderType:        provider.Type,
			ProviderName:        provider.Name,
			ProviderSlug:        provider.Slug,
			ProviderLogoURL:     provider.LogoURL,
			ProviderDisplayName: providerDisplayName,
			Email:               normalizedEmail,
			EmailVerified:       emailVerified,
			LinkedAt:            existingIdentity.LinkedAt,
			LastLoginAt:         &now,
		}, nil
	}
	if !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}
	if normalizedEmail != "" {
		existingUser, findErr := s.repo.GetByEmail(ctx, normalizedEmail)
		if findErr == nil && existingUser.ID != userID {
			return nil, fmt.Errorf("provider email belongs to another account; sign in to that account or change its email before binding")
		}
		if findErr != nil && !errors.Is(findErr, repository.ErrNotFound) {
			return nil, findErr
		}
	}
	currentIdentities, err := s.repo.ListUserIdentitiesByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	for _, identity := range currentIdentities {
		if identity.ProviderID == provider.ID {
			return nil, fmt.Errorf("provider is already bound")
		}
	}

	created, err := s.createProviderIdentity(ctx, userID, *provider, subject, providerDisplayName, normalizedEmail, emailVerified, string(profileJSON), now)
	if err != nil {
		return nil, err
	}
	normalizedAuditCtx := s.resolveSessionAuditContext(ctx, auditCtx)
	s.RecordAuthEvent(
		ctx,
		userID,
		requestID,
		"provider_bind",
		"success",
		"",
		normalizedAuditCtx.ClientIP,
		normalizedAuditCtx.UserAgent,
		marshalAuthEventDetail(map[string]interface{}{
			"provider": provider.Slug,
			"subject":  subject,
			"email":    normalizedEmail,
		}),
	)
	return &UserIdentityView{
		ID:                  created.ID,
		ProviderID:          provider.ID,
		ProviderType:        provider.Type,
		ProviderName:        provider.Name,
		ProviderSlug:        provider.Slug,
		ProviderLogoURL:     provider.LogoURL,
		ProviderDisplayName: created.ProviderDisplayName,
		Email:               created.Email,
		EmailVerified:       created.EmailVerified,
		LinkedAt:            created.LinkedAt,
		LastLoginAt:         created.LastLoginAt,
	}, nil
}

func (s *Service) normalizeProviderInput(input UpsertIdentityProviderInput, current *domainuser.IdentityProvider) (*domainuser.IdentityProvider, error) {
	providerType := strings.ToLower(strings.TrimSpace(input.Type))
	if providerType != domainuser.IdentityProviderTypeOIDC && providerType != domainuser.IdentityProviderTypeOAuth2 {
		return nil, fmt.Errorf("provider type must be oidc or oauth2")
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, fmt.Errorf("provider name is required")
	}
	slug := normalizeProviderSlug(input.Slug)
	if slug == "" {
		slug = normalizeProviderSlug(name)
	}
	if slug == "" {
		return nil, fmt.Errorf("provider slug is required")
	}
	scopes := strings.TrimSpace(input.Scopes)
	if scopes == "" && providerType == domainuser.IdentityProviderTypeOIDC {
		scopes = "openid profile email"
	}
	if scopes == "" {
		scopes = "profile email"
	}
	defaultRole := strings.TrimSpace(input.DefaultRole)
	if defaultRole == "" {
		defaultRole = domainuser.RoleUser
	}
	if defaultRole != domainuser.RoleUser && defaultRole != domainuser.RoleAdmin && defaultRole != domainuser.RoleSuperAdmin {
		return nil, fmt.Errorf("default role must be user, admin or superadmin")
	}
	if defaultRole == domainuser.RoleSuperAdmin && input.ActorRole != domainuser.RoleSuperAdmin {
		return nil, ErrIdentityProviderSuperAdminDefaultRoleNotAllowed
	}
	logoURL := strings.TrimSpace(input.LogoURL)
	if logoURL != "" && !isValidProviderLogoURL(logoURL) {
		return nil, fmt.Errorf("logo url must be a valid http(s) or absolute path")
	}
	provider := &domainuser.IdentityProvider{
		Type:                providerType,
		Name:                name,
		Slug:                slug,
		LogoURL:             logoURL,
		LoginEnabled:        boolValue(input.LoginEnabled, true),
		RegistrationEnabled: boolValue(input.RegistrationEnabled, true),
		ClientID:            strings.TrimSpace(input.ClientID),
		IssuerURL:           strings.TrimSpace(input.IssuerURL),
		DiscoveryURL:        strings.TrimSpace(input.DiscoveryURL),
		AuthURL:             strings.TrimSpace(input.AuthURL),
		TokenURL:            strings.TrimSpace(input.TokenURL),
		UserInfoURL:         strings.TrimSpace(input.UserInfoURL),
		JWKSURL:             strings.TrimSpace(input.JWKSURL),
		Scopes:              scopes,
		PKCEEnabled:         true,
		DefaultRole:         defaultRole,
		SubjectField:        firstNonEmpty(strings.TrimSpace(input.SubjectField), "sub"),
		EmailField:          firstNonEmpty(strings.TrimSpace(input.EmailField), "email"),
		EmailVerifiedField:  firstNonEmpty(strings.TrimSpace(input.EmailVerifiedField), "email_verified"),
		NameField:           firstNonEmpty(strings.TrimSpace(input.NameField), "name"),
		AvatarField:         firstNonEmpty(strings.TrimSpace(input.AvatarField), "picture"),
		SortOrder:           100,
	}
	if provider.RegistrationEnabled && !provider.LoginEnabled {
		return nil, fmt.Errorf("provider registration requires provider login to be enabled")
	}
	if current != nil {
		provider.PublicID = current.PublicID
		provider.ClientSecret = current.ClientSecret
	}
	if strings.TrimSpace(input.ClientSecret) != "" {
		encrypted, err := secretbox.EncryptString(s.cfg.Snapshot().DataEncryptionKey, strings.TrimSpace(input.ClientSecret))
		if err != nil {
			return nil, err
		}
		provider.ClientSecret = encrypted
	}
	if provider.ClientID == "" {
		return nil, fmt.Errorf("client id is required")
	}
	if provider.ClientSecret == "" {
		return nil, fmt.Errorf("client secret is required")
	}
	if providerType == domainuser.IdentityProviderTypeOIDC {
		if provider.IssuerURL == "" && provider.DiscoveryURL == "" {
			return nil, fmt.Errorf("OIDC issuer url or discovery url is required")
		}
	} else if provider.AuthURL == "" || provider.TokenURL == "" || provider.UserInfoURL == "" {
		return nil, fmt.Errorf("OAuth2 auth url, token url and userinfo url are required")
	}
	return provider, nil
}

func isValidProviderLogoURL(value string) bool {
	parsedLogoURL, err := url.Parse(value)
	if err != nil {
		return false
	}
	if (parsedLogoURL.Scheme == "http" || parsedLogoURL.Scheme == "https") && parsedLogoURL.Host != "" {
		return true
	}
	return parsedLogoURL.Scheme == "" &&
		parsedLogoURL.Host == "" &&
		strings.HasPrefix(value, "/") &&
		!strings.HasPrefix(value, "//") &&
		!strings.Contains(value, "\\")
}

func toProviderViews(items []domainuser.IdentityProvider, includeSensitive bool) []IdentityProviderView {
	results := make([]IdentityProviderView, 0, len(items))
	for _, item := range items {
		results = append(results, toProviderView(item, includeSensitive))
	}
	return results
}

func toProviderView(item domainuser.IdentityProvider, includeSensitive bool) IdentityProviderView {
	clientID := ""
	if includeSensitive {
		clientID = item.ClientID
	}
	return IdentityProviderView{
		PublicID:            item.PublicID,
		Type:                item.Type,
		Name:                item.Name,
		Slug:                item.Slug,
		LogoURL:             item.LogoURL,
		LoginEnabled:        item.LoginEnabled,
		RegistrationEnabled: item.RegistrationEnabled,
		ClientID:            clientID,
		IssuerURL:           item.IssuerURL,
		DiscoveryURL:        item.DiscoveryURL,
		AuthURL:             item.AuthURL,
		TokenURL:            item.TokenURL,
		UserInfoURL:         item.UserInfoURL,
		JWKSURL:             item.JWKSURL,
		Scopes:              item.Scopes,
		DefaultRole:         item.DefaultRole,
		SubjectField:        item.SubjectField,
		EmailField:          item.EmailField,
		EmailVerifiedField:  item.EmailVerifiedField,
		NameField:           item.NameField,
		AvatarField:         item.AvatarField,
		CreatedAt:           item.CreatedAt,
		UpdatedAt:           item.UpdatedAt,
	}
}

func providerUpdateInput(provider *domainuser.IdentityProvider) repository.UpdateIdentityProviderInput {
	pkceEnabled := true
	return repository.UpdateIdentityProviderInput{
		Type:                &provider.Type,
		Name:                &provider.Name,
		Slug:                &provider.Slug,
		LogoURL:             &provider.LogoURL,
		LoginEnabled:        &provider.LoginEnabled,
		RegistrationEnabled: &provider.RegistrationEnabled,
		ClientID:            &provider.ClientID,
		ClientSecret:        &provider.ClientSecret,
		IssuerURL:           &provider.IssuerURL,
		DiscoveryURL:        &provider.DiscoveryURL,
		AuthURL:             &provider.AuthURL,
		TokenURL:            &provider.TokenURL,
		UserInfoURL:         &provider.UserInfoURL,
		JWKSURL:             &provider.JWKSURL,
		Scopes:              &provider.Scopes,
		PKCEEnabled:         &pkceEnabled,
		DefaultRole:         &provider.DefaultRole,
		SubjectField:        &provider.SubjectField,
		EmailField:          &provider.EmailField,
		EmailVerifiedField:  &provider.EmailVerifiedField,
		NameField:           &provider.NameField,
		AvatarField:         &provider.AvatarField,
	}
}

var providerSlugPattern = regexp.MustCompile(`[^a-z0-9_-]+`)

const (
	providerIntentLogin    = "login"
	providerIntentRegister = "register"
	providerIntentBind     = "bind"
	providerHTTPTimeout    = 10 * time.Second
	providerLogoMaxBytes   = 2 << 20
)

func normalizeProviderSlug(value string) string {
	slug := strings.ToLower(strings.TrimSpace(value))
	slug = strings.ReplaceAll(slug, " ", "-")
	slug = providerSlugPattern.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-_")
	return slug
}

func normalizeProviderIntent(value string) string {
	switch strings.TrimSpace(value) {
	case providerIntentRegister:
		return providerIntentRegister
	case providerIntentBind:
		return providerIntentBind
	default:
		return providerIntentLogin
	}
}

func (s *Service) GetIdentityProviderLogo(ctx context.Context, slug string) (*IdentityProviderLogoAsset, error) {
	provider, err := s.repo.GetIdentityProviderBySlug(ctx, normalizeProviderSlug(slug))
	if err != nil {
		return nil, ErrIdentityProviderLogoUnavailable
	}
	logoURL := strings.TrimSpace(provider.LogoURL)
	parsed, err := url.Parse(logoURL)
	if err != nil || parsed == nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, ErrIdentityProviderLogoUnavailable
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8")

	response, err := s.providerHTTPClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, ErrIdentityProviderLogoUnavailable
	}

	content, err := io.ReadAll(io.LimitReader(response.Body, providerLogoMaxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(content) == 0 || len(content) > providerLogoMaxBytes {
		return nil, ErrIdentityProviderLogoUnavailable
	}
	contentType := resolveIdentityProviderLogoContentType(response.Header.Get("Content-Type"), parsed.Path, content)
	if contentType == "" {
		return nil, ErrIdentityProviderLogoUnavailable
	}
	return &IdentityProviderLogoAsset{
		ContentType: contentType,
		Content:     content,
	}, nil
}

func resolveIdentityProviderLogoContentType(headerValue string, requestPath string, content []byte) string {
	contentType := normalizeIdentityProviderLogoContentType(headerValue)
	if isAllowedIdentityProviderLogoContentType(contentType) {
		return contentType
	}
	if contentType == "" || contentType == "application/octet-stream" {
		contentType = normalizeIdentityProviderLogoContentType(http.DetectContentType(content))
		if isAllowedIdentityProviderLogoContentType(contentType) {
			return contentType
		}
		contentType = providerLogoContentTypeByExtension(requestPath)
		if contentType == "image/svg+xml" && !looksLikeSVG(content) {
			return ""
		}
		if isAllowedIdentityProviderLogoContentType(contentType) {
			return contentType
		}
	}
	return ""
}

func normalizeIdentityProviderLogoContentType(value string) string {
	contentType, _, err := mime.ParseMediaType(strings.TrimSpace(value))
	if err != nil {
		contentType = strings.TrimSpace(value)
	}
	return strings.ToLower(contentType)
}

func isAllowedIdentityProviderLogoContentType(contentType string) bool {
	switch contentType {
	case "image/avif",
		"image/gif",
		"image/jpeg",
		"image/png",
		"image/svg+xml",
		"image/vnd.microsoft.icon",
		"image/webp",
		"image/x-icon":
		return true
	default:
		return false
	}
}

func providerLogoContentTypeByExtension(requestPath string) string {
	switch strings.ToLower(path.Ext(requestPath)) {
	case ".avif":
		return "image/avif"
	case ".gif":
		return "image/gif"
	case ".ico":
		return "image/x-icon"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".svg":
		return "image/svg+xml"
	case ".webp":
		return "image/webp"
	default:
		return ""
	}
}

func looksLikeSVG(content []byte) bool {
	trimmed := bytes.TrimSpace(content)
	if len(trimmed) == 0 {
		return false
	}
	lowered := bytes.ToLower(trimmed)
	return bytes.HasPrefix(lowered, []byte("<svg")) || (bytes.HasPrefix(lowered, []byte("<?xml")) && bytes.Contains(lowered, []byte("<svg")))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func boolValue(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func buildProviderAuthURL(provider domainuser.IdentityProvider, authURL string, redirectURI string, state string, codeChallenge string) (string, error) {
	if authURL == "" {
		return "", fmt.Errorf("provider auth url is not configured")
	}
	parsed, err := url.Parse(authURL)
	if err != nil {
		return "", err
	}
	values := parsed.Query()
	values.Set("client_id", provider.ClientID)
	values.Set("redirect_uri", redirectURI)
	values.Set("response_type", "code")
	values.Set("scope", provider.Scopes)
	values.Set("state", state)
	if strings.TrimSpace(codeChallenge) != "" {
		values.Set("code_challenge", strings.TrimSpace(codeChallenge))
		values.Set("code_challenge_method", "S256")
	}
	parsed.RawQuery = values.Encode()
	return parsed.String(), nil
}

func (s *Service) BuildProviderAuthURL(ctx context.Context, slug string, redirectURI string, nextPath string, codeChallenge string, intent string) (string, error) {
	if !s.cfg.Snapshot().ThirdPartyLoginEnabled {
		return "", fmt.Errorf("third-party login is disabled")
	}
	if err := s.validateProviderRedirectURI(slug, redirectURI); err != nil {
		return "", err
	}
	if err := validateProviderCodeChallenge(codeChallenge); err != nil {
		return "", err
	}
	provider, err := s.repo.GetIdentityProviderBySlug(ctx, slug)
	if err != nil {
		return "", err
	}
	normalizedIntent := normalizeProviderIntent(intent)
	if normalizedIntent == providerIntentLogin && !provider.LoginEnabled {
		return "", fmt.Errorf("provider login is disabled")
	}
	if normalizedIntent == providerIntentBind && !provider.LoginEnabled {
		return "", fmt.Errorf("provider login is disabled")
	}
	if normalizedIntent == providerIntentRegister {
		if !provider.LoginEnabled || !provider.RegistrationEnabled {
			return "", fmt.Errorf("provider registration is disabled")
		}
	}
	authURL, _, _, err := s.resolveProviderEndpoints(ctx, *provider)
	if err != nil {
		return "", err
	}
	state, err := s.signProviderState(providerOAuthState{
		Provider:      slug,
		RedirectURI:   redirectURI,
		Next:          normalizeProviderNextPath(nextPath),
		Intent:        normalizedIntent,
		CodeChallenge: strings.TrimSpace(codeChallenge),
		Nonce:         conv.NormalizePublicID(uuid.NewString()),
		ExpiresAt:     time.Now().Add(10 * time.Minute).Unix(),
	})
	if err != nil {
		return "", err
	}
	return buildProviderAuthURL(*provider, authURL, redirectURI, state, codeChallenge)
}

func (s *Service) exchangeProviderCode(ctx context.Context, provider domainuser.IdentityProvider, code string, redirectURI string, codeVerifier string) (*oauthTokenResponse, error) {
	_, tokenURL, _, err := s.resolveProviderEndpoints(ctx, provider)
	if err != nil {
		return nil, err
	}
	clientSecret, err := secretbox.DecryptString(s.cfg.Snapshot().DataEncryptionKey, provider.ClientSecret)
	if err != nil {
		return nil, err
	}
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)
	form.Set("client_id", provider.ClientID)
	form.Set("client_secret", clientSecret)
	if strings.TrimSpace(codeVerifier) != "" {
		form.Set("code_verifier", codeVerifier)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")
	response, err := s.providerHTTPClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("provider token exchange failed: %s", response.Status)
	}
	var tokenResponse oauthTokenResponse
	if tokenResponse, err = parseOAuthTokenResponse(body); err != nil {
		return nil, err
	}
	if strings.TrimSpace(tokenResponse.AccessToken) == "" {
		return nil, fmt.Errorf("provider token response missing access token")
	}
	return &tokenResponse, nil
}

func (s *Service) fetchProviderUserInfo(ctx context.Context, provider domainuser.IdentityProvider, accessToken string) (map[string]interface{}, error) {
	_, _, userInfoURL, err := s.resolveProviderEndpoints(ctx, provider)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, userInfoURL, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", "Bearer "+accessToken)
	response, err := s.providerHTTPClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("provider userinfo failed: %s", response.Status)
	}
	var profile map[string]interface{}
	if err = json.Unmarshal(body, &profile); err != nil {
		return nil, err
	}
	if githubEmailsURL, ok := githubEmailsEndpoint(provider, userInfoURL); ok {
		if err = s.enrichGitHubVerifiedEmail(ctx, accessToken, profile, githubEmailsURL); err != nil {
			return nil, err
		}
	}
	return profile, nil
}

func (s *Service) enrichGitHubVerifiedEmail(ctx context.Context, accessToken string, profile map[string]interface{}, emailsURL string) error {
	if strings.TrimSpace(accessToken) == "" || strings.TrimSpace(emailsURL) == "" {
		return nil
	}
	existingEmail, _ := normalizeProviderEmail(claimString(profile, "email"))
	if existingEmail != "" && resolveProviderEmailVerified(profile, domainuser.IdentityProvider{EmailVerifiedField: "email_verified"}) {
		return nil
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, emailsURL, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("Authorization", "Bearer "+accessToken)
	response, err := s.providerHTTPClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("github provider emails failed: %s", response.Status)
	}
	var emails []githubEmailAddress
	if err = json.Unmarshal(body, &emails); err != nil {
		return err
	}
	verifiedEmail := selectGitHubVerifiedEmail(existingEmail, emails)
	if verifiedEmail == "" {
		return nil
	}
	profile["email"] = verifiedEmail
	profile["email_verified"] = true
	profile["verified_email"] = true
	return nil
}

func githubEmailsEndpoint(provider domainuser.IdentityProvider, userInfoURL string) (string, bool) {
	if !isGitHubProvider(provider, userInfoURL) {
		return "", false
	}
	parsed, err := url.Parse(strings.TrimSpace(userInfoURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", false
	}
	pathValue := strings.TrimRight(parsed.Path, "/")
	if !strings.HasSuffix(pathValue, "/user") {
		return "", false
	}
	parsed.Path = pathValue + "/emails"
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), true
}

func isGitHubProvider(provider domainuser.IdentityProvider, userInfoURL string) bool {
	if normalizeProviderSlug(provider.Slug) == "github" || normalizeProviderSlug(provider.Name) == "github" {
		return true
	}
	parsed, err := url.Parse(strings.TrimSpace(userInfoURL))
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	return host == "api.github.com" || strings.HasSuffix(host, ".github.com")
}

func selectGitHubVerifiedEmail(existingEmail string, emails []githubEmailAddress) string {
	normalizedExistingEmail := strings.ToLower(strings.TrimSpace(existingEmail))
	firstVerified := ""
	for _, item := range emails {
		if !item.Verified {
			continue
		}
		normalizedEmail, err := normalizeProviderEmail(item.Email)
		if err != nil || normalizedEmail == "" {
			continue
		}
		if normalizedExistingEmail != "" && normalizedEmail == normalizedExistingEmail {
			return normalizedEmail
		}
		if item.Primary {
			return normalizedEmail
		}
		if firstVerified == "" {
			firstVerified = normalizedEmail
		}
	}
	return firstVerified
}

func (s *Service) resolveProviderEndpoints(ctx context.Context, provider domainuser.IdentityProvider) (string, string, string, error) {
	authURL := strings.TrimSpace(provider.AuthURL)
	tokenURL := strings.TrimSpace(provider.TokenURL)
	userInfoURL := strings.TrimSpace(provider.UserInfoURL)
	if authURL != "" && tokenURL != "" && userInfoURL != "" {
		return authURL, tokenURL, userInfoURL, nil
	}
	if provider.Type != domainuser.IdentityProviderTypeOIDC {
		return authURL, tokenURL, userInfoURL, nil
	}
	discoveryURL := strings.TrimSpace(provider.DiscoveryURL)
	if discoveryURL == "" && strings.TrimSpace(provider.IssuerURL) != "" {
		discoveryURL = strings.TrimRight(strings.TrimSpace(provider.IssuerURL), "/") + "/.well-known/openid-configuration"
	}
	if discoveryURL == "" {
		return authURL, tokenURL, userInfoURL, nil
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, discoveryURL, nil)
	if err != nil {
		return "", "", "", err
	}
	request.Header.Set("Accept", "application/json")
	response, err := s.providerHTTPClient.Do(request)
	if err != nil {
		return "", "", "", err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", "", "", fmt.Errorf("provider discovery failed: %s", response.Status)
	}
	metadata, err := parseOIDCDiscoveryDocument(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return "", "", "", err
	}
	return firstNonEmpty(authURL, metadata.AuthorizationEndpoint),
		firstNonEmpty(tokenURL, metadata.TokenEndpoint),
		firstNonEmpty(userInfoURL, metadata.UserInfoEndpoint),
		nil
}

func parseOAuthTokenResponse(raw []byte) (oauthTokenResponse, error) {
	var payload map[string]interface{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return oauthTokenResponse{}, err
	}
	return oauthTokenResponse{
		AccessToken: claimString(payload, "access_token"),
		TokenType:   claimString(payload, "token_type"),
		IDToken:     claimString(payload, "id_token"),
	}, nil
}

func parseOIDCDiscoveryDocument(reader io.Reader) (oidcDiscoveryDocument, error) {
	var payload map[string]interface{}
	if err := json.NewDecoder(reader).Decode(&payload); err != nil {
		return oidcDiscoveryDocument{}, err
	}
	return oidcDiscoveryDocument{
		AuthorizationEndpoint: claimString(payload, "authorization_endpoint"),
		TokenEndpoint:         claimString(payload, "token_endpoint"),
		UserInfoEndpoint:      claimString(payload, "userinfo_endpoint"),
	}, nil
}

func (s *Service) resolveProviderUser(ctx context.Context, provider domainuser.IdentityProvider, subject string, email string, displayName string, avatarURL string, emailVerified bool, profileJSON string, intent string) (*domainuser.User, error) {
	identity, err := s.repo.GetUserIdentityByProviderSubject(ctx, provider.ID, subject)
	if err == nil {
		if !provider.LoginEnabled {
			return nil, fmt.Errorf("provider login is disabled")
		}
		userItem, getErr := s.repo.GetByID(ctx, identity.UserID)
		if getErr != nil {
			return nil, getErr
		}
		if err = ensureProviderLoginUserActive(userItem); err != nil {
			return nil, err
		}
		if updateErr := s.repo.UpdateUserIdentityLogin(ctx, identity.ID, profileJSON, displayName, strings.TrimSpace(email), emailVerified); updateErr != nil {
			return nil, updateErr
		}
		return userItem, nil
	}
	if !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}

	cfg := s.cfg.Snapshot()
	now := time.Now()
	normalizedEmail, err := normalizeProviderEmail(email)
	if err != nil {
		return nil, err
	}
	if cfg.AutoLinkVerifiedEmail && emailVerified && normalizedEmail != "" {
		existingUser, findErr := s.repo.GetByEmail(ctx, normalizedEmail)
		if findErr == nil {
			if err = ensureProviderLoginUserActive(existingUser); err != nil {
				return nil, err
			}
			if _, createErr := s.createProviderIdentity(ctx, existingUser.ID, provider, subject, displayName, normalizedEmail, emailVerified, profileJSON, now); createErr != nil {
				return nil, createErr
			}
			return existingUser, nil
		}
		if !errors.Is(findErr, repository.ErrNotFound) {
			return nil, findErr
		}
	} else if normalizedEmail != "" {
		if _, findErr := s.repo.GetByEmail(ctx, normalizedEmail); findErr == nil {
			return nil, &ProviderEmailConflictError{
				ProviderSlug: provider.Slug,
				Email:        normalizedEmail,
				Action:       ProviderEmailConflictActionSignInThenBind,
			}
		} else if !errors.Is(findErr, repository.ErrNotFound) {
			return nil, findErr
		}
	}
	if !provider.RegistrationEnabled {
		return nil, fmt.Errorf("provider account is not registered")
	}

	emailVerifiedAt := (*time.Time)(nil)
	emailSource := domainuser.EmailSourceProviderUnverified
	if emailVerified && normalizedEmail != "" {
		emailVerifiedAt = &now
		emailSource = domainuser.EmailSourceProviderVerified
	}
	userItem := &domainuser.User{
		PublicID:        conv.NormalizePublicID(uuid.NewString()),
		Username:        providerUsername(provider.Slug, subject),
		DisplayName:     userapp.NormalizeGeneratedDisplayName(firstNonEmpty(displayName, provider.Name+" 用户")),
		AvatarURL:       strings.TrimSpace(avatarURL),
		Email:           normalizedEmail,
		EmailSource:     emailSource,
		Role:            firstNonEmpty(provider.DefaultRole, domainuser.RoleUser),
		Status:          domainuser.StatusActive,
		Timezone:        "Etc/UTC",
		Locale:          "en-US",
		EmailVerifiedAt: emailVerifiedAt,
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(uuid.NewString()), passwordHashCost)
	if err != nil {
		return nil, err
	}
	providerIdentity := s.newProviderIdentity(userItem.ID, provider, subject, displayName, normalizedEmail, emailVerified, profileJSON, now)
	if err = s.createWithCredentialAndIdentityUsingAvailableUsername(ctx, userItem, domainuser.Credential{
		PasswordHash:      string(passwordHash),
		PasswordAlgo:      "bcrypt",
		PasswordEnabled:   false,
		PasswordUpdatedAt: &now,
		PasswordOrigin:    domainuser.PasswordOriginSSOPlaceholder,
	}, providerIdentity, 0, 0, nil, false); err != nil {
		return nil, err
	}
	return userItem, nil
}

func ensureProviderLoginUserActive(item *domainuser.User) error {
	if item == nil {
		return ErrInvalidCredentials
	}
	if item.Status == domainuser.StatusLocked {
		return ErrAccountLocked
	}
	if item.Status != domainuser.StatusActive {
		return ErrInvalidCredentials
	}
	return nil
}

func (s *Service) createProviderIdentity(ctx context.Context, userID uint, provider domainuser.IdentityProvider, subject string, providerDisplayName string, email string, emailVerified bool, profileJSON string, now time.Time) (*domainuser.UserIdentity, error) {
	return s.repo.CreateUserIdentity(ctx, s.newProviderIdentity(userID, provider, subject, providerDisplayName, email, emailVerified, profileJSON, now))
}

func (s *Service) newProviderIdentity(userID uint, provider domainuser.IdentityProvider, subject string, providerDisplayName string, email string, emailVerified bool, profileJSON string, now time.Time) *domainuser.UserIdentity {
	return &domainuser.UserIdentity{
		UserID:              userID,
		ProviderID:          provider.ID,
		ProviderType:        provider.Type,
		ProviderSubject:     strings.TrimSpace(subject),
		ProviderDisplayName: strings.TrimSpace(providerDisplayName),
		Email:               strings.TrimSpace(email),
		EmailVerified:       emailVerified,
		ProfileJSON:         profileJSON,
		LinkedAt:            now,
		LastLoginAt:         &now,
	}
}

func (s *Service) createWithCredentialAndIdentityUsingAvailableUsername(
	ctx context.Context,
	userItem *domainuser.User,
	credential domainuser.Credential,
	identity *domainuser.UserIdentity,
	subscriptionPlanID uint,
	subscriptionPriceID uint,
	subscriptionEndAt *time.Time,
	autoRenew bool,
) error {
	baseUsername := userItem.Username
	for attempt := 0; attempt < 20; attempt++ {
		userItem.ID = 0
		userItem.Username = generatedUsernameWithSuffix(baseUsername, attempt)
		err := s.repo.CreateWithCredentialAndIdentity(ctx, userItem, credential, identity, subscriptionPlanID, subscriptionPriceID, subscriptionEndAt, autoRenew)
		if errors.Is(err, repository.ErrDuplicateUsername) {
			continue
		}
		return err
	}
	return ErrUsernameTaken
}

func (s *Service) signProviderState(state providerOAuthState) (string, error) {
	payload, err := json.Marshal(state)
	if err != nil {
		return "", err
	}
	encodedPayload := base64.RawURLEncoding.EncodeToString(payload)
	signature := providerStateSignature(s.cfg.Snapshot().JWTSecret, encodedPayload)
	return encodedPayload + "." + signature, nil
}

func (s *Service) verifyProviderState(slug string, redirectURI string, rawState string) (*providerOAuthState, error) {
	parts := strings.Split(strings.TrimSpace(rawState), ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil, fmt.Errorf("invalid oauth state")
	}
	expected := providerStateSignature(s.cfg.Snapshot().JWTSecret, parts[0])
	if !hmac.Equal([]byte(expected), []byte(parts[1])) {
		return nil, fmt.Errorf("invalid oauth state")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid oauth state")
	}
	var state providerOAuthState
	if err = json.Unmarshal(payload, &state); err != nil {
		return nil, fmt.Errorf("invalid oauth state")
	}
	if state.Provider != slug || state.RedirectURI != redirectURI {
		return nil, fmt.Errorf("oauth state mismatch")
	}
	if time.Now().Unix() > state.ExpiresAt {
		return nil, fmt.Errorf("oauth state expired")
	}
	if err = s.validateProviderRedirectURI(slug, redirectURI); err != nil {
		return nil, err
	}
	return &state, nil
}

func providerStateSignature(secret string, encodedPayload string) string {
	mac := hmac.New(sha256.New, []byte(strings.TrimSpace(secret)))
	mac.Write([]byte(encodedPayload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func providerCodeChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

var providerPKCEPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{43,128}$`)

func validateProviderCodeChallenge(codeChallenge string) error {
	if !providerPKCEPattern.MatchString(strings.TrimSpace(codeChallenge)) {
		return fmt.Errorf("valid pkce code challenge is required")
	}
	return nil
}

func validateProviderCodeVerifier(codeVerifier string, expectedChallenge string) error {
	trimmedVerifier := strings.TrimSpace(codeVerifier)
	if !providerPKCEPattern.MatchString(trimmedVerifier) {
		return fmt.Errorf("valid pkce code verifier is required")
	}
	if !hmac.Equal([]byte(providerCodeChallenge(trimmedVerifier)), []byte(strings.TrimSpace(expectedChallenge))) {
		return fmt.Errorf("pkce code verifier mismatch")
	}
	return nil
}

func (s *Service) validateProviderRedirectURI(slug string, redirectURI string) error {
	parsed, err := url.Parse(strings.TrimSpace(redirectURI))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return fmt.Errorf("invalid redirect uri")
	}
	if parsed.Path != "/auth/callback" || parsed.Query().Get("provider") != slug {
		return fmt.Errorf("invalid redirect uri")
	}
	if s.isAllowedProviderRedirectOrigin(parsed) {
		return nil
	}
	return fmt.Errorf("redirect uri origin is not allowed")
}

func (s *Service) isAllowedProviderRedirectOrigin(parsed *url.URL) bool {
	cfg := s.cfg.Snapshot()
	if cfg.Env != "prod" && isLoopbackHost(parsed.Hostname()) {
		return true
	}
	origin := parsed.Scheme + "://" + parsed.Host
	for _, allowed := range strings.Split(cfg.CORSAllowOrigin, ",") {
		trimmed := strings.TrimRight(strings.TrimSpace(allowed), "/")
		if trimmed != "" && trimmed != "*" && trimmed == origin {
			return true
		}
	}
	return false
}

func isLoopbackHost(host string) bool {
	normalized := strings.ToLower(strings.Trim(host, "[]"))
	return normalized == "localhost" || normalized == "127.0.0.1" || normalized == "::1"
}

func normalizeProviderNextPath(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || !strings.HasPrefix(trimmed, "/") || strings.HasPrefix(trimmed, "//") {
		return "/chat"
	}
	return trimmed
}

func providerUsername(slug string, subject string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(slug) + ":" + strings.TrimSpace(subject)))
	prefix := normalizeProviderSlug(slug)
	if prefix == "" {
		prefix = "oauth"
	}
	suffix := hex.EncodeToString(sum[:])[:8]
	maxPrefixLength := userapp.UsernameMaxLength - len(suffix) - 1
	if len(prefix) > maxPrefixLength {
		prefix = strings.Trim(prefix[:maxPrefixLength], "-_")
	}
	if prefix == "" {
		prefix = "oauth"
	}
	return prefix + "-" + suffix
}

func claimString(profile map[string]interface{}, field string) string {
	value, ok := claimValue(profile, field)
	if !ok {
		return ""
	}
	return conv.GetStringFromAny(value)
}

func normalizeProviderEmail(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil
	}
	normalized, err := normalizeRegistrationEmail(trimmed)
	if err != nil {
		return "", fmt.Errorf("provider email is invalid")
	}
	return normalized, nil
}

func resolveProviderEmailVerified(profile map[string]interface{}, provider domainuser.IdentityProvider) bool {
	fields := make([]string, 0, 3)
	if strings.TrimSpace(provider.EmailVerifiedField) != "" {
		fields = append(fields, provider.EmailVerifiedField)
	}
	fields = append(fields, "email_verified", "verified_email")
	fields = append(fields, providerSpecificEmailVerifiedFields(provider)...)
	return claimBool(profile, uniqueClaimFields(fields)...)
}

func providerSpecificEmailVerifiedFields(provider domainuser.IdentityProvider) []string {
	if isDiscordProvider(provider, "") {
		return []string{"verified"}
	}
	return nil
}

func isDiscordProvider(provider domainuser.IdentityProvider, userInfoURL string) bool {
	if normalizeProviderSlug(provider.Slug) == "discord" || normalizeProviderSlug(provider.Name) == "discord" {
		return true
	}
	parsed, err := url.Parse(strings.TrimSpace(userInfoURL))
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	return host == "discord.com" || strings.HasSuffix(host, ".discord.com") || host == "discordapp.com" || strings.HasSuffix(host, ".discordapp.com")
}

func uniqueClaimFields(fields []string) []string {
	seen := make(map[string]struct{}, len(fields))
	results := make([]string, 0, len(fields))
	for _, field := range fields {
		normalized := strings.TrimSpace(field)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		results = append(results, normalized)
	}
	return results
}

func claimBool(profile map[string]interface{}, fields ...string) bool {
	for _, field := range fields {
		value, ok := claimValue(profile, field)
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case bool:
			return typed
		case string:
			normalized := strings.ToLower(strings.TrimSpace(typed))
			return normalized == "true" || normalized == "1" || normalized == "yes"
		}
	}
	return false
}

func claimValue(profile map[string]interface{}, field string) (interface{}, bool) {
	normalizedField := strings.TrimSpace(field)
	if normalizedField == "" {
		return nil, false
	}
	current := interface{}(profile)
	for _, part := range strings.Split(normalizedField, ".") {
		object, ok := current.(map[string]interface{})
		if !ok {
			return nil, false
		}
		current, ok = object[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}
