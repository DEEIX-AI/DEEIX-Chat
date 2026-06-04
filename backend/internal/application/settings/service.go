package settings

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/extraction"
	domainsettings "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/settings"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	mineruextract "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/extract/mineru"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

// Service 封装 settings 业务逻辑。
type Service struct {
	repo              repository.SettingsRepository
	dataEncryptionKey string
	authSafety        authSafetyService
	auditWriter       auditWriter
}

type authSafetyService interface {
	HasActiveSuperAdminIdentity(ctx context.Context) (bool, error)
}

type auditWriter interface {
	Write(ctx context.Context, requestID string, actorUserID uint, action string, resource string, resourceID string, ip string, userAgent string, detail interface{})
}

// NewService 创建服务。
func NewService(repo repository.SettingsRepository, dataEncryptionKey string) *Service {
	return &Service{repo: repo, dataEncryptionKey: strings.TrimSpace(dataEncryptionKey)}
}

func (s *Service) SetAuthSafetyService(service authSafetyService) {
	s.authSafety = service
}

// SetAuditWriter 注入系统设置审计写入器。
func (s *Service) SetAuditWriter(writer auditWriter) {
	s.auditWriter = writer
}

// AuditInput 描述系统设置审计写入。
type AuditInput struct {
	UserID    uint
	RequestID string
	Action    string
	ClientIP  string
	UserAgent string
	Detail    interface{}
}

// RecordAudit 记录系统设置审计日志。
func (s *Service) RecordAudit(ctx context.Context, input AuditInput) {
	if s.auditWriter == nil {
		return
	}
	s.auditWriter.Write(
		ctx,
		strings.TrimSpace(input.RequestID),
		input.UserID,
		strings.TrimSpace(input.Action),
		"system_settings",
		"",
		strings.TrimSpace(input.ClientIP),
		strings.TrimSpace(input.UserAgent),
		input.Detail,
	)
}

// Seed 将默认配置写入数据库（仅插入不存在的 key）。
func (s *Service) Seed(ctx context.Context, cfg config.Config) error {
	for _, item := range obsoleteSettings() {
		if err := s.repo.Delete(ctx, item.Namespace, item.Key); err != nil {
			return err
		}
	}
	items, err := s.encryptSettingsForStorage(defaultSettingsWithConfig(cfg))
	if err != nil {
		return err
	}
	return s.repo.UpsertWithDescription(ctx, items)
}

// ListAll 查询全部配置，按 namespace 分组。
func (s *Service) ListAll(ctx context.Context) (map[string][]SettingItem, error) {
	items, err := s.repo.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	return s.groupByNamespace(items), nil
}

// ListByNamespace 查询指定 namespace 的配置。
func (s *Service) ListByNamespace(ctx context.Context, namespace string) ([]SettingItem, error) {
	items, err := s.repo.ListByNamespace(ctx, namespace)
	if err != nil {
		return nil, err
	}
	result := make([]SettingItem, 0, len(items))
	for _, item := range items {
		result = append(result, s.settingResponse(item))
	}
	return result, nil
}

// RuntimeValuesByNamespace 返回服务端运行时使用的配置值，敏感项会被解密。
func (s *Service) RuntimeValuesByNamespace(ctx context.Context, namespace string) (map[string]string, error) {
	items, err := s.repo.ListByNamespace(ctx, namespace)
	if err != nil {
		return nil, err
	}
	result := make(map[string]string, len(items))
	for _, item := range items {
		value, err := s.decryptSettingValue(item)
		if err != nil {
			return nil, err
		}
		result[item.Key] = strings.TrimSpace(value)
	}
	return result, nil
}

// validNamespaces 合法的 namespace 集合。
var validNamespaces = map[string]bool{
	"auth":    true,
	"billing": true,
	"chat":    true,
	"storage": true,
	"file":    true,
	"extract": true,
	"mcp":     true,
	"circuit": true,
}

// IsValidNamespace 判断 namespace 是否允许被动态配置。
func IsValidNamespace(namespace string) bool {
	return validNamespaces[namespace]
}

var validSettingKeys = buildValidSettingKeys()

// BatchUpdate 批量更新配置项。
func (s *Service) BatchUpdate(ctx context.Context, patches []PatchItem) (map[string][]SettingItem, error) {
	// 校验 namespace
	for _, p := range patches {
		if !validNamespaces[p.Namespace] {
			return nil, fmt.Errorf("%w: invalid namespace: %s", ErrInvalidSetting, p.Namespace)
		}
		if err := validatePatchItem(p); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidSetting, err)
		}
	}

	patches, err := s.applyAuthSettingDependencies(ctx, patches)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidSetting, err)
	}
	patches, err = s.applyEmbeddingDependentCascades(ctx, patches)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidSetting, err)
	}

	if err := s.validateFileProcessingSettings(ctx, patches); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidSetting, err)
	}
	if err := s.validateEmbeddingDependentSettings(ctx, patches); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidSetting, err)
	}
	if err := s.validateBillingPaymentSettings(ctx, patches); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidSetting, err)
	}
	items, err := s.preparePatchItemsForStorage(patches)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidSetting, err)
	}
	if err := s.repo.Upsert(ctx, items); err != nil {
		return nil, err
	}

	return s.ListAll(ctx)
}

func (s *Service) groupByNamespace(items []domainsettings.SystemSetting) map[string][]SettingItem {
	result := make(map[string][]SettingItem)
	for _, item := range items {
		if _, ok := validSettingKeys[item.Namespace+":"+item.Key]; !ok {
			continue
		}
		result[item.Namespace] = append(result[item.Namespace], s.settingResponse(item))
	}
	return result
}

func validatePatchItem(item PatchItem) error {
	key := item.Namespace + ":" + item.Key
	if _, ok := validSettingKeys[key]; !ok {
		return fmt.Errorf("invalid setting key: %s", key)
	}
	if item.Clear && !isSensitiveSetting(item.Namespace, item.Key) {
		return fmt.Errorf("clear is only supported for sensitive setting: %s", key)
	}
	if item.Clear {
		return nil
	}
	value := strings.TrimSpace(item.Value)
	switch key {
	case "billing:mode":
		switch value {
		case "self", "period", "usage":
			return nil
		default:
			return fmt.Errorf("%s must be one of: self, period, usage", key)
		}
	case "billing:payment_providers":
		for _, provider := range normalizePaymentProvidersSetting(value) {
			switch provider {
			case "stripe", "epay":
			default:
				return fmt.Errorf("%s must contain only: stripe, epay", key)
			}
		}
		return nil
	case "billing:usd_to_cny_rate":
		return validateFloatMinMax(value, 0.000001, 1000, key)
	case "billing:prepaid_amount_usd":
		return validateFloatMinMax(value, 0, 1000000, key)
	case "billing:stripe_publishable_key", "billing:stripe_secret_key", "billing:stripe_webhook_secret", "billing:epay_pid", "billing:epay_key":
		return validateStringMax(value, 512, key)
	case "billing:epay_types":
		if err := validateStringMax(value, 4000, key); err != nil {
			return err
		}
		return validateEPayTypesJSON(value, key)
	case "billing:native_tool_pricing_json":
		return validateNativeToolPricingJSON(value, key)
	case "billing:epay_gateway_url":
		if err := validateStringMax(value, 512, key); err != nil {
			return err
		}
		return validateOptionalHTTPURL(value, key)
	case "auth:login_page_title":
		return validateStringMax(value, 80, key)
	case "chat:model_option_policy_mode":
		switch value {
		case "allowlist", "denylist", "disabled":
			return nil
		default:
			return fmt.Errorf("%s must be one of: allowlist, denylist, disabled", key)
		}
	case "chat:model_option_allowed_paths", "chat:model_option_denied_paths":
		return validateModelOptionPathsJSON(value, key)
	case "auth:login_default_next_path":
		if value == "" {
			return fmt.Errorf("%s cannot be empty", key)
		}
		if !strings.HasPrefix(value, "/") || strings.HasPrefix(value, "//") {
			return fmt.Errorf("%s must be a local path", key)
		}
		return validateStringMax(value, 120, key)
	case "auth:token_ttl_hours":
		return validateIntMinMax(value, 1, 168, key)
	case "auth:refresh_token_ttl_hours":
		return validateIntMinMax(value, 1, 8760, key)
	case "auth:rate_limit_rpm", "auth:public_auth_rate_limit_rpm":
		return validateIntMinMax(value, 1, 100000, key)
	case "auth:smtp_host":
		return validateStringMax(value, 255, key)
	case "auth:smtp_username", "auth:smtp_password", "auth:smtp_from":
		return validateStringMax(value, 255, key)
	case "auth:turnstile_site_key", "auth:turnstile_secret_key":
		return validateStringMax(value, 512, key)
	case "chat:default_system_prompt":
		return validateStringMax(value, 20000, key)
	case "auth:smtp_port":
		return validateIntMinMax(value, 1, 65535, key)
	case "auth:email_registration_allowed_domains":
		return validateEmailDomainList(value, key)
	case "storage:max_upload_file_bytes", "storage:user_storage_quota_bytes":
		return validateInt64Min(value, 1, key)
	case "file:file_full_context_max_bytes":
		return validateOptionalInt64Min(value, 0, key)
	case "file:image_max_bytes", "file:doc_max_bytes":
		if value == "" {
			return nil
		}
		return validateInt64Min(value, 1, key)
	case "storage:max_message_files":
		return validateIntMinMax(value, 1, 50, key)
	case "file:image_max_dimension":
		return validateIntMinMax(value, 0, 8192, key)
	case "file:full_context_max_tokens":
		return validateOptionalIntZeroOrMinMax(value, 128, 1000000, key)
	case "file:full_context_pdf_max_pages":
		return validateOptionalIntZeroOrMinMax(value, 1, 500, key)
	case "chat:rag_wait_ready_ms":
		return validateIntMinMax(value, 1000, 120000, key)
	case "chat:context_artifact_retention_days":
		return validateIntMinMax(value, 0, 3650, key)
	case "file:embedding_timeout_seconds":
		return validateIntMinMax(value, 1, 600, key)
	case "file:embedding_host":
		if value == "" {
			return nil
		}
		if !strings.HasPrefix(value, "http://") && !strings.HasPrefix(value, "https://") {
			return fmt.Errorf("%s must start with http:// or https://", key)
		}
		return nil
	case "file:embedding_output_dimensions":
		return validateIntMinMax(value, 64, 4096, key)
	case "file:embedding_model_dimensions":
		return validateIntMinMax(value, 64, 4096, key)
	case "file:allowed_mime_types":
		if len(value) == 0 {
			return fmt.Errorf("%s cannot be empty", key)
		}
		for _, part := range strings.Split(value, ",") {
			part = strings.TrimSpace(part)
			if !strings.Contains(part, "/") {
				return fmt.Errorf("%s contains invalid mime: %s", key, part)
			}
		}
	case "extract:engine":
		switch value {
		case extraction.EngineBuiltin, extraction.EngineTika, extraction.EngineDocling, extraction.EngineMinerU:
			return nil
		default:
			return fmt.Errorf("%s must be one of: %s, %s, %s, %s", key, extraction.EngineBuiltin, extraction.EngineTika, extraction.EngineDocling, extraction.EngineMinerU)
		}
	case "extract:ocr_engine":
		switch value {
		case extraction.OCREngineRapidOCR, extraction.OCREngineTesseract, extraction.OCREnginePaddle, extraction.OCREngineTencent, extraction.OCREngineAliyun, extraction.OCREngineLLM:
			return nil
		default:
			return fmt.Errorf("%s must be one of: %s, %s, %s, %s, %s, %s", key, extraction.OCREngineRapidOCR, extraction.OCREngineTesseract, extraction.OCREnginePaddle, extraction.OCREngineTencent, extraction.OCREngineAliyun, extraction.OCREngineLLM)
		}
	case "extract:tika_source":
		switch value {
		case extraction.TikaSourceExternal, extraction.TikaSourceManaged:
			return nil
		default:
			return fmt.Errorf("%s must be one of: %s, %s", key, extraction.TikaSourceExternal, extraction.TikaSourceManaged)
		}
	case "extract:mineru_source":
		switch value {
		case mineruextract.SourceCloud, mineruextract.SourceSelfHosted:
			return nil
		default:
			return fmt.Errorf("%s must be one of: %s, %s", key, mineruextract.SourceCloud, mineruextract.SourceSelfHosted)
		}
	case "extract:tika_base_url":
		if value == "" {
			return nil
		}
		if !strings.HasPrefix(value, "http://") && !strings.HasPrefix(value, "https://") {
			return fmt.Errorf("%s must start with http:// or https://", key)
		}
		return nil
	case "extract:docling_base_url", "extract:tesseract_ocr_base_url", "extract:rapidocr_base_url", "extract:paddle_ocr_base_url", "extract:mineru_base_url", "extract:llm_ocr_base_url":
		if value == "" {
			return nil
		}
		if !strings.HasPrefix(value, "http://") && !strings.HasPrefix(value, "https://") {
			return fmt.Errorf("%s must start with http:// or https://", key)
		}
		return nil
	case "extract:rapidocr_source":
		switch value {
		case extraction.TikaSourceExternal, extraction.TikaSourceManaged:
			return nil
		default:
			return fmt.Errorf("%s must be one of: %s, %s", key, extraction.TikaSourceExternal, extraction.TikaSourceManaged)
		}
	case "extract:tencent_ocr_endpoint", "extract:aliyun_ocr_endpoint", "extract:tencent_ocr_region", "extract:aliyun_ocr_region":
		return validateStringMax(value, 255, key)
	case "extract:tencent_ocr_secret_id", "extract:tencent_ocr_secret_key", "extract:aliyun_ocr_access_key_id", "extract:aliyun_ocr_access_key_secret":
		return validateStringMax(value, 512, key)
	case "auth:username_login_enabled", "auth:email_login_enabled", "auth:third_party_login_enabled", "auth:email_registration_enabled", "auth:email_verification_enabled", "auth:email_registration_block_plus_alias", "auth:auto_link_verified_email", "auth:turnstile_registration_enabled", "auth:rate_limit_enabled", "billing:native_tool_billing_enabled", "chat:rag_enabled", "chat:message_embedding_enabled", "chat:semantic_context_enabled", "file:full_context_limit_enabled", "file:embedding_enabled", "file:embed_trigger_on_upload", "file:embedding_normalize", "extract:image_ocr_enabled", "extract:pdf_ocr_fallback_enabled", "mcp:mcp_enable":
		if _, err := strconv.ParseBool(value); err != nil {
			return fmt.Errorf("%s must be bool", key)
		}
	case "extract:tika_timeout_seconds":
		return validateIntMinMax(value, 1, 120, key)
	case "extract:docling_timeout_seconds", "extract:tesseract_ocr_timeout_seconds", "extract:rapidocr_timeout_seconds", "extract:paddle_ocr_timeout_seconds", "extract:tencent_ocr_timeout_seconds", "extract:aliyun_ocr_timeout_seconds", "extract:mineru_timeout_seconds", "extract:llm_ocr_timeout_seconds":
		return validateIntMinMax(value, 1, 600, key)
	case "mcp:mcp_max_llm_calls_per_run":
		return validateIntMinMax(value, 2, 32, key)
	case "mcp:mcp_max_tool_calls_per_run":
		return validateIntMinMax(value, 1, 64, key)
	case "mcp:mcp_max_concurrent_calls":
		return validateIntMinMax(value, 1, 64, key)
	case "mcp:mcp_max_selected_tools_per_message":
		return validateIntMinMax(value, 1, config.MaxMCPSelectedToolsPerMessage, key)
	case "mcp:mcp_tool_timeout_seconds":
		return validateIntMinMax(value, 1, 120, key)
	case "mcp:mcp_tool_retry_count":
		return validateIntMinMax(value, 0, 5, key)
	}
	return nil
}

func (s *Service) validateFileProcessingSettings(ctx context.Context, patches []PatchItem) error {
	hasRelevantPatch := false
	for _, item := range patches {
		if item.Namespace == "extract" || item.Namespace == "file" {
			hasRelevantPatch = true
			break
		}
	}
	if !hasRelevantPatch {
		return nil
	}

	next, err := s.loadEffectiveSettings(ctx, "extract", "file")
	if err != nil {
		return err
	}
	applyPatchesToEffectiveSettings(next, patches, "extract", "file")

	if strings.TrimSpace(next["extract:engine"]) == extraction.EngineTika {
		if strings.TrimSpace(next["extract:tika_base_url"]) == "" {
			return fmt.Errorf("extract:tika_base_url is required when extract:engine is tika")
		}
	}
	if strings.TrimSpace(next["extract:engine"]) == extraction.EngineDocling && strings.TrimSpace(next["extract:docling_base_url"]) == "" {
		return fmt.Errorf("extract:docling_base_url is required when extract:engine is docling")
	}
	if strings.TrimSpace(next["extract:engine"]) == extraction.EngineMinerU && strings.TrimSpace(next["extract:mineru_base_url"]) == "" {
		return fmt.Errorf("extract:mineru_base_url is required when extract:engine is mineru")
	}

	imageOCREnabled, _ := strconv.ParseBool(strings.TrimSpace(next["extract:image_ocr_enabled"]))
	pdfOCRFallbackEnabled, _ := strconv.ParseBool(strings.TrimSpace(next["extract:pdf_ocr_fallback_enabled"]))
	if !imageOCREnabled && !pdfOCRFallbackEnabled {
		return nil
	}

	switch strings.TrimSpace(next["extract:ocr_engine"]) {
	case extraction.OCREngineTesseract:
		if strings.TrimSpace(next["extract:tesseract_ocr_base_url"]) == "" {
			return fmt.Errorf("extract:tesseract_ocr_base_url is required when OCR engine is tesseract")
		}
	case extraction.OCREngineRapidOCR:
		if strings.TrimSpace(next["extract:rapidocr_base_url"]) == "" {
			return fmt.Errorf("extract:rapidocr_base_url is required when OCR engine is rapidocr")
		}
	case extraction.OCREnginePaddle:
		if strings.TrimSpace(next["extract:paddle_ocr_base_url"]) == "" {
			return fmt.Errorf("extract:paddle_ocr_base_url is required when OCR engine is paddle")
		}
	case extraction.OCREngineTencent:
		if strings.TrimSpace(next["extract:tencent_ocr_secret_id"]) == "" {
			return fmt.Errorf("extract:tencent_ocr_secret_id is required when OCR engine is tencent")
		}
		if strings.TrimSpace(next["extract:tencent_ocr_secret_key"]) == "" {
			return fmt.Errorf("extract:tencent_ocr_secret_key is required when OCR engine is tencent")
		}
		if strings.TrimSpace(next["extract:tencent_ocr_region"]) == "" {
			return fmt.Errorf("extract:tencent_ocr_region is required when OCR engine is tencent")
		}
	case extraction.OCREngineAliyun:
		if strings.TrimSpace(next["extract:aliyun_ocr_access_key_id"]) == "" {
			return fmt.Errorf("extract:aliyun_ocr_access_key_id is required when OCR engine is aliyun")
		}
		if strings.TrimSpace(next["extract:aliyun_ocr_access_key_secret"]) == "" {
			return fmt.Errorf("extract:aliyun_ocr_access_key_secret is required when OCR engine is aliyun")
		}
		if strings.TrimSpace(next["extract:aliyun_ocr_region"]) == "" {
			return fmt.Errorf("extract:aliyun_ocr_region is required when OCR engine is aliyun")
		}
	case extraction.OCREngineLLM:
		if strings.TrimSpace(next["extract:llm_ocr_base_url"]) == "" {
			return fmt.Errorf("extract:llm_ocr_base_url is required when OCR engine is llm")
		}
		if strings.TrimSpace(next["extract:llm_ocr_model"]) == "" {
			return fmt.Errorf("extract:llm_ocr_model is required when OCR engine is llm")
		}
	}

	return nil
}

func (s *Service) applyAuthSettingDependencies(ctx context.Context, patches []PatchItem) ([]PatchItem, error) {
	hasAuthPatch := false
	for _, item := range patches {
		if item.Namespace == "auth" {
			hasAuthPatch = true
			break
		}
	}
	if !hasAuthPatch {
		return patches, nil
	}

	next, err := s.loadEffectiveSettings(ctx, "auth")
	if err != nil {
		return nil, err
	}
	applyPatchesToEffectiveSettings(next, patches, "auth")

	emailLoginEnabled, _ := strconv.ParseBool(next["auth:email_login_enabled"])
	usernameLoginEnabled, _ := strconv.ParseBool(next["auth:username_login_enabled"])
	thirdPartyLoginEnabled, _ := strconv.ParseBool(next["auth:third_party_login_enabled"])
	if !emailLoginEnabled && !usernameLoginEnabled {
		if !thirdPartyLoginEnabled {
			return nil, fmt.Errorf("auth:third_party_login_enabled must be enabled before disabling username and email login")
		}
		if s.authSafety == nil {
			return nil, fmt.Errorf("at least one superadmin must bind a third-party identity before disabling username and email login")
		}
		hasBoundAdmin, err := s.authSafety.HasActiveSuperAdminIdentity(ctx)
		if err != nil {
			return nil, err
		}
		if !hasBoundAdmin {
			return nil, fmt.Errorf("at least one superadmin must bind a third-party identity before disabling username and email login")
		}
	}
	if !emailLoginEnabled {
		if patchValueIsTrue(patches, "auth", "email_registration_enabled") {
			return nil, fmt.Errorf("auth:email_registration_enabled requires auth:email_login_enabled")
		}
		patches = upsertPatch(patches, PatchItem{Namespace: "auth", Key: "email_registration_enabled", Value: "false"})
		next["auth:email_registration_enabled"] = "false"
	}
	emailRegistrationEnabled, _ := strconv.ParseBool(next["auth:email_registration_enabled"])
	turnstileRegistrationEnabled, _ := strconv.ParseBool(next["auth:turnstile_registration_enabled"])
	if !emailRegistrationEnabled {
		if patchValueIsTrue(patches, "auth", "turnstile_registration_enabled") {
			return nil, fmt.Errorf("auth:turnstile_registration_enabled requires auth:email_registration_enabled")
		}
		patches = upsertPatch(patches, PatchItem{Namespace: "auth", Key: "turnstile_registration_enabled", Value: "false"})
		next["auth:turnstile_registration_enabled"] = "false"
		turnstileRegistrationEnabled = false
	}
	if turnstileRegistrationEnabled {
		if strings.TrimSpace(next["auth:turnstile_site_key"]) == "" {
			return nil, fmt.Errorf("auth:turnstile_site_key is required when auth:turnstile_registration_enabled is true")
		}
		if strings.TrimSpace(next["auth:turnstile_secret_key"]) == "" {
			return nil, fmt.Errorf("auth:turnstile_secret_key is required when auth:turnstile_registration_enabled is true")
		}
	}
	emailVerificationEnabled, _ := strconv.ParseBool(next["auth:email_verification_enabled"])
	if emailVerificationEnabled {
		if err := validateEmailVerificationSMTPSettings(next); err != nil {
			return nil, err
		}
	}

	return patches, nil
}

func validateEmailVerificationSMTPSettings(next map[string]string) error {
	required := []struct {
		key   string
		label string
	}{
		{key: "auth:smtp_host", label: "auth:smtp_host"},
		{key: "auth:smtp_port", label: "auth:smtp_port"},
		{key: "auth:smtp_username", label: "auth:smtp_username"},
		{key: "auth:smtp_password", label: "auth:smtp_password"},
	}
	missing := make([]string, 0, len(required))
	for _, item := range required {
		if strings.TrimSpace(next[item.key]) == "" {
			missing = append(missing, item.label)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("email verification requires settings: %s", strings.Join(missing, ", "))
	}
	port, err := strconv.Atoi(strings.TrimSpace(next["auth:smtp_port"]))
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("auth:smtp_port must be an integer between 1 and 65535")
	}
	return nil
}

func (s *Service) applyEmbeddingDependentCascades(ctx context.Context, patches []PatchItem) ([]PatchItem, error) {
	hasEmbeddingPatch := false
	for _, item := range patches {
		if item.Namespace == "file" && (item.Key == "embedding_enabled" || item.Key == "embedding_host" || item.Key == "rag_model") {
			hasEmbeddingPatch = true
			break
		}
	}
	if !hasEmbeddingPatch {
		return patches, nil
	}

	next, err := s.loadEffectiveSettings(ctx, "chat", "file")
	if err != nil {
		return nil, err
	}
	applyPatchesToEffectiveSettings(next, patches, "chat", "file")
	if embeddingServiceReady(next) {
		return patches, nil
	}

	hasRAGPatch := false
	hasMessagePatch := false
	hasSemanticPatch := false
	for _, item := range patches {
		if item.Namespace != "chat" {
			continue
		}
		switch item.Key {
		case "rag_enabled":
			hasRAGPatch = true
		case "message_embedding_enabled":
			hasMessagePatch = true
		case "semantic_context_enabled":
			hasSemanticPatch = true
		}
	}
	if !hasRAGPatch {
		patches = append(patches, PatchItem{Namespace: "chat", Key: "rag_enabled", Value: "false"})
	}
	if !hasMessagePatch {
		patches = append(patches, PatchItem{Namespace: "chat", Key: "message_embedding_enabled", Value: "false"})
	}
	if !hasSemanticPatch {
		patches = append(patches, PatchItem{Namespace: "chat", Key: "semantic_context_enabled", Value: "false"})
	}
	return patches, nil
}

func patchValueIsTrue(patches []PatchItem, namespace string, key string) bool {
	for _, item := range patches {
		if item.Namespace == namespace && item.Key == key {
			value, _ := strconv.ParseBool(strings.TrimSpace(item.Value))
			return value
		}
	}
	return false
}

func upsertPatch(patches []PatchItem, next PatchItem) []PatchItem {
	for index, item := range patches {
		if item.Namespace == next.Namespace && item.Key == next.Key {
			patches[index] = next
			return patches
		}
	}
	return append(patches, next)
}

func (s *Service) validateEmbeddingDependentSettings(ctx context.Context, patches []PatchItem) error {
	requiresValidation := false
	for _, item := range patches {
		if item.Namespace != "chat" {
			continue
		}
		if (item.Key == "rag_enabled" || item.Key == "message_embedding_enabled" || item.Key == "semantic_context_enabled") && strings.EqualFold(strings.TrimSpace(item.Value), "true") {
			requiresValidation = true
			break
		}
	}
	if !requiresValidation {
		return nil
	}

	next, err := s.loadEffectiveSettings(ctx, "chat", "file")
	if err != nil {
		return err
	}
	applyPatchesToEffectiveSettings(next, patches, "chat", "file")

	ragEnabled, _ := strconv.ParseBool(next["chat:rag_enabled"])
	messageEmbeddingEnabled, _ := strconv.ParseBool(next["chat:message_embedding_enabled"])
	semanticContextEnabled, _ := strconv.ParseBool(next["chat:semantic_context_enabled"])
	if semanticContextEnabled && !messageEmbeddingEnabled {
		return fmt.Errorf("chat:message_embedding_enabled is required when chat:semantic_context_enabled is true")
	}
	if ragEnabled || messageEmbeddingEnabled || semanticContextEnabled {
		if !embeddingServiceReady(next) {
			return fmt.Errorf("embedding service must be enabled and configured before enabling RAG or semantic enhancement")
		}
	}
	return nil
}

func (s *Service) validateBillingPaymentSettings(ctx context.Context, patches []PatchItem) error {
	hasBillingPatch := false
	for _, item := range patches {
		if item.Namespace == "billing" {
			hasBillingPatch = true
			break
		}
	}
	if !hasBillingPatch {
		return nil
	}

	next, err := s.loadEffectiveSettings(ctx, "billing")
	if err != nil {
		return err
	}
	applyPatchesToEffectiveSettings(next, patches, "billing")

	providers := normalizePaymentProvidersSetting(next["billing:payment_providers"])
	if len(providers) == 0 {
		return nil
	}
	if err := validateFloatMinMax(next["billing:usd_to_cny_rate"], 0.000001, 1000, "billing:usd_to_cny_rate"); err != nil {
		return err
	}
	for _, provider := range providers {
		switch provider {
		case "stripe":
			if err := requireSettingFields(next, []requiredSettingField{
				{key: "billing:stripe_secret_key", label: "Stripe Secret Key"},
				{key: "billing:stripe_webhook_secret", label: "Stripe Webhook Secret"},
			}); err != nil {
				return err
			}
		case "epay":
			if err := requireSettingFields(next, []requiredSettingField{
				{key: "billing:epay_gateway_url", label: "billing:epay_gateway_url"},
				{key: "billing:epay_types", label: "billing:epay_types"},
				{key: "billing:epay_pid", label: "billing:epay_pid"},
				{key: "billing:epay_key", label: "billing:epay_key"},
			}); err != nil {
				return err
			}
		default:
			return fmt.Errorf("billing:payment_providers must contain only: stripe, epay")
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

type epayTypeSetting struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

func validateEPayTypesJSON(value string, key string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("%s is required", key)
	}
	var items []epayTypeSetting
	if err := json.Unmarshal([]byte(value), &items); err != nil {
		return fmt.Errorf("%s must be a JSON array", key)
	}
	if len(items) == 0 || len(items) > 10 {
		return fmt.Errorf("%s must contain 1-10 payment types", key)
	}
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		name := strings.TrimSpace(item.Name)
		paymentType := strings.TrimSpace(item.Type)
		if name == "" || paymentType == "" {
			return fmt.Errorf("%s items require name and type", key)
		}
		if len(name) > 64 || len(paymentType) > 32 {
			return fmt.Errorf("%s item is too long", key)
		}
		if !validPaymentSettingToken(paymentType) {
			return fmt.Errorf("%s type contains invalid characters", key)
		}
		if _, ok := seen[paymentType]; ok {
			return fmt.Errorf("%s type must be unique", key)
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

type requiredSettingField struct {
	key   string
	label string
}

func requireSettingFields(values map[string]string, fields []requiredSettingField) error {
	missing := make([]string, 0, len(fields))
	for _, item := range fields {
		if strings.TrimSpace(values[item.key]) == "" {
			missing = append(missing, item.label)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required settings: %s", strings.Join(missing, ", "))
	}
	return nil
}

func embeddingServiceReady(settings map[string]string) bool {
	embeddingEnabled, _ := strconv.ParseBool(settings["file:embedding_enabled"])
	return embeddingEnabled &&
		strings.TrimSpace(settings["file:rag_model"]) != "" &&
		strings.TrimSpace(settings["file:embedding_host"]) != ""
}

func validateIntMinMax(value string, min int, max int, key string) error {
	v, err := strconv.Atoi(value)
	if err != nil || v < min || v > max {
		return fmt.Errorf("%s must be between %d and %d", key, min, max)
	}
	return nil
}

func validateInt64Min(value string, min int64, key string) error {
	v, err := strconv.ParseInt(value, 10, 64)
	if err != nil || v < min {
		return fmt.Errorf("%s must be >= %d", key, min)
	}
	return nil
}

func validateOptionalInt64Min(value string, min int64, key string) error {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	v, err := strconv.ParseInt(value, 10, 64)
	if err != nil || v < min {
		return fmt.Errorf("%s must be empty or >= %d", key, min)
	}
	return nil
}

func validateOptionalIntZeroOrMinMax(value string, min int, max int, key string) error {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	v, err := strconv.Atoi(value)
	if err != nil || v < 0 || (v > 0 && (v < min || v > max)) {
		return fmt.Errorf("%s must be empty, 0, or between %d and %d", key, min, max)
	}
	return nil
}

func validateFloatMinMax(value string, min float64, max float64, key string) error {
	v, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || v < min || v > max {
		return fmt.Errorf("%s must be between %g and %g", key, min, max)
	}
	return nil
}

func validateStringMax(value string, max int, key string) error {
	if len([]rune(value)) > max {
		return fmt.Errorf("%s length must be <= %d", key, max)
	}
	return nil
}

func validateOptionalHTTPURL(value string, key string) error {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	if !strings.HasPrefix(value, "http://") && !strings.HasPrefix(value, "https://") {
		return fmt.Errorf("%s must start with http:// or https://", key)
	}
	return nil
}

func validateEmailDomainList(value string, key string) error {
	if len([]rune(value)) > 1024 {
		return fmt.Errorf("%s length must be <= 1024", key)
	}
	for _, domain := range splitList(value) {
		domain = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(domain)), "@")
		if strings.Contains(domain, "@") || strings.Contains(domain, "://") || !strings.Contains(domain, ".") {
			return fmt.Errorf("%s contains invalid domain: %s", key, domain)
		}
	}
	return nil
}

func splitList(value string) []string {
	return strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r' || r == '\t' || r == ' '
	})
}

func buildValidSettingKeys() map[string]struct{} {
	result := make(map[string]struct{})
	for _, item := range defaultSettings() {
		result[item.Namespace+":"+item.Key] = struct{}{}
	}
	return result
}
