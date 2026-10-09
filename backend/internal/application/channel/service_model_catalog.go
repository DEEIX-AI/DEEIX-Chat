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

// 模型目录数据来源，取值与领域层一致。
const (
	ModelCatalogOriginRemote  = domainchannel.ModelCatalogOriginRemote
	ModelCatalogOriginBuiltin = domainchannel.ModelCatalogOriginBuiltin
)

// 目录同步与 OpenRouter 官方定价一致采用按需策略：不常驻定时任务，只在管理员查看目录状态时检查是否过期，
// 过期则在后台同步一次。离线 / 内网部署不查看就不会发起出站请求，运行时始终有内置快照兜底。
const (
	// modelCatalogRefreshInterval 为目录数据的有效期，超过后查看状态时会在后台重新拉取。
	modelCatalogRefreshInterval = 24 * time.Hour
	// modelCatalogRetryInterval 为按需同步失败后的最短重试间隔，避免每次查看都请求不可达的地址。
	modelCatalogRetryInterval = time.Hour
	modelCatalogFetchTimeout  = 90 * time.Second
	modelCatalogLoadTimeout   = 10 * time.Second
	maxModelCatalogErrorRunes = 500
)

var errModelCatalogEmpty = errors.New("model catalog is empty")

// modelCatalogFetcher 拉取 models.dev 目录并精简为模型目录条目。
type modelCatalogFetcher interface {
	FetchCatalog(ctx context.Context) ([]domainchannel.ModelCatalogEntry, error)
}

type modelCatalogState struct {
	// catalog 只索引带推理选项的条目；modalities 索引所有声明了输入模态的条目，并带出展示用的输出模态与上下文窗口。
	catalog    *domainchannel.ModelCatalog
	modalities *domainchannel.ModelCatalog
	origin     string
	fetchedAt  time.Time
	// entryCount 为目录中的模型条目总数（provider 下的每个模型各算一条）。
	entryCount int
}

// modelCatalogStore 持有当前生效的模型目录：内存索引只读共享，替换时整体换指针。
type modelCatalogStore struct {
	mu        sync.RWMutex
	state     *modelCatalogState
	lastError string
	// lastAttemptAt 为最近一次发起同步（按需或手动）的时间，用于失败后的重试间隔。
	lastAttemptAt time.Time
	// backgroundPending 表示已认领一次按需同步、尚未结束；inFlight 为正在执行的同步数。
	backgroundPending bool
	inFlight          int
	// refreshMu 串行化远端拉取，避免按需同步与手动刷新并发请求。
	refreshMu sync.Mutex

	fetcher modelCatalogFetcher
	cache   repository.ModelCatalogCacheRepository
	builtin *domainchannel.ModelCatalogSnapshot
}

// ModelCatalogStatus 是管理端展示的模型目录状态。
type ModelCatalogStatus struct {
	// Origin 为 remote / builtin；尚未加载任何目录时为空。
	Origin    string
	FetchedAt *time.Time
	// ModelCount 为目录中的模型条目总数。
	ModelCount int
	LastError  string
	// Refreshing 表示正在同步。
	Refreshing bool
}

// SetModelCatalogSources 注入 models.dev 模型目录的远端拉取器、本地缓存与内置快照；均可为空。
func (s *Service) SetModelCatalogSources(fetcher modelCatalogFetcher, cache repository.ModelCatalogCacheRepository, builtin *domainchannel.ModelCatalogSnapshot) {
	store := &s.modelsDevCatalog
	store.mu.Lock()
	defer store.mu.Unlock()
	store.fetcher = fetcher
	store.cache = cache
	store.builtin = builtin
}

// LoadModelCatalog 在启动时从 storage 恢复目录（缺失或比内置快照旧时由内置快照写入），不发起网络请求。
func (s *Service) LoadModelCatalog(ctx context.Context) {
	if ctx == nil {
		return
	}
	loadCtx, cancel := context.WithTimeout(ctx, modelCatalogLoadTimeout)
	defer cancel()
	s.loadModelCatalog(loadCtx)
}

// loadModelCatalog 以 storage 中的目录文件为准；文件缺失、损坏或比内置快照旧时，用内置快照写入 storage
// （首次启动或升级后的种子），两者都不可用时保持无目录。
func (s *Service) loadModelCatalog(ctx context.Context) {
	store := &s.modelsDevCatalog
	store.mu.RLock()
	builtin, cache := store.builtin, store.cache
	store.mu.RUnlock()
	if builtin != nil && len(builtin.Entries) == 0 {
		builtin = nil
	}

	var selected *domainchannel.ModelCatalogSnapshot
	if cache != nil {
		snapshot, found, err := cache.Load(ctx)
		switch {
		case err != nil:
			s.warn("model_catalog_cache_load_failed", zap.Error(err))
		case found && len(snapshot.Entries) > 0 && (builtin == nil || !builtin.FetchedAt.After(snapshot.FetchedAt)):
			selected = &snapshot
		}
	}
	if selected == nil && builtin != nil {
		seed := *builtin
		seed.Origin = ModelCatalogOriginBuiltin
		selected = &seed
		if cache != nil {
			if err := cache.Store(ctx, seed); err != nil {
				s.warn("model_catalog_seed_store_failed", zap.Error(err))
			}
		}
	}
	if selected == nil {
		return
	}
	state := newModelCatalogState(*selected)
	store.mu.Lock()
	// 启动期间手动刷新已写入更新的目录时不回退。
	if store.state == nil || state.fetchedAt.After(store.state.fetchedAt) {
		store.state = state
	}
	store.mu.Unlock()
}

func newModelCatalogState(snapshot domainchannel.ModelCatalogSnapshot) *modelCatalogState {
	origin := strings.TrimSpace(snapshot.Origin)
	if origin == "" {
		origin = ModelCatalogOriginRemote
	}
	reasoningEntries, modalityEntries := domainchannel.SplitCatalogEntries(snapshot.Entries)
	return &modelCatalogState{
		catalog:    domainchannel.NewModelCatalog(reasoningEntries),
		modalities: domainchannel.NewModelCatalog(modalityEntries),
		origin:     origin,
		fetchedAt:  snapshot.FetchedAt,
		entryCount: len(snapshot.Entries),
	}
}

// RefreshModelCatalogIfStale 在目录过期（或尚无目录）时于后台同步一次，返回是否发起了同步。
// 同步进行中或距上次尝试不足重试间隔时不再发起；失败只记 debug 日志，原因经状态展示给管理员。
// ctx 通常是查看状态的请求上下文；同步与之解耦（WithoutCancel），请求结束不会中断拉取。
func (s *Service) RefreshModelCatalogIfStale(ctx context.Context) bool {
	if !s.claimModelCatalogRefresh(time.Now()) {
		return false
	}
	go s.runClaimedModelCatalogRefresh(context.WithoutCancel(ctx))
	return true
}

// claimModelCatalogRefresh 判断是否需要按需同步，需要时在同一把锁内认领，避免并发查看重复拉取。
func (s *Service) claimModelCatalogRefresh(now time.Time) bool {
	store := &s.modelsDevCatalog
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.fetcher == nil || store.backgroundPending || store.inFlight > 0 {
		return false
	}
	if store.state != nil && now.Sub(store.state.fetchedAt) < modelCatalogRefreshInterval {
		return false
	}
	if !store.lastAttemptAt.IsZero() && now.Sub(store.lastAttemptAt) < modelCatalogRetryInterval {
		return false
	}
	store.backgroundPending = true
	store.lastAttemptAt = now
	return true
}

func (s *Service) runClaimedModelCatalogRefresh(ctx context.Context) {
	defer func() {
		s.modelsDevCatalog.mu.Lock()
		s.modelsDevCatalog.backgroundPending = false
		s.modelsDevCatalog.mu.Unlock()
	}()
	if _, err := s.RefreshModelCatalog(ctx); err != nil {
		s.debug("model_catalog_refresh_failed", zap.Error(err))
	}
}

// RefreshModelCatalog 立即拉取 models.dev 目录，成功后替换内存目录并写入本地缓存；
// 失败时保留当前目录并记录错误，供管理端展示。
func (s *Service) RefreshModelCatalog(ctx context.Context) (ModelCatalogStatus, error) {
	store := &s.modelsDevCatalog
	store.mu.RLock()
	fetcher, cache := store.fetcher, store.cache
	store.mu.RUnlock()
	if fetcher == nil {
		return s.ModelCatalogStatus(), ErrModelCatalogFetchFailed
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
	fetchCtx, cancel := context.WithTimeout(ctx, modelCatalogFetchTimeout)
	defer cancel()
	entries, err := fetcher.FetchCatalog(fetchCtx)
	if err == nil && len(entries) == 0 {
		err = errModelCatalogEmpty
	}
	if err != nil {
		store.mu.Lock()
		store.lastError = truncateModelCatalogError(err.Error())
		store.mu.Unlock()
		return s.ModelCatalogStatus(), fmt.Errorf("%w: %v", ErrModelCatalogFetchFailed, err)
	}

	snapshot := domainchannel.ModelCatalogSnapshot{
		Origin:    ModelCatalogOriginRemote,
		FetchedAt: time.Now().UTC(),
		Entries:   entries,
	}
	if cache != nil {
		if storeErr := cache.Store(ctx, snapshot); storeErr != nil {
			// 缓存只影响重启后的恢复，内存目录照常替换。
			s.warn("model_catalog_cache_store_failed", zap.Error(storeErr))
		}
	}
	store.mu.Lock()
	store.state = newModelCatalogState(snapshot)
	store.lastError = ""
	store.mu.Unlock()
	return s.ModelCatalogStatus(), nil
}

// ModelCatalogStatus 返回当前模型目录状态。
func (s *Service) ModelCatalogStatus() ModelCatalogStatus {
	store := &s.modelsDevCatalog
	store.mu.RLock()
	defer store.mu.RUnlock()
	status := ModelCatalogStatus{
		LastError:  store.lastError,
		Refreshing: store.backgroundPending || store.inFlight > 0,
	}
	if state := store.state; state != nil {
		fetchedAt := state.fetchedAt
		status.Origin = state.origin
		status.FetchedAt = &fetchedAt
		status.ModelCount = state.entryCount
	}
	return status
}

// ModelCatalogResolveInput 是管理端编辑模型时的自动识别查询：模型身份、已绑定路由的协议与当前能力 JSON。
type ModelCatalogResolveInput struct {
	Name             string
	Vendor           string
	Protocols        []string
	CapabilitiesJSON string
}

// ModelCatalogResolution 是模型编辑表单「自动识别」所需的全部信息。
type ModelCatalogResolution struct {
	// Matched 表示 models.dev 目录中找到了对应条目；Provider / ModelID 为命中的条目。
	Matched  bool
	Provider string
	ModelID  string
	// InputModalities / OutputModalities 为目录声明的模态；ContextWindow 为目录上下文窗口，未知时为 0。
	InputModalities  []string
	OutputModalities []string
	ContextWindow    int
	// AutoReasoning 为不考虑显式 reasoning 声明时的生效推理能力（旧配置推断或目录），与请求链路的解析一致；
	// AutoReasoningSource 为 inferred / catalog，无能力时两者均为空。
	AutoReasoning       *domainchannel.ReasoningCapability
	AutoReasoningSource string
	// ReasoningTemplate 为自定义推理强度的编辑模板；尚无可用协议且无能力时为 nil。
	ReasoningTemplate *domainchannel.ReasoningTemplate
}

// ResolveModelCatalog 返回模型编辑表单的自动识别结果，匹配规则与请求链路一致；目录过期时顺带在后台同步一次。
func (s *Service) ResolveModelCatalog(ctx context.Context, input ModelCatalogResolveInput) ModelCatalogResolution {
	s.RefreshModelCatalogIfStale(ctx)
	name := strings.TrimSpace(input.Name)
	vendor := strings.TrimSpace(input.Vendor)
	protocolKeys := normalizeReasoningProtocolKeys(input.Protocols)
	var result ModelCatalogResolution
	if name == "" {
		return result
	}

	reasoningCatalog := s.activeModelCatalog()
	var reasoningEntry *domainchannel.ModelCatalogEntry
	if reasoningCatalog != nil {
		reasoningEntry = reasoningCatalog.Match(name, vendor, protocolKeys)
	}
	entry := reasoningEntry
	if modalityCatalog := s.activeModalityCatalog(); modalityCatalog != nil {
		if matched := modalityCatalog.Match(name, vendor, protocolKeys); matched != nil {
			entry = matched
		}
	}
	if entry != nil {
		result.Matched = true
		result.Provider = entry.Provider
		result.ModelID = entry.ModelID
		result.InputModalities = append([]string(nil), entry.InputModalities...)
		result.OutputModalities = append([]string(nil), entry.OutputModalities...)
		result.ContextWindow = entry.ContextWindow
	}

	var suggestion *domainchannel.ReasoningCapability
	if len(protocolKeys) > 0 {
		suggestion, _ = domainchannel.CatalogReasoningCapabilityForProtocols(reasoningEntry, protocolKeys, vendor)
	}
	result.AutoReasoning, result.AutoReasoningSource = domainchannel.ResolveReasoningCapabilityForProtocols(
		protocolKeys,
		domainchannel.WithoutExplicitReasoning(input.CapabilitiesJSON),
		suggestion,
	)
	budgetMin, budgetMax := domainchannel.CatalogReasoningBudgetBounds(reasoningEntry)
	result.ReasoningTemplate = domainchannel.ReasoningCustomizationTemplate(protocolKeys, result.AutoReasoning, suggestion, budgetMin, budgetMax)
	return result
}

// activeModalityCatalog 返回当前生效的输入模态目录；尚未加载时返回 nil。
func (s *Service) activeModalityCatalog() *domainchannel.ModelCatalog {
	if s == nil {
		return nil
	}
	s.modelsDevCatalog.mu.RLock()
	defer s.modelsDevCatalog.mu.RUnlock()
	if s.modelsDevCatalog.state == nil {
		return nil
	}
	return s.modelsDevCatalog.state.modalities
}

// catalogInputModalitiesForRoute 返回目录中该路由模型声明的输入模态；未匹配时返回 nil。
// 显式声明的优先级由请求链路的 ResolveInputModalities 处理。
func (s *Service) catalogInputModalitiesForRoute(route *ResolvedRoute) []string {
	if route == nil {
		return nil
	}
	catalog := s.activeModalityCatalog()
	if catalog == nil {
		return nil
	}
	entry := catalog.Match(route.PlatformModelName, route.ModelVendor, []string{llm.OptionPolicyProtocolKey(route.Protocol)})
	if entry == nil {
		return nil
	}
	return append([]string(nil), entry.InputModalities...)
}

// activeModelCatalog 返回当前生效的目录；尚未加载时返回 nil。
func (s *Service) activeModelCatalog() *domainchannel.ModelCatalog {
	if s == nil {
		return nil
	}
	s.modelsDevCatalog.mu.RLock()
	defer s.modelsDevCatalog.mu.RUnlock()
	if s.modelsDevCatalog.state == nil {
		return nil
	}
	return s.modelsDevCatalog.state.catalog
}

func truncateModelCatalogError(message string) string {
	runes := []rune(strings.TrimSpace(message))
	if len(runes) > maxModelCatalogErrorRunes {
		return string(runes[:maxModelCatalogErrorRunes]) + "…"
	}
	return string(runes)
}

// catalogReasoningForRoute 返回目录在路由协议上推导出的推理能力；未匹配或协议不支持时返回 nil。
// 优先级判断（显式声明、旧配置推断优先）由请求链路的 ResolveReasoningCapability 完成。
func (s *Service) catalogReasoningForRoute(route *ResolvedRoute) *domainchannel.ReasoningCapability {
	if route == nil || domainchannel.HasExplicitReasoning(route.ModelCapabilitiesJSON) {
		return nil
	}
	catalog := s.activeModelCatalog()
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

// ModelCapabilityInfo 是单个平台模型的推理能力解析结果。
type ModelCapabilityInfo struct {
	// View 为生效的推理能力；无能力时为 nil。
	View *ModelReasoningView
	// Source 为生效能力的来源（explicit / inferred / catalog）；显式声明无效时仍为 explicit，无能力时为空。
	Source string
	// Controls 为用户端可操作的模型控件（含思考强度控件），顺序即展示顺序；包含管理员隐藏的控件。
	Controls []domainchannel.ModelControl
	// InputModalities 为生效的输入模态（显式声明优先，其次 models.dev 目录）；来源为空表示未知。
	InputModalities domainchannel.InputModalities
	// OutputModalities 为 models.dev 目录声明的输出模态，仅用于展示；未匹配时为空。
	OutputModalities []string
	// ContextWindow 为生效的上下文窗口（Token）：能力 JSON 中的 contextWindow 优先，其次 models.dev 目录；未知时为 0。
	ContextWindow int
}

// ModelCapabilityResolver 在一次请求内复用同一份目录快照解析多个模型的推理能力。
type ModelCapabilityResolver struct {
	catalog    *domainchannel.ModelCatalog
	modalities *domainchannel.ModelCatalog
}

// ModelCapabilityResolver 返回绑定当前目录快照的解析器；目录尚未加载时只解析显式声明与旧配置推断。
func (s *Service) ModelCapabilityResolver() ModelCapabilityResolver {
	return ModelCapabilityResolver{catalog: s.activeModelCatalog(), modalities: s.activeModalityCatalog()}
}

// ResolveExternal 解析外部模型（用户自带 Key）的能力：与 Resolve 相同，但输入模态与上下文窗口
// 改用 MatchExternal，使中转站常用的 OpenRouter 式 id 也能识别，与 BuildExternalRoute 的发送判断一致。
func (r ModelCapabilityResolver) ResolveExternal(view ModelView) ModelCapabilityInfo {
	info := r.Resolve(view)
	if r.modalities == nil {
		return info
	}
	entry := r.modalities.MatchExternal(view.PlatformModelName, view.Vendor, modelReasoningProtocolKeys(view.ProtocolsJSON))
	if entry == nil {
		return info
	}
	info.InputModalities = domainchannel.ResolveInputModalities(view.CapabilitiesJSON, entry.InputModalities)
	info.OutputModalities = domainchannel.NormalizeOutputModalities(entry.OutputModalities)
	info.ContextWindow = domainchannel.ResolveDisplayContextWindow(view.CapabilitiesJSON, entry.ContextWindow)
	return info
}

// Resolve 解析模型的生效推理能力与用户控件；目录能力只在没有显式声明与原生参数推断时兜底。
func (r ModelCapabilityResolver) Resolve(view ModelView) ModelCapabilityInfo {
	protocolKeys := modelReasoningProtocolKeys(view.ProtocolsJSON)
	var suggestion *domainchannel.ReasoningCapability
	if r.catalog != nil && len(protocolKeys) > 0 {
		entry := r.catalog.Match(view.PlatformModelName, view.Vendor, protocolKeys)
		suggestion, _ = domainchannel.CatalogReasoningCapabilityForProtocols(entry, protocolKeys, view.Vendor)
	}
	capability, source := domainchannel.ResolveReasoningCapabilityForProtocols(protocolKeys, view.CapabilitiesJSON, suggestion)
	var catalogModalities, catalogOutputs []string
	catalogContextWindow := 0
	if r.modalities != nil {
		if entry := r.modalities.Match(view.PlatformModelName, view.Vendor, protocolKeys); entry != nil {
			catalogModalities = entry.InputModalities
			catalogOutputs = domainchannel.NormalizeOutputModalities(entry.OutputModalities)
			catalogContextWindow = entry.ContextWindow
		}
	}
	info := ModelCapabilityInfo{
		Source:           source,
		Controls:         domainchannel.ResolveModelControls(view.CapabilitiesJSON, protocolKeys, capability, source),
		InputModalities:  domainchannel.ResolveInputModalities(view.CapabilitiesJSON, catalogModalities),
		OutputModalities: catalogOutputs,
		ContextWindow:    domainchannel.ResolveDisplayContextWindow(view.CapabilitiesJSON, catalogContextWindow),
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
