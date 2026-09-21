package uicomponent

import "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/apperr"

// 由 transport 层直接判定的请求错误。错误码与文案是前端依赖的 API 契约，改动需同步 frontend/i18n/messages/*/errors.json。
var (
	errInvalidComponentID = apperr.New("ui_component.invalid_id", "invalid ui component id")
)
