package conversation

import "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/apperr"

// 由 transport 层直接判定的请求错误（路径参数、必填校验、运行时依赖缺失等）。
// 错误码与文案是前端依赖的 API 契约，改动需同步 frontend/i18n/messages/*/errors.json。
var (
	errAtLeastOneOfFileNameOrRAGOptOutRequired = apperr.New("request.required", "at least one of file_name or rag_opt_out is required")
	errConversationNoTitleableContent          = apperr.New("conversation.no_titleable_content", "conversation has no titleable content")
	errConversationProjectLimitExceeded        = apperr.New("conversation.project_limit_exceeded", "conversation project limit exceeded")
	errFileExtractNotReady                     = apperr.New("file.extract_not_ready", "file extract is not ready")
	errFileEmbeddingUnavailable                = apperr.New("file.embedding_unavailable", "embedding is unavailable for this file size")
	errFileRequired                            = apperr.New("file.required", "file is required")
	errGenerationStreamNotFound                = apperr.New("conversation_run.stream_not_found", "generation stream not found")
	errInvalidBeforeMessageID                  = apperr.New("before_message.invalid_id", "invalid before message id")
	errInvalidContextArtifactID                = apperr.New("context_artifact.invalid_id", "invalid context artifact id")
	errInvalidConversationID                   = apperr.New("conversation.invalid_id", "invalid conversation id")
	errInvalidConversationProjectID            = apperr.New("conversation_project.invalid_id", "invalid conversation project id")
	errInvalidFileID                           = apperr.New("file.invalid_id", "invalid file id")
	errInvalidFile                             = apperr.New("request.invalid_file", "invalid file")
	errInvalidFileReference                    = apperr.New("file.invalid_reference", "invalid file reference")
	errInvalidFileStream                       = apperr.New("file.invalid_stream", "invalid file stream")
	errInvalidMessageID                        = apperr.New("message.invalid_id", "invalid message id")
	errInvalidRunID                            = apperr.New("run.invalid_id", "invalid run id")
	errInvalidRunIDs                           = apperr.New("request.invalid_run_ids", "invalid run ids")
	errInvalidShareID                          = apperr.New("share.invalid_id", "invalid share id")
	errInvalidTemporaryChatMessages            = apperr.New("request.invalid_temporary_chat_messages", "invalid temporary chat messages")
	errInvalidToolCallID                       = apperr.New("tool_call.invalid_id", "invalid tool call id")
	errLabelsRequired                          = apperr.New("request.required", "labels are required")
	errSharedFileNotFound                      = apperr.New("shared_file.not_found", "shared file not found")
	errStorageQuotaExceeded                    = apperr.New("quota.exceeded", "storage quota exceeded")
	errTemporaryChatContextTooLarge            = apperr.New("temporary_chat.context_too_large", "temporary chat context is too large")
	errTooManyFilesInOneMessage                = apperr.New("message.too_many_files", "too many files in one message")
)
