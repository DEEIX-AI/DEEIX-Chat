package repository

import (
	"context"
	"time"

	domainbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/billing"
	domainuser "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/user"
)

// UpdateUserFieldsInput 定义用户基础资料更新字段。
type UpdateUserFieldsInput struct {
	AvatarURL             *string
	DisplayName           *string
	Email                 *string
	EmailVerifiedAt       **time.Time
	EmailSource           *string
	EmailBootstrapUsedAt  **time.Time
	Phone                 *string
	PhoneVerifiedAt       **time.Time
	Role                  *string
	Timezone              *string
	Locale                *string
	ProfilePreferences    *string
	AppearancePreferences *string
	OnboardingCompletedAt **time.Time
}

// UpdateUserTwoFactorInput 定义用户二次验证配置更新字段。
type UpdateUserTwoFactorInput struct {
	TOTPEnabled            *bool
	TOTPSetupExpiresAt     **time.Time
	RecoveryCodesHash      *string
	ExpectedRecoveryHash   *string
	Enforced               *bool
	EnabledAt              **time.Time
	LastVerifiedAt         **time.Time
	TrustedDeviceExpiresAt **time.Time
}

// UpdateSessionActivityInput 定义会话活动元数据更新字段。
type UpdateSessionActivityInput struct {
	LastSeenAt       *time.Time
	ClientIP         *string
	UserAgent        *string
	DeviceName       *string
	BrowserName      *string
	OSName           *string
	DeviceType       *string
	GeoSource        *string
	GeoAccuracy      *string
	CountryCode      *string
	RegionName       *string
	CityName         *string
	TimezoneName     *string
	IPLatitude       **float64
	IPLongitude      **float64
	PreciseLatitude  *float64
	PreciseLongitude *float64
	PreciseAccuracyM *float64
	PreciseLocatedAt *time.Time
}

// RotateSessionTokensInput 定义 refresh token 轮换所需的会话状态。
type RotateSessionTokensInput struct {
	UserID               uint
	SessionID            string
	PresentedRefreshHash string
	NextRefreshHash      string
	NextAccessJTI        string
	IssuedAt             time.Time
	ExpiresAt            time.Time
	Now                  time.Time
	PreviousTokenGrace   time.Duration
}

// UpdateIdentityProviderInput 定义第三方身份提供方更新字段。
type UpdateIdentityProviderInput struct {
	Type                *string
	Name                *string
	Slug                *string
	LogoURL             *string
	LoginEnabled        *bool
	RegistrationEnabled *bool
	ClientID            *string
	ClientSecret        *string
	IssuerURL           *string
	DiscoveryURL        *string
	AuthURL             *string
	TokenURL            *string
	UserInfoURL         *string
	JWKSURL             *string
	Scopes              *string
	PKCEEnabled         *bool
	DefaultRole         *string
	SubjectField        *string
	EmailField          *string
	EmailVerifiedField  *string
	NameField           *string
	AvatarField         *string
}

// IsZero 判断是否没有任何身份提供方字段更新。
func (input UpdateIdentityProviderInput) IsZero() bool {
	return input.Type == nil &&
		input.Name == nil &&
		input.Slug == nil &&
		input.LogoURL == nil &&
		input.LoginEnabled == nil &&
		input.RegistrationEnabled == nil &&
		input.ClientID == nil &&
		input.ClientSecret == nil &&
		input.IssuerURL == nil &&
		input.DiscoveryURL == nil &&
		input.AuthURL == nil &&
		input.TokenURL == nil &&
		input.UserInfoURL == nil &&
		input.JWKSURL == nil &&
		input.Scopes == nil &&
		input.PKCEEnabled == nil &&
		input.DefaultRole == nil &&
		input.SubjectField == nil &&
		input.EmailField == nil &&
		input.EmailVerifiedField == nil &&
		input.NameField == nil &&
		input.AvatarField == nil
}

// IsZero 判断是否没有任何会话活动字段更新。
func (input UpdateSessionActivityInput) IsZero() bool {
	return input.LastSeenAt == nil &&
		input.ClientIP == nil &&
		input.UserAgent == nil &&
		input.DeviceName == nil &&
		input.BrowserName == nil &&
		input.OSName == nil &&
		input.DeviceType == nil &&
		input.GeoSource == nil &&
		input.GeoAccuracy == nil &&
		input.CountryCode == nil &&
		input.RegionName == nil &&
		input.CityName == nil &&
		input.TimezoneName == nil &&
		input.IPLatitude == nil &&
		input.IPLongitude == nil &&
		input.PreciseLatitude == nil &&
		input.PreciseLongitude == nil &&
		input.PreciseAccuracyM == nil &&
		input.PreciseLocatedAt == nil
}

// IsZero 判断是否没有任何二次验证字段更新。
func (input UpdateUserTwoFactorInput) IsZero() bool {
	return input.TOTPEnabled == nil &&
		input.TOTPSetupExpiresAt == nil &&
		input.RecoveryCodesHash == nil &&
		input.Enforced == nil &&
		input.EnabledAt == nil &&
		input.LastVerifiedAt == nil &&
		input.TrustedDeviceExpiresAt == nil
}

// IsZero 判断是否没有任何用户字段更新。
func (input UpdateUserFieldsInput) IsZero() bool {
	return input.AvatarURL == nil &&
		input.DisplayName == nil &&
		input.Email == nil &&
		input.EmailVerifiedAt == nil &&
		input.EmailSource == nil &&
		input.EmailBootstrapUsedAt == nil &&
		input.Phone == nil &&
		input.PhoneVerifiedAt == nil &&
		input.Role == nil &&
		input.Timezone == nil &&
		input.Locale == nil &&
		input.ProfilePreferences == nil &&
		input.AppearancePreferences == nil &&
		input.OnboardingCompletedAt == nil
}

// UserRepository 定义用户域依赖的持久化能力。
type UserRepository interface {
	GetByUsername(ctx context.Context, username string) (*domainuser.User, error)
	GetByEmail(ctx context.Context, email string) (*domainuser.User, error)
	GetByID(ctx context.Context, userID uint) (*domainuser.User, error)
	UpdateFields(ctx context.Context, userID uint, input UpdateUserFieldsInput) (*domainuser.User, error)
	ListUsers(ctx context.Context, offset int, limit int) ([]domainuser.User, int64, error)
	CountSuperAdmins(ctx context.Context) (int64, error)
	GetActivePlanByCode(ctx context.Context, code string) (*domainbilling.Plan, error)
	GetActiveDefaultPriceByPlanID(ctx context.Context, planID uint) (*domainbilling.Price, error)
	CreateWithCredential(
		ctx context.Context,
		user *domainuser.User,
		credential domainuser.Credential,
		subscriptionPlanID uint,
		subscriptionPriceID uint,
		subscriptionEndAt *time.Time,
		autoRenew bool,
	) error
	CreateWithCredentialAndIdentity(
		ctx context.Context,
		user *domainuser.User,
		credential domainuser.Credential,
		identity *domainuser.UserIdentity,
		subscriptionPlanID uint,
		subscriptionPriceID uint,
		subscriptionEndAt *time.Time,
		autoRenew bool,
	) error
	GetCredentialByUserID(ctx context.Context, userID uint) (*domainuser.Credential, error)
	GetUserTwoFactorByUserID(ctx context.Context, userID uint) (*domainuser.UserTwoFactor, error)
	UpsertUserTwoFactor(ctx context.Context, item *domainuser.UserTwoFactor) (*domainuser.UserTwoFactor, error)
	UpdateUserTwoFactor(ctx context.Context, userID uint, input UpdateUserTwoFactorInput) (*domainuser.UserTwoFactor, error)
	DeleteUserTwoFactor(ctx context.Context, userID uint) error
	MarkLoginFailure(ctx context.Context, userID uint, lockThreshold int, lockUntil time.Time) (*domainuser.Credential, error)
	ResetLoginFailure(ctx context.Context, userID uint) error
	UpdateUserStatus(ctx context.Context, userID uint, status string) error
	UpdatePassword(ctx context.Context, userID uint, passwordHash string, passwordOrigin string, mustResetPassword bool) error
	ResetPasswordByAdmin(ctx context.Context, userID uint, passwordHash string, mustResetPassword bool) error
	MarkBootstrapSuperAdminPasswordResetRequired(ctx context.Context, username string) error
	UpdateLastLogin(ctx context.Context, userID uint) error
	DeleteAccountHard(ctx context.Context, userID uint) error
	ListDistinctFileStoragePathsByUserID(ctx context.Context, userID uint) ([]string, error)
	RecordAuthEvent(
		ctx context.Context,
		userID uint,
		requestID string,
		eventType string,
		result string,
		reason string,
		clientIP string,
		userAgent string,
		detailJSON string,
	) error
	CreateSession(ctx context.Context, item *domainuser.Session) error
	GetSessionByUserAndSessionID(ctx context.Context, userID uint, sessionID string) (*domainuser.Session, error)
	RotateSessionTokens(ctx context.Context, input RotateSessionTokensInput) error
	TouchSessionActivity(ctx context.Context, userID uint, sessionID string, input UpdateSessionActivityInput) error
	RevokeSession(ctx context.Context, userID uint, sessionID string, reason string) error
	RevokeAllSessions(ctx context.Context, userID uint, reason string) error
	ListActiveSessionsByUserID(ctx context.Context, userID uint, now time.Time) ([]domainuser.Session, error)
	HasActiveSuperAdminIdentity(ctx context.Context) (bool, error)
	ListAuthEvents(ctx context.Context, userID uint, eventType string, result string, offset int, limit int) ([]domainuser.AuthEvent, int64, error)
	ListIdentityProviders(ctx context.Context, includeDisabled bool) ([]domainuser.IdentityProvider, error)
	GetIdentityProviderByPublicID(ctx context.Context, publicID string) (*domainuser.IdentityProvider, error)
	GetIdentityProviderBySlug(ctx context.Context, slug string) (*domainuser.IdentityProvider, error)
	CreateIdentityProvider(ctx context.Context, provider *domainuser.IdentityProvider) (*domainuser.IdentityProvider, error)
	UpdateIdentityProvider(ctx context.Context, publicID string, input UpdateIdentityProviderInput) (*domainuser.IdentityProvider, error)
	UpdateIdentityProviderSortOrders(ctx context.Context, publicIDs []string) error
	DeleteIdentityProvider(ctx context.Context, publicID string, force bool) error
	ListUserIdentitiesByUserID(ctx context.Context, userID uint) ([]domainuser.UserIdentity, error)
	GetUserIdentityByProviderSubject(ctx context.Context, providerID uint, subject string) (*domainuser.UserIdentity, error)
	CreateUserIdentity(ctx context.Context, identity *domainuser.UserIdentity) (*domainuser.UserIdentity, error)
	UpdateUserIdentityLogin(ctx context.Context, identityID uint, profileJSON string, providerDisplayName string, email string, emailVerified bool) error
	DeleteUserIdentity(ctx context.Context, userID uint, identityID uint) error
	CancelPendingContactVerifications(ctx context.Context, channel string, purpose string, target string) error
	CancelPendingContactVerificationsForUser(ctx context.Context, userID uint, channel string, purpose string, target string) error
	CreateContactVerification(ctx context.Context, item *domainuser.ContactVerification) (*domainuser.ContactVerification, error)
	GetPendingContactVerification(ctx context.Context, channel string, purpose string, target string, now time.Time) (*domainuser.ContactVerification, error)
	GetPendingContactVerificationForUser(ctx context.Context, userID uint, channel string, purpose string, target string, now time.Time) (*domainuser.ContactVerification, error)
	IncrementContactVerificationAttempt(ctx context.Context, verificationID uint) error
	MarkContactVerificationVerified(ctx context.Context, verificationID uint, now time.Time) error
}
