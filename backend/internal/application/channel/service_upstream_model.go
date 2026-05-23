package channel

import (
	"context"
	"errors"
	"strings"

	domainchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/channel"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
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

// ListUpstreamModels 分页查询上游真实模型及路由绑定。
func (s *Service) ListUpstreamModels(ctx context.Context, upstreamID uint, page int, pageSize int, input ListUpstreamModelsInput) ([]UpstreamModelView, int64, error) {
	if _, err := s.repo.GetUpstreamByID(ctx, upstreamID); err != nil {
		return nil, 0, err
	}
	offset, limit := normalizePage(page, pageSize)
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
	views := make([]UpstreamModelView, 0, len(items))
	for _, item := range items {
		v := toUpstreamModelView(item)
		v.CircuitOpen, v.CircuitUntil = s.cache.QueryModelCircuitStatus(ctx, upstreamID, bindingCircuitKey(item.BindingCode))
		views = append(views, v)
	}
	return views, total, nil
}

// UpsertUpstreamModel 新增或更新平台模型到上游真实模型的路由绑定。
func (s *Service) UpsertUpstreamModel(ctx context.Context, upstreamID uint, input UpsertUpstreamModelInput) (*UpstreamModelView, error) {
	upstream, err := s.repo.GetUpstreamByID(ctx, upstreamID)
	if err != nil {
		return nil, err
	}

	platformModelName, err := normalizePlatformModelName(input.PlatformModelName)
	if err != nil {
		return nil, err
	}
	upstreamModelName := strings.TrimSpace(input.UpstreamModelName)
	if upstreamModelName == "" {
		return nil, ErrUpstreamModelNotFound
	}
	if err := validateOptionalJSON(strings.TrimSpace(input.HeadersJSON)); err != nil {
		return nil, ErrInvalidJSONConfig
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
	protocol, err := resolveRouteProtocol(input.Protocol, upstream.Compatible, upstream.ProtocolDefaultsJSON, kindsJSON)
	if err != nil {
		return nil, err
	}

	platformModel, platformModelCreated, err := s.ensurePlatformModel(ctx, platformModelName, kindsJSON, upstreamModelName)
	if err != nil {
		return nil, err
	}
	if !platformModelCreated && kindsExplicit && strings.TrimSpace(platformModel.KindsJSON) != kindsJSON {
		if err := s.repo.UpdateModel(ctx, platformModel.ID, repository.UpdateChannelModelInput{KindsJSON: &kindsJSON}); err != nil {
			return nil, err
		}
		platformModel.KindsJSON = kindsJSON
	}
	upstreamModelVendor := normalizeUpstreamModelVendor("", upstreamModelName, upstream.Name, upstream.BaseURL)
	upstreamModelIcon := normalizeModelIcon("", upstreamModelVendor, upstreamModelName)
	upstreamModel, err := s.upsertUpstreamCatalogModel(ctx, upstream.ID, upstreamModelName, protocol, kindsJSON, upstreamModelVendor, upstreamModelIcon, "active", normalizeSource(input.Source), "{}")
	if err != nil {
		return nil, err
	}
	if err := s.validateRouteProtocolCombination(ctx, upstream.ID, platformModel.ID, upstreamModel.ID, input.RouteID, protocol); err != nil {
		return nil, err
	}

	route := &domainchannel.PlatformModelRoute{
		PlatformModelID:    platformModel.ID,
		UpstreamModelID:    upstreamModel.ID,
		Protocol:           protocol,
		Status:             normalizeStatus(input.Status),
		Priority:           normalizePriority(input.Priority),
		Weight:             normalizeWeight(input.Weight),
		Source:             normalizeSource(input.Source),
		CbFailureThreshold: input.CbFailureThreshold,
		CbDurationMin:      input.CbDurationMin,
		CbWindowMin:        input.CbWindowMin,
		HeadersJSON:        strings.TrimSpace(input.HeadersJSON),
	}

	if input.RouteID > 0 {
		if _, err := s.repo.GetPlatformModelRouteByID(ctx, input.RouteID, upstream.ID); err != nil {
			return nil, err
		}
		if err := s.repo.UpdatePlatformModelRouteByID(ctx, input.RouteID, upstream.ID, repository.UpdateChannelPlatformRouteInput{
			PlatformModelID:    &route.PlatformModelID,
			UpstreamModelID:    &route.UpstreamModelID,
			Protocol:           &route.Protocol,
			Status:             &route.Status,
			Priority:           &route.Priority,
			Weight:             &route.Weight,
			Source:             &route.Source,
			CbFailureThreshold: &route.CbFailureThreshold,
			CbDurationMin:      &route.CbDurationMin,
			CbWindowMin:        &route.CbWindowMin,
			HeadersJSON:        &route.HeadersJSON,
		}); err != nil {
			if isDuplicateKeyError(err) {
				return nil, ErrUpstreamModelConflict
			}
			return nil, err
		}
		route.ID = input.RouteID
	} else if err := s.repo.UpsertPlatformModelRoute(ctx, route); err != nil {
		if isDuplicateKeyError(err) {
			return nil, ErrUpstreamModelConflict
		}
		return nil, err
	}

	s.InvalidateModelCatalog()
	return s.findUpstreamModelViewByRoute(ctx, upstream.ID, route.ID, upstreamModel.ID)
}

func (s *Service) validateRouteProtocolCombination(
	ctx context.Context,
	upstreamID uint,
	platformModelID uint,
	upstreamModelID uint,
	routeID uint,
	protocol string,
) error {
	// 同一个平台模型到同一个上游真实模型只允许单协议，或 OpenAI 图片生成/编辑这一个成对组合。
	routes, err := s.repo.ListPlatformModelRoutesByPair(ctx, upstreamID, platformModelID, upstreamModelID)
	if err != nil {
		return err
	}
	protocols := make([]string, 0, len(routes)+1)
	for _, route := range routes {
		if route.ID == routeID {
			continue
		}
		protocols = append(protocols, route.Protocol)
	}
	protocols = append(protocols, protocol)
	if !isSupportedRouteProtocolCombination(protocols) {
		return ErrInvalidRouteProtocolCombination
	}
	return nil
}

func (s *Service) ensurePlatformModel(ctx context.Context, platformModelName string, kindsJSON string, candidates ...string) (*domainchannel.PlatformModel, bool, error) {
	if item, err := s.repo.GetModelByName(ctx, platformModelName); err == nil {
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
	if err := s.repo.CreateModel(ctx, item); err != nil {
		if !isDuplicateKeyError(err) {
			return nil, false, err
		}
		item, err = s.repo.GetModelByName(ctx, platformModelName)
		if err != nil {
			return nil, false, err
		}
		return item, false, nil
	}
	return item, true, nil
}

func (s *Service) upsertUpstreamCatalogModel(
	ctx context.Context,
	upstreamID uint,
	upstreamModelName string,
	suggestedProtocol string,
	kindsJSON string,
	vendor string,
	icon string,
	status string,
	source string,
	rawJSON string,
) (*domainchannel.UpstreamModel, error) {
	bindingCode := generateBindingCode()
	normalizedSource := normalizeSource(source)
	if existing, err := s.repo.GetUpstreamModelByUpstreamName(ctx, upstreamID, upstreamModelName); err == nil {
		bindingCode = existing.BindingCode
		if normalizedSource != "sync" && strings.TrimSpace(existing.Source) != "" {
			normalizedSource = normalizeSource(existing.Source)
		}
		if strings.TrimSpace(vendor) == "" {
			vendor = existing.Vendor
		}
		if strings.TrimSpace(icon) == "" {
			icon = existing.Icon
		}
		if strings.TrimSpace(rawJSON) == "" || strings.TrimSpace(rawJSON) == "{}" {
			rawJSON = existing.RawJSON
		}
	} else if !errors.Is(err, ErrUpstreamModelNotFound) {
		return nil, err
	}

	if strings.TrimSpace(rawJSON) == "" {
		rawJSON = "{}"
	}
	item := &domainchannel.UpstreamModel{
		UpstreamID:        upstreamID,
		BindingCode:       bindingCode,
		UpstreamModelName: upstreamModelName,
		SuggestedProtocol: suggestedProtocol,
		KindsJSON:         kindsJSON,
		Status:            normalizeStatus(status),
		Source:            normalizedSource,
		RawJSON:           strings.TrimSpace(rawJSON),
	}
	item.Vendor = normalizeUpstreamModelVendor(vendor, upstreamModelName)
	item.Icon = normalizeModelIcon(icon, item.Vendor, upstreamModelName)
	if err := s.repo.UpsertUpstreamModel(ctx, item); err != nil {
		if isDuplicateKeyError(err) {
			return nil, ErrUpstreamModelConflict
		}
		return nil, err
	}
	return item, nil
}

func (s *Service) findUpstreamModelViewByRoute(ctx context.Context, upstreamID uint, routeID uint, upstreamModelID uint) (*UpstreamModelView, error) {
	row, err := s.repo.GetUpstreamModelRouteByID(ctx, upstreamID, routeID)
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
			result.Results = append(result.Results, BatchDeleteResultView{ID: routeID, Status: BatchDeleteStatusFailed, Error: err.Error()})
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
	return s.cache.OpenModelCircuit(ctx, upstreamID, bindingCircuitKey(bindingCode))
}

// ResetUpstreamModelCircuit 重置上游模型级熔断。
func (s *Service) ResetUpstreamModelCircuit(ctx context.Context, upstreamID uint, routeID uint) error {
	bindingCode, err := s.routeBindingCode(ctx, upstreamID, routeID)
	if err != nil {
		return err
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
