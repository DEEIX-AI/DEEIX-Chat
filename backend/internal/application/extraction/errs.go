package extraction

import (
	"errors"
	"strings"
)

// ErrorCodeProvider 暴露提取错误的稳定处理码。
// 底层错误仍可通过 Unwrap 供日志使用。
type ErrorCodeProvider interface {
	ErrorCode() string
}

// NewOCRError 为 cause 附加提供方作用域的稳定 OCR 失败码。
// 原因由提取流程选定，绝不从
// 底层提供方错误文本推断。
func NewOCRError(provider string, reason string, cause error) error {
	provider = normalizeOCREngine(provider)
	suffix := "ocr_failed"
	switch strings.TrimSpace(reason) {
	case "disabled", "ocr_disabled":
		suffix = "ocr_disabled"
	case "empty_content", "ocr_empty_content":
		suffix = "ocr_empty_content"
	case "unprocessable", "ocr_unprocessable":
		suffix = "ocr_unprocessable"
	case "unauthorized", "ocr_unauthorized":
		suffix = "ocr_unauthorized"
	case "forbidden", "ocr_forbidden":
		suffix = "ocr_forbidden"
	case "invalid_response", "ocr_invalid_response":
		suffix = "ocr_invalid_response"
	case "http_error", "ocr_http_error":
		suffix = "ocr_http_error"
	case "unavailable", "ocr_unavailable":
		suffix = "ocr_unavailable"
	}
	if cause == nil {
		cause = errors.New(suffix)
	}
	return NewError(provider+"_"+suffix, cause)
}

type codedError struct {
	code  string
	cause error
}

// NewError 为底层原因附加稳定的提取错误码。
func NewError(code string, cause error) error {
	if cause == nil {
		return nil
	}
	if code == "" {
		code = "extract_failed"
	}
	return &codedError{code: code, cause: cause}
}

func (e *codedError) Error() string     { return e.cause.Error() }
func (e *codedError) Unwrap() error     { return e.cause }
func (e *codedError) ErrorCode() string { return e.code }

func withErrorCode(err error) error {
	if err == nil {
		return nil
	}
	var provider ErrorCodeProvider
	if errors.As(err, &provider) {
		return err
	}
	return &codedError{code: "extract_failed", cause: err}
}

// ErrorCode 返回稳定的提取错误码；对无关错误返回空字符串。
func ErrorCode(err error) string {
	if err == nil {
		return ""
	}
	var provider ErrorCodeProvider
	if !errors.As(err, &provider) {
		return ""
	}
	return provider.ErrorCode()
}

// IsEmptyContent 报告 err 是否表示提取器或 OCR 引擎运行
// 成功但未发现文本。所有引擎生成此类错误码时都带有
// `_empty_content` 后缀，因此调用方将文件视为空而非失败。
func IsEmptyContent(err error) bool {
	return strings.HasSuffix(ErrorCode(err), "_empty_content")
}
