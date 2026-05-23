package channel

import (
	"errors"
	"net/http"
	"strconv"

	appchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
)

// Handler 封装上游与模型管理 HTTP 处理。
type Handler struct {
	service *appchannel.Service
}

// NewHandler 创建处理器。
func NewHandler(service *appchannel.Service) *Handler {
	return &Handler{service: service}
}

func upstreamConfigErrorMessage(err error) string {
	switch {
	case errors.Is(err, appchannel.ErrInvalidHeadersConfig):
		return "invalid headers json config"
	case errors.Is(err, appchannel.ErrInvalidAPIKeysConfig):
		return "invalid api keys config"
	case errors.Is(err, appchannel.ErrInvalidProtocolDefaultsConfig):
		return "invalid protocol defaults config"
	case errors.Is(err, appchannel.ErrInvalidJSONConfig):
		return "invalid json config"
	case errors.Is(err, appchannel.ErrInvalidUpstreamBaseURL):
		return "invalid upstream base url"
	default:
		return ""
	}
}

// ---------------------------------------------------------------------------
// 用户侧模型目录
// ---------------------------------------------------------------------------

// ListPublicModels godoc
// @Summary 查询可用模型目录
// @Description 用户侧查询启用模型目录，用于聊天模型选择器
// @Tags llm
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} PublicModelListResponseDoc
// @Failure 500 {object} ErrorDoc
// @Router /models [get]
func (h *Handler) ListPublicModels(c *gin.Context) {
	items, err := h.service.ListActiveModels(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "list models failed")
		return
	}

	views := make([]PublicModelResponse, 0, len(items))
	for _, item := range items {
		views = append(views, toPublicModelResponse(item))
	}
	response.Success(c, views)
}

// ---------------------------------------------------------------------------
// 上游管理
// ---------------------------------------------------------------------------

// ListUpstreams godoc
// @Summary 管理员查询上游列表
// @Description 管理员分页查询 LLM 上游配置
// @Tags llm
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Param q query string false "搜索关键词"
// @Param status query string false "状态：active/inactive/circuit"
// @Param compatible query string false "兼容类型"
// @Param sort query string false "排序：id_desc/id_asc/name_asc/updated_desc"
// @Success 200 {object} UpstreamListResponseDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/llm/upstreams [get]
func (h *Handler) ListUpstreams(c *gin.Context) {
	page, pageSize := pageParams(c)
	items, total, err := h.service.ListUpstreams(c.Request.Context(), page, pageSize, appchannel.ListUpstreamsInput{
		Query:      c.Query("q"),
		Status:     c.Query("status"),
		Compatible: c.Query("compatible"),
		Sort:       c.Query("sort"),
	})
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "list upstreams failed")
		return
	}
	results := make([]UpstreamResponse, 0, len(items))
	for _, item := range items {
		results = append(results, toUpstreamResponse(item))
	}
	response.SuccessPage(c, total, results)
}

// CreateUpstream godoc
// @Summary 管理员创建上游
// @Description 管理员新增上游来源配置，内部标识自动分配
// @Tags llm
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body CreateUpstreamRequest true "上游参数"
// @Success 200 {object} CreateUpstreamResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 409 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/llm/upstreams [post]
func (h *Handler) CreateUpstream(c *gin.Context) {
	var req CreateUpstreamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}

	item, err := h.service.CreateUpstream(c.Request.Context(), appchannel.CreateUpstreamInput{
		Name:                 req.Name,
		BaseURL:              req.BaseURL,
		Compatible:           req.Compatible,
		ProtocolDefaultsJSON: req.ProtocolDefaultsJSON,
		APIKeys:              req.APIKeys,
		Status:               req.Status,
		ConnectTimeoutMS:     req.ConnectTimeoutMS,
		ReadTimeoutMS:        req.ReadTimeoutMS,
		StreamIdleTimeoutMS:  req.StreamIdleTimeoutMS,
		CbFailureThreshold:   req.CbFailureThreshold,
		CbModelThreshold:     req.CbModelThreshold,
		CbThresholdLogic:     req.CbThresholdLogic,
		CbDurationMin:        req.CbDurationMin,
		CbWindowMin:          req.CbWindowMin,
		HeadersJSON:          req.HeadersJSON,
	})
	if err != nil {
		switch {
		case upstreamConfigErrorMessage(err) != "":
			response.Error(c, http.StatusBadRequest, upstreamConfigErrorMessage(err))
		case errors.Is(err, appchannel.ErrInvalidJSONConfig):
			response.Error(c, http.StatusBadRequest, "invalid json config")
		case errors.Is(err, appchannel.ErrInvalidAdapter):
			response.Error(c, http.StatusBadRequest, "invalid adapter")
		case errors.Is(err, appchannel.ErrInvalidCompatible):
			response.Error(c, http.StatusBadRequest, "invalid compatible")
		default:
			response.Error(c, http.StatusInternalServerError, "create upstream failed")
		}
		return
	}
	response.Success(c, UpstreamDataResponse{Upstream: toUpstreamResponse(*item)})
}

// UpdateUpstream godoc
// @Summary 管理员更新上游
// @Description 管理员更新上游配置（地址、密钥、状态等）
// @Tags llm
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "上游ID"
// @Param body body UpdateUpstreamRequest true "上游参数"
// @Success 200 {object} UpdateUpstreamResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/llm/upstreams/{id} [patch]
func (h *Handler) UpdateUpstream(c *gin.Context) {
	upstreamID, err := uintParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid upstream id")
		return
	}

	var req UpdateUpstreamRequest
	if err = c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}

	item, err := h.service.UpdateUpstream(c.Request.Context(), upstreamID, appchannel.UpdateUpstreamInput{
		Name:                 req.Name,
		BaseURL:              req.BaseURL,
		Compatible:           req.Compatible,
		ProtocolDefaultsJSON: req.ProtocolDefaultsJSON,
		APIKeys:              req.APIKeys,
		Status:               req.Status,
		ConnectTimeoutMS:     req.ConnectTimeoutMS,
		ReadTimeoutMS:        req.ReadTimeoutMS,
		StreamIdleTimeoutMS:  req.StreamIdleTimeoutMS,
		CbFailureThreshold:   req.CbFailureThreshold,
		CbModelThreshold:     req.CbModelThreshold,
		CbThresholdLogic:     req.CbThresholdLogic,
		CbDurationMin:        req.CbDurationMin,
		CbWindowMin:          req.CbWindowMin,
		HeadersJSON:          req.HeadersJSON,
	})
	if err != nil {
		switch {
		case errors.Is(err, appchannel.ErrUpstreamNotFound):
			response.Error(c, http.StatusNotFound, "upstream not found")
		case upstreamConfigErrorMessage(err) != "":
			response.Error(c, http.StatusBadRequest, upstreamConfigErrorMessage(err))
		case errors.Is(err, appchannel.ErrInvalidJSONConfig):
			response.Error(c, http.StatusBadRequest, "invalid json config")
		case errors.Is(err, appchannel.ErrInvalidAdapter):
			response.Error(c, http.StatusBadRequest, "invalid adapter")
		case errors.Is(err, appchannel.ErrInvalidCompatible):
			response.Error(c, http.StatusBadRequest, "invalid compatible")
		default:
			response.Error(c, http.StatusInternalServerError, "update upstream failed")
		}
		return
	}
	response.Success(c, UpstreamDataResponse{Upstream: toUpstreamResponse(*item)})
}

// DeleteUpstream godoc
// @Summary 管理员删除上游
// @Description 管理员删除上游配置及其关联路由绑定
// @Tags llm
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "上游ID"
// @Success 200 {object} response.SuccessDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/llm/upstreams/{id} [delete]
func (h *Handler) DeleteUpstream(c *gin.Context) {
	upstreamID, err := uintParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid upstream id")
		return
	}

	if err = h.service.DeleteUpstream(c.Request.Context(), upstreamID); err != nil {
		if errors.Is(err, appchannel.ErrUpstreamNotFound) {
			response.Error(c, http.StatusNotFound, "upstream not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, "delete upstream failed")
		return
	}
	response.Success(c, nil)
}

// BatchDeleteUpstreams godoc
// @Summary 管理员批量删除上游
// @Description 管理员批量删除上游及其关联路由绑定，保留模型目录
// @Tags llm
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body BatchDeleteRequest true "批量删除请求"
// @Success 200 {object} BatchDeleteResponseDoc
// @Failure 400 {object} ErrorDoc
// @Router /admin/llm/upstreams/batch-delete [post]
func (h *Handler) BatchDeleteUpstreams(c *gin.Context) {
	var req BatchDeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}

	response.Success(c, toBatchDeleteResponse(*h.service.BatchDeleteUpstreams(c.Request.Context(), req.IDs)))
}

// OpenUpstreamCircuit godoc
// @Summary 管理员手动触发上游熔断
// @Description 管理员手动开启上游熔断状态
// @Tags llm
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "上游ID"
// @Success 200 {object} response.SuccessDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/llm/upstreams/{id}/circuit/open [post]
func (h *Handler) OpenUpstreamCircuit(c *gin.Context) {
	upstreamID, err := uintParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid upstream id")
		return
	}

	if err = h.service.OpenUpstreamCircuit(c.Request.Context(), upstreamID); err != nil {
		if errors.Is(err, appchannel.ErrUpstreamNotFound) {
			response.Error(c, http.StatusNotFound, "upstream not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, "open upstream circuit failed")
		return
	}
	response.Success(c, nil)
}

// ResetUpstreamCircuit godoc
// @Summary 管理员重置上游熔断
// @Description 管理员手动清空上游失败计数并关闭熔断状态
// @Tags llm
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "上游ID"
// @Success 200 {object} ResetUpstreamCircuitResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/llm/upstreams/{id}/circuit/reset [post]
func (h *Handler) ResetUpstreamCircuit(c *gin.Context) {
	upstreamID, err := uintParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid upstream id")
		return
	}

	if err = h.service.ResetUpstreamCircuit(c.Request.Context(), upstreamID); err != nil {
		if errors.Is(err, appchannel.ErrUpstreamNotFound) {
			response.Error(c, http.StatusNotFound, "upstream not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, "reset upstream circuit failed")
		return
	}
	response.Success(c, CircuitResetResponse{Reset: true})
}

// ---------------------------------------------------------------------------
// 上游模型路由绑定
// ---------------------------------------------------------------------------

// ListUpstreamModels godoc
// @Summary 管理员查询上游模型路由绑定
// @Description 管理员分页查询指定上游的路由绑定列表
// @Tags llm
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "上游ID"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Param q query string false "搜索关键词"
// @Param route_status query string false "路由状态：bound/active/inactive"
// @Param upstream_status query string false "上游模型状态：active/inactive"
// @Param protocol query string false "接口协议"
// @Param sort query string false "排序：upstream_asc/upstream_desc/platform_asc/platform_desc/status_asc/protocol_asc"
// @Success 200 {object} UpstreamModelListResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/llm/upstreams/{id}/models [get]
func (h *Handler) ListUpstreamModels(c *gin.Context) {
	upstreamID, err := uintParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid upstream id")
		return
	}

	page, pageSize := pageParams(c)
	items, total, err := h.service.ListUpstreamModels(c.Request.Context(), upstreamID, page, pageSize, appchannel.ListUpstreamModelsInput{
		Query:          c.Query("q"),
		RouteStatus:    c.Query("route_status"),
		UpstreamStatus: c.Query("upstream_status"),
		Protocol:       c.Query("protocol"),
		Sort:           c.Query("sort"),
	})
	if err != nil {
		if errors.Is(err, appchannel.ErrUpstreamNotFound) {
			response.Error(c, http.StatusNotFound, "upstream not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, "list upstream models failed")
		return
	}
	results := make([]UpstreamModelResponse, 0, len(items))
	for _, item := range items {
		results = append(results, toUpstreamModelResponse(item))
	}
	response.SuccessPage(c, total, results)
}

// UpsertUpstreamModel godoc
// @Summary 管理员新增或更新上游模型路由绑定
// @Description 管理员配置平台模型到指定上游真实模型的路由绑定与覆盖请求头
// @Tags llm
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "上游ID"
// @Param body body UpsertUpstreamModelRequest true "路由绑定参数"
// @Success 200 {object} UpsertUpstreamModelResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 409 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/llm/upstreams/{id}/models [post]
func (h *Handler) UpsertUpstreamModel(c *gin.Context) {
	upstreamID, err := uintParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid upstream id")
		return
	}

	var req UpsertUpstreamModelRequest
	if err = c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}

	item, err := h.service.UpsertUpstreamModel(c.Request.Context(), upstreamID, appchannel.UpsertUpstreamModelInput{
		RouteID:            req.RouteID,
		PlatformModelName:  req.PlatformModelName,
		UpstreamModelName:  req.UpstreamModelName,
		Protocol:           req.Protocol,
		KindsJSON:          req.KindsJSON,
		Status:             req.Status,
		Priority:           req.Priority,
		Weight:             req.Weight,
		Source:             req.Source,
		CbFailureThreshold: req.CbFailureThreshold,
		CbDurationMin:      req.CbDurationMin,
		CbWindowMin:        req.CbWindowMin,
		HeadersJSON:        req.HeadersJSON,
	})
	if err != nil {
		switch {
		case errors.Is(err, appchannel.ErrUpstreamNotFound):
			response.Error(c, http.StatusNotFound, "upstream not found")
		case errors.Is(err, appchannel.ErrModelNotFound):
			response.Error(c, http.StatusNotFound, "model not found")
		case errors.Is(err, appchannel.ErrUpstreamModelConflict):
			response.Error(c, http.StatusConflict, "target model already bound on this upstream")
		case errors.Is(err, appchannel.ErrInvalidJSONConfig):
			response.Error(c, http.StatusBadRequest, "invalid json config")
		case errors.Is(err, appchannel.ErrInvalidAdapter):
			response.Error(c, http.StatusBadRequest, "invalid adapter")
		case errors.Is(err, appchannel.ErrInvalidKinds):
			response.Error(c, http.StatusBadRequest, "invalid kinds")
		case errors.Is(err, appchannel.ErrInvalidPlatformModelName):
			response.Error(c, http.StatusBadRequest, "invalid platform model name")
		case errors.Is(err, appchannel.ErrProtocolRequired):
			response.Error(c, http.StatusBadRequest, "protocol required")
		default:
			response.Error(c, http.StatusInternalServerError, "upsert upstream model failed")
		}
		return
	}
	response.Success(c, UpstreamModelDataResponse{Binding: toUpstreamModelResponse(*item)})
}

// DeleteUpstreamModel godoc
// @Summary 管理员删除上游模型路由绑定
// @Description 管理员删除指定上游的路由绑定
// @Tags llm
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "上游ID"
// @Param route_id path int true "路由绑定ID"
// @Success 200 {object} response.SuccessDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/llm/upstreams/{id}/models/{route_id} [delete]
func (h *Handler) DeleteUpstreamModel(c *gin.Context) {
	upstreamID, err := uintParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid upstream id")
		return
	}
	routeID, err := uintParam(c, "route_id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid route id")
		return
	}

	if err = h.service.DeleteUpstreamModel(c.Request.Context(), upstreamID, routeID); err != nil {
		if errors.Is(err, appchannel.ErrUpstreamModelNotFound) {
			response.Error(c, http.StatusNotFound, "upstream model not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, "delete upstream model failed")
		return
	}
	response.Success(c, nil)
}

// DisableUpstreamModel godoc
// @Summary 管理员停用上游模型路由绑定
// @Description 管理员停用该路由绑定，后续路由不会选中
// @Tags llm
// @Produce json
// @Security BearerAuth
// @Param id path int true "上游ID"
// @Param route_id path int true "路由绑定ID"
// @Success 200 {object} response.SuccessDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/llm/upstreams/{id}/models/{route_id}/disable [patch]
func (h *Handler) DisableUpstreamModel(c *gin.Context) {
	upstreamID, err := uintParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid upstream id")
		return
	}
	routeID, err := uintParam(c, "route_id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid route id")
		return
	}

	if err = h.service.DisableUpstreamModel(c.Request.Context(), upstreamID, routeID); err != nil {
		if errors.Is(err, appchannel.ErrUpstreamModelNotFound) {
			response.Error(c, http.StatusNotFound, "upstream model not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, "disable route binding failed")
		return
	}
	response.Success(c, nil)
}

// EnableUpstreamModel godoc
// @Summary 管理员启用上游模型路由绑定
// @Description 管理员启用该路由绑定，使该上游模型重新参与路由
// @Tags llm
// @Produce json
// @Security BearerAuth
// @Param id path int true "上游ID"
// @Param route_id path int true "路由绑定ID"
// @Success 200 {object} response.SuccessDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/llm/upstreams/{id}/models/{route_id}/enable [patch]
func (h *Handler) EnableUpstreamModel(c *gin.Context) {
	upstreamID, err := uintParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid upstream id")
		return
	}
	routeID, err := uintParam(c, "route_id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid route id")
		return
	}

	if err = h.service.EnableUpstreamModel(c.Request.Context(), upstreamID, routeID); err != nil {
		if errors.Is(err, appchannel.ErrUpstreamModelNotFound) {
			response.Error(c, http.StatusNotFound, "upstream model not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, "enable route binding failed")
		return
	}
	response.Success(c, nil)
}

// BatchDeleteUpstreamModels godoc
// @Summary 管理员批量删除上游模型路由绑定
// @Description 管理员批量删除指定上游下的路由绑定，保留模型目录
// @Tags llm
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "上游ID"
// @Param body body BatchDeleteRequest true "批量删除请求"
// @Success 200 {object} BatchDeleteResponseDoc
// @Failure 400 {object} ErrorDoc
// @Router /admin/llm/upstreams/{id}/models/batch-delete [post]
func (h *Handler) BatchDeleteUpstreamModels(c *gin.Context) {
	upstreamID, err := uintParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid upstream id")
		return
	}

	var req BatchDeleteRequest
	if err = c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}

	response.Success(c, toBatchDeleteResponse(*h.service.BatchDeleteUpstreamModels(c.Request.Context(), upstreamID, req.IDs)))
}

// OpenUpstreamModelCircuit godoc
// @Summary 管理员手动触发上游模型路由熔断
// @Description 管理员手动开启上游模型路由绑定熔断状态
// @Tags llm
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "上游ID"
// @Param route_id path int true "路由绑定ID"
// @Success 200 {object} response.SuccessDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/llm/upstreams/{id}/models/{route_id}/circuit/open [post]
func (h *Handler) OpenUpstreamModelCircuit(c *gin.Context) {
	upstreamID, err := uintParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid upstream id")
		return
	}
	routeID, err := uintParam(c, "route_id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid route id")
		return
	}

	if err = h.service.OpenUpstreamModelCircuit(c.Request.Context(), upstreamID, routeID); err != nil {
		if errors.Is(err, appchannel.ErrUpstreamModelNotFound) {
			response.Error(c, http.StatusNotFound, "upstream model not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, "open upstream model circuit failed")
		return
	}
	response.Success(c, nil)
}

// ResetUpstreamModelCircuit godoc
// @Summary 管理员重置上游模型路由熔断
// @Description 管理员手动清空上游模型路由绑定失败计数并关闭熔断状态
// @Tags llm
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "上游ID"
// @Param route_id path int true "路由绑定ID"
// @Success 200 {object} ResetUpstreamCircuitResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/llm/upstreams/{id}/models/{route_id}/circuit/reset [post]
func (h *Handler) ResetUpstreamModelCircuit(c *gin.Context) {
	upstreamID, err := uintParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid upstream id")
		return
	}
	routeID, err := uintParam(c, "route_id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid route id")
		return
	}

	if err = h.service.ResetUpstreamModelCircuit(c.Request.Context(), upstreamID, routeID); err != nil {
		if errors.Is(err, appchannel.ErrUpstreamModelNotFound) {
			response.Error(c, http.StatusNotFound, "upstream model not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, "reset upstream model circuit failed")
		return
	}
	response.Success(c, CircuitResetResponse{Reset: true})
}

// ---------------------------------------------------------------------------
// 远端模型发现
// ---------------------------------------------------------------------------

// ListRemoteModels godoc
// @Summary 管理员预览上游远程模型
// @Description 调用上游 models 接口，仅返回可导入预览，不直接落库
// @Tags llm
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "上游ID"
// @Success 200 {object} UpstreamRemoteModelsResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 502 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/llm/upstreams/{id}/models/remote [get]
func (h *Handler) ListRemoteModels(c *gin.Context) {
	upstreamID, err := uintParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid upstream id")
		return
	}

	data, err := h.service.ListRemoteModels(c.Request.Context(), upstreamID)
	if err != nil {
		switch {
		case errors.Is(err, appchannel.ErrUpstreamNotFound):
			response.Error(c, http.StatusNotFound, "upstream not found")
		case errors.Is(err, appchannel.ErrNoActiveKey):
			response.Error(c, http.StatusBadRequest, "no active api key")
		case errors.Is(err, appchannel.ErrRemoteModelsUnavailable):
			response.Error(c, http.StatusBadGateway, "remote models unavailable")
		default:
			response.Error(c, http.StatusInternalServerError, "list remote models failed")
		}
		return
	}
	response.Success(c, toUpstreamRemoteModelsResponse(*data))
}

// SyncUpstreamModels godoc
// @Summary 管理员同步上游模型目录
// @Description 调用上游 models 接口写入上游真实模型清单，不自动绑定平台模型
// @Tags llm
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "上游ID"
// @Success 200 {object} SyncUpstreamModelsResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 502 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/llm/upstreams/{id}/models/sync [post]
func (h *Handler) SyncUpstreamModels(c *gin.Context) {
	upstreamID, err := uintParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid upstream id")
		return
	}

	data, err := h.service.SyncUpstreamModels(c.Request.Context(), upstreamID)
	if err != nil {
		switch {
		case errors.Is(err, appchannel.ErrUpstreamNotFound):
			response.Error(c, http.StatusNotFound, "upstream not found")
		case errors.Is(err, appchannel.ErrNoActiveKey):
			response.Error(c, http.StatusBadRequest, "no active api key")
		case errors.Is(err, appchannel.ErrRemoteModelsUnavailable):
			response.Error(c, http.StatusBadGateway, "remote models unavailable")
		default:
			response.Error(c, http.StatusInternalServerError, "sync upstream models failed")
		}
		return
	}
	response.Success(c, toSyncUpstreamModelsResponse(*data))
}

// ImportUpstreamModels godoc
// @Summary 管理员批量导入上游模型
// @Description 选择性导入上游模型，支持绑定平台模型与自定义条目
// @Tags llm
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "上游ID"
// @Param body body ImportUpstreamModelsRequest true "导入参数"
// @Success 200 {object} ImportUpstreamModelsResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 502 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/llm/upstreams/{id}/models/import [post]
func (h *Handler) ImportUpstreamModels(c *gin.Context) {
	upstreamID, err := uintParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid upstream id")
		return
	}

	var req ImportUpstreamModelsRequest
	if err = c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}

	items := make([]appchannel.ImportUpstreamModelItemInput, 0, len(req.Items))
	for _, item := range req.Items {
		items = append(items, appchannel.ImportUpstreamModelItemInput{
			PlatformModelName: item.PlatformModelName,
			UpstreamModelName: item.UpstreamModelName,
			Protocol:          item.Protocol,
			KindsJSON:         item.KindsJSON,
			Status:            item.Status,
			Priority:          item.Priority,
		})
	}

	data, err := h.service.ImportUpstreamModels(c.Request.Context(), upstreamID, appchannel.ImportUpstreamModelsInput{
		Items: items,
	})
	if err != nil {
		switch {
		case errors.Is(err, appchannel.ErrUpstreamNotFound):
			response.Error(c, http.StatusNotFound, "upstream not found")
		case errors.Is(err, appchannel.ErrInvalidAdapter):
			response.ErrorFrom(c, http.StatusBadRequest, err)
		case errors.Is(err, appchannel.ErrInvalidPlatformModelName):
			response.ErrorFrom(c, http.StatusBadRequest, err)
		case errors.Is(err, appchannel.ErrNoActiveKey):
			response.ErrorFrom(c, http.StatusBadRequest, err)
		case errors.Is(err, appchannel.ErrRemoteModelsUnavailable):
			response.ErrorFrom(c, http.StatusBadGateway, err)
		default:
			response.ErrorFrom(c, http.StatusInternalServerError, err)
		}
		return
	}
	response.Success(c, toImportUpstreamModelsResponse(*data))
}

// ---------------------------------------------------------------------------
// 模型管理
// ---------------------------------------------------------------------------

// ListModels godoc
// @Summary 管理员查询模型目录
// @Description 管理员分页查询平台模型目录，可按 only_active 过滤
// @Tags llm
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Param only_active query bool false "仅查询启用模型"
// @Param q query string false "搜索关键词"
// @Param status query string false "状态：active/inactive"
// @Param vendor query string false "模型厂商"
// @Param protocol query string false "接口协议"
// @Param sort query string false "排序：sortOrder_asc/updated_desc/id_desc/platformModelName_asc/sourceCount_desc"
// @Success 200 {object} ModelListResponseDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/llm/models [get]
func (h *Handler) ListModels(c *gin.Context) {
	page, pageSize := pageParams(c)
	onlyActive := c.Query("only_active") == "true"
	items, total, err := h.service.ListModels(c.Request.Context(), page, pageSize, onlyActive, appchannel.ListModelsInput{
		Query:    c.Query("q"),
		Status:   c.Query("status"),
		Vendor:   c.Query("vendor"),
		Protocol: c.Query("protocol"),
		Sort:     c.Query("sort"),
	})
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "list models failed")
		return
	}
	results := make([]ModelResponse, 0, len(items))
	for _, item := range items {
		results = append(results, toModelResponse(item))
	}
	response.SuccessPage(c, total, results)
}

// CreateModel godoc
// @Summary 管理员创建模型
// @Description 管理员新增平台模型目录项
// @Tags llm
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body CreateModelRequest true "模型参数"
// @Success 200 {object} CreateModelResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 409 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/llm/models [post]
func (h *Handler) CreateModel(c *gin.Context) {
	var req CreateModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}

	item, err := h.service.CreateModel(c.Request.Context(), appchannel.CreateModelInput{
		PlatformModelName: req.PlatformModelName,
		Vendor:            req.Vendor,
		KindsJSON:         req.KindsJSON,
		Icon:              req.Icon,
		CapabilitiesJSON:  req.CapabilitiesJSON,
		SystemPrompt:      req.SystemPrompt,
		Status:            req.Status,
		Description:       req.Description,
	})
	if err != nil {
		switch {
		case errors.Is(err, appchannel.ErrDuplicatePlatformModelName):
			response.Error(c, http.StatusConflict, "platform model name already exists")
		case errors.Is(err, appchannel.ErrInvalidJSONConfig):
			response.Error(c, http.StatusBadRequest, "invalid json config")
		case errors.Is(err, appchannel.ErrInvalidKinds):
			response.Error(c, http.StatusBadRequest, "invalid kinds")
		case errors.Is(err, appchannel.ErrSystemPromptTooLong):
			response.Error(c, http.StatusBadRequest, "system prompt too long")
		case errors.Is(err, appchannel.ErrInvalidPlatformModelName):
			response.Error(c, http.StatusBadRequest, "invalid platform model name")
		default:
			response.Error(c, http.StatusInternalServerError, "create model failed")
		}
		return
	}
	response.Success(c, ModelDataResponse{Model: toModelResponse(*item)})
}

// UpdateModel godoc
// @Summary 管理员更新模型
// @Description 管理员更新平台模型目录项
// @Tags llm
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "模型ID"
// @Param body body UpdateModelRequest true "模型参数"
// @Success 200 {object} UpdateModelResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/llm/models/{id} [patch]
func (h *Handler) UpdateModel(c *gin.Context) {
	modelID, err := uintParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid model id")
		return
	}

	var req UpdateModelRequest
	if err = c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}

	item, err := h.service.UpdateModel(c.Request.Context(), modelID, appchannel.UpdateModelInput{
		PlatformModelName: req.PlatformModelName,
		Vendor:            req.Vendor,
		KindsJSON:         req.KindsJSON,
		Icon:              req.Icon,
		CapabilitiesJSON:  req.CapabilitiesJSON,
		SystemPrompt:      req.SystemPrompt,
		Status:            req.Status,
		Description:       req.Description,
	})
	if err != nil {
		switch {
		case errors.Is(err, appchannel.ErrModelNotFound):
			response.Error(c, http.StatusNotFound, "model not found")
		case errors.Is(err, appchannel.ErrInvalidJSONConfig):
			response.Error(c, http.StatusBadRequest, "invalid json config")
		case errors.Is(err, appchannel.ErrInvalidKinds):
			response.Error(c, http.StatusBadRequest, "invalid kinds")
		case errors.Is(err, appchannel.ErrSystemPromptTooLong):
			response.Error(c, http.StatusBadRequest, "system prompt too long")
		case errors.Is(err, appchannel.ErrInvalidPlatformModelName):
			response.Error(c, http.StatusBadRequest, "invalid platform model name")
		default:
			response.Error(c, http.StatusInternalServerError, "update model failed")
		}
		return
	}
	response.Success(c, ModelDataResponse{Model: toModelResponse(*item)})
}

// ReorderModels godoc
// @Summary 管理员调整模型顺序
// @Description 管理员调整平台模型在用户侧模型选择器中的展示顺序
// @Tags llm
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body ReorderModelsRequest true "模型 ID 顺序"
// @Success 200 {object} response.SuccessDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/llm/models/order [post]
func (h *Handler) ReorderModels(c *gin.Context) {
	var req ReorderModelsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}

	if err := h.service.ReorderModels(c.Request.Context(), req.ModelIDs); err != nil {
		switch {
		case errors.Is(err, appchannel.ErrInvalidModelOrder):
			response.Error(c, http.StatusBadRequest, "invalid model order")
		case errors.Is(err, appchannel.ErrModelNotFound):
			response.Error(c, http.StatusNotFound, "model not found")
		default:
			response.Error(c, http.StatusInternalServerError, "reorder models failed")
		}
		return
	}
	response.Success(c, nil)
}

// DeleteModel godoc
// @Summary 管理员删除模型
// @Description 管理员删除平台模型目录项及其关联路由绑定
// @Tags llm
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "模型ID"
// @Success 200 {object} response.SuccessDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/llm/models/{id} [delete]
func (h *Handler) DeleteModel(c *gin.Context) {
	modelID, err := uintParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid model id")
		return
	}

	if err = h.service.DeleteModel(c.Request.Context(), modelID); err != nil {
		if errors.Is(err, appchannel.ErrModelNotFound) {
			response.Error(c, http.StatusNotFound, "model not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, "delete model failed")
		return
	}
	response.Success(c, nil)
}

// BatchDeleteModels godoc
// @Summary 管理员批量删除模型
// @Description 管理员批量删除模型目录及其关联路由绑定，保留上游
// @Tags llm
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body BatchDeleteRequest true "批量删除请求"
// @Success 200 {object} BatchDeleteResponseDoc
// @Failure 400 {object} ErrorDoc
// @Router /admin/llm/models/batch-delete [post]
func (h *Handler) BatchDeleteModels(c *gin.Context) {
	var req BatchDeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}

	response.Success(c, toBatchDeleteResponse(*h.service.BatchDeleteModels(c.Request.Context(), req.IDs)))
}

// ListModelUpstreamSources godoc
// @Summary 管理员查询模型上游来源
// @Description 管理员分页查询指定模型在各上游上的路由来源
// @Tags llm
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "模型ID"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} ModelUpstreamSourceListResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/llm/models/{id}/sources [get]
func (h *Handler) ListModelUpstreamSources(c *gin.Context) {
	modelID, err := uintParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid model id")
		return
	}

	page, pageSize := pageParams(c)
	items, total, err := h.service.ListModelUpstreamSources(c.Request.Context(), modelID, page, pageSize)
	if err != nil {
		if errors.Is(err, appchannel.ErrModelNotFound) {
			response.Error(c, http.StatusNotFound, "model not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, "list model upstream sources failed")
		return
	}
	results := make([]ModelUpstreamSourceResponse, 0, len(items))
	for _, item := range items {
		results = append(results, toModelUpstreamSourceResponse(item))
	}
	response.SuccessPage(c, total, results)
}

// UpdateModelUpstreamSource godoc
// @Summary 管理员更新模型上游来源
// @Description 管理员快速启停指定模型在某上游上的来源
// @Tags llm
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "模型ID"
// @Param route_id path int true "路由绑定ID"
// @Param body body UpdateModelUpstreamSourceRequest true "来源参数"
// @Success 200 {object} UpdateModelUpstreamSourceResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/llm/models/{id}/sources/{route_id} [patch]
func (h *Handler) UpdateModelUpstreamSource(c *gin.Context) {
	modelID, err := uintParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid model id")
		return
	}
	routeID, err := uintParam(c, "route_id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid route id")
		return
	}

	var req UpdateModelUpstreamSourceRequest
	if err = c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}

	item, err := h.service.UpdateModelUpstreamSource(c.Request.Context(), modelID, routeID, appchannel.UpdateModelUpstreamSourceInput{
		Protocol: req.Protocol,
		Status:   req.Status,
		Priority: req.Priority,
		Weight:   req.Weight,
	})
	if err != nil {
		switch {
		case errors.Is(err, appchannel.ErrModelNotFound):
			response.Error(c, http.StatusNotFound, "model not found")
		case errors.Is(err, appchannel.ErrUpstreamModelNotFound):
			response.Error(c, http.StatusNotFound, "upstream model not found")
		default:
			response.Error(c, http.StatusInternalServerError, "update model upstream source failed")
		}
		return
	}
	response.Success(c, ModelUpstreamSourceDataResponse{Source: toModelUpstreamSourceResponse(*item)})
}

// ---------------------------------------------------------------------------
// 全局设置管理
// ---------------------------------------------------------------------------

// ListLLMSettings godoc
// @Summary 管理员查询全局设置
// @Description 管理员查询 LLM 全局设置列表
// @Tags llm
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.SuccessDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/llm/settings [get]
func (h *Handler) ListLLMSettings(c *gin.Context) {
	items, err := h.service.ListLLMSettings(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "list settings failed")
		return
	}
	results := make([]LLMSettingResponse, 0, len(items))
	for _, item := range items {
		results = append(results, toLLMSettingResponse(item))
	}
	response.Success(c, results)
}

// UpdateLLMSetting godoc
// @Summary 管理员更新全局设置
// @Description 管理员更新指定 LLM 全局设置项
// @Tags llm
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param key path string true "设置键"
// @Param body body map[string]string true "设置值 {\"value\": \"...\"}"
// @Success 200 {object} response.SuccessDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/llm/settings/{key} [patch]
func (h *Handler) UpdateLLMSetting(c *gin.Context) {
	key := c.Param("key")
	if key == "" {
		response.Error(c, http.StatusBadRequest, "invalid setting key")
		return
	}

	var body struct {
		Value string `json:"value" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}

	item, err := h.service.UpdateLLMSetting(c.Request.Context(), key, body.Value)
	if err != nil {
		switch {
		case errors.Is(err, appchannel.ErrLLMSettingNotFound):
			response.Error(c, http.StatusNotFound, "setting not found")
		case errors.Is(err, appchannel.ErrInvalidJSONConfig):
			response.Error(c, http.StatusBadRequest, "invalid json config")
		default:
			response.Error(c, http.StatusInternalServerError, "update setting failed")
		}
		return
	}
	response.Success(c, toLLMSettingResponse(*item))
}

// ---------------------------------------------------------------------------
// HTTP 辅助
// ---------------------------------------------------------------------------

func pageParams(c *gin.Context) (int, int) {
	page := 1
	pageSize := 20
	if raw := c.Query("page"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			page = parsed
		}
	}
	if raw := c.Query("page_size"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			if parsed > 100 {
				parsed = 100
			}
			pageSize = parsed
		}
	}
	return page, pageSize
}

func uintParam(c *gin.Context, key string) (uint, error) {
	value, err := strconv.ParseUint(c.Param(key), 10, 64)
	if err != nil {
		return 0, err
	}
	return uint(value), nil
}
