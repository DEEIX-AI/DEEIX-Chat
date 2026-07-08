package announcement

import (
	"encoding/json"
	"strings"
	"time"

	appannouncement "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/announcement"
	domainannouncement "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/announcement"
)

// ErrorDoc 用于 Swagger 标注通用错误响应。
type ErrorDoc struct {
	ErrorMsg  string      `json:"errorMsg" example:"invalid request"`
	ErrorCode string      `json:"errorCode,omitempty" example:"invalid_request"`
	Details   interface{} `json:"details,omitempty"`
	RequestID string      `json:"requestId,omitempty" example:""`
	Data      interface{} `json:"data"`
}

// AnnouncementResponse 面向前端的公告响应。
type AnnouncementResponse struct {
	ID              uint       `json:"id"`
	Title           string     `json:"title"`
	ContentMarkdown string     `json:"contentMarkdown"`
	Status          string     `json:"status"`
	Type            string     `json:"type"`
	Pinned          bool       `json:"pinned"`
	Priority        int        `json:"priority"`
	StartsAt        *time.Time `json:"startsAt"`
	ExpiresAt       *time.Time `json:"expiresAt"`
	CreatedByUserID uint       `json:"createdByUserID"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
	ClosedAt        *time.Time `json:"closedAt"`
}

// CreateAnnouncementRequest 创建公告请求。
type CreateAnnouncementRequest struct {
	Title           string     `json:"title" binding:"required,min=1,max=120"`
	ContentMarkdown string     `json:"contentMarkdown" binding:"required,min=1,max=20000"`
	Status          string     `json:"status" binding:"omitempty,oneof=active inactive"`
	Type            string     `json:"type" binding:"omitempty,oneof=critical warning info normal general"`
	Pinned          bool       `json:"pinned"`
	Priority        int        `json:"priority"`
	StartsAt        *time.Time `json:"startsAt"`
	ExpiresAt       *time.Time `json:"expiresAt"`
}

// PatchAnnouncementRequest 更新公告请求。
type PatchAnnouncementRequest struct {
	Title           *string             `json:"title" binding:"omitempty,min=1,max=120"`
	ContentMarkdown *string             `json:"contentMarkdown" binding:"omitempty,min=1,max=20000"`
	Status          *string             `json:"status" binding:"omitempty,oneof=active inactive"`
	Type            *string             `json:"type" binding:"omitempty,oneof=critical warning info normal general"`
	Pinned          *bool               `json:"pinned"`
	Priority        *int                `json:"priority"`
	StartsAt        nullableTimeRequest `json:"startsAt"`
	ExpiresAt       nullableTimeRequest `json:"expiresAt"`
}

// PatchAnnouncementRequestDoc 用于 Swagger 展示 nullable 字段。
type PatchAnnouncementRequestDoc struct {
	Title           *string    `json:"title" maxLength:"120"`
	ContentMarkdown *string    `json:"contentMarkdown" maxLength:"20000"`
	Status          *string    `json:"status" enums:"active,inactive"`
	Type            *string    `json:"type" enums:"critical,warning,info,normal,general"`
	Pinned          *bool      `json:"pinned"`
	Priority        *int       `json:"priority"`
	StartsAt        *time.Time `json:"startsAt"`
	ExpiresAt       *time.Time `json:"expiresAt"`
}

// AnnouncementStateRequest 更新用户公告状态请求。
type AnnouncementStateRequest struct {
	UpdatedAt time.Time `json:"updatedAt" binding:"required"`
}

type GenerateAnnouncementDraftRequest struct {
	Requirement string `json:"requirement" binding:"required,min=1,max=4000"`
	Locale      string `json:"locale" binding:"omitempty,max=32"`
}

type AnnouncementDraftResponse struct {
	Title           string `json:"title"`
	ContentMarkdown string `json:"contentMarkdown"`
	Type            string `json:"type"`
	Pinned          bool   `json:"pinned"`
	Priority        int    `json:"priority"`
}

type nullableTimeRequest struct {
	Set   bool
	Value *time.Time
}

func (v *nullableTimeRequest) UnmarshalJSON(raw []byte) error {
	v.Set = true
	if string(raw) == "null" {
		v.Value = nil
		return nil
	}
	var text string
	if err := json.Unmarshal(raw, &text); err != nil {
		return err
	}
	text = strings.TrimSpace(text)
	if text == "" {
		v.Value = nil
		return nil
	}
	parsed, err := time.Parse(time.RFC3339, text)
	if err != nil {
		return err
	}
	v.Value = &parsed
	return nil
}

// AnnouncementListResponseDoc 公告列表响应文档。
type AnnouncementListResponseDoc struct {
	ErrorMsg string                 `json:"errorMsg"`
	Data     []AnnouncementResponse `json:"data"`
}

// AdminAnnouncementListResponseDoc 后台公告分页响应文档。
type AdminAnnouncementListResponseDoc struct {
	ErrorMsg string `json:"errorMsg"`
	Data     struct {
		Total   int64                  `json:"total"`
		Results []AnnouncementResponse `json:"results"`
	} `json:"data"`
}

// AnnouncementDataResponse 公告操作响应。
type AnnouncementDataResponse struct {
	Announcement AnnouncementResponse `json:"announcement"`
}

type AnnouncementDraftDataResponse struct {
	Draft AnnouncementDraftResponse `json:"draft"`
}

// AnnouncementResponseDoc 公告操作响应文档。
type AnnouncementResponseDoc struct {
	ErrorMsg string                   `json:"errorMsg"`
	Data     AnnouncementDataResponse `json:"data"`
}

type AnnouncementDraftResponseDoc struct {
	ErrorMsg string                        `json:"errorMsg"`
	Data     AnnouncementDraftDataResponse `json:"data"`
}

// AnnouncementDeleteDataResponse 公告删除响应。
type AnnouncementDeleteDataResponse struct {
	Deleted bool `json:"deleted"`
}

// AnnouncementDeleteResponseDoc 公告删除响应文档。
type AnnouncementDeleteResponseDoc struct {
	ErrorMsg string                         `json:"errorMsg"`
	Data     AnnouncementDeleteDataResponse `json:"data"`
}

// AnnouncementDismissDataResponse 公告不再显示响应。
type AnnouncementDismissDataResponse struct {
	Dismissed bool `json:"dismissed"`
}

// AnnouncementDismissResponseDoc 公告不再显示响应文档。
type AnnouncementDismissResponseDoc struct {
	ErrorMsg string                          `json:"errorMsg"`
	Data     AnnouncementDismissDataResponse `json:"data"`
}

// AnnouncementCloseDataResponse 公告关闭响应。
type AnnouncementCloseDataResponse struct {
	Closed bool `json:"closed"`
}

// AnnouncementCloseResponseDoc 公告关闭响应文档。
type AnnouncementCloseResponseDoc struct {
	ErrorMsg string                        `json:"errorMsg"`
	Data     AnnouncementCloseDataResponse `json:"data"`
}

func toAnnouncementResponse(item domainannouncement.Announcement) AnnouncementResponse {
	return AnnouncementResponse{
		ID:              item.ID,
		Title:           item.Title,
		ContentMarkdown: item.ContentMarkdown,
		Status:          item.Status,
		Type:            item.Type,
		Pinned:          item.Pinned,
		Priority:        item.Priority,
		StartsAt:        item.StartsAt,
		ExpiresAt:       item.ExpiresAt,
		CreatedByUserID: item.CreatedByUserID,
		CreatedAt:       item.CreatedAt,
		UpdatedAt:       item.UpdatedAt,
		ClosedAt:        item.ClosedAt,
	}
}

func toAnnouncementResponses(items []domainannouncement.Announcement) []AnnouncementResponse {
	results := make([]AnnouncementResponse, 0, len(items))
	for _, item := range items {
		results = append(results, toAnnouncementResponse(item))
	}
	return results
}

func createInputFromRequest(req CreateAnnouncementRequest) appannouncement.WriteInput {
	status := req.Status
	if strings.TrimSpace(status) == "" {
		status = domainannouncement.StatusActive
	}
	return appannouncement.WriteInput{
		Title:           req.Title,
		ContentMarkdown: req.ContentMarkdown,
		Status:          status,
		Type:            req.Type,
		Pinned:          req.Pinned,
		Priority:        req.Priority,
		StartsAt:        req.StartsAt,
		ExpiresAt:       req.ExpiresAt,
	}
}

func toAnnouncementDraftResponse(item *appannouncement.GenerateDraftResult) AnnouncementDraftResponse {
	if item == nil {
		return AnnouncementDraftResponse{}
	}
	return AnnouncementDraftResponse{
		Title:           item.Title,
		ContentMarkdown: item.ContentMarkdown,
		Type:            item.Type,
		Pinned:          item.Pinned,
		Priority:        item.Priority,
	}
}
