package pagination

import "strconv"

const (
	DefaultPage     = 1
	DefaultPageSize = 20
	MaxPageSize     = 1000
)

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

func Parse(rawPage string, rawPageSize string) (int, int) {
	page, _ := strconv.Atoi(rawPage)
	pageSize, _ := strconv.Atoi(rawPageSize)
	return Normalize(page, pageSize)
}

func Offset(page int, pageSize int) (int, int) {
	page, pageSize = Normalize(page, pageSize)
	pageIndex := page - 1
	maxInt := int(^uint(0) >> 1)
	if pageIndex > maxInt/pageSize {
		return maxInt, pageSize
	}
	return pageIndex * pageSize, pageSize
}
