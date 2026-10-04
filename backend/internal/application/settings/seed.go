package settings

import (
	domainsettings "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/settings"
)

const (
	defaultAllowedMIMETypes = "image/jpeg,image/png,image/webp,image/gif,video/mp4,video/webm,video/quicktime,video/mpeg,audio/mpeg,audio/wav,audio/mp4,audio/aac,audio/ogg,audio/flac,audio/aiff,text/plain,text/markdown,text/csv,text/yaml,application/json,application/yaml,application/x-yaml,application/toml,application/pdf,application/msword,application/vnd.openxmlformats-officedocument.wordprocessingml.document,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet,application/vnd.ms-excel"
	defaultRAGModel         = "sentence-transformers/all-MiniLM-L6-v2"
)

// legacyDefaultAllowedMIMETypes 是历史版本的白名单默认值。数据库中仍是其中之一（管理员未改动过）时，
// 启动时升级为当前默认值；改动过的白名单保持不变。
var legacyDefaultAllowedMIMETypes = []string{
	// 不含视频。
	"image/jpeg,image/png,image/webp,image/gif,text/plain,text/markdown,text/csv,text/yaml,application/json,application/yaml,application/x-yaml,application/toml,application/pdf,application/msword,application/vnd.openxmlformats-officedocument.wordprocessingml.document,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet,application/vnd.ms-excel",
	// 含 mp4 / webm，不含音频。
	"image/jpeg,image/png,image/webp,image/gif,video/mp4,video/webm,text/plain,text/markdown,text/csv,text/yaml,application/json,application/yaml,application/x-yaml,application/toml,application/pdf,application/msword,application/vnd.openxmlformats-officedocument.wordprocessingml.document,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet,application/vnd.ms-excel",
}

// obsoleteSettings 列出已从注册表移除、启动时需要从数据库清理的历史配置项。
func obsoleteSettings() []domainsettings.SystemSetting {
	return []domainsettings.SystemSetting{
		{Namespace: "mcp", Key: "mcp_connect_timeout_ms"},
		{Namespace: "mcp", Key: "mcp_tool_timeout_ms"},
		{Namespace: "chat", Key: "context_max_input_tokens"},
		{Namespace: "chat", Key: "context_compact_trigger_tokens"},
		{Namespace: "chat", Key: "process_trace_visible_to_user"},
		{Namespace: "chat", Key: "process_trace_store_upstream_think"},
		{Namespace: "chat", Key: "models_dev_enabled"},
		{Namespace: "chat", Key: "models_dev_url"},
	}
}
