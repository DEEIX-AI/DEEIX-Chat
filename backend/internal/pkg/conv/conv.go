package conv

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// NormalizePublicID 移除 UUID 连字符并去除首尾空白。
func NormalizePublicID(raw string) string {
	return strings.ReplaceAll(strings.TrimSpace(raw), "-", "")
}

// GetStringFromAny 将任意类型转换为字符串。
func GetStringFromAny(raw interface{}) string {
	switch value := raw.(type) {
	case string:
		return value
	case fmt.Stringer:
		return value.String()
	case json.Number:
		return value.String()
	case float64:
		return strconv.FormatFloat(value, 'f', -1, 64)
	case int:
		return strconv.Itoa(value)
	case int64:
		return strconv.FormatInt(value, 10)
	case bool:
		if value {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}

// GetIntFromAny 将任意类型转换为 int。
func GetIntFromAny(raw interface{}) int {
	switch value := raw.(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	case json.Number:
		parsed, err := value.Int64()
		if err != nil {
			return 0
		}
		return int(parsed)
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil {
			return 0
		}
		return parsed
	default:
		return 0
	}
}
