package repository

import (
	"context"
	"time"

	domainchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/channel"
)

// CircuitFailureInput 记录熔断失败所需的全量参数（时长均以秒为单位）。
type CircuitFailureInput struct {
	UpstreamID               uint
	ModelKey                 string
	ModelWindowSec           int
	ModelFailureThreshold    int
	ModelDurationSec         int
	UpstreamWindowSec        int
	UpstreamFailureThreshold int
	UpstreamModelThreshold   int
	UpstreamThresholdLogic   string
	UpstreamDurationSec      int
	// ActiveModelKeys 用于 upstream_model_threshold 逻辑（当前上游下所有活跃路由绑定 key）。
	ActiveModelKeys []string
}

// RateLimitBackoffParams rate limit 指数退避计算参数。
type RateLimitBackoffParams struct {
	BackoffBaseSec    int
	BackoffMaxSec     int
	BackoffMultiplier int
}

// ChannelCacheRepository 封装 channel 模块的缓存能力，屏蔽 Redis 细节。
type ChannelCacheRepository interface {
	// CheckUpstreamCircuitState 检查上游级熔断状态。
	// 返回值为 "closed" / "open" / "half_open_granted" / "half_open_denied"。
	CheckUpstreamCircuitState(ctx context.Context, upstreamID uint) (string, error)

	// CheckModelCircuitState 检查模型级熔断状态。
	// 返回值同上。
	CheckModelCircuitState(ctx context.Context, upstreamID uint, modelKey string) (string, error)

	// RecordCircuitFailure 使用 Lua 脚本原子记录失败并按阈值触发熔断。
	RecordCircuitFailure(ctx context.Context, input CircuitFailureInput) error

	// RecordFailureMetadata 记录上游最近失败时间与错误信息；写入失败不阻塞主请求。
	RecordFailureMetadata(ctx context.Context, upstreamID uint, lastError string)

	// RecordSuccessMetadata 记录上游最近成功时间。
	RecordSuccessMetadata(ctx context.Context, upstreamID uint)

	// ClearUpstreamCircuitKeys 清除 probe 成功后的上游熔断关键键（fails/open/until/probe）。
	ClearUpstreamCircuitKeys(ctx context.Context, upstreamID uint) error

	// ClearModelCircuitKeys 清除 probe 成功后的模型级熔断关键键。
	ClearModelCircuitKeys(ctx context.Context, upstreamID uint, modelKey string) error

	// ReleaseRouteProbes 释放路由上的 probe 令牌（ignore/rate_limit 时使用）。
	// modelKey 为空则仅释放上游 probe。
	ReleaseRouteProbes(ctx context.Context, upstreamID uint, modelKey string) error

	// OpenUpstreamCircuit 手动打开上游熔断（24 小时）。
	OpenUpstreamCircuit(ctx context.Context, upstreamID uint) error

	// ResetUpstreamCircuit 重置上游全量熔断与计数键。
	ResetUpstreamCircuit(ctx context.Context, upstreamID uint) error

	// OpenModelCircuit 手动打开模型级熔断（24 小时）。
	OpenModelCircuit(ctx context.Context, upstreamID uint, modelKey string) error

	// ResetModelCircuit 重置模型级熔断与计数键。
	ResetModelCircuit(ctx context.Context, upstreamID uint, modelKey string) error

	// QueryUpstreamCircuitStatus 查询上游熔断展示状态（列表用）。
	QueryUpstreamCircuitStatus(ctx context.Context, upstreamID uint) (open bool, until string)

	// QueryModelCircuitStatus 查询模型级熔断展示状态（列表用）。
	QueryModelCircuitStatus(ctx context.Context, upstreamID uint, modelKey string) (open bool, until string)

	// IsRateLimited 判断上游当前是否处于 rate limit 退避中。
	IsRateLimited(ctx context.Context, upstreamID uint) bool

	// RecordRateLimitBackoff 根据指数退避参数记录退避状态。
	RecordRateLimitBackoff(ctx context.Context, upstreamID uint, params RateLimitBackoffParams) error

	// IncrAPIKeyCounter 原子递增 API Key 轮询计数器，返回当前值（用于 round-robin）。
	// 若 Redis 不可用则返回 (0, false)。
	IncrAPIKeyCounter(ctx context.Context, upstreamID uint) (int64, bool)
}

// ChannelUpstreamRouteRow 定义路由查询结果。
type ChannelUpstreamRouteRow struct {
	RouteID                    uint
	UpstreamModelID            uint
	UpstreamID                 uint
	UpstreamName               string
	PlatformModelID            uint
	PlatformModelName          string
	ModelVendor                string
	ModelIcon                  string
	ModelKindsJSON             string
	ModelCapabilitiesJSON      string
	ModelSystemPrompt          string
	Protocol                   string
	BaseURL                    string
	APIKeysEnc                 string
	ConnectTimeoutMS           int
	ReadTimeoutMS              int
	StreamIdleTimeoutMS        int
	HeadersJSON                string
	RouteHeadersJSON           string
	BindingCode                string
	UpstreamModelName          string
	Weight                     int
	RoutePriority              int
	UpstreamCbFailureThreshold int
	UpstreamCbModelThreshold   int
	UpstreamCbThresholdLogic   string
	UpstreamCbDurationMin      int
	UpstreamCbWindowMin        int
	ModelCbFailureThreshold    int
	ModelCbDurationMin         int
	ModelCbWindowMin           int
}

// ChannelUpstreamListRow 定义上游列表查询结果。
type ChannelUpstreamListRow struct {
	domainchannel.Upstream
	ModelsCount       int64
	ActiveModelsCount int64
}

// ChannelModelListRow 定义模型列表查询结果。
type ChannelModelListRow struct {
	domainchannel.PlatformModel
	SourceCount       int64
	ActiveSourceCount int64
	ProtocolsJSON     string
}

// ChannelUpstreamModelListRow 定义上游模型路由绑定列表查询结果。
type ChannelUpstreamModelListRow struct {
	domainchannel.UpstreamModel
	RouteID            uint
	PlatformModelID    uint
	PlatformModelName  string
	ModelVendor        string
	ModelKindsJSON     string
	ModelIcon          string
	Protocol           string
	RouteStatus        string
	Priority           int
	Weight             int
	RouteSource        string
	CbFailureThreshold int
	CbDurationMin      int
	CbWindowMin        int
	HeadersJSON        string
}

// ChannelModelSourceRow 定义模型来源列表查询结果。
type ChannelModelSourceRow struct {
	domainchannel.PlatformModelRoute
	UpstreamID             uint
	UpstreamName           string
	BaseURL                string
	BindingCode            string
	UpstreamModelName      string
	UpstreamModelVendor    string
	UpstreamModelIcon      string
	UpstreamModelKindsJSON string
	SuggestedProtocol      string
	UpstreamModelStatus    string
}

// ListChannelUpstreamModelsInput 定义上游模型路由绑定列表查询条件。
type ListChannelUpstreamModelsInput struct {
	Offset         int
	Limit          int
	Query          string
	RouteStatus    string
	UpstreamStatus string
	Protocol       string
	Sort           string
}

// ListChannelUpstreamsInput 定义上游列表查询条件。
type ListChannelUpstreamsInput struct {
	Offset     int
	Limit      int
	Query      string
	Status     string
	Compatible string
	Sort       string
}

// ListChannelModelsInput 定义模型列表查询条件。
type ListChannelModelsInput struct {
	Offset     int
	Limit      int
	OnlyActive bool
	Query      string
	Status     string
	Vendor     string
	Protocol   string
	Sort       string
}

// UpdateChannelModelInput 定义平台模型更新字段。
type UpdateChannelModelInput struct {
	PlatformModelName *string
	Vendor            *string
	KindsJSON         *string
	Icon              *string
	CapabilitiesJSON  *string
	SystemPrompt      *string
	Status            *string
	Description       *string
}

// UpdateChannelUpstreamInput 定义上游配置更新字段。
type UpdateChannelUpstreamInput struct {
	Name                 *string
	BaseURL              *string
	Compatible           *string
	ProtocolDefaultsJSON *string
	APIKeysEnc           *string
	Status               *string
	ConnectTimeoutMS     *int
	ReadTimeoutMS        *int
	StreamIdleTimeoutMS  *int
	CbFailureThreshold   *int
	CbModelThreshold     *int
	CbThresholdLogic     *string
	CbDurationMin        *int
	CbWindowMin          *int
	HeadersJSON          *string
}

// UpdateChannelUpstreamModelInput 定义上游真实模型更新字段。
type UpdateChannelUpstreamModelInput struct {
	UpstreamModelName *string
	Status            *string
	Source            *string
	SuggestedProtocol *string
	KindsJSON         *string
	LastSyncedAt      **time.Time
	RawJSON           *string
}

// UpdateChannelPlatformRouteInput 定义平台模型路由绑定更新字段。
type UpdateChannelPlatformRouteInput struct {
	PlatformModelID    *uint
	UpstreamModelID    *uint
	Protocol           *string
	Status             *string
	Priority           *int
	Weight             *int
	Source             *string
	CbFailureThreshold *int
	CbDurationMin      *int
	CbWindowMin        *int
	HeadersJSON        *string
}

// IsZero 判断是否没有任何上游配置更新字段。
func (input UpdateChannelUpstreamInput) IsZero() bool {
	return input.Name == nil &&
		input.BaseURL == nil &&
		input.Compatible == nil &&
		input.ProtocolDefaultsJSON == nil &&
		input.APIKeysEnc == nil &&
		input.Status == nil &&
		input.ConnectTimeoutMS == nil &&
		input.ReadTimeoutMS == nil &&
		input.StreamIdleTimeoutMS == nil &&
		input.CbFailureThreshold == nil &&
		input.CbModelThreshold == nil &&
		input.CbThresholdLogic == nil &&
		input.CbDurationMin == nil &&
		input.CbWindowMin == nil &&
		input.HeadersJSON == nil
}

// IsZero 判断是否没有任何路由绑定更新字段。
func (input UpdateChannelUpstreamModelInput) IsZero() bool {
	return input.UpstreamModelName == nil &&
		input.Status == nil &&
		input.Source == nil &&
		input.SuggestedProtocol == nil &&
		input.KindsJSON == nil &&
		input.LastSyncedAt == nil &&
		input.RawJSON == nil
}

func (input UpdateChannelPlatformRouteInput) IsZero() bool {
	return input.PlatformModelID == nil &&
		input.UpstreamModelID == nil &&
		input.Protocol == nil &&
		input.Status == nil &&
		input.Priority == nil &&
		input.Weight == nil &&
		input.Source == nil &&
		input.CbFailureThreshold == nil &&
		input.CbDurationMin == nil &&
		input.CbWindowMin == nil &&
		input.HeadersJSON == nil
}

// IsZero 判断是否没有任何更新字段。
func (input UpdateChannelModelInput) IsZero() bool {
	return input.PlatformModelName == nil &&
		input.Vendor == nil &&
		input.KindsJSON == nil &&
		input.Icon == nil &&
		input.CapabilitiesJSON == nil &&
		input.SystemPrompt == nil &&
		input.Status == nil &&
		input.Description == nil
}

// ChannelRepository 定义渠道管理依赖的仓储能力。
type ChannelRepository interface {
	CreateUpstream(ctx context.Context, item *domainchannel.Upstream) error
	UpdateUpstream(ctx context.Context, upstreamID uint, input UpdateChannelUpstreamInput) error
	GetUpstreamByID(ctx context.Context, upstreamID uint) (*domainchannel.Upstream, error)
	ListUpstreams(ctx context.Context, input ListChannelUpstreamsInput) ([]ChannelUpstreamListRow, int64, error)
	CreateModel(ctx context.Context, item *domainchannel.PlatformModel) error
	UpdateModel(ctx context.Context, modelID uint, input UpdateChannelModelInput) error
	ReorderModels(ctx context.Context, orderedModelIDs []uint) error
	GetModelByID(ctx context.Context, modelID uint) (*domainchannel.PlatformModel, error)
	GetModelListRowByID(ctx context.Context, modelID uint) (*ChannelModelListRow, error)
	GetModelByName(ctx context.Context, platformModelName string) (*domainchannel.PlatformModel, error)
	GetActiveModelByName(ctx context.Context, platformModelName string) (*domainchannel.PlatformModel, error)
	ListModels(ctx context.Context, input ListChannelModelsInput) ([]ChannelModelListRow, int64, error)
	UpsertUpstreamModel(ctx context.Context, item *domainchannel.UpstreamModel) error
	GetUpstreamModelByID(ctx context.Context, sourceID uint, upstreamID uint) (*domainchannel.UpstreamModel, error)
	GetUpstreamModelByUpstreamName(ctx context.Context, upstreamID uint, upstreamModelName string) (*domainchannel.UpstreamModel, error)
	UpdateUpstreamModelByID(ctx context.Context, sourceID uint, upstreamID uint, input UpdateChannelUpstreamModelInput) error
	DeleteUpstreamModel(ctx context.Context, sourceID uint, upstreamID uint) error
	MarkMissingSyncedUpstreamModelsInactive(ctx context.Context, upstreamID uint, activeNames []string) (int64, error)
	ListUpstreamModels(ctx context.Context, upstreamID uint, input ListChannelUpstreamModelsInput) ([]ChannelUpstreamModelListRow, int64, error)
	UpsertPlatformModelRoute(ctx context.Context, item *domainchannel.PlatformModelRoute) error
	GetPlatformModelRouteByID(ctx context.Context, routeID uint, upstreamID uint) (*domainchannel.PlatformModelRoute, error)
	UpdatePlatformModelRouteByID(ctx context.Context, routeID uint, upstreamID uint, input UpdateChannelPlatformRouteInput) error
	DeletePlatformModelRoute(ctx context.Context, routeID uint, upstreamID uint) error
	ListModelUpstreamSources(ctx context.Context, platformModelName string, offset int, limit int) ([]ChannelModelSourceRow, int64, error)
	ListActiveRoutesByModel(ctx context.Context, platformModelName string) ([]ChannelUpstreamRouteRow, error)
	ListActiveRouteBindingCodesForUpstream(ctx context.Context, upstreamID uint) ([]string, error)
	GetLLMSetting(ctx context.Context, key string) (*domainchannel.LLMSetting, error)
	ListLLMSettings(ctx context.Context) ([]domainchannel.LLMSetting, error)
	UpsertLLMSetting(ctx context.Context, item *domainchannel.LLMSetting) error
	// GetBreakerErrorClassification 从全局配置读取并返回熔断错误分类（含默认值）。
	GetBreakerErrorClassification(ctx context.Context) (domainchannel.BreakerErrorClassification, error)
	// GetBreakerDefaults 从全局配置读取并返回熔断器默认参数（含默认值）。
	GetBreakerDefaults(ctx context.Context) (domainchannel.BreakerDefaults, error)
	// GetRateLimitDefaults 从全局配置读取并返回限流退避默认参数（含默认值）。
	GetRateLimitDefaults(ctx context.Context) (domainchannel.RateLimitDefaults, error)
	DeleteUpstreamCascade(ctx context.Context, upstreamID uint) error
	DeleteModelCascade(ctx context.Context, modelID uint) error
}
