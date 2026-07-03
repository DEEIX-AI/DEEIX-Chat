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
	// ErrConversationProjectNotFound 会话项目不存在或无权限。
	ErrConversationProjectNotFound = errors.New("conversation project not found")
	// ErrInvalidConversationProject 会话项目请求不合法。
	ErrInvalidConversationProject = errors.New("invalid conversation project")
	// ErrInvalidFileReference 文件引用无效。
	ErrInvalidFileReference = errors.New("invalid file reference")
	// ErrInvalidFileName 文件名不合法。
	ErrInvalidFileName = errors.New("invalid file name")
	// ErrFileNotFound 文件不存在。
	ErrFileNotFound = errors.New("file not found")
	// ErrFileInUse 文件正在被头像等资源使用。
	ErrFileInUse = errors.New("file in use")
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
	// ErrTooManySelectedTools 单条消息选择的 MCP 工具数超限。
	ErrTooManySelectedTools = errors.New("too many selected tools")
	// ErrTooManySelectedSkills 单条消息选择的 Skill 数超限。
	ErrTooManySelectedSkills = errors.New("too many selected skills")
	// ErrSkillNotFound 技能不存在或当前用户不可用。
	ErrSkillNotFound = errors.New("skill not found")
	// ErrInvalidSkillUse 技能使用入参不合法。
	ErrInvalidSkillUse = errors.New("invalid skill use")
	// ErrInvalidMessageBranch 消息分支参数无效。
	ErrInvalidMessageBranch = errors.New("invalid message branch")
	// ErrInvalidMessageContent 消息内容不合法。
	ErrInvalidMessageContent = errors.New("invalid message content")
	// ErrMessageNotFound 消息不存在或无权限。
	ErrMessageNotFound = errors.New("message not found")
	// ErrContextArtifactNotFound 上下文证据不存在或无权限。
	ErrContextArtifactNotFound = errors.New("context artifact not found")
	// ErrInvalidMessageFeedback 消息反馈值不合法。
	ErrInvalidMessageFeedback = errors.New("invalid message feedback")
	// ErrMessageFeedbackTargetInvalid 反馈目标消息不合法。
	ErrMessageFeedbackTargetInvalid = errors.New("invalid message feedback target")
	// ErrMessageEditTargetInvalid 编辑目标消息不合法。
	ErrMessageEditTargetInvalid = errors.New("invalid message edit target")
	// ErrMessageEditStateInvalid 当前消息状态不允许编辑。
	ErrMessageEditStateInvalid = errors.New("invalid message edit state")
	// ErrModelRouteNotConfigured 模型路由未配置。
	ErrModelRouteNotConfigured = errors.New("model route not configured")
	// ErrModelAccessDenied 当前用户无权使用此模型。
	ErrModelAccessDenied = errors.New("model access denied by group policy")
	// ErrUpstreamRequestFailed 上游请求失败。
	ErrUpstreamRequestFailed = errors.New("upstream request failed")
	// ErrUpstreamEmptyResponse 上游返回空响应。
	ErrUpstreamEmptyResponse = errors.New("upstream returned empty response")
	// ErrToolRunFinalAnswerMissing 工具循环结束后上游仍未产出最终回答。
	ErrToolRunFinalAnswerMissing = errors.New("tool run ended without a final answer")
	// ErrMessageGenerationCanceled 用户主动停止生成。
	ErrMessageGenerationCanceled = errors.New("message generation canceled")
	// ErrInvalidMediaGenerationTask 媒体生成任务类型或输入不合法。
	ErrInvalidMediaGenerationTask = errors.New("invalid media generation task")
	// ErrMediaImagePromptRequired 图片任务提示词不能为空。
	ErrMediaImagePromptRequired = errors.New("image prompt is required")
	// ErrMediaImageGenerationRejectsInputs 图片生成任务不能携带输入图。
	ErrMediaImageGenerationRejectsInputs = errors.New("image generation does not accept input images")
	// ErrMediaImageEditInputRequired 图片编辑任务必须携带至少一张输入图。
	ErrMediaImageEditInputRequired = errors.New("image edit requires at least one input image")
	// ErrMediaImageEditTooManyInputs 图片编辑输入图数量超限。
	ErrMediaImageEditTooManyInputs = errors.New("too many image edit input images")
	// ErrMediaImageEditInputInvalid 图片编辑输入图不合法。
	ErrMediaImageEditInputInvalid = errors.New("image edit input image is invalid")
	// ErrMediaRouteProtocolMismatch 图片任务命中的路由协议与任务类型不匹配。
	ErrMediaRouteProtocolMismatch = errors.New("media route protocol does not match task")
	// ErrDuplicateMessageGenerationRun 表示客户端重复提交同一个生成 run。
	ErrDuplicateMessageGenerationRun = errors.New("duplicate message generation run")
)
