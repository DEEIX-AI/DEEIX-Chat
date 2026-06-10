package billing

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	domainbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/billing"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/nativetool"
)

const (
	defaultPageSize            = 20
	maxPageSize                = 200
	publicModelPricingCacheTTL = 30 * time.Second
	nativeToolPricingSource    = "provider_official_defaults"
)

// UserSubscriptionSnapshot 描述用户当前订阅的派生结果。
type UserSubscriptionSnapshot struct {
	UserID    uint
	PlanID    *uint
	PlanName  string
	Tier      string
	Status    string
	ExpiresAt *time.Time
}

// UserBillingAccountSnapshot 描述用户按量余额的派生结果。
type UserBillingAccountSnapshot struct {
	UserID         uint
	Currency       string
	BalanceNanousd int64
	Status         string
}

// Service 封装计费业务能力。
type Service struct {
	repo repository.BillingRepository

	publicPricingMu               sync.RWMutex
	publicPricingByModel          map[string]PublicModelPricing
	publicPricingValidUntil       time.Time
	modelPricingInvalidator       func()
	platformModelIdentityResolver platformModelIdentityResolver
	modelPricingCatalog           modelPricingCatalogProvider
	nativeToolCatalog             nativeToolCatalogProvider
	auditWriter                   auditWriter
	redemptionCodeSecret          string
}

type platformModelIdentityResolver interface {
	ResolvePlatformModelIdentity(ctx context.Context, platformModelName string) (PlatformModelIdentity, error)
}

type modelPricingCatalogProvider interface {
	ListActivePlatformModelNames(ctx context.Context) (map[string]struct{}, error)
}

type nativeToolCatalogProvider interface {
	ListNativeToolDefinitions(ctx context.Context) ([]nativetool.Definition, error)
}

// UsagePricingInput 定义账单计算入参。
type UsagePricingInput struct {
	UserID              uint
	ConversationID      uint
	PlatformModelName   string
	RoutedBindingCode   string
	ProviderProtocol    string
	UpstreamName        string
	UpstreamModelName   string
	CacheTimeout        string
	RequestSpeed        string
	UsageSpeed          string
	RequestServiceTier  string
	UsageServiceTier    string
	ServiceOnly         bool
	InputTokens         int64
	CacheReadTokens     int64
	CacheWriteTokens    int64
	CacheWrite5mTokens  int64
	CacheWrite1hTokens  int64
	OutputTokens        int64
	ReasoningTokens     int64
	CallCount           int64
	DurationSeconds     int64
	LatencyMS           int64
	ServerSideToolUsage map[string]int64
	ServiceItems        []ServiceUsageInput
	RawUsageJSON        string
}

func upstreamUsageSnapshot(input UsagePricingInput) interface{} {
	raw := strings.TrimSpace(input.RawUsageJSON)
	if raw == "" {
		return map[string]interface{}{}
	}
	var decoded interface{}
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		return map[string]interface{}{}
	}
	switch value := decoded.(type) {
	case map[string]interface{}:
		return value
	case []interface{}:
		return value
	default:
		return map[string]interface{}{}
	}
}

// PlatformModelIdentity 描述一次计费需要用到的平台模型身份。
type PlatformModelIdentity struct {
	PlatformModelName string
	ModelVendor       string
	ModelIcon         string
}

// ServiceUsageInput 定义基础服务计费入参。
type ServiceUsageInput struct {
	ServiceCode        string
	ServiceName        string
	PlatformModelName  string
	UpstreamModelName  string
	ProviderProtocol   string
	CacheTimeout       string
	RequestSpeed       string
	UsageSpeed         string
	RequestServiceTier string
	UsageServiceTier   string
	InputTokens        int64
	CacheReadTokens    int64
	CacheWriteTokens   int64
	CacheWrite5mTokens int64
	CacheWrite1hTokens int64
	OutputTokens       int64
	ReasoningTokens    int64
	CallCount          int64
	DurationSeconds    int64
}

// NativeToolPricingView 描述内置原生工具默认计费价格。
type NativeToolPricingView struct {
	Provider     string
	ToolKey      string
	Label        string
	Description  string
	Type         string
	PriceNanousd int64
	Unit         string
	PriceLabel   string
	Billable     bool
}

// ModelPricingInput 定义模型单价保存入参。金额字段均为 nano USD。
type ModelPricingInput struct {
	PlatformModelName           string
	Currency                    string
	IsFree                      bool
	PricingMode                 string
	InputNanousdPerMTokens      int64
	CacheReadNanousdPerMTokens  int64
	CacheWriteNanousdPerMTokens int64
	OutputNanousdPerMTokens     int64
	CallNanousdPerCall          int64
	DurationNanousdPerSecond    int64
	TieredPricingJSON           string
}

// UsageListFilter 描述用户用量账本的筛选和排序条件。
type UsageListFilter struct {
	Query  string
	Status string
	Sort   string
}

// UsageLogListFilter 描述管理员调用日志筛选和排序条件。
type UsageLogListFilter struct {
	Query             string
	PlatformModelName string
	BillingMode       string
	UserID            uint
	CreatedFrom       *time.Time
	CreatedTo         *time.Time
	Sort              string
}

// PaymentOrderListFilter 描述管理员支付订单筛选和排序条件。
type PaymentOrderListFilter struct {
	Query       string
	OrderType   string
	Provider    string
	Status      string
	UserID      uint
	CreatedFrom *time.Time
	CreatedTo   *time.Time
	Sort        string
}

type tieredPricingConfig struct {
	Tiers []tieredPricingTier `json:"tiers"`
}

type tieredPricingTier struct {
	UpToTokens                  int64   `json:"upToTokens"`
	InputUSDPerMTokens          float64 `json:"inputUSDPerMTokens"`
	CacheReadUSDPerMTokens      float64 `json:"cacheReadUSDPerMTokens"`
	CacheWriteUSDPerMTokens     float64 `json:"cacheWriteUSDPerMTokens"`
	OutputUSDPerMTokens         float64 `json:"outputUSDPerMTokens"`
	inputNanousdPerMTokens      int64
	cacheReadNanousdPerMTokens  int64
	cacheWriteNanousdPerMTokens int64
	outputNanousdPerMTokens     int64
}

type billingRateMultiplier struct {
	Numerator   int64
	Denominator int64
}

type resolvedTieredPricingTier struct {
	tier       tieredPricingTier
	fromTokens int64
	upToTokens *int64
}

// PlanUpdateInput 定义周期套餐保存入参。
type PlanUpdateInput struct {
	Name                string
	Description         string
	PeriodCreditNanousd int64
	DiscountPercent     int
	Currency            string
	AmountCents         int64
	BillingInterval     string
}

// PaymentOrderInput 定义创建支付单入参。
type PaymentOrderInput struct {
	UserID       uint
	PriceID      uint
	Cycles       int
	Provider     string
	USDToCNYRate float64
}

// TopUpPaymentOrderInput 定义创建按量充值支付单入参。
type TopUpPaymentOrderInput struct {
	UserID       uint
	AmountCents  int64
	Provider     string
	USDToCNYRate float64
}

type paymentQuote struct {
	BaseCurrency    string
	BaseAmountCents int64
	PayCurrency     string
	PayAmountCents  int64
	FXRate          float64
}

// BillingAccountBalanceInput 定义管理员设置余额入参。
type BillingAccountBalanceInput struct {
	UserID      uint
	BalanceUSD  float64
	RefNo       string
	Description string
}

// NewService 创建服务。
func NewService(repo repository.BillingRepository) *Service {
	return &Service{repo: repo}
}

// SetModelPricingInvalidator 注入模型定价变更后的外部缓存失效回调。
func (s *Service) SetModelPricingInvalidator(invalidator func()) {
	if s == nil {
		return
	}
	s.modelPricingInvalidator = invalidator
}

// SetPlatformModelIdentityResolver 注入平台模型身份解析器。
func (s *Service) SetPlatformModelIdentityResolver(resolver platformModelIdentityResolver) {
	if s == nil {
		return
	}
	s.platformModelIdentityResolver = resolver
}

// SetModelPricingCatalogProvider 注入可定价模型目录，用于隐藏和拒绝孤立定价。
func (s *Service) SetModelPricingCatalogProvider(provider modelPricingCatalogProvider) {
	if s == nil {
		return
	}
	s.modelPricingCatalog = provider
}

// SetNativeToolCatalogProvider 注入平台级官方原生工具目录提供者。
func (s *Service) SetNativeToolCatalogProvider(provider nativeToolCatalogProvider) {
	if s == nil {
		return
	}
	s.nativeToolCatalog = provider
}

// SetRedemptionCodeSecret 注入兑换码 HMAC 与密文存储密钥。
func (s *Service) SetRedemptionCodeSecret(secret string) {
	if s == nil {
		return
	}
	s.redemptionCodeSecret = strings.TrimSpace(secret)
}

func (s *Service) invalidatePublicModelPricingCache() {
	if s == nil {
		return
	}
	s.publicPricingMu.Lock()
	s.publicPricingByModel = nil
	s.publicPricingValidUntil = time.Time{}
	s.publicPricingMu.Unlock()
	if s.modelPricingInvalidator != nil {
		s.modelPricingInvalidator()
	}
}

// GetBillingMode 查询当前计费模式。
func (s *Service) GetBillingMode(ctx context.Context) (string, error) {
	return s.repo.GetBillingMode(ctx)
}

// ListNativeToolPricing 返回应用管理员覆盖后的平台级原生工具计费价格目录。
func (s *Service) ListNativeToolPricing(ctx context.Context, rawPricingJSON string) ([]NativeToolPricingView, error) {
	definitions, err := s.nativeToolDefinitions(ctx)
	if err != nil {
		return nil, err
	}
	return nativeToolPricingViews(nativetool.PricingDefinitionsWithOverridesFromDefinitions(rawPricingJSON, definitions)), nil
}

// NormalizeNativeToolPricingJSON 基于当前平台级原生工具目录校验并格式化价格配置。
func (s *Service) NormalizeNativeToolPricingJSON(ctx context.Context, overrides map[string]nativetool.PricingOverride) (string, error) {
	definitions, err := s.nativeToolDefinitions(ctx)
	if err != nil {
		return "", err
	}
	return nativetool.PricingOverridesJSONForDefinitions(overrides, definitions)
}

func (s *Service) nativeToolDefinitions(ctx context.Context) ([]nativetool.Definition, error) {
	if s == nil || s.nativeToolCatalog == nil {
		return nativetool.Definitions(), nil
	}
	return s.nativeToolCatalog.ListNativeToolDefinitions(ctx)
}

func nativeToolPricingViews(items []nativetool.PricingDefinition) []NativeToolPricingView {
	results := make([]NativeToolPricingView, 0, len(items))
	for _, item := range items {
		results = append(results, NativeToolPricingView{
			Provider:     item.Provider,
			ToolKey:      item.ToolKey,
			Label:        item.Label,
			Description:  item.Description,
			Type:         item.Type,
			PriceNanousd: item.PriceNanousd,
			Unit:         item.Unit,
			PriceLabel:   item.PriceLabel,
			Billable:     item.Billable,
		})
	}
	return results
}

// ListBillingAccountSnapshots 批量查询用户按量余额。
func (s *Service) ListBillingAccountSnapshots(ctx context.Context, userIDs []uint) (map[uint]UserBillingAccountSnapshot, error) {
	results := make(map[uint]UserBillingAccountSnapshot, len(userIDs))
	for _, userID := range userIDs {
		if userID == 0 {
			continue
		}
		results[userID] = UserBillingAccountSnapshot{
			UserID:         userID,
			Currency:       "USD",
			BalanceNanousd: 0,
			Status:         "active",
		}
	}
	if len(results) == 0 {
		return results, nil
	}
	accounts, err := s.repo.ListBillingAccountsByUserIDs(ctx, userIDs)
	if err != nil {
		return nil, err
	}
	for _, account := range accounts {
		results[account.UserID] = UserBillingAccountSnapshot{
			UserID:         account.UserID,
			Currency:       firstNonEmpty(account.Currency, "USD"),
			BalanceNanousd: account.BalanceNanousd,
			Status:         firstNonEmpty(account.Status, "active"),
		}
	}
	return results, nil
}

// ListPlans 查询可用套餐及其价格。
func (s *Service) ListPlans(ctx context.Context) ([]BillingPlanView, error) {
	plans, err := s.repo.ListActivePlans(ctx)
	if err != nil {
		return nil, err
	}
	if len(plans) == 0 {
		return []BillingPlanView{}, nil
	}

	planIDs := make([]uint, 0, len(plans))
	for _, item := range plans {
		planIDs = append(planIDs, item.ID)
	}

	prices, err := s.repo.ListActivePricesByPlanIDs(ctx, planIDs)
	if err != nil {
		return nil, err
	}

	priceMap := make(map[uint][]BillingPriceView, len(plans))
	for _, item := range prices {
		priceMap[item.PlanID] = append(priceMap[item.PlanID], BillingPriceView{
			ID:              item.ID,
			PlanID:          item.PlanID,
			Code:            item.Code,
			BillingInterval: item.BillingInterval,
			Currency:        item.Currency,
			AmountCents:     item.AmountCents,
			IsDefault:       item.IsDefault,
		})
	}

	results := make([]BillingPlanView, 0, len(plans))
	for _, item := range plans {
		results = append(results, BillingPlanView{
			ID:                  item.ID,
			Code:                item.Code,
			Name:                item.Name,
			Description:         item.Description,
			FeatureJSON:         item.FeatureJSON,
			PeriodCreditNanousd: item.PeriodCreditNanousd,
			DiscountPercent:     item.DiscountPercent,
			SortOrder:           item.SortOrder,
			IsActive:            item.IsActive,
			Prices:              priceMap[item.ID],
		})
	}

	return results, nil
}

// GetCurrentSubscriptionSnapshot 查询单个用户当前订阅派生状态。
func (s *Service) GetCurrentSubscriptionSnapshot(
	ctx context.Context,
	userID uint,
	now time.Time,
) (*UserSubscriptionSnapshot, error) {
	results, err := s.ListCurrentSubscriptionSnapshots(ctx, []uint{userID}, now)
	if err != nil {
		return nil, err
	}

	item, ok := results[userID]
	if !ok {
		return nil, nil
	}
	return &item, nil
}

// ListCurrentSubscriptionSnapshots 批量查询用户当前订阅派生状态。
func (s *Service) ListCurrentSubscriptionSnapshots(
	ctx context.Context,
	userIDs []uint,
	now time.Time,
) (map[uint]UserSubscriptionSnapshot, error) {
	results := make(map[uint]UserSubscriptionSnapshot)
	if len(userIDs) == 0 {
		return results, nil
	}

	subscriptions, planMap, err := s.listSubscriptionEntitlements(ctx, userIDs, now)
	if err != nil {
		return nil, err
	}
	if len(subscriptions) == 0 {
		return results, nil
	}

	subscriptionsByUserID := make(map[uint][]domainbilling.Subscription, len(userIDs))
	for _, item := range subscriptions {
		subscriptionsByUserID[item.UserID] = append(subscriptionsByUserID[item.UserID], item)
	}

	for userID, items := range subscriptionsByUserID {
		subscription, ok := selectCurrentSubscription(items, planMap, now)
		if !ok {
			continue
		}
		planID := subscription.PlanID
		planCode := ""
		planName := ""
		if plan, ok := planMap[planID]; ok {
			planCode = strings.TrimSpace(plan.Code)
			planName = strings.TrimSpace(plan.Name)
		}

		status := strings.TrimSpace(subscription.Status)
		expiresAt := contiguousSubscriptionEnd(subscription, items)
		if planCode == "free" {
			status = "free"
			expiresAt = nil
		}

		results[userID] = UserSubscriptionSnapshot{
			UserID:    userID,
			PlanID:    &planID,
			PlanName:  firstNonEmpty(planName, strings.ToUpper(planCode)),
			Tier:      firstNonEmpty(planCode, "free"),
			Status:    firstNonEmpty(status, "free"),
			ExpiresAt: expiresAt,
		}
	}

	return results, nil
}

// Subscribe 创建用户订阅。
func (s *Service) Subscribe(ctx context.Context, userID uint, priceID uint, cycles int) (*domainbilling.Subscription, error) {
	if cycles <= 0 {
		cycles = 1
	}

	price, err := s.repo.GetPriceByID(ctx, priceID)
	if err != nil {
		return nil, err
	}
	plan, err := s.repo.GetPlanByID(ctx, price.PlanID)
	if err != nil {
		return nil, err
	}
	if !plan.IsActive || !price.IsActive {
		return nil, repository.ErrNotFound
	}
	now := time.Now()
	if strings.TrimSpace(plan.Code) == "free" {
		subscriptions, planMap, entitlementErr := s.listSubscriptionEntitlements(ctx, []uint{userID}, now)
		if entitlementErr != nil {
			return nil, entitlementErr
		}
		for _, subscription := range subscriptions {
			subscriptionPlan, ok := planMap[subscription.PlanID]
			if !ok || strings.TrimSpace(subscriptionPlan.Code) == "free" {
				continue
			}
			return nil, ErrSubscriptionEntitlementActive
		}
	}
	if price.AmountCents > 0 {
		return nil, ErrPaymentRequired
	}

	endAt := resolvePeriodEnd(now, price.BillingInterval, cycles)
	item := &domainbilling.Subscription{
		UserID:               userID,
		PlanID:               plan.ID,
		PriceID:              price.ID,
		Status:               "active",
		StartAt:              now,
		CurrentPeriodStartAt: now,
		CurrentPeriodEndAt:   endAt,
		CancelAtPeriodEnd:    false,
		CanceledAt:           nil,
		AutoRenew:            price.BillingInterval != domainbilling.IntervalLifetime,
	}
	if err := s.repo.ReplaceSubscription(ctx, item); err != nil {
		return nil, err
	}
	return item, nil
}

// SetUserSubscriptionByPlanCode 由后台管理员直接设置用户当前订阅。
func (s *Service) SetUserSubscriptionByPlanCode(
	ctx context.Context,
	userID uint,
	planCode string,
	expiresAt *time.Time,
) (*UserSubscriptionSnapshot, error) {
	if userID == 0 {
		return nil, repository.ErrInvalidInput
	}

	mode, err := s.repo.GetBillingMode(ctx)
	if err != nil {
		return nil, err
	}
	if mode != "period" {
		return nil, ErrPaymentRequired
	}

	normalizedCode := strings.ToLower(strings.TrimSpace(planCode))
	if normalizedCode == "" {
		normalizedCode = "free"
	}
	plan, err := s.repo.GetActivePlanByCode(ctx, normalizedCode)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrInvalidSubscriptionTier
		}
		return nil, err
	}

	prices, err := s.repo.ListActivePricesByPlanIDs(ctx, []uint{plan.ID})
	if err != nil {
		return nil, err
	}
	var defaultPrice *domainbilling.Price
	for index := range prices {
		if prices[index].IsDefault {
			defaultPrice = &prices[index]
			break
		}
	}
	if defaultPrice == nil {
		return nil, ErrInvalidSubscriptionTier
	}

	now := time.Now()
	var periodEndAt *time.Time
	autoRenew := false
	if plan.Code != "free" {
		if expiresAt == nil {
			return nil, ErrSubscriptionExpiryRequired
		}
		normalizedExpiresAt := expiresAt.UTC()
		if !normalizedExpiresAt.After(now.UTC()) {
			return nil, ErrInvalidSubscriptionExpiry
		}
		periodEndAt = &normalizedExpiresAt
	} else if defaultPrice.BillingInterval != domainbilling.IntervalLifetime {
		autoRenew = true
	}

	item := &domainbilling.Subscription{
		UserID:               userID,
		PlanID:               plan.ID,
		PriceID:              defaultPrice.ID,
		Status:               "active",
		StartAt:              now,
		CurrentPeriodStartAt: now,
		CurrentPeriodEndAt:   periodEndAt,
		CancelAtPeriodEnd:    false,
		CanceledAt:           nil,
		AutoRenew:            autoRenew,
	}
	if err = s.repo.ReplaceSubscription(ctx, item); err != nil {
		return nil, err
	}

	return s.GetCurrentSubscriptionSnapshot(ctx, userID, now)
}

// CreatePaymentOrder 创建待支付订单。
func (s *Service) CreatePaymentOrder(ctx context.Context, input PaymentOrderInput) (*domainbilling.PaymentOrder, *domainbilling.Plan, *domainbilling.Price, error) {
	mode, err := s.repo.GetBillingMode(ctx)
	if err != nil {
		return nil, nil, nil, err
	}
	if mode != "period" {
		return nil, nil, nil, ErrPaymentRequired
	}

	provider := strings.TrimSpace(input.Provider)
	switch provider {
	case domainbilling.PaymentProviderStripe, domainbilling.PaymentProviderEPay:
	default:
		return nil, nil, nil, ErrPaymentProviderUnavailable
	}
	cycles := input.Cycles
	if cycles <= 0 {
		cycles = 1
	}

	price, err := s.repo.GetPriceByID(ctx, input.PriceID)
	if err != nil {
		return nil, nil, nil, err
	}
	plan, err := s.repo.GetPlanByID(ctx, price.PlanID)
	if err != nil {
		return nil, nil, nil, err
	}
	if input.UserID == 0 || !plan.IsActive || !price.IsActive {
		return nil, nil, nil, repository.ErrInvalidInput
	}
	if price.AmountCents <= 0 {
		return nil, nil, nil, repository.ErrInvalidInput
	}
	baseCurrency := normalizeCurrency(price.Currency)
	baseAmountCents := price.AmountCents * int64(cycles)
	if baseAmountCents <= 0 {
		return nil, nil, nil, repository.ErrInvalidInput
	}
	quote := resolvePaymentQuote(provider, baseCurrency, baseAmountCents, input.USDToCNYRate)
	if quote.PayAmountCents <= 0 {
		return nil, nil, nil, repository.ErrInvalidInput
	}

	orderNo, err := generateOrderNo()
	if err != nil {
		return nil, nil, nil, err
	}
	now := time.Now()
	expiredAt := now.Add(30 * time.Minute)
	snapshot := map[string]interface{}{
		"plan_id":           plan.ID,
		"plan_code":         plan.Code,
		"plan_name":         plan.Name,
		"price_id":          price.ID,
		"price_code":        price.Code,
		"billing_interval":  price.BillingInterval,
		"cycles":            cycles,
		"base_currency":     quote.BaseCurrency,
		"base_amount_cents": quote.BaseAmountCents,
		"pay_currency":      quote.PayCurrency,
		"pay_amount_cents":  quote.PayAmountCents,
		"fx_rate":           formatFXRate(quote.FXRate),
		"provider":          provider,
	}
	snapshotJSON := "{}"
	if raw, marshalErr := json.Marshal(snapshot); marshalErr == nil {
		snapshotJSON = string(raw)
	}
	order, err := s.repo.CreatePaymentOrder(ctx, &domainbilling.PaymentOrder{
		OrderNo:         orderNo,
		OrderType:       domainbilling.PaymentOrderTypeSubscription,
		UserID:          input.UserID,
		PlanID:          plan.ID,
		PriceID:         price.ID,
		Provider:        provider,
		Status:          domainbilling.PaymentStatusPending,
		BaseCurrency:    quote.BaseCurrency,
		BaseAmountCents: quote.BaseAmountCents,
		PayCurrency:     quote.PayCurrency,
		PayAmountCents:  quote.PayAmountCents,
		FXRate:          formatFXRate(quote.FXRate),
		BillingInterval: price.BillingInterval,
		Cycles:          cycles,
		ExpiredAt:       &expiredAt,
		SnapshotJSON:    snapshotJSON,
	})
	if err != nil {
		return nil, nil, nil, err
	}
	return order, plan, price, nil
}

// CreateTopUpPaymentOrder 创建按量余额充值支付单。
func (s *Service) CreateTopUpPaymentOrder(ctx context.Context, input TopUpPaymentOrderInput) (*domainbilling.PaymentOrder, error) {
	mode, err := s.repo.GetBillingMode(ctx)
	if err != nil {
		return nil, err
	}
	if mode != "usage" {
		return nil, ErrPaymentRequired
	}
	provider := strings.TrimSpace(input.Provider)
	switch provider {
	case domainbilling.PaymentProviderStripe, domainbilling.PaymentProviderEPay:
	default:
		return nil, ErrPaymentProviderUnavailable
	}
	if input.UserID == 0 || input.AmountCents <= 0 {
		return nil, repository.ErrInvalidInput
	}

	baseCurrency := "USD"
	baseAmountCents := input.AmountCents
	quote := resolvePaymentQuote(provider, baseCurrency, baseAmountCents, input.USDToCNYRate)
	creditNanousd := centsToNanousd(baseAmountCents)
	if quote.PayAmountCents <= 0 || creditNanousd <= 0 {
		return nil, repository.ErrInvalidInput
	}

	orderNo, err := generateOrderNo()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	expiredAt := now.Add(30 * time.Minute)
	snapshot := map[string]interface{}{
		"order_type":        domainbilling.PaymentOrderTypeTopUp,
		"base_currency":     quote.BaseCurrency,
		"base_amount_cents": quote.BaseAmountCents,
		"pay_currency":      quote.PayCurrency,
		"pay_amount_cents":  quote.PayAmountCents,
		"fx_rate":           formatFXRate(quote.FXRate),
		"credit_nanousd":    creditNanousd,
		"provider":          provider,
	}
	snapshotJSON := "{}"
	if raw, marshalErr := json.Marshal(snapshot); marshalErr == nil {
		snapshotJSON = string(raw)
	}
	return s.repo.CreatePaymentOrder(ctx, &domainbilling.PaymentOrder{
		OrderNo:         orderNo,
		OrderType:       domainbilling.PaymentOrderTypeTopUp,
		UserID:          input.UserID,
		Provider:        provider,
		Status:          domainbilling.PaymentStatusPending,
		BaseCurrency:    quote.BaseCurrency,
		BaseAmountCents: quote.BaseAmountCents,
		PayCurrency:     quote.PayCurrency,
		PayAmountCents:  quote.PayAmountCents,
		FXRate:          formatFXRate(quote.FXRate),
		CreditNanousd:   creditNanousd,
		BillingInterval: domainbilling.IntervalLifetime,
		Cycles:          1,
		ExpiredAt:       &expiredAt,
		SnapshotJSON:    snapshotJSON,
	})
}

// AttachPaymentCheckout 保存外部收银台信息。
func (s *Service) AttachPaymentCheckout(ctx context.Context, orderNo string, externalCheckoutID string, checkoutURL string) error {
	return s.repo.UpdatePaymentOrderCheckout(ctx, orderNo, externalCheckoutID, checkoutURL)
}

// GetPaymentOrder 查询支付单。
func (s *Service) GetPaymentOrder(ctx context.Context, orderNo string) (*domainbilling.PaymentOrder, error) {
	return s.repo.GetPaymentOrderByOrderNo(ctx, orderNo)
}

// CompletePaymentOrder 支付成功后开通订阅。
func (s *Service) CompletePaymentOrder(ctx context.Context, orderNo string, externalPaymentID string, paidAt time.Time) (*domainbilling.PaymentOrder, bool, error) {
	order, err := s.repo.GetPaymentOrderByOrderNo(ctx, orderNo)
	if err != nil {
		return nil, false, err
	}
	if order.Status == domainbilling.PaymentStatusPaid {
		return order, false, nil
	}
	if paidAt.IsZero() {
		paidAt = time.Now()
	}
	if order.OrderType == domainbilling.PaymentOrderTypeTopUp {
		return s.repo.MarkPaymentOrderPaidAndCreditBalance(ctx, orderNo, externalPaymentID, paidAt)
	}
	endAt := resolvePeriodEnd(paidAt, order.BillingInterval, order.Cycles)
	subscription := &domainbilling.Subscription{
		UserID:               order.UserID,
		PlanID:               order.PlanID,
		PriceID:              order.PriceID,
		Status:               "active",
		StartAt:              paidAt,
		CurrentPeriodStartAt: paidAt,
		CurrentPeriodEndAt:   endAt,
		CancelAtPeriodEnd:    false,
		CanceledAt:           nil,
		AutoRenew:            order.BillingInterval != domainbilling.IntervalLifetime,
	}
	return s.repo.MarkPaymentOrderPaidAndGrantSubscription(ctx, orderNo, externalPaymentID, paidAt, subscription)
}

// UpdatePlan 保存周期套餐与默认价格。
func (s *Service) UpdatePlan(ctx context.Context, planID uint, input PlanUpdateInput) (*BillingPlanView, error) {
	if planID == 0 {
		return nil, repository.ErrInvalidInput
	}
	current, err := s.repo.GetPlanByID(ctx, planID)
	if err != nil {
		return nil, err
	}

	plan := &domainbilling.Plan{
		ID:                  current.ID,
		Code:                current.Code,
		Name:                firstNonEmpty(input.Name, current.Name),
		Description:         strings.TrimSpace(input.Description),
		FeatureJSON:         current.FeatureJSON,
		PeriodCreditNanousd: clampNonNegative(input.PeriodCreditNanousd),
		DiscountPercent:     clampPercent(input.DiscountPercent),
		SortOrder:           current.SortOrder,
		IsActive:            true,
	}
	price := &domainbilling.Price{
		PlanID:          current.ID,
		Code:            current.Code + "-default",
		BillingInterval: normalizeInterval(input.BillingInterval),
		Currency:        "USD",
		AmountCents:     clampNonNegative(input.AmountCents),
		IsActive:        true,
		IsDefault:       true,
	}
	if err := s.repo.UpdatePlanWithDefaultPrice(ctx, plan, price); err != nil {
		return nil, err
	}
	return &BillingPlanView{
		ID:                  plan.ID,
		Code:                plan.Code,
		Name:                plan.Name,
		Description:         plan.Description,
		FeatureJSON:         plan.FeatureJSON,
		PeriodCreditNanousd: plan.PeriodCreditNanousd,
		DiscountPercent:     plan.DiscountPercent,
		SortOrder:           plan.SortOrder,
		IsActive:            plan.IsActive,
		Prices: []BillingPriceView{
			{
				PlanID:          plan.ID,
				Code:            price.Code,
				BillingInterval: price.BillingInterval,
				Currency:        price.Currency,
				AmountCents:     price.AmountCents,
				IsDefault:       true,
			},
		},
	}, nil
}

// RecordUsage 记录用量。
func (s *Service) RecordUsage(ctx context.Context, usage *domainbilling.UsageLedger) error {
	return s.RecordUsageWithReservation(ctx, usage, nil)
}

// RecordUsageWithReservation 记录用量，并在按量模式下结算预扣差额。
func (s *Service) RecordUsageWithReservation(ctx context.Context, usage *domainbilling.UsageLedger, reservation *domainbilling.UsageBalanceReservation) error {
	mode, err := s.repo.GetBillingMode(ctx)
	if err != nil {
		return err
	}
	if mode == "usage" || reservation != nil {
		if err := s.repo.AddUsageAndSettleBalance(ctx, usage, reservation); err != nil {
			if errors.Is(err, repository.ErrInsufficientBalance) {
				return ErrUsageBalanceInsufficient
			}
			return err
		}
		return nil
	}
	return s.repo.AddUsage(ctx, usage)
}

// ReserveUsageBalance 在按量模式下按配置金额预扣余额；非按量或免费模型不预扣。
func (s *Service) ReserveUsageBalance(ctx context.Context, userID uint, platformModelName string, refNo string) (*domainbilling.UsageBalanceReservation, error) {
	mode, err := s.repo.GetBillingMode(ctx)
	if err != nil {
		return nil, err
	}
	if mode != "usage" {
		return nil, nil
	}
	pricing, err := s.getResolvedModelPricing(ctx, platformModelName)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}
	if pricing == nil {
		return nil, ErrModelPricingRequired
	}
	if pricing.IsFree {
		return nil, nil
	}
	prepaidNanousd, err := s.repo.GetBillingPrepaidAmountNanousd(ctx)
	if err != nil {
		return nil, err
	}
	if prepaidNanousd <= 0 {
		return nil, nil
	}
	reservation, err := s.repo.ReserveUsageBalance(ctx, userID, prepaidNanousd, refNo)
	if err != nil {
		if errors.Is(err, repository.ErrInsufficientBalance) {
			return nil, ErrUsageBalanceInsufficient
		}
		return nil, err
	}
	return reservation, nil
}

// ReleaseUsageBalanceReservation 在调用失败时退回已预扣余额。
func (s *Service) ReleaseUsageBalanceReservation(ctx context.Context, reservation *domainbilling.UsageBalanceReservation, description string) error {
	if reservation == nil || reservation.AmountNanousd <= 0 {
		return nil
	}
	return s.repo.ReleaseUsageBalanceReservation(ctx, reservation.UserID, reservation.RefNo, description)
}

// EnsureModelUsable 按当前计费方式校验用户是否还能使用指定模型。
func (s *Service) EnsureModelUsable(ctx context.Context, userID uint, platformModelName string, now time.Time) error {
	mode, err := s.repo.GetBillingMode(ctx)
	if err != nil {
		return err
	}
	if mode == "self" {
		return nil
	}

	pricing, err := s.getResolvedModelPricing(ctx, platformModelName)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return err
	}
	if pricing != nil && pricing.IsFree {
		return nil
	}
	if pricing == nil {
		return ErrModelPricingRequired
	}
	if mode == "usage" {
		account, accountErr := s.repo.GetOrCreateBillingAccount(ctx, userID)
		if accountErr != nil {
			return accountErr
		}
		prepaidNanousd, prepaidErr := s.repo.GetBillingPrepaidAmountNanousd(ctx)
		if prepaidErr != nil {
			return prepaidErr
		}
		requiredBalance := int64(1)
		if prepaidNanousd > requiredBalance {
			requiredBalance = prepaidNanousd
		}
		if account.BalanceNanousd < requiredBalance {
			return ErrUsageBalanceInsufficient
		}
		return nil
	}
	if mode != "period" {
		return nil
	}

	plan, startAt, endAt, err := s.currentPeriodPlan(ctx, userID, now)
	if err != nil {
		return err
	}
	if plan.PeriodCreditNanousd <= 0 {
		return ErrPeriodCreditExceeded
	}
	usedNanousd, err := s.repo.SumBillableNanousd(ctx, userID, startAt, endAt)
	if err != nil {
		return err
	}
	if usedNanousd >= plan.PeriodCreditNanousd {
		return ErrPeriodCreditExceeded
	}
	return nil
}

func (s *Service) currentPeriodPlan(
	ctx context.Context,
	userID uint,
	now time.Time,
) (domainbilling.Plan, time.Time, time.Time, error) {
	monthStart, monthEnd := monthBounds(now)
	subscriptions, planMap, err := s.listSubscriptionEntitlements(ctx, []uint{userID}, now)
	if err != nil {
		return domainbilling.Plan{}, time.Time{}, time.Time{}, err
	}
	subscription, ok := selectCurrentSubscription(subscriptions, planMap, now)
	if !ok {
		plan, planErr := s.repo.GetActivePlanByCode(ctx, "free")
		if planErr != nil {
			return domainbilling.Plan{}, time.Time{}, time.Time{}, planErr
		}
		return *plan, monthStart, monthEnd, nil
	}
	plan, ok := planMap[subscription.PlanID]
	if !ok {
		return domainbilling.Plan{}, time.Time{}, time.Time{}, repository.ErrNotFound
	}
	return plan, monthStart, monthEnd, nil
}

func (s *Service) listSubscriptionEntitlements(
	ctx context.Context,
	userIDs []uint,
	now time.Time,
) ([]domainbilling.Subscription, map[uint]domainbilling.Plan, error) {
	subscriptions, err := s.repo.ListSubscriptionEntitlementsByUserIDs(ctx, userIDs, now)
	if err != nil {
		return nil, nil, err
	}
	planIDs := make([]uint, 0, len(subscriptions))
	seenPlanIDs := make(map[uint]struct{}, len(subscriptions))
	for _, item := range subscriptions {
		if item.PlanID == 0 {
			continue
		}
		if _, exists := seenPlanIDs[item.PlanID]; exists {
			continue
		}
		seenPlanIDs[item.PlanID] = struct{}{}
		planIDs = append(planIDs, item.PlanID)
	}
	plans, err := s.repo.ListPlansByIDs(ctx, planIDs)
	if err != nil {
		return nil, nil, err
	}
	planMap := make(map[uint]domainbilling.Plan, len(plans))
	for _, item := range plans {
		planMap[item.ID] = item
	}
	return subscriptions, planMap, nil
}

func selectCurrentSubscription(
	subscriptions []domainbilling.Subscription,
	plans map[uint]domainbilling.Plan,
	now time.Time,
) (domainbilling.Subscription, bool) {
	var result domainbilling.Subscription
	found := false
	for _, item := range subscriptions {
		if item.Status != "active" || item.CurrentPeriodStartAt.After(now) {
			continue
		}
		if item.CurrentPeriodEndAt != nil && !item.CurrentPeriodEndAt.After(now) {
			continue
		}
		if !found || isHigherPrioritySubscription(item, result, plans) {
			result = item
			found = true
		}
	}
	return result, found
}

func contiguousSubscriptionEnd(
	current domainbilling.Subscription,
	subscriptions []domainbilling.Subscription,
) *time.Time {
	if current.CurrentPeriodEndAt == nil {
		return nil
	}
	endAt := *current.CurrentPeriodEndAt
	for {
		extended := false
		for _, item := range subscriptions {
			if item.ID == current.ID || item.PlanID != current.PlanID || item.CurrentPeriodEndAt == nil {
				continue
			}
			if item.CurrentPeriodStartAt.After(endAt) || !item.CurrentPeriodEndAt.After(endAt) {
				continue
			}
			endAt = *item.CurrentPeriodEndAt
			extended = true
		}
		if !extended {
			break
		}
	}
	return &endAt
}

func buildSubscriptionEntitlementViews(
	subscriptions []domainbilling.Subscription,
	plans map[uint]domainbilling.Plan,
	now time.Time,
) []SubscriptionEntitlementView {
	current, hasCurrent := selectCurrentSubscription(subscriptions, plans, now)
	items := make([]domainbilling.Subscription, 0, len(subscriptions))
	for _, item := range subscriptions {
		plan, ok := plans[item.PlanID]
		if !ok || strings.TrimSpace(plan.Code) == "free" || item.Status != "active" {
			continue
		}
		if item.CurrentPeriodEndAt != nil && !item.CurrentPeriodEndAt.After(now) {
			continue
		}
		items = append(items, item)
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].CurrentPeriodStartAt.Equal(items[j].CurrentPeriodStartAt) {
			leftRank := subscriptionPlanRank(plans[items[i].PlanID])
			rightRank := subscriptionPlanRank(plans[items[j].PlanID])
			if leftRank != rightRank {
				return leftRank > rightRank
			}
			return items[i].ID < items[j].ID
		}
		return items[i].CurrentPeriodStartAt.Before(items[j].CurrentPeriodStartAt)
	})

	results := make([]SubscriptionEntitlementView, 0, len(items))
	for _, item := range items {
		plan := plans[item.PlanID]
		view := SubscriptionEntitlementView{
			Subscription: item,
			Plan:         toBillingPlanView(plan),
			IsCurrent:    hasCurrent && item.ID == current.ID,
		}
		lastIndex := len(results) - 1
		if lastIndex >= 0 && canMergeSubscriptionEntitlement(results[lastIndex].Subscription, view.Subscription) {
			last := &results[lastIndex]
			if subscriptionEndsAfter(view.Subscription, last.Subscription) {
				last.Subscription.CurrentPeriodEndAt = view.Subscription.CurrentPeriodEndAt
			}
			last.IsCurrent = last.IsCurrent || view.IsCurrent
			continue
		}
		results = append(results, view)
	}
	return results
}

func canMergeSubscriptionEntitlement(left domainbilling.Subscription, right domainbilling.Subscription) bool {
	if left.PlanID != right.PlanID || left.PriceID != right.PriceID || left.CurrentPeriodEndAt == nil {
		return false
	}
	return !right.CurrentPeriodStartAt.After(*left.CurrentPeriodEndAt)
}

func subscriptionEndsAfter(left domainbilling.Subscription, right domainbilling.Subscription) bool {
	if left.CurrentPeriodEndAt == nil {
		return true
	}
	if right.CurrentPeriodEndAt == nil {
		return false
	}
	return left.CurrentPeriodEndAt.After(*right.CurrentPeriodEndAt)
}

func toBillingPlanView(plan domainbilling.Plan) BillingPlanView {
	return BillingPlanView{
		ID:                  plan.ID,
		Code:                plan.Code,
		Name:                plan.Name,
		Description:         plan.Description,
		FeatureJSON:         plan.FeatureJSON,
		PeriodCreditNanousd: plan.PeriodCreditNanousd,
		DiscountPercent:     plan.DiscountPercent,
		SortOrder:           plan.SortOrder,
		IsActive:            plan.IsActive,
	}
}

func isHigherPrioritySubscription(candidate domainbilling.Subscription, current domainbilling.Subscription, plans map[uint]domainbilling.Plan) bool {
	candidateRank := subscriptionPlanRank(plans[candidate.PlanID])
	currentRank := subscriptionPlanRank(plans[current.PlanID])
	if candidateRank != currentRank {
		return candidateRank > currentRank
	}
	candidateEnd := subscriptionEndSortTime(candidate)
	currentEnd := subscriptionEndSortTime(current)
	if !candidateEnd.Equal(currentEnd) {
		return candidateEnd.After(currentEnd)
	}
	return candidate.ID > current.ID
}

func subscriptionPlanRank(plan domainbilling.Plan) int {
	if isFreePlanCode(plan.Code) {
		return 0
	}
	if plan.SortOrder > 0 {
		return plan.SortOrder
	}
	return int(plan.ID)
}

func isFreePlanCode(code string) bool {
	return strings.TrimSpace(code) == "free"
}

func subscriptionEndSortTime(subscription domainbilling.Subscription) time.Time {
	if subscription.CurrentPeriodEndAt == nil {
		return time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC)
	}
	return *subscription.CurrentPeriodEndAt
}

// BuildUsageLedger 根据模型单价与用量构建账本记录。
func (s *Service) BuildUsageLedger(ctx context.Context, input UsagePricingInput) (*domainbilling.UsageLedger, error) {
	platformModelName := strings.TrimSpace(input.PlatformModelName)
	providerProtocol := strings.TrimSpace(input.ProviderProtocol)
	usageSpeed := normalizeUsageSpeed(input.UsageSpeed)
	requestSpeed := normalizeUsageSpeed(input.RequestSpeed)
	billingSpeed := resolveBillingSpeed(providerProtocol, usageSpeed, requestSpeed)
	requestServiceTier := normalizeOpenAIServiceTier(input.RequestServiceTier)
	usageServiceTier := normalizeOpenAIServiceTier(input.UsageServiceTier)
	billingServiceTier := resolveBillingServiceTier(providerProtocol, usageServiceTier)
	fastMode := isAnthropicFastMode(providerProtocol, usageSpeed, requestSpeed)
	rateMultiplier := resolveUsageRateMultiplier(providerProtocol, platformModelName, input.UpstreamModelName, fastMode, billingServiceTier)
	cacheWriteTokens, cacheWrite5mTokens, cacheWrite1hTokens := normalizeCacheWriteTokenBreakdown(
		input.CacheWriteTokens,
		input.CacheWrite5mTokens,
		input.CacheWrite1hTokens,
		providerProtocol,
		input.CacheTimeout,
	)
	mode, err := s.repo.GetBillingMode(ctx)
	if err != nil {
		return nil, err
	}
	identity, err := s.resolvePlatformModelIdentity(ctx, platformModelName)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}
	pricing, err := s.repo.GetModelPricing(ctx, platformModelName)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}

	currency := "USD"
	var inputNanousdPerMTokens int64
	var cacheReadNanousdPerMTokens int64
	var cacheWriteNanousdPerMTokens int64
	var outputNanousdPerMTokens int64
	var callNanousdPerCall int64
	var durationNanousdPerSecond int64
	var baseInputNanousdPerMTokens int64
	var baseCacheReadNanousdPerMTokens int64
	var baseCacheWriteNanousdPerMTokens int64
	var baseCacheWrite5mNanousdPerMTokens int64
	var baseCacheWrite1hNanousdPerMTokens int64
	var baseOutputNanousdPerMTokens int64
	var baseCallNanousdPerCall int64
	var baseDurationNanousdPerSecond int64
	var cacheWrite5mNanousdPerMTokens int64
	var cacheWrite1hNanousdPerMTokens int64
	var tieredPricingJSON string
	var tieredTiers []tieredPricingTier
	pricingMode := domainbilling.PricingModeToken
	isFreeModel := pricing != nil && pricing.IsFree
	if pricing != nil {
		currency = pricing.Currency
		pricingMode = normalizePricingMode(pricing.PricingMode)
		tieredPricingJSON = strings.TrimSpace(pricing.TieredPricingJSON)
	}
	if !input.ServiceOnly && mode != "self" && pricing != nil && !pricing.IsFree {
		switch pricingMode {
		case domainbilling.PricingModeCall:
			baseCallNanousdPerCall = pricing.CallNanousdPerCall
			callNanousdPerCall = applyRateMultiplier(baseCallNanousdPerCall, rateMultiplier)
		case domainbilling.PricingModeDuration:
			baseDurationNanousdPerSecond = pricing.DurationNanousdPerSecond
			durationNanousdPerSecond = applyRateMultiplier(baseDurationNanousdPerSecond, rateMultiplier)
		case domainbilling.PricingModeTiered:
			tieredTiers, err = parseTieredPricingTiers(tieredPricingJSON)
			if err != nil {
				return nil, err
			}
		case domainbilling.PricingModeToken:
			baseInputNanousdPerMTokens = pricing.InputNanousdPerMTokens
			baseCacheReadNanousdPerMTokens = pricing.CacheReadNanousdPerMTokens
			baseCacheWriteNanousdPerMTokens = resolveCacheWriteNanousdPerMTokens(
				pricing.CacheWriteNanousdPerMTokens,
				providerProtocol,
				input.CacheTimeout,
			)
			baseCacheWrite5mNanousdPerMTokens = resolveCacheWriteNanousdPerMTokens(
				pricing.CacheWriteNanousdPerMTokens,
				providerProtocol,
				"5m",
			)
			baseCacheWrite1hNanousdPerMTokens = resolveCacheWriteNanousdPerMTokens(
				pricing.CacheWriteNanousdPerMTokens,
				providerProtocol,
				"1h",
			)
			baseOutputNanousdPerMTokens = pricing.OutputNanousdPerMTokens
			inputNanousdPerMTokens = applyRateMultiplier(baseInputNanousdPerMTokens, rateMultiplier)
			cacheReadNanousdPerMTokens = applyRateMultiplier(baseCacheReadNanousdPerMTokens, rateMultiplier)
			cacheWriteNanousdPerMTokens = applyRateMultiplier(baseCacheWriteNanousdPerMTokens, rateMultiplier)
			cacheWrite5mNanousdPerMTokens = applyRateMultiplier(baseCacheWrite5mNanousdPerMTokens, rateMultiplier)
			cacheWrite1hNanousdPerMTokens = applyRateMultiplier(baseCacheWrite1hNanousdPerMTokens, rateMultiplier)
			outputNanousdPerMTokens = applyRateMultiplier(baseOutputNanousdPerMTokens, rateMultiplier)
		}
	}

	callCount := input.CallCount
	if callCount <= 0 {
		callCount = 1
	}
	durationSeconds := input.DurationSeconds
	if durationSeconds < 0 {
		durationSeconds = 0
	}
	if pricingMode == domainbilling.PricingModeDuration && durationSeconds <= 0 {
		durationSeconds = 1
	}
	var inputBilledNanousd int64
	var cacheReadBilledNanousd int64
	var cacheWriteBilledNanousd int64
	var outputBilledNanousd int64
	var callBilledNanousd int64
	var durationBilledNanousd int64
	var tieredFromTokens int64
	var tieredUpToTokens *int64
	switch pricingMode {
	case domainbilling.PricingModeCall:
		if !input.ServiceOnly {
			callBilledNanousd = callCount * callNanousdPerCall
		}
	case domainbilling.PricingModeDuration:
		if !input.ServiceOnly {
			durationBilledNanousd = durationSeconds * durationNanousdPerSecond
		}
	case domainbilling.PricingModeTiered:
		if !input.ServiceOnly {
			tieredInputTokens := tieredPricingInputTokens(input.InputTokens, input.CacheReadTokens, cacheWriteTokens)
			resolvedTier := resolveTieredPricingTier(tieredInputTokens, tieredTiers)
			tier := resolvedTier.tier
			tieredFromTokens = resolvedTier.fromTokens
			tieredUpToTokens = resolvedTier.upToTokens
			baseInputNanousdPerMTokens = tier.inputNanousdPerMTokens
			baseCacheReadNanousdPerMTokens = tierCacheReadRate(tier)
			baseCacheWriteNanousdPerMTokens = resolveCacheWriteNanousdPerMTokens(
				tierCacheWriteRate(tier),
				providerProtocol,
				input.CacheTimeout,
			)
			baseCacheWrite5mNanousdPerMTokens = resolveCacheWriteNanousdPerMTokens(
				tierCacheWriteRate(tier),
				providerProtocol,
				"5m",
			)
			baseCacheWrite1hNanousdPerMTokens = resolveCacheWriteNanousdPerMTokens(
				tierCacheWriteRate(tier),
				providerProtocol,
				"1h",
			)
			baseOutputNanousdPerMTokens = tier.outputNanousdPerMTokens
			inputNanousdPerMTokens = applyRateMultiplier(baseInputNanousdPerMTokens, rateMultiplier)
			cacheReadNanousdPerMTokens = applyRateMultiplier(baseCacheReadNanousdPerMTokens, rateMultiplier)
			cacheWriteNanousdPerMTokens = applyRateMultiplier(baseCacheWriteNanousdPerMTokens, rateMultiplier)
			cacheWrite5mNanousdPerMTokens = applyRateMultiplier(baseCacheWrite5mNanousdPerMTokens, rateMultiplier)
			cacheWrite1hNanousdPerMTokens = applyRateMultiplier(baseCacheWrite1hNanousdPerMTokens, rateMultiplier)
			outputNanousdPerMTokens = applyRateMultiplier(baseOutputNanousdPerMTokens, rateMultiplier)
			inputBilledNanousd = calcNanousdByToken(input.InputTokens, inputNanousdPerMTokens)
			cacheReadBilledNanousd = calcNanousdByToken(input.CacheReadTokens, cacheReadNanousdPerMTokens)
			cacheWriteBilledNanousd = calcCacheWriteBilledNanousd(cacheWriteTokens, cacheWrite5mTokens, cacheWrite1hTokens, cacheWriteNanousdPerMTokens, cacheWrite5mNanousdPerMTokens, cacheWrite1hNanousdPerMTokens)
			outputBilledNanousd = calcNanousdByToken(input.OutputTokens+input.ReasoningTokens, outputNanousdPerMTokens)
		}
	default:
		if !input.ServiceOnly {
			inputBilledNanousd = calcNanousdByToken(input.InputTokens, inputNanousdPerMTokens)
			cacheReadBilledNanousd = calcNanousdByToken(input.CacheReadTokens, cacheReadNanousdPerMTokens)
			cacheWriteBilledNanousd = calcCacheWriteBilledNanousd(cacheWriteTokens, cacheWrite5mTokens, cacheWrite1hTokens, cacheWriteNanousdPerMTokens, cacheWrite5mNanousdPerMTokens, cacheWrite1hNanousdPerMTokens)
			outputBilledNanousd = calcNanousdByToken(input.OutputTokens+input.ReasoningTokens, outputNanousdPerMTokens)
		}
	}
	serviceItems, serviceBilledNanousd, err := s.buildUsageServiceItems(ctx, input.ServiceItems, mode)
	if err != nil {
		return nil, err
	}
	nativeToolBillingEnabled, err := s.repo.GetNativeToolBillingEnabled(ctx)
	if err != nil {
		return nil, err
	}
	nativeToolPricingJSON, err := s.repo.GetNativeToolPricingJSON(ctx)
	if err != nil {
		return nil, err
	}
	nativeToolDefinitions, err := s.nativeToolDefinitions(ctx)
	if err != nil {
		return nil, err
	}
	nativeToolPricingOverrides, err := nativetool.ParsePricingOverridesJSONForDefinitions(nativeToolPricingJSON, nativeToolDefinitions)
	if err != nil {
		nativeToolPricingOverrides = map[string]nativetool.PricingOverride{}
	}
	nativeToolItems, nativeToolBilledNanousd := buildNativeToolServiceItems(input, mode, isFreeModel, nativeToolBillingEnabled, nativeToolPricingOverrides, nativeToolDefinitions)
	if len(nativeToolItems) > 0 {
		serviceItems = append(serviceItems, nativeToolItems...)
		serviceBilledNanousd += nativeToolBilledNanousd
	}
	billedNanousd := inputBilledNanousd + cacheReadBilledNanousd + cacheWriteBilledNanousd + outputBilledNanousd + callBilledNanousd + durationBilledNanousd + serviceBilledNanousd
	cacheWriteNanousdPerMTokens = resolveSnapshotRateFromBilled(cacheWriteTokens, cacheWriteBilledNanousd, cacheWriteNanousdPerMTokens)

	snapshot := map[string]interface{}{
		"platform_model_name":                      platformModelName,
		"routed_binding_code":                      strings.TrimSpace(input.RoutedBindingCode),
		"model_vendor":                             strings.TrimSpace(identity.ModelVendor),
		"model_icon":                               strings.TrimSpace(identity.ModelIcon),
		"provider_protocol":                        providerProtocol,
		"upstream_name":                            strings.TrimSpace(input.UpstreamName),
		"upstream_model_name":                      strings.TrimSpace(input.UpstreamModelName),
		"cache_timeout":                            billingCacheTimeoutSnapshot(providerProtocol, input.CacheTimeout),
		"request_speed":                            requestSpeed,
		"usage_speed":                              usageSpeed,
		"billing_speed":                            billingSpeed,
		"request_service_tier":                     requestServiceTier,
		"usage_service_tier":                       usageServiceTier,
		"billing_service_tier":                     billingServiceTier,
		"fast_mode":                                fastMode,
		"rate_multiplier":                          billingRateMultiplierValue(rateMultiplier),
		"billing_mode":                             mode,
		"pricing_mode":                             pricingMode,
		"is_free_model":                            isFreeModel,
		"currency":                                 currency,
		"input_nanousd_per_m_tokens":               inputNanousdPerMTokens,
		"cache_read_nanousd_per_m_tokens":          cacheReadNanousdPerMTokens,
		"cache_write_nanousd_per_m_tokens":         cacheWriteNanousdPerMTokens,
		"output_nanousd_per_m_tokens":              outputNanousdPerMTokens,
		"call_nanousd_per_call":                    callNanousdPerCall,
		"duration_nanousd_per_second":              durationNanousdPerSecond,
		"base_input_nanousd_per_m_tokens":          baseInputNanousdPerMTokens,
		"base_cache_read_nanousd_per_m_tokens":     baseCacheReadNanousdPerMTokens,
		"base_cache_write_nanousd_per_m_tokens":    baseCacheWriteNanousdPerMTokens,
		"base_cache_write_5m_nanousd_per_m_tokens": baseCacheWrite5mNanousdPerMTokens,
		"base_cache_write_1h_nanousd_per_m_tokens": baseCacheWrite1hNanousdPerMTokens,
		"cache_write_5m_nanousd_per_m_tokens":      cacheWrite5mNanousdPerMTokens,
		"cache_write_1h_nanousd_per_m_tokens":      cacheWrite1hNanousdPerMTokens,
		"cache_write_5m_tokens":                    cacheWrite5mTokens,
		"cache_write_1h_tokens":                    cacheWrite1hTokens,
		"base_output_nanousd_per_m_tokens":         baseOutputNanousdPerMTokens,
		"base_call_nanousd_per_call":               baseCallNanousdPerCall,
		"base_duration_nanousd_per_second":         baseDurationNanousdPerSecond,
		"tiered_pricing_json":                      tieredPricingJSON,
		"tiered_from_tokens":                       tieredFromTokens,
		"tiered_up_to_tokens":                      tieredUpToTokens,
		"input_billed_nanousd":                     inputBilledNanousd,
		"cache_read_billed_nanousd":                cacheReadBilledNanousd,
		"cache_write_billed_nanousd":               cacheWriteBilledNanousd,
		"cache_write_5m_billed_nanousd":            calcNanousdByToken(cacheWrite5mTokens, cacheWrite5mNanousdPerMTokens),
		"cache_write_1h_billed_nanousd":            calcNanousdByToken(cacheWrite1hTokens, cacheWrite1hNanousdPerMTokens),
		"output_billed_nanousd":                    outputBilledNanousd,
		"call_billed_nanousd":                      callBilledNanousd,
		"duration_billed_nanousd":                  durationBilledNanousd,
		"upstream_usage":                           upstreamUsageSnapshot(input),
		"server_side_tool_usage":                   normalizeUsageCountMap(input.ServerSideToolUsage),
		"native_tool_billing_enabled":              nativeToolBillingEnabled,
		"native_tool_pricing_source":               nativeToolPricingSourceForSnapshot(nativeToolPricingJSON, nativeToolDefinitions),
		"native_tool_billed_nanousd":               nativeToolBilledNanousd,
		"base_service_billed_nanousd":              serviceBilledNanousd,
		"service_items":                            usageServiceItemSnapshots(serviceItems),
	}
	snapshotJSON := "{}"
	if raw, marshalErr := json.Marshal(snapshot); marshalErr == nil {
		snapshotJSON = string(raw)
	}

	ledger := &domainbilling.UsageLedger{
		UserID:              input.UserID,
		ConversationID:      input.ConversationID,
		ProviderProtocol:    providerProtocol,
		UpstreamName:        strings.TrimSpace(input.UpstreamName),
		PlatformModelName:   platformModelName,
		RoutedBindingCode:   strings.TrimSpace(input.RoutedBindingCode),
		UpstreamModelName:   strings.TrimSpace(input.UpstreamModelName),
		IsFreeModel:         isFreeModel,
		UsageDate:           time.Now(),
		InputTokens:         input.InputTokens,
		CacheReadTokens:     input.CacheReadTokens,
		CacheWriteTokens:    cacheWriteTokens,
		CacheWrite5mTokens:  cacheWrite5mTokens,
		CacheWrite1hTokens:  cacheWrite1hTokens,
		OutputTokens:        input.OutputTokens,
		ReasoningTokens:     input.ReasoningTokens,
		CallCount:           callCount,
		DurationSeconds:     durationSeconds,
		LatencyMS:           input.LatencyMS,
		UsageSpeed:          billingSpeed,
		ServiceTier:         billingServiceTier,
		BilledCurrency:      currency,
		BilledNanousd:       billedNanousd,
		PricingSnapshotJSON: snapshotJSON,
	}
	return ledger, nil
}

func monthBounds(now time.Time) (time.Time, time.Time) {
	if now.IsZero() {
		now = time.Now()
	}
	year, month, _ := now.Date()
	location := now.Location()
	start := time.Date(year, month, 1, 0, 0, 0, 0, location)
	return start, start.AddDate(0, 1, 0)
}

// ListModelPricing 分页查询模型单价，并补充平台模型身份。
func (s *Service) ListModelPricing(ctx context.Context, query string, page int, pageSize int) ([]ModelPricingView, int64, error) {
	offset, limit := normalizePage(page, pageSize)
	if s.modelPricingCatalog == nil {
		items, total, err := s.repo.ListModelPricing(ctx, query, offset, limit)
		if err != nil {
			return nil, 0, err
		}
		views, err := s.buildModelPricingViews(ctx, items)
		return views, total, err
	}
	validNames, err := s.modelPricingCatalog.ListActivePlatformModelNames(ctx)
	if err != nil {
		return nil, 0, err
	}
	items, _, err := s.repo.ListModelPricing(ctx, query, 0, 5000)
	if err != nil {
		return nil, 0, err
	}
	filtered := filterModelPricingByPlatformNames(items, validNames)
	views, err := s.buildModelPricingViews(ctx, paginateModelPricing(filtered, offset, limit))
	return views, int64(len(filtered)), err
}

// ListPublicModelPricing 查询用户侧模型选择器使用的结构化价格。
func (s *Service) ListPublicModelPricing(ctx context.Context) (map[string]PublicModelPricing, error) {
	now := time.Now()
	useCache := s.modelPricingCatalog == nil
	if useCache {
		s.publicPricingMu.RLock()
		if s.publicPricingByModel != nil && now.Before(s.publicPricingValidUntil) {
			result := clonePublicModelPricingMap(s.publicPricingByModel)
			s.publicPricingMu.RUnlock()
			return result, nil
		}
		s.publicPricingMu.RUnlock()
	}

	items, _, err := s.repo.ListModelPricing(ctx, "", 0, 5000)
	if err != nil {
		return nil, err
	}
	validNames, err := s.activePlatformModelNames(ctx)
	if err != nil {
		return nil, err
	}
	results := make(map[string]PublicModelPricing, len(items))
	for _, item := range items {
		name := strings.TrimSpace(item.PlatformModelName)
		if name == "" {
			continue
		}
		if validNames != nil {
			if _, ok := validNames[name]; !ok {
				continue
			}
		}
		results[name] = toPublicModelPricing(item)
	}
	if useCache {
		s.publicPricingMu.Lock()
		s.publicPricingByModel = clonePublicModelPricingMap(results)
		s.publicPricingValidUntil = now.Add(publicModelPricingCacheTTL)
		s.publicPricingMu.Unlock()
	}
	return results, nil
}

func clonePublicModelPricingMap(input map[string]PublicModelPricing) map[string]PublicModelPricing {
	if len(input) == 0 {
		return map[string]PublicModelPricing{}
	}
	result := make(map[string]PublicModelPricing, len(input))
	for key, value := range input {
		if len(value.Tiers) > 0 {
			value.Tiers = append([]PublicModelPricingTier(nil), value.Tiers...)
		}
		result[key] = value
	}
	return result
}

func toPublicModelPricing(item domainbilling.ModelPricing) PublicModelPricing {
	mode := normalizePricingMode(item.PricingMode)
	result := PublicModelPricing{
		Currency:                firstNonEmpty(item.Currency, "USD"),
		IsFree:                  item.IsFree,
		Mode:                    mode,
		InputUSDPerMTokens:      nanousdToUSD(item.InputNanousdPerMTokens),
		CacheReadUSDPerMTokens:  nanousdToUSD(item.CacheReadNanousdPerMTokens),
		CacheWriteUSDPerMTokens: nanousdToUSD(item.CacheWriteNanousdPerMTokens),
		OutputUSDPerMTokens:     nanousdToUSD(item.OutputNanousdPerMTokens),
		CallUSDPerCall:          nanousdToUSD(item.CallNanousdPerCall),
		DurationUSDPerSecond:    nanousdToUSD(item.DurationNanousdPerSecond),
	}
	if mode == domainbilling.PricingModeTiered {
		tiers, err := parseTieredPricingTiers(item.TieredPricingJSON)
		if err == nil {
			result.Tiers = toPublicModelPricingTiers(tiers)
		}
	}
	return result
}

func toPublicModelPricingTiers(tiers []tieredPricingTier) []PublicModelPricingTier {
	results := make([]PublicModelPricingTier, 0, len(tiers))
	previousLimit := int64(0)
	for _, tier := range tiers {
		var upToTokens *int64
		if tier.UpToTokens > 0 {
			value := tier.UpToTokens
			upToTokens = &value
		}
		results = append(results, PublicModelPricingTier{
			FromTokens:              previousLimit,
			UpToTokens:              upToTokens,
			InputUSDPerMTokens:      nanousdToUSD(tier.inputNanousdPerMTokens),
			CacheReadUSDPerMTokens:  nanousdToUSD(tier.cacheReadNanousdPerMTokens),
			CacheWriteUSDPerMTokens: nanousdToUSD(tier.cacheWriteNanousdPerMTokens),
			OutputUSDPerMTokens:     nanousdToUSD(tier.outputNanousdPerMTokens),
		})
		if tier.UpToTokens > 0 {
			previousLimit = tier.UpToTokens
		}
	}
	return results
}

func nanousdToUSD(value int64) float64 {
	if value <= 0 {
		return 0
	}
	return float64(value) / 1000000000
}

// UpsertModelPricing 保存模型单价。
func (s *Service) UpsertModelPricing(ctx context.Context, input ModelPricingInput) (*ModelPricingView, error) {
	platformModelName := strings.TrimSpace(input.PlatformModelName)
	if platformModelName == "" {
		return nil, ErrInvalidModelPricing
	}
	if err := s.ensurePlatformModelExistsForPricing(ctx, platformModelName); err != nil {
		if errors.Is(err, repository.ErrModelNotFound) {
			return nil, ErrInvalidModelPricing
		}
		return nil, err
	}
	pricingMode := normalizePricingMode(input.PricingMode)
	var inputNanousdPerMTokens int64
	var cacheReadNanousdPerMTokens int64
	var cacheWriteNanousdPerMTokens int64
	var outputNanousdPerMTokens int64
	var callNanousdPerCall int64
	var durationNanousdPerSecond int64
	tieredPricingJSON := "{}"
	switch pricingMode {
	case domainbilling.PricingModeCall:
		callNanousdPerCall = clampNonNegative(input.CallNanousdPerCall)
	case domainbilling.PricingModeDuration:
		durationNanousdPerSecond = clampNonNegative(input.DurationNanousdPerSecond)
	case domainbilling.PricingModeTiered:
		var err error
		tieredPricingJSON, err = normalizeTieredPricingJSON(input.TieredPricingJSON)
		if err != nil {
			return nil, ErrInvalidModelPricing
		}
	default:
		inputNanousdPerMTokens = clampNonNegative(input.InputNanousdPerMTokens)
		cacheReadNanousdPerMTokens = clampNonNegative(input.CacheReadNanousdPerMTokens)
		cacheWriteNanousdPerMTokens = clampNonNegative(input.CacheWriteNanousdPerMTokens)
		outputNanousdPerMTokens = clampNonNegative(input.OutputNanousdPerMTokens)
	}
	item, err := s.repo.UpsertModelPricing(ctx, &domainbilling.ModelPricing{
		PlatformModelName:           platformModelName,
		Currency:                    "USD",
		IsFree:                      input.IsFree,
		PricingMode:                 pricingMode,
		InputNanousdPerMTokens:      inputNanousdPerMTokens,
		CacheReadNanousdPerMTokens:  cacheReadNanousdPerMTokens,
		CacheWriteNanousdPerMTokens: cacheWriteNanousdPerMTokens,
		OutputNanousdPerMTokens:     outputNanousdPerMTokens,
		CallNanousdPerCall:          callNanousdPerCall,
		DurationNanousdPerSecond:    durationNanousdPerSecond,
		TieredPricingJSON:           tieredPricingJSON,
	})
	if err != nil {
		if errors.Is(err, repository.ErrInvalidInput) || errors.Is(err, repository.ErrModelNotFound) {
			return nil, ErrInvalidModelPricing
		}
		return nil, err
	}
	s.invalidatePublicModelPricingCache()
	view, err := s.buildModelPricingView(ctx, *item)
	if err != nil {
		return nil, err
	}
	return &view, nil
}

func (s *Service) buildUsageServiceItems(ctx context.Context, inputs []ServiceUsageInput, billingMode string) ([]domainbilling.UsageServiceItem, int64, error) {
	if len(inputs) == 0 {
		return []domainbilling.UsageServiceItem{}, 0, nil
	}
	results := make([]domainbilling.UsageServiceItem, 0, len(inputs))
	var total int64
	for _, input := range inputs {
		item, err := s.buildUsageServiceItem(ctx, input, billingMode)
		if err != nil {
			return nil, 0, err
		}
		results = append(results, item)
		total += item.BilledNanousd
	}
	return results, total, nil
}

func usageServiceItemSnapshots(items []domainbilling.UsageServiceItem) []map[string]interface{} {
	results := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		results = append(results, map[string]interface{}{
			"service_code":                        item.ServiceCode,
			"service_name":                        item.ServiceName,
			"platform_model_name":                 item.PlatformModelName,
			"provider_protocol":                   item.ProviderProtocol,
			"cache_timeout":                       item.CacheTimeout,
			"request_speed":                       item.RequestSpeed,
			"usage_speed":                         item.UsageSpeed,
			"billing_speed":                       item.BillingSpeed,
			"request_service_tier":                item.RequestServiceTier,
			"usage_service_tier":                  item.UsageServiceTier,
			"billing_service_tier":                item.BillingServiceTier,
			"fast_mode":                           item.FastMode,
			"rate_multiplier":                     item.RateMultiplier,
			"pricing_mode":                        item.PricingMode,
			"input_tokens":                        item.InputTokens,
			"cache_read_tokens":                   item.CacheReadTokens,
			"cache_write_tokens":                  item.CacheWriteTokens,
			"cache_write_5m_tokens":               item.CacheWrite5mTokens,
			"cache_write_1h_tokens":               item.CacheWrite1hTokens,
			"output_tokens":                       item.OutputTokens,
			"reasoning_tokens":                    item.ReasoningTokens,
			"call_count":                          item.CallCount,
			"duration_seconds":                    item.DurationSeconds,
			"input_nanousd_per_m_tokens":          item.InputNanousdPerMTokens,
			"cache_read_nanousd_per_m_tokens":     item.CacheReadNanousdPerMTokens,
			"cache_write_nanousd_per_m_tokens":    item.CacheWriteNanousdPerMTokens,
			"cache_write_5m_nanousd_per_m_tokens": item.CacheWrite5mNanousdPerMTokens,
			"cache_write_1h_nanousd_per_m_tokens": item.CacheWrite1hNanousdPerMTokens,
			"output_nanousd_per_m_tokens":         item.OutputNanousdPerMTokens,
			"call_nanousd_per_call":               item.CallNanousdPerCall,
			"duration_nanousd_per_second":         item.DurationNanousdPerSecond,
			"tiered_from_tokens":                  item.TieredFromTokens,
			"tiered_up_to_tokens":                 item.TieredUpToTokens,
			"input_billed_nanousd":                item.InputBilledNanousd,
			"cache_read_billed_nanousd":           item.CacheReadBilledNanousd,
			"cache_write_billed_nanousd":          item.CacheWriteBilledNanousd,
			"cache_write_5m_billed_nanousd":       item.CacheWrite5mBilledNanousd,
			"cache_write_1h_billed_nanousd":       item.CacheWrite1hBilledNanousd,
			"output_billed_nanousd":               item.OutputBilledNanousd,
			"call_billed_nanousd":                 item.CallBilledNanousd,
			"duration_billed_nanousd":             item.DurationBilledNanousd,
			"billed_nanousd":                      item.BilledNanousd,
		})
	}
	return results
}

func (s *Service) buildUsageServiceItem(ctx context.Context, input ServiceUsageInput, billingMode string) (domainbilling.UsageServiceItem, error) {
	usageSpeed := normalizeUsageSpeed(input.UsageSpeed)
	requestSpeed := normalizeUsageSpeed(input.RequestSpeed)
	billingSpeed := resolveBillingSpeed(input.ProviderProtocol, usageSpeed, requestSpeed)
	requestServiceTier := normalizeOpenAIServiceTier(input.RequestServiceTier)
	usageServiceTier := normalizeOpenAIServiceTier(input.UsageServiceTier)
	billingServiceTier := resolveBillingServiceTier(input.ProviderProtocol, usageServiceTier)
	fastMode := isAnthropicFastMode(input.ProviderProtocol, usageSpeed, requestSpeed)
	rateMultiplier := resolveUsageRateMultiplier(input.ProviderProtocol, input.PlatformModelName, input.UpstreamModelName, fastMode, billingServiceTier)
	cacheWriteTokens, cacheWrite5mTokens, cacheWrite1hTokens := normalizeCacheWriteTokenBreakdown(
		input.CacheWriteTokens,
		input.CacheWrite5mTokens,
		input.CacheWrite1hTokens,
		input.ProviderProtocol,
		input.CacheTimeout,
	)
	item := domainbilling.UsageServiceItem{
		ServiceCode:        strings.TrimSpace(input.ServiceCode),
		ServiceName:        strings.TrimSpace(input.ServiceName),
		PlatformModelName:  strings.TrimSpace(input.PlatformModelName),
		ProviderProtocol:   strings.TrimSpace(input.ProviderProtocol),
		CacheTimeout:       billingCacheTimeoutSnapshot(input.ProviderProtocol, input.CacheTimeout),
		RequestSpeed:       requestSpeed,
		UsageSpeed:         usageSpeed,
		BillingSpeed:       billingSpeed,
		RequestServiceTier: requestServiceTier,
		UsageServiceTier:   usageServiceTier,
		BillingServiceTier: billingServiceTier,
		FastMode:           fastMode,
		RateMultiplier:     billingRateMultiplierValue(rateMultiplier),
		PricingMode:        domainbilling.PricingModeToken,
		InputTokens:        clampNonNegative(input.InputTokens),
		CacheReadTokens:    clampNonNegative(input.CacheReadTokens),
		CacheWriteTokens:   cacheWriteTokens,
		CacheWrite5mTokens: cacheWrite5mTokens,
		CacheWrite1hTokens: cacheWrite1hTokens,
		OutputTokens:       clampNonNegative(input.OutputTokens),
		ReasoningTokens:    clampNonNegative(input.ReasoningTokens),
		CallCount:          input.CallCount,
		DurationSeconds:    input.DurationSeconds,
	}
	if item.ServiceName == "" {
		item.ServiceName = item.ServiceCode
	}
	if item.CallCount <= 0 {
		item.CallCount = 1
	}
	if item.DurationSeconds < 0 {
		item.DurationSeconds = 0
	}
	identity, err := s.resolvePlatformModelIdentity(ctx, item.PlatformModelName)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return item, err
	}
	var pricing *domainbilling.ModelPricing
	resolvedPlatformModelName := strings.TrimSpace(identity.PlatformModelName)
	if resolvedPlatformModelName == "" {
		resolvedPlatformModelName = item.PlatformModelName
	}
	if resolvedPlatformModelName != "" {
		pricing, err = s.repo.GetModelPricing(ctx, resolvedPlatformModelName)
		if err != nil && !errors.Is(err, repository.ErrNotFound) {
			return item, err
		}
	}
	if billingMode == "self" || pricing == nil || pricing.IsFree {
		return item, nil
	}
	item.PricingMode = normalizePricingMode(pricing.PricingMode)
	switch item.PricingMode {
	case domainbilling.PricingModeCall:
		item.CallNanousdPerCall = applyRateMultiplier(pricing.CallNanousdPerCall, rateMultiplier)
		item.CallBilledNanousd = item.CallCount * item.CallNanousdPerCall
	case domainbilling.PricingModeDuration:
		item.DurationNanousdPerSecond = applyRateMultiplier(pricing.DurationNanousdPerSecond, rateMultiplier)
		if item.DurationSeconds <= 0 {
			item.DurationSeconds = 1
		}
		item.DurationBilledNanousd = item.DurationSeconds * item.DurationNanousdPerSecond
	case domainbilling.PricingModeTiered:
		tiers, parseErr := parseTieredPricingTiers(pricing.TieredPricingJSON)
		if parseErr != nil {
			return item, parseErr
		}
		tieredInputTokens := tieredPricingInputTokens(item.InputTokens, item.CacheReadTokens, item.CacheWriteTokens)
		resolvedTier := resolveTieredPricingTier(tieredInputTokens, tiers)
		tier := resolvedTier.tier
		item.TieredFromTokens = resolvedTier.fromTokens
		item.TieredUpToTokens = resolvedTier.upToTokens
		baseInputNanousdPerMTokens := tier.inputNanousdPerMTokens
		baseCacheReadNanousdPerMTokens := tierCacheReadRate(tier)
		baseCacheWriteNanousdPerMTokens := resolveCacheWriteNanousdPerMTokens(
			tierCacheWriteRate(tier),
			input.ProviderProtocol,
			input.CacheTimeout,
		)
		baseCacheWrite5mNanousdPerMTokens := resolveCacheWriteNanousdPerMTokens(
			tierCacheWriteRate(tier),
			input.ProviderProtocol,
			"5m",
		)
		baseCacheWrite1hNanousdPerMTokens := resolveCacheWriteNanousdPerMTokens(
			tierCacheWriteRate(tier),
			input.ProviderProtocol,
			"1h",
		)
		baseOutputNanousdPerMTokens := tier.outputNanousdPerMTokens
		item.InputNanousdPerMTokens = applyRateMultiplier(baseInputNanousdPerMTokens, rateMultiplier)
		item.CacheReadNanousdPerMTokens = applyRateMultiplier(baseCacheReadNanousdPerMTokens, rateMultiplier)
		item.CacheWriteNanousdPerMTokens = applyRateMultiplier(baseCacheWriteNanousdPerMTokens, rateMultiplier)
		item.CacheWrite5mNanousdPerMTokens = applyRateMultiplier(baseCacheWrite5mNanousdPerMTokens, rateMultiplier)
		item.CacheWrite1hNanousdPerMTokens = applyRateMultiplier(baseCacheWrite1hNanousdPerMTokens, rateMultiplier)
		item.OutputNanousdPerMTokens = applyRateMultiplier(baseOutputNanousdPerMTokens, rateMultiplier)
		item.InputBilledNanousd = calcNanousdByToken(item.InputTokens, item.InputNanousdPerMTokens)
		item.CacheReadBilledNanousd = calcNanousdByToken(item.CacheReadTokens, item.CacheReadNanousdPerMTokens)
		item.CacheWriteBilledNanousd = calcCacheWriteBilledNanousd(item.CacheWriteTokens, item.CacheWrite5mTokens, item.CacheWrite1hTokens, item.CacheWriteNanousdPerMTokens, item.CacheWrite5mNanousdPerMTokens, item.CacheWrite1hNanousdPerMTokens)
		item.CacheWrite5mBilledNanousd = calcNanousdByToken(item.CacheWrite5mTokens, item.CacheWrite5mNanousdPerMTokens)
		item.CacheWrite1hBilledNanousd = calcNanousdByToken(item.CacheWrite1hTokens, item.CacheWrite1hNanousdPerMTokens)
		item.CacheWriteNanousdPerMTokens = resolveSnapshotRateFromBilled(item.CacheWriteTokens, item.CacheWriteBilledNanousd, item.CacheWriteNanousdPerMTokens)
		item.OutputBilledNanousd = calcNanousdByToken(item.OutputTokens+item.ReasoningTokens, item.OutputNanousdPerMTokens)
	default:
		baseInputNanousdPerMTokens := pricing.InputNanousdPerMTokens
		baseCacheReadNanousdPerMTokens := pricing.CacheReadNanousdPerMTokens
		baseCacheWriteNanousdPerMTokens := resolveCacheWriteNanousdPerMTokens(
			pricing.CacheWriteNanousdPerMTokens,
			input.ProviderProtocol,
			input.CacheTimeout,
		)
		baseCacheWrite5mNanousdPerMTokens := resolveCacheWriteNanousdPerMTokens(
			pricing.CacheWriteNanousdPerMTokens,
			input.ProviderProtocol,
			"5m",
		)
		baseCacheWrite1hNanousdPerMTokens := resolveCacheWriteNanousdPerMTokens(
			pricing.CacheWriteNanousdPerMTokens,
			input.ProviderProtocol,
			"1h",
		)
		baseOutputNanousdPerMTokens := pricing.OutputNanousdPerMTokens
		item.InputNanousdPerMTokens = applyRateMultiplier(baseInputNanousdPerMTokens, rateMultiplier)
		item.CacheReadNanousdPerMTokens = applyRateMultiplier(baseCacheReadNanousdPerMTokens, rateMultiplier)
		item.CacheWriteNanousdPerMTokens = applyRateMultiplier(baseCacheWriteNanousdPerMTokens, rateMultiplier)
		item.CacheWrite5mNanousdPerMTokens = applyRateMultiplier(baseCacheWrite5mNanousdPerMTokens, rateMultiplier)
		item.CacheWrite1hNanousdPerMTokens = applyRateMultiplier(baseCacheWrite1hNanousdPerMTokens, rateMultiplier)
		item.OutputNanousdPerMTokens = applyRateMultiplier(baseOutputNanousdPerMTokens, rateMultiplier)
		item.InputBilledNanousd = calcNanousdByToken(item.InputTokens, item.InputNanousdPerMTokens)
		item.CacheReadBilledNanousd = calcNanousdByToken(item.CacheReadTokens, item.CacheReadNanousdPerMTokens)
		item.CacheWriteBilledNanousd = calcCacheWriteBilledNanousd(item.CacheWriteTokens, item.CacheWrite5mTokens, item.CacheWrite1hTokens, item.CacheWriteNanousdPerMTokens, item.CacheWrite5mNanousdPerMTokens, item.CacheWrite1hNanousdPerMTokens)
		item.CacheWrite5mBilledNanousd = calcNanousdByToken(item.CacheWrite5mTokens, item.CacheWrite5mNanousdPerMTokens)
		item.CacheWrite1hBilledNanousd = calcNanousdByToken(item.CacheWrite1hTokens, item.CacheWrite1hNanousdPerMTokens)
		item.CacheWriteNanousdPerMTokens = resolveSnapshotRateFromBilled(item.CacheWriteTokens, item.CacheWriteBilledNanousd, item.CacheWriteNanousdPerMTokens)
		item.OutputBilledNanousd = calcNanousdByToken(item.OutputTokens+item.ReasoningTokens, item.OutputNanousdPerMTokens)
	}
	item.BilledNanousd = item.InputBilledNanousd +
		item.CacheReadBilledNanousd +
		item.CacheWriteBilledNanousd +
		item.OutputBilledNanousd +
		item.CallBilledNanousd +
		item.DurationBilledNanousd
	return item, nil
}

// ListUsage 分页查询账本。
func (s *Service) ListUsage(ctx context.Context, userID uint, page int, pageSize int, filter UsageListFilter) ([]domainbilling.UsageLedger, int64, error) {
	offset, limit := normalizePage(page, pageSize)
	return s.repo.ListUsageByUser(ctx, userID, repository.UsageListFilter{
		Query:  filter.Query,
		Status: filter.Status,
		Sort:   filter.Sort,
	}, offset, limit)
}

// ListUsageLogs 分页查询管理员调用日志。
func (s *Service) ListUsageLogs(ctx context.Context, page int, pageSize int, filter UsageLogListFilter) ([]domainbilling.UsageLedger, int64, error) {
	offset, limit := normalizePage(page, pageSize)
	return s.repo.ListUsageLogs(ctx, repository.UsageLogListFilter{
		Query:             filter.Query,
		PlatformModelName: filter.PlatformModelName,
		BillingMode:       filter.BillingMode,
		UserID:            filter.UserID,
		CreatedFrom:       filter.CreatedFrom,
		CreatedTo:         filter.CreatedTo,
		Sort:              filter.Sort,
	}, offset, limit)
}

// ListPaymentOrders 分页查询管理员支付订单记录。
func (s *Service) ListPaymentOrders(ctx context.Context, page int, pageSize int, filter PaymentOrderListFilter) ([]domainbilling.PaymentOrder, int64, error) {
	offset, limit := normalizePage(page, pageSize)
	return s.repo.ListPaymentOrders(ctx, repository.PaymentOrderListFilter{
		Query:       filter.Query,
		OrderType:   filter.OrderType,
		Provider:    filter.Provider,
		Status:      filter.Status,
		UserID:      filter.UserID,
		CreatedFrom: filter.CreatedFrom,
		CreatedTo:   filter.CreatedTo,
		Sort:        filter.Sort,
	}, offset, limit)
}

func normalizePage(page int, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	offset := (page - 1) * pageSize
	if offset < 0 {
		offset = 0
	}
	return offset, pageSize
}

// ListMonthlyUsage 查询用户月度用量聚合。
func (s *Service) ListMonthlyUsage(ctx context.Context, userID uint, months int) ([]domainbilling.UsageMonthlySummary, error) {
	if months <= 0 {
		months = 12
	}
	if months > 24 {
		months = 24
	}
	items, err := s.repo.ListMonthlyUsageByUser(ctx, userID, months)
	if err != nil {
		return nil, err
	}
	return fillMonthlyUsageSummaries(items, months, time.Now()), nil
}

func fillMonthlyUsageSummaries(items []domainbilling.UsageMonthlySummary, months int, now time.Time) []domainbilling.UsageMonthlySummary {
	if months <= 0 {
		months = 12
	}
	if now.IsZero() {
		now = time.Now()
	}
	location := now.Location()
	currentMonth := time.Date(now.In(location).Year(), now.In(location).Month(), 1, 0, 0, 0, 0, location)
	startMonth := currentMonth.AddDate(0, -(months - 1), 0)

	byMonth := make(map[string]domainbilling.UsageMonthlySummary, len(items))
	for _, item := range items {
		month := item.MonthStartAt.In(location)
		month = time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, location)
		item.MonthStartAt = month
		byMonth[month.Format("2006-01")] = item
	}

	results := make([]domainbilling.UsageMonthlySummary, 0, months)
	for month := startMonth; !month.After(currentMonth); month = month.AddDate(0, 1, 0) {
		if item, ok := byMonth[month.Format("2006-01")]; ok {
			results = append(results, item)
			continue
		}
		results = append(results, domainbilling.UsageMonthlySummary{MonthStartAt: month})
	}
	return results
}

// ListDailyUsage 查询用户每日用量聚合。
func (s *Service) ListDailyUsage(ctx context.Context, userID uint, days int, now time.Time) ([]domainbilling.UsageDailySummary, error) {
	if days <= 0 {
		days = 30
	}
	if days > 90 {
		days = 90
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	startDate := today.AddDate(0, 0, -(days - 1))
	endDate := today.AddDate(0, 0, 1)

	items, err := s.repo.ListDailyUsageByUser(ctx, userID, startDate, endDate)
	if err != nil {
		return nil, err
	}

	byDate := make(map[string]domainbilling.UsageDailySummary, len(items))
	for _, item := range items {
		byDate[item.UsageDate.Format("2006-01-02")] = item
	}
	results := make([]domainbilling.UsageDailySummary, 0, days)
	for day := startDate; day.Before(endDate); day = day.AddDate(0, 0, 1) {
		if item, ok := byDate[day.Format("2006-01-02")]; ok {
			results = append(results, item)
			continue
		}
		results = append(results, domainbilling.UsageDailySummary{UsageDate: day})
	}
	return results, nil
}

// ListCurrentCycleDailyUsage 查询当前注册锚定月周期的每日用量聚合。
func (s *Service) ListCurrentCycleDailyUsage(ctx context.Context, userID uint, now time.Time) ([]domainbilling.UsageDailySummary, error) {
	createdAt, err := s.repo.GetUserCreatedAt(ctx, userID)
	if err != nil {
		return nil, err
	}
	start, end := resolveAnchoredMonthlyCycle(createdAt, now)
	return s.listDailyUsageBetween(ctx, userID, start, end)
}

// ListDailyUsageRange 查询用户指定日期区间内的每日用量聚合。
func (s *Service) ListDailyUsageRange(ctx context.Context, userID uint, startDate time.Time, endDate time.Time) ([]domainbilling.UsageDailySummary, error) {
	start := time.Date(startDate.Year(), startDate.Month(), startDate.Day(), 0, 0, 0, 0, startDate.Location())
	end := time.Date(endDate.Year(), endDate.Month(), endDate.Day(), 0, 0, 0, 0, endDate.Location())
	if !end.After(start) {
		end = start.AddDate(0, 0, 1)
	}
	if end.After(start.AddDate(0, 0, 90)) {
		end = start.AddDate(0, 0, 90)
	}
	return s.listDailyUsageBetween(ctx, userID, start, end)
}

func (s *Service) listDailyUsageBetween(ctx context.Context, userID uint, start time.Time, end time.Time) ([]domainbilling.UsageDailySummary, error) {
	items, err := s.repo.ListDailyUsageByUser(ctx, userID, start, end)
	if err != nil {
		return nil, err
	}

	byDate := make(map[string]domainbilling.UsageDailySummary, len(items))
	for _, item := range items {
		byDate[item.UsageDate.Format("2006-01-02")] = item
	}
	days := int(end.Sub(start).Hours() / 24)
	results := make([]domainbilling.UsageDailySummary, 0, days)
	for day := start; day.Before(end); day = day.AddDate(0, 0, 1) {
		if item, ok := byDate[day.Format("2006-01-02")]; ok {
			results = append(results, item)
			continue
		}
		results = append(results, domainbilling.UsageDailySummary{UsageDate: day})
	}
	return results, nil
}

func resolveAnchoredMonthlyCycle(anchor time.Time, now time.Time) (time.Time, time.Time) {
	if now.IsZero() {
		now = time.Now()
	}
	location := now.Location()
	if anchor.IsZero() {
		anchor = now
	}
	anchorDay := time.Date(anchor.In(location).Year(), anchor.In(location).Month(), anchor.In(location).Day(), 0, 0, 0, 0, location)
	today := time.Date(now.In(location).Year(), now.In(location).Month(), now.In(location).Day(), 0, 0, 0, 0, location)
	monthOffset := (today.Year()-anchorDay.Year())*12 + int(today.Month()) - int(anchorDay.Month())
	start := addAnchoredMonths(anchorDay, monthOffset)
	if today.Before(start) {
		monthOffset--
		start = addAnchoredMonths(anchorDay, monthOffset)
	}
	end := addAnchoredMonths(anchorDay, monthOffset+1)
	for !today.Before(end) {
		monthOffset++
		start = end
		end = addAnchoredMonths(anchorDay, monthOffset+1)
	}
	return start, end
}

func addAnchoredMonths(anchor time.Time, monthOffset int) time.Time {
	monthIndex := int(anchor.Month()) - 1 + monthOffset
	year := anchor.Year() + floorDiv(monthIndex, 12)
	month := time.Month(mod(monthIndex, 12) + 1)
	day := anchor.Day()
	if lastDay := lastDayOfMonth(year, month, anchor.Location()); day > lastDay {
		day = lastDay
	}
	return time.Date(year, month, day, 0, 0, 0, 0, anchor.Location())
}

func floorDiv(value int, divisor int) int {
	result := value / divisor
	if value%divisor != 0 && value < 0 {
		result--
	}
	return result
}

func mod(value int, divisor int) int {
	result := value % divisor
	if result < 0 {
		result += divisor
	}
	return result
}

func lastDayOfMonth(year int, month time.Month, location *time.Location) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, location).Day()
}

// GetBillingOverview 查询当前用户计费概览。
func (s *Service) GetBillingOverview(ctx context.Context, userID uint, now time.Time) (*BillingOverview, error) {
	mode, err := s.repo.GetBillingMode(ctx)
	if err != nil {
		return nil, err
	}

	overview := &BillingOverview{Mode: mode}
	if mode == "usage" {
		account, accountErr := s.repo.GetOrCreateBillingAccount(ctx, userID)
		if accountErr != nil {
			return nil, accountErr
		}
		overview.Account = toBillingAccountView(account)
		return overview, nil
	}
	if mode != "period" {
		return overview, nil
	}

	plan, startAt, endAt, err := s.currentPeriodPlan(ctx, userID, now)
	if err != nil {
		return nil, err
	}
	usedNanousd, err := s.repo.SumBillableNanousd(ctx, userID, startAt, endAt)
	if err != nil {
		return nil, err
	}
	remainingNanousd := plan.PeriodCreditNanousd - usedNanousd
	if remainingNanousd < 0 {
		remainingNanousd = 0
	}
	planView := BillingPlanView{
		ID:                  plan.ID,
		Code:                plan.Code,
		Name:                plan.Name,
		Description:         plan.Description,
		FeatureJSON:         plan.FeatureJSON,
		PeriodCreditNanousd: plan.PeriodCreditNanousd,
		DiscountPercent:     plan.DiscountPercent,
		SortOrder:           plan.SortOrder,
		IsActive:            plan.IsActive,
	}
	prices, priceErr := s.repo.ListActivePricesByPlanIDs(ctx, []uint{plan.ID})
	if priceErr != nil {
		return nil, priceErr
	}
	for _, price := range prices {
		planView.Prices = append(planView.Prices, BillingPriceView{
			ID:              price.ID,
			PlanID:          price.PlanID,
			Code:            price.Code,
			BillingInterval: price.BillingInterval,
			Currency:        price.Currency,
			AmountCents:     price.AmountCents,
			IsDefault:       price.IsDefault,
		})
	}
	subscriptions, planMap, err := s.listSubscriptionEntitlements(ctx, []uint{userID}, now)
	if err != nil {
		return nil, err
	}

	overview.Plan = &planView
	overview.PeriodStartAt = &startAt
	overview.PeriodEndAt = &endAt
	overview.PeriodCreditNanousd = plan.PeriodCreditNanousd
	overview.PeriodUsedNanousd = usedNanousd
	overview.PeriodRemainingNanousd = remainingNanousd
	overview.SubscriptionEntitlements = buildSubscriptionEntitlementViews(subscriptions, planMap, now)
	return overview, nil
}

// GetBillingAccount 查询或创建当前用户按量余额账户。
func (s *Service) GetBillingAccount(ctx context.Context, userID uint) (*domainbilling.BillingAccount, error) {
	return s.repo.GetOrCreateBillingAccount(ctx, userID)
}

// SetBillingAccountBalance 管理员设置用户按量余额。
func (s *Service) SetBillingAccountBalance(ctx context.Context, input BillingAccountBalanceInput) (*domainbilling.BillingAccount, error) {
	if input.UserID == 0 || input.BalanceUSD < 0 || math.IsNaN(input.BalanceUSD) || math.IsInf(input.BalanceUSD, 0) {
		return nil, repository.ErrInvalidInput
	}
	mode, err := s.repo.GetBillingMode(ctx)
	if err != nil {
		return nil, err
	}
	if mode != "usage" {
		return nil, ErrPaymentRequired
	}
	return s.repo.SetBillingAccountBalance(ctx, input.UserID, usdToNanousd(input.BalanceUSD), input.RefNo, input.Description)
}

func toBillingAccountView(account *domainbilling.BillingAccount) *BillingAccountView {
	if account == nil {
		return nil
	}
	return &BillingAccountView{
		UserID:         account.UserID,
		Currency:       account.Currency,
		BalanceNanousd: account.BalanceNanousd,
		Status:         account.Status,
		UpdatedAt:      account.UpdatedAt,
	}
}

func resolvePeriodEnd(now time.Time, interval string, cycles int) *time.Time {
	if cycles <= 0 {
		cycles = 1
	}

	switch interval {
	case domainbilling.IntervalYear:
		end := now.AddDate(cycles, 0, 0)
		return &end
	case domainbilling.IntervalLifetime:
		return nil
	case domainbilling.IntervalMonth:
		fallthrough
	default:
		end := now.AddDate(0, cycles, 0)
		return &end
	}
}

func calcNanousdByToken(tokens int64, nanousdPerMTokens int64) int64 {
	if tokens <= 0 || nanousdPerMTokens <= 0 {
		return 0
	}
	return (tokens*nanousdPerMTokens + 500000) / 1000000
}

func calcCacheWriteBilledNanousd(totalTokens int64, fiveMinuteTokens int64, oneHourTokens int64, aggregateRate int64, fiveMinuteRate int64, oneHourRate int64) int64 {
	if totalTokens <= 0 {
		return 0
	}
	if fiveMinuteTokens <= 0 && oneHourTokens <= 0 {
		return calcNanousdByToken(totalTokens, aggregateRate)
	}
	return calcNanousdByToken(fiveMinuteTokens, fiveMinuteRate) + calcNanousdByToken(oneHourTokens, oneHourRate)
}

func resolveSnapshotRateFromBilled(tokens int64, billedNanousd int64, fallbackRate int64) int64 {
	if tokens <= 0 || billedNanousd <= 0 {
		return fallbackRate
	}
	return (billedNanousd*1000000 + tokens/2) / tokens
}

func resolveCacheWriteNanousdPerMTokens(configuredRate int64, providerProtocol string, cacheTimeout string) int64 {
	if strings.TrimSpace(providerProtocol) != "anthropic_messages" {
		return configuredRate
	}
	if configuredRate <= 0 {
		return 0
	}
	switch normalizeAnthropicCacheTimeout(cacheTimeout) {
	case "1h":
		return configuredRate * 2
	default:
		return configuredRate * 5 / 4
	}
}

func normalizeCacheWriteTokenBreakdown(totalTokens int64, fiveMinuteTokens int64, oneHourTokens int64, providerProtocol string, cacheTimeout string) (int64, int64, int64) {
	totalTokens = clampNonNegative(totalTokens)
	fiveMinuteTokens = clampNonNegative(fiveMinuteTokens)
	oneHourTokens = clampNonNegative(oneHourTokens)
	if strings.TrimSpace(providerProtocol) != "anthropic_messages" {
		return totalTokens, 0, 0
	}
	splitTokens := fiveMinuteTokens + oneHourTokens
	if splitTokens > totalTokens {
		totalTokens = splitTokens
	}
	remainder := totalTokens - splitTokens
	if remainder <= 0 {
		return totalTokens, fiveMinuteTokens, oneHourTokens
	}
	if normalizeAnthropicCacheTimeout(cacheTimeout) == "1h" {
		oneHourTokens += remainder
	} else {
		fiveMinuteTokens += remainder
	}
	return totalTokens, fiveMinuteTokens, oneHourTokens
}

func applyRateMultiplier(rate int64, multiplier billingRateMultiplier) int64 {
	if rate <= 0 {
		return 0
	}
	multiplier = normalizeBillingRateMultiplier(multiplier)
	if multiplier.Numerator == multiplier.Denominator {
		return rate
	}
	return (rate*multiplier.Numerator + multiplier.Denominator/2) / multiplier.Denominator
}

func normalizeBillingRateMultiplier(multiplier billingRateMultiplier) billingRateMultiplier {
	if multiplier.Numerator <= 0 || multiplier.Denominator <= 0 {
		return billingRateMultiplier{Numerator: 1, Denominator: 1}
	}
	return multiplier
}

func billingRateMultiplierValue(multiplier billingRateMultiplier) float64 {
	multiplier = normalizeBillingRateMultiplier(multiplier)
	return float64(multiplier.Numerator) / float64(multiplier.Denominator)
}

func resolveUsageRateMultiplier(providerProtocol string, platformModelName string, upstreamModelName string, fastMode bool, billingServiceTier string) billingRateMultiplier {
	if fastMode {
		return billingRateMultiplier{Numerator: 6, Denominator: 1}
	}
	if !isOpenAIProviderProtocol(providerProtocol) {
		return billingRateMultiplier{Numerator: 1, Denominator: 1}
	}
	switch normalizeOpenAIServiceTier(billingServiceTier) {
	case "flex":
		return billingRateMultiplier{Numerator: 1, Denominator: 2}
	case "priority":
		if isGPT55Series(platformModelName) || isGPT55Series(upstreamModelName) {
			return billingRateMultiplier{Numerator: 5, Denominator: 2}
		}
		return billingRateMultiplier{Numerator: 2, Denominator: 1}
	default:
		return billingRateMultiplier{Numerator: 1, Denominator: 1}
	}
}

func isOpenAIProviderProtocol(providerProtocol string) bool {
	switch strings.TrimSpace(providerProtocol) {
	case "openai_chat_completions", "openai_responses":
		return true
	default:
		return false
	}
}

func isGPT55Series(modelName string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(modelName)), "gpt-5.5")
}

func normalizeUsageSpeed(value string) string {
	return strings.TrimSpace(strings.ToLower(value))
}

func resolveBillingSpeed(providerProtocol string, usageSpeed string, requestSpeed string) string {
	if strings.TrimSpace(providerProtocol) != "anthropic_messages" {
		return ""
	}
	if usageSpeed != "" {
		return normalizeUsageSpeed(usageSpeed)
	}
	return normalizeUsageSpeed(requestSpeed)
}

func normalizeOpenAIServiceTier(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "default", "flex", "priority":
		return strings.TrimSpace(strings.ToLower(value))
	default:
		return ""
	}
}

func resolveBillingServiceTier(providerProtocol string, usageServiceTier string) string {
	if !isOpenAIProviderProtocol(providerProtocol) {
		return ""
	}
	if usageServiceTier != "" {
		return normalizeOpenAIServiceTier(usageServiceTier)
	}
	return "default"
}

func isAnthropicFastMode(providerProtocol string, usageSpeed string, requestSpeed string) bool {
	if strings.TrimSpace(providerProtocol) != "anthropic_messages" {
		return false
	}
	usageSpeed = normalizeUsageSpeed(usageSpeed)
	requestSpeed = normalizeUsageSpeed(requestSpeed)
	if usageSpeed != "" {
		return usageSpeed == "fast"
	}
	return requestSpeed == "fast"
}

func normalizeAnthropicCacheTimeout(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "1h":
		return "1h"
	default:
		return "5m"
	}
}

func billingCacheTimeoutSnapshot(providerProtocol string, cacheTimeout string) string {
	if strings.TrimSpace(providerProtocol) != "anthropic_messages" {
		return ""
	}
	return normalizeAnthropicCacheTimeout(cacheTimeout)
}

func tierUpperLimit(limit int64) *int64 {
	if limit <= 0 {
		return nil
	}
	value := limit
	return &value
}

func tierCacheReadRate(tier tieredPricingTier) int64 {
	return tier.cacheReadNanousdPerMTokens
}

func tierCacheWriteRate(tier tieredPricingTier) int64 {
	return tier.cacheWriteNanousdPerMTokens
}

func resolveTieredPricingTier(inputTokens int64, tiers []tieredPricingTier) resolvedTieredPricingTier {
	if len(tiers) == 0 {
		return resolvedTieredPricingTier{}
	}
	previousLimit := int64(0)
	lastFromTokens := int64(0)
	for _, tier := range tiers {
		if tier.UpToTokens <= 0 || inputTokens <= tier.UpToTokens {
			return resolvedTieredPricingTier{
				tier:       tier,
				fromTokens: previousLimit,
				upToTokens: tierUpperLimit(tier.UpToTokens),
			}
		}
		lastFromTokens = previousLimit
		previousLimit = tier.UpToTokens
	}
	lastTier := tiers[len(tiers)-1]
	return resolvedTieredPricingTier{
		tier:       lastTier,
		fromTokens: lastFromTokens,
		upToTokens: nil,
	}
}

func tieredPricingInputTokens(inputTokens int64, cacheReadTokens int64, cacheWriteTokens int64) int64 {
	return clampNonNegative(inputTokens) + clampNonNegative(cacheReadTokens) + clampNonNegative(cacheWriteTokens)
}

func normalizeTieredPricingJSON(raw string) (string, error) {
	tiers, err := parseTieredPricingTiers(raw)
	if err != nil {
		return "", err
	}
	normalized := tieredPricingConfig{Tiers: tiers}
	payload, err := json.Marshal(normalized)
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

func parseTieredPricingTiers(raw string) ([]tieredPricingTier, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = "{}"
	}
	var config tieredPricingConfig
	if err := json.Unmarshal([]byte(raw), &config); err != nil {
		return nil, err
	}
	if len(config.Tiers) == 0 || len(config.Tiers) > 100 {
		return nil, repository.ErrInvalidInput
	}
	previousLimit := int64(0)
	for index := range config.Tiers {
		tier := &config.Tiers[index]
		if tier.UpToTokens > 0 {
			if tier.UpToTokens <= previousLimit {
				return nil, repository.ErrInvalidInput
			}
			previousLimit = tier.UpToTokens
		}
		tier.inputNanousdPerMTokens = usdToNanousd(tier.InputUSDPerMTokens)
		tier.cacheReadNanousdPerMTokens = usdToNanousd(tier.CacheReadUSDPerMTokens)
		tier.cacheWriteNanousdPerMTokens = usdToNanousd(tier.CacheWriteUSDPerMTokens)
		tier.outputNanousdPerMTokens = usdToNanousd(tier.OutputUSDPerMTokens)
	}
	return config.Tiers, nil
}

func normalizePricingMode(value string) string {
	switch strings.TrimSpace(value) {
	case domainbilling.PricingModeCall:
		return domainbilling.PricingModeCall
	case domainbilling.PricingModeDuration:
		return domainbilling.PricingModeDuration
	case domainbilling.PricingModeTiered:
		return domainbilling.PricingModeTiered
	default:
		return domainbilling.PricingModeToken
	}
}

func centsToNanousd(value int64) int64 {
	if value <= 0 {
		return 0
	}
	return value * 10000000
}

func usdToNanousd(value float64) int64 {
	if value <= 0 {
		return 0
	}
	return int64(math.Round(value * 1000000000))
}

// resolvePlatformModelIdentity 解析平台模型的稳定身份。
func (s *Service) resolvePlatformModelIdentity(ctx context.Context, platformModelName string) (PlatformModelIdentity, error) {
	name := strings.TrimSpace(platformModelName)
	if name == "" {
		return PlatformModelIdentity{}, repository.ErrNotFound
	}
	if s != nil && s.platformModelIdentityResolver != nil {
		identity, err := s.platformModelIdentityResolver.ResolvePlatformModelIdentity(ctx, name)
		if err != nil {
			if errors.Is(err, repository.ErrModelNotFound) {
				return PlatformModelIdentity{}, repository.ErrNotFound
			}
			return PlatformModelIdentity{}, err
		}
		identity.PlatformModelName = strings.TrimSpace(identity.PlatformModelName)
		if identity.PlatformModelName == "" {
			identity.PlatformModelName = name
		}
		return identity, nil
	}
	return PlatformModelIdentity{
		PlatformModelName: name,
	}, nil
}

func (s *Service) getResolvedModelPricing(ctx context.Context, platformModelName string) (*domainbilling.ModelPricing, error) {
	identity, err := s.resolvePlatformModelIdentity(ctx, platformModelName)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(identity.PlatformModelName)
	if name == "" {
		return nil, repository.ErrNotFound
	}
	return s.repo.GetModelPricing(ctx, name)
}

func (s *Service) activePlatformModelNames(ctx context.Context) (map[string]struct{}, error) {
	if s == nil || s.modelPricingCatalog == nil {
		return nil, nil
	}
	return s.modelPricingCatalog.ListActivePlatformModelNames(ctx)
}

func (s *Service) ensurePlatformModelExistsForPricing(ctx context.Context, platformModelName string) error {
	validNames, err := s.activePlatformModelNames(ctx)
	if err != nil {
		return err
	}
	if validNames == nil {
		return nil
	}
	if _, ok := validNames[strings.TrimSpace(platformModelName)]; !ok {
		return repository.ErrModelNotFound
	}
	return nil
}

// buildModelPricingViews 补充模型单价对应的平台模型身份。
func (s *Service) buildModelPricingViews(ctx context.Context, items []domainbilling.ModelPricing) ([]ModelPricingView, error) {
	results := make([]ModelPricingView, 0, len(items))
	for _, item := range items {
		view, err := s.buildModelPricingView(ctx, item)
		if err != nil {
			return nil, err
		}
		results = append(results, view)
	}
	return results, nil
}

func (s *Service) buildModelPricingView(ctx context.Context, item domainbilling.ModelPricing) (ModelPricingView, error) {
	identity, err := s.resolvePlatformModelIdentity(ctx, item.PlatformModelName)
	if err != nil {
		return ModelPricingView{}, err
	}
	return ModelPricingView{
		ModelPricing: item,
		ModelVendor:  identity.ModelVendor,
		ModelIcon:    identity.ModelIcon,
	}, nil
}

func filterModelPricingByPlatformNames(items []domainbilling.ModelPricing, validNames map[string]struct{}) []domainbilling.ModelPricing {
	if validNames == nil {
		return items
	}
	results := make([]domainbilling.ModelPricing, 0, len(items))
	for _, item := range items {
		if _, ok := validNames[strings.TrimSpace(item.PlatformModelName)]; !ok {
			continue
		}
		results = append(results, item)
	}
	return results
}

func paginateModelPricing(items []domainbilling.ModelPricing, offset int, limit int) []domainbilling.ModelPricing {
	if len(items) == 0 || offset >= len(items) {
		return []domainbilling.ModelPricing{}
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	return items[offset:end]
}

// buildNativeToolServiceItems 将原生 server-side tool 调用转换为账单服务项。
func buildNativeToolServiceItems(input UsagePricingInput, billingMode string, isFreeModel bool, enabled bool, pricingOverrides map[string]nativetool.PricingOverride, definitions []nativetool.Definition) ([]domainbilling.UsageServiceItem, int64) {
	if billingMode == "self" || isFreeModel || !enabled || len(input.ServerSideToolUsage) == 0 {
		return []domainbilling.UsageServiceItem{}, 0
	}
	counts := normalizeUsageCountMap(input.ServerSideToolUsage)
	if len(counts) == 0 {
		return []domainbilling.UsageServiceItem{}, 0
	}
	results := make([]domainbilling.UsageServiceItem, 0, len(counts))
	var total int64
	for toolName, count := range counts {
		price, ok := nativeToolDefaultCallPrice(input, toolName, pricingOverrides, definitions)
		if !ok || price.NanousdPerCall <= 0 || count <= 0 {
			continue
		}
		billed := count * price.NanousdPerCall
		results = append(results, domainbilling.UsageServiceItem{
			ServiceCode:        nativeToolServiceCode(price.Provider, toolName),
			ServiceName:        price.ServiceName,
			PlatformModelName:  strings.TrimSpace(input.PlatformModelName),
			ProviderProtocol:   strings.TrimSpace(input.ProviderProtocol),
			RateMultiplier:     1,
			PricingMode:        domainbilling.PricingModeCall,
			CallCount:          count,
			CallNanousdPerCall: price.NanousdPerCall,
			CallBilledNanousd:  billed,
			BilledNanousd:      billed,
		})
		total += billed
	}
	return results, total
}

// nativeToolDefaultCallPrice 返回当前已适配厂商原生工具的官方默认按次价格。
func nativeToolDefaultCallPrice(input UsagePricingInput, toolName string, pricingOverrides map[string]nativetool.PricingOverride, definitions []nativetool.Definition) (nativetool.UsagePrice, bool) {
	return nativetool.UsagePriceForToolWithOverrides(input.ProviderProtocol, toolName, definitions, pricingOverrides)
}

func nativeToolPricingSourceForSnapshot(raw string, definitions []nativetool.Definition) string {
	if nativetool.PricingOverridesUseDefaultsForDefinitions(raw, definitions) {
		return nativeToolPricingSource
	}
	return "admin_configured"
}

// nativeToolServiceCode 生成原生工具服务项编码，供账单明细和快照稳定引用。
func nativeToolServiceCode(provider string, toolName string) string {
	provider = strings.TrimSpace(provider)
	tool := strings.TrimSpace(toolName)
	if provider == "" {
		provider = "unknown"
	}
	if tool == "" {
		tool = "unknown"
	}
	return "native_tool." + provider + "." + tool
}

func normalizeCurrency(value string) string {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	if normalized == "" {
		return "USD"
	}
	return normalized
}

func resolveUSDToCNYRate(value float64) float64 {
	if value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return 7.2
	}
	return value
}

func resolvePaymentQuote(provider string, baseCurrency string, baseAmountCents int64, usdToCNYRate float64) paymentQuote {
	baseCurrency = normalizeCurrency(baseCurrency)
	quote := paymentQuote{
		BaseCurrency:    baseCurrency,
		BaseAmountCents: baseAmountCents,
		PayCurrency:     baseCurrency,
		PayAmountCents:  baseAmountCents,
		FXRate:          1,
	}
	if provider != domainbilling.PaymentProviderEPay {
		return quote
	}
	quote.PayCurrency = "CNY"
	quote.FXRate = resolveUSDToCNYRate(usdToCNYRate)
	quote.PayAmountCents = convertPaymentAmountCents(baseAmountCents, baseCurrency, quote.PayCurrency, quote.FXRate)
	return quote
}

func convertPaymentAmountCents(baseAmountCents int64, baseCurrency string, payCurrency string, rate float64) int64 {
	if baseAmountCents <= 0 {
		return 0
	}
	baseCurrency = normalizeCurrency(baseCurrency)
	payCurrency = normalizeCurrency(payCurrency)
	if baseCurrency == payCurrency {
		return baseAmountCents
	}
	if baseCurrency == "USD" && payCurrency == "CNY" {
		return int64(math.Round(float64(baseAmountCents) * resolveUSDToCNYRate(rate)))
	}
	return 0
}

func formatFXRate(value float64) string {
	return strconv.FormatFloat(resolveUSDToCNYRate(value), 'f', -1, 64)
}

func clampNonNegative(value int64) int64 {
	if value < 0 {
		return 0
	}
	return value
}

func normalizeUsageCountMap(items map[string]int64) map[string]int64 {
	if len(items) == 0 {
		return map[string]int64{}
	}
	result := make(map[string]int64, len(items))
	for key, value := range items {
		normalizedKey := strings.TrimSpace(key)
		if normalizedKey == "" || value <= 0 {
			continue
		}
		result[normalizedKey] += value
	}
	if len(result) == 0 {
		return map[string]int64{}
	}
	return result
}

func clampPercent(value int) int {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}

func normalizeInterval(value string) string {
	switch strings.TrimSpace(value) {
	case domainbilling.IntervalYear:
		return domainbilling.IntervalYear
	case domainbilling.IntervalLifetime:
		return domainbilling.IntervalLifetime
	default:
		return domainbilling.IntervalMonth
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func generateOrderNo() (string, error) {
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", err
	}
	return fmt.Sprintf("pay_%s_%s", time.Now().UTC().Format("20060102150405"), hex.EncodeToString(random[:])), nil
}
