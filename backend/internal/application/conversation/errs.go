package conversation

import (
	"errors"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/apperr"
)

// 错误码与对外文案是前端本地化依赖的 API 契约（frontend/i18n/messages/*/errors.json 按错误码取文案），
// 因此用 apperr 随哨兵一起声明，传输层用 response.ErrorFrom / response.Describe 直接读取。
// apperr.NewMasked 的第三个参数保留原有内部文本，供日志与错误链使用。
// 仓储层哨兵暂时保留原始身份；transport 必须在边界显式映射为稳定的 apperr 契约。
var (
	// ErrConversationNotFound 会话不存在或无权限。
	ErrConversationNotFound = apperr.New("conversation.not_found", "conversation not found")
	// ErrConversationEventNotFound 对话事件日志不存在。
	ErrConversationEventNotFound = apperr.New("conversation_event.not_found", "conversation event not found")
	// ErrToolCallNotFound 工具调用不存在或当前用户无权访问。
	ErrToolCallNotFound = apperr.New("tool_call.not_found", "tool call not found")
	// ErrConversationShareNotFound 会话分享不存在、已关闭或原会话已删除。
	ErrConversationShareNotFound = apperr.New("conversation_share.not_found", "conversation share not found")
	// ErrInvalidConversationShare 会话分享请求不合法。
	ErrInvalidConversationShare = apperr.New("conversation_share.invalid", "invalid conversation share")
	// ErrConversationShareSchemaOutdated 会话分享表结构未更新。
	ErrConversationShareSchemaOutdated = apperr.NewMasked("conversation_share.schema_outdated", "conversation share schema is outdated", "conversation share schema outdated")
	// ErrInvalidConversationTitle 会话标题不合法。
	ErrInvalidConversationTitle = apperr.New("conversation.invalid_title", "invalid conversation title")
	// ErrInvalidConversationLabels 会话标签不合法。
	ErrInvalidConversationLabels = apperr.New("request.invalid_conversation_labels", "invalid conversation labels")
	// ErrConversationProjectNotFound 会话项目不存在或无权限。
	ErrConversationProjectNotFound = apperr.New("conversation_project.not_found", "conversation project not found")
	// ErrInvalidConversationProject 会话项目请求不合法。
	ErrInvalidConversationProject = apperr.New("request.invalid_conversation_project", "invalid conversation project")
	// ErrConversationProjectLimitExceeded 单用户会话项目数量超限。
	ErrConversationProjectLimitExceeded = repository.ErrConversationProjectLimitExceeded
	// ErrInvalidFileReference 文件引用无效。
	ErrInvalidFileReference = apperr.New("file.invalid_reference", "invalid file reference")
	// ErrInvalidFileName 文件名不合法。
	ErrInvalidFileName = apperr.New("file.invalid_name", "invalid file name")
	// ErrFileNotFound 文件不存在。
	ErrFileNotFound = apperr.New("file.not_found", "file not found")
	// ErrFileInUse 文件正在被头像、知识库等资源使用。
	ErrFileInUse = apperr.NewMasked("file.in_use", "file is in use", "file in use")
	// ErrStorageQuotaExceeded 文件配额超限。
	// 仓储层仍返回 repository.ErrStorageQuotaExceeded，由应用边界转换为此带契约的哨兵。
	ErrStorageQuotaExceeded = apperr.NewMasked(MessageErrorCodeQuotaExceeded, "quota exceeded", "storage quota exceeded")
	// ErrFileTooLarge 文件过大。
	ErrFileTooLarge = apperr.New("file.too_large", "file too large")
	// ErrMIMEBlocked 文件类型不被允许。
	ErrMIMEBlocked = apperr.NewMasked("file.type_blocked", "file type is not allowed", "mime blocked")
	// ErrDangerousMIMEType 危险文件类型不被允许。
	ErrDangerousMIMEType = apperr.NewMasked("file.type_blocked", "file type is not allowed", "dangerous file type not allowed")
	// ErrFileProcessingNotReady 文件处理尚未就绪。
	ErrFileProcessingNotReady = apperr.NewMasked("file.not_ready", "file processing is not ready", "file processing not ready")
	// ErrFileTooLargeForFullContext 文件过大，无法全文注入。
	ErrFileTooLargeForFullContext = apperr.NewMasked("file.too_large_for_context", "file is too large for full context", "file too large for full context")
	// ErrEmbeddingUnavailable 当前未配置可用 embedding，无法处理大文档 / RAG。
	ErrEmbeddingUnavailable = apperr.NewMasked("file.embedding_unavailable", "embedding is unavailable for current file capability", "embedding unavailable")
	// ErrInvalidKnowledgeBaseReference 知识库不存在、已停用或当前用户不可见。
	ErrInvalidKnowledgeBaseReference = apperr.New(MessageErrorCodeKnowledgeBaseInvalidReference, "invalid knowledge base reference")
	// ErrKnowledgeBaseUnavailable 当前未启用可用的知识库检索能力。
	ErrKnowledgeBaseUnavailable = apperr.NewMasked(MessageErrorCodeKnowledgeBaseUnavailable, "knowledge base retrieval is unavailable", "knowledge base retrieval unavailable")
	// ErrKnowledgeBaseNotReady 所选知识库尚无可检索文件。
	ErrKnowledgeBaseNotReady = apperr.NewMasked(MessageErrorCodeKnowledgeBaseNotReady, "selected knowledge base has no ready files", "knowledge base not ready")
	// ErrTooManyMessageFiles 单条消息文件数超限。
	ErrTooManyMessageFiles = apperr.NewMasked("message.too_many_files", "too many files in one message", "too many message files")
	// ErrTooManySelectedTools 单条消息选择的 MCP 工具数超限。
	ErrTooManySelectedTools = apperr.New("message.too_many_selected_tools", "too many selected tools")
	// ErrMultipleImageAttachmentProcessors 单条消息不能同时选择多个图片附件处理器。
	ErrMultipleImageAttachmentProcessors = apperr.NewMasked("message.multiple_image_processors", "select only one image attachment processor", "multiple image attachment processors selected")
	// ErrImageAttachmentProcessingFailed 图片附件处理器调用失败。
	ErrImageAttachmentProcessingFailed = apperr.NewMasked("mcp.image_processing_failed", "image processing tool failed", "image attachment processing failed")
	// ErrTooManySelectedSkills 单条消息选择的 Skill 数超限。
	ErrTooManySelectedSkills = apperr.New("message.too_many_selected_skills", "too many selected skills")
	// ErrSkillNotFound 技能不存在或当前用户不可用。
	ErrSkillNotFound = apperr.New("skill.not_found", "skill not found")
	// ErrInvalidSkillUse 技能使用入参不合法。
	ErrInvalidSkillUse = apperr.New("request.invalid_skill_use", "invalid skill use")
	// ErrInvalidMessageBranch 消息分支参数无效。
	ErrInvalidMessageBranch = apperr.New("message.invalid_branch", "invalid message branch")
	// ErrInvalidMessageContent 消息内容不合法。
	ErrInvalidMessageContent = apperr.New("message.invalid_content", "invalid message content")
	// ErrMessageNotFound 消息不存在或无权限。
	ErrMessageNotFound = apperr.New("message.not_found", "message not found")
	// ErrContextArtifactNotFound 上下文证据不存在或无权限。
	ErrContextArtifactNotFound = apperr.New("context_artifact.not_found", "context artifact not found")
	// ErrInvalidMessageFeedback 消息反馈值不合法。
	ErrInvalidMessageFeedback = apperr.New("message.invalid_feedback", "invalid message feedback")
	// ErrMessageFeedbackTargetInvalid 反馈目标消息不合法。
	ErrMessageFeedbackTargetInvalid = apperr.NewMasked("message.feedback_target_invalid", "message feedback target invalid", "invalid message feedback target")
	// ErrMessageEditTargetInvalid 编辑目标消息不合法。
	ErrMessageEditTargetInvalid = apperr.NewMasked("message.edit_target_invalid", "message edit target invalid", "invalid message edit target")
	// ErrMessageEditStateInvalid 当前消息状态不允许编辑。
	ErrMessageEditStateInvalid = apperr.NewMasked("message.edit_state_invalid", "message edit state invalid", "invalid message edit state")
	// ErrMessageForkStateInvalid 当前消息状态不允许 fork。
	ErrMessageForkStateInvalid = apperr.New("conversation.message_fork_state_invalid", "message is still generating")
	// ErrMessageForkTargetInvalid 当前消息角色不允许 fork。
	ErrMessageForkTargetInvalid = apperr.New("conversation.message_fork_target_invalid", "only assistant messages can be forked")
	// ErrMessageForkHistoryIncomplete 消息祖先链超过安全上限或已损坏，无法完整 fork。
	ErrMessageForkHistoryIncomplete = apperr.New("conversation.message_fork_history_incomplete", "message history is too deep or incomplete")
	// ErrModelRouteNotConfigured 模型路由未配置。
	ErrModelRouteNotConfigured = apperr.NewMasked("llm.model_route_not_configured", "model route is not configured", "model route not configured")
	// ErrModelAccessDenied 当前用户无权使用此模型。
	ErrModelAccessDenied = apperr.NewMasked("llm.model_access_denied", "you do not have access to this model", "model access denied by group policy")
	// ErrUpstreamRequestFailed 上游请求失败。对外错误码与文案取决于上游错误特征，由 transport 按 MessageErrorCode /
	// messageErrorSummary 逐次判定，因此不在这里声明。
	ErrUpstreamRequestFailed = errors.New("upstream request failed")
	// ErrGeneratedMediaArtifactUnavailable 上游已完成媒体生成，但结果制品暂时无法获取或校验。
	ErrGeneratedMediaArtifactUnavailable = apperr.New(MessageErrorCodeMediaArtifactUnavailable, "generated media artifact is temporarily unavailable")
	// ErrUpstreamEmptyResponse 上游返回空响应。
	ErrUpstreamEmptyResponse = apperr.NewMasked(MessageErrorCodeUpstreamEmptyResponse, "model returned empty response", "upstream returned empty response")
	// ErrToolRunFinalAnswerMissing 工具循环结束后上游仍未产出最终回答。
	ErrToolRunFinalAnswerMissing = apperr.New(MessageErrorCodeToolRunFinalAnswerMissing, "tool run ended without a final answer")
	// ErrMessageGenerationCanceled 用户主动停止生成。
	ErrMessageGenerationCanceled = apperr.New("conversation_run.canceled", "message generation canceled")
	// ErrMessageGenerationInterrupted 活跃生成租约消失，当前生成流无法继续恢复。
	ErrMessageGenerationInterrupted = apperr.New("conversation_run.stream_interrupted", "generation stream was interrupted; retry this message")
	// ErrInvalidMediaGenerationTask 媒体生成任务类型或输入不合法。
	ErrInvalidMediaGenerationTask = apperr.New("media.invalid_task", "invalid media generation task")
	// ErrMediaImagePromptRequired 图片任务提示词不能为空。
	ErrMediaImagePromptRequired = apperr.New("media.image_prompt_required", "image prompt is required")
	// ErrMediaImageGenerationRejectsInputs 图片生成任务不能携带输入图。
	ErrMediaImageGenerationRejectsInputs = apperr.New("media.image_generation_rejects_inputs", "image generation does not accept input images")
	// ErrMediaImageEditInputRequired 图片编辑任务必须携带至少一张输入图。
	ErrMediaImageEditInputRequired = apperr.New("media.image_edit_input_required", "image edit requires at least one input image")
	// ErrMediaImageEditTooManyInputs 图片编辑输入图数量超限。
	ErrMediaImageEditTooManyInputs = apperr.New("media.image_edit_too_many_inputs", "too many image edit input images")
	// ErrMediaImageEditInputInvalid 图片编辑输入图不合法。
	ErrMediaImageEditInputInvalid = apperr.New("media.image_edit_input_invalid", "image edit input image is invalid")
	// ErrMediaVideoPromptRequired 视频任务提示词不能为空。
	ErrMediaVideoPromptRequired = apperr.New("media.video_prompt_required", "video prompt is required")
	// ErrMediaVideoInputInvalid 视频生成输入不合法。
	ErrMediaVideoInputInvalid = apperr.New("media.video_input_invalid", "video generation input is invalid")
	// ErrMediaVideoTooManyInputs 视频生成输入图数量超限。
	ErrMediaVideoTooManyInputs = apperr.New("media.video_too_many_inputs", "too many video generation input images")
	// ErrMediaRouteProtocolMismatch 图片任务命中的路由协议与任务类型不匹配。
	ErrMediaRouteProtocolMismatch = apperr.New("media.route_protocol_mismatch", "media route protocol does not match task")
	// ErrDuplicateMessageGenerationRun 表示客户端重复提交同一个生成 run。
	ErrDuplicateMessageGenerationRun = apperr.NewMasked("message_generation_run.already_exists", "message generation run already exists", "duplicate message generation run")
)
