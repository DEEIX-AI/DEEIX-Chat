package user

// UserDailyActivityItem 单日活跃度聚合响应。
type UserDailyActivityItem struct {
	Date         string `json:"date"`
	RequestCount int64  `json:"requestCount"`
	TokenUsage   int64  `json:"tokenUsage"`
}

// UserDailyActivityListResponseDoc 每日活跃度聚合响应。
type UserDailyActivityListResponseDoc struct {
	ErrorMsg string                  `json:"errorMsg"`
	Data     []UserDailyActivityItem `json:"data"`
}

// ErrorDoc 错误响应。
type ErrorDoc struct {
	ErrorMsg string `json:"errorMsg"`
}
