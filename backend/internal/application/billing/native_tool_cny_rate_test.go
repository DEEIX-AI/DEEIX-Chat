package billing

import (
	"context"
	"testing"
)

func TestListNativeToolPricingFollowsPlatformCNYRate(t *testing.T) {
	prices := func(rate float64) map[string]int64 {
		service := NewService(&billingRepositoryStub{usdToCNYRate: rate})
		items, err := service.ListNativeToolPricing(context.Background(), "")
		if err != nil {
			t.Fatalf("list native tool pricing: %v", err)
		}
		result := make(map[string]int64, len(items))
		for _, item := range items {
			result[item.ToolKey] = item.PriceNanousd
		}
		return result
	}
	atEight := prices(8)
	if atEight["ark.doubao_search"] != 2_500_000 {
		t.Fatalf("CNY 0.02 at 8 CNY/USD = %d, want 2500000", atEight["ark.doubao_search"])
	}
	if atEight["openai.web_search"] != 10_000_000 {
		t.Fatalf("USD tool price changed with CNY rate: %d", atEight["openai.web_search"])
	}
	if unset := prices(0); unset["ark.doubao_search"] != 2_777_778 {
		t.Fatalf("unset rate must fall back to 7.2, got %d", unset["ark.doubao_search"])
	}
}
