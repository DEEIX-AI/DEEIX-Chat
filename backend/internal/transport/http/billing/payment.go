package billing

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	appbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/billing"
	domainbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/billing"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/middleware"
	"github.com/gin-gonic/gin"
)

const (
	// Stripe Webhook 只需要验证事件摘要，限制请求体可避免异常回调占用过多内存。
	stripeWebhookMaxBodyBytes = 1 << 20
	stripeResponseMaxBytes    = 1 << 20
)

var stripeHTTPClient = &http.Client{Timeout: 15 * time.Second}

type billingPaymentSettings struct {
	Providers            []string
	USDToCNYRate         float64
	StripePublishableKey string
	StripeSecretKey      string
	StripeWebhookSecret  string
	EPayGatewayURL       string
	EPayTypes            []PaymentTypeResponse
	EPayPID              string
	EPayKey              string
}

// CreateCheckout godoc
// @Summary 创建支付收银台
// @Description 为当前用户创建套餐支付单，并返回支付跳转地址
// @Tags billing
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body CreateCheckoutRequest true "支付参数"
// @Success 200 {object} CheckoutResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /billing/payments/checkout [post]
func (h *Handler) CreateCheckout(c *gin.Context) {
	var req CreateCheckoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	settings, err := h.resolvePaymentSettings(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "resolve payment settings failed")
		return
	}
	provider, err := resolvePaymentProvider(req.PaymentProvider, settings.Providers)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "payment provider is unavailable")
		return
	}

	userID := middleware.MustUserID(c)
	orderType := resolveCheckoutOrderType(req)
	var order *domainbilling.PaymentOrder
	var plan *domainbilling.Plan
	var price *domainbilling.Price
	switch orderType {
	case domainbilling.PaymentOrderTypeTopUp:
		order, err = h.service.CreateTopUpPaymentOrder(c.Request.Context(), appbilling.TopUpPaymentOrderInput{
			UserID:       userID,
			AmountCents:  usdToCents(req.AmountUSD),
			Provider:     provider,
			USDToCNYRate: settings.USDToCNYRate,
		})
	default:
		order, plan, price, err = h.service.CreatePaymentOrder(c.Request.Context(), appbilling.PaymentOrderInput{
			UserID:       userID,
			PriceID:      req.PriceID,
			Cycles:       req.Cycles,
			Provider:     provider,
			USDToCNYRate: settings.USDToCNYRate,
		})
	}
	if err != nil {
		response.ErrorFrom(c, http.StatusBadRequest, err)
		return
	}

	checkoutID := ""
	checkoutURL := ""
	switch provider {
	case domainbilling.PaymentProviderStripe:
		checkoutID, checkoutURL, err = h.createStripeCheckoutSession(c, settings, order, plan, price, req)
	case domainbilling.PaymentProviderEPay:
		checkoutURL, err = h.createEPayCheckoutURL(c, settings, order, plan, price, req)
	default:
		err = appbilling.ErrPaymentProviderUnavailable
	}
	if err != nil {
		response.ErrorWithCode(c, http.StatusInternalServerError, "payment.checkout_failed", "create checkout failed")
		return
	}
	if err = h.service.AttachPaymentCheckout(c.Request.Context(), order.OrderNo, checkoutID, checkoutURL); err != nil {
		response.Error(c, http.StatusInternalServerError, "save checkout failed")
		return
	}
	order.ExternalCheckoutID = checkoutID
	order.CheckoutURL = checkoutURL

	h.recordAudit(
		c,
		userID,
		"billing.payment.checkout",
		"billing_payment_order",
		order.OrderNo,
		map[string]interface{}{
			"provider":          order.Provider,
			"order_type":        order.OrderType,
			"plan_id":           order.PlanID,
			"price_id":          order.PriceID,
			"base_amount_cents": order.BaseAmountCents,
			"base_currency":     order.BaseCurrency,
			"pay_amount_cents":  order.PayAmountCents,
			"pay_currency":      order.PayCurrency,
			"fx_rate":           order.FXRate,
		},
	)

	response.Success(c, CheckoutDataResponse{Checkout: toCheckoutResponse(order)})
}

// StripeWebhook godoc
// @Summary Stripe 支付回调
// @Tags billing
// @Accept json
// @Produce json
// @Success 200 {object} response.Envelope
// @Router /billing/payments/stripe/webhook [post]
func (h *Handler) StripeWebhook(c *gin.Context) {
	settings, err := h.resolvePaymentSettings(c.Request.Context())
	if err != nil || strings.TrimSpace(settings.StripeWebhookSecret) == "" {
		response.Error(c, http.StatusBadRequest, "stripe webhook is not configured")
		return
	}
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, stripeWebhookMaxBodyBytes+1))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "read webhook body failed")
		return
	}
	if len(body) > stripeWebhookMaxBodyBytes {
		response.Error(c, http.StatusRequestEntityTooLarge, "webhook body too large")
		return
	}
	if !verifyStripeSignature(body, c.GetHeader("Stripe-Signature"), settings.StripeWebhookSecret, 5*time.Minute) {
		response.Error(c, http.StatusBadRequest, "invalid stripe signature")
		return
	}

	var event struct {
		Type string `json:"type"`
		Data struct {
			Object stripeCheckoutSession `json:"object"`
		} `json:"data"`
	}
	if err = json.Unmarshal(body, &event); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid stripe event")
		return
	}
	if event.Type != "checkout.session.completed" {
		response.Success(c, PaymentWebhookIgnoredResponse{Ignored: true})
		return
	}
	session := event.Data.Object
	if session.PaymentStatus != "" && session.PaymentStatus != "paid" {
		response.Success(c, PaymentWebhookIgnoredResponse{Ignored: true})
		return
	}
	orderNo := strings.TrimSpace(session.Metadata["order_no"])
	if orderNo == "" {
		orderNo = strings.TrimSpace(session.ClientReferenceID)
	}
	if orderNo == "" {
		response.Error(c, http.StatusBadRequest, "missing order_no")
		return
	}
	order, err := h.service.GetPaymentOrder(c.Request.Context(), orderNo)
	if err != nil {
		response.ErrorFrom(c, http.StatusBadRequest, err)
		return
	}
	if err = validateStripeCheckoutSession(order, session); err != nil {
		response.ErrorFrom(c, http.StatusBadRequest, err)
		return
	}
	order, activated, err := h.service.CompletePaymentOrder(c.Request.Context(), orderNo, firstNonEmpty(session.PaymentIntent, session.ID), time.Now())
	if err != nil {
		response.ErrorFrom(c, http.StatusBadRequest, err)
		return
	}
	h.writePaymentAudit(c, order, activated, "stripe.webhook")
	response.Success(c, PaymentWebhookOKResponse{OK: true})
}

// EPayNotify godoc
// @Summary 易支付异步通知
// @Tags billing
// @Produce text/plain
// @Success 200 {string} string "success"
// @Router /billing/payments/epay/notify [post]
func (h *Handler) EPayNotify(c *gin.Context) {
	settings, err := h.resolvePaymentSettings(c.Request.Context())
	if err != nil || strings.TrimSpace(settings.EPayKey) == "" {
		c.String(http.StatusBadRequest, "fail")
		return
	}
	values := collectEPayNotifyValues(c)
	if !verifyEPaySign(values, settings.EPayKey) {
		c.String(http.StatusBadRequest, "fail")
		return
	}
	if strings.TrimSpace(values.Get("trade_status")) != "TRADE_SUCCESS" {
		c.String(http.StatusOK, "success")
		return
	}
	orderNo := strings.TrimSpace(values.Get("out_trade_no"))
	if orderNo == "" {
		c.String(http.StatusBadRequest, "fail")
		return
	}
	order, err := h.service.GetPaymentOrder(c.Request.Context(), orderNo)
	if err != nil {
		c.String(http.StatusBadRequest, "fail")
		return
	}
	if err = validateEPayNotification(order, values, settings); err != nil {
		c.String(http.StatusBadRequest, "fail")
		return
	}
	order, activated, err := h.service.CompletePaymentOrder(c.Request.Context(), orderNo, values.Get("trade_no"), time.Now())
	if err != nil {
		c.String(http.StatusBadRequest, "fail")
		return
	}
	h.writePaymentAudit(c, order, activated, "epay.notify")
	c.String(http.StatusOK, "success")
}

func (h *Handler) resolvePaymentSettings(ctx context.Context) (billingPaymentSettings, error) {
	if h.settings == nil {
		return billingPaymentSettings{}, appbilling.ErrPaymentProviderUnavailable
	}
	values, err := h.settings.RuntimeValuesByNamespace(ctx, "billing")
	if err != nil {
		return billingPaymentSettings{}, err
	}
	return billingPaymentSettings{
		Providers:            normalizePaymentProviders(values["payment_providers"]),
		USDToCNYRate:         parsePositiveFloat(values["usd_to_cny_rate"], 7.2),
		StripePublishableKey: values["stripe_publishable_key"],
		StripeSecretKey:      values["stripe_secret_key"],
		StripeWebhookSecret:  values["stripe_webhook_secret"],
		EPayGatewayURL:       values["epay_gateway_url"],
		EPayTypes:            normalizeEPayTypes(values["epay_types"]),
		EPayPID:              values["epay_pid"],
		EPayKey:              values["epay_key"],
	}, nil
}

type stripeCheckoutSession struct {
	ID                string            `json:"id"`
	URL               string            `json:"url"`
	PaymentStatus     string            `json:"payment_status"`
	PaymentIntent     string            `json:"payment_intent"`
	ClientReferenceID string            `json:"client_reference_id"`
	Created           int64             `json:"created"`
	AmountTotal       int64             `json:"amount_total"`
	Currency          string            `json:"currency"`
	Metadata          map[string]string `json:"metadata"`
}

func (h *Handler) createStripeCheckoutSession(
	c *gin.Context,
	settings billingPaymentSettings,
	order *domainbilling.PaymentOrder,
	plan *domainbilling.Plan,
	price *domainbilling.Price,
	req CreateCheckoutRequest,
) (string, string, error) {
	_ = price
	if strings.TrimSpace(settings.StripeSecretKey) == "" {
		return "", "", fmt.Errorf("stripe secret key is not configured")
	}
	successURL, err := h.paymentReturnURL(c, req.SuccessURL, "/settings?section=account&payment=success")
	if err != nil {
		return "", "", err
	}
	cancelURL, err := h.paymentReturnURL(c, req.CancelURL, "/settings?section=account&payment=cancel")
	if err != nil {
		return "", "", err
	}
	currency := strings.ToLower(firstNonEmpty(order.PayCurrency, order.BaseCurrency, "USD"))
	form := url.Values{}
	form.Set("mode", "payment")
	form.Set("success_url", successURL)
	form.Set("cancel_url", cancelURL)
	form.Set("client_reference_id", order.OrderNo)
	form.Set("metadata[order_no]", order.OrderNo)
	form.Set("metadata[order_type]", order.OrderType)
	form.Set("metadata[user_id]", strconv.FormatUint(uint64(order.UserID), 10))
	form.Set("metadata[base_currency]", order.BaseCurrency)
	form.Set("metadata[base_amount_cents]", strconv.FormatInt(order.BaseAmountCents, 10))
	form.Set("metadata[pay_currency]", order.PayCurrency)
	form.Set("metadata[pay_amount_cents]", strconv.FormatInt(order.PayAmountCents, 10))
	form.Set("metadata[fx_rate]", order.FXRate)
	form.Set("line_items[0][quantity]", "1")
	form.Set("line_items[0][price_data][currency]", currency)
	form.Set("line_items[0][price_data][unit_amount]", strconv.FormatInt(order.PayAmountCents, 10))
	form.Set("line_items[0][price_data][product_data][name]", paymentProductName(order, plan))
	form.Set("line_items[0][price_data][product_data][description]", paymentProductDescription(order, plan))

	httpReq, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, "https://api.stripe.com/v1/checkout/sessions", strings.NewReader(form.Encode()))
	if err != nil {
		return "", "", err
	}
	httpReq.SetBasicAuth(settings.StripeSecretKey, "")
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := stripeHTTPClient.Do(httpReq)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, stripeResponseMaxBytes+1))
	if len(body) > stripeResponseMaxBytes {
		return "", "", fmt.Errorf("stripe response is too large")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", fmt.Errorf("stripe checkout failed with status %d", resp.StatusCode)
	}
	var session stripeCheckoutSession
	if err = json.Unmarshal(body, &session); err != nil {
		return "", "", err
	}
	if strings.TrimSpace(session.URL) == "" {
		return "", "", fmt.Errorf("stripe checkout url is missing")
	}
	return session.ID, session.URL, nil
}

func (h *Handler) createEPayCheckoutURL(
	c *gin.Context,
	settings billingPaymentSettings,
	order *domainbilling.PaymentOrder,
	plan *domainbilling.Plan,
	price *domainbilling.Price,
	req CreateCheckoutRequest,
) (string, error) {
	_ = price
	gateway := strings.TrimRight(strings.TrimSpace(settings.EPayGatewayURL), "/")
	if gateway == "" || strings.TrimSpace(settings.EPayPID) == "" || strings.TrimSpace(settings.EPayKey) == "" {
		return "", fmt.Errorf("epay settings are incomplete")
	}
	if !isHTTPURL(gateway) {
		return "", fmt.Errorf("epay gateway url must be an http(s) url")
	}
	notifyURL, err := h.paymentNotifyURL(c, "/api/v1/billing/payments/epay/notify")
	if err != nil {
		return "", err
	}
	returnURL, err := h.paymentReturnURL(c, req.SuccessURL, "/settings?section=account&payment=success")
	if err != nil {
		return "", err
	}

	params := url.Values{}
	params.Set("pid", settings.EPayPID)
	epayType, err := resolveEPayType(req.EPayType, settings.EPayTypes)
	if err != nil {
		return "", err
	}
	params.Set("type", epayType)
	params.Set("out_trade_no", order.OrderNo)
	params.Set("notify_url", notifyURL)
	params.Set("return_url", returnURL)
	params.Set("name", paymentProductName(order, plan))
	params.Set("money", fmt.Sprintf("%.2f", float64(order.PayAmountCents)/100))
	params.Set("sign", signEPayValues(params, settings.EPayKey))
	params.Set("sign_type", "MD5")
	return gateway + "/submit.php?" + params.Encode(), nil
}

func verifyStripeSignature(payload []byte, header string, secret string, tolerance time.Duration) bool {
	parts := strings.Split(header, ",")
	timestamp := ""
	signatures := make([]string, 0, 1)
	for _, part := range parts {
		key, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		switch key {
		case "t":
			timestamp = value
		case "v1":
			signatures = append(signatures, value)
		}
	}
	if timestamp == "" || len(signatures) == 0 || strings.TrimSpace(secret) == "" {
		return false
	}
	if parsed, err := strconv.ParseInt(timestamp, 10, 64); err != nil || absDuration(time.Since(time.Unix(parsed, 0))) > tolerance {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write(payload)
	expected := mac.Sum(nil)
	for _, signature := range signatures {
		got, err := hex.DecodeString(signature)
		if err == nil && hmac.Equal(got, expected) {
			return true
		}
	}
	return false
}

func absDuration(value time.Duration) time.Duration {
	if value < 0 {
		return -value
	}
	return value
}

func collectEPayNotifyValues(c *gin.Context) url.Values {
	values := url.Values{}
	for key, items := range c.Request.URL.Query() {
		for _, item := range items {
			values.Add(key, item)
		}
	}
	if err := c.Request.ParseForm(); err == nil {
		for key, items := range c.Request.PostForm {
			values.Del(key)
			for _, item := range items {
				values.Add(key, item)
			}
		}
	}
	return values
}

func verifyEPaySign(values url.Values, key string) bool {
	provided := strings.ToLower(strings.TrimSpace(values.Get("sign")))
	if provided == "" {
		return false
	}
	expected := signEPayValues(values, key)
	return hmac.Equal([]byte(provided), []byte(expected))
}

func signEPayValues(values url.Values, key string) string {
	keys := make([]string, 0, len(values))
	for itemKey := range values {
		if itemKey == "sign" || itemKey == "sign_type" || strings.TrimSpace(values.Get(itemKey)) == "" {
			continue
		}
		keys = append(keys, itemKey)
	}
	sort.Strings(keys)
	var buffer bytes.Buffer
	for index, itemKey := range keys {
		if index > 0 {
			buffer.WriteByte('&')
		}
		buffer.WriteString(itemKey)
		buffer.WriteByte('=')
		buffer.WriteString(values.Get(itemKey))
	}
	buffer.WriteString(key)
	sum := md5.Sum(buffer.Bytes())
	return hex.EncodeToString(sum[:])
}

func (h *Handler) writePaymentAudit(c *gin.Context, order *domainbilling.PaymentOrder, activated bool, source string) {
	if order == nil {
		return
	}
	h.recordAudit(
		c,
		order.UserID,
		"billing.payment.completed",
		"billing_payment_order",
		order.OrderNo,
		map[string]interface{}{
			"provider":          order.Provider,
			"order_type":        order.OrderType,
			"status":            order.Status,
			"activated":         activated,
			"source":            source,
			"base_amount_cents": order.BaseAmountCents,
			"base_currency":     order.BaseCurrency,
			"pay_amount_cents":  order.PayAmountCents,
			"pay_currency":      order.PayCurrency,
			"fx_rate":           order.FXRate,
		},
	)
}

func validateStripeCheckoutSession(order *domainbilling.PaymentOrder, session stripeCheckoutSession) error {
	if order == nil {
		return fmt.Errorf("order not found")
	}
	if order.Provider != domainbilling.PaymentProviderStripe {
		return fmt.Errorf("provider mismatch")
	}
	if strings.TrimSpace(session.ID) != "" && strings.TrimSpace(order.ExternalCheckoutID) != "" && session.ID != order.ExternalCheckoutID {
		return fmt.Errorf("checkout id mismatch")
	}
	if session.AmountTotal != order.PayAmountCents {
		return fmt.Errorf("amount mismatch")
	}
	if strings.ToUpper(strings.TrimSpace(session.Currency)) != strings.ToUpper(order.PayCurrency) {
		return fmt.Errorf("currency mismatch")
	}
	return nil
}

func validateEPayNotification(order *domainbilling.PaymentOrder, values url.Values, settings billingPaymentSettings) error {
	if order == nil {
		return fmt.Errorf("order not found")
	}
	if order.Provider != domainbilling.PaymentProviderEPay {
		return fmt.Errorf("provider mismatch")
	}
	if strings.TrimSpace(values.Get("pid")) != strings.TrimSpace(settings.EPayPID) {
		return fmt.Errorf("merchant mismatch")
	}
	if strings.ToUpper(strings.TrimSpace(order.PayCurrency)) != "CNY" {
		return fmt.Errorf("currency mismatch")
	}
	expected := fmt.Sprintf("%.2f", float64(order.PayAmountCents)/100)
	actual := strings.TrimSpace(values.Get("money"))
	if actual != expected {
		parsed, err := strconv.ParseFloat(actual, 64)
		if err != nil || int64(parsed*100+0.5) != order.PayAmountCents {
			return fmt.Errorf("amount mismatch")
		}
	}
	return nil
}

func resolvePaymentProvider(requested string, enabledProviders []string) (string, error) {
	if len(enabledProviders) == 0 {
		return "", appbilling.ErrPaymentProviderUnavailable
	}
	selected := strings.TrimSpace(requested)
	if selected == "" {
		return enabledProviders[0], nil
	}
	for _, provider := range enabledProviders {
		if selected == provider {
			return selected, nil
		}
	}
	return "", appbilling.ErrPaymentProviderUnavailable
}

func normalizePaymentProviders(raw string) []string {
	parts := strings.Split(raw, ",")
	results := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		provider := strings.ToLower(strings.TrimSpace(part))
		switch provider {
		case domainbilling.PaymentProviderStripe, domainbilling.PaymentProviderEPay:
		default:
			continue
		}
		if _, ok := seen[provider]; ok {
			continue
		}
		seen[provider] = struct{}{}
		results = append(results, provider)
	}
	return results
}

func resolveEPayType(requested string, enabledTypes []PaymentTypeResponse) (string, error) {
	types := enabledTypes
	if len(types) == 0 {
		types = defaultEPayTypes()
	}
	selected := strings.TrimSpace(requested)
	if selected == "" {
		return types[0].Type, nil
	}
	for _, item := range types {
		if selected == item.Type {
			return selected, nil
		}
	}
	return "", fmt.Errorf("epay payment type is not supported")
}

func normalizeEPayTypes(raw string) []PaymentTypeResponse {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultEPayTypes()
	}
	var parsed []PaymentTypeResponse
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return defaultEPayTypes()
	}
	results := make([]PaymentTypeResponse, 0, len(parsed))
	seen := make(map[string]struct{}, len(parsed))
	for _, item := range parsed {
		name := strings.TrimSpace(item.Name)
		value := strings.TrimSpace(item.Type)
		if name == "" || value == "" || !validEPayType(value) {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		results = append(results, PaymentTypeResponse{Name: name, Type: value})
		if len(results) >= 10 {
			break
		}
	}
	if len(results) == 0 {
		return defaultEPayTypes()
	}
	return results
}

func defaultEPayTypes() []PaymentTypeResponse {
	return []PaymentTypeResponse{
		{Name: "支付宝", Type: "alipay"},
		{Name: "微信支付", Type: "wxpay"},
	}
}

func validEPayType(value string) bool {
	if len(value) > 32 {
		return false
	}
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '_' || char == '-' {
			continue
		}
		return false
	}
	return true
}

func parsePositiveFloat(value string, fallback float64) float64 {
	parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func resolveCheckoutOrderType(req CreateCheckoutRequest) string {
	switch strings.TrimSpace(req.OrderType) {
	case domainbilling.PaymentOrderTypeTopUp:
		return domainbilling.PaymentOrderTypeTopUp
	case domainbilling.PaymentOrderTypeSubscription:
		return domainbilling.PaymentOrderTypeSubscription
	default:
		if req.PriceID == 0 && req.AmountUSD > 0 {
			return domainbilling.PaymentOrderTypeTopUp
		}
		return domainbilling.PaymentOrderTypeSubscription
	}
}

func paymentProductName(order *domainbilling.PaymentOrder, plan *domainbilling.Plan) string {
	if order != nil && order.OrderType == domainbilling.PaymentOrderTypeTopUp {
		return "按量余额充值"
	}
	if plan != nil {
		return firstNonEmpty(plan.Name, plan.Code)
	}
	return "订阅方案"
}

func paymentProductDescription(order *domainbilling.PaymentOrder, plan *domainbilling.Plan) string {
	if order != nil && order.OrderType == domainbilling.PaymentOrderTypeTopUp {
		return fmt.Sprintf("充值 $%.2f 至按量余额", float64(order.BaseAmountCents)/100)
	}
	if plan != nil {
		return firstNonEmpty(plan.Description, plan.Code)
	}
	return "订阅方案支付"
}

func requestBaseURL(c *gin.Context) string {
	proto := firstNonEmpty(c.GetHeader("X-Forwarded-Proto"), "http")
	host := firstNonEmpty(c.GetHeader("X-Forwarded-Host"), c.Request.Host)
	return proto + "://" + host
}

func requestOrigin(c *gin.Context) string {
	origin := strings.TrimRight(c.GetHeader("Origin"), "/")
	if isHTTPURL(origin) {
		return origin
	}
	return requestBaseURL(c)
}

func (h *Handler) paymentNotifyURL(c *gin.Context, path string) (string, error) {
	baseURL, err := h.publicAPIBaseURL(c)
	if err != nil {
		return "", err
	}
	return joinPublicBaseURL(baseURL, path)
}

func (h *Handler) paymentReturnURL(c *gin.Context, raw string, defaultPath string) (string, error) {
	baseURL, err := h.publicWebBaseURL(c)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(raw) == "" {
		return joinPublicBaseURL(baseURL, defaultPath)
	}
	return sameOriginPublicURL(baseURL, raw)
}

func (h *Handler) publicAPIBaseURL(c *gin.Context) (string, error) {
	if h != nil && h.cfg != nil {
		cfg := h.cfg.Snapshot()
		if value := strings.TrimRight(strings.TrimSpace(cfg.PublicAPIBaseURL), "/"); value != "" {
			if !isHTTPURL(value) {
				return "", fmt.Errorf("public api base url must be an http(s) url")
			}
			return value, nil
		}
		if strings.EqualFold(strings.TrimSpace(cfg.Env), "prod") {
			return "", fmt.Errorf("public api base url is not configured")
		}
	}
	// 开发环境允许从请求元数据推导公开地址，生产环境必须显式配置，避免代理头污染支付回调地址。
	return strings.TrimRight(requestBaseURL(c), "/"), nil
}

func (h *Handler) publicWebBaseURL(c *gin.Context) (string, error) {
	if h != nil && h.cfg != nil {
		cfg := h.cfg.Snapshot()
		if value := strings.TrimRight(strings.TrimSpace(cfg.PublicWebBaseURL), "/"); value != "" {
			if !isHTTPURL(value) {
				return "", fmt.Errorf("public web base url must be an http(s) url")
			}
			return value, nil
		}
		if strings.EqualFold(strings.TrimSpace(cfg.Env), "prod") {
			return "", fmt.Errorf("public web base url is not configured")
		}
	}
	// 开发环境允许使用请求 Origin，生产环境必须显式配置，避免将支付完成页跳转到非预期来源。
	return strings.TrimRight(requestOrigin(c), "/"), nil
}

func joinPublicBaseURL(baseURL string, path string) (string, error) {
	if !isHTTPURL(baseURL) {
		return "", fmt.Errorf("public base url must be an http(s) url")
	}
	if strings.TrimSpace(path) == "" || strings.HasPrefix(strings.TrimSpace(path), "//") {
		return "", fmt.Errorf("payment redirect path is invalid")
	}
	parsed, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil {
		return "", err
	}
	relative, err := url.Parse(path)
	if err != nil || relative.IsAbs() {
		return "", fmt.Errorf("payment redirect path is invalid")
	}
	return parsed.ResolveReference(relative).String(), nil
}

func sameOriginPublicURL(baseURL string, raw string) (string, error) {
	base, err := url.Parse(strings.TrimRight(strings.TrimSpace(baseURL), "/"))
	if err != nil || base.Scheme == "" || base.Host == "" {
		return "", fmt.Errorf("public base url is invalid")
	}
	value := strings.TrimSpace(raw)
	if strings.HasPrefix(value, "//") {
		return "", fmt.Errorf("payment return url must use the configured public web origin")
	}
	if strings.HasPrefix(value, "/") {
		return joinPublicBaseURL(baseURL, value)
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("payment return url is invalid")
	}
	// 外部传入的 return_url 只能指向配置的前端站点，避免支付完成后被用作开放跳转。
	if !strings.EqualFold(parsed.Scheme, base.Scheme) || !strings.EqualFold(parsed.Host, base.Host) {
		return "", fmt.Errorf("payment return url must use the configured public web origin")
	}
	return parsed.String(), nil
}

func isHTTPURL(value string) bool {
	parsed, err := url.Parse(strings.TrimSpace(value))
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
