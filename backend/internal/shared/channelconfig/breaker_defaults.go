package channelconfig

import (
	"encoding/json"
	"errors"
	"strings"

	domainchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/channel"
)

// BreakerDefaultsKey 是熔断默认参数在 llm 设置命名空间中的键。
const BreakerDefaultsKey = "circuit_breaker.defaults"

// ErrInvalidBreakerDefaults 表示熔断默认参数不是有效的配置对象。
var ErrInvalidBreakerDefaults = errors.New("invalid circuit breaker defaults")

type breakerDefaultsDocument struct {
	Enabled                  bool   `json:"enabled"`
	ModelFailureThreshold    int    `json:"model_failure_threshold"`
	ModelDurationMin         int    `json:"model_duration_min"`
	ModelWindowMin           int    `json:"model_window_min"`
	UpstreamFailureThreshold int    `json:"upstream_failure_threshold"`
	UpstreamModelThreshold   int    `json:"upstream_model_threshold"`
	UpstreamThresholdLogic   string `json:"upstream_threshold_logic"`
	UpstreamDurationMin      int    `json:"upstream_duration_min"`
	UpstreamWindowMin        int    `json:"upstream_window_min"`
}

type breakerDefaultsInput struct {
	Enabled                  *bool   `json:"enabled"`
	ModelFailureThreshold    *int    `json:"model_failure_threshold"`
	ModelDurationMin         *int    `json:"model_duration_min"`
	ModelWindowMin           *int    `json:"model_window_min"`
	UpstreamFailureThreshold *int    `json:"upstream_failure_threshold"`
	UpstreamModelThreshold   *int    `json:"upstream_model_threshold"`
	UpstreamThresholdLogic   *string `json:"upstream_threshold_logic"`
	UpstreamDurationMin      *int    `json:"upstream_duration_min"`
	UpstreamWindowMin        *int    `json:"upstream_window_min"`
}

var breakerDefaultKeys = []string{
	"enabled",
	"model_failure_threshold",
	"model_duration_min",
	"model_window_min",
	"upstream_failure_threshold",
	"upstream_model_threshold",
	"upstream_threshold_logic",
	"upstream_duration_min",
	"upstream_window_min",
}

// ParseBreakerDefaults 统一解析并补齐熔断默认参数；缺失或为 0 的数值沿用内置默认值。
func ParseBreakerDefaults(value string) (domainchannel.BreakerDefaults, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal([]byte(value), &object); err != nil || object == nil {
		return domainchannel.BreakerDefaults{}, ErrInvalidBreakerDefaults
	}
	var input breakerDefaultsInput
	if err := json.Unmarshal([]byte(value), &input); err != nil {
		return domainchannel.BreakerDefaults{}, ErrInvalidBreakerDefaults
	}
	for _, key := range breakerDefaultKeys {
		if raw, exists := object[key]; exists && strings.TrimSpace(string(raw)) == "null" {
			return domainchannel.BreakerDefaults{}, ErrInvalidBreakerDefaults
		}
	}

	valueOrZero := func(value *int) int {
		if value == nil {
			return 0
		}
		return *value
	}
	logic := ""
	if input.UpstreamThresholdLogic != nil {
		logic = *input.UpstreamThresholdLogic
	}
	if !(domainchannel.BreakerDefaults{
		ModelFailureThreshold:    valueOrZero(input.ModelFailureThreshold),
		ModelDurationMin:         valueOrZero(input.ModelDurationMin),
		ModelWindowMin:           valueOrZero(input.ModelWindowMin),
		UpstreamFailureThreshold: valueOrZero(input.UpstreamFailureThreshold),
		UpstreamModelThreshold:   valueOrZero(input.UpstreamModelThreshold),
		UpstreamThresholdLogic:   logic,
		UpstreamDurationMin:      valueOrZero(input.UpstreamDurationMin),
		UpstreamWindowMin:        valueOrZero(input.UpstreamWindowMin),
	}).Valid() {
		return domainchannel.BreakerDefaults{}, ErrInvalidBreakerDefaults
	}

	defaults := domainchannel.DefaultBreakerDefaults()
	if input.Enabled != nil {
		defaults.Enabled = *input.Enabled
	}
	applyPositive := func(target *int, value *int) {
		if value != nil && *value > 0 {
			*target = *value
		}
	}
	applyPositive(&defaults.ModelFailureThreshold, input.ModelFailureThreshold)
	applyPositive(&defaults.ModelDurationMin, input.ModelDurationMin)
	applyPositive(&defaults.ModelWindowMin, input.ModelWindowMin)
	applyPositive(&defaults.UpstreamFailureThreshold, input.UpstreamFailureThreshold)
	applyPositive(&defaults.UpstreamModelThreshold, input.UpstreamModelThreshold)
	applyPositive(&defaults.UpstreamDurationMin, input.UpstreamDurationMin)
	applyPositive(&defaults.UpstreamWindowMin, input.UpstreamWindowMin)
	if input.UpstreamThresholdLogic != nil && strings.TrimSpace(*input.UpstreamThresholdLogic) != "" {
		defaults.UpstreamThresholdLogic = strings.TrimSpace(*input.UpstreamThresholdLogic)
	}
	return defaults, nil
}

// MarshalBreakerDefaults 将领域配置编码为持久化 JSON 契约。
func MarshalBreakerDefaults(defaults domainchannel.BreakerDefaults) (string, error) {
	value, err := json.Marshal(breakerDefaultsDocument{
		Enabled:                  defaults.Enabled,
		ModelFailureThreshold:    defaults.ModelFailureThreshold,
		ModelDurationMin:         defaults.ModelDurationMin,
		ModelWindowMin:           defaults.ModelWindowMin,
		UpstreamFailureThreshold: defaults.UpstreamFailureThreshold,
		UpstreamModelThreshold:   defaults.UpstreamModelThreshold,
		UpstreamThresholdLogic:   defaults.UpstreamThresholdLogic,
		UpstreamDurationMin:      defaults.UpstreamDurationMin,
		UpstreamWindowMin:        defaults.UpstreamWindowMin,
	})
	if err != nil {
		return "", err
	}
	return string(value), nil
}
