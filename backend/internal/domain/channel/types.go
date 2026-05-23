package channel

import "time"

// APIKey API 密钥条目。
type APIKey struct {
	Key    string
	Status string
	Note   string
}

// APIKeysConfig API 密钥轮询配置。
type APIKeysConfig struct {
	Strategy string
	Keys     []APIKey
}

// BreakerErrorClassification 熔断错误分类配置（来自 circuit_breaker.error_classification 全局设置）。
type BreakerErrorClassification struct {
	CircuitErrors   []string
	RateLimitErrors []string
	IgnoreErrors    []string
}

// BreakerDefaults 熔断器全局默认参数（来自 circuit_breaker.defaults 全局设置）。
type BreakerDefaults struct {
	ModelFailureThreshold    int
	ModelDurationMin         int
	ModelWindowMin           int
	UpstreamFailureThreshold int
	UpstreamModelThreshold   int
	UpstreamThresholdLogic   string
	UpstreamDurationMin      int
	UpstreamWindowMin        int
}

// RateLimitDefaults 限流退避全局默认参数（来自 rate_limit.defaults 全局设置）。
type RateLimitDefaults struct {
	BackoffBaseSec    int
	BackoffMaxSec     int
	BackoffMultiplier int
}

// Upstream 表示上游配置。
type Upstream struct {
	ID                   uint
	Name                 string
	BaseURL              string
	Compatible           string
	ProtocolDefaultsJSON string
	Status               string
	ConnectTimeoutMS     int
	ReadTimeoutMS        int
	StreamIdleTimeoutMS  int
	APIKeysEnc           string
	CbFailureThreshold   int
	CbModelThreshold     int
	CbThresholdLogic     string
	CbDurationMin        int
	CbWindowMin          int
	HeadersJSON          string
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// PlatformModel 表示平台对用户提供和计费的模型。
type PlatformModel struct {
	ID                uint
	PlatformModelName string
	Vendor            string
	KindsJSON         string
	Icon              string
	CapabilitiesJSON  string
	SystemPrompt      string
	Status            string
	Description       string
	SortOrder         int
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// UpstreamModel 表示上游真实模型清单。
type UpstreamModel struct {
	ID                uint
	UpstreamID        uint
	BindingCode       string
	UpstreamModelName string
	Vendor            string
	Icon              string
	SuggestedProtocol string
	KindsJSON         string
	Status            string
	Source            string
	LastSyncedAt      *time.Time
	RawJSON           string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// PlatformModelRoute 表示平台模型到上游真实模型的路由绑定。
type PlatformModelRoute struct {
	ID                 uint
	PlatformModelID    uint
	UpstreamModelID    uint
	Protocol           string
	Status             string
	Priority           int
	Weight             int
	Source             string
	CbFailureThreshold int
	CbDurationMin      int
	CbWindowMin        int
	HeadersJSON        string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// LLMSetting 表示 LLM 全局设置。
type LLMSetting struct {
	ID          uint
	Key         string
	Value       string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
