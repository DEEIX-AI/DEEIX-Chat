package model

import "time"

// MCPServer 存储管理员配置的 MCP 服务。
type MCPServer struct {
	ControlPlaneModel
	Name         string     `gorm:"size:128;not null;default:'';comment:MCP服务名称"`
	BaseURL      string     `gorm:"size:512;not null;default:'';comment:MCP服务地址"`
	AuthTokenEnc string     `gorm:"type:text;not null;default:'';comment:加密后的鉴权Token"`
	HeadersJSON  string     `gorm:"type:text;not null;default:'{}';comment:附加请求头JSON"`
	Status       string     `gorm:"size:32;not null;default:'active';index:idx_mcp_servers_status;comment:服务状态(active/inactive)"`
	SortOrder    int        `gorm:"not null;default:0;index:idx_mcp_servers_sort_order;comment:展示顺序"`
	ToolCount    int        `gorm:"not null;default:0;comment:最近发现工具数量"`
	LastSyncedAt *time.Time `gorm:"comment:最近同步工具时间"`
	LastError    string     `gorm:"type:text;not null;default:'';comment:最近同步或调用错误"`
}

func (MCPServer) TableName() string {
	return "mcp_servers"
}

// MCPTool 存储 MCP 服务发现的工具。
type MCPTool struct {
	ControlPlaneModel
	ServerID        uint   `gorm:"not null;default:0;uniqueIndex:idx_mcp_tools_server_name,priority:1;index:idx_mcp_tools_server_id;comment:MCP服务ID"`
	Name            string `gorm:"size:160;not null;default:'';uniqueIndex:idx_mcp_tools_server_name,priority:2;comment:工具名称"`
	DisplayName     string `gorm:"size:160;not null;default:'';comment:展示名称"`
	Description     string `gorm:"type:text;not null;default:'';comment:工具说明"`
	InputSchemaJSON string `gorm:"type:text;not null;default:'{}';comment:输入JSON Schema"`
	Status          string `gorm:"size:32;not null;default:'inactive';index:idx_mcp_tools_status;comment:工具状态(active/inactive)"`
	SortOrder       int    `gorm:"not null;default:0;index:idx_mcp_tools_sort_order;comment:展示顺序"`
}

func (MCPTool) TableName() string {
	return "mcp_tools"
}
