package knowledgebase

import "errors"

var (
	// ErrKnowledgeBaseNotFound 表示知识库不存在或当前用户无权访问。
	ErrKnowledgeBaseNotFound = errors.New("knowledge base not found")
	// ErrInvalidKnowledgeBase 表示知识库请求不合法。
	ErrInvalidKnowledgeBase = errors.New("invalid knowledge base")
	// ErrKnowledgeBaseConflict 表示同一作用域下知识库名称冲突。
	ErrKnowledgeBaseConflict = errors.New("knowledge base conflict")
	// ErrKnowledgeBaseFileNotFound 表示文件不存在、不可访问或未关联。
	ErrKnowledgeBaseFileNotFound = errors.New("knowledge base file not found")
	// ErrKnowledgeBaseFileContentUnavailable 表示文件内容读取能力暂不可用。
	ErrKnowledgeBaseFileContentUnavailable = errors.New("knowledge base file content unavailable")
	// ErrKnowledgeBaseFileCleanupUnavailable 表示请求同步删除文件，但文件安全清理能力不可用。
	ErrKnowledgeBaseFileCleanupUnavailable = errors.New("knowledge base file cleanup unavailable")
	// ErrPlatformFileInUse 表示平台资料仍被知识库或其他资源引用，不能删除。
	ErrPlatformFileInUse = errors.New("platform file is in use")
)
