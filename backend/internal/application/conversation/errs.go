package conversation

import "errors"

var (
	// ErrConversationNotFound 会话不存在或无权限。
	ErrConversationNotFound = errors.New("conversation not found")
	// ErrConversationShareNotFound 会话分享不存在、已关闭或原会话已删除。
	ErrConversationShareNotFound = errors.New("conversation share not found")
	// ErrInvalidConversationShare 会话分享请求不合法。
	ErrInvalidConversationShare = errors.New("invalid conversation share")
	// ErrConversationShareSchemaOutdated 会话分享表结构未更新。
	ErrConversationShareSchemaOutdated = errors.New("conversation share schema outdated")
	// ErrInvalidConversationTitle 会话标题不合法。
	ErrInvalidConversationTitle = errors.New("invalid conversation title")
	// ErrInvalidFileReference 文件引用无效。
	ErrInvalidFileReference = errors.New("invalid file reference")
	// ErrInvalidFileName 文件名不合法。
	ErrInvalidFileName = errors.New("invalid file name")
	// ErrFileNotFound 文件不存在。
	ErrFileNotFound = errors.New("file not found")
	// ErrStorageQuotaExceeded 文件配额超限。
	ErrStorageQuotaExceeded = errors.New("storage quota exceeded")
	// ErrFileTooLarge 文件过大。
	ErrFileTooLarge = errors.New("file too large")
	// ErrMIMEBlocked 文件类型不被允许。
	ErrMIMEBlocked = errors.New("mime blocked")
	// ErrDangerousMIMEType 危险文件类型不被允许。
	ErrDangerousMIMEType = errors.New("dangerous file type not allowed")
	// ErrFileProcessingNotReady 文件处理尚未就绪。
	ErrFileProcessingNotReady = errors.New("file processing not ready")
	// ErrFileTooLargeForFullContext 文件过大，无法全文注入。
	ErrFileTooLargeForFullContext = errors.New("file too large for full context")
	// ErrEmbeddingUnavailable 当前未配置可用 embedding，无法处理大文档 / RAG。
	ErrEmbeddingUnavailable = errors.New("embedding unavailable")
	// ErrTooManyMessageFiles 单条消息文件数超限。
	ErrTooManyMessageFiles = errors.New("too many message files")
	// ErrInvalidMessageBranch 消息分支参数无效。
	ErrInvalidMessageBranch = errors.New("invalid message branch")
	// ErrMessageNotFound 消息不存在或无权限。
	ErrMessageNotFound = errors.New("message not found")
	// ErrContextArtifactNotFound 上下文证据不存在或无权限。
	ErrContextArtifactNotFound = errors.New("context artifact not found")
	// ErrInvalidMessageFeedback 消息反馈值不合法。
	ErrInvalidMessageFeedback = errors.New("invalid message feedback")
	// ErrMessageFeedbackTargetInvalid 反馈目标消息不合法。
	ErrMessageFeedbackTargetInvalid = errors.New("invalid message feedback target")
	// ErrModelRouteNotConfigured 模型路由未配置。
	ErrModelRouteNotConfigured = errors.New("model route not configured")
	// ErrUpstreamRequestFailed 上游请求失败。
	ErrUpstreamRequestFailed = errors.New("upstream request failed")
	// ErrUpstreamEmptyResponse 上游返回空响应。
	ErrUpstreamEmptyResponse = errors.New("upstream returned empty response")
	// ErrMessageGenerationCanceled 用户主动停止生成。
	ErrMessageGenerationCanceled = errors.New("message generation canceled")
	// ErrInvalidMediaGenerationTask 媒体生成任务类型或输入不合法。
	ErrInvalidMediaGenerationTask = errors.New("invalid media generation task")
	// ErrDuplicateMessageGenerationRun 表示客户端重复提交同一个生成 run。
	ErrDuplicateMessageGenerationRun = errors.New("duplicate message generation run")
	// ErrMediaImageEditNotImplemented 图片编辑协议尚未实现。
	ErrMediaImageEditNotImplemented = errors.New("image edit protocol not implemented")
)
