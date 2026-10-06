package models

import "time"

// MCPServer 存储管理员配置的 MCP 服务。
type MCPServer struct {
	ControlPlaneModel
	Name              string     `gorm:"size:128;not null;default:'';comment:MCP服务名称"`
	BaseURL           string     `gorm:"size:512;not null;default:'';comment:MCP服务地址"`
	AuthTokenEnc      string     `gorm:"type:text;not null;default:'';comment:加密后的鉴权Token"`
	HeadersJSON       string     `gorm:"type:text;not null;default:'{}';comment:附加请求头JSON"`
	Status            string     `gorm:"size:32;not null;default:'active';index:idx_mcp_servers_status;comment:服务状态(active/inactive)"`
	FileCreateEnabled bool       `gorm:"not null;default:false;comment:允许工具产物创建用户文件"`
	FileCreateEpoch   uint       `gorm:"not null;default:0;comment:文件创建授权撤销代数"`
	SortOrder         int        `gorm:"not null;default:0;index:idx_mcp_servers_sort_order;comment:展示顺序"`
	ToolCount         int        `gorm:"not null;default:0;comment:最近发现工具数量"`
	LastSyncedAt      *time.Time `gorm:"comment:最近同步工具时间"`
	LastError         string     `gorm:"type:text;not null;default:'';comment:最近同步或调用错误"`
}

func (MCPServer) TableName() string {
	return "mcp_servers"
}

// MCPTool 存储 MCP 服务发现的工具。
type MCPTool struct {
	ControlPlaneModel
	ServerID                 uint   `gorm:"not null;default:0;uniqueIndex:idx_mcp_tools_server_name,priority:1;index:idx_mcp_tools_server_id;comment:MCP服务ID"`
	Name                     string `gorm:"size:160;not null;default:'';uniqueIndex:idx_mcp_tools_server_name,priority:2;comment:工具名称"`
	DisplayName              string `gorm:"size:160;not null;default:'';comment:展示名称"`
	Description              string `gorm:"type:text;not null;default:'';comment:工具说明"`
	MetadataCustomized       *bool  `gorm:"comment:名称或说明是否由管理员修改(NULL表示升级前状态待确认)"`
	InputSchemaJSON          string `gorm:"type:text;not null;default:'{}';comment:输入JSON Schema"`
	AttachmentInputMode      string `gorm:"size:32;not null;default:'none';comment:附件输入模式(none/image)"`
	AttachmentArgument       string `gorm:"size:128;not null;default:'';comment:附件内容对应的顶层参数名"`
	AttachmentEncoding       string `gorm:"size:32;not null;default:'';comment:附件编码(base64/data_url)"`
	AttachmentPromptArgument string `gorm:"size:128;not null;default:'';comment:用户提示词对应的顶层参数名"`
	PriceNanousd             int64  `gorm:"not null;default:0;comment:单次调用价格(nano USD),0表示不单独计费"`
	Status                   string `gorm:"size:32;not null;default:'inactive';index:idx_mcp_tools_status;comment:工具状态(active/inactive)"`
	SortOrder                int    `gorm:"not null;default:0;index:idx_mcp_tools_sort_order;comment:展示顺序"`
}

func (MCPTool) TableName() string {
	return "mcp_tools"
}

// MCPFileCreateGrant stores only the hash of a short-lived, one-file capability.
// FileID is committed in the same transaction as file metadata and quota.
type MCPFileCreateGrant struct {
	TokenHash    string    `gorm:"primaryKey;size:64"`
	ServerID     uint      `gorm:"not null;index"`
	ToolID       uint      `gorm:"not null"`
	UserID       uint      `gorm:"not null;index"`
	Epoch        uint      `gorm:"not null"`
	CallID       string    `gorm:"size:36;not null"`
	RequestID    string    `gorm:"type:text;not null;default:''"`
	ExpiresAt    time.Time `gorm:"not null;index"`
	CreatedAt    time.Time `gorm:"not null"`
	MaintainedAt time.Time `gorm:"not null"`
	Fingerprint  string    `gorm:"size:64;not null;default:''"`
	CandidateID  string    `gorm:"size:64;not null"`
	StoragePath  string    `gorm:"size:1024;not null;default:''"`
	FileID       string    `gorm:"size:64;not null;default:''"`
	Reused       bool      `gorm:"not null;default:false"`
}

func (MCPFileCreateGrant) TableName() string { return "mcp_file_create_grants" }
