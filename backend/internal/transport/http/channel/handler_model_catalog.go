package channel

import (
	"errors"
	"net/http"

	appchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
)

// ---------------------------------------------------------------------------
// models.dev 模型目录
// ---------------------------------------------------------------------------

// GetModelCatalog godoc
// @Summary 管理员查询 models.dev 模型目录状态
// @Description 返回目录数据来源（远端同步或内置快照）、拉取时间、条目数、是否正在同步与最近一次同步错误；目录过期时在后台发起一次同步
// @Tags llm
// @Produce json
// @Security BearerAuth
// @Success 200 {object} ModelCatalogStatusResponseDoc
// @Router /admin/llm/model-catalog [get]
func (h *Handler) GetModelCatalog(c *gin.Context) {
	// 与 OpenRouter 官方定价一致按需同步：管理员查看状态时目录已过期，就在后台同步一次。
	h.service.RefreshModelCatalogIfStale(c.Request.Context())
	response.Success(c, toModelCatalogStatusResponse(h.service.ModelCatalogStatus()))
}

// ResolveModelCatalog godoc
// @Summary 管理员查询模型编辑表单的自动识别结果
// @Description 按表单中的平台模型名、技术厂商、路由协议与能力 JSON，返回 models.dev 目录中的输入/输出模态与上下文窗口、自动识别的推理强度，以及自定义推理强度的编辑模板；目录过期时在后台发起一次同步
// @Tags llm
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body ResolveModelCatalogRequest true "表单当前值"
// @Success 200 {object} ModelCatalogResolveResponseDoc
// @Failure 400 {object} ErrorDoc
// @Router /admin/llm/model-catalog/resolve [post]
func (h *Handler) ResolveModelCatalog(c *gin.Context) {
	var req ResolveModelCatalogRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	resolution := h.service.ResolveModelCatalog(c.Request.Context(), appchannel.ModelCatalogResolveInput{
		Name:             req.Name,
		Vendor:           req.Vendor,
		Protocols:        req.Protocols,
		CapabilitiesJSON: req.CapabilitiesJSON,
	})
	response.Success(c, toModelCatalogResolveResponse(resolution))
}

// RefreshModelCatalog godoc
// @Summary 管理员立即同步 models.dev 模型目录
// @Description 拉取最新目录并替换当前目录；失败时保留当前目录并记录错误
// @Tags llm
// @Produce json
// @Security BearerAuth
// @Success 200 {object} ModelCatalogStatusResponseDoc
// @Failure 502 {object} ErrorDoc
// @Router /admin/llm/model-catalog/refresh [post]
func (h *Handler) RefreshModelCatalog(c *gin.Context) {
	status, err := h.service.RefreshModelCatalog(c.Request.Context())
	if err != nil {
		switch {
		case errors.Is(err, appchannel.ErrModelCatalogFetchFailed):
			response.ErrorFrom(c, http.StatusBadGateway, err)
		default:
			response.InternalError(c)
		}
		return
	}
	response.Success(c, toModelCatalogStatusResponse(status))
}
