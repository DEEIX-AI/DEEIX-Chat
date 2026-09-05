package admin

import "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/apperr"

// 由 transport 层直接判定的请求错误（路径参数、必填校验、运行时依赖缺失等）。
// 错误码与文案是前端依赖的 API 契约，改动需同步 frontend/i18n/messages/*/errors.json。
var (
	errInvalidConversationEventID = apperr.New("conversation_event.invalid_id", "invalid conversation event id")
	errInvalidModelID             = apperr.New("model.invalid_id", "invalid model id")
	errInvalidPermissionGroupID   = apperr.New("permission_group.invalid_id", "invalid permission group id")
	errInvalidUserID              = apperr.New("user.invalid_id", "invalid user id")
)
