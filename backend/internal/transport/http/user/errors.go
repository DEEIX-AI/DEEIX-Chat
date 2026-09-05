package user

import "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/apperr"

// 由 transport 层直接判定的请求错误（路径参数、必填校验、运行时依赖缺失等）。
// 错误码与文案是前端依赖的 API 契约，改动需同步 frontend/i18n/messages/*/errors.json。
var (
	errAvatarNotFound           = apperr.New("avatar.not_found", "avatar not found")
	errInvalidDailyActivityDays = apperr.New("request.invalid_daily_activity_days", "invalid daily activity days")
	errInvalidUserID            = apperr.New("user.invalid_id", "invalid user id")
)
