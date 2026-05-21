package channel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	domainchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/channel"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/llm"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"go.uber.org/zap"
)

// ---------------------------------------------------------------------------
// 远端模型发现
// ---------------------------------------------------------------------------

// ListRemoteModels 预览上游远程模型列表（不落库）。
func (s *Service) ListRemoteModels(ctx context.Context, upstreamID uint) (*UpstreamRemoteModelsData, error) {
	upstreamItem, err := s.repo.GetUpstreamByID(ctx, upstreamID)
	if err != nil {
		if errors.Is(err, ErrUpstreamNotFound) {
			return nil, ErrUpstreamNotFound
		}
		return nil, err
	}

	items, err := s.fetchRemoteModels(ctx, upstreamItem)
	if err != nil {
		return nil, err
	}

	rows, _, _ := s.repo.ListUpstreamModels(ctx, upstreamID, repository.ListChannelUpstreamModelsInput{Offset: 0, Limit: 5000})
	existingByName := make(map[string]repositoryUpstreamModelSnapshot, len(rows))
	for _, row := range rows {
		name := strings.TrimSpace(row.UpstreamModelName)
		if name == "" {
			continue
		}
		snapshot := existingByName[name]
		snapshot.BindingCode = row.BindingCode
		snapshot.Status = row.Status
		if platformName := strings.TrimSpace(row.PlatformModelName); platformName != "" {
			snapshot.BoundPlatformModels = appendUniqueString(snapshot.BoundPlatformModels, platformName)
		}
		existingByName[name] = snapshot
	}

	views := make([]UpstreamRemoteModelView, 0, len(items))
	for _, item := range items {
		name := strings.TrimSpace(item.ID)
		if name == "" {
			continue
		}
		kindsJSON := inferKindsJSON(name)
		suggestedProtocol, _ := resolveRouteProtocol("", upstreamItem.Compatible, upstreamItem.ProtocolDefaultsJSON, kindsJSON)
		snapshot, alreadySynced := existingByName[name]
		views = append(views, UpstreamRemoteModelView{
			UpstreamModelName:          name,
			SuggestedPlatformModelName: name,
			SuggestedKindsJSON:         kindsJSON,
			SuggestedProtocol:          suggestedProtocol,
			BindingCode:                snapshot.BindingCode,
			BoundPlatformModels:        snapshot.BoundPlatformModels,
			UpstreamModelStatus:        snapshot.Status,
			AlreadySynced:              alreadySynced,
			AlreadyBound:               len(snapshot.BoundPlatformModels) > 0,
		})
	}

	return &UpstreamRemoteModelsData{Total: len(views), Items: views}, nil
}

type repositoryUpstreamModelSnapshot struct {
	BindingCode         string
	Status              string
	BoundPlatformModels []string
}

func appendUniqueString(items []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return items
	}
	for _, item := range items {
		if item == value {
			return items
		}
	}
	return append(items, value)
}

// SyncUpstreamModels 拉取上游 models 并写入上游真实模型清单。
func (s *Service) SyncUpstreamModels(ctx context.Context, upstreamID uint) (*SyncUpstreamModelsData, error) {
	upstreamItem, err := s.repo.GetUpstreamByID(ctx, upstreamID)
	if err != nil {
		if errors.Is(err, ErrUpstreamNotFound) {
			return nil, ErrUpstreamNotFound
		}
		return nil, err
	}

	items, err := s.fetchRemoteModels(ctx, upstreamItem)
	if err != nil {
		return nil, err
	}

	slices.SortFunc(items, func(a, b llm.ModelItem) int {
		return strings.Compare(a.ID, b.ID)
	})

	result := &SyncUpstreamModelsData{
		TotalUpstream: len(items),
		SyncedModels:  make([]UpstreamSyncModelView, 0, len(items)),
	}
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		name := strings.TrimSpace(item.ID)
		if name == "" {
			continue
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}

		view, syncErr := s.syncSingleUpstreamModel(ctx, upstreamItem, item)
		if syncErr != nil {
			result.SkippedUpstreamModels++
			continue
		}
		if view.Created {
			result.CreatedUpstreamModels++
		} else {
			result.ExistingUpstreamModels++
		}
		result.SyncedModels = append(result.SyncedModels, view)
	}

	activeNames := make([]string, 0, len(seen))
	for name := range seen {
		activeNames = append(activeNames, name)
	}
	inactivated, err := s.repo.MarkMissingSyncedUpstreamModelsInactive(ctx, upstreamID, activeNames)
	if err != nil {
		return nil, err
	}
	result.InactivatedModels = inactivated

	s.InvalidateModelCatalog()
	return result, nil
}

// ImportUpstreamModels 批量把上游真实模型绑定到平台模型。
func (s *Service) ImportUpstreamModels(ctx context.Context, upstreamID uint, input ImportUpstreamModelsInput) (*ImportUpstreamModelsData, error) {
	if _, err := s.repo.GetUpstreamByID(ctx, upstreamID); err != nil {
		if errors.Is(err, ErrUpstreamNotFound) {
			return nil, ErrUpstreamNotFound
		}
		return nil, err
	}

	result := &ImportUpstreamModelsData{
		Total:   len(input.Items),
		Results: make([]ImportUpstreamModelResultView, 0, len(input.Items)),
	}
	for _, item := range input.Items {
		imported, importErr := s.importSingleUpstreamModel(ctx, upstreamID, item)
		if importErr != nil {
			result.FailedCount++
			result.Results = append(result.Results, ImportUpstreamModelResultView{
				UpstreamModelName: strings.TrimSpace(item.UpstreamModelName),
				PlatformModelName: strings.TrimSpace(item.PlatformModelName),
				Status:            ImportUpstreamModelStatusFailed,
				Error:             importErr.Error(),
			})
			continue
		}
		result.ImportedCount++
		status := ImportUpstreamModelStatusExisting
		if imported.CreatedRoute {
			result.CreatedRoutes++
			status = ImportUpstreamModelStatusCreated
		} else {
			result.ExistingRoutes++
		}
		if imported.CreatedPlatform {
			result.CreatedPlatform++
		}
		imported.Status = status
		result.Results = append(result.Results, imported)
	}

	s.InvalidateModelCatalog()
	return result, nil
}

// ---------------------------------------------------------------------------
// 同步与导入辅助
// ---------------------------------------------------------------------------

func (s *Service) fetchRemoteModels(ctx context.Context, up *domainchannel.Upstream) ([]llm.ModelItem, error) {
	if s.llmClient == nil {
		return nil, ErrRemoteModelsUnavailable
	}
	keyCfg, err := s.parseAPIKeysConfig(up.APIKeysEnc)
	if err != nil {
		return nil, ErrNoActiveKey
	}
	apiKey, err := s.selectAPIKey(ctx, up.ID, keyCfg)
	if err != nil {
		return nil, ErrNoActiveKey
	}
	protocol, err := resolveRouteProtocol("", up.Compatible, up.ProtocolDefaultsJSON, `["chat"]`)
	if err != nil {
		return nil, err
	}
	attributionReferer, attributionTitle := s.llmAttribution()
	items, err := s.llmClient.ListModels(ctx, llm.RouteConfig{
		Protocol:           protocol,
		BaseURL:            up.BaseURL,
		APIKey:             apiKey,
		HeadersJSON:        up.HeadersJSON,
		ConnectTimeoutMS:   up.ConnectTimeoutMS,
		ReadTimeoutMS:      up.ReadTimeoutMS,
		AttributionReferer: attributionReferer,
		AttributionTitle:   attributionTitle,
	})
	if err != nil {
		s.warn("fetch_remote_models_failed",
			zap.Uint("upstream_id", up.ID),
			zap.String("compatible", up.Compatible),
			zap.String("base_url", up.BaseURL),
			zap.Error(err),
		)
		return nil, fmt.Errorf("%w: %v", ErrRemoteModelsUnavailable, err)
	}
	return items, nil
}

func (s *Service) syncSingleUpstreamModel(ctx context.Context, up *domainchannel.Upstream, item llm.ModelItem) (UpstreamSyncModelView, error) {
	upstreamModelName := strings.TrimSpace(item.ID)
	kindsJSON := inferKindsJSON(upstreamModelName)
	protocol, err := resolveRouteProtocol("", up.Compatible, up.ProtocolDefaultsJSON, kindsJSON)
	if err != nil {
		return UpstreamSyncModelView{}, err
	}
	created := false
	bindingCode := generateBindingCode()
	if existing, err := s.repo.GetUpstreamModelByUpstreamName(ctx, up.ID, upstreamModelName); err == nil {
		bindingCode = existing.BindingCode
	} else if errors.Is(err, ErrUpstreamModelNotFound) {
		created = true
	} else {
		return UpstreamSyncModelView{}, err
	}
	now := time.Now()
	rawJSON, _ := json.Marshal(map[string]string{
		"id":       item.ID,
		"owned_by": item.OwnedBy,
	})
	vendor := normalizeUpstreamModelVendor(item.OwnedBy, upstreamModelName, up.Name, up.BaseURL)
	upstreamModel := &domainchannel.UpstreamModel{
		UpstreamID:        up.ID,
		BindingCode:       bindingCode,
		UpstreamModelName: upstreamModelName,
		Vendor:            vendor,
		Icon:              normalizeModelIcon("", vendor, upstreamModelName),
		SuggestedProtocol: protocol,
		KindsJSON:         kindsJSON,
		Status:            "active",
		Source:            "sync",
		LastSyncedAt:      &now,
		RawJSON:           string(rawJSON),
	}
	if err := s.repo.UpsertUpstreamModel(ctx, upstreamModel); err != nil {
		return UpstreamSyncModelView{}, err
	}
	return UpstreamSyncModelView{
		UpstreamModelName: upstreamModel.UpstreamModelName,
		BindingCode:       upstreamModel.BindingCode,
		SuggestedProtocol: upstreamModel.SuggestedProtocol,
		KindsJSON:         upstreamModel.KindsJSON,
		Status:            upstreamModel.Status,
		Created:           created,
	}, nil
}

func (s *Service) importSingleUpstreamModel(ctx context.Context, upstreamID uint, input ImportUpstreamModelItemInput) (ImportUpstreamModelResultView, error) {
	platformModelName, err := normalizePlatformModelName(input.PlatformModelName)
	if err != nil {
		return ImportUpstreamModelResultView{}, err
	}
	upstreamModelName := strings.TrimSpace(input.UpstreamModelName)
	if upstreamModelName == "" {
		return ImportUpstreamModelResultView{}, ErrUpstreamModelNotFound
	}
	_, platformErr := s.repo.GetModelByName(ctx, platformModelName)
	createdPlatform := errors.Is(platformErr, ErrModelNotFound)
	if platformErr != nil && !createdPlatform {
		return ImportUpstreamModelResultView{}, platformErr
	}
	createdRoute := !s.routeExists(ctx, upstreamID, platformModelName, upstreamModelName)

	view, err := s.UpsertUpstreamModel(ctx, upstreamID, UpsertUpstreamModelInput{
		PlatformModelName: platformModelName,
		UpstreamModelName: upstreamModelName,
		Protocol:          input.Protocol,
		KindsJSON:         input.KindsJSON,
		Status:            input.Status,
		Priority:          input.Priority,
		Weight:            1,
		Source:            "import",
	})
	if err != nil {
		return ImportUpstreamModelResultView{}, err
	}
	return ImportUpstreamModelResultView{
		UpstreamModelName: view.UpstreamModelName,
		PlatformModelName: view.PlatformModelName,
		BindingCode:       view.BindingCode,
		CreatedRoute:      createdRoute,
		CreatedPlatform:   createdPlatform,
	}, nil
}

func (s *Service) routeExists(ctx context.Context, upstreamID uint, platformModelName string, upstreamModelName string) bool {
	rows, _, err := s.repo.ListUpstreamModels(ctx, upstreamID, repository.ListChannelUpstreamModelsInput{Offset: 0, Limit: 5000})
	if err != nil {
		return false
	}
	for _, row := range rows {
		if strings.TrimSpace(row.PlatformModelName) == platformModelName &&
			strings.TrimSpace(row.UpstreamModelName) == upstreamModelName &&
			row.RouteID > 0 {
			return true
		}
	}
	return false
}
