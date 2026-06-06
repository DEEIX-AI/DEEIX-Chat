package settings

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	appembedding "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/embedding"
	appruntime "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/runtime"
	appsettings "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/settings"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/nativetool"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/middleware"
	"github.com/gin-gonic/gin"
)

const runtimeActionTimeout = 5 * time.Minute

type nativeToolCatalogProvider interface {
	ListNativeToolDefinitions(ctx context.Context) ([]nativetool.Definition, error)
}

// Handler 封装 settings HTTP 处理。
type Handler struct {
	service         *appsettings.Service
	runtimeSettings *appsettings.RuntimeSettings
	runtimeSvc      *appruntime.Service
	runtime         *config.Runtime
	embeddingSvc    *appembedding.Service // 可选，用于模型变更后触发向量失效
	nativeTools     nativeToolCatalogProvider
}

// NewHandler 创建处理器。
func NewHandler(service *appsettings.Service, runtimeSettings *appsettings.RuntimeSettings, runtimeSvc *appruntime.Service, runtime *config.Runtime) *Handler {
	return &Handler{
		service:         service,
		runtimeSettings: runtimeSettings,
		runtimeSvc:      runtimeSvc,
		runtime:         runtime,
	}
}

// SetEmbeddingService 注入 Embedding 服务（可选），用于在模型配置变更时自动标记向量失效。
func (h *Handler) SetEmbeddingService(svc *appembedding.Service) {
	h.embeddingSvc = svc
}

// SetNativeToolCatalogProvider 注入平台级官方原生工具目录提供者。
func (h *Handler) SetNativeToolCatalogProvider(provider nativeToolCatalogProvider) {
	h.nativeTools = provider
}

// ListAll godoc
// @Summary 查询全部动态配置
// @Description 按 namespace 分组返回全部动态配置项
// @Tags admin/settings
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Envelope
// @Router /admin/settings [get]
func (h *Handler) ListAll(c *gin.Context) {
	data, err := h.service.ListAll(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "list settings failed")
		return
	}
	response.Success(c, toSettingResponseMap(data))
}

// ListByNamespace godoc
// @Summary 查询指定 namespace 的配置
// @Description 查询指定 namespace 下的全部配置项
// @Tags admin/settings
// @Produce json
// @Security BearerAuth
// @Param namespace path string true "命名空间"
// @Success 200 {object} response.Envelope
// @Router /admin/settings/{namespace} [get]
func (h *Handler) ListByNamespace(c *gin.Context) {
	ns := c.Param("namespace")
	if !appsettings.IsValidNamespace(ns) {
		response.Error(c, http.StatusBadRequest, "invalid namespace")
		return
	}

	data, err := h.service.ListByNamespace(c.Request.Context(), ns)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "list settings failed")
		return
	}
	response.Success(c, toSettingResponseList(data))
}

// GetLoginPageSettings godoc
// @Summary 查询公开登录页配置
// @Tags settings
// @Produce json
// @Success 200 {object} response.Envelope
// @Router /settings/login-page [get]
func (h *Handler) GetLoginPageSettings(c *gin.Context) {
	items, err := h.service.ListByNamespace(c.Request.Context(), "auth")
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "list login page settings failed")
		return
	}
	values := map[string]string{
		"login_page_title":        "Sign in to DEEIX Chat",
		"login_default_next_path": "/chat",
	}
	for _, item := range items {
		if _, ok := values[item.Key]; ok {
			values[item.Key] = item.Value
		}
	}
	if strings.TrimSpace(values["login_page_title"]) == "" {
		values["login_page_title"] = "Sign in to DEEIX Chat"
	}
	if strings.TrimSpace(values["login_default_next_path"]) == "" ||
		!strings.HasPrefix(values["login_default_next_path"], "/") ||
		strings.HasPrefix(values["login_default_next_path"], "//") {
		values["login_default_next_path"] = "/chat"
	}
	response.Success(c, LoginPageSettingsResponse{
		Title:           values["login_page_title"],
		DefaultNextPath: values["login_default_next_path"],
	})
}

// GetModelOptionPolicy godoc
// @Summary 查询模型 options 透传策略
// @Tags settings
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Envelope
// @Router /settings/model-option-policy [get]
func (h *Handler) GetModelOptionPolicy(c *gin.Context) {
	items, err := h.service.RuntimeValuesByNamespace(c.Request.Context(), "chat")
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "list model option policy failed")
		return
	}
	mode := strings.TrimSpace(items["model_option_policy_mode"])
	if mode == "" {
		mode = "allowlist"
	}
	allowedPathsJSON := strings.TrimSpace(items["model_option_allowed_paths"])
	if allowedPathsJSON == "" {
		allowedPathsJSON = config.DefaultModelOptionAllowedPathsJSON()
	}
	deniedPathsJSON := strings.TrimSpace(items["model_option_denied_paths"])
	if deniedPathsJSON == "" {
		deniedPathsJSON = config.DefaultModelOptionDeniedPathsJSON()
	}
	nativeTools := nativetool.Definitions()
	if h.nativeTools != nil {
		items, err := h.nativeTools.ListNativeToolDefinitions(c.Request.Context())
		if err != nil {
			response.Error(c, http.StatusInternalServerError, "list native tools failed")
			return
		}
		nativeTools = items
	}
	response.Success(c, ModelOptionPolicyResponse{
		Mode:             mode,
		AllowedPathsJSON: allowedPathsJSON,
		DeniedPathsJSON:  deniedPathsJSON,
		NativeTools:      toNativeToolDefinitionResponses(nativeTools),
	})
}

// GetMCPPolicy godoc
// @Summary 查询 MCP 工具运行策略
// @Tags settings
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Envelope
// @Router /settings/mcp-policy [get]
func (h *Handler) GetMCPPolicy(c *gin.Context) {
	cfg := h.runtime.Snapshot()
	limit := cfg.MCPMaxSelectedToolsPerMessage
	if limit <= 0 {
		limit = config.DefaultMCPMaxSelectedToolsPerMessage
	}
	if limit > config.MaxMCPSelectedToolsPerMessage {
		limit = config.MaxMCPSelectedToolsPerMessage
	}
	response.Success(c, MCPPolicyResponse{MaxSelectedToolsPerMessage: limit})
}

// Patch godoc
// @Summary 批量更新配置项
// @Description 批量更新动态配置并清除缓存，下次读取自动刷新
// @Tags admin/settings
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body PatchSettingsRequest true "更新项"
// @Success 200 {object} response.Envelope
// @Router /admin/settings [patch]
func (h *Handler) Patch(c *gin.Context) {
	var req PatchSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}

	// 记录变更前的模型配置，用于检测模型变更
	prevCfg := h.runtime.Snapshot()
	prevSignature := appembedding.ComputeModelSignature(prevCfg.RAGModel, prevCfg.EmbeddingOutputDimensions)

	data, err := h.service.BatchUpdate(c.Request.Context(), toAppPatchItems(req.Items))
	if err != nil {
		if errors.Is(err, appsettings.ErrInvalidSetting) {
			response.ErrorFrom(c, http.StatusBadRequest, err)
			return
		}
		response.Error(c, http.StatusInternalServerError, "update settings failed")
		return
	}

	// 清除 Redis 缓存，下次读取自动从 DB 刷新
	h.runtimeSettings.InvalidateCacheMulti(c.Request.Context(), toAppPatchItems(req.Items))
	if err = h.runtimeSettings.ApplyTo(c.Request.Context(), h.runtime); err != nil {
		response.Error(c, http.StatusInternalServerError, "refresh runtime settings failed")
		return
	}

	// 检测 Embedding 模型签名：模型变更时标记旧向量为 stale；签名缺失时只补写当前签名。
	newCfg := h.runtime.Snapshot()
	newSignature := appembedding.ComputeModelSignature(newCfg.RAGModel, newCfg.EmbeddingOutputDimensions)
	signatureMissing := strings.TrimSpace(newCfg.EmbeddingModelSignature) == "" && strings.TrimSpace(newCfg.RAGModel) != ""
	if (newSignature != prevSignature || signatureMissing) && h.embeddingSvc != nil {
		go func() {
			staleCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if newSignature != prevSignature {
				if _, staleErr := h.embeddingSvc.MarkAllFilesStale(staleCtx); staleErr != nil {
					return
				}
			}
			_, _ = h.service.BatchUpdate(staleCtx, []appsettings.PatchItem{
				{Namespace: "file", Key: "embedding_model_signature", Value: newSignature},
			})
			_ = h.runtimeSettings.ApplyTo(staleCtx, h.runtime)
		}()
	}

	h.service.RecordAudit(c.Request.Context(), appsettings.AuditInput{
		UserID:    middleware.MustUserID(c),
		RequestID: middleware.MustRequestID(c),
		Action:    "settings.update",
		ClientIP:  c.ClientIP(),
		UserAgent: c.Request.UserAgent(),
		Detail:    sanitizePatchItemsForAudit(req.Items),
	})

	response.Success(c, toSettingResponseMap(data))
}

// GetTikaRuntime godoc
// @Summary 查询 Tika 运行状态
// @Tags admin/settings
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Envelope
// @Router /admin/settings/tika/runtime [get]
func (h *Handler) GetTikaRuntime(c *gin.Context) {
	if h.runtimeSvc == nil {
		response.Error(c, http.StatusInternalServerError, "tika runtime service unavailable")
		return
	}
	response.Success(c, toTikaRuntimeResponse(h.runtimeSvc.GetTikaStatus(c.Request.Context())))
}

// GetDoclingRuntime godoc
// @Summary 查询 Docling 运行状态
// @Tags admin/settings
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Envelope
// @Router /admin/settings/docling/runtime [get]
func (h *Handler) GetDoclingRuntime(c *gin.Context) {
	if h.runtimeSvc == nil {
		response.Error(c, http.StatusInternalServerError, "docling runtime service unavailable")
		return
	}
	response.Success(c, toDoclingRuntimeResponse(h.runtimeSvc.GetDoclingStatus(c.Request.Context())))
}

// GetTesseractRuntime godoc
// @Summary 查询 Tesseract OCR 运行状态
// @Tags admin/settings
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Envelope
// @Router /admin/settings/tesseract/runtime [get]
func (h *Handler) GetTesseractRuntime(c *gin.Context) {
	if h.runtimeSvc == nil {
		response.Error(c, http.StatusInternalServerError, "tesseract runtime service unavailable")
		return
	}
	response.Success(c, toTesseractRuntimeResponse(h.runtimeSvc.GetTesseractStatus(c.Request.Context())))
}

// GetRapidOCRRuntime godoc
// @Summary 查询 RapidOCR 运行状态
// @Tags admin/settings
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Envelope
// @Router /admin/settings/rapidocr/runtime [get]
func (h *Handler) GetRapidOCRRuntime(c *gin.Context) {
	if h.runtimeSvc == nil {
		response.Error(c, http.StatusInternalServerError, "rapidocr runtime service unavailable")
		return
	}
	response.Success(c, toRapidOCRRuntimeResponse(h.runtimeSvc.GetRapidOCRStatus(c.Request.Context())))
}

// GetMinerURuntime godoc
// @Summary 查询 MinerU 运行状态
// @Tags admin/settings
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Envelope
// @Router /admin/settings/mineru/runtime [get]
func (h *Handler) GetMinerURuntime(c *gin.Context) {
	if h.runtimeSvc == nil {
		response.Error(c, http.StatusInternalServerError, "mineru runtime service unavailable")
		return
	}
	response.Success(c, toMinerURuntimeResponse(h.runtimeSvc.GetMinerUStatus(c.Request.Context())))
}

// StartTikaRuntime godoc
// @Summary 启动托管 Tika
// @Tags admin/settings
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Envelope
// @Router /admin/settings/tika/runtime/start [post]
func (h *Handler) StartTikaRuntime(c *gin.Context) {
	if h.runtimeSvc == nil {
		response.Error(c, http.StatusInternalServerError, "tika runtime service unavailable")
		return
	}
	h.handleTikaRuntimeAction(c, h.runtimeSvc.StartTika)
}

// StartRapidOCRRuntime godoc
// @Summary 启动托管 RapidOCR
// @Tags admin/settings
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Envelope
// @Router /admin/settings/rapidocr/runtime/start [post]
func (h *Handler) StartRapidOCRRuntime(c *gin.Context) {
	if h.runtimeSvc == nil {
		response.Error(c, http.StatusInternalServerError, "rapidocr runtime service unavailable")
		return
	}
	h.handleRapidOCRRuntimeAction(c, h.runtimeSvc.StartRapidOCR)
}

// StopTikaRuntime godoc
// @Summary 停止托管 Tika
// @Tags admin/settings
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Envelope
// @Router /admin/settings/tika/runtime/stop [post]
func (h *Handler) StopTikaRuntime(c *gin.Context) {
	if h.runtimeSvc == nil {
		response.Error(c, http.StatusInternalServerError, "tika runtime service unavailable")
		return
	}
	h.handleTikaRuntimeAction(c, h.runtimeSvc.StopTika)
}

// StopRapidOCRRuntime godoc
// @Summary 停止托管 RapidOCR
// @Tags admin/settings
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Envelope
// @Router /admin/settings/rapidocr/runtime/stop [post]
func (h *Handler) StopRapidOCRRuntime(c *gin.Context) {
	if h.runtimeSvc == nil {
		response.Error(c, http.StatusInternalServerError, "rapidocr runtime service unavailable")
		return
	}
	h.handleRapidOCRRuntimeAction(c, h.runtimeSvc.StopRapidOCR)
}

// RestartTikaRuntime godoc
// @Summary 重启托管 Tika
// @Tags admin/settings
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Envelope
// @Router /admin/settings/tika/runtime/restart [post]
func (h *Handler) RestartTikaRuntime(c *gin.Context) {
	if h.runtimeSvc == nil {
		response.Error(c, http.StatusInternalServerError, "tika runtime service unavailable")
		return
	}
	h.handleTikaRuntimeAction(c, h.runtimeSvc.RestartTika)
}

// RestartRapidOCRRuntime godoc
// @Summary 重启托管 RapidOCR
// @Tags admin/settings
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Envelope
// @Router /admin/settings/rapidocr/runtime/restart [post]
func (h *Handler) RestartRapidOCRRuntime(c *gin.Context) {
	if h.runtimeSvc == nil {
		response.Error(c, http.StatusInternalServerError, "rapidocr runtime service unavailable")
		return
	}
	h.handleRapidOCRRuntimeAction(c, h.runtimeSvc.RestartRapidOCR)
}

func (h *Handler) handleTikaRuntimeAction(c *gin.Context, action func(ctx context.Context) (appruntime.ServiceRuntimeView, error)) {
	if h.runtimeSvc == nil {
		response.Error(c, http.StatusInternalServerError, "tika runtime service unavailable")
		return
	}
	actionCtx, cancel := context.WithTimeout(context.Background(), runtimeActionTimeout)
	defer cancel()
	view, err := action(actionCtx)
	if err != nil {
		response.ErrorFrom(c, http.StatusInternalServerError, err)
		return
	}
	response.Success(c, toTikaRuntimeResponse(view))
}

func (h *Handler) handleRapidOCRRuntimeAction(c *gin.Context, action func(ctx context.Context) (appruntime.ServiceRuntimeView, error)) {
	if h.runtimeSvc == nil {
		response.Error(c, http.StatusInternalServerError, "rapidocr runtime service unavailable")
		return
	}
	actionCtx, cancel := context.WithTimeout(context.Background(), runtimeActionTimeout)
	defer cancel()
	view, err := action(actionCtx)
	if err != nil {
		response.ErrorFrom(c, http.StatusInternalServerError, err)
		return
	}
	response.Success(c, toRapidOCRRuntimeResponse(view))
}
