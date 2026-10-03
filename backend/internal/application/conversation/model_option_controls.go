package conversation

import (
	"strings"

	domainchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/channel"
)

// userChatOptionKeys 是普通用户在聊天请求 options 中仍可提交的键：原生工具选择（由模型配置收敛）。
// 其余参数只能通过模型控件选择，参数内容由管理员声明。
var userChatOptionKeys = map[string]struct{}{"tools": {}}

// restrictUserChatOptions 收敛聊天请求的 options 与 controls：管理员（allowRaw）保留高级 JSON；普通用户只保留
// 原生工具选择，旧客户端的 options.reasoning_effort 转为思考强度控件选择（控件未提交时）。
func restrictUserChatOptions(options map[string]any, controls map[string]any, allowRaw bool) (map[string]any, map[string]any) {
	if allowRaw {
		return options, controls
	}
	if text, ok := options[reasoningEffortOptionKey].(string); ok && strings.TrimSpace(text) != "" {
		if _, selected := controls[domainchannel.ModelControlReasoningID]; !selected {
			next := make(map[string]any, len(controls)+1)
			for key, value := range controls {
				next[key] = value
			}
			next[domainchannel.ModelControlReasoningID] = strings.TrimSpace(text)
			controls = next
		}
	}
	restricted := make(map[string]any, len(userChatOptionKeys))
	for key := range userChatOptionKeys {
		if value, ok := options[key]; ok {
			restricted[key] = value
		}
	}
	if len(restricted) == 0 {
		return nil, controls
	}
	return restricted, controls
}
