package toolresult

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// SanitizeOpaque preserves readable tool output while removing data URIs and
// base64-like payloads that should not enter model context or display projections.
func SanitizeOpaque(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	if looksLikeOpaque(value) {
		return opaqueSummary(len([]rune(value)))
	}
	decoder := json.NewDecoder(strings.NewReader(value))
	decoder.UseNumber()
	var payload any
	if err := decoder.Decode(&payload); err != nil {
		return value
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return value
	}
	sanitized, changed := sanitizeOpaqueJSON(payload)
	if !changed {
		return value
	}
	encoded, err := json.Marshal(sanitized)
	if err != nil {
		return value
	}
	return string(encoded)
}

func sanitizeOpaqueJSON(value any) (any, bool) {
	switch item := value.(type) {
	case string:
		if looksLikeOpaque(item) {
			return opaqueSummary(len([]rune(item))), true
		}
		return item, false
	case []any:
		changed := false
		for index, child := range item {
			sanitized, childChanged := sanitizeOpaqueJSON(child)
			if childChanged {
				item[index] = sanitized
			}
			changed = changed || childChanged
		}
		return item, changed
	case map[string]any:
		changed := false
		for key, child := range item {
			sanitized, childChanged := sanitizeOpaqueJSON(child)
			if childChanged {
				item[key] = sanitized
			}
			changed = changed || childChanged
		}
		return item, changed
	default:
		return value, false
	}
}

func looksLikeOpaque(value string) bool {
	text := strings.TrimSpace(value)
	prefix := text
	if len(prefix) > 128 {
		prefix = prefix[:128]
	}
	if strings.HasPrefix(strings.ToLower(prefix), "data:") && strings.Contains(strings.ToLower(prefix), ";base64,") {
		return true
	}
	runes := []rune(text)
	if len(runes) < 1024 || strings.ContainsAny(text, " \n\t{}[],:") {
		return false
	}
	base64ish := 0
	for _, char := range runes {
		if char >= 'A' && char <= 'Z' ||
			char >= 'a' && char <= 'z' ||
			char >= '0' && char <= '9' ||
			char == '+' || char == '/' || char == '=' || char == '-' || char == '_' {
			base64ish++
		}
	}
	return float64(base64ish)/float64(len(runes)) > 0.95
}

func opaqueSummary(originalChars int) string {
	return fmt.Sprintf("[Opaque tool payload omitted from model context: %d characters]", originalChars)
}
