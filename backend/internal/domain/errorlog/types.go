package errorlog

import "time"

// Log 表示一次失败请求的错误记录。Cause 是写入前已脱敏、截断的原始错误链；Detail 是错误链携带的原始诊断内容
// （如上游响应体），同样只遮蔽凭证。两者只对管理员可见。
type Log struct {
	ID         uint
	RequestID  string
	TraceID    string
	UserID     uint
	Method     string
	Route      string
	Path       string
	StatusCode int
	ErrorCode  string
	Cause      string
	Detail     string
	LatencyMS  int64
	ClientIP   string
	UserAgent  string
	CreatedAt  time.Time
}
