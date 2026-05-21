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
	protocolGoogleImagen           = "google_imagen"
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
			modelKindChat:     llm.AdapterGoogleGenerateContent,
			modelKindAudio:    llm.AdapterGoogleGenerateContent,
			modelKindImageGen: protocolGoogleImagen,
		}
	case compatibleXAI:
		return map[string]string{
			modelKindChat:  llm.AdapterXAIResponses,
			modelKindAudio: llm.AdapterXAIResponses,
		}
	case compatibleOpenRouter:
		return map[string]string{
			modelKindChat:      llm.AdapterOpenAIResponses,
			modelKindAudio:     llm.AdapterOpenAIResponses,
			modelKindImageGen:  protocolOpenAIImageGenerations,
			modelKindImageEdit: protocolOpenAIImageEdits,
			modelKindVideoGen:  protocolOpenAIVideoGenerations,
		}
	default:
		return map[string]string{}
	}
}

func isKnownProtocol(raw string) bool {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case llm.AdapterOpenAIResponses,
		llm.AdapterOpenAIChatCompletions,
		llm.AdapterAnthropicMessages,
		llm.AdapterGoogleGenerateContent,
		llm.AdapterXAIResponses,
		protocolOpenAIImageGenerations,
		protocolOpenAIImageEdits,
		protocolOpenAIVideoGenerations,
		protocolGoogleImagen:
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
		if kind != "" && !isProtocolAllowedForKind(kind, protocol) {
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
			protocolGoogleImagen:
			return true
		default:
			return false
		}
	case modelKindImageEdit:
		switch protocol {
		case protocolOpenAIImageEdits:
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
			return protocol == protocolOpenAIImageGenerations
		case TaskTypeImageEdit:
			return protocol == protocolOpenAIImageEdits
		default:
			return isProtocolAllowedForKind(modelKindChat, protocol) || isProtocolAllowedForKind(modelKindAudio, protocol)
		}
	}
	switch NormalizeTaskType(taskType) {
	case TaskTypeImageGeneration:
		return hasModelKind(kinds, modelKindImageGen) && protocol == protocolOpenAIImageGenerations
	case TaskTypeImageEdit:
		return hasModelKind(kinds, modelKindImageEdit) && protocol == protocolOpenAIImageEdits
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
	case strings.HasPrefix(code, "gpt-image-"), code == "chatgpt-image-latest", code == "dall-e-2":
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
