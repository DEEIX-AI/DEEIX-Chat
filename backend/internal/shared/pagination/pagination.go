package pagination

import "strconv"

const (
	// DefaultPage 是未提供或非法页码时使用的默认页码。
	DefaultPage = 1
	// DefaultPageSize 是未提供或非法每页数量时使用的默认值。
	DefaultPageSize = 20
	// MaxPageSize 是所有分页列表接口统一允许的每页数量上限，与前端 TablePagination 的最大选项一致。
	MaxPageSize = 1000
)

// Normalize 将页码与每页数量规范到合法范围：非正值回退为默认值，每页数量超过 MaxPageSize 时截断。
func Normalize(page int, pageSize int) (int, int) {
	if page <= 0 {
		page = DefaultPage
	}
	if pageSize <= 0 {
		pageSize = DefaultPageSize
	}
	if pageSize > MaxPageSize {
		pageSize = MaxPageSize
	}
	return page, pageSize
}

// Parse 解析 page 与 page_size 查询参数原始值；无法解析时按默认值处理。
func Parse(rawPage string, rawPageSize string) (int, int) {
	page, _ := strconv.Atoi(rawPage)
	pageSize, _ := strconv.Atoi(rawPageSize)
	return Normalize(page, pageSize)
}

// Offset 将页码与每页数量换算为 offset 与 limit；页码过大导致乘法溢出时 offset 饱和为最大 int。
func Offset(page int, pageSize int) (int, int) {
	page, pageSize = Normalize(page, pageSize)
	pageIndex := page - 1
	maxInt := int(^uint(0) >> 1)
	if pageIndex > maxInt/pageSize {
		return maxInt, pageSize
	}
	return pageIndex * pageSize, pageSize
}
