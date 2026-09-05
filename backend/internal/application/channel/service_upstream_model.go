package channel

import (
	"context"
	"errors"
	"sort"
	"strings"

	domainchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/channel"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/apperr"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/pagination"
)

// ---------------------------------------------------------------------------
// 上游真实模型与平台路由
// ---------------------------------------------------------------------------

// ListUpstreamModelsInput 定义上游模型路由绑定列表筛选排序条件。
type ListUpstreamModelsInput struct {
	Query          string
	RouteStatus    string
	UpstreamStatus string
	Protocol       string
	Sort           string
}

type ensureUpstreamCatalogModelInput struct {
	UpstreamID        uint
	UpstreamModelName string
	SuggestedProtocol string
	KindsJSON         string
	Vendor            string
	Icon              string
	Source            string
}

// ListUpstreamModels 分页查询上游真实模型及路由绑定。
func (s *Service) ListUpstreamModels(ctx context.Context, upstreamID uint, page int, pageSize int, input ListUpstreamModelsInput) ([]UpstreamModelView, int64, error) {
	if _, err := s.repo.GetUpstreamByID(ctx, upstreamID); err != nil {
		return nil, 0, err
	}
	offset, limit := pagination.Offset(page, pageSize)
	items, total, err := s.repo.ListUpstreamModels(ctx, upstreamID, repository.ListChannelUpstreamModelsInput{
		Offset:         offset,
		Limit:          limit,
		Query:          input.Query,
		RouteStatus:    input.RouteStatus,
		UpstreamStatus: input.UpstreamStatus,
		Protocol:       input.Protocol,
		Sort:           input.Sort,
	})
	if err != nil {
		return nil, 0, err
	}
	breakerEnabled := s.cache != nil && s.loadBreakerDefaults(ctx).Enabled
	views := make([]UpstreamModelView, 0, len(items))
	for _, item := range items {
		v := toUpstreamModelView(item)
		if breakerEnabled && s.cache != nil {
			v.CircuitOpen, v.CircuitUntil = s.cache.QueryModelCircuitStatus(ctx, upstreamID, bindingCircuitKey(item.BindingCode))
		}
		views = append(views, v)
	}
	return views, total, nil
}

// UpsertUpstreamModel 新增或更新平台模型到上游真实模型的路由绑定。
func (s *Service) UpsertUpstreamModel(ctx context.Context, upstreamID uint, input UpsertUpstreamModelInput) (*UpstreamModelView, error) {
	platformModelName, err := normalizePlatformModelName(input.PlatformModelName)
	if err != nil {
		return nil, err
	}
	upstreamModelName := strings.TrimSpace(input.UpstreamModelName)
	if upstreamModelName == "" {
		return nil, ErrUpstreamModelNotFound
	}
	if input.HeadersJSON != nil {
		if err := validateOptionalJSON(strings.TrimSpace(*input.HeadersJSON)); err != nil {
			return nil, ErrInvalidJSONConfig
		}
	}

	rawKindsJSON := strings.TrimSpace(input.KindsJSON)
	kindsExplicit := rawKindsJSON != ""
	kindsJSON := rawKindsJSON
	if kindsJSON == "" {
		kindsJSON = inferKindsJSON(platformModelName)
	}
	kindsJSON, err = normalizeKindsJSON(kindsJSON)
	if err != nil {
		return nil, err
	}
	var view *UpstreamModelView
	err = s.repo.WithinTransaction(ctx, func(txRepo repository.ChannelRepository) error {
		upstream, txErr := txRepo.GetUpstreamByID(ctx, upstreamID)
		if txErr != nil {
			return txErr
		}
		protocols, txErr := resolveRouteProtocols(input.Protocols, upstream.Compatible, upstream.ProtocolDefaultsJSON, kindsJSON)
		if txErr != nil {
			return txErr
		}

		platformModel, platformModelCreated, txErr := ensurePlatformModel(ctx, txRepo, platformModelName, kindsJSON, upstreamModelName)
		if txErr != nil {
			return txErr
		}
		if !platformModelCreated && kindsExplicit && strings.TrimSpace(platformModel.KindsJSON) != kindsJSON {
			if txErr := txRepo.UpdateModel(ctx, platformModel.ID, repository.UpdateChannelModelInput{KindsJSON: &kindsJSON}); txErr != nil {
				return txErr
			}
			platformModel.KindsJSON = kindsJSON
		}

		upstreamModelVendor := normalizeModelVendor("", upstreamModelName, upstream.Name, upstream.BaseURL)
		upstreamModelIcon := normalizeModelIcon("", upstreamModelVendor, upstreamModelName)
		upstreamModelSource := "manual"
		if input.CatalogSource != nil {
			upstreamModelSource = normalizeSource(*input.CatalogSource)
		} else if input.Source != nil {
			upstreamModelSource = normalizeSource(*input.Source)
		}
		upstreamModel, txErr := ensureUpstreamCatalogModel(ctx, txRepo, ensureUpstreamCatalogModelInput{
			UpstreamID:        upstream.ID,
			UpstreamModelName: upstreamModelName,
			SuggestedProtocol: protocols[0],
			KindsJSON:         kindsJSON,
			Vendor:            upstreamModelVendor,
			Icon:              upstreamModelIcon,
			Source:            upstreamModelSource,
		})
		if txErr != nil {
			return txErr
		}

		existingRoutes, txErr := bindingRoutesForReplacement(
			ctx,
			txRepo,
			upstream.ID,
			platformModel.ID,
			upstreamModel.ID,
			input.RouteIDs,
		)
		if txErr != nil {
			return txErr
		}
		existingRouteIDs := append([]uint(nil), input.RouteIDs...)
		if len(existingRouteIDs) == 0 {
			existingRouteIDs = make([]uint, 0, len(existingRoutes))
			for _, route := range existingRoutes {
				existingRouteIDs = append(existingRouteIDs, route.ID)
			}
		}

		routes := make([]domainchannel.PlatformModelRoute, 0, len(protocols))
		for _, desiredProtocol := range protocols {
			route := replacementRouteTemplate(existingRoutes, desiredProtocol)
			route.PlatformModelID = platformModel.ID
			route.UpstreamModelID = upstreamModel.ID
			route.Protocol = desiredProtocol
			applyRouteOverrides(&route, input)
			routes = append(routes, route)
		}
		replaced, txErr := txRepo.ReplacePlatformModelRoutes(ctx, []repository.ReplaceChannelPlatformRoutesInput{{
			UpstreamID:       upstream.ID,
			ExistingRouteIDs: existingRouteIDs,
			Routes:           routes,
		}})
		if txErr != nil {
			return txErr
		}
		view, txErr = findUpstreamModelViewByRoute(ctx, txRepo, upstream.ID, replaced[0].ID, upstreamModel.ID)
		return txErr
	})
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrDuplicate):
			return nil, ErrUpstreamModelConflict
		case errors.Is(err, repository.ErrConflict):
			return nil, ErrUpstreamModelBindingChanged
		default:
			return nil, err
		}
	}

	s.InvalidateModelCatalog()
	return view, nil
}

func bindingRoutesForReplacement(
	ctx context.Context,
	repo repository.ChannelRepository,
	upstreamID uint,
	platformModelID uint,
	upstreamModelID uint,
	existingRouteIDs []uint,
) ([]domainchannel.PlatformModelRoute, error) {
	if len(existingRouteIDs) == 0 {
		return repo.ListPlatformModelRoutesByPair(ctx, upstreamID, platformModelID, upstreamModelID)
	}
	routes := make([]domainchannel.PlatformModelRoute, 0, len(existingRouteIDs))
	for _, routeID := range existingRouteIDs {
		route, err := repo.GetPlatformModelRouteByID(ctx, routeID, upstreamID)
		if err != nil {
			return nil, err
		}
		routes = append(routes, *route)
	}
	sort.Slice(routes, func(i int, j int) bool { return routes[i].ID < routes[j].ID })
	return routes, nil
}

func (s *Service) validateRouteProtocolCombination(
	ctx context.Context,
	upstreamID uint,
	platformModelID uint,
	upstreamModelID uint,
	routeID uint,
	protocol string,
) error {
	routes, err := s.repo.ListPlatformModelRoutesByPair(ctx, upstreamID, platformModelID, upstreamModelID)
	if err != nil {
		return err
	}
	protocols := make([]string, 0, len(routes)+1)
	for _, route := range routes {
		if route.ID != routeID {
			protocols = append(protocols, route.Protocol)
		}
	}
	protocols = append(protocols, protocol)
	if !isSupportedRouteProtocolCombination(protocols) {
		return ErrInvalidRouteProtocolCombination
	}
	return nil
}

func replacementRouteTemplate(existingRoutes []domainchannel.PlatformModelRoute, protocol string) domainchannel.PlatformModelRoute {
	for _, route := range existingRoutes {
		if route.Protocol == protocol {
			return route
		}
	}
	if len(existingRoutes) > 0 {
		return existingRoutes[0]
	}
	return domainchannel.PlatformModelRoute{
		Status:   "active",
		Priority: 1,
		Weight:   1,
		Source:   "manual",
	}
}

func applyRouteOverrides(route *domainchannel.PlatformModelRoute, input UpsertUpstreamModelInput) {
	if input.Status != nil {
		route.Status = normalizeStatus(*input.Status)
	}
	if input.Priority != nil {
		route.Priority = normalizePriority(*input.Priority)
	}
	if input.Weight != nil {
		route.Weight = normalizeWeight(*input.Weight)
	}
	if input.Source != nil {
		route.Source = normalizeSource(*input.Source)
	}
	if input.CbFailureThreshold != nil {
		route.CbFailureThreshold = normalizeNonNegative(*input.CbFailureThreshold)
	}
	if input.CbDurationMin != nil {
		route.CbDurationMin = normalizeNonNegative(*input.CbDurationMin)
	}
	if input.CbWindowMin != nil {
		route.CbWindowMin = normalizeNonNegative(*input.CbWindowMin)
	}
	if input.HeadersJSON != nil {
		route.HeadersJSON = strings.TrimSpace(*input.HeadersJSON)
	}
}

func ensurePlatformModel(ctx context.Context, repo repository.ChannelRepository, platformModelName string, kindsJSON string, candidates ...string) (*domainchannel.PlatformModel, bool, error) {
	if item, err := repo.GetModelByName(ctx, platformModelName); err == nil {
		return item, false, nil
	} else if !errors.Is(err, ErrModelNotFound) {
		return nil, false, err
	}

	item := &domainchannel.PlatformModel{
		PlatformModelName: platformModelName,
		Vendor:            normalizeModelVendor("", platformModelName, strings.Join(candidates, " ")),
		KindsJSON:         kindsJSON,
		Icon:              normalizeModelIcon("", "", platformModelName, strings.Join(candidates, " ")),
		CapabilitiesJSON:  "{}",
		Status:            "active",
		Description:       "",
	}
	if err := repo.CreateModel(ctx, item); err != nil {
		if !errors.Is(err, repository.ErrDuplicate) {
			return nil, false, err
		}
		item, err = repo.GetModelByName(ctx, platformModelName)
		if err != nil {
			return nil, false, err
		}
		return item, false, nil
	}
	return item, true, nil
}

func ensureUpstreamCatalogModel(ctx context.Context, repo repository.ChannelRepository, input ensureUpstreamCatalogModelInput) (*domainchannel.UpstreamModel, error) {
	if existing, err := repo.GetUpstreamModelByUpstreamName(ctx, input.UpstreamID, input.UpstreamModelName); err == nil {
		return existing, nil
	} else if !errors.Is(err, ErrUpstreamModelNotFound) {
		return nil, err
	}

	item := &domainchannel.UpstreamModel{
		UpstreamID:        input.UpstreamID,
		BindingCode:       generateBindingCode(),
		UpstreamModelName: input.UpstreamModelName,
		SuggestedProtocol: input.SuggestedProtocol,
		KindsJSON:         input.KindsJSON,
		Status:            "active",
		Source:            normalizeSource(input.Source),
		RawJSON:           "{}",
	}
	item.Vendor = normalizeModelVendor(input.Vendor, input.UpstreamModelName)
	item.Icon = normalizeModelIcon(input.Icon, item.Vendor, input.UpstreamModelName)
	if err := repo.CreateUpstreamModel(ctx, item); err != nil {
		if errors.Is(err, repository.ErrDuplicate) {
			return repo.GetUpstreamModelByUpstreamName(ctx, input.UpstreamID, input.UpstreamModelName)
		}
		return nil, err
	}
	return item, nil
}

func findUpstreamModelViewByRoute(ctx context.Context, repo repository.ChannelRepository, upstreamID uint, routeID uint, upstreamModelID uint) (*UpstreamModelView, error) {
	row, err := repo.GetUpstreamModelRouteByID(ctx, upstreamID, routeID)
	if err != nil {
		return nil, err
	}
	if upstreamModelID > 0 && row.ID != upstreamModelID {
		return nil, ErrUpstreamModelNotFound
	}
	view := toUpstreamModelView(*row)
	return &view, nil
}

// DeleteUpstreamModel 删除平台路由绑定，保留上游真实模型清单。
func (s *Service) DeleteUpstreamModel(ctx context.Context, upstreamID uint, routeID uint) error {
	if err := s.repo.DeletePlatformModelRoute(ctx, routeID, upstreamID); err != nil {
		return err
	}
	s.InvalidateModelCatalog()
	return nil
}

// DisableUpstreamModel 停用平台路由。
func (s *Service) DisableUpstreamModel(ctx context.Context, upstreamID uint, routeID uint) error {
	status := "inactive"
	if err := s.repo.UpdatePlatformModelRouteByID(ctx, routeID, upstreamID, repository.UpdateChannelPlatformRouteInput{Status: &status}); err != nil {
		return err
	}
	s.InvalidateModelCatalog()
	return nil
}

// EnableUpstreamModel 启用平台路由。
func (s *Service) EnableUpstreamModel(ctx context.Context, upstreamID uint, routeID uint) error {
	status := "active"
	if err := s.repo.UpdatePlatformModelRouteByID(ctx, routeID, upstreamID, repository.UpdateChannelPlatformRouteInput{Status: &status}); err != nil {
		return err
	}
	s.InvalidateModelCatalog()
	return nil
}

// BatchDeleteUpstreamModels 批量删除平台路由，逐项返回结果。
func (s *Service) BatchDeleteUpstreamModels(ctx context.Context, upstreamID uint, routeIDs []uint) *BatchDeleteData {
	result := &BatchDeleteData{
		Total:   len(routeIDs),
		Results: make([]BatchDeleteResultView, 0, len(routeIDs)),
	}

	for _, routeID := range routeIDs {
		err := s.DeleteUpstreamModel(ctx, upstreamID, routeID)
		switch {
		case err == nil:
			result.SuccessCount += 1
			result.Results = append(result.Results, BatchDeleteResultView{ID: routeID, Status: BatchDeleteStatusDeleted})
		case errors.Is(err, ErrUpstreamModelNotFound):
			result.NotFoundCount += 1
			result.Results = append(result.Results, BatchDeleteResultView{ID: routeID, Status: BatchDeleteStatusNotFound})
		default:
			result.FailedCount += 1
			result.Results = append(result.Results, BatchDeleteResultView{ID: routeID, Status: BatchDeleteStatusFailed, Error: apperr.MessageOr(err, "batch delete failed")})
		}
	}

	return result
}

// OpenUpstreamModelCircuit 手动打开上游模型级熔断。
func (s *Service) OpenUpstreamModelCircuit(ctx context.Context, upstreamID uint, routeID uint) error {
	bindingCode, err := s.routeBindingCode(ctx, upstreamID, routeID)
	if err != nil {
		return err
	}
	if s.cache == nil || !s.loadBreakerDefaults(ctx).Enabled {
		return ErrCircuitBreakerDisabled
	}
	return s.cache.OpenModelCircuit(ctx, upstreamID, bindingCircuitKey(bindingCode))
}

// ResetUpstreamModelCircuit 重置上游模型级熔断。
func (s *Service) ResetUpstreamModelCircuit(ctx context.Context, upstreamID uint, routeID uint) error {
	bindingCode, err := s.routeBindingCode(ctx, upstreamID, routeID)
	if err != nil {
		return err
	}
	if s.cache == nil {
		return nil
	}
	return s.cache.ResetModelCircuit(ctx, upstreamID, bindingCircuitKey(bindingCode))
}

func (s *Service) routeBindingCode(ctx context.Context, upstreamID uint, routeID uint) (string, error) {
	row, err := s.repo.GetUpstreamModelRouteByID(ctx, upstreamID, routeID)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(row.BindingCode) == "" {
		return "", ErrUpstreamModelNotFound
	}
	return row.BindingCode, nil
}
