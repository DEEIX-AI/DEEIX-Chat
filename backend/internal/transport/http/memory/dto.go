package memory

import (
	"time"

	domainmemory "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/memory"
)

// ── 请求 DTO ─────────────────────────────────────────────────────────────────

// UpsertUserMemoryRequest 用户记忆更新请求。
type UpsertUserMemoryRequest struct {
	MemoryKey string `json:"memoryKey" binding:"required,max=128"`
	Value     string `json:"value" binding:"required,max=10000"`
	Scope     string `json:"scope" binding:"required,oneof=profile preference custom"`
}

// ── 响应 DTO ─────────────────────────────────────────────────────────────────

// UserMemoryResponse 用户长期记忆响应。
type UserMemoryResponse struct {
	ID        uint      `json:"id"`
	UserID    uint      `json:"userID"`
	MemoryKey string    `json:"memoryKey"`
	Value     string    `json:"value"`
	Scope     string    `json:"scope"`
	UpdatedBy string    `json:"updatedBy"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// UpsertMemoryResponse 写入记忆响应。
type UpsertMemoryResponse struct {
	Saved bool `json:"saved"`
}

// ── Swagger 文档 DTO ─────────────────────────────────────────────────────────

// UserMemoryListResponseDoc 用户长期记忆响应。
type UserMemoryListResponseDoc struct {
	ErrorMsg string               `json:"errorMsg"`
	Data     []UserMemoryResponse `json:"data"`
}

// UpsertUserMemoryResponseDoc 写入用户记忆响应。
type UpsertUserMemoryResponseDoc struct {
	ErrorMsg string               `json:"errorMsg"`
	Data     UpsertMemoryResponse `json:"data"`
}

// ErrorDoc 错误响应。
type ErrorDoc struct {
	ErrorMsg  string      `json:"errorMsg"`
	ErrorCode string      `json:"errorCode,omitempty"`
	Details   interface{} `json:"details,omitempty"`
	RequestID string      `json:"requestId,omitempty"`
	Data      interface{} `json:"data"`
}

// ── mapping 函数 ─────────────────────────────────────────────────────────────

func toUserMemoryResponse(m domainmemory.UserMemory) UserMemoryResponse {
	return UserMemoryResponse{
		ID:        m.ID,
		UserID:    m.UserID,
		MemoryKey: m.MemoryKey,
		Value:     m.Value,
		Scope:     m.Scope,
		UpdatedBy: m.UpdatedBy,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}
}
