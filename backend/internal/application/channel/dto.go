package channel

import appbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/billing"

const (
	BatchDeleteStatusDeleted  = "deleted"
	BatchDeleteStatusNotFound = "not_found"
	BatchDeleteStatusFailed   = "failed"

	ImportUpstreamModelStatusCreated  = "created"
	ImportUpstreamModelStatusExisting = "existing"
	ImportUpstreamModelStatusFailed   = "failed"
)

// BatchDeleteResultView 单个批量删除结果。
type BatchDeleteResultView struct {
	ID     uint
	Status string
	Error  string
}

// BatchDeleteData 批量删除结果数据。
type BatchDeleteData struct {
	Total         int
	SuccessCount  int
	NotFoundCount int
	FailedCount   int
	Results       []BatchDeleteResultView
}

// UpstreamRemoteModelsData 上游远程模型预览响应数据（内部传输，不携带序列化标记）。
type UpstreamRemoteModelsData struct {
	Total int
	Items []UpstreamRemoteModelView
}

// UpstreamRemoteModelView 上游远程模型预览项（内部传输，不携带序列化标记）。
type UpstreamRemoteModelView struct {
	UpstreamModelName          string
	SuggestedPlatformModelName string
	SuggestedKindsJSON         string
	SuggestedProtocol          string
	SuggestedProtocols         []string
	BindingCode                string
	BoundPlatformModels        []string
	UpstreamModelStatus        string
	AlreadySynced              bool
	AlreadyBound               bool
}

// SyncUpstreamModelsData 同步上游模型响应数据（内部传输，不携带序列化标记）。
type SyncUpstreamModelsData struct {
	TotalUpstream          int
	CreatedUpstreamModels  int
	ExistingUpstreamModels int
	SkippedUpstreamModels  int
	InactivatedModels      int64
	SyncedModels           []UpstreamSyncModelView
}

// UpstreamSyncModelView 单个同步结果（内部传输，不携带序列化标记）。
type UpstreamSyncModelView struct {
	UpstreamModelName string
	BindingCode       string
	SuggestedProtocol string
	KindsJSON         string
	Status            string
	Created           bool
}

// ImportUpstreamModelsData 批量导入上游模型响应数据（内部传输，不携带序列化标记）。
type ImportUpstreamModelsData struct {
	Total           int
	ImportedCount   int
	FailedCount     int
	CreatedRoutes   int
	ExistingRoutes  int
	CreatedPlatform int
	Results         []ImportUpstreamModelResultView
}

// ImportUpstreamModelResultView 单个导入结果（内部传输，不携带序列化标记）。
type ImportUpstreamModelResultView struct {
	UpstreamModelName string
	PlatformModelName string
	BindingCode       string
	Status            string
	CreatedRoute      bool
	CreatedRoutes     int
	ExistingRoutes    int
	Protocols         []string
	CreatedPlatform   bool
	Error             string
}

// UpstreamView 上游展示数据（内部传输，不携带序列化标记）。
type UpstreamView struct {
	ID                   uint
	Name                 string
	BaseURL              string
	Compatible           string
	ProtocolDefaultsJSON string
	APIKeysMasked        string
	APIKeyItems          []UpstreamAPIKeyView
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
	ModelsCount          int64
	ActiveModelsCount    int64
	CircuitOpen          bool
	CircuitUntil         string
	CreatedAt            string
	UpdatedAt            string
}

// UpstreamAPIKeyView 表示脱敏后的上游 API Key 展示项。
type UpstreamAPIKeyView struct {
	ID        string
	Index     int
	KeyMasked string
	Status    string
	Note      string
}

// ModelView 模型展示数据（内部传输，不携带序列化标记）。
type ModelView struct {
	ID                uint
	PlatformModelName string
	Vendor            string
	KindsJSON         string
	Icon              string
	CapabilitiesJSON  string
	SystemPrompt      string
	AccessScope       string
	Status            string
	Description       string
	SortOrder         int
	SourceCount       int64
	ActiveSourceCount int64
	ProtocolsJSON     string
	Pricing           *appbilling.PublicModelPricing
	CreatedAt         string
	UpdatedAt         string
}

// UpstreamModelView 上游模型路由绑定展示数据（内部传输，不携带序列化标记）。
type UpstreamModelView struct {
	ID                     uint
	RouteID                uint
	UpstreamID             uint
	BindingCode            string
	PlatformModelID        uint
	PlatformModelName      string
	ModelVendor            string
	ModelKindsJSON         string
	ModelIcon              string
	UpstreamModelName      string
	UpstreamModelVendor    string
	UpstreamModelIcon      string
	UpstreamModelKindsJSON string
	SuggestedProtocol      string
	Protocol               string
	UpstreamModelStatus    string
	RouteStatus            string
	Priority               int
	Weight                 int
	Source                 string
	CbFailureThreshold     int
	CbDurationMin          int
	CbWindowMin            int
	HeadersJSON            string
	CircuitOpen            bool
	CircuitUntil           string
	CreatedAt              string
	UpdatedAt              string
}

// ModelUpstreamSourceView 模型上游来源展示数据（内部传输，不携带序列化标记）。
type ModelUpstreamSourceView struct {
	ID                     uint
	UpstreamID             uint
	UpstreamName           string
	BaseURL                string
	BindingCode            string
	UpstreamModelName      string
	UpstreamModelKindsJSON string
	UpstreamModelVendor    string
	UpstreamModelIcon      string
	SuggestedProtocol      string
	UpstreamModelStatus    string
	Protocol               string
	Status                 string
	Priority               int
	Weight                 int
	Source                 string
	HeadersJSON            string
	CircuitOpen            bool
	CircuitUntil           string
	CreatedAt              string
	UpdatedAt              string
}

// UpstreamHealthView 上游健康状态展示数据（内部传输，不携带序列化标记）。
type UpstreamHealthView struct {
	UpstreamID    uint
	UpstreamName  string
	Status        string
	FailureCount  int64
	CircuitOpen   bool
	CircuitUntil  string
	LastError     string
	LastFailureAt string
	LastSuccessAt string
}

// ModelProbeResult 模型连通性测试结果（内部传输，不携带序列化标记）。
type ModelProbeResult struct {
	Success            bool
	Status             string
	ErrorCode          string
	ErrorMessage       string
	LatencyMS          int64
	Protocol           string
	Endpoint           string
	PlatformModelID    uint
	PlatformModelName  string
	UpstreamID         uint
	UpstreamName       string
	UpstreamModelID    uint
	UpstreamModelName  string
	RouteID            uint
	BindingCode        string
	UpstreamStatusCode int
	Debug              *ModelProbeDebugView
}

// ModelProbeBatchResult 模型批量连通性测试结果。
type ModelProbeBatchResult struct {
	TotalCount       int
	SuccessCount     int
	FailedCount      int
	UnsupportedCount int
	Results          []ModelProbeResult
}

// ModelProbeDebugView 模型测试调试快照（内部传输，不携带序列化标记）。
type ModelProbeDebugView struct {
	Request  ModelProbeDebugRequestView
	Response ModelProbeDebugResponseView
}

// ModelProbeDebugRequestView 模型测试请求调试信息。
type ModelProbeDebugRequestView struct {
	Method  string
	Path    string
	Headers map[string]string
	Body    string
}

// ModelProbeDebugResponseView 模型测试响应调试信息。
type ModelProbeDebugResponseView struct {
	StatusCode int
	Headers    map[string]string
	Body       string
}
