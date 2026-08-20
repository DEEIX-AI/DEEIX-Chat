package channel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	domainchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/channel"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/channelconfig"
)

// ---------------------------------------------------------------------------
// 私有 JSON 解析类型（仅限 infra 层使用，不对外暴露）
// ---------------------------------------------------------------------------

type jsonBreakerErrorClassification struct {
	CircuitErrors   []string `json:"circuit_errors"`
	RateLimitErrors []string `json:"rate_limit_errors"`
	IgnoreErrors    []string `json:"ignore_errors"`
}

type jsonRateLimitDefaults struct {
	BackoffBaseSec    int `json:"backoff_base_sec"`
	BackoffMaxSec     int `json:"backoff_max_sec"`
	BackoffMultiplier int `json:"backoff_multiplier"`
}

// ---------------------------------------------------------------------------
// 全局配置解析方法（实现 ChannelRepository 接口新增方法）
// ---------------------------------------------------------------------------

// GetBreakerErrorClassification 从全局配置读取熔断错误分类，返回含默认值的 domain 类型。
func (r *Repo) GetBreakerErrorClassification(ctx context.Context) (domainchannel.BreakerErrorClassification, error) {
	defaults := domainchannel.BreakerErrorClassification{
		CircuitErrors:   []string{"5xx", "timeout", "connection_error"},
		RateLimitErrors: []string{"429"},
		IgnoreErrors:    []string{"4xx"},
	}
	item, err := r.GetLLMSetting(ctx, "circuit_breaker.error_classification")
	if err != nil || strings.TrimSpace(item.Value) == "" {
		return defaults, nil
	}
	var raw jsonBreakerErrorClassification
	if err = json.Unmarshal([]byte(item.Value), &raw); err != nil {
		return defaults, nil
	}
	if len(raw.CircuitErrors) > 0 {
		defaults.CircuitErrors = raw.CircuitErrors
	}
	if len(raw.RateLimitErrors) > 0 {
		defaults.RateLimitErrors = raw.RateLimitErrors
	}
	if len(raw.IgnoreErrors) > 0 {
		defaults.IgnoreErrors = raw.IgnoreErrors
	}
	return defaults, nil
}

// GetBreakerDefaults 从全局配置读取熔断器默认参数，返回含默认值的 domain 类型。
func (r *Repo) GetBreakerDefaults(ctx context.Context) (domainchannel.BreakerDefaults, error) {
	defaults := domainchannel.DefaultBreakerDefaults()
	item, err := r.GetLLMSetting(ctx, channelconfig.BreakerDefaultsKey)
	if errors.Is(err, ErrLLMSettingNotFound) {
		return defaults, nil
	}
	if err != nil {
		return defaults, err
	}
	if strings.TrimSpace(item.Value) == "" {
		return defaults, nil
	}
	parsed, err := channelconfig.ParseBreakerDefaults(item.Value)
	if err != nil {
		return defaults, fmt.Errorf("parse circuit breaker defaults: %w", err)
	}
	return parsed, nil
}

// GetRateLimitDefaults 从全局配置读取限流退避默认参数，返回含默认值的 domain 类型。
func (r *Repo) GetRateLimitDefaults(ctx context.Context) (domainchannel.RateLimitDefaults, error) {
	defaults := domainchannel.RateLimitDefaults{
		BackoffBaseSec:    5,
		BackoffMaxSec:     60,
		BackoffMultiplier: 2,
	}
	item, err := r.GetLLMSetting(ctx, "rate_limit.defaults")
	if err != nil || strings.TrimSpace(item.Value) == "" {
		return defaults, nil
	}
	var raw jsonRateLimitDefaults
	if err = json.Unmarshal([]byte(item.Value), &raw); err != nil {
		return defaults, nil
	}
	if raw.BackoffBaseSec > 0 {
		defaults.BackoffBaseSec = raw.BackoffBaseSec
	}
	if raw.BackoffMaxSec > 0 {
		defaults.BackoffMaxSec = raw.BackoffMaxSec
	}
	if raw.BackoffMultiplier > 0 {
		defaults.BackoffMultiplier = raw.BackoffMultiplier
	}
	return defaults, nil
}
