package channel

import (
	"context"
	"errors"
	"strings"
	"time"

	appbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/billing"
	domainchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/channel"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

// ---------------------------------------------------------------------------
// 模型管理
// ---------------------------------------------------------------------------

const maxSystemPromptChars = 20000

// ListModelsInput 定义模型列表筛选排序条件。
type ListModelsInput struct {
	Query    string
	Status   string
	Vendor   string
	Protocol string
	Sort     string
}

// ListModels 分页查询模型目录。
func (s *Service) ListModels(ctx context.Context, page int, pageSize int, onlyActive bool, input ListModelsInput) ([]ModelView, int64, error) {
	offset, limit := normalizePage(page, pageSize)
	items, total, err := s.repo.ListModels(ctx, repository.ListChannelModelsInput{
		Offset:     offset,
		Limit:      limit,
		OnlyActive: onlyActive,
		Query:      input.Query,
		Status:     input.Status,
		Vendor:     input.Vendor,
		Protocol:   input.Protocol,
		Sort:       input.Sort,
	})
	if err != nil {
		return nil, 0, err
	}
	views := make([]ModelView, 0, len(items))
	for _, item := range items {
		views = append(views, toModelView(item))
	}
	return views, total, nil
}

// ListActiveModels 查询全部启用模型目录（用于公开接口）。
func (s *Service) ListActiveModels(ctx context.Context) ([]ModelView, error) {
	now := time.Now()
	if s.modelPricingFilter == nil {
		items, err := s.listAllActiveModelRows(ctx)
		if err != nil {
			return nil, err
		}
		return filterRoutableModels(items), nil
	}
	mode, err := s.modelPricingFilter.GetBillingMode(ctx)
	if err != nil {
		return nil, err
	}
	if mode == "self" {
		items, err := s.listAllActiveModelRows(ctx)
		if err != nil {
			return nil, err
		}
		return filterRoutableModels(items), nil
	}

	s.modelCatalogMu.RLock()
	if s.modelCatalog != nil && now.Before(s.modelCatalogValidUntil) {
		result := cloneModelViews(s.modelCatalog)
		s.modelCatalogMu.RUnlock()
		return result, nil
	}
	s.modelCatalogMu.RUnlock()

	items, err := s.listAllActiveModelRows(ctx)
	if err != nil {
		return nil, err
	}
	views := filterRoutableModels(items)
	pricingByPlatformModelName, err := s.modelPricingFilter.ListPublicModelPricing(ctx)
	if err != nil {
		return nil, err
	}
	views = filterPricedModelViews(views, pricingByPlatformModelName)
	s.storeModelCatalog(now, views)
	return cloneModelViews(views), nil
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

func (s *Service) storeModelCatalog(now time.Time, views []ModelView) {
	if s == nil {
		return
	}
	s.modelCatalogMu.Lock()
	s.modelCatalog = cloneModelViews(views)
	s.modelCatalogValidUntil = now.Add(modelCatalogCacheTTL)
	s.modelCatalogMu.Unlock()
}

func cloneModelViews(items []ModelView) []ModelView {
	if len(items) == 0 {
		return []ModelView{}
	}
	results := make([]ModelView, 0, len(items))
	for _, item := range items {
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

// filterRoutableModels 过滤出有有效上游来源的可路由模型。
func filterRoutableModels(items []repository.ChannelModelListRow) []ModelView {
	results := make([]ModelView, 0, len(items))
	for _, item := range items {
		if item.ActiveSourceCount <= 0 {
			continue
		}
		results = append(results, toModelView(item))
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
	systemPrompt := strings.TrimSpace(input.SystemPrompt)
	if len([]rune(systemPrompt)) > maxSystemPromptChars {
		return nil, ErrSystemPromptTooLong
	}

	item := &domainchannel.PlatformModel{
		PlatformModelName: platformModelName,
		Vendor:            normalizeModelVendor(input.Vendor, platformModelName),
		KindsJSON:         kindsJSON,
		Icon:              normalizeModelIcon(input.Icon, input.Vendor, platformModelName),
		CapabilitiesJSON:  strings.TrimSpace(input.CapabilitiesJSON),
		SystemPrompt:      systemPrompt,
		Status:            normalizeStatus(input.Status),
		Description:       strings.TrimSpace(input.Description),
	}
	if err := s.repo.CreateModel(ctx, item); err != nil {
		if isDuplicateKeyError(err) {
			return nil, ErrDuplicatePlatformModelName
		}
		return nil, err
	}
	s.InvalidateModelCatalog()
	view := toModelView(repository.ChannelModelListRow{PlatformModel: *item})
	return &view, nil
}

// UpdateModel 更新平台模型目录项。
func (s *Service) UpdateModel(ctx context.Context, modelID uint, input UpdateModelInput) (*ModelView, error) {
	current, err := s.repo.GetModelByID(ctx, modelID)
	if err != nil {
		return nil, err
	}

	nextVendor := normalizeModelVendor(current.Vendor, current.PlatformModelName)
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
		nextVendor = normalizeModelVendor(*input.Vendor, nextPlatformModelName)
		update.Vendor = &nextVendor
	}
	if input.KindsJSON != nil {
		kindsJSON, err := normalizeKindsJSON(*input.KindsJSON)
		if err != nil {
			return nil, err
		}
		update.KindsJSON = &kindsJSON
	}
	if input.Icon != nil {
		icon := normalizeModelIcon(*input.Icon, nextVendor, nextPlatformModelName)
		update.Icon = &icon
	}
	if input.CapabilitiesJSON != nil {
		normalized := strings.TrimSpace(*input.CapabilitiesJSON)
		if err := validateOptionalJSON(normalized); err != nil {
			return nil, ErrInvalidJSONConfig
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
	if input.Status != nil {
		status := normalizeStatus(*input.Status)
		update.Status = &status
	}
	if input.Description != nil {
		description := strings.TrimSpace(*input.Description)
		update.Description = &description
	}
	if input.Vendor == nil && input.PlatformModelName != nil {
		autoVendor := normalizeModelVendor("", nextPlatformModelName)
		if autoVendor != nextVendor {
			update.Vendor = &autoVendor
			nextVendor = autoVendor
		}
	}
	if input.Icon == nil && (input.PlatformModelName != nil || input.Vendor != nil) && shouldRefreshAutoIcon(current) {
		icon := normalizeModelIcon("", nextVendor, nextPlatformModelName)
		update.Icon = &icon
	}

	if update.IsZero() {
		return s.getModelViewByID(ctx, modelID)
	}

	if err := s.repo.UpdateModel(ctx, modelID, update); err != nil {
		return nil, err
	}
	s.InvalidateModelCatalog()
	return s.getModelViewByID(ctx, modelID)
}

func (s *Service) getModelViewByID(ctx context.Context, modelID uint) (*ModelView, error) {
	item, err := s.repo.GetModelListRowByID(ctx, modelID)
	if err != nil {
		return nil, err
	}
	view := toModelView(*item)
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
		v.CircuitOpen, v.CircuitUntil = s.cache.QueryModelCircuitStatus(ctx, item.UpstreamID, bindingCircuitKey(item.BindingCode))
		views = append(views, v)
	}
	return views, total, nil
}

// UpdateModelUpstreamSource 更新模型上游来源配置。
func (s *Service) UpdateModelUpstreamSource(ctx context.Context, modelID uint, routeID uint, input UpdateModelUpstreamSourceInput) (*ModelUpstreamSourceView, error) {
	modelItem, err := s.repo.GetModelByID(ctx, modelID)
	if err != nil {
		return nil, err
	}

	updateInput := repository.UpdateChannelPlatformRouteInput{}
	if input.Protocol != nil {
		protocol := strings.TrimSpace(*input.Protocol)
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

	allSources, _, listErr := s.repo.ListModelUpstreamSources(ctx, modelItem.PlatformModelName, 0, 2000)
	if listErr != nil {
		return nil, listErr
	}
	var targetUpstreamID uint
	for _, src := range allSources {
		if src.ID == routeID {
			targetUpstreamID = src.UpstreamID
			break
		}
	}
	if targetUpstreamID == 0 {
		return nil, ErrUpstreamModelNotFound
	}

	if updateInput.IsZero() {
		for _, src := range allSources {
			if src.ID == routeID {
				view := toModelUpstreamSourceView(src)
				return &view, nil
			}
		}
		return nil, ErrUpstreamModelNotFound
	}

	if err := s.repo.UpdatePlatformModelRouteByID(ctx, routeID, targetUpstreamID, updateInput); err != nil {
		return nil, err
	}
	s.InvalidateModelCatalog()

	allSources, _, err = s.repo.ListModelUpstreamSources(ctx, modelItem.PlatformModelName, 0, 500)
	if err != nil {
		return nil, err
	}
	for _, src := range allSources {
		if src.ID == routeID {
			view := toModelUpstreamSourceView(src)
			return &view, nil
		}
	}
	return nil, ErrUpstreamModelNotFound
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
	if err := validateOptionalJSON(strings.TrimSpace(value)); err != nil {
		return nil, ErrInvalidJSONConfig
	}
	current.Value = strings.TrimSpace(value)
	if err := s.repo.UpsertLLMSetting(ctx, current); err != nil {
		return nil, err
	}
	return current, nil
}
