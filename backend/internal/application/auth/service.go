package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/billing"
	appstorage "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/objectstorage"
	userapp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/user"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/userview"
	domainuser "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/user"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/geoip"
	platformtracing "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/observability/tracing"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/conv"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/token"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/requestmeta"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/security"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

const passwordHashCost = 12
const refreshTokenPreviousHashGrace = 15 * time.Second
const accessTokenSessionClockSkew = 2 * time.Minute

// Service 封装认证业务能力。
type Service struct {
	cfg                  *config.Runtime
	repo                 repository.AuthRepository
	geoResolver          *geoip.Client
	subscriptionResolver subscriptionResolver
	providerHTTPClient   *http.Client
	logger               *zap.Logger
	storeProvider        appstorage.Provider
	auditWriter          auditWriter
}

type subscriptionResolver interface {
	GetCurrentSubscriptionSnapshot(
		ctx context.Context,
		userID uint,
		now time.Time,
	) (*billing.UserSubscriptionSnapshot, error)
}

type auditWriter interface {
	Write(ctx context.Context, requestID string, actorUserID uint, action string, resource string, resourceID string, ip string, userAgent string, detail interface{})
}

// NewService 创建服务。
func NewService(cfg config.Config, repo repository.AuthRepository, geoResolver *geoip.Client) *Service {
	return NewServiceWithRuntime(config.NewRuntime(cfg), repo, geoResolver)
}

// NewServiceWithRuntime 创建使用运行时配置容器的服务。
func NewServiceWithRuntime(cfg *config.Runtime, repo repository.AuthRepository, geoResolver *geoip.Client) *Service {
	env := ""
	ssrfProtectionEnabled := false
	if cfg != nil {
		snapshot := cfg.Snapshot()
		env = snapshot.Env
		ssrfProtectionEnabled = snapshot.SSRFProtectionEnabled
	}
	providerHTTPClient := newAuthOutboundHTTPClient(env, ssrfProtectionEnabled)
	return &Service{
		cfg:                cfg,
		repo:               repo,
		geoResolver:        geoResolver,
		providerHTTPClient: providerHTTPClient,
		storeProvider:      appstorage.NewRuntimeProvider(cfg, nil),
	}
}

func newAuthOutboundHTTPClient(env string, ssrfProtectionEnabled bool) *http.Client {
	client := security.NewOutboundHTTPClient(env, ssrfProtectionEnabled, providerHTTPTimeout)
	client.Transport = platformtracing.NewHTTPTransport(client.Transport)
	return client
}

// SetSubscriptionResolver 注入订阅派生解析能力。
func (s *Service) SetSubscriptionResolver(resolver subscriptionResolver) {
	s.subscriptionResolver = resolver
}

// SetLogger 注入结构化日志记录器。
func (s *Service) SetLogger(logger *zap.Logger) {
	s.logger = logger
}

// SetObjectStoreProvider 注入对象存储 provider。
func (s *Service) SetObjectStoreProvider(provider appstorage.Provider) {
	if provider != nil {
		s.storeProvider = provider
	}
}

// ShouldUseSecureCookies 判断当前运行环境是否必须写入 Secure Cookie。
func (s *Service) ShouldUseSecureCookies() bool {
	if s == nil || s.cfg == nil {
		return false
	}
	env := strings.ToLower(strings.TrimSpace(s.cfg.Snapshot().Env))
	return env == "prod" || env == "production"
}

// SetAuditWriter 注入认证域审计写入器。
func (s *Service) SetAuditWriter(writer auditWriter) {
	s.auditWriter = writer
}

// AuditInput 描述认证域审计写入。
type AuditInput struct {
	UserID     uint
	RequestID  string
	Action     string
	Resource   string
	ResourceID string
	ClientIP   string
	UserAgent  string
	Detail     interface{}
}

// RecordAudit 记录认证域审计日志。
func (s *Service) RecordAudit(ctx context.Context, input AuditInput) {
	if s.auditWriter == nil {
		return
	}
	s.auditWriter.Write(
		ctx,
		strings.TrimSpace(input.RequestID),
		input.UserID,
		strings.TrimSpace(input.Action),
		strings.TrimSpace(input.Resource),
		strings.TrimSpace(input.ResourceID),
		strings.TrimSpace(input.ClientIP),
		strings.TrimSpace(input.UserAgent),
		input.Detail,
	)
}

func (s *Service) warn(message string, fields ...zap.Field) {
	if s.logger == nil {
		return
	}
	s.logger.Warn(message, fields...)
}

func (s *Service) info(message string, fields ...zap.Field) {
	if s.logger == nil {
		return
	}
	s.logger.Info(message, fields...)
}

// EnsureBootstrapSuperAdmin 确保系统至少存在一个 superadmin。
func (s *Service) EnsureBootstrapSuperAdmin(ctx context.Context) error {
	count, err := s.repo.CountSuperAdmins(ctx)
	if err != nil {
		return err
	}
	if count > 0 {
		return s.repo.MarkBootstrapSuperAdminPasswordResetRequired(ctx, s.cfg.Snapshot().AdminUsername)
	}

	cfg := s.cfg.Snapshot()
	bootstrapPassword, err := generateBootstrapAdminPassword()
	if err != nil {
		return err
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(bootstrapPassword), passwordHashCost)
	if err != nil {
		return err
	}
	now := time.Now()

	username := strings.TrimSpace(cfg.AdminUsername)
	displayName := strings.TrimSpace(cfg.AdminDisplayName)
	if displayName == "" {
		displayName = username
	}

	item := &domainuser.User{
		PublicID:    conv.NormalizePublicID(uuid.NewString()),
		Username:    username,
		DisplayName: displayName,
		Email:       "",
		Role:        domainuser.RoleSuperAdmin,
		Status:      domainuser.StatusActive,
		Timezone:    "Etc/UTC",
		Locale:      "en-US",
	}

	if err = s.repo.CreateWithCredential(ctx, item, domainuser.Credential{
		PasswordHash:      string(passwordHash),
		PasswordAlgo:      "bcrypt",
		PasswordEnabled:   true,
		PasswordUpdatedAt: &now,
		PasswordSetAt:     &now,
		PasswordOrigin:    domainuser.PasswordOriginAdminCreated,
		MustResetPassword: true,
	}, 0, 0, nil, false); err != nil {
		return err
	}
	s.info("bootstrap superadmin created", zap.String("username", username), zap.String("password", bootstrapPassword))
	return nil
}

func generateBootstrapAdminPassword() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate bootstrap admin password: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// Login 登录鉴权并返回访问令牌，成功与失败均记录认证事件。
func (s *Service) Login(
	ctx context.Context,
	username string,
	password string,
	requestID string,
	auditCtx requestmeta.SessionAuditContext,
) (*LoginResult, error) {
	normalizedAuditCtx := s.resolveSessionAuditContext(ctx, auditCtx)
	result, err := s.doLogin(ctx, username, password, normalizedAuditCtx)
	if err != nil {
		reason := "invalid_credentials_or_inactive"
		if errors.Is(err, ErrAccountLocked) {
			reason = "account_locked"
		}
		s.RecordAuthEvent(
			ctx, 0, requestID, "login", "failure", reason,
			normalizedAuditCtx.ClientIP, normalizedAuditCtx.UserAgent,
			marshalAuthEventDetail(map[string]string{"username": strings.TrimSpace(username)}),
		)
		return nil, err
	}
	if result.TwoFactorRequired {
		s.RecordAuthEvent(
			ctx, result.User.ID, requestID, "login", "challenge", "two_factor_required",
			normalizedAuditCtx.ClientIP, normalizedAuditCtx.UserAgent,
			marshalAuthEventDetail(map[string]string{"username": result.User.Username}),
		)
		return result, nil
	}
	s.RecordAuthEvent(
		ctx, result.User.ID, requestID, "login", "success", "",
		normalizedAuditCtx.ClientIP, normalizedAuditCtx.UserAgent,
		marshalAuthEventDetail(map[string]interface{}{
			"username":       result.User.Username,
			"session_id":     result.SessionID,
			"client_ip":      normalizedAuditCtx.ClientIP,
			"location_label": normalizedAuditCtx.LocationLabel(),
		}),
	)
	return result, nil
}

// doLogin 执行登录核心逻辑：验证凭据、创建会话并签发令牌，不记录事件。
func (s *Service) doLogin(
	ctx context.Context,
	username string,
	password string,
	normalizedAuditCtx requestmeta.SessionAuditContext,
) (*LoginResult, error) {
	normalizedUsername := strings.TrimSpace(username)
	cfg := s.cfg.Snapshot()
	var item *domainuser.User
	var err error
	if strings.Contains(normalizedUsername, "@") {
		if !cfg.EmailLoginEnabled {
			return nil, ErrInvalidCredentials
		}
		item, err = s.repo.GetByEmail(ctx, normalizedUsername)
	} else {
		if !cfg.UsernameLoginEnabled {
			return nil, ErrInvalidCredentials
		}
		item, err = s.repo.GetByUsername(ctx, normalizedUsername)
	}
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	credential, err := s.repo.GetCredentialByUserID(ctx, item.ID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}
	if !credential.PasswordEnabled {
		return nil, ErrInvalidCredentials
	}

	now := time.Now()
	if credential.LockedUntil != nil && now.After(*credential.LockedUntil) {
		if err = s.repo.ResetLoginFailure(ctx, item.ID); err != nil {
			return nil, err
		}
		credential.FailedLoginCount = 0
		credential.LockedUntil = nil
		if item.Status == domainuser.StatusLocked {
			if err = s.repo.UpdateUserStatus(ctx, item.ID, domainuser.StatusActive); err != nil {
				return nil, err
			}
			item.Status = domainuser.StatusActive
		}
	}

	if item.Status == domainuser.StatusLocked {
		return nil, ErrAccountLocked
	}
	if item.Status != domainuser.StatusActive {
		return nil, ErrInvalidCredentials
	}
	if credential.LockedUntil != nil && now.Before(*credential.LockedUntil) {
		if item.Status != domainuser.StatusLocked {
			if lockErr := s.repo.UpdateUserStatus(ctx, item.ID, domainuser.StatusLocked); lockErr != nil {
				s.warn("lock_account_failed", zap.Uint("user_id", item.ID), zap.Error(lockErr))
			}
		}
		return nil, ErrAccountLocked
	}

	if err = bcrypt.CompareHashAndPassword([]byte(credential.PasswordHash), []byte(password)); err != nil {
		lockUntil := now.Add(s.loginLockDuration())
		updatedCredential, markErr := s.repo.MarkLoginFailure(ctx, item.ID, s.loginLockThreshold(), lockUntil)
		if markErr != nil {
			return nil, markErr
		}
		if updatedCredential.LockedUntil != nil && now.Before(*updatedCredential.LockedUntil) {
			if lockErr := s.repo.UpdateUserStatus(ctx, item.ID, domainuser.StatusLocked); lockErr != nil {
				s.warn("lock_account_failed", zap.Uint("user_id", item.ID), zap.Error(lockErr))
			}
			return nil, ErrAccountLocked
		}
		return nil, ErrInvalidCredentials
	}

	if credential.FailedLoginCount > 0 || credential.LockedUntil != nil {
		if err = s.repo.ResetLoginFailure(ctx, item.ID); err != nil {
			return nil, err
		}
	}
	if item.Status == domainuser.StatusLocked {
		if err = s.repo.UpdateUserStatus(ctx, item.ID, domainuser.StatusActive); err != nil {
			return nil, err
		}
	}
	requireTwoFactor, err := s.shouldRequireTwoFactor(ctx, item)
	if err != nil {
		return nil, err
	}
	if requireTwoFactor {
		return s.buildTwoFactorChallenge(ctx, item)
	}

	return s.issueLoginResult(ctx, item, normalizedAuditCtx, now)
}

func (s *Service) issueLoginResult(
	ctx context.Context,
	item *domainuser.User,
	normalizedAuditCtx requestmeta.SessionAuditContext,
	now time.Time,
) (*LoginResult, error) {
	sessionID := uuid.NewString()
	tokenBundle, err := s.buildSessionTokenPair(item, sessionID, now)
	if err != nil {
		return nil, err
	}
	sessionSnapshot := buildSessionAuditSnapshot(normalizedAuditCtx)

	session := &domainuser.Session{
		SessionID:        sessionID,
		UserID:           item.ID,
		RefreshTokenHash: hashToken(tokenBundle.RefreshToken),
		AccessJTI:        tokenBundle.AccessJTI,
		ClientIP:         sessionSnapshot.ClientIP,
		UserAgent:        sessionSnapshot.UserAgent,
		DeviceName:       sessionSnapshot.DeviceName,
		BrowserName:      sessionSnapshot.BrowserName,
		OSName:           sessionSnapshot.OSName,
		DeviceType:       sessionSnapshot.DeviceType,
		GeoSource:        sessionSnapshot.GeoSource,
		GeoAccuracy:      sessionSnapshot.GeoAccuracy,
		CountryCode:      sessionSnapshot.CountryCode,
		RegionName:       sessionSnapshot.RegionName,
		CityName:         sessionSnapshot.CityName,
		TimezoneName:     sessionSnapshot.TimezoneName,
		IPLatitude:       sessionSnapshot.IPLatitude,
		IPLongitude:      sessionSnapshot.IPLongitude,
		IssuedAt:         now,
		LastSeenAt:       &now,
		ExpiresAt:        tokenBundle.RefreshExpiresAt,
		RevokedAt:        nil,
		RevokeReason:     "",
	}
	if err = s.repo.CreateSession(ctx, session); err != nil {
		return nil, err
	}

	if err = s.repo.UpdateLastLogin(ctx, item.ID); err != nil {
		return nil, err
	}

	userView, err := s.buildUserView(ctx, *item)
	if err != nil {
		return nil, err
	}

	return &LoginResult{
		AccessToken:      tokenBundle.AccessToken,
		RefreshToken:     tokenBundle.RefreshToken,
		SessionID:        sessionID,
		ExpiresAt:        tokenBundle.ExpiresAt,
		RefreshExpiresAt: tokenBundle.RefreshExpiresAt,
		User:             userView,
	}, nil
}

// UpdateProfileInput 当前用户资料更新输入。
type UpdateProfileInput struct {
	AvatarURL             *string
	DisplayName           *string
	Timezone              *string
	Locale                *string
	ProfilePreferences    *string
	AppearancePreferences *string
}

type UpdateUsernameInput struct {
	Username string
}

// UpdateCurrentSessionLocationInput 当前会话精确位置输入。
type UpdateCurrentSessionLocationInput struct {
	Latitude       float64
	Longitude      float64
	AccuracyMeters *float64
	Timezone       string
}

// GetProfile 查询当前用户资料。
func (s *Service) GetProfile(ctx context.Context, userID uint) (*domainuser.User, error) {
	return s.repo.GetByID(ctx, userID)
}

// buildUserView 内部构建带订阅快照的用户视图。
func (s *Service) buildUserView(ctx context.Context, item domainuser.User) (userview.UserView, error) {
	credential, credentialErr := s.repo.GetCredentialByUserID(ctx, item.ID)
	if credentialErr != nil && !errors.Is(credentialErr, repository.ErrNotFound) {
		return userview.UserView{}, credentialErr
	}
	if s.subscriptionResolver == nil {
		view := userview.FromUser(item, nil)
		s.applyCredentialView(&view, item, credential)
		if err := s.applyTwoFactorView(ctx, &view); err != nil {
			return userview.UserView{}, err
		}
		return view, nil
	}

	subscription, err := s.subscriptionResolver.GetCurrentSubscriptionSnapshot(ctx, item.ID, time.Now())
	if err != nil {
		return userview.UserView{}, err
	}
	if subscription == nil {
		view := userview.FromUser(item, nil)
		s.applyCredentialView(&view, item, credential)
		if err := s.applyTwoFactorView(ctx, &view); err != nil {
			return userview.UserView{}, err
		}
		return view, nil
	}

	view := userview.FromUser(item, &userview.SubscriptionState{
		PlanID:    subscription.PlanID,
		PlanName:  subscription.PlanName,
		Tier:      subscription.Tier,
		Status:    subscription.Status,
		ExpiresAt: subscription.ExpiresAt,
	})
	s.applyCredentialView(&view, item, credential)
	if err := s.applyTwoFactorView(ctx, &view); err != nil {
		return userview.UserView{}, err
	}
	return view, nil
}

func (s *Service) applyCredentialView(view *userview.UserView, item domainuser.User, credential *domainuser.Credential) {
	if view == nil || credential == nil {
		return
	}
	view.PasswordEnabled = credential.PasswordEnabled
	view.PasswordSetAt = credential.PasswordSetAt
	view.PasswordOrigin = credential.PasswordOrigin
	view.MustResetPassword = credential.MustResetPassword || isBootstrapSuperAdminAdminCreatedPassword(item, credential)
	view.InitialUsernameRequired = shouldRequireInitialUsername(item, s.cfg.Snapshot().AdminUsername)
	view.InitialSecurityRequired = view.MustResetPassword || item.OnboardingCompletedAt == nil
}

func shouldRequireInitialUsername(item domainuser.User, adminUsername string) bool {
	if item.UsernameChangedAt != nil {
		return false
	}
	if item.Role == domainuser.RoleSuperAdmin {
		return strings.EqualFold(strings.TrimSpace(item.Username), strings.TrimSpace(adminUsername))
	}
	return item.Role == domainuser.RoleUser &&
		(item.EmailSource == domainuser.EmailSourceLocalRegister ||
			item.EmailSource == domainuser.EmailSourceProviderVerified ||
			item.EmailSource == domainuser.EmailSourceProviderUnverified)
}

func (s *Service) CompleteOnboarding(
	ctx context.Context,
	userID uint,
	newPassword string,
	requestID string,
	auditCtx requestmeta.SessionAuditContext,
) (*domainuser.User, bool, error) {
	item, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return nil, false, err
	}
	credential, credentialErr := s.repo.GetCredentialByUserID(ctx, userID)
	if credentialErr != nil && !errors.Is(credentialErr, repository.ErrNotFound) {
		return nil, false, credentialErr
	}
	cfg := s.cfg.Snapshot()
	if shouldRequireInitialUsername(*item, cfg.AdminUsername) {
		return nil, false, fmt.Errorf("username change required")
	}
	passwordChanged := false
	if credential != nil && (credential.MustResetPassword || isBootstrapSuperAdminAdminCreatedPassword(*item, credential)) {
		trimmedPassword, policyErr := userapp.NormalizePassword(newPassword)
		if policyErr != nil {
			return nil, false, policyErr
		}
		if isBootstrapSuperAdminAdminCreatedPassword(*item, credential) && passwordMatchesCredential(trimmedPassword, credential) {
			return nil, false, fmt.Errorf("new password must be different from the bootstrap password")
		}
		passwordHash, hashErr := bcrypt.GenerateFromPassword([]byte(trimmedPassword), passwordHashCost)
		if hashErr != nil {
			return nil, false, hashErr
		}
		if err = s.repo.UpdatePassword(ctx, userID, string(passwordHash), domainuser.PasswordOriginUserSet, false); err != nil {
			return nil, false, err
		}
		passwordChanged = true
	}
	now := time.Now()
	completedAt := &now
	updated, err := s.repo.UpdateProfile(ctx, userID, repository.UpdateUserFieldsInput{
		OnboardingCompletedAt: &completedAt,
	})
	if err != nil {
		return nil, passwordChanged, err
	}
	if passwordChanged {
		if err = s.repo.RevokeAllSessions(ctx, userID, "password_change"); err != nil {
			return nil, passwordChanged, err
		}
		normalizedAuditCtx := s.resolveSessionAuditContext(ctx, auditCtx)
		s.RecordAuthEvent(
			ctx, userID, requestID, "password_change", "success", "",
			normalizedAuditCtx.ClientIP, normalizedAuditCtx.UserAgent,
			marshalAuthEventDetail(map[string]interface{}{"initial_onboarding": true}),
		)
	}
	return updated, passwordChanged, nil
}

func isBootstrapSuperAdminAdminCreatedPassword(item domainuser.User, credential *domainuser.Credential) bool {
	return credential != nil &&
		item.Role == domainuser.RoleSuperAdmin &&
		credential.PasswordEnabled &&
		credential.PasswordOrigin == domainuser.PasswordOriginAdminCreated
}

func passwordMatchesCredential(password string, credential *domainuser.Credential) bool {
	if credential == nil || strings.TrimSpace(credential.PasswordHash) == "" {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(credential.PasswordHash), []byte(password)) == nil
}

func (s *Service) applyTwoFactorView(ctx context.Context, view *userview.UserView) error {
	if view == nil {
		return nil
	}
	status, err := s.GetCurrentTwoFactorStatus(ctx, view.ID)
	if err != nil {
		return err
	}
	view.TwoFactorAvailable = status.Available
	view.TwoFactorEnabled = status.TOTPEnabled
	view.TwoFactorRequired = status.Required
	view.TwoFactorRecoveryCount = status.RecoveryCount
	return nil
}

// BuildUserView 构建对外可用的用户展示视图。
func (s *Service) BuildUserView(ctx context.Context, item domainuser.User) (userview.UserView, error) {
	return s.buildUserView(ctx, item)
}

// resolveSessionAuditContext 标准化审计上下文，并在无地理信息时尝试通过 GeoIP 补全。
func (s *Service) resolveSessionAuditContext(
	ctx context.Context,
	auditCtx requestmeta.SessionAuditContext,
) requestmeta.SessionAuditContext {
	normalized := auditCtx.Normalize()
	if normalized.CountryCode != "" || normalized.RegionName != "" || normalized.CityName != "" || normalized.TimezoneName != "" {
		return normalized
	}
	if s.geoResolver == nil {
		return normalized
	}

	enriched, err := s.geoResolver.Lookup(ctx, normalized.ClientIP)
	if err != nil {
		return normalized
	}
	return mergeSessionAuditContext(normalized, enriched)
}

// mergeSessionAuditContext 将 enriched 中的地理信息补填到 base 中，仅覆盖 base 的空字段。
func mergeSessionAuditContext(
	base requestmeta.SessionAuditContext,
	enriched requestmeta.SessionAuditContext,
) requestmeta.SessionAuditContext {
	result := base.Normalize()
	addon := enriched.Normalize()
	if result.GeoSource == "" {
		result.GeoSource = addon.GeoSource
	}
	if result.GeoAccuracy == "" {
		result.GeoAccuracy = addon.GeoAccuracy
	}
	if result.CountryCode == "" {
		result.CountryCode = addon.CountryCode
	}
	if result.RegionName == "" {
		result.RegionName = addon.RegionName
	}
	if result.CityName == "" {
		result.CityName = addon.CityName
	}
	if result.TimezoneName == "" {
		result.TimezoneName = addon.TimezoneName
	}
	if result.IPLatitude == nil {
		result.IPLatitude = addon.IPLatitude
	}
	if result.IPLongitude == nil {
		result.IPLongitude = addon.IPLongitude
	}
	return result
}

func sessionActivityInputFromSnapshot(snapshot sessionAuditSnapshot, lastSeenAt time.Time) repository.UpdateSessionActivityInput {
	return repository.UpdateSessionActivityInput{
		LastSeenAt:   &lastSeenAt,
		ClientIP:     &snapshot.ClientIP,
		UserAgent:    &snapshot.UserAgent,
		DeviceName:   &snapshot.DeviceName,
		BrowserName:  &snapshot.BrowserName,
		OSName:       &snapshot.OSName,
		DeviceType:   &snapshot.DeviceType,
		GeoSource:    &snapshot.GeoSource,
		GeoAccuracy:  &snapshot.GeoAccuracy,
		CountryCode:  &snapshot.CountryCode,
		RegionName:   &snapshot.RegionName,
		CityName:     &snapshot.CityName,
		TimezoneName: &snapshot.TimezoneName,
		IPLatitude:   &snapshot.IPLatitude,
		IPLongitude:  &snapshot.IPLongitude,
	}
}

// UpdateProfile 更新当前用户资料。
func (s *Service) UpdateProfile(ctx context.Context, userID uint, input UpdateProfileInput) (*domainuser.User, error) {
	updateInput := repository.UpdateUserFieldsInput{}

	if input.AvatarURL != nil {
		nextAvatarURL := strings.TrimSpace(*input.AvatarURL)
		if err := validateAvatarURL(nextAvatarURL); err != nil {
			return nil, err
		}
		updateInput.AvatarURL = &nextAvatarURL
	}
	if input.DisplayName != nil {
		displayName, err := userapp.NormalizeDisplayName(*input.DisplayName)
		if err != nil {
			return nil, err
		}
		updateInput.DisplayName = &displayName
	}
	if input.Timezone != nil {
		nextTimezone := strings.TrimSpace(*input.Timezone)
		if nextTimezone == "" {
			nextTimezone = "Etc/UTC"
		}
		if _, err := time.LoadLocation(nextTimezone); err != nil {
			return nil, ErrInvalidTimeZone
		}
		updateInput.Timezone = &nextTimezone
	}
	if input.Locale != nil {
		nextLocale, err := normalizeLocale(*input.Locale)
		if err != nil {
			return nil, err
		}
		updateInput.Locale = &nextLocale
	}
	if input.ProfilePreferences != nil {
		profilePreferences := strings.TrimSpace(*input.ProfilePreferences)
		updateInput.ProfilePreferences = &profilePreferences
	}
	if input.AppearancePreferences != nil {
		appearancePreferences := strings.TrimSpace(*input.AppearancePreferences)
		normalizedAppearancePreferences, err := normalizeAppearancePreferences(appearancePreferences)
		if err != nil {
			return nil, err
		}
		updateInput.AppearancePreferences = &normalizedAppearancePreferences
	}

	return s.repo.UpdateProfile(ctx, userID, updateInput)
}

func normalizeAppearancePreferences(raw string) (string, error) {
	if raw == "" {
		return "", nil
	}

	var payload map[string]string
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return "", ErrInvalidAppearancePreferences
	}

	normalized := make(map[string]string, len(payload))
	for key, value := range payload {
		switch key {
		case "theme":
			if value != "light" && value != "dark" && value != "system" {
				return "", ErrInvalidAppearancePreferences
			}
			normalized[key] = value
		case "preset":
			if value != "default" && value != "azure" && value != "cobalt" && value != "graphite" && value != "lagoon" && value != "ink" && value != "ochre" && value != "sepia" {
				return "", ErrInvalidAppearancePreferences
			}
			normalized[key] = value
		case "chatFont":
			if value != "default" && value != "songti" && value != "heiti" && value != "mono" {
				return "", ErrInvalidAppearancePreferences
			}
			normalized[key] = value
		case "chatFontWeight":
			if value != "regular" && value != "medium" && value != "semibold" && value != "bold" {
				return "", ErrInvalidAppearancePreferences
			}
			normalized[key] = value
		case "fontSize":
			if value != "small" && value != "standard" && value != "medium" && value != "large" {
				value = "standard"
			}
			normalized[key] = value
		default:
			return "", ErrInvalidAppearancePreferences
		}
	}

	next, err := json.Marshal(normalized)
	if err != nil {
		return "", ErrInvalidAppearancePreferences
	}
	return string(next), nil
}

func normalizeLocale(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "en-US", nil
	}
	normalized := strings.ReplaceAll(trimmed, "_", "-")
	switch normalized {
	case "en", "en-US", "zh", "zh-CN":
		return normalized, nil
	default:
		return "", ErrInvalidLocale
	}
}

// UpdateUsernameOnce 修改当前用户用户名，仅允许自主修改一次。
func (s *Service) UpdateUsernameOnce(ctx context.Context, userID uint, input UpdateUsernameInput) (*domainuser.User, error) {
	username, err := normalizeEditableUsername(input.Username)
	if err != nil {
		return nil, err
	}
	current, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if shouldRequireInitialUsername(*current, s.cfg.Snapshot().AdminUsername) &&
		strings.EqualFold(strings.TrimSpace(current.Username), username) {
		return nil, ErrInvalidUsername
	}
	item, err := s.repo.UpdateUsernameOnce(ctx, userID, username, time.Now())
	if errors.Is(err, repository.ErrDuplicateUsername) || errors.Is(err, repository.ErrDuplicate) {
		return nil, ErrUsernameTaken
	}
	if errors.Is(err, repository.ErrConflict) {
		return nil, ErrUsernameChangeUsed
	}
	return item, err
}

// DeleteAccount 删除当前用户账户及主要用户域数据。
func (s *Service) DeleteAccount(
	ctx context.Context,
	userID uint,
	verificationMethod string,
	code string,
	requestID string,
	auditCtx requestmeta.SessionAuditContext,
) error {
	item, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	if item.Role == domainuser.RoleSuperAdmin {
		return ErrDeleteSuperAdminNotAllowed
	}
	methods, err := s.resolveSecurityVerificationMethods(ctx, item)
	if err != nil {
		return err
	}
	method := methods[0]
	if normalizedMethod := normalizeSecurityVerificationMethod(verificationMethod); normalizedMethod != "" {
		method = normalizedMethod
	}
	if method == SecurityVerificationMethodNone {
		return ErrAccountDeleteVerificationRequired
	}
	if !containsSecurityVerificationMethod(methods, method) {
		return fmt.Errorf("verification method is unavailable")
	}
	normalizedEmail := ""
	if method == SecurityVerificationMethodEmail {
		normalizedEmail, err = normalizeRegistrationEmail(item.Email)
		if err != nil {
			return fmt.Errorf("user email is invalid")
		}
	}
	if err = s.verifySecurityCodeWithMethod(ctx, item, method, domainuser.ContactVerificationPurposeAccountDelete, normalizedEmail, code, time.Now()); err != nil {
		return fmt.Errorf("verification code is invalid or expired")
	}

	normalizedAuditCtx := s.resolveSessionAuditContext(ctx, auditCtx)
	storagePaths, err := s.repo.ListDistinctFileStoragePathsByUserID(ctx, userID)
	if err != nil {
		s.RecordAuthEvent(
			ctx,
			userID,
			requestID,
			"account_delete",
			"failure",
			"list_storage_paths_failed",
			normalizedAuditCtx.ClientIP,
			normalizedAuditCtx.UserAgent,
			marshalAuthEventDetail(map[string]interface{}{
				"user_id":   userID,
				"username":  item.Username,
				"public_id": item.PublicID,
			}),
		)
		return err
	}

	if err = s.repo.DeleteAccountHard(ctx, userID); err != nil {
		s.RecordAuthEvent(
			ctx,
			userID,
			requestID,
			"account_delete",
			"failure",
			"delete_account_failed",
			normalizedAuditCtx.ClientIP,
			normalizedAuditCtx.UserAgent,
			marshalAuthEventDetail(map[string]interface{}{
				"user_id":            userID,
				"username":           item.Username,
				"public_id":          item.PublicID,
				"storage_file_count": len(storagePaths),
			}),
		)
		return err
	}

	failedPaths := s.cleanupDeletedAccountFiles(ctx, storagePaths)
	s.RecordAuthEvent(
		ctx,
		userID,
		requestID,
		"account_delete",
		"success",
		"",
		normalizedAuditCtx.ClientIP,
		normalizedAuditCtx.UserAgent,
		marshalAuthEventDetail(map[string]interface{}{
			"user_id":                  userID,
			"username":                 item.Username,
			"public_id":                item.PublicID,
			"client_ip":                normalizedAuditCtx.ClientIP,
			"location":                 normalizedAuditCtx.LocationLabel(),
			"storage_file_count":       len(storagePaths),
			"storage_cleanup_failures": len(failedPaths),
		}),
	)
	if len(failedPaths) > 0 {
		s.RecordAuthEvent(
			ctx,
			userID,
			requestID,
			"account_delete_cleanup",
			"failure",
			"storage_cleanup_failed",
			normalizedAuditCtx.ClientIP,
			normalizedAuditCtx.UserAgent,
			marshalAuthEventDetail(map[string]interface{}{
				"user_id":           userID,
				"failed_path_count": len(failedPaths),
				"failed_paths":      trimStringSlice(failedPaths, 10),
			}),
		)
	}

	return nil
}

// RequestAccountDeleteVerification 发送当前用户删除账号前的安全验证码。
func (s *Service) RequestAccountDeleteVerification(ctx context.Context, userID uint, requestedMethod string, requestID string, auditCtx requestmeta.SessionAuditContext) (*EmailChangeVerificationStartResult, error) {
	item, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if item.Role == domainuser.RoleSuperAdmin {
		return nil, ErrDeleteSuperAdminNotAllowed
	}
	methods, err := s.resolveSecurityVerificationMethods(ctx, item)
	if err != nil {
		return nil, err
	}
	method := methods[0]
	if normalizedMethod := normalizeSecurityVerificationMethod(requestedMethod); normalizedMethod != "" {
		method = normalizedMethod
	}
	if method == SecurityVerificationMethodNone {
		return nil, ErrAccountDeleteVerificationRequired
	}
	if !containsSecurityVerificationMethod(methods, method) {
		return nil, fmt.Errorf("verification method is unavailable")
	}
	if method != SecurityVerificationMethodEmail {
		return &EmailChangeVerificationStartResult{Sent: false, Method: method, AvailableMethods: methods}, nil
	}
	normalizedEmail, err := normalizeRegistrationEmail(item.Email)
	if err != nil {
		return nil, fmt.Errorf("user email is invalid")
	}
	return s.requestEmailVerificationCode(ctx, userID, domainuser.ContactVerificationPurposeAccountDelete, normalizedEmail, "account_delete_code", requestID, auditCtx)
}

// cleanupDeletedAccountFiles 从对象存储删除用户文件，返回删除失败的路径列表。
func (s *Service) cleanupDeletedAccountFiles(ctx context.Context, storagePaths []string) []string {
	if s.storeProvider == nil {
		s.storeProvider = appstorage.NewRuntimeProvider(s.cfg, nil)
	}
	store, err := s.storeProvider.Open(ctx)
	if err != nil {
		return append([]string(nil), storagePaths...)
	}
	seen := make(map[string]struct{}, len(storagePaths))
	failed := make([]string, 0)
	for _, rawPath := range storagePaths {
		normalizedPath := strings.TrimSpace(rawPath)
		if normalizedPath == "" {
			continue
		}
		if _, ok := seen[normalizedPath]; ok {
			continue
		}
		seen[normalizedPath] = struct{}{}

		if err := store.Delete(ctx, normalizedPath); err != nil {
			failed = append(failed, normalizedPath)
		}
	}
	sort.Strings(failed)
	return failed
}

// trimStringSlice 截取切片前 limit 个元素，不超出原始长度。
func trimStringSlice(items []string, limit int) []string {
	if limit <= 0 || len(items) <= limit {
		return items
	}
	return append([]string(nil), items[:limit]...)
}

// Refresh 使用 refresh token 轮换并签发新的 access/refresh。
func (s *Service) Refresh(
	ctx context.Context,
	refreshToken string,
	requestID string,
	auditCtx requestmeta.SessionAuditContext,
) (*LoginResult, error) {
	normalizedAuditCtx := s.resolveSessionAuditContext(ctx, auditCtx)
	trimmedRefreshToken := strings.TrimSpace(refreshToken)
	cfg := s.cfg.Snapshot()
	claims, err := token.Parse(cfg.JWTSecret, trimmedRefreshToken)
	if err != nil {
		s.RecordAuthEvent(ctx, 0, requestID, "token_refresh", "failure", "invalid_refresh_token_parse", normalizedAuditCtx.ClientIP, normalizedAuditCtx.UserAgent, "")
		return nil, ErrInvalidRefreshToken
	}
	if claims.TokenType != "refresh" || claims.SessionID == "" || claims.UserID == 0 {
		s.RecordAuthEvent(ctx, claims.UserID, requestID, "token_refresh", "failure", "invalid_refresh_claims", normalizedAuditCtx.ClientIP, normalizedAuditCtx.UserAgent, "")
		return nil, ErrInvalidRefreshToken
	}

	session, err := s.repo.GetSessionByUserAndSessionID(ctx, claims.UserID, claims.SessionID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			s.RecordAuthEvent(ctx, claims.UserID, requestID, "token_refresh", "failure", "session_not_found", normalizedAuditCtx.ClientIP, normalizedAuditCtx.UserAgent, "")
			return nil, ErrInvalidRefreshToken
		}
		return nil, err
	}
	if session.RevokedAt != nil || time.Now().After(session.ExpiresAt) {
		s.RecordAuthEvent(ctx, claims.UserID, requestID, "token_refresh", "failure", "session_revoked_or_expired", normalizedAuditCtx.ClientIP, normalizedAuditCtx.UserAgent, "")
		return nil, ErrSessionRevoked
	}

	userItem, err := s.repo.GetByID(ctx, claims.UserID)
	if err != nil {
		return nil, err
	}
	if userItem.Status != domainuser.StatusActive {
		s.RecordAuthEvent(ctx, claims.UserID, requestID, "token_refresh", "failure", "user_not_active", normalizedAuditCtx.ClientIP, normalizedAuditCtx.UserAgent, "")
		return nil, ErrSessionRevoked
	}

	now := time.Now()
	tokenBundle, err := s.buildSessionTokenPair(userItem, claims.SessionID, now)
	if err != nil {
		return nil, err
	}

	if err = s.repo.RotateSessionTokens(
		ctx,
		repository.RotateSessionTokensInput{
			UserID:               userItem.ID,
			SessionID:            claims.SessionID,
			PresentedRefreshHash: hashToken(trimmedRefreshToken),
			NextRefreshHash:      hashToken(tokenBundle.RefreshToken),
			NextAccessJTI:        tokenBundle.AccessJTI,
			IssuedAt:             now,
			ExpiresAt:            tokenBundle.RefreshExpiresAt,
			Now:                  now,
			PreviousTokenGrace:   refreshTokenPreviousHashGrace,
		},
	); err != nil {
		if errors.Is(err, repository.ErrInvalidInput) {
			s.RecordAuthEvent(ctx, claims.UserID, requestID, "token_refresh", "failure", "refresh_token_hash_mismatch", normalizedAuditCtx.ClientIP, normalizedAuditCtx.UserAgent, "")
			return nil, ErrInvalidRefreshToken
		}
		return nil, err
	}

	sessionSnapshot := buildSessionAuditSnapshot(normalizedAuditCtx)
	if err = s.repo.TouchSessionActivity(ctx, userItem.ID, claims.SessionID, sessionActivityInputFromSnapshot(sessionSnapshot, now)); err != nil {
		return nil, err
	}

	s.RecordAuthEvent(
		ctx,
		userItem.ID,
		requestID,
		"token_refresh",
		"success",
		"",
		sessionSnapshot.ClientIP,
		sessionSnapshot.UserAgent,
		marshalSessionAuthEventDetail(claims.SessionID, sessionSnapshot),
	)

	userView, err := s.buildUserView(ctx, *userItem)
	if err != nil {
		return nil, err
	}

	return &LoginResult{
		AccessToken:      tokenBundle.AccessToken,
		RefreshToken:     tokenBundle.RefreshToken,
		SessionID:        claims.SessionID,
		ExpiresAt:        tokenBundle.ExpiresAt,
		RefreshExpiresAt: tokenBundle.RefreshExpiresAt,
		User:             userView,
	}, nil
}

// Logout 吊销当前会话。
func (s *Service) Logout(
	ctx context.Context,
	userID uint,
	sessionID string,
	requestID string,
	auditCtx requestmeta.SessionAuditContext,
) error {
	normalizedSessionID := strings.TrimSpace(sessionID)
	normalizedAuditCtx := auditCtx.Normalize()
	sessionSnapshot := buildSessionAuditSnapshot(normalizedAuditCtx)
	if normalizedSessionID == "" {
		return ErrSessionRevoked
	}

	if err := s.repo.RevokeSession(ctx, userID, normalizedSessionID, "user_logout"); err != nil {
		s.RecordAuthEvent(
			ctx,
			userID,
			requestID,
			"logout",
			"failure",
			"revoke_session_failed",
			normalizedAuditCtx.ClientIP,
			normalizedAuditCtx.UserAgent,
			marshalSessionAuthEventDetail(normalizedSessionID, sessionSnapshot),
		)
		return err
	}

	s.RecordAuthEvent(
		ctx,
		userID,
		requestID,
		"logout",
		"success",
		"",
		normalizedAuditCtx.ClientIP,
		normalizedAuditCtx.UserAgent,
		marshalSessionAuthEventDetail(normalizedSessionID, sessionSnapshot),
	)

	return nil
}

// LogoutAll 吊销用户所有会话。
func (s *Service) LogoutAll(
	ctx context.Context,
	userID uint,
	requestID string,
	auditCtx requestmeta.SessionAuditContext,
) error {
	normalizedAuditCtx := auditCtx.Normalize()
	if err := s.repo.RevokeAllSessions(ctx, userID, "user_logout_all"); err != nil {
		s.RecordAuthEvent(ctx, userID, requestID, "logout_all", "failure", "revoke_all_sessions_failed", normalizedAuditCtx.ClientIP, normalizedAuditCtx.UserAgent, "")
		return err
	}

	s.RecordAuthEvent(ctx, userID, requestID, "logout_all", "success", "", normalizedAuditCtx.ClientIP, normalizedAuditCtx.UserAgent, "")
	return nil
}

// ValidateAccessSession 校验 access token 绑定会话有效性。
func (s *Service) ValidateAccessSession(
	ctx context.Context,
	userID uint,
	sessionID string,
	accessIssuedAt time.Time,
	auditCtx requestmeta.SessionAuditContext,
) error {
	if userID == 0 || strings.TrimSpace(sessionID) == "" || accessIssuedAt.IsZero() {
		return ErrSessionRevoked
	}

	session, err := s.repo.GetSessionByUserAndSessionID(ctx, userID, strings.TrimSpace(sessionID))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrSessionRevoked
		}
		return err
	}
	if session.RevokedAt != nil || time.Now().After(session.ExpiresAt) {
		return ErrSessionRevoked
	}
	if accessIssuedAt.Add(accessTokenSessionClockSkew).Before(session.CreatedAt) {
		return ErrSessionRevoked
	}

	now := time.Now()
	sessionSnapshot := buildSessionAuditSnapshot(auditCtx)
	if shouldTouchSessionActivity(session, sessionSnapshot, now) {
		if err = s.repo.TouchSessionActivity(ctx, userID, strings.TrimSpace(sessionID), sessionActivityInputFromSnapshot(sessionSnapshot, now)); err != nil {
			return err
		}
	}

	return nil
}

// ListCurrentActiveSessions 查询当前用户仍然有效的活跃会话。
func (s *Service) ListCurrentActiveSessions(
	ctx context.Context,
	userID uint,
	currentSessionID string,
) ([]ActiveSessionResult, error) {
	items, err := s.repo.ListActiveSessionsByUserID(ctx, userID, time.Now())
	if err != nil {
		return nil, err
	}

	results := make([]ActiveSessionResult, 0, len(items))
	for _, item := range items {
		session := item
		if enrichedSession, err := s.ensureSessionGeoResolved(ctx, userID, &session); err == nil && enrichedSession != nil {
			session = *enrichedSession
		}
		results = append(results, ActiveSessionResult{
			SessionID:        session.SessionID,
			Current:          strings.TrimSpace(session.SessionID) == strings.TrimSpace(currentSessionID),
			DeviceLabel:      resolveSessionDeviceLabel(&session),
			DeviceName:       session.DeviceName,
			BrowserName:      session.BrowserName,
			OSName:           session.OSName,
			DeviceType:       session.DeviceType,
			ClientIP:         session.ClientIP,
			LocationLabel:    resolveSessionLocationLabel(&session),
			GeoSource:        session.GeoSource,
			GeoAccuracy:      session.GeoAccuracy,
			CountryCode:      session.CountryCode,
			RegionName:       session.RegionName,
			CityName:         session.CityName,
			TimezoneName:     session.TimezoneName,
			IPLatitude:       session.IPLatitude,
			IPLongitude:      session.IPLongitude,
			PreciseLatitude:  session.PreciseLatitude,
			PreciseLongitude: session.PreciseLongitude,
			PreciseAccuracyM: session.PreciseAccuracyM,
			PreciseLocatedAt: session.PreciseLocatedAt,
			CreatedAt:        session.CreatedAt,
			UpdatedAt:        session.UpdatedAt,
			LastSeenAt:       session.LastSeenAt,
			ExpiresAt:        session.ExpiresAt,
		})
	}

	sort.SliceStable(results, func(i int, j int) bool {
		if results[i].Current != results[j].Current {
			return results[i].Current
		}
		return results[i].UpdatedAt.After(results[j].UpdatedAt)
	})

	return results, nil
}

// ensureSessionGeoResolved 若会话尚无地理信息则通过 GeoIP 补全，并持久化到数据库。
func (s *Service) ensureSessionGeoResolved(
	ctx context.Context,
	userID uint,
	session *domainuser.Session,
) (*domainuser.Session, error) {
	if session == nil {
		return nil, nil
	}
	if session.CountryCode != "" || session.RegionName != "" || session.CityName != "" || s.geoResolver == nil {
		return session, nil
	}

	enriched, err := s.geoResolver.Lookup(ctx, session.ClientIP)
	if err != nil {
		return session, err
	}
	merged := mergeSessionAuditContext(
		requestmeta.SessionAuditContext{
			ClientIP:     session.ClientIP,
			UserAgent:    session.UserAgent,
			GeoSource:    session.GeoSource,
			GeoAccuracy:  session.GeoAccuracy,
			CountryCode:  session.CountryCode,
			RegionName:   session.RegionName,
			CityName:     session.CityName,
			TimezoneName: session.TimezoneName,
			IPLatitude:   session.IPLatitude,
			IPLongitude:  session.IPLongitude,
		},
		enriched,
	)
	if err = s.repo.TouchSessionActivity(ctx, userID, session.SessionID, repository.UpdateSessionActivityInput{
		GeoSource:    &merged.GeoSource,
		GeoAccuracy:  &merged.GeoAccuracy,
		CountryCode:  &merged.CountryCode,
		RegionName:   &merged.RegionName,
		CityName:     &merged.CityName,
		TimezoneName: &merged.TimezoneName,
		IPLatitude:   &merged.IPLatitude,
		IPLongitude:  &merged.IPLongitude,
	}); err != nil {
		return session, err
	}
	session.GeoSource = merged.GeoSource
	session.GeoAccuracy = merged.GeoAccuracy
	session.CountryCode = merged.CountryCode
	session.RegionName = merged.RegionName
	session.CityName = merged.CityName
	session.TimezoneName = merged.TimezoneName
	session.IPLatitude = merged.IPLatitude
	session.IPLongitude = merged.IPLongitude
	return session, nil
}

// UpdateCurrentSessionLocation 更新当前登录会话的精确位置，成功时记录认证事件。
func (s *Service) UpdateCurrentSessionLocation(
	ctx context.Context,
	userID uint,
	sessionID string,
	requestID string,
	auditCtx requestmeta.SessionAuditContext,
	input UpdateCurrentSessionLocationInput,
) (*ActiveSessionResult, error) {
	normalizedSessionID := strings.TrimSpace(sessionID)
	if userID == 0 || normalizedSessionID == "" {
		return nil, ErrSessionRevoked
	}
	if input.Latitude < -90 || input.Latitude > 90 || input.Longitude < -180 || input.Longitude > 180 {
		return nil, ErrInvalidLocation
	}
	if input.AccuracyMeters != nil && *input.AccuracyMeters < 0 {
		return nil, ErrInvalidLocation
	}

	timezoneName := strings.TrimSpace(input.Timezone)
	if timezoneName != "" {
		if _, err := time.LoadLocation(timezoneName); err != nil {
			return nil, ErrInvalidTimeZone
		}
	}

	now := time.Now()
	geoSource := "browser_geolocation"
	geoAccuracy := "precise"
	updateInput := repository.UpdateSessionActivityInput{
		GeoSource:        &geoSource,
		GeoAccuracy:      &geoAccuracy,
		PreciseLatitude:  &input.Latitude,
		PreciseLongitude: &input.Longitude,
		PreciseLocatedAt: &now,
		LastSeenAt:       &now,
	}
	if input.AccuracyMeters != nil {
		updateInput.PreciseAccuracyM = input.AccuracyMeters
	}
	if timezoneName != "" {
		updateInput.TimezoneName = &timezoneName
	}

	if err := s.repo.TouchSessionActivity(ctx, userID, normalizedSessionID, updateInput); err != nil {
		return nil, err
	}

	results, err := s.ListCurrentActiveSessions(ctx, userID, normalizedSessionID)
	if err != nil {
		return nil, err
	}
	for _, item := range results {
		if strings.TrimSpace(item.SessionID) == normalizedSessionID {
			session := item
			normalizedAuditCtx := s.resolveSessionAuditContext(ctx, auditCtx)
			s.RecordAuthEvent(
				ctx, userID, requestID, "session_location_update", "success", "",
				normalizedAuditCtx.ClientIP, normalizedAuditCtx.UserAgent,
				marshalAuthEventDetail(map[string]interface{}{
					"session_id":              normalizedSessionID,
					"precise_latitude":        session.PreciseLatitude,
					"precise_longitude":       session.PreciseLongitude,
					"precise_accuracy_meters": session.PreciseAccuracyM,
					"timezone_name":           session.TimezoneName,
				}),
			)
			return &session, nil
		}
	}
	return nil, repository.ErrNotFound
}

// RecordAuthEvent 写入认证事件。
func (s *Service) RecordAuthEvent(
	ctx context.Context,
	userID uint,
	requestID string,
	eventType string,
	result string,
	reason string,
	clientIP string,
	userAgent string,
	detailJSON string,
) {
	if err := s.repo.RecordAuthEvent(
		ctx,
		userID,
		requestID,
		eventType,
		result,
		reason,
		clientIP,
		userAgent,
		detailJSON,
	); err != nil {
		s.warn("record_auth_event_failed",
			zap.Uint("user_id", userID),
			zap.String("event", eventType),
			zap.Error(err),
		)
	}
}

type issuedTokens struct {
	AccessToken      string
	RefreshToken     string
	AccessJTI        string
	ExpiresAt        time.Time
	RefreshExpiresAt time.Time
}

// loginLockThreshold 返回触发账户锁定的连续失败次数阈值，默认 5。
func (s *Service) loginLockThreshold() int {
	cfg := s.cfg.Snapshot()
	if cfg.LoginMaxFailures <= 0 {
		return 5
	}
	return cfg.LoginMaxFailures
}

// loginLockDuration 返回账户锁定时长，默认 15 分钟。
func (s *Service) loginLockDuration() time.Duration {
	cfg := s.cfg.Snapshot()
	if cfg.LoginLockMinutes <= 0 {
		return 15 * time.Minute
	}
	return time.Duration(cfg.LoginLockMinutes) * time.Minute
}

// marshalAuthEventDetail 将事件详情序列化为 JSON 字符串；序列化失败时返回空字符串。
func marshalAuthEventDetail(detail interface{}) string {
	if detail == nil {
		return ""
	}
	payload, err := json.Marshal(detail)
	if err != nil {
		return ""
	}
	return string(payload)
}

// buildSessionTokenPair 签发访问令牌与刷新令牌对，TTL 从运行时配置读取。
func (s *Service) buildSessionTokenPair(user *domainuser.User, sessionID string, now time.Time) (*issuedTokens, error) {
	cfg := s.cfg.Snapshot()
	accessTTL := time.Duration(cfg.TokenTTLHours) * time.Hour
	if accessTTL <= 0 {
		accessTTL = 24 * time.Hour
	}
	refreshTTL := time.Duration(cfg.RefreshTokenTTLHours) * time.Hour
	if refreshTTL <= 0 {
		refreshTTL = 30 * 24 * time.Hour
	}

	accessJTI := conv.NormalizePublicID(uuid.NewString())
	refreshJTI := conv.NormalizePublicID(uuid.NewString())

	accessToken, err := token.GenerateWithClaims(
		cfg.JWTSecret,
		user.ID,
		user.Username,
		user.Role,
		sessionID,
		accessJTI,
		"access",
		accessTTL,
	)
	if err != nil {
		return nil, err
	}
	refreshToken, err := token.GenerateWithClaims(
		cfg.JWTSecret,
		user.ID,
		user.Username,
		user.Role,
		sessionID,
		refreshJTI,
		"refresh",
		refreshTTL,
	)
	if err != nil {
		return nil, err
	}

	return &issuedTokens{
		AccessToken:      accessToken,
		RefreshToken:     refreshToken,
		AccessJTI:        accessJTI,
		ExpiresAt:        now.Add(accessTTL),
		RefreshExpiresAt: now.Add(refreshTTL),
	}, nil
}

// hashToken 返回 token 的 SHA-256 十六进制摘要，用于数据库中安全存储。
func hashToken(raw string) string {
	value := sha256.Sum256([]byte(strings.TrimSpace(raw)))
	return hex.EncodeToString(value[:])
}

func normalizeEditableUsername(raw string) (string, error) {
	username, err := userapp.NormalizeUsername(raw)
	if err != nil {
		return "", ErrInvalidUsername
	}
	return username, nil
}

// validateAvatarURL 校验头像 URL 合法性；空值、相对路径和 generated: 前缀均视为合法。
func validateAvatarURL(raw string) error {
	if raw == "" || strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "generated:github:") {
		return nil
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return ErrInvalidAvatarURL
	}
	if (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return ErrInvalidAvatarURL
	}
	return nil
}
