package settings

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	appadmin "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/admin"
	appembedding "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/embedding"
	appsettings "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/settings"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/pagination"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/middleware"
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
		view.Message = "Embedding 服务连接失败，请检查服务配置和运行状态"
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
// @Success 200 {object} EmbeddingIndexStatusResponseDoc
// @Router /admin/settings/embedding/status [get]
func (h *Handler) GetEmbeddingStatus(c *gin.Context) {
	if h.embeddingSvc == nil {
		response.ErrorFrom(c, http.StatusServiceUnavailable, errEmbeddingServiceNotAvailable)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	status, err := h.embeddingSvc.GetIndexStatus(ctx)
	if err != nil {
		response.InternalError(c, err)
		return
	}
	response.Success(c, toEmbeddingIndexStatusResponse(status))
}

// TriggerReindex godoc
// @Summary 补建向量索引
// @Description 为尚未索引、失效、失败及停滞的文件补建向量，已就绪的文件不会重建。已有文本的文件直接向量化；处理失败或排队停滞的文件重新提取后再向量化；include_empty=true 时无文本文件也重新提取，适用于更换 OCR 引擎后
// @Tags admin/settings
// @Produce json
// @Security BearerAuth
// @Param include_empty query bool false "是否同时重新提取无文本文件"
// @Success 200 {object} EmbeddingReindexResponseDoc
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 409 {object} response.Envelope
// @Failure 503 {object} response.Envelope
// @Router /admin/settings/embedding/reindex [post]
func (h *Handler) TriggerReindex(c *gin.Context) {
	if h.embeddingSvc == nil || h.fileEmbedder == nil {
		response.ErrorFrom(c, http.StatusServiceUnavailable, errEmbeddingServiceNotAvailable)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	includeEmpty, _ := strconv.ParseBool(c.Query("include_empty"))
	submitted, err := h.embeddingSvc.ReindexStaleFiles(ctx)
	if err != nil && !errors.Is(err, appembedding.ErrReindexInProgress) {
		writeReindexError(c, err)
		return
	}
	reindexBusy := err != nil
	// 没有可用文本的文件不在补建任务中：它们重新走处理流水线，与正在运行的补建互不影响。
	reprocessed, err := h.fileEmbedder.ReprocessFilesWithoutText(ctx, includeEmpty)
	if err != nil {
		writeReindexError(c, err)
		return
	}
	if reindexBusy && reprocessed == 0 {
		response.ErrorFrom(c, http.StatusConflict, appembedding.ErrReindexInProgress)
		return
	}
	h.recordEmbeddingAudit(c, "settings.embedding_reindex", map[string]any{
		"include_empty": includeEmpty,
		"submitted":     submitted,
		"reprocessed":   reprocessed,
	})
	response.Success(c, EmbeddingReindexResponse{Submitted: submitted + reprocessed, Message: "reindex jobs submitted"})
}

func writeReindexError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, appembedding.ErrEmbeddingServiceNotConfigured):
		response.ErrorFrom(c, http.StatusBadRequest, err)
	case errors.Is(err, appembedding.ErrEmbeddingServiceUnavailable):
		response.RecordError(c, err)
		response.ErrorWithCode(c, http.StatusServiceUnavailable, "embedding.service_unavailable")
	case errors.Is(err, appembedding.ErrReindexInProgress):
		response.ErrorFrom(c, http.StatusConflict, err)
	default:
		response.InternalError(c, err)
	}
}

// ListEmbeddingTasks godoc
// @Summary 查询全平台向量化任务
// @Description 按分组分页列出全平台文件的向量化任务，并返回每个文件当前能否重试
// @Tags admin/settings
// @Produce json
// @Security BearerAuth
// @Param bucket query string true "任务分组" Enums(ready, pending, failed, stale, empty, unsupported)
// @Param query query string false "按文件 ID 或文件名搜索"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} EmbeddingTaskListResponseDoc
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 503 {object} response.Envelope
// @Router /admin/settings/embedding/tasks [get]
func (h *Handler) ListEmbeddingTasks(c *gin.Context) {
	if h.embeddingSvc == nil {
		response.ErrorFrom(c, http.StatusServiceUnavailable, errEmbeddingServiceNotAvailable)
		return
	}
	bucket := strings.TrimSpace(c.Query("bucket"))
	if !appembedding.IsTaskBucket(bucket) {
		response.InvalidQueryParam(c, "bucket")
		return
	}
	page, pageSize := pagination.Parse(c.Query("page"), c.Query("page_size"))
	result, err := h.embeddingSvc.ListFileTasks(c.Request.Context(), appembedding.ListFileTasksInput{
		Bucket:   bucket,
		Query:    c.Query("query"),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		response.InternalError(c, err)
		return
	}
	userIDs := make([]uint, 0, len(result.Items))
	for _, item := range result.Items {
		userIDs = append(userIDs, item.File.UserID)
	}
	labels := h.resolveUserLabels(c.Request.Context(), userIDs)
	results := make([]EmbeddingTaskResponse, 0, len(result.Items))
	for _, item := range result.Items {
		results = append(results, toEmbeddingTaskResponse(item, labels[item.File.UserID]))
	}
	response.SuccessPage(c, result.Total, results)
}

// RetryEmbeddingTasks godoc
// @Summary 重试指定文件的向量化任务
// @Description 管理员跨用户重试失败、待处理、失效或无文本文件的向量化任务；不可重试的文件在 skipped 中返回原因
// @Tags admin/settings
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body EmbeddingTaskRetryRequest true "待重试的文件 ID"
// @Success 200 {object} EmbeddingTaskRetryResponseDoc
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 503 {object} response.Envelope
// @Router /admin/settings/embedding/tasks/retry [post]
func (h *Handler) RetryEmbeddingTasks(c *gin.Context) {
	if h.fileEmbedder == nil {
		response.ErrorFrom(c, http.StatusServiceUnavailable, errEmbeddingServiceNotAvailable)
		return
	}
	var req EmbeddingTaskRetryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	result, err := h.fileEmbedder.SubmitAdminFileEmbeddings(c.Request.Context(), req.FileIDs)
	if err != nil {
		switch {
		case errors.Is(err, appembedding.ErrTooManyTargetedFiles):
			response.RecordError(c, err)
			response.ErrorWithCode(c, http.StatusBadRequest, "embedding.too_many_files")
		case errors.Is(err, appembedding.ErrEmbeddingServiceNotConfigured):
			response.ErrorFrom(c, http.StatusServiceUnavailable, err)
		case errors.Is(err, appembedding.ErrEmbeddingServiceUnavailable):
			response.RecordError(c, err)
			response.ErrorWithCode(c, http.StatusServiceUnavailable, "embedding.service_unavailable")
		default:
			response.InternalError(c, err)
		}
		return
	}
	h.recordEmbeddingAudit(c, "settings.embedding_retry", map[string]any{
		"requested_file_ids": req.FileIDs,
		"submitted_file_ids": result.SubmittedFileIDs,
		"skipped_count":      len(result.Skipped),
	})
	response.Success(c, toEmbeddingTaskRetryResponse(result))
}

func (h *Handler) resolveUserLabels(ctx context.Context, userIDs []uint) map[uint]appadmin.UserLabel {
	if h.userLabels == nil {
		return map[uint]appadmin.UserLabel{}
	}
	return h.userLabels.ResolveUserLabels(ctx, userIDs)
}

// recordEmbeddingAudit 记录管理员对向量索引的写操作：重试会跨用户投递任务，需要可追溯。
func (h *Handler) recordEmbeddingAudit(c *gin.Context, action string, detail any) {
	h.service.RecordAudit(c.Request.Context(), appsettings.AuditInput{
		UserID:    middleware.MustUserID(c),
		RequestID: middleware.MustRequestID(c),
		Action:    action,
		ClientIP:  c.ClientIP(),
		UserAgent: c.Request.UserAgent(),
		Detail:    detail,
	})
}
