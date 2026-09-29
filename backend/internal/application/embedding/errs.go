package embedding

import (
	"errors"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/apperr"
)

// 向量化服务的错误哨兵；带 apperr 错误码的是对外 API 契约。
var (
	ErrEmbeddingServiceNotConfigured = apperr.NewMasked("embedding.service_not_configured", "embedding service is not configured", "embedding service not configured")
	ErrEmbeddingServiceUnavailable   = errors.New("embedding service unavailable")
	ErrEmbeddingQueueUnavailable     = errors.New("embedding queue unavailable")
	ErrTooManyTargetedFiles          = errors.New("too many files for targeted embedding")
	errNoExtractableText             = errors.New("no extractable text in file")
	errEmptyChunks                   = errors.New("embedding produced no chunks")
	errImageTooLarge                 = errors.New("image exceeds embedding size limit after resize")
	errImageFormatUnsupported        = errors.New("image format is not supported for embedding")
	errEmbeddingConfigurationChanged = errors.New("embedding configuration changed")
)
