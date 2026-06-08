package settings

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/nativetool"
)

var validModelOptionProtocolKeys = map[string]struct{}{
	"default":                  {},
	"openai_chat_completions":  {},
	"openai_image_generations": {},
	"openai_image_edits":       {},
	"openai_responses":         {},
	"openrouter_responses":     {},
	"anthropic_messages":       {},
	"xai_responses":            {},
	"xai_image":                {},
	"xai_image_edits":          {},
	"gemini_generate_content":  {},
	"google_image_generation":  {},
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

// validateNativeToolPricingJSON 校验官方原生工具计费覆盖配置，只允许后端已定义的计费项。
func validateNativeToolPricingJSON(value string, key string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s cannot be empty", key)
	}
	if len([]rune(value)) > 20000 {
		return fmt.Errorf("%s length must be <= 20000", key)
	}
	if _, err := nativetool.ParsePricingOverridesJSON(value); err != nil {
		return fmt.Errorf("%s invalid: %w", key, err)
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
