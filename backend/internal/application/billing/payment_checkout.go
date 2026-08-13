package billing

import (
	"context"
	"fmt"

	domainbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/billing"
	stripeinfra "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/payment/stripe"
)

type stripeCheckoutProvider interface {
	CreateCheckoutSession(ctx context.Context, input stripeinfra.CheckoutInput) (stripeinfra.CheckoutResult, error)
}

// PaymentCheckoutService 编排支付收银台创建和第三方协议映射。
type PaymentCheckoutService struct {
	stripe stripeCheckoutProvider
}

// StripeCheckoutInput 定义创建 Stripe Checkout Session 的应用层入参。
type StripeCheckoutInput struct {
	SecretKey  string
	SuccessURL string
	CancelURL  string
	Order      *domainbilling.PaymentOrder
	Plan       *domainbilling.Plan
}

// PaymentCheckoutResult 表示第三方支付收银台标识与跳转地址。
type PaymentCheckoutResult struct {
	ID  string
	URL string
}

// PaymentProduct 表示支付渠道展示的商品名称和说明。
type PaymentProduct struct {
	Name        string
	Description string
}

// NewPaymentCheckoutService 创建依赖完整的支付收银台应用服务。
func NewPaymentCheckoutService(stripe stripeCheckoutProvider) *PaymentCheckoutService {
	return &PaymentCheckoutService{stripe: stripe}
}

// CreateStripeCheckoutSession 编排 Stripe 收银台创建，并将领域订单映射为第三方协议字段。
func (s *PaymentCheckoutService) CreateStripeCheckoutSession(ctx context.Context, input StripeCheckoutInput) (PaymentCheckoutResult, error) {
	if s == nil || s.stripe == nil {
		return PaymentCheckoutResult{}, ErrPaymentProviderUnavailable
	}
	if input.Order == nil {
		return PaymentCheckoutResult{}, fmt.Errorf("payment order is required")
	}
	product := DescribePaymentProduct(input.Order, input.Plan)
	result, err := s.stripe.CreateCheckoutSession(ctx, stripeinfra.CheckoutInput{
		SecretKey:          input.SecretKey,
		SuccessURL:         input.SuccessURL,
		CancelURL:          input.CancelURL,
		OrderNo:            input.Order.OrderNo,
		OrderType:          input.Order.OrderType,
		UserID:             input.Order.UserID,
		BaseCurrency:       input.Order.BaseCurrency,
		BaseAmountCents:    input.Order.BaseAmountCents,
		PayCurrency:        input.Order.PayCurrency,
		PayAmountCents:     input.Order.PayAmountCents,
		FXRate:             input.Order.FXRate,
		ProductName:        product.Name,
		ProductDescription: product.Description,
	})
	if err != nil {
		return PaymentCheckoutResult{}, err
	}
	return PaymentCheckoutResult{ID: result.ID, URL: result.URL}, nil
}

// DescribePaymentProduct 统一生成各支付渠道使用的商品展示信息。
func DescribePaymentProduct(order *domainbilling.PaymentOrder, plan *domainbilling.Plan) PaymentProduct {
	if order != nil && order.OrderType == domainbilling.PaymentOrderTypeTopUp {
		amountCents := order.PayAmountCents
		if amountCents <= 0 {
			amountCents = order.BaseAmountCents
		}
		return PaymentProduct{
			Name:        "按量余额充值",
			Description: fmt.Sprintf("充值 %s %.2f 至按量余额", firstNonEmpty(order.PayCurrency, order.BaseCurrency, "USD"), float64(amountCents)/100),
		}
	}
	if plan != nil {
		return PaymentProduct{
			Name:        firstNonEmpty(plan.Name, plan.Code),
			Description: firstNonEmpty(plan.Description, plan.Code),
		}
	}
	return PaymentProduct{Name: "订阅方案", Description: "订阅方案支付"}
}
