package skill

import (
	"time"

	domainskill "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/skill"
)

// SkillResponse 表示技能响应。
type SkillResponse struct {
	ID              uint      `json:"id"`
	Scope           string    `json:"scope"`
	Title           string    `json:"title"`
	Trigger         string    `json:"trigger"`
	Description     string    `json:"description"`
	Markdown        string    `json:"markdown"`
	Enabled         bool      `json:"enabled"`
	SortOrder       int       `json:"sortOrder"`
	CreatedByUserID uint      `json:"createdByUserID"`
	UpdatedByUserID uint      `json:"updatedByUserID"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

// SkillSummaryResponse 表示技能发现列表响应，不包含 SKILL.md 内容。
type SkillSummaryResponse struct {
	ID          uint      `json:"id"`
	Scope       string    `json:"scope"`
	Title       string    `json:"title"`
	Trigger     string    `json:"trigger"`
	Description string    `json:"description"`
	Enabled     bool      `json:"enabled"`
	SortOrder   int       `json:"sortOrder"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// SkillDataResponse 包裹单条技能响应。
type SkillDataResponse struct {
	Skill SkillResponse `json:"skill"`
}

// SkillDeleteDataResponse 表示删除响应。
type SkillDeleteDataResponse struct {
	Deleted bool `json:"deleted"`
}

// WriteSkillRequest 表示创建技能请求。
type WriteSkillRequest struct {
	Title       string `json:"title" binding:"required,max=64"`
	Trigger     string `json:"trigger" binding:"required,max=64"`
	Description string `json:"description" binding:"max=256"`
	Markdown    string `json:"markdown" binding:"required,max=10000"`
	Enabled     bool   `json:"enabled"`
	SortOrder   int    `json:"sortOrder"`
}

// PatchSkillRequest 表示更新技能请求。
type PatchSkillRequest struct {
	Title       *string `json:"title" binding:"omitempty,max=64"`
	Trigger     *string `json:"trigger" binding:"omitempty,max=64"`
	Description *string `json:"description" binding:"omitempty,max=256"`
	Markdown    *string `json:"markdown" binding:"omitempty,max=10000"`
	Enabled     *bool   `json:"enabled"`
	SortOrder   *int    `json:"sortOrder"`
}

// SkillSummaryPageResponseDoc 用于 Swagger 展示技能发现分页响应。
type SkillSummaryPageResponseDoc struct {
	ErrorMsg string `json:"errorMsg"`
	Data     struct {
		Total   int64                  `json:"total"`
		Results []SkillSummaryResponse `json:"results"`
	} `json:"data"`
}

// SkillPageResponseDoc 用于 Swagger 展示完整分页响应。
type SkillPageResponseDoc struct {
	ErrorMsg string `json:"errorMsg"`
	Data     struct {
		Total   int64           `json:"total"`
		Results []SkillResponse `json:"results"`
	} `json:"data"`
}

// SkillResponseDoc 用于 Swagger 展示单条响应。
type SkillResponseDoc struct {
	ErrorMsg string            `json:"errorMsg"`
	Data     SkillDataResponse `json:"data"`
}

// SkillDeleteResponseDoc 用于 Swagger 展示删除响应。
type SkillDeleteResponseDoc struct {
	ErrorMsg string                  `json:"errorMsg"`
	Data     SkillDeleteDataResponse `json:"data"`
}

// ErrorDoc 表示错误响应。
type ErrorDoc struct {
	ErrorMsg string `json:"errorMsg"`
}

func toSkillSummaryResponses(items []domainskill.Skill) []SkillSummaryResponse {
	results := make([]SkillSummaryResponse, 0, len(items))
	for _, item := range items {
		results = append(results, toSkillSummaryResponse(item))
	}
	return results
}

func toSkillSummaryResponse(item domainskill.Skill) SkillSummaryResponse {
	return SkillSummaryResponse{
		ID:          item.ID,
		Scope:       item.Scope,
		Title:       item.Title,
		Trigger:     item.Trigger,
		Description: item.Description,
		Enabled:     item.Enabled,
		SortOrder:   item.SortOrder,
		CreatedAt:   item.CreatedAt,
		UpdatedAt:   item.UpdatedAt,
	}
}

func toSkillResponses(items []domainskill.Skill) []SkillResponse {
	results := make([]SkillResponse, 0, len(items))
	for _, item := range items {
		results = append(results, toSkillResponse(item))
	}
	return results
}

func toSkillResponse(item domainskill.Skill) SkillResponse {
	return SkillResponse{
		ID:              item.ID,
		Scope:           item.Scope,
		Title:           item.Title,
		Trigger:         item.Trigger,
		Description:     item.Description,
		Markdown:        item.Markdown,
		Enabled:         item.Enabled,
		SortOrder:       item.SortOrder,
		CreatedByUserID: item.CreatedByUserID,
		UpdatedByUserID: item.UpdatedByUserID,
		CreatedAt:       item.CreatedAt,
		UpdatedAt:       item.UpdatedAt,
	}
}
