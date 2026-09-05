package channel

// CreateUpstreamInput 定义创建上游入参。
type CreateUpstreamInput struct {
	Name                 string
	BaseURL              string
	Compatible           string
	ProtocolDefaultsJSON string
	APIKeys              string
	Status               string
	ConnectTimeoutMS     int
	ReadTimeoutMS        int
	StreamIdleTimeoutMS  int
	CbFailureThreshold   int
	CbModelThreshold     int
	CbThresholdLogic     string
	CbDurationMin        int
	CbWindowMin          int
	HeadersJSON          string
}

// UpdateUpstreamInput 定义更新上游入参。
type UpdateUpstreamInput struct {
	Name                 *string
	BaseURL              *string
	Compatible           *string
	ProtocolDefaultsJSON *string
	APIKeys              *string
	AddAPIKeys           *string
	DeleteAPIKeyIDs      []string
	Status               *string
	ConnectTimeoutMS     *int
	ReadTimeoutMS        *int
	StreamIdleTimeoutMS  *int
	CbFailureThreshold   *int
	CbModelThreshold     *int
	CbThresholdLogic     *string
	CbDurationMin        *int
	CbWindowMin          *int
	HeadersJSON          *string
}

// CreateModelInput 定义创建模型入参。
type CreateModelInput struct {
	PlatformModelName  string
	Vendor             string
	DisplayGroupID     uint
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
}

// UpdateModelInput 定义更新模型入参。
type UpdateModelInput struct {
	PlatformModelName  *string
	Vendor             *string
	DisplayGroupID     *uint
	KindsJSON          *string
	Icon               *string
	CapabilitiesJSON   *string
	SystemPrompt       *string
	AccessScope        *string
	Status             *string
	Description        *string
	CbPolicyMode       *string
	CbFailureThreshold *int
	CbDurationMin      *int
	CbWindowMin        *int
}

// SetModelProtocolsInput 定义平台模型全部来源的目标协议集合。
type SetModelProtocolsInput struct {
	Protocols []string
	KindsJSON string
}

// CreateModelVendorInput 定义创建技术厂商入参。
type CreateModelVendorInput struct {
	Key  string
	Name string
	Icon string
}

// UpdateModelVendorInput 定义更新技术厂商展示信息入参。
type UpdateModelVendorInput struct {
	Name *string
	Icon *string
}

// CreateModelDisplayGroupInput 定义创建模型展示分组入参。
type CreateModelDisplayGroupInput struct {
	Name     string
	Icon     string
	ModelIDs []uint
}

// UpdateModelDisplayGroupInput 定义更新模型展示分组入参。
type UpdateModelDisplayGroupInput struct {
	Name     *string
	Icon     *string
	ModelIDs *[]uint
}

// UpsertUpstreamModelInput 定义上游真实模型与平台路由保存入参。
type UpsertUpstreamModelInput struct {
	RouteIDs           []uint
	PlatformModelName  string
	UpstreamModelName  string
	Protocols          []string
	KindsJSON          string
	Status             *string
	Priority           *int
	Weight             *int
	Source             *string // 路由配置来源。
	CatalogSource      *string // 上游目录生命周期来源；为空时沿用 Source。
	CbFailureThreshold *int
	CbDurationMin      *int
	CbWindowMin        *int
	HeadersJSON        *string
}

// SyncUpstreamModelsInput 定义远端模型目录对账约束。
type SyncUpstreamModelsInput struct {
	ExpectedSnapshot string
	AllowEmpty       bool
}

// UpdateModelUpstreamSourceInput 定义更新模型来源入参。
type UpdateModelUpstreamSourceInput struct {
	Protocol           *string
	Status             *string
	Priority           *int
	Weight             *int
	CbFailureThreshold *int
	CbDurationMin      *int
	CbWindowMin        *int
}

// BindModelUpstreamSourceInput 定义模型侧新增上游来源绑定入参。
type BindModelUpstreamSourceInput struct {
	UpstreamID         uint
	UpstreamModelID    uint
	Protocol           string
	Status             string
	Priority           int
	Weight             int
	CbFailureThreshold int
	CbDurationMin      int
	CbWindowMin        int
}

// ImportUpstreamModelsInput 定义批量导入上游模型入参。
type ImportUpstreamModelsInput struct {
	Items              []ImportUpstreamModelItemInput
	PermissionGroupIDs []uint
}

// ImportUpstreamModelItemInput 定义单个导入项入参。
type ImportUpstreamModelItemInput struct {
	PlatformModelName string
	UpstreamModelName string
	Protocol          string
	Protocols         []string
	KindsJSON         string
	Status            string
	Priority          int
}

// ModelProbeInput 定义后台模型连通性测试入参。
type ModelProbeInput struct {
	TaskType string
}
