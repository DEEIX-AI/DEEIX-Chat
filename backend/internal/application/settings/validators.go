package settings

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	domainbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/billing"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/security"
)

// settingValidator 校验去除首尾空白后的配置值；key 是 namespace:key，用于错误文案。
type settingValidator func(value string, key string) error

func allOf(validators ...settingValidator) settingValidator {
	return func(value string, key string) error {
		for _, validate := range validators {
			if err := validate(value, key); err != nil {
				return err
			}
		}
		return nil
	}
}

func boolValue() settingValidator {
	return func(value string, key string) error {
		if _, err := strconv.ParseBool(value); err != nil {
			return settingRule("bool", "")
		}
		return nil
	}
}

func integerValue() settingValidator {
	return func(value string, key string) error {
		if _, err := strconv.Atoi(value); err != nil {
			return settingRule("integer", "")
		}
		return nil
	}
}

func intRange(min int, max int) settingValidator {
	return func(value string, key string) error {
		v, err := strconv.Atoi(value)
		if err != nil || v < min || v > max {
			return settingRule("integer_range", fmt.Sprintf("%d,%d", min, max))
		}
		return nil
	}
}

// optionalIntZeroOrRange 接受空值、0 或区间内的整数：空与 0 都表示关闭该限制。
func optionalIntZeroOrRange(min int, max int) settingValidator {
	return func(value string, key string) error {
		if value == "" {
			return nil
		}
		v, err := strconv.Atoi(value)
		if err != nil || v < 0 || (v > 0 && (v < min || v > max)) {
			return settingRule("optional_integer_range", fmt.Sprintf("%d,%d", min, max))
		}
		return nil
	}
}

func int64Min(min int64) settingValidator {
	return func(value string, key string) error {
		v, err := strconv.ParseInt(value, 10, 64)
		if err != nil || v < min {
			return settingRule("integer_min", strconv.FormatInt(min, 10))
		}
		return nil
	}
}

func optionalInt64Min(min int64) settingValidator {
	return func(value string, key string) error {
		if value == "" {
			return nil
		}
		v, err := strconv.ParseInt(value, 10, 64)
		if err != nil || v < min {
			return settingRule("optional_integer_min", strconv.FormatInt(min, 10))
		}
		return nil
	}
}

func floatRange(min float64, max float64) settingValidator {
	return func(value string, key string) error {
		v, err := strconv.ParseFloat(value, 64)
		if err != nil || v < min || v > max {
			return settingRule("float_range", fmt.Sprintf("%g,%g", min, max))
		}
		return nil
	}
}

func maxLength(max int) settingValidator {
	return func(value string, key string) error {
		if len([]rune(value)) > max {
			return settingRule("max_length", strconv.Itoa(max))
		}
		return nil
	}
}

func oneOf(allowed ...string) settingValidator {
	return func(value string, key string) error {
		for _, candidate := range allowed {
			if value == candidate {
				return nil
			}
		}
		return settingRule("enum", strings.Join(allowed, ","))
	}
}

// optionalHTTPURL 只要求 http(s) 前缀，用于允许指向内网自建服务的地址。
func optionalHTTPURL() settingValidator {
	return func(value string, key string) error {
		if value == "" {
			return nil
		}
		if !strings.HasPrefix(value, "http://") && !strings.HasPrefix(value, "https://") {
			return settingRule("http_url", "")
		}
		return nil
	}
}

// optionalTrustedHTTPURL 要求地址通过出站信任策略，用于会携带凭据访问的第三方端点。
func optionalTrustedHTTPURL() settingValidator {
	return func(value string, key string) error {
		if value == "" {
			return nil
		}
		if err := security.ValidateTrustedOutboundHTTPURL(value); err != nil {
			return settingRule("trusted_http_url", "")
		}
		return nil
	}
}

func validateLoginDefaultNextPath(value string, key string) error {
	if value == "" {
		return settingRule("required", "")
	}
	if !strings.HasPrefix(value, "/") || strings.HasPrefix(value, "//") {
		return settingRule("local_path", "")
	}
	return maxLength(120)(value, key)
}

func validateEmailDomainList(value string, key string) error {
	if len([]rune(value)) > 1024 {
		return settingRule("max_length", "1024")
	}
	for _, domain := range splitList(value) {
		domain = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(domain)), "@")
		if strings.Contains(domain, "@") || strings.Contains(domain, "://") || !strings.Contains(domain, ".") {
			return settingRule("domain", "")
		}
	}
	return nil
}

func splitList(value string) []string {
	return strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r' || r == '\t' || r == ' '
	})
}

func validateAllowedMIMETypes(value string, key string) error {
	if value == "" {
		return settingRule("required", "")
	}
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if !strings.Contains(part, "/") {
			return settingRule("mime", "")
		}
	}
	return nil
}

func validateMinerUFileTypes(value string, key string) error {
	allowed := map[string]struct{}{
		"pdf":          {},
		"word":         {},
		"presentation": {},
		"excel":        {},
	}
	for _, part := range strings.Split(value, ",") {
		item := strings.ToLower(strings.TrimSpace(part))
		if item == "" {
			continue
		}
		if _, ok := allowed[item]; !ok {
			return settingRule("file_type", "pdf,word,presentation,excel")
		}
	}
	return nil
}

func validatePaymentProviders(value string, key string) error {
	for _, provider := range normalizePaymentProvidersSetting(value) {
		switch provider {
		case "stripe", "epay":
		default:
			return settingRule("payment_provider", "stripe,epay")
		}
	}
	return nil
}

func normalizePaymentProvidersSetting(raw string) []string {
	parts := strings.Split(raw, ",")
	results := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		provider := strings.ToLower(strings.TrimSpace(part))
		if provider == "" || provider == "disabled" {
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

func validateEPayGatewayURL(value string, key string) error {
	if err := maxLength(512)(value, key); err != nil {
		return err
	}
	if value == "" {
		return nil
	}
	if _, err := domainbilling.ResolveEPaySubmitURL(value); err != nil {
		return settingRule("epay_url", "")
	}
	return nil
}

type epayTypeSetting struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

func validateEPayTypesJSON(value string, key string) error {
	if value == "" {
		return settingRule("required", "")
	}
	var items []epayTypeSetting
	if err := json.Unmarshal([]byte(value), &items); err != nil {
		return settingRule("json_array", "")
	}
	if len(items) == 0 || len(items) > 10 {
		return settingRule("payment_count", "1,10")
	}
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		name := strings.TrimSpace(item.Name)
		paymentType := strings.TrimSpace(item.Type)
		if name == "" || paymentType == "" {
			return settingRule("payment_fields", "name,type")
		}
		if len(name) > 64 || len(paymentType) > 32 {
			return settingRule("payment_item_length", "name:64,type:32")
		}
		if !validPaymentSettingToken(paymentType) {
			return settingRule("payment_type_chars", "")
		}
		if _, ok := seen[paymentType]; ok {
			return settingRule("payment_type_unique", "")
		}
		seen[paymentType] = struct{}{}
	}
	return nil
}

func validPaymentSettingToken(value string) bool {
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '_' || char == '-' {
			continue
		}
		return false
	}
	return true
}
