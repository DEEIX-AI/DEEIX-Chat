package settings

import (
	"encoding/json"
	"strings"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/nativetool"
)

var validModelOptionProtocolKeys = map[string]struct{}{
	"default":                     {},
	"openai_chat_completions":     {},
	"openrouter_chat_completions": {},
	"openrouter_responses":        {},
	"openai_image_generations":    {},
	"openai_image_edits":          {},
	"openai_responses":            {},
	"anthropic_messages":          {},
	"xai_responses":               {},
	"xai_image":                   {},
	"xai_image_edits":             {},
	"xai_video":                   {},
	"xai_video_extensions":        {},
	"gemini_generate_content":     {},
	"google_image_generation":     {},
	"gemini_interactions":         {},
}

// validateModelOptionPathsJSON 校验模型参数透传路径配置，防止保存不可解析或越界的策略。
func validateModelOptionPathsJSON(value string, key string) error {
	if strings.TrimSpace(value) == "" {
		return settingRule("required", "")
	}
	if len([]rune(value)) > 20000 {
		return settingRule("max_length", "20000")
	}
	var raw map[string][]string
	if err := json.Unmarshal([]byte(value), &raw); err != nil {
		return settingRule("json_object", "string_arrays")
	}
	for protocol, paths := range raw {
		protocol = strings.TrimSpace(protocol)
		if _, ok := validModelOptionProtocolKeys[protocol]; !ok {
			return settingRule("model_option_protocol", "")
		}
		for _, path := range paths {
			if err := validateModelOptionPath(path); err != nil {
				return err
			}
		}
	}
	return nil
}

// validateNativeToolPricingJSON 校验官方原生工具计费覆盖配置，只允许后端已定义的计费项。
func validateNativeToolPricingJSON(value string, key string) error {
	if strings.TrimSpace(value) == "" {
		return settingRule("required", "")
	}
	if len([]rune(value)) > 20000 {
		return settingRule("max_length", "20000")
	}
	if _, err := nativetool.ParsePricingOverridesJSON(value); err != nil {
		return settingRule("native_tool_pricing", "")
	}
	return nil
}

func validateModelOptionPath(path string) error {
	value := strings.TrimSpace(path)
	if value == "" {
		return settingRule("model_option_path", "empty")
	}
	if strings.ContainsAny(value, " \t\r\n") {
		return settingRule("model_option_path", "whitespace")
	}
	if strings.Contains(value, "..") || strings.HasPrefix(value, ".") || strings.HasSuffix(value, ".") {
		return settingRule("model_option_path", "segments")
	}
	for _, segment := range strings.Split(value, ".") {
		if segment == "" {
			return settingRule("model_option_path", "segments")
		}
		for _, r := range segment {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
				continue
			}
			return settingRule("model_option_path", "characters")
		}
	}
	return nil
}
