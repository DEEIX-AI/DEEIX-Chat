package knowledgebase

import "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/apperr"

var (
	// ErrKnowledgeBaseNotFound 表示知识库不存在或当前用户无权访问。
	ErrKnowledgeBaseNotFound = apperr.New("knowledge_base.not_found", "knowledge base not found")
	// ErrInvalidKnowledgeBase 表示知识库请求不合法。
	ErrInvalidKnowledgeBase = apperr.NewMasked("knowledge_base.invalid", "invalid knowledge base request", "invalid knowledge base")
	// ErrKnowledgeBaseConflict 表示同一作用域下知识库名称冲突。
	ErrKnowledgeBaseConflict = apperr.New("knowledge_base.conflict", "knowledge base conflict")
	// ErrKnowledgeBaseFileNotFound 表示文件不存在、不可访问或未关联。
	ErrKnowledgeBaseFileNotFound = apperr.NewMasked("knowledge_base.not_found", "knowledge base not found", "knowledge base file not found")
	// ErrKnowledgeBaseFileContentUnavailable 表示文件内容读取能力暂不可用。
	ErrKnowledgeBaseFileContentUnavailable = apperr.NewMasked("knowledge_base.internal", "knowledge base operation failed", "knowledge base file content unavailable")
	// ErrKnowledgeBaseFileCleanupUnavailable 表示请求同步删除文件，但文件安全清理能力不可用。
	ErrKnowledgeBaseFileCleanupUnavailable = apperr.NewMasked("knowledge_base.file_cleanup_unavailable", "platform file cleanup unavailable", "knowledge base file cleanup unavailable")
	// ErrPlatformFileInUse 表示平台资料仍被知识库或其他资源引用，不能删除。
	ErrPlatformFileInUse = apperr.New("knowledge_base.platform_file_in_use", "platform file is in use")
)
