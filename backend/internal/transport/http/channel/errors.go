package channel

import "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/apperr"

// 由 transport 层直接判定的请求错误（路径参数、必填校验、运行时依赖缺失等）。
// 错误码与文案是前端依赖的 API 契约，改动需同步 frontend/i18n/messages/*/errors.json。
var (
	errInvalidModelDisplayGroupID            = apperr.New("model_display_group.invalid_id", "invalid model display group id")
	errInvalidModelID                        = apperr.New("model.invalid_id", "invalid model id")
	errInvalidModelIconFile                  = apperr.New("llm.model_icon_file_invalid", "invalid model icon file")
	errInvalidRouteID                        = apperr.New("route.invalid_id", "invalid route id")
	errInvalidSettingKey                     = apperr.New("settings.invalid_key", "invalid setting key")
	errInvalidUpstreamID                     = apperr.New("upstream.invalid_id", "invalid upstream id")
	errModelDisplayGroupNotFound             = apperr.New("model_display_group.not_found", "model display group not found")
	errModelIconFileTooLarge                 = apperr.New("llm.model_icon_file_too_large", "model icon file too large")
	errModelNotFound                         = apperr.New("model.not_found", "model not found")
	errModelUpstreamSourcesNotFound          = apperr.New("model_upstream_sources.not_found", "model upstream sources not found")
	errModelVendorNotFound                   = apperr.New("model_vendor.not_found", "model vendor not found")
	errPlatformModelNameAlreadyExists        = apperr.New("llm.platform_model_name_exists", "platform model name already exists")
	errSettingNotFound                       = apperr.New("settings.not_found", "setting not found")
	errTargetModelAlreadyBoundOnThisUpstream = apperr.New("llm.route_conflict", "target model is already bound on this upstream")
	errUpstreamModelNotFound                 = apperr.New("upstream_model.not_found", "upstream model not found")
	errUpstreamNotFound                      = apperr.New("upstream.not_found", "upstream not found")
)
