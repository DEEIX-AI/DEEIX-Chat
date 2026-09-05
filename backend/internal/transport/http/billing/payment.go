package billing

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/textutil"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	appbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/billing"
	domainbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/billing"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/apperr"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/middleware"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const (
	// Stripe Webhook 只需要验证事件摘要，限制请求体可避免异常回调占用过多内存。
	stripeWebhookMaxBodyBytes = 1 << 20
)

type billingPaymentSettings struct {
	Providers            []string
	USDToCNYRate         float64
	DisplayCurrency      string
	StripePublishableKey string
	StripeSecretKey      string
	StripeWebhookSecret  string
	EPayGatewayURL       string
	EPayTypes            []PaymentTypeResponse
	EPayPID              string
	EPayKey              string
}

type paymentCheckoutPreparation struct {
	successURL string
	cancelURL  string
	notifyURL  string
	epayType   string
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
		response.InternalError(c)
		return
	}
	provider, err := resolvePaymentProvider(req.PaymentProvider, settings.Providers)
	if err != nil {
		response.ErrorFrom(c, http.StatusBadRequest, errPaymentProviderUnavailable)
		return
	}
	preparation, err := h.preparePaymentCheckout(c, provider, settings, req)
	if err != nil {
		h.respondPaymentCheckoutError(c, provider, "validate", err)
		return
	}

	userID := middleware.MustUserID(c)
	orderType := resolveCheckoutOrderType(req)
	var order *domainbilling.PaymentOrder
	var plan *domainbilling.Plan
	switch orderType {
	case domainbilling.PaymentOrderTypeTopUp:
		order, err = h.service.CreateTopUpPaymentOrder(c.Request.Context(), appbilling.TopUpPaymentOrderInput{
			UserID:               userID,
			AmountMinorUnits:     req.AmountMinorUnits,
			AmountCurrency:       settings.DisplayCurrency,
			Provider:             provider,
			USDToCNYRate:         settings.USDToCNYRate,
			PreferredPayCurrency: settings.DisplayCurrency,
		})
	default:
		order, plan, _, err = h.service.CreatePaymentOrder(c.Request.Context(), appbilling.PaymentOrderInput{
			UserID:               userID,
			PriceID:              req.PriceID,
			Cycles:               optionalIntValue(req.Cycles),
			Provider:             provider,
			USDToCNYRate:         settings.USDToCNYRate,
			PreferredPayCurrency: settings.DisplayCurrency,
		})
	}
	if err != nil {
		if isPublicPaymentOrderError(err) {
			response.ErrorFrom(c, http.StatusBadRequest, err)
		} else {
			response.InternalError(c)
		}
		return
	}

	checkoutID := ""
	checkoutURL := ""
	switch provider {
	case domainbilling.PaymentProviderStripe:
		checkoutID, checkoutURL, err = h.createStripeCheckoutSession(c, settings, order, plan, preparation)
	case domainbilling.PaymentProviderEPay:
		checkoutURL, err = h.createEPayCheckoutURL(c, settings, order, plan, preparation)
	default:
		err = appbilling.ErrPaymentProviderUnavailable
	}
	if err != nil {
		h.respondPaymentCheckoutError(c, provider, "create", err)
		return
	}
	if err = h.service.AttachPaymentCheckout(c.Request.Context(), order.OrderNo, checkoutID, checkoutURL); err != nil {
		if h.logger != nil {
			h.logger.Error("billing payment checkout persistence failed",
				zap.String("request_id", middleware.MustRequestID(c)),
				zap.String("provider", provider),
				zap.String("order_no", order.OrderNo),
				zap.Error(err),
			)
		}
		response.InternalError(c)
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
		map[string]any{
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
		response.ErrorFrom(c, http.StatusBadRequest, errStripeWebhookNotConfigured)
		return
	}
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, stripeWebhookMaxBodyBytes+1))
	if err != nil {
		response.ErrorFrom(c, http.StatusBadRequest, errInvalidWebhookBody)
		return
	}
	if len(body) > stripeWebhookMaxBodyBytes {
		response.ErrorFrom(c, http.StatusRequestEntityTooLarge, errWebhookBodyTooLarge)
		return
	}
	if !verifyStripeSignature(body, c.GetHeader("Stripe-Signature"), settings.StripeWebhookSecret, 5*time.Minute) {
		response.ErrorFrom(c, http.StatusBadRequest, errInvalidStripeSignature)
		return
	}

	var event struct {
		Type string `json:"type"`
		Data struct {
			Object stripeCheckoutSession `json:"object"`
		} `json:"data"`
	}
	if err = json.Unmarshal(body, &event); err != nil {
		response.ErrorFrom(c, http.StatusBadRequest, errInvalidStripeEvent)
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
		response.ErrorFrom(c, http.StatusBadRequest, errOrderNoRequired)
		return
	}
	order, err := h.service.GetPaymentOrder(c.Request.Context(), orderNo)
	if err != nil {
		writePaymentWebhookError(c, err)
		return
	}
	if err = validateStripeCheckoutSession(order, session); err != nil {
		writePaymentWebhookError(c, err)
		return
	}
	order, activated, err := h.service.CompletePaymentOrder(c.Request.Context(), orderNo, textutil.FirstNonEmpty(session.PaymentIntent, session.ID), time.Now())
	if err != nil {
		writePaymentWebhookError(c, err)
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
	if h.paymentCheckout == nil || !h.paymentCheckout.VerifyEPaySignature(values, settings.EPayKey) {
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
		DisplayCurrency:      normalizePaymentDisplayCurrency(values["display_currency"]),
		StripePublishableKey: values["stripe_publishable_key"],
		StripeSecretKey:      values["stripe_secret_key"],
		StripeWebhookSecret:  values["stripe_webhook_secret"],
		EPayGatewayURL:       values["epay_gateway_url"],
		EPayTypes:            normalizeEPayTypes(values["epay_types"]),
		EPayPID:              values["epay_pid"],
		EPayKey:              values["epay_key"],
	}, nil
}

func (h *Handler) preparePaymentCheckout(
	c *gin.Context,
	provider string,
	settings billingPaymentSettings,
	req CreateCheckoutRequest,
) (paymentCheckoutPreparation, error) {
	preparation := paymentCheckoutPreparation{}
	switch provider {
	case domainbilling.PaymentProviderStripe:
		if strings.TrimSpace(settings.StripeSecretKey) == "" {
			return preparation, appbilling.ErrPaymentProviderUnavailable
		}
		var err error
		preparation.successURL, err = h.paymentReturnURL(c, req.SuccessURL, "/settings?section=account&payment=success")
		if err != nil {
			return preparation, err
		}
		preparation.cancelURL, err = h.paymentReturnURL(c, req.CancelURL, "/settings?section=account&payment=cancel")
		if err != nil {
			return preparation, err
		}
	case domainbilling.PaymentProviderEPay:
		if strings.TrimSpace(settings.EPayPID) == "" || strings.TrimSpace(settings.EPayKey) == "" {
			return preparation, appbilling.ErrPaymentProviderUnavailable
		}
		if _, err := domainbilling.ResolveEPaySubmitURL(settings.EPayGatewayURL); err != nil {
			return preparation, err
		}
		var err error
		preparation.epayType, err = resolveEPayType(req.EPayType, settings.EPayTypes)
		if err != nil {
			return preparation, err
		}
		preparation.notifyURL, err = h.paymentNotifyURL(c, "/api/v1/billing/payments/epay/notify")
		if err != nil {
			return preparation, err
		}
		preparation.successURL, err = h.paymentReturnURL(c, req.SuccessURL, "/settings?section=account&payment=success")
		if err != nil {
			return preparation, err
		}
	default:
		return preparation, appbilling.ErrPaymentProviderUnavailable
	}
	return preparation, nil
}

func (h *Handler) respondPaymentCheckoutError(c *gin.Context, provider string, stage string, err error) {
	logger := h.logger
	if logger == nil {
		logger = zap.NewNop()
	}
	logFields := []zap.Field{
		zap.String("request_id", middleware.MustRequestID(c)),
		zap.String("provider", provider),
		zap.String("stage", stage),
		zap.Error(err),
	}
	if stage == "validate" {
		logger.Warn("billing payment checkout validation failed", logFields...)
	} else {
		logger.Error("billing payment checkout creation failed", logFields...)
	}

	switch {
	case errors.Is(err, domainbilling.ErrEPayGatewayInvalid):
		response.ErrorWithCode(c, http.StatusServiceUnavailable, "payment.epay_gateway_invalid")
	case errors.Is(err, appbilling.ErrPaymentProviderUnavailable):
		response.ErrorWithCode(c, http.StatusServiceUnavailable, "payment.provider_unavailable")
	case errors.Is(err, appbilling.ErrEPayTypeUnsupported),
		errors.Is(err, errPaymentReturnURLInvalid),
		errors.Is(err, errPaymentReturnURLCrossOrigin):
		response.ErrorFrom(c, http.StatusBadRequest, err)
	case stage == "validate":
		response.ErrorWithCode(c, http.StatusServiceUnavailable, "payment.provider_unavailable")
	default:
		response.ErrorWithCode(c, http.StatusBadGateway, "payment.checkout_failed")
	}
}

func normalizePaymentDisplayCurrency(value string) string {
	if strings.EqualFold(strings.TrimSpace(value), "CNY") {
		return "CNY"
	}
	return "USD"
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
	preparation paymentCheckoutPreparation,
) (string, string, error) {
	if h.paymentCheckout == nil {
		return "", "", appbilling.ErrPaymentProviderUnavailable
	}
	checkout, err := h.paymentCheckout.CreateStripeCheckoutSession(c.Request.Context(), appbilling.StripeCheckoutInput{
		SecretKey:  settings.StripeSecretKey,
		SuccessURL: preparation.successURL,
		CancelURL:  preparation.cancelURL,
		Order:      order,
		Plan:       plan,
	})
	if err != nil {
		return "", "", err
	}
	return checkout.ID, checkout.URL, nil
}

func (h *Handler) createEPayCheckoutURL(
	c *gin.Context,
	settings billingPaymentSettings,
	order *domainbilling.PaymentOrder,
	plan *domainbilling.Plan,
	preparation paymentCheckoutPreparation,
) (string, error) {
	if h.paymentCheckout == nil {
		return "", appbilling.ErrPaymentProviderUnavailable
	}
	checkout, err := h.paymentCheckout.CreateEPayCheckout(c.Request.Context(), appbilling.EPayCheckoutInput{
		GatewayURL:  settings.EPayGatewayURL,
		MerchantID:  settings.EPayPID,
		MerchantKey: settings.EPayKey,
		PaymentType: preparation.epayType,
		NotifyURL:   preparation.notifyURL,
		ReturnURL:   preparation.successURL,
		Order:       order,
		Plan:        plan,
	})
	if err != nil {
		return "", err
	}
	return checkout.URL, nil
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
		map[string]any{
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

func isPublicPaymentOrderError(err error) bool {
	switch {
	case errors.Is(err, appbilling.ErrPaymentRequired),
		errors.Is(err, appbilling.ErrPaymentProviderUnavailable),
		errors.Is(err, appbilling.ErrBillingPlanNotFound),
		errors.Is(err, appbilling.ErrInvalidPaymentOrder):
		return true
	default:
		return false
	}
}

func writePaymentWebhookError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, appbilling.ErrPaymentOrderNotFound),
		errors.Is(err, appbilling.ErrInvalidPaymentOrder),
		errors.Is(err, appbilling.ErrPaymentOrderStateInvalid),
		errors.Is(err, errPaymentNotificationMismatch):
		response.ErrorFrom(c, http.StatusBadRequest, err)
	default:
		response.InternalError(c)
	}
}

func validateStripeCheckoutSession(order *domainbilling.PaymentOrder, session stripeCheckoutSession) error {
	if order == nil {
		return appbilling.ErrPaymentOrderNotFound
	}
	if order.Provider != domainbilling.PaymentProviderStripe {
		return errPaymentNotificationMismatch
	}
	if strings.TrimSpace(session.ID) != "" && strings.TrimSpace(order.ExternalCheckoutID) != "" && session.ID != order.ExternalCheckoutID {
		return errPaymentNotificationMismatch
	}
	if session.AmountTotal != order.PayAmountCents {
		return errPaymentNotificationMismatch
	}
	if !strings.EqualFold(strings.TrimSpace(session.Currency), order.PayCurrency) {
		return errPaymentNotificationMismatch
	}
	return nil
}

func validateEPayNotification(order *domainbilling.PaymentOrder, values url.Values, settings billingPaymentSettings) error {
	if order == nil {
		return appbilling.ErrPaymentOrderNotFound
	}
	if order.Provider != domainbilling.PaymentProviderEPay {
		return errPaymentNotificationMismatch
	}
	if strings.TrimSpace(values.Get("pid")) != strings.TrimSpace(settings.EPayPID) {
		return errPaymentNotificationMismatch
	}
	if !strings.EqualFold(strings.TrimSpace(order.PayCurrency), "CNY") {
		return errPaymentNotificationMismatch
	}
	expected := domainbilling.FormatEPayAmount(order.PayAmountCents)
	actual := strings.TrimSpace(values.Get("money"))
	if actual != expected {
		parsed, err := strconv.ParseFloat(actual, 64)
		if err != nil || int64(parsed*100+0.5) != order.PayAmountCents {
			return errPaymentNotificationMismatch
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
	return "", appbilling.ErrEPayTypeUnsupported
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
		if req.PriceID == 0 && req.AmountMinorUnits > 0 {
			return domainbilling.PaymentOrderTypeTopUp
		}
		return domainbilling.PaymentOrderTypeSubscription
	}
}

func requestBaseURL(c *gin.Context) string {
	proto := textutil.FirstNonEmpty(c.GetHeader("X-Forwarded-Proto"), "http")
	host := textutil.FirstNonEmpty(c.GetHeader("X-Forwarded-Host"), c.Request.Host)
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

// 客户端传入的 return_url 校验失败属于请求错误（400），与公开地址未配置等服务端问题区分开。
var (
	errPaymentReturnURLInvalid     = apperr.New("payment.return_url_invalid", "payment return url is invalid")
	errPaymentReturnURLCrossOrigin = apperr.New("payment.return_url_cross_origin", "payment return url must use the configured public web origin")
)

func sameOriginPublicURL(baseURL string, raw string) (string, error) {
	base, err := url.Parse(strings.TrimRight(strings.TrimSpace(baseURL), "/"))
	if err != nil || base.Scheme == "" || base.Host == "" {
		return "", fmt.Errorf("public base url is invalid")
	}
	value := strings.TrimSpace(raw)
	if strings.HasPrefix(value, "//") {
		return "", errPaymentReturnURLCrossOrigin
	}
	if strings.HasPrefix(value, "/") {
		joined, err := joinPublicBaseURL(baseURL, value)
		if err != nil {
			return "", fmt.Errorf("%w: %w", errPaymentReturnURLInvalid, err)
		}
		return joined, nil
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", errPaymentReturnURLInvalid
	}
	// 外部传入的 return_url 只能指向配置的前端站点，避免支付完成后被用作开放跳转。
	if !strings.EqualFold(parsed.Scheme, base.Scheme) || !strings.EqualFold(parsed.Host, base.Host) {
		return "", errPaymentReturnURLCrossOrigin
	}
	return parsed.String(), nil
}

func isHTTPURL(value string) bool {
	parsed, err := url.Parse(strings.TrimSpace(value))
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}
