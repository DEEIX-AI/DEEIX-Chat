package mcp

import "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/apperr"

// MCP 服务与工具管理的错误哨兵，错误码与文案是前端依赖的 API 契约。
var (
	ErrInvalidServerName           = apperr.New("mcp.invalid_server_name", "invalid mcp server name")
	ErrInvalidServerBaseURL        = apperr.New("mcp.invalid_server_base_url", "invalid mcp server base url")
	ErrInvalidServerStatus         = apperr.New("mcp.invalid_server_status", "invalid mcp server status")
	ErrInvalidServerHeaders        = apperr.New("mcp.invalid_server_headers", "invalid mcp server headers json")
	ErrInvalidToolStatus           = apperr.New("mcp.invalid_tool_status", "invalid mcp tool status")
	ErrInvalidToolName             = apperr.New("mcp.invalid_tool_name", "invalid mcp tool display name")
	ErrInvalidToolDesc             = apperr.New("mcp.invalid_tool_description", "invalid mcp tool description")
	ErrInvalidToolAttachmentConfig = apperr.NewMasked("mcp.invalid_attachment_configuration", "invalid MCP tool attachment configuration", "invalid mcp tool attachment configuration")
	ErrInvalidToolSelection        = apperr.New("mcp.invalid_tool_selection", "invalid mcp tool selection")
	ErrInvalidToolPrice            = apperr.New("request.invalid_mcp_tool_price", "invalid mcp tool price")
	ErrMCPClientUnavailable        = apperr.New("mcp.client_unavailable", "mcp client unavailable")
	// ErrServerLimitExceeded MCP 服务数量超限。
	ErrServerLimitExceeded = apperr.New("mcp.server_limit_exceeded", "mcp server limit exceeded")
)
