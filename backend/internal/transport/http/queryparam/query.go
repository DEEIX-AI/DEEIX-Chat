// Package queryparam 提供可复用的 HTTP 查询参数解析。
package queryparam

import "strconv"

// OptionalBool 解析可选布尔值；空值或非法值返回 nil。
func OptionalBool(raw string) *bool {
	if raw == "" {
		return nil
	}
	parsed, err := strconv.ParseBool(raw)
	if err != nil {
		return nil
	}
	return &parsed
}
