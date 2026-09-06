package knowledgebase

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	appacl "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/acl"
	appknowledgebase "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/knowledgebase"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/middleware"
	"github.com/gin-gonic/gin"
)

// GrantKnowledgeBaseACLRequest 授权请求。
type GrantKnowledgeBaseACLRequest struct {
	Username string `json:"username" binding:"required,max=64"`
	Role     string `json:"role" binding:"required,oneof=viewer editor"`
}

// KnowledgeBaseACLEntryResponse ACL 条目。
type KnowledgeBaseACLEntryResponse struct {
	GranteeUserID   uint      `json:"granteeUserID"`
	GranteeUsername string    `json:"granteeUsername"`
	Role            string    `json:"role"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

// ListKnowledgeBaseACL godoc
// @Summary 列出知识库 ACL
// @Tags knowledge-bases
// @Security BearerAuth
// @Param id path string true "knowledge base public id"
// @Success 200 {object} map[string]any
// @Router /knowledge-bases/mine/{id}/acl [get]
func (h *Handler) ListKnowledgeBaseACL(c *gin.Context) {
	items, err := h.service.ListKnowledgeBaseACL(c.Request.Context(), middleware.MustUserID(c), c.Param("id"))
	if err != nil {
		handleKnowledgeBaseACLError(c, err)
		return
	}
	views := make([]KnowledgeBaseACLEntryResponse, 0, len(items))
	for _, item := range items {
		views = append(views, KnowledgeBaseACLEntryResponse{
			GranteeUserID:   item.GranteeUserID,
			GranteeUsername: item.GranteeUsername,
			Role:            item.Role,
			CreatedAt:       item.CreatedAt,
			UpdatedAt:       item.UpdatedAt,
		})
	}
	response.Success(c, views)
}

// GrantKnowledgeBaseACL godoc
// @Summary 授予知识库 ACL
// @Tags knowledge-bases
// @Security BearerAuth
// @Param id path string true "knowledge base public id"
// @Param body body GrantKnowledgeBaseACLRequest true "grant"
// @Success 200 {object} map[string]any
// @Router /knowledge-bases/mine/{id}/acl [put]
func (h *Handler) GrantKnowledgeBaseACL(c *gin.Context) {
	var req GrantKnowledgeBaseACLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	item, err := h.service.GrantKnowledgeBaseACL(c.Request.Context(), middleware.MustUserID(c), c.Param("id"), req.Username, req.Role)
	if err != nil {
		handleKnowledgeBaseACLError(c, err)
		return
	}
	response.Success(c, KnowledgeBaseACLEntryResponse{
		GranteeUserID:   item.GranteeUserID,
		GranteeUsername: item.GranteeUsername,
		Role:            item.Role,
		CreatedAt:       item.CreatedAt,
		UpdatedAt:       item.UpdatedAt,
	})
}

// RevokeKnowledgeBaseACL godoc
// @Summary 撤销知识库 ACL
// @Tags knowledge-bases
// @Security BearerAuth
// @Param id path string true "knowledge base public id"
// @Param grantee_user_id path int true "grantee user id"
// @Success 200 {object} map[string]any
// @Router /knowledge-bases/mine/{id}/acl/{grantee_user_id} [delete]
func (h *Handler) RevokeKnowledgeBaseACL(c *gin.Context) {
	granteeUserID, err := strconv.ParseUint(c.Param("grantee_user_id"), 10, 64)
	if err != nil || granteeUserID == 0 {
		response.ErrorFrom(c, http.StatusBadRequest, appacl.ErrInvalidUsername)
		return
	}
	if err := h.service.RevokeKnowledgeBaseACL(c.Request.Context(), middleware.MustUserID(c), c.Param("id"), uint(granteeUserID)); err != nil {
		handleKnowledgeBaseACLError(c, err)
		return
	}
	response.Success(c, map[string]bool{"revoked": true})
}

func handleKnowledgeBaseACLError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, appknowledgebase.ErrKnowledgeBaseNotFound):
		response.ErrorFrom(c, http.StatusNotFound, err)
	case errors.Is(err, appacl.ErrDisabled), errors.Is(err, appacl.ErrForbidden):
		response.ErrorFrom(c, http.StatusForbidden, err)
	case errors.Is(err, appacl.ErrUserNotFound), errors.Is(err, appacl.ErrInvalidUsername),
		errors.Is(err, appacl.ErrInvalidRole), errors.Is(err, appacl.ErrCannotShareSelf),
		errors.Is(err, appacl.ErrEntryNotFound), errors.Is(err, appacl.ErrInvalidResource):
		response.ErrorFrom(c, http.StatusBadRequest, err)
	default:
		response.InternalError(c)
	}
}
