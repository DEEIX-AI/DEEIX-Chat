package userview

import (
	"strings"
	"time"

	domainuser "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/user"
)

// SubscriptionState 描述用户当前订阅的派生状态。
type SubscriptionState struct {
	PlanID    *uint
	PlanName  string
	Tier      string
	Status    string
	ExpiresAt *time.Time
}

// BillingAccountState 描述用户按量余额的派生状态。
type BillingAccountState struct {
	Currency       string
	BalanceNanousd int64
	Status         string
}

// UserView 面向应用层传递的用户视图
// 序列化由 transport 层的响应 DTO 负责。
type UserView struct {
	ID                      uint
	PublicID                string
	Username                string
	DisplayName             string
	AvatarURL               string
	Email                   string
	Phone                   string
	Role                    string
	Status                  string
	Timezone                string
	Locale                  string
	ProfilePreferences      string
	AppearancePreferences   string
	OnboardingCompletedAt   *time.Time
	EmailVerifiedAt         *time.Time
	EmailSource             string
	EmailBootstrapUsedAt    *time.Time
	PhoneVerifiedAt         *time.Time
	UsernameChangedAt       *time.Time
	PasswordEnabled         bool
	PasswordSetAt           *time.Time
	PasswordOrigin          string
	MustResetPassword       bool
	InitialUsernameRequired bool
	InitialSecurityRequired bool
	TwoFactorAvailable      bool
	TwoFactorEnabled        bool
	TwoFactorRequired       bool
	TwoFactorRecoveryCount  int
	LastLoginAt             *time.Time
	CreatedAt               time.Time
	UpdatedAt               time.Time
	SubscriptionTier        string
	SubscriptionPlanID      *uint
	SubscriptionPlanName    string
	SubscriptionStatus      string
	SubscriptionExpiresAt   *time.Time
	BillingAccountCurrency  string
	BillingBalanceNanousd   int64
	BillingAccountStatus    string
}

// FromUser 将用户领域模型转换为前端可用的用户视图。
func FromUser(item domainuser.User, subscription *SubscriptionState) UserView {
	view := UserView{
		ID:                     item.ID,
		PublicID:               item.PublicID,
		Username:               item.Username,
		DisplayName:            item.DisplayName,
		AvatarURL:              item.AvatarURL,
		Email:                  item.Email,
		Phone:                  item.Phone,
		Role:                   item.Role,
		Status:                 item.Status,
		Timezone:               item.Timezone,
		Locale:                 item.Locale,
		ProfilePreferences:     item.ProfilePreferences,
		AppearancePreferences:  item.AppearancePreferences,
		OnboardingCompletedAt:  item.OnboardingCompletedAt,
		EmailVerifiedAt:        item.EmailVerifiedAt,
		EmailSource:            item.EmailSource,
		EmailBootstrapUsedAt:   item.EmailBootstrapUsedAt,
		PhoneVerifiedAt:        item.PhoneVerifiedAt,
		UsernameChangedAt:      item.UsernameChangedAt,
		LastLoginAt:            item.LastLoginAt,
		CreatedAt:              item.CreatedAt,
		UpdatedAt:              item.UpdatedAt,
		SubscriptionTier:       "free",
		SubscriptionPlanID:     nil,
		SubscriptionPlanName:   "free",
		SubscriptionStatus:     "free",
		SubscriptionExpiresAt:  nil,
		BillingAccountCurrency: "USD",
		BillingBalanceNanousd:  0,
		BillingAccountStatus:   "active",
	}

	if subscription == nil {
		return view
	}

	if normalizedTier := strings.TrimSpace(subscription.Tier); normalizedTier != "" {
		view.SubscriptionTier = normalizedTier
	}
	if normalizedPlanName := strings.TrimSpace(subscription.PlanName); normalizedPlanName != "" {
		view.SubscriptionPlanName = normalizedPlanName
	}
	if normalizedStatus := strings.TrimSpace(subscription.Status); normalizedStatus != "" {
		view.SubscriptionStatus = normalizedStatus
	}
	view.SubscriptionPlanID = subscription.PlanID
	view.SubscriptionExpiresAt = subscription.ExpiresAt

	return view
}

// WithBillingAccount 设置用户视图中的按量余额信息。
func WithBillingAccount(view UserView, account *BillingAccountState) UserView {
	if account == nil {
		return view
	}
	if normalizedCurrency := strings.TrimSpace(account.Currency); normalizedCurrency != "" {
		view.BillingAccountCurrency = normalizedCurrency
	}
	view.BillingBalanceNanousd = account.BalanceNanousd
	if normalizedStatus := strings.TrimSpace(account.Status); normalizedStatus != "" {
		view.BillingAccountStatus = normalizedStatus
	}
	return view
}
