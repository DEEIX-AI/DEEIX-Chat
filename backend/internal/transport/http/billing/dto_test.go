package billing

import (
	"testing"

	"github.com/gin-gonic/gin/binding"
)

func TestRequiredZeroValueBillingFields(t *testing.T) {
	zeroFloat := 0.0
	zeroInt := 0
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
				DiscountPercent: &zeroInt,
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
