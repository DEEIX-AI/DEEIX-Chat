package middleware

import (
	"strings"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/traceid"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// maxRequestIDLength 与审计日志 request_id 列宽一致。
const maxRequestIDLength = 64

// RequestID 为每个请求注入可追踪 ID。
// RequestID 是用户反馈与管理员检索日志的“错误 ID”，必须由服务端掌控：入站 X-Request-ID 只在来自受信代理
// （需在 TrustedProxyHeaders 之后注册）且格式合法时沿用，便于网关串联；普通客户端传入的值一律忽略，
// 防止伪造或撞用他人的 ID 干扰排查，也防止日志注入与超长值。
// 日志 trace_id 优先使用当前 OpenTelemetry span，未启用 OTel 时才回退到入口透传值或本地生成值。
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := ""
		if requestCameFromTrustedProxy(c) {
			requestID = strings.TrimSpace(c.GetHeader("X-Request-ID"))
		}
		if !validRequestID(requestID) {
			requestID = uuid.NewString()
		}

		traceID := traceid.FromContext(c.Request.Context())
		if traceID == "" {
			if incomingTraceID := strings.TrimSpace(c.GetHeader("X-Trace-ID")); traceid.Valid(incomingTraceID) {
				traceID = strings.ToLower(incomingTraceID)
			}
		}
		if traceID == "" {
			traceID = traceid.Generate()
		}

		c.Set(ContextKeyRequestID, requestID)
		c.Set(ContextKeyTraceID, traceID)
		c.Writer.Header().Set("X-Request-ID", requestID)
		c.Writer.Header().Set("X-Trace-ID", traceID)

		// 将 trace_id 注入 request context，供下游 service 层使用。
		ctx := traceid.WithTraceID(c.Request.Context(), traceID)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}

// validRequestID 只接受 [A-Za-z0-9._:-]，长度 1-64；这些字符不会破坏日志行或被误读为字段分隔。
func validRequestID(value string) bool {
	if value == "" || len(value) > maxRequestIDLength {
		return false
	}
	for i := 0; i < len(value); i++ {
		char := value[i]
		switch {
		case char >= 'a' && char <= 'z', char >= 'A' && char <= 'Z', char >= '0' && char <= '9':
		case char == '.', char == '_', char == ':', char == '-':
		default:
			return false
		}
	}
	return true
}
