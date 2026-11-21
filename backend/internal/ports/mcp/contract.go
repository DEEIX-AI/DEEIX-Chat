// Package mcp 定义 MCP 工具调用端口的数据契约。
package mcp

import "encoding/json"

// CallConfig 定义 MCP 调用配置。
type CallConfig struct {
	BaseURL   string
	AuthToken string
	TimeoutMS int
	Headers   map[string]string
	// InputSchema 是被调用工具的 inputSchema，用于把 x-mcp-header 标注的参数镜像为 Mcp-Param-* 请求头。
	InputSchema json.RawMessage
}

// CallInput 定义 MCP 工具调用入参。
type CallInput struct {
	ToolName       string
	ArgumentsJSON  string
	UserID         uint
	ConversationID uint
	RequestID      string
}

// Tool 定义 MCP 工具元数据。
type Tool struct {
	Name        string          `json:"name"`
	Title       string          `json:"title,omitempty"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"inputSchema,omitempty"`
}
