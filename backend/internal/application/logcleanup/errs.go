package logcleanup

import "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/apperr"

// 日志清理请求的校验错误，错误码与文案是前端依赖的 API 契约。
var (
	ErrInvalidType   = apperr.New("request.invalid_log_cleanup_type", "invalid log cleanup type")
	ErrInvalidBefore = apperr.New("request.invalid_log_cleanup_before", "invalid log cleanup before")
	ErrFutureBefore  = apperr.New("log_cleanup.before_in_future", "log cleanup before must not be in the future")
)
