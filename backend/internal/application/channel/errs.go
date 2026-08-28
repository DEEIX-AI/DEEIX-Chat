package channel

import (
	"errors"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

var (
	// ErrUpstreamNotFound 上游不存在。
	ErrUpstreamNotFound = repository.ErrUpstreamNotFound
	// ErrModelNotFound 模型不存在。
	ErrModelNotFound = repository.ErrModelNotFound
	// ErrRouteNotFound 路由未配置。
	ErrRouteNotFound = errors.New("route not found")
	// ErrAllRoutesUnavailable 所有候选路由暂时不可用。
	ErrAllRoutesUnavailable = errors.New("all routes unavailable")
	// ErrAllRoutesRateLimited 所有可用候选路由都处于短期限流退避中。
	ErrAllRoutesRateLimited = errors.New("all routes rate limited")
	// ErrCircuitBreakerDisabled 全局模型熔断功能未开启。
	ErrCircuitBreakerDisabled = errors.New("circuit breaker is disabled")
	// ErrDuplicatePlatformModelName 平台模型名重复。
	ErrDuplicatePlatformModelName = repository.ErrDuplicatePlatformModelName
	// ErrInvalidPlatformModelName 平台模型名无效。
	ErrInvalidPlatformModelName = errors.New("invalid platform model name")
	// ErrInvalidJSONConfig JSON 配置无效。
	ErrInvalidJSONConfig = errors.New("invalid json config")
	// ErrInvalidModelCapsConfig 模型上下文窗口或输出 Token 上限无效。
	ErrInvalidModelCapsConfig = errors.New("invalid model capability limits")
	// ErrInvalidHeadersConfig 请求头 JSON 配置无效。
	ErrInvalidHeadersConfig = errors.New("invalid headers config")
	// ErrInvalidAPIKeysConfig 上游 API Key 配置无效。
	ErrInvalidAPIKeysConfig = errors.New("invalid api keys config")
	// ErrInvalidProtocolDefaultsConfig 默认协议配置无效。
	ErrInvalidProtocolDefaultsConfig = errors.New("invalid protocol defaults config")
	// ErrInvalidAdapter 适配器无效。
	ErrInvalidAdapter = errors.New("invalid adapter")
	// ErrInvalidCompatible 上游兼容风格无效。
	ErrInvalidCompatible = errors.New("invalid compatible")
	// ErrInvalidUpstreamBaseURL 上游地址不满足安全边界。
	ErrInvalidUpstreamBaseURL = errors.New("invalid upstream base url")
	// ErrInvalidKinds 模型类型无效。
	ErrInvalidKinds = errors.New("invalid kinds")
	// ErrInvalidModelAccessScope 模型使用范围无效。
	ErrInvalidModelAccessScope = errors.New("invalid model access scope")
	// ErrModelAccessDenied 模型不允许当前调用范围使用。
	ErrModelAccessDenied = errors.New("model access denied")
	// ErrSystemPromptTooLong 系统提示词长度超过允许范围。
	ErrSystemPromptTooLong = errors.New("system prompt too long")
	// ErrInvalidModelOrder 模型排序参数无效。
	ErrInvalidModelOrder = errors.New("invalid model order")
	// ErrModelVendorNotFound 技术厂商不存在。
	ErrModelVendorNotFound = repository.ErrModelVendorNotFound
	// ErrModelVendorConflict 技术厂商 key 重复。
	ErrModelVendorConflict = errors.New("model vendor conflict")
	// ErrInvalidModelVendor 技术厂商参数无效。
	ErrInvalidModelVendor = errors.New("invalid model vendor")
	// ErrBuiltInModelVendorDelete 内置技术厂商不可删除。
	ErrBuiltInModelVendorDelete = errors.New("built-in model vendor cannot be deleted")
	// ErrModelVendorInUse 技术厂商仍被平台模型引用。
	ErrModelVendorInUse = errors.New("model vendor is in use")
	// ErrModelDisplayGroupNotFound 展示分组不存在。
	ErrModelDisplayGroupNotFound = repository.ErrModelDisplayGroupNotFound
	// ErrModelDisplayGroupConflict 展示分组名称重复。
	ErrModelDisplayGroupConflict = errors.New("model display group conflict")
	// ErrInvalidModelDisplayGroup 展示分组参数无效。
	ErrInvalidModelDisplayGroup = errors.New("invalid model display group")
	// ErrModelIconAssetNotFound 自定义模型图标资产不存在。
	ErrModelIconAssetNotFound = errors.New("model icon asset not found")
	// ErrModelIconAssetUnavailable 图标对象存储未配置或对象暂时不可用。
	ErrModelIconAssetUnavailable = errors.New("model icon asset unavailable")
	// ErrModelIconAssetInUse 图标仍被模型、厂商、分组或会话快照引用。
	ErrModelIconAssetInUse = errors.New("model icon asset is in use")
	// ErrModelIconFileTooLarge 图标文件超过允许大小。
	ErrModelIconFileTooLarge = errors.New("model icon file too large")
	// ErrInvalidModelIconFile 图标文件类型、内容或尺寸无效。
	ErrInvalidModelIconFile = errors.New("invalid model icon file")
	// ErrInvalidModelIconReference 图标引用格式无效或不允许直接保存内联数据。
	ErrInvalidModelIconReference = errors.New("invalid model icon reference")
	// ErrInvalidPermissionGroupModels 模型权限组参数无效。
	ErrInvalidPermissionGroupModels = errors.New("invalid permission group models")
	// ErrPermissionGroupRepoUnavailable 权限组仓储未注入。
	ErrPermissionGroupRepoUnavailable = errors.New("permission group repo unavailable")
	// ErrProtocolRequired 无法通过瀑布规则推断协议。
	ErrProtocolRequired = errors.New("protocol required")
	// ErrInvalidRouteProtocolCombination 路由协议组合无效。
	ErrInvalidRouteProtocolCombination = errors.New("invalid route protocol combination")
	// ErrUpstreamModelNotFound 上游模型路由绑定不存在。
	ErrUpstreamModelNotFound = repository.ErrUpstreamModelNotFound
	// ErrUpstreamModelConflict 上游模型路由绑定冲突。
	ErrUpstreamModelConflict = repository.ErrUpstreamModelConflict
	// ErrUpstreamModelBindingChanged 上游模型绑定已被其他操作修改。
	ErrUpstreamModelBindingChanged = errors.New("upstream model binding changed")
	// ErrUpstreamSourceUnavailable 上游或上游模型当前不可用。
	ErrUpstreamSourceUnavailable = errors.New("upstream source unavailable")
	// ErrRemoteModelsUnavailable 上游远程模型目录不可用。
	ErrRemoteModelsUnavailable = errors.New("remote models unavailable")
	// ErrNoActiveKey 无可用密钥。
	ErrNoActiveKey = errors.New("no active api key")
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
