package channel

import (
	"strings"
	"time"

	appbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/billing"
	appchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
	apppersonalprovider "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/personalprovider"
	domainchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/channel"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/security"
)

// UpstreamResponse 上游响应 DTO。
type UpstreamResponse struct {
	ID                   uint                     `json:"id"`
	Name                 string                   `json:"name"`
	BaseURL              string                   `json:"baseURL"`
	Compatible           string                   `json:"compatible"`
	ProtocolDefaultsJSON string                   `json:"protocolDefaultsJSON"`
	APIKeysMasked        string                   `json:"apiKeysMasked"`
	APIKeyItems          []UpstreamAPIKeyResponse `json:"apiKeyItems"`
	Status               string                   `json:"status"`
	ConnectTimeoutMS     int                      `json:"connectTimeoutMS"`
	ReadTimeoutMS        int                      `json:"readTimeoutMS"`
	StreamIdleTimeoutMS  int                      `json:"streamIdleTimeoutMS"`
	CbFailureThreshold   int                      `json:"cbFailureThreshold"`
	CbModelThreshold     int                      `json:"cbModelThreshold"`
	CbThresholdLogic     string                   `json:"cbThresholdLogic"`
	CbDurationMin        int                      `json:"cbDurationMin"`
	CbWindowMin          int                      `json:"cbWindowMin"`
	HeadersJSON          string                   `json:"headersJSON"`
	ModelsCount          int64                    `json:"modelsCount"`
	ActiveModelsCount    int64                    `json:"activeModelsCount"`
	CircuitOpen          bool                     `json:"circuitOpen"`
	CircuitUntil         string                   `json:"circuitUntil"`
	CreatedAt            string                   `json:"createdAt"`
	UpdatedAt            string                   `json:"updatedAt"`
}

// UpstreamAPIKeyResponse 上游脱敏 API Key 展示项。
type UpstreamAPIKeyResponse struct {
	ID        string `json:"id"`
	Index     int    `json:"index"`
	KeyMasked string `json:"keyMasked"`
	Status    string `json:"status"`
	Note      string `json:"note"`
}

func toUpstreamResponse(v appchannel.UpstreamView) UpstreamResponse {
	return UpstreamResponse{
		ID:                   v.ID,
		Name:                 v.Name,
		BaseURL:              v.BaseURL,
		Compatible:           v.Compatible,
		ProtocolDefaultsJSON: v.ProtocolDefaultsJSON,
		APIKeysMasked:        v.APIKeysMasked,
		APIKeyItems:          toUpstreamAPIKeyResponses(v.APIKeyItems),
		Status:               v.Status,
		ConnectTimeoutMS:     v.ConnectTimeoutMS,
		ReadTimeoutMS:        v.ReadTimeoutMS,
		StreamIdleTimeoutMS:  v.StreamIdleTimeoutMS,
		CbFailureThreshold:   v.CbFailureThreshold,
		CbModelThreshold:     v.CbModelThreshold,
		CbThresholdLogic:     v.CbThresholdLogic,
		CbDurationMin:        v.CbDurationMin,
		CbWindowMin:          v.CbWindowMin,
		HeadersJSON:          security.RedactHeadersJSON(v.HeadersJSON),
		ModelsCount:          v.ModelsCount,
		ActiveModelsCount:    v.ActiveModelsCount,
		CircuitOpen:          v.CircuitOpen,
		CircuitUntil:         v.CircuitUntil,
		CreatedAt:            v.CreatedAt,
		UpdatedAt:            v.UpdatedAt,
	}
}

func toUpstreamAPIKeyResponses(items []appchannel.UpstreamAPIKeyView) []UpstreamAPIKeyResponse {
	results := make([]UpstreamAPIKeyResponse, 0, len(items))
	for _, item := range items {
		results = append(results, UpstreamAPIKeyResponse{
			ID:        item.ID,
			Index:     item.Index,
			KeyMasked: item.KeyMasked,
			Status:    item.Status,
			Note:      item.Note,
		})
	}
	return results
}

// ModelResponse 模型响应 DTO。
type ModelResponse struct {
	ID                 uint   `json:"id"`
	PlatformModelName  string `json:"platformModelName"`
	Vendor             string `json:"vendor"`
	VendorName         string `json:"vendorName"`
	VendorIcon         string `json:"vendorIcon"`
	DisplayGroupID     *uint  `json:"displayGroupID" extensions:"x-nullable,!x-omitempty"`
	DisplayGroupName   string `json:"displayGroupName"`
	DisplayGroupIcon   string `json:"displayGroupIcon"`
	KindsJSON          string `json:"kindsJSON"`
	Icon               string `json:"icon"`
	CapabilitiesJSON   string `json:"capabilitiesJSON"`
	ContextWindow      int    `json:"contextWindow"`
	SystemPrompt       string `json:"systemPrompt"`
	AccessScope        string `json:"accessScope"`
	Status             string `json:"status"`
	Description        string `json:"description"`
	CbPolicyMode       string `json:"cbPolicyMode"`
	CbFailureThreshold int    `json:"cbFailureThreshold"`
	CbDurationMin      int    `json:"cbDurationMin"`
	CbWindowMin        int    `json:"cbWindowMin"`
	SortOrder          int    `json:"sortOrder"`
	SourceCount        int64  `json:"sourceCount"`
	ActiveSourceCount  int64  `json:"activeSourceCount"`
	ProtocolsJSON      string `json:"protocolsJSON"`
	UpstreamNamesJSON  string `json:"upstreamNamesJSON"`
	CreatedAt          string `json:"createdAt"`
	UpdatedAt          string `json:"updatedAt"`
}

func toModelResponse(v appchannel.ModelView) ModelResponse {
	return ModelResponse{
		ID:                 v.ID,
		PlatformModelName:  v.PlatformModelName,
		Vendor:             v.Vendor,
		VendorName:         v.VendorName,
		VendorIcon:         v.VendorIcon,
		DisplayGroupID:     v.DisplayGroupID,
		DisplayGroupName:   v.DisplayGroupName,
		DisplayGroupIcon:   v.DisplayGroupIcon,
		KindsJSON:          v.KindsJSON,
		Icon:               v.Icon,
		CapabilitiesJSON:   v.CapabilitiesJSON,
		ContextWindow:      v.ContextWindow,
		SystemPrompt:       v.SystemPrompt,
		AccessScope:        v.AccessScope,
		Status:             v.Status,
		Description:        v.Description,
		CbPolicyMode:       v.CbPolicyMode,
		CbFailureThreshold: v.CbFailureThreshold,
		CbDurationMin:      v.CbDurationMin,
		CbWindowMin:        v.CbWindowMin,
		SortOrder:          v.SortOrder,
		SourceCount:        v.SourceCount,
		ActiveSourceCount:  v.ActiveSourceCount,
		ProtocolsJSON:      v.ProtocolsJSON,
		UpstreamNamesJSON:  v.UpstreamNamesJSON,
		CreatedAt:          v.CreatedAt,
		UpdatedAt:          v.UpdatedAt,
	}
}

// optionalPositiveInt 把 0 与负数视为未知，返回 nil。
func optionalPositiveInt(value int) *int {
	if value <= 0 {
		return nil
	}
	return &value
}

func optionalTrimmedString(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

// UpstreamModelResponse 上游模型路由绑定响应 DTO。
type UpstreamModelResponse struct {
	ID                     uint   `json:"id"`
	RouteID                uint   `json:"routeID"`
	UpstreamID             uint   `json:"upstreamID"`
	BindingCode            string `json:"bindingCode"`
	PlatformModelID        uint   `json:"platformModelID"`
	PlatformModelName      string `json:"platformModelName"`
	ModelVendor            string `json:"modelVendor"`
	ModelKindsJSON         string `json:"modelKindsJSON"`
	ModelIcon              string `json:"modelIcon"`
	UpstreamModelName      string `json:"upstreamModelName"`
	UpstreamModelVendor    string `json:"upstreamModelVendor"`
	UpstreamModelIcon      string `json:"upstreamModelIcon"`
	UpstreamModelKindsJSON string `json:"upstreamModelKindsJSON"`
	SuggestedProtocol      string `json:"suggestedProtocol"`
	Protocol               string `json:"protocol"`
	UpstreamModelStatus    string `json:"upstreamModelStatus"`
	RouteStatus            string `json:"routeStatus"`
	Priority               int    `json:"priority"`
	Weight                 int    `json:"weight"`
	Source                 string `json:"source"`
	CbFailureThreshold     int    `json:"cbFailureThreshold"`
	CbDurationMin          int    `json:"cbDurationMin"`
	CbWindowMin            int    `json:"cbWindowMin"`
	HeadersJSON            string `json:"headersJSON"`
	CircuitOpen            bool   `json:"circuitOpen"`
	CircuitUntil           string `json:"circuitUntil"`
	CreatedAt              string `json:"createdAt"`
	UpdatedAt              string `json:"updatedAt"`
}

func toUpstreamModelResponse(v appchannel.UpstreamModelView) UpstreamModelResponse {
	return UpstreamModelResponse{
		ID:                     v.ID,
		RouteID:                v.RouteID,
		UpstreamID:             v.UpstreamID,
		BindingCode:            v.BindingCode,
		PlatformModelID:        v.PlatformModelID,
		PlatformModelName:      v.PlatformModelName,
		ModelVendor:            v.ModelVendor,
		ModelKindsJSON:         v.ModelKindsJSON,
		ModelIcon:              v.ModelIcon,
		UpstreamModelName:      v.UpstreamModelName,
		UpstreamModelVendor:    v.UpstreamModelVendor,
		UpstreamModelIcon:      v.UpstreamModelIcon,
		UpstreamModelKindsJSON: v.UpstreamModelKindsJSON,
		SuggestedProtocol:      v.SuggestedProtocol,
		Protocol:               v.Protocol,
		UpstreamModelStatus:    v.UpstreamModelStatus,
		RouteStatus:            v.RouteStatus,
		Priority:               v.Priority,
		Weight:                 v.Weight,
		Source:                 v.Source,
		CbFailureThreshold:     v.CbFailureThreshold,
		CbDurationMin:          v.CbDurationMin,
		CbWindowMin:            v.CbWindowMin,
		HeadersJSON:            security.RedactHeadersJSON(v.HeadersJSON),
		CircuitOpen:            v.CircuitOpen,
		CircuitUntil:           v.CircuitUntil,
		CreatedAt:              v.CreatedAt,
		UpdatedAt:              v.UpdatedAt,
	}
}

// ModelUpstreamSourceResponse 模型上游来源响应 DTO。
type ModelUpstreamSourceResponse struct {
	ID                     uint   `json:"id"`
	UpstreamID             uint   `json:"upstreamID"`
	UpstreamName           string `json:"upstreamName"`
	UpstreamStatus         string `json:"upstreamStatus"`
	BaseURL                string `json:"baseURL"`
	BindingCode            string `json:"bindingCode"`
	UpstreamModelName      string `json:"upstreamModelName"`
	UpstreamModelKindsJSON string `json:"upstreamModelKindsJSON"`
	UpstreamModelVendor    string `json:"upstreamModelVendor"`
	UpstreamModelIcon      string `json:"upstreamModelIcon"`
	SuggestedProtocol      string `json:"suggestedProtocol"`
	UpstreamModelStatus    string `json:"upstreamModelStatus"`
	Protocol               string `json:"protocol"`
	Status                 string `json:"status"`
	Priority               int    `json:"priority"`
	Weight                 int    `json:"weight"`
	Source                 string `json:"source"`
	CbFailureThreshold     int    `json:"cbFailureThreshold"`
	CbDurationMin          int    `json:"cbDurationMin"`
	CbWindowMin            int    `json:"cbWindowMin"`
	HeadersJSON            string `json:"headersJSON"`
	CircuitOpen            bool   `json:"circuitOpen"`
	CircuitUntil           string `json:"circuitUntil"`
	CircuitScope           string `json:"circuitScope"`
	CreatedAt              string `json:"createdAt"`
	UpdatedAt              string `json:"updatedAt"`
}

func toModelUpstreamSourceResponse(v appchannel.ModelUpstreamSourceView) ModelUpstreamSourceResponse {
	return ModelUpstreamSourceResponse{
		ID:                     v.ID,
		UpstreamID:             v.UpstreamID,
		UpstreamName:           v.UpstreamName,
		UpstreamStatus:         v.UpstreamStatus,
		BaseURL:                v.BaseURL,
		BindingCode:            v.BindingCode,
		UpstreamModelName:      v.UpstreamModelName,
		UpstreamModelKindsJSON: v.UpstreamModelKindsJSON,
		UpstreamModelVendor:    v.UpstreamModelVendor,
		UpstreamModelIcon:      v.UpstreamModelIcon,
		SuggestedProtocol:      v.SuggestedProtocol,
		UpstreamModelStatus:    v.UpstreamModelStatus,
		Protocol:               v.Protocol,
		Status:                 v.Status,
		Priority:               v.Priority,
		Weight:                 v.Weight,
		Source:                 v.Source,
		CbFailureThreshold:     v.CbFailureThreshold,
		CbDurationMin:          v.CbDurationMin,
		CbWindowMin:            v.CbWindowMin,
		HeadersJSON:            security.RedactHeadersJSON(v.HeadersJSON),
		CircuitOpen:            v.CircuitOpen,
		CircuitUntil:           v.CircuitUntil,
		CircuitScope:           v.CircuitScope,
		CreatedAt:              v.CreatedAt,
		UpdatedAt:              v.UpdatedAt,
	}
}

// UpstreamHealthResponse 上游健康状态响应 DTO。
type UpstreamHealthResponse struct {
	UpstreamID    uint   `json:"upstreamID"`
	UpstreamName  string `json:"upstreamName"`
	Status        string `json:"status"`
	FailureCount  int64  `json:"failureCount"`
	CircuitOpen   bool   `json:"circuitOpen"`
	CircuitUntil  string `json:"circuitUntil"`
	LastError     string `json:"lastError"`
	LastFailureAt string `json:"lastFailureAt"`
	LastSuccessAt string `json:"lastSuccessAt"`
}

// ModelProbeResponse 模型连通性测试响应 DTO。
type ModelProbeResponse struct {
	Success            bool                     `json:"success"`
	Status             string                   `json:"status"`
	ErrorCode          string                   `json:"errorCode,omitempty"`
	ErrorMessage       string                   `json:"errorMessage,omitempty"`
	LatencyMS          int64                    `json:"latencyMS"`
	Protocol           string                   `json:"protocol"`
	Endpoint           string                   `json:"endpoint"`
	PlatformModelID    uint                     `json:"platformModelID"`
	PlatformModelName  string                   `json:"platformModelName"`
	UpstreamID         uint                     `json:"upstreamID"`
	UpstreamName       string                   `json:"upstreamName"`
	UpstreamModelID    uint                     `json:"upstreamModelID"`
	UpstreamModelName  string                   `json:"upstreamModelName"`
	RouteID            uint                     `json:"routeID"`
	BindingCode        string                   `json:"bindingCode"`
	UpstreamStatusCode int                      `json:"upstreamStatusCode,omitempty"`
	Debug              *ModelProbeDebugResponse `json:"debug,omitempty"`
}

// ModelProbeBatchResponse 模型批量连通性测试响应 DTO。
type ModelProbeBatchResponse struct {
	TotalCount       int                  `json:"totalCount"`
	SuccessCount     int                  `json:"successCount"`
	FailedCount      int                  `json:"failedCount"`
	UnsupportedCount int                  `json:"unsupportedCount"`
	Results          []ModelProbeResponse `json:"results"`
}

// ModelProbeDebugResponse 模型测试调试快照 DTO。
type ModelProbeDebugResponse struct {
	Request  ModelProbeDebugRequestResponse  `json:"request"`
	Response ModelProbeDebugResponseResponse `json:"response"`
}

// ModelProbeDebugRequestResponse 模型测试请求调试信息 DTO。
type ModelProbeDebugRequestResponse struct {
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    string            `json:"body"`
}

// ModelProbeDebugResponseResponse 模型测试响应调试信息 DTO。
type ModelProbeDebugResponseResponse struct {
	StatusCode int               `json:"statusCode"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       string            `json:"body"`
}

func toModelProbeResponse(v appchannel.ModelProbeResult) ModelProbeResponse {
	return ModelProbeResponse{
		Success:            v.Success,
		Status:             v.Status,
		ErrorCode:          v.ErrorCode,
		ErrorMessage:       v.ErrorMessage,
		LatencyMS:          v.LatencyMS,
		Protocol:           v.Protocol,
		Endpoint:           v.Endpoint,
		PlatformModelID:    v.PlatformModelID,
		PlatformModelName:  v.PlatformModelName,
		UpstreamID:         v.UpstreamID,
		UpstreamName:       v.UpstreamName,
		UpstreamModelID:    v.UpstreamModelID,
		UpstreamModelName:  v.UpstreamModelName,
		RouteID:            v.RouteID,
		BindingCode:        v.BindingCode,
		UpstreamStatusCode: v.UpstreamStatusCode,
		Debug:              toModelProbeDebugResponse(v.Debug),
	}
}

func toModelProbeBatchResponse(v appchannel.ModelProbeBatchResult) ModelProbeBatchResponse {
	results := make([]ModelProbeResponse, 0, len(v.Results))
	for _, item := range v.Results {
		results = append(results, toModelProbeResponse(item))
	}
	return ModelProbeBatchResponse{
		TotalCount:       v.TotalCount,
		SuccessCount:     v.SuccessCount,
		FailedCount:      v.FailedCount,
		UnsupportedCount: v.UnsupportedCount,
		Results:          results,
	}
}

func toModelProbeDebugResponse(v *appchannel.ModelProbeDebugView) *ModelProbeDebugResponse {
	if v == nil {
		return nil
	}
	return &ModelProbeDebugResponse{
		Request: ModelProbeDebugRequestResponse{
			Method:  v.Request.Method,
			Path:    v.Request.Path,
			Headers: v.Request.Headers,
			Body:    v.Request.Body,
		},
		Response: ModelProbeDebugResponseResponse{
			StatusCode: v.Response.StatusCode,
			Headers:    v.Response.Headers,
			Body:       v.Response.Body,
		},
	}
}

// UpstreamRemoteModelResponse 上游远程模型预览项响应 DTO。
type UpstreamRemoteModelResponse struct {
	UpstreamModelName          string   `json:"upstreamModelName"`
	SuggestedPlatformModelName string   `json:"suggestedPlatformModelName"`
	SuggestedKindsJSON         string   `json:"suggestedKindsJSON"`
	SuggestedProtocol          string   `json:"suggestedProtocol"`
	SuggestedProtocols         []string `json:"suggestedProtocols"`
	BindingCode                string   `json:"bindingCode"`
	BoundPlatformModels        []string `json:"boundPlatformModels"`
	UpstreamModelStatus        string   `json:"upstreamModelStatus"`
	AlreadySynced              bool     `json:"alreadySynced"`
	AlreadyBound               bool     `json:"alreadyBound"`
}

// UpstreamRemoteModelsResponse 上游远程模型预览列表响应 DTO。
type UpstreamRemoteModelsResponse struct {
	Total      int                           `json:"total"`
	Items      []UpstreamRemoteModelResponse `json:"items"`
	SnapshotID string                        `json:"snapshotID"`
	SyncPlan   UpstreamModelSyncPlanResponse `json:"syncPlan"`
}

// UpstreamModelSyncPlanResponse 描述确认同步后将应用的目录变化。
type UpstreamModelSyncPlanResponse struct {
	AddedModels       []string `json:"addedModels"`
	UpdatedModels     []string `json:"updatedModels"`
	ReactivatedModels []string `json:"reactivatedModels"`
	InactivatedModels []string `json:"inactivatedModels"`
	UnchangedModels   []string `json:"unchangedModels"`
	ProtectedModels   []string `json:"protectedModels"`
	// UnresolvedProtocolModels 为同步后没有建议协议的远端模型：模型类型推断不出协议，需要为上游设置对应默认协议，
	// 或绑定时手动选择。它们仍会写入目录，与上面的分类不互斥。
	UnresolvedProtocolModels []string `json:"unresolvedProtocolModels"`
}

func toUpstreamRemoteModelsResponse(d appchannel.UpstreamRemoteModelsData) UpstreamRemoteModelsResponse {
	items := make([]UpstreamRemoteModelResponse, 0, len(d.Items))
	for _, item := range d.Items {
		items = append(items, UpstreamRemoteModelResponse{
			UpstreamModelName:          item.UpstreamModelName,
			SuggestedPlatformModelName: item.SuggestedPlatformModelName,
			SuggestedKindsJSON:         item.SuggestedKindsJSON,
			SuggestedProtocol:          item.SuggestedProtocol,
			SuggestedProtocols:         stringList(item.SuggestedProtocols),
			BindingCode:                item.BindingCode,
			BoundPlatformModels:        stringList(item.BoundPlatformModels),
			UpstreamModelStatus:        item.UpstreamModelStatus,
			AlreadySynced:              item.AlreadySynced,
			AlreadyBound:               item.AlreadyBound,
		})
	}
	return UpstreamRemoteModelsResponse{
		Total:      d.Total,
		Items:      items,
		SnapshotID: d.SnapshotID,
		SyncPlan: UpstreamModelSyncPlanResponse{
			AddedModels:              stringList(d.SyncPlan.AddedModels),
			UpdatedModels:            stringList(d.SyncPlan.UpdatedModels),
			ReactivatedModels:        stringList(d.SyncPlan.ReactivatedModels),
			InactivatedModels:        stringList(d.SyncPlan.InactivatedModels),
			UnchangedModels:          stringList(d.SyncPlan.UnchangedModels),
			ProtectedModels:          stringList(d.SyncPlan.ProtectedModels),
			UnresolvedProtocolModels: stringList(d.SyncPlan.UnresolvedProtocolModels),
		},
	}
}

func stringList(items []string) []string {
	if items == nil {
		return []string{}
	}
	return items
}

// UpstreamSyncModelResponse 单个同步结果响应 DTO。
type UpstreamSyncModelResponse struct {
	UpstreamModelName string `json:"upstreamModelName"`
	BindingCode       string `json:"bindingCode"`
	SuggestedProtocol string `json:"suggestedProtocol"`
	KindsJSON         string `json:"kindsJSON"`
	Status            string `json:"status"`
	Created           bool   `json:"created"`
	Updated           bool   `json:"updated"`
	Reactivated       bool   `json:"reactivated"`
	Protected         bool   `json:"protected"`
}

// SyncUpstreamModelsResponse 同步上游模型响应 DTO。
type SyncUpstreamModelsResponse struct {
	SnapshotID              string                      `json:"snapshotID"`
	TotalUpstream           int                         `json:"totalUpstream"`
	CreatedUpstreamModels   int                         `json:"createdUpstreamModels"`
	UpdatedUpstreamModels   int                         `json:"updatedUpstreamModels"`
	UnchangedUpstreamModels int                         `json:"unchangedUpstreamModels"`
	ProtectedUpstreamModels int                         `json:"protectedUpstreamModels"`
	ExistingUpstreamModels  int                         `json:"existingUpstreamModels"`
	SkippedUpstreamModels   int                         `json:"skippedUpstreamModels"`
	InactivatedModels       int64                       `json:"inactivatedModels"`
	ReactivatedModels       int                         `json:"reactivatedModels"`
	SyncedModels            []UpstreamSyncModelResponse `json:"syncedModels"`
	// UnresolvedProtocolModels 为写入目录但没有建议协议的远端模型，含义同同步计划中的同名字段。
	UnresolvedProtocolModels []string `json:"unresolvedProtocolModels"`
}

func toSyncUpstreamModelsResponse(d appchannel.SyncUpstreamModelsData) SyncUpstreamModelsResponse {
	models := make([]UpstreamSyncModelResponse, 0, len(d.SyncedModels))
	for _, m := range d.SyncedModels {
		models = append(models, UpstreamSyncModelResponse{
			UpstreamModelName: m.UpstreamModelName,
			BindingCode:       m.BindingCode,
			SuggestedProtocol: m.SuggestedProtocol,
			KindsJSON:         m.KindsJSON,
			Status:            m.Status,
			Created:           m.Created,
			Updated:           m.Updated,
			Reactivated:       m.Reactivated,
			Protected:         m.Protected,
		})
	}
	return SyncUpstreamModelsResponse{
		SnapshotID:               d.SnapshotID,
		TotalUpstream:            d.TotalUpstream,
		CreatedUpstreamModels:    d.CreatedUpstreamModels,
		UpdatedUpstreamModels:    d.UpdatedUpstreamModels,
		UnchangedUpstreamModels:  d.UnchangedUpstreamModels,
		ProtectedUpstreamModels:  d.ProtectedUpstreamModels,
		ExistingUpstreamModels:   d.ExistingUpstreamModels,
		SkippedUpstreamModels:    d.SkippedUpstreamModels,
		InactivatedModels:        d.InactivatedModels,
		ReactivatedModels:        d.ReactivatedModels,
		SyncedModels:             models,
		UnresolvedProtocolModels: stringList(d.UnresolvedProtocolModels),
	}
}

// ImportUpstreamModelsResponse 批量导入上游模型响应 DTO。
type ImportUpstreamModelsResponse struct {
	Total           int                                 `json:"total"`
	ImportedCount   int                                 `json:"importedCount"`
	FailedCount     int                                 `json:"failedCount"`
	CreatedRoutes   int                                 `json:"createdRoutes"`
	ExistingRoutes  int                                 `json:"existingRoutes"`
	CreatedPlatform int                                 `json:"createdPlatform"`
	Results         []ImportUpstreamModelResultResponse `json:"results"`
}

type ImportUpstreamModelResultResponse struct {
	UpstreamModelName string   `json:"upstreamModelName"`
	PlatformModelName string   `json:"platformModelName"`
	BindingCode       string   `json:"bindingCode"`
	Status            string   `json:"status"`
	CreatedRoute      bool     `json:"createdRoute"`
	CreatedRoutes     int      `json:"createdRoutes"`
	ExistingRoutes    int      `json:"existingRoutes"`
	Protocols         []string `json:"protocols"`
	CreatedPlatform   bool     `json:"createdPlatform"`
	Error             string   `json:"error,omitempty"`
}

func toImportUpstreamModelsResponse(d appchannel.ImportUpstreamModelsData) ImportUpstreamModelsResponse {
	results := make([]ImportUpstreamModelResultResponse, 0, len(d.Results))
	for _, item := range d.Results {
		results = append(results, ImportUpstreamModelResultResponse{
			UpstreamModelName: item.UpstreamModelName,
			PlatformModelName: item.PlatformModelName,
			BindingCode:       item.BindingCode,
			Status:            item.Status,
			CreatedRoute:      item.CreatedRoute,
			CreatedRoutes:     item.CreatedRoutes,
			ExistingRoutes:    item.ExistingRoutes,
			Protocols:         item.Protocols,
			CreatedPlatform:   item.CreatedPlatform,
			Error:             item.Error,
		})
	}
	return ImportUpstreamModelsResponse{
		Total:           d.Total,
		ImportedCount:   d.ImportedCount,
		FailedCount:     d.FailedCount,
		CreatedRoutes:   d.CreatedRoutes,
		ExistingRoutes:  d.ExistingRoutes,
		CreatedPlatform: d.CreatedPlatform,
		Results:         results,
	}
}

// UpstreamDataResponse 单个上游包装响应 DTO。
type UpstreamDataResponse struct {
	Upstream UpstreamResponse `json:"upstream"`
}

// ModelDataResponse 单个模型包装响应 DTO。
type ModelDataResponse struct {
	Model ModelResponse `json:"model"`
}

// UpstreamModelDataResponse 单个上游模型路由绑定包装响应 DTO。
type UpstreamModelDataResponse struct {
	Binding UpstreamModelResponse `json:"binding"`
}

// ModelUpstreamSourceDataResponse 单个模型上游来源包装响应 DTO。
type ModelUpstreamSourceDataResponse struct {
	Source ModelUpstreamSourceResponse `json:"source"`
}

// BatchDeleteResultResponse 单个批量删除结果响应 DTO。
type BatchDeleteResultResponse struct {
	ID     uint   `json:"id"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// BatchDeleteResponse 批量删除响应 DTO。
type BatchDeleteResponse struct {
	Total         int                         `json:"total"`
	SuccessCount  int                         `json:"successCount"`
	NotFoundCount int                         `json:"notFoundCount"`
	FailedCount   int                         `json:"failedCount"`
	Results       []BatchDeleteResultResponse `json:"results"`
}

func toBatchDeleteResponse(d appchannel.BatchDeleteData) BatchDeleteResponse {
	results := make([]BatchDeleteResultResponse, 0, len(d.Results))
	for _, item := range d.Results {
		results = append(results, BatchDeleteResultResponse{
			ID:     item.ID,
			Status: item.Status,
			Error:  item.Error,
		})
	}
	return BatchDeleteResponse{
		Total:         d.Total,
		SuccessCount:  d.SuccessCount,
		NotFoundCount: d.NotFoundCount,
		FailedCount:   d.FailedCount,
		Results:       results,
	}
}

// CircuitResetResponse 熔断重置响应 DTO。
type CircuitResetResponse struct {
	Reset bool `json:"reset"`
}

// PublicModelResponse 面向前端的可用模型展示 DTO。
type PublicModelResponse struct {
	PlatformModelName string                      `json:"platformModelName"`
	Vendor            string                      `json:"vendor"`
	VendorName        string                      `json:"vendorName"`
	VendorIcon        string                      `json:"vendorIcon"`
	DisplayGroupID    *uint                       `json:"displayGroupID" extensions:"x-nullable,!x-omitempty"`
	DisplayGroupName  string                      `json:"displayGroupName"`
	DisplayGroupIcon  string                      `json:"displayGroupIcon"`
	KindsJSON         string                      `json:"kindsJSON"`
	Icon              string                      `json:"icon"`
	ProtocolsJSON     string                      `json:"protocolsJSON"`
	CapabilitiesJSON  string                      `json:"capabilitiesJSON"`
	Description       string                      `json:"description"`
	SortOrder         int                         `json:"sortOrder"`
	Pricing           *PublicModelPricingResponse `json:"pricing" extensions:"x-nullable,!x-omitempty"`
	// Reasoning 是后端归一化的推理强度能力（显式声明或由旧版参数控件推断）；无能力时为 null。
	Reasoning *PublicModelReasoningResponse `json:"reasoning" extensions:"x-nullable,!x-omitempty"`
	// Controls 是用户端可操作的模型控件（管理员隐藏的控件不下发），顺序即展示顺序。
	// 用户请求只提交 {控件 id: 取值}，参数片段只保存在服务端。
	Controls []PublicModelControlResponse `json:"controls"`
	// InputModalities 是模型可接收的输入模态（text / image / pdf / audio / video），按规范顺序排列；
	// 能力未知时为空数组。它描述模型本身的能力，实际是否原生发送还取决于接入协议与大小限制，由服务端决定。
	InputModalities []string `json:"inputModalities"`
	// InputModalitiesSource 为输入模态来源：explicit 为管理员在能力 JSON 中声明，catalog 为 models.dev 目录；未知时为 null。
	InputModalitiesSource *string `json:"inputModalitiesSource" enums:"explicit,catalog" extensions:"x-nullable,!x-omitempty"`
	// OutputModalities 为 models.dev 目录声明的输出模态（text / image / audio / video …），仅用于展示；未知时为空数组。
	OutputModalities []string `json:"outputModalities"`
	// ContextWindow 为上下文窗口（Token）：能力 JSON 显式配置优先，其次 models.dev 目录；未知时为 null。
	ContextWindow *int `json:"contextWindow" extensions:"x-nullable,!x-omitempty"`
	// Source 区分平台模型与用户自带 Key 的模型；personal 模型不扣平台余额、只对本人可见。
	Source string `json:"source" enums:"platform,personal"`
	// ProviderName 为个人模型所属服务的名称；平台模型为空串。
	ProviderName string `json:"providerName"`
	// ProviderIcon 为个人模型所属服务的内置图标 slug；空串表示由前端按 ProviderHost 自动匹配。
	ProviderIcon string `json:"providerIcon"`
	// ProviderHost 为个人模型所属服务的主机名；平台模型为空串。
	ProviderHost string `json:"providerHost"`
}

// PublicModelControlResponse 是一个用户端模型控件。
type PublicModelControlResponse struct {
	ID string `json:"id"`
	// Kind 为 reasoning 时是统一思考强度控件，选项为规范档位（目录来源另有 auto）。
	Kind        string `json:"kind" enums:"generic,reasoning"`
	Type        string `json:"type" enums:"select,toggle,number"`
	Label       string `json:"label"`
	Description string `json:"description"`
	// Icon 为 lucide 图标名（kebab-case）；为空时由客户端按类型选择通用图标。
	Icon      string `json:"icon"`
	Placement string `json:"placement" enums:"toolbar,menu"`
	// Default 为默认取值：select 为选项值，toggle 为 on/off，number 为十进制数值；为 null 表示不修改参数。
	Default *string `json:"default" extensions:"x-nullable,!x-omitempty"`
	// Locked 为 true 时固定使用默认值，客户端只展示不可修改。
	Locked  bool                               `json:"locked"`
	Options []PublicModelControlOptionResponse `json:"options"`
	Min     *float64                           `json:"min" extensions:"x-nullable,!x-omitempty"`
	Max     *float64                           `json:"max" extensions:"x-nullable,!x-omitempty"`
	Step    *float64                           `json:"step" extensions:"x-nullable,!x-omitempty"`
	Integer bool                               `json:"integer"`
	// Protocols 非空时控件只在这些协议键上生效。
	Protocols []string `json:"protocols"`
}

// PublicModelControlOptionResponse 是 select / toggle 控件的一个选项。
type PublicModelControlOptionResponse struct {
	Value       string `json:"value"`
	Label       string `json:"label"`
	Description string `json:"description"`
}

func toPublicModelControlResponses(controls []domainchannel.ModelControl) []PublicModelControlResponse {
	result := make([]PublicModelControlResponse, 0, len(controls))
	for _, control := range controls {
		if control.Hidden {
			continue
		}
		options := make([]PublicModelControlOptionResponse, 0, len(control.Options))
		for _, option := range control.Options {
			options = append(options, PublicModelControlOptionResponse{Value: option.Value, Label: option.Label, Description: option.Description})
		}
		result = append(result, PublicModelControlResponse{
			ID:          control.ID,
			Kind:        control.Kind,
			Type:        control.Type,
			Label:       control.Label,
			Description: control.Description,
			Icon:        control.Icon,
			Placement:   control.Placement,
			Default:     optionalTrimmedString(control.Default),
			Locked:      control.Locked,
			Options:     options,
			Min:         control.Min,
			Max:         control.Max,
			Step:        control.Step,
			Integer:     control.Integer,
			Protocols:   append([]string{}, control.Protocols...),
		})
	}
	return result
}

// PublicModelReasoningResponse 面向聊天模型选择器的推理强度能力 DTO。
type PublicModelReasoningResponse struct {
	// Levels 为可选规范档位，按由低到高排列。
	Levels []string `json:"levels" enums:"none,minimal,low,medium,high,xhigh,max"`
	// Default 为未显式选择时使用的档位。
	Default string `json:"default" enums:"none,minimal,low,medium,high,xhigh,max"`
	// ControlPath 为推断来源的原生参数路径（高级参数面板需隐藏该控件）；显式声明时为 null。
	ControlPath *string `json:"controlPath" extensions:"x-nullable,!x-omitempty"`
	// Source 为能力来源。catalog 表示由 models.dev 目录自动识别：未显式选择档位时不下发推理参数，由上游决定默认行为。
	Source string `json:"source" enums:"explicit,inferred,catalog"`
	// Locked 为 true 时档位被管理员锁定为 default，客户端传入的档位不生效。
	Locked bool `json:"locked"`
}

// PublicModelPricingResponse 面向前端的模型价格 DTO。
type PublicModelPricingResponse struct {
	Currency                string                           `json:"currency"`
	IsFree                  bool                             `json:"isFree"`
	Mode                    string                           `json:"mode"`
	InputUSDPerMTokens      float64                          `json:"inputUSDPerMTokens"`
	CacheReadUSDPerMTokens  float64                          `json:"cacheReadUSDPerMTokens"`
	CacheWriteUSDPerMTokens float64                          `json:"cacheWriteUSDPerMTokens"`
	CacheWrite5mMultiplier  float64                          `json:"cacheWrite5mMultiplier"`
	CacheWrite1hMultiplier  float64                          `json:"cacheWrite1hMultiplier"`
	OutputUSDPerMTokens     float64                          `json:"outputUSDPerMTokens"`
	CallUSDPerCall          float64                          `json:"callUSDPerCall"`
	DurationUSDPerSecond    float64                          `json:"durationUSDPerSecond"`
	Tiers                   []PublicModelPricingTierResponse `json:"tiers"`
	// 时段倍率按服务器本地时区定义；客户端用 scheduleUTCOffsetMinutes 判断当前命中的时段。
	SchedulePeriods          []PublicSchedulePeriodResponse `json:"schedulePeriods"`
	ScheduleUTCOffsetMinutes int                            `json:"scheduleUTCOffsetMinutes"`
}

// PublicSchedulePeriodResponse 面向前端的时段倍率 DTO。
type PublicSchedulePeriodResponse struct {
	Name        string `json:"name"`
	Weekdays    []int  `json:"weekdays"`
	Start       string `json:"start"`
	End         string `json:"end"`
	RatePercent int    `json:"ratePercent"`
}

// PublicModelPricingTierResponse 面向前端的模型阶梯价格 DTO。
type PublicModelPricingTierResponse struct {
	FromTokens              int64   `json:"fromTokens"`
	UpToTokens              *int64  `json:"upToTokens" extensions:"x-nullable,!x-omitempty"`
	InputUSDPerMTokens      float64 `json:"inputUSDPerMTokens"`
	CacheReadUSDPerMTokens  float64 `json:"cacheReadUSDPerMTokens"`
	CacheWriteUSDPerMTokens float64 `json:"cacheWriteUSDPerMTokens"`
	OutputUSDPerMTokens     float64 `json:"outputUSDPerMTokens"`
}

// ModelCatalogStatusResponse 是 models.dev 模型目录状态 DTO。
type ModelCatalogStatusResponse struct {
	// Origin 为当前目录数据来源：remote 为远端同步（含本地缓存恢复），builtin 为随版本发布的内置快照；无目录时为 null。
	Origin *string `json:"origin" enums:"remote,builtin" extensions:"x-nullable,!x-omitempty"`
	// FetchedAt 为目录数据的拉取时间（RFC3339）；无目录时为 null。
	FetchedAt *string `json:"fetchedAt" extensions:"x-nullable,!x-omitempty"`
	// ModelCount 为目录中的模型条目总数（含推理能力、模态与上下文窗口信息）。
	ModelCount int `json:"modelCount"`
	// LastError 为最近一次同步失败的原因；最近一次同步成功时为 null。
	LastError *string `json:"lastError" extensions:"x-nullable,!x-omitempty"`
	// Refreshing 表示目录正在同步（按需后台同步或手动同步）。
	Refreshing bool `json:"refreshing"`
}

func toModelCatalogStatusResponse(v appchannel.ModelCatalogStatus) ModelCatalogStatusResponse {
	var fetchedAt *string
	if v.FetchedAt != nil {
		formatted := v.FetchedAt.UTC().Format(time.RFC3339)
		fetchedAt = &formatted
	}
	return ModelCatalogStatusResponse{
		Origin:     optionalTrimmedString(v.Origin),
		FetchedAt:  fetchedAt,
		ModelCount: v.ModelCount,
		LastError:  optionalTrimmedString(v.LastError),
		Refreshing: v.Refreshing,
	}
}

// ModelCatalogResolveResponse 是模型编辑表单的自动识别结果。
type ModelCatalogResolveResponse struct {
	// Matched 表示 models.dev 目录中找到了对应条目；未找到时模态为空数组，请求链路按「未知」处理。
	Matched bool `json:"matched"`
	// Provider 与 ModelID 为命中的目录条目；未匹配时为 null。
	Provider *string `json:"provider" extensions:"x-nullable,!x-omitempty"`
	ModelID  *string `json:"modelId" extensions:"x-nullable,!x-omitempty"`
	// InputModalities / OutputModalities 为目录声明的输入、输出模态（text / image / pdf / audio / video）。
	InputModalities  []string `json:"inputModalities"`
	OutputModalities []string `json:"outputModalities"`
	// ContextWindow 为目录声明的上下文窗口（Token）；未知时为 null。
	ContextWindow *int `json:"contextWindow" extensions:"x-nullable,!x-omitempty"`
	// Reasoning 为不考虑显式 reasoning 声明时的推理能力（旧版思考参数推断或目录识别）；无能力时为 null。
	Reasoning *ModelCatalogReasoningResponse `json:"reasoning" extensions:"x-nullable,!x-omitempty"`
	// ReasoningTemplate 为自定义推理强度的编辑模板；尚未配置路由协议且无能力时为 null。
	ReasoningTemplate *ModelReasoningTemplateResponse `json:"reasoningTemplate" extensions:"x-nullable,!x-omitempty"`
}

// ModelCatalogReasoningResponse 是自动识别的推理能力。
type ModelCatalogReasoningResponse struct {
	Format string `json:"format" enums:"openai,anthropic_effort,anthropic_budget,gemini_level,gemini_budget,qwen,toggle"`
	// Levels 为可选规范档位，按由低到高排列。
	Levels []string `json:"levels" enums:"none,minimal,low,medium,high,xhigh,max"`
	// Default 为未显式选择时使用的档位。
	Default string `json:"default" enums:"none,minimal,low,medium,high,xhigh,max"`
	// Source 为 inferred（由旧版思考参数推断）或 catalog（models.dev 目录）。
	Source string `json:"source" enums:"inferred,catalog"`
}

// ModelReasoningTemplateResponse 是自定义推理强度的编辑模板：在 Levels 中勾选档位并指定默认档，
// 预算类格式按 Budgets 写入对应档位的预算。
type ModelReasoningTemplateResponse struct {
	Format string `json:"format" enums:"openai,anthropic_effort,anthropic_budget,gemini_level,gemini_budget,qwen,toggle"`
	// Levels 为该格式可声明的规范档位，按由低到高排列。
	Levels []string `json:"levels" enums:"none,minimal,low,medium,high,xhigh,max"`
	// Budgets 为各档位的建议预算（Token），仅预算类格式非 null。
	Budgets map[string]int `json:"budgets" extensions:"x-nullable,!x-omitempty"`
}

func toModelCatalogResolveResponse(v appchannel.ModelCatalogResolution) ModelCatalogResolveResponse {
	result := ModelCatalogResolveResponse{
		Matched:          v.Matched,
		InputModalities:  append([]string{}, v.InputModalities...),
		OutputModalities: append([]string{}, v.OutputModalities...),
	}
	if v.Matched {
		result.Provider = optionalTrimmedString(v.Provider)
		result.ModelID = optionalTrimmedString(v.ModelID)
	}
	if v.ContextWindow > 0 {
		contextWindow := v.ContextWindow
		result.ContextWindow = &contextWindow
	}
	if v.AutoReasoning != nil {
		result.Reasoning = &ModelCatalogReasoningResponse{
			Format:  v.AutoReasoning.Format,
			Levels:  append([]string{}, v.AutoReasoning.Levels...),
			Default: v.AutoReasoning.Default,
			Source:  v.AutoReasoningSource,
		}
	}
	if v.ReasoningTemplate != nil {
		template := &ModelReasoningTemplateResponse{
			Format: v.ReasoningTemplate.Format,
			Levels: append([]string{}, v.ReasoningTemplate.Levels...),
		}
		if len(v.ReasoningTemplate.Budgets) > 0 {
			template.Budgets = make(map[string]int, len(v.ReasoningTemplate.Budgets))
			for level, budget := range v.ReasoningTemplate.Budgets {
				template.Budgets[level] = budget
			}
		}
		result.ReasoningTemplate = template
	}
	return result
}

// ---------- Swagger 文档类型 ----------

// ModelCatalogResolveResponseDoc 模型编辑表单自动识别响应文档。
type ModelCatalogResolveResponseDoc struct {
	ErrorMsg string                      `json:"errorMsg"`
	Data     ModelCatalogResolveResponse `json:"data"`
}

// ModelCatalogStatusResponseDoc models.dev 模型目录状态响应文档。
type ModelCatalogStatusResponseDoc struct {
	ErrorMsg string                     `json:"errorMsg"`
	Data     ModelCatalogStatusResponse `json:"data"`
}

// UpstreamListResponseDoc 上游分页响应文档。
type UpstreamListResponseDoc struct {
	ErrorMsg string `json:"errorMsg"`
	Data     struct {
		Total   int64              `json:"total"`
		Results []UpstreamResponse `json:"results"`
	} `json:"data"`
}

// ModelListResponseDoc 模型分页响应文档。
type ModelListResponseDoc struct {
	ErrorMsg string `json:"errorMsg"`
	Data     struct {
		Total   int64           `json:"total"`
		Results []ModelResponse `json:"results"`
	} `json:"data"`
}

// UpstreamModelListResponseDoc 上游模型路由绑定分页响应文档。
type UpstreamModelListResponseDoc struct {
	ErrorMsg string `json:"errorMsg"`
	Data     struct {
		Total   int64                   `json:"total"`
		Results []UpstreamModelResponse `json:"results"`
	} `json:"data"`
}

// ModelUpstreamSourceListResponseDoc 模型上游来源分页响应文档。
type ModelUpstreamSourceListResponseDoc struct {
	ErrorMsg string `json:"errorMsg"`
	Data     struct {
		Total   int64                         `json:"total"`
		Results []ModelUpstreamSourceResponse `json:"results"`
	} `json:"data"`
}

// PublicModelListResponseDoc 可用模型列表响应文档。
type PublicModelListResponseDoc struct {
	ErrorMsg string                `json:"errorMsg"`
	Data     []PublicModelResponse `json:"data"`
}

// BatchDeleteResponseDoc 批量删除响应文档。
type BatchDeleteResponseDoc struct {
	ErrorMsg string              `json:"errorMsg"`
	Data     BatchDeleteResponse `json:"data"`
}

// CreateUpstreamResponseDoc 创建上游响应文档。
type CreateUpstreamResponseDoc struct {
	ErrorMsg string               `json:"errorMsg"`
	Data     UpstreamDataResponse `json:"data"`
}

// UpdateUpstreamResponseDoc 更新上游响应文档。
type UpdateUpstreamResponseDoc struct {
	ErrorMsg string               `json:"errorMsg"`
	Data     UpstreamDataResponse `json:"data"`
}

// CreateModelResponseDoc 创建模型响应文档。
type CreateModelResponseDoc struct {
	ErrorMsg string            `json:"errorMsg"`
	Data     ModelDataResponse `json:"data"`
}

// UpdateModelResponseDoc 更新模型响应文档。
type UpdateModelResponseDoc struct {
	ErrorMsg string            `json:"errorMsg"`
	Data     ModelDataResponse `json:"data"`
}

// SetModelProtocolsResponseDoc 平台模型协议集合更新响应文档。
type SetModelProtocolsResponseDoc struct {
	ErrorMsg string            `json:"errorMsg"`
	Data     ModelDataResponse `json:"data"`
}

// UpsertUpstreamModelResponseDoc 上游模型路由绑定响应文档。
type UpsertUpstreamModelResponseDoc struct {
	ErrorMsg string                    `json:"errorMsg"`
	Data     UpstreamModelDataResponse `json:"data"`
}

// UpdateModelUpstreamSourceResponseDoc 模型上游来源响应文档。
type UpdateModelUpstreamSourceResponseDoc struct {
	ErrorMsg string                          `json:"errorMsg"`
	Data     ModelUpstreamSourceDataResponse `json:"data"`
}

// SyncUpstreamModelsResponseDoc 同步上游模型响应文档。
type SyncUpstreamModelsResponseDoc struct {
	ErrorMsg string                     `json:"errorMsg"`
	Data     SyncUpstreamModelsResponse `json:"data"`
}

// UpstreamRemoteModelsResponseDoc 上游远程模型预览响应文档。
type UpstreamRemoteModelsResponseDoc struct {
	ErrorMsg string                       `json:"errorMsg"`
	Data     UpstreamRemoteModelsResponse `json:"data"`
}

// ImportUpstreamModelsResponseDoc 批量导入上游模型响应文档。
type ImportUpstreamModelsResponseDoc struct {
	ErrorMsg string                       `json:"errorMsg"`
	Data     ImportUpstreamModelsResponse `json:"data"`
}

// ResetUpstreamCircuitResponseDoc 重置熔断响应文档。
type ResetUpstreamCircuitResponseDoc struct {
	ErrorMsg string               `json:"errorMsg"`
	Data     CircuitResetResponse `json:"data"`
}

// ModelProbeResponseDoc 模型连通性测试响应文档。
type ModelProbeResponseDoc struct {
	ErrorMsg string             `json:"errorMsg"`
	Data     ModelProbeResponse `json:"data"`
}

// ModelProbeBatchResponseDoc 模型批量连通性测试响应文档。
type ModelProbeBatchResponseDoc struct {
	ErrorMsg string                  `json:"errorMsg"`
	Data     ModelProbeBatchResponse `json:"data"`
}

// LLMSettingResponse 全局设置项响应 DTO。
type LLMSettingResponse struct {
	ID          uint   `json:"id"`
	Key         string `json:"key"`
	Value       string `json:"value"`
	Description string `json:"description"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

func toLLMSettingResponse(v domainchannel.LLMSetting) LLMSettingResponse {
	return LLMSettingResponse{
		ID:          v.ID,
		Key:         v.Key,
		Value:       v.Value,
		Description: v.Description,
		CreatedAt:   v.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   v.UpdatedAt.Format(time.RFC3339),
	}
}

// toPublicModelResponse 将模型视图转为面向前端的响应 DTO。
func toPublicModelResponse(v appchannel.ModelView, resolver appchannel.ModelCapabilityResolver) PublicModelResponse {
	return publicModelResponseFromCapability(v, resolver.Resolve(v))
}

func publicModelResponseFromCapability(v appchannel.ModelView, reasoning appchannel.ModelCapabilityInfo) PublicModelResponse {
	return PublicModelResponse{
		PlatformModelName:     v.PlatformModelName,
		Vendor:                v.Vendor,
		VendorName:            v.VendorName,
		VendorIcon:            v.VendorIcon,
		DisplayGroupID:        v.DisplayGroupID,
		DisplayGroupName:      v.DisplayGroupName,
		DisplayGroupIcon:      v.DisplayGroupIcon,
		KindsJSON:             v.KindsJSON,
		Icon:                  v.Icon,
		ProtocolsJSON:         v.ProtocolsJSON,
		CapabilitiesJSON:      v.CapabilitiesJSON,
		Description:           v.Description,
		SortOrder:             v.SortOrder,
		Pricing:               toPublicModelPricingResponse(v.Pricing),
		Reasoning:             toPublicModelReasoningResponse(reasoning.View),
		Controls:              toPublicModelControlResponses(reasoning.Controls),
		InputModalities:       append([]string{}, reasoning.InputModalities.Values...),
		InputModalitiesSource: optionalTrimmedString(reasoning.InputModalities.Source),
		OutputModalities:      append([]string{}, reasoning.OutputModalities...),
		ContextWindow:         optionalPositiveInt(reasoning.ContextWindow),
		Source:                modelSourcePlatform,
	}
}

const (
	modelSourcePlatform = "platform"
	modelSourcePersonal = "personal"
)

// toPersonalModelResponse 构造个人模型的目录项：能力按上游真实模型名从内置目录推断，
// 对外名称替换为 personal: 引用，并按服务名分组，价格恒为空（不扣平台余额）。
func toPersonalModelResponse(service *appchannel.Service, entry apppersonalprovider.ModelEntry, resolver appchannel.ModelCapabilityResolver) PublicModelResponse {
	view := service.ExternalModelView(appchannel.ExternalModel{
		Ref:          entry.Ref,
		Model:        entry.Model,
		ProviderName: entry.ProviderName,
		Protocol:     entry.Protocol,
	})
	result := publicModelResponseFromCapability(view, resolver.ResolveExternal(view))
	result.PlatformModelName = entry.Ref
	result.Description = entry.Model
	result.Pricing = nil
	result.Source = modelSourcePersonal
	result.ProviderName = entry.ProviderName
	result.ProviderIcon = entry.ProviderIcon
	result.ProviderHost = entry.ProviderHost
	return result
}

func toPublicModelReasoningResponse(v *appchannel.ModelReasoningView) *PublicModelReasoningResponse {
	if v == nil {
		return nil
	}
	var controlPath *string
	if path := strings.TrimSpace(v.ControlPath); path != "" {
		controlPath = &path
	}
	return &PublicModelReasoningResponse{
		Levels:      append([]string{}, v.Levels...),
		Default:     v.Default,
		ControlPath: controlPath,
		Source:      v.Source,
		Locked:      v.Locked,
	}
}

func toPublicModelPricingResponse(v *appbilling.PublicModelPricing) *PublicModelPricingResponse {
	if v == nil {
		return nil
	}
	tiers := make([]PublicModelPricingTierResponse, 0, len(v.Tiers))
	for _, tier := range v.Tiers {
		tiers = append(tiers, PublicModelPricingTierResponse{
			FromTokens:              tier.FromTokens,
			UpToTokens:              tier.UpToTokens,
			InputUSDPerMTokens:      tier.InputUSDPerMTokens,
			CacheReadUSDPerMTokens:  tier.CacheReadUSDPerMTokens,
			CacheWriteUSDPerMTokens: tier.CacheWriteUSDPerMTokens,
			OutputUSDPerMTokens:     tier.OutputUSDPerMTokens,
		})
	}
	periods := make([]PublicSchedulePeriodResponse, 0, len(v.SchedulePeriods))
	for _, period := range v.SchedulePeriods {
		periods = append(periods, PublicSchedulePeriodResponse{
			Name:        period.Name,
			Weekdays:    period.Weekdays,
			Start:       period.Start,
			End:         period.End,
			RatePercent: period.RatePercent,
		})
	}
	return &PublicModelPricingResponse{
		Currency:                v.Currency,
		IsFree:                  v.IsFree,
		Mode:                    v.Mode,
		InputUSDPerMTokens:      v.InputUSDPerMTokens,
		CacheReadUSDPerMTokens:  v.CacheReadUSDPerMTokens,
		CacheWriteUSDPerMTokens: v.CacheWriteUSDPerMTokens,
		CacheWrite5mMultiplier:  v.CacheWrite5mMultiplier,
		CacheWrite1hMultiplier:  v.CacheWrite1hMultiplier,
		OutputUSDPerMTokens:     v.OutputUSDPerMTokens,
		CallUSDPerCall:          v.CallUSDPerCall,
		DurationUSDPerSecond:    v.DurationUSDPerSecond,
		Tiers:                   tiers,

		SchedulePeriods:          periods,
		ScheduleUTCOffsetMinutes: v.ScheduleUTCOffsetMinutes,
	}
}

// ErrorDoc 错误响应文档。
type ErrorDoc struct {
	ErrorMsg  string `json:"errorMsg"`
	ErrorCode string `json:"errorCode,omitempty"`
	Details   any    `json:"details,omitempty"`
	RequestID string `json:"requestId,omitempty"`
	Data      any    `json:"data"`
}
