package channel

import (
	"time"

	appbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/billing"
	appchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
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
	ID                uint   `json:"id"`
	PlatformModelName string `json:"platformModelName"`
	Vendor            string `json:"vendor"`
	KindsJSON         string `json:"kindsJSON"`
	Icon              string `json:"icon"`
	CapabilitiesJSON  string `json:"capabilitiesJSON"`
	SystemPrompt      string `json:"systemPrompt"`
	AccessScope       string `json:"accessScope"`
	Status            string `json:"status"`
	Description       string `json:"description"`
	SortOrder         int    `json:"sortOrder"`
	SourceCount       int64  `json:"sourceCount"`
	ActiveSourceCount int64  `json:"activeSourceCount"`
	ProtocolsJSON     string `json:"protocolsJSON"`
	CreatedAt         string `json:"createdAt"`
	UpdatedAt         string `json:"updatedAt"`
}

func toModelResponse(v appchannel.ModelView) ModelResponse {
	return ModelResponse{
		ID:                v.ID,
		PlatformModelName: v.PlatformModelName,
		Vendor:            v.Vendor,
		KindsJSON:         v.KindsJSON,
		Icon:              v.Icon,
		CapabilitiesJSON:  v.CapabilitiesJSON,
		SystemPrompt:      v.SystemPrompt,
		AccessScope:       v.AccessScope,
		Status:            v.Status,
		Description:       v.Description,
		SortOrder:         v.SortOrder,
		SourceCount:       v.SourceCount,
		ActiveSourceCount: v.ActiveSourceCount,
		ProtocolsJSON:     v.ProtocolsJSON,
		CreatedAt:         v.CreatedAt,
		UpdatedAt:         v.UpdatedAt,
	}
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
	HeadersJSON            string `json:"headersJSON"`
	CircuitOpen            bool   `json:"circuitOpen"`
	CircuitUntil           string `json:"circuitUntil"`
	CreatedAt              string `json:"createdAt"`
	UpdatedAt              string `json:"updatedAt"`
}

func toModelUpstreamSourceResponse(v appchannel.ModelUpstreamSourceView) ModelUpstreamSourceResponse {
	return ModelUpstreamSourceResponse{
		ID:                     v.ID,
		UpstreamID:             v.UpstreamID,
		UpstreamName:           v.UpstreamName,
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
		HeadersJSON:            security.RedactHeadersJSON(v.HeadersJSON),
		CircuitOpen:            v.CircuitOpen,
		CircuitUntil:           v.CircuitUntil,
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

func toUpstreamHealthResponse(v appchannel.UpstreamHealthView) UpstreamHealthResponse {
	return UpstreamHealthResponse{
		UpstreamID:    v.UpstreamID,
		UpstreamName:  v.UpstreamName,
		Status:        v.Status,
		FailureCount:  v.FailureCount,
		CircuitOpen:   v.CircuitOpen,
		CircuitUntil:  v.CircuitUntil,
		LastError:     v.LastError,
		LastFailureAt: v.LastFailureAt,
		LastSuccessAt: v.LastSuccessAt,
	}
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
	Total int                           `json:"total"`
	Items []UpstreamRemoteModelResponse `json:"items"`
}

func toUpstreamRemoteModelsResponse(d appchannel.UpstreamRemoteModelsData) UpstreamRemoteModelsResponse {
	items := make([]UpstreamRemoteModelResponse, 0, len(d.Items))
	for _, item := range d.Items {
		items = append(items, UpstreamRemoteModelResponse{
			UpstreamModelName:          item.UpstreamModelName,
			SuggestedPlatformModelName: item.SuggestedPlatformModelName,
			SuggestedKindsJSON:         item.SuggestedKindsJSON,
			SuggestedProtocol:          item.SuggestedProtocol,
			SuggestedProtocols:         item.SuggestedProtocols,
			BindingCode:                item.BindingCode,
			BoundPlatformModels:        item.BoundPlatformModels,
			UpstreamModelStatus:        item.UpstreamModelStatus,
			AlreadySynced:              item.AlreadySynced,
			AlreadyBound:               item.AlreadyBound,
		})
	}
	return UpstreamRemoteModelsResponse{Total: d.Total, Items: items}
}

// UpstreamSyncModelResponse 单个同步结果响应 DTO。
type UpstreamSyncModelResponse struct {
	UpstreamModelName string `json:"upstreamModelName"`
	BindingCode       string `json:"bindingCode"`
	SuggestedProtocol string `json:"suggestedProtocol"`
	KindsJSON         string `json:"kindsJSON"`
	Status            string `json:"status"`
	Created           bool   `json:"created"`
}

// SyncUpstreamModelsResponse 同步上游模型响应 DTO。
type SyncUpstreamModelsResponse struct {
	TotalUpstream          int                         `json:"totalUpstream"`
	CreatedUpstreamModels  int                         `json:"createdUpstreamModels"`
	ExistingUpstreamModels int                         `json:"existingUpstreamModels"`
	SkippedUpstreamModels  int                         `json:"skippedUpstreamModels"`
	InactivatedModels      int64                       `json:"inactivatedModels"`
	SyncedModels           []UpstreamSyncModelResponse `json:"syncedModels"`
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
		})
	}
	return SyncUpstreamModelsResponse{
		TotalUpstream:          d.TotalUpstream,
		CreatedUpstreamModels:  d.CreatedUpstreamModels,
		ExistingUpstreamModels: d.ExistingUpstreamModels,
		SkippedUpstreamModels:  d.SkippedUpstreamModels,
		InactivatedModels:      d.InactivatedModels,
		SyncedModels:           models,
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
	KindsJSON         string                      `json:"kindsJSON"`
	Icon              string                      `json:"icon"`
	ProtocolsJSON     string                      `json:"protocolsJSON"`
	CapabilitiesJSON  string                      `json:"capabilitiesJSON"`
	Description       string                      `json:"description"`
	SortOrder         int                         `json:"sortOrder"`
	Pricing           *PublicModelPricingResponse `json:"pricing"`
}

// PublicModelPricingResponse 面向前端的模型价格 DTO。
type PublicModelPricingResponse struct {
	Currency                string                           `json:"currency"`
	IsFree                  bool                             `json:"isFree"`
	Mode                    string                           `json:"mode"`
	InputUSDPerMTokens      float64                          `json:"inputUSDPerMTokens"`
	CacheReadUSDPerMTokens  float64                          `json:"cacheReadUSDPerMTokens"`
	CacheWriteUSDPerMTokens float64                          `json:"cacheWriteUSDPerMTokens"`
	OutputUSDPerMTokens     float64                          `json:"outputUSDPerMTokens"`
	CallUSDPerCall          float64                          `json:"callUSDPerCall"`
	DurationUSDPerSecond    float64                          `json:"durationUSDPerSecond"`
	Tiers                   []PublicModelPricingTierResponse `json:"tiers"`
}

// PublicModelPricingTierResponse 面向前端的模型阶梯价格 DTO。
type PublicModelPricingTierResponse struct {
	FromTokens              int64   `json:"fromTokens"`
	UpToTokens              *int64  `json:"upToTokens"`
	InputUSDPerMTokens      float64 `json:"inputUSDPerMTokens"`
	CacheReadUSDPerMTokens  float64 `json:"cacheReadUSDPerMTokens"`
	CacheWriteUSDPerMTokens float64 `json:"cacheWriteUSDPerMTokens"`
	OutputUSDPerMTokens     float64 `json:"outputUSDPerMTokens"`
}

// ---------- Swagger 文档类型 ----------

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
func toPublicModelResponse(v appchannel.ModelView) PublicModelResponse {
	return PublicModelResponse{
		PlatformModelName: v.PlatformModelName,
		Vendor:            v.Vendor,
		KindsJSON:         v.KindsJSON,
		Icon:              v.Icon,
		ProtocolsJSON:     v.ProtocolsJSON,
		CapabilitiesJSON:  v.CapabilitiesJSON,
		Description:       v.Description,
		SortOrder:         v.SortOrder,
		Pricing:           toPublicModelPricingResponse(v.Pricing),
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
	return &PublicModelPricingResponse{
		Currency:                v.Currency,
		IsFree:                  v.IsFree,
		Mode:                    v.Mode,
		InputUSDPerMTokens:      v.InputUSDPerMTokens,
		CacheReadUSDPerMTokens:  v.CacheReadUSDPerMTokens,
		CacheWriteUSDPerMTokens: v.CacheWriteUSDPerMTokens,
		OutputUSDPerMTokens:     v.OutputUSDPerMTokens,
		CallUSDPerCall:          v.CallUSDPerCall,
		DurationUSDPerSecond:    v.DurationUSDPerSecond,
		Tiers:                   tiers,
	}
}

// ErrorDoc 错误响应文档。
type ErrorDoc struct {
	ErrorMsg  string      `json:"errorMsg"`
	ErrorCode string      `json:"errorCode,omitempty"`
	Details   interface{} `json:"details,omitempty"`
	RequestID string      `json:"requestId,omitempty"`
	Data      interface{} `json:"data"`
}
