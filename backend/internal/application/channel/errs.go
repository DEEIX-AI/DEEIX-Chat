package channel

import (
	"errors"

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
	// ErrDuplicatePlatformModelName 平台模型名重复。
	ErrDuplicatePlatformModelName = repository.ErrDuplicatePlatformModelName
	// ErrInvalidPlatformModelName 平台模型名无效。
	ErrInvalidPlatformModelName = errors.New("invalid platform model name")
	// ErrInvalidJSONConfig JSON 配置无效。
	ErrInvalidJSONConfig = errors.New("invalid json config")
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
	// ErrProtocolRequired 无法通过瀑布规则推断协议。
	ErrProtocolRequired = errors.New("protocol required")
	// ErrInvalidRouteProtocolCombination 路由协议组合无效。
	ErrInvalidRouteProtocolCombination = errors.New("invalid route protocol combination")
	// ErrUpstreamModelNotFound 上游模型路由绑定不存在。
	ErrUpstreamModelNotFound = repository.ErrUpstreamModelNotFound
	// ErrUpstreamModelConflict 上游模型路由绑定冲突。
	ErrUpstreamModelConflict = repository.ErrUpstreamModelConflict
	// ErrRemoteModelsUnavailable 上游远程模型目录不可用。
	ErrRemoteModelsUnavailable = errors.New("remote models unavailable")
	// ErrNoActiveKey 无可用密钥。
	ErrNoActiveKey = errors.New("no active api key")
	// ErrLLMSettingNotFound LLM 全局设置不存在。
	ErrLLMSettingNotFound = repository.ErrLLMSettingNotFound
)
