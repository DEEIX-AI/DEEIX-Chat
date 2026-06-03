package channel

// BatchDeleteRequest 批量删除请求。
type BatchDeleteRequest struct {
	IDs []uint `json:"ids" binding:"required,min=1,dive,gt=0"`
}

// CreateUpstreamRequest 创建上游请求。
type CreateUpstreamRequest struct {
	Name                 string `json:"name" binding:"required,min=2,max=128"`
	BaseURL              string `json:"baseURL" binding:"required,url,max=512"`
	Compatible           string `json:"compatible" binding:"omitempty,oneof=openai anthropic google xai openrouter custom"`
	ProtocolDefaultsJSON string `json:"protocolDefaultsJSON" binding:"max=10000"`
	APIKeys              string `json:"apiKeys" binding:"required,min=2,max=10000"`
	Status               string `json:"status" binding:"omitempty,oneof=active inactive"`
	ConnectTimeoutMS     int    `json:"connectTimeoutMS"`
	ReadTimeoutMS        int    `json:"readTimeoutMS"`
	StreamIdleTimeoutMS  int    `json:"streamIdleTimeoutMS"`
	CbFailureThreshold   int    `json:"cbFailureThreshold"`
	CbModelThreshold     int    `json:"cbModelThreshold"`
	CbThresholdLogic     string `json:"cbThresholdLogic" binding:"omitempty,oneof=or and"`
	CbDurationMin        int    `json:"cbDurationMin"`
	CbWindowMin          int    `json:"cbWindowMin"`
	HeadersJSON          string `json:"headersJSON" binding:"max=10000"`
}

// UpdateUpstreamRequest 更新上游请求。
type UpdateUpstreamRequest struct {
	Name                 *string  `json:"name" binding:"omitempty,min=2,max=128"`
	BaseURL              *string  `json:"baseURL" binding:"omitempty,url,max=512"`
	Compatible           *string  `json:"compatible" binding:"omitempty,oneof=openai anthropic google xai openrouter custom"`
	ProtocolDefaultsJSON *string  `json:"protocolDefaultsJSON" binding:"omitempty,max=10000"`
	APIKeys              *string  `json:"apiKeys" binding:"omitempty,min=2,max=10000"`
	AddAPIKeys           *string  `json:"addAPIKeys" binding:"omitempty,min=2,max=10000"`
	DeleteAPIKeyIDs      []string `json:"deleteAPIKeyIDs" binding:"omitempty,dive,min=8,max=128"`
	Status               *string  `json:"status" binding:"omitempty,oneof=active inactive"`
	ConnectTimeoutMS     *int     `json:"connectTimeoutMS"`
	ReadTimeoutMS        *int     `json:"readTimeoutMS"`
	StreamIdleTimeoutMS  *int     `json:"streamIdleTimeoutMS"`
	CbFailureThreshold   *int     `json:"cbFailureThreshold"`
	CbModelThreshold     *int     `json:"cbModelThreshold"`
	CbThresholdLogic     *string  `json:"cbThresholdLogic" binding:"omitempty,oneof=or and"`
	CbDurationMin        *int     `json:"cbDurationMin"`
	CbWindowMin          *int     `json:"cbWindowMin"`
	HeadersJSON          *string  `json:"headersJSON" binding:"omitempty,max=10000"`
}

// CreateModelRequest 创建模型请求。
type CreateModelRequest struct {
	PlatformModelName string `json:"platformModelName" binding:"required,min=2,max=128"`
	Vendor            string `json:"vendor" binding:"omitempty,max=64"`
	KindsJSON         string `json:"kindsJSON" binding:"omitempty,max=1000"`
	Icon              string `json:"icon" binding:"max=128"`
	CapabilitiesJSON  string `json:"capabilitiesJSON" binding:"max=10000"`
	SystemPrompt      string `json:"systemPrompt" binding:"max=20000"`
	AccessScope       string `json:"accessScope" binding:"omitempty,oneof=public internal"`
	Status            string `json:"status" binding:"omitempty,oneof=active inactive"`
	Description       string `json:"description" binding:"max=10000"`
}

// UpdateModelRequest 更新模型请求。
type UpdateModelRequest struct {
	PlatformModelName *string `json:"platformModelName" binding:"omitempty,min=2,max=128"`
	Vendor            *string `json:"vendor" binding:"omitempty,max=64"`
	KindsJSON         *string `json:"kindsJSON" binding:"omitempty,max=1000"`
	Icon              *string `json:"icon" binding:"omitempty,max=128"`
	CapabilitiesJSON  *string `json:"capabilitiesJSON" binding:"omitempty,max=10000"`
	SystemPrompt      *string `json:"systemPrompt" binding:"omitempty,max=20000"`
	AccessScope       *string `json:"accessScope" binding:"omitempty,oneof=public internal"`
	Status            *string `json:"status" binding:"omitempty,oneof=active inactive"`
	Description       *string `json:"description" binding:"omitempty,max=10000"`
}

// ReorderModelsRequest 调整模型展示顺序请求。
type ReorderModelsRequest struct {
	ModelIDs []uint `json:"modelIDs" binding:"required,min=1,dive,gt=0"`
}

// UpsertUpstreamModelRequest 上游模型路由绑定请求。
type UpsertUpstreamModelRequest struct {
	RouteID            uint   `json:"routeID"`
	PlatformModelName  string `json:"platformModelName" binding:"required,min=2,max=128"`
	UpstreamModelName  string `json:"upstreamModelName" binding:"required,min=1,max=128"`
	Protocol           string `json:"protocol" binding:"omitempty,max=64"`
	KindsJSON          string `json:"kindsJSON" binding:"omitempty,max=1000"`
	Status             string `json:"status" binding:"omitempty,oneof=active inactive"`
	Priority           int    `json:"priority"`
	Weight             int    `json:"weight"`
	Source             string `json:"source" binding:"omitempty,max=64"`
	CbFailureThreshold int    `json:"cbFailureThreshold"`
	CbDurationMin      int    `json:"cbDurationMin"`
	CbWindowMin        int    `json:"cbWindowMin"`
	HeadersJSON        string `json:"headersJSON" binding:"max=10000"`
}

// UpdateModelUpstreamSourceRequest 更新模型上游来源请求。
//
// 任意字段省略则不变更。
type UpdateModelUpstreamSourceRequest struct {
	Protocol *string `json:"protocol" binding:"omitempty,max=64"`
	Status   *string `json:"status" binding:"omitempty,oneof=active inactive"`
	Priority *int    `json:"priority"`
	Weight   *int    `json:"weight"`
}

// BindModelUpstreamSourceRequest 模型侧新增上游来源绑定请求。
type BindModelUpstreamSourceRequest struct {
	UpstreamID      uint   `json:"upstreamID" binding:"required,gt=0"`
	UpstreamModelID uint   `json:"upstreamModelID" binding:"required,gt=0"`
	Protocol        string `json:"protocol" binding:"omitempty,max=64"`
	Status          string `json:"status" binding:"omitempty,oneof=active inactive"`
	Priority        int    `json:"priority"`
	Weight          int    `json:"weight"`
}

// ImportUpstreamModelsRequest 批量导入上游模型请求。
type ImportUpstreamModelsRequest struct {
	Items []ImportUpstreamModelItemRequest `json:"items" binding:"required,min=1,dive"`
}

// ImportUpstreamModelItemRequest 单个导入项请求。
type ImportUpstreamModelItemRequest struct {
	PlatformModelName string   `json:"platformModelName" binding:"required,min=2,max=128"`
	UpstreamModelName string   `json:"upstreamModelName" binding:"required,min=1,max=128"`
	Protocol          string   `json:"protocol" binding:"omitempty,max=64"`
	Protocols         []string `json:"protocols" binding:"omitempty,dive,max=64"`
	KindsJSON         string   `json:"kindsJSON" binding:"omitempty,max=1000"`
	Status            string   `json:"status" binding:"omitempty,oneof=active inactive"`
	Priority          int      `json:"priority"`
}

// ModelProbeRequest 后台模型连通性测试请求。
type ModelProbeRequest struct {
	TaskType string `json:"taskType" binding:"omitempty,oneof=chat image_generation image_edit"`
}
