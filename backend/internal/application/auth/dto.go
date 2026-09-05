package auth

import (
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/userview"
)

// LoginResult 登录成功后的内部传输结构，不携带序列化标记。
type LoginResult struct {
	AccessToken             string
	RefreshToken            string
	SessionID               string
	ExpiresAt               time.Time
	RefreshExpiresAt        time.Time
	User                    userview.UserView
	TwoFactorRequired       bool
	TwoFactorChallengeToken string
	VerificationMethods     []SecurityVerificationMethod
}

// MeResult 当前用户信息内部传输结构，不携带序列化标记。
type MeResult struct {
	User userview.UserView
}

// LogoutResult 登出结果内部传输结构，不携带序列化标记。
type LogoutResult struct {
	Revoked bool
}

// DeleteAccountResult 删除账户结果内部传输结构，不携带序列化标记。
type DeleteAccountResult struct {
	Deleted bool
}

// ActiveSessionResult 活跃会话内部传输结构，不携带序列化标记。
type ActiveSessionResult struct {
	SessionID        string
	Current          bool
	DeviceLabel      string
	DeviceName       string
	BrowserName      string
	OSName           string
	DeviceType       string
	ClientIP         string
	LocationLabel    string
	GeoSource        string
	GeoAccuracy      string
	CountryCode      string
	RegionName       string
	CityName         string
	TimezoneName     string
	IPLatitude       *float64
	IPLongitude      *float64
	PreciseLatitude  *float64
	PreciseLongitude *float64
	PreciseAccuracyM *float64
	PreciseLocatedAt *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
	LastSeenAt       *time.Time
	ExpiresAt        time.Time
}

// ActiveSessionListResult 活跃会话列表内部传输结构，不携带序列化标记。
type ActiveSessionListResult struct {
	Total   int64
	Results []ActiveSessionResult
}

// TwoFactorStatusResult describes the current two-factor authentication state.
type TwoFactorStatusResult struct {
	Available     bool
	TOTPEnabled   bool
	Required      bool
	RecoveryCount int
	EnabledAt     *time.Time
}

// TwoFactorSetupStartResult contains the temporary setup secret and expiry.
type TwoFactorSetupStartResult struct {
	Secret     string
	OTPAuthURL string
	ExpiresAt  time.Time
}

// TwoFactorSetupConfirmResult contains recovery codes issued after setup.
type TwoFactorSetupConfirmResult struct {
	RecoveryCodes []string
	Status        TwoFactorStatusResult
}
