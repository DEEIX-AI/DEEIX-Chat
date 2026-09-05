package user

// DailyActivity 表示单用户单日的真实模型调用活跃度聚合。
// 领域类型不携带 JSON/ORM 协议标签，序列化契约由 transport 层 DTO 承担。
type DailyActivity struct {
	// Date 是计费归属日期，格式为 YYYY-MM-DD。
	Date string
	// RequestCount 是当日由用户发起并产生真实用量的模型请求数。
	RequestCount int64
	// TokenUsage 是这些模型请求产生的 Token 总量。
	TokenUsage int64
}
