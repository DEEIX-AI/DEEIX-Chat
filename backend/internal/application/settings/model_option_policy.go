package settings

import (
	"encoding/json"
	"fmt"
	"strings"
)

var validModelOptionProtocolKeys = map[string]struct{}{
	"default":                  {},
	"openai_chat_completions":  {},
	"openai_image_generations": {},
	"openai_image_edits":       {},
	"openai_responses":         {},
	"anthropic_messages":       {},
	"xai_responses":            {},
	"xai_image":                {},
	"gemini_generate_content":  {},
	"google_image_generation":  {},
}

var validNativeToolTypesByProtocol = map[string]map[string]struct{}{
	"openai_chat_completions": {
		"web_search":         {},
		"web_search_preview": {},
	},
	"openai_responses": {
		"web_search":         {},
		"web_search_preview": {},
		"shell":              {},
		"image_generation":   {},
		"code_interpreter":   {},
	},
	"anthropic_messages": {
		"web_search_20250305":             {},
		"web_search_20260209":             {},
		"web_fetch_20250910":              {},
		"web_fetch_20260209":              {},
		"code_execution_20250825":         {},
		"code_execution_20260120":         {},
		"advisor_20260301":                {},
		"tool_search_tool_regex_20251119": {},
		"tool_search_tool_bm25_20251119":  {},
	},
	"xai_responses": {
		"web_search":       {},
		"x_search":         {},
		"code_interpreter": {},
	},
}

// validateModelOptionPathsJSON 校验模型参数透传路径配置，防止保存不可解析或越界的策略。
func validateModelOptionPathsJSON(value string, key string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s cannot be empty", key)
	}
	if len([]rune(value)) > 20000 {
		return fmt.Errorf("%s length must be <= 20000", key)
	}
	var raw map[string][]string
	if err := json.Unmarshal([]byte(value), &raw); err != nil {
		return fmt.Errorf("%s must be a JSON object whose values are string arrays", key)
	}
	for protocol, paths := range raw {
		protocol = strings.TrimSpace(protocol)
		if _, ok := validModelOptionProtocolKeys[protocol]; !ok {
			return fmt.Errorf("%s contains unsupported protocol key: %s", key, protocol)
		}
		for _, path := range paths {
			if err := validateModelOptionPath(path); err != nil {
				return fmt.Errorf("%s contains invalid path %q: %w", key, path, err)
			}
		}
	}
	return nil
}

// validateNativeToolAllowedTypesJSON 校验官方原生工具控制配置，只允许后端已适配的协议和工具类型。
func validateNativeToolAllowedTypesJSON(value string, key string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s cannot be empty", key)
	}
	if len([]rune(value)) > 20000 {
		return fmt.Errorf("%s length must be <= 20000", key)
	}
	var raw map[string][]string
	if err := json.Unmarshal([]byte(value), &raw); err != nil {
		return fmt.Errorf("%s must be a JSON object whose values are string arrays", key)
	}
	for protocol, types := range raw {
		protocol = strings.TrimSpace(protocol)
		allowedTypes, ok := validNativeToolTypesByProtocol[protocol]
		if !ok {
			return fmt.Errorf("%s contains unsupported protocol key: %s", key, protocol)
		}
		for _, toolType := range types {
			value := strings.TrimSpace(toolType)
			if value == "" {
				return fmt.Errorf("%s contains empty tool type for %s", key, protocol)
			}
			if _, ok := allowedTypes[value]; !ok {
				return fmt.Errorf("%s contains unsupported tool type %q for %s", key, value, protocol)
			}
		}
	}
	return nil
}

func validateModelOptionPath(path string) error {
	value := strings.TrimSpace(path)
	if value == "" {
		return fmt.Errorf("path cannot be empty")
	}
	if strings.ContainsAny(value, " \t\r\n") {
		return fmt.Errorf("path cannot contain whitespace")
	}
	if strings.Contains(value, "..") || strings.HasPrefix(value, ".") || strings.HasSuffix(value, ".") {
		return fmt.Errorf("path must use non-empty dot-separated segments")
	}
	for _, segment := range strings.Split(value, ".") {
		if segment == "" {
			return fmt.Errorf("path must use non-empty dot-separated segments")
		}
		for _, r := range segment {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
				continue
			}
			return fmt.Errorf("path segment contains unsupported character %q", r)
		}
	}
	return nil
}
