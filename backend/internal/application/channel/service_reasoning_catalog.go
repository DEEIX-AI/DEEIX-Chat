package channel

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	domainchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/channel"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"go.uber.org/zap"
)

// 推理目录数据来源，取值与领域层一致。
const (
	ReasoningCatalogOriginRemote  = domainchannel.ReasoningCatalogOriginRemote
	ReasoningCatalogOriginBuiltin = domainchannel.ReasoningCatalogOriginBuiltin
)

// 目录同步与 OpenRouter 官方定价一致采用按需策略：不常驻定时任务，只在管理员查看目录状态时检查是否过期，
// 过期则在后台同步一次。离线 / 内网部署不查看就不会发起出站请求，运行时始终有内置快照兜底。
const (
	// reasoningCatalogRefreshInterval 为目录数据的有效期，超过后查看状态时会在后台重新拉取。
	reasoningCatalogRefreshInterval = 24 * time.Hour
	// reasoningCatalogRetryInterval 为按需同步失败后的最短重试间隔，避免每次查看都请求不可达的地址。
	reasoningCatalogRetryInterval = time.Hour
	reasoningCatalogFetchTimeout  = 90 * time.Second
	reasoningCatalogLoadTimeout   = 10 * time.Second
	maxReasoningCatalogErrorRunes = 500
)

var errReasoningCatalogEmpty = errors.New("reasoning catalog is empty")

// reasoningCatalogFetcher 拉取 models.dev 目录并精简为推理目录条目。
type reasoningCatalogFetcher interface {
	FetchReasoningCatalog(ctx context.Context) ([]domainchannel.ReasoningCatalogEntry, error)
}

type reasoningCatalogState struct {
	catalog   *domainchannel.ReasoningCatalog
	origin    string
	fetchedAt time.Time
}

// reasoningCatalogStore 持有当前生效的推理目录：内存索引只读共享，替换时整体换指针。
type reasoningCatalogStore struct {
	mu        sync.RWMutex
	state     *reasoningCatalogState
	lastError string
	// lastAttemptAt 为最近一次发起同步（按需或手动）的时间，用于失败后的重试间隔。
	lastAttemptAt time.Time
	// backgroundPending 表示已认领一次按需同步、尚未结束；inFlight 为正在执行的同步数。
	backgroundPending bool
	inFlight          int
	// refreshMu 串行化远端拉取，避免按需同步与手动刷新并发请求。
	refreshMu sync.Mutex

	fetcher reasoningCatalogFetcher
	cache   repository.ReasoningCatalogCacheRepository
	builtin *domainchannel.ReasoningCatalogSnapshot
}

// ReasoningCatalogStatus 是管理端展示的推理目录状态。
type ReasoningCatalogStatus struct {
	// Origin 为 remote / builtin；尚未加载任何目录时为空。
	Origin     string
	FetchedAt  *time.Time
	ModelCount int
	LastError  string
	// Refreshing 表示正在同步。
	Refreshing bool
}

// SetReasoningCatalogSources 注入 models.dev 推理目录的远端拉取器、本地缓存与内置快照；均可为空。
func (s *Service) SetReasoningCatalogSources(fetcher reasoningCatalogFetcher, cache repository.ReasoningCatalogCacheRepository, builtin *domainchannel.ReasoningCatalogSnapshot) {
	store := &s.reasoningCatalog
	store.mu.Lock()
	defer store.mu.Unlock()
	store.fetcher = fetcher
	store.cache = cache
	store.builtin = builtin
}

// LoadReasoningCatalog 在启动时从 storage 恢复目录（缺失或比内置快照旧时由内置快照写入），不发起网络请求。
func (s *Service) LoadReasoningCatalog(ctx context.Context) {
	if ctx == nil {
		return
	}
	loadCtx, cancel := context.WithTimeout(ctx, reasoningCatalogLoadTimeout)
	defer cancel()
	s.loadReasoningCatalog(loadCtx)
}

// loadReasoningCatalog 以 storage 中的目录文件为准；文件缺失、损坏或比内置快照旧时，用内置快照写入 storage
// （首次启动或升级后的种子），两者都不可用时保持无目录。
func (s *Service) loadReasoningCatalog(ctx context.Context) {
	store := &s.reasoningCatalog
	store.mu.RLock()
	builtin, cache := store.builtin, store.cache
	store.mu.RUnlock()
	if builtin != nil && len(builtin.Entries) == 0 {
		builtin = nil
	}

	var selected *domainchannel.ReasoningCatalogSnapshot
	if cache != nil {
		snapshot, found, err := cache.Load(ctx)
		switch {
		case err != nil:
			s.warn("reasoning_catalog_cache_load_failed", zap.Error(err))
		case found && len(snapshot.Entries) > 0 && (builtin == nil || !builtin.FetchedAt.After(snapshot.FetchedAt)):
			selected = &snapshot
		}
	}
	if selected == nil && builtin != nil {
		seed := *builtin
		seed.Origin = ReasoningCatalogOriginBuiltin
		selected = &seed
		if cache != nil {
			if err := cache.Store(ctx, seed); err != nil {
				s.warn("reasoning_catalog_seed_store_failed", zap.Error(err))
			}
		}
	}
	if selected == nil {
		return
	}
	state := newReasoningCatalogState(*selected)
	store.mu.Lock()
	// 启动期间手动刷新已写入更新的目录时不回退。
	if store.state == nil || state.fetchedAt.After(store.state.fetchedAt) {
		store.state = state
	}
	store.mu.Unlock()
}

func newReasoningCatalogState(snapshot domainchannel.ReasoningCatalogSnapshot) *reasoningCatalogState {
	origin := strings.TrimSpace(snapshot.Origin)
	if origin == "" {
		origin = ReasoningCatalogOriginRemote
	}
	return &reasoningCatalogState{
		catalog:   domainchannel.NewReasoningCatalog(snapshot.Entries),
		origin:    origin,
		fetchedAt: snapshot.FetchedAt,
	}
}

// RefreshReasoningCatalogIfStale 在目录过期（或尚无目录）时于后台同步一次，返回是否发起了同步。
// 同步进行中或距上次尝试不足重试间隔时不再发起；失败只记 debug 日志，原因经状态展示给管理员。
// ctx 通常是查看状态的请求上下文；同步与之解耦（WithoutCancel），请求结束不会中断拉取。
func (s *Service) RefreshReasoningCatalogIfStale(ctx context.Context) bool {
	if !s.claimReasoningCatalogRefresh(time.Now()) {
		return false
	}
	go s.runClaimedReasoningCatalogRefresh(context.WithoutCancel(ctx))
	return true
}

// claimReasoningCatalogRefresh 判断是否需要按需同步，需要时在同一把锁内认领，避免并发查看重复拉取。
func (s *Service) claimReasoningCatalogRefresh(now time.Time) bool {
	store := &s.reasoningCatalog
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.fetcher == nil || store.backgroundPending || store.inFlight > 0 {
		return false
	}
	if store.state != nil && now.Sub(store.state.fetchedAt) < reasoningCatalogRefreshInterval {
		return false
	}
	if !store.lastAttemptAt.IsZero() && now.Sub(store.lastAttemptAt) < reasoningCatalogRetryInterval {
		return false
	}
	store.backgroundPending = true
	store.lastAttemptAt = now
	return true
}

func (s *Service) runClaimedReasoningCatalogRefresh(ctx context.Context) {
	defer func() {
		s.reasoningCatalog.mu.Lock()
		s.reasoningCatalog.backgroundPending = false
		s.reasoningCatalog.mu.Unlock()
	}()
	if _, err := s.RefreshReasoningCatalog(ctx); err != nil {
		s.debug("reasoning_catalog_refresh_failed", zap.Error(err))
	}
}

// RefreshReasoningCatalog 立即拉取 models.dev 目录，成功后替换内存目录并写入本地缓存；
// 失败时保留当前目录并记录错误，供管理端展示。
func (s *Service) RefreshReasoningCatalog(ctx context.Context) (ReasoningCatalogStatus, error) {
	store := &s.reasoningCatalog
	store.mu.RLock()
	fetcher, cache := store.fetcher, store.cache
	store.mu.RUnlock()
	if fetcher == nil {
		return s.ReasoningCatalogStatus(), ErrReasoningCatalogFetchFailed
	}

	store.mu.Lock()
	store.inFlight++
	store.lastAttemptAt = time.Now()
	store.mu.Unlock()
	defer func() {
		store.mu.Lock()
		store.inFlight--
		store.mu.Unlock()
	}()

	store.refreshMu.Lock()
	defer store.refreshMu.Unlock()

	// 与发起请求解耦：管理员关闭页面不应中断拉取、污染 lastError 或留下未写入的缓存。
	ctx = context.WithoutCancel(ctx)
	fetchCtx, cancel := context.WithTimeout(ctx, reasoningCatalogFetchTimeout)
	defer cancel()
	entries, err := fetcher.FetchReasoningCatalog(fetchCtx)
	if err == nil && len(entries) == 0 {
		err = errReasoningCatalogEmpty
	}
	if err != nil {
		store.mu.Lock()
		store.lastError = truncateReasoningCatalogError(err.Error())
		store.mu.Unlock()
		return s.ReasoningCatalogStatus(), fmt.Errorf("%w: %v", ErrReasoningCatalogFetchFailed, err)
	}

	snapshot := domainchannel.ReasoningCatalogSnapshot{
		Origin:    ReasoningCatalogOriginRemote,
		FetchedAt: time.Now().UTC(),
		Entries:   entries,
	}
	if cache != nil {
		if storeErr := cache.Store(ctx, snapshot); storeErr != nil {
			// 缓存只影响重启后的恢复，内存目录照常替换。
			s.warn("reasoning_catalog_cache_store_failed", zap.Error(storeErr))
		}
	}
	store.mu.Lock()
	store.state = newReasoningCatalogState(snapshot)
	store.lastError = ""
	store.mu.Unlock()
	return s.ReasoningCatalogStatus(), nil
}

// ReasoningCatalogStatus 返回当前推理目录状态。
func (s *Service) ReasoningCatalogStatus() ReasoningCatalogStatus {
	store := &s.reasoningCatalog
	store.mu.RLock()
	defer store.mu.RUnlock()
	status := ReasoningCatalogStatus{
		LastError:  store.lastError,
		Refreshing: store.backgroundPending || store.inFlight > 0,
	}
	if state := store.state; state != nil {
		fetchedAt := state.fetchedAt
		status.Origin = state.origin
		status.FetchedAt = &fetchedAt
		status.ModelCount = state.catalog.Len()
	}
	return status
}

// activeReasoningCatalog 返回当前生效的目录；尚未加载时返回 nil。
func (s *Service) activeReasoningCatalog() *domainchannel.ReasoningCatalog {
	if s == nil {
		return nil
	}
	s.reasoningCatalog.mu.RLock()
	defer s.reasoningCatalog.mu.RUnlock()
	if s.reasoningCatalog.state == nil {
		return nil
	}
	return s.reasoningCatalog.state.catalog
}

func truncateReasoningCatalogError(message string) string {
	runes := []rune(strings.TrimSpace(message))
	if len(runes) > maxReasoningCatalogErrorRunes {
		return string(runes[:maxReasoningCatalogErrorRunes]) + "…"
	}
	return string(runes)
}

// catalogReasoningForRoute 返回目录在路由协议上推导出的推理能力；未匹配或协议不支持时返回 nil。
// 优先级判断（显式声明、旧配置推断优先）由请求链路的 ResolveReasoningCapability 完成。
func (s *Service) catalogReasoningForRoute(route *ResolvedRoute) *domainchannel.ReasoningCapability {
	if route == nil || domainchannel.HasExplicitReasoning(route.ModelCapabilitiesJSON) {
		return nil
	}
	catalog := s.activeReasoningCatalog()
	if catalog == nil {
		return nil
	}
	protocolKey := llm.OptionPolicyProtocolKey(route.Protocol)
	entry := catalog.Match(route.PlatformModelName, route.ModelVendor, []string{protocolKey})
	capability, _ := domainchannel.CatalogReasoningCapability(entry, protocolKey, route.ModelVendor)
	return capability
}

// ---------------------------------------------------------------------------
// 模型目录中的推理能力
// ---------------------------------------------------------------------------

// ModelReasoningInfo 是单个平台模型的推理能力解析结果。
type ModelReasoningInfo struct {
	// View 为生效的推理能力；无能力时为 nil。
	View *ModelReasoningView
	// Source 为生效能力的来源（explicit / inferred / catalog）；显式声明无效时仍为 explicit，无能力时为空。
	Source string
	// Controls 为用户端可操作的模型控件（含思考强度控件），顺序即展示顺序；包含管理员隐藏的控件。
	Controls []domainchannel.ModelControl
}

// ModelReasoningResolver 在一次请求内复用同一份目录快照解析多个模型的推理能力。
type ModelReasoningResolver struct {
	catalog *domainchannel.ReasoningCatalog
}

// ModelReasoningResolver 返回绑定当前目录快照的解析器；目录尚未加载时只解析显式声明与旧配置推断。
func (s *Service) ModelReasoningResolver() ModelReasoningResolver {
	return ModelReasoningResolver{catalog: s.activeReasoningCatalog()}
}

// Resolve 解析模型的生效推理能力与用户控件；目录能力只在没有显式声明与原生参数推断时兜底。
func (r ModelReasoningResolver) Resolve(view ModelView) ModelReasoningInfo {
	protocolKeys := modelReasoningProtocolKeys(view.ProtocolsJSON)
	var suggestion *domainchannel.ReasoningCapability
	if r.catalog != nil && len(protocolKeys) > 0 {
		entry := r.catalog.Match(view.PlatformModelName, view.Vendor, protocolKeys)
		suggestion, _ = domainchannel.CatalogReasoningCapabilityForProtocols(entry, protocolKeys, view.Vendor)
	}
	capability, source := domainchannel.ResolveReasoningCapabilityForProtocols(protocolKeys, view.CapabilitiesJSON, suggestion)
	info := ModelReasoningInfo{
		Source:   source,
		Controls: domainchannel.ResolveModelControls(view.CapabilitiesJSON, protocolKeys, capability, source),
	}
	if capability != nil {
		info.View = &ModelReasoningView{
			Levels:      append([]string(nil), capability.Levels...),
			Default:     capability.Default,
			ControlPath: capability.ControlPath,
			Source:      source,
			Locked:      modelReasoningLocked(*capability, protocolKeys, view.CapabilitiesJSON),
		}
	}
	return info
}
