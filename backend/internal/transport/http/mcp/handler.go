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
