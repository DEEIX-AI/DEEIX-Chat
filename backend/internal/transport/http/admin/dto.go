package admin

import (
	"time"

	appadmin "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/admin"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/userview"
	domainaudit "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/audit"
	domainbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/billing"
	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	domainsystemevent "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/systemevent"
	domainuser "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/user"
)

// ── 请求 DTO ────────────────────────────────────────────────────────────────

// CreateUserRequest 管理员创建用户请求。
type CreateUserRequest struct {
	Username              string     `json:"username" binding:"required,min=3,max=16"`
	Password              string     `json:"password" binding:"required,min=8,max=128"`
	AvatarURL             string     `json:"avatarURL" binding:"max=2048"`
	DisplayName           string     `json:"displayName" binding:"omitempty,min=3,max=16"`
	Email                 string     `json:"email" binding:"omitempty,max=128,email"`
	Phone                 string     `json:"phone" binding:"max=32"`
	Timezone              string     `json:"timezone" binding:"max=64"`
	Locale                string     `json:"locale" binding:"max=16"`
	SubscriptionTier      string     `json:"subscriptionTier" binding:"max=32"`
	SubscriptionExpiresAt *time.Time `json:"subscriptionExpiresAt"`
}

// UpdateUserStatusRequest 管理员更新用户状态请求。
type UpdateUserStatusRequest struct {
	Status string `json:"status" binding:"required,max=32"`
	Reason string `json:"reason" binding:"max=255"`
}

// PatchUserRequest 管理员局部更新用户请求。
type PatchUserRequest struct {
	AvatarURL             *string    `json:"avatarURL" binding:"omitempty,max=2048"`
	DisplayName           *string    `json:"displayName" binding:"omitempty,min=3,max=16"`
	Email                 *string    `json:"email" binding:"omitempty,max=128"`
	Phone                 *string    `json:"phone" binding:"omitempty,max=32"`
	Role                  *string    `json:"role" binding:"omitempty,max=32"`
	Status                *string    `json:"status" binding:"omitempty,max=32"`
	Timezone              *string    `json:"timezone" binding:"omitempty,max=64"`
	Locale                *string    `json:"locale" binding:"omitempty,max=16"`
	ProfilePreferences    *string    `json:"profilePreferences" binding:"omitempty,max=1024"`
	SubscriptionTier      *string    `json:"subscriptionTier" binding:"omitempty,max=32"`
	SubscriptionExpiresAt *time.Time `json:"subscriptionExpiresAt"`
	Reason                string     `json:"reason" binding:"max=255"`
}

// ResetUserPasswordRequest 管理员重置用户密码请求。
type ResetUserPasswordRequest struct {
	NewPassword       string `json:"newPassword" binding:"required,min=8,max=128"`
	MustResetPassword *bool  `json:"mustResetPassword"`
}

// ── 响应 DTO ────────────────────────────────────────────────────────────────

// UserResponse 面向前端的用户视图响应。
type UserResponse struct {
	ID                     uint       `json:"id"`
	PublicID               string     `json:"publicID"`
	Username               string     `json:"username"`
	DisplayName            string     `json:"displayName"`
	AvatarURL              string     `json:"avatarURL"`
	Email                  string     `json:"email"`
	Phone                  string     `json:"phone"`
	Role                   string     `json:"role"`
	Status                 string     `json:"status"`
	Timezone               string     `json:"timezone"`
	Locale                 string     `json:"locale"`
	ProfilePreferences     string     `json:"profilePreferences"`
	AppearancePreferences  string     `json:"appearancePreferences"`
	EmailVerifiedAt        *time.Time `json:"emailVerifiedAt"`
	PhoneVerifiedAt        *time.Time `json:"phoneVerifiedAt"`
	TwoFactorAvailable     bool       `json:"twoFactorAvailable"`
	TwoFactorEnabled       bool       `json:"twoFactorEnabled"`
	TwoFactorRequired      bool       `json:"twoFactorRequired"`
	TwoFactorRecoveryCount int        `json:"twoFactorRecoveryCount"`
	LastLoginAt            *time.Time `json:"lastLoginAt"`
	CreatedAt              time.Time  `json:"createdAt"`
	UpdatedAt              time.Time  `json:"updatedAt"`
	SubscriptionTier       string     `json:"subscriptionTier"`
	SubscriptionPlanID     *uint      `json:"subscriptionPlanID"`
	SubscriptionPlanName   string     `json:"subscriptionPlanName"`
	SubscriptionStatus     string     `json:"subscriptionStatus"`
	SubscriptionExpiresAt  *time.Time `json:"subscriptionExpiresAt"`
	BillingAccountCurrency string     `json:"billingAccountCurrency"`
	BillingBalanceNanousd  int64      `json:"billingBalanceNanousd"`
	BillingBalanceUSD      float64    `json:"billingBalanceUSD"`
	BillingAccountStatus   string     `json:"billingAccountStatus"`
}

// UserDataResponse 用户操作响应。
type UserDataResponse struct {
	User UserResponse `json:"user"`
}

// RevokeUserSessionsResponse 管理员吊销用户会话响应。
type RevokeUserSessionsResponse struct {
	Revoked bool `json:"revoked"`
}

// ResetUserPasswordResponse 管理员重置密码响应。
type ResetUserPasswordResponse struct {
	Reset bool `json:"reset"`
}

type ResetUserTwoFactorResponse struct {
	Reset bool `json:"reset"`
}

// DeleteUserResponse 管理员删除用户响应。
type DeleteUserResponse struct {
	Deleted bool `json:"deleted"`
}

// AuthEventResponse 认证事件响应。
type AuthEventResponse struct {
	ID              uint      `json:"id"`
	RequestID       string    `json:"requestID"`
	UserID          uint      `json:"userID"`
	Username        string    `json:"username"`
	UserDisplayName string    `json:"userDisplayName"`
	UserLabel       string    `json:"userLabel"`
	EventType       string    `json:"eventType"`
	Result          string    `json:"result"`
	Reason          string    `json:"reason"`
	ClientIP        string    `json:"clientIP"`
	UserAgent       string    `json:"userAgent"`
	DetailJSON      string    `json:"detailJSON"`
	OccurredAt      time.Time `json:"occurredAt"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

// AuditLogResponse 审计日志响应。
type AuditLogResponse struct {
	ID               uint      `json:"id"`
	RequestID        string    `json:"requestID"`
	ActorUserID      uint      `json:"actorUserID"`
	ActorUsername    string    `json:"actorUsername"`
	ActorDisplayName string    `json:"actorDisplayName"`
	ActorLabel       string    `json:"actorLabel"`
	Action           string    `json:"action"`
	Resource         string    `json:"resource"`
	ResourceID       string    `json:"resourceID"`
	IP               string    `json:"ip"`
	UserAgent        string    `json:"userAgent"`
	DetailJSON       string    `json:"detailJSON"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

// SystemEventResponse 系统事件响应。
type SystemEventResponse struct {
	ID         uint      `json:"id"`
	RequestID  string    `json:"requestID"`
	TraceID    string    `json:"traceID"`
	Level      string    `json:"level"`
	Source     string    `json:"source"`
	Event      string    `json:"event"`
	Resource   string    `json:"resource"`
	ResourceID string    `json:"resourceID"`
	Message    string    `json:"message"`
	DetailJSON string    `json:"detailJSON"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// UsageLogResponse 调用日志响应。
type UsageLogResponse struct {
	ID                  uint      `json:"id"`
	UserID              uint      `json:"userID"`
	Username            string    `json:"username"`
	UserDisplayName     string    `json:"userDisplayName"`
	UserLabel           string    `json:"userLabel"`
	ConversationID      uint      `json:"conversationID"`
	ProviderProtocol    string    `json:"providerProtocol"`
	UpstreamName        string    `json:"upstreamName"`
	PlatformModelName   string    `json:"platformModelName"`
	RoutedBindingCode   string    `json:"routedBindingCode"`
	UpstreamModelName   string    `json:"upstreamModelName"`
	IsFreeModel         bool      `json:"isFreeModel"`
	UsageDate           time.Time `json:"usageDate"`
	InputTokens         int64     `json:"inputTokens"`
	CacheReadTokens     int64     `json:"cacheReadTokens"`
	CacheWriteTokens    int64     `json:"cacheWriteTokens"`
	CacheWrite5mTokens  int64     `json:"cacheWrite5mTokens"`
	CacheWrite1hTokens  int64     `json:"cacheWrite1hTokens"`
	OutputTokens        int64     `json:"outputTokens"`
	ReasoningTokens     int64     `json:"reasoningTokens"`
	CallCount           int64     `json:"callCount"`
	DurationSeconds     int64     `json:"durationSeconds"`
	LatencyMS           int64     `json:"latencyMS"`
	UsageSpeed          string    `json:"usageSpeed"`
	ServiceTier         string    `json:"serviceTier"`
	BilledCurrency      string    `json:"billedCurrency"`
	BilledNanousd       int64     `json:"billedNanousd"`
	BilledUSD           float64   `json:"billedUSD"`
	PricingSnapshotJSON string    `json:"pricingSnapshotJSON"`
	CreatedAt           time.Time `json:"createdAt"`
	UpdatedAt           time.Time `json:"updatedAt"`
}

// PaymentOrderResponse 支付订单记录响应。
type PaymentOrderResponse struct {
	ID                 uint       `json:"id"`
	OrderNo            string     `json:"orderNo"`
	OrderType          string     `json:"orderType"`
	UserID             uint       `json:"userID"`
	Username           string     `json:"username"`
	UserDisplayName    string     `json:"userDisplayName"`
	UserLabel          string     `json:"userLabel"`
	PlanID             uint       `json:"planID"`
	PriceID            uint       `json:"priceID"`
	Provider           string     `json:"provider"`
	Status             string     `json:"status"`
	BaseCurrency       string     `json:"baseCurrency"`
	BaseAmountCents    int64      `json:"baseAmountCents"`
	PayCurrency        string     `json:"payCurrency"`
	PayAmountCents     int64      `json:"payAmountCents"`
	FXRate             string     `json:"fxRate"`
	CreditNanousd      int64      `json:"creditNanousd"`
	CreditUSD          float64    `json:"creditUSD"`
	BillingInterval    string     `json:"billingInterval"`
	Cycles             int        `json:"cycles"`
	ExternalPaymentID  string     `json:"externalPaymentID"`
	ExternalCheckoutID string     `json:"externalCheckoutID"`
	PaidAt             *time.Time `json:"paidAt"`
	ExpiredAt          *time.Time `json:"expiredAt"`
	SnapshotJSON       string     `json:"snapshotJSON"`
	CreatedAt          time.Time  `json:"createdAt"`
	UpdatedAt          time.Time  `json:"updatedAt"`
}

// ConversationEventResponse 对话事件响应。
type ConversationEventResponse struct {
	ID              uint       `json:"id"`
	MessageID       uint       `json:"messageID"`
	ConversationID  uint       `json:"conversationID"`
	UserID          uint       `json:"userID"`
	Username        string     `json:"username"`
	UserDisplayName string     `json:"userDisplayName"`
	UserLabel       string     `json:"userLabel"`
	RunID           string     `json:"runID"`
	EventScope      string     `json:"eventScope"`
	EventID         string     `json:"eventID"`
	EventType       string     `json:"eventType"`
	Phase           string     `json:"phase"`
	Stage           string     `json:"stage"`
	RoundID         string     `json:"roundID"`
	ParentEventID   string     `json:"parentEventID"`
	Status          string     `json:"status"`
	Title           string     `json:"title"`
	Summary         string     `json:"summary"`
	ContentMarkdown string     `json:"contentMarkdown"`
	PayloadJSON     string     `json:"payloadJSON"`
	Seq             int        `json:"seq"`
	ToolCallID      string     `json:"toolCallID"`
	ToolName        string     `json:"toolName"`
	LatencyMS       int64      `json:"latencyMS"`
	InputJSON       string     `json:"inputJSON"`
	OutputJSON      string     `json:"outputJSON"`
	ErrorJSON       string     `json:"errorJSON"`
	StartedAt       time.Time  `json:"startedAt"`
	EndedAt         *time.Time `json:"endedAt"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

// ── Swagger 文档 DTO ────────────────────────────────────────────────────────

// UserListResponseDoc 用户分页响应。
type UserListResponseDoc struct {
	ErrorMsg string `json:"errorMsg"`
	Data     struct {
		Total   int64          `json:"total"`
		Results []UserResponse `json:"results"`
	} `json:"data"`
}

// CreateUserResponseDoc 创建用户响应。
type CreateUserResponseDoc struct {
	ErrorMsg string           `json:"errorMsg"`
	Data     UserDataResponse `json:"data"`
}

// RevokeUserSessionsResponseDoc 管理员吊销用户会话响应。
type RevokeUserSessionsResponseDoc struct {
	ErrorMsg string                     `json:"errorMsg"`
	Data     RevokeUserSessionsResponse `json:"data"`
}

// UpdateUserStatusResponseDoc 管理员更新用户状态响应。
type UpdateUserStatusResponseDoc struct {
	ErrorMsg string           `json:"errorMsg"`
	Data     UserDataResponse `json:"data"`
}

// ResetUserPasswordResponseDoc 管理员重置用户密码响应。
type ResetUserPasswordResponseDoc struct {
	ErrorMsg string                    `json:"errorMsg"`
	Data     ResetUserPasswordResponse `json:"data"`
}

// DeleteUserResponseDoc 管理员删除用户响应。
type DeleteUserResponseDoc struct {
	ErrorMsg string             `json:"errorMsg"`
	Data     DeleteUserResponse `json:"data"`
}

// UserAuthEventListResponseDoc 用户认证事件分页响应。
type UserAuthEventListResponseDoc struct {
	ErrorMsg string `json:"errorMsg"`
	Data     struct {
		Total   int64               `json:"total"`
		Results []AuthEventResponse `json:"results"`
	} `json:"data"`
}

// AuditLogListResponseDoc 审计日志分页响应。
type AuditLogListResponseDoc struct {
	ErrorMsg string `json:"errorMsg"`
	Data     struct {
		Total   int64              `json:"total"`
		Results []AuditLogResponse `json:"results"`
	} `json:"data"`
}

// SystemEventListResponseDoc 系统事件分页响应。
type SystemEventListResponseDoc struct {
	ErrorMsg string `json:"errorMsg"`
	Data     struct {
		Total   int64                 `json:"total"`
		Results []SystemEventResponse `json:"results"`
	} `json:"data"`
}

// UsageLogListResponseDoc 调用日志分页响应。
type UsageLogListResponseDoc struct {
	ErrorMsg string `json:"errorMsg"`
	Data     struct {
		Total   int64              `json:"total"`
		Results []UsageLogResponse `json:"results"`
	} `json:"data"`
}

// PaymentOrderListResponseDoc 支付订单分页响应。
type PaymentOrderListResponseDoc struct {
	ErrorMsg string `json:"errorMsg"`
	Data     struct {
		Total   int64                  `json:"total"`
		Results []PaymentOrderResponse `json:"results"`
	} `json:"data"`
}

// ConversationEventListResponseDoc 对话事件分页响应。
type ConversationEventListResponseDoc struct {
	ErrorMsg string `json:"errorMsg"`
	Data     struct {
		Total   int64                       `json:"total"`
		Results []ConversationEventResponse `json:"results"`
	} `json:"data"`
}

// ErrorDoc 错误响应。
type ErrorDoc struct {
	ErrorMsg  string      `json:"errorMsg"`
	ErrorCode string      `json:"errorCode,omitempty"`
	Details   interface{} `json:"details,omitempty"`
	RequestID string      `json:"requestId,omitempty"`
	Data      interface{} `json:"data"`
}

// ── mapping 函数 ──────────────────────────────────────────────────────────────

func toUserResponse(v userview.UserView) UserResponse {
	return UserResponse{
		ID:                     v.ID,
		PublicID:               v.PublicID,
		Username:               v.Username,
		DisplayName:            v.DisplayName,
		AvatarURL:              v.AvatarURL,
		Email:                  v.Email,
		Phone:                  v.Phone,
		Role:                   v.Role,
		Status:                 v.Status,
		Timezone:               v.Timezone,
		Locale:                 v.Locale,
		ProfilePreferences:     v.ProfilePreferences,
		AppearancePreferences:  v.AppearancePreferences,
		EmailVerifiedAt:        v.EmailVerifiedAt,
		PhoneVerifiedAt:        v.PhoneVerifiedAt,
		TwoFactorAvailable:     v.TwoFactorAvailable,
		TwoFactorEnabled:       v.TwoFactorEnabled,
		TwoFactorRequired:      v.TwoFactorRequired,
		TwoFactorRecoveryCount: v.TwoFactorRecoveryCount,
		LastLoginAt:            v.LastLoginAt,
		CreatedAt:              v.CreatedAt,
		UpdatedAt:              v.UpdatedAt,
		SubscriptionTier:       v.SubscriptionTier,
		SubscriptionPlanID:     v.SubscriptionPlanID,
		SubscriptionPlanName:   v.SubscriptionPlanName,
		SubscriptionStatus:     v.SubscriptionStatus,
		SubscriptionExpiresAt:  v.SubscriptionExpiresAt,
		BillingAccountCurrency: v.BillingAccountCurrency,
		BillingBalanceNanousd:  v.BillingBalanceNanousd,
		BillingBalanceUSD:      float64(v.BillingBalanceNanousd) / 1000000000.0,
		BillingAccountStatus:   v.BillingAccountStatus,
	}
}

func toAuthEventResponse(e domainuser.AuthEvent, label appadmin.UserLabel) AuthEventResponse {
	return AuthEventResponse{
		ID:              e.ID,
		RequestID:       e.RequestID,
		UserID:          e.UserID,
		Username:        label.Username,
		UserDisplayName: label.DisplayName,
		UserLabel:       label.Label,
		EventType:       e.EventType,
		Result:          e.Result,
		Reason:          e.Reason,
		ClientIP:        e.ClientIP,
		UserAgent:       e.UserAgent,
		DetailJSON:      e.DetailJSON,
		OccurredAt:      e.OccurredAt,
		CreatedAt:       e.CreatedAt,
		UpdatedAt:       e.UpdatedAt,
	}
}

func toAuditLogResponse(l domainaudit.Log, label appadmin.UserLabel) AuditLogResponse {
	return AuditLogResponse{
		ID:               l.ID,
		RequestID:        l.RequestID,
		ActorUserID:      l.ActorUserID,
		ActorUsername:    label.Username,
		ActorDisplayName: label.DisplayName,
		ActorLabel:       label.Label,
		Action:           l.Action,
		Resource:         l.Resource,
		ResourceID:       l.ResourceID,
		IP:               l.IP,
		UserAgent:        l.UserAgent,
		DetailJSON:       l.DetailJSON,
		CreatedAt:        l.CreatedAt,
		UpdatedAt:        l.UpdatedAt,
	}
}

func toSystemEventResponse(item domainsystemevent.Event) SystemEventResponse {
	return SystemEventResponse{
		ID:         item.ID,
		RequestID:  item.RequestID,
		TraceID:    item.TraceID,
		Level:      item.Level,
		Source:     item.Source,
		Event:      item.Event,
		Resource:   item.Resource,
		ResourceID: item.ResourceID,
		Message:    item.Message,
		DetailJSON: item.DetailJSON,
		CreatedAt:  item.CreatedAt,
		UpdatedAt:  item.UpdatedAt,
	}
}

func toUsageLogResponse(item domainbilling.UsageLedger, label appadmin.UserLabel) UsageLogResponse {
	return UsageLogResponse{
		ID:                  item.ID,
		UserID:              item.UserID,
		Username:            label.Username,
		UserDisplayName:     label.DisplayName,
		UserLabel:           label.Label,
		ConversationID:      item.ConversationID,
		ProviderProtocol:    item.ProviderProtocol,
		UpstreamName:        item.UpstreamName,
		PlatformModelName:   item.PlatformModelName,
		RoutedBindingCode:   item.RoutedBindingCode,
		UpstreamModelName:   item.UpstreamModelName,
		IsFreeModel:         item.IsFreeModel,
		UsageDate:           item.UsageDate,
		InputTokens:         item.InputTokens,
		CacheReadTokens:     item.CacheReadTokens,
		CacheWriteTokens:    item.CacheWriteTokens,
		CacheWrite5mTokens:  item.CacheWrite5mTokens,
		CacheWrite1hTokens:  item.CacheWrite1hTokens,
		OutputTokens:        item.OutputTokens,
		ReasoningTokens:     item.ReasoningTokens,
		CallCount:           item.CallCount,
		DurationSeconds:     item.DurationSeconds,
		LatencyMS:           item.LatencyMS,
		UsageSpeed:          item.UsageSpeed,
		ServiceTier:         item.ServiceTier,
		BilledCurrency:      item.BilledCurrency,
		BilledNanousd:       item.BilledNanousd,
		BilledUSD:           float64(item.BilledNanousd) / 1_000_000_000,
		PricingSnapshotJSON: item.PricingSnapshotJSON,
		CreatedAt:           item.CreatedAt,
		UpdatedAt:           item.UpdatedAt,
	}
}

func toPaymentOrderResponse(item domainbilling.PaymentOrder, label appadmin.UserLabel) PaymentOrderResponse {
	return PaymentOrderResponse{
		ID:                 item.ID,
		OrderNo:            item.OrderNo,
		OrderType:          item.OrderType,
		UserID:             item.UserID,
		Username:           label.Username,
		UserDisplayName:    label.DisplayName,
		UserLabel:          label.Label,
		PlanID:             item.PlanID,
		PriceID:            item.PriceID,
		Provider:           item.Provider,
		Status:             item.Status,
		BaseCurrency:       item.BaseCurrency,
		BaseAmountCents:    item.BaseAmountCents,
		PayCurrency:        item.PayCurrency,
		PayAmountCents:     item.PayAmountCents,
		FXRate:             item.FXRate,
		CreditNanousd:      item.CreditNanousd,
		CreditUSD:          float64(item.CreditNanousd) / 1_000_000_000,
		BillingInterval:    item.BillingInterval,
		Cycles:             item.Cycles,
		ExternalPaymentID:  item.ExternalPaymentID,
		ExternalCheckoutID: item.ExternalCheckoutID,
		PaidAt:             item.PaidAt,
		ExpiredAt:          item.ExpiredAt,
		SnapshotJSON:       item.SnapshotJSON,
		CreatedAt:          item.CreatedAt,
		UpdatedAt:          item.UpdatedAt,
	}
}

func toConversationEventResponse(item domainconversation.EventLog, label appadmin.UserLabel) ConversationEventResponse {
	return ConversationEventResponse{
		ID:              item.ID,
		MessageID:       item.MessageID,
		ConversationID:  item.ConversationID,
		UserID:          item.UserID,
		Username:        label.Username,
		UserDisplayName: label.DisplayName,
		UserLabel:       label.Label,
		RunID:           item.RunID,
		EventScope:      item.EventScope,
		EventID:         item.EventID,
		EventType:       item.EventType,
		Phase:           item.Phase,
		Stage:           item.Stage,
		RoundID:         item.RoundID,
		ParentEventID:   item.ParentEventID,
		Status:          item.Status,
		Title:           item.Title,
		Summary:         item.Summary,
		ContentMarkdown: item.ContentMarkdown,
		PayloadJSON:     item.PayloadJSON,
		Seq:             item.Seq,
		ToolCallID:      item.ToolCallID,
		ToolName:        item.ToolName,
		LatencyMS:       item.LatencyMS,
		InputJSON:       item.InputJSON,
		OutputJSON:      item.OutputJSON,
		ErrorJSON:       item.ErrorJSON,
		StartedAt:       item.StartedAt,
		EndedAt:         item.EndedAt,
		CreatedAt:       item.CreatedAt,
		UpdatedAt:       item.UpdatedAt,
	}
}

func toAppPatchUserInput(req PatchUserRequest) appadmin.PatchUserInput {
	return appadmin.PatchUserInput{
		AvatarURL:             req.AvatarURL,
		DisplayName:           req.DisplayName,
		Email:                 req.Email,
		Phone:                 req.Phone,
		Role:                  req.Role,
		Status:                req.Status,
		Timezone:              req.Timezone,
		Locale:                req.Locale,
		ProfilePreferences:    req.ProfilePreferences,
		SubscriptionTier:      req.SubscriptionTier,
		SubscriptionExpiresAt: req.SubscriptionExpiresAt,
		Reason:                req.Reason,
	}
}
