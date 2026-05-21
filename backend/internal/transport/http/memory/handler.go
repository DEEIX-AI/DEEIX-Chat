package memory

import (
	"net/http"

	appmemory "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/memory"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/middleware"
	"github.com/gin-gonic/gin"
)

// Handler 封装记忆 HTTP 处理。
type Handler struct {
	service *appmemory.Service
}

// NewHandler 创建处理器。
func NewHandler(service *appmemory.Service) *Handler {
	return &Handler{
		service: service,
	}
}

// ListUserMemories godoc
// @Summary 查询用户个性化记忆
// @Description 查询当前用户的长期个性化记忆
// @Tags memory
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} UserMemoryListResponseDoc
// @Failure 500 {object} ErrorDoc
// @Router /memories/profile [get]
// ListUserMemories 查询用户记忆。
func (h *Handler) ListUserMemories(c *gin.Context) {
	userID := middleware.MustUserID(c)
	items, err := h.service.ListUserMemories(c.Request.Context(), userID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "list user memories failed")
		return
	}
	memories := make([]UserMemoryResponse, 0, len(items))
	for _, m := range items {
		memories = append(memories, toUserMemoryResponse(m))
	}
	response.Success(c, memories)
}

// UpsertUserMemory godoc
// @Summary 更新用户个性化记忆
// @Description 新增或更新当前用户的长期个性化记忆
// @Tags memory
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body UpsertUserMemoryRequest true "用户记忆参数"
// @Success 200 {object} UpsertUserMemoryResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /memories/profile [put]
// UpsertUserMemory 写入用户记忆。
func (h *Handler) UpsertUserMemory(c *gin.Context) {
	userID := middleware.MustUserID(c)

	var req UpsertUserMemoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}

	if err := h.service.UpsertUserMemory(
		c.Request.Context(),
		userID,
		req.MemoryKey,
		req.Value,
		req.Scope,
		"user",
	); err != nil {
		response.Error(c, http.StatusInternalServerError, "upsert user memory failed")
		return
	}

	h.service.RecordAudit(c.Request.Context(), appmemory.AuditInput{
		UserID:    userID,
		RequestID: middleware.MustRequestID(c),
		Action:    "upsert_user_memory",
		MemoryKey: req.MemoryKey,
		ClientIP:  c.ClientIP(),
		UserAgent: c.Request.UserAgent(),
		Detail:    map[string]string{"scope": req.Scope},
	})

	response.Success(c, UpsertMemoryResponse{Saved: true})
}

// DeleteUserMemory godoc
// @Summary 删除用户个性化记忆
// @Description 删除当前用户的指定 key 长期记忆
// @Tags memory
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param memory_key path string true "记忆 Key"
// @Success 200 {object} UpsertUserMemoryResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /memories/profile/{memory_key} [delete]
// DeleteUserMemory 删除用户记忆。
func (h *Handler) DeleteUserMemory(c *gin.Context) {
	userID := middleware.MustUserID(c)
	memoryKey := c.Param("memory_key")
	if memoryKey == "" {
		response.Error(c, http.StatusBadRequest, "memory_key is required")
		return
	}

	if err := h.service.DeleteUserMemory(c.Request.Context(), userID, memoryKey); err != nil {
		response.Error(c, http.StatusInternalServerError, "delete user memory failed")
		return
	}

	h.service.RecordAudit(c.Request.Context(), appmemory.AuditInput{
		UserID:    userID,
		RequestID: middleware.MustRequestID(c),
		Action:    "delete_user_memory",
		MemoryKey: memoryKey,
		ClientIP:  c.ClientIP(),
		UserAgent: c.Request.UserAgent(),
	})

	response.Success(c, UpsertMemoryResponse{Saved: true})
}
