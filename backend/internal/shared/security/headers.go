package security

import (
	"encoding/json"
	"strings"
)

const redactedHeaderValue = "********"

// RedactHeadersJSON 对自定义请求头 JSON 中的敏感头做脱敏，避免 API 响应扩大密钥泄漏面。
func RedactHeadersJSON(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return value
	}
	payload := map[string]any{}
	if err := json.Unmarshal([]byte(value), &payload); err != nil || payload == nil {
		return "{}"
	}
	result := make(map[string]any, len(payload))
	for key, item := range payload {
		if IsSensitiveHeaderName(key) {
			result[key] = redactedHeaderValue
			continue
		}
		result[key] = item
	}
	normalized, err := json.Marshal(result)
	if err != nil {
		return "{}"
	}
	return string(normalized)
}

// IsSensitiveHeaderName 判断 header 名称是否可能承载密钥、Token 或 Cookie。
func IsSensitiveHeaderName(name string) bool {
	normalized := strings.ToLower(strings.TrimSpace(name))
	normalized = strings.ReplaceAll(normalized, "_", "-")
	if normalized == "" {
		return false
	}
	if normalized == "cookie" || normalized == "set-cookie" {
		return true
	}
	for _, marker := range []string{"authorization", "api-key", "apikey", "token", "secret"} {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return false
}
