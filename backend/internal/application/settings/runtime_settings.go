package settings

import (
	"context"
	"strconv"
	"strings"

	domainsettings "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/settings"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/secretbox"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

// RuntimeSettings 负责把数据库中的动态配置应用到运行时配置，并维护配置缓存。
type RuntimeSettings struct {
	repo              repository.SettingsRepository
	cache             repository.SettingsCacheRepository
	dataEncryptionKey string
}

// NewRuntimeSettings 创建运行时配置应用器。
func NewRuntimeSettings(repo repository.SettingsRepository, cache repository.SettingsCacheRepository, dataEncryptionKey string) *RuntimeSettings {
	return &RuntimeSettings{repo: repo, cache: cache, dataEncryptionKey: strings.TrimSpace(dataEncryptionKey)}
}

// ApplyTo 从 DB 加载动态配置并覆盖到 cfg，同时写入 Redis 缓存。
func (r *RuntimeSettings) ApplyTo(ctx context.Context, runtime *config.Runtime) error {
	items, err := r.repo.ListAll(ctx)
	if err != nil {
		return err
	}

	next := runtime.Snapshot()
	for _, item := range items {
		r.cacheSet(ctx, item)
		value, err := r.runtimeValue(item)
		if err != nil {
			return err
		}
		item.Value = value
		r.applyItem(&next, item)
	}
	r.normalizeConfig(&next)
	runtime.Store(next)
	return nil
}

func (r *RuntimeSettings) runtimeValue(item domainsettings.SystemSetting) (string, error) {
	if !isSensitiveSetting(item.Namespace, item.Key) || strings.TrimSpace(item.Value) == "" {
		return item.Value, nil
	}
	return secretbox.DecryptString(r.dataEncryptionKey, item.Value)
}

// InvalidateCache 删除指定配置项的缓存。
func (r *RuntimeSettings) InvalidateCache(ctx context.Context, namespace, key string) {
	if r.cache != nil {
		_ = r.cache.Del(ctx, namespace, key)
	}
}

// InvalidateCacheMulti 批量删除缓存。
func (r *RuntimeSettings) InvalidateCacheMulti(ctx context.Context, items []PatchItem) {
	for _, item := range items {
		r.InvalidateCache(ctx, item.Namespace, item.Key)
	}
}

// cacheSet 将单个配置项写入缓存；缓存不可用不影响配置持久化结果。
func (r *RuntimeSettings) cacheSet(ctx context.Context, item domainsettings.SystemSetting) {
	if r.cache != nil {
		_ = r.cache.Set(ctx, item.Namespace, item.Key, item.Value)
	}
}

func (r *RuntimeSettings) applyItem(cfg *config.Config, item domainsettings.SystemSetting) {
	switch item.Namespace + ":" + item.Key {
	// 认证配置
	case "auth:token_ttl_hours":
		cfg.TokenTTLHours = toInt(item.Value, cfg.TokenTTLHours)
	case "auth:refresh_token_ttl_hours":
		cfg.RefreshTokenTTLHours = toInt(item.Value, cfg.RefreshTokenTTLHours)
	case "auth:login_max_failures":
		cfg.LoginMaxFailures = toInt(item.Value, cfg.LoginMaxFailures)
	case "auth:login_lock_minutes":
		cfg.LoginLockMinutes = toInt(item.Value, cfg.LoginLockMinutes)
	case "auth:rate_limit_rpm":
		cfg.RateLimitRPM = toInt(item.Value, cfg.RateLimitRPM)
	case "auth:public_auth_rate_limit_rpm":
		cfg.PublicAuthRateLimitRPM = toInt(item.Value, cfg.PublicAuthRateLimitRPM)
	case "auth:username_login_enabled":
		cfg.UsernameLoginEnabled = toBool(item.Value, cfg.UsernameLoginEnabled)
	case "auth:email_login_enabled":
		cfg.EmailLoginEnabled = toBool(item.Value, cfg.EmailLoginEnabled)
	case "auth:third_party_login_enabled":
		cfg.ThirdPartyLoginEnabled = toBool(item.Value, cfg.ThirdPartyLoginEnabled)
	case "auth:email_registration_enabled":
		cfg.EmailRegistrationEnabled = toBool(item.Value, cfg.EmailRegistrationEnabled)
	case "auth:email_verification_enabled":
		cfg.EmailVerificationEnabled = toBool(item.Value, cfg.EmailVerificationEnabled)
	case "auth:smtp_host":
		cfg.SMTPHost = strings.TrimSpace(item.Value)
	case "auth:smtp_port":
		cfg.SMTPPort = toInt(item.Value, cfg.SMTPPort)
	case "auth:smtp_username":
		cfg.SMTPUsername = strings.TrimSpace(item.Value)
	case "auth:smtp_password":
		cfg.SMTPPassword = strings.TrimSpace(item.Value)
	case "auth:smtp_from":
		cfg.SMTPFrom = strings.TrimSpace(item.Value)
	case "auth:email_registration_allowed_domains":
		cfg.EmailRegistrationDomains = strings.TrimSpace(item.Value)
	case "auth:email_registration_block_plus_alias":
		cfg.EmailRegistrationNoAlias = toBool(item.Value, cfg.EmailRegistrationNoAlias)
	case "auth:auto_link_verified_email":
		cfg.AutoLinkVerifiedEmail = toBool(item.Value, cfg.AutoLinkVerifiedEmail)

		// 对话配置
	case "chat:max_context_messages":
		cfg.MaxContextMessages = toInt(item.Value, cfg.MaxContextMessages)
	case "chat:context_max_turns":
		cfg.ContextMaxTurns = toInt(item.Value, cfg.ContextMaxTurns)
	case "chat:context_max_input_tokens":
		cfg.ContextMaxInputTokens = toInt(item.Value, cfg.ContextMaxInputTokens)
	case "chat:context_compact_trigger_tokens":
		cfg.ContextCompactTrigger = toInt(item.Value, cfg.ContextCompactTrigger)
	case "chat:context_compact_preserve_recent_turns":
		cfg.ContextCompactPreserve = toInt(item.Value, cfg.ContextCompactPreserve)
	case "chat:conversation_task_model":
		cfg.ConversationTaskModel = item.Value
	case "chat:conversation_title_prompt":
		cfg.ConversationTitlePrompt = item.Value
	case "chat:conversation_labels_prompt":
		cfg.ConversationLabelsPrompt = item.Value
	case "chat:default_system_prompt":
		cfg.DefaultSystemPrompt = item.Value
	case "chat:model_option_policy_mode":
		cfg.ModelOptionPolicyMode = strings.TrimSpace(item.Value)
	case "chat:model_option_allowed_paths":
		cfg.ModelOptionAllowedPaths = item.Value
	case "chat:model_option_denied_paths":
		cfg.ModelOptionDeniedPaths = item.Value
	case "chat:model_option_native_tool_types":
		cfg.NativeToolAllowedTypes = item.Value

		// 存储配置
	case "storage:user_storage_quota_bytes":
		cfg.UserStorageQuotaBytes = toInt64(item.Value, cfg.UserStorageQuotaBytes)
	case "storage:max_upload_file_bytes":
		cfg.MaxUploadFileBytes = toInt64(item.Value, cfg.MaxUploadFileBytes)
	case "storage:max_message_files":
		cfg.MaxMessageFiles = toInt(item.Value, cfg.MaxMessageFiles)

		// 文件处理配置
	case "file:image_max_dimension":
		cfg.ImageMaxDimension = toInt(item.Value, cfg.ImageMaxDimension)
	case "file:full_context_limit_enabled":
		cfg.FileFullContextLimitEnabled = toBool(item.Value, cfg.FileFullContextLimitEnabled)
	case "file:file_full_context_max_bytes":
		cfg.FileFullContextMaxBytes = toOptionalInt64(item.Value, cfg.FileFullContextMaxBytes)
	case "file:full_context_max_tokens":
		cfg.FileFullContextMaxTokens = toOptionalInt(item.Value, cfg.FileFullContextMaxTokens)
	case "file:image_max_bytes":
		cfg.FileImageMaxBytes = toOptionalInt64(item.Value, cfg.FileImageMaxBytes)
	case "file:doc_max_bytes":
		cfg.FileDocMaxBytes = toOptionalInt64(item.Value, cfg.FileDocMaxBytes)
	case "file:full_context_pdf_max_pages":
		cfg.FileFullContextPDFMaxPages = toOptionalInt(item.Value, cfg.FileFullContextPDFMaxPages)
	case "file:allowed_mime_types":
		cfg.FileAllowedMIMETypes = item.Value
	case "extract:engine":
		cfg.ExtractEngine = item.Value
	case "extract:ocr_engine":
		cfg.ExtractOCREngine = item.Value
	case "extract:image_ocr_enabled":
		cfg.ExtractImageOCREnabled = toBool(item.Value, cfg.ExtractImageOCREnabled)
	case "extract:pdf_ocr_fallback_enabled":
		cfg.ExtractPDFOCRFallbackEnabled = toBool(item.Value, cfg.ExtractPDFOCRFallbackEnabled)
	case "extract:tika_source":
		cfg.ExtractTikaSource = item.Value
	case "extract:tika_base_url":
		cfg.ExtractTikaBaseURL = item.Value
	case "extract:tika_timeout_seconds":
		cfg.ExtractTikaTimeoutSeconds = toInt(item.Value, cfg.ExtractTikaTimeoutSeconds)
	case "extract:tika_auth_token":
		cfg.ExtractTikaAuthToken = item.Value
	case "extract:docling_base_url":
		cfg.ExtractDoclingBaseURL = item.Value
	case "extract:docling_timeout_seconds":
		cfg.ExtractDoclingTimeoutSeconds = toInt(item.Value, cfg.ExtractDoclingTimeoutSeconds)
	case "extract:docling_auth_token":
		cfg.ExtractDoclingAuthToken = item.Value
	case "extract:tesseract_ocr_base_url":
		cfg.ExtractTesseractOCRBaseURL = item.Value
	case "extract:tesseract_ocr_timeout_seconds":
		cfg.ExtractTesseractOCRTimeoutSeconds = toInt(item.Value, cfg.ExtractTesseractOCRTimeoutSeconds)
	case "extract:tesseract_ocr_auth_token":
		cfg.ExtractTesseractOCRAuthToken = item.Value
	case "extract:rapidocr_source":
		cfg.ExtractRapidOCRSource = item.Value
	case "extract:rapidocr_base_url":
		cfg.ExtractRapidOCRBaseURL = item.Value
	case "extract:rapidocr_timeout_seconds":
		cfg.ExtractRapidOCRTimeoutSeconds = toInt(item.Value, cfg.ExtractRapidOCRTimeoutSeconds)
	case "extract:rapidocr_auth_token":
		cfg.ExtractRapidOCRAuthToken = item.Value
	case "extract:paddle_ocr_base_url":
		cfg.ExtractPaddleOCRBaseURL = item.Value
	case "extract:paddle_ocr_timeout_seconds":
		cfg.ExtractPaddleOCRTimeoutSeconds = toInt(item.Value, cfg.ExtractPaddleOCRTimeoutSeconds)
	case "extract:paddle_ocr_auth_token":
		cfg.ExtractPaddleOCRAuthToken = item.Value
	case "extract:tencent_ocr_secret_id":
		cfg.ExtractTencentOCRSecretID = item.Value
	case "extract:tencent_ocr_secret_key":
		cfg.ExtractTencentOCRSecretKey = item.Value
	case "extract:tencent_ocr_region":
		cfg.ExtractTencentOCRRegion = item.Value
	case "extract:tencent_ocr_endpoint":
		cfg.ExtractTencentOCREndpoint = item.Value
	case "extract:tencent_ocr_timeout_seconds":
		cfg.ExtractTencentOCRTimeoutSeconds = toInt(item.Value, cfg.ExtractTencentOCRTimeoutSeconds)
	case "extract:aliyun_ocr_access_key_id":
		cfg.ExtractAliyunOCRAccessKeyID = item.Value
	case "extract:aliyun_ocr_access_key_secret":
		cfg.ExtractAliyunOCRAccessKeySecret = item.Value
	case "extract:aliyun_ocr_region":
		cfg.ExtractAliyunOCRRegion = item.Value
	case "extract:aliyun_ocr_endpoint":
		cfg.ExtractAliyunOCREndpoint = item.Value
	case "extract:aliyun_ocr_timeout_seconds":
		cfg.ExtractAliyunOCRTimeoutSeconds = toInt(item.Value, cfg.ExtractAliyunOCRTimeoutSeconds)
	case "extract:mineru_source":
		cfg.ExtractMinerUSource = item.Value
	case "extract:mineru_base_url":
		cfg.ExtractMinerUBaseURL = item.Value
	case "extract:mineru_timeout_seconds":
		cfg.ExtractMinerUTimeoutSeconds = toInt(item.Value, cfg.ExtractMinerUTimeoutSeconds)
	case "extract:mineru_auth_token":
		cfg.ExtractMinerUAuthToken = item.Value
	case "extract:llm_ocr_base_url":
		cfg.ExtractLLMOCRBaseURL = item.Value
	case "extract:llm_ocr_model":
		cfg.ExtractLLMOCRModel = item.Value
	case "extract:llm_ocr_timeout_seconds":
		cfg.ExtractLLMOCRTimeoutSeconds = toInt(item.Value, cfg.ExtractLLMOCRTimeoutSeconds)
	case "extract:llm_ocr_auth_token":
		cfg.ExtractLLMOCRAuthToken = item.Value
	case "extract:llm_ocr_prompt":
		cfg.ExtractLLMOCRPrompt = item.Value
	case "file:embedding_enabled":
		cfg.EmbeddingEnabled = toBool(item.Value, cfg.EmbeddingEnabled)
	case "file:embedding_host":
		cfg.EmbeddingHost = item.Value
	case "file:embedding_key":
		cfg.EmbeddingKey = item.Value
	case "file:embedding_timeout_seconds":
		cfg.EmbeddingTimeoutSeconds = toInt(item.Value, cfg.EmbeddingTimeoutSeconds)
	case "file:embedding_output_dimensions":
		cfg.EmbeddingOutputDimensions = toInt(item.Value, cfg.EmbeddingOutputDimensions)
	case "file:embedding_normalize":
		cfg.EmbeddingNormalize = toBool(item.Value, cfg.EmbeddingNormalize)
	case "file:embedding_model_signature":
		cfg.EmbeddingModelSignature = item.Value
	case "file:embed_trigger_on_upload":
		cfg.EmbedTriggerOnUpload = toBool(item.Value, cfg.EmbedTriggerOnUpload)
	case "file:embed_chunk_size_tokens":
		cfg.EmbedChunkSizeTokens = toInt(item.Value, cfg.EmbedChunkSizeTokens)
	case "file:embed_chunk_overlap_tokens":
		cfg.EmbedChunkOverlapTokens = toInt(item.Value, cfg.EmbedChunkOverlapTokens)
	case "file:embed_batch_size":
		cfg.EmbedBatchSize = toInt(item.Value, cfg.EmbedBatchSize)
	case "file:rag_top_k":
		cfg.RAGTopK = toInt(item.Value, cfg.RAGTopK)
	case "file:rag_model":
		cfg.RAGModel = item.Value
		// chat (补充)
	case "chat:rag_enabled":
		cfg.RAGEnabled = toBool(item.Value, cfg.RAGEnabled)
	case "chat:rag_min_similarity":
		cfg.RAGMinSimilarity = toFloat(item.Value, cfg.RAGMinSimilarity)
	case "chat:rag_token_budget":
		cfg.RAGTokenBudget = toInt(item.Value, cfg.RAGTokenBudget)
	case "chat:rag_fetch_multiplier":
		cfg.RAGFetchMultiplier = toInt(item.Value, cfg.RAGFetchMultiplier)
	case "chat:rag_wait_ready_ms":
		cfg.RAGWaitReadyMS = toInt(item.Value, cfg.RAGWaitReadyMS)
	case "chat:rag_query_history_turns":
		cfg.RAGQueryHistoryTurns = toInt(item.Value, cfg.RAGQueryHistoryTurns)
	case "chat:rag_retrieval_cache_ttl_seconds":
		cfg.RAGRetrievalCacheTTL = toInt(item.Value, cfg.RAGRetrievalCacheTTL)
	case "chat:rag_hybrid_enabled":
		cfg.RAGHybridEnabled = toBool(item.Value, cfg.RAGHybridEnabled)
	case "chat:context_compact_highlights_per_role":
		cfg.ContextCompactHighlightsPerRole = toInt(item.Value, cfg.ContextCompactHighlightsPerRole)
	case "chat:context_compact_snippet_chars":
		cfg.ContextCompactSnippetChars = toInt(item.Value, cfg.ContextCompactSnippetChars)
	case "chat:compact_llm_enabled":
		cfg.CompactLLMEnabled = toBool(item.Value, cfg.CompactLLMEnabled)
	case "chat:compact_task_model":
		cfg.CompactTaskModel = item.Value
	case "chat:compact_async_enabled":
		cfg.CompactAsyncEnabled = toBool(item.Value, cfg.CompactAsyncEnabled)
	case "chat:compact_max_failures":
		cfg.CompactMaxFailures = toInt(item.Value, cfg.CompactMaxFailures)
	case "chat:compact_system_prompt":
		cfg.CompactSystemPrompt = item.Value
	case "chat:compact_light_prompt":
		cfg.CompactLightPrompt = item.Value
	case "chat:context_token_budget_enabled":
		cfg.ContextTokenBudgetEnabled = toBool(item.Value, cfg.ContextTokenBudgetEnabled)
	case "chat:message_embedding_enabled":
		cfg.MessageEmbeddingEnabled = toBool(item.Value, cfg.MessageEmbeddingEnabled)
	case "chat:semantic_context_enabled":
		cfg.SemanticContextEnabled = toBool(item.Value, cfg.SemanticContextEnabled)
	case "chat:process_trace_enabled":
		cfg.ProcessTraceEnabled = toBool(item.Value, cfg.ProcessTraceEnabled)
	case "chat:process_trace_visible_to_user":
		cfg.ProcessTraceVisibleToUser = toBool(item.Value, cfg.ProcessTraceVisibleToUser)
	case "chat:process_trace_store_upstream_think":
		cfg.ProcessTraceStoreUpstreamThink = toBool(item.Value, cfg.ProcessTraceStoreUpstreamThink)
	case "chat:process_trace_persist_inflight":
		cfg.ProcessTracePersistInflight = toBool(item.Value, cfg.ProcessTracePersistInflight)
	case "chat:context_artifact_retention_days":
		cfg.ContextArtifactRetentionDays = toInt(item.Value, cfg.ContextArtifactRetentionDays)
		// MCP 配置
	case "mcp:mcp_enable":
		cfg.MCPEnable = toBool(item.Value, cfg.MCPEnable)
	case "mcp:mcp_tool_timeout_seconds":
		cfg.MCPToolTimeoutSeconds = toInt(item.Value, cfg.MCPToolTimeoutSeconds)
	case "mcp:mcp_tool_retry_count":
		cfg.MCPToolRetryCount = toInt(item.Value, cfg.MCPToolRetryCount)
	case "mcp:mcp_max_concurrent_calls":
		cfg.MCPMaxConcurrentCalls = toInt(item.Value, cfg.MCPMaxConcurrentCalls)
	case "mcp:mcp_max_llm_calls_per_run":
		cfg.MCPMaxLLMCallsPerRun = toInt(item.Value, cfg.MCPMaxLLMCallsPerRun)
	case "mcp:mcp_max_tool_calls_per_run":
		cfg.MCPMaxToolCallsPerRun = toInt(item.Value, cfg.MCPMaxToolCallsPerRun)

	}
}

func (r *RuntimeSettings) normalizeConfig(cfg *config.Config) {
	if !cfg.EmailLoginEnabled {
		cfg.EmailRegistrationEnabled = false
	}
	if !cfg.EmbeddingEnabled || strings.TrimSpace(cfg.EmbeddingHost) == "" || strings.TrimSpace(cfg.RAGModel) == "" {
		cfg.RAGEnabled = false
		cfg.MessageEmbeddingEnabled = false
		cfg.SemanticContextEnabled = false
	}
	if !cfg.MessageEmbeddingEnabled {
		cfg.SemanticContextEnabled = false
	}
	if cfg.TokenTTLHours <= 0 {
		cfg.TokenTTLHours = 24
	}
	if cfg.RefreshTokenTTLHours <= 0 {
		cfg.RefreshTokenTTLHours = 720
	}
	switch strings.TrimSpace(cfg.ModelOptionPolicyMode) {
	case "allowlist", "denylist", "disabled":
	default:
		cfg.ModelOptionPolicyMode = "allowlist"
	}
	if strings.TrimSpace(cfg.ModelOptionAllowedPaths) == "" {
		cfg.ModelOptionAllowedPaths = config.DefaultModelOptionAllowedPathsJSON()
	}
	if strings.TrimSpace(cfg.ModelOptionDeniedPaths) == "" {
		cfg.ModelOptionDeniedPaths = config.DefaultModelOptionDeniedPathsJSON()
	}
	if strings.TrimSpace(cfg.NativeToolAllowedTypes) == "" {
		cfg.NativeToolAllowedTypes = config.DefaultNativeToolAllowedTypesJSON()
	}
	if !cfg.FileFullContextLimitEnabled {
		cfg.FileFullContextMaxBytes = 0
		cfg.FileFullContextMaxTokens = 0
		cfg.FileFullContextPDFMaxPages = 0
	}
}

func toInt(s string, fallback int) int {
	v, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return fallback
	}
	return v
}

func toOptionalInt(s string, fallback int) int {
	if strings.TrimSpace(s) == "" {
		return 0
	}
	return toInt(s, fallback)
}

func toInt64(s string, fallback int64) int64 {
	v, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return fallback
	}
	return v
}

func toOptionalInt64(s string, fallback int64) int64 {
	if strings.TrimSpace(s) == "" {
		return 0
	}
	return toInt64(s, fallback)
}

func toBool(s string, fallback bool) bool {
	v, err := strconv.ParseBool(s)
	if err != nil {
		return fallback
	}
	return v
}

func toFloat(s string, fallback float64) float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return fallback
	}
	return v
}
