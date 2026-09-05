// Package tokenestimate 提供提示词预算、向量化、检索、压缩和工具结果预览共用的文本 token 估算。
package tokenestimate

// Estimate 对混合 CJK 与非 CJK 文本做保守估算：CJK 约 1.5 字符/token，其他字符约 4 字符/token。
func Estimate(content string) int64 {
	if content == "" {
		return 0
	}
	var cjk int64
	var other int64
	for _, char := range content {
		if isCJK(char) {
			cjk++
		} else {
			other++
		}
	}
	return (cjk*2+2)/3 + (other+3)/4
}

func isCJK(char rune) bool {
	return (char >= 0x2E80 && char <= 0x9FFF) ||
		(char >= 0xAC00 && char <= 0xD7AF) ||
		(char >= 0xF900 && char <= 0xFAFF) ||
		(char >= 0x20000 && char <= 0x2A6DF)
}
