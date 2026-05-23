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

// ModelView 模型展示数据（内部传输，不携带序列化标记）。
type ModelView struct {
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
