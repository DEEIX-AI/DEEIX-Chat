package channel

import (
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/apperr"
)

var (
	// ErrUpstreamNotFound 上游不存在。
	ErrUpstreamNotFound = repository.ErrUpstreamNotFound
	// ErrModelNotFound 模型不存在。
	ErrModelNotFound = repository.ErrModelNotFound
	// ErrRouteNotFound 路由未配置。
	ErrRouteNotFound = apperr.New("route.not_found", "route not found")
	// ErrAllRoutesUnavailable 所有候选路由暂时不可用。
	ErrAllRoutesUnavailable = apperr.New("llm.routes_unavailable", "all model routes are unavailable")
	// ErrAllRoutesRateLimited 所有可用候选路由都处于短期限流退避中。
	ErrAllRoutesRateLimited = apperr.NewMasked("upstream.rate_limited", "upstream rate limited", "all routes rate limited")
	// ErrCircuitBreakerDisabled 全局模型熔断功能未开启。
	ErrCircuitBreakerDisabled = apperr.New("llm.circuit_breaker_disabled", "circuit breaker is disabled")
	// ErrDuplicatePlatformModelName 平台模型名重复。
	ErrDuplicatePlatformModelName = repository.ErrDuplicatePlatformModelName
	// ErrInvalidPlatformModelName 平台模型名无效。
	ErrInvalidPlatformModelName = apperr.New("llm.invalid_platform_model_name", "invalid platform model name")
	// ErrInvalidJSONConfig JSON 配置无效。
	ErrInvalidJSONConfig = apperr.New("config.invalid_json", "invalid json config")
	// ErrInvalidModelCapsConfig 模型上下文窗口或输出 Token 上限无效。
	ErrInvalidModelCapsConfig = apperr.New("llm.invalid_model_capabilities", "invalid model capability limits")
	// ErrInvalidHeadersConfig 请求头 JSON 配置无效。
	ErrInvalidHeadersConfig = apperr.NewMasked("llm.invalid_headers_config", "invalid headers json config", "invalid headers config")
	// ErrInvalidAPIKeysConfig 上游 API Key 配置无效。
	ErrInvalidAPIKeysConfig = apperr.New("llm.invalid_api_keys_config", "invalid api keys config")
	// ErrInvalidProtocolDefaultsConfig 默认协议配置无效。
	ErrInvalidProtocolDefaultsConfig = apperr.New("llm.invalid_protocol_defaults_config", "invalid protocol defaults config")
	// ErrInvalidAdapter 适配器无效。
	ErrInvalidAdapter = apperr.New("llm.invalid_adapter", "invalid adapter")
	// ErrInvalidCompatible 上游兼容风格无效。
	ErrInvalidCompatible = apperr.New("llm.invalid_compatible", "invalid compatible")
	// ErrInvalidUpstreamBaseURL 上游地址不满足安全边界。
	ErrInvalidUpstreamBaseURL = apperr.New("request.invalid_upstream_base_url", "invalid upstream base url")
	// ErrInvalidKinds 模型类型无效。
	ErrInvalidKinds = apperr.NewMasked("llm.invalid_kinds", "invalid model kinds", "invalid kinds")
	// ErrInvalidModelAccessScope 模型使用范围无效。
	ErrInvalidModelAccessScope = apperr.New("request.invalid_model_access_scope", "invalid model access scope")
	// ErrModelAccessDenied 模型不允许当前调用范围使用。
	ErrModelAccessDenied = apperr.NewMasked("llm.model_access_denied", "you do not have access to this model", "model access denied")
	// ErrSystemPromptTooLong 系统提示词长度超过允许范围。
	ErrSystemPromptTooLong = apperr.New("llm.system_prompt_too_long", "system prompt too long")
	// ErrInvalidModelOrder 模型排序参数无效。
	ErrInvalidModelOrder = apperr.New("request.invalid_model_order", "invalid model order")
	// ErrModelVendorNotFound 技术厂商不存在。
	ErrModelVendorNotFound = repository.ErrModelVendorNotFound
	// ErrModelVendorConflict 技术厂商 key 重复。
	ErrModelVendorConflict = apperr.NewMasked("model_vendor.already_exists", "model vendor already exists", "model vendor conflict")
	// ErrInvalidModelVendor 技术厂商参数无效。
	ErrInvalidModelVendor = apperr.New("request.invalid_model_vendor", "invalid model vendor")
	// ErrBuiltInModelVendorDelete 内置技术厂商不可删除。
	ErrBuiltInModelVendorDelete = apperr.New("llm.model_vendor_builtin", "built-in model vendor cannot be deleted")
	// ErrModelVendorInUse 技术厂商仍被平台模型引用。
	ErrModelVendorInUse = apperr.New("llm.model_vendor_in_use", "model vendor is in use")
	// ErrModelDisplayGroupNotFound 展示分组不存在。
	ErrModelDisplayGroupNotFound = repository.ErrModelDisplayGroupNotFound
	// ErrModelDisplayGroupConflict 展示分组名称重复。
	ErrModelDisplayGroupConflict = apperr.NewMasked("model_display_group.already_exists", "model display group already exists", "model display group conflict")
	// ErrInvalidModelDisplayGroup 展示分组参数无效。
	ErrInvalidModelDisplayGroup = apperr.New("request.invalid_model_display_group", "invalid model display group")
	// ErrModelIconAssetNotFound 自定义模型图标资产不存在。
	ErrModelIconAssetNotFound = apperr.New("llm.model_icon_asset_not_found", "model icon asset not found")
	// ErrModelIconAssetUnavailable 图标对象存储未配置或对象暂时不可用。
	ErrModelIconAssetUnavailable = apperr.New("llm.model_icon_asset_unavailable", "model icon asset unavailable")
	// ErrModelIconAssetInUse 图标仍被模型、厂商、分组或会话快照引用。
	ErrModelIconAssetInUse = apperr.New("llm.model_icon_asset_in_use", "model icon asset is in use")
	// ErrModelIconFileTooLarge 图标文件超过允许大小。
	ErrModelIconFileTooLarge = apperr.New("llm.model_icon_file_too_large", "model icon file too large")
	// ErrInvalidModelIconFile 图标文件类型、内容或尺寸无效。
	ErrInvalidModelIconFile = apperr.New("llm.model_icon_file_invalid", "invalid model icon file")
	// ErrInvalidModelIconReference 图标引用格式无效或不允许直接保存内联数据。
	ErrInvalidModelIconReference = apperr.NewMasked("llm.model_icon_invalid", "invalid model icon", "invalid model icon reference")
	// ErrInvalidPermissionGroupModels 模型权限组参数无效。
	ErrInvalidPermissionGroupModels = apperr.New("admin.invalid_permission_group_models", "invalid permission group models")
	// ErrPermissionGroupRepoUnavailable 权限组仓储未注入。
	ErrPermissionGroupRepoUnavailable = apperr.New("admin.permission_group_repo_unavailable", "permission group repo unavailable")
	// ErrProtocolRequired 无法通过瀑布规则推断协议。
	ErrProtocolRequired = apperr.NewMasked("llm.protocol_required", "protocol is required", "protocol required")
	// ErrInvalidRouteProtocolCombination 路由协议组合无效。
	ErrInvalidRouteProtocolCombination = apperr.New("llm.invalid_route_protocol_combination", "invalid route protocol combination")
	// ErrUpstreamModelNotFound 上游模型路由绑定不存在。
	ErrUpstreamModelNotFound = repository.ErrUpstreamModelNotFound
	// ErrUpstreamModelConflict 上游模型路由绑定冲突。
	ErrUpstreamModelConflict = repository.ErrUpstreamModelConflict
	// ErrUpstreamModelBindingChanged 上游模型绑定已被其他操作修改。
	ErrUpstreamModelBindingChanged = apperr.New("llm.upstream_model_binding_changed", "upstream model binding changed")
	// ErrUpstreamSourceUnavailable 上游或上游模型当前不可用。
	ErrUpstreamSourceUnavailable = apperr.New("llm.upstream_source_unavailable", "upstream source unavailable")
	// ErrRemoteModelsUnavailable 上游远程模型目录不可用。
	ErrRemoteModelsUnavailable = apperr.New("llm.remote_models_unavailable", "remote models unavailable")
	// ErrEmptyRemoteModels 上游返回空模型目录，必须显式确认后才允许对账。
	ErrEmptyRemoteModels = apperr.NewMasked("llm.remote_models_empty_confirmation_required", "remote models snapshot is empty", "remote models snapshot is empty")
	// ErrRemoteModelsSnapshotChanged 表示确认后远端目录已变化，必须重新预览。
	ErrRemoteModelsSnapshotChanged = apperr.New("llm.remote_models_snapshot_changed", "remote models snapshot changed")
	// ErrNoActiveKey 无可用密钥。
	ErrNoActiveKey = apperr.New("llm.no_active_api_key", "no active api key")
	// ErrLLMSettingNotFound LLM 全局设置不存在。
	ErrLLMSettingNotFound = repository.ErrLLMSettingNotFound
)

// RoutesRateLimitedError 携带全部候选路由恢复前的最短等待时间。
type RoutesRateLimitedError struct {
	RetryAfter time.Duration
}

func (e *RoutesRateLimitedError) Error() string {
	return ErrAllRoutesRateLimited.Error()
}

func (e *RoutesRateLimitedError) Unwrap() error {
	return ErrAllRoutesRateLimited
}
