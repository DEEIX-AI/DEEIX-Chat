package settings

import (
	"errors"
	"fmt"
	"strings"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/apperr"
)

var ErrInvalidSetting = errors.New("invalid setting")

const (
	settingCodeInvalidNamespace         = "settings.invalid_namespace"
	settingCodeInvalidKey               = "settings.invalid_key"
	settingCodeInvalidValue             = "settings.invalid_value"
	settingCodeSMTPInvalid              = "settings.smtp_invalid"
	settingCodeBillingPaymentInvalid    = "settings.billing_payment_invalid"
	settingCodeEmbeddingInvalid         = "settings.embedding_invalid"
	settingCodeExtractInvalid           = "settings.extract_invalid"
	settingCodeModelOptionPolicyInvalid = "settings.model_option_policy_invalid"
)

type SettingValidationDetails struct {
	Field  string
	Fields []string
	Rule   string
	Param  string
}

type SettingValidationError struct {
	contract *apperr.Error
	details  SettingValidationDetails
	internal string
}

func (e *SettingValidationError) Error() string {
	if e == nil {
		return ErrInvalidSetting.Error()
	}
	return e.internal
}

func (e *SettingValidationError) Unwrap() []error {
	if e == nil {
		return nil
	}
	return []error{ErrInvalidSetting, e.contract}
}

func (e *SettingValidationError) Code() string {
	if e == nil || e.contract == nil {
		return ""
	}
	return e.contract.Code()
}

func (e *SettingValidationError) Details() SettingValidationDetails {
	if e == nil {
		return SettingValidationDetails{}
	}
	details := e.details
	details.Fields = append([]string(nil), details.Fields...)
	return details
}

type settingRuleError struct {
	rule  string
	param string
}

func (e *settingRuleError) Error() string {
	if e == nil {
		return "setting validation failed"
	}
	if e.param == "" {
		return e.rule
	}
	return e.rule + ": " + e.param
}

func settingRule(rule string, param string) error {
	return &settingRuleError{rule: rule, param: param}
}

var settingContracts = map[string]*apperr.Error{
	settingCodeInvalidNamespace:         apperr.NewMasked(settingCodeInvalidNamespace, "invalid setting namespace", "invalid setting namespace"),
	settingCodeInvalidKey:               apperr.NewMasked(settingCodeInvalidKey, "invalid setting key", "invalid setting key"),
	settingCodeInvalidValue:             apperr.NewMasked(settingCodeInvalidValue, "invalid setting value", "invalid setting value"),
	settingCodeSMTPInvalid:              apperr.NewMasked(settingCodeSMTPInvalid, "invalid SMTP settings", "invalid SMTP settings"),
	settingCodeBillingPaymentInvalid:    apperr.NewMasked(settingCodeBillingPaymentInvalid, "invalid billing payment settings", "invalid billing payment settings"),
	settingCodeEmbeddingInvalid:         apperr.NewMasked(settingCodeEmbeddingInvalid, "invalid embedding settings", "invalid embedding settings"),
	settingCodeExtractInvalid:           apperr.NewMasked(settingCodeExtractInvalid, "invalid file extraction settings", "invalid file extraction settings"),
	settingCodeModelOptionPolicyInvalid: apperr.NewMasked(settingCodeModelOptionPolicyInvalid, "invalid model option policy settings", "invalid model option policy settings"),
}

func newSettingValidationError(code string, details SettingValidationDetails) *SettingValidationError {
	contract, ok := settingContracts[code]
	if !ok {
		code = settingCodeInvalidValue
		contract = settingContracts[code]
	}
	if strings.TrimSpace(details.Rule) == "" {
		details.Rule = "invalid"
	}
	details.Field = strings.TrimSpace(details.Field)
	details.Param = strings.TrimSpace(details.Param)
	fields := make([]string, 0, len(details.Fields))
	for _, field := range details.Fields {
		field = strings.TrimSpace(field)
		if field != "" {
			fields = append(fields, field)
		}
	}
	details.Fields = fields
	return &SettingValidationError{
		contract: contract,
		details:  details,
		internal: formatSettingValidationError(details),
	}
}

func formatSettingValidationError(details SettingValidationDetails) string {
	field := details.Field
	if field == "" && len(details.Fields) > 0 {
		field = strings.Join(details.Fields, ", ")
	}
	if field == "" {
		field = "setting"
	}
	if details.Param == "" {
		return fmt.Sprintf("%s validation failed (%s)", field, details.Rule)
	}
	return fmt.Sprintf("%s validation failed (%s: %s)", field, details.Rule, details.Param)
}

func settingValidationForRule(code string, field string, issue error) *SettingValidationError {
	details := SettingValidationDetails{Field: field, Rule: "invalid"}
	var ruleErr *settingRuleError
	if errors.As(issue, &ruleErr) {
		details.Rule = ruleErr.rule
		details.Param = ruleErr.param
	}
	return newSettingValidationError(code, details)
}

func settingValidationForFields(code string, fields []string, rule string, param string) *SettingValidationError {
	return newSettingValidationError(code, SettingValidationDetails{Fields: fields, Rule: rule, Param: param})
}

func settingValidationCode(namespace string, key string) string {
	switch namespace + ":" + key {
	case "auth:smtp_host", "auth:smtp_port", "auth:smtp_username", "auth:smtp_password", "auth:smtp_from":
		return settingCodeSMTPInvalid
	case "billing:payment_providers", "billing:epay_gateway_url", "billing:epay_types", "billing:epay_pid", "billing:epay_key", "billing:stripe_publishable_key", "billing:stripe_secret_key", "billing:stripe_webhook_secret":
		return settingCodeBillingPaymentInvalid
	case "chat:model_option_policy_mode", "chat:model_option_allowed_paths", "chat:model_option_denied_paths":
		return settingCodeModelOptionPolicyInvalid
	case "file:embedding_enabled", "file:embedding_host", "file:embedding_key", "file:embedding_timeout_seconds", "file:embedding_output_dimensions", "file:embedding_normalize", "file:embedding_model_signature", "file:embed_trigger_on_upload", "file:embed_chunk_size_tokens", "file:embed_chunk_overlap_tokens", "file:embed_batch_size":
		return settingCodeEmbeddingInvalid
	case "extract:engine", "extract:ocr_engine", "extract:image_ocr_enabled", "extract:pdf_ocr_fallback_enabled", "extract:tika_source", "extract:tika_base_url", "extract:tika_auth_token", "extract:tika_timeout_seconds", "extract:docling_base_url", "extract:docling_auth_token", "extract:docling_timeout_seconds", "extract:tesseract_ocr_base_url", "extract:tesseract_ocr_auth_token", "extract:tesseract_ocr_timeout_seconds", "extract:rapidocr_base_url", "extract:rapidocr_auth_token", "extract:rapidocr_source", "extract:rapidocr_timeout_seconds", "extract:paddle_ocr_base_url", "extract:paddle_ocr_auth_token", "extract:paddle_ocr_timeout_seconds", "extract:tencent_ocr_secret_id", "extract:tencent_ocr_secret_key", "extract:tencent_ocr_region", "extract:tencent_ocr_endpoint", "extract:tencent_ocr_timeout_seconds", "extract:aliyun_ocr_access_key_id", "extract:aliyun_ocr_access_key_secret", "extract:aliyun_ocr_region", "extract:aliyun_ocr_endpoint", "extract:aliyun_ocr_timeout_seconds", "extract:mineru_source", "extract:mineru_base_url", "extract:mineru_file_types", "extract:mineru_auth_token", "extract:mineru_timeout_seconds", "extract:mistral_ocr_base_url", "extract:mistral_ocr_auth_token", "extract:mistral_ocr_model", "extract:mistral_ocr_timeout_seconds", "extract:llm_ocr_base_url", "extract:llm_ocr_model", "extract:llm_ocr_auth_token", "extract:llm_ocr_timeout_seconds", "extract:llm_ocr_prompt":
		return settingCodeExtractInvalid
	default:
		return settingCodeInvalidValue
	}
}

func settingValidationForKey(namespace string, key string, issue error) *SettingValidationError {
	return settingValidationForRule(settingValidationCode(namespace, key), namespace+":"+key, issue)
}
