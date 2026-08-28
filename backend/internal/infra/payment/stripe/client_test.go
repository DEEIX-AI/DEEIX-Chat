package stripe

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	paymentport "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/payment"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestCreateCheckoutSessionMapsProtocolAndValidatesResponse(t *testing.T) {
	client := &Client{httpClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodPost || request.URL.String() != checkoutSessionsURL {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL)
		}
		username, _, ok := request.BasicAuth()
		if !ok || username != "sk_test_example" {
			t.Fatal("stripe secret was not sent with HTTP Basic auth")
		}
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		form, err := url.ParseQuery(string(body))
		if err != nil {
			t.Fatalf("parse request body: %v", err)
		}
		if form.Get("client_reference_id") != "order-123" || form.Get("line_items[0][price_data][currency]") != "usd" {
			t.Fatalf("unexpected checkout form: %#v", form)
		}
		if form.Get("metadata[user_id]") != "42" || form.Get("metadata[pay_amount_cents]") != "1999" {
			t.Fatalf("missing checkout metadata: %#v", form)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"id":"cs_test_123","url":"https://checkout.stripe.com/c/pay/cs_test_123"}`)),
			Header:     make(http.Header),
		}, nil
	})}}

	result, err := client.CreateCheckoutSession(t.Context(), paymentport.StripeCheckoutInput{
		SecretKey:          "sk_test_example",
		SuccessURL:         "https://chat.example.com/settings?payment=success",
		CancelURL:          "https://chat.example.com/settings?payment=cancel",
		OrderNo:            "order-123",
		OrderType:          "subscription",
		UserID:             42,
		BaseCurrency:       "USD",
		BaseAmountCents:    1999,
		PayCurrency:        "USD",
		PayAmountCents:     1999,
		FXRate:             "1",
		ProductName:        "Pro",
		ProductDescription: "Pro subscription",
	})
	if err != nil {
		t.Fatalf("create checkout session: %v", err)
	}
	if result.ID != "cs_test_123" || result.URL != "https://checkout.stripe.com/c/pay/cs_test_123" {
		t.Fatalf("unexpected checkout result: %#v", result)
	}
}

func TestCreateCheckoutSessionRejectsUntrustedCheckoutURL(t *testing.T) {
	client := &Client{httpClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"id":"cs_test_123","url":"http://internal.example/checkout"}`)),
			Header:     make(http.Header),
		}, nil
	})}}

	_, err := client.CreateCheckoutSession(t.Context(), paymentport.StripeCheckoutInput{
		SecretKey:      "sk_test_example",
		SuccessURL:     "https://chat.example/success",
		CancelURL:      "https://chat.example/cancel",
		OrderNo:        "order-123",
		PayAmountCents: 100,
	})
	if err == nil {
		t.Fatal("expected insecure checkout URL to be rejected")
	}
}
