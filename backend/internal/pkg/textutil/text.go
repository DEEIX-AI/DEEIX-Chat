// Package textutil 提供无业务语义的字符串处理工具。
package textutil

import "strings"

// FirstNonEmpty 返回第一个去除首尾空白后非空的字符串。
func FirstNonEmpty(values ...string) string {
	for _, value := range values {
		if normalized := strings.TrimSpace(value); normalized != "" {
			return normalized
		}
	}
	return ""
}

// FirstNonBlank 返回第一个包含非空白字符的原始字符串。
func FirstNonBlank(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

// IsASCIIAlpha 判断字符串是否仅由 ASCII 字母组成。
func IsASCIIAlpha(value string) bool {
	for _, item := range value {
		if (item < 'a' || item > 'z') && (item < 'A' || item > 'Z') {
			return false
		}
	}
	return true
}

// CompactSnippet 合并空白并按 Unicode 字符数截取可读摘要。
func CompactSnippet(content string, maxRunes int) string {
	value := strings.Join(strings.Fields(strings.TrimSpace(content)), " ")
	if value == "" {
		return ""
	}
	if maxRunes <= 0 {
		maxRunes = 120
	}
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return string(runes[:maxRunes]) + "..."
}

// TruncateTrimmed 去除首尾空白后按 Unicode 字符数截取文本。
func TruncateTrimmed(value string, maxRunes int) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if maxRunes <= 0 || len(runes) <= maxRunes {
		return value
	}
	return string(runes[:maxRunes])
}

// SanitizeForTextColumn 让可能来自工具输出的文本可安全写入数据库文本列。
//
// 工具可能返回二进制内容（例如抓取到的 GIF），其字节解码成字符串后会带来
// NUL 与非法 UTF-8；PostgreSQL 的 text 列会整体拒绝写入（SQLSTATE 22021），
// 一次写入失败就会丢掉整条事件（例如轨迹停留在「进行中」）。
func SanitizeForTextColumn(value string) string {
	if !strings.ContainsRune(value, 0) {
		return strings.ToValidUTF8(value, "\uFFFD")
	}
	return strings.ToValidUTF8(strings.ReplaceAll(value, "\x00", ""), "\uFFFD")
}
