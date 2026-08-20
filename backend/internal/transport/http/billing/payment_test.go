package billing

import (
	"errors"
	"net/http/httptest"
	"testing"

	domainbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/billing"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func TestPreparePaymentCheckoutRejectsUnsafeEPayEndpoint(t *testing.T) {
	handler := &Handler{
		cfg:    config.NewRuntime(config.Config{Env: "dev"}),
		logger: zap.NewNop(),
	}
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("POST", "https://chat.example.com/api/v1/billing/payments/checkout", nil)
	ctx.Request.Header.Set("Origin", "https://chat.example.com")

	_, err := handler.preparePaymentCheckout(ctx, domainbilling.PaymentProviderEPay, billingPaymentSettings{
		EPayGatewayURL: "https://pay.example.com/?token=secret",
		EPayPID:        "merchant-1",
		EPayKey:        "secret",
		EPayTypes:      defaultEPayTypes(),
	}, CreateCheckoutRequest{EPayType: "alipay"})
	if !errors.Is(err, domainbilling.ErrEPayGatewayInvalid) {
		t.Fatalf("preparePaymentCheckout() error = %v, want ErrEPayGatewayInvalid", err)
	}
}

func TestPreparePaymentCheckoutAcceptsClassicEPayEndpoint(t *testing.T) {
	handler := &Handler{
		cfg: config.NewRuntime(config.Config{
			Env:              "prod",
			PublicAPIBaseURL: "https://api.example.com",
			PublicWebBaseURL: "https://chat.example.com",
		}),
		logger: zap.NewNop(),
	}
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("POST", "https://api.example.com/api/v1/billing/payments/checkout", nil)

	preparation, err := handler.preparePaymentCheckout(ctx, domainbilling.PaymentProviderEPay, billingPaymentSettings{
		EPayGatewayURL: "https://pay.example.com/submit.php",
		EPayPID:        "merchant-1",
		EPayKey:        "secret",
		EPayTypes:      defaultEPayTypes(),
	}, CreateCheckoutRequest{
		EPayType:   "alipay",
		SuccessURL: "https://chat.example.com/setting/subscription?payment=success",
	})
	if err != nil {
		t.Fatalf("preparePaymentCheckout() error = %v", err)
	}
	if preparation.epayType != "alipay" || preparation.notifyURL != "https://api.example.com/api/v1/billing/payments/epay/notify" {
		t.Fatalf("unexpected preparation: %#v", preparation)
	}
}
