package promptpreset

import (
	"time"

	domainpromptpreset "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/promptpreset"
)

// PromptPresetResponse 表示预制提示词响应。
type PromptPresetResponse struct {
	ID              uint      `json:"id"`
	Scope           string    `json:"scope"`
	Title           string    `json:"title"`
	Trigger         string    `json:"trigger"`
	Description     string    `json:"description"`
	Content         string    `json:"content"`
	Enabled         bool      `json:"enabled"`
	SortOrder       int       `json:"sortOrder"`
	CreatedByUserID uint      `json:"createdByUserID"`
	UpdatedByUserID uint      `json:"updatedByUserID"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

// PromptPresetDataResponse 包裹单条预制提示词响应。
type PromptPresetDataResponse struct {
	PromptPreset PromptPresetResponse `json:"promptPreset"`
}

// PromptPresetDeleteDataResponse 表示删除响应。
type PromptPresetDeleteDataResponse struct {
	Deleted bool `json:"deleted"`
}

// WritePromptPresetRequest 表示创建预制提示词请求。
type WritePromptPresetRequest struct {
	Title       string `json:"title" binding:"required,max=64"`
	Trigger     string `json:"trigger" binding:"required,max=64"`
	Description string `json:"description,omitempty" binding:"max=256"`
	Content     string `json:"content" binding:"required,max=10000"`
	Enabled     bool   `json:"enabled,omitempty"`
	SortOrder   int    `json:"sortOrder,omitempty"`
}

// PatchPromptPresetRequest 表示更新预制提示词请求。
type PatchPromptPresetRequest struct {
	Title       *string `json:"title,omitempty" binding:"omitempty,max=64"`
	Trigger     *string `json:"trigger,omitempty" binding:"omitempty,max=64"`
	Description *string `json:"description,omitempty" binding:"omitempty,max=256"`
	Content     *string `json:"content,omitempty" binding:"omitempty,max=10000"`
	Enabled     *bool   `json:"enabled,omitempty"`
	SortOrder   *int    `json:"sortOrder,omitempty"`
}

// PromptPresetPageResponseDoc 用于 Swagger 展示分页响应。
type PromptPresetPageResponseDoc struct {
	ErrorMsg string `json:"errorMsg"`
	Data     struct {
		Total   int64                  `json:"total"`
		Results []PromptPresetResponse `json:"results"`
	} `json:"data"`
}

// PromptPresetResponseDoc 用于 Swagger 展示单条响应。
type PromptPresetResponseDoc struct {
	ErrorMsg string                   `json:"errorMsg"`
	Data     PromptPresetDataResponse `json:"data"`
}

// PromptPresetDeleteResponseDoc 用于 Swagger 展示删除响应。
type PromptPresetDeleteResponseDoc struct {
	ErrorMsg string                         `json:"errorMsg"`
	Data     PromptPresetDeleteDataResponse `json:"data"`
}

// ErrorDoc 表示错误响应。
type ErrorDoc struct {
	ErrorMsg string `json:"errorMsg"`
}

func toPromptPresetResponses(items []domainpromptpreset.PromptPreset) []PromptPresetResponse {
	results := make([]PromptPresetResponse, 0, len(items))
	for _, item := range items {
		results = append(results, toPromptPresetResponse(item))
	}
	return results
}

func toPromptPresetResponse(item domainpromptpreset.PromptPreset) PromptPresetResponse {
	return PromptPresetResponse{
		ID:              item.ID,
		Scope:           item.Scope,
		Title:           item.Title,
		Trigger:         item.Trigger,
		Description:     item.Description,
		Content:         item.Content,
		Enabled:         item.Enabled,
		SortOrder:       item.SortOrder,
		CreatedByUserID: item.CreatedByUserID,
		UpdatedByUserID: item.UpdatedByUserID,
		CreatedAt:       item.CreatedAt,
		UpdatedAt:       item.UpdatedAt,
	}
}
