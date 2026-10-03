package channel

import (
	"errors"
	"net/http"

	appchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
)

// ---------------------------------------------------------------------------
// models.dev 推理目录
// ---------------------------------------------------------------------------

// GetModelCatalog godoc
// @Summary 管理员查询 models.dev 推理目录状态
// @Description 返回目录数据来源（远端同步或内置快照）、拉取时间、条目数、是否正在同步与最近一次同步错误；目录过期时在后台发起一次同步
// @Tags llm
// @Produce json
// @Security BearerAuth
// @Success 200 {object} ModelCatalogStatusResponseDoc
// @Router /admin/llm/model-catalog [get]
func (h *Handler) GetModelCatalog(c *gin.Context) {
	// 与 OpenRouter 官方定价一致按需同步：管理员查看状态时目录已过期，就在后台同步一次。
	h.service.RefreshReasoningCatalogIfStale(c.Request.Context())
	response.Success(c, toModelCatalogStatusResponse(h.service.ReasoningCatalogStatus()))
}

// RefreshModelCatalog godoc
// @Summary 管理员立即同步 models.dev 推理目录
// @Description 拉取最新目录并替换当前目录；失败时保留当前目录并记录错误
// @Tags llm
// @Produce json
// @Security BearerAuth
// @Success 200 {object} ModelCatalogStatusResponseDoc
// @Failure 502 {object} ErrorDoc
// @Router /admin/llm/model-catalog/refresh [post]
func (h *Handler) RefreshModelCatalog(c *gin.Context) {
	status, err := h.service.RefreshReasoningCatalog(c.Request.Context())
	if err != nil {
		switch {
		case errors.Is(err, appchannel.ErrReasoningCatalogFetchFailed):
			response.ErrorFrom(c, http.StatusBadGateway, err)
		default:
			response.InternalError(c)
		}
		return
	}
	response.Success(c, toModelCatalogStatusResponse(status))
}
