package middleware

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	apperrorlog "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/errorlog"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// maxLoggedErrorBytes 限制单条访问日志中原始错误文本的长度，避免上游响应体等大文本撑爆日志。
const maxLoggedErrorBytes = 4096

// ErrorRecorder 持久化需要管理员按错误 ID 回溯的失败请求。
type ErrorRecorder interface {
	Record(ctx context.Context, input apperrorlog.RecordInput)
}

// AccessLog 输出请求访问日志。
// 错误响应额外记录对外错误码与原始失败原因：客户端只拿到错误码和 RequestID，
// 管理员凭 RequestID 在这里找到完整原因。每个请求只输出这一条，handler 不再重复记录同一错误。
// recorder 非空时，5xx 与写出 200 后才失败的流式请求还会写入错误日志，供后台日志中心检索；
// 普通 4xx 是调用方可自行处理的错误，只留在访问日志里。nil 表示不持久化（测试与本地 sidecar）。
func AccessLog(logger *zap.Logger, recorder ErrorRecorder) gin.HandlerFunc {
	if logger == nil {
		logger = zap.NewNop()
	}
	return func(c *gin.Context) {
		if skipAccessLog(c) {
			c.Next()
			return
		}

		started := time.Now()
		c.Next()

		latency := time.Since(started)
		status := c.Writer.Status()
		traceID := MustTraceID(c)
		requestID := MustRequestID(c)
		userID := MustUserID(c)
		path := c.Request.URL.Path
		if rawQuery := c.Request.URL.RawQuery; rawQuery != "" {
			path = path + "?" + redactQuery(rawQuery)
		}
		route := c.FullPath()
		if route == "" {
			route = "-"
		}

		var message strings.Builder
		fmt.Fprintf(&message,
			"[AccessLog] %s %s\nRoute: %s\nStartTime: %s\nEndTime: %s\nLatency: %d\nClientIP: %s\nResponse: %d %d\nRequestID: %s\nTraceID: %s\nUserID: %d\nUserAgent: %s",
			c.Request.Method,
			path,
			route,
			started.Format(time.RFC3339),
			started.Add(latency).Format(time.RFC3339),
			latency.Milliseconds(),
			c.ClientIP(),
			status,
			c.Writer.Size(),
			requestID,
			traceID,
			userID,
			c.Request.UserAgent(),
		)
		errorCode := response.WrittenErrorCode(c)
		if errorCode != "" {
			fmt.Fprintf(&message, "\nErrorCode: %s", errorCode)
		}
		cause := response.RecordedErrorCause(c)
		causeText := ""
		switch {
		case cause != nil:
			causeText = truncateLoggedError(redactSecrets(cause.Error()))
		case status >= 500:
			// 5xx 却没有记录原因，说明调用点漏传错误；显式标出来，避免管理员误以为日志不全。
			causeText = noCauseRecorded
		}
		if causeText != "" {
			fmt.Fprintf(&message, "\nError: %s", causeText)
		}

		if recorder != nil && shouldPersistRequestError(status, errorCode, cause) {
			recorder.Record(c.Request.Context(), apperrorlog.RecordInput{
				RequestID:  requestID,
				TraceID:    traceID,
				UserID:     userID,
				Method:     c.Request.Method,
				Route:      route,
				Path:       path,
				StatusCode: status,
				ErrorCode:  errorCode,
				Cause:      causeText,
				Detail:     errorDiagnosticDetail(cause),
				LatencyMS:  latency.Milliseconds(),
				ClientIP:   c.ClientIP(),
				UserAgent:  c.Request.UserAgent(),
				OccurredAt: started,
			})
		}

		switch {
		case status >= 500:
			logger.Error(message.String())
		case status >= 400 || errorCode != "" || cause != nil:
			// 流式接口在写出 200 后才可能失败，终态错误同样按告警记录。
			logger.Warn(message.String())
		default:
			logger.Info(message.String())
		}
	}
}

const noCauseRecorded = "<no cause recorded>"

// errorDiagnostic 由错误类型实现，用于把错误链携带的原始诊断内容（如上游响应体）交给错误日志。
// 内容会与 Cause 一样经过凭证遮蔽与长度截断；错误日志只对管理员开放。
type errorDiagnostic interface {
	DiagnosticDetail() string
}

// errorDiagnosticDetail 从错误链中取出第一个可用的原始诊断内容，避免在中间件里依赖具体错误类型。
func errorDiagnosticDetail(err error) string {
	if err == nil {
		return ""
	}
	var diagnostic errorDiagnostic
	if !errors.As(err, &diagnostic) {
		return ""
	}
	return redactSecrets(diagnostic.DiagnosticDetail())
}

// shouldPersistRequestError 选出需要进入错误日志的请求：服务端失败（5xx），以及响应头已是 2xx、
// 但以终态错误事件结束的流式请求（生成失败的错误 ID 对任意错误码都展示，管理员需要能查到）。
func shouldPersistRequestError(status int, errorCode string, cause error) bool {
	if status >= 500 {
		return true
	}
	return status < 400 && (errorCode != "" || cause != nil)
}

func truncateLoggedError(text string) string {
	if len(text) <= maxLoggedErrorBytes {
		return text
	}
	cut := maxLoggedErrorBytes
	for cut > 0 && !utf8.RuneStart(text[cut]) {
		cut--
	}
	return fmt.Sprintf("%s...(truncated, %d bytes total)", text[:cut], len(text))
}

const redactedValue = "REDACTED"

// sensitiveQueryKeys 是查询参数中可能携带凭证的键（小写比较）。OAuth 回调、分享链接、
// 签名下载地址都会把一次性凭证放在 query 里，访问日志只保留键名。
var sensitiveQueryKeys = map[string]struct{}{
	"access_token": {}, "api_key": {}, "apikey": {}, "code": {}, "id_token": {}, "key": {},
	"password": {}, "refresh_token": {}, "secret": {}, "sig": {}, "signature": {}, "state": {},
	"ticket": {}, "token": {}, "x-amz-credential": {}, "x-amz-security-token": {}, "x-amz-signature": {},
	"x-goog-credential": {}, "x-goog-signature": {},
}

// redactQuery 保留查询参数的顺序与键名，只替换敏感键的值。
func redactQuery(rawQuery string) string {
	parts := strings.Split(rawQuery, "&")
	for index, part := range parts {
		key, _, hasValue := strings.Cut(part, "=")
		if !hasValue {
			continue
		}
		if _, sensitive := sensitiveQueryKeys[strings.ToLower(key)]; sensitive {
			parts[index] = key + "=" + redactedValue
		}
	}
	return strings.Join(parts, "&")
}

var (
	// loggedURLPattern 匹配错误文本中的 URL；Go 的 *url.Error 会把完整请求地址（含签名 query）写进 Error()。
	loggedURLPattern = regexp.MustCompile(`[A-Za-z][A-Za-z0-9+.-]*://[^\s"'<>]+`)
	// bearerPattern 匹配 Authorization 头被拼进错误文本的情况；要求 16 位以上，避免误伤 "bearer authentication" 这类文本。
	bearerPattern = regexp.MustCompile(`(?i)\b(bearer)\s+[A-Za-z0-9._~+/=-]{16,}`)
	// secretAssignmentPattern 只匹配 key=value 形式的凭证；不匹配 "key: value"，以免误伤 "invalid token: expired" 这类诊断文本。
	secretAssignmentPattern = regexp.MustCompile(`(?i)\b(api[_-]?key|access[_-]?token|refresh[_-]?token|client[_-]?secret|secret|password|token)=[^\s&"',;]+`)
)

// redactSecrets 在写入日志前去掉错误文本中的凭证：URL 的 userinfo 与敏感 query 值、Bearer 凭证、
// 显式的 key=value 凭证。它是兜底措施，不替代“不要把凭证拼进错误”的约定。
func redactSecrets(text string) string {
	text = loggedURLPattern.ReplaceAllStringFunc(text, redactLoggedURL)
	text = bearerPattern.ReplaceAllString(text, "$1 "+redactedValue)
	return secretAssignmentPattern.ReplaceAllString(text, "${1}="+redactedValue)
}

func redactLoggedURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	if parsed.User != nil {
		parsed.User = url.User(redactedValue)
	}
	if parsed.RawQuery != "" {
		parsed.RawQuery = redactQuery(parsed.RawQuery)
	}
	return parsed.String()
}

func skipAccessLog(c *gin.Context) bool {
	return !strings.HasPrefix(c.Request.URL.Path, "/api/")
}
