package conversation

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	appacl "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/acl"
	appconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/conversation"
	domainacl "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/acl"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/middleware"
	"github.com/gin-gonic/gin"
)

// GrantConversationACLRequest 授权请求。
type GrantConversationACLRequest struct {
	Username string `json:"username" binding:"required,max=64"`
	Role     string `json:"role" binding:"required,oneof=viewer editor"`
}

// ConversationACLEntryResponse ACL 条目响应。
type ConversationACLEntryResponse struct {
	GranteeUserID   uint      `json:"granteeUserID"`
	GranteeUsername string    `json:"granteeUsername"`
	Role            string    `json:"role"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

// ListConversationACL godoc
// @Summary 列出会话 ACL
// @Tags chat
// @Security BearerAuth
// @Param id path string true "conversation public id"
// @Success 200 {object} ConversationACLListResponseDoc
// @Router /conversations/{id}/acl [get]
func (h *Handler) ListConversationACL(c *gin.Context) {
	publicID, err := stringParam(c, "id")
	if err != nil {
		response.ErrorFrom(c, http.StatusBadRequest, errInvalidConversationID)
		return
	}
	items, err := h.service.ListConversationACL(c.Request.Context(), middleware.MustUserID(c), publicID)
	if err != nil {
		handleConversationACLError(c, err)
		return
	}
	views := make([]ConversationACLEntryResponse, 0, len(items))
	for _, item := range items {
		views = append(views, toConversationACLEntryResponse(item))
	}
	response.Success(c, views)
}

// GrantConversationACL godoc
// @Summary 授予会话 ACL
// @Tags chat
// @Security BearerAuth
// @Param id path string true "conversation public id"
// @Param body body GrantConversationACLRequest true "grant"
// @Success 200 {object} ConversationACLEntryResponseDoc
// @Router /conversations/{id}/acl [put]
func (h *Handler) GrantConversationACL(c *gin.Context) {
	publicID, err := stringParam(c, "id")
	if err != nil {
		response.ErrorFrom(c, http.StatusBadRequest, errInvalidConversationID)
		return
	}
	var req GrantConversationACLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	item, err := h.service.GrantConversationACL(c.Request.Context(), middleware.MustUserID(c), publicID, req.Username, req.Role)
	if err != nil {
		handleConversationACLError(c, err)
		return
	}
	response.Success(c, toConversationACLEntryResponse(*item))
}

// RevokeConversationACL godoc
// @Summary 撤销会话 ACL
// @Tags chat
// @Security BearerAuth
// @Param id path string true "conversation public id"
// @Param grantee_user_id path int true "grantee user id"
// @Success 200 {object} map[string]any
// @Router /conversations/{id}/acl/{grantee_user_id} [delete]
func (h *Handler) RevokeConversationACL(c *gin.Context) {
	publicID, err := stringParam(c, "id")
	if err != nil {
		response.ErrorFrom(c, http.StatusBadRequest, errInvalidConversationID)
		return
	}
	granteeRaw := c.Param("grantee_user_id")
	granteeUserID, parseErr := strconv.ParseUint(granteeRaw, 10, 64)
	if parseErr != nil || granteeUserID == 0 {
		response.ErrorFrom(c, http.StatusBadRequest, appacl.ErrInvalidUsername)
		return
	}
	if err := h.service.RevokeConversationACL(c.Request.Context(), middleware.MustUserID(c), publicID, uint(granteeUserID)); err != nil {
		handleConversationACLError(c, err)
		return
	}
	response.Success(c, map[string]bool{"revoked": true})
}

func handleConversationACLError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, appconversation.ErrConversationNotFound):
		response.ErrorFrom(c, http.StatusNotFound, err)
	case errors.Is(err, appacl.ErrDisabled):
		response.ErrorFrom(c, http.StatusForbidden, err)
	case errors.Is(err, appacl.ErrUserNotFound), errors.Is(err, appacl.ErrInvalidUsername),
		errors.Is(err, appacl.ErrInvalidRole), errors.Is(err, appacl.ErrCannotShareSelf),
		errors.Is(err, appacl.ErrInvalidResource):
		response.ErrorFrom(c, http.StatusBadRequest, err)
	case errors.Is(err, appacl.ErrForbidden), errors.Is(err, appacl.ErrEntryNotFound):
		response.ErrorFrom(c, http.StatusForbidden, err)
	default:
		response.InternalError(c)
	}
}

func toConversationACLEntryResponse(item domainacl.Entry) ConversationACLEntryResponse {
	return ConversationACLEntryResponse{
		GranteeUserID:   item.GranteeUserID,
		GranteeUsername: item.GranteeUsername,
		Role:            item.Role,
		CreatedAt:       item.CreatedAt,
		UpdatedAt:       item.UpdatedAt,
	}
}

// ConversationACLListResponseDoc swagger.
type ConversationACLListResponseDoc struct {
	ErrorMsg string                         `json:"errorMsg"`
	Data     []ConversationACLEntryResponse `json:"data"`
}

// ConversationACLEntryResponseDoc swagger.
type ConversationACLEntryResponseDoc struct {
	ErrorMsg string                       `json:"errorMsg"`
	Data     ConversationACLEntryResponse `json:"data"`
}
