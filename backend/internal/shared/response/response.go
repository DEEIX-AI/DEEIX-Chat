package response

import (
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
func ErrorFrom(c *gin.Context, status int, err error) {
	ErrorDescribed(c, Describe(status, err))
}

// ErrorDescribed 写出已确定的错误描述。
func ErrorDescribed(c *gin.Context, description Description) {
	write(c, description.Status, description.Code, description.Message, nil)
}

// InternalError 返回通用内部错误响应（500 / internal.error）。失败原因只应进入日志。
func InternalError(c *gin.Context) {
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
	c.JSON(status, Envelope{
		ErrorMsg:  message,
		ErrorCode: code,
		Details:   details,
		RequestID: requestID(c),
		Data:      nil,
	})
}

func requestID(c *gin.Context) string {
	if c == nil {
		return ""
	}
	return c.GetString("ctx_request_id")
}
