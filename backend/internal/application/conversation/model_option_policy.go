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
	allowedTools := nativeToolCapabilitiesFromConfig(capabilitiesJSON, protocolKey)
	if len(allowedTools) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(rawTools))
	tools := make([]map[string]interface{}, 0, len(rawTools))
	for _, rawTool := range rawTools {
		tool, identity, ok := nativeProviderToolPayload(protocolKey, rawTool, allowedTools)
		if !ok {
			continue
		}
		if _, exists := seen[identity]; exists {
			continue
		}
		seen[identity] = struct{}{}
		tools = append(tools, tool)
	}
	return tools
}

func nativeProviderToolPayload(protocolKey string, rawTool map[string]interface{}, allowedTools []nativeToolCapability) (map[string]interface{}, string, bool) {
	definition, tool, ok := nativetool.PayloadFromOption(protocolKey, rawTool)
	if ok {
		if capability, allowed := nativeToolCapabilityByDefinition(allowedTools, definition); allowed {
			return nativetool.CanonicalPayload(definition, mergeNativeToolPayload(tool, capability.Payload)), capability.Identity(), true
		}
	}
	for _, capability := range allowedTools {
		if !nativeToolCapabilityMatchesRawTool(capability, rawTool) {
			continue
		}
		return mergeNativeToolPayload(rawTool, capability.Payload), capability.Identity(), true
	}
	return nil, "", false
}

type nativeToolCapability struct {
	Key      string
	Protocol string
	Type     string
	Payload  map[string]interface{}
}

func (tool nativeToolCapability) Identity() string {
	parts := []string{
		strings.TrimSpace(tool.Key),
		strings.TrimSpace(tool.Protocol),
		strings.TrimSpace(tool.Type),
	}
	if parts[0] != "" {
		return strings.Join(parts, ":")
	}
	return strings.Join(parts[1:], ":")
}

func nativeToolCapabilitiesFromConfig(raw string, protocolKey string) []nativeToolCapability {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil
	}
	var config struct {
		NativeTools []struct {
			Key            string                 `json:"key"`
			ToolKey        string                 `json:"toolKey"`
			Protocol       string                 `json:"protocol"`
			Protocols      []string               `json:"protocols"`
			Type           string                 `json:"type"`
			Enabled        *bool                  `json:"enabled"`
			Payload        map[string]interface{} `json:"payload"`
			DefaultEnabled bool                   `json:"defaultEnabled"`
		} `json:"nativeTools"`
		NativeToolKeys []string               `json:"nativeToolKeys"`
		DefaultOptions map[string]interface{} `json:"defaultOptions"`
	}
	if err := json.Unmarshal([]byte(value), &config); err != nil {
		return nil
	}
	capabilities := make([]nativeToolCapability, 0, len(config.NativeTools)+len(config.NativeToolKeys))
	seen := make(map[string]struct{})
	addCapability := func(tool nativeToolCapability) {
		tool.Key = strings.TrimSpace(tool.Key)
		tool.Protocol = strings.TrimSpace(tool.Protocol)
		tool.Type = strings.TrimSpace(tool.Type)
		if tool.Type == "" {
			tool.Type = strings.TrimSpace(modelOptionStringValue(tool.Payload["type"]))
		}
		if tool.Protocol == "" {
			tool.Protocol = protocolKey
		}
		if tool.Type == "" && len(tool.Payload) == 0 {
			return
		}
		identity := tool.Identity()
		if _, ok := seen[identity]; ok {
			return
		}
		seen[identity] = struct{}{}
		capabilities = append(capabilities, tool)
	}

	for _, item := range config.NativeTools {
		if item.Enabled != nil && !*item.Enabled {
			continue
		}
		key := strings.TrimSpace(item.Key)
		if key == "" {
			key = strings.TrimSpace(item.ToolKey)
		}
		protocols := nativeToolCapabilityProtocols(item.Protocols, item.Protocol)
		definitions := nativeToolDefinitionsByKey(key)
		if len(protocols) == 0 {
			protocols = nativeToolDefinitionProtocols(definitions)
		}
		if len(protocols) == 0 {
			protocols = []string{protocolKey}
		}
		if len(definitions) > 0 {
			for _, protocol := range protocols {
				definition, ok := nativeToolDefinitionForProtocol(definitions, protocol)
				if !ok {
					addCapability(nativeToolCapability{
						Key:      key,
						Protocol: protocol,
						Type:     item.Type,
						Payload:  cloneModelOptionMap(item.Payload),
					})
					continue
				}
				addCapability(nativeToolCapability{
					Key:      key,
					Protocol: firstNonEmpty(protocol, definition.Protocol, protocolKey),
					Type:     firstNonEmpty(item.Type, definition.Type),
					Payload:  nativetool.CanonicalPayload(definition, item.Payload),
				})
			}
			continue
		}
		for _, protocol := range protocols {
			addCapability(nativeToolCapability{
				Key:      key,
				Protocol: firstNonEmpty(protocol, protocolKey),
				Type:     item.Type,
				Payload:  cloneModelOptionMap(item.Payload),
			})
		}
	}

	for _, key := range config.NativeToolKeys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		for _, definition := range nativetool.Definitions() {
			if definition.Key != key {
				continue
			}
			addCapability(nativeToolCapability{
				Key:      definition.Key,
				Protocol: definition.Protocol,
				Type:     definition.Type,
				Payload:  definition.Payload,
			})
		}
	}

	for _, tool := range providerToolOptionPayloads(config.DefaultOptions["tools"]) {
		definition, _, ok := nativetool.PayloadFromOption(protocolKey, tool)
		if ok {
			addCapability(nativeToolCapability{
				Key:      definition.Key,
				Protocol: definition.Protocol,
				Type:     definition.Type,
				Payload:  nativetool.CanonicalPayload(definition, tool),
			})
		}
	}
	return capabilities
}

func nativeToolCapabilityProtocols(values []string, single string) []string {
	protocols := make([]string, 0, len(values)+1)
	seen := make(map[string]struct{}, len(values)+1)
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		protocols = append(protocols, value)
	}
	for _, value := range values {
		add(value)
	}
	add(single)
	return protocols
}

func nativeToolDefinitionsByKey(key string) []nativetool.Definition {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil
	}
	definitions := make([]nativetool.Definition, 0, 2)
	for _, definition := range nativetool.Definitions() {
		if definition.Key == key {
			definitions = append(definitions, definition)
		}
	}
	return definitions
}

func nativeToolDefinitionProtocols(definitions []nativetool.Definition) []string {
	protocols := make([]string, 0, len(definitions))
	seen := make(map[string]struct{}, len(definitions))
	for _, definition := range definitions {
		protocol := strings.TrimSpace(definition.Protocol)
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

func nativeToolDefinitionForProtocol(definitions []nativetool.Definition, protocol string) (nativetool.Definition, bool) {
	protocol = strings.TrimSpace(protocol)
	for _, definition := range definitions {
		if definition.Protocol == protocol {
			return definition, true
		}
	}
	return nativetool.Definition{}, false
}

func nativeToolCapabilityByDefinition(items []nativeToolCapability, definition nativetool.Definition) (nativeToolCapability, bool) {
	for _, item := range items {
		if item.Key != "" && item.Key == definition.Key {
			if item.Protocol == "" || item.Protocol == definition.Protocol {
				return item, true
			}
		}
		if item.Type == definition.Type && (item.Protocol == "" || item.Protocol == definition.Protocol) {
			return item, true
		}
	}
	return nativeToolCapability{}, false
}

func nativeToolCapabilityMatchesRawTool(capability nativeToolCapability, rawTool map[string]interface{}) bool {
	rawType := strings.TrimSpace(modelOptionStringValue(rawTool["type"]))
	if rawType != "" && capability.Type != "" {
		return rawType == capability.Type
	}
	payloadType := strings.TrimSpace(modelOptionStringValue(capability.Payload["type"]))
	if rawType != "" && payloadType != "" {
		return rawType == payloadType
	}
	if capability.Type != "" {
		return false
	}
	for key := range capability.Payload {
		if key == "type" {
			continue
		}
		if _, ok := rawTool[key]; ok {
			return true
		}
	}
	return false
}

func mergeNativeToolPayload(raw map[string]interface{}, base map[string]interface{}) map[string]interface{} {
	payload := cloneModelOptionMap(raw)
	if payload == nil {
		payload = make(map[string]interface{})
	}
	for _, path := range hardDeniedModelOptionPaths {
		deleteModelOptionPath(payload, path)
	}
	mergeModelOptionMap(payload, base)
	return payload
}

func mergeModelOptionMap(dst map[string]interface{}, src map[string]interface{}) {
	for key, value := range src {
		srcMap, srcIsMap := value.(map[string]interface{})
		dstMap, dstIsMap := dst[key].(map[string]interface{})
		if srcIsMap && dstIsMap {
			mergeModelOptionMap(dstMap, srcMap)
			continue
		}
		dst[key] = cloneModelOptionValue(value)
	}
}

func modelOptionStringValue(value interface{}) string {
	if typed, ok := value.(string); ok {
		return typed
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
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
	case "openai":
		return "openai_responses"
	case "anthropic", "claude":
		return "anthropic_messages"
	case "xai", "grok":
		return "xai_responses"
	case "google", "gemini":
		return "gemini_generate_content"
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
