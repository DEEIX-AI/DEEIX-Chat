package billing

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	domainbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/billing"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
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

func TestSameOriginPublicURLReturnsTypedReturnURLErrors(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want error
	}{
		{name: "protocol relative", raw: "//evil.example.com/callback", want: errPaymentReturnURLCrossOrigin},
		{name: "foreign origin", raw: "https://evil.example.com/callback", want: errPaymentReturnURLCrossOrigin},
		{name: "not a url", raw: "callback", want: errPaymentReturnURLInvalid},
		{name: "unparsable relative path", raw: "/settings%zz", want: errPaymentReturnURLInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := sameOriginPublicURL("https://chat.example.com", tc.raw)
			if !errors.Is(err, tc.want) {
				t.Fatalf("sameOriginPublicURL(%q) error = %v, want %v", tc.raw, err, tc.want)
			}
		})
	}
	if got, err := sameOriginPublicURL("https://chat.example.com", "/settings?payment=success"); err != nil || got != "https://chat.example.com/settings?payment=success" {
		t.Fatalf("relative path = %q, %v", got, err)
	}
}

func TestRespondPaymentCheckoutErrorMapsReturnURLErrorsToBadRequest(t *testing.T) {
	handler := &Handler{logger: zap.NewNop()}
	cases := []struct {
		err  error
		code string
	}{
		{err: errPaymentReturnURLCrossOrigin, code: "payment.return_url_cross_origin"},
		{err: fmt.Errorf("%w: %w", errPaymentReturnURLInvalid, errors.New("invalid URL escape")), code: "payment.return_url_invalid"},
	}
	for _, tc := range cases {
		t.Run(tc.code, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			w := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(w)
			ctx.Request = httptest.NewRequest("POST", "/api/v1/billing/payments/checkout", nil)

			handler.respondPaymentCheckoutError(ctx, domainbilling.PaymentProviderStripe, "validate", tc.err)

			var payload response.Envelope
			if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if w.Code != http.StatusBadRequest || payload.ErrorCode != tc.code {
				t.Fatalf("status = %d, envelope = %#v; want 400 with code %q", w.Code, payload, tc.code)
			}
		})
	}
}
