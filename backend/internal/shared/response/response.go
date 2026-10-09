package response

import (
	"errors"
	"net/http"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/apperr"
	"github.com/gin-gonic/gin"
)

// Envelope 是统一接口响应体。
type Envelope struct {
	ErrorMsg  string `json:"errorMsg"`
	ErrorCode string `json:"errorCode,omitempty"`
	Details   any    `json:"details,omitempty"`
	RequestID string `json:"requestId,omitempty"`
	Data      any    `json:"data"`
}

// SuccessDoc 用于 swagger 标注通用成功响应。
type SuccessDoc struct {
	ErrorMsg  string `json:"errorMsg" example:""`
	ErrorCode string `json:"errorCode,omitempty" example:""`
	Details   any    `json:"details,omitempty"`
	RequestID string `json:"requestId,omitempty" example:""`
	Data      any    `json:"data"`
}

// PageData 是分页响应数据。
type PageData[T any] struct {
	Total   int64 `json:"total"`
	Results []T   `json:"results"`
}

// Success 返回成功响应。
func Success(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Envelope{
		ErrorMsg: "",
		Data:     data,
	})
}

// SuccessPage 返回分页成功响应。
func SuccessPage[T any](c *gin.Context, total int64, results []T) {
	Success(c, PageData[T]{
		Total:   total,
		Results: results,
	})
}

// Description 是 HTTP 响应与 NDJSON 流式终态事件共用的错误契约。
type Description struct {
	Status  int
	Code    string
	Message string
}

// Describe 从错误链读取类型化应用错误。普通错误不会向客户端泄露，也不会参与错误码推断。
func Describe(status int, err error) Description {
	if coded, ok := apperr.Find(err); ok {
		return Description{Status: status, Code: coded.Code(), Message: coded.Message()}
	}
	return defaultDescription(status)
}

// DescribeCode 从响应边界登记的稳定错误码构造描述。未登记错误码安全退化为对应状态的通用契约。
func DescribeCode(status int, code string) Description {
	message, ok := canonicalMessage(code)
	if !ok {
		return defaultDescription(status)
	}
	return Description{Status: status, Code: code, Message: message}
}

// ErrorFrom 把类型化应用错误写成统一错误响应；普通错误按状态码安全退化。
// 原始错误链只记入请求上下文，由访问日志按 request_id 记录，不进入响应体。
func ErrorFrom(c *gin.Context, status int, err error) {
	recordError(c, err)
	ErrorDescribed(c, Describe(status, err))
}

// ErrorDescribed 写出已确定的错误描述。
func ErrorDescribed(c *gin.Context, description Description) {
	write(c, description.Status, description.Code, description.Message, nil)
}

// InternalError 返回通用内部错误响应（500 / internal.error）。
// err 是失败原因，只进入日志；不得传 nil 掩盖原因，依赖未配置等场景也应构造说明性错误。
func InternalError(c *gin.Context, err error) {
	recordError(c, err)
	ErrorDescribed(c, defaultDescription(http.StatusInternalServerError))
}

// InvalidQueryParam 返回查询参数解析失败响应；动态文案只包含由服务端选定的参数名。
func InvalidQueryParam(c *gin.Context, key string) {
	write(c, http.StatusBadRequest, CodeRequestInvalidQuery, "invalid "+key, nil)
}

// ErrorWithCode 写出响应边界登记的稳定错误码。未登记错误码安全退化为对应状态的通用契约。
func ErrorWithCode(c *gin.Context, status int, code string) {
	ErrorDescribed(c, DescribeCode(status, code))
}

// ErrorWithDetails 写出响应边界登记的稳定错误码及结构化详情。
func ErrorWithDetails(c *gin.Context, status int, code string, details any) {
	description := DescribeCode(status, code)
	write(c, description.Status, description.Code, description.Message, details)
}

func write(c *gin.Context, status int, code string, message string, details any) {
	if c != nil && code != "" {
		c.Set(contextKeyErrorCode, code)
	}
	c.JSON(status, Envelope{
		ErrorMsg:  message,
		ErrorCode: code,
		Details:   details,
		RequestID: requestID(c),
		Data:      nil,
	})
}

const (
	// contextKeyErrorCode 保存本次请求已写出的错误码，供访问日志与响应体保持一致。
	contextKeyErrorCode = "ctx_error_code"
	// contextKeyErrorCause 保存本次请求的原始失败原因，只供访问日志读取。
	// 不使用 gin.Context.Error：otelgin 会把 c.Errors 一律标成 span Error，4xx 也会被误报。
	contextKeyErrorCause = "ctx_error_cause"
)

// WrittenErrorCode 返回本次请求已写出的错误码；未写出错误响应时为空。
func WrittenErrorCode(c *gin.Context) string {
	if c == nil {
		return ""
	}
	return c.GetString(contextKeyErrorCode)
}

// RecordedErrorCause 返回本次请求记录的原始失败原因；未记录时为 nil。
func RecordedErrorCause(c *gin.Context) error {
	if c == nil {
		return nil
	}
	value, ok := c.Get(contextKeyErrorCause)
	if !ok {
		return nil
	}
	err, _ := value.(error)
	return err
}

// RecordError 把不经过错误响应写出的失败原因（如流式终态错误）挂到请求上下文，供访问日志记录。
func RecordError(c *gin.Context, err error) {
	recordError(c, err)
}

// RecordErrorCode 记录不经过 write 写出的错误码（如 NDJSON 终态事件），与 WrittenErrorCode 共用同一来源。
func RecordErrorCode(c *gin.Context, code string) {
	if c != nil && code != "" {
		c.Set(contextKeyErrorCode, code)
	}
}

func recordError(c *gin.Context, err error) {
	if c == nil || err == nil {
		return
	}
	previous := RecordedErrorCause(c)
	switch {
	case previous == nil:
	case errors.Is(previous, err):
		// 错误映射 helper 入口已记录同一错误，后续 ErrorFrom 不再重复拼接。
		return
	case errors.Is(err, previous):
		// 新错误包裹了已记录的错误并补充了上下文，用更完整的链替换。
	default:
		err = errors.Join(previous, err)
	}
	c.Set(contextKeyErrorCause, err)
}

func requestID(c *gin.Context) string {
	if c == nil {
		return ""
	}
	return c.GetString("ctx_request_id")
}
