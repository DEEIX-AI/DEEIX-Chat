package billing

import (
	"context"
	"testing"

	domainbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/billing"
	stripeinfra "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/payment/stripe"
)

type stripeCheckoutProviderStub struct {
	input stripeinfra.CheckoutInput
}

func (s *stripeCheckoutProviderStub) CreateCheckoutSession(_ context.Context, input stripeinfra.CheckoutInput) (stripeinfra.CheckoutResult, error) {
	s.input = input
	return stripeinfra.CheckoutResult{ID: "cs_test", URL: "https://checkout.stripe.com/test"}, nil
}

func TestCreateStripeCheckoutSessionMapsOrderAtApplicationBoundary(t *testing.T) {
	provider := &stripeCheckoutProviderStub{}
	service := NewPaymentCheckoutService(provider)
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
