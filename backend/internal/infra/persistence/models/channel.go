package model

import "time"

// LLMUpstream 存储上游配置。
type LLMUpstream struct {
	ControlPlaneModel
	Name                 string `gorm:"size:128;not null;default:'';comment:上游名称"`
	BaseURL              string `gorm:"size:512;not null;default:'';comment:上游服务地址"`
	Compatible           string `gorm:"size:32;not null;default:'openai';index:idx_llm_upstreams_compatible;comment:上游API兼容风格"`
	ProtocolDefaultsJSON string `gorm:"type:text;not null;default:'{}';comment:按模型类型配置的默认协议JSON"`
	Status               string `gorm:"size:32;not null;default:'active';index:idx_llm_upstreams_status;comment:上游状态"`
	ConnectTimeoutMS     int    `gorm:"not null;default:10000;comment:TCP建连超时毫秒"`
	ReadTimeoutMS        int    `gorm:"not null;default:120000;comment:非流式整体超时/流式首字节超时毫秒"`
	StreamIdleTimeoutMS  int    `gorm:"not null;default:60000;comment:流式chunk间隔超时毫秒"`
	APIKeysEnc           string `gorm:"type:text;comment:AES加密后的API密钥配置"`
	CbFailureThreshold   int    `gorm:"not null;default:0;comment:上游级熔断失败次数阈值"`
	CbModelThreshold     int    `gorm:"not null;default:0;comment:上游级熔断模型数阈值"`
	CbThresholdLogic     string `gorm:"size:8;not null;default:'or';comment:上游级熔断判定逻辑"`
	CbDurationMin        int    `gorm:"not null;default:0;comment:上游级熔断持续时间分钟"`
	CbWindowMin          int    `gorm:"not null;default:0;comment:上游级熔断滑动窗口分钟"`
	HeadersJSON          string `gorm:"type:text;not null;default:'';comment:附加请求头JSON"`
}

// TableName 指定表名。
func (LLMUpstream) TableName() string {
	return "llm_upstreams"
}

// LLMPlatformModel 存储平台对用户提供的模型。
//
// Name 是用户请求、公开模型列表、会话默认模型和计费配置使用的唯一模型名。
type LLMPlatformModel struct {
	ControlPlaneModel
	Name             string `gorm:"size:128;not null;default:'';uniqueIndex:idx_llm_platform_models_name;comment:平台模型名"`
	Vendor           string `gorm:"size:64;not null;default:'';index:idx_llm_platform_models_vendor;comment:平台展示厂商"`
	KindsJSON        string `gorm:"type:text;not null;default:'[\"chat\"]';comment:模型类型JSON数组"`
	CapabilitiesJSON string `gorm:"type:text;not null;default:'{}';comment:平台能力配置JSON"`
	SystemPrompt     string `gorm:"type:text;not null;default:'';comment:模型级系统提示词"`
	AccessScope      string `gorm:"size:32;not null;default:'public';index:idx_llm_platform_models_access_scope;comment:模型使用范围: public用户可用 internal仅内部任务"`
	Icon             string `gorm:"size:64;comment:模型图标标识"`
	Description      string `gorm:"type:text;comment:模型说明"`
	Status           string `gorm:"size:32;not null;default:'active';index:idx_llm_platform_models_status;comment:平台模型状态"`
	SortOrder        int    `gorm:"not null;default:0;index:idx_llm_platform_models_sort_order;comment:排序权重"`
}

func (LLMPlatformModel) TableName() string {
	return "llm_platform_models"
}

// LLMUpstreamModel 存储上游真实模型清单。
//
// BindingCode 是每个上游真实模型的内部链路编码；UpstreamModelName 是实际传给上游 API 的 model。
type LLMUpstreamModel struct {
	ControlPlaneModel
	UpstreamID        uint       `gorm:"not null;default:0;uniqueIndex:idx_llm_upstream_models_upstream_name;index:idx_llm_upstream_models_upstream_id;comment:上游ID"`
	BindingCode       string     `gorm:"size:64;not null;default:'';uniqueIndex:idx_llm_upstream_models_binding_code;comment:上游模型内部链路编码"`
	UpstreamModelName string     `gorm:"size:256;not null;default:'';uniqueIndex:idx_llm_upstream_models_upstream_name;comment:上游真实模型名"`
	Vendor            string     `gorm:"size:64;not null;default:'';index:idx_llm_upstream_models_vendor;comment:上游真实模型厂商"`
	Icon              string     `gorm:"size:64;not null;default:'';comment:上游真实模型图标标识"`
	SuggestedProtocol string     `gorm:"size:64;not null;default:'';index:idx_llm_upstream_models_suggested_protocol;comment:同步推断协议"`
	KindsJSON         string     `gorm:"type:text;not null;default:'[\"chat\"]';comment:模型类型JSON数组"`
	Status            string     `gorm:"size:32;not null;default:'active';index:idx_llm_upstream_models_status;comment:上游模型状态"`
	Source            string     `gorm:"size:16;not null;default:'sync';index:idx_llm_upstream_models_source;comment:来源"`
	LastSyncedAt      *time.Time `gorm:"comment:最近同步时间"`
	RawJSON           string     `gorm:"type:text;not null;default:'{}';comment:上游原始模型元数据"`
}

func (LLMUpstreamModel) TableName() string {
	return "llm_upstream_models"
}

// LLMPlatformModelRoute 存储平台模型到上游真实模型的路由绑定。
type LLMPlatformModelRoute struct {
	ControlPlaneModel
	PlatformModelID    uint   `gorm:"not null;default:0;index:idx_llm_model_routes_model;uniqueIndex:idx_llm_model_routes_unique;comment:平台模型ID"`
	UpstreamModelID    uint   `gorm:"not null;default:0;index:idx_llm_model_routes_upstream_model;uniqueIndex:idx_llm_model_routes_unique;comment:上游模型ID"`
	Protocol           string `gorm:"size:64;not null;index:idx_llm_model_routes_protocol;uniqueIndex:idx_llm_model_routes_unique;comment:最终适配器协议"`
	Status             string `gorm:"size:32;not null;default:'active';index:idx_llm_model_routes_status;comment:路由状态"`
	Priority           int    `gorm:"not null;default:1;index:idx_llm_model_routes_priority;comment:路由优先级"`
	Weight             int    `gorm:"not null;default:1;comment:负载均衡权重"`
	Source             string `gorm:"size:16;not null;default:'manual';index:idx_llm_model_routes_source;comment:绑定来源"`
	CbFailureThreshold int    `gorm:"not null;default:0;comment:模型级熔断失败次数阈值"`
	CbDurationMin      int    `gorm:"not null;default:0;comment:模型级熔断持续时间分钟"`
	CbWindowMin        int    `gorm:"not null;default:0;comment:模型级熔断滑动窗口分钟"`
	HeadersJSON        string `gorm:"type:text;not null;default:'';comment:路由覆盖请求头JSON"`
}

func (LLMPlatformModelRoute) TableName() string {
	return "llm_model_routes"
}
