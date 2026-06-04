package conversation

import (
	"encoding/json"
	"strings"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/llm"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/nativetool"
)

const (
	modelOptionPolicyAllowlist = "allowlist"
	modelOptionPolicyDenylist  = "denylist"
	modelOptionPolicyDisabled  = "disabled"
)

var hardDeniedModelOptionPaths = [][]string{
	{"model"},
	{"messages"},
	{"input"},
	{"instructions"},
	{"prompt"},
	{"system"},
	{"systemInstruction"},
	{"headers"},
	{"api_key"},
	{"apiKey"},
	{"base_url"},
	{"baseURL"},
	{"stream"},
	{"previous_response_id"},
}

type modelOptionPolicyConfig struct {
	Mode                  string
	AllowedPathsJSON      string
	DeniedPathsJSON       string
	ModelCapabilitiesJSON string
}

func filterModelOptions(options map[string]interface{}, protocol string, cfg modelOptionPolicyConfig) map[string]interface{} {
	if len(options) == 0 {
		return nil
	}

	mode := strings.TrimSpace(cfg.Mode)
	if mode == "" {
		mode = modelOptionPolicyAllowlist
	}
	if mode == modelOptionPolicyDisabled {
		return nil
	}

	protocolKey := modelOptionPolicyProtocolKey(protocol)
	nativeTools := nativeProviderToolsFromOption(protocolKey, options["tools"], cfg.ModelCapabilitiesJSON)
	policyOptions := cloneModelOptionMap(options)
	delete(policyOptions, "tools")
	denied := append([][]string{}, hardDeniedModelOptionPaths...)

	var filtered map[string]interface{}
	switch mode {
	case modelOptionPolicyDenylist:
		denied = append(denied, modelOptionPathsForProtocol(cfg.DeniedPathsJSON, protocolKey)...)
		filtered = policyOptions
	default:
		filtered = make(map[string]interface{})
		for _, path := range modelOptionPathsForProtocol(cfg.AllowedPathsJSON, protocolKey) {
			copyModelOptionPath(filtered, policyOptions, path)
		}
	}

	for _, path := range denied {
		deleteModelOptionPath(filtered, path)
	}
	sanitizeModelOptionValues(filtered, protocolKey)
	if len(nativeTools) > 0 {
		if filtered == nil {
			filtered = make(map[string]interface{})
		}
		filtered["tools"] = nativeTools
	}
	if len(filtered) == 0 {
		return nil
	}
	return filtered
}

// nativeProviderToolsFromOption 将用户 options.tools 收敛为当前协议允许的官方原生工具。
// 普通参数白名单不处理 tools，避免用户通过自由 JSON 绕过官方工具控制。
func nativeProviderToolsFromOption(protocolKey string, raw interface{}, capabilitiesJSON string) []map[string]interface{} {
	rawTools := providerToolOptionPayloads(raw)
	if len(rawTools) == 0 {
		return nil
	}
	allowedKeys := nativeToolKeysFromCapabilities(capabilitiesJSON, protocolKey)
	if len(allowedKeys) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(rawTools))
	tools := make([]map[string]interface{}, 0, len(rawTools))
	for _, rawTool := range rawTools {
		tool, definition, ok := nativeProviderToolPayload(protocolKey, rawTool, allowedKeys)
		if !ok {
			continue
		}
		if _, exists := seen[definition.Key]; exists {
			continue
		}
		seen[definition.Key] = struct{}{}
		tools = append(tools, tool)
	}
	return tools
}

func nativeProviderToolPayload(protocolKey string, rawTool map[string]interface{}, allowedKeys map[string]struct{}) (map[string]interface{}, nativetool.Definition, bool) {
	definition, tool, ok := nativetool.PayloadFromOption(protocolKey, rawTool)
	if ok {
		if _, allowed := allowedKeys[definition.Key]; allowed {
			return tool, definition, true
		}
	}
	for _, candidate := range nativetool.Definitions() {
		if _, allowed := allowedKeys[candidate.Key]; !allowed {
			continue
		}
		definition, tool, ok := nativetool.PayloadFromKey(candidate.Key, rawTool)
		if !ok {
			continue
		}
		return tool, definition, true
	}
	return nil, nativetool.Definition{}, false
}

// nativeToolKeysFromCapabilities 解析模型能力 JSON 中管理员显式声明或默认配置启用的官方原生工具。
func nativeToolKeysFromCapabilities(raw string, protocolKey string) map[string]struct{} {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil
	}
	var config struct {
		NativeToolKeys []string               `json:"nativeToolKeys"`
		DefaultOptions map[string]interface{} `json:"defaultOptions"`
	}
	if err := json.Unmarshal([]byte(value), &config); err != nil {
		return nil
	}
	known := make(map[string]struct{})
	for _, definition := range nativetool.Definitions() {
		known[definition.Key] = struct{}{}
	}
	allowed := make(map[string]struct{}, len(config.NativeToolKeys))
	for _, key := range config.NativeToolKeys {
		key = strings.TrimSpace(key)
		if _, ok := known[key]; ok {
			allowed[key] = struct{}{}
		}
	}
	for _, tool := range providerToolOptionPayloads(config.DefaultOptions["tools"]) {
		definition, _, ok := nativetool.PayloadFromOption(protocolKey, tool)
		if ok {
			allowed[definition.Key] = struct{}{}
		}
	}
	return allowed
}

// providerToolOptionPayloads 从自由 JSON 中提取 tools 数组对象。
func providerToolOptionPayloads(raw interface{}) []map[string]interface{} {
	switch typed := raw.(type) {
	case []map[string]interface{}:
		return append([]map[string]interface{}(nil), typed...)
	case []interface{}:
		items := make([]map[string]interface{}, 0, len(typed))
		for _, item := range typed {
			if payload, ok := item.(map[string]interface{}); ok {
				items = append(items, payload)
			}
		}
		return items
	default:
		return nil
	}
}

func sanitizeModelOptionValues(options map[string]interface{}, protocolKey string) {
	if len(options) == 0 {
		return
	}
	switch protocolKey {
	case "openai_chat_completions", "openai_responses":
		serviceTier, ok := options["service_tier"]
		if !ok {
			return
		}
		value, ok := serviceTier.(string)
		if !ok {
			delete(options, "service_tier")
			return
		}
		switch strings.TrimSpace(strings.ToLower(value)) {
		case "default", "flex", "priority":
			options["service_tier"] = strings.TrimSpace(strings.ToLower(value))
		default:
			delete(options, "service_tier")
		}
	case "openai_image_generations", "openai_image_edits":
		value, ok := modelParamIntFromOption(options["partial_images"])
		if !ok {
			delete(options, "partial_images")
			return
		}
		if value < 0 || value > 3 {
			delete(options, "partial_images")
		}
	}
}

func modelParamIntFromOption(value interface{}) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int64:
		return int(typed), true
	case float64:
		return int(typed), true
	default:
		return 0, false
	}
}

func modelOptionPolicyProtocolKey(protocol string) string {
	switch llm.NormalizeAdapter(protocol) {
	case llm.AdapterGoogleGenerateContent:
		return "gemini_generate_content"
	case llm.AdapterGoogleImageGeneration:
		return "google_image_generation"
	case llm.AdapterOpenAIChatCompletions:
		return "openai_chat_completions"
	case llm.AdapterOpenAIImageGenerations:
		return "openai_image_generations"
	case llm.AdapterOpenAIImageEdits:
		return "openai_image_edits"
	case llm.AdapterAnthropicMessages:
		return "anthropic_messages"
	case llm.AdapterXAIImage:
		return "xai_image"
	case llm.AdapterXAIImageEdits:
		return "xai_image_edits"
	case llm.AdapterXAIResponses:
		return "xai_responses"
	default:
		return "openai_responses"
	}
}

func modelOptionPathsForProtocol(raw string, protocol string) [][]string {
	var config map[string][]string
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &config); err != nil {
		return nil
	}
	paths := make([][]string, 0, len(config["default"])+len(config[protocol]))
	for _, value := range append(config["default"], config[protocol]...) {
		if path := splitModelOptionPath(value); len(path) > 0 {
			paths = append(paths, path)
		}
	}
	return paths
}

func splitModelOptionPath(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" || strings.ContainsAny(value, " \t\r\n") {
		return nil
	}
	parts := strings.Split(value, ".")
	for _, part := range parts {
		if part == "" {
			return nil
		}
	}
	return parts
}

func copyModelOptionPath(dst map[string]interface{}, src map[string]interface{}, path []string) {
	value, ok := readModelOptionPath(src, path)
	if !ok {
		return
	}
	writeModelOptionPath(dst, path, cloneModelOptionValue(value))
}

func readModelOptionPath(src map[string]interface{}, path []string) (interface{}, bool) {
	if len(path) == 0 {
		return nil, false
	}
	current := src
	for index, segment := range path {
		value, ok := current[segment]
		if !ok {
			return nil, false
		}
		if index == len(path)-1 {
			return value, true
		}
		next, ok := value.(map[string]interface{})
		if !ok {
			return nil, false
		}
		current = next
	}
	return nil, false
}

func writeModelOptionPath(dst map[string]interface{}, path []string, value interface{}) {
	current := dst
	for index, segment := range path {
		if index == len(path)-1 {
			current[segment] = value
			return
		}
		next, ok := current[segment].(map[string]interface{})
		if !ok {
			next = make(map[string]interface{})
			current[segment] = next
		}
		current = next
	}
}

func deleteModelOptionPath(dst map[string]interface{}, path []string) {
	if len(path) == 0 || len(dst) == 0 {
		return
	}
	current := dst
	for index, segment := range path {
		if index == len(path)-1 {
			delete(current, segment)
			return
		}
		next, ok := current[segment].(map[string]interface{})
		if !ok {
			return
		}
		current = next
	}
}

func cloneModelOptionMap(src map[string]interface{}) map[string]interface{} {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]interface{}, len(src))
	for key, value := range src {
		dst[key] = cloneModelOptionValue(value)
	}
	return dst
}

func cloneModelOptionValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		return cloneModelOptionMap(typed)
	case []interface{}:
		items := make([]interface{}, len(typed))
		for index, item := range typed {
			items[index] = cloneModelOptionValue(item)
		}
		return items
	default:
		return typed
	}
}
