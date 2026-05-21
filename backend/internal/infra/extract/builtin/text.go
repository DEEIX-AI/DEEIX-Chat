package builtin

import "strings"

// ExtractText 直接返回文本文件内容。
func ExtractText(data []byte) string {
	return strings.TrimSpace(string(data))
}
