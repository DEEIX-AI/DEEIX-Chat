package auth

import (
	"time"

	appauth "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/auth"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/userview"
)

// LoginRequest 登录请求。
type LoginRequest struct {
	Username string `json:"username" binding:"required,min=3,max=128"`
	Password string `json:"password" binding:"required,min=6,max=128"`
}

type TwoFactorVerifyRequest struct {
	ChallengeToken     string `json:"challengeToken" binding:"required,min=20,max=4096"`
	VerificationMethod string `json:"verificationMethod,omitempty" binding:"omitempty,oneof=two_factor email"`
	Code               string `json:"code" binding:"required,min=6,max=32"`
}

type TwoFactorEmailStartRequest struct {
	ChallengeToken string `json:"challengeToken" binding:"required,min=20,max=4096"`
}

type TwoFactorCodeRequest struct {
	Code string `json:"code" binding:"required,min=6,max=32"`
}

type TwoFactorStatusResponse struct {
	Available     bool       `json:"available"`
	TOTPEnabled   bool       `json:"totpEnabled"`
	Required      bool       `json:"required"`
	RecoveryCount int        `json:"recoveryCount"`
	EnabledAt     *time.Time `json:"enabledAt" extensions:"x-nullable,!x-omitempty"`
}

type TwoFactorSetupStartResponse struct {
	Secret     string    `json:"secret"`
	OTPAuthURL string    `json:"otpauthURL"`
	ExpiresAt  time.Time `json:"expiresAt"`
}

type TwoFactorRecoveryCodesResponse struct {
	RecoveryCodes []string                `json:"recoveryCodes"`
	Status        TwoFactorStatusResponse `json:"status"`
}

type TwoFactorDisableResponse struct {
	Disabled bool `json:"disabled"`
}

type TwoFactorSetupCancelResponse struct {
	Canceled bool `json:"canceled"`
}

type EmailRegistrationStartRequest struct {
	Email          string `json:"email" binding:"required,max=128,email"`
	TurnstileToken string `json:"turnstileToken,omitempty" binding:"omitempty,max=2048"`
}

type EmailRegistrationStartResponse struct {
	Sent      bool      `json:"sent"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type PasswordChangeVerificationStartResponse struct {
	Sent               bool      `json:"sent"`
	ExpiresAt          time.Time `json:"expiresAt"`
	VerificationMethod string    `json:"verificationMethod"`
	AvailableMethods   []string  `json:"availableMethods"`
}

type EmailRegistrationCompleteRequest struct {
	Email          string `json:"email" binding:"required,max=128,email"`
	Password       string `json:"password" binding:"required,min=8,max=128"`
	Code           string `json:"code,omitempty" binding:"omitempty,len=6"`
	TurnstileToken string `json:"turnstileToken,omitempty" binding:"omitempty,max=2048"`
}

type PasswordResetStartRequest struct {
	Email string `json:"email" binding:"required,max=128,email"`
}

type PasswordResetCompleteRequest struct {
	Email       string `json:"email" binding:"required,max=128,email"`
	Code        string `json:"code" binding:"required,len=6"`
	NewPassword string `json:"newPassword" binding:"required,min=8,max=128"`
}

type PasswordResetStartResponse struct {
	Sent      bool      `json:"sent"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type PasswordResetCompleteResponse struct {
	Changed bool `json:"changed"`
}

type ChangePasswordRequest struct {
	CurrentPassword    string `json:"currentPassword,omitempty" binding:"omitempty,max=128"`
	NewPassword        string `json:"newPassword" binding:"required,min=8,max=128"`
	VerificationMethod string `json:"verificationMethod,omitempty" binding:"omitempty,oneof=none two_factor email"`
	Code               string `json:"code,omitempty" binding:"omitempty,min=6,max=32"`
}

type ChangePasswordResponse struct {
	Changed bool `json:"changed"`
}

type CompleteOnboardingRequest struct {
	NewPassword string `json:"newPassword,omitempty" binding:"omitempty,min=8,max=128"`
}

type EmailVerificationStartRequest struct {
	Email string `json:"email" binding:"required,max=128,email"`
}

type SecurityVerificationStartRequest struct {
	VerificationMethod string `json:"verificationMethod,omitempty" binding:"omitempty,oneof=none two_factor email"`
}

type DeleteAccountRequest struct {
	VerificationMethod string `json:"verificationMethod" binding:"required,oneof=two_factor email"`
	Code               string `json:"code" binding:"required,min=6,max=32"`
}

type EmailVerificationStartResponse struct {
	Sent               bool      `json:"sent"`
	ExpiresAt          time.Time `json:"expiresAt"`
	VerificationMethod string    `json:"verificationMethod"`
	AvailableMethods   []string  `json:"availableMethods"`
}

type EmailBootstrapCompleteRequest struct {
	Email string `json:"email" binding:"required,max=128,email"`
	Code  string `json:"code,omitempty" binding:"omitempty,len=6"`
}

type EmailVerificationCompleteRequest struct {
	Code string `json:"code" binding:"required,len=6"`
}

type EmailChangeCompleteRequest struct {
	Email                     string `json:"email" binding:"required,max=128,email"`
	CurrentVerificationMethod string `json:"currentVerificationMethod,omitempty" binding:"omitempty,oneof=none two_factor email"`
	CurrentCode               string `json:"currentCode,omitempty" binding:"omitempty,min=6,max=32"`
	NewCode                   string `json:"newCode,omitempty" binding:"omitempty,len=6"`
}

type IdentityProviderResponse struct {
	PublicID            string    `json:"publicID"`
	Type                string    `json:"type" enums:"oidc,oauth2"`
	Name                string    `json:"name"`
	Slug                string    `json:"slug"`
	LogoURL             string    `json:"logoURL"`
	LoginEnabled        bool      `json:"loginEnabled"`
	RegistrationEnabled bool      `json:"registrationEnabled"`
	ClientID            string    `json:"clientID,omitempty"`
	IssuerURL           string    `json:"issuerURL,omitempty"`
	DiscoveryURL        string    `json:"discoveryURL,omitempty"`
	AuthURL             string    `json:"authURL,omitempty"`
	TokenURL            string    `json:"tokenURL,omitempty"`
	UserInfoURL         string    `json:"userinfoURL,omitempty"`
	JWKSURL             string    `json:"jwksURL,omitempty"`
	Scopes              string    `json:"scopes"`
	DefaultRole         string    `json:"defaultRole" enums:"user,admin,superadmin"`
	SubjectField        string    `json:"subjectField"`
	EmailField          string    `json:"emailField"`
	EmailVerifiedField  string    `json:"emailVerifiedField"`
	NameField           string    `json:"nameField"`
	AvatarField         string    `json:"avatarField"`
	CreatedAt           time.Time `json:"createdAt"`
	UpdatedAt           time.Time `json:"updatedAt"`
}

type IdentityProviderListResponse struct {
	Results []IdentityProviderResponse `json:"results"`
	Total   int                        `json:"total"`
}

type IdentityProviderReorderResponse struct {
	Updated bool `json:"updated"`
}

type IdentityProviderDeleteResponse struct {
	Deleted bool `json:"deleted"`
}

type UserIdentityResponse struct {
	ID                  uint       `json:"id"`
	ProviderID          uint       `json:"providerID"`
	ProviderType        string     `json:"providerType"`
	ProviderName        string     `json:"providerName"`
	ProviderSlug        string     `json:"providerSlug"`
	ProviderLogoURL     string     `json:"providerLogoURL"`
	ProviderDisplayName string     `json:"providerDisplayName"`
	Email               string     `json:"email"`
	EmailVerified       bool       `json:"emailVerified"`
	LinkedAt            time.Time  `json:"linkedAt"`
	LastLoginAt         *time.Time `json:"lastLoginAt" extensions:"x-nullable,!x-omitempty"`
}

type UserIdentityListResponse struct {
	Results []UserIdentityResponse `json:"results"`
}

type DeleteUserIdentityResponse struct {
	Deleted bool `json:"deleted"`
}

type LoginOptionsResponse struct {
	UsernameEnabled              bool                       `json:"usernameEnabled"`
	EmailEnabled                 bool                       `json:"emailEnabled"`
	EmailRegistrationEnabled     bool                       `json:"emailRegistrationEnabled"`
	EmailVerificationEnabled     bool                       `json:"emailVerificationEnabled"`
	PasswordResetEnabled         bool                       `json:"passwordResetEnabled"`
	TurnstileRegistrationEnabled bool                       `json:"turnstileRegistrationEnabled"`
	TurnstileSiteKey             string                     `json:"turnstileSiteKey"`
	Providers                    []IdentityProviderResponse `json:"providers"`
}

type UpsertIdentityProviderRequest struct {
	Type                string `json:"type" binding:"required,oneof=oidc oauth2"`
	Name                string `json:"name" binding:"required,max=80"`
	Slug                string `json:"slug,omitempty" binding:"omitempty,max=64"`
	LogoURL             string `json:"logoURL,omitempty" binding:"omitempty,max=512"`
	LoginEnabled        *bool  `json:"loginEnabled,omitempty"`
	RegistrationEnabled *bool  `json:"registrationEnabled,omitempty"`
	ClientID            string `json:"clientID" binding:"required,max=255"`
	ClientSecret        string `json:"clientSecret,omitempty" binding:"omitempty,max=4096"`
	IssuerURL           string `json:"issuerURL,omitempty" binding:"omitempty,max=512"`
	DiscoveryURL        string `json:"discoveryURL,omitempty" binding:"omitempty,max=512"`
	AuthURL             string `json:"authURL,omitempty" binding:"omitempty,max=512"`
	TokenURL            string `json:"tokenURL,omitempty" binding:"omitempty,max=512"`
	UserInfoURL         string `json:"userinfoURL,omitempty" binding:"omitempty,max=512"`
	JWKSURL             string `json:"jwksURL,omitempty" binding:"omitempty,max=512"`
	Scopes              string `json:"scopes,omitempty" binding:"omitempty,max=255"`
	DefaultRole         string `json:"defaultRole,omitempty" binding:"omitempty,oneof=user admin superadmin"`
	SubjectField        string `json:"subjectField,omitempty" binding:"omitempty,max=64"`
	EmailField          string `json:"emailField,omitempty" binding:"omitempty,max=64"`
	EmailVerifiedField  string `json:"emailVerifiedField,omitempty" binding:"omitempty,max=64"`
	NameField           string `json:"nameField,omitempty" binding:"omitempty,max=64"`
	AvatarField         string `json:"avatarField,omitempty" binding:"omitempty,max=64"`
}

type ReorderIdentityProvidersRequest struct {
	ProviderIDs []string `json:"providerIDs" binding:"required,dive,required,max=64"`
}

type CompleteProviderLoginRequest struct {
	Code         string `json:"code" binding:"required"`
	State        string `json:"state" binding:"required,max=4096"`
	RedirectURI  string `json:"redirectURI" binding:"required,max=2048"`
	CodeVerifier string `json:"codeVerifier" binding:"required,min=43,max=128"`
	Intent       string `json:"intent,omitempty" binding:"omitempty,oneof=login register bind"`
}

type CompleteProviderBindRequest struct {
	Code         string `json:"code" binding:"required"`
	State        string `json:"state" binding:"required,max=4096"`
	RedirectURI  string `json:"redirectURI" binding:"required,max=2048"`
	CodeVerifier string `json:"codeVerifier" binding:"required,min=43,max=128"`
}

type UserIdentityResponseData struct {
	Identity UserIdentityResponse `json:"identity"`
}

// PatchMeRequest 更新当前用户资料请求。
type PatchMeRequest struct {
	AvatarURL             *string `json:"avatarURL,omitempty" binding:"omitempty,max=2048"`
	DisplayName           *string `json:"displayName,omitempty" binding:"omitempty,min=3,max=16"`
	Timezone              *string `json:"timezone,omitempty" binding:"omitempty,max=64"`
	Locale                *string `json:"locale,omitempty" binding:"omitempty,max=16"`
	ProfilePreferences    *string `json:"profilePreferences,omitempty" binding:"omitempty,max=1024"`
	AppearancePreferences *string `json:"appearancePreferences,omitempty" binding:"omitempty,max=2048"`
}

type PatchUsernameRequest struct {
	Username string `json:"username" binding:"required,min=3,max=16"`
}

// UpdateCurrentSessionLocationRequest 更新当前会话的精确位置请求。
type UpdateCurrentSessionLocationRequest struct {
	Latitude       float64  `json:"latitude" binding:"required"`
	Longitude      float64  `json:"longitude" binding:"required"`
	AccuracyMeters *float64 `json:"accuracyMeters,omitempty" binding:"omitempty,min=0,max=1000000"`
	Timezone       string   `json:"timezone,omitempty" binding:"omitempty,max=64"`
}

// UserResponse 面向前端的用户视图响应。
type UserResponse struct {
	ID                      uint                                  `json:"id"`
	PublicID                string                                `json:"publicID"`
	Username                string                                `json:"username"`
	DisplayName             string                                `json:"displayName"`
	AvatarURL               string                                `json:"avatarURL"`
	Email                   string                                `json:"email"`
	Phone                   string                                `json:"phone"`
	Role                    string                                `json:"role"`
	Status                  string                                `json:"status"`
	Timezone                string                                `json:"timezone"`
	Locale                  string                                `json:"locale"`
	ProfilePreferences      string                                `json:"profilePreferences"`
	AppearancePreferences   string                                `json:"appearancePreferences"`
	OnboardingCompletedAt   *time.Time                            `json:"onboardingCompletedAt" extensions:"x-nullable,!x-omitempty"`
	EmailVerifiedAt         *time.Time                            `json:"emailVerifiedAt" extensions:"x-nullable,!x-omitempty"`
	EmailSource             string                                `json:"emailSource"`
	EmailBootstrapUsedAt    *time.Time                            `json:"emailBootstrapUsedAt" extensions:"x-nullable,!x-omitempty"`
	PhoneVerifiedAt         *time.Time                            `json:"phoneVerifiedAt" extensions:"x-nullable,!x-omitempty"`
	UsernameChangedAt       *time.Time                            `json:"usernameChangedAt" extensions:"x-nullable,!x-omitempty"`
	PasswordEnabled         bool                                  `json:"passwordEnabled"`
	PasswordSetAt           *time.Time                            `json:"passwordSetAt" extensions:"x-nullable,!x-omitempty"`
	PasswordOrigin          string                                `json:"passwordOrigin"`
	MustResetPassword       bool                                  `json:"mustResetPassword"`
	InitialUsernameRequired bool                                  `json:"initialUsernameRequired"`
	InitialSecurityRequired bool                                  `json:"initialSecurityRequired"`
	TwoFactorAvailable      bool                                  `json:"twoFactorAvailable"`
	TwoFactorEnabled        bool                                  `json:"twoFactorEnabled"`
	TwoFactorRequired       bool                                  `json:"twoFactorRequired"`
	TwoFactorRecoveryCount  int                                   `json:"twoFactorRecoveryCount"`
	LastLoginAt             *time.Time                            `json:"lastLoginAt" extensions:"x-nullable,!x-omitempty"`
	LastActiveAt            *time.Time                            `json:"lastActiveAt" extensions:"x-nullable,!x-omitempty"`
	CreatedAt               time.Time                             `json:"createdAt"`
	UpdatedAt               time.Time                             `json:"updatedAt"`
	SubscriptionTier        string                                `json:"subscriptionTier"`
	SubscriptionPlanID      *uint                                 `json:"subscriptionPlanID" extensions:"x-nullable,!x-omitempty"`
	SubscriptionPlanName    string                                `json:"subscriptionPlanName"`
	SubscriptionStatus      string                                `json:"subscriptionStatus"`
	SubscriptionExpiresAt   *time.Time                            `json:"subscriptionExpiresAt" extensions:"x-nullable,!x-omitempty"`
	IdentityProviders       []UserIdentityProviderSummaryResponse `json:"identityProviders"`
}

// UserIdentityProviderSummaryResponse 用户绑定身份源摘要。
type UserIdentityProviderSummaryResponse struct {
	ID      uint   `json:"id"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Slug    string `json:"slug"`
	LogoURL string `json:"logoURL"`
}

// LoginResponse 登录响应。
type LoginResponse struct {
	AccessToken             string       `json:"accessToken"`
	SessionID               string       `json:"sessionID"`
	ExpiresAt               time.Time    `json:"expiresAt"`
	RefreshExpiresAt        time.Time    `json:"refreshExpiresAt"`
	User                    UserResponse `json:"user"`
	TwoFactorRequired       bool         `json:"twoFactorRequired"`
	TwoFactorChallengeToken string       `json:"twoFactorChallengeToken,omitempty"`
	VerificationMethods     []string     `json:"verificationMethods,omitempty"`
}

// MeResponse 当前用户信息响应。
type MeResponse struct {
	User UserResponse `json:"user"`
}

// LogoutResponse 登出响应。
type LogoutResponse struct {
	Revoked bool `json:"revoked"`
}

// DeleteAccountResponse 删除账户响应。
type DeleteAccountResponse struct {
	Deleted bool `json:"deleted"`
}

// ActiveSessionResponse 活跃会话响应。
type ActiveSessionResponse struct {
	SessionID        string     `json:"sessionID"`
	Current          bool       `json:"current"`
	DeviceLabel      string     `json:"deviceLabel"`
	DeviceName       string     `json:"deviceName"`
	BrowserName      string     `json:"browserName"`
	OSName           string     `json:"osName"`
	DeviceType       string     `json:"deviceType"`
	ClientIP         string     `json:"clientIP"`
	LocationLabel    string     `json:"locationLabel"`
	GeoSource        string     `json:"geoSource"`
	GeoAccuracy      string     `json:"geoAccuracy"`
	CountryCode      string     `json:"countryCode"`
	RegionName       string     `json:"regionName"`
	CityName         string     `json:"cityName"`
	TimezoneName     string     `json:"timezoneName"`
	IPLatitude       *float64   `json:"ipLatitude" extensions:"x-nullable,!x-omitempty"`
	IPLongitude      *float64   `json:"ipLongitude" extensions:"x-nullable,!x-omitempty"`
	PreciseLatitude  *float64   `json:"preciseLatitude" extensions:"x-nullable,!x-omitempty"`
	PreciseLongitude *float64   `json:"preciseLongitude" extensions:"x-nullable,!x-omitempty"`
	PreciseAccuracyM *float64   `json:"preciseAccuracyMeters" extensions:"x-nullable,!x-omitempty"`
	PreciseLocatedAt *time.Time `json:"preciseLocatedAt" extensions:"x-nullable,!x-omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
	LastSeenAt       *time.Time `json:"lastSeenAt" extensions:"x-nullable,!x-omitempty"`
	ExpiresAt        time.Time  `json:"expiresAt"`
}

// ActiveSessionListResponse 活跃会话列表响应。
type ActiveSessionListResponse struct {
	Total   int64                   `json:"total"`
	Results []ActiveSessionResponse `json:"results"`
}

// Swagger 文档类型

// LoginResponseDoc 登录响应（Swagger 用）。
type LoginResponseDoc struct {
	ErrorMsg string        `json:"errorMsg"`
	Data     LoginResponse `json:"data"`
}

// LoginOptionsResponseDoc 登录入口配置响应（Swagger 用）。
type LoginOptionsResponseDoc struct {
	ErrorMsg string               `json:"errorMsg"`
	Data     LoginOptionsResponse `json:"data"`
}

// IdentityProviderListResponseDoc 管理员身份源列表响应（Swagger 用）。
type IdentityProviderListResponseDoc struct {
	ErrorMsg string                       `json:"errorMsg"`
	Data     IdentityProviderListResponse `json:"data"`
}

// IdentityProviderResponseDoc 管理员身份源详情响应（Swagger 用）。
type IdentityProviderResponseDoc struct {
	ErrorMsg string                   `json:"errorMsg"`
	Data     IdentityProviderResponse `json:"data"`
}

// IdentityProviderReorderResponseDoc 管理员身份源排序响应（Swagger 用）。
type IdentityProviderReorderResponseDoc struct {
	ErrorMsg string                          `json:"errorMsg"`
	Data     IdentityProviderReorderResponse `json:"data"`
}

// IdentityProviderDeleteResponseDoc 管理员身份源删除响应（Swagger 用）。
type IdentityProviderDeleteResponseDoc struct {
	ErrorMsg string                         `json:"errorMsg"`
	Data     IdentityProviderDeleteResponse `json:"data"`
}

// EmailRegistrationStartResponseDoc 邮箱注册验证码发送响应（Swagger 用）。
type EmailRegistrationStartResponseDoc struct {
	ErrorMsg string                         `json:"errorMsg"`
	Data     EmailRegistrationStartResponse `json:"data"`
}

type PasswordResetStartResponseDoc struct {
	ErrorMsg string                     `json:"errorMsg"`
	Data     PasswordResetStartResponse `json:"data"`
}

type PasswordResetCompleteResponseDoc struct {
	ErrorMsg string                        `json:"errorMsg"`
	Data     PasswordResetCompleteResponse `json:"data"`
}

// MeResponseDoc 当前用户信息响应（Swagger 用）。
type MeResponseDoc struct {
	ErrorMsg string     `json:"errorMsg"`
	Data     MeResponse `json:"data"`
}

// PatchMeResponseDoc 更新当前用户资料响应（Swagger 用）。
type PatchMeResponseDoc struct {
	ErrorMsg string     `json:"errorMsg"`
	Data     MeResponse `json:"data"`
}

type PasswordChangeVerificationStartResponseDoc struct {
	ErrorMsg string                                  `json:"errorMsg"`
	Data     PasswordChangeVerificationStartResponse `json:"data"`
}

type ChangePasswordResponseDoc struct {
	ErrorMsg string                 `json:"errorMsg"`
	Data     ChangePasswordResponse `json:"data"`
}

type EmailVerificationStartResponseDoc struct {
	ErrorMsg string                         `json:"errorMsg"`
	Data     EmailVerificationStartResponse `json:"data"`
}

// RefreshTokenResponseDoc 刷新令牌响应（Swagger 用）。
type RefreshTokenResponseDoc struct {
	ErrorMsg string        `json:"errorMsg"`
	Data     LoginResponse `json:"data"`
}

// LogoutResponseDoc 登出响应（Swagger 用）。
type LogoutResponseDoc struct {
	ErrorMsg string         `json:"errorMsg"`
	Data     LogoutResponse `json:"data"`
}

// DeleteAccountResponseDoc 删除账户响应（Swagger 用）。
type DeleteAccountResponseDoc struct {
	ErrorMsg string                `json:"errorMsg"`
	Data     DeleteAccountResponse `json:"data"`
}

// ActiveSessionListResponseDoc 活跃会话列表响应（Swagger 用）。
type ActiveSessionListResponseDoc struct {
	ErrorMsg string                    `json:"errorMsg"`
	Data     ActiveSessionListResponse `json:"data"`
}

// UpdateCurrentSessionLocationResponseDoc 更新精确位置响应（Swagger 用）。
type UpdateCurrentSessionLocationResponseDoc struct {
	ErrorMsg string                `json:"errorMsg"`
	Data     ActiveSessionResponse `json:"data"`
}

// ErrorDoc 错误响应（Swagger 用）。
type ErrorDoc struct {
	ErrorMsg  string      `json:"errorMsg"`
	ErrorCode string      `json:"errorCode,omitempty"`
	Details   interface{} `json:"details,omitempty"`
	RequestID string      `json:"requestId,omitempty"`
	Data      interface{} `json:"data"`
}

// toUserResponse 将 userview.UserView 映射为响应 DTO。
func toUserResponse(v userview.UserView) UserResponse {
	return UserResponse{
		ID:                      v.ID,
		PublicID:                v.PublicID,
		Username:                v.Username,
		DisplayName:             v.DisplayName,
		AvatarURL:               v.AvatarURL,
		Email:                   v.Email,
		Phone:                   v.Phone,
		Role:                    v.Role,
		Status:                  v.Status,
		Timezone:                v.Timezone,
		Locale:                  v.Locale,
		ProfilePreferences:      v.ProfilePreferences,
		AppearancePreferences:   v.AppearancePreferences,
		OnboardingCompletedAt:   v.OnboardingCompletedAt,
		EmailVerifiedAt:         v.EmailVerifiedAt,
		EmailSource:             v.EmailSource,
		EmailBootstrapUsedAt:    v.EmailBootstrapUsedAt,
		PhoneVerifiedAt:         v.PhoneVerifiedAt,
		UsernameChangedAt:       v.UsernameChangedAt,
		PasswordEnabled:         v.PasswordEnabled,
		PasswordSetAt:           v.PasswordSetAt,
		PasswordOrigin:          v.PasswordOrigin,
		MustResetPassword:       v.MustResetPassword,
		InitialUsernameRequired: v.InitialUsernameRequired,
		InitialSecurityRequired: v.InitialSecurityRequired,
		TwoFactorAvailable:      v.TwoFactorAvailable,
		TwoFactorEnabled:        v.TwoFactorEnabled,
		TwoFactorRequired:       v.TwoFactorRequired,
		TwoFactorRecoveryCount:  v.TwoFactorRecoveryCount,
		LastLoginAt:             v.LastLoginAt,
		LastActiveAt:            v.LastActiveAt,
		CreatedAt:               v.CreatedAt,
		UpdatedAt:               v.UpdatedAt,
		SubscriptionTier:        v.SubscriptionTier,
		SubscriptionPlanID:      v.SubscriptionPlanID,
		SubscriptionPlanName:    v.SubscriptionPlanName,
		SubscriptionStatus:      v.SubscriptionStatus,
		SubscriptionExpiresAt:   v.SubscriptionExpiresAt,
		IdentityProviders:       toUserIdentityProviderSummaryResponses(v.IdentityProviders),
	}
}

func toUserIdentityProviderSummaryResponses(items []userview.IdentityProviderSummary) []UserIdentityProviderSummaryResponse {
	results := make([]UserIdentityProviderSummaryResponse, 0, len(items))
	for _, item := range items {
		results = append(results, UserIdentityProviderSummaryResponse{
			ID:      item.ID,
			Type:    item.Type,
			Name:    item.Name,
			Slug:    item.Slug,
			LogoURL: item.LogoURL,
		})
	}
	return results
}

// toLoginResponse 将 LoginResult 映射为响应 DTO。
func toLoginResponse(d *appauth.LoginResult) LoginResponse {
	return LoginResponse{
		AccessToken:             d.AccessToken,
		SessionID:               d.SessionID,
		ExpiresAt:               d.ExpiresAt,
		RefreshExpiresAt:        d.RefreshExpiresAt,
		User:                    toUserResponse(d.User),
		TwoFactorRequired:       d.TwoFactorRequired,
		TwoFactorChallengeToken: d.TwoFactorChallengeToken,
		VerificationMethods:     toSecurityVerificationMethods(d.VerificationMethods),
	}
}

func toTwoFactorStatusResponse(d *appauth.TwoFactorStatusResult) TwoFactorStatusResponse {
	return TwoFactorStatusResponse{
		Available:     d.Available,
		TOTPEnabled:   d.TOTPEnabled,
		Required:      d.Required,
		RecoveryCount: d.RecoveryCount,
		EnabledAt:     d.EnabledAt,
	}
}

func toEmailRegistrationStartResponse(d *appauth.EmailRegistrationStartResult) EmailRegistrationStartResponse {
	return EmailRegistrationStartResponse{
		Sent:      d.Sent,
		ExpiresAt: d.ExpiresAt,
	}
}

func toPasswordResetStartResponse(d *appauth.PasswordResetStartResult) PasswordResetStartResponse {
	return PasswordResetStartResponse{
		Sent:      d.Sent,
		ExpiresAt: d.ExpiresAt,
	}
}

func toSecurityVerificationMethods(methods []appauth.SecurityVerificationMethod) []string {
	result := make([]string, 0, len(methods))
	for _, method := range methods {
		result = append(result, string(method))
	}
	return result
}

func toPasswordChangeVerificationStartResponse(d *appauth.PasswordChangeVerificationStartResult) PasswordChangeVerificationStartResponse {
	return PasswordChangeVerificationStartResponse{
		Sent:               d.Sent,
		ExpiresAt:          d.ExpiresAt,
		VerificationMethod: string(d.Method),
		AvailableMethods:   toSecurityVerificationMethods(d.AvailableMethods),
	}
}

func toEmailVerificationStartResponse(d *appauth.EmailChangeVerificationStartResult) EmailVerificationStartResponse {
	return EmailVerificationStartResponse{
		Sent:               d.Sent,
		ExpiresAt:          d.ExpiresAt,
		VerificationMethod: string(d.Method),
		AvailableMethods:   toSecurityVerificationMethods(d.AvailableMethods),
	}
}

func toLoginOptionsResponse(d *appauth.LoginOptions) LoginOptionsResponse {
	return LoginOptionsResponse{
		UsernameEnabled:              d.UsernameEnabled,
		EmailEnabled:                 d.EmailEnabled,
		EmailRegistrationEnabled:     d.EmailRegistrationEnabled,
		EmailVerificationEnabled:     d.EmailVerificationEnabled,
		PasswordResetEnabled:         d.PasswordResetEnabled,
		TurnstileRegistrationEnabled: d.TurnstileRegistrationEnabled,
		TurnstileSiteKey:             d.TurnstileSiteKey,
		Providers:                    toIdentityProviderResponses(d.Providers),
	}
}

func toIdentityProviderResponses(items []appauth.IdentityProviderView) []IdentityProviderResponse {
	results := make([]IdentityProviderResponse, 0, len(items))
	for _, item := range items {
		results = append(results, toIdentityProviderResponse(item))
	}
	return results
}

func toUserIdentityResponses(items []appauth.UserIdentityView) []UserIdentityResponse {
	results := make([]UserIdentityResponse, 0, len(items))
	for _, item := range items {
		results = append(results, toUserIdentityResponse(item))
	}
	return results
}

func toUserIdentityResponse(item appauth.UserIdentityView) UserIdentityResponse {
	return UserIdentityResponse{
		ID:                  item.ID,
		ProviderID:          item.ProviderID,
		ProviderType:        item.ProviderType,
		ProviderName:        item.ProviderName,
		ProviderSlug:        item.ProviderSlug,
		ProviderLogoURL:     item.ProviderLogoURL,
		ProviderDisplayName: item.ProviderDisplayName,
		Email:               item.Email,
		EmailVerified:       item.EmailVerified,
		LinkedAt:            item.LinkedAt,
		LastLoginAt:         item.LastLoginAt,
	}
}

func toIdentityProviderResponse(item appauth.IdentityProviderView) IdentityProviderResponse {
	return IdentityProviderResponse{
		PublicID:            item.PublicID,
		Type:                item.Type,
		Name:                item.Name,
		Slug:                item.Slug,
		LogoURL:             item.LogoURL,
		LoginEnabled:        item.LoginEnabled,
		RegistrationEnabled: item.RegistrationEnabled,
		ClientID:            item.ClientID,
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

func toUpsertIdentityProviderInput(req UpsertIdentityProviderRequest, actorRole string) appauth.UpsertIdentityProviderInput {
	return appauth.UpsertIdentityProviderInput{
		ActorRole:           actorRole,
		Type:                req.Type,
		Name:                req.Name,
		Slug:                req.Slug,
		LogoURL:             req.LogoURL,
		LoginEnabled:        req.LoginEnabled,
		RegistrationEnabled: req.RegistrationEnabled,
		ClientID:            req.ClientID,
		ClientSecret:        req.ClientSecret,
		IssuerURL:           req.IssuerURL,
		DiscoveryURL:        req.DiscoveryURL,
		AuthURL:             req.AuthURL,
		TokenURL:            req.TokenURL,
		UserInfoURL:         req.UserInfoURL,
		JWKSURL:             req.JWKSURL,
		Scopes:              req.Scopes,
		DefaultRole:         req.DefaultRole,
		SubjectField:        req.SubjectField,
		EmailField:          req.EmailField,
		EmailVerifiedField:  req.EmailVerifiedField,
		NameField:           req.NameField,
		AvatarField:         req.AvatarField,
	}
}

// toMeResponse 将 MeResult 映射为响应 DTO。
func toMeResponse(d *appauth.MeResult) MeResponse {
	return MeResponse{User: toUserResponse(d.User)}
}

// toActiveSessionResponse 将 ActiveSessionResult 映射为响应 DTO。
func toActiveSessionResponse(d appauth.ActiveSessionResult) ActiveSessionResponse {
	return ActiveSessionResponse{
		SessionID:        d.SessionID,
		Current:          d.Current,
		DeviceLabel:      d.DeviceLabel,
		DeviceName:       d.DeviceName,
		BrowserName:      d.BrowserName,
		OSName:           d.OSName,
		DeviceType:       d.DeviceType,
		ClientIP:         d.ClientIP,
		LocationLabel:    d.LocationLabel,
		GeoSource:        d.GeoSource,
		GeoAccuracy:      d.GeoAccuracy,
		CountryCode:      d.CountryCode,
		RegionName:       d.RegionName,
		CityName:         d.CityName,
		TimezoneName:     d.TimezoneName,
		IPLatitude:       d.IPLatitude,
		IPLongitude:      d.IPLongitude,
		PreciseLatitude:  d.PreciseLatitude,
		PreciseLongitude: d.PreciseLongitude,
		PreciseAccuracyM: d.PreciseAccuracyM,
		PreciseLocatedAt: d.PreciseLocatedAt,
		CreatedAt:        d.CreatedAt,
		UpdatedAt:        d.UpdatedAt,
		LastSeenAt:       d.LastSeenAt,
		ExpiresAt:        d.ExpiresAt,
	}
}

// toActiveSessionListResponse 将 ActiveSessionListResult 映射为响应 DTO。
func toActiveSessionListResponse(d *appauth.ActiveSessionListResult) ActiveSessionListResponse {
	items := make([]ActiveSessionResponse, 0, len(d.Results))
	for _, s := range d.Results {
		items = append(items, toActiveSessionResponse(s))
	}
	return ActiveSessionListResponse{
		Total:   d.Total,
		Results: items,
	}
}

// toUpdateProfileInput 将更新资料 HTTP 请求映射为应用层输入。
func toUpdateProfileInput(req PatchMeRequest) appauth.UpdateProfileInput {
	return appauth.UpdateProfileInput{
		AvatarURL:             req.AvatarURL,
		DisplayName:           req.DisplayName,
		Timezone:              req.Timezone,
		Locale:                req.Locale,
		ProfilePreferences:    req.ProfilePreferences,
		AppearancePreferences: req.AppearancePreferences,
	}
}

func toUpdateUsernameInput(req PatchUsernameRequest) appauth.UpdateUsernameInput {
	return appauth.UpdateUsernameInput{Username: req.Username}
}

// toUpdateCurrentSessionLocationInput 将更新会话位置 HTTP 请求映射为应用层输入。
func toUpdateCurrentSessionLocationInput(req UpdateCurrentSessionLocationRequest) appauth.UpdateCurrentSessionLocationInput {
	return appauth.UpdateCurrentSessionLocationInput{
		Latitude:       req.Latitude,
		Longitude:      req.Longitude,
		AccuracyMeters: req.AccuracyMeters,
		Timezone:       req.Timezone,
	}
}
