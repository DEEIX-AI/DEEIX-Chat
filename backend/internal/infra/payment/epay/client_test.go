package epay

import (
	"net/url"
	"testing"

	paymentport "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/payment"
)

func TestCreateCheckoutBuildsClassicEPayRequest(t *testing.T) {
	result, err := New().CreateCheckout(t.Context(), paymentport.EPayCheckoutInput{
		GatewayURL:     "https://pay.example.com/epay/",
		MerchantID:     "merchant-1",
		MerchantKey:    "secret",
		PaymentType:    "alipay",
		OrderNo:        "order-123",
		NotifyURL:      "https://api.example.com/api/v1/billing/payments/epay/notify",
		ReturnURL:      "https://chat.example.com/setting/subscription?payment=success",
		PayCurrency:    "CNY",
		PayAmountCents: 1234,
		ProductName:    "测试套餐",
	})
	if err != nil {
		t.Fatalf("CreateCheckout() error = %v", err)
	}
	parsed, err := url.Parse(result.URL)
	if err != nil {
		t.Fatalf("parse checkout url: %v", err)
	}
	if parsed.Scheme != "https" || parsed.Host != "pay.example.com" || parsed.Path != "/epay/submit.php" {
		t.Fatalf("unexpected checkout endpoint: %s", result.URL)
	}
	query := parsed.Query()
	for key, want := range map[string]string{
		"pid":          "merchant-1",
		"type":         "alipay",
		"out_trade_no": "order-123",
		"money":        "12.34",
		"name":         "测试套餐",
		"sign_type":    "MD5",
	} {
		if got := query.Get(key); got != want {
			t.Fatalf("query[%q] = %q, want %q", key, got, want)
		}
	}
	providedSign := query.Get("sign")
	const expectedSign = "145ce27900fbe84c65702eea6b2c5218"
	if providedSign != expectedSign {
		t.Fatalf("sign = %q, want %q", providedSign, expectedSign)
	}
	if query.Has("key") {
		t.Fatal("merchant key must never be included in the checkout url")
	}
}

func TestCreateCheckoutAcceptsExactSubmitEndpoint(t *testing.T) {
	result, err := New().CreateCheckout(t.Context(), paymentport.EPayCheckoutInput{
		GatewayURL:     "https://pay.example.com/custom/submit.php",
		MerchantID:     "merchant-1",
		MerchantKey:    "secret",
		PaymentType:    "wxpay",
		OrderNo:        "order-123",
		NotifyURL:      "https://api.example.com/notify",
		ReturnURL:      "https://chat.example.com/return",
		PayCurrency:    "CNY",
		PayAmountCents: 100,
		ProductName:    "Plan",
	})
	if err != nil {
		t.Fatalf("CreateCheckout() error = %v", err)
	}
	parsed, _ := url.Parse(result.URL)
	if parsed.Path != "/custom/submit.php" {
		t.Fatalf("checkout path = %q", parsed.Path)
	}
}

func TestCreateCheckoutPreservesLegacySubdirectoryBaseURLBehavior(t *testing.T) {
	result, err := New().CreateCheckout(t.Context(), paymentport.EPayCheckoutInput{
		GatewayURL:     "https://pay.example.com/api/pay",
		MerchantID:     "merchant-1",
		MerchantKey:    "secret",
		PaymentType:    "alipay",
		OrderNo:        "order-123",
		NotifyURL:      "https://api.example.com/notify",
		ReturnURL:      "https://chat.example.com/return",
		PayCurrency:    "CNY",
		PayAmountCents: 100,
		ProductName:    "Plan",
	})
	if err != nil {
		t.Fatalf("CreateCheckout() error = %v", err)
	}
	parsed, _ := url.Parse(result.URL)
	if parsed.Path != "/api/pay/submit.php" {
		t.Fatalf("checkout path = %q", parsed.Path)
	}
}

func TestVerifySignature(t *testing.T) {
	values := url.Values{
		"pid":          {"merchant-1"},
		"out_trade_no": {"order-123"},
		"money":        {"12.34"},
		"sign_type":    {"MD5"},
	}
	values.Set("sign", "c2752306f4b5b5dffc443a8d9ef06673")
	if !New().VerifySignature(values, "secret") {
		t.Fatal("expected signature to be valid")
	}
	values.Set("money", "99.99")
	if New().VerifySignature(values, "secret") {
		t.Fatal("expected tampered notification to be rejected")
	}
}
