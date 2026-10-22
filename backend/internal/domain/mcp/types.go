package mcp

import "time"

const (
	AttachmentInputModeNone  = "none"
	AttachmentInputModeImage = "image"

	AttachmentEncodingBase64  = "base64"
	AttachmentEncodingDataURL = "data_url"
)

// Server 表示管理员维护的 MCP 服务。
type Server struct {
	ID                                   uint
	Name                                 string
	BaseURL                              string
	AuthTokenEnc                         string
	HeadersJSON                          string
	Status                               string
	FileCreateEnabled                    bool
	FileCreateEpoch                      uint
	SortOrder                            int
	ToolCount                            int
	ActiveToolCount                      int
	RequiresToolMetadataSyncConfirmation bool
	LastSyncedAt                         *time.Time
	LastError                            string
	CreatedAt                            time.Time
	UpdatedAt                            time.Time
}

type ServerWithTools struct {
	Server Server
	Tools  []Tool
}

// FileCreateGrant is a DEEIX-issued capability for one file from one tool call.
// TokenHash is never a credential; the random bearer is only sent to that MCP.
type FileCreateGrant struct {
	TokenHash   string
	ServerID    uint
	ToolID      uint
	UserID      uint
	Epoch       uint
	CallID      string
	RequestID   string
	ExpiresAt   time.Time
	CreatedAt   time.Time
	Fingerprint string
	CandidateID string
	StoragePath string
	FileID      string
	Reused      bool
}

// Tool 表示从 MCP 服务发现并由管理员控制可用性的工具。
type Tool struct {
	ID                       uint
	ServerID                 uint
	ServerName               string
	Name                     string
	DisplayName              string
	Description              string
	InputSchemaJSON          string
	AttachmentInputMode      string
	AttachmentArgument       string
	AttachmentEncoding       string
	AttachmentPromptArgument string
	// PriceNanousd 为管理员配置的单次调用价格（nano USD），0 表示该工具不单独计费。
	PriceNanousd int64
	Status       string
	SortOrder    int
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
