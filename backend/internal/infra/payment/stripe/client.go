package stripe

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	platformtracing "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/observability/tracing"
	paymentport "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/payment"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/security"
)

const (
	checkoutSessionsURL    = "https://api.stripe.com/v1/checkout/sessions"
	checkoutRequestTimeout = 15 * time.Second
	maxCheckoutResponse    = 1 << 20
)

// Client 复用受严格出站策略保护的 Stripe HTTP 客户端。
type Client struct {
	httpClient *http.Client
}

// New 创建 Stripe 适配器。Stripe 使用固定公网端点，因此必须注入严格出站策略。
func New(outboundPolicy security.OutboundPolicy) *Client {
	httpClient := security.NewOutboundHTTPClient(outboundPolicy, checkoutRequestTimeout)
	httpClient.Transport = platformtracing.NewHTTPTransport(httpClient.Transport)
	return &Client{httpClient: httpClient}
}

// CreateCheckoutSession 创建 Stripe Checkout Session。
func (c *Client) CreateCheckoutSession(ctx context.Context, input paymentport.StripeCheckoutInput) (paymentport.CheckoutResult, error) {
	if c == nil || c.httpClient == nil {
		return paymentport.CheckoutResult{}, fmt.Errorf("stripe checkout client is not configured")
	}
	if strings.TrimSpace(input.SecretKey) == "" {
		return paymentport.CheckoutResult{}, fmt.Errorf("stripe secret key is not configured")
	}
	if strings.TrimSpace(input.OrderNo) == "" || input.PayAmountCents <= 0 {
		return paymentport.CheckoutResult{}, fmt.Errorf("stripe checkout order is invalid")
	}
	if !isHTTPURL(input.SuccessURL) || !isHTTPURL(input.CancelURL) {
		return paymentport.CheckoutResult{}, fmt.Errorf("stripe checkout return url is invalid")
	}

	form := checkoutForm(input)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, checkoutSessionsURL, strings.NewReader(form.Encode()))
	if err != nil {
		return paymentport.CheckoutResult{}, fmt.Errorf("build stripe checkout request: %w", err)
	}
	request.SetBasicAuth(input.SecretKey, "")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return paymentport.CheckoutResult{}, fmt.Errorf("request stripe checkout session: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxCheckoutResponse+1))
	if err != nil {
		return paymentport.CheckoutResult{}, fmt.Errorf("read stripe checkout response: %w", err)
	}
	if len(body) > maxCheckoutResponse {
		return paymentport.CheckoutResult{}, fmt.Errorf("stripe checkout response exceeds %d bytes", maxCheckoutResponse)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return paymentport.CheckoutResult{}, fmt.Errorf("stripe checkout request failed: status %d", response.StatusCode)
	}

	var session struct {
		ID  string `json:"id"`
		URL string `json:"url"`
	}
	if err := json.Unmarshal(body, &session); err != nil {
		return paymentport.CheckoutResult{}, fmt.Errorf("decode stripe checkout response: %w", err)
	}
	checkoutID := strings.TrimSpace(session.ID)
	checkoutURL, err := url.Parse(strings.TrimSpace(session.URL))
	if checkoutID == "" || err != nil || !strings.EqualFold(checkoutURL.Scheme, "https") || checkoutURL.Host == "" {
		return paymentport.CheckoutResult{}, fmt.Errorf("stripe checkout response contains an invalid session identity or url")
	}
	return paymentport.CheckoutResult{ID: checkoutID, URL: checkoutURL.String()}, nil
}

func checkoutForm(input paymentport.StripeCheckoutInput) url.Values {
	currency := strings.ToLower(firstNonEmpty(input.PayCurrency, input.BaseCurrency, "USD"))
	form := url.Values{}
	form.Set("mode", "payment")
	form.Set("success_url", input.SuccessURL)
	form.Set("cancel_url", input.CancelURL)
	form.Set("client_reference_id", input.OrderNo)
	form.Set("metadata[order_no]", input.OrderNo)
	form.Set("metadata[order_type]", input.OrderType)
	form.Set("metadata[user_id]", strconv.FormatUint(uint64(input.UserID), 10))
	form.Set("metadata[base_currency]", input.BaseCurrency)
	form.Set("metadata[base_amount_cents]", strconv.FormatInt(input.BaseAmountCents, 10))
	form.Set("metadata[pay_currency]", input.PayCurrency)
	form.Set("metadata[pay_amount_cents]", strconv.FormatInt(input.PayAmountCents, 10))
	form.Set("metadata[fx_rate]", input.FXRate)
	form.Set("line_items[0][quantity]", "1")
	form.Set("line_items[0][price_data][currency]", currency)
	form.Set("line_items[0][price_data][unit_amount]", strconv.FormatInt(input.PayAmountCents, 10))
	form.Set("line_items[0][price_data][product_data][name]", input.ProductName)
	form.Set("line_items[0][price_data][product_data][description]", input.ProductDescription)
	return form
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func isHTTPURL(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" {
		return false
	}
	return strings.EqualFold(parsed.Scheme, "http") || strings.EqualFold(parsed.Scheme, "https")
}
