package uicomponent

import (
	"time"

	domainuicomponent "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/uicomponent"
)

// UIComponentResponse 表示组件响应。可见目录也返回渲染源，前端据此渲染自定义组件。
type UIComponentResponse struct {
	ID              uint      `json:"id"`
	Scope           string    `json:"scope"`
	Name            string    `json:"name"`
	Version         int       `json:"version"`
	Description     string    `json:"description"`
	PropsSummary    string    `json:"propsSummary"`
	PropsSchema     string    `json:"propsSchema"`
	RendererKind    string    `json:"rendererKind"`
	RendererSource  string    `json:"rendererSource"`
	Enabled         bool      `json:"enabled"`
	SortOrder       int       `json:"sortOrder"`
	CreatedByUserID uint      `json:"createdByUserID"`
	UpdatedByUserID uint      `json:"updatedByUserID"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

// UIComponentDataResponse 包裹单条组件响应。
type UIComponentDataResponse struct {
	Component UIComponentResponse `json:"component"`
}

// UIComponentDeleteDataResponse 表示删除响应。
type UIComponentDeleteDataResponse struct {
	Deleted bool `json:"deleted"`
}

// WriteUIComponentRequest 表示创建组件请求。
type WriteUIComponentRequest struct {
	Name           string `json:"name" binding:"required,max=64"`
	Version        int    `json:"version,omitempty" binding:"omitempty,min=1"`
	Description    string `json:"description" binding:"required,max=256"`
	PropsSummary   string `json:"propsSummary" binding:"required,max=1024"`
	PropsSchema    string `json:"propsSchema,omitempty" binding:"max=16384"`
	RendererSource string `json:"rendererSource" binding:"required,max=262144"`
	Enabled        bool   `json:"enabled,omitempty"`
	SortOrder      int    `json:"sortOrder,omitempty"`
}

// PatchUIComponentRequest 表示更新组件请求。
type PatchUIComponentRequest struct {
	Name           *string `json:"name,omitempty" binding:"omitempty,max=64"`
	Version        *int    `json:"version,omitempty" binding:"omitempty,min=1"`
	Description    *string `json:"description,omitempty" binding:"omitempty,max=256"`
	PropsSummary   *string `json:"propsSummary,omitempty" binding:"omitempty,max=1024"`
	PropsSchema    *string `json:"propsSchema,omitempty" binding:"omitempty,max=16384"`
	RendererSource *string `json:"rendererSource,omitempty" binding:"omitempty,max=262144"`
	Enabled        *bool   `json:"enabled,omitempty"`
	SortOrder      *int    `json:"sortOrder,omitempty"`
}

// UIComponentPageResponseDoc 用于 Swagger 展示分页响应。
type UIComponentPageResponseDoc struct {
	ErrorMsg string `json:"errorMsg"`
	Data     struct {
		Total   int64                 `json:"total"`
		Results []UIComponentResponse `json:"results"`
	} `json:"data"`
}

// UIComponentResponseDoc 用于 Swagger 展示单条响应。
type UIComponentResponseDoc struct {
	ErrorMsg string                  `json:"errorMsg"`
	Data     UIComponentDataResponse `json:"data"`
}

// UIComponentDeleteResponseDoc 用于 Swagger 展示删除响应。
type UIComponentDeleteResponseDoc struct {
	ErrorMsg string                        `json:"errorMsg"`
	Data     UIComponentDeleteDataResponse `json:"data"`
}

// ErrorDoc 表示错误响应。
type ErrorDoc struct {
	ErrorMsg string `json:"errorMsg"`
}

func toResponses(items []domainuicomponent.Component) []UIComponentResponse {
	results := make([]UIComponentResponse, 0, len(items))
	for _, item := range items {
		results = append(results, toResponse(item))
	}
	return results
}

func toResponse(item domainuicomponent.Component) UIComponentResponse {
	return UIComponentResponse{
		ID:              item.ID,
		Scope:           item.Scope,
		Name:            item.Name,
		Version:         item.Version,
		Description:     item.Description,
		PropsSummary:    item.PropsSummary,
		PropsSchema:     item.PropsSchema,
		RendererKind:    item.RendererKind,
		RendererSource:  item.RendererSource,
		Enabled:         item.Enabled,
		SortOrder:       item.SortOrder,
		CreatedByUserID: item.CreatedByUserID,
		UpdatedByUserID: item.UpdatedByUserID,
		CreatedAt:       item.CreatedAt,
		UpdatedAt:       item.UpdatedAt,
	}
}
