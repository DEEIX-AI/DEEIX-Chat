// Package apperr 定义携带稳定错误码的应用层错误。
//
// 错误码与对外文案随错误值一起声明，传输层用 errors.As 读取后直接写入响应，
// 不再从 err.Error() 的文案推断；errors.Is 仍按哨兵身份比较，用法与 errors.New 一致：
//
//	var ErrConversationNotFound = apperr.New("conversation.not_found", "conversation not found")
//
//	if errors.Is(err, ErrConversationNotFound) { ... }
//	response.ErrorFrom(c, http.StatusNotFound, err) // errorCode=conversation.not_found
//
// 错误码是前端本地化与业务分支依赖的 API 契约，形如 "<namespace>.<snake_case>"；改动错误码等同于改动接口。
package apperr

import "errors"

// Error 是携带稳定错误码的错误值。Code 与 Message 构成对外 API 契约；Error() 返回的内部文本
// 只用于日志与错误链，可以与对外文案不同（例如出于安全原因不对外区分"账号已锁定"与"密码错误"）。
type Error struct {
	code    string
	message string
	text    string
}

// New 定义对外文案与内部文本一致的错误。code 与 message 为空属于编程错误，直接 panic。
func New(code string, message string) *Error {
	return NewMasked(code, message, message)
}

// NewMasked 定义对外文案与内部文本不同的错误：日志与 Error() 使用 text，API 只暴露 message。
func NewMasked(code string, message string, text string) *Error {
	if code == "" || message == "" || text == "" {
		panic("apperr: code, message and text are required")
	}
	return &Error{code: code, message: message, text: text}
}

// Error 返回内部文本。
func (e *Error) Error() string { return e.text }

// Code 返回稳定错误码。
func (e *Error) Code() string { return e.code }

// Message 返回可对外展示的文案。
func (e *Error) Message() string { return e.message }

// Find 返回错误链中最先出现的 *Error。
func Find(err error) (*Error, bool) {
	var target *Error
	if errors.As(err, &target) {
		return target, true
	}
	return nil, false
}

// Code 返回错误链中最先出现的类型化错误的错误码；链中没有类型化错误时返回空串。
func Code(err error) string {
	if target, ok := Find(err); ok {
		return target.Code()
	}
	return ""
}

// MessageOr returns the public message from the first typed error in err's
// chain, or fallback when the chain contains no typed application error.
func MessageOr(err error, fallback string) string {
	if target, ok := Find(err); ok {
		return target.Message()
	}
	return fallback
}
