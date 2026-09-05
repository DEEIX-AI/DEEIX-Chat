package mcp

import "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/apperr"

// 由 transport 层直接判定的请求错误（路径参数、必填校验等）。
// 错误码与文案是前端依赖的 API 契约，改动需同步 frontend/i18n/messages/*/errors.json。
var (
	errInvalidMCPServerID                 = apperr.New("mcp.server.invalid_id", "invalid mcp server id")
	errInvalidMCPToolID                   = apperr.New("mcp.tool.invalid_id", "invalid mcp tool id")
	errInvalidOverwriteCustomizedMetadata = apperr.New("request.invalid_overwrite_customized_metadata", "invalid overwrite_customized_metadata")
)
