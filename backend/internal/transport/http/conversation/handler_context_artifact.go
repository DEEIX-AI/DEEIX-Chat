package conversation

import (
	"errors"
	"net/http"
	"strconv"

	appconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/middleware"
	"github.com/gin-gonic/gin"
)

// GetContextArtifact godoc
// @Summary 查询上下文证据详情
// @Description 查询当前用户可访问的上下文证据详情，用于 Prompt Trace 来源查看
// @Tags chat
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "上下文证据 ID"
// @Success 200 {object} ContextArtifactResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /context-artifacts/{id} [get]
func (h *Handler) GetContextArtifact(c *gin.Context) {
	userID := middleware.MustUserID(c)
	rawID, err := stringParam(c, "id")
	if err != nil {
		response.ErrorFrom(c, http.StatusBadRequest, errInvalidContextArtifactID)
		return
	}
	parsedID, err := strconv.ParseUint(rawID, 10, strconv.IntSize)
	if err != nil || parsedID == 0 {
		response.ErrorFrom(c, http.StatusBadRequest, errInvalidContextArtifactID)
		return
	}

	item, err := h.service.GetContextArtifact(c.Request.Context(), userID, uint(parsedID))
	if err != nil {
		if errors.Is(err, appconversation.ErrContextArtifactNotFound) {
			response.ErrorFrom(c, http.StatusNotFound, err)
			return
		}
		response.InternalError(c)
		return
	}
	response.Success(c, toContextArtifactResponse(item))
}
