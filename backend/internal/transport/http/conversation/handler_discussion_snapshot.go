package conversation

import (
	"errors"
	"net/http"
	"strings"

	appconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/conversation"
	domaindiscussion "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/discussion"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/middleware"
	"github.com/gin-gonic/gin"
)

type DiscussionSnapshotRequest struct {
	Models                 map[string]string `json:"models,omitempty"`
	ExpectedPendingVersion int64             `json:"expectedPendingVersion" binding:"min=0"`
}

func (h *Handler) discussionSnapshotConversation(c *gin.Context) (uint, uint, string, bool) {
	userID := middleware.MustUserID(c)
	conversation, err := h.service.GetConversationByPublicID(c.Request.Context(), userID, strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.ErrorFrom(c, http.StatusNotFound, appconversation.ErrConversationNotFound)
		return 0, 0, "", false
	}
	return userID, conversation.ID, strings.TrimSpace(c.Param("templateID")), true
}

func writeDiscussionSnapshotError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, appconversation.ErrDiscussionSnapshotConflict), errors.Is(err, domaindiscussion.ErrSnapshotConflict):
		response.ErrorFrom(c, http.StatusConflict, appconversation.ErrDiscussionSnapshotConflict)
	case errors.Is(err, appconversation.ErrDiscussionPendingNotFound), errors.Is(err, domaindiscussion.ErrPendingNotFound):
		response.ErrorFrom(c, http.StatusNotFound, appconversation.ErrDiscussionPendingNotFound)
	case errors.Is(err, domaindiscussion.ErrTemplateNotFound):
		response.ErrorFrom(c, http.StatusNotFound, err)
	case errors.Is(err, domaindiscussion.ErrTemplateInvalid):
		response.ErrorFrom(c, http.StatusBadRequest, err)
	default:
		response.InternalError(c, err)
	}
}

func (h *Handler) GetDiscussionSnapshot(c *gin.Context) {
	userID, conversationID, templateID, ok := h.discussionSnapshotConversation(c)
	if !ok {
		return
	}
	snapshot, err := h.service.GetDiscussionSnapshot(c.Request.Context(), userID, conversationID, templateID)
	if err != nil {
		writeDiscussionSnapshotError(c, err)
		return
	}
	if snapshot == nil {
		snapshot = &domaindiscussion.ConversationSnapshot{ConversationID: conversationID, TemplatePublicID: templateID}
	}
	response.Success(c, appconversation.DiscussionSnapshotPayloadOf(snapshot))
}

func (h *Handler) RefreshDiscussionSnapshot(c *gin.Context) {
	userID, conversationID, templateID, ok := h.discussionSnapshotConversation(c)
	if !ok {
		return
	}
	var req DiscussionSnapshotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	snapshot, err := h.service.RefreshDiscussionSnapshot(c.Request.Context(), userID, conversationID, templateID, req.Models, req.ExpectedPendingVersion)
	if err != nil {
		writeDiscussionSnapshotError(c, err)
		return
	}
	response.Success(c, appconversation.DiscussionSnapshotPayloadOf(snapshot))
}

func (h *Handler) RollbackDiscussionSnapshot(c *gin.Context) {
	userID, conversationID, templateID, ok := h.discussionSnapshotConversation(c)
	if !ok {
		return
	}
	var req DiscussionSnapshotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	snapshot, err := h.service.RollbackDiscussionSnapshot(c.Request.Context(), userID, conversationID, templateID, req.ExpectedPendingVersion)
	if err != nil {
		writeDiscussionSnapshotError(c, err)
		return
	}
	response.Success(c, appconversation.DiscussionSnapshotPayloadOf(snapshot))
}

func (h *Handler) AdoptDiscussionSnapshot(c *gin.Context) {
	userID, conversationID, templateID, ok := h.discussionSnapshotConversation(c)
	if !ok {
		return
	}
	var req DiscussionSnapshotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	snapshot, err := h.service.AdoptDiscussionSnapshot(c.Request.Context(), userID, conversationID, templateID, req.ExpectedPendingVersion)
	if err != nil {
		writeDiscussionSnapshotError(c, err)
		return
	}
	response.Success(c, appconversation.DiscussionSnapshotPayloadOf(snapshot))
}
