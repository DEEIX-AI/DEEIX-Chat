package knowledgebase

import "errors"

// ErrReferenceUnavailable 表示请求引用的知识库不存在、已停用或当前用户不可见。
// 该稳定领域错误允许会话等其他应用模块在不依赖知识库应用实现的情况下映射错误。
var (
	ErrReferenceUnavailable = errors.New("knowledge base reference unavailable")
	// ErrBuiltinFileOwnerDeleteBlocked 表示账号仍拥有被内置知识库引用的文件，不能直接删除。
	ErrBuiltinFileOwnerDeleteBlocked = errors.New("user owns files referenced by builtin knowledge bases")
)
