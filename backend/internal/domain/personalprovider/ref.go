package personalprovider

import "strings"

// ModelRefPrefix 是个人模型引用的固定前缀。个人模型与平台模型分属不同命名空间：
// 平台模型名不允许以此开头，路由与计费据此把个人模型与平台资源完全隔离。
const ModelRefPrefix = "personal:"

// MaxModelNameLength 限制上游模型名长度，保证完整引用放得进会话的模型字段（128）。
const MaxModelNameLength = 100

// FormatModelRef 生成个人模型引用：personal:<provider public id>/<上游模型名>。
func FormatModelRef(providerPublicID string, model string) string {
	return ModelRefPrefix + strings.TrimSpace(providerPublicID) + "/" + strings.TrimSpace(model)
}

// IsModelRef 报告 name 是否落在个人模型命名空间。只看前缀，格式是否合法由 ParseModelRef 判断。
func IsModelRef(name string) bool {
	return strings.HasPrefix(strings.TrimSpace(name), ModelRefPrefix)
}

// ParseModelRef 解析个人模型引用；格式不合法时 ok 为 false。
func ParseModelRef(ref string) (providerPublicID string, model string, ok bool) {
	rest, found := strings.CutPrefix(strings.TrimSpace(ref), ModelRefPrefix)
	if !found {
		return "", "", false
	}
	providerPublicID, model, found = strings.Cut(rest, "/")
	if !found || !IsValidPublicID(providerPublicID) || !IsValidModelName(model) {
		return "", "", false
	}
	return providerPublicID, model, true
}

// IsValidPublicID 报告供应商公开 ID 是否为服务端生成的格式（小写字母与数字）。
func IsValidPublicID(value string) bool {
	if len(value) < 8 || len(value) > 32 {
		return false
	}
	for _, char := range value {
		if (char < 'a' || char > 'z') && (char < '0' || char > '9') {
			return false
		}
	}
	return true
}

// MaxIconLength 是图标 slug 的最大长度。
const MaxIconLength = 64

// IsValidIcon 报告图标是否为内置图标 slug（小写字母、数字与连字符）。只接受 slug、不接受 URL，
// 避免用户配置的外链图片在管理员浏览器里加载（追踪像素）。空串表示自动匹配，也合法。
func IsValidIcon(icon string) bool {
	if icon == "" {
		return true
	}
	if len(icon) > MaxIconLength || icon[0] == '-' {
		return false
	}
	for _, char := range icon {
		if (char < 'a' || char > 'z') && (char < '0' || char > '9') && char != '-' {
			return false
		}
	}
	return true
}

// IsValidModelName 报告上游模型名是否可安全保存与展示：非空、长度受限、不含空白与控制字符。
func IsValidModelName(model string) bool {
	if model == "" || len(model) > MaxModelNameLength || strings.TrimSpace(model) != model {
		return false
	}
	for _, char := range model {
		if char < 0x21 || char == 0x7f {
			return false
		}
	}
	return true
}
