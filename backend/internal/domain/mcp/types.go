package mcp

import "time"

// Server 表示管理员维护的 MCP 服务。
type Server struct {
	ID              uint
	Name            string
	BaseURL         string
	AuthTokenEnc    string
	HeadersJSON     string
	Status          string
	ToolCount       int
	ActiveToolCount int
	LastSyncedAt    *time.Time
	LastError       string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// Tool 表示从 MCP 服务发现并由管理员控制可用性的工具。
type Tool struct {
	ID              uint
	ServerID        uint
	ServerName      string
	Name            string
	DisplayName     string
	Description     string
	InputSchemaJSON string
	Status          string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
