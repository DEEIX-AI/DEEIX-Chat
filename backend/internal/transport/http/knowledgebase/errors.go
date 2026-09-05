package knowledgebase

import "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/apperr"

// 由 transport 层直接判定的请求错误（路径参数、必填校验、运行时依赖缺失等）。
// 错误码与文案是前端依赖的 API 契约，改动需同步 frontend/i18n/messages/*/errors.json。
var (
	errFileEmbeddingUnavailable = apperr.New("file.embedding_unavailable", "embedding is unavailable for this file size")
	errFileRequired             = apperr.New("file.required", "file is required")
	errInvalidFile              = apperr.New("request.invalid_file", "invalid file")
	errInvalidFileStream        = apperr.New("file.invalid_stream", "invalid file stream")
	errStorageQuotaExceeded     = apperr.New("quota.exceeded", "storage quota exceeded")
)
