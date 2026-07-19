package mcp

import (
	"errors"
	"net/http"
	"strconv"

	appmcp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/mcp"
	domainmcp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/mcp"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/security"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/middleware"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *appmcp.Service
}

func NewHandler(service *appmcp.Service) *Handler {
	return &Handler{service: service}
}

// ListServers godoc
// @Summary 获取 MCP 服务列表
// @Description 管理员查看已配置的 MCP 服务及其工具统计
// @Tags admin-mcp
// @Produce json
// @Security BearerAuth
// @Success 200 {object} ServerListResponseDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/mcp/servers [get]
func (h *Handler) ListServers(c *gin.Context) {
	items, err := h.service.ListServers(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "list mcp servers failed")
		return
	}
	results := make([]ServerResponse, 0, len(items))
	for _, item := range items {
		results = append(results, toServerResponse(item))
	}
	response.Success(c, ServerListResponse{Results: results})
}

// ListAvailableTools godoc
// @Summary 获取可用 MCP 工具
// @Description 获取当前聊天侧可选择的 MCP 工具
// @Tags mcp
// @Produce json
// @Security BearerAuth
// @Success 200 {object} ToolListResponseDoc
// @Failure 500 {object} ErrorDoc
// @Router /mcp/tools [get]
func (h *Handler) ListAvailableTools(c *gin.Context) {
	items, err := h.service.ListAvailableTools(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "list mcp tools failed")
		return
	}
	results := make([]ToolResponse, 0, len(items))
	for _, item := range items {
		results = append(results, toToolResponse(item))
	}
	response.Success(c, ToolListResponse{Results: results})
}

// CreateServer godoc
// @Summary 创建 MCP 服务
// @Description 管理员创建一个 MCP 服务配置
// @Tags admin-mcp
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body CreateServerRequest true "MCP 服务配置"
// @Success 200 {object} ServerDataResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/mcp/servers [post]
func (h *Handler) CreateServer(c *gin.Context) {
	var req CreateServerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	item, err := h.service.CreateServer(c.Request.Context(), appmcp.ServerInput{
		Name:        req.Name,
		BaseURL:     req.BaseURL,
		AuthToken:   req.AuthToken,
		HeadersJSON: req.HeadersJSON,
		Status:      req.Status,
	})
	if err != nil {
		writeServiceError(c, err)
		return
	}
	response.Success(c, ServerDataResponse{Server: toServerResponse(*item)})
}

// UpdateServer godoc
// @Summary 更新 MCP 服务
// @Description 管理员更新一个 MCP 服务配置
// @Tags admin-mcp
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "MCP 服务 ID"
// @Param body body CreateServerRequest true "MCP 服务配置"
// @Success 200 {object} ServerDataResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/mcp/servers/{id} [patch]
func (h *Handler) UpdateServer(c *gin.Context) {
	serverID, ok := parseIDParam(c, "id", "mcp server")
	if !ok {
		return
	}
	var req CreateServerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	item, err := h.service.UpdateServer(c.Request.Context(), serverID, appmcp.ServerInput{
		Name:        req.Name,
		BaseURL:     req.BaseURL,
		AuthToken:   req.AuthToken,
		HeadersJSON: req.HeadersJSON,
		Status:      req.Status,
	})
	if err != nil {
		writeServiceError(c, err)
		return
	}
	response.Success(c, ServerDataResponse{Server: toServerResponse(*item)})
}

// DeleteServer godoc
// @Summary 删除 MCP 服务
// @Description 管理员删除一个 MCP 服务及其工具
// @Tags admin-mcp
// @Produce json
// @Security BearerAuth
// @Param id path int true "MCP 服务 ID"
// @Success 200 {object} DeleteServerResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/mcp/servers/{id} [delete]
func (h *Handler) DeleteServer(c *gin.Context) {
	serverID, ok := parseIDParam(c, "id", "mcp server")
	if !ok {
		return
	}
	if err := h.service.DeleteServer(c.Request.Context(), serverID); err != nil {
		response.Error(c, http.StatusInternalServerError, "delete mcp server failed")
		return
	}
	response.Success(c, DeleteServerResponse{Deleted: true})
}

// SyncServerTools godoc
// @Summary 同步 MCP 工具
// @Description 管理员从 MCP 服务同步工具定义
// @Tags admin-mcp
// @Produce json
// @Security BearerAuth
// @Param id path int true "MCP 服务 ID"
// @Success 200 {object} ToolListResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/mcp/servers/{id}/sync [post]
func (h *Handler) SyncServerTools(c *gin.Context) {
	serverID, ok := parseIDParam(c, "id", "mcp server")
	if !ok {
		return
	}
	items, err := h.service.SyncServerTools(c.Request.Context(), appmcp.SyncServerToolsInput{
		ServerID:  serverID,
		RequestID: middleware.MustRequestID(c),
	})
	if err != nil {
		writeServiceError(c, err)
		return
	}
	results := make([]ToolResponse, 0, len(items))
	for _, item := range items {
		results = append(results, toToolResponse(item))
	}
	response.Success(c, ToolListResponse{Results: results})
}

// ListServerTools godoc
// @Summary 获取 MCP 服务工具
// @Description 管理员查看指定 MCP 服务已同步的工具
// @Tags admin-mcp
// @Produce json
// @Security BearerAuth
// @Param id path int true "MCP 服务 ID"
// @Success 200 {object} ToolListResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/mcp/servers/{id}/tools [get]
func (h *Handler) ListServerTools(c *gin.Context) {
	serverID, ok := parseIDParam(c, "id", "mcp server")
	if !ok {
		return
	}
	items, err := h.service.ListTools(c.Request.Context(), serverID, false)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "list mcp tools failed")
		return
	}
	results := make([]ToolResponse, 0, len(items))
	for _, item := range items {
		results = append(results, toToolResponse(item))
	}
	response.Success(c, ToolListResponse{Results: results})
}

// UpdateTool godoc
// @Summary 更新 MCP 工具
// @Description 管理员更新 MCP 工具的展示信息或状态
// @Tags admin-mcp
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "MCP 工具 ID"
// @Param body body UpdateToolRequest true "MCP 工具配置"
// @Success 200 {object} ToolResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/mcp/tools/{id} [patch]
func (h *Handler) UpdateTool(c *gin.Context) {
	toolID, ok := parseIDParam(c, "id", "mcp tool")
	if !ok {
		return
	}
	var req UpdateToolRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	item, err := h.service.UpdateTool(c.Request.Context(), toolID, appmcp.ToolInput{
		DisplayName: req.DisplayName,
		Description: req.Description,
		Status:      req.Status,
	})
	if err != nil {
		writeServiceError(c, err)
		return
	}
	response.Success(c, toToolResponse(*item))
}

// UpdateServerToolsStatus godoc
// @Summary 批量更新 MCP 工具状态
// @Description 管理员批量启用或停用指定 MCP 服务的工具
// @Tags admin-mcp
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "MCP 服务 ID"
// @Param body body UpdateServerToolsStatusRequest true "MCP 工具状态"
// @Success 200 {object} ToolListResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/mcp/servers/{id}/tools/status [patch]
func (h *Handler) UpdateServerToolsStatus(c *gin.Context) {
	serverID, ok := parseIDParam(c, "id", "mcp server")
	if !ok {
		return
	}
	var req UpdateServerToolsStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	items, err := h.service.UpdateServerToolsStatus(c.Request.Context(), serverID, req.ToolIDs, req.Status)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	results := make([]ToolResponse, 0, len(items))
	for _, item := range items {
		results = append(results, toToolResponse(item))
	}
	response.Success(c, ToolListResponse{Results: results})
}

// ReorderServers godoc
// @Summary 调整 MCP 服务及工具顺序
// @Description 管理员保存 MCP 服务及其工具的展示顺序
// @Tags admin-mcp
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body ReorderServersRequest true "MCP 排序配置"
// @Success 200 {object} ServerToolOrderListResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/mcp/servers/order [patch]
func (h *Handler) ReorderServers(c *gin.Context) {
	var req ReorderServersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	input := make([]appmcp.ReorderServerInput, 0, len(req.Servers))
	for _, item := range req.Servers {
		input = append(input, appmcp.ReorderServerInput{
			ServerID: item.ServerID,
			ToolIDs:  item.ToolIDs,
		})
	}
	items, err := h.service.ReorderServersWithTools(c.Request.Context(), input)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	results := make([]ServerToolOrderResponse, 0, len(items))
	for _, item := range items {
		tools := make([]ToolResponse, 0, len(item.Tools))
		for _, tool := range item.Tools {
			tools = append(tools, toToolResponse(tool))
		}
		results = append(results, ServerToolOrderResponse{
			Server: toServerResponse(item.Server),
			Tools:  tools,
		})
	}
	response.Success(c, ServerToolOrderListResponse{Results: results})
}

func parseIDParam(c *gin.Context, key string, resource string) (uint, bool) {
	raw := c.Param(key)
	parsed, err := strconv.ParseUint(raw, 10, strconv.IntSize)
	if err != nil || parsed == 0 {
		response.Error(c, http.StatusBadRequest, "invalid "+resource+" id")
		return 0, false
	}
	return uint(parsed), true
}

func writeServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, appmcp.ErrInvalidServerName),
		errors.Is(err, appmcp.ErrInvalidServerBaseURL),
		errors.Is(err, appmcp.ErrInvalidServerStatus),
		errors.Is(err, appmcp.ErrInvalidServerHeaders),
		errors.Is(err, appmcp.ErrInvalidToolStatus),
		errors.Is(err, appmcp.ErrInvalidToolName),
		errors.Is(err, appmcp.ErrInvalidToolDesc),
		errors.Is(err, appmcp.ErrInvalidToolSelection):
		response.ErrorFrom(c, http.StatusBadRequest, err)
	default:
		response.ErrorFrom(c, http.StatusInternalServerError, err)
	}
}

func toServerResponse(item domainmcp.Server) ServerResponse {
	return ServerResponse{
		ID:              item.ID,
		Name:            item.Name,
		BaseURL:         item.BaseURL,
		HeadersJSON:     security.RedactHeadersJSON(item.HeadersJSON),
		Status:          item.Status,
		SortOrder:       item.SortOrder,
		ToolCount:       item.ToolCount,
		ActiveToolCount: item.ActiveToolCount,
		LastSyncedAt:    item.LastSyncedAt,
		LastError:       item.LastError,
		CreatedAt:       item.CreatedAt,
		UpdatedAt:       item.UpdatedAt,
	}
}

func toToolResponse(item domainmcp.Tool) ToolResponse {
	return ToolResponse{
		ID:              item.ID,
		ServerID:        item.ServerID,
		ServerName:      item.ServerName,
		Name:            item.Name,
		DisplayName:     item.DisplayName,
		Description:     item.Description,
		InputSchemaJSON: item.InputSchemaJSON,
		Status:          item.Status,
		SortOrder:       item.SortOrder,
		CreatedAt:       item.CreatedAt,
		UpdatedAt:       item.UpdatedAt,
	}
}
