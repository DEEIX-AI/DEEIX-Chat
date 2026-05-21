package response

import (
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func TestInferErrorCode(t *testing.T) {
	cases := []struct {
		name   string
		status int
		msg    string
		want   string
	}{
		{name: "invalid id", status: http.StatusBadRequest, msg: "invalid conversation id", want: "conversation.invalid_id"},
		{name: "two factor", status: http.StatusUnauthorized, msg: "invalid two factor code", want: CodeAuthInvalidTwoFactorCode},
		{name: "pricing", status: http.StatusPaymentRequired, msg: "model pricing is required", want: CodeBillingPricingRequired},
		{name: "quota", status: http.StatusConflict, msg: "storage quota exceeded", want: CodeQuotaExceeded},
		{name: "upstream", status: http.StatusBadGateway, msg: "remote models unavailable", want: "llm.remote_models_unavailable"},
		{name: "generation canceled", status: http.StatusBadRequest, msg: "message generation canceled", want: "conversation_run.canceled"},
		{name: "internal", status: http.StatusInternalServerError, msg: "update settings failed: pq: bad column", want: CodeInternal},
		{name: "provider", status: http.StatusBadRequest, msg: "invalid oauth state", want: "auth.oauth_state_invalid"},
		{name: "verification", status: http.StatusBadRequest, msg: "verification code is invalid or expired", want: "auth.verification_code_invalid"},
		{name: "email registration", status: http.StatusBadRequest, msg: "email registration is disabled", want: "auth.email_registration_disabled"},
		{name: "provider email conflict", status: http.StatusBadRequest, msg: "email already exists; bind the provider before login", want: "auth.provider_email_conflict"},
		{name: "two factor already enabled", status: http.StatusBadRequest, msg: "two factor authentication is already enabled", want: "auth.two_factor_already_enabled"},
		{name: "provider redirect uri", status: http.StatusBadRequest, msg: "invalid redirect uri", want: "auth.invalid_redirect_uri"},
		{name: "user setting value", status: http.StatusBadRequest, msg: "invalid value for chat.file_mode: must be one of 'auto', 'rag'", want: "user_settings.invalid_value"},
		{name: "payment mismatch", status: http.StatusBadRequest, msg: "provider mismatch", want: "payment.notification_mismatch"},
		{name: "third party login disabled", status: http.StatusBadRequest, msg: "third-party login is disabled", want: "auth.provider_login_disabled"},
		{name: "admin user status", status: http.StatusBadRequest, msg: "invalid user status", want: "user.invalid_status"},
		{name: "user phone", status: http.StatusBadRequest, msg: "invalid user phone", want: "user.invalid_phone"},
		{name: "auth email", status: http.StatusBadRequest, msg: "invalid email", want: "auth.invalid_email"},
		{name: "superadmin delete protected", status: http.StatusConflict, msg: "superadmin delete not allowed", want: "user.superadmin_delete_protected"},
		{name: "billing pricing invalid", status: http.StatusBadRequest, msg: "invalid model pricing", want: "billing.invalid_model_pricing"},
		{name: "settings nested namespace", status: http.StatusBadRequest, msg: "invalid setting: invalid namespace: foo", want: "settings.invalid_namespace"},
		{name: "settings nested smtp", status: http.StatusBadRequest, msg: "invalid setting: auth:smtp_port must be an integer between 1 and 65535", want: "settings.smtp_invalid"},
		{name: "mcp server id", status: http.StatusBadRequest, msg: "invalid mcp server id", want: "mcp.server.invalid_id"},
		{name: "mcp tool id", status: http.StatusBadRequest, msg: "invalid mcp tool id", want: "mcp.tool.invalid_id"},
		{name: "bad gateway detail", status: http.StatusBadGateway, msg: "provider said secret", want: CodeUpstreamUnavailable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := InferErrorCode(tc.status, tc.msg); got != tc.want {
				t.Fatalf("InferErrorCode() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestTransportLiteralClientErrorsHaveStableCodes(t *testing.T) {
	statusCodes := map[string]int{
		"BadRequest":            http.StatusBadRequest,
		"Unauthorized":          http.StatusUnauthorized,
		"Forbidden":             http.StatusForbidden,
		"NotFound":              http.StatusNotFound,
		"Conflict":              http.StatusConflict,
		"PaymentRequired":       http.StatusPaymentRequired,
		"RequestEntityTooLarge": http.StatusRequestEntityTooLarge,
		"TooManyRequests":       http.StatusTooManyRequests,
		"BadGateway":            http.StatusBadGateway,
		"ServiceUnavailable":    http.StatusServiceUnavailable,
	}
	callPattern := regexp.MustCompile(`response\.Error\(c,\s*http\.Status([A-Za-z]+),\s*"([^"]+)"\)`)
	root := filepath.Clean("../../transport/http")
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() || filepath.Ext(path) != ".go" {
			return walkErr
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		for _, match := range callPattern.FindAllStringSubmatch(string(content), -1) {
			status, ok := statusCodes[match[1]]
			if !ok || status >= http.StatusInternalServerError {
				continue
			}
			code := InferErrorCode(status, match[2])
			if code == "" || code == CodeRequestInvalid || code == CodeResourceConflict {
				t.Fatalf("%s uses generic error code %q for %q", path, code, match[2])
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestPublicErrorMessage(t *testing.T) {
	cases := []struct {
		name   string
		status int
		code   string
		msg    string
		want   string
	}{
		{name: "preserves bad request context", status: http.StatusBadRequest, code: CodeRequestInvalid, msg: "invalid user id", want: "invalid user id"},
		{name: "hides internal details", status: http.StatusInternalServerError, code: CodeInternal, msg: "update settings failed: pq: bad column", want: "internal server error"},
		{name: "hides upstream details", status: http.StatusBadGateway, code: CodeUpstreamUnavailable, msg: "provider said secret", want: "upstream service unavailable"},
		{name: "normalizes service unavailable", status: http.StatusServiceUnavailable, code: CodeServiceUnavailable, msg: "embedding host down", want: "service unavailable"},
		{name: "normalizes billing", status: http.StatusPaymentRequired, code: CodeBillingInsufficientFunds, msg: "usage balance is insufficient", want: "insufficient balance"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := PublicErrorMessage(tc.status, tc.code, tc.msg); got != tc.want {
				t.Fatalf("PublicErrorMessage() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestTransportDoesNotBypassErrorEnvelope(t *testing.T) {
	root := filepath.Clean("../../transport/http")
	abortPattern := regexp.MustCompile(`AbortWithStatus\(\s*http\.Status([A-Za-z]+)\s*\)`)
	rawErrorPattern := regexp.MustCompile(`gin\.H\s*\{\s*"errorMsg"\s*:`)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() || filepath.Ext(path) != ".go" {
			return walkErr
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		text := string(content)
		for _, match := range abortPattern.FindAllStringSubmatch(text, -1) {
			if match[1] != "NoContent" {
				t.Fatalf("%s uses AbortWithStatus(%s); errors should use response.Error", path, match[1])
			}
		}
		if rawErrorPattern.MatchString(text) {
			t.Fatalf("%s writes raw errorMsg JSON; use response.Error instead", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestServiceErrorsAreEnglishForI18nFallbacks(t *testing.T) {
	pattern := regexp.MustCompile(`(fmt\.Errorf|errors\.New)\("([^"]*[\p{Han}][^"]*)"`)
	for _, root := range []string{
		filepath.Clean("../../application"),
		filepath.Clean("../../transport/http"),
		filepath.Clean("../../shared/response"),
	} {
		err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil || entry.IsDir() || filepath.Ext(path) != ".go" {
				return walkErr
			}
			content, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			if match := pattern.FindStringSubmatch(string(content)); len(match) > 0 {
				t.Fatalf("%s has non-English API error constructor: %q", path, match[2])
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
