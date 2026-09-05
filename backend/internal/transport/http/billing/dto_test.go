package billing

import (
	"testing"

	appbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/billing"
	domainbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/billing"
	"github.com/gin-gonic/gin/binding"
)

func TestRequiredZeroValueBillingFields(t *testing.T) {
	zeroFloat := 0.0
	falseValue := false
	emptyString := ""

	tests := []struct {
		name  string
		value any
	}{
		{
			name: "account balance accepts explicit zero",
			value: UpdateBillingAccountBalanceRequest{
				BalanceUSD: &zeroFloat,
			},
		},
		{
			name: "plan accepts explicit zero values",
			value: UpdateBillingPlanRequest{
				Name:            "Free",
				Description:     &emptyString,
				PeriodCreditUSD: &zeroFloat,
				AmountUSD:       &zeroFloat,
				BillingInterval: "month",
			},
		},
		{
			name: "model pricing accepts explicit zero and false values",
			value: UpsertModelPricingRequest{
				PlatformModelName:       "test-model",
				IsFree:                  &falseValue,
				PricingMode:             "token",
				InputUSDPerMTokens:      &zeroFloat,
				CacheReadUSDPerMTokens:  &zeroFloat,
				CacheWriteUSDPerMTokens: &zeroFloat,
				OutputUSDPerMTokens:     &zeroFloat,
				CallUSDPerCall:          &zeroFloat,
				DurationUSDPerSecond:    &zeroFloat,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := binding.Validator.ValidateStruct(tt.value); err != nil {
				t.Fatalf("expected explicit zero values to pass validation: %v", err)
			}
		})
	}
}

func TestRequiredBillingFieldsRejectMissingValues(t *testing.T) {
	tests := []struct {
		name  string
		value any
	}{
		{
			name:  "account balance",
			value: UpdateBillingAccountBalanceRequest{},
		},
		{
			name: "plan values",
			value: UpdateBillingPlanRequest{
				Name:            "Free",
				BillingInterval: "month",
			},
		},
		{
			name: "model pricing values",
			value: UpsertModelPricingRequest{
				PlatformModelName: "test-model",
				PricingMode:       "token",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := binding.Validator.ValidateStruct(tt.value); err == nil {
				t.Fatal("expected missing required values to fail validation")
			}
		})
	}
}

func TestOptionalBillingCyclesUseServiceDefault(t *testing.T) {
	if err := binding.Validator.ValidateStruct(SubscribeRequest{PriceID: 1}); err != nil {
		t.Fatalf("expected omitted subscription cycles to pass validation: %v", err)
	}
	if err := binding.Validator.ValidateStruct(CreateCheckoutRequest{}); err != nil {
		t.Fatalf("expected omitted checkout cycles to pass validation: %v", err)
	}

	zero := 0
	if err := binding.Validator.ValidateStruct(SubscribeRequest{PriceID: 1, Cycles: &zero}); err == nil {
		t.Fatal("expected explicit zero subscription cycles to fail validation")
	}
	if err := binding.Validator.ValidateStruct(CreateCheckoutRequest{Cycles: &zero}); err == nil {
		t.Fatal("expected explicit zero checkout cycles to fail validation")
	}
}

func TestBillingAccountResponsesPreserveNegativeBalance(t *testing.T) {
	const balanceNanousd int64 = -1_250_000_000
	const wantBalanceUSD = -1.25

	account := toBillingAccountResponse(&domainbilling.BillingAccount{BalanceNanousd: balanceNanousd})
	if account.BalanceUSD != wantBalanceUSD {
		t.Fatalf("expected account balance %v, got %v", wantBalanceUSD, account.BalanceUSD)
	}

	accountView := toBillingAccountViewResponse(&appbilling.BillingAccountView{BalanceNanousd: balanceNanousd})
	if accountView == nil {
		t.Fatal("expected account view response")
	}
	if accountView.BalanceUSD != wantBalanceUSD {
		t.Fatalf("expected account view balance %v, got %v", wantBalanceUSD, accountView.BalanceUSD)
	}
}

func TestNanousdToUSDClampsNegativeNonBalanceAmount(t *testing.T) {
	if got := nanousdToUSD(-1_250_000_000); got != 0 {
		t.Fatalf("expected negative non-balance amount to be clamped, got %v", got)
	}
}

func TestUsageLedgerResponsePreservesBalanceSnapshot(t *testing.T) {
	balanceNanousd := int64(-1_250_000_000)
	response := toUsageLedgerResponse(domainbilling.UsageLedger{BalanceAfterNanousd: &balanceNanousd})
	if response.BalanceAfterNanousd == nil || *response.BalanceAfterNanousd != balanceNanousd {
		t.Fatalf("expected raw balance snapshot %d, got %v", balanceNanousd, response.BalanceAfterNanousd)
	}
	if response.BalanceAfterUSD == nil || *response.BalanceAfterUSD != -1.25 {
		t.Fatalf("expected USD balance snapshot -1.25, got %v", response.BalanceAfterUSD)
	}

	response = toUsageLedgerResponse(domainbilling.UsageLedger{})
	if response.BalanceAfterNanousd != nil || response.BalanceAfterUSD != nil {
		t.Fatalf("expected unavailable balance snapshot to remain nil, got %+v", response)
	}
}
