package contentmoderation

import (
	"encoding/json"
	"time"

	appcm "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/contentmoderation"
	domaincm "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/contentmoderation"
)

// ContentModerationPolicyRequest configures the categories enabled for each surface.
type ContentModerationPolicyRequest struct {
	InputTextCategories   []string `json:"inputTextCategories"`
	OutputTextCategories  []string `json:"outputTextCategories"`
	InputImageCategories  []string `json:"inputImageCategories"`
	OutputImageCategories []string `json:"outputImageCategories"`
}

// ContentModerationUpdateConfigRequest updates the moderation service and policy.
// Pointer fields preserve the difference between omission and explicit zero values.
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

// ContentModerationPolicyResponse is the normalized saved policy.
type ContentModerationPolicyResponse struct {
	InputTextCategories   []string `json:"inputTextCategories"`
	OutputTextCategories  []string `json:"outputTextCategories"`
	InputImageCategories  []string `json:"inputImageCategories"`
	OutputImageCategories []string `json:"outputImageCategories"`
	Version               int64    `json:"version"`
}

// ContentModerationServiceConfigResponse is the masked moderation configuration.
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

// ContentModerationCategoryCatalogResponse lists categories supported by modality.
type ContentModerationCategoryCatalogResponse struct {
	Text  []string `json:"text"`
	Image []string `json:"image"`
}

// ContentModerationConfigDataResponse is the GET config response payload.
type ContentModerationConfigDataResponse struct {
	Config     ContentModerationServiceConfigResponse   `json:"config"`
	Categories ContentModerationCategoryCatalogResponse `json:"categories"`
}

// ContentModerationConfigResponseDoc documents the standard config response envelope.
type ContentModerationConfigResponseDoc struct {
	ErrorMsg string                              `json:"errorMsg"`
	Data     ContentModerationConfigDataResponse `json:"data"`
}

// ContentModerationConfigUpdateDataResponse is the PUT config response payload.
type ContentModerationConfigUpdateDataResponse struct {
	Config ContentModerationServiceConfigResponse `json:"config"`
}

// ContentModerationConfigUpdateResponseDoc documents the standard update response envelope.
type ContentModerationConfigUpdateResponseDoc struct {
	ErrorMsg string                                    `json:"errorMsg"`
	Data     ContentModerationConfigUpdateDataResponse `json:"data"`
}

// ContentModerationProbeResultResponse describes one probe surface.
type ContentModerationProbeResultResponse struct {
	Valid     bool   `json:"valid"`
	Model     string `json:"model,omitempty"`
	LatencyMS int64  `json:"latencyMS"`
	Error     string `json:"error,omitempty"`
}

// ContentModerationProbeResponse is the probe response payload.
type ContentModerationProbeResponse struct {
	Text  ContentModerationProbeResultResponse `json:"text"`
	Image ContentModerationProbeResultResponse `json:"image"`
}

// ContentModerationProbeResponseDoc documents the standard probe response envelope.
type ContentModerationProbeResponseDoc struct {
	ErrorMsg string                         `json:"errorMsg"`
	Data     ContentModerationProbeResponse `json:"data"`
}

// ContentModerationDailyStatResponse is an anonymous aggregate statistics row.
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

// ContentModerationStatsDataResponse is the statistics response payload.
type ContentModerationStatsDataResponse struct {
	Items []ContentModerationDailyStatResponse `json:"items"`
}

// ContentModerationStatsResponseDoc documents the standard statistics response envelope.
type ContentModerationStatsResponseDoc struct {
	ErrorMsg string                             `json:"errorMsg"`
	Data     ContentModerationStatsDataResponse `json:"data"`
}

// ContentModerationEventResponse is a retained event metadata row.
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

// ContentModerationEventListDataResponse is the paginated event list payload.
type ContentModerationEventListDataResponse struct {
	Items    []ContentModerationEventResponse `json:"items"`
	Total    int64                            `json:"total"`
	Page     int                              `json:"page"`
	PageSize int                              `json:"pageSize"`
}

// ContentModerationEventListResponseDoc documents the standard event list response envelope.
type ContentModerationEventListResponseDoc struct {
	ErrorMsg string                                 `json:"errorMsg"`
	Data     ContentModerationEventListDataResponse `json:"data"`
}

// ContentModerationIsolatedImageResponse exposes review metadata without storage paths.
type ContentModerationIsolatedImageResponse struct {
	Index        int    `json:"index"`
	SHA256       string `json:"sha256"`
	MimeType     string `json:"mimeType"`
	SizeBytes    int64  `json:"sizeBytes"`
	SourceFileID string `json:"sourceFileID,omitempty"`
}

// ContentModerationEventDetailResponse is the super-admin event detail payload.
type ContentModerationEventDetailResponse struct {
	Event           ContentModerationEventResponse           `json:"event"`
	CategoryScores  map[string]float64                       `json:"categoryScores"`
	DecryptedText   string                                   `json:"decryptedText,omitempty"`
	TextAvailable   bool                                     `json:"textAvailable"`
	ImagesAvailable bool                                     `json:"imagesAvailable"`
	Images          []ContentModerationIsolatedImageResponse `json:"images"`
}

// ContentModerationEventDetailResponseDoc documents the standard event detail response envelope.
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
