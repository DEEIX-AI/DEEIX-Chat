package conversation

import (
	"encoding/json"
	"strings"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/llm"
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
	Mode                       string
	AllowedPathsJSON           string
	DeniedPathsJSON            string
	NativeToolAllowedTypesJSON string
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
	nativeTools := nativeProviderToolsFromOption(protocolKey, options["tools"], cfg.NativeToolAllowedTypesJSON)
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
func nativeProviderToolsFromOption(protocolKey string, raw interface{}, allowedTypesJSON string) []map[string]interface{} {
	rawTools := providerToolOptionPayloads(raw)
	if len(rawTools) == 0 {
		return nil
	}
	allowedTypes := nativeToolAllowedTypesForProtocol(protocolKey, allowedTypesJSON)
	if len(allowedTypes) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(rawTools))
	tools := make([]map[string]interface{}, 0, len(rawTools))
	for _, rawTool := range rawTools {
		tool, ok := sanitizeNativeProviderTool(protocolKey, rawTool)
		if !ok {
			continue
		}
		toolType := stringModelOptionValue(tool["type"])
		if _, allowed := allowedTypes[toolType]; !allowed {
			continue
		}
		if _, exists := seen[toolType]; exists {
			continue
		}
		seen[toolType] = struct{}{}
		tools = append(tools, tool)
	}
	return tools
}

// nativeToolAllowedTypesForProtocol 解析后台配置的官方工具允许列表。
// 配置缺失或格式错误时回退默认值，运行时保存入口会负责严格校验。
func nativeToolAllowedTypesForProtocol(protocolKey string, raw string) map[string]struct{} {
	defaults := defaultNativeToolAllowedTypes(protocolKey)
	value := strings.TrimSpace(raw)
	if value == "" {
		return defaults
	}
	var config map[string][]string
	if err := json.Unmarshal([]byte(value), &config); err != nil {
		return defaults
	}
	types, ok := config[protocolKey]
	if !ok {
		return defaults
	}
	allowed := make(map[string]struct{}, len(types))
	for _, toolType := range types {
		toolType = strings.TrimSpace(toolType)
		if _, ok := defaults[toolType]; ok {
			allowed[toolType] = struct{}{}
		}
	}
	return allowed
}

// defaultNativeToolAllowedTypes 返回当前后端已显式适配的官方工具类型。
func defaultNativeToolAllowedTypes(protocolKey string) map[string]struct{} {
	types := []string{}
	switch protocolKey {
	case "openai_responses":
		types = []string{"web_search", "web_search_preview", "shell", "image_generation", "code_interpreter"}
	case "openai_chat_completions":
		types = []string{"web_search", "web_search_preview"}
	case "xai_responses":
		types = []string{"web_search", "x_search", "code_interpreter"}
	case "anthropic_messages":
		types = []string{
			"web_search_20250305",
			"web_search_20260209",
			"web_fetch_20250910",
			"web_fetch_20260209",
			"code_execution_20250825",
			"code_execution_20260120",
			"advisor_20260301",
			"tool_search_tool_regex_20251119",
			"tool_search_tool_bm25_20251119",
		}
	}
	allowed := make(map[string]struct{}, len(types))
	for _, toolType := range types {
		allowed[toolType] = struct{}{}
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

// sanitizeNativeProviderTool 按协议选择官方工具清洗规则，未知协议和未知类型一律丢弃。
func sanitizeNativeProviderTool(protocolKey string, tool map[string]interface{}) (map[string]interface{}, bool) {
	toolType := strings.TrimSpace(stringModelOptionValue(tool["type"]))
	if toolType == "" {
		return nil, false
	}
	switch protocolKey {
	case "openai_chat_completions", "openai_responses":
		return sanitizeOpenAINativeProviderTool(toolType)
	case "xai_responses":
		return sanitizeXAINativeProviderTool(toolType)
	case "anthropic_messages":
		return sanitizeAnthropicNativeProviderTool(toolType, tool)
	default:
		return nil, false
	}
}

// sanitizeOpenAINativeProviderTool 只保留 OpenAI 官方工具允许透传的固定字段。
func sanitizeOpenAINativeProviderTool(toolType string) (map[string]interface{}, bool) {
	switch toolType {
	case "web_search", "web_search_preview":
		return map[string]interface{}{"type": toolType}, true
	case "shell":
		return map[string]interface{}{"type": "shell", "environment": map[string]interface{}{"type": "container_auto"}}, true
	case "image_generation":
		return map[string]interface{}{"type": "image_generation"}, true
	case "code_interpreter":
		return map[string]interface{}{"type": "code_interpreter", "container": map[string]interface{}{"type": "auto"}}, true
	default:
		return nil, false
	}
}

// sanitizeXAINativeProviderTool 只保留 xAI 官方工具允许透传的固定字段。
func sanitizeXAINativeProviderTool(toolType string) (map[string]interface{}, bool) {
	switch toolType {
	case "web_search", "x_search", "code_interpreter":
		return map[string]interface{}{"type": toolType}, true
	default:
		return nil, false
	}
}

// sanitizeAnthropicNativeProviderTool 只保留 Anthropic 官方工具允许透传的固定字段。
func sanitizeAnthropicNativeProviderTool(toolType string, raw map[string]interface{}) (map[string]interface{}, bool) {
	switch toolType {
	case "web_search_20250305":
		return map[string]interface{}{"type": toolType, "name": "web_search"}, true
	case "web_search_20260209":
		return map[string]interface{}{"type": toolType, "name": "web_search", "allowed_callers": []string{"direct"}}, true
	case "web_fetch_20250910":
		return map[string]interface{}{"type": toolType, "name": "web_fetch"}, true
	case "web_fetch_20260209":
		return map[string]interface{}{"type": toolType, "name": "web_fetch", "allowed_callers": []string{"direct"}}, true
	case "code_execution_20250825", "code_execution_20260120":
		return map[string]interface{}{"type": toolType, "name": "code_execution"}, true
	case "advisor_20260301":
		tool := map[string]interface{}{"type": toolType, "name": "advisor"}
		if model := strings.TrimSpace(stringModelOptionValue(raw["model"])); model != "" {
			tool["model"] = model
		}
		return tool, true
	case "tool_search_tool_regex_20251119":
		return map[string]interface{}{"type": toolType, "name": "tool_search_tool_regex"}, true
	case "tool_search_tool_bm25_20251119":
		return map[string]interface{}{"type": toolType, "name": "tool_search_tool_bm25"}, true
	default:
		return nil, false
	}
}

// stringModelOptionValue 从自由 JSON 值中安全读取字符串。
func stringModelOptionValue(value interface{}) string {
	if typed, ok := value.(string); ok {
		return typed
	}
	return ""
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
