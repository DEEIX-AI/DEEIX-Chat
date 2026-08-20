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
	Name               string `gorm:"size:128;not null;default:'';uniqueIndex:idx_llm_platform_models_name;comment:平台模型名"`
	Vendor             string `gorm:"size:64;not null;default:'';index:idx_llm_platform_models_vendor;comment:平台模型技术厂商标识"`
	DisplayGroupID     *uint  `gorm:"index:idx_llm_platform_models_display_group;comment:可选展示分组ID，为空时按技术厂商展示"`
	KindsJSON          string `gorm:"type:text;not null;default:'[\"chat\"]';comment:模型类型JSON数组"`
	CapabilitiesJSON   string `gorm:"type:text;not null;default:'{}';comment:平台能力配置JSON"`
	SystemPrompt       string `gorm:"type:text;not null;default:'';comment:模型级系统提示词"`
	AccessScope        string `gorm:"size:32;not null;default:'public';index:idx_llm_platform_models_access_scope;comment:模型使用范围: public用户可用 internal仅内部任务"`
	Icon               string `gorm:"size:2048;index:idx_llm_platform_models_asset_icon,where:icon LIKE 'asset:%';comment:模型图标标识、资产引用或图片 URL"`
	Description        string `gorm:"type:text;comment:模型说明"`
	CbPolicyMode       string `gorm:"size:16;not null;default:'default';comment:具体模型熔断策略模式: default默认配置 enforced统一覆盖"`
	CbFailureThreshold int    `gorm:"not null;default:0;comment:具体模型默认熔断失败次数阈值"`
	CbDurationMin      int    `gorm:"not null;default:0;comment:具体模型默认熔断持续时间分钟"`
	CbWindowMin        int    `gorm:"not null;default:0;comment:具体模型默认熔断滑动窗口分钟"`
	Status             string `gorm:"size:32;not null;default:'active';index:idx_llm_platform_models_status;comment:平台模型状态"`
	SortOrder          int    `gorm:"not null;default:0;index:idx_llm_platform_models_sort_order;comment:排序权重"`
}

func (LLMPlatformModel) TableName() string {
	return "llm_platform_models"
}

// LLMModelVendor 存储平台模型技术厂商目录。
type LLMModelVendor struct {
	ControlPlaneModel
	Key       string `gorm:"size:64;not null;uniqueIndex:idx_llm_model_vendors_key;comment:稳定技术厂商标识"`
	Name      string `gorm:"size:64;not null;index:idx_llm_model_vendors_name;comment:厂商展示名称"`
	Icon      string `gorm:"size:2048;not null;default:'';index:idx_llm_model_vendors_asset_icon,where:icon LIKE 'asset:%';comment:厂商图标标识或图片 URL"`
	BuiltIn   bool   `gorm:"not null;default:false;comment:是否内置厂商"`
	SortOrder int    `gorm:"not null;default:0;index:idx_llm_model_vendors_sort_order;comment:厂商展示顺序"`
}

// TableName 指定技术厂商目录表名。
func (LLMModelVendor) TableName() string {
	return "llm_model_vendors"
}

// LLMModelDisplayGroup 存储管理员定义的模型展示分组。
type LLMModelDisplayGroup struct {
	ControlPlaneModel
	Name      string `gorm:"size:64;not null;uniqueIndex:idx_llm_model_display_groups_name;comment:展示分组名称"`
	Icon      string `gorm:"size:2048;not null;default:'';index:idx_llm_model_display_groups_asset_icon,where:icon LIKE 'asset:%';comment:展示分组图标标识或图片 URL"`
	SortOrder int    `gorm:"not null;default:0;index:idx_llm_model_display_groups_sort_order;comment:展示分组顺序"`
}

// TableName 指定模型展示分组表名。
func (LLMModelDisplayGroup) TableName() string {
	return "llm_model_display_groups"
}

// LLMModelIconAsset 存储管理员上传的模型展示图标元数据。
// 图片内容由统一对象存储承载，业务表仅引用公开 ID。
type LLMModelIconAsset struct {
	ControlPlaneModel
	PublicID          string     `gorm:"size:64;not null;uniqueIndex:idx_llm_model_icon_assets_public_id;comment:公开资源ID"`
	SHA256            string     `gorm:"size:64;not null;uniqueIndex:idx_llm_model_icon_assets_sha256;comment:内容SHA256"`
	StoragePath       string     `gorm:"size:512;not null;comment:对象存储路径"`
	ContentType       string     `gorm:"size:64;not null;comment:后端探测媒体类型"`
	SizeBytes         int64      `gorm:"not null;comment:文件大小(Byte)"`
	Width             int        `gorm:"not null;comment:图片宽度"`
	Height            int        `gorm:"not null;comment:图片高度"`
	CreatedByUserID   uint       `gorm:"not null;default:0;index:idx_llm_model_icon_assets_created_by;comment:创建管理员ID"`
	ReadyAt           *time.Time `gorm:"index:idx_llm_model_icon_assets_ready_at;comment:对象内容写入并校验完成时间"`
	LeaseExpiresAt    time.Time  `gorm:"not null;default:CURRENT_TIMESTAMP;index:idx_llm_model_icon_assets_lease_expires_at;comment:上传或引用操作持有的清理保护租约截止时间"`
	UnreferencedAt    *time.Time `gorm:"index:idx_llm_model_icon_assets_unreferenced_at;comment:首次确认连续无引用的时间"`
	DeleteRequestedAt *time.Time `gorm:"index:idx_llm_model_icon_assets_delete_requested_at;comment:管理员从图标库移除的时间"`
	DeletingAt        *time.Time `gorm:"index:idx_llm_model_icon_assets_deleting_at;comment:进入可重试对象删除流程的时间"`
}

func (LLMModelIconAsset) TableName() string {
	return "llm_model_icon_assets"
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
