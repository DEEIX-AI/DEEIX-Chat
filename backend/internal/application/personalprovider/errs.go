package personalprovider

import "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/apperr"

// 错误码与文案是前端依赖的 API 契约，改动需同步 apps/web/i18n/messages/*/errors.json。
var (
	// ErrFeatureDisabled 表示管理员未开启自带 Key，或当前用户不在允许的权限组内。
	ErrFeatureDisabled = apperr.New("personal_provider.disabled", "personal model providers are not enabled for this account")
	// ErrNotFound 表示服务不存在或不属于当前用户。
	ErrNotFound = apperr.New("personal_provider.not_found", "model provider not found")
	// ErrLimitReached 表示已达到每个用户的服务数量上限。
	ErrLimitReached = apperr.New("personal_provider.limit_reached", "model provider limit reached")
	// ErrDuplicate 表示同一地址的同一个 Key 已经添加过。
	ErrDuplicate = apperr.New("personal_provider.duplicate", "this API key is already added for this address")
	// ErrInvalidIcon 表示图标不是内置图标 slug。
	ErrInvalidIcon = apperr.New("personal_provider.invalid_icon", "invalid model provider icon")
	// ErrInvalidName 表示名称为空或过长。
	ErrInvalidName = apperr.New("personal_provider.invalid_name", "invalid model provider name")
	// ErrInvalidProtocol 表示协议不在允许范围内。
	ErrInvalidProtocol = apperr.New("personal_provider.invalid_protocol", "unsupported model provider protocol")
	// ErrInvalidBaseURL 表示地址格式不合法（非 http(s)、含账号密码、查询参数等）。
	ErrInvalidBaseURL = apperr.New("personal_provider.invalid_base_url", "invalid model provider address")
	// ErrBlockedHost 表示地址指向内网、回环、元数据地址，或在管理员配置的域名黑名单中。
	ErrBlockedHost = apperr.New("personal_provider.blocked_host", "this address is not allowed")
	// ErrInvalidAPIKey 表示 Key 为空、过长或含非法字符。
	ErrInvalidAPIKey = apperr.New("personal_provider.invalid_api_key", "invalid API key")
	// ErrInvalidModels 表示模型选择不合法。
	ErrInvalidModels = apperr.New("personal_provider.invalid_models", "invalid model selection")
	// ErrInvalidModelProtocol 表示模型的协议不受支持，或不是平台允许的配套协议组合。
	ErrInvalidModelProtocol = apperr.New("personal_provider.invalid_model_protocol", "unsupported protocol for this model")
	// ErrSuspended 表示服务已被管理员停用，用户不能自行启用。
	ErrSuspended = apperr.New("personal_provider.suspended", "this model provider was disabled by an administrator")
	// ErrUpstreamRejected 表示上游拒绝了请求（通常是 Key 无效或无权限）。
	ErrUpstreamRejected = apperr.New("personal_provider.upstream_rejected", "the provider rejected the request; check the API key")
	// ErrUpstreamUnavailable 表示无法连接上游或上游返回异常。
	ErrUpstreamUnavailable = apperr.New("personal_provider.upstream_unavailable", "could not reach the model provider")
	// ErrModelUnavailable 表示所选个人模型不可用（服务停用、模型未启用或功能已关闭）。
	ErrModelUnavailable = apperr.New("personal_provider.model_unavailable", "this personal model is not available")
)
