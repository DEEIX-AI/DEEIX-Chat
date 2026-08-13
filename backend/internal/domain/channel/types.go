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
	ID                 uint
	PlatformModelName  string
	Vendor             string
	DisplayGroupID     *uint
	KindsJSON          string
	Icon               string
	CapabilitiesJSON   string
	SystemPrompt       string
	AccessScope        string
	Status             string
	Description        string
	CbPolicyMode       string
	CbFailureThreshold int
	CbDurationMin      int
	CbWindowMin        int
	SortOrder          int
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// ModelVendor 表示平台模型的技术厂商目录项。
// Key 是路由、权限与计费使用的稳定标识；Name 和 Icon 仅用于展示。
type ModelVendor struct {
	ID        uint
	Key       string
	Name      string
	Icon      string
	BuiltIn   bool
	SortOrder int
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ModelDisplayGroup 表示管理员定义的可选模型展示分组。
// 模型未绑定分组时继续按技术厂商展示。
type ModelDisplayGroup struct {
	ID        uint
	Name      string
	Icon      string
	SortOrder int
	CreatedAt time.Time
	UpdatedAt time.Time
}

// BuiltInModelVendors 返回内置技术厂商目录。
// 返回新切片，避免调用方修改全局共享状态。
func BuiltInModelVendors() []ModelVendor {
	return []ModelVendor{
		{Key: "openai", Name: "OpenAI", Icon: "openai", BuiltIn: true, SortOrder: 100},
		{Key: "anthropic", Name: "Anthropic", Icon: "anthropic", BuiltIn: true, SortOrder: 200},
		{Key: "google", Name: "Google", Icon: "google", BuiltIn: true, SortOrder: 300},
		{Key: "meta", Name: "Meta", Icon: "meta", BuiltIn: true, SortOrder: 400},
		{Key: "microsoft", Name: "Microsoft", Icon: "microsoft", BuiltIn: true, SortOrder: 500},
		{Key: "amazon", Name: "Amazon", Icon: "aws", BuiltIn: true, SortOrder: 600},
		{Key: "nvidia", Name: "NVIDIA", Icon: "nvidia", BuiltIn: true, SortOrder: 700},
		{Key: "deepseek", Name: "DeepSeek", Icon: "deepseek", BuiltIn: true, SortOrder: 800},
		{Key: "moonshot", Name: "MoonShot", Icon: "moonshot", BuiltIn: true, SortOrder: 900},
		{Key: "zhipu", Name: "ZhiPu", Icon: "zhipu", BuiltIn: true, SortOrder: 1000},
		{Key: "minimax", Name: "MiniMax", Icon: "minimax", BuiltIn: true, SortOrder: 1100},
		{Key: "bytedance", Name: "ByteDance", Icon: "bytedance", BuiltIn: true, SortOrder: 1200},
		{Key: "tencent", Name: "Tencent", Icon: "tencent", BuiltIn: true, SortOrder: 1300},
		{Key: "longcat", Name: "LongCat", Icon: "longcat", BuiltIn: true, SortOrder: 1400},
		{Key: "mistral", Name: "Mistral", Icon: "mistral", BuiltIn: true, SortOrder: 1500},
		{Key: "alibaba", Name: "Alibaba", Icon: "alibaba", BuiltIn: true, SortOrder: 1600},
		{Key: "xai", Name: "xAI", Icon: "xai", BuiltIn: true, SortOrder: 1700},
		{Key: "xiaomi", Name: "Xiaomi", Icon: "xiaomimimo", BuiltIn: true, SortOrder: 1800},
		{Key: "iflytek", Name: "iFlytek", Icon: "iflytekcloud", BuiltIn: true, SortOrder: 1900},
		{Key: "stepfun", Name: "StepFun", Icon: "stepfun", BuiltIn: true, SortOrder: 2000},
		{Key: "baichuan", Name: "Baichuan", Icon: "baichuan", BuiltIn: true, SortOrder: 2100},
		{Key: "baidu", Name: "Baidu", Icon: "baidu", BuiltIn: true, SortOrder: 2200},
		{Key: "openrouter", Name: "OpenRouter", Icon: "openrouter", BuiltIn: true, SortOrder: 2300},
		{Key: "copilot", Name: "GitHub Copilot", Icon: "copilot", BuiltIn: true, SortOrder: 2400},
		{Key: "unknown", Name: "Unknown", Icon: "", BuiltIn: true, SortOrder: 2500},
	}
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
