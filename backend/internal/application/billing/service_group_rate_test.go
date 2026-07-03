package billing

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	domainbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/billing"
)

// ---------------------------------------------------------------------------
// Pure function tests
// ---------------------------------------------------------------------------

func TestComposeGroupRatePercent(t *testing.T) {
	discounted := composeGroupRatePercent(billingRateMultiplier{Numerator: 1, Denominator: 1}, 80)
	if got := applyRateMultiplier(1000, discounted); got != 800 {
		t.Fatalf("expected 80%% multiplier to yield 800, got %d", got)
	}

	composed := composeGroupRatePercent(billingRateMultiplier{Numerator: 6, Denominator: 1}, 80)
	if got := applyRateMultiplier(1000, composed); got != 4800 {
		t.Fatalf("expected composed 6x * 80%% multiplier to yield 4800, got %d", got)
	}
}

func TestComposeGroupRatePercentIdentity(t *testing.T) {
	base := billingRateMultiplier{Numerator: 2, Denominator: 1}
	result := composeGroupRatePercent(base, 100)
	if result != base {
		t.Fatalf("100%% should return base unchanged, got %+v", result)
	}
	result = composeGroupRatePercent(base, 0)
	if result != base {
		t.Fatalf("0%% should return base unchanged, got %+v", result)
	}
}

// ---------------------------------------------------------------------------
// Resolver stub
// ---------------------------------------------------------------------------

type groupRateResolverStub struct {
	percent int
	err     error
	called  bool
}

func (s *groupRateResolverStub) GetUserGroupRateMultiplierPercent(_ context.Context, _ uint, _ []uint) (int, error) {
	s.called = true
	return s.percent, s.err
}

// ---------------------------------------------------------------------------
// applyGroupRateMultiplier unit tests
// ---------------------------------------------------------------------------

func TestApplyGroupRateMultiplierNoResolverKeepsBase(t *testing.T) {
	s := &Service{}
	base := billingRateMultiplier{Numerator: 2, Denominator: 1}
	got, err := s.applyGroupRateMultiplier(nil, 1, nil, base)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != base {
		t.Fatalf("expected base unchanged without resolver, got %+v", got)
	}
}

func TestApplyGroupRateMultiplierReturnsErrorOnResolverFailure(t *testing.T) {
	resolver := &groupRateResolverStub{err: errors.New("db down")}
	s := &Service{groupRateResolver: resolver}
	base := billingRateMultiplier{Numerator: 1, Denominator: 1}
	_, err := s.applyGroupRateMultiplier(context.Background(), 1, nil, base)
	if err == nil {
		t.Fatal("expected error propagation from resolver")
	}
}

func TestApplyGroupRateMultiplierPassesSubscriptionGroupID(t *testing.T) {
	var receivedExtra []uint
	resolver := &groupRateResolverStub{percent: 80}
	orig := resolver.GetUserGroupRateMultiplierPercent
	_ = orig
	s := &Service{groupRateResolver: &captureExtraGroupResolver{
		percent: 80,
		capture: func(extra []uint) { receivedExtra = extra },
	}}
	subID := uint(42)
	_, err := s.applyGroupRateMultiplier(context.Background(), 1, &subID, billingRateMultiplier{Numerator: 1, Denominator: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(receivedExtra) != 1 || receivedExtra[0] != 42 {
		t.Fatalf("expected subscription group ID 42 passed as extra, got %v", receivedExtra)
	}
}

type captureExtraGroupResolver struct {
	percent int
	capture func(extra []uint)
}

func (c *captureExtraGroupResolver) GetUserGroupRateMultiplierPercent(_ context.Context, _ uint, extra []uint) (int, error) {
	c.capture(extra)
	return c.percent, nil
}

// ---------------------------------------------------------------------------
// BuildUsageLedger integration: group rate applied to main ledger
// ---------------------------------------------------------------------------

func TestBuildUsageLedgerAppliesGroupRateMultiplier(t *testing.T) {
	repo := &billingRepositoryStub{
		mode: "usage",
		pricing: &domainbilling.ModelPricing{
			PlatformModelName:      "gpt-test",
			Currency:               "USD",
			PricingMode:            domainbilling.PricingModeToken,
			InputNanousdPerMTokens: 1_000_000_000,
		},
	}
	service := NewService(repo)
	service.SetGroupRateMultiplierResolver(&groupRateResolverStub{percent: 80})

	ledger, err := service.BuildUsageLedger(context.Background(), UsagePricingInput{
		UserID:            1,
		PlatformModelName: "gpt-test",
		InputTokens:       1_000_000,
	})
	if err != nil {
		t.Fatalf("BuildUsageLedger: %v", err)
	}
	// 1M tokens * 1_000_000_000 / 1M * 0.8 = 800_000_000
	if ledger.BilledNanousd != 800_000_000 {
		t.Fatalf("expected 80%% group rate, got billed=%d (want 800000000)", ledger.BilledNanousd)
	}

	var snapshot map[string]interface{}
	if err := json.Unmarshal([]byte(ledger.PricingSnapshotJSON), &snapshot); err != nil {
		t.Fatalf("unmarshal snapshot: %v", err)
	}
	if snapshot["rate_multiplier"] != 0.8 {
		t.Fatalf("expected rate_multiplier=0.8, got %v", snapshot["rate_multiplier"])
	}
}

// ---------------------------------------------------------------------------
// BuildUsageLedger integration: group rate applied to service items
// ---------------------------------------------------------------------------

func TestBuildUsageLedgerAppliesGroupRateToServiceItems(t *testing.T) {
	repo := &billingRepositoryStub{
		mode: "usage",
		pricing: &domainbilling.ModelPricing{
			PlatformModelName:       "gpt-test",
			Currency:                "USD",
			PricingMode:             domainbilling.PricingModeToken,
			InputNanousdPerMTokens:  1_000_000_000,
			OutputNanousdPerMTokens: 2_000_000_000,
		},
	}
	service := NewService(repo)
	service.SetGroupRateMultiplierResolver(&groupRateResolverStub{percent: 50})

	ledger, err := service.BuildUsageLedger(context.Background(), UsagePricingInput{
		UserID:            1,
		PlatformModelName: "gpt-test",
		ServiceOnly:       true,
		ServiceItems: []ServiceUsageInput{{
			ServiceCode:       "context",
			ServiceName:       "Context",
			PlatformModelName: "gpt-test",
			InputTokens:       1_000_000,
			OutputTokens:      1_000_000,
		}},
	})
	if err != nil {
		t.Fatalf("BuildUsageLedger: %v", err)
	}
	// service item: (1M * 1B + 1M * 2B) / 1M * 0.5 = 1_500_000_000
	if ledger.BilledNanousd != 1_500_000_000 {
		t.Fatalf("expected 50%% group rate on service items, got billed=%d (want 1500000000)", ledger.BilledNanousd)
	}

	var snapshot map[string]interface{}
	if err := json.Unmarshal([]byte(ledger.PricingSnapshotJSON), &snapshot); err != nil {
		t.Fatalf("unmarshal snapshot: %v", err)
	}
	items, ok := snapshot["service_items"].([]interface{})
	if !ok || len(items) != 1 {
		t.Fatalf("expected 1 service item, got %v", snapshot["service_items"])
	}
	item := items[0].(map[string]interface{})
	if item["rate_multiplier"] != 0.5 {
		t.Fatalf("expected service item rate_multiplier=0.5, got %v", item["rate_multiplier"])
	}
}

// ---------------------------------------------------------------------------
// BuildUsageLedger: resolver error propagation
// ---------------------------------------------------------------------------

func TestBuildUsageLedgerReturnsErrorOnGroupResolverFailure(t *testing.T) {
	repo := &billingRepositoryStub{
		mode: "usage",
		pricing: &domainbilling.ModelPricing{
			PlatformModelName: "gpt-test",
			Currency:          "USD",
			PricingMode:       domainbilling.PricingModeToken,
		},
	}
	service := NewService(repo)
	service.SetGroupRateMultiplierResolver(&groupRateResolverStub{err: errors.New("db connection lost")})

	_, err := service.BuildUsageLedger(context.Background(), UsagePricingInput{
		UserID:            1,
		PlatformModelName: "gpt-test",
		InputTokens:       1_000,
	})
	if err == nil {
		t.Fatal("expected error from group resolver to propagate")
	}
}

// ---------------------------------------------------------------------------
// BuildUsageLedger: self mode skips group rate
// ---------------------------------------------------------------------------

func TestBuildUsageLedgerSelfModeSkipsGroupRate(t *testing.T) {
	resolver := &groupRateResolverStub{percent: 50}
	repo := &billingRepositoryStub{mode: "self"}
	service := NewService(repo)
	service.SetGroupRateMultiplierResolver(resolver)

	_, err := service.BuildUsageLedger(context.Background(), UsagePricingInput{
		UserID:            1,
		PlatformModelName: "gpt-test",
		InputTokens:       1_000,
	})
	if err != nil {
		t.Fatalf("BuildUsageLedger: %v", err)
	}
	if resolver.called {
		t.Fatal("group rate resolver should not be called in self mode")
	}
}

// ---------------------------------------------------------------------------
// BuildUsageLedger: fast mode + group rate composition
// ---------------------------------------------------------------------------

func TestBuildUsageLedgerComposesGroupRateWithFastMode(t *testing.T) {
	repo := &billingRepositoryStub{
		mode: "usage",
		pricing: &domainbilling.ModelPricing{
			PlatformModelName:       "claude-opus-4.6",
			Currency:                "USD",
			PricingMode:             domainbilling.PricingModeToken,
			InputNanousdPerMTokens:  1_000_000_000,
			OutputNanousdPerMTokens: 5_000_000_000,
		},
	}
	service := NewService(repo)
	service.SetGroupRateMultiplierResolver(&groupRateResolverStub{percent: 80})

	ledger, err := service.BuildUsageLedger(context.Background(), UsagePricingInput{
		UserID:            1,
		PlatformModelName: "claude-opus-4.6",
		ProviderProtocol:  "anthropic_messages",
		RequestSpeed:      "fast",
		UsageSpeed:        "fast",
		InputTokens:       1_000_000,
		OutputTokens:      1_000_000,
	})
	if err != nil {
		t.Fatalf("BuildUsageLedger: %v", err)
	}
	// fast=6x, group=0.8, combined=4.8x
	// input: 1M * 1B/M * 4.8 = 4_800_000_000
	// output: 1M * 5B/M * 4.8 = 24_000_000_000
	// total = 28_800_000_000
	if ledger.BilledNanousd != 28_800_000_000 {
		t.Fatalf("expected fast(6x) * group(0.8) = 4.8x billing, got %d (want 28800000000)", ledger.BilledNanousd)
	}

	var snapshot map[string]interface{}
	if err := json.Unmarshal([]byte(ledger.PricingSnapshotJSON), &snapshot); err != nil {
		t.Fatalf("unmarshal snapshot: %v", err)
	}
	if snapshot["rate_multiplier"] != 4.8 {
		t.Fatalf("expected rate_multiplier=4.8, got %v", snapshot["rate_multiplier"])
	}
}
