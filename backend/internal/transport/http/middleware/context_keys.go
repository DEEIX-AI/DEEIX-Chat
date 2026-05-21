package middleware

const (
	// ContextKeyUserID 当前登录用户ID。
	ContextKeyUserID = "ctx_user_id"
	// ContextKeyUsername 当前登录用户名。
	ContextKeyUsername = "ctx_username"
	// ContextKeyUserRole 当前登录用户角色。
	ContextKeyUserRole = "ctx_user_role"
	// ContextKeySessionID 当前登录会话ID。
	ContextKeySessionID = "ctx_session_id"
	// ContextKeyRequestID 请求追踪ID。
	ContextKeyRequestID = "ctx_request_id"
	// ContextKeyTraceID 分布式链路追踪 ID（对齐 OpenTelemetry TraceID 格式）。
	ContextKeyTraceID = "ctx_trace_id"
)
