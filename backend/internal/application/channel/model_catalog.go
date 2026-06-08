package channel

import (
	"encoding/json"
	"strings"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/llm"
)

const (
	// TaskTypeChat 表示普通聊天或内部文本任务。
	TaskTypeChat = "chat"
	// TaskTypeImageGeneration 表示图片生成任务。
	TaskTypeImageGeneration = "image_generation"
	// TaskTypeImageEdit 表示图片编辑任务。
	TaskTypeImageEdit = "image_edit"

	modelKindChat      = "chat"
	modelKindAudio     = "audio"
	modelKindImageGen  = "image_gen"
	modelKindImageEdit = "image_edit"
	modelKindVideoGen  = "video_gen"

	compatibleOpenAI     = "openai"
	compatibleAnthropic  = "anthropic"
	compatibleGoogle     = "google"
	compatibleXAI        = "xai"
	compatibleOpenRouter = "openrouter"
	compatibleCustom     = "custom"

	protocolOpenAIImageGenerations = llm.AdapterOpenAIImageGenerations
	protocolOpenAIImageEdits       = llm.AdapterOpenAIImageEdits
	protocolOpenAIVideoGenerations = "openai_video_generations"
	protocolGoogleImageGeneration  = llm.AdapterGoogleImageGeneration
	protocolXAIImage               = llm.AdapterXAIImage
	protocolXAIImageEdits          = llm.AdapterXAIImageEdits
)

var protocolDefaultKindOrder = []string{
	modelKindChat,
	modelKindAudio,
	modelKindImageGen,
	modelKindImageEdit,
	modelKindVideoGen,
}

func normalizeCompatible(raw string) string {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case compatibleAnthropic:
		return compatibleAnthropic
	case compatibleGoogle:
		return compatibleGoogle
	case compatibleXAI:
		return compatibleXAI
	case compatibleOpenRouter:
		return compatibleOpenRouter
	case compatibleCustom:
		return compatibleCustom
	case "", compatibleOpenAI:
		return compatibleOpenAI
	default:
		return ""
	}
}

func protocolDefaultsForCompatible(compatible string) string {
	defaults := map[string]string{}
	for kind, protocol := range systemFallbackProtocols(normalizeCompatible(compatible)) {
		defaults[kind] = protocol
	}
	payload, _ := json.Marshal(defaults)
	return string(payload)
}

func normalizeProtocolDefaultsJSON(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return `{}`, nil
	}
	var payload map[string]*string
	if err := json.Unmarshal([]byte(raw), &payload); err != nil || payload == nil {
		return "", ErrInvalidJSONConfig
	}

	defaults := make(map[string]string)
	for _, kind := range protocolDefaultKindOrder {
		value, ok := payload[kind]
		if !ok || value == nil {
			continue
		}
		protocol := strings.TrimSpace(strings.ToLower(*value))
		if protocol == "" {
			continue
		}
		if !isKnownProtocol(protocol) {
			return "", ErrInvalidAdapter
		}
		if !isProtocolAllowedForKind(kind, protocol) {
			return "", ErrInvalidAdapter
		}
		defaults[kind] = protocol
	}

	normalized, _ := json.Marshal(defaults)
	return string(normalized), nil
}

func systemFallbackProtocols(compatible string) map[string]string {
	switch normalizeCompatible(compatible) {
	case compatibleOpenAI:
		return map[string]string{
			modelKindChat:      llm.AdapterOpenAIChatCompletions,
			modelKindAudio:     llm.AdapterOpenAIChatCompletions,
			modelKindImageGen:  protocolOpenAIImageGenerations,
			modelKindImageEdit: protocolOpenAIImageEdits,
			modelKindVideoGen:  protocolOpenAIVideoGenerations,
		}
	case compatibleAnthropic:
		return map[string]string{
			modelKindChat:  llm.AdapterAnthropicMessages,
			modelKindAudio: llm.AdapterAnthropicMessages,
		}
	case compatibleGoogle:
		return map[string]string{
			modelKindChat:      llm.AdapterGoogleGenerateContent,
			modelKindAudio:     llm.AdapterGoogleGenerateContent,
			modelKindImageGen:  protocolGoogleImageGeneration,
			modelKindImageEdit: protocolGoogleImageGeneration,
		}
	case compatibleXAI:
		return map[string]string{
			modelKindChat:      llm.AdapterXAIResponses,
			modelKindAudio:     llm.AdapterXAIResponses,
			modelKindImageGen:  protocolXAIImage,
			modelKindImageEdit: protocolXAIImageEdits,
		}
	case compatibleOpenRouter:
		return map[string]string{
			modelKindChat:      llm.AdapterOpenRouterResponses,
			modelKindAudio:     llm.AdapterOpenRouterResponses,
			modelKindImageGen:  protocolOpenAIImageGenerations,
			modelKindImageEdit: protocolOpenAIImageEdits,
			modelKindVideoGen:  protocolOpenAIVideoGenerations,
		}
	case compatibleCustom:
		return map[string]string{
			modelKindChat:      llm.AdapterOpenAIChatCompletions,
			modelKindAudio:     llm.AdapterOpenAIChatCompletions,
			modelKindImageGen:  protocolOpenAIImageGenerations,
			modelKindImageEdit: protocolOpenAIImageEdits,
		}
	default:
		return map[string]string{}
	}
}

func isKnownProtocol(raw string) bool {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case llm.AdapterOpenAIResponses,
		llm.AdapterOpenRouterResponses,
		llm.AdapterOpenAIChatCompletions,
		llm.AdapterAnthropicMessages,
		llm.AdapterGoogleGenerateContent,
		llm.AdapterXAIResponses,
		protocolOpenAIImageGenerations,
		protocolOpenAIImageEdits,
		protocolOpenAIVideoGenerations,
		protocolGoogleImageGeneration,
		protocolXAIImage,
		protocolXAIImageEdits:
		return true
	default:
		return false
	}
}

func resolveRouteProtocol(explicit string, upCompatible string, defaultsJSON string, kindsJSON string) (string, error) {
	kind := primaryKindFromKinds(kindsJSON)
	if protocol := strings.TrimSpace(strings.ToLower(explicit)); protocol != "" {
		if !isKnownProtocol(protocol) {
			return "", ErrInvalidAdapter
		}
		if !isProtocolAllowedForKinds(kindsJSON, protocol) {
			return "", ErrInvalidAdapter
		}
		return protocol, nil
	}

	if kind == "" {
		return "", ErrProtocolRequired
	}
	if protocol := protocolDefaultForKind(defaultsJSON, kind); protocol != "" {
		return protocol, nil
	}
	if protocol := systemFallbackProtocols(upCompatible)[kind]; protocol != "" {
		return protocol, nil
	}
	return "", ErrProtocolRequired
}

// resolveRouteProtocols 解析批量导入时的协议列表，图片生成/编辑模型会按协议能力生成一到两条绑定。
func resolveRouteProtocols(explicit []string, upCompatible string, defaultsJSON string, kindsJSON string) ([]string, error) {
	protocols := make([]string, 0, len(explicit))
	seen := make(map[string]struct{}, len(explicit))
	for _, raw := range explicit {
		value := strings.TrimSpace(strings.ToLower(raw))
		if value == "" {
			continue
		}
		protocol, err := resolveRouteProtocol(value, upCompatible, defaultsJSON, kindsJSON)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[protocol]; ok {
			continue
		}
		seen[protocol] = struct{}{}
		protocols = append(protocols, protocol)
	}
	if len(protocols) > 0 {
		if !isSupportedRouteProtocolCombination(protocols) {
			return nil, ErrInvalidRouteProtocolCombination
		}
		return protocols, nil
	}

	kinds := parseKinds(kindsJSON)
	if hasModelKind(kinds, modelKindImageGen) && hasModelKind(kinds, modelKindImageEdit) {
		generationProtocol := defaultRouteProtocolForKind(upCompatible, defaultsJSON, modelKindImageGen)
		editProtocol := defaultRouteProtocolForKind(upCompatible, defaultsJSON, modelKindImageEdit)
		if generationProtocol != "" && editProtocol != "" {
			protocols = uniqueRouteProtocols(generationProtocol, editProtocol)
			if isSupportedRouteProtocolCombination(protocols) {
				return protocols, nil
			}
		}
	}

	protocol, err := resolveRouteProtocol("", upCompatible, defaultsJSON, kindsJSON)
	if err != nil {
		return nil, err
	}
	return []string{protocol}, nil
}

// uniqueRouteProtocols 保留协议声明顺序，同时避免 Google 图片这类同协议双能力模型创建重复绑定。
func uniqueRouteProtocols(values ...string) []string {
	protocols := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, raw := range values {
		protocol := strings.TrimSpace(strings.ToLower(raw))
		if protocol == "" {
			continue
		}
		if _, ok := seen[protocol]; ok {
			continue
		}
		seen[protocol] = struct{}{}
		protocols = append(protocols, protocol)
	}
	return protocols
}

// defaultRouteProtocolForKind 优先使用上游配置的类型默认协议，其次回退到兼容类型内置协议。
func defaultRouteProtocolForKind(upCompatible string, defaultsJSON string, kind string) string {
	if protocol := protocolDefaultForKind(defaultsJSON, kind); protocol != "" {
		return protocol
	}
	return systemFallbackProtocols(upCompatible)[kind]
}

func isProtocolAllowedForKinds(kindsJSON string, protocol string) bool {
	kinds := parseKinds(kindsJSON)
	if len(kinds) == 0 {
		return true
	}
	for _, kind := range kinds {
		if isProtocolAllowedForKind(kind, protocol) {
			return true
		}
	}
	return false
}

// isSupportedRouteProtocolCombination 限制同一绑定 pair 只能单协议，或图片生成/编辑双协议。
func isSupportedRouteProtocolCombination(protocols []string) bool {
	seen := make(map[string]struct{}, len(protocols))
	for _, raw := range protocols {
		protocol := strings.TrimSpace(strings.ToLower(raw))
		if protocol == "" {
			continue
		}
		seen[protocol] = struct{}{}
	}
	if len(seen) <= 1 {
		return true
	}
	if len(seen) != 2 {
		return false
	}
	_, hasGeneration := seen[protocolOpenAIImageGenerations]
	_, hasEdit := seen[protocolOpenAIImageEdits]
	if hasGeneration && hasEdit {
		return true
	}
	_, hasGeneration = seen[protocolXAIImage]
	_, hasEdit = seen[protocolXAIImageEdits]
	return hasGeneration && hasEdit
}

func protocolDefaultForKind(defaultsJSON string, kind string) string {
	defaults := make(map[string]*string)
	if err := json.Unmarshal([]byte(strings.TrimSpace(defaultsJSON)), &defaults); err != nil {
		return ""
	}
	value, ok := defaults[kind]
	if !ok || value == nil {
		return ""
	}
	protocol := strings.TrimSpace(strings.ToLower(*value))
	if !isKnownProtocol(protocol) {
		return ""
	}
	if !isProtocolAllowedForKind(kind, protocol) {
		return ""
	}
	return protocol
}

func isProtocolAllowedForKind(kind string, protocol string) bool {
	switch kind {
	case modelKindChat, modelKindAudio:
		switch protocol {
		case llm.AdapterOpenAIResponses,
			llm.AdapterOpenRouterResponses,
			llm.AdapterOpenAIChatCompletions,
			llm.AdapterAnthropicMessages,
			llm.AdapterGoogleGenerateContent,
			llm.AdapterXAIResponses:
			return true
		default:
			return false
		}
	case modelKindImageGen:
		switch protocol {
		case protocolOpenAIImageGenerations,
			protocolGoogleImageGeneration,
			protocolXAIImage:
			return true
		default:
			return false
		}
	case modelKindImageEdit:
		switch protocol {
		case protocolOpenAIImageEdits,
			protocolGoogleImageGeneration,
			protocolXAIImageEdits:
			return true
		default:
			return false
		}
	case modelKindVideoGen:
		switch protocol {
		case protocolOpenAIVideoGenerations:
			return true
		default:
			return false
		}
	default:
		return false
	}
}

// NormalizeTaskType 归一化模型路由任务类型。
// 未传任务类型时按聊天处理，保留旧调用方的默认行为。
func NormalizeTaskType(raw string) string {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case TaskTypeImageGeneration:
		return TaskTypeImageGeneration
	case TaskTypeImageEdit:
		return TaskTypeImageEdit
	default:
		return TaskTypeChat
	}
}

// IsRouteAllowedForTask 判断指定模型 kind 与协议是否可服务当前任务。
// 图片任务必须命中图片协议；聊天任务不会误用图片生成/编辑协议。
func IsRouteAllowedForTask(taskType string, kindsJSON string, protocol string) bool {
	kinds := parseKinds(kindsJSON)
	protocol = strings.TrimSpace(strings.ToLower(protocol))
	if len(kinds) == 0 {
		switch NormalizeTaskType(taskType) {
		case TaskTypeImageGeneration:
			return isProtocolAllowedForKind(modelKindImageGen, protocol)
		case TaskTypeImageEdit:
			return isProtocolAllowedForKind(modelKindImageEdit, protocol)
		default:
			return isProtocolAllowedForKind(modelKindChat, protocol) || isProtocolAllowedForKind(modelKindAudio, protocol)
		}
	}
	switch NormalizeTaskType(taskType) {
	case TaskTypeImageGeneration:
		return hasModelKind(kinds, modelKindImageGen) && isProtocolAllowedForKind(modelKindImageGen, protocol)
	case TaskTypeImageEdit:
		return hasModelKind(kinds, modelKindImageEdit) && isProtocolAllowedForKind(modelKindImageEdit, protocol)
	default:
		for _, kind := range kinds {
			if (kind == modelKindChat || kind == modelKindAudio) && isProtocolAllowedForKind(kind, protocol) {
				return true
			}
		}
		return false
	}
}

// hasModelKind 判断模型 kind 列表是否包含目标能力。
func hasModelKind(kinds []string, target string) bool {
	for _, kind := range kinds {
		if kind == target {
			return true
		}
	}
	return false
}

func primaryKindFromKinds(kindsJSON string) string {
	kinds := parseKinds(kindsJSON)
	for _, candidate := range protocolDefaultKindOrder {
		for _, kind := range kinds {
			if kind == candidate {
				return kind
			}
		}
	}
	return ""
}

func inferKindsJSON(platformModelName string) string {
	code := strings.ToLower(strings.TrimSpace(platformModelName))
	switch {
	case strings.HasPrefix(code, "gpt-image-"), code == "chatgpt-image-latest", code == "dall-e-2",
		isGeminiImageGenerationModel(code), isXAIImageGenerationModel(code):
		return `["image_gen","image_edit"]`
	case code == "dall-e-3", strings.HasPrefix(code, "imagen-"):
		return `["image_gen"]`
	case code == "sora", code == "veo-2", strings.HasPrefix(code, "kling"):
		return `["video_gen"]`
	case strings.HasPrefix(code, "gpt-4o-audio"):
		return `["audio"]`
	case strings.HasPrefix(code, "claude-3"), strings.HasPrefix(code, "claude-2"),
		strings.HasPrefix(code, "gpt-4o"), strings.HasPrefix(code, "gpt-4-turbo"),
		strings.HasPrefix(code, "gemini-1.5"), strings.HasPrefix(code, "gemini-2.5"),
		code == "grok-3", code == "grok-2":
		return `["chat"]`
	default:
		return `["chat"]`
	}
}

func isGeminiImageGenerationModel(code string) bool {
	switch strings.TrimSpace(strings.ToLower(code)) {
	case "nano-banana", "nano-banana-2", "nano-banana-pro",
		"gemini-2.5-flash-image",
		"gemini-3.1-flash-image",
		"gemini-3-pro-image",
		"gemini-3.1-flash-image-preview",
		"gemini-3-pro-image-preview":
		return true
	default:
		return false
	}
}

func isXAIImageGenerationModel(code string) bool {
	return strings.HasPrefix(strings.TrimSpace(strings.ToLower(code)), "grok-imagine-image")
}
