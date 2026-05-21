package billing

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	domainbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/billing"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

type modelIdentityResolverStub struct {
	identity PlatformModelIdentity
}

func (s modelIdentityResolverStub) ResolvePlatformModelIdentity(context.Context, string) (PlatformModelIdentity, error) {
	return s.identity, nil
}

type billingRepositoryStub struct {
	mode                       string
	pricing                    *domainbilling.ModelPricing
	listPricing                []domainbilling.ModelPricing
	requestedPlatformModelName string
}

func (r *billingRepositoryStub) GetBillingMode(context.Context) (string, error) {
	return r.mode, nil
}

func (r *billingRepositoryStub) GetBillingPrepaidAmountNanousd(context.Context) (int64, error) {
	return 0, nil
}

func (r *billingRepositoryStub) GetModelPricing(_ context.Context, platformModelName string) (*domainbilling.ModelPricing, error) {
	r.requestedPlatformModelName = platformModelName
	if r.pricing == nil {
		return nil, repository.ErrNotFound
	}
	return r.pricing, nil
}

func (r *billingRepositoryStub) ListActivePlans(context.Context) ([]domainbilling.Plan, error) {
	panic("not used")
}
func (r *billingRepositoryStub) ListActivePricesByPlanIDs(context.Context, []uint) ([]domainbilling.Price, error) {
	panic("not used")
}
func (r *billingRepositoryStub) GetPriceByID(context.Context, uint) (*domainbilling.Price, error) {
	panic("not used")
}
func (r *billingRepositoryStub) GetPlanByID(context.Context, uint) (*domainbilling.Plan, error) {
	panic("not used")
}
func (r *billingRepositoryStub) ListPlansByIDs(context.Context, []uint) ([]domainbilling.Plan, error) {
	panic("not used")
}
func (r *billingRepositoryStub) GetActivePlanByCode(context.Context, string) (*domainbilling.Plan, error) {
	panic("not used")
}
func (r *billingRepositoryStub) UpdatePlanWithDefaultPrice(context.Context, *domainbilling.Plan, *domainbilling.Price) error {
	panic("not used")
}
func (r *billingRepositoryStub) ListCurrentSubscriptionsByUserIDs(context.Context, []uint, time.Time) ([]domainbilling.Subscription, error) {
	panic("not used")
}
func (r *billingRepositoryStub) ReplaceSubscription(context.Context, *domainbilling.Subscription) error {
	panic("not used")
}
func (r *billingRepositoryStub) CreatePaymentOrder(context.Context, *domainbilling.PaymentOrder) (*domainbilling.PaymentOrder, error) {
	panic("not used")
}
func (r *billingRepositoryStub) UpdatePaymentOrderCheckout(context.Context, string, string, string) error {
	panic("not used")
}
func (r *billingRepositoryStub) GetPaymentOrderByOrderNo(context.Context, string) (*domainbilling.PaymentOrder, error) {
	panic("not used")
}
func (r *billingRepositoryStub) MarkPaymentOrderPaidAndReplaceSubscription(context.Context, string, string, time.Time, *domainbilling.Subscription) (*domainbilling.PaymentOrder, bool, error) {
	panic("not used")
}
func (r *billingRepositoryStub) AddUsage(context.Context, *domainbilling.UsageLedger) error {
	panic("not used")
}
func (r *billingRepositoryStub) AddUsageAndDebitBalance(context.Context, *domainbilling.UsageLedger) error {
	panic("not used")
}
func (r *billingRepositoryStub) AddUsageAndSettleBalance(context.Context, *domainbilling.UsageLedger, *domainbilling.UsageBalanceReservation) error {
	panic("not used")
}
func (r *billingRepositoryStub) ReserveUsageBalance(context.Context, uint, int64, string) (*domainbilling.UsageBalanceReservation, error) {
	panic("not used")
}
func (r *billingRepositoryStub) ReleaseUsageBalanceReservation(context.Context, uint, string, string) error {
	panic("not used")
}
func (r *billingRepositoryStub) GetOrCreateBillingAccount(context.Context, uint) (*domainbilling.BillingAccount, error) {
	panic("not used")
}
func (r *billingRepositoryStub) ListBillingAccountsByUserIDs(context.Context, []uint) ([]domainbilling.BillingAccount, error) {
	panic("not used")
}
func (r *billingRepositoryStub) SetBillingAccountBalance(context.Context, uint, int64, string, string) (*domainbilling.BillingAccount, error) {
	panic("not used")
}
func (r *billingRepositoryStub) MarkPaymentOrderPaidAndCreditBalance(context.Context, string, string, time.Time) (*domainbilling.PaymentOrder, bool, error) {
	panic("not used")
}
func (r *billingRepositoryStub) ListModelPricing(context.Context, string, int, int) ([]domainbilling.ModelPricing, int64, error) {
	return r.listPricing, int64(len(r.listPricing)), nil
}
func (r *billingRepositoryStub) UpsertModelPricing(_ context.Context, item *domainbilling.ModelPricing) (*domainbilling.ModelPricing, error) {
	return item, nil
}
func (r *billingRepositoryStub) ListUsageByUser(context.Context, uint, repository.UsageListFilter, int, int) ([]domainbilling.UsageLedger, int64, error) {
	panic("not used")
}
func (r *billingRepositoryStub) ListUsageLogs(context.Context, repository.UsageLogListFilter, int, int) ([]domainbilling.UsageLedger, int64, error) {
	panic("not used")
}
func (r *billingRepositoryStub) GetUserCreatedAt(context.Context, uint) (time.Time, error) {
	panic("not used")
}
func (r *billingRepositoryStub) ListMonthlyUsageByUser(context.Context, uint, int) ([]domainbilling.UsageMonthlySummary, error) {
	panic("not used")
}
func (r *billingRepositoryStub) ListDailyUsageByUser(context.Context, uint, time.Time, time.Time) ([]domainbilling.UsageDailySummary, error) {
	panic("not used")
}
func (r *billingRepositoryStub) SumBillableNanousd(context.Context, uint, time.Time, time.Time) (int64, error) {
	panic("not used")
}

func TestBuildUsageLedgerSnapshotsModelIdentity(t *testing.T) {
	repo := &billingRepositoryStub{
		mode: "usage",
		pricing: &domainbilling.ModelPricing{
			PlatformModelName:          "gpt-5.5",
			Currency:                   "USD",
			PricingMode:                domainbilling.PricingModeToken,
			InputNanousdPerMTokens:     1_000_000_000,
			OutputNanousdPerMTokens:    2_000_000_000,
			CacheReadNanousdPerMTokens: 200_000_000,
		},
	}
	service := NewService(repo)
	service.SetPlatformModelIdentityResolver(modelIdentityResolverStub{identity: PlatformModelIdentity{
		PlatformModelName: "gpt-5.5",
		ModelVendor:       "openai",
		ModelIcon:         "openai",
	}})

	ledger, err := service.BuildUsageLedger(context.Background(), UsagePricingInput{
		UserID:            1,
		PlatformModelName: "gpt-5.5",
		RoutedBindingCode: "upm_gpt55_20260514",
		UpstreamName:      "premium-channel",
		UpstreamModelName: "gpt-5.5-upstream",
		InputTokens:       1_000_000,
		OutputTokens:      1_000_000,
		ServiceItems: []ServiceUsageInput{{
			ServiceCode:       "context",
			ServiceName:       "上下文处理",
			PlatformModelName: "gpt-5.5",
			InputTokens:       1_000,
			OutputTokens:      1_000,
		}},
	})
	if err != nil {
		t.Fatalf("build usage ledger: %v", err)
	}
	if repo.requestedPlatformModelName != "gpt-5.5" {
		t.Fatalf("expected platform model pricing key, got %q", repo.requestedPlatformModelName)
	}

	var snapshot map[string]interface{}
	if err := json.Unmarshal([]byte(ledger.PricingSnapshotJSON), &snapshot); err != nil {
		t.Fatalf("unmarshal pricing snapshot: %v", err)
	}
	if snapshot["platform_model_name"] != "gpt-5.5" {
		t.Fatalf("expected platform model snapshot, got %#v", snapshot["platform_model_name"])
	}
	if snapshot["model_vendor"] != "openai" || snapshot["model_icon"] != "openai" {
		t.Fatalf("expected vendor/icon snapshot, got vendor=%#v icon=%#v", snapshot["model_vendor"], snapshot["model_icon"])
	}
	if snapshot["upstream_name"] != "premium-channel" {
		t.Fatalf("expected upstream name snapshot, got %#v", snapshot["upstream_name"])
	}
	if snapshot["routed_binding_code"] != "upm_gpt55_20260514" || snapshot["upstream_model_name"] != "gpt-5.5-upstream" {
		t.Fatalf("expected routed binding/upstream snapshot, got routed=%#v upstream_model=%#v", snapshot["routed_binding_code"], snapshot["upstream_model_name"])
	}
	if _, ok := snapshot["billing_multiplier"]; ok {
		t.Fatalf("did not expect billing multiplier snapshot, got %#v", snapshot["billing_multiplier"])
	}
	if snapshot["input_nanousd_per_m_tokens"] != float64(1_000_000_000) || snapshot["base_input_nanousd_per_m_tokens"] != float64(1_000_000_000) {
		t.Fatalf("expected product/base input rates to match, got effective=%#v base=%#v", snapshot["input_nanousd_per_m_tokens"], snapshot["base_input_nanousd_per_m_tokens"])
	}
	if ledger.BilledNanousd != 3_003_000_000 {
		t.Fatalf("expected product-price billing total, got %d", ledger.BilledNanousd)
	}
	serviceItems, ok := snapshot["service_items"].([]interface{})
	if !ok || len(serviceItems) != 1 {
		t.Fatalf("expected one service item snapshot, got %#v", snapshot["service_items"])
	}
	serviceItem, ok := serviceItems[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected service item map, got %#v", serviceItems[0])
	}
	if serviceItem["platform_model_name"] != "gpt-5.5" {
		t.Fatalf("expected service platform model snapshot, got %#v", serviceItem["platform_model_name"])
	}
	if _, ok := serviceItem["billing_multiplier"]; ok {
		t.Fatalf("did not expect service multiplier snapshot, got %#v", serviceItem["billing_multiplier"])
	}
}

func TestBuildUsageLedgerAppliesAnthropicFastModeAndCacheRates(t *testing.T) {
	repo := &billingRepositoryStub{
		mode: "usage",
		pricing: &domainbilling.ModelPricing{
			PlatformModelName:           "claude-opus-4.6",
			Currency:                    "USD",
			PricingMode:                 domainbilling.PricingModeToken,
			InputNanousdPerMTokens:      1_000_000_000,
			OutputNanousdPerMTokens:     5_000_000_000,
			CacheReadNanousdPerMTokens:  200_000_000,
			CacheWriteNanousdPerMTokens: 1_100_000_000,
		},
	}
	service := NewService(repo)

	ledger, err := service.BuildUsageLedger(context.Background(), UsagePricingInput{
		UserID:             1,
		PlatformModelName:  "claude-opus-4.6",
		ProviderProtocol:   "anthropic_messages",
		RequestSpeed:       "fast",
		UsageSpeed:         "fast",
		InputTokens:        1_000,
		CacheReadTokens:    2_000,
		CacheWriteTokens:   7_000,
		CacheWrite5mTokens: 3_000,
		CacheWrite1hTokens: 4_000,
		OutputTokens:       5_000,
	})
	if err != nil {
		t.Fatalf("build usage ledger: %v", err)
	}

	const want = int64(235_950_000)
	if ledger.BilledNanousd != want {
		t.Fatalf("billed nanousd = %d, want %d", ledger.BilledNanousd, want)
	}
	if ledger.UsageSpeed != "fast" || ledger.CacheWrite5mTokens != 3_000 || ledger.CacheWrite1hTokens != 4_000 {
		t.Fatalf("unexpected ledger metadata: %+v", ledger)
	}

	var snapshot map[string]interface{}
	if err := json.Unmarshal([]byte(ledger.PricingSnapshotJSON), &snapshot); err != nil {
		t.Fatalf("unmarshal pricing snapshot: %v", err)
	}
	if snapshot["fast_mode"] != true || snapshot["rate_multiplier"] != float64(6) {
		t.Fatalf("expected fast mode snapshot, got %#v", snapshot)
	}
	if snapshot["cache_read_nanousd_per_m_tokens"] != float64(1_200_000_000) {
		t.Fatalf("cache read rate = %#v, want 1200000000", snapshot["cache_read_nanousd_per_m_tokens"])
	}
	if snapshot["cache_write_5m_nanousd_per_m_tokens"] != float64(8_250_000_000) {
		t.Fatalf("cache write 5m rate = %#v, want 8250000000", snapshot["cache_write_5m_nanousd_per_m_tokens"])
	}
	if snapshot["cache_write_1h_nanousd_per_m_tokens"] != float64(13_200_000_000) {
		t.Fatalf("cache write 1h rate = %#v, want 13200000000", snapshot["cache_write_1h_nanousd_per_m_tokens"])
	}
}

func TestBuildUsageLedgerAppliesAnthropicFastModeAndCacheRatesToServiceItems(t *testing.T) {
	repo := &billingRepositoryStub{
		mode: "usage",
		pricing: &domainbilling.ModelPricing{
			PlatformModelName:           "claude-opus-4.6",
			Currency:                    "USD",
			PricingMode:                 domainbilling.PricingModeToken,
			InputNanousdPerMTokens:      1_000_000_000,
			OutputNanousdPerMTokens:     5_000_000_000,
			CacheReadNanousdPerMTokens:  200_000_000,
			CacheWriteNanousdPerMTokens: 1_100_000_000,
		},
	}
	service := NewService(repo)

	ledger, err := service.BuildUsageLedger(context.Background(), UsagePricingInput{
		UserID:            1,
		PlatformModelName: "claude-opus-4.6",
		ProviderProtocol:  "anthropic_messages",
		ServiceOnly:       true,
		ServiceItems: []ServiceUsageInput{{
			ServiceCode:        "compact",
			ServiceName:        "上下文压缩",
			PlatformModelName:  "claude-opus-4.6",
			ProviderProtocol:   "anthropic_messages",
			UsageSpeed:         "fast",
			InputTokens:        1_000,
			CacheReadTokens:    2_000,
			CacheWriteTokens:   7_000,
			CacheWrite5mTokens: 3_000,
			CacheWrite1hTokens: 4_000,
			OutputTokens:       5_000,
			CallCount:          1,
		}},
	})
	if err != nil {
		t.Fatalf("build usage ledger: %v", err)
	}

	const want = int64(235_950_000)
	if ledger.BilledNanousd != want {
		t.Fatalf("billed nanousd = %d, want %d", ledger.BilledNanousd, want)
	}

	var snapshot map[string]interface{}
	if err := json.Unmarshal([]byte(ledger.PricingSnapshotJSON), &snapshot); err != nil {
		t.Fatalf("unmarshal pricing snapshot: %v", err)
	}
	serviceItems, ok := snapshot["service_items"].([]interface{})
	if !ok || len(serviceItems) != 1 {
		t.Fatalf("expected one service item snapshot, got %#v", snapshot["service_items"])
	}
	item, ok := serviceItems[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected service item map, got %#v", serviceItems[0])
	}
	if item["fast_mode"] != true || item["rate_multiplier"] != float64(6) {
		t.Fatalf("expected service fast mode snapshot, got %#v", item)
	}
	if item["cache_write_5m_nanousd_per_m_tokens"] != float64(8_250_000_000) {
		t.Fatalf("service cache write 5m rate = %#v, want 8250000000", item["cache_write_5m_nanousd_per_m_tokens"])
	}
	if item["cache_write_1h_nanousd_per_m_tokens"] != float64(13_200_000_000) {
		t.Fatalf("service cache write 1h rate = %#v, want 13200000000", item["cache_write_1h_nanousd_per_m_tokens"])
	}
	if item["cache_write_billed_nanousd"] != float64(77_550_000) {
		t.Fatalf("service cache write billed = %#v, want 77550000", item["cache_write_billed_nanousd"])
	}
}

func TestBuildUsageLedgerAppliesOpenAIServiceTierRates(t *testing.T) {
	repo := &billingRepositoryStub{
		mode: "usage",
		pricing: &domainbilling.ModelPricing{
			PlatformModelName:           "gpt-5.5",
			Currency:                    "USD",
			PricingMode:                 domainbilling.PricingModeToken,
			InputNanousdPerMTokens:      1_000_000_000,
			OutputNanousdPerMTokens:     5_000_000_000,
			CacheReadNanousdPerMTokens:  200_000_000,
			CacheWriteNanousdPerMTokens: 100_000_000,
		},
	}
	service := NewService(repo)

	ledger, err := service.BuildUsageLedger(context.Background(), UsagePricingInput{
		UserID:             1,
		PlatformModelName:  "chat-pro",
		UpstreamModelName:  "gpt-5.5-20260516",
		ProviderProtocol:   "openai_responses",
		RequestServiceTier: "priority",
		UsageServiceTier:   "priority",
		InputTokens:        1_000,
		CacheReadTokens:    500,
		CacheWriteTokens:   100,
		OutputTokens:       2_000,
	})
	if err != nil {
		t.Fatalf("build usage ledger: %v", err)
	}

	const want = int64(27_775_000)
	if ledger.BilledNanousd != want {
		t.Fatalf("billed nanousd = %d, want %d", ledger.BilledNanousd, want)
	}
	if ledger.ServiceTier != "priority" {
		t.Fatalf("service tier = %q, want priority", ledger.ServiceTier)
	}

	var snapshot map[string]interface{}
	if err := json.Unmarshal([]byte(ledger.PricingSnapshotJSON), &snapshot); err != nil {
		t.Fatalf("unmarshal pricing snapshot: %v", err)
	}
	if snapshot["billing_service_tier"] != "priority" || snapshot["rate_multiplier"] != 2.5 {
		t.Fatalf("expected priority multiplier snapshot, got %#v", snapshot)
	}
	if snapshot["input_nanousd_per_m_tokens"] != float64(2_500_000_000) {
		t.Fatalf("input rate = %#v, want 2500000000", snapshot["input_nanousd_per_m_tokens"])
	}
}

func TestBuildUsageLedgerUsesActualOpenAIServiceTierOverRequested(t *testing.T) {
	repo := &billingRepositoryStub{
		mode: "usage",
		pricing: &domainbilling.ModelPricing{
			PlatformModelName:          "gpt-5",
			Currency:                   "USD",
			PricingMode:                domainbilling.PricingModeToken,
			InputNanousdPerMTokens:     1_000_000_000,
			OutputNanousdPerMTokens:    2_000_000_000,
			CacheReadNanousdPerMTokens: 100_000_000,
		},
	}
	service := NewService(repo)

	ledger, err := service.BuildUsageLedger(context.Background(), UsagePricingInput{
		UserID:             1,
		PlatformModelName:  "gpt-5",
		ProviderProtocol:   "openai_chat_completions",
		RequestServiceTier: "priority",
		UsageServiceTier:   "default",
		InputTokens:        1_000,
		OutputTokens:       1_000,
	})
	if err != nil {
		t.Fatalf("build usage ledger: %v", err)
	}
	if ledger.BilledNanousd != 3_000_000 {
		t.Fatalf("billed nanousd = %d, want 3000000", ledger.BilledNanousd)
	}

	var snapshot map[string]interface{}
	if err := json.Unmarshal([]byte(ledger.PricingSnapshotJSON), &snapshot); err != nil {
		t.Fatalf("unmarshal pricing snapshot: %v", err)
	}
	if snapshot["request_service_tier"] != "priority" || snapshot["usage_service_tier"] != "default" || snapshot["billing_service_tier"] != "default" || snapshot["rate_multiplier"] != float64(1) {
		t.Fatalf("expected actual default service tier snapshot, got %#v", snapshot)
	}
}

func TestBuildUsageLedgerAppliesOpenAIFlexRate(t *testing.T) {
	repo := &billingRepositoryStub{
		mode: "usage",
		pricing: &domainbilling.ModelPricing{
			PlatformModelName:       "gpt-5",
			Currency:                "USD",
			PricingMode:             domainbilling.PricingModeToken,
			InputNanousdPerMTokens:  1_000_000_000,
			OutputNanousdPerMTokens: 2_000_000_000,
		},
	}
	service := NewService(repo)

	ledger, err := service.BuildUsageLedger(context.Background(), UsagePricingInput{
		UserID:             1,
		PlatformModelName:  "gpt-5",
		ProviderProtocol:   "openai_responses",
		RequestServiceTier: "flex",
		UsageServiceTier:   "flex",
		InputTokens:        1_000,
		OutputTokens:       1_000,
	})
	if err != nil {
		t.Fatalf("build usage ledger: %v", err)
	}
	if ledger.BilledNanousd != 1_500_000 {
		t.Fatalf("billed nanousd = %d, want 1500000", ledger.BilledNanousd)
	}

	var snapshot map[string]interface{}
	if err := json.Unmarshal([]byte(ledger.PricingSnapshotJSON), &snapshot); err != nil {
		t.Fatalf("unmarshal pricing snapshot: %v", err)
	}
	if snapshot["billing_service_tier"] != "flex" || snapshot["rate_multiplier"] != 0.5 {
		t.Fatalf("expected flex multiplier snapshot, got %#v", snapshot)
	}
}

func TestBuildUsageLedgerDefaultsOpenAIServiceTierWhenUpstreamDoesNotReturnIt(t *testing.T) {
	repo := &billingRepositoryStub{
		mode: "usage",
		pricing: &domainbilling.ModelPricing{
			PlatformModelName:       "gpt-5",
			Currency:                "USD",
			PricingMode:             domainbilling.PricingModeToken,
			InputNanousdPerMTokens:  1_000_000_000,
			OutputNanousdPerMTokens: 2_000_000_000,
		},
	}
	service := NewService(repo)

	ledger, err := service.BuildUsageLedger(context.Background(), UsagePricingInput{
		UserID:             1,
		PlatformModelName:  "gpt-5",
		ProviderProtocol:   "openai_responses",
		RequestServiceTier: "priority",
		InputTokens:        1_000,
		OutputTokens:       1_000,
	})
	if err != nil {
		t.Fatalf("build usage ledger: %v", err)
	}
	if ledger.BilledNanousd != 3_000_000 {
		t.Fatalf("billed nanousd = %d, want 3000000", ledger.BilledNanousd)
	}
	if ledger.ServiceTier != "default" {
		t.Fatalf("service tier = %q, want default", ledger.ServiceTier)
	}

	var snapshot map[string]interface{}
	if err := json.Unmarshal([]byte(ledger.PricingSnapshotJSON), &snapshot); err != nil {
		t.Fatalf("unmarshal pricing snapshot: %v", err)
	}
	if snapshot["request_service_tier"] != "priority" || snapshot["usage_service_tier"] != "" || snapshot["billing_service_tier"] != "default" || snapshot["rate_multiplier"] != float64(1) {
		t.Fatalf("expected missing upstream service tier to default to 1x, got %#v", snapshot)
	}
}

func TestBuildUsageLedgerIgnoresUnsupportedOpenAIServiceTier(t *testing.T) {
	repo := &billingRepositoryStub{
		mode: "usage",
		pricing: &domainbilling.ModelPricing{
			PlatformModelName:       "gpt-5",
			Currency:                "USD",
			PricingMode:             domainbilling.PricingModeToken,
			InputNanousdPerMTokens:  1_000_000_000,
			OutputNanousdPerMTokens: 2_000_000_000,
		},
	}
	service := NewService(repo)

	ledger, err := service.BuildUsageLedger(context.Background(), UsagePricingInput{
		UserID:             1,
		PlatformModelName:  "gpt-5",
		ProviderProtocol:   "openai_responses",
		RequestServiceTier: "auto",
		UsageServiceTier:   "scale",
		InputTokens:        1_000,
		OutputTokens:       1_000,
	})
	if err != nil {
		t.Fatalf("build usage ledger: %v", err)
	}
	if ledger.BilledNanousd != 3_000_000 || ledger.ServiceTier != "default" {
		t.Fatalf("unexpected ledger tier billing: %+v", ledger)
	}

	var snapshot map[string]interface{}
	if err := json.Unmarshal([]byte(ledger.PricingSnapshotJSON), &snapshot); err != nil {
		t.Fatalf("unmarshal pricing snapshot: %v", err)
	}
	if snapshot["request_service_tier"] != "" || snapshot["usage_service_tier"] != "" || snapshot["billing_service_tier"] != "default" || snapshot["rate_multiplier"] != float64(1) {
		t.Fatalf("expected unsupported service tiers to be ignored, got %#v", snapshot)
	}
}

func TestListModelPricingReturnsModelDisplayIdentity(t *testing.T) {
	repo := &billingRepositoryStub{
		listPricing: []domainbilling.ModelPricing{{
			PlatformModelName: "gpt-5.5",
			Currency:          "USD",
			PricingMode:       domainbilling.PricingModeToken,
		}},
	}
	service := NewService(repo)
	service.SetPlatformModelIdentityResolver(modelIdentityResolverStub{identity: PlatformModelIdentity{
		PlatformModelName: "gpt-5.5",
		ModelVendor:       "openai",
		ModelIcon:         "openai",
	}})

	items, total, err := service.ListModelPricing(context.Background(), "", 1, 25)
	if err != nil {
		t.Fatalf("list model pricing: %v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("expected one item, got total=%d len=%d", total, len(items))
	}
	item := items[0]
	if item.PlatformModelName != "gpt-5.5" {
		t.Fatalf("expected platform model pricing identity, got %#v", item)
	}
	if item.ModelVendor != "openai" || item.ModelIcon != "openai" {
		t.Fatalf("expected model metadata identity, got %#v", item)
	}
}
