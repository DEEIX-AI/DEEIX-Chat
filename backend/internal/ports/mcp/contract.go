// Package mcp 定义 MCP 工具调用端口的数据契约。
package mcp

import "encoding/json"

// CallConfig 定义 MCP 调用配置。
type CallConfig struct {
	BaseURL   string
	AuthToken string
	TimeoutMS int
	Headers   map[string]string
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
