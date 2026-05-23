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
	PlatformModelName string
	Vendor            string
	KindsJSON         string
	Icon              string
	CapabilitiesJSON  string
	SystemPrompt      string
	Status            string
	Description       string
}

// UpdateModelInput 定义更新模型入参。
type UpdateModelInput struct {
	PlatformModelName *string
	Vendor            *string
	KindsJSON         *string
	Icon              *string
	CapabilitiesJSON  *string
	SystemPrompt      *string
	Status            *string
	Description       *string
}

// UpsertUpstreamModelInput 定义上游真实模型与平台路由保存入参。
type UpsertUpstreamModelInput struct {
	RouteID            uint
	PlatformModelName  string
	UpstreamModelName  string
	Protocol           string
	KindsJSON          string
	Status             string
	Priority           int
	Weight             int
	Source             string
	CbFailureThreshold int
	CbDurationMin      int
	CbWindowMin        int
	HeadersJSON        string
}

// UpdateModelUpstreamSourceInput 定义更新模型来源入参。
type UpdateModelUpstreamSourceInput struct {
	Protocol *string
	Status   *string
	Priority *int
	Weight   *int
}

// ImportUpstreamModelsInput 定义批量导入上游模型入参。
type ImportUpstreamModelsInput struct {
	Items []ImportUpstreamModelItemInput
}

// ImportUpstreamModelItemInput 定义单个导入项入参。
type ImportUpstreamModelItemInput struct {
	PlatformModelName string
	UpstreamModelName string
	Protocol          string
	KindsJSON         string
	Status            string
	Priority          int
}
