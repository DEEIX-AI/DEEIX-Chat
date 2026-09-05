package settings

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	appaudit "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/audit"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/extraction"
	domainsettings "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/settings"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

// Service 封装 settings 业务逻辑。
type Service struct {
	repo              repository.SettingsRepository
	dataEncryptionKey string
	authSafety        authSafetyService
	vectorStore       vectorStoreAvailabilityService
	auditWriter       auditWriter
}

type authSafetyService interface {
	HasActiveSuperAdminIdentity(ctx context.Context) (bool, error)
}

type vectorStoreAvailabilityService interface {
	VectorStoreAvailable(ctx context.Context) (bool, error)
}

type auditWriter interface {
	Write(ctx context.Context, input appaudit.WriteInput)
}

// NewService 创建服务。
func NewService(repo repository.SettingsRepository, dataEncryptionKey string) *Service {
	return &Service{repo: repo, dataEncryptionKey: strings.TrimSpace(dataEncryptionKey)}
}

func (s *Service) SetAuthSafetyService(service authSafetyService) {
	s.authSafety = service
}

func (s *Service) SetVectorStoreAvailabilityService(service vectorStoreAvailabilityService) {
	s.vectorStore = service
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
	Detail    any
}

// RecordAudit 记录系统设置审计日志。
func (s *Service) RecordAudit(ctx context.Context, input AuditInput) {
	if s.auditWriter == nil {
		return
	}
	s.auditWriter.Write(ctx, appaudit.WriteInput{
		RequestID:   input.RequestID,
		ActorUserID: input.UserID,
		Action:      input.Action,
		Resource:    "system_settings",
		IP:          input.ClientIP,
		UserAgent:   input.UserAgent,
		Detail:      input.Detail,
	})
}

// Seed 将注册表中的默认配置写入数据库（仅插入不存在的 key），并清理已废弃的配置项。
func (s *Service) Seed(ctx context.Context) error {
	items, err := s.encryptSettingsForStorage(defaultSettings())
	if err != nil {
		return err
	}
	if err := s.repo.UpsertWithDescription(ctx, items); err != nil {
		return err
	}
	// Install replacement defaults before deleting obsolete keys so a partial
	// startup failure never leaves the deployment without either configuration.
	for _, item := range obsoleteSettings() {
		if err := s.repo.Delete(ctx, item.Namespace, item.Key); err != nil {
			return err
		}
	}
	if err := s.migrateDefaultAllowedMIMETypes(ctx); err != nil {
		return err
	}
	return s.migrateDefaultModelOptionAllowedPaths(ctx)
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

func (s *Service) migrateDefaultAllowedMIMETypes(ctx context.Context) error {
	items, err := s.repo.ListByNamespace(ctx, "file")
	if err != nil {
		return err
	}
	for _, item := range items {
		if item.Key != "allowed_mime_types" {
			continue
		}
		value := strings.TrimSpace(item.Value)
		if value == "" || !sameCSVSet(value, legacyDefaultAllowedMIMETypes) {
			return nil
		}
		return s.resetToRegistryDefault(ctx, "file", "allowed_mime_types")
	}
	return nil
}

// resetToRegistryDefault 把仍停留在历史默认值上的配置项覆盖为注册表当前默认值。
func (s *Service) resetToRegistryDefault(ctx context.Context, namespace string, key string) error {
	spec, ok := lookupSettingSpec(namespace, key)
	if !ok {
		return newSettingValidationError(settingCodeInvalidKey, SettingValidationDetails{Rule: "invalid_key"})
	}
	updates, err := s.encryptSettingsForStorage([]domainsettings.SystemSetting{spec.seedSetting()})
	if err != nil {
		return err
	}
	return s.repo.Upsert(ctx, updates)
}

func (s *Service) migrateDefaultModelOptionAllowedPaths(ctx context.Context) error {
	items, err := s.repo.ListByNamespace(ctx, "chat")
	if err != nil {
		return err
	}
	for _, item := range items {
		if item.Key != "model_option_allowed_paths" || !isLegacyDefaultModelOptionAllowedPaths(item.Value) {
			continue
		}
		return s.resetToRegistryDefault(ctx, "chat", "model_option_allowed_paths")
	}
	return nil
}

func isLegacyDefaultModelOptionAllowedPaths(value string) bool {
	current := map[string][]string{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(value)), &current); err != nil {
		return false
	}
	latestDefault := map[string][]string{}
	if err := json.Unmarshal([]byte(config.DefaultModelOptionAllowedPathsJSON()), &latestDefault); err != nil {
		return false
	}
	previousGenerateContentDefault := cloneStringSliceMap(latestDefault)
	previousGenerateContentDefault["gemini_generate_content"] = removeStringValue(
		removeStringValue(
			previousGenerateContentDefault["gemini_generate_content"],
			"generationConfig.thinkingConfig.includeThoughts",
		),
		"generationConfig.thinkingConfig.thinkingLevel",
	)
	previousInteractionsDefault := cloneStringSliceMap(latestDefault)
	previousInteractionsDefault["gemini_interactions"] = removeStringValue(
		previousInteractionsDefault["gemini_interactions"],
		"generation_config.thinking_summaries",
	)
	previousCombinedDefault := cloneStringSliceMap(previousGenerateContentDefault)
	previousCombinedDefault["gemini_interactions"] = removeStringValue(
		previousCombinedDefault["gemini_interactions"],
		"generation_config.thinking_summaries",
	)
	legacyInteractionsDefault := cloneStringSliceMap(latestDefault)
	legacyInteractionsDefault["gemini_interactions"] = append(
		removeStringValue(legacyInteractionsDefault["gemini_interactions"], "response_format.schema"),
		"responseFormat.type",
		"responseFormat.aspectRatio",
		"responseFormat.imageSize",
		"responseFormat.mimeType",
		"generationConfig.videoConfig.task",
	)
	legacyInteractionsWithoutSummaries := cloneStringSliceMap(legacyInteractionsDefault)
	legacyInteractionsWithoutSummaries["gemini_interactions"] = removeStringValue(
		legacyInteractionsWithoutSummaries["gemini_interactions"],
		"generation_config.thinking_summaries",
	)
	legacyCombinedDefault := cloneStringSliceMap(legacyInteractionsWithoutSummaries)
	legacyCombinedDefault["gemini_generate_content"] = previousGenerateContentDefault["gemini_generate_content"]
	previousWithoutXAIVideoExtensions := cloneStringSliceMap(latestDefault)
	delete(previousWithoutXAIVideoExtensions, "xai_video_extensions")
	previousDefaults := []map[string][]string{
		previousWithoutXAIVideoExtensions,
		previousGenerateContentDefault,
		previousInteractionsDefault,
		previousCombinedDefault,
		legacyInteractionsDefault,
		legacyInteractionsWithoutSummaries,
		legacyCombinedDefault,
	}
	for _, previousDefault := range previousDefaults {
		if sameStringSliceMap(current, previousDefault) {
			return true
		}
	}
	olderDefaults := append([]map[string][]string{cloneStringSliceMap(latestDefault)}, previousDefaults...)
	for _, olderDefault := range olderDefaults {
		delete(olderDefault, "xai_video")
		if sameStringSliceMap(current, olderDefault) {
			return true
		}
		olderDefault["xai_responses"] = []string{"reasoning.effort"}
		if sameStringSliceMap(current, olderDefault) {
			return true
		}
	}
	return false
}

func cloneStringSliceMap(value map[string][]string) map[string][]string {
	result := make(map[string][]string, len(value))
	for key, items := range value {
		result[key] = append([]string(nil), items...)
	}
	return result
}

func removeStringValue(values []string, target string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value != target {
			result = append(result, value)
		}
	}
	return result
}

func sameStringSliceMap(left map[string][]string, right map[string][]string) bool {
	if len(left) != len(right) {
		return false
	}
	for key, leftValues := range left {
		rightValues, ok := right[key]
		if !ok || len(leftValues) != len(rightValues) {
			return false
		}
		values := make(map[string]int, len(leftValues))
		for _, value := range leftValues {
			values[value]++
		}
		for _, value := range rightValues {
			values[value]--
		}
		for _, count := range values {
			if count != 0 {
				return false
			}
		}
	}
	return true
}

func sameCSVSet(left string, right string) bool {
	leftSet := csvSet(left)
	rightSet := csvSet(right)
	if len(leftSet) != len(rightSet) {
		return false
	}
	for item := range leftSet {
		if _, ok := rightSet[item]; !ok {
			return false
		}
	}
	return true
}

func csvSet(raw string) map[string]struct{} {
	parts := strings.Split(raw, ",")
	result := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		value := strings.ToLower(strings.TrimSpace(part))
		if value == "" {
			continue
		}
		result[value] = struct{}{}
	}
	return result
}

// BatchUpdate 批量更新配置项。
func (s *Service) BatchUpdate(ctx context.Context, patches []PatchItem) (map[string][]SettingItem, error) {
	for _, p := range patches {
		if !IsValidNamespace(p.Namespace) {
			return nil, newSettingValidationError(settingCodeInvalidNamespace, SettingValidationDetails{Rule: "invalid_namespace"})
		}
		if err := validatePatchItem(p); err != nil {
			return nil, err
		}
	}

	patches, err := s.applyAuthSettingDependencies(ctx, patches)
	if err != nil {
		return nil, err
	}
	patches, err = s.applyEmbeddingDependentCascades(ctx, patches)
	if err != nil {
		return nil, err
	}

	if err := s.validateFileProcessingSettings(ctx, patches); err != nil {
		return nil, err
	}
	if err := s.validateEmbeddingDependentSettings(ctx, patches); err != nil {
		return nil, err
	}
	if err := s.validateBillingPaymentSettings(ctx, patches); err != nil {
		return nil, err
	}
	items, err := s.preparePatchItemsForStorage(patches)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Upsert(ctx, items); err != nil {
		return nil, err
	}

	return s.ListAll(ctx)
}

func (s *Service) groupByNamespace(items []domainsettings.SystemSetting) map[string][]SettingItem {
	result := make(map[string][]SettingItem)
	for _, item := range items {
		if _, ok := lookupSettingSpec(item.Namespace, item.Key); !ok {
			continue
		}
		result[item.Namespace] = append(result[item.Namespace], s.settingResponse(item))
	}
	return result
}

func validatePatchItem(item PatchItem) error {
	spec, ok := lookupSettingSpec(item.Namespace, item.Key)
	if !ok {
		return newSettingValidationError(settingCodeInvalidKey, SettingValidationDetails{Rule: "invalid_key"})
	}
	if item.Clear {
		if !spec.Sensitive {
			return newSettingValidationError(settingValidationCode(item.Namespace, item.Key), SettingValidationDetails{
				Field: spec.fullKey(),
				Rule:  "clear_not_allowed",
			})
		}
		return nil
	}
	return validateSettingValue(item.Namespace, item.Key, item.Value)
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
			return newSettingValidationError(settingCodeExtractInvalid, SettingValidationDetails{
				Field: "extract:tika_base_url", Rule: "required_when", Param: "extract:engine=tika",
			})
		}
	}
	if strings.TrimSpace(next["extract:engine"]) == extraction.EngineDocling && strings.TrimSpace(next["extract:docling_base_url"]) == "" {
		return newSettingValidationError(settingCodeExtractInvalid, SettingValidationDetails{
			Field: "extract:docling_base_url", Rule: "required_when", Param: "extract:engine=docling",
		})
	}
	if strings.TrimSpace(next["extract:engine"]) == extraction.EngineMinerU && strings.TrimSpace(next["extract:mineru_base_url"]) == "" {
		return newSettingValidationError(settingCodeExtractInvalid, SettingValidationDetails{
			Field: "extract:mineru_base_url", Rule: "required_when", Param: "extract:engine=mineru",
		})
	}

	imageOCREnabled, _ := strconv.ParseBool(strings.TrimSpace(next["extract:image_ocr_enabled"]))
	pdfOCRFallbackEnabled, _ := strconv.ParseBool(strings.TrimSpace(next["extract:pdf_ocr_fallback_enabled"]))
	if !imageOCREnabled && !pdfOCRFallbackEnabled {
		return nil
	}

	switch strings.TrimSpace(next["extract:ocr_engine"]) {
	case extraction.OCREngineTesseract:
		if strings.TrimSpace(next["extract:tesseract_ocr_base_url"]) == "" {
			return newSettingValidationError(settingCodeExtractInvalid, SettingValidationDetails{
				Field: "extract:tesseract_ocr_base_url", Rule: "required_when", Param: "extract:ocr_engine=tesseract",
			})
		}
	case extraction.OCREngineRapidOCR:
		if strings.TrimSpace(next["extract:rapidocr_base_url"]) == "" {
			return newSettingValidationError(settingCodeExtractInvalid, SettingValidationDetails{
				Field: "extract:rapidocr_base_url", Rule: "required_when", Param: "extract:ocr_engine=rapidocr",
			})
		}
	case extraction.OCREnginePaddle:
		if strings.TrimSpace(next["extract:paddle_ocr_base_url"]) == "" {
			return newSettingValidationError(settingCodeExtractInvalid, SettingValidationDetails{
				Field: "extract:paddle_ocr_base_url", Rule: "required_when", Param: "extract:ocr_engine=paddle",
			})
		}
	case extraction.OCREngineTencent:
		if strings.TrimSpace(next["extract:tencent_ocr_secret_id"]) == "" {
			return newSettingValidationError(settingCodeExtractInvalid, SettingValidationDetails{
				Field: "extract:tencent_ocr_secret_id", Rule: "required_when", Param: "extract:ocr_engine=tencent",
			})
		}
		if strings.TrimSpace(next["extract:tencent_ocr_secret_key"]) == "" {
			return newSettingValidationError(settingCodeExtractInvalid, SettingValidationDetails{
				Field: "extract:tencent_ocr_secret_key", Rule: "required_when", Param: "extract:ocr_engine=tencent",
			})
		}
		if strings.TrimSpace(next["extract:tencent_ocr_region"]) == "" {
			return newSettingValidationError(settingCodeExtractInvalid, SettingValidationDetails{
				Field: "extract:tencent_ocr_region", Rule: "required_when", Param: "extract:ocr_engine=tencent",
			})
		}
	case extraction.OCREngineAliyun:
		if strings.TrimSpace(next["extract:aliyun_ocr_access_key_id"]) == "" {
			return newSettingValidationError(settingCodeExtractInvalid, SettingValidationDetails{
				Field: "extract:aliyun_ocr_access_key_id", Rule: "required_when", Param: "extract:ocr_engine=aliyun",
			})
		}
		if strings.TrimSpace(next["extract:aliyun_ocr_access_key_secret"]) == "" {
			return newSettingValidationError(settingCodeExtractInvalid, SettingValidationDetails{
				Field: "extract:aliyun_ocr_access_key_secret", Rule: "required_when", Param: "extract:ocr_engine=aliyun",
			})
		}
		if strings.TrimSpace(next["extract:aliyun_ocr_region"]) == "" {
			return newSettingValidationError(settingCodeExtractInvalid, SettingValidationDetails{
				Field: "extract:aliyun_ocr_region", Rule: "required_when", Param: "extract:ocr_engine=aliyun",
			})
		}
	case extraction.OCREngineMistral:
		if strings.TrimSpace(next["extract:mistral_ocr_base_url"]) == "" {
			return newSettingValidationError(settingCodeExtractInvalid, SettingValidationDetails{
				Field: "extract:mistral_ocr_base_url", Rule: "required_when", Param: "extract:ocr_engine=mistral",
			})
		}
		if strings.TrimSpace(next["extract:mistral_ocr_auth_token"]) == "" {
			return newSettingValidationError(settingCodeExtractInvalid, SettingValidationDetails{
				Field: "extract:mistral_ocr_auth_token", Rule: "required_when", Param: "extract:ocr_engine=mistral",
			})
		}
		if strings.TrimSpace(next["extract:mistral_ocr_model"]) == "" {
			return newSettingValidationError(settingCodeExtractInvalid, SettingValidationDetails{
				Field: "extract:mistral_ocr_model", Rule: "required_when", Param: "extract:ocr_engine=mistral",
			})
		}
	case extraction.OCREngineLLM:
		if strings.TrimSpace(next["extract:llm_ocr_base_url"]) == "" {
			return newSettingValidationError(settingCodeExtractInvalid, SettingValidationDetails{
				Field: "extract:llm_ocr_base_url", Rule: "required_when", Param: "extract:ocr_engine=llm",
			})
		}
		if strings.TrimSpace(next["extract:llm_ocr_model"]) == "" {
			return newSettingValidationError(settingCodeExtractInvalid, SettingValidationDetails{
				Field: "extract:llm_ocr_model", Rule: "required_when", Param: "extract:ocr_engine=llm",
			})
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
			return nil, newSettingValidationError(settingCodeInvalidValue, SettingValidationDetails{
				Field: "auth:third_party_login_enabled", Rule: "dependency", Param: "username_or_email_login",
			})
		}
		if s.authSafety == nil {
			return nil, newSettingValidationError(settingCodeInvalidValue, SettingValidationDetails{
				Field: "auth:third_party_login_enabled", Rule: "dependency", Param: "superadmin_identity",
			})
		}
		hasBoundAdmin, err := s.authSafety.HasActiveSuperAdminIdentity(ctx)
		if err != nil {
			return nil, err
		}
		if !hasBoundAdmin {
			return nil, newSettingValidationError(settingCodeInvalidValue, SettingValidationDetails{
				Field: "auth:third_party_login_enabled", Rule: "dependency", Param: "superadmin_identity",
			})
		}
	}
	if !emailLoginEnabled {
		if patchValueIsTrue(patches, "auth", "email_registration_enabled") {
			return nil, newSettingValidationError(settingCodeInvalidValue, SettingValidationDetails{
				Field: "auth:email_registration_enabled", Rule: "dependency", Param: "auth:email_login_enabled",
			})
		}
		patches = upsertPatch(patches, PatchItem{Namespace: "auth", Key: "email_registration_enabled", Value: "false"})
		next["auth:email_registration_enabled"] = "false"
	}
	emailRegistrationEnabled, _ := strconv.ParseBool(next["auth:email_registration_enabled"])
	turnstileRegistrationEnabled, _ := strconv.ParseBool(next["auth:turnstile_registration_enabled"])
	if !emailRegistrationEnabled {
		if patchValueIsTrue(patches, "auth", "turnstile_registration_enabled") {
			return nil, newSettingValidationError(settingCodeInvalidValue, SettingValidationDetails{
				Field: "auth:turnstile_registration_enabled", Rule: "dependency", Param: "auth:email_registration_enabled",
			})
		}
		patches = upsertPatch(patches, PatchItem{Namespace: "auth", Key: "turnstile_registration_enabled", Value: "false"})
		next["auth:turnstile_registration_enabled"] = "false"
		turnstileRegistrationEnabled = false
	}
	if turnstileRegistrationEnabled {
		if strings.TrimSpace(next["auth:turnstile_site_key"]) == "" {
			return nil, newSettingValidationError(settingCodeInvalidValue, SettingValidationDetails{
				Field: "auth:turnstile_site_key", Rule: "required_when", Param: "auth:turnstile_registration_enabled=true",
			})
		}
		if strings.TrimSpace(next["auth:turnstile_secret_key"]) == "" {
			return nil, newSettingValidationError(settingCodeInvalidValue, SettingValidationDetails{
				Field: "auth:turnstile_secret_key", Rule: "required_when", Param: "auth:turnstile_registration_enabled=true",
			})
		}
	}
	emailVerificationEnabled, _ := strconv.ParseBool(next["auth:email_verification_enabled"])
	passwordResetEnabled, _ := strconv.ParseBool(next["auth:password_reset_enabled"])
	if !emailVerificationEnabled {
		if patchValueIsTrue(patches, "auth", "password_reset_enabled") {
			return nil, newSettingValidationError(settingCodeInvalidValue, SettingValidationDetails{
				Field: "auth:password_reset_enabled", Rule: "dependency", Param: "auth:email_verification_enabled",
			})
		}
		patches = upsertPatch(patches, PatchItem{Namespace: "auth", Key: "password_reset_enabled", Value: "false"})
		next["auth:password_reset_enabled"] = "false"
		passwordResetEnabled = false
	}
	if emailVerificationEnabled {
		if err := validateEmailVerificationSMTPSettings(next); err != nil {
			return nil, err
		}
	}
	if passwordResetEnabled && !usernameLoginEnabled && !emailLoginEnabled {
		return nil, newSettingValidationError(settingCodeInvalidValue, SettingValidationDetails{
			Field: "auth:password_reset_enabled", Rule: "dependency", Param: "username_or_email_login",
		})
	}

	return patches, nil
}

func validateEmailVerificationSMTPSettings(next map[string]string) error {
	required := []string{"auth:smtp_host", "auth:smtp_port", "auth:smtp_username", "auth:smtp_password"}
	missing := make([]string, 0, len(required))
	for _, key := range required {
		if strings.TrimSpace(next[key]) == "" {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		return settingValidationForFields(settingCodeSMTPInvalid, missing, "required_together", "auth:email_verification_enabled=true")
	}
	return validateSettingValue("auth", "smtp_port", next["auth:smtp_port"])
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
		if item.Namespace == "file" {
			switch item.Key {
			case "embedding_enabled", "embedding_host", "rag_model":
				requiresValidation = true
			}
		}
		if item.Namespace == "chat" {
			switch item.Key {
			case "rag_enabled", "message_embedding_enabled", "semantic_context_enabled":
				requiresValidation = true
			}
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

	embeddingEnabled, _ := strconv.ParseBool(next["file:embedding_enabled"])
	ragEnabled, _ := strconv.ParseBool(next["chat:rag_enabled"])
	messageEmbeddingEnabled, _ := strconv.ParseBool(next["chat:message_embedding_enabled"])
	semanticContextEnabled, _ := strconv.ParseBool(next["chat:semantic_context_enabled"])
	if semanticContextEnabled && !messageEmbeddingEnabled {
		return newSettingValidationError(settingCodeEmbeddingInvalid, SettingValidationDetails{
			Field: "chat:message_embedding_enabled", Rule: "dependency", Param: "chat:semantic_context_enabled=true",
		})
	}
	if embeddingEnabled || ragEnabled || messageEmbeddingEnabled || semanticContextEnabled {
		if !embeddingServiceReady(next) {
			return newSettingValidationError(settingCodeEmbeddingInvalid, SettingValidationDetails{
				Field: "file:embedding_enabled", Rule: "dependency", Param: "embedding_service_ready",
			})
		}
		if s.vectorStore != nil {
			available, err := s.vectorStore.VectorStoreAvailable(ctx)
			if err != nil {
				return err
			}
			if !available {
				return newSettingValidationError(settingCodeEmbeddingInvalid, SettingValidationDetails{
					Field: "file:embedding_enabled", Rule: "dependency", Param: "vector_store_available",
				})
			}
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
	for _, provider := range providers {
		switch provider {
		case "stripe":
			if err := requireSettingFields(next, []requiredSettingField{
				{key: "billing:stripe_secret_key"},
				{key: "billing:stripe_webhook_secret"},
			}); err != nil {
				return err
			}
		case "epay":
			if err := validateSettingValue("billing", "usd_to_cny_rate", next["billing:usd_to_cny_rate"]); err != nil {
				return err
			}
			if err := requireSettingFields(next, []requiredSettingField{
				{key: "billing:epay_gateway_url"},
				{key: "billing:epay_types"},
				{key: "billing:epay_pid"},
				{key: "billing:epay_key"},
			}); err != nil {
				return err
			}
			if err := validateSettingValue("billing", "epay_gateway_url", next["billing:epay_gateway_url"]); err != nil {
				return err
			}
		default:
			return newSettingValidationError(settingCodeBillingPaymentInvalid, SettingValidationDetails{
				Field: "billing:payment_providers", Rule: "payment_provider", Param: "stripe,epay",
			})
		}
	}
	return nil
}

type requiredSettingField struct {
	key string
}

func requireSettingFields(values map[string]string, fields []requiredSettingField) error {
	missing := make([]string, 0, len(fields))
	for _, item := range fields {
		if strings.TrimSpace(values[item.key]) == "" {
			missing = append(missing, item.key)
		}
	}
	if len(missing) > 0 {
		return settingValidationForFields(settingCodeBillingPaymentInvalid, missing, "required_together", "billing:payment_providers")
	}
	return nil
}

func embeddingServiceReady(settings map[string]string) bool {
	embeddingEnabled, _ := strconv.ParseBool(settings["file:embedding_enabled"])
	return embeddingEnabled &&
		strings.TrimSpace(settings["file:rag_model"]) != "" &&
		strings.TrimSpace(settings["file:embedding_host"]) != ""
}
