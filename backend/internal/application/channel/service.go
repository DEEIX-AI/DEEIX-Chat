package channel

import (
	"context"
	"sync"
	"time"

	appbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/billing"
	appstorage "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/objectstorage"
	domainchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/channel"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"
)

type billingModelPricingFilter interface {
	GetBillingMode(ctx context.Context) (string, error)
	ListPublicModelPricing(ctx context.Context) (map[string]appbilling.PublicModelPricing, error)
}

// permissionGroupRepo 提供模型访问权限组的查询能力。
type permissionGroupRepo interface {
	ListModelsWithGroupAccess(ctx context.Context) (map[uint][]uint, error)
	ListUserGroupIDs(ctx context.Context, userID uint) ([]uint, error)
	ListDefaultGroupIDs(ctx context.Context) ([]uint, error)
	ListModelGroupIDs(ctx context.Context, platformModelID uint) ([]uint, error)
}

type subscriptionGroupResolver interface {
	GetUserSubscriptionGroupID(ctx context.Context, userID uint) (*uint, error)
}

type modelPermissionGroupWriter interface {
	PermissionGroupExists(ctx context.Context, id uint) (bool, error)
	ListModelManualGroupIDs(ctx context.Context, platformModelID uint) ([]uint, error)
	SetModelManualGroups(ctx context.Context, platformModelID uint, groupIDs []uint) error
}

// resolveUserGroupIDs 返回用户的全部归属权限组 ID（手动权限组 + 默认权限组 + 订阅绑定权限组）。
func (s *Service) resolveUserGroupIDs(ctx context.Context, userID uint) (map[uint]struct{}, error) {
	groups := make(map[uint]struct{})
	if s.permGroupRepo == nil || userID == 0 {
		return groups, nil
	}
	ids, err := s.permGroupRepo.ListUserGroupIDs(ctx, userID)
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		groups[id] = struct{}{}
	}
	defaultIDs, err := s.permGroupRepo.ListDefaultGroupIDs(ctx)
	if err != nil {
		return nil, err
	}
	for _, id := range defaultIDs {
		groups[id] = struct{}{}
	}
	if s.subGroupResolver != nil {
		subGroupID, err := s.subGroupResolver.GetUserSubscriptionGroupID(ctx, userID)
		if err != nil {
			return nil, err
		}
		if subGroupID != nil {
			groups[*subGroupID] = struct{}{}
		}
	}
	return groups, nil
}

// isModelAccessible 判断用户是否可访问指定模型（基于权限组归属）。
func (s *Service) isModelAccessible(ctx context.Context, platformModelID uint, userID uint) (bool, error) {
	if s.permGroupRepo == nil || userID == 0 {
		return true, nil
	}
	modelGroups, err := s.permGroupRepo.ListModelGroupIDs(ctx, platformModelID)
	if err != nil {
		return false, err
	}
	if len(modelGroups) == 0 {
		return false, nil
	}
	userGroups, err := s.resolveUserGroupIDs(ctx, userID)
	if err != nil {
		return false, err
	}
	for _, gid := range modelGroups {
		if _, ok := userGroups[gid]; ok {
			return true, nil
		}
	}
	return false, nil
}

// llmGateway 定义渠道探活与模型目录同步所需的上游调用能力。
type llmGateway interface {
	Generate(ctx context.Context, route llm.RouteConfig, input llm.GenerateInput) (*llm.GenerateOutput, error)
	ListModels(ctx context.Context, route llm.RouteConfig) ([]llm.ModelItem, error)
}

// Service 封装上游、平台模型与路由绑定业务能力。
type Service struct {
	cfg                 *config.Runtime
	repo                repository.ChannelRepository
	presentationRepo    repository.ModelPresentationRepository
	iconAssetRepo       repository.ModelIconAssetRepository
	cache               repository.ChannelCacheRepository
	llmClient           llmGateway
	modelPricingFilter  billingModelPricingFilter
	permGroupRepo       permissionGroupRepo
	subGroupResolver    subscriptionGroupResolver
	logger              *zap.Logger
	objectStoreProvider appstorage.Provider
	localAPIKeyCounters sync.Map

	modelCatalogMu         sync.RWMutex
	modelCatalog           []ModelView
	modelCatalogMode       string
	modelCatalogValidUntil time.Time
	modelCatalogGeneration uint64
	modelCatalogRequests   singleflight.Group

	breakerDefaultsMu         sync.RWMutex
	breakerDefaults           domainchannel.BreakerDefaults
	breakerDefaultsLoaded     bool
	breakerDefaultsValidUntil time.Time
}

// SetObjectStoreProvider 注入运行时对象存储，用于管理员自定义模型图标。
func (s *Service) SetObjectStoreProvider(provider appstorage.Provider) {
	s.objectStoreProvider = provider
}

// SetModelIconAssetRepository 注入自定义模型图标元数据仓储。
func (s *Service) SetModelIconAssetRepository(repo repository.ModelIconAssetRepository) {
	s.iconAssetRepo = repo
}

func (s *Service) llmAttribution() (string, string) {
	if s == nil || s.cfg == nil {
		return "", ""
	}
	cfg := s.cfg.Snapshot()
	return cfg.PublicWebBaseURL, cfg.AppName
}

// ResolvedRoute 模型请求路由结果。
type ResolvedRoute struct {
	RouteID                         uint
	PlatformModelID                 uint
	PlatformModelName               string
	UpstreamModelID                 uint
	UpstreamID                      uint
	UpstreamName                    string
	BindingCode                     string
	Protocol                        string
	BaseURL                         string
	APIKey                          string
	ConnectTimeoutMS                int
	ReadTimeoutMS                   int
	StreamIdleTimeoutMS             int
	HeadersJSON                     string
	ModelVendor                     string
	ModelIcon                       string
	ModelCapabilitiesJSON           string
	ModelSystemPrompt               string
	UpstreamModel                   string
	ReasoningContentPassback        bool
	ReasoningPassbackRequestOptions map[string]any
	UpstreamCbFailureThreshold      int
	UpstreamCbModelThreshold        int
	UpstreamCbThresholdLogic        string
	UpstreamCbDurationMin           int
	UpstreamCbWindowMin             int
	PlatformModelCbPolicyMode       string
	PlatformModelCbFailureThreshold int
	PlatformModelCbDurationMin      int
	PlatformModelCbWindowMin        int
	ModelCbFailureThreshold         int
	ModelCbDurationMin              int
	ModelCbWindowMin                int
	UpstreamProbeGranted            bool
	ModelProbeGranted               bool
}

// ResolveRouteInput 路由解析输入。
type ResolveRouteInput struct {
	PlatformModelName string
	TaskType          string
	Scope             string
	UserID            uint
	ConversationID    uint
	RequestID         string
	ExcludedRouteIDs  []uint
}

type routeCandidate struct {
	row    repository.ChannelUpstreamRouteRow
	apiKey string
}

type routeFailureClass string

const (
	routeFailureCircuit          routeFailureClass = "circuit"
	routeFailureRateLimit        routeFailureClass = "rate_limit"
	routeFailureIgnore           routeFailureClass = "ignore"
	circuitProbeTTLSec                             = 30
	modelCatalogCacheTTL                           = 30 * time.Second
	breakerDefaultsCacheTTL                        = 5 * time.Second
	breakerDefaultsErrorRetryTTL                   = time.Second
)

const (
	ModelAccessScopePublic   = "public"
	ModelAccessScopeInternal = "internal"

	RouteScopeUser     = "user"
	RouteScopeInternal = "internal"
)

// NewServiceWithRuntime 创建使用运行时配置容器的服务。
func NewServiceWithRuntime(cfg *config.Runtime, repo repository.ChannelRepository, presentationRepo repository.ModelPresentationRepository, cache repository.ChannelCacheRepository, llmClient llmGateway) *Service {
	return &Service{
		cfg:              cfg,
		repo:             repo,
		presentationRepo: presentationRepo,
		cache:            cache,
		llmClient:        llmClient,
	}
}

// SetBillingModelPricingFilter 注入计费模型过滤器，用于用户侧模型选择列表。
func (s *Service) SetBillingModelPricingFilter(filter billingModelPricingFilter) {
	s.modelPricingFilter = filter
}

// SetPermissionGroupRepo 注入模型访问权限组仓储，用于按用户过滤模型访问。
func (s *Service) SetPermissionGroupRepo(repo permissionGroupRepo) {
	s.permGroupRepo = repo
}

// SetSubscriptionGroupResolver 注入订阅绑定权限组解析能力。
func (s *Service) SetSubscriptionGroupResolver(resolver subscriptionGroupResolver) {
	s.subGroupResolver = resolver
}

// SetLogger 注入结构化日志记录器。
func (s *Service) SetLogger(logger *zap.Logger) {
	s.logger = logger
}

func (s *Service) warn(message string, fields ...zap.Field) {
	if s.logger == nil {
		return
	}
	s.logger.Warn(message, fields...)
}

// InvalidateModelCatalog 清除用户侧模型目录缓存。
func (s *Service) InvalidateModelCatalog() {
	if s == nil {
		return
	}
	s.modelCatalogMu.Lock()
	s.modelCatalog = nil
	s.modelCatalogMode = ""
	s.modelCatalogValidUntil = time.Time{}
	s.modelCatalogGeneration++
	s.modelCatalogMu.Unlock()
}
