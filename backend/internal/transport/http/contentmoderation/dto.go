package contentmoderation

import (
	"encoding/json"
	"time"

	appcm "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/contentmoderation"
	domaincm "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/contentmoderation"
)

// ContentModerationPolicyRequest 配置各检查面启用的分类。
type ContentModerationPolicyRequest struct {
	InputTextCategories   []string `json:"inputTextCategories"`
	OutputTextCategories  []string `json:"outputTextCategories"`
	InputImageCategories  []string `json:"inputImageCategories"`
	OutputImageCategories []string `json:"outputImageCategories"`
}

// ContentModerationUpdateConfigRequest 更新审核服务与策略。
// 指针字段用于区分字段省略与显式零值。
type ContentModerationUpdateConfigRequest struct {
	Enabled        *bool                           `json:"enabled,omitempty"`
	BaseURL        *string                         `json:"baseUrl,omitempty"`
	APIKey         *string                         `json:"apiKey,omitempty"`
	ClearAPIKey    *bool                           `json:"clearAPIKey,omitempty"`
	Model          *string                         `json:"model,omitempty"`
	TimeoutSeconds *int                            `json:"timeoutSeconds,omitempty"`
	MaxConcurrency *int                            `json:"maxConcurrency,omitempty"`
	QueueCapacity  *int                            `json:"queueCapacity,omitempty"`
	Policy         *ContentModerationPolicyRequest `json:"policy,omitempty"`
}

// ContentModerationPolicyResponse 是规范化后的已保存策略。
type ContentModerationPolicyResponse struct {
	InputTextCategories   []string `json:"inputTextCategories"`
	OutputTextCategories  []string `json:"outputTextCategories"`
	InputImageCategories  []string `json:"inputImageCategories"`
	OutputImageCategories []string `json:"outputImageCategories"`
	Version               int64    `json:"version"`
}

// ContentModerationServiceConfigResponse 是脱敏后的审核配置。
type ContentModerationServiceConfigResponse struct {
	Enabled        bool                            `json:"enabled"`
	BaseURL        string                          `json:"baseUrl"`
	APIKeyMasked   string                          `json:"apiKeyMasked,omitempty"`
	HasAPIKey      bool                            `json:"hasAPIKey"`
	Model          string                          `json:"model"`
	TimeoutSeconds int                             `json:"timeoutSeconds"`
	MaxConcurrency int                             `json:"maxConcurrency"`
	QueueCapacity  int                             `json:"queueCapacity"`
	Policy         ContentModerationPolicyResponse `json:"policy"`
}

// ContentModerationCategoryCatalogResponse 按模态列出支持的分类。
type ContentModerationCategoryCatalogResponse struct {
	Text  []string `json:"text"`
	Image []string `json:"image"`
}

// ContentModerationConfigDataResponse 是 GET 配置接口的响应载荷。
type ContentModerationConfigDataResponse struct {
	Config     ContentModerationServiceConfigResponse   `json:"config"`
	Categories ContentModerationCategoryCatalogResponse `json:"categories"`
}

// ContentModerationConfigResponseDoc 描述配置接口的标准响应包装。
type ContentModerationConfigResponseDoc struct {
	ErrorMsg string                              `json:"errorMsg"`
	Data     ContentModerationConfigDataResponse `json:"data"`
}

// ContentModerationConfigUpdateDataResponse 是 PUT 配置接口的响应载荷。
type ContentModerationConfigUpdateDataResponse struct {
	Config ContentModerationServiceConfigResponse `json:"config"`
}

// ContentModerationConfigUpdateResponseDoc 描述更新接口的标准响应包装。
type ContentModerationConfigUpdateResponseDoc struct {
	ErrorMsg string                                    `json:"errorMsg"`
	Data     ContentModerationConfigUpdateDataResponse `json:"data"`
}

// ContentModerationProbeResultResponse 描述单个探测面。
type ContentModerationProbeResultResponse struct {
	Valid     bool   `json:"valid"`
	Model     string `json:"model,omitempty"`
	LatencyMS int64  `json:"latencyMS"`
	Error     string `json:"error,omitempty"`
}

// ContentModerationProbeResponse 是探测接口的响应载荷。
type ContentModerationProbeResponse struct {
	Text  ContentModerationProbeResultResponse `json:"text"`
	Image ContentModerationProbeResultResponse `json:"image"`
}

// ContentModerationProbeResponseDoc 描述探测接口的标准响应包装。
type ContentModerationProbeResponseDoc struct {
	ErrorMsg string                         `json:"errorMsg"`
	Data     ContentModerationProbeResponse `json:"data"`
}

// ContentModerationDailyStatResponse 是一行匿名聚合统计数据。
type ContentModerationDailyStatResponse struct {
	StatDate     time.Time `json:"statDate"`
	Direction    string    `json:"direction"`
	Modality     string    `json:"modality"`
	Result       string    `json:"result"`
	Category     string    `json:"category"`
	CheckCount   int64     `json:"checkCount"`
	ContentItems int64     `json:"contentItems"`
	HitCount     int64     `json:"hitCount"`
	FailureCount int64     `json:"failureCount"`
	LatencySumMS int64     `json:"latencySumMS"`
	LatencyCount int64     `json:"latencyCount"`
}

// ContentModerationStatsDataResponse 是统计接口的响应载荷。
type ContentModerationStatsDataResponse struct {
	Items []ContentModerationDailyStatResponse `json:"items"`
}

// ContentModerationStatsResponseDoc 描述统计接口的标准响应包装。
type ContentModerationStatsResponseDoc struct {
	ErrorMsg string                             `json:"errorMsg"`
	Data     ContentModerationStatsDataResponse `json:"data"`
}

// ContentModerationEventResponse 是一行保留的事件元数据。
type ContentModerationEventResponse struct {
	PublicID        string    `json:"publicID"`
	UserID          uint      `json:"userID"`
	UserLabel       string    `json:"userLabel,omitempty"`
	Username        string    `json:"username,omitempty"`
	ConversationID  uint      `json:"conversationID"`
	RunID           string    `json:"runID"`
	MessagePublicID string    `json:"messagePublicID"`
	Direction       string    `json:"direction"`
	Modality        string    `json:"modality"`
	Model           string    `json:"model"`
	PolicyVersion   int64     `json:"policyVersion"`
	Result          string    `json:"result"`
	Categories      []string  `json:"categories"`
	LatencyMS       int64     `json:"latencyMS"`
	ErrorCode       string    `json:"errorCode"`
	ErrorMessage    string    `json:"errorMessage"`
	ContentSummary  string    `json:"contentSummary"`
	CreatedAt       time.Time `json:"createdAt"`
}

// ContentModerationEventListDataResponse 是分页事件列表载荷。
type ContentModerationEventListDataResponse struct {
	Items    []ContentModerationEventResponse `json:"items"`
	Total    int64                            `json:"total"`
	Page     int                              `json:"page"`
	PageSize int                              `json:"pageSize"`
}

// ContentModerationEventListResponseDoc 描述事件列表接口的标准响应包装。
type ContentModerationEventListResponseDoc struct {
	ErrorMsg string                                 `json:"errorMsg"`
	Data     ContentModerationEventListDataResponse `json:"data"`
}

// ContentModerationIsolatedImageResponse 暴露审核元数据但不含存储路径。
type ContentModerationIsolatedImageResponse struct {
	Index        int    `json:"index"`
	SHA256       string `json:"sha256"`
	MimeType     string `json:"mimeType"`
	SizeBytes    int64  `json:"sizeBytes"`
	SourceFileID string `json:"sourceFileID,omitempty"`
}

// ContentModerationEventDetailResponse 是超级管理员事件详情载荷。
type ContentModerationEventDetailResponse struct {
	Event           ContentModerationEventResponse           `json:"event"`
	CategoryScores  map[string]float64                       `json:"categoryScores"`
	DecryptedText   string                                   `json:"decryptedText,omitempty"`
	TextAvailable   bool                                     `json:"textAvailable"`
	ImagesAvailable bool                                     `json:"imagesAvailable"`
	Images          []ContentModerationIsolatedImageResponse `json:"images"`
}

// ContentModerationEventDetailResponseDoc 描述事件详情接口的标准响应包装。
type ContentModerationEventDetailResponseDoc struct {
	ErrorMsg string                               `json:"errorMsg"`
	Data     ContentModerationEventDetailResponse `json:"data"`
}

func (request ContentModerationUpdateConfigRequest) toApplicationInput() appcm.UpdateConfigInput {
	input := appcm.UpdateConfigInput{
		Enabled:        request.Enabled,
		BaseURL:        request.BaseURL,
		APIKey:         request.APIKey,
		Model:          request.Model,
		TimeoutSeconds: request.TimeoutSeconds,
		MaxConcurrency: request.MaxConcurrency,
		QueueCapacity:  request.QueueCapacity,
	}
	if request.ClearAPIKey != nil {
		input.ClearAPIKey = *request.ClearAPIKey
	}
	if request.Policy != nil {
		input.Policy = &appcm.Policy{
			InputTextCategories:   request.Policy.InputTextCategories,
			OutputTextCategories:  request.Policy.OutputTextCategories,
			InputImageCategories:  request.Policy.InputImageCategories,
			OutputImageCategories: request.Policy.OutputImageCategories,
		}
	}
	return input
}

func toConfigResponse(config *appcm.ServiceConfig) ContentModerationServiceConfigResponse {
	if config == nil {
		return ContentModerationServiceConfigResponse{}
	}
	return ContentModerationServiceConfigResponse{
		Enabled:        config.Enabled,
		BaseURL:        config.BaseURL,
		APIKeyMasked:   config.APIKeyMasked,
		HasAPIKey:      config.HasAPIKey,
		Model:          config.Model,
		TimeoutSeconds: config.TimeoutSeconds,
		MaxConcurrency: config.MaxConcurrency,
		QueueCapacity:  config.QueueCapacity,
		Policy: ContentModerationPolicyResponse{
			InputTextCategories:   config.Policy.InputTextCategories,
			OutputTextCategories:  config.Policy.OutputTextCategories,
			InputImageCategories:  config.Policy.InputImageCategories,
			OutputImageCategories: config.Policy.OutputImageCategories,
			Version:               config.Policy.Version,
		},
	}
}

func toProbeResponse(result *appcm.ProbeResponse) ContentModerationProbeResponse {
	if result == nil {
		return ContentModerationProbeResponse{}
	}
	return ContentModerationProbeResponse{
		Text: ContentModerationProbeResultResponse{
			Valid:     result.Text.Valid,
			Model:     result.Text.Model,
			LatencyMS: result.Text.Latency,
			Error:     result.Text.Error,
		},
		Image: ContentModerationProbeResultResponse{
			Valid:     result.Image.Valid,
			Model:     result.Image.Model,
			LatencyMS: result.Image.Latency,
			Error:     result.Image.Error,
		},
	}
}

func toDailyStatResponse(item domaincm.DailyStat) ContentModerationDailyStatResponse {
	return ContentModerationDailyStatResponse{
		StatDate:     item.StatDate,
		Direction:    item.Direction,
		Modality:     item.Modality,
		Result:       item.Result,
		Category:     item.Category,
		CheckCount:   item.CheckCount,
		ContentItems: item.ContentItems,
		HitCount:     item.HitCount,
		FailureCount: item.FailureCount,
		LatencySumMS: item.LatencySumMS,
		LatencyCount: item.LatencyCount,
	}
}

func toEventResponse(item domaincm.Event, label string, username string) ContentModerationEventResponse {
	categories := make([]string, 0)
	_ = json.Unmarshal([]byte(item.CategoriesJSON), &categories)
	return ContentModerationEventResponse{
		PublicID:        item.PublicID,
		UserID:          item.UserID,
		UserLabel:       label,
		Username:        username,
		ConversationID:  item.ConversationID,
		RunID:           item.RunID,
		MessagePublicID: item.MessagePublicID,
		Direction:       item.Direction,
		Modality:        item.Modality,
		Model:           item.Model,
		PolicyVersion:   item.PolicyVersion,
		Result:          item.Result,
		Categories:      categories,
		LatencyMS:       item.LatencyMS,
		ErrorCode:       item.ErrorCode,
		ErrorMessage:    item.ErrorMessage,
		ContentSummary:  item.ContentSummary,
		CreatedAt:       item.CreatedAt,
	}
}

func toEventDetailResponse(
	detail *appcm.EventDetail,
	userLabel string,
	username string,
) ContentModerationEventDetailResponse {
	if detail == nil {
		return ContentModerationEventDetailResponse{}
	}
	categoryScores := detail.CategoryScores
	if categoryScores == nil {
		categoryScores = map[string]float64{}
	}
	images := make([]ContentModerationIsolatedImageResponse, 0, len(detail.Images))
	for _, image := range detail.Images {
		images = append(images, ContentModerationIsolatedImageResponse{
			Index:        image.Index,
			SHA256:       image.SHA256,
			MimeType:     image.MimeType,
			SizeBytes:    image.SizeBytes,
			SourceFileID: image.SourceFileID,
		})
	}
	return ContentModerationEventDetailResponse{
		Event:           toEventResponse(detail.Event, userLabel, username),
		CategoryScores:  categoryScores,
		DecryptedText:   detail.DecryptedText,
		TextAvailable:   detail.TextAvailable,
		ImagesAvailable: detail.ImagesAvailable,
		Images:          images,
	}
}

// ErrorDoc 错误响应。
type ErrorDoc struct {
	ErrorMsg  string `json:"errorMsg"`
	ErrorCode string `json:"errorCode,omitempty"`
	Details   any    `json:"details,omitempty"`
	RequestID string `json:"requestId,omitempty"`
	Data      any    `json:"data"`
}
