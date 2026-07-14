package billing

import (
	"context"
	"errors"
	"testing"
	"time"

	domainbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/billing"
)

func TestListCurrentSubscriptionSnapshotsExtendsContiguousSamePlan(t *testing.T) {
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	day := 24 * time.Hour
	firstEnd := now.Add(30 * day)
	secondEnd := now.Add(60 * day)
	service := NewService(&billingRepositoryStub{
		plans: []domainbilling.Plan{
			{ID: 2, Code: "pro", Name: "Pro", SortOrder: 20, IsActive: true},
		},
		subscriptions: []domainbilling.Subscription{
			{ID: 10, UserID: 1, PlanID: 2, PriceID: 20, Status: "active", CurrentPeriodStartAt: now, CurrentPeriodEndAt: &firstEnd},
			{ID: 11, UserID: 1, PlanID: 2, PriceID: 20, Status: "active", CurrentPeriodStartAt: firstEnd, CurrentPeriodEndAt: &secondEnd},
		},
	})

	snapshots, err := service.ListCurrentSubscriptionSnapshots(context.Background(), []uint{1}, now)
	if err != nil {
		t.Fatalf("ListCurrentSubscriptionSnapshots() error = %v", err)
	}
	snapshot, ok := snapshots[1]
	if !ok {
		t.Fatal("ListCurrentSubscriptionSnapshots() missing user snapshot")
	}
	if snapshot.ExpiresAt == nil || !snapshot.ExpiresAt.Equal(secondEnd) {
		t.Fatalf("snapshot.ExpiresAt = %v, want %v", snapshot.ExpiresAt, secondEnd)
	}
}

func TestSubscribeFreePlanRejectsActivePaidEntitlements(t *testing.T) {
	now := time.Now().Add(30 * 24 * time.Hour)
	repo := &billingRepositoryStub{
		plans: []domainbilling.Plan{
			{ID: 1, Code: "free", Name: "Free", IsActive: true},
			{ID: 2, Code: "pro", Name: "Pro", SortOrder: 20, IsActive: true},
		},
		prices: []domainbilling.Price{
			{ID: 10, PlanID: 1, BillingInterval: domainbilling.IntervalLifetime, AmountCents: 0, IsActive: true},
		},
		subscriptions: []domainbilling.Subscription{
			{ID: 20, UserID: 1, PlanID: 2, PriceID: 20, Status: "active", CurrentPeriodStartAt: time.Now(), CurrentPeriodEndAt: &now},
		},
	}
	service := NewService(repo)

	_, err := service.Subscribe(context.Background(), 1, 10, 1)
	if !errors.Is(err, ErrSubscriptionEntitlementActive) {
		t.Fatalf("Subscribe() error = %v, want ErrSubscriptionEntitlementActive", err)
	}
	if repo.replacedSubscription != nil {
		t.Fatal("Subscribe() replaced subscription despite active paid entitlement")
	}
}

func TestCreatePaymentOrderAllowsLowerTierRenewalAfterActiveEntitlement(t *testing.T) {
	now := time.Now().Add(30 * 24 * time.Hour)
	repo := &billingRepositoryStub{
		mode: "period",
		plans: []domainbilling.Plan{
			{ID: 2, Code: "pro", Name: "Pro", SortOrder: 20, IsActive: true},
			{ID: 4, Code: "ultra", Name: "Ultra", SortOrder: 40, IsActive: true},
		},
		prices: []domainbilling.Price{
			{ID: 20, PlanID: 2, BillingInterval: domainbilling.IntervalMonth, Currency: "USD", AmountCents: 2000, IsActive: true},
		},
		subscriptions: []domainbilling.Subscription{
			{ID: 40, UserID: 1, PlanID: 4, PriceID: 40, Status: "active", CurrentPeriodStartAt: time.Now(), CurrentPeriodEndAt: &now},
		},
	}
	service := NewService(repo)

	_, _, _, err := service.CreatePaymentOrder(context.Background(), PaymentOrderInput{
		UserID:   1,
		PriceID:  20,
		Provider: domainbilling.PaymentProviderStripe,
	})
	if err != nil {
		t.Fatalf("CreatePaymentOrder() error = %v", err)
	}
}

func TestCreatePaymentOrderAllowsUpgradeWithActivePaidEntitlement(t *testing.T) {
	now := time.Now().Add(30 * 24 * time.Hour)
	repo := &billingRepositoryStub{
		mode: "period",
		plans: []domainbilling.Plan{
			{ID: 2, Code: "pro", Name: "Pro", SortOrder: 20, IsActive: true},
			{ID: 4, Code: "ultra", Name: "Ultra", SortOrder: 40, IsActive: true},
		},
		prices: []domainbilling.Price{
			{ID: 40, PlanID: 4, BillingInterval: domainbilling.IntervalMonth, Currency: "USD", AmountCents: 20000, IsActive: true},
		},
		subscriptions: []domainbilling.Subscription{
			{ID: 20, UserID: 1, PlanID: 2, PriceID: 20, Status: "active", CurrentPeriodStartAt: time.Now(), CurrentPeriodEndAt: &now},
		},
	}
	service := NewService(repo)

	order, _, _, err := service.CreatePaymentOrder(context.Background(), PaymentOrderInput{
		UserID:   1,
		PriceID:  40,
		Provider: domainbilling.PaymentProviderStripe,
	})
	if err != nil {
		t.Fatalf("CreatePaymentOrder() error = %v", err)
	}
	if order == nil {
		t.Fatal("CreatePaymentOrder() returned nil order")
	}
}

func TestCreatePaymentOrderResolvesProviderPaymentCurrency(t *testing.T) {
	tests := []struct {
		name                 string
		provider             string
		preferredPayCurrency string
		wantPayCurrency      string
		wantPayAmountCents   int64
		wantFXRate           string
	}{
		{
			name:               "stripe uses base currency",
			provider:           domainbilling.PaymentProviderStripe,
			wantPayCurrency:    "USD",
			wantPayAmountCents: 2000,
			wantFXRate:         "1",
		},
		{
			name:                 "stripe follows cny display currency",
			provider:             domainbilling.PaymentProviderStripe,
			preferredPayCurrency: "CNY",
			wantPayCurrency:      "CNY",
			wantPayAmountCents:   14400,
			wantFXRate:           "7.2",
		},
		{
			name:               "epay converts usd to cny",
			provider:           domainbilling.PaymentProviderEPay,
			wantPayCurrency:    "CNY",
			wantPayAmountCents: 14400,
			wantFXRate:         "7.2",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &billingRepositoryStub{
				mode: "period",
				plans: []domainbilling.Plan{
					{ID: 2, Code: "pro", Name: "Pro", IsActive: true},
				},
				prices: []domainbilling.Price{
					{ID: 20, PlanID: 2, BillingInterval: domainbilling.IntervalMonth, Currency: "USD", AmountCents: 2000, IsActive: true},
				},
			}
			service := NewService(repo)

			order, _, _, err := service.CreatePaymentOrder(context.Background(), PaymentOrderInput{
				UserID:               1,
				PriceID:              20,
				Provider:             tt.provider,
				USDToCNYRate:         7.2,
				PreferredPayCurrency: tt.preferredPayCurrency,
			})
			if err != nil {
				t.Fatalf("CreatePaymentOrder() error = %v", err)
			}
			if order.PayCurrency != tt.wantPayCurrency {
				t.Fatalf("PayCurrency = %q, want %q", order.PayCurrency, tt.wantPayCurrency)
			}
			if order.PayAmountCents != tt.wantPayAmountCents {
				t.Fatalf("PayAmountCents = %d, want %d", order.PayAmountCents, tt.wantPayAmountCents)
			}
			if order.FXRate != tt.wantFXRate {
				t.Fatalf("FXRate = %q, want %q", order.FXRate, tt.wantFXRate)
			}
		})
	}
}

func TestCreateTopUpPaymentOrderResolvesProviderPaymentCurrency(t *testing.T) {
	repo := &billingRepositoryStub{mode: "usage"}
	service := NewService(repo)

	stripeOrder, err := service.CreateTopUpPaymentOrder(context.Background(), TopUpPaymentOrderInput{
		UserID:               1,
		AmountMinorUnits:     5000,
		AmountCurrency:       "USD",
		Provider:             domainbilling.PaymentProviderStripe,
		PreferredPayCurrency: "CNY",
		USDToCNYRate:         7.2,
	})
	if err != nil {
		t.Fatalf("CreateTopUpPaymentOrder(stripe) error = %v", err)
	}
	if stripeOrder.PayCurrency != "CNY" || stripeOrder.PayAmountCents != 36000 || stripeOrder.FXRate != "7.2" {
		t.Fatalf("stripe order pay = %s %d fx %s, want CNY 36000 fx 7.2", stripeOrder.PayCurrency, stripeOrder.PayAmountCents, stripeOrder.FXRate)
	}

	epayOrder, err := service.CreateTopUpPaymentOrder(context.Background(), TopUpPaymentOrderInput{
		UserID:           1,
		AmountMinorUnits: 5000,
		AmountCurrency:   "USD",
		Provider:         domainbilling.PaymentProviderEPay,
		USDToCNYRate:     7.2,
	})
	if err != nil {
		t.Fatalf("CreateTopUpPaymentOrder(epay) error = %v", err)
	}
	if epayOrder.PayCurrency != "CNY" || epayOrder.PayAmountCents != 36000 || epayOrder.FXRate != "7.2" {
		t.Fatalf("epay order pay = %s %d fx %s, want CNY 36000 fx 7.2", epayOrder.PayCurrency, epayOrder.PayAmountCents, epayOrder.FXRate)
	}
}

func TestCreateTopUpPaymentOrderKeepsDisplayCurrencyAmountExact(t *testing.T) {
	repo := &billingRepositoryStub{mode: "usage"}
	service := NewService(repo)

	order, err := service.CreateTopUpPaymentOrder(context.Background(), TopUpPaymentOrderInput{
		UserID:               1,
		AmountMinorUnits:     1000,
		AmountCurrency:       "CNY",
		Provider:             domainbilling.PaymentProviderStripe,
		PreferredPayCurrency: "CNY",
		USDToCNYRate:         7.2,
	})
	if err != nil {
		t.Fatalf("CreateTopUpPaymentOrder() error = %v", err)
	}
	if order.PayCurrency != "CNY" || order.PayAmountCents != 1000 {
		t.Fatalf("pay = %s %d, want CNY 1000", order.PayCurrency, order.PayAmountCents)
	}
	if order.CreditNanousd != 1388888889 {
		t.Fatalf("CreditNanousd = %d, want 1388888889", order.CreditNanousd)
	}
}

func TestCreateTopUpPaymentOrderAllowsPeriodModeOverageBalance(t *testing.T) {
	repo := &billingRepositoryStub{mode: "period"}
	service := NewService(repo)

	order, err := service.CreateTopUpPaymentOrder(context.Background(), TopUpPaymentOrderInput{
		UserID:           1,
		AmountMinorUnits: 5000,
		AmountCurrency:   "USD",
		Provider:         domainbilling.PaymentProviderStripe,
	})
	if err != nil {
		t.Fatalf("CreateTopUpPaymentOrder() error = %v", err)
	}
	if order.OrderType != domainbilling.PaymentOrderTypeTopUp || order.CreditNanousd <= 0 {
		t.Fatalf("unexpected top up order: %+v", order)
	}
}

func TestReserveUsageBalancePassesPeriodBudgetSnapshot(t *testing.T) {
	now := time.Now()
	endAt := now.Add(30 * 24 * time.Hour)
	repo := &billingRepositoryStub{
		mode:           "period",
		prepaidNanousd: 300,
		pricing: &domainbilling.ModelPricing{
			PlatformModelName:       "gpt-test",
			Currency:                "USD",
			InputNanousdPerMTokens:  1,
			OutputNanousdPerMTokens: 1,
		},
		plans: []domainbilling.Plan{
			{ID: 2, Code: "pro", Name: "Pro", PeriodCreditNanousd: 1000, IsActive: true},
		},
		subscriptions: []domainbilling.Subscription{
			{ID: 20, UserID: 1, PlanID: 2, Status: "active", CurrentPeriodStartAt: now.Add(-time.Hour), CurrentPeriodEndAt: &endAt},
		},
	}
	service := NewService(repo)

	authorization, err := service.AuthorizeUsage(context.Background(), 1, "gpt-test", "run_1")
	if err != nil {
		t.Fatalf("AuthorizeUsage() error = %v", err)
	}
	if authorization == nil || authorization.Reservation == nil || repo.reservationRequest == nil {
		t.Fatalf("authorization = %+v, request = %+v", authorization, repo.reservationRequest)
	}
	if repo.reservationRequest.RequestedNanousd != 300 || repo.reservationRequest.PeriodCreditNanousd != 1000 {
		t.Fatalf("reservation request = %+v, want requested 300 and period credit 1000", repo.reservationRequest)
	}
	if repo.reservationRequest.PeriodStartAt == nil || repo.reservationRequest.PeriodEndAt == nil {
		t.Fatalf("reservation period is missing: %+v", repo.reservationRequest)
	}
}

func TestReserveUsageBalanceUsesRepositoryFallbackWhenPrepaidIsDisabled(t *testing.T) {
	repo := &billingRepositoryStub{
		mode:           "usage",
		prepaidNanousd: 0,
		pricing: &domainbilling.ModelPricing{
			PlatformModelName:       "gpt-test",
			Currency:                "USD",
			InputNanousdPerMTokens:  1,
			OutputNanousdPerMTokens: 1,
		},
	}
	service := NewService(repo)

	authorization, err := service.AuthorizeUsage(context.Background(), 1, "gpt-test", "run_default_budget")
	if err != nil {
		t.Fatalf("AuthorizeUsage() error = %v", err)
	}
	if authorization == nil || authorization.Reservation == nil || repo.reservationRequest == nil {
		t.Fatalf("authorization = %+v, request = %+v", authorization, repo.reservationRequest)
	}
	if repo.reservationRequest.Mode != "usage" || repo.reservationRequest.RequestedNanousd != 0 {
		t.Fatalf("reservation request = %+v, want usage fallback budget", repo.reservationRequest)
	}
}

func TestRecordUsageUsesReservationModeSnapshot(t *testing.T) {
	periodStart := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	periodEnd := periodStart.Add(30 * 24 * time.Hour)
	usage := &domainbilling.UsageLedger{
		UserID:            1,
		PlatformModelName: "gpt-test",
		BillingAt:         periodStart.Add(time.Hour),
		UsageDate:         periodStart,
		BilledCurrency:    "USD",
		BilledNanousd:     100,
	}

	periodRepo := &billingRepositoryStub{mode: "usage"}
	periodService := NewService(periodRepo)
	err := periodService.RecordUsageWithAuthorization(context.Background(), usage, &domainbilling.UsageAuthorization{
		Mode: "period",
		Reservation: &domainbilling.UsageBalanceReservation{
			UserID:             1,
			RefNo:              "run_period_snapshot",
			Mode:               "period",
			PeriodLimitNanousd: 1000,
			PeriodStartAt:      &periodStart,
			PeriodEndAt:        &periodEnd,
		},
	})
	if err != nil {
		t.Fatalf("period snapshot settlement error = %v", err)
	}
	if !periodRepo.periodUsageSettled || periodRepo.usageSettled || periodRepo.periodCreditNanousd != 1000 {
		t.Fatalf("period settlement state = %+v", periodRepo)
	}

	usageRepo := &billingRepositoryStub{mode: "period"}
	usageService := NewService(usageRepo)
	err = usageService.RecordUsageWithAuthorization(context.Background(), usage, &domainbilling.UsageAuthorization{
		Mode: "usage",
		Reservation: &domainbilling.UsageBalanceReservation{
			UserID: 1,
			RefNo:  "run_usage_snapshot",
			Mode:   "usage",
		},
	})
	if err != nil {
		t.Fatalf("usage snapshot settlement error = %v", err)
	}
	if !usageRepo.usageSettled || usageRepo.periodUsageSettled {
		t.Fatalf("usage settlement state = %+v", usageRepo)
	}
}

func TestBuildSubscriptionEntitlementViewsShowsCurrentAndQueuedPlans(t *testing.T) {
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	day := 24 * time.Hour
	maxEnd := now.Add(30 * day)
	proEnd := now.Add(60 * day)
	plans := map[uint]domainbilling.Plan{
		2: {ID: 2, Code: "pro", Name: "Pro", SortOrder: 20, IsActive: true},
		3: {ID: 3, Code: "max", Name: "Max", SortOrder: 30, IsActive: true},
	}

	views := buildSubscriptionEntitlementViews([]domainbilling.Subscription{
		{ID: 20, UserID: 1, PlanID: 3, PriceID: 30, Status: "active", CurrentPeriodStartAt: now, CurrentPeriodEndAt: &maxEnd},
		{ID: 21, UserID: 1, PlanID: 2, PriceID: 20, Status: "active", CurrentPeriodStartAt: maxEnd, CurrentPeriodEndAt: &proEnd},
	}, plans, now)

	if len(views) != 2 {
		t.Fatalf("entitlement views len = %d, want 2", len(views))
	}
	if !views[0].IsCurrent || views[0].Plan.Code != "max" {
		t.Fatalf("views[0] = %+v, want current max", views[0])
	}
	if views[1].IsCurrent || views[1].Plan.Code != "pro" {
		t.Fatalf("views[1] = %+v, want queued pro", views[1])
	}
}
