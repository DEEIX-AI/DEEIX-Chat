// Package payment 定义支付网关收银台端口的数据契约。
package payment

// StripeCheckoutInput 定义 Stripe Checkout Session 所需的协议字段。
type StripeCheckoutInput struct {
	SecretKey          string
	SuccessURL         string
	CancelURL          string
	OrderNo            string
	OrderType          string
	UserID             uint
	BaseCurrency       string
	BaseAmountCents    int64
	PayCurrency        string
	PayAmountCents     int64
	FXRate             string
	ProductName        string
	ProductDescription string
}

// EPayCheckoutInput 定义易支付 submit.php 页面跳转所需的协议字段。
type EPayCheckoutInput struct {
	GatewayURL     string
	MerchantID     string
	MerchantKey    string
	PaymentType    string
	OrderNo        string
	NotifyURL      string
	ReturnURL      string
	PayCurrency    string
	PayAmountCents int64
	ProductName    string
}

// CheckoutResult 表示支付网关返回的收银台标识与跳转地址。
type CheckoutResult struct {
	ID  string
	URL string
}
