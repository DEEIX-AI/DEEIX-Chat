package billing

import (
	"encoding/json"
	"math"
	"strings"
	"time"

	appbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/billing"
	domainbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/billing"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/nativetool"
)

// ── 请求 DTO ─────────────────────────────────────────────────────────────────

// SubscribeRequest 订阅请求。
type SubscribeRequest struct {
	PriceID uint `json:"priceID" binding:"required,min=1"`
	Cycles  int  `json:"cycles" binding:"min=1,max=120"`
}

// CreateCheckoutRequest 创建支付收银台请求。
type CreateCheckoutRequest struct {
	OrderType       string  `json:"orderType" binding:"omitempty,oneof=subscription topup"`
	PriceID         uint    `json:"priceID" binding:"omitempty,min=1"`
	AmountUSD       float64 `json:"amountUSD" binding:"omitempty,min=0"`
	Cycles          int     `json:"cycles" binding:"min=1,max=120"`
	PaymentProvider string  `json:"paymentProvider" binding:"omitempty,oneof=stripe epay"`
	EPayType        string  `json:"epayType" binding:"omitempty,max=32"`
	SuccessURL      string  `json:"successURL" binding:"omitempty,max=512"`
	CancelURL       string  `json:"cancelURL" binding:"omitempty,max=512"`
}

// UpsertModelPricingRequest 保存模型计费单价。金额单位均为美元。
type UpsertModelPricingRequest struct {
	PlatformModelName       string  `json:"platformModelName" binding:"required,max=128"`
	Currency                string  `json:"currency" binding:"omitempty,max=16"`
	IsFree                  bool    `json:"isFree"`
	PricingMode             string  `json:"pricingMode" binding:"omitempty,oneof=token call duration tiered"`
	InputUSDPerMTokens      float64 `json:"inputUSDPerMTokens" binding:"min=0"`
	CacheReadUSDPerMTokens  float64 `json:"cacheReadUSDPerMTokens" binding:"min=0"`
	CacheWriteUSDPerMTokens float64 `json:"cacheWriteUSDPerMTokens" binding:"min=0"`
	OutputUSDPerMTokens     float64 `json:"outputUSDPerMTokens" binding:"min=0"`
	CallUSDPerCall          float64 `json:"callUSDPerCall" binding:"min=0"`
	DurationUSDPerSecond    float64 `json:"durationUSDPerSecond" binding:"min=0"`
	TieredPricingJSON       string  `json:"tieredPricingJSON" binding:"max=20000"`
}

// BillingConfigRequest 保存计费全局配置。
type BillingConfigRequest struct {
	Mode                     string                     `json:"mode" binding:"required,oneof=self period usage"`
	PrepaidAmountUSD         *float64                   `json:"prepaidAmountUSD" binding:"omitempty,min=0"`
	NativeToolBillingEnabled *bool                      `json:"nativeToolBillingEnabled"`
	NativeToolPricing        []NativeToolPricingRequest `json:"nativeToolPricing"`
}

// UpdateBillingAccountBalanceRequest 管理员设置用户按量余额。
type UpdateBillingAccountBalanceRequest struct {
	BalanceUSD  float64 `json:"balanceUSD" binding:"min=0"`
	Description string  `json:"description" binding:"omitempty,max=255"`
}

// CreateRedemptionCodeRequest 创建兑换码请求。
type CreateRedemptionCodeRequest struct {
	Code           string     `json:"code" binding:"omitempty,min=3,max=64"`
	Quantity       int        `json:"quantity" binding:"omitempty,min=1,max=100"`
	Mode           string     `json:"mode" binding:"required,oneof=usage period"`
	CreditUSD      float64    `json:"creditUSD" binding:"omitempty,min=0"`
	PlanID         uint       `json:"planID" binding:"omitempty,min=1"`
	DurationDays   int        `json:"durationDays" binding:"omitempty,min=0,max=3660"`
	MaxRedemptions *int       `json:"maxRedemptions" binding:"omitempty,min=1"`
	PerUserLimit   int        `json:"perUserLimit" binding:"omitempty,min=1,max=100"`
	ExpiresAt      *time.Time `json:"expiresAt"`
	Description    string     `json:"description" binding:"omitempty,max=255"`
}

// PatchRedemptionCodeRequest 更新兑换码请求。
type PatchRedemptionCodeRequest struct {
	Status         *string             `json:"status" binding:"omitempty,oneof=active inactive"`
	MaxRedemptions nullableIntRequest  `json:"maxRedemptions"`
	PerUserLimit   *int                `json:"perUserLimit" binding:"omitempty,min=1,max=100"`
	ExpiresAt      nullableTimeRequest `json:"expiresAt"`
	Description    *string             `json:"description" binding:"omitempty,max=255"`
}

// PatchRedemptionCodeRequestDoc 用于 Swagger 展示 nullable 字段的真实 JSON 形态。
type PatchRedemptionCodeRequestDoc struct {
	Status         *string    `json:"status" enums:"active,inactive"`
	MaxRedemptions *int       `json:"maxRedemptions"`
	PerUserLimit   *int       `json:"perUserLimit" minimum:"1" maximum:"100"`
	ExpiresAt      *time.Time `json:"expiresAt"`
	Description    *string    `json:"description" maxLength:"255"`
}

// BatchDeleteRedemptionCodeRequest 批量删除兑换码请求。
type BatchDeleteRedemptionCodeRequest struct {
	IDs []uint `json:"ids" binding:"required,min=1,dive,gt=0"`
}

// RedeemCodeRequest 用户兑换请求。
type RedeemCodeRequest struct {
	Code string `json:"code" binding:"required,min=3,max=64"`
}

// UpdateBillingPlanRequest 保存周期套餐。
type UpdateBillingPlanRequest struct {
	Name            string  `json:"name" binding:"required,min=1,max=64"`
	Description     string  `json:"description" binding:"max=255"`
	PeriodCreditUSD float64 `json:"periodCreditUSD" binding:"min=0"`
	DiscountPercent int     `json:"discountPercent" binding:"min=0,max=100"`
	Currency        string  `json:"currency" binding:"omitempty,max=16"`
	AmountUSD       float64 `json:"amountUSD" binding:"min=0"`
	BillingInterval string  `json:"billingInterval" binding:"required,oneof=month year lifetime"`
}

type nullableIntRequest struct {
	Set   bool
	Value *int
}

func (v *nullableIntRequest) UnmarshalJSON(raw []byte) error {
	v.Set = true
	if string(raw) == "null" {
		v.Value = nil
		return nil
	}
	var parsed int
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return err
	}
	v.Value = &parsed
	return nil
}

type nullableTimeRequest struct {
	Set   bool
	Value *time.Time
}

func (v *nullableTimeRequest) UnmarshalJSON(raw []byte) error {
	v.Set = true
	if string(raw) == "null" {
		v.Value = nil
		return nil
	}
	var text string
	if err := json.Unmarshal(raw, &text); err != nil {
		return err
	}
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(text))
	if err != nil {
		return err
	}
	v.Value = &parsed
	return nil
}

// ── 响应 DTO ─────────────────────────────────────────────────────────────────

// BillingPriceResponse 面向前端的价格视图响应。
type BillingPriceResponse struct {
	ID              uint   `json:"id"`
	PlanID          uint   `json:"planID"`
	Code            string `json:"code"`
	BillingInterval string `json:"billingInterval"`
	Currency        string `json:"currency"`
	AmountCents     int64  `json:"amountCents"`
	IsDefault       bool   `json:"isDefault"`
}

// BillingPlanResponse 面向前端的套餐视图响应。
type BillingPlanResponse struct {
	ID                  uint                   `json:"id"`
	Code                string                 `json:"code"`
	Name                string                 `json:"name"`
	Description         string                 `json:"description"`
	FeatureJSON         string                 `json:"featureJSON"`
	PeriodCreditUSD     float64                `json:"periodCreditUSD"`
	PeriodCreditNanousd int64                  `json:"periodCreditNanousd"`
	DiscountPercent     int                    `json:"discountPercent"`
	SortOrder           int                    `json:"sortOrder"`
	IsActive            bool                   `json:"isActive"`
	Prices              []BillingPriceResponse `json:"prices"`
}

// SubscriptionResponse 面向前端的订阅视图响应。
type SubscriptionResponse struct {
	ID                   uint       `json:"id"`
	UserID               uint       `json:"userID"`
	PlanID               uint       `json:"planID"`
	PriceID              uint       `json:"priceID"`
	Status               string     `json:"status"`
	StartAt              time.Time  `json:"startAt"`
	CurrentPeriodStartAt time.Time  `json:"currentPeriodStartAt"`
	CurrentPeriodEndAt   *time.Time `json:"currentPeriodEndAt"`
	CancelAtPeriodEnd    bool       `json:"cancelAtPeriodEnd"`
	AutoRenew            bool       `json:"autoRenew"`
}

// SubscriptionEntitlementResponse 面向前端的订阅权益队列响应。
type SubscriptionEntitlementResponse struct {
	SubscriptionResponse
	Plan      BillingPlanResponse `json:"plan"`
	IsCurrent bool                `json:"isCurrent"`
}

// SubscriptionDataResponse 订阅操作响应。
type SubscriptionDataResponse struct {
	Subscription SubscriptionResponse `json:"subscription"`
}

// CheckoutResponse 支付收银台响应。
type CheckoutResponse struct {
	OrderNo            string     `json:"orderNo"`
	OrderType          string     `json:"orderType"`
	Provider           string     `json:"provider"`
	Status             string     `json:"status"`
	CheckoutURL        string     `json:"checkoutURL"`
	ExternalCheckoutID string     `json:"externalCheckoutID"`
	BaseAmountCents    int64      `json:"baseAmountCents"`
	BaseCurrency       string     `json:"baseCurrency"`
	PayAmountCents     int64      `json:"payAmountCents"`
	PayCurrency        string     `json:"payCurrency"`
	FXRate             string     `json:"fxRate"`
	CreditNanousd      int64      `json:"creditNanousd"`
	CreditUSD          float64    `json:"creditUSD"`
	ExpiredAt          *time.Time `json:"expiredAt"`
}

// CheckoutDataResponse 支付收银台操作响应。
type CheckoutDataResponse struct {
	Checkout CheckoutResponse `json:"checkout"`
}

type PaymentWebhookIgnoredResponse struct {
	Ignored bool `json:"ignored"`
}

type PaymentWebhookOKResponse struct {
	OK bool `json:"ok"`
}

// UsageLedgerResponse 用量账本响应。
type UsageLedgerResponse struct {
	ID                  uint      `json:"id"`
	UserID              uint      `json:"userID"`
	ConversationID      uint      `json:"conversationID"`
	ProviderProtocol    string    `json:"providerProtocol"`
	PlatformModelName   string    `json:"platformModelName"`
	RoutedBindingCode   string    `json:"routedBindingCode"`
	UpstreamModelName   string    `json:"upstreamModelName"`
	ModelVendor         string    `json:"modelVendor"`
	ModelIcon           string    `json:"modelIcon"`
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

// UsageMonthlyResponse 月度用量聚合响应。
type UsageMonthlyResponse struct {
	MonthStartAt     time.Time `json:"monthStartAt"`
	RecordCount      int64     `json:"recordCount"`
	InputTokens      int64     `json:"inputTokens"`
	CacheReadTokens  int64     `json:"cacheReadTokens"`
	CacheWriteTokens int64     `json:"cacheWriteTokens"`
	OutputTokens     int64     `json:"outputTokens"`
	ReasoningTokens  int64     `json:"reasoningTokens"`
	TotalTokens      int64     `json:"totalTokens"`
	CallCount        int64     `json:"callCount"`
	DurationSeconds  int64     `json:"durationSeconds"`
	AvgLatencyMS     int64     `json:"avgLatencyMS"`
	BilledNanousd    int64     `json:"billedNanousd"`
	BilledUSD        float64   `json:"billedUSD"`
}

// UsageDailyResponse 每日用量聚合响应。
type UsageDailyResponse struct {
	UsageDate        time.Time                 `json:"usageDate"`
	RecordCount      int64                     `json:"recordCount"`
	InputTokens      int64                     `json:"inputTokens"`
	CacheReadTokens  int64                     `json:"cacheReadTokens"`
	CacheWriteTokens int64                     `json:"cacheWriteTokens"`
	OutputTokens     int64                     `json:"outputTokens"`
	ReasoningTokens  int64                     `json:"reasoningTokens"`
	TotalTokens      int64                     `json:"totalTokens"`
	CallCount        int64                     `json:"callCount"`
	DurationSeconds  int64                     `json:"durationSeconds"`
	AvgLatencyMS     int64                     `json:"avgLatencyMS"`
	BilledNanousd    int64                     `json:"billedNanousd"`
	BilledUSD        float64                   `json:"billedUSD"`
	Models           []UsageDailyModelResponse `json:"models"`
}

// UsageDailyModelResponse 每日模型维度用量聚合响应。
type UsageDailyModelResponse struct {
	PlatformModelName string  `json:"platformModelName"`
	RecordCount       int64   `json:"recordCount"`
	InputTokens       int64   `json:"inputTokens"`
	CacheReadTokens   int64   `json:"cacheReadTokens"`
	CacheWriteTokens  int64   `json:"cacheWriteTokens"`
	OutputTokens      int64   `json:"outputTokens"`
	ReasoningTokens   int64   `json:"reasoningTokens"`
	TotalTokens       int64   `json:"totalTokens"`
	CallCount         int64   `json:"callCount"`
	DurationSeconds   int64   `json:"durationSeconds"`
	AvgLatencyMS      int64   `json:"avgLatencyMS"`
	BilledNanousd     int64   `json:"billedNanousd"`
	BilledUSD         float64 `json:"billedUSD"`
}

// BillingAccountResponse 按量计费账户响应。
type BillingAccountResponse struct {
	UserID         uint      `json:"userID"`
	Currency       string    `json:"currency"`
	BalanceNanousd int64     `json:"balanceNanousd"`
	BalanceUSD     float64   `json:"balanceUSD"`
	Status         string    `json:"status"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// BillingAccountDataResponse 按量计费账户操作响应。
type BillingAccountDataResponse struct {
	Account BillingAccountResponse `json:"account"`
}

// BillingOverviewResponse 当前用户计费概览响应。
type BillingOverviewResponse struct {
	Mode                     string                            `json:"mode"`
	Plan                     *BillingPlanResponse              `json:"plan"`
	PeriodStartAt            *time.Time                        `json:"periodStartAt"`
	PeriodEndAt              *time.Time                        `json:"periodEndAt"`
	PeriodCreditUSD          float64                           `json:"periodCreditUSD"`
	PeriodCreditNanousd      int64                             `json:"periodCreditNanousd"`
	PeriodUsedUSD            float64                           `json:"periodUsedUSD"`
	PeriodUsedNanousd        int64                             `json:"periodUsedNanousd"`
	PeriodRemainingUSD       float64                           `json:"periodRemainingUSD"`
	PeriodRemainingNanousd   int64                             `json:"periodRemainingNanousd"`
	Account                  *BillingAccountResponse           `json:"account"`
	SubscriptionEntitlements []SubscriptionEntitlementResponse `json:"subscriptionEntitlements"`
}

// BillingOverviewDataResponse 当前用户计费概览操作响应。
type BillingOverviewDataResponse struct {
	Overview BillingOverviewResponse `json:"overview"`
}

// RedemptionCodeResponse 后台兑换码响应。
type RedemptionCodeResponse struct {
	ID                   uint       `json:"id"`
	Code                 string     `json:"code,omitempty"`
	CodeHint             string     `json:"codeHint"`
	Mode                 string     `json:"mode"`
	RewardType           string     `json:"rewardType"`
	CreditUSD            float64    `json:"creditUSD"`
	CreditNanousd        int64      `json:"creditNanousd"`
	PlanID               uint       `json:"planID"`
	DurationDays         int        `json:"durationDays"`
	MaxRedemptions       *int       `json:"maxRedemptions"`
	PerUserLimit         int        `json:"perUserLimit"`
	RedeemedCount        int        `json:"redeemedCount"`
	RemainingRedemptions *int       `json:"remainingRedemptions"`
	Status               string     `json:"status"`
	ExpiresAt            *time.Time `json:"expiresAt"`
	Description          string     `json:"description"`
	CreatedByUserID      uint       `json:"createdByUserID"`
	CreatedAt            time.Time  `json:"createdAt"`
	UpdatedAt            time.Time  `json:"updatedAt"`
}

// RedemptionResponse 用户兑换记录响应。
type RedemptionResponse struct {
	ID                   uint      `json:"id"`
	CodeID               uint      `json:"codeID"`
	UserID               uint      `json:"userID"`
	Mode                 string    `json:"mode"`
	RewardType           string    `json:"rewardType"`
	CreditUSD            float64   `json:"creditUSD"`
	CreditNanousd        int64     `json:"creditNanousd"`
	PlanID               uint      `json:"planID"`
	SubscriptionID       uint      `json:"subscriptionID"`
	BalanceTransactionID uint      `json:"balanceTransactionID"`
	CreatedAt            time.Time `json:"createdAt"`
}

// RedemptionCodeListDataResponse 后台兑换码分页响应。
type RedemptionCodeListDataResponse struct {
	Total   int64                    `json:"total"`
	Results []RedemptionCodeResponse `json:"results"`
}

// RedemptionCodeDataResponse 后台兑换码操作响应。
type RedemptionCodeDataResponse struct {
	Code RedemptionCodeResponse `json:"code"`
}

// RedemptionCodeCreateDataResponse 后台批量创建兑换码响应。
type RedemptionCodeCreateDataResponse struct {
	Results []RedemptionCodeResponse `json:"results"`
}

// RedemptionCodeDeleteDataResponse 后台兑换码删除响应。
type RedemptionCodeDeleteDataResponse struct {
	Deleted bool `json:"deleted"`
}

// BatchDeleteRedemptionCodeResultResponse 单个批量删除结果响应。
type BatchDeleteRedemptionCodeResultResponse struct {
	ID     uint   `json:"id"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// BatchDeleteRedemptionCodeDataResponse 批量删除兑换码响应。
type BatchDeleteRedemptionCodeDataResponse struct {
	Total         int                                       `json:"total"`
	SuccessCount  int                                       `json:"successCount"`
	NotFoundCount int                                       `json:"notFoundCount"`
	FailedCount   int                                       `json:"failedCount"`
	Results       []BatchDeleteRedemptionCodeResultResponse `json:"results"`
}

// RedemptionApplyDataResponse 用户兑换响应。
type RedemptionApplyDataResponse struct {
	Redemption   RedemptionResponse      `json:"redemption"`
	Account      *BillingAccountResponse `json:"account,omitempty"`
	Subscription *SubscriptionResponse   `json:"subscription,omitempty"`
	Overview     BillingOverviewResponse `json:"overview"`
}

// ModelPricingResponse 模型计费单价响应。金额单位均为美元。
type ModelPricingResponse struct {
	ID                          uint      `json:"id"`
	PlatformModelName           string    `json:"platformModelName"`
	ModelVendor                 string    `json:"modelVendor"`
	ModelIcon                   string    `json:"modelIcon"`
	Currency                    string    `json:"currency"`
	IsFree                      bool      `json:"isFree"`
	PricingMode                 string    `json:"pricingMode"`
	InputUSDPerMTokens          float64   `json:"inputUSDPerMTokens"`
	CacheReadUSDPerMTokens      float64   `json:"cacheReadUSDPerMTokens"`
	CacheWriteUSDPerMTokens     float64   `json:"cacheWriteUSDPerMTokens"`
	OutputUSDPerMTokens         float64   `json:"outputUSDPerMTokens"`
	CallUSDPerCall              float64   `json:"callUSDPerCall"`
	DurationUSDPerSecond        float64   `json:"durationUSDPerSecond"`
	TieredPricingJSON           string    `json:"tieredPricingJSON"`
	InputNanousdPerMTokens      int64     `json:"inputNanousdPerMTokens"`
	CacheReadNanousdPerMTokens  int64     `json:"cacheReadNanousdPerMTokens"`
	CacheWriteNanousdPerMTokens int64     `json:"cacheWriteNanousdPerMTokens"`
	OutputNanousdPerMTokens     int64     `json:"outputNanousdPerMTokens"`
	CallNanousdPerCall          int64     `json:"callNanousdPerCall"`
	DurationNanousdPerSecond    int64     `json:"durationNanousdPerSecond"`
	CreatedAt                   time.Time `json:"createdAt"`
	UpdatedAt                   time.Time `json:"updatedAt"`
}

// ModelPricingDataResponse 模型单价操作响应。
type ModelPricingDataResponse struct {
	ModelPricing ModelPricingResponse `json:"modelPricing"`
}

// BillingConfigResponse 计费全局配置响应。
type BillingConfigResponse struct {
	Mode                     string                      `json:"mode"`
	PrepaidAmountUSD         float64                     `json:"prepaidAmountUSD"`
	PrepaidAmountNanousd     int64                       `json:"prepaidAmountNanousd"`
	NativeToolBillingEnabled bool                        `json:"nativeToolBillingEnabled"`
	NativeToolPricing        []NativeToolPricingResponse `json:"nativeToolPricing"`
	PaymentProviders         []string                    `json:"paymentProviders"`
	USDToCNYRate             float64                     `json:"usdToCNYRate"`
	EPayTypes                []PaymentTypeResponse       `json:"epayTypes"`
}

// NativeToolPricingResponse 原生工具默认价格响应。
type NativeToolPricingResponse struct {
	Provider     string `json:"provider"`
	ToolKey      string `json:"toolKey"`
	Label        string `json:"label"`
	Description  string `json:"description"`
	Type         string `json:"type"`
	PriceNanousd int64  `json:"priceNanousd"`
	Unit         string `json:"unit"`
	PriceLabel   string `json:"priceLabel"`
	Billable     bool   `json:"billable"`
}

// NativeToolPricingRequest 原生工具价格保存请求。
type NativeToolPricingRequest struct {
	ToolKey      string `json:"toolKey"`
	PriceNanousd int64  `json:"priceNanousd"`
	Unit         string `json:"unit"`
	PriceLabel   string `json:"priceLabel"`
	Billable     bool   `json:"billable"`
}

// PaymentTypeResponse 支付类型响应。
type PaymentTypeResponse struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// BillingConfigDataResponse 计费全局配置操作响应。
type BillingConfigDataResponse struct {
	Config BillingConfigResponse `json:"config"`
}

// BillingPlanDataResponse 套餐操作响应。
type BillingPlanDataResponse struct {
	Plan BillingPlanResponse `json:"plan"`
}

// ── Swagger 文档 DTO ─────────────────────────────────────────────────────────

// PlanListResponseDoc 套餐列表响应。
type PlanListResponseDoc struct {
	ErrorMsg string                `json:"errorMsg"`
	Data     []BillingPlanResponse `json:"data"`
}

// SubscribeResponseDoc 订阅响应。
type SubscribeResponseDoc struct {
	ErrorMsg string                   `json:"errorMsg"`
	Data     SubscriptionDataResponse `json:"data"`
}

// CheckoutResponseDoc 支付收银台响应。
type CheckoutResponseDoc struct {
	ErrorMsg string               `json:"errorMsg"`
	Data     CheckoutDataResponse `json:"data"`
}

// BillingAccountResponseDoc 按量计费账户响应。
type BillingAccountResponseDoc struct {
	ErrorMsg string                     `json:"errorMsg"`
	Data     BillingAccountDataResponse `json:"data"`
}

// BillingOverviewResponseDoc 当前用户计费概览响应。
type BillingOverviewResponseDoc struct {
	ErrorMsg string                      `json:"errorMsg"`
	Data     BillingOverviewDataResponse `json:"data"`
}

// UsageLedgerListResponseDoc 用量账本分页响应。
type UsageLedgerListResponseDoc struct {
	ErrorMsg string `json:"errorMsg"`
	Data     struct {
		Total   int64                 `json:"total"`
		Results []UsageLedgerResponse `json:"results"`
	} `json:"data"`
}

// UsageMonthlyListResponseDoc 月度用量聚合响应。
type UsageMonthlyListResponseDoc struct {
	ErrorMsg string                 `json:"errorMsg"`
	Data     []UsageMonthlyResponse `json:"data"`
}

// UsageDailyListResponseDoc 每日用量聚合响应。
type UsageDailyListResponseDoc struct {
	ErrorMsg string               `json:"errorMsg"`
	Data     []UsageDailyResponse `json:"data"`
}

// ModelPricingListResponseDoc 模型单价分页响应。
type ModelPricingListResponseDoc struct {
	ErrorMsg string `json:"errorMsg"`
	Data     struct {
		Total   int64                  `json:"total"`
		Results []ModelPricingResponse `json:"results"`
	} `json:"data"`
}

// BillingConfigResponseDoc 计费全局配置响应文档。
type BillingConfigResponseDoc struct {
	ErrorMsg string                    `json:"errorMsg"`
	Data     BillingConfigDataResponse `json:"data"`
}

// BillingPlanResponseDoc 套餐操作响应文档。
type BillingPlanResponseDoc struct {
	ErrorMsg string                  `json:"errorMsg"`
	Data     BillingPlanDataResponse `json:"data"`
}

// RedemptionCodeListResponseDoc 后台兑换码列表响应文档。
type RedemptionCodeListResponseDoc struct {
	ErrorMsg string                         `json:"errorMsg"`
	Data     RedemptionCodeListDataResponse `json:"data"`
}

// RedemptionCodeResponseDoc 后台兑换码操作响应文档。
type RedemptionCodeResponseDoc struct {
	ErrorMsg string                     `json:"errorMsg"`
	Data     RedemptionCodeDataResponse `json:"data"`
}

// RedemptionCodeCreateResponseDoc 后台兑换码创建响应文档。
type RedemptionCodeCreateResponseDoc struct {
	ErrorMsg string                           `json:"errorMsg"`
	Data     RedemptionCodeCreateDataResponse `json:"data"`
}

// RedemptionCodeDeleteResponseDoc 后台兑换码删除响应文档。
type RedemptionCodeDeleteResponseDoc struct {
	ErrorMsg string                           `json:"errorMsg"`
	Data     RedemptionCodeDeleteDataResponse `json:"data"`
}

// BatchDeleteRedemptionCodeResponseDoc 后台兑换码批量删除响应文档。
type BatchDeleteRedemptionCodeResponseDoc struct {
	ErrorMsg string                                `json:"errorMsg"`
	Data     BatchDeleteRedemptionCodeDataResponse `json:"data"`
}

// RedemptionApplyResponseDoc 用户兑换响应文档。
type RedemptionApplyResponseDoc struct {
	ErrorMsg string                      `json:"errorMsg"`
	Data     RedemptionApplyDataResponse `json:"data"`
}

// ErrorDoc 错误响应。
type ErrorDoc struct {
	ErrorMsg  string      `json:"errorMsg"`
	ErrorCode string      `json:"errorCode,omitempty"`
	Details   interface{} `json:"details,omitempty"`
	RequestID string      `json:"requestId,omitempty"`
	Data      interface{} `json:"data"`
}

// ── mapping 函数 ─────────────────────────────────────────────────────────────

func toPlanListResponse(views []appbilling.BillingPlanView) []BillingPlanResponse {
	result := make([]BillingPlanResponse, 0, len(views))
	for _, v := range views {
		prices := make([]BillingPriceResponse, 0, len(v.Prices))
		for _, p := range v.Prices {
			prices = append(prices, BillingPriceResponse{
				ID:              p.ID,
				PlanID:          p.PlanID,
				Code:            p.Code,
				BillingInterval: p.BillingInterval,
				Currency:        p.Currency,
				AmountCents:     p.AmountCents,
				IsDefault:       p.IsDefault,
			})
		}
		result = append(result, BillingPlanResponse{
			ID:                  v.ID,
			Code:                v.Code,
			Name:                v.Name,
			Description:         v.Description,
			FeatureJSON:         v.FeatureJSON,
			PeriodCreditUSD:     nanousdToUSD(v.PeriodCreditNanousd),
			PeriodCreditNanousd: v.PeriodCreditNanousd,
			DiscountPercent:     v.DiscountPercent,
			SortOrder:           v.SortOrder,
			IsActive:            v.IsActive,
			Prices:              prices,
		})
	}
	return result
}

func toNativeToolPricingResponses(items []appbilling.NativeToolPricingView) []NativeToolPricingResponse {
	results := make([]NativeToolPricingResponse, 0, len(items))
	for _, item := range items {
		results = append(results, NativeToolPricingResponse{
			Provider:     strings.TrimSpace(item.Provider),
			ToolKey:      strings.TrimSpace(item.ToolKey),
			Label:        strings.TrimSpace(item.Label),
			Description:  strings.TrimSpace(item.Description),
			Type:         strings.TrimSpace(item.Type),
			PriceNanousd: item.PriceNanousd,
			Unit:         strings.TrimSpace(item.Unit),
			PriceLabel:   strings.TrimSpace(item.PriceLabel),
			Billable:     item.Billable,
		})
	}
	return results
}

func nativeToolPricingOverridesFromRequests(items []NativeToolPricingRequest) map[string]nativetool.PricingOverride {
	overrides := make(map[string]nativetool.PricingOverride, len(items))
	for _, item := range items {
		key := strings.TrimSpace(item.ToolKey)
		if key == "" {
			continue
		}
		overrides[key] = nativetool.PricingOverride{
			PriceNanousd: item.PriceNanousd,
			Unit:         strings.TrimSpace(item.Unit),
			PriceLabel:   strings.TrimSpace(item.PriceLabel),
			Billable:     item.Billable,
		}
	}
	return overrides
}

func toSubscriptionResponse(sub *domainbilling.Subscription) SubscriptionResponse {
	return SubscriptionResponse{
		ID:                   sub.ID,
		UserID:               sub.UserID,
		PlanID:               sub.PlanID,
		PriceID:              sub.PriceID,
		Status:               sub.Status,
		StartAt:              sub.StartAt,
		CurrentPeriodStartAt: sub.CurrentPeriodStartAt,
		CurrentPeriodEndAt:   sub.CurrentPeriodEndAt,
		CancelAtPeriodEnd:    sub.CancelAtPeriodEnd,
		AutoRenew:            sub.AutoRenew,
	}
}

func toSubscriptionEntitlementResponses(items []appbilling.SubscriptionEntitlementView) []SubscriptionEntitlementResponse {
	results := make([]SubscriptionEntitlementResponse, 0, len(items))
	for _, item := range items {
		subscription := toSubscriptionResponse(&item.Subscription)
		plan := toPlanListResponse([]appbilling.BillingPlanView{item.Plan})[0]
		results = append(results, SubscriptionEntitlementResponse{
			SubscriptionResponse: subscription,
			Plan:                 plan,
			IsCurrent:            item.IsCurrent,
		})
	}
	return results
}

func toCheckoutResponse(item *domainbilling.PaymentOrder) CheckoutResponse {
	if item == nil {
		return CheckoutResponse{}
	}
	return CheckoutResponse{
		OrderNo:            item.OrderNo,
		OrderType:          item.OrderType,
		Provider:           item.Provider,
		Status:             item.Status,
		CheckoutURL:        item.CheckoutURL,
		ExternalCheckoutID: item.ExternalCheckoutID,
		BaseAmountCents:    item.BaseAmountCents,
		BaseCurrency:       item.BaseCurrency,
		PayAmountCents:     item.PayAmountCents,
		PayCurrency:        item.PayCurrency,
		FXRate:             item.FXRate,
		CreditNanousd:      item.CreditNanousd,
		CreditUSD:          nanousdToUSD(item.CreditNanousd),
		ExpiredAt:          item.ExpiredAt,
	}
}

func toBillingAccountResponse(item *domainbilling.BillingAccount) BillingAccountResponse {
	if item == nil {
		return BillingAccountResponse{}
	}
	return BillingAccountResponse{
		UserID:         item.UserID,
		Currency:       item.Currency,
		BalanceNanousd: item.BalanceNanousd,
		BalanceUSD:     nanousdToUSD(item.BalanceNanousd),
		Status:         item.Status,
		UpdatedAt:      item.UpdatedAt,
	}
}

func toBillingAccountViewResponse(item *appbilling.BillingAccountView) *BillingAccountResponse {
	if item == nil {
		return nil
	}
	return &BillingAccountResponse{
		UserID:         item.UserID,
		Currency:       item.Currency,
		BalanceNanousd: item.BalanceNanousd,
		BalanceUSD:     nanousdToUSD(item.BalanceNanousd),
		Status:         item.Status,
		UpdatedAt:      item.UpdatedAt,
	}
}

func toBillingOverviewResponse(item *appbilling.BillingOverview) BillingOverviewResponse {
	if item == nil {
		return BillingOverviewResponse{}
	}
	var plan *BillingPlanResponse
	if item.Plan != nil {
		planResponse := toPlanListResponse([]appbilling.BillingPlanView{*item.Plan})[0]
		plan = &planResponse
	}
	return BillingOverviewResponse{
		Mode:                     item.Mode,
		Plan:                     plan,
		PeriodStartAt:            item.PeriodStartAt,
		PeriodEndAt:              item.PeriodEndAt,
		PeriodCreditUSD:          nanousdToUSD(item.PeriodCreditNanousd),
		PeriodCreditNanousd:      item.PeriodCreditNanousd,
		PeriodUsedUSD:            nanousdToUSD(item.PeriodUsedNanousd),
		PeriodUsedNanousd:        item.PeriodUsedNanousd,
		PeriodRemainingUSD:       nanousdToUSD(item.PeriodRemainingNanousd),
		PeriodRemainingNanousd:   item.PeriodRemainingNanousd,
		Account:                  toBillingAccountViewResponse(item.Account),
		SubscriptionEntitlements: toSubscriptionEntitlementResponses(item.SubscriptionEntitlements),
	}
}

func toRedemptionCodeResponse(item appbilling.RedemptionCodeView) RedemptionCodeResponse {
	remaining := (*int)(nil)
	if item.MaxRedemptions != nil {
		value := *item.MaxRedemptions - item.RedeemedCount
		if value < 0 {
			value = 0
		}
		remaining = &value
	}
	return RedemptionCodeResponse{
		ID:                   item.ID,
		Code:                 item.Code,
		CodeHint:             item.CodeHint,
		Mode:                 item.Mode,
		RewardType:           item.RewardType,
		CreditUSD:            nanousdToUSD(item.CreditNanousd),
		CreditNanousd:        item.CreditNanousd,
		PlanID:               item.PlanID,
		DurationDays:         item.DurationDays,
		MaxRedemptions:       item.MaxRedemptions,
		PerUserLimit:         item.PerUserLimit,
		RedeemedCount:        item.RedeemedCount,
		RemainingRedemptions: remaining,
		Status:               item.Status,
		ExpiresAt:            item.ExpiresAt,
		Description:          item.Description,
		CreatedByUserID:      item.CreatedByUserID,
		CreatedAt:            item.CreatedAt,
		UpdatedAt:            item.UpdatedAt,
	}
}

func toRedemptionCodeResponses(items []appbilling.RedemptionCodeView) []RedemptionCodeResponse {
	results := make([]RedemptionCodeResponse, 0, len(items))
	for _, item := range items {
		results = append(results, toRedemptionCodeResponse(item))
	}
	return results
}

func toBatchDeleteRedemptionCodeResponse(data appbilling.BatchDeleteData) BatchDeleteRedemptionCodeDataResponse {
	results := make([]BatchDeleteRedemptionCodeResultResponse, 0, len(data.Results))
	for _, item := range data.Results {
		results = append(results, BatchDeleteRedemptionCodeResultResponse{
			ID:     item.ID,
			Status: item.Status,
			Error:  item.Error,
		})
	}
	return BatchDeleteRedemptionCodeDataResponse{
		Total:         data.Total,
		SuccessCount:  data.SuccessCount,
		NotFoundCount: data.NotFoundCount,
		FailedCount:   data.FailedCount,
		Results:       results,
	}
}

func toRedemptionResponse(item appbilling.RedemptionApplyView) RedemptionResponse {
	return RedemptionResponse{
		ID:                   item.Redemption.ID,
		CodeID:               item.Redemption.CodeID,
		UserID:               item.Redemption.UserID,
		Mode:                 item.Redemption.Mode,
		RewardType:           item.Redemption.RewardType,
		CreditUSD:            nanousdToUSD(item.Redemption.CreditNanousd),
		CreditNanousd:        item.Redemption.CreditNanousd,
		PlanID:               item.Redemption.PlanID,
		SubscriptionID:       item.Redemption.SubscriptionID,
		BalanceTransactionID: item.Redemption.BalanceTransactionID,
		CreatedAt:            item.Redemption.CreatedAt,
	}
}

func toUsageLedgerResponse(u domainbilling.UsageLedger) UsageLedgerResponse {
	snapshotIdentity := usagePricingSnapshotIdentity(u.PricingSnapshotJSON)
	return UsageLedgerResponse{
		ID:                  u.ID,
		UserID:              u.UserID,
		ConversationID:      u.ConversationID,
		ProviderProtocol:    u.ProviderProtocol,
		PlatformModelName:   u.PlatformModelName,
		RoutedBindingCode:   u.RoutedBindingCode,
		UpstreamModelName:   u.UpstreamModelName,
		ModelVendor:         snapshotIdentity.ModelVendor,
		ModelIcon:           snapshotIdentity.ModelIcon,
		IsFreeModel:         u.IsFreeModel,
		UsageDate:           u.UsageDate,
		InputTokens:         u.InputTokens,
		CacheReadTokens:     u.CacheReadTokens,
		CacheWriteTokens:    u.CacheWriteTokens,
		CacheWrite5mTokens:  u.CacheWrite5mTokens,
		CacheWrite1hTokens:  u.CacheWrite1hTokens,
		OutputTokens:        u.OutputTokens,
		ReasoningTokens:     u.ReasoningTokens,
		CallCount:           u.CallCount,
		DurationSeconds:     u.DurationSeconds,
		LatencyMS:           u.LatencyMS,
		UsageSpeed:          u.UsageSpeed,
		ServiceTier:         u.ServiceTier,
		BilledCurrency:      u.BilledCurrency,
		BilledNanousd:       u.BilledNanousd,
		BilledUSD:           nanousdToUSD(u.BilledNanousd),
		PricingSnapshotJSON: sanitizeUsagePricingSnapshotJSON(u.PricingSnapshotJSON),
		CreatedAt:           u.CreatedAt,
		UpdatedAt:           u.UpdatedAt,
	}
}

type usagePricingSnapshotIdentityResult struct {
	ModelVendor string
	ModelIcon   string
}

func usagePricingSnapshotIdentity(value string) usagePricingSnapshotIdentityResult {
	if strings.TrimSpace(value) == "" {
		return usagePricingSnapshotIdentityResult{}
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(value), &payload); err != nil {
		return usagePricingSnapshotIdentityResult{}
	}
	return usagePricingSnapshotIdentityResult{
		ModelVendor: stringSnapshotField(payload, "model_vendor"),
		ModelIcon:   stringSnapshotField(payload, "model_icon"),
	}
}

func stringSnapshotField(payload map[string]interface{}, key string) string {
	value, ok := payload[key]
	if !ok {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}

func sanitizeUsagePricingSnapshotJSON(value string) string {
	if strings.TrimSpace(value) == "" {
		return "{}"
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(value), &payload); err != nil {
		return "{}"
	}
	deleteUpstreamNameSnapshotFields(payload, "")
	result, err := json.Marshal(payload)
	if err != nil {
		return "{}"
	}
	return string(result)
}

func deleteUpstreamNameSnapshotFields(payload map[string]interface{}, parentKey string) {
	for key, value := range payload {
		if isUpstreamNameSnapshotField(key, parentKey) {
			delete(payload, key)
			continue
		}
		switch child := value.(type) {
		case map[string]interface{}:
			deleteUpstreamNameSnapshotFields(child, key)
		case []interface{}:
			for _, item := range child {
				if itemMap, ok := item.(map[string]interface{}); ok {
					deleteUpstreamNameSnapshotFields(itemMap, key)
				}
			}
		}
	}
}

func isUpstreamNameSnapshotField(key string, parentKey string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(key), "_", ""))
	if normalized == "upstreamname" {
		return true
	}
	return strings.ToLower(strings.TrimSpace(parentKey)) == "upstream" && (normalized == "name" || normalized == "displayname")
}

func toUsageMonthlyResponse(item domainbilling.UsageMonthlySummary) UsageMonthlyResponse {
	totalTokens := item.InputTokens + item.CacheReadTokens + item.CacheWriteTokens + item.OutputTokens + item.ReasoningTokens
	return UsageMonthlyResponse{
		MonthStartAt:     item.MonthStartAt,
		RecordCount:      item.RecordCount,
		InputTokens:      item.InputTokens,
		CacheReadTokens:  item.CacheReadTokens,
		CacheWriteTokens: item.CacheWriteTokens,
		OutputTokens:     item.OutputTokens,
		ReasoningTokens:  item.ReasoningTokens,
		TotalTokens:      totalTokens,
		CallCount:        item.CallCount,
		DurationSeconds:  item.DurationSeconds,
		AvgLatencyMS:     item.AvgLatencyMS,
		BilledNanousd:    item.BilledNanousd,
		BilledUSD:        nanousdToUSD(item.BilledNanousd),
	}
}

func toUsageDailyResponse(item domainbilling.UsageDailySummary) UsageDailyResponse {
	totalTokens := item.InputTokens + item.CacheReadTokens + item.CacheWriteTokens + item.OutputTokens + item.ReasoningTokens
	return UsageDailyResponse{
		UsageDate:        item.UsageDate,
		RecordCount:      item.RecordCount,
		InputTokens:      item.InputTokens,
		CacheReadTokens:  item.CacheReadTokens,
		CacheWriteTokens: item.CacheWriteTokens,
		OutputTokens:     item.OutputTokens,
		ReasoningTokens:  item.ReasoningTokens,
		TotalTokens:      totalTokens,
		CallCount:        item.CallCount,
		DurationSeconds:  item.DurationSeconds,
		AvgLatencyMS:     item.AvgLatencyMS,
		BilledNanousd:    item.BilledNanousd,
		BilledUSD:        nanousdToUSD(item.BilledNanousd),
		Models:           toUsageDailyModelResponses(item.Models),
	}
}

func toUsageDailyModelResponses(items []domainbilling.UsageDailyModelSummary) []UsageDailyModelResponse {
	results := make([]UsageDailyModelResponse, 0, len(items))
	for _, item := range items {
		totalTokens := item.InputTokens + item.CacheReadTokens + item.CacheWriteTokens + item.OutputTokens + item.ReasoningTokens
		results = append(results, UsageDailyModelResponse{
			PlatformModelName: item.PlatformModelName,
			RecordCount:       item.RecordCount,
			InputTokens:       item.InputTokens,
			CacheReadTokens:   item.CacheReadTokens,
			CacheWriteTokens:  item.CacheWriteTokens,
			OutputTokens:      item.OutputTokens,
			ReasoningTokens:   item.ReasoningTokens,
			TotalTokens:       totalTokens,
			CallCount:         item.CallCount,
			DurationSeconds:   item.DurationSeconds,
			AvgLatencyMS:      item.AvgLatencyMS,
			BilledNanousd:     item.BilledNanousd,
			BilledUSD:         nanousdToUSD(item.BilledNanousd),
		})
	}
	return results
}

func toModelPricingResponse(item appbilling.ModelPricingView) ModelPricingResponse {
	return ModelPricingResponse{
		ID:                          item.ID,
		PlatformModelName:           item.PlatformModelName,
		ModelVendor:                 item.ModelVendor,
		ModelIcon:                   item.ModelIcon,
		Currency:                    item.Currency,
		IsFree:                      item.IsFree,
		PricingMode:                 item.PricingMode,
		InputUSDPerMTokens:          nanousdToUSD(item.InputNanousdPerMTokens),
		CacheReadUSDPerMTokens:      nanousdToUSD(item.CacheReadNanousdPerMTokens),
		CacheWriteUSDPerMTokens:     nanousdToUSD(item.CacheWriteNanousdPerMTokens),
		OutputUSDPerMTokens:         nanousdToUSD(item.OutputNanousdPerMTokens),
		CallUSDPerCall:              nanousdToUSD(item.CallNanousdPerCall),
		DurationUSDPerSecond:        nanousdToUSD(item.DurationNanousdPerSecond),
		TieredPricingJSON:           item.TieredPricingJSON,
		InputNanousdPerMTokens:      item.InputNanousdPerMTokens,
		CacheReadNanousdPerMTokens:  item.CacheReadNanousdPerMTokens,
		CacheWriteNanousdPerMTokens: item.CacheWriteNanousdPerMTokens,
		OutputNanousdPerMTokens:     item.OutputNanousdPerMTokens,
		CallNanousdPerCall:          item.CallNanousdPerCall,
		DurationNanousdPerSecond:    item.DurationNanousdPerSecond,
		CreatedAt:                   item.CreatedAt,
		UpdatedAt:                   item.UpdatedAt,
	}
}

func modelPricingInputFromRequest(req UpsertModelPricingRequest) appbilling.ModelPricingInput {
	return appbilling.ModelPricingInput{
		PlatformModelName:           req.PlatformModelName,
		Currency:                    req.Currency,
		IsFree:                      req.IsFree,
		PricingMode:                 req.PricingMode,
		InputNanousdPerMTokens:      usdToNanousd(req.InputUSDPerMTokens),
		CacheReadNanousdPerMTokens:  usdToNanousd(req.CacheReadUSDPerMTokens),
		CacheWriteNanousdPerMTokens: usdToNanousd(req.CacheWriteUSDPerMTokens),
		OutputNanousdPerMTokens:     usdToNanousd(req.OutputUSDPerMTokens),
		CallNanousdPerCall:          usdToNanousd(req.CallUSDPerCall),
		DurationNanousdPerSecond:    usdToNanousd(req.DurationUSDPerSecond),
		TieredPricingJSON:           req.TieredPricingJSON,
	}
}

func planUpdateInputFromRequest(req UpdateBillingPlanRequest) appbilling.PlanUpdateInput {
	return appbilling.PlanUpdateInput{
		Name:                req.Name,
		Description:         req.Description,
		PeriodCreditNanousd: usdToNanousd(req.PeriodCreditUSD),
		DiscountPercent:     req.DiscountPercent,
		Currency:            req.Currency,
		AmountCents:         usdToCents(req.AmountUSD),
		BillingInterval:     req.BillingInterval,
	}
}

func usdToNanousd(value float64) int64 {
	if value <= 0 {
		return 0
	}
	return int64(math.Round(value * 1000000000))
}

func usdToCents(value float64) int64 {
	if value <= 0 {
		return 0
	}
	return int64(math.Round(value * 100))
}

func nanousdToUSD(value int64) float64 {
	if value <= 0 {
		return 0
	}
	return float64(value) / 1000000000
}
