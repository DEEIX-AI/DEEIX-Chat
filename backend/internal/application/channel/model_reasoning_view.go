package channel

import (
	"encoding/json"
	"strings"

	domainchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/channel"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
)

// ModelReasoningView 是面向聊天模型选择器的归一化推理强度能力；format 与预算只在后端使用。
type ModelReasoningView struct {
	Levels  []string
	Default string
	// ControlPath 为由旧版 optionControls 推断时被接管的原生参数路径，显式声明时为空。
	ControlPath string
	// Source 为能力来源：explicit / inferred / catalog。catalog 来源的档位只在用户显式选择时下发，
	// 未选择时保持上游默认行为。
	Source string
	// Locked 表示档位被管理员锁定为默认值：lockedOptionPaths 含 reasoning_effort，
	// 或含该格式在任一可用协议上的原生档位路径（与请求链路的锁定规则一致）。
	Locked bool
}

func modelReasoningLocked(capability domainchannel.ReasoningCapability, protocolKeys []string, capabilitiesJSON string) bool {
	var config struct {
		LockedOptionPaths []string `json:"lockedOptionPaths"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(capabilitiesJSON)), &config); err != nil {
		return false
	}
	nativePaths := map[string]struct{}{}
	if capability.ControlPath != "" {
		nativePaths[capability.ControlPath] = struct{}{}
	}
	for _, protocolKey := range protocolKeys {
		if path := domainchannel.ReasoningNativeLevelPath(capability.Format, protocolKey); path != "" {
			nativePaths[path] = struct{}{}
		}
	}
	for _, raw := range config.LockedOptionPaths {
		path := strings.TrimSpace(raw)
		if path == modelReasoningEffortOptionKey {
			return true
		}
		if _, ok := nativePaths[path]; ok {
			return true
		}
	}
	return false
}

// modelReasoningEffortOptionKey 与会话参数中的规范推理档位键保持一致。
const modelReasoningEffortOptionKey = "reasoning_effort"

func modelReasoningProtocolKeys(protocolsJSON string) []string {
	var protocols []string
	if err := json.Unmarshal([]byte(strings.TrimSpace(protocolsJSON)), &protocols); err != nil {
		return nil
	}
	return normalizeReasoningProtocolKeys(protocols)
}

// normalizeReasoningProtocolKeys 把路由协议映射为推理能力使用的协议键，去重并保持原有顺序。
func normalizeReasoningProtocolKeys(protocols []string) []string {
	keys := make([]string, 0, len(protocols))
	seen := make(map[string]struct{}, len(protocols))
	for _, protocol := range protocols {
		if strings.TrimSpace(protocol) == "" {
			continue
		}
		key := llm.OptionPolicyProtocolKey(protocol)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	}
	return keys
}
