package repository

import (
	"context"

	domainmcp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/mcp"
)

// CreateMCPServerInput 定义创建 MCP 服务字段。
type CreateMCPServerInput struct {
	Name         string
	BaseURL      string
	AuthTokenEnc string
	HeadersJSON  string
	Status       string
}

// UpdateMCPServerInput 定义更新 MCP 服务字段。
type UpdateMCPServerInput struct {
	Name         *string
	BaseURL      *string
	AuthTokenEnc *string
	HeadersJSON  *string
	Status       *string
	LastError    *string
}

// UpdateMCPToolInput 定义更新 MCP 工具字段。
type UpdateMCPToolInput struct {
	DisplayName *string
	Description *string
	Status      *string
}

type ReorderMCPServerInput struct {
	ServerID uint
	ToolIDs  []uint
}

// MCPRepository 封装 MCP 控制面持久化。
type MCPRepository interface {
	CreateServer(ctx context.Context, input CreateMCPServerInput) (*domainmcp.Server, error)
	UpdateServer(ctx context.Context, serverID uint, input UpdateMCPServerInput) (*domainmcp.Server, error)
	ListServers(ctx context.Context) ([]domainmcp.Server, error)
	GetServer(ctx context.Context, serverID uint) (*domainmcp.Server, error)
	DeleteServer(ctx context.Context, serverID uint) error
	ReplaceServerTools(ctx context.Context, serverID uint, tools []domainmcp.Tool) error
	ListTools(ctx context.Context, serverID uint, onlyActive bool) ([]domainmcp.Tool, error)
	ListToolsByIDs(ctx context.Context, toolIDs []uint) ([]domainmcp.Tool, error)
	UpdateTool(ctx context.Context, toolID uint, input UpdateMCPToolInput) (*domainmcp.Tool, error)
	UpdateServerToolsStatus(ctx context.Context, serverID uint, toolIDs []uint, status string) ([]domainmcp.Tool, error)
	ReorderServersWithTools(ctx context.Context, order []ReorderMCPServerInput) ([]domainmcp.ServerWithTools, error)
}
