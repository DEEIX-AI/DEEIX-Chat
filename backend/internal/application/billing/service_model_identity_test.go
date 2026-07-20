package billing

import (
	"context"
	"encoding/json"
	"errors"
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

func TestUpstreamUsageSnapshotReturnsEmptyObjectWhenRawUsageIsMissing(t *testing.T) {
	snapshot, ok := upstreamUsageSnapshot(UsagePricingInput{
		InputTokens:  10,
		OutputTokens: 20,
	}).(map[string]interface{})
	if !ok || len(snapshot) != 0 {
		t.Fatalf("expected empty upstream usage snapshot, got %#v", snapshot)
	}
}

func TestUpdatePlanRejectsUnknownPermissionGroup(t *testing.T) {
	repo := &billingRepositoryStub{
		plans: []domainbilling.Plan{{ID: 1, Code: "pro", Name: "Pro"}},
	}
	service := NewService(repo)
	groupID := uint(99)
	service.SetPermissionGroupLookup(permissionGroupLookupStub{})

	_, err := service.UpdatePlan(context.Background(), 1, PlanUpdateInput{
		Name:              "Pro",
		PermissionGroupID: &groupID,
	})
	if !errors.Is(err, ErrInvalidPermissionGroup) {
		t.Fatalf("expected invalid permission group error, got %v", err)
	}
}

func TestUpdatePlanDefaultsPermissionGroup(t *testing.T) {
	defaultGroupID := uint(7)
	repo := &billingRepositoryStub{
		plans: []domainbilling.Plan{{ID: 1, Code: "pro", Name: "Pro"}},
	}
	service := NewService(repo)
	service.SetPermissionGroupLookup(permissionGroupLookupStub{
		validIDs:   map[uint]struct{}{defaultGroupID: {}},
		defaultIDs: []uint{defaultGroupID},
	})

	view, err := service.UpdatePlan(context.Background(), 1, PlanUpdateInput{
		Name: "Pro",
	})
	if err != nil {
		t.Fatalf("UpdatePlan() error = %v", err)
	}
	if view.PermissionGroupID == nil || *view.PermissionGroupID != defaultGroupID {
		t.Fatalf("PermissionGroupID = %v, want %d", view.PermissionGroupID, defaultGroupID)
	}
	if repo.updatedPlan == nil || repo.updatedPlan.PermissionGroupID == nil || *repo.updatedPlan.PermissionGroupID != defaultGroupID {
		t.Fatalf("updated plan PermissionGroupID = %v, want %d", repo.updatedPlan, defaultGroupID)
	}
}

func TestUpdatePlanRejectsMissingDefaultPermissionGroup(t *testing.T) {
	repo := &billingRepositoryStub{
		plans: []domainbilling.Plan{{ID: 1, Code: "pro", Name: "Pro"}},
	}
	service := NewService(repo)
	service.SetPermissionGroupLookup(permissionGroupLookupStub{})

	_, err := service.UpdatePlan(context.Background(), 1, PlanUpdateInput{
		Name: "Pro",
	})
	if !errors.Is(err, ErrInvalidPermissionGroup) {
		t.Fatalf("expected invalid permission group error, got %v", err)
	}
}

type permissionGroupLookupStub struct {
	validIDs   map[uint]struct{}
	defaultIDs []uint
}

func (s permissionGroupLookupStub) PermissionGroupExists(_ context.Context, id uint) (bool, error) {
	_, ok := s.validIDs[id]
	return ok, nil
}

func (s permissionGroupLookupStub) ListDefaultGroupIDs(context.Context) ([]uint, error) {
	return s.defaultIDs, nil
}

type billingRepositoryStub struct {
	mode                       string
	pricing                    *domainbilling.ModelPricing
	listPricing                []domainbilling.ModelPricing
	plans                      []domainbilling.Plan
	prices                     []domainbilling.Price
	subscriptions              []domainbilling.Subscription
	prepaidNanousd             int64
	nativeToolBillingEnabled   bool
	nativeToolPricingJSON      string
	requestedPlatformModelName string
	replacedSubscription       *domainbilling.Subscription
	reservationRequest         *domainbilling.UsageBalanceReservationRequest
	reservationErr             error
	periodUsageSettled         bool
	usageSettled               bool
	usageAdded                 bool
	periodStartAt              time.Time
	periodEndAt                time.Time
	periodCreditNanousd        int64
	updatedPlan                *domainbilling.Plan
	updatedPrice               *domainbilling.Price
}

func (r *billingRepositoryStub) GetBillingMode(context.Context) (string, error) {
	return r.mode, nil
}

func (r *billingRepositoryStub) GetBillingPrepaidAmountNanousd(context.Context) (int64, error) {
	return r.prepaidNanousd, nil
}

func (r *billingRepositoryStub) GetNativeToolBillingEnabled(context.Context) (bool, error) {
	return r.nativeToolBillingEnabled, nil
}

func (r *billingRepositoryStub) GetNativeToolPricingJSON(context.Context) (string, error) {
	return r.nativeToolPricingJSON, nil
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
func (r *billingRepositoryStub) GetPriceByID(_ context.Context, id uint) (*domainbilling.Price, error) {
	for _, item := range r.prices {
		if item.ID == id {
			return &item, nil
		}
	}
	return nil, repository.ErrNotFound
}
func (r *billingRepositoryStub) GetPlanByID(_ context.Context, id uint) (*domainbilling.Plan, error) {
	for _, item := range r.plans {
		if item.ID == id {
			return &item, nil
		}
	}
	return nil, repository.ErrNotFound
}
func (r *billingRepositoryStub) ListPlansByIDs(_ context.Context, planIDs []uint) ([]domainbilling.Plan, error) {
	if len(planIDs) == 0 {
		return []domainbilling.Plan{}, nil
	}
	allowed := make(map[uint]struct{}, len(planIDs))
	for _, id := range planIDs {
		allowed[id] = struct{}{}
	}
	results := make([]domainbilling.Plan, 0, len(planIDs))
	for _, item := range r.plans {
		if _, ok := allowed[item.ID]; ok {
			results = append(results, item)
		}
	}
	return results, nil
}
func (r *billingRepositoryStub) GetActivePlanByCode(context.Context, string) (*domainbilling.Plan, error) {
	panic("not used")
}
func (r *billingRepositoryStub) UpdatePlanWithDefaultPrice(_ context.Context, plan *domainbilling.Plan, price *domainbilling.Price) error {
	r.updatedPlan = plan
	r.updatedPrice = price
	return nil
}
func (r *billingRepositoryStub) ListCurrentSubscriptionsByUserIDs(context.Context, []uint, time.Time) ([]domainbilling.Subscription, error) {
	panic("not used")
}
func (r *billingRepositoryStub) ListSubscriptionEntitlementsByUserIDs(_ context.Context, userIDs []uint, now time.Time) ([]domainbilling.Subscription, error) {
	allowed := make(map[uint]struct{}, len(userIDs))
	for _, id := range userIDs {
		allowed[id] = struct{}{}
	}
	results := make([]domainbilling.Subscription, 0, len(r.subscriptions))
	for _, item := range r.subscriptions {
		if _, ok := allowed[item.UserID]; !ok {
			continue
		}
		if item.CurrentPeriodEndAt != nil && !item.CurrentPeriodEndAt.After(now) {
			continue
		}
		results = append(results, item)
	}
	return results, nil
}
func (r *billingRepositoryStub) ReplaceSubscription(_ context.Context, item *domainbilling.Subscription) error {
	r.replacedSubscription = item
	return nil
}
func (r *billingRepositoryStub) CreatePaymentOrder(_ context.Context, item *domainbilling.PaymentOrder) (*domainbilling.PaymentOrder, error) {
	if item.ID == 0 {
		item.ID = 1
	}
	return item, nil
}
func (r *billingRepositoryStub) UpdatePaymentOrderCheckout(context.Context, string, string, string) error {
	panic("not used")
}
func (r *billingRepositoryStub) GetPaymentOrderByOrderNo(context.Context, string) (*domainbilling.PaymentOrder, error) {
	panic("not used")
}
func (r *billingRepositoryStub) MarkPaymentOrderPaidAndGrantSubscription(context.Context, string, string, time.Time, *domainbilling.Subscription) (*domainbilling.PaymentOrder, bool, error) {
	panic("not used")
}
func (r *billingRepositoryStub) AddUsage(context.Context, *domainbilling.UsageLedger) error {
	r.usageAdded = true
	return nil
}
func (r *billingRepositoryStub) AddUsageAndSettleBalance(context.Context, *domainbilling.UsageLedger, *domainbilling.UsageBalanceReservation) error {
	r.usageSettled = true
	return nil
}
func (r *billingRepositoryStub) AddPeriodUsageAndSettleOverage(_ context.Context, _ *domainbilling.UsageLedger, periodStart time.Time, periodEnd time.Time, periodCreditNanousd int64, _ *domainbilling.UsageBalanceReservation) error {
	r.periodUsageSettled = true
	r.periodStartAt = periodStart
	r.periodEndAt = periodEnd
	r.periodCreditNanousd = periodCreditNanousd
	return nil
}
func (r *billingRepositoryStub) ReserveUsageBalance(_ context.Context, input domainbilling.UsageBalanceReservationRequest) (*domainbilling.UsageBalanceReservation, error) {
	r.reservationRequest = &input
	if r.reservationErr != nil {
		return nil, r.reservationErr
	}
	return &domainbilling.UsageBalanceReservation{
		UserID: input.UserID,
		RefNo:  input.RefNo,
		Mode:   input.Mode,
	}, nil
}
func (r *billingRepositoryStub) RenewUsageBalanceReservation(context.Context, uint, string) error {
	panic("not used")
}
func (r *billingRepositoryStub) ReleaseUsageBalanceReservation(context.Context, uint, string) error {
	panic("not used")
}
func (r *billingRepositoryStub) MarkUsageReservationReconciliationRequired(context.Context, uint, string, string) error {
	panic("not used")
}
func (r *billingRepositoryStub) GetOrCreateBillingAccount(_ context.Context, userID uint) (*domainbilling.BillingAccount, error) {
	return &domainbilling.BillingAccount{UserID: userID, Currency: "USD", Status: "active"}, nil
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
func (r *billingRepositoryStub) ListRedemptionCodes(context.Context, repository.RedemptionCodeListFilter, int, int) ([]domainbilling.RedemptionCode, int64, error) {
	panic("not used")
}
func (r *billingRepositoryStub) GetRedemptionCodeByID(context.Context, uint) (*domainbilling.RedemptionCode, error) {
	panic("not used")
}
func (r *billingRepositoryStub) CreateRedemptionCode(context.Context, *domainbilling.RedemptionCode) (*domainbilling.RedemptionCode, error) {
	panic("not used")
}
func (r *billingRepositoryStub) PatchRedemptionCode(context.Context, uint, repository.RedemptionCodePatch) (*domainbilling.RedemptionCode, error) {
	panic("not used")
}
func (r *billingRepositoryStub) DeleteRedemptionCode(context.Context, uint) error {
	panic("not used")
}
func (r *billingRepositoryStub) RedeemCode(context.Context, repository.RedemptionApplyInput) (*repository.RedemptionApplyResult, error) {
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
func (r *billingRepositoryStub) ListPaymentOrders(context.Context, repository.PaymentOrderListFilter, int, int) ([]domainbilling.PaymentOrder, int64, error) {
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
	return 0, nil
}

func TestRecordUsageWithAuthorizationUsesBillingAtForPeriod(t *testing.T) {
	billingAt := time.Date(2026, 6, 30, 23, 59, 58, 0, time.UTC)
	usageDate := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	endAt := billingAt.Add(24 * time.Hour)
	repo := &billingRepositoryStub{
		mode: "period",
		plans: []domainbilling.Plan{{
			ID:                  2,
			Code:                "pro",
			PeriodCreditNanousd: 1000,
			IsActive:            true,
		}},
		subscriptions: []domainbilling.Subscription{{
			ID:                   10,
			UserID:               1,
			PlanID:               2,
			Status:               "active",
			CurrentPeriodStartAt: billingAt.Add(-24 * time.Hour),
			CurrentPeriodEndAt:   &endAt,
		}},
	}
	service := NewService(repo)

	err := service.RecordUsageWithAuthorization(context.Background(), &domainbilling.UsageLedger{
		UserID:              1,
		PlatformModelName:   "gpt-test",
		BillingAt:           billingAt,
		UsageDate:           usageDate,
		BilledCurrency:      "USD",
		BilledNanousd:       100,
		PricingSnapshotJSON: `{}`,
	}, nil)
	if err != nil {
		t.Fatalf("RecordUsageWithAuthorization() error = %v", err)
	}
	if !repo.periodUsageSettled {
		t.Fatalf("period usage was not settled")
	}
	if got, want := repo.periodStartAt.Format("2006-01-02"), "2026-06-01"; got != want {
		t.Fatalf("period start = %s, want %s", got, want)
	}
	if repo.periodCreditNanousd != 1000 {
		t.Fatalf("period credit = %d, want 1000", repo.periodCreditNanousd)
	}
}

func TestUsageAuthorizationKeepsSelfModeAfterAdminModeChange(t *testing.T) {
	repo := &billingRepositoryStub{
		mode: "self",
		pricing: &domainbilling.ModelPricing{
			PlatformModelName:       "gpt-test",
			Currency:                "USD",
			InputNanousdPerMTokens:  1_000_000_000,
			OutputNanousdPerMTokens: 1_000_000_000,
		},
	}
	service := NewService(repo)
	authorization, err := service.AuthorizeUsage(context.Background(), 1, "gpt-test", "run_self_snapshot")
	if err != nil {
		t.Fatalf("AuthorizeUsage() error = %v", err)
	}
	if authorization == nil || authorization.Mode != "self" || authorization.Reservation != nil {
		t.Fatalf("authorization = %+v, want self mode without reservation", authorization)
	}

	repo.mode = "usage"
	ledger, err := service.BuildUsageLedger(context.Background(), UsagePricingInput{
		Authorization:     authorization,
		UserID:            1,
		PlatformModelName: "gpt-test",
		InputTokens:       100,
		OutputTokens:      100,
		BillingAt:         time.Now(),
	})
	if err != nil {
		t.Fatalf("BuildUsageLedger() error = %v", err)
	}
	if ledger.BilledNanousd != 0 {
		t.Fatalf("ledger billed nanousd = %d, want 0 from self-mode snapshot", ledger.BilledNanousd)
	}
	if err = service.RecordUsageWithAuthorization(context.Background(), ledger, authorization); err != nil {
		t.Fatalf("RecordUsageWithAuthorization() error = %v", err)
	}
	if !repo.usageAdded || repo.usageSettled || repo.periodUsageSettled {
		t.Fatalf("usage persistence state = %+v", repo)
	}
}

func TestAuthorizeUsageMapsConcurrentReservationLimit(t *testing.T) {
	repo := &billingRepositoryStub{
		mode:           "usage",
		reservationErr: repository.ErrUsageReservationLimitExceeded,
		pricing: &domainbilling.ModelPricing{
			PlatformModelName:       "gpt-test",
			InputNanousdPerMTokens:  1,
			OutputNanousdPerMTokens: 1,
		},
	}
	service := NewService(repo)

	_, err := service.AuthorizeUsage(context.Background(), 1, "gpt-test", "run_limit")
	if !errors.Is(err, ErrUsageConcurrencyLimitExceeded) {
		t.Fatalf("AuthorizeUsage() error = %v, want ErrUsageConcurrencyLimitExceeded", err)
	}
}

func TestBuildUsageLedgerRejectsMissingPaidModelPricing(t *testing.T) {
	service := NewService(&billingRepositoryStub{mode: "usage"})

	_, err := service.BuildUsageLedger(context.Background(), UsagePricingInput{
		Authorization:     &domainbilling.UsageAuthorization{Mode: "usage"},
		UserID:            1,
		PlatformModelName: "deleted-model",
		InputTokens:       100,
		BillingAt:         time.Now(),
	})
	if !errors.Is(err, ErrModelPricingRequired) {
		t.Fatalf("BuildUsageLedger() error = %v, want ErrModelPricingRequired", err)
	}
}

func TestBuildUsageLedgerRejectsMissingBasicServiceModelPricing(t *testing.T) {
	service := NewService(&billingRepositoryStub{mode: "usage"})

	_, err := service.BuildUsageLedger(context.Background(), UsagePricingInput{
		Authorization:     &domainbilling.UsageAuthorization{Mode: "usage"},
		UserID:            1,
		ConversationID:    2,
		PlatformModelName: "deleted-model",
		ServiceOnly:       true,
		ServiceItems: []ServiceUsageInput{{
			ServiceCode:       "title",
			PlatformModelName: "deleted-model",
			InputTokens:       100,
		}},
		BillingAt: time.Now(),
	})
	if !errors.Is(err, ErrModelPricingRequired) {
		t.Fatalf("BuildUsageLedger() error = %v, want ErrModelPricingRequired", err)
	}
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
		UsageSource:       "estimated",
		RawUsageJSON:      `{"input_tokens":1000000,"output_tokens":1000000,"vendor_extra":"kept"}`,
		ServerSideToolUsage: map[string]int64{
			"web_search": 2,
			"ignored":    0,
		},
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
	if snapshot["usage_source"] != "estimated" {
		t.Fatalf("expected usage source snapshot, got %#v", snapshot["usage_source"])
	}
	if snapshot["routed_binding_code"] != "upm_gpt55_20260514" || snapshot["upstream_model_name"] != "gpt-5.5-upstream" {
		t.Fatalf("expected routed binding/upstream snapshot, got routed=%#v upstream_model=%#v", snapshot["routed_binding_code"], snapshot["upstream_model_name"])
	}
	upstreamUsage, ok := snapshot["upstream_usage"].(map[string]interface{})
	if !ok || upstreamUsage["vendor_extra"] != "kept" || upstreamUsage["input_tokens"] != float64(1_000_000) {
		t.Fatalf("expected raw upstream usage snapshot, got %#v", snapshot["upstream_usage"])
	}
	if _, ok := snapshot["billing_multiplier"]; ok {
		t.Fatalf("did not expect billing multiplier snapshot, got %#v", snapshot["billing_multiplier"])
	}
	serverSideToolUsage, ok := snapshot["server_side_tool_usage"].(map[string]interface{})
	if !ok || serverSideToolUsage["web_search"] != float64(2) {
		t.Fatalf("expected server-side tool usage snapshot, got %#v", snapshot["server_side_tool_usage"])
	}
	if _, ok := serverSideToolUsage["ignored"]; ok {
		t.Fatalf("expected empty server-side tool usage entries to be removed, got %#v", serverSideToolUsage)
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

func TestBuildUsageLedgerBillsNativeToolDefaultsWhenEnabled(t *testing.T) {
	repo := &billingRepositoryStub{
		mode:                     "usage",
		nativeToolBillingEnabled: true,
		pricing: &domainbilling.ModelPricing{
			PlatformModelName: "grok-4.3",
			Currency:          "USD",
			PricingMode:       domainbilling.PricingModeToken,
		},
	}
	service := NewService(repo)

	ledger, err := service.BuildUsageLedger(context.Background(), UsagePricingInput{
		UserID:            1,
		PlatformModelName: "grok-4.3",
		ProviderProtocol:  "xai_responses",
		ServerSideToolUsage: map[string]int64{
			"web_search":         2,
			"x_search":           1,
			"code_interpreter":   1,
			"attachment_search":  1,
			"file_search":        1,
			"collections_search": 1,
			"unknown":            3,
		},
	})
	if err != nil {
		t.Fatalf("build usage ledger: %v", err)
	}
	if ledger.BilledNanousd != 35_000_000 {
		t.Fatalf("expected native tool billing total, got %d", ledger.BilledNanousd)
	}

	var snapshot map[string]interface{}
	if err := json.Unmarshal([]byte(ledger.PricingSnapshotJSON), &snapshot); err != nil {
		t.Fatalf("unmarshal pricing snapshot: %v", err)
	}
	if snapshot["native_tool_billing_enabled"] != true || snapshot["native_tool_billed_nanousd"] != float64(35_000_000) {
		t.Fatalf("expected native tool billing snapshot, got %#v", snapshot)
	}
	serviceItems, ok := snapshot["service_items"].([]interface{})
	if !ok || len(serviceItems) != 6 {
		t.Fatalf("expected six native tool service items, got %#v", snapshot["service_items"])
	}
}

func TestBuildUsageLedgerUsesNativeToolPricingOverrides(t *testing.T) {
	repo := &billingRepositoryStub{
		mode:                     "usage",
		nativeToolBillingEnabled: true,
		nativeToolPricingJSON:    `{"xai.web_search":{"priceNanousd":123000000,"unit":"call","priceLabel":"","billable":true}}`,
		pricing: &domainbilling.ModelPricing{
			PlatformModelName: "grok-4.3",
			Currency:          "USD",
			PricingMode:       domainbilling.PricingModeToken,
		},
	}
	service := NewService(repo)

	ledger, err := service.BuildUsageLedger(context.Background(), UsagePricingInput{
		UserID:            1,
		PlatformModelName: "grok-4.3",
		ProviderProtocol:  "xai_responses",
		ServerSideToolUsage: map[string]int64{
			"web_search": 2,
		},
	})
	if err != nil {
		t.Fatalf("build usage ledger: %v", err)
	}
	if ledger.BilledNanousd != 246_000_000 {
		t.Fatalf("expected native tool override billing total, got %d", ledger.BilledNanousd)
	}

	var snapshot map[string]interface{}
	if err := json.Unmarshal([]byte(ledger.PricingSnapshotJSON), &snapshot); err != nil {
		t.Fatalf("unmarshal pricing snapshot: %v", err)
	}
	if snapshot["native_tool_pricing_source"] != "admin_configured" {
		t.Fatalf("expected admin native tool pricing source, got %#v", snapshot["native_tool_pricing_source"])
	}
}

func TestBuildUsageLedgerBillsOpenAIWebSearchPreviewByModelFamily(t *testing.T) {
	cases := []struct {
		name              string
		platformModelName string
		upstreamModelName string
		wantNanousd       int64
	}{
		{name: "gpt-5 model", platformModelName: "gpt-5.4", wantNanousd: 25_000_000},
		{name: "gpt-4o model", platformModelName: "gpt-4o-mini", upstreamModelName: "gpt-4o-mini-search-preview", wantNanousd: 25_000_000},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			modelName := firstNonEmpty(tc.platformModelName, tc.upstreamModelName)
			repo := &billingRepositoryStub{
				mode:                     "usage",
				nativeToolBillingEnabled: true,
				pricing: &domainbilling.ModelPricing{
					PlatformModelName: modelName,
					Currency:          "USD",
					PricingMode:       domainbilling.PricingModeToken,
				},
			}
			service := NewService(repo)

			ledger, err := service.BuildUsageLedger(context.Background(), UsagePricingInput{
				UserID:            1,
				PlatformModelName: tc.platformModelName,
				UpstreamModelName: tc.upstreamModelName,
				ProviderProtocol:  "openai_responses",
				ServerSideToolUsage: map[string]int64{
					"web_search_preview": 1,
				},
			})
			if err != nil {
				t.Fatalf("build usage ledger: %v", err)
			}
			if ledger.BilledNanousd != tc.wantNanousd {
				t.Fatalf("expected %d billed nanousd, got %d", tc.wantNanousd, ledger.BilledNanousd)
			}
		})
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
