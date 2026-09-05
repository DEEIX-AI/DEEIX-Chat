package memory

import "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/apperr"

// 由 transport 层直接判定的请求错误（路径参数、必填校验、运行时依赖缺失等）。
// 错误码与文案是前端依赖的 API 契约，改动需同步 frontend/i18n/messages/*/errors.json。
var (
	errMemoryKeyRequired       = apperr.New("memory.key_required", "memory_key is required")
	errUserMemoryLimitExceeded = apperr.New("memory.limit_exceeded", "user memory limit exceeded")
)
