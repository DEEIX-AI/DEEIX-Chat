package settings

import (
	domainsettings "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/settings"
)

const (
	legacyDefaultAllowedMIMETypes = "image/jpeg,image/png,image/webp,image/gif,text/plain,text/markdown,text/csv,text/yaml,application/json,application/yaml,application/x-yaml,application/toml,application/pdf,application/msword,application/vnd.openxmlformats-officedocument.wordprocessingml.document,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet,application/vnd.ms-excel"
	defaultAllowedMIMETypes       = "image/jpeg,image/png,image/webp,image/gif,video/mp4,video/webm,text/plain,text/markdown,text/csv,text/yaml,application/json,application/yaml,application/x-yaml,application/toml,application/pdf,application/msword,application/vnd.openxmlformats-officedocument.wordprocessingml.document,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet,application/vnd.ms-excel"
	defaultRAGModel               = "sentence-transformers/all-MiniLM-L6-v2"
)

// obsoleteSettings 列出已从注册表移除、启动时需要从数据库清理的历史配置项。
func obsoleteSettings() []domainsettings.SystemSetting {
	return []domainsettings.SystemSetting{
		{Namespace: "mcp", Key: "mcp_connect_timeout_ms"},
		{Namespace: "mcp", Key: "mcp_tool_timeout_ms"},
		{Namespace: "chat", Key: "context_max_input_tokens"},
		{Namespace: "chat", Key: "context_compact_trigger_tokens"},
	}
}
