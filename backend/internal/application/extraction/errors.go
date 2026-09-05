package extraction

import (
	"errors"
	"strings"
)

// ErrorCodeProvider exposes the stable processing code for an extraction error.
// The underlying error remains available to logs through Unwrap.
type ErrorCodeProvider interface {
	ErrorCode() string
}

// NewOCRError attaches a provider-scoped, stable OCR failure code to cause.
// The reason is selected by the extraction workflow and is never inferred from
// the underlying provider error text.
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

// NewError attaches a stable extraction code to an underlying cause.
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

// ErrorCode returns a stable extraction code, or an empty string for unrelated errors.
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
