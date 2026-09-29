package processing

import (
	"errors"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/apperr"
)

// 文件处理流程的错误哨兵。
var (
	// ErrFileProcessingFailed 表示文件处理失败。
	ErrFileProcessingFailed = errors.New("file processing failed")
	// errExtractionServiceNotConfigured 表示文件处理服务未注入抽取服务。
	errExtractionServiceNotConfigured = errors.New("extraction service not configured")
	// ErrFileNotFound 表示当前用户名下不存在该活跃文件。
	ErrFileNotFound            = apperr.New("file.not_found", "file not found")
	errFileProcessingClaimLost = errors.New("file processing claim lost")
)
