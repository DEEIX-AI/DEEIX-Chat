package channel

// BatchDeleteRequest 批量删除请求。
type BatchDeleteRequest struct {
	IDs []uint `json:"ids" binding:"required,min=1,dive,gt=0"`
}

// CreateUpstreamRequest 创建上游请求。
type CreateUpstreamRequest struct {
	Name                 string `json:"name" binding:"required,min=2,max=128"`
	BaseURL              string `json:"baseURL" binding:"required,url,max=512"`
	Compatible           string `json:"compatible,omitempty" binding:"omitempty,oneof=openai anthropic google xai openrouter custom"`
	ProtocolDefaultsJSON string `json:"protocolDefaultsJSON,omitempty" binding:"max=10000"`
	APIKeys              string `json:"apiKeys" binding:"required,min=2,max=10000"`
	Status               string `json:"status,omitempty" binding:"omitempty,oneof=active inactive"`
	ConnectTimeoutMS     int    `json:"connectTimeoutMS,omitempty"`
	ReadTimeoutMS        int    `json:"readTimeoutMS,omitempty"`
	StreamIdleTimeoutMS  int    `json:"streamIdleTimeoutMS,omitempty"`
	CbFailureThreshold   int    `json:"cbFailureThreshold,omitempty"`
	CbModelThreshold     int    `json:"cbModelThreshold,omitempty"`
	CbThresholdLogic     string `json:"cbThresholdLogic,omitempty" binding:"omitempty,oneof=or and"`
	CbDurationMin        int    `json:"cbDurationMin,omitempty"`
	CbWindowMin          int    `json:"cbWindowMin,omitempty"`
	HeadersJSON          string `json:"headersJSON,omitempty" binding:"max=10000"`
}

// UpdateUpstreamRequest 更新上游请求。
type UpdateUpstreamRequest struct {
	Name                 *string  `json:"name,omitempty" binding:"omitempty,min=2,max=128"`
	BaseURL              *string  `json:"baseURL,omitempty" binding:"omitempty,url,max=512"`
	Compatible           *string  `json:"compatible,omitempty" binding:"omitempty,oneof=openai anthropic google xai openrouter custom"`
	ProtocolDefaultsJSON *string  `json:"protocolDefaultsJSON,omitempty" binding:"omitempty,max=10000"`
	APIKeys              *string  `json:"apiKeys,omitempty" binding:"omitempty,min=2,max=10000"`
	AddAPIKeys           *string  `json:"addAPIKeys,omitempty" binding:"omitempty,min=2,max=10000"`
	DeleteAPIKeyIDs      []string `json:"deleteAPIKeyIDs,omitempty" binding:"omitempty,dive,min=8,max=128"`
	Status               *string  `json:"status,omitempty" binding:"omitempty,oneof=active inactive"`
	ConnectTimeoutMS     *int     `json:"connectTimeoutMS,omitempty"`
	ReadTimeoutMS        *int     `json:"readTimeoutMS,omitempty"`
	StreamIdleTimeoutMS  *int     `json:"streamIdleTimeoutMS,omitempty"`
	CbFailureThreshold   *int     `json:"cbFailureThreshold,omitempty"`
	CbModelThreshold     *int     `json:"cbModelThreshold,omitempty"`
	CbThresholdLogic     *string  `json:"cbThresholdLogic,omitempty" binding:"omitempty,oneof=or and"`
	CbDurationMin        *int     `json:"cbDurationMin,omitempty"`
	CbWindowMin          *int     `json:"cbWindowMin,omitempty"`
	HeadersJSON          *string  `json:"headersJSON,omitempty" binding:"omitempty,max=10000"`
}

// CreateModelRequest 创建模型请求。
type CreateModelRequest struct {
	PlatformModelName  string `json:"platformModelName" binding:"required,min=2,max=128"`
	Vendor             string `json:"vendor,omitempty" binding:"omitempty,max=64"`
	KindsJSON          string `json:"kindsJSON,omitempty" binding:"omitempty,max=1000"`
	Icon               string `json:"icon,omitempty" binding:"max=128"`
	CapabilitiesJSON   string `json:"capabilitiesJSON,omitempty" binding:"max=10000"`
	SystemPrompt       string `json:"systemPrompt,omitempty" binding:"max=20000"`
	AccessScope        string `json:"accessScope,omitempty" binding:"omitempty,oneof=public internal"`
	Status             string `json:"status,omitempty" binding:"omitempty,oneof=active inactive"`
	Description        string `json:"description,omitempty" binding:"max=10000"`
	CbPolicyMode       string `json:"cbPolicyMode,omitempty" binding:"omitempty,oneof=default enforced"`
	CbFailureThreshold int    `json:"cbFailureThreshold,omitempty" binding:"gte=0"`
	CbDurationMin      int    `json:"cbDurationMin,omitempty" binding:"gte=0"`
	CbWindowMin        int    `json:"cbWindowMin,omitempty" binding:"gte=0"`
}

// UpdateModelRequest 更新模型请求。
type UpdateModelRequest struct {
	PlatformModelName  *string `json:"platformModelName,omitempty" binding:"omitempty,min=2,max=128"`
	Vendor             *string `json:"vendor,omitempty" binding:"omitempty,max=64"`
	KindsJSON          *string `json:"kindsJSON,omitempty" binding:"omitempty,max=1000"`
	Icon               *string `json:"icon,omitempty" binding:"omitempty,max=128"`
	CapabilitiesJSON   *string `json:"capabilitiesJSON,omitempty" binding:"omitempty,max=10000"`
	SystemPrompt       *string `json:"systemPrompt,omitempty" binding:"omitempty,max=20000"`
	AccessScope        *string `json:"accessScope,omitempty" binding:"omitempty,oneof=public internal"`
	Status             *string `json:"status,omitempty" binding:"omitempty,oneof=active inactive"`
	Description        *string `json:"description,omitempty" binding:"omitempty,max=10000"`
	CbPolicyMode       *string `json:"cbPolicyMode,omitempty" binding:"omitempty,oneof=default enforced"`
	CbFailureThreshold *int    `json:"cbFailureThreshold,omitempty" binding:"omitempty,gte=0"`
	CbDurationMin      *int    `json:"cbDurationMin,omitempty" binding:"omitempty,gte=0"`
	CbWindowMin        *int    `json:"cbWindowMin,omitempty" binding:"omitempty,gte=0"`
}

// ReorderModelsRequest 调整模型展示顺序请求。
type ReorderModelsRequest struct {
	ModelIDs []uint `json:"modelIDs" binding:"required,min=1,dive,gt=0"`
}

// UpsertUpstreamModelRequest 上游模型路由绑定请求。
type UpsertUpstreamModelRequest struct {
	RouteID            uint   `json:"routeID,omitempty"`
	PlatformModelName  string `json:"platformModelName" binding:"required,min=2,max=128"`
	UpstreamModelName  string `json:"upstreamModelName" binding:"required,min=1,max=128"`
	Protocol           string `json:"protocol,omitempty" binding:"omitempty,max=64"`
	KindsJSON          string `json:"kindsJSON,omitempty" binding:"omitempty,max=1000"`
	Status             string `json:"status,omitempty" binding:"omitempty,oneof=active inactive"`
	Priority           int    `json:"priority,omitempty"`
	Weight             int    `json:"weight,omitempty"`
	Source             string `json:"source,omitempty" binding:"omitempty,max=64"`
	CbFailureThreshold int    `json:"cbFailureThreshold,omitempty"`
	CbDurationMin      int    `json:"cbDurationMin,omitempty"`
	CbWindowMin        int    `json:"cbWindowMin,omitempty"`
	HeadersJSON        string `json:"headersJSON,omitempty" binding:"max=10000"`
}

// UpdateModelUpstreamSourceRequest 更新模型上游来源请求。
//
// 任意字段省略则不变更。
type UpdateModelUpstreamSourceRequest struct {
	Protocol           *string `json:"protocol,omitempty" binding:"omitempty,max=64"`
	Status             *string `json:"status,omitempty" binding:"omitempty,oneof=active inactive"`
	Priority           *int    `json:"priority,omitempty"`
	Weight             *int    `json:"weight,omitempty"`
	CbFailureThreshold *int    `json:"cbFailureThreshold,omitempty" binding:"omitempty,gte=0"`
	CbDurationMin      *int    `json:"cbDurationMin,omitempty" binding:"omitempty,gte=0"`
	CbWindowMin        *int    `json:"cbWindowMin,omitempty" binding:"omitempty,gte=0"`
}

// BindModelUpstreamSourceRequest 模型侧新增上游来源绑定请求。
type BindModelUpstreamSourceRequest struct {
	UpstreamID         uint   `json:"upstreamID" binding:"required,gt=0"`
	UpstreamModelID    uint   `json:"upstreamModelID" binding:"required,gt=0"`
	Protocol           string `json:"protocol,omitempty" binding:"omitempty,max=64"`
	Status             string `json:"status,omitempty" binding:"omitempty,oneof=active inactive"`
	Priority           int    `json:"priority,omitempty"`
	Weight             int    `json:"weight,omitempty"`
	CbFailureThreshold int    `json:"cbFailureThreshold,omitempty" binding:"gte=0"`
	CbDurationMin      int    `json:"cbDurationMin,omitempty" binding:"gte=0"`
	CbWindowMin        int    `json:"cbWindowMin,omitempty" binding:"gte=0"`
}

// ImportUpstreamModelsRequest 批量导入上游模型请求。
type ImportUpstreamModelsRequest struct {
	Items              []ImportUpstreamModelItemRequest `json:"items" binding:"required,min=1,dive"`
	PermissionGroupIDs []uint                           `json:"permissionGroupIDs,omitempty"`
}

// ImportUpstreamModelItemRequest 单个导入项请求。
type ImportUpstreamModelItemRequest struct {
	PlatformModelName string   `json:"platformModelName" binding:"required,min=2,max=128"`
	UpstreamModelName string   `json:"upstreamModelName" binding:"required,min=1,max=128"`
	Protocol          string   `json:"protocol,omitempty" binding:"omitempty,max=64"`
	Protocols         []string `json:"protocols,omitempty" binding:"omitempty,dive,max=64"`
	KindsJSON         string   `json:"kindsJSON,omitempty" binding:"omitempty,max=1000"`
	Status            string   `json:"status,omitempty" binding:"omitempty,oneof=active inactive"`
	Priority          int      `json:"priority,omitempty"`
}

// ModelProbeRequest 后台模型连通性测试请求。
type ModelProbeRequest struct {
	TaskType string `json:"taskType,omitempty" binding:"omitempty,oneof=chat image_generation image_edit video_generation"`
}
