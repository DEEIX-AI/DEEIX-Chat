package channel

import (
	appbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/billing"
)

const (
	BatchDeleteStatusDeleted  = "deleted"
	BatchDeleteStatusNotFound = "not_found"
	BatchDeleteStatusFailed   = "failed"

	ImportUpstreamModelStatusCreated  = "created"
	ImportUpstreamModelStatusExisting = "existing"
	ImportUpstreamModelStatusFailed   = "failed"

	ModelVendorDeleteReasonBuiltIn          = "built_in"
	ModelVendorDeleteReasonReferencedModels = "referenced_models"
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
	Total      int
	Items      []UpstreamRemoteModelView
	SnapshotID string
	SyncPlan   UpstreamModelSyncPlanView
}

// UpstreamModelSyncPlanView 描述应用指定远端快照时将发生的目录变化。
type UpstreamModelSyncPlanView struct {
	AddedModels       []string
	UpdatedModels     []string
	ReactivatedModels []string
	InactivatedModels []string
	UnchangedModels   []string
	ProtectedModels   []string
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
	SnapshotID              string
	TotalUpstream           int
	CreatedUpstreamModels   int
	UpdatedUpstreamModels   int
	UnchangedUpstreamModels int
	ProtectedUpstreamModels int
	// ExistingUpstreamModels 保留旧版响应语义，表示远端目录中未新增的模型总数。
	ExistingUpstreamModels int
	// SkippedUpstreamModels 保留旧版响应字段；原子对账失败时整体回滚，因此始终为 0。
	SkippedUpstreamModels int
	InactivatedModels     int64
	ReactivatedModels     int
	SyncedModels          []UpstreamSyncModelView
}

// UpstreamSyncModelView 单个同步结果（内部传输，不携带序列化标记）。
type UpstreamSyncModelView struct {
	UpstreamModelName string
	BindingCode       string
	SuggestedProtocol string
	KindsJSON         string
	Status            string
	Created           bool
	Updated           bool
	Reactivated       bool
	Protected         bool
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
	PlatformModelID   uint
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
	ID                 uint
	PlatformModelName  string
	Vendor             string
	VendorName         string
	VendorIcon         string
	DisplayGroupID     *uint
	DisplayGroupName   string
	DisplayGroupIcon   string
	KindsJSON          string
	Icon               string
	CapabilitiesJSON   string
	ContextWindow      int
	SystemPrompt       string
	AccessScope        string
	Status             string
	Description        string
	CbPolicyMode       string
	CbFailureThreshold int
	CbDurationMin      int
	CbWindowMin        int
	SortOrder          int
	SourceCount        int64
	ActiveSourceCount  int64
	ProtocolsJSON      string
	UpstreamNamesJSON  string
	Pricing            *appbilling.PublicModelPricing
	CreatedAt          string
	UpdatedAt          string
}

// ModelVendorView 表示技术厂商目录展示数据。
type ModelVendorView struct {
	ID        uint
	Key       string
	Name      string
	Icon      string
	BuiltIn   bool
	SortOrder int
	CreatedAt string
	UpdatedAt string
}

// ModelVendorReferenceView 描述阻止删除厂商的平台模型引用。
type ModelVendorReferenceView struct {
	ID                uint
	PlatformModelName string
}

// ModelVendorDeleteBlockedError 携带厂商删除被拒绝的稳定原因和引用预览。
type ModelVendorDeleteBlockedError struct {
	Reason         string
	ReferenceCount int64
	Models         []ModelVendorReferenceView
}

func (e *ModelVendorDeleteBlockedError) Error() string {
	if e != nil && e.Reason == ModelVendorDeleteReasonBuiltIn {
		return ErrBuiltInModelVendorDelete.Error()
	}
	return ErrModelVendorInUse.Error()
}

func (e *ModelVendorDeleteBlockedError) Unwrap() error {
	if e != nil && e.Reason == ModelVendorDeleteReasonBuiltIn {
		return ErrBuiltInModelVendorDelete
	}
	return ErrModelVendorInUse
}

// ModelDisplayGroupView 表示自定义模型展示分组数据。
type ModelDisplayGroupView struct {
	ID        uint
	Name      string
	Icon      string
	SortOrder int
	CreatedAt string
	UpdatedAt string
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
	UpstreamStatus         string
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
	CbFailureThreshold     int
	CbDurationMin          int
	CbWindowMin            int
	HeadersJSON            string
	CircuitOpen            bool
	CircuitUntil           string
	CircuitScope           string
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
