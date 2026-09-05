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

const (
	defaultMCPToolTimeoutSeconds = 10
	maxMCPToolTimeoutSeconds     = 1800
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

// applyItem 按注册表把单个配置项写入运行时配置；未注册或不进入 config.Config 的项被忽略。
func (r *RuntimeSettings) applyItem(cfg *config.Config, item domainsettings.SystemSetting) {
	spec, ok := lookupSettingSpec(item.Namespace, item.Key)
	if !ok || spec.Apply == nil {
		return
	}
	spec.Apply(cfg, item.Value)
}

// applyField 返回把配置值解析后写入 cfg 指定字段的函数；解析失败时 parse 保留字段原值。
func applyField[T any](field func(*config.Config) *T, parse func(value string, fallback T) T) func(*config.Config, string) {
	return func(cfg *config.Config, value string) {
		target := field(cfg)
		*target = parse(value, *target)
	}
}

func rawText(value string, _ string) string {
	return value
}

func trimmedText(value string, _ string) string {
	return strings.TrimSpace(value)
}

func (r *RuntimeSettings) normalizeConfig(cfg *config.Config) {
	if !cfg.EmailLoginEnabled {
		cfg.EmailRegistrationEnabled = false
	}
	if !cfg.EmailRegistrationEnabled {
		cfg.TurnstileRegistrationEnabled = false
	}
	if !cfg.EmailVerificationEnabled || (!cfg.UsernameLoginEnabled && !cfg.EmailLoginEnabled) {
		cfg.PasswordResetEnabled = false
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
	if cfg.ContextWindowFallbackTokens < config.MinContextWindowFallbackTokens || cfg.ContextWindowFallbackTokens > config.MaxContextWindowFallbackTokens {
		cfg.ContextWindowFallbackTokens = config.DefaultContextWindowFallbackTokens
	}
	if cfg.ContextCompactTriggerPercent < 0 ||
		cfg.ContextCompactTriggerPercent > config.MaxContextCompactTriggerPercent ||
		(cfg.ContextCompactTriggerPercent > 0 && cfg.ContextCompactTriggerPercent < config.MinContextCompactTriggerPercent) {
		cfg.ContextCompactTriggerPercent = config.DefaultContextCompactTriggerPercent
	}
	if cfg.MCPMaxSelectedToolsPerMessage <= 0 {
		cfg.MCPMaxSelectedToolsPerMessage = config.DefaultMCPMaxSelectedToolsPerMessage
	}
	if cfg.MCPMaxSelectedToolsPerMessage > config.MaxMCPSelectedToolsPerMessage {
		cfg.MCPMaxSelectedToolsPerMessage = config.MaxMCPSelectedToolsPerMessage
	}
	if cfg.MCPToolTimeoutSeconds <= 0 {
		cfg.MCPToolTimeoutSeconds = defaultMCPToolTimeoutSeconds
	}
	if cfg.MCPToolTimeoutSeconds > maxMCPToolTimeoutSeconds {
		cfg.MCPToolTimeoutSeconds = maxMCPToolTimeoutSeconds
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
