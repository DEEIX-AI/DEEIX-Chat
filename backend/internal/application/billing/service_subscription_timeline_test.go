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
		name               string
		provider           string
		wantPayCurrency    string
		wantPayAmountCents int64
		wantFXRate         string
	}{
		{
			name:               "stripe uses base currency",
			provider:           domainbilling.PaymentProviderStripe,
			wantPayCurrency:    "USD",
			wantPayAmountCents: 2000,
			wantFXRate:         "1",
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
				UserID:       1,
				PriceID:      20,
				Provider:     tt.provider,
				USDToCNYRate: 7.2,
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
		UserID:      1,
		AmountCents: 5000,
		Provider:    domainbilling.PaymentProviderStripe,
	})
	if err != nil {
		t.Fatalf("CreateTopUpPaymentOrder(stripe) error = %v", err)
	}
	if stripeOrder.PayCurrency != "USD" || stripeOrder.PayAmountCents != 5000 || stripeOrder.FXRate != "1" {
		t.Fatalf("stripe order pay = %s %d fx %s, want USD 5000 fx 1", stripeOrder.PayCurrency, stripeOrder.PayAmountCents, stripeOrder.FXRate)
	}

	epayOrder, err := service.CreateTopUpPaymentOrder(context.Background(), TopUpPaymentOrderInput{
		UserID:       1,
		AmountCents:  5000,
		Provider:     domainbilling.PaymentProviderEPay,
		USDToCNYRate: 7.2,
	})
	if err != nil {
		t.Fatalf("CreateTopUpPaymentOrder(epay) error = %v", err)
	}
	if epayOrder.PayCurrency != "CNY" || epayOrder.PayAmountCents != 36000 || epayOrder.FXRate != "7.2" {
		t.Fatalf("epay order pay = %s %d fx %s, want CNY 36000 fx 7.2", epayOrder.PayCurrency, epayOrder.PayAmountCents, epayOrder.FXRate)
	}
}

func TestCreateTopUpPaymentOrderAllowsPeriodModeOverageBalance(t *testing.T) {
	repo := &billingRepositoryStub{mode: "period"}
	service := NewService(repo)

	order, err := service.CreateTopUpPaymentOrder(context.Background(), TopUpPaymentOrderInput{
		UserID:      1,
		AmountCents: 5000,
		Provider:    domainbilling.PaymentProviderStripe,
	})
	if err != nil {
		t.Fatalf("CreateTopUpPaymentOrder() error = %v", err)
	}
	if order.OrderType != domainbilling.PaymentOrderTypeTopUp || order.CreditNanousd <= 0 {
		t.Fatalf("unexpected top up order: %+v", order)
	}
}

func TestEnsureModelUsableAllowsPeriodOverageWhenBalanceIsAvailable(t *testing.T) {
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	endAt := now.Add(30 * 24 * time.Hour)
	repo := &billingRepositoryStub{
		mode:            "period",
		prepaidNanousd:  300,
		billableNanousd: 1000,
		account:         &domainbilling.BillingAccount{UserID: 1, BalanceNanousd: 500, Currency: "USD", Status: "active"},
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
			{ID: 20, UserID: 1, PlanID: 2, Status: "active", CurrentPeriodStartAt: now, CurrentPeriodEndAt: &endAt},
		},
	}
	service := NewService(repo)

	if err := service.EnsureModelUsable(context.Background(), 1, "gpt-test", now); err != nil {
		t.Fatalf("EnsureModelUsable() error = %v", err)
	}
}

func TestEnsureModelUsableRejectsPeriodOverageWhenBalanceIsInsufficient(t *testing.T) {
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	endAt := now.Add(30 * 24 * time.Hour)
	repo := &billingRepositoryStub{
		mode:            "period",
		prepaidNanousd:  300,
		billableNanousd: 1000,
		account:         &domainbilling.BillingAccount{UserID: 1, BalanceNanousd: 100, Currency: "USD", Status: "active"},
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
			{ID: 20, UserID: 1, PlanID: 2, Status: "active", CurrentPeriodStartAt: now, CurrentPeriodEndAt: &endAt},
		},
	}
	service := NewService(repo)

	err := service.EnsureModelUsable(context.Background(), 1, "gpt-test", now)
	if !errors.Is(err, ErrUsageBalanceInsufficient) {
		t.Fatalf("EnsureModelUsable() error = %v, want ErrUsageBalanceInsufficient", err)
	}
}

func TestReserveUsageBalancePeriodModeReservesOnlyPotentialOverage(t *testing.T) {
	now := time.Now()
	endAt := now.Add(30 * 24 * time.Hour)
	repo := &billingRepositoryStub{
		mode:            "period",
		prepaidNanousd:  300,
		billableNanousd: 900,
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

	reservation, err := service.ReserveUsageBalance(context.Background(), 1, "gpt-test", "run_1")
	if err != nil {
		t.Fatalf("ReserveUsageBalance() error = %v", err)
	}
	if reservation == nil || reservation.AmountNanousd != 200 || repo.reservedNanousd != 200 {
		t.Fatalf("reservation = %+v, reserved = %d, want 200", reservation, repo.reservedNanousd)
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
