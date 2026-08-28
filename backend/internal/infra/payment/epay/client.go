package epay

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"net/url"
	"sort"
	"strings"

	domainbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/billing"
	paymentport "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/payment"
)

// Client 构建易支付 submit.php 签名跳转请求。
type Client struct{}

// New 创建易支付适配器。
func New() *Client {
	return &Client{}
}

// CreateCheckout 构建签名后的易支付 submit.php 页面跳转地址。
func (c *Client) CreateCheckout(_ context.Context, input paymentport.EPayCheckoutInput) (paymentport.CheckoutResult, error) {
	if c == nil {
		return paymentport.CheckoutResult{}, fmt.Errorf("epay checkout client is not configured")
	}
	endpoint, err := domainbilling.ResolveEPaySubmitURL(input.GatewayURL)
	if err != nil {
		return paymentport.CheckoutResult{}, err
	}
	if strings.TrimSpace(input.MerchantID) == "" || strings.TrimSpace(input.MerchantKey) == "" {
		return paymentport.CheckoutResult{}, fmt.Errorf("epay merchant credentials are not configured")
	}
	if strings.TrimSpace(input.OrderNo) == "" || input.PayAmountCents <= 0 {
		return paymentport.CheckoutResult{}, fmt.Errorf("epay checkout order is invalid")
	}
	if !strings.EqualFold(strings.TrimSpace(input.PayCurrency), "CNY") {
		return paymentport.CheckoutResult{}, fmt.Errorf("epay checkout currency must be CNY")
	}
	if !validPaymentType(input.PaymentType) {
		return paymentport.CheckoutResult{}, fmt.Errorf("epay payment type is not supported")
	}
	if !isHTTPURL(input.NotifyURL) || !isHTTPURL(input.ReturnURL) {
		return paymentport.CheckoutResult{}, fmt.Errorf("epay checkout callback url is invalid")
	}
	if strings.TrimSpace(input.ProductName) == "" {
		return paymentport.CheckoutResult{}, fmt.Errorf("epay checkout product name is required")
	}

	params := url.Values{}
	params.Set("pid", strings.TrimSpace(input.MerchantID))
	params.Set("type", strings.TrimSpace(input.PaymentType))
	params.Set("out_trade_no", strings.TrimSpace(input.OrderNo))
	params.Set("notify_url", strings.TrimSpace(input.NotifyURL))
	params.Set("return_url", strings.TrimSpace(input.ReturnURL))
	params.Set("name", strings.TrimSpace(input.ProductName))
	params.Set("money", domainbilling.FormatEPayAmount(input.PayAmountCents))
	params.Set("sign", signValues(params, input.MerchantKey))
	params.Set("sign_type", "MD5")

	parsed, err := url.Parse(endpoint)
	if err != nil {
		return paymentport.CheckoutResult{}, domainbilling.ErrEPayGatewayInvalid
	}
	parsed.RawQuery = params.Encode()
	return paymentport.CheckoutResult{URL: parsed.String()}, nil
}

// VerifySignature 使用易支付 submit.php 协议的 MD5 规则校验通知签名。
func (c *Client) VerifySignature(values url.Values, key string) bool {
	if c == nil || strings.TrimSpace(key) == "" {
		return false
	}
	provided := strings.ToLower(strings.TrimSpace(values.Get("sign")))
	if provided == "" {
		return false
	}
	return hmac.Equal([]byte(provided), []byte(signValues(values, key)))
}

func signValues(values url.Values, key string) string {
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
	buffer.WriteString(strings.TrimSpace(key))
	sum := md5.Sum(buffer.Bytes())
	return hex.EncodeToString(sum[:])
}

func validPaymentType(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 32 {
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

func isHTTPURL(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" || parsed.User != nil {
		return false
	}
	return strings.EqualFold(parsed.Scheme, "http") || strings.EqualFold(parsed.Scheme, "https")
}
