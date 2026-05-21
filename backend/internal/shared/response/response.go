package response

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// Envelope 是统一接口响应体。
type Envelope struct {
	ErrorMsg  string      `json:"errorMsg"`
	ErrorCode string      `json:"errorCode,omitempty"`
	Details   interface{} `json:"details,omitempty"`
	RequestID string      `json:"requestId,omitempty"`
	Data      interface{} `json:"data"`
}

// SuccessDoc 用于 swagger 标注通用成功响应。
type SuccessDoc struct {
	ErrorMsg  string      `json:"errorMsg" example:""`
	ErrorCode string      `json:"errorCode,omitempty" example:""`
	Details   interface{} `json:"details,omitempty"`
	RequestID string      `json:"requestId,omitempty" example:""`
	Data      interface{} `json:"data"`
}

// PageData 是分页响应数据。
type PageData[T any] struct {
	Total   int64 `json:"total"`
	Results []T   `json:"results"`
}

// Success 返回成功响应。
func Success(c *gin.Context, data interface{}) {
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

// Error 返回错误响应。
func Error(c *gin.Context, status int, msg string) {
	code := InferErrorCode(status, msg)
	ErrorWithDetails(c, status, code, PublicErrorMessage(status, code, msg), nil)
}

// ErrorFrom maps an error through the same normalized error response pipeline.
func ErrorFrom(c *gin.Context, status int, err error) {
	msg := ""
	if err != nil {
		msg = strings.TrimSpace(err.Error())
	}
	Error(c, status, msg)
}

// ErrorWithCode 返回带稳定错误码的错误响应。
func ErrorWithCode(c *gin.Context, status int, code string, msg string) {
	ErrorWithDetails(c, status, code, PublicErrorMessage(status, code, msg), nil)
}

// ErrorWithDetails 返回带稳定错误码、调试信息和请求 ID 的错误响应。
func ErrorWithDetails(c *gin.Context, status int, code string, msg string, details interface{}) {
	if code == "" {
		code = InferErrorCode(status, msg)
	}
	msg = PublicErrorMessage(status, code, msg)
	c.JSON(status, Envelope{
		ErrorMsg:  msg,
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
