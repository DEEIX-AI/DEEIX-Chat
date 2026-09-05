package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"
)

// MaxUpstreamDebugBodyBytes 是调试请求/响应体保留的最大字节数。
const MaxUpstreamDebugBodyBytes = 128 * 1024

// SanitizedUpstreamDebugBody 是调试请求/响应体经过脱敏与限长后的结果。
type SanitizedUpstreamDebugBody struct {
	Body          string
	OriginalBytes int
	Truncated     bool
	RedactedParts int
}

// SanitizeUpstreamDebugBody removes inline binary data and bounds a debug body
// before it crosses into application-level errors or trace payloads.
func SanitizeUpstreamDebugBody(raw string) string {
	return SanitizeUpstreamDebugPayload([]byte(raw)).Body
}

// SanitizeUpstreamDebugPayload 对原始调试体做脱敏与限长，返回含统计信息的结果。
func SanitizeUpstreamDebugPayload(raw []byte) SanitizedUpstreamDebugBody {
	result := SanitizedUpstreamDebugBody{OriginalBytes: len(raw)}
	if len(raw) == 0 {
		return result
	}
	if len(raw) > MaxUpstreamDebugBodyBytes {
		result.Body = upstreamDebugBodySummary(result.OriginalBytes, 0, "body_too_large")
		result.Truncated = true
		return result
	}

	var payload any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err == nil {
		payload = sanitizeUpstreamDebugValue(payload, "", nil, &result.RedactedParts)
		encoded, marshalErr := json.Marshal(payload)
		if marshalErr == nil && len(encoded) <= MaxUpstreamDebugBodyBytes {
			result.Body = string(encoded)
			return result
		}
		result.Body = upstreamDebugBodySummary(result.OriginalBytes, result.RedactedParts, "body_too_large")
		result.Truncated = true
		return result
	}

	if !utf8.Valid(raw) {
		result.Body = upstreamDebugBodySummary(result.OriginalBytes, 1, "binary_body")
		result.RedactedParts = 1
		result.Truncated = true
		return result
	}

	if sanitized, redacted, ok := sanitizeUpstreamDebugSSE(string(raw)); ok {
		result.RedactedParts = redacted
		if len(sanitized) <= MaxUpstreamDebugBodyBytes {
			result.Body = sanitized
			return result
		}
	}
	result.Body = redactDebugDataURLs(string(raw), &result.RedactedParts)
	return result
}

func sanitizeUpstreamDebugValue(value any, parentKey string, parent map[string]any, redactedParts *int) any {
	switch current := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(current))
		for key, child := range current {
			if text, ok := child.(string); ok && shouldRedactDebugString(key, parentKey, current, text) {
				result[key] = upstreamDebugBinaryPlaceholder(key, text)
				(*redactedParts)++
				continue
			}
			result[key] = sanitizeUpstreamDebugValue(child, key, current, redactedParts)
		}
		return result
	case []any:
		result := make([]any, len(current))
		for index, child := range current {
			result[index] = sanitizeUpstreamDebugValue(child, parentKey, parent, redactedParts)
		}
		return result
	case string:
		if isDebugDataURL(current) {
			(*redactedParts)++
			return upstreamDebugBinaryPlaceholder(parentKey, current)
		}
		if sanitized, ok := sanitizeNestedDebugJSONString(current, redactedParts); ok {
			return sanitized
		}
	}
	return value
}

func sanitizeNestedDebugJSONString(value string, redactedParts *int) (string, bool) {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) < 2 || (trimmed[0] != '{' && trimmed[0] != '[') {
		return value, false
	}
	var payload any
	decoder := json.NewDecoder(strings.NewReader(trimmed))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return value, false
	}
	switch payload.(type) {
	case map[string]any, []any:
	default:
		return value, false
	}
	before := *redactedParts
	sanitized := sanitizeUpstreamDebugValue(payload, "", nil, redactedParts)
	if *redactedParts == before {
		return value, true
	}
	encoded, err := json.Marshal(sanitized)
	if err != nil {
		return value, false
	}
	return string(encoded), true
}

func shouldRedactDebugString(key string, parentKey string, parent map[string]any, value string) bool {
	if isDebugDataURL(value) {
		return true
	}
	normalizedKey := normalizeDebugFieldName(key)
	switch normalizedKey {
	case "b64json", "base64", "imagebase64", "audiobase64", "filebase64", "filedata":
		return strings.TrimSpace(value) != ""
	case "data":
		normalizedParent := normalizeDebugFieldName(parentKey)
		if normalizedParent == "inlinedata" || normalizedParent == "inlineimage" || normalizedParent == "inputaudio" || normalizedParent == "source" {
			return strings.TrimSpace(value) != ""
		}
		if debugMapDeclaresBase64(parent) {
			return strings.TrimSpace(value) != ""
		}
		return looksLikeBase64Payload(value)
	default:
		return false
	}
}

func normalizeDebugFieldName(value string) string {
	replacer := strings.NewReplacer("_", "", "-", "", ".", "")
	return strings.ToLower(replacer.Replace(strings.TrimSpace(value)))
}

func debugMapDeclaresBase64(value map[string]any) bool {
	for _, key := range []string{"type", "encoding", "format"} {
		if text, ok := value[key].(string); ok && strings.EqualFold(strings.TrimSpace(text), "base64") {
			return true
		}
	}
	for _, key := range []string{"media_type", "mediaType", "mime_type", "mimeType"} {
		if text, ok := value[key].(string); ok && strings.HasPrefix(strings.ToLower(strings.TrimSpace(text)), "image/") {
			return true
		}
	}
	return false
}

func isDebugDataURL(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	return strings.HasPrefix(normalized, "data:") && strings.Contains(normalized[:min(len(normalized), 256)], ";base64,")
}

func looksLikeBase64Payload(value string) bool {
	normalized := strings.TrimSpace(value)
	if len(normalized) < 256 || len(normalized)%4 != 0 {
		return false
	}
	for _, current := range normalized {
		if (current >= 'a' && current <= 'z') || (current >= 'A' && current <= 'Z') || (current >= '0' && current <= '9') || current == '+' || current == '/' || current == '=' || current == '\r' || current == '\n' {
			continue
		}
		return false
	}
	return true
}

func upstreamDebugBinaryPlaceholder(key string, value string) string {
	mimeType := "application/octet-stream"
	encodedBytes := len(strings.TrimSpace(value))
	if isDebugDataURL(value) {
		normalized := strings.TrimSpace(value)
		if separator := strings.Index(normalized, ";"); separator > len("data:") {
			mimeType = normalized[len("data:"):separator]
		}
		if comma := strings.Index(normalized, ","); comma >= 0 {
			encodedBytes = len(normalized) - comma - 1
		}
	} else if strings.Contains(strings.ToLower(key), "image") {
		mimeType = "image/*"
	} else if strings.Contains(strings.ToLower(key), "audio") {
		mimeType = "audio/*"
	}
	return fmt.Sprintf("[binary omitted; mime=%s; encoded_bytes=%d]", mimeType, encodedBytes)
}

func upstreamDebugBodySummary(originalBytes int, redactedParts int, reason string) string {
	payload := map[string]any{
		"_debug": map[string]any{
			"bodyOmitted":   true,
			"originalBytes": originalBytes,
			"reason":        reason,
			"redactedParts": redactedParts,
		},
	}
	encoded, _ := json.Marshal(payload)
	return string(encoded)
}

func sanitizeUpstreamDebugSSE(raw string) (string, int, bool) {
	lines := strings.SplitAfter(raw, "\n")
	redactedParts := 0
	parsed := false
	for index, line := range lines {
		content := strings.TrimSuffix(line, "\n")
		lineEnding := strings.TrimPrefix(line, content)
		trimmed := strings.TrimSpace(content)
		if !strings.HasPrefix(trimmed, "data:") {
			continue
		}
		body := strings.TrimSpace(strings.TrimPrefix(trimmed, "data:"))
		if body == "" || body == "[DONE]" {
			continue
		}
		var payload any
		decoder := json.NewDecoder(strings.NewReader(body))
		decoder.UseNumber()
		if err := decoder.Decode(&payload); err != nil {
			continue
		}
		parsed = true
		previousRedactedParts := redactedParts
		payload = sanitizeUpstreamDebugValue(payload, "", nil, &redactedParts)
		if redactedParts == previousRedactedParts {
			continue
		}
		encoded, err := json.Marshal(payload)
		if err != nil {
			continue
		}
		prefixIndex := strings.Index(content, "data:")
		lines[index] = content[:prefixIndex] + "data: " + string(encoded) + lineEnding
	}
	return strings.Join(lines, ""), redactedParts, parsed
}

func redactDebugDataURLs(value string, redactedParts *int) string {
	result := value
	searchOffset := 0
	for {
		lower := strings.ToLower(result)
		relativeStart := strings.Index(lower[searchOffset:], "data:")
		if relativeStart < 0 {
			return result
		}
		start := searchOffset + relativeStart
		candidateEnd := min(len(lower), start+256)
		for index := start; index < candidateEnd; index++ {
			if isDebugDataURLBoundary(result[index]) {
				candidateEnd = index
				break
			}
		}
		markerOffset := strings.Index(lower[start:candidateEnd], ";base64,")
		if markerOffset < 0 {
			searchOffset = candidateEnd
			continue
		}
		marker := start + markerOffset
		payloadStart := marker + len(";base64,")
		end := payloadStart
		for end < len(result) {
			if isDebugDataURLBoundary(result[end]) {
				break
			}
			end++
		}
		placeholder := upstreamDebugBinaryPlaceholder("", result[start:end])
		result = result[:start] + placeholder + result[end:]
		(*redactedParts)++
		searchOffset = start + len(placeholder)
	}
}

func isDebugDataURLBoundary(value byte) bool {
	switch value {
	case '"', '\'', ' ', '\r', '\n', '\t', '}', ']':
		return true
	default:
		return false
	}
}
