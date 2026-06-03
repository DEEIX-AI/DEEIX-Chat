package model

import "time"

const (
	// RoleSuperAdmin 是超级管理员角色。
	RoleSuperAdmin = "superadmin"
	// RoleAdmin 是后台管理员角色。
	RoleAdmin = "admin"
	// RoleUser 是普通用户角色。
	RoleUser = "user"
)

const (
	// UserStatusPendingActivation 表示用户待激活。
	UserStatusPendingActivation = "pending_activation"
	// UserStatusActive 表示用户可正常登录。
	UserStatusActive = "active"
	// UserStatusLocked 表示用户被临时锁定。
	UserStatusLocked = "locked"
	// UserStatusSuspended 表示用户被管理员停用。
	UserStatusSuspended = "suspended"
	// UserStatusDeactivated 表示用户被逻辑注销。
	UserStatusDeactivated = "deactivated"
)

// User 存储账户身份与角色信息。
type User struct {
	BaseModel
	PublicID              string     `gorm:"size:32;not null;default:'';uniqueIndex:idx_identity_users_public_id;comment:公开用户ID"`
	Username              string     `gorm:"size:64;not null;uniqueIndex:idx_identity_users_username;comment:登录用户名"`
	DisplayName           string     `gorm:"size:128;not null;default:'';comment:展示名称"`
	AvatarURL             string     `gorm:"size:2048;not null;default:'';comment:头像地址"`
	Email                 string     `gorm:"size:128;not null;default:'';index:idx_identity_users_email;comment:邮箱"`
	Phone                 string     `gorm:"size:32;not null;default:'';index:idx_identity_users_phone;comment:手机号"`
	Role                  string     `gorm:"size:32;not null;default:'user';index:idx_identity_users_role;comment:角色(superadmin/admin/user)"`
	Status                string     `gorm:"size:32;not null;default:'active';index:idx_identity_users_status;comment:账户状态"`
	Timezone              string     `gorm:"size:64;not null;default:'Etc/UTC';comment:时区"`
	Locale                string     `gorm:"size:16;not null;default:'en-US';comment:语言区域"`
	ProfilePreferences    string     `gorm:"type:text;not null;default:'';comment:对话偏好"`
	AppearancePreferences string     `gorm:"type:text;not null;default:'';comment:外观偏好JSON"`
	OnboardingCompletedAt *time.Time `gorm:"index:idx_identity_users_onboarding_completed_at;comment:首次引导完成时间"`
	EmailVerifiedAt       *time.Time `gorm:"comment:邮箱验证时间"`
	EmailSource           string     `gorm:"size:32;not null;default:'';comment:邮箱来源(local_register/user_set/provider_verified/provider_unverified/admin_set)"`
	EmailBootstrapUsedAt  *time.Time `gorm:"comment:SSO 首个邮箱补齐时间"`
	PhoneVerifiedAt       *time.Time `gorm:"comment:手机号验证时间"`
	UsernameChangedAt     *time.Time `gorm:"comment:用户名自主修改时间"`
	LastLoginAt           *time.Time `gorm:"comment:最后登录时间"`
}

// TableName 指定表名。
func (User) TableName() string {
	return "identity_users"
}

const (
	// ContactVerificationChannelEmail 表示邮箱验证。
	ContactVerificationChannelEmail = "email"
	// ContactVerificationChannelPhone 表示手机号验证。
	ContactVerificationChannelPhone = "phone"
)

const (
	// ContactVerificationStatusPending 表示待验证。
	ContactVerificationStatusPending = "pending"
	// ContactVerificationStatusVerified 表示已验证。
	ContactVerificationStatusVerified = "verified"
	// ContactVerificationStatusExpired 表示已过期。
	ContactVerificationStatusExpired = "expired"
	// ContactVerificationStatusCanceled 表示已取消。
	ContactVerificationStatusCanceled = "canceled"
)

// UserContactVerification 为邮箱/手机号验证码与链接验证流程预留持久化记录。
type UserContactVerification struct {
	BaseModel
	UserID       uint       `gorm:"not null;default:0;index:idx_identity_contact_verifications_user_id;comment:用户ID"`
	Channel      string     `gorm:"size:16;not null;default:'';index:idx_identity_contact_verifications_channel;comment:验证通道(email/phone)"`
	Purpose      string     `gorm:"size:32;not null;default:'';index:idx_identity_contact_verifications_purpose;comment:验证用途(bind/change/login)"`
	Target       string     `gorm:"size:128;not null;default:'';comment:待验证目标值"`
	Token        string     `gorm:"size:96;not null;default:'';uniqueIndex:idx_identity_contact_verifications_token;comment:验证请求令牌"`
	CodeHash     string     `gorm:"size:255;not null;default:'';comment:验证码哈希"`
	Status       string     `gorm:"size:16;not null;default:'pending';index:idx_identity_contact_verifications_status;comment:验证状态"`
	SentAt       *time.Time `gorm:"comment:发送时间"`
	ExpiresAt    *time.Time `gorm:"index:idx_identity_contact_verifications_expires_at;comment:过期时间"`
	VerifiedAt   *time.Time `gorm:"comment:验证成功时间"`
	ConsumedAt   *time.Time `gorm:"comment:凭证消费时间"`
	AttemptCount int        `gorm:"not null;default:0;comment:尝试次数"`
}

// TableName 指定表名。
func (UserContactVerification) TableName() string {
	return "identity_contact_verifications"
}

// UserCredential 存储登录凭据，不暴露到业务响应。
type UserCredential struct {
	BaseModel
	UserID            uint       `gorm:"not null;default:0;uniqueIndex:idx_identity_credentials_user_id;comment:用户ID"`
	PasswordHash      string     `gorm:"size:255;not null;default:'';comment:密码哈希"`
	PasswordAlgo      string     `gorm:"size:32;not null;default:'bcrypt';comment:密码算法"`
	PasswordEnabled   bool       `gorm:"not null;default:true;comment:是否允许本地密码登录"`
	PasswordUpdatedAt *time.Time `gorm:"comment:密码更新时间"`
	PasswordSetAt     *time.Time `gorm:"comment:用户可用密码设置时间"`
	PasswordOrigin    string     `gorm:"size:32;not null;default:'local_register';comment:密码来源(local_register/admin_created/admin_reset/user_set/sso_placeholder)"`
	MustResetPassword bool       `gorm:"not null;default:false;comment:是否强制重置密码"`
	FailedLoginCount  int        `gorm:"not null;default:0;comment:连续失败登录次数"`
	LockedUntil       *time.Time `gorm:"index:idx_identity_credentials_locked_until;comment:锁定截止时间"`
}

// TableName 指定表名。
func (UserCredential) TableName() string {
	return "identity_credentials"
}

// UserSession 存储用户会话信息。
type UserSession struct {
	BaseModel
	SessionID                string     `gorm:"size:64;not null;default:'';uniqueIndex:idx_identity_sessions_session_id;comment:会话ID"`
	UserID                   uint       `gorm:"not null;default:0;index:idx_identity_sessions_user_id;comment:用户ID"`
	RefreshTokenHash         string     `gorm:"size:255;not null;default:'';comment:刷新令牌哈希"`
	PreviousRefreshTokenHash string     `gorm:"size:255;not null;default:'';comment:上一枚刷新令牌哈希"`
	RefreshRotatedAt         *time.Time `gorm:"index:idx_identity_sessions_refresh_rotated_at;comment:刷新令牌轮换时间"`
	AccessJTI                string     `gorm:"size:64;not null;default:'';index:idx_identity_sessions_access_jti;comment:访问令牌JTI"`
	ClientIP                 string     `gorm:"size:64;not null;default:'';comment:客户端IP"`
	UserAgent                string     `gorm:"size:512;not null;default:'';comment:用户代理"`
	DeviceName               string     `gorm:"size:128;not null;default:'';comment:设备名称"`
	BrowserName              string     `gorm:"size:64;not null;default:'';comment:浏览器名称"`
	OSName                   string     `gorm:"size:64;not null;default:'';comment:操作系统名称"`
	DeviceType               string     `gorm:"size:32;not null;default:'';comment:设备类型"`
	GeoSource                string     `gorm:"size:32;not null;default:'';comment:地理信息来源"`
	GeoAccuracy              string     `gorm:"size:32;not null;default:'';comment:地理信息精度"`
	CountryCode              string     `gorm:"size:32;not null;default:'';comment:国家或地区代码"`
	RegionName               string     `gorm:"size:64;not null;default:'';comment:区域名称"`
	CityName                 string     `gorm:"size:64;not null;default:'';comment:城市名称"`
	TimezoneName             string     `gorm:"size:64;not null;default:'';comment:时区名称"`
	IPLatitude               *float64   `gorm:"comment:IP定位纬度"`
	IPLongitude              *float64   `gorm:"comment:IP定位经度"`
	PreciseLatitude          *float64   `gorm:"comment:精确定位纬度"`
	PreciseLongitude         *float64   `gorm:"comment:精确定位经度"`
	PreciseAccuracyM         *float64   `gorm:"comment:精确定位精度(米)"`
	PreciseLocatedAt         *time.Time `gorm:"comment:精确定位采集时间"`
	IssuedAt                 time.Time  `gorm:"not null;comment:签发时间"`
	LastSeenAt               *time.Time `gorm:"comment:最近活跃时间"`
	ExpiresAt                time.Time  `gorm:"not null;index:idx_identity_sessions_expires_at;comment:过期时间"`
	RevokedAt                *time.Time `gorm:"index:idx_identity_sessions_revoked_at;comment:吊销时间"`
	RevokeReason             string     `gorm:"size:128;not null;default:'';comment:吊销原因"`
}

// TableName 指定表名。
func (UserSession) TableName() string {
	return "identity_sessions"
}

// UserAuthEvent 存储认证事件日志。
type UserAuthEvent struct {
	BaseModel
	RequestID  string    `gorm:"size:64;not null;default:'';index:idx_identity_auth_events_request_id;comment:请求ID"`
	UserID     uint      `gorm:"not null;default:0;index:idx_identity_auth_events_user_id;comment:用户ID"`
	EventType  string    `gorm:"size:64;not null;default:'';index:idx_identity_auth_events_event_type;comment:事件类型"`
	Result     string    `gorm:"size:32;not null;default:'';index:idx_identity_auth_events_result;comment:事件结果"`
	Reason     string    `gorm:"size:255;not null;default:'';comment:失败或阻断原因"`
	ClientIP   string    `gorm:"size:64;not null;default:'';comment:客户端IP"`
	UserAgent  string    `gorm:"size:512;not null;default:'';comment:用户代理"`
	DetailJSON string    `gorm:"type:text;not null;default:'';comment:附加详情JSON"`
	OccurredAt time.Time `gorm:"not null;index:idx_identity_auth_events_occurred_at;comment:发生时间"`
}

// TableName 指定表名。
func (UserAuthEvent) TableName() string {
	return "identity_auth_events"
}

// AuthIdentityProvider 存储企业登录身份源配置。
type AuthIdentityProvider struct {
	BaseModel
	PublicID              string `gorm:"size:32;not null;default:'';uniqueIndex:idx_identity_providers_public_id;comment:公开ID"`
	Type                  string `gorm:"size:16;not null;default:'';index:idx_identity_providers_type;comment:类型(oidc/oauth2)"`
	Name                  string `gorm:"size:80;not null;default:'';comment:显示名称"`
	Slug                  string `gorm:"size:64;not null;default:'';uniqueIndex:idx_identity_providers_slug;comment:登录入口标识"`
	LogoURL               string `gorm:"size:512;not null;default:'';comment:自定义 Logo 地址"`
	LoginEnabled          bool   `gorm:"not null;default:true;index:idx_identity_providers_login_enabled;comment:是否允许登录"`
	RegistrationEnabled   bool   `gorm:"not null;default:true;index:idx_identity_providers_registration_enabled;comment:是否允许注册"`
	ClientID              string `gorm:"size:255;not null;default:'';comment:Client ID"`
	ClientSecretEncrypted string `gorm:"type:text;not null;default:'';comment:Client Secret 密文"`
	IssuerURL             string `gorm:"size:512;not null;default:'';comment:OIDC Issuer URL"`
	DiscoveryURL          string `gorm:"size:512;not null;default:'';comment:OIDC Discovery URL"`
	AuthURL               string `gorm:"size:512;not null;default:'';comment:OAuth2 授权地址"`
	TokenURL              string `gorm:"size:512;not null;default:'';comment:OAuth2 Token 地址"`
	UserInfoURL           string `gorm:"size:512;not null;default:'';comment:OAuth2 UserInfo 地址"`
	JWKSURL               string `gorm:"size:512;not null;default:'';comment:JWKS 地址"`
	Scopes                string `gorm:"size:255;not null;default:'';comment:授权范围，空格分隔"`
	PKCEEnabled           bool   `gorm:"not null;default:true;comment:是否启用 PKCE"`
	DefaultRole           string `gorm:"size:32;not null;default:'user';comment:自动创建用户默认角色"`
	SubjectField          string `gorm:"size:64;not null;default:'sub';comment:用户唯一ID字段"`
	EmailField            string `gorm:"size:64;not null;default:'email';comment:邮箱字段"`
	EmailVerifiedField    string `gorm:"size:64;not null;default:'email_verified';comment:邮箱验证状态字段"`
	NameField             string `gorm:"size:64;not null;default:'name';comment:昵称字段"`
	AvatarField           string `gorm:"size:64;not null;default:'picture';comment:头像字段"`
	SortOrder             int    `gorm:"not null;default:100;index:idx_identity_providers_sort_order;comment:展示顺序"`
}

func (AuthIdentityProvider) TableName() string {
	return "identity_providers"
}

// UserIdentity 存储用户绑定的第三方身份。
type UserIdentity struct {
	BaseModel
	UserID              uint       `gorm:"not null;default:0;index:idx_identity_user_links_user_id;comment:用户ID"`
	ProviderID          uint       `gorm:"not null;default:0;uniqueIndex:uk_identity_user_links_provider_subject,priority:1;comment:身份源ID"`
	ProviderType        string     `gorm:"size:16;not null;default:'';comment:身份源类型"`
	ProviderSubject     string     `gorm:"size:255;not null;default:'';uniqueIndex:uk_identity_user_links_provider_subject,priority:2;comment:第三方用户唯一ID"`
	ProviderDisplayName string     `gorm:"size:128;not null;default:'';comment:第三方原始名称"`
	Email               string     `gorm:"size:128;not null;default:'';index:idx_identity_user_links_email;comment:第三方邮箱"`
	EmailVerified       bool       `gorm:"not null;default:false;comment:第三方邮箱是否已验证"`
	ProfileJSON         string     `gorm:"type:text;not null;default:'';comment:第三方用户资料JSON"`
	LinkedAt            time.Time  `gorm:"not null;comment:绑定时间"`
	LastLoginAt         *time.Time `gorm:"comment:最近使用此身份登录时间"`
}

func (UserIdentity) TableName() string {
	return "identity_user_links"
}

// UserTwoFactor 存储用户 2FA 配置。
type UserTwoFactor struct {
	BaseModel
	UserID                 uint       `gorm:"not null;default:0;uniqueIndex:idx_identity_mfa_settings_user_id;comment:用户ID"`
	TOTPEnabled            bool       `gorm:"not null;default:false;comment:是否启用 TOTP"`
	TOTPSecretEncrypted    string     `gorm:"type:text;not null;default:'';comment:TOTP Secret 密文"`
	TOTPSetupExpiresAt     *time.Time `gorm:"index:idx_identity_mfa_settings_setup_expires_at;comment:TOTP 设置过期时间"`
	RecoveryCodesHash      string     `gorm:"type:text;not null;default:'';comment:恢复码哈希JSON"`
	Enforced               bool       `gorm:"not null;default:false;comment:是否强制开启"`
	EnabledAt              *time.Time `gorm:"comment:启用时间"`
	LastVerifiedAt         *time.Time `gorm:"comment:最近验证时间"`
	TrustedDeviceExpiresAt *time.Time `gorm:"comment:可信设备默认过期时间"`
}

func (UserTwoFactor) TableName() string {
	return "identity_mfa_settings"
}

// TrustedDevice 存储 2FA 可信设备。
type TrustedDevice struct {
	BaseModel
	UserID    uint      `gorm:"not null;default:0;index:idx_identity_trusted_devices_user_id;comment:用户ID"`
	DeviceID  string    `gorm:"size:64;not null;default:'';uniqueIndex:idx_identity_trusted_devices_device_id;comment:设备ID"`
	TokenHash string    `gorm:"size:255;not null;default:'';comment:可信设备令牌哈希"`
	UserAgent string    `gorm:"size:512;not null;default:'';comment:用户代理"`
	ClientIP  string    `gorm:"size:64;not null;default:'';comment:客户端IP"`
	ExpiresAt time.Time `gorm:"not null;index:idx_identity_trusted_devices_expires_at;comment:过期时间"`
}

func (TrustedDevice) TableName() string {
	return "identity_trusted_devices"
}
