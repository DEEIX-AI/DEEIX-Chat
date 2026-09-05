package conversation

import (
	"errors"
	"io"
	"net/http"

	appconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/filecontent"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/middleware"
	"github.com/gin-gonic/gin"
)

// GetConversationShare godoc
// @Summary 查询会话分享状态
// @Description 查询当前用户指定会话的最近分享状态
// @Tags chat
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "会话 public_id"
// @Success 200 {object} ConversationShareResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /conversations/{id}/share [get]
func (h *Handler) GetConversationShare(c *gin.Context) {
	userID := middleware.MustUserID(c)
	publicID, err := stringParam(c, "id")
	if err != nil {
		response.ErrorFrom(c, http.StatusBadRequest, errInvalidConversationID)
		return
	}
	result, err := h.service.GetConversationShare(c.Request.Context(), userID, publicID)
	if err != nil {
		if errors.Is(err, appconversation.ErrConversationNotFound) {
			response.ErrorFrom(c, http.StatusNotFound, err)
			return
		}
		response.InternalError(c)
		return
	}
	response.Success(c, toConversationShareResponse(result))
}

// CreateConversationShare godoc
// @Summary 创建会话公开分享
// @Description 创建当前会话全部分支的公开快照分享链接
// @Tags chat
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "会话 public_id"
// @Param body body CreateConversationShareRequest false "分享参数"
// @Success 200 {object} ConversationShareResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /conversations/{id}/share [post]
func (h *Handler) CreateConversationShare(c *gin.Context) {
	userID := middleware.MustUserID(c)
	publicID, err := stringParam(c, "id")
	if err != nil {
		response.ErrorFrom(c, http.StatusBadRequest, errInvalidConversationID)
		return
	}
	var req CreateConversationShareRequest
	if c.Request.Body != nil {
		if err = c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
			response.InvalidRequestBody(c, err)
			return
		}
	}
	result, err := h.service.CreateConversationShare(c.Request.Context(), userID, publicID, req.DefaultMessagePublicIDs)
	if err != nil {
		writeConversationShareError(c, err)
		return
	}
	h.recordAudit(c, "create_conversation_share",
		"conversation",
		publicID,
		map[string]string{"share_id": result.ShareID},
	)
	response.Success(c, toConversationShareResponse(result))
}

// RegenerateConversationShare godoc
// @Summary 重新生成会话分享链接
// @Description 关闭当前有效分享并创建新的公开快照链接
// @Tags chat
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "会话 public_id"
// @Param body body CreateConversationShareRequest false "分享参数"
// @Success 200 {object} ConversationShareResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /conversations/{id}/share/regenerate [post]
func (h *Handler) RegenerateConversationShare(c *gin.Context) {
	userID := middleware.MustUserID(c)
	publicID, err := stringParam(c, "id")
	if err != nil {
		response.ErrorFrom(c, http.StatusBadRequest, errInvalidConversationID)
		return
	}
	var req CreateConversationShareRequest
	if c.Request.Body != nil {
		if err = c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
			response.InvalidRequestBody(c, err)
			return
		}
	}
	result, err := h.service.RegenerateConversationShare(c.Request.Context(), userID, publicID, req.DefaultMessagePublicIDs)
	if err != nil {
		writeConversationShareError(c, err)
		return
	}
	h.recordAudit(c, "regenerate_conversation_share",
		"conversation",
		publicID,
		map[string]string{"share_id": result.ShareID},
	)
	response.Success(c, toConversationShareResponse(result))
}

// RevokeConversationShare godoc
// @Summary 关闭会话公开分享
// @Description 关闭当前会话的有效公开分享链接
// @Tags chat
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "会话 public_id"
// @Success 200 {object} ConversationShareResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /conversations/{id}/share [delete]
func (h *Handler) RevokeConversationShare(c *gin.Context) {
	userID := middleware.MustUserID(c)
	publicID, err := stringParam(c, "id")
	if err != nil {
		response.ErrorFrom(c, http.StatusBadRequest, errInvalidConversationID)
		return
	}
	result, err := h.service.RevokeConversationShare(c.Request.Context(), userID, publicID)
	if err != nil {
		writeConversationShareError(c, err)
		return
	}
	h.recordAudit(c, "revoke_conversation_share",
		"conversation",
		publicID,
		map[string]bool{"revoked": true},
	)
	response.Success(c, toConversationShareResponse(result))
}

// RevokeConversationShares godoc
// @Summary 批量关闭会话公开分享
// @Description 批量关闭当前用户会话的公开分享链接
// @Tags chat
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body RevokeConversationSharesRequest true "批量关闭参数"
// @Success 200 {object} RevokeConversationSharesResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /conversations/shares/revoke [post]
func (h *Handler) RevokeConversationShares(c *gin.Context) {
	userID := middleware.MustUserID(c)
	var req RevokeConversationSharesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	if err := h.service.RevokeConversationShares(c.Request.Context(), userID, req.ConversationPublicIDs); err != nil {
		writeConversationShareError(c, err)
		return
	}
	h.recordAudit(c, "revoke_conversation_shares",
		"conversation",
		"",
		map[string]int{"count": len(req.ConversationPublicIDs)},
	)
	response.Success(c, RevokeConversationSharesResponse{Revoked: true})
}

// GetPublicSharedConversation godoc
// @Summary 查询公开分享会话
// @Description 公开读取会话分享快照
// @Tags chat
// @Accept json
// @Produce json
// @Param share_id path string true "分享 ID"
// @Success 200 {object} PublicSharedConversationResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /shared-conversations/{share_id} [get]
func (h *Handler) GetPublicSharedConversation(c *gin.Context) {
	shareID, err := stringParam(c, "share_id")
	if err != nil {
		response.ErrorFrom(c, http.StatusBadRequest, errInvalidShareID)
		return
	}
	result, err := h.service.GetPublicSharedConversation(c.Request.Context(), shareID)
	if err != nil {
		if errors.Is(err, appconversation.ErrConversationShareNotFound) {
			response.ErrorFrom(c, http.StatusNotFound, err)
			return
		}
		response.InternalError(c)
		return
	}
	response.Success(c, toPublicSharedConversationResponse(result))
}

// CloneSharedConversation godoc
// @Summary 克隆公开分享会话
// @Description 将公开分享快照克隆到当前登录用户账户，包含全部分支消息和分享内附件
// @Tags chat
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param share_id path string true "分享 ID"
// @Success 200 {object} ConversationUpdateResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 401 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /shared-conversations/{share_id}/clone [post]
func (h *Handler) CloneSharedConversation(c *gin.Context) {
	userID := middleware.MustUserID(c)
	shareID, err := stringParam(c, "share_id")
	if err != nil {
		response.ErrorFrom(c, http.StatusBadRequest, errInvalidShareID)
		return
	}
	result, err := h.service.CloneSharedConversation(c.Request.Context(), userID, shareID)
	if err != nil {
		switch {
		case errors.Is(err, appconversation.ErrConversationShareNotFound):
			response.ErrorFrom(c, http.StatusNotFound, err)
		case errors.Is(err, appconversation.ErrFileNotFound):
			response.ErrorFrom(c, http.StatusNotFound, errSharedFileNotFound)
		case errors.Is(err, appconversation.ErrStorageQuotaExceeded):
			response.ErrorFrom(c, http.StatusBadRequest, errStorageQuotaExceeded)
		default:
			response.InternalError(c)
		}
		return
	}
	h.recordAudit(c, "clone_shared_conversation",
		"conversation",
		result.PublicID,
		map[string]string{"share_id": shareID},
	)
	response.Success(c, toConversationResponse(result))
}

// GetPublicSharedFileContent godoc
// @Summary 获取公开分享附件内容
// @Description 只允许读取公开分享快照中实际引用的附件内容
// @Tags chat
// @Produce application/octet-stream
// @Param share_id path string true "分享 ID"
// @Param file_id path string true "文件 ID"
// @Success 200 {file} binary
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /shared-conversations/{share_id}/files/{file_id}/content [get]
func (h *Handler) GetPublicSharedFileContent(c *gin.Context) {
	shareID, err := stringParam(c, "share_id")
	if err != nil {
		response.ErrorFrom(c, http.StatusBadRequest, errInvalidShareID)
		return
	}
	fileID := c.Param("file_id")
	result, err := h.service.OpenSharedConversationFileContent(c.Request.Context(), shareID, fileID)
	if err != nil {
		switch {
		case errors.Is(err, appconversation.ErrConversationShareNotFound):
			response.ErrorFrom(c, http.StatusNotFound, err)
			return
		case errors.Is(err, appconversation.ErrInvalidFileReference):
			response.ErrorFrom(c, http.StatusBadRequest, errInvalidFileID)
			return
		case errors.Is(err, appconversation.ErrFileNotFound):
			response.ErrorFrom(c, http.StatusNotFound, appconversation.ErrFileNotFound)
			return
		default:
			response.InternalError(c)
			return
		}
	}
	_ = filecontent.Write(c, result, true)
}

func writeConversationShareError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, appconversation.ErrConversationNotFound):
		response.ErrorFrom(c, http.StatusNotFound, err)
	case errors.Is(err, appconversation.ErrConversationShareNotFound):
		response.ErrorFrom(c, http.StatusNotFound, err)
	case errors.Is(err, appconversation.ErrInvalidConversationShare):
		response.ErrorFrom(c, http.StatusBadRequest, err)
	case errors.Is(err, appconversation.ErrConversationShareSchemaOutdated):
		response.ErrorFrom(c, http.StatusInternalServerError, err)
	default:
		response.InternalError(c)
	}
}
