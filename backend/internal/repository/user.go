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

// UserImportRecord 描述一次管理员导入需要原子写入的用户、凭据和余额。
type UserImportRecord struct {
	User                      domainuser.User
	Credential                domainuser.Credential
	BillingBalanceNanousd     int64
	BillingBalanceRefNo       string
	BillingBalanceDescription string
}

// OpenWebUIUserRow 描述从外部 OpenWebUI 数据库读取的用户行。
type OpenWebUIUserRow struct {
	PublicID    string
	Username    string
	DisplayName string
	Email       string
	Balance     float64
}

// UserListFilter 定义管理员用户列表过滤条件。
type UserListFilter struct {
	Query              string
	SubscriptionStatus string
	IdentityProvider   string
}

// AuthEventListInput 描述认证事件查询的筛选与分页条件。
type AuthEventListInput struct {
	UserID    uint
	EventType string
	Result    string
	Offset    int
	Limit     int
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

// UserRepository 定义用户资料与管理员用户管理流程依赖的持久化能力。
// 会话、身份提供方、二次验证与联系方式验证等认证流程能力由 AuthRepository 声明，
// 两者由同一个持久化实现满足，但各自只包含对应用例真正调用的方法。
type UserRepository interface {
	GetByUsername(ctx context.Context, username string) (*domainuser.User, error)
	GetByID(ctx context.Context, userID uint) (*domainuser.User, error)
	GetByPublicID(ctx context.Context, publicID string) (*domainuser.User, error)
	ListUsersByLowerEmails(ctx context.Context, emails []string) (map[string]domainuser.User, error)
	ListAllUsernames(ctx context.Context) ([]string, error)
	UpdateFields(ctx context.Context, userID uint, input UpdateUserFieldsInput) (*domainuser.User, error)
	ListUsers(ctx context.Context, offset int, limit int, filter UserListFilter) ([]domainuser.User, int64, error)
	CountSuperAdmins(ctx context.Context) (int64, error)
	GetActivePlanByCode(ctx context.Context, code string) (*domainbilling.Plan, error)
	GetActiveDefaultPriceByPlanID(ctx context.Context, planID uint) (*domainbilling.Price, error)
	CreateWithCredential(ctx context.Context, input CreateWithCredentialInput) error
	ImportUsersWithCredentialsAndBalances(ctx context.Context, records []UserImportRecord) ([]domainuser.User, error)
	ResetLoginFailure(ctx context.Context, userID uint) error
	UpdateUserStatus(ctx context.Context, userID uint, status string) error
	ResetPasswordByAdmin(ctx context.Context, userID uint, passwordHash string, mustResetPassword bool) error
	ListLatestSessionActivityByUserIDs(ctx context.Context, userIDs []uint) (map[uint]time.Time, error)
	DeleteAccountHard(ctx context.Context, userID uint) error
	RecordAuthEvent(ctx context.Context, input AuthEventInput) error
	RevokeAllSessions(ctx context.Context, userID uint, reason string) error
	ListAuthEvents(ctx context.Context, input AuthEventListInput) ([]domainuser.AuthEvent, int64, error)
	ListIdentityProviders(ctx context.Context, includeDisabled bool) ([]domainuser.IdentityProvider, error)
	ListUserIdentitiesByUserIDs(ctx context.Context, userIDs []uint) (map[uint][]domainuser.UserIdentity, error)
}
