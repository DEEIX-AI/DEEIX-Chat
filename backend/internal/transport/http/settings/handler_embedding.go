package settings

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	appembedding "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/embedding"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
)

// GetEmbeddingRuntime godoc
// @Summary 查询 Embedding 服务运行状态
// @Tags admin/settings
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Envelope
// @Router /admin/settings/embedding/runtime [get]
func (h *Handler) GetEmbeddingRuntime(c *gin.Context) {
	cfg := h.runtime.Snapshot()
	baseURL := strings.TrimSpace(cfg.EmbeddingHost)
	model := strings.TrimSpace(cfg.RAGModel)
	view := ServiceRuntimeResponse{
		Source:  "external",
		BaseURL: baseURL,
		Status:  "unconfigured",
		Message: "Embedding 服务未启用",
	}
	if !cfg.EmbeddingEnabled {
		response.Success(c, view)
		return
	}
	if baseURL == "" {
		view.Message = "Embedding 服务地址未配置"
		response.Success(c, view)
		return
	}
	if model == "" {
		view.Message = "Embedding 请求模型未配置"
		response.Success(c, view)
		return
	}
	if h.embeddingSvc == nil {
		view.Status = "unavailable"
		view.Message = "embedding service not available"
		response.Success(c, view)
		return
	}

	timeout := time.Duration(cfg.EmbeddingTimeoutSeconds) * time.Second
	if timeout <= 0 || timeout > 10*time.Second {
		timeout = 10 * time.Second
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
	defer cancel()
	if _, err := h.embeddingSvc.EmbedTexts(ctx, []string{"health check"}); err != nil {
		view.Status = "unhealthy"
		view.Message = err.Error()
		response.Success(c, view)
		return
	}
	view.Status = "running"
	view.Reachable = true
	view.Message = "连接正常"
	response.Success(c, view)
}

// GetEmbeddingStatus godoc
// @Summary 查询向量索引健康状态
// @Tags admin/settings
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Envelope
// @Router /admin/settings/embedding/status [get]
func (h *Handler) GetEmbeddingStatus(c *gin.Context) {
	if h.embeddingSvc == nil {
		response.Error(c, http.StatusServiceUnavailable, "embedding service not available")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	status, err := h.embeddingSvc.GetIndexStatus(ctx)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "get embedding status failed")
		return
	}
	response.Success(c, toEmbeddingIndexStatusResponse(status))
}

// TriggerReindex godoc
// @Summary 触发向量重建（重索引所有 stale/failed 文件）
// @Tags admin/settings
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Envelope
// @Router /admin/settings/embedding/reindex [post]
func (h *Handler) TriggerReindex(c *gin.Context) {
	if h.embeddingSvc == nil {
		response.Error(c, http.StatusServiceUnavailable, "embedding service not available")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	submitted, err := h.embeddingSvc.ReindexStaleFiles(ctx)
	if err != nil {
		if errors.Is(err, appembedding.ErrEmbeddingServiceNotConfigured) {
			response.ErrorFrom(c, http.StatusBadRequest, err)
			return
		}
		response.Error(c, http.StatusInternalServerError, "reindex failed")
		return
	}
	response.Success(c, EmbeddingReindexResponse{Submitted: submitted, Message: "reindex jobs submitted"})
}
