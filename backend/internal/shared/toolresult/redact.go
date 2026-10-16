package toolresult

import (
	"encoding/json"
	"io"
	"strings"
)

// RedactSecret strips an issued bearer from JSON values/keys (including JSON
// text blocks) after decoding escapes, before tool results are saved or shown.
func RedactSecret(raw string, secret string) string {
	if secret == "" {
		return raw
	}
	text := strings.ReplaceAll(raw, secret, "[redacted]")
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.UseNumber()
	var value any
	if decoder.Decode(&value) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return redactEmbeddedSecret(text, secret)
	}
	encoded, err := json.Marshal(redactSecretValue(value, secret))
	if err != nil {
		return text
	}
	return string(encoded)
}

func redactEmbeddedSecret(text string, secret string) string {
	var result strings.Builder
	for offset := 0; offset < len(text); {
		next := strings.IndexAny(text[offset:], "{[")
		if next < 0 {
			result.WriteString(text[offset:])
			break
		}
		next += offset
		result.WriteString(text[offset:next])
		decoder := json.NewDecoder(strings.NewReader(text[next:]))
		decoder.UseNumber()
		var value any
		if decoder.Decode(&value) == nil {
			encoded, err := json.Marshal(redactSecretValue(value, secret))
			if err == nil {
				result.Write(encoded)
				offset = next + int(decoder.InputOffset())
				continue
			}
		}
		// Presentation only decodes complete JSON blocks. Skip malformed lines
		// once, bounding work on arbitrary tool prose with many opening brackets.
		lineEnd := strings.IndexByte(text[next:], '\n')
		if lineEnd < 0 {
			result.WriteString(text[next:])
			break
		}
		lineEnd += next + 1
		result.WriteString(text[next:lineEnd])
		offset = lineEnd
	}
	return result.String()
}

func redactSecretValue(value any, secret string) any {
	switch item := value.(type) {
	case string:
		return RedactSecret(item, secret)
	case []any:
		for i, child := range item {
			item[i] = redactSecretValue(child, secret)
		}
	case map[string]any:
		result := make(map[string]any, len(item))
		for key, child := range item {
			result[strings.ReplaceAll(key, secret, "[redacted]")] = redactSecretValue(child, secret)
		}
		return result
	}
	return value
}
