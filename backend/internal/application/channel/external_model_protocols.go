package channel

import (
	"encoding/json"
	"strings"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
)

// 外部模型（用户自带 Key）的协议规则与平台模型绑定共用一套：同一模型只能是单个协议，
// 或同一媒体模型配套的一组协议（图片生成 + 编辑、xAI 视频生成 + 扩展）；模型类型由协议决定。

// compatibleForProtocol 把服务的接口协议映射到平台的兼容类型，用于推断媒体模型的配套协议。
func compatibleForProtocol(protocol string) string {
	switch llm.NormalizeAdapter(protocol) {
	case llm.AdapterAnthropicMessages:
		return compatibleAnthropic
	case llm.AdapterGoogleGenerateContent, llm.AdapterGoogleImageGeneration, llm.AdapterGeminiInteractions:
		return compatibleGoogle
	case llm.AdapterXAIResponses, llm.AdapterXAIImage, llm.AdapterXAIImageEdits, llm.AdapterXAIVideo, llm.AdapterXAIVideoExtensions:
		return compatibleXAI
	case llm.AdapterOpenRouterChat, llm.AdapterOpenRouterResponses, llm.AdapterOpenRouterImages:
		return compatibleOpenRouter
	default:
		return compatibleOpenAI
	}
}

// SuggestExternalModelProtocols 按模型名推断外部模型的协议，规则同平台导入远端模型：
// 对话模型沿用服务协议；图片、视频模型取服务厂商对应的媒体协议（及配套协议）。推断不出时回退到服务协议。
func SuggestExternalModelProtocols(providerProtocol string, model string) []string {
	providerProtocol = llm.NormalizeAdapter(providerProtocol)
	name := externalModelBaseName(model)
	kindsJSON := inferKindsJSON(name)
	if routeProtocolSupportsAllKinds(providerProtocol, parseKinds(kindsJSON)) {
		return []string{providerProtocol}
	}
	compatible := compatibleForProtocol(providerProtocol)
	// 服务按 OpenAI 兼容方式接入（中转站的常见情况）时，xAI 的图片、视频模型仍走 xAI 自己的媒体协议：
	// 模型名已经指明了接口，OpenAI 也没有对应的视频协议。
	if compatible == compatibleOpenAI && (isXAIImageGenerationModel(name) || isXAIVideoGenerationModel(name)) {
		compatible = compatibleXAI
	}
	protocols, err := resolveRouteProtocols(nil, compatible, "", kindsJSON)
	if err != nil || len(protocols) == 0 {
		return []string{providerProtocol}
	}
	for _, protocol := range protocols {
		if !llm.IsImplementedAdapter(protocol) {
			return []string{providerProtocol}
		}
	}
	return protocols
}

// NormalizeExternalModelProtocols 校验外部模型的协议组合：每个协议都必须已实现，组合必须是平台允许的配套协议。
// 返回去重、小写后的列表；不合法时返回 false。
func NormalizeExternalModelProtocols(raw []string) ([]string, bool) {
	protocols := make([]string, 0, len(raw))
	seen := make(map[string]struct{}, len(raw))
	for _, value := range raw {
		protocol := strings.TrimSpace(strings.ToLower(value))
		if protocol == "" {
			continue
		}
		if !isKnownProtocol(protocol) || !llm.IsImplementedAdapter(protocol) {
			return nil, false
		}
		if _, ok := seen[protocol]; ok {
			continue
		}
		seen[protocol] = struct{}{}
		protocols = append(protocols, protocol)
	}
	if len(protocols) == 0 || !isSupportedRouteProtocolCombination(protocols) {
		return nil, false
	}
	return protocols, true
}

// externalModelKinds 由协议决定外部模型的类型：先取按模型名推断且协议支持的类型，
// 没有交集时（例如用户把未知名称的模型指定为图片协议）取协议支持的全部类型。
func externalModelKinds(model string, protocols []string) []string {
	inferred := parseKinds(inferKindsJSON(externalModelBaseName(model)))
	supported := func(kind string) bool {
		for _, protocol := range protocols {
			if isProtocolAllowedForKind(kind, protocol) {
				return true
			}
		}
		return false
	}
	kinds := make([]string, 0, len(protocolDefaultKindOrder))
	for _, kind := range inferred {
		if supported(kind) {
			kinds = append(kinds, kind)
		}
	}
	if len(kinds) > 0 {
		return kinds
	}
	for _, kind := range protocolDefaultKindOrder {
		// 音频沿用对话协议，未指定时不单独列出。
		if kind != modelKindAudio && supported(kind) {
			kinds = append(kinds, kind)
		}
	}
	return kinds
}

// externalRouteProtocol 选出外部模型执行某个任务所用的协议；没有能执行该任务的协议时返回空串。
func externalRouteProtocol(model string, protocols []string, taskType string) string {
	kindsJSON := marshalKinds(externalModelKinds(model, protocols))
	for _, protocol := range protocols {
		if IsRouteAllowedForTask(taskType, kindsJSON, protocol) {
			return protocol
		}
	}
	return ""
}

// externalModelBaseName 去掉中转站常见的厂商前缀（如 openai/gpt-image-1），按模型本名识别类型。
func externalModelBaseName(model string) string {
	model = strings.TrimSpace(model)
	if index := strings.LastIndex(model, "/"); index >= 0 {
		return model[index+1:]
	}
	return model
}

func marshalKinds(kinds []string) string {
	raw, err := json.Marshal(kinds)
	if err != nil {
		return "[]"
	}
	return string(raw)
}
