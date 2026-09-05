package conv

import (
	"fmt"
	"strconv"
	"strings"
)

// NormalizePublicID 移除 UUID 连字符并去除首尾空白。
func NormalizePublicID(raw string) string {
	return strings.ReplaceAll(strings.TrimSpace(raw), "-", "")
}

// GetStringFromAny 将任意类型转换为字符串。
func GetStringFromAny(raw any) string {
	switch value := raw.(type) {
	case string:
		return value
	case fmt.Stringer:
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
