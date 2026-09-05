package settings

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/apperr"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
)

func TestSettingValidationErrorCarriesTypedContractAndDetails(t *testing.T) {
	err := newSettingValidationError(settingCodeSMTPInvalid, SettingValidationDetails{
		Field: "auth:smtp_port",
		Rule:  "integer_range",
		Param: "1,65535",
	})

	if !errors.Is(err, ErrInvalidSetting) {
		t.Fatal("validation error must match ErrInvalidSetting")
	}
	if apperr.Code(err) != settingCodeSMTPInvalid {
		t.Fatalf("apperr.Code() = %q", apperr.Code(err))
	}
	if err.Error() == "" || err.Error() == "invalid setting: auth:smtp_port must be between 1 and 65535" {
		t.Fatalf("validation error must use the structured internal form, got %q", err.Error())
	}
	if got := err.Details(); got.Field != "auth:smtp_port" || got.Rule != "integer_range" || got.Param != "1,65535" {
		t.Fatalf("unexpected details: %+v", got)
	}

	got := response.Describe(http.StatusBadRequest, err)
	if got.Code != settingCodeSMTPInvalid || got.Message != "invalid SMTP settings" {
		t.Fatalf("unexpected response description: %+v", got)
	}
}

func TestSettingValidationErrorDoesNotExposeSubmittedValue(t *testing.T) {
	err := settingValidationForKey("billing", "epay_gateway_url", settingRule("epay_url", ""))
	for _, value := range []string{"secret", "token", "https://"} {
		if strings.Contains(err.Error(), value) {
			t.Fatalf("validation error leaked submitted value: %q", err.Error())
		}
	}
	if details := err.Details(); details.Param != "" {
		t.Fatalf("unexpected sensitive detail: %+v", details)
	}
}

func TestErrInvalidSettingRemainsUntypedSentinel(t *testing.T) {
	if apperr.Code(ErrInvalidSetting) != "" {
		t.Fatal("ErrInvalidSetting must remain an untyped identity")
	}
}
