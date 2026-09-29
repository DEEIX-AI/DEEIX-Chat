package contentmoderation

import domaincm "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/contentmoderation"

// 供应用层代码使用的方向 / 模态别名。
const (
	DirectionInput  = domaincm.DirectionInput
	DirectionOutput = domaincm.DirectionOutput
	ModalityText    = domaincm.ModalityText
	ModalityImage   = domaincm.ModalityImage
)

// 官方 Omni Moderation 类别（共 13 个）。
var allTextCategories = []string{
	"hate",
	"hate/threatening",
	"harassment",
	"harassment/threatening",
	"self-harm",
	"self-harm/intent",
	"self-harm/instructions",
	"sexual",
	"sexual/minors",
	"violence",
	"violence/graphic",
	"illicit",
	"illicit/violent",
}

// 适用于图片的类别（不含官方仅文本类别）。
var imageCategories = []string{
	"self-harm",
	"self-harm/intent",
	"self-harm/instructions",
	"sexual",
	"violence",
	"violence/graphic",
}

// textOnlyCategories 不在图片策略页面展示或配置。
var textOnlyCategories = map[string]struct{}{
	"hate":                   {},
	"hate/threatening":       {},
	"harassment":             {},
	"harassment/threatening": {},
	"sexual/minors":          {},
	"illicit":                {},
	"illicit/violent":        {},
}

// AllTextCategories 返回完整官方类别集合，作为管理界面默认值。
func AllTextCategories() []string {
	return append([]string(nil), allTextCategories...)
}

// ImageCategories 返回可用于图片审核的类别。
func ImageCategories() []string {
	return append([]string(nil), imageCategories...)
}

// IsTextOnlyCategory 报告类别是否仅适用于文本。
func IsTextOnlyCategory(category string) bool {
	_, ok := textOnlyCategories[category]
	return ok
}

// IsKnownCategory 报告类别是否在官方列表中。
func IsKnownCategory(category string) bool {
	for _, item := range allTextCategories {
		if item == category {
			return true
		}
	}
	return false
}

// IsImageCategory 报告类别是否适用于图片。
func IsImageCategory(category string) bool {
	for _, item := range imageCategories {
		if item == category {
			return true
		}
	}
	return false
}
