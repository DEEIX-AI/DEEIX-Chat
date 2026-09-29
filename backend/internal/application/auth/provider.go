package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/textutil"
	"net/url"
	"regexp"
	"strings"
	"time"

	userapp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/user"
	domainuser "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/user"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/conv"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/secretbox"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/requestmeta"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/security"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// LoginOptions 描述登录界面已启用的认证方式。
type LoginOptions struct {
	UsernameEnabled              bool
	EmailEnabled                 bool
	EmailRegistrationEnabled     bool
	EmailVerificationEnabled     bool
	PasswordResetEnabled         bool
	TurnstileRegistrationEnabled bool
	TurnstileSiteKey             string
	ProviderAuthBridge           ProviderAuthBridgeOptions
	Providers                    []IdentityProviderView
}

// ProviderAuthBridgeOptions 描述原生 OAuth 交接的可用性。
type ProviderAuthBridgeOptions struct {
	Enabled         bool
	ProtocolVersion int
	CallbackBaseURL string
}

// IdentityProviderView 是身份提供方面向用户的安全视图。
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

// UserIdentityView 描述关联到用户账号的身份。
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

// UpsertIdentityProviderInput 包含管理员维护的提供方设置。
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

type resolveProviderUserInput struct {
	Provider      domainuser.IdentityProvider
	Subject       string
	Email         string
	DisplayName   string
	AvatarURL     string
	EmailVerified bool
	ProfileJSON   string
}

type providerIdentityInput struct {
	UserID              uint
	Provider            domainuser.IdentityProvider
	Subject             string
	ProviderDisplayName string
	Email               string
	EmailVerified       bool
	ProfileJSON         string
	LinkedAt            time.Time
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

// GetLoginOptions 返回客户端可用的认证方式与提供方。
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
		ProviderAuthBridge:           s.GetProviderAuthBridgeOptions(),
		Providers:                    providerViews,
	}, nil
}

// ListIdentityProviders 返回供管理使用的已配置身份提供方。
func (s *Service) ListIdentityProviders(ctx context.Context) ([]IdentityProviderView, error) {
	providers, err := s.repo.ListIdentityProviders(ctx, true)
	if err != nil {
		return nil, err
	}
	return toProviderViews(providers, true), nil
}

// CreateIdentityProvider 校验并持久化新的身份提供方。
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

// UpdateIdentityProvider 校验并持久化身份提供方的变更。
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

// DeleteIdentityProvider 在依赖关系允许时删除身份提供方。
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

// HasActiveSuperAdminIdentity 报告是否存在启用的超级管理员身份提供方。
func (s *Service) HasActiveSuperAdminIdentity(ctx context.Context) (bool, error) {
	return s.repo.HasActiveSuperAdminIdentity(ctx)
}

// ListCurrentUserIdentities 返回关联到用户账号的身份。
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

// UnlinkCurrentUserIdentity 在通过登录安全检查后移除已关联身份。
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

// ReorderIdentityProviders 持久化管理员选定的提供方顺序。
func (s *Service) ReorderIdentityProviders(ctx context.Context, publicIDs []string) error {
	normalizedIDs := make([]string, 0, len(publicIDs))
	seen := make(map[string]struct{}, len(publicIDs))
	for _, publicID := range publicIDs {
		normalizedID := conv.NormalizePublicID(publicID)
		if normalizedID == "" {
			return ErrProviderIDRequired
		}
		if _, ok := seen[normalizedID]; ok {
			return ErrProviderOrderInvalid
		}
		seen[normalizedID] = struct{}{}
		normalizedIDs = append(normalizedIDs, normalizedID)
	}
	return s.repo.UpdateIdentityProviderSortOrders(ctx, normalizedIDs)
}

// providerProfileClaims 是用授权码从身份源取回并归一化后的用户资料。
type providerProfileClaims struct {
	Subject       string
	Email         string
	DisplayName   string
	AvatarURL     string
	EmailVerified bool
	ProfileJSON   string
}

// fetchProviderProfileClaims 用授权码换取令牌并拉取用户信息，不读写本地用户。
func (s *Service) fetchProviderProfileClaims(
	ctx context.Context,
	provider domainuser.IdentityProvider,
	code string,
	redirectURI string,
	codeVerifier string,
) (*providerProfileClaims, error) {
	tokenResponse, err := s.exchangeProviderCode(ctx, provider, code, redirectURI, codeVerifier)
	if err != nil {
		return nil, err
	}
	profile, err := s.fetchProviderUserInfo(ctx, provider, tokenResponse.AccessToken)
	if err != nil {
		return nil, err
	}
	profileJSON, _ := json.Marshal(profile)
	subject := claimString(profile, provider.SubjectField)
	if subject == "" {
		return nil, ErrProviderSubjectMissing
	}
	email, err := normalizeProviderEmail(claimString(profile, provider.EmailField))
	if err != nil {
		return nil, err
	}
	return &providerProfileClaims{
		Subject:       subject,
		Email:         email,
		DisplayName:   textutil.FirstNonEmpty(claimString(profile, provider.NameField), email, subject),
		AvatarURL:     claimString(profile, provider.AvatarField),
		EmailVerified: resolveProviderEmailVerified(profile, provider),
		ProfileJSON:   string(profileJSON),
	}, nil
}

func (s *Service) resolveProviderLoginCode(
	ctx context.Context,
	provider domainuser.IdentityProvider,
	code string,
	redirectURI string,
	codeVerifier string,
) (*domainuser.User, string, error) {
	claims, err := s.fetchProviderProfileClaims(ctx, provider, code, redirectURI, codeVerifier)
	if err != nil {
		return nil, "", err
	}
	userItem, err := s.resolveProviderUser(ctx, resolveProviderUserInput{
		Provider:      provider,
		Subject:       claims.Subject,
		Email:         claims.Email,
		DisplayName:   claims.DisplayName,
		AvatarURL:     claims.AvatarURL,
		EmailVerified: claims.EmailVerified,
		ProfileJSON:   claims.ProfileJSON,
	})
	if err != nil {
		return nil, "", err
	}
	return userItem, claims.Subject, nil
}

func (s *Service) completeProviderLoginForUser(
	ctx context.Context,
	userItem *domainuser.User,
	providerSlug string,
	subject string,
	requestID string,
	auditCtx requestmeta.SessionAuditContext,
) (*LoginResult, error) {
	if err := ensureProviderLoginUserActive(userItem); err != nil {
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
			repository.AuthEventInput{
				UserID:    result.User.ID,
				RequestID: requestID,
				EventType: "provider_login",
				Result:    "challenge",
				Reason:    "two_factor_required",
				ClientIP:  normalizedAuditCtx.ClientIP,
				UserAgent: normalizedAuditCtx.UserAgent,
				DetailJSON: marshalAuthEventDetail(map[string]any{
					"provider": providerSlug,
					"subject":  subject,
				}),
			},
		)
		return result, nil
	}
	result, err := s.issueLoginResult(ctx, userItem, normalizedAuditCtx, time.Now())
	if err != nil {
		return nil, err
	}
	s.RecordAuthEvent(
		ctx,
		repository.AuthEventInput{
			UserID:    result.User.ID,
			RequestID: requestID,
			EventType: "provider_login",
			Result:    "success",
			ClientIP:  normalizedAuditCtx.ClientIP,
			UserAgent: normalizedAuditCtx.UserAgent,
			DetailJSON: marshalAuthEventDetail(map[string]any{
				"provider":   providerSlug,
				"subject":    subject,
				"session_id": result.SessionID,
			}),
		},
	)
	return result, nil
}

// bindProviderIdentity 把已取回的身份源资料绑到指定用户；同一主体重复绑定只刷新资料。
func (s *Service) bindProviderIdentity(
	ctx context.Context,
	userID uint,
	provider domainuser.IdentityProvider,
	claims providerProfileClaims,
	requestID string,
	auditCtx requestmeta.SessionAuditContext,
) (*UserIdentityView, error) {
	subject := claims.Subject
	normalizedEmail := claims.Email
	profileJSON := claims.ProfileJSON
	providerDisplayName := claims.DisplayName
	emailVerified := claims.EmailVerified
	now := time.Now()

	existingIdentity, err := s.repo.GetUserIdentityByProviderSubject(ctx, provider.ID, subject)
	if err == nil {
		if existingIdentity.UserID != userID {
			return nil, ErrProviderIdentityConflict
		}
		if err = s.repo.UpdateUserIdentityLogin(ctx, existingIdentity.ID, profileJSON, providerDisplayName, normalizedEmail, emailVerified); err != nil {
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
			return nil, fmt.Errorf("provider email belongs to another account; sign in to that account or change its email before binding: %w", ErrProviderEmailConflict)
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
			return nil, ErrProviderAlreadyBound
		}
	}

	created, err := s.createProviderIdentity(ctx, providerIdentityInput{
		UserID:              userID,
		Provider:            provider,
		Subject:             subject,
		ProviderDisplayName: providerDisplayName,
		Email:               normalizedEmail,
		EmailVerified:       emailVerified,
		ProfileJSON:         profileJSON,
		LinkedAt:            now,
	})
	if err != nil {
		return nil, err
	}
	normalizedAuditCtx := s.resolveSessionAuditContext(ctx, auditCtx)
	s.RecordAuthEvent(
		ctx,
		repository.AuthEventInput{
			UserID:    userID,
			RequestID: requestID,
			EventType: "provider_bind",
			Result:    "success",
			ClientIP:  normalizedAuditCtx.ClientIP,
			UserAgent: normalizedAuditCtx.UserAgent,
			DetailJSON: marshalAuthEventDetail(map[string]any{
				"provider": provider.Slug,
				"subject":  subject,
				"email":    normalizedEmail,
			}),
		},
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
		return nil, ErrProviderTypeInvalid
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, ErrProviderNameRequired
	}
	slug := normalizeProviderSlug(input.Slug)
	if slug == "" {
		slug = normalizeProviderSlug(name)
	}
	if slug == "" {
		return nil, ErrProviderSlugRequired
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
		return nil, ErrProviderDefaultRoleInvalid
	}
	if defaultRole == domainuser.RoleSuperAdmin && input.ActorRole != domainuser.RoleSuperAdmin {
		return nil, ErrIdentityProviderSuperAdminDefaultRoleNotAllowed
	}
	logoURL := strings.TrimSpace(input.LogoURL)
	if logoURL != "" && !isValidProviderLogoURL(logoURL) {
		return nil, ErrProviderLogoURLInvalid
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
		SubjectField:        textutil.FirstNonEmpty(strings.TrimSpace(input.SubjectField), "sub"),
		EmailField:          textutil.FirstNonEmpty(strings.TrimSpace(input.EmailField), "email"),
		EmailVerifiedField:  textutil.FirstNonEmpty(strings.TrimSpace(input.EmailVerifiedField), "email_verified"),
		NameField:           textutil.FirstNonEmpty(strings.TrimSpace(input.NameField), "name"),
		AvatarField:         textutil.FirstNonEmpty(strings.TrimSpace(input.AvatarField), "picture"),
		SortOrder:           100,
	}
	if provider.RegistrationEnabled && !provider.LoginEnabled {
		return nil, ErrProviderRegistrationRequiresLogin
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
		return nil, ErrProviderClientIDRequired
	}
	if provider.ClientSecret == "" {
		return nil, ErrProviderClientSecretRequired
	}
	if err := validateIdentityProviderEndpoints(*provider); err != nil {
		return nil, err
	}
	if providerType == domainuser.IdentityProviderTypeOIDC {
		if provider.IssuerURL == "" && provider.DiscoveryURL == "" {
			return nil, ErrProviderOIDCIssuerRequired
		}
	} else if provider.AuthURL == "" || provider.TokenURL == "" || provider.UserInfoURL == "" {
		return nil, ErrProviderOAuthURLsRequired
	}
	return provider, nil
}

func validateIdentityProviderEndpoints(provider domainuser.IdentityProvider) error {
	endpoints := []struct {
		name  string
		value string
	}{
		{name: "issuer url", value: provider.IssuerURL},
		{name: "discovery url", value: provider.DiscoveryURL},
		{name: "auth url", value: provider.AuthURL},
		{name: "token url", value: provider.TokenURL},
		{name: "userinfo url", value: provider.UserInfoURL},
		{name: "jwks url", value: provider.JWKSURL},
	}
	for _, endpoint := range endpoints {
		if strings.TrimSpace(endpoint.value) == "" {
			continue
		}
		if err := security.ValidateTrustedOutboundHTTPURL(endpoint.value); err != nil {
			return fmt.Errorf("provider %s must be a valid http(s) endpoint without credentials; metadata and link-local targets are not allowed: %w", endpoint.name, ErrProviderEndpointInvalid)
		}
	}
	return nil
}

func isValidProviderLogoURL(value string) bool {
	parsedLogoURL, err := url.Parse(value)
	if err != nil {
		return false
	}
	if (parsedLogoURL.Scheme == "http" || parsedLogoURL.Scheme == "https") && parsedLogoURL.Host != "" && parsedLogoURL.User == nil {
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

func boolValue(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func buildProviderAuthURL(provider domainuser.IdentityProvider, authURL string, redirectURI string, state string, codeChallenge string) (string, error) {
	if authURL == "" {
		return "", ErrProviderAuthURLNotConfigured
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
	response, err := s.providerHTTPClient.PostForm(
		ctx,
		tokenURL,
		providerTrustedEndpoints(provider),
		form,
		map[string]string{"Accept": "application/json"},
	)
	if err != nil {
		return nil, err
	}
	if !response.Successful() {
		return nil, fmt.Errorf("provider token exchange failed: %s: %w", response.Status, ErrProviderUpstreamFailed)
	}
	var tokenResponse oauthTokenResponse
	if tokenResponse, err = parseOAuthTokenResponse(response.Body); err != nil {
		return nil, fmt.Errorf("provider token response decode failed: %w", ErrProviderUpstreamFailed)
	}
	if strings.TrimSpace(tokenResponse.AccessToken) == "" {
		return nil, fmt.Errorf("provider token response missing access token: %w", ErrProviderUpstreamFailed)
	}
	return &tokenResponse, nil
}

func (s *Service) fetchProviderUserInfo(ctx context.Context, provider domainuser.IdentityProvider, accessToken string) (map[string]any, error) {
	_, _, userInfoURL, err := s.resolveProviderEndpoints(ctx, provider)
	if err != nil {
		return nil, err
	}
	trustedEndpoints := providerTrustedEndpoints(provider)
	response, err := s.providerHTTPClient.Get(
		ctx,
		userInfoURL,
		trustedEndpoints,
		map[string]string{
			"Accept":        "application/json",
			"Authorization": "Bearer " + accessToken,
		},
	)
	if err != nil {
		return nil, err
	}
	if !response.Successful() {
		return nil, fmt.Errorf("provider userinfo failed: %s: %w", response.Status, ErrProviderUpstreamFailed)
	}
	var profile map[string]any
	if err = json.Unmarshal(response.Body, &profile); err != nil {
		return nil, fmt.Errorf("provider userinfo response decode failed: %w", ErrProviderUpstreamFailed)
	}
	if githubEmailsURL, ok := githubEmailsEndpoint(provider, userInfoURL); ok {
		if err = s.enrichGitHubVerifiedEmail(ctx, accessToken, profile, githubEmailsURL, trustedEndpoints); err != nil {
			return nil, err
		}
	}
	return profile, nil
}

func (s *Service) enrichGitHubVerifiedEmail(
	ctx context.Context,
	accessToken string,
	profile map[string]any,
	emailsURL string,
	trustedEndpoints []string,
) error {
	if strings.TrimSpace(accessToken) == "" || strings.TrimSpace(emailsURL) == "" {
		return nil
	}
	existingEmail, _ := normalizeProviderEmail(claimString(profile, "email"))
	if existingEmail != "" && resolveProviderEmailVerified(profile, domainuser.IdentityProvider{EmailVerifiedField: "email_verified"}) {
		return nil
	}
	response, err := s.providerHTTPClient.Get(
		ctx,
		emailsURL,
		trustedEndpoints,
		map[string]string{
			"Accept":        "application/vnd.github+json",
			"Authorization": "Bearer " + accessToken,
		},
	)
	if err != nil {
		return err
	}
	if !response.Successful() {
		return fmt.Errorf("github provider emails failed: %s: %w", response.Status, ErrProviderUpstreamFailed)
	}
	var emails []githubEmailAddress
	if err = json.Unmarshal(response.Body, &emails); err != nil {
		return fmt.Errorf("github provider emails response decode failed: %w", ErrProviderUpstreamFailed)
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
	if err := validateIdentityProviderEndpoints(provider); err != nil {
		return "", "", "", err
	}
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
	response, err := s.providerHTTPClient.Get(
		ctx,
		discoveryURL,
		providerTrustedEndpoints(provider),
		map[string]string{"Accept": "application/json"},
	)
	if err != nil {
		return "", "", "", err
	}
	if !response.Successful() {
		return "", "", "", fmt.Errorf("provider discovery failed: %s: %w", response.Status, ErrProviderUpstreamFailed)
	}
	metadata, err := parseOIDCDiscoveryDocument(response.Body)
	if err != nil {
		return "", "", "", err
	}
	resolvedAuthURL := textutil.FirstNonEmpty(authURL, metadata.AuthorizationEndpoint)
	resolvedTokenURL := textutil.FirstNonEmpty(tokenURL, metadata.TokenEndpoint)
	resolvedUserInfoURL := textutil.FirstNonEmpty(userInfoURL, metadata.UserInfoEndpoint)
	if err = validateIdentityProviderEndpoints(domainuser.IdentityProvider{
		AuthURL:     resolvedAuthURL,
		TokenURL:    resolvedTokenURL,
		UserInfoURL: resolvedUserInfoURL,
	}); err != nil {
		return "", "", "", err
	}
	return resolvedAuthURL, resolvedTokenURL, resolvedUserInfoURL, nil
}

func parseOAuthTokenResponse(raw []byte) (oauthTokenResponse, error) {
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return oauthTokenResponse{}, err
	}
	return oauthTokenResponse{
		AccessToken: claimString(payload, "access_token"),
		TokenType:   claimString(payload, "token_type"),
		IDToken:     claimString(payload, "id_token"),
	}, nil
}

func parseOIDCDiscoveryDocument(raw []byte) (oidcDiscoveryDocument, error) {
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return oidcDiscoveryDocument{}, err
	}
	return oidcDiscoveryDocument{
		AuthorizationEndpoint: claimString(payload, "authorization_endpoint"),
		TokenEndpoint:         claimString(payload, "token_endpoint"),
		UserInfoEndpoint:      claimString(payload, "userinfo_endpoint"),
	}, nil
}

func providerTrustedEndpoints(provider domainuser.IdentityProvider) []string {
	return []string{
		provider.IssuerURL,
		provider.DiscoveryURL,
		provider.AuthURL,
		provider.TokenURL,
		provider.UserInfoURL,
		provider.JWKSURL,
	}
}

func (s *Service) resolveProviderUser(ctx context.Context, input resolveProviderUserInput) (*domainuser.User, error) {
	identity, err := s.repo.GetUserIdentityByProviderSubject(ctx, input.Provider.ID, input.Subject)
	if err == nil {
		if !input.Provider.LoginEnabled {
			return nil, ErrProviderLoginDisabled
		}
		userItem, getErr := s.repo.GetByID(ctx, identity.UserID)
		if getErr != nil {
			return nil, getErr
		}
		if err = ensureProviderLoginUserActive(userItem); err != nil {
			return nil, err
		}
		if updateErr := s.repo.UpdateUserIdentityLogin(ctx, identity.ID, input.ProfileJSON, input.DisplayName, strings.TrimSpace(input.Email), input.EmailVerified); updateErr != nil {
			return nil, updateErr
		}
		return userItem, nil
	}
	if !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}

	cfg := s.cfg.Snapshot()
	now := time.Now()
	normalizedEmail, err := normalizeProviderEmail(input.Email)
	if err != nil {
		return nil, err
	}
	if cfg.AutoLinkVerifiedEmail && input.EmailVerified && normalizedEmail != "" {
		existingUser, findErr := s.repo.GetByEmail(ctx, normalizedEmail)
		if findErr == nil {
			if err = ensureProviderLoginUserActive(existingUser); err != nil {
				return nil, err
			}
			if _, createErr := s.createProviderIdentity(ctx, providerIdentityInput{
				UserID:              existingUser.ID,
				Provider:            input.Provider,
				Subject:             input.Subject,
				ProviderDisplayName: input.DisplayName,
				Email:               normalizedEmail,
				EmailVerified:       input.EmailVerified,
				ProfileJSON:         input.ProfileJSON,
				LinkedAt:            now,
			}); createErr != nil {
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
				ProviderSlug: input.Provider.Slug,
				Email:        normalizedEmail,
				Action:       ProviderEmailConflictActionSignInThenBind,
			}
		} else if !errors.Is(findErr, repository.ErrNotFound) {
			return nil, findErr
		}
	}
	if !input.Provider.RegistrationEnabled {
		return nil, ErrProviderAccountNotRegistered
	}

	emailVerifiedAt := (*time.Time)(nil)
	emailSource := domainuser.EmailSourceProviderUnverified
	if input.EmailVerified && normalizedEmail != "" {
		emailVerifiedAt = &now
		emailSource = domainuser.EmailSourceProviderVerified
	}
	userItem := &domainuser.User{
		PublicID:        conv.NormalizePublicID(uuid.NewString()),
		Username:        providerUsername(input.Provider.Slug, input.Subject),
		DisplayName:     userapp.NormalizeGeneratedDisplayName(textutil.FirstNonEmpty(input.DisplayName, input.Provider.Name+" 用户")),
		AvatarURL:       strings.TrimSpace(input.AvatarURL),
		Email:           normalizedEmail,
		EmailSource:     emailSource,
		Role:            textutil.FirstNonEmpty(input.Provider.DefaultRole, domainuser.RoleUser),
		Status:          domainuser.StatusActive,
		Timezone:        "Etc/UTC",
		Locale:          "en-US",
		EmailVerifiedAt: emailVerifiedAt,
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(uuid.NewString()), passwordHashCost)
	if err != nil {
		return nil, err
	}
	providerIdentity := s.newProviderIdentity(providerIdentityInput{
		UserID:              userItem.ID,
		Provider:            input.Provider,
		Subject:             input.Subject,
		ProviderDisplayName: input.DisplayName,
		Email:               normalizedEmail,
		EmailVerified:       input.EmailVerified,
		ProfileJSON:         input.ProfileJSON,
		LinkedAt:            now,
	})
	if err = s.createWithCredentialAndIdentityUsingAvailableUsername(ctx, repository.CreateWithCredentialAndIdentityInput{
		CreateWithCredentialInput: repository.CreateWithCredentialInput{
			User: userItem,
			Credential: domainuser.Credential{
				PasswordHash:      string(passwordHash),
				PasswordAlgo:      "bcrypt",
				PasswordEnabled:   false,
				PasswordUpdatedAt: &now,
				PasswordOrigin:    domainuser.PasswordOriginSSOPlaceholder,
			},
		},
		Identity: providerIdentity,
	}); err != nil {
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

func (s *Service) createProviderIdentity(ctx context.Context, input providerIdentityInput) (*domainuser.UserIdentity, error) {
	return s.repo.CreateUserIdentity(ctx, s.newProviderIdentity(input))
}

func (s *Service) newProviderIdentity(input providerIdentityInput) *domainuser.UserIdentity {
	return &domainuser.UserIdentity{
		UserID:              input.UserID,
		ProviderID:          input.Provider.ID,
		ProviderType:        input.Provider.Type,
		ProviderSubject:     strings.TrimSpace(input.Subject),
		ProviderDisplayName: strings.TrimSpace(input.ProviderDisplayName),
		Email:               strings.TrimSpace(input.Email),
		EmailVerified:       input.EmailVerified,
		ProfileJSON:         input.ProfileJSON,
		LinkedAt:            input.LinkedAt,
		LastLoginAt:         &input.LinkedAt,
	}
}

func (s *Service) createWithCredentialAndIdentityUsingAvailableUsername(ctx context.Context, input repository.CreateWithCredentialAndIdentityInput) error {
	baseUsername := input.User.Username
	for attempt := 0; attempt < 20; attempt++ {
		input.User.ID = 0
		input.User.Username = generatedUsernameWithSuffix(baseUsername, attempt)
		err := s.repo.CreateWithCredentialAndIdentity(ctx, input)
		if errors.Is(err, repository.ErrDuplicateUsername) {
			continue
		}
		return err
	}
	return ErrUsernameTaken
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
		return ErrPKCEChallengeRequired
	}
	return nil
}

func (s *Service) validateProviderRedirectURI(slug string, redirectURI string) error {
	parsed, err := url.Parse(strings.TrimSpace(redirectURI))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return ErrInvalidRedirectURI
	}
	if parsed.Path != "/auth/callback" || parsed.Query().Get("provider") != slug {
		return ErrInvalidRedirectURI
	}
	if s.isAllowedProviderRedirectOrigin(parsed) {
		return nil
	}
	return ErrRedirectURIOriginNotAllowed
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

func claimString(profile map[string]any, field string) string {
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
		return "", ErrProviderEmailInvalid
	}
	return normalized, nil
}

func resolveProviderEmailVerified(profile map[string]any, provider domainuser.IdentityProvider) bool {
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

func claimBool(profile map[string]any, fields ...string) bool {
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

func claimValue(profile map[string]any, field string) (any, bool) {
	normalizedField := strings.TrimSpace(field)
	if normalizedField == "" {
		return nil, false
	}
	current := any(profile)
	for _, part := range strings.Split(normalizedField, ".") {
		object, ok := current.(map[string]any)
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
