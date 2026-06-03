package user

import "time"

const (
	// RoleSuperAdmin 是超级管理员角色。
	RoleSuperAdmin = "superadmin"
	// RoleAdmin 是后台管理员角色。
	RoleAdmin = "admin"
	// RoleUser 是普通用户角色。
	RoleUser = "user"
)

func IsAdminRole(role string) bool {
	return role == RoleAdmin || role == RoleSuperAdmin
}

const (
	// StatusPendingActivation 表示用户待激活。
	StatusPendingActivation = "pending_activation"
	// StatusActive 表示用户可正常登录。
	StatusActive = "active"
	// StatusLocked 表示用户被临时锁定。
	StatusLocked = "locked"
	// StatusSuspended 表示用户被管理员停用。
	StatusSuspended = "suspended"
	// StatusDeactivated 表示用户被逻辑注销。
	StatusDeactivated = "deactivated"
)

// User 表示用户账号聚合根。
type User struct {
	ID                    uint
	PublicID              string
	Username              string
	DisplayName           string
	AvatarURL             string
	Email                 string
	Phone                 string
	Role                  string
	Status                string
	Timezone              string
	Locale                string
	ProfilePreferences    string
	AppearancePreferences string
	OnboardingCompletedAt *time.Time
	EmailVerifiedAt       *time.Time
	EmailSource           string
	EmailBootstrapUsedAt  *time.Time
	PhoneVerifiedAt       *time.Time
	UsernameChangedAt     *time.Time
	LastLoginAt           *time.Time
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

// Credential 表示用户登录凭据。
type Credential struct {
	ID                uint
	UserID            uint
	PasswordHash      string
	PasswordAlgo      string
	PasswordEnabled   bool
	PasswordUpdatedAt *time.Time
	PasswordSetAt     *time.Time
	PasswordOrigin    string
	MustResetPassword bool
	FailedLoginCount  int
	LockedUntil       *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// UserTwoFactor 表示用户双因素认证配置。
type UserTwoFactor struct {
	ID                     uint
	UserID                 uint
	TOTPEnabled            bool
	TOTPSecretEncrypted    string
	TOTPSetupExpiresAt     *time.Time
	RecoveryCodesHash      string
	Enforced               bool
	EnabledAt              *time.Time
	LastVerifiedAt         *time.Time
	TrustedDeviceExpiresAt *time.Time
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

// Session 表示用户登录会话。
type Session struct {
	ID                       uint
	SessionID                string
	UserID                   uint
	RefreshTokenHash         string
	PreviousRefreshTokenHash string
	RefreshRotatedAt         *time.Time
	AccessJTI                string
	ClientIP                 string
	UserAgent                string
	DeviceName               string
	BrowserName              string
	OSName                   string
	DeviceType               string
	GeoSource                string
	GeoAccuracy              string
	CountryCode              string
	RegionName               string
	CityName                 string
	TimezoneName             string
	IPLatitude               *float64
	IPLongitude              *float64
	PreciseLatitude          *float64
	PreciseLongitude         *float64
	PreciseAccuracyM         *float64
	PreciseLocatedAt         *time.Time
	IssuedAt                 time.Time
	LastSeenAt               *time.Time
	ExpiresAt                time.Time
	RevokedAt                *time.Time
	RevokeReason             string
	CreatedAt                time.Time
	UpdatedAt                time.Time
}

// AuthEvent 表示认证事件日志。
type AuthEvent struct {
	ID         uint
	RequestID  string
	UserID     uint
	EventType  string
	Result     string
	Reason     string
	ClientIP   string
	UserAgent  string
	DetailJSON string
	OccurredAt time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// ContactVerification 表示邮箱/手机号验证请求。
type ContactVerification struct {
	ID           uint
	UserID       uint
	Channel      string
	Purpose      string
	Target       string
	Token        string
	CodeHash     string
	Status       string
	SentAt       *time.Time
	ExpiresAt    *time.Time
	VerifiedAt   *time.Time
	ConsumedAt   *time.Time
	AttemptCount int
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

const (
	ContactVerificationChannelEmail              = "email"
	ContactVerificationPurposeRegister           = "register"
	ContactVerificationPurposePasswordChange     = "password_change"
	ContactVerificationPurposeLogin              = "login"
	ContactVerificationPurposeAccountDelete      = "account_delete"
	ContactVerificationPurposeEmailChangeCurrent = "email_change_current"
	ContactVerificationPurposeEmailChangeNew     = "email_change_new"
	ContactVerificationPurposeEmailBootstrapNew  = "email_bootstrap_new"
	ContactVerificationPurposeEmailVerifyCurrent = "email_verify_current"
	ContactVerificationPurposeIdentityBind       = "identity_bind"
	ContactVerificationStatusPending             = "pending"
	ContactVerificationStatusVerified            = "verified"
	ContactVerificationStatusCanceled            = "canceled"
)

const (
	EmailSourceLocalRegister      = "local_register"
	EmailSourceUserSet            = "user_set"
	EmailSourceProviderVerified   = "provider_verified"
	EmailSourceProviderUnverified = "provider_unverified"
	EmailSourceAdminSet           = "admin_set"
)

const (
	PasswordOriginLocalRegister  = "local_register"
	PasswordOriginAdminCreated   = "admin_created"
	PasswordOriginAdminReset     = "admin_reset"
	PasswordOriginUserSet        = "user_set"
	PasswordOriginSSOPlaceholder = "sso_placeholder"
)

const (
	IdentityProviderTypeOIDC   = "oidc"
	IdentityProviderTypeOAuth2 = "oauth2"
)

// IdentityProvider 表示企业登录身份源配置。
type IdentityProvider struct {
	ID                  uint
	PublicID            string
	Type                string
	Name                string
	Slug                string
	LogoURL             string
	LoginEnabled        bool
	RegistrationEnabled bool
	ClientID            string
	ClientSecret        string
	IssuerURL           string
	DiscoveryURL        string
	AuthURL             string
	TokenURL            string
	UserInfoURL         string
	JWKSURL             string
	Scopes              string
	PKCEEnabled         bool
	DefaultRole         string
	SubjectField        string
	EmailField          string
	EmailVerifiedField  string
	NameField           string
	AvatarField         string
	SortOrder           int
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// UserIdentity 表示用户绑定的第三方身份。
type UserIdentity struct {
	ID                  uint
	UserID              uint
	ProviderID          uint
	ProviderType        string
	ProviderSubject     string
	ProviderDisplayName string
	Email               string
	EmailVerified       bool
	ProfileJSON         string
	LinkedAt            time.Time
	LastLoginAt         *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
}
