package channel

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	appbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/billing"
	domainchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/channel"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/channelconfig"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/nativetool"
	"go.uber.org/zap"
)

// ---------------------------------------------------------------------------
// 模型管理
// ---------------------------------------------------------------------------

const maxSystemPromptChars = 20000

// ListModelsInput 定义模型列表筛选排序条件。
type ListModelsInput struct {
	OnlyActive    bool
	OnlyAvailable bool
	Query         string
	Status        string
	Vendor        string
	Protocol      string
	UpstreamID    uint
	Sort          string
}

// ListModels 分页查询模型目录。
func (s *Service) ListModels(ctx context.Context, page int, pageSize int, input ListModelsInput) ([]ModelView, int64, error) {
	offset, limit := normalizePage(page, pageSize)
	items, total, err := s.repo.ListModels(ctx, repository.ListChannelModelsInput{
		Offset:        offset,
		Limit:         limit,
		OnlyActive:    input.OnlyActive,
		OnlyAvailable: input.OnlyAvailable,
		Query:         input.Query,
		Status:        input.Status,
		Vendor:        input.Vendor,
		Protocol:      input.Protocol,
		UpstreamID:    input.UpstreamID,
		Sort:          input.Sort,
	})
	if err != nil {
		return nil, 0, err
	}
	views := make([]ModelView, 0, len(items))
	for _, item := range items {
		views = append(views, s.toModelView(item))
	}
	if err := s.normalizeModelAvailability(ctx, views); err != nil {
		return nil, 0, err
	}
	return views, total, nil
}

// ListActiveModels 查询全部启用模型目录（用于公开接口）。
//
// userID > 0 时按权限组过滤模型访问；userID == 0 表示内部调用，不做权限过滤。
func (s *Service) ListActiveModels(ctx context.Context, userID uint) ([]ModelView, error) {
	views, err := s.listActiveModelViews(ctx)
	if err != nil {
		return nil, err
	}
	return s.filterModelsByPermission(ctx, userID, views)
}

func (s *Service) listActiveModelViews(ctx context.Context) ([]ModelView, error) {
	now := time.Now()
	mode := "unfiltered"
	if s.modelPricingFilter != nil {
		var err error
		mode, err = s.modelPricingFilter.GetBillingMode(ctx)
		if err != nil {
			return nil, err
		}
	}

	s.modelCatalogMu.RLock()
	generation := s.modelCatalogGeneration
	if s.modelCatalog != nil && s.modelCatalogMode == mode && now.Before(s.modelCatalogValidUntil) {
		result := cloneModelViews(s.modelCatalog)
		s.modelCatalogMu.RUnlock()
		return result, nil
	}
	s.modelCatalogMu.RUnlock()

	resultCh := s.modelCatalogRequests.DoChan(mode+":"+strconv.FormatUint(generation, 10), func() (any, error) {
		refreshCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		refreshStartedAt := time.Now()
		s.modelCatalogMu.RLock()
		if s.modelCatalog != nil && s.modelCatalogMode == mode && refreshStartedAt.Before(s.modelCatalogValidUntil) {
			result := cloneModelViews(s.modelCatalog)
			s.modelCatalogMu.RUnlock()
			return result, nil
		}
		s.modelCatalogMu.RUnlock()

		items, err := s.listAllActiveModelRows(refreshCtx)
		if err != nil {
			return nil, err
		}
		views := s.filterPublicRoutableModels(items)
		if s.modelPricingFilter != nil && mode != "self" {
			pricingByPlatformModelName, err := s.modelPricingFilter.ListPublicModelPricing(refreshCtx)
			if err != nil {
				return nil, err
			}
			views = filterPricedModelViews(views, pricingByPlatformModelName)
		}
		s.storeModelCatalog(refreshStartedAt, mode, generation, views)
		return views, nil
	})
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-resultCh:
		if result.Err != nil {
			return nil, result.Err
		}
		return cloneModelViews(result.Val.([]ModelView)), nil
	}
}

// filterModelsByPermission 按权限组过滤用户可访问的模型。
//
// 未绑定到任何有效权限组的模型对用户隐藏；
// 绑定到权限组的模型仅对归属权限组成员可见。
// 用户归属权限组 = 手动权限组 + 默认权限组（is_default） + 订阅套餐绑定权限组。
func (s *Service) filterModelsByPermission(ctx context.Context, userID uint, views []ModelView) ([]ModelView, error) {
	if s.permGroupRepo == nil || userID == 0 {
		return views, nil
	}
	modelsWithGroups, err := s.permGroupRepo.ListModelsWithGroupAccess(ctx)
	if err != nil {
		return nil, err
	}
	if len(modelsWithGroups) == 0 {
		return []ModelView{}, nil
	}

	userGroups, err := s.resolveUserGroupIDs(ctx, userID)
	if err != nil {
		return nil, err
	}

	results := make([]ModelView, 0, len(views))
	for _, view := range views {
		groups, inGroup := modelsWithGroups[view.ID]
		if !inGroup {
			continue
		}
		for _, gid := range groups {
			if _, ok := userGroups[gid]; ok {
				results = append(results, view)
				break
			}
		}
	}
	return results, nil
}

func (s *Service) listAllActiveModelRows(ctx context.Context) ([]repository.ChannelModelListRow, error) {
	const batchSize = 500
	results := make([]repository.ChannelModelListRow, 0)
	for offset := 0; ; offset += batchSize {
		items, _, err := s.repo.ListModels(ctx, repository.ListChannelModelsInput{
			Offset:     offset,
			Limit:      batchSize,
			OnlyActive: true,
			Sort:       "sortOrder_asc",
		})
		if err != nil {
			return nil, err
		}
		results = append(results, items...)
		if len(items) < batchSize {
			return results, nil
		}
	}
}

// ListNativeToolDefinitions 返回内置目录叠加所有模型能力 JSON 中声明的官方原生工具。
func (s *Service) ListNativeToolDefinitions(ctx context.Context) ([]nativetool.Definition, error) {
	const batchSize = 500
	dynamic := make([]nativetool.Definition, 0)
	for offset := 0; ; offset += batchSize {
		items, _, err := s.repo.ListModels(ctx, repository.ListChannelModelsInput{
			Offset: offset,
			Limit:  batchSize,
			Sort:   "sortOrder_asc",
		})
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			dynamic = append(dynamic, nativetool.DefinitionsFromCapabilitiesJSON(item.CapabilitiesJSON)...)
		}
		if len(items) < batchSize {
			return nativetool.MergeDefinitions(dynamic), nil
		}
	}
}

func (s *Service) storeModelCatalog(now time.Time, mode string, generation uint64, views []ModelView) {
	if s == nil {
		return
	}
	s.modelCatalogMu.Lock()
	defer s.modelCatalogMu.Unlock()
	if s.modelCatalogGeneration != generation {
		return
	}
	s.modelCatalog = cloneModelViews(views)
	s.modelCatalogMode = mode
	s.modelCatalogValidUntil = now.Add(modelCatalogCacheTTL)
}

func cloneModelViews(items []ModelView) []ModelView {
	if len(items) == 0 {
		return []ModelView{}
	}
	results := make([]ModelView, 0, len(items))
	for _, item := range items {
		if item.DisplayGroupID != nil {
			displayGroupID := *item.DisplayGroupID
			item.DisplayGroupID = &displayGroupID
		}
		if item.Pricing != nil {
			pricing := *item.Pricing
			if len(pricing.Tiers) > 0 {
				pricing.Tiers = append([]appbilling.PublicModelPricingTier(nil), pricing.Tiers...)
			}
			item.Pricing = &pricing
		}
		results = append(results, item)
	}
	return results
}

// filterPublicRoutableModels 过滤出公开接口可展示的有效可路由模型。
func (s *Service) filterPublicRoutableModels(items []repository.ChannelModelListRow) []ModelView {
	results := make([]ModelView, 0, len(items))
	for _, item := range items {
		if item.ActiveSourceCount <= 0 {
			continue
		}
		if normalizeModelAccessScopeValue(item.AccessScope) != ModelAccessScopePublic {
			continue
		}
		results = append(results, s.toModelView(item))
	}
	return results
}

func filterPricedModelViews(items []ModelView, pricingByPlatformModelName map[string]appbilling.PublicModelPricing) []ModelView {
	results := make([]ModelView, 0, len(items))
	for _, item := range items {
		pricing, ok := pricingByPlatformModelName[strings.TrimSpace(item.PlatformModelName)]
		if !ok {
			continue
		}
		item.Pricing = &pricing
		results = append(results, item)
	}
	return results
}

func (s *Service) normalizeModelAvailability(ctx context.Context, items []ModelView) error {
	return s.normalizeModelAvailabilityWithRepo(ctx, s.repo, items)
}

func (s *Service) normalizeModelAvailabilityWithRepo(ctx context.Context, repo repository.ChannelRepository, items []ModelView) error {
	breakerEnabled := s.cache != nil && s.loadBreakerDefaults(ctx).Enabled
	for index := range items {
		if items[index].Status != "active" {
			items[index].ActiveSourceCount = 0
			continue
		}
		if !breakerEnabled || items[index].SourceCount <= 0 || items[index].ActiveSourceCount <= 0 {
			continue
		}
		sources, _, err := repo.ListModelUpstreamSources(ctx, items[index].PlatformModelName, 0, int(items[index].SourceCount))
		if err != nil {
			return err
		}
		var active int64
		for _, source := range sources {
			view := toModelUpstreamSourceView(source)
			s.applyModelSourceCircuitStatus(ctx, &view)
			if modelSourceAvailable(view) {
				active++
			}
		}
		items[index].ActiveSourceCount = active
	}
	return nil
}

func modelSourceAvailable(view ModelUpstreamSourceView) bool {
	return view.Status == "active" &&
		view.UpstreamStatus == "active" &&
		view.UpstreamModelStatus == "active" &&
		!view.CircuitOpen
}

// ResolvePlatformModelIdentity 将平台模型名解析为统一平台身份。
func (s *Service) ResolvePlatformModelIdentity(ctx context.Context, platformModelName string) (appbilling.PlatformModelIdentity, error) {
	name, err := normalizePlatformModelName(platformModelName)
	if err != nil {
		return appbilling.PlatformModelIdentity{}, ErrModelNotFound
	}
	item, err := s.repo.GetModelByName(ctx, name)
	if err != nil {
		return appbilling.PlatformModelIdentity{}, err
	}
	return appbilling.PlatformModelIdentity{
		PlatformModelID:   item.ID,
		PlatformModelName: item.PlatformModelName,
		ModelVendor:       strings.TrimSpace(item.Vendor),
		ModelIcon:         strings.TrimSpace(item.Icon),
	}, nil
}

// ListActivePlatformModelNames 返回当前真实可路由的平台模型名集合。
func (s *Service) ListActivePlatformModelNames(ctx context.Context) (map[string]struct{}, error) {
	items, err := s.listAllActiveModelRows(ctx)
	if err != nil {
		return nil, err
	}
	keys := make(map[string]struct{}, len(items))
	for _, item := range items {
		if item.ActiveSourceCount <= 0 {
			continue
		}
		key := strings.TrimSpace(item.PlatformModelName)
		if key == "" {
			continue
		}
		keys[key] = struct{}{}
	}
	return keys, nil
}

// SupportsVideoGeneration 返回平台模型是否具有可按时长计费的视频能力。
func (s *Service) SupportsVideoGeneration(ctx context.Context, platformModelName string) (bool, error) {
	name, err := normalizePlatformModelName(platformModelName)
	if err != nil {
		return false, nil
	}
	kindsJSON, found, err := s.repo.GetActiveRoutableModelKindsJSON(ctx, name)
	if err != nil {
		return false, err
	}
	if !found {
		return false, nil
	}
	kinds := parseKinds(kindsJSON)
	return hasModelKind(kinds, modelKindVideoGen) || hasModelKind(kinds, modelKindVideoExtension), nil
}

// CreateModel 创建平台模型目录项。
//
// 创建模型只负责本地目录与展示元数据。
func (s *Service) CreateModel(ctx context.Context, input CreateModelInput) (*ModelView, error) {
	platformModelName, err := normalizePlatformModelName(input.PlatformModelName)
	if err != nil {
		return nil, err
	}
	kindsJSON := strings.TrimSpace(input.KindsJSON)
	if kindsJSON == "" {
		kindsJSON = inferKindsJSON(platformModelName)
	}
	kindsJSON, err = normalizeKindsJSON(kindsJSON)
	if err != nil {
		return nil, err
	}
	if err := validateOptionalJSON(strings.TrimSpace(input.CapabilitiesJSON)); err != nil {
		return nil, ErrInvalidJSONConfig
	}
	if err := domainchannel.ValidateModelCapsOverrides(input.CapabilitiesJSON); err != nil {
		return nil, ErrInvalidModelCapsConfig
	}
	systemPrompt := strings.TrimSpace(input.SystemPrompt)
	if len([]rune(systemPrompt)) > maxSystemPromptChars {
		return nil, ErrSystemPromptTooLong
	}
	accessScope, err := normalizeModelAccessScope(input.AccessScope)
	if err != nil {
		return nil, err
	}
	cbPolicyMode := normalizeModelCircuitPolicyMode(input.CbPolicyMode)
	vendor, err := s.resolvePlatformModelVendor(ctx, input.Vendor, platformModelName)
	if err != nil {
		return nil, err
	}
	if err := s.validateModelDisplayGroup(ctx, input.DisplayGroupID); err != nil {
		return nil, err
	}
	explicitIcon, err := normalizeModelPresentationIcon(input.Icon)
	if err != nil {
		return nil, err
	}
	var displayGroupID *uint
	if input.DisplayGroupID > 0 {
		value := input.DisplayGroupID
		displayGroupID = &value
	}

	item := &domainchannel.PlatformModel{
		PlatformModelName:  platformModelName,
		Vendor:             vendor,
		DisplayGroupID:     displayGroupID,
		KindsJSON:          kindsJSON,
		Icon:               normalizeModelIcon(explicitIcon, vendor, platformModelName),
		CapabilitiesJSON:   strings.TrimSpace(input.CapabilitiesJSON),
		SystemPrompt:       systemPrompt,
		AccessScope:        accessScope,
		Status:             normalizeStatus(input.Status),
		Description:        strings.TrimSpace(input.Description),
		CbPolicyMode:       cbPolicyMode,
		CbFailureThreshold: normalizeNonNegative(input.CbFailureThreshold),
		CbDurationMin:      normalizeNonNegative(input.CbDurationMin),
		CbWindowMin:        normalizeNonNegative(input.CbWindowMin),
	}
	if err = s.reserveModelIconReference(ctx, item.Icon); err != nil {
		return nil, err
	}
	if err := s.repo.CreateModel(ctx, item); err != nil {
		if isDuplicateKeyError(err) {
			return nil, ErrDuplicatePlatformModelName
		}
		if errors.Is(err, repository.ErrModelVendorNotFound) {
			return nil, ErrModelVendorNotFound
		}
		if errors.Is(err, repository.ErrModelDisplayGroupNotFound) {
			return nil, ErrModelDisplayGroupNotFound
		}
		return nil, err
	}
	s.InvalidateModelCatalog()
	return s.getModelViewByID(ctx, item.ID)
}

// UpdateModel 更新平台模型目录项。
func (s *Service) UpdateModel(ctx context.Context, modelID uint, input UpdateModelInput) (*ModelView, error) {
	current, err := s.repo.GetModelByID(ctx, modelID)
	if err != nil {
		return nil, err
	}

	nextVendor, err := normalizeModelVendorKey(current.Vendor)
	if err != nil {
		return nil, err
	}
	currentVendor := nextVendor
	nextPlatformModelName := current.PlatformModelName

	update := repository.UpdateChannelModelInput{}
	if input.PlatformModelName != nil {
		nextPlatformModelName, err = normalizePlatformModelName(*input.PlatformModelName)
		if err != nil {
			return nil, err
		}
		update.PlatformModelName = &nextPlatformModelName
	}
	if input.Vendor != nil {
		nextVendor, err = s.resolvePlatformModelVendor(ctx, *input.Vendor, nextPlatformModelName)
		if err != nil {
			return nil, err
		}
		update.Vendor = &nextVendor
	}
	if input.DisplayGroupID != nil {
		if err := s.validateModelDisplayGroup(ctx, *input.DisplayGroupID); err != nil {
			return nil, err
		}
		value := *input.DisplayGroupID
		update.DisplayGroupID = &value
	}
	if input.KindsJSON != nil {
		kindsJSON, err := normalizeKindsJSON(*input.KindsJSON)
		if err != nil {
			return nil, err
		}
		update.KindsJSON = &kindsJSON
	}
	if input.Icon != nil {
		explicitIcon, normalizeErr := normalizeModelPresentationIcon(*input.Icon)
		if normalizeErr != nil {
			return nil, normalizeErr
		}
		icon := normalizeModelIcon(explicitIcon, nextVendor, nextPlatformModelName)
		update.Icon = &icon
	}
	if input.CapabilitiesJSON != nil {
		normalized := strings.TrimSpace(*input.CapabilitiesJSON)
		if err := validateOptionalJSON(normalized); err != nil {
			return nil, ErrInvalidJSONConfig
		}
		if err := domainchannel.ValidateModelCapsOverrides(normalized); err != nil {
			return nil, ErrInvalidModelCapsConfig
		}
		update.CapabilitiesJSON = &normalized
	}
	if input.SystemPrompt != nil {
		systemPrompt := strings.TrimSpace(*input.SystemPrompt)
		if len([]rune(systemPrompt)) > maxSystemPromptChars {
			return nil, ErrSystemPromptTooLong
		}
		update.SystemPrompt = &systemPrompt
	}
	if input.AccessScope != nil {
		accessScope, err := normalizeModelAccessScope(*input.AccessScope)
		if err != nil {
			return nil, err
		}
		update.AccessScope = &accessScope
	}
	if input.Status != nil {
		status := normalizeStatus(*input.Status)
		update.Status = &status
	}
	if input.Description != nil {
		description := strings.TrimSpace(*input.Description)
		update.Description = &description
	}
	if input.CbPolicyMode != nil {
		value := normalizeModelCircuitPolicyMode(*input.CbPolicyMode)
		update.CbPolicyMode = &value
	}
	if input.CbFailureThreshold != nil {
		value := normalizeNonNegative(*input.CbFailureThreshold)
		update.CbFailureThreshold = &value
	}
	if input.CbDurationMin != nil {
		value := normalizeNonNegative(*input.CbDurationMin)
		update.CbDurationMin = &value
	}
	if input.CbWindowMin != nil {
		value := normalizeNonNegative(*input.CbWindowMin)
		update.CbWindowMin = &value
	}
	if input.Vendor == nil && input.PlatformModelName != nil {
		autoVendor, err := s.resolvePlatformModelVendor(ctx, "", nextPlatformModelName)
		if err != nil {
			return nil, err
		}
		if autoVendor != nextVendor {
			update.Vendor = &autoVendor
			nextVendor = autoVendor
		}
	}
	identityChanged := nextPlatformModelName != current.PlatformModelName || nextVendor != currentVendor
	if identityChanged && input.CapabilitiesJSON == nil {
		if capabilitiesJSON, changed := clearAutomaticContextWindow(current.CapabilitiesJSON); changed {
			update.CapabilitiesJSON = &capabilitiesJSON
		}
	}
	if input.Icon == nil && (input.PlatformModelName != nil || input.Vendor != nil) && shouldRefreshAutoIcon(current) {
		icon := normalizeModelIcon("", nextVendor, nextPlatformModelName)
		update.Icon = &icon
	}

	if update.IsZero() {
		return s.getModelViewByID(ctx, modelID)
	}
	if update.Icon != nil {
		if err = s.reserveModelIconReference(ctx, *update.Icon); err != nil {
			return nil, err
		}
	}

	if err := s.repo.UpdateModel(ctx, modelID, update); err != nil {
		switch {
		case errors.Is(err, repository.ErrModelVendorNotFound):
			return nil, ErrModelVendorNotFound
		case errors.Is(err, repository.ErrModelDisplayGroupNotFound):
			return nil, ErrModelDisplayGroupNotFound
		default:
			return nil, err
		}
	}
	s.InvalidateModelCatalog()
	return s.getModelViewByID(ctx, modelID)
}

// clearAutomaticContextWindow removes a catalog-derived context window after
// the model identity changes. Explicit administrator overrides do not carry
// the marker and are intentionally preserved.
func clearAutomaticContextWindow(raw string) (string, bool) {
	var payload map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &payload); err != nil || payload == nil {
		return raw, false
	}
	mode, ok := payload["_deeixContextWindowMode"].(string)
	if !ok || !strings.EqualFold(strings.TrimSpace(mode), "auto") {
		return raw, false
	}

	delete(payload, "_deeixContextWindowMode")
	delete(payload, "contextWindow")
	delete(payload, "context_window")
	delete(payload, "contextWindowTokens")
	delete(payload, "context_window_tokens")
	if len(payload) == 0 {
		return "", true
	}
	normalized, err := json.Marshal(payload)
	if err != nil {
		return raw, false
	}
	return string(normalized), true
}

func (s *Service) getModelViewByID(ctx context.Context, modelID uint) (*ModelView, error) {
	item, err := s.repo.GetModelListRowByID(ctx, modelID)
	if err != nil {
		return nil, err
	}
	view := s.toModelView(*item)
	views := []ModelView{view}
	if err := s.normalizeModelAvailability(ctx, views); err != nil {
		return nil, err
	}
	view = views[0]
	return &view, nil
}

// ReorderModels 按管理员指定顺序调整平台模型展示顺序。
func (s *Service) ReorderModels(ctx context.Context, modelIDs []uint) error {
	if len(modelIDs) == 0 {
		return ErrInvalidModelOrder
	}
	seen := make(map[uint]struct{}, len(modelIDs))
	normalized := make([]uint, 0, len(modelIDs))
	for _, modelID := range modelIDs {
		if modelID == 0 {
			return ErrInvalidModelOrder
		}
		if _, exists := seen[modelID]; exists {
			return ErrInvalidModelOrder
		}
		seen[modelID] = struct{}{}
		normalized = append(normalized, modelID)
	}
	if err := s.repo.ReorderModels(ctx, normalized); err != nil {
		if errors.Is(err, repository.ErrInvalidInput) {
			return ErrInvalidModelOrder
		}
		return err
	}
	s.InvalidateModelCatalog()
	return nil
}

// DeleteModel 硬删除平台模型目录项及其所有路由绑定。
func (s *Service) DeleteModel(ctx context.Context, modelID uint) error {
	if err := s.repo.DeleteModelCascade(ctx, modelID); err != nil {
		return err
	}
	s.InvalidateModelCatalog()
	return nil
}

// BatchDeleteModels 批量删除模型，逐项返回结果。
func (s *Service) BatchDeleteModels(ctx context.Context, modelIDs []uint) *BatchDeleteData {
	result := &BatchDeleteData{
		Total:   len(modelIDs),
		Results: make([]BatchDeleteResultView, 0, len(modelIDs)),
	}

	for _, modelID := range modelIDs {
		err := s.DeleteModel(ctx, modelID)
		switch {
		case err == nil:
			result.SuccessCount += 1
			result.Results = append(result.Results, BatchDeleteResultView{
				ID:     modelID,
				Status: BatchDeleteStatusDeleted,
			})
		case errors.Is(err, ErrModelNotFound):
			result.NotFoundCount += 1
			result.Results = append(result.Results, BatchDeleteResultView{
				ID:     modelID,
				Status: BatchDeleteStatusNotFound,
			})
		default:
			result.FailedCount += 1
			result.Results = append(result.Results, BatchDeleteResultView{
				ID:     modelID,
				Status: BatchDeleteStatusFailed,
				Error:  err.Error(),
			})
		}
	}

	return result
}

// ---------------------------------------------------------------------------
// 模型上游来源
// ---------------------------------------------------------------------------

// ListModelUpstreamSources 查询模型在各上游的路由来源。
func (s *Service) ListModelUpstreamSources(ctx context.Context, modelID uint, page int, pageSize int) ([]ModelUpstreamSourceView, int64, error) {
	modelItem, err := s.repo.GetModelByID(ctx, modelID)
	if err != nil {
		return nil, 0, err
	}
	offset, limit := normalizePage(page, pageSize)
	items, total, err := s.repo.ListModelUpstreamSources(ctx, modelItem.PlatformModelName, offset, limit)
	if err != nil {
		return nil, 0, err
	}
	views := make([]ModelUpstreamSourceView, 0, len(items))
	for _, item := range items {
		v := toModelUpstreamSourceView(item)
		s.applyModelSourceCircuitStatus(ctx, &v)
		views = append(views, v)
	}
	return views, total, nil
}

// BindModelUpstreamSource 将当前平台模型绑定到一个已存在的上游模型。
func (s *Service) BindModelUpstreamSource(ctx context.Context, modelID uint, input BindModelUpstreamSourceInput) (*ModelUpstreamSourceView, error) {
	modelItem, err := s.repo.GetModelByID(ctx, modelID)
	if err != nil {
		return nil, err
	}
	upstream, err := s.repo.GetUpstreamByID(ctx, input.UpstreamID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(upstream.Status) != "active" {
		return nil, ErrUpstreamSourceUnavailable
	}
	upstreamModel, err := s.repo.GetUpstreamModelByID(ctx, input.UpstreamModelID, input.UpstreamID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(upstreamModel.Status) != "active" {
		return nil, ErrUpstreamSourceUnavailable
	}

	protocolInput := strings.TrimSpace(input.Protocol)
	if protocolInput == "" {
		protocolInput = strings.TrimSpace(upstreamModel.SuggestedProtocol)
	}
	protocol, err := resolveRouteProtocol(protocolInput, upstream.Compatible, upstream.ProtocolDefaultsJSON, upstreamModel.KindsJSON)
	if err != nil {
		return nil, err
	}
	if err := s.validateRouteProtocolCombination(ctx, upstream.ID, modelItem.ID, upstreamModel.ID, 0, protocol); err != nil {
		return nil, err
	}

	route := &domainchannel.PlatformModelRoute{
		PlatformModelID:    modelItem.ID,
		UpstreamModelID:    upstreamModel.ID,
		Protocol:           protocol,
		Status:             normalizeStatus(input.Status),
		Priority:           normalizePriority(input.Priority),
		Weight:             normalizeWeight(input.Weight),
		Source:             "manual",
		CbFailureThreshold: normalizeNonNegative(input.CbFailureThreshold),
		CbDurationMin:      normalizeNonNegative(input.CbDurationMin),
		CbWindowMin:        normalizeNonNegative(input.CbWindowMin),
	}
	if err := s.repo.UpsertPlatformModelRoute(ctx, route); err != nil {
		if isDuplicateKeyError(err) {
			return nil, ErrUpstreamModelConflict
		}
		return nil, err
	}
	s.InvalidateModelCatalog()

	source, err := s.repo.GetModelUpstreamSourceByRouteID(ctx, modelItem.PlatformModelName, route.ID)
	if err != nil {
		return nil, err
	}
	view := toModelUpstreamSourceView(*source)
	s.applyModelSourceCircuitStatus(ctx, &view)
	return &view, nil
}

// UpdateModelUpstreamSource 更新模型上游来源配置。
func (s *Service) UpdateModelUpstreamSource(ctx context.Context, modelID uint, routeID uint, input UpdateModelUpstreamSourceInput) (*ModelUpstreamSourceView, error) {
	modelItem, err := s.repo.GetModelByID(ctx, modelID)
	if err != nil {
		return nil, err
	}
	source, err := s.repo.GetModelUpstreamSourceByRouteID(ctx, modelItem.PlatformModelName, routeID)
	if err != nil {
		return nil, err
	}

	updateInput := repository.UpdateChannelPlatformRouteInput{}
	if input.Protocol != nil {
		protocol, err := normalizeProtocol(*input.Protocol)
		if err != nil {
			return nil, err
		}
		if err := s.validateRouteProtocolCombination(ctx, source.UpstreamID, modelItem.ID, source.UpstreamModelID, routeID, protocol); err != nil {
			return nil, err
		}
		updateInput.Protocol = &protocol
	}
	if input.Status != nil {
		status := normalizeStatus(*input.Status)
		updateInput.Status = &status
	}
	if input.Priority != nil {
		priority := normalizePriority(*input.Priority)
		updateInput.Priority = &priority
	}
	if input.Weight != nil {
		weight := normalizeWeight(*input.Weight)
		updateInput.Weight = &weight
	}
	if input.CbFailureThreshold != nil {
		value := normalizeNonNegative(*input.CbFailureThreshold)
		updateInput.CbFailureThreshold = &value
	}
	if input.CbDurationMin != nil {
		value := normalizeNonNegative(*input.CbDurationMin)
		updateInput.CbDurationMin = &value
	}
	if input.CbWindowMin != nil {
		value := normalizeNonNegative(*input.CbWindowMin)
		updateInput.CbWindowMin = &value
	}

	if updateInput.IsZero() {
		view := toModelUpstreamSourceView(*source)
		s.applyModelSourceCircuitStatus(ctx, &view)
		return &view, nil
	}

	if err := s.repo.UpdatePlatformModelRouteByID(ctx, routeID, source.UpstreamID, updateInput); err != nil {
		if isDuplicateKeyError(err) {
			return nil, ErrUpstreamModelConflict
		}
		return nil, err
	}
	s.InvalidateModelCatalog()

	source, err = s.repo.GetModelUpstreamSourceByRouteID(ctx, modelItem.PlatformModelName, routeID)
	if err != nil {
		return nil, err
	}
	view := toModelUpstreamSourceView(*source)
	s.applyModelSourceCircuitStatus(ctx, &view)
	return &view, nil
}

func (s *Service) applyModelSourceCircuitStatus(ctx context.Context, view *ModelUpstreamSourceView) {
	if view == nil || s.cache == nil || !s.loadBreakerDefaults(ctx).Enabled {
		return
	}
	if upstreamOpen, upstreamUntil := s.cache.QueryUpstreamCircuitStatus(ctx, view.UpstreamID); upstreamOpen {
		view.CircuitOpen = true
		view.CircuitUntil = upstreamUntil
		view.CircuitScope = "upstream"
		return
	}
	if modelOpen, modelUntil := s.cache.QueryModelCircuitStatus(ctx, view.UpstreamID, bindingCircuitKey(view.BindingCode)); modelOpen {
		view.CircuitOpen = true
		view.CircuitUntil = modelUntil
		view.CircuitScope = "source"
		return
	}
	view.CircuitOpen = false
	view.CircuitUntil = ""
	view.CircuitScope = ""
}

// ---------------------------------------------------------------------------
// 全局设置
// ---------------------------------------------------------------------------

// ListLLMSettings 列出 LLM 全局设置。
func (s *Service) ListLLMSettings(ctx context.Context) ([]domainchannel.LLMSetting, error) {
	return s.repo.ListLLMSettings(ctx)
}

// UpdateLLMSetting 更新全局 LLM 设置项。
func (s *Service) UpdateLLMSetting(ctx context.Context, key string, value string) (*domainchannel.LLMSetting, error) {
	current, err := s.repo.GetLLMSetting(ctx, key)
	if err != nil {
		return nil, err
	}
	normalizedValue := strings.TrimSpace(value)
	if err := validateOptionalJSON(normalizedValue); err != nil {
		return nil, ErrInvalidJSONConfig
	}
	isBreakerDefaults := key == channelconfig.BreakerDefaultsKey
	currentBreakerDefaults := domainchannel.DefaultBreakerDefaults()
	nextBreakerDefaults := domainchannel.DefaultBreakerDefaults()
	if isBreakerDefaults {
		// 非法历史值按关闭处理，但不能阻止管理员用有效配置修复它。
		if parsed, parseErr := parseCircuitBreakerDefaults(current.Value); parseErr == nil {
			currentBreakerDefaults = parsed
		}
		nextBreakerDefaults, err = parseCircuitBreakerDefaults(normalizedValue)
		if err != nil {
			return nil, ErrInvalidJSONConfig
		}
		// 从关闭切换到开启前清理不会生效的历史状态。写入失败时熔断仍为关闭，
		// 清理失败时则中止开启，避免旧状态在新配置下立即生效。
		if !currentBreakerDefaults.Enabled && nextBreakerDefaults.Enabled && s.cache != nil {
			if err := s.cache.ResetAllCircuitStates(ctx); err != nil {
				return nil, err
			}
		}
	}
	current.Value = normalizedValue
	if err := s.repo.UpsertLLMSetting(ctx, current); err != nil {
		return nil, err
	}
	if isBreakerDefaults {
		s.storeBreakerDefaults(nextBreakerDefaults)
		// 关闭后不会再读取旧熔断状态，因此这里只做尽力清理，清理失败不回滚已持久化配置。
		if currentBreakerDefaults.Enabled && !nextBreakerDefaults.Enabled && s.cache != nil {
			if err := s.cache.ResetAllCircuitStates(ctx); err != nil {
				s.warn("reset_circuit_states_after_settings_update_failed", zap.Error(err))
			}
		}
	}
	return current, nil
}

func parseCircuitBreakerDefaults(value string) (domainchannel.BreakerDefaults, error) {
	defaults, err := channelconfig.ParseBreakerDefaults(value)
	if err != nil {
		return domainchannel.BreakerDefaults{}, ErrInvalidJSONConfig
	}
	return defaults, nil
}
