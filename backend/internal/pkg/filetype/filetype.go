// Package filetype 提供基于 MIME 与文件名的无状态文件类型判断。
package filetype

import "strings"

// IsText 判断文件是否属于可按文本读取的常见格式。
func IsText(mimeType string, fileName string) bool {
	normalizedMIME := strings.ToLower(strings.TrimSpace(mimeType))
	if strings.HasPrefix(normalizedMIME, "text/") {
		return true
	}
	switch normalizedMIME {
	case "application/json", "application/xml", "application/javascript", "application/typescript",
		"application/yaml", "application/x-yaml", "application/toml":
		return true
	}

	extension := ""
	if index := strings.LastIndex(fileName, "."); index >= 0 {
		extension = strings.ToLower(fileName[index+1:])
	}
	switch extension {
	case "txt", "md", "markdown", "csv", "json", "xml", "html", "htm",
		"css", "js", "ts", "jsx", "tsx", "py", "go", "rs", "java",
		"c", "cpp", "h", "hpp", "cs", "rb", "php", "swift", "kt",
		"sh", "bash", "zsh", "yaml", "yml", "toml", "ini", "conf", "sql":
		return true
	default:
		return false
	}
}

// ImageExtension 返回受支持图片 MIME 对应的文件扩展名，未知格式默认使用 PNG。
func ImageExtension(mimeType string) string {
	switch strings.ToLower(strings.TrimSpace(mimeType)) {
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	default:
		return ".png"
	}
}
