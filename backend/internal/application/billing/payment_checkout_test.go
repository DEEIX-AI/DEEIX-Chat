package billing

import (
	"context"
	"net/url"
	"testing"

	domainbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/billing"
	paymentport "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/payment"
)

type stripeCheckoutProviderStub struct {
	input paymentport.StripeCheckoutInput
}

func (s *stripeCheckoutProviderStub) CreateCheckoutSession(_ context.Context, input paymentport.StripeCheckoutInput) (paymentport.CheckoutResult, error) {
	s.input = input
	return paymentport.CheckoutResult{ID: "cs_test", URL: "https://checkout.stripe.com/test"}, nil
}

type epayCheckoutProviderStub struct {
	input paymentport.EPayCheckoutInput
}

func (s *epayCheckoutProviderStub) CreateCheckout(_ context.Context, input paymentport.EPayCheckoutInput) (paymentport.CheckoutResult, error) {
	s.input = input
	return paymentport.CheckoutResult{URL: "https://pay.example.com/submit.php?sign=test"}, nil
}

func (s *epayCheckoutProviderStub) VerifySignature(_ url.Values, _ string) bool {
	return true
}

func TestCreateStripeCheckoutSessionMapsOrderAtApplicationBoundary(t *testing.T) {
	provider := &stripeCheckoutProviderStub{}
	service := NewPaymentCheckoutService(provider, nil)
	order := &domainbilling.PaymentOrder{
		OrderNo:         "order-123",
		OrderType:       domainbilling.PaymentOrderTypeTopUp,
		UserID:          42,
		BaseCurrency:    "USD",
		BaseAmountCents: 1250,
		PayCurrency:     "USD",
		PayAmountCents:  1250,
		FXRate:          "1",
	}

	result, err := service.CreateStripeCheckoutSession(t.Context(), StripeCheckoutInput{
		SecretKey:  "secret",
		SuccessURL: "https://chat.example/success",
		CancelURL:  "https://chat.example/cancel",
		Order:      order,
	})
	if err != nil {
		t.Fatalf("create checkout session: %v", err)
	}
	if result.ID != "cs_test" || provider.input.OrderNo != order.OrderNo {
		t.Fatalf("unexpected checkout mapping: result=%#v input=%#v", result, provider.input)
	}
	if provider.input.ProductName != "按量余额充值" || provider.input.ProductDescription != "充值 USD 12.50 至按量余额" {
		t.Fatalf("unexpected payment product: %#v", provider.input)
	}
}

func TestCreateEPayCheckoutMapsOrderAtApplicationBoundary(t *testing.T) {
	provider := &epayCheckoutProviderStub{}
	service := NewPaymentCheckoutService(nil, provider)
	order := &domainbilling.PaymentOrder{
		OrderNo:         "order-123",
		OrderType:       domainbilling.PaymentOrderTypeTopUp,
		UserID:          42,
		BaseCurrency:    "USD",
		BaseAmountCents: 1250,
		PayCurrency:     "CNY",
		PayAmountCents:  9000,
		FXRate:          "7.2",
	}

	result, err := service.CreateEPayCheckout(t.Context(), EPayCheckoutInput{
		GatewayURL:  "https://pay.example.com/",
		MerchantID:  "merchant-1",
		MerchantKey: "secret",
		PaymentType: "alipay",
		NotifyURL:   "https://api.example.com/notify",
		ReturnURL:   "https://chat.example.com/return",
		Order:       order,
	})
	if err != nil {
		t.Fatalf("CreateEPayCheckout() error = %v", err)
	}
	if result.URL == "" || provider.input.OrderNo != order.OrderNo {
		t.Fatalf("unexpected checkout mapping: result=%#v input=%#v", result, provider.input)
	}
	if provider.input.PayCurrency != "CNY" || provider.input.PayAmountCents != 9000 {
		t.Fatalf("unexpected payment amount mapping: %#v", provider.input)
	}
	if provider.input.ProductName != "按量余额充值" {
		t.Fatalf("unexpected payment product: %#v", provider.input)
	}
}
