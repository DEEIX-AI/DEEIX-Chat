package channel

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	domainchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/channel"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/apperr"
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
	items = normalizeRemoteModelItems(items)

	remoteNames := make([]string, 0, len(items))
	for _, item := range items {
		if name := strings.TrimSpace(item.ID); name != "" {
			remoteNames = append(remoteNames, name)
		}
	}
	rows, err := s.repo.ListUpstreamModelsByNames(ctx, upstreamID, remoteNames)
	if err != nil {
		return nil, err
	}
	managedModels, err := s.repo.ListManagedUpstreamModels(ctx, upstreamID)
	if err != nil {
		return nil, err
	}
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
		suggestedProtocols, _ := resolveRouteProtocols(nil, upstreamItem.Compatible, upstreamItem.ProtocolDefaultsJSON, kindsJSON)
		suggestedProtocol := ""
		if len(suggestedProtocols) > 0 {
			suggestedProtocol = suggestedProtocols[0]
		}
		snapshot, alreadySynced := existingByName[name]
		views = append(views, UpstreamRemoteModelView{
			UpstreamModelName:          name,
			SuggestedPlatformModelName: name,
			SuggestedKindsJSON:         kindsJSON,
			SuggestedProtocol:          suggestedProtocol,
			SuggestedProtocols:         suggestedProtocols,
			BindingCode:                snapshot.BindingCode,
			BoundPlatformModels:        snapshot.BoundPlatformModels,
			UpstreamModelStatus:        snapshot.Status,
			AlreadySynced:              alreadySynced,
			AlreadyBound:               len(snapshot.BoundPlatformModels) > 0,
		})
	}

	syncPlan, err := buildUpstreamModelSyncPlan(upstreamItem, items, managedModels, existingByName)
	if err != nil {
		return nil, err
	}
	return &UpstreamRemoteModelsData{
		Total:      len(views),
		Items:      views,
		SnapshotID: remoteModelsSnapshotID(items),
		SyncPlan:   syncPlan,
	}, nil
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

// SyncUpstreamModels 拉取上游完整模型快照，并与本地远端管理目录进行原子软对账。
func (s *Service) SyncUpstreamModels(ctx context.Context, upstreamID uint, input SyncUpstreamModelsInput) (*SyncUpstreamModelsData, error) {
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
	items = normalizeRemoteModelItems(items)
	if expected := strings.TrimSpace(input.ExpectedSnapshot); expected != "" && expected != remoteModelsSnapshotID(items) {
		return nil, ErrRemoteModelsSnapshotChanged
	}
	return s.reconcileRemoteModelSnapshot(ctx, upstreamItem, items, input.AllowEmpty)
}

// reconcileRemoteModelSnapshot 在一个事务内完成目录读取、分类与批量写入；
// 上游网络请求已在事务外完成，避免长事务占用数据库连接。
func (s *Service) reconcileRemoteModelSnapshot(
	ctx context.Context,
	upstreamItem *domainchannel.Upstream,
	items []llm.ModelItem,
	allowEmpty bool,
) (*SyncUpstreamModelsData, error) {
	items = normalizeRemoteModelItems(items)
	if len(items) == 0 && !allowEmpty {
		return nil, ErrEmptyRemoteModels
	}

	result := &SyncUpstreamModelsData{
		SnapshotID:    remoteModelsSnapshotID(items),
		TotalUpstream: len(items),
		SyncedModels:  make([]UpstreamSyncModelView, 0, len(items)),
	}
	remoteNames := make([]string, 0, len(items))
	for _, item := range items {
		remoteNames = append(remoteNames, item.ID)
	}

	err := s.repo.WithinTransaction(ctx, func(txRepo repository.ChannelRepository) error {
		existingRows, syncErr := txRepo.ListUpstreamModelsByNames(ctx, upstreamItem.ID, remoteNames)
		if syncErr != nil {
			return syncErr
		}
		managedModels, syncErr := txRepo.ListManagedUpstreamModels(ctx, upstreamItem.ID)
		if syncErr != nil {
			return syncErr
		}

		existingByName := make(map[string]repositoryUpstreamModelSnapshot, len(existingRows))
		for _, row := range existingRows {
			name := strings.TrimSpace(row.UpstreamModelName)
			if name == "" {
				continue
			}
			existingByName[name] = repositoryUpstreamModelSnapshot{
				BindingCode: row.BindingCode,
				Status:      row.Status,
			}
		}
		managedByName := make(map[string]domainchannel.UpstreamModel, len(managedModels))
		for _, model := range managedModels {
			name := strings.TrimSpace(model.UpstreamModelName)
			if name != "" {
				managedByName[name] = model
			}
		}

		remoteNameSet := make(map[string]struct{}, len(items))
		changes := repository.ApplyUpstreamModelCatalogChangesInput{
			Create: make([]domainchannel.UpstreamModel, 0),
			Update: make([]domainchannel.UpstreamModel, 0),
		}
		now := time.Now()
		for _, item := range items {
			name := item.ID
			remoteNameSet[name] = struct{}{}
			kindsJSON := inferKindsJSON(name)
			protocol, resolveErr := resolveRouteProtocol("", upstreamItem.Compatible, upstreamItem.ProtocolDefaultsJSON, kindsJSON)
			if resolveErr != nil {
				return resolveErr
			}

			if existing, managed := managedByName[name]; managed {
				desired := syncedUpstreamModel(upstreamItem, item, existing.BindingCode, &now, protocol, kindsJSON)
				desired.ID = existing.ID
				desired.CreatedAt = existing.CreatedAt
				desired.UpdatedAt = existing.UpdatedAt
				reactivated := !strings.EqualFold(strings.TrimSpace(existing.Status), "active")
				updated := !reactivated && upstreamModelMetadataChanged(existing, desired)
				switch {
				case reactivated:
					result.ReactivatedModels++
				case updated:
					result.UpdatedUpstreamModels++
				default:
					result.UnchangedUpstreamModels++
				}
				changes.Update = append(changes.Update, *desired)
				result.SyncedModels = append(result.SyncedModels, UpstreamSyncModelView{
					UpstreamModelName: desired.UpstreamModelName,
					BindingCode:       desired.BindingCode,
					SuggestedProtocol: desired.SuggestedProtocol,
					KindsJSON:         desired.KindsJSON,
					Status:            desired.Status,
					Updated:           updated,
					Reactivated:       reactivated,
				})
				continue
			}

			if existing, protected := existingByName[name]; protected {
				result.ProtectedUpstreamModels++
				result.SyncedModels = append(result.SyncedModels, UpstreamSyncModelView{
					UpstreamModelName: name,
					BindingCode:       existing.BindingCode,
					SuggestedProtocol: protocol,
					KindsJSON:         kindsJSON,
					Status:            existing.Status,
					Protected:         true,
				})
				continue
			}

			created := syncedUpstreamModel(upstreamItem, item, generateBindingCode(), &now, protocol, kindsJSON)
			changes.Create = append(changes.Create, *created)
			result.CreatedUpstreamModels++
			result.SyncedModels = append(result.SyncedModels, UpstreamSyncModelView{
				UpstreamModelName: created.UpstreamModelName,
				BindingCode:       created.BindingCode,
				SuggestedProtocol: created.SuggestedProtocol,
				KindsJSON:         created.KindsJSON,
				Status:            created.Status,
				Created:           true,
			})
		}

		for _, model := range managedModels {
			if !strings.EqualFold(strings.TrimSpace(model.Status), "active") {
				continue
			}
			if _, present := remoteNameSet[strings.TrimSpace(model.UpstreamModelName)]; !present {
				changes.InactivateIDs = append(changes.InactivateIDs, model.ID)
			}
		}

		inactivated, syncErr := txRepo.ApplyUpstreamModelCatalogChanges(ctx, upstreamItem.ID, changes)
		if syncErr != nil {
			if errors.Is(syncErr, repository.ErrDuplicate) {
				return ErrRemoteModelsSnapshotChanged
			}
			return syncErr
		}
		result.InactivatedModels = inactivated
		result.ExistingUpstreamModels = result.TotalUpstream - result.CreatedUpstreamModels
		return nil
	})
	if err != nil {
		return nil, err
	}

	s.InvalidateModelCatalog()
	return result, nil
}

func normalizeRemoteModelItems(items []llm.ModelItem) []llm.ModelItem {
	unique := make(map[string]llm.ModelItem, len(items))
	for _, item := range items {
		name := strings.TrimSpace(item.ID)
		if name == "" {
			continue
		}
		if _, exists := unique[name]; exists {
			continue
		}
		item.ID = name
		item.OwnedBy = strings.TrimSpace(item.OwnedBy)
		unique[name] = item
	}
	result := make([]llm.ModelItem, 0, len(unique))
	for _, item := range unique {
		result = append(result, item)
	}
	slices.SortFunc(result, func(a, b llm.ModelItem) int {
		return strings.Compare(a.ID, b.ID)
	})
	return result
}

func remoteModelsSnapshotID(items []llm.ModelItem) string {
	payload, _ := json.Marshal(items)
	sum := sha256.Sum256(payload)
	return fmt.Sprintf("%x", sum)
}

func buildUpstreamModelSyncPlan(
	upstream *domainchannel.Upstream,
	remoteItems []llm.ModelItem,
	managedModels []domainchannel.UpstreamModel,
	existingByName map[string]repositoryUpstreamModelSnapshot,
) (UpstreamModelSyncPlanView, error) {
	plan := UpstreamModelSyncPlanView{
		AddedModels:       []string{},
		UpdatedModels:     []string{},
		ReactivatedModels: []string{},
		InactivatedModels: []string{},
		UnchangedModels:   []string{},
		ProtectedModels:   []string{},
	}
	managedByName := make(map[string]domainchannel.UpstreamModel, len(managedModels))
	for _, item := range managedModels {
		name := strings.TrimSpace(item.UpstreamModelName)
		if name != "" {
			managedByName[name] = item
		}
	}
	remoteNames := make(map[string]struct{}, len(remoteItems))
	for _, item := range remoteItems {
		name := strings.TrimSpace(item.ID)
		remoteNames[name] = struct{}{}
		existing, managed := managedByName[name]
		if !managed {
			if _, protected := existingByName[name]; protected {
				plan.ProtectedModels = append(plan.ProtectedModels, name)
			} else {
				plan.AddedModels = append(plan.AddedModels, name)
			}
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(existing.Status), "active") {
			plan.ReactivatedModels = append(plan.ReactivatedModels, name)
			continue
		}
		kindsJSON := inferKindsJSON(name)
		protocol, err := resolveRouteProtocol("", upstream.Compatible, upstream.ProtocolDefaultsJSON, kindsJSON)
		if err != nil {
			return UpstreamModelSyncPlanView{}, err
		}
		desired := syncedUpstreamModel(upstream, item, existing.BindingCode, nil, protocol, kindsJSON)
		if upstreamModelMetadataChanged(existing, desired) {
			plan.UpdatedModels = append(plan.UpdatedModels, name)
		} else {
			plan.UnchangedModels = append(plan.UnchangedModels, name)
		}
	}
	for _, item := range managedModels {
		name := strings.TrimSpace(item.UpstreamModelName)
		if !strings.EqualFold(strings.TrimSpace(item.Status), "active") {
			continue
		}
		if _, present := remoteNames[name]; !present {
			plan.InactivatedModels = append(plan.InactivatedModels, name)
		}
	}
	return plan, nil
}

func upstreamModelMetadataChanged(existing domainchannel.UpstreamModel, desired *domainchannel.UpstreamModel) bool {
	return strings.TrimSpace(existing.Vendor) != strings.TrimSpace(desired.Vendor) ||
		strings.TrimSpace(existing.Icon) != strings.TrimSpace(desired.Icon) ||
		strings.TrimSpace(existing.SuggestedProtocol) != strings.TrimSpace(desired.SuggestedProtocol) ||
		strings.TrimSpace(existing.KindsJSON) != strings.TrimSpace(desired.KindsJSON) ||
		strings.TrimSpace(existing.RawJSON) != strings.TrimSpace(desired.RawJSON)
}

// ImportUpstreamModels 批量把上游真实模型绑定到平台模型。
func (s *Service) ImportUpstreamModels(ctx context.Context, upstreamID uint, input ImportUpstreamModelsInput) (*ImportUpstreamModelsData, error) {
	upstreamItem, err := s.repo.GetUpstreamByID(ctx, upstreamID)
	if err != nil {
		if errors.Is(err, ErrUpstreamNotFound) {
			return nil, ErrUpstreamNotFound
		}
		return nil, err
	}

	permissionGroupIDs, groupWriter, err := s.normalizeImportPermissionGroupIDs(ctx, input.PermissionGroupIDs)
	if err != nil {
		return nil, err
	}

	result := &ImportUpstreamModelsData{
		Total:   len(input.Items),
		Results: make([]ImportUpstreamModelResultView, 0, len(input.Items)),
	}
	for _, item := range input.Items {
		imported, importErr := s.importSingleUpstreamModel(ctx, upstreamItem, item)
		if importErr != nil {
			result.FailedCount++
			result.Results = append(result.Results, ImportUpstreamModelResultView{
				UpstreamModelName: strings.TrimSpace(item.UpstreamModelName),
				PlatformModelName: strings.TrimSpace(item.PlatformModelName),
				Status:            ImportUpstreamModelStatusFailed,
				Error:             apperr.MessageOr(importErr, "upstream model import failed"),
			})
			continue
		}
		result.ImportedCount++
		status := ImportUpstreamModelStatusExisting
		result.CreatedRoutes += imported.CreatedRoutes
		result.ExistingRoutes += imported.ExistingRoutes
		if imported.CreatedRoutes > 0 {
			status = ImportUpstreamModelStatusCreated
		}
		if imported.CreatedPlatform {
			result.CreatedPlatform++
		}
		imported.Status = status
		if groupWriter != nil {
			groupIDs, err := mergeImportedModelPermissionGroupIDs(ctx, groupWriter, imported.PlatformModelID, permissionGroupIDs)
			if err != nil {
				return nil, err
			}
			if err := groupWriter.SetModelManualGroups(ctx, imported.PlatformModelID, groupIDs); err != nil {
				return nil, err
			}
		}
		result.Results = append(result.Results, imported)
	}

	s.InvalidateModelCatalog()
	return result, nil
}

func (s *Service) normalizeImportPermissionGroupIDs(ctx context.Context, groupIDs []uint) ([]uint, modelPermissionGroupWriter, error) {
	if len(groupIDs) == 0 {
		return nil, nil, nil
	}
	writer, ok := s.permGroupRepo.(modelPermissionGroupWriter)
	if !ok {
		return nil, nil, ErrPermissionGroupRepoUnavailable
	}
	seen := make(map[uint]struct{}, len(groupIDs))
	result := make([]uint, 0, len(groupIDs))
	for _, id := range groupIDs {
		if id == 0 {
			return nil, nil, ErrInvalidPermissionGroupModels
		}
		if _, ok := seen[id]; ok {
			continue
		}
		exists, err := writer.PermissionGroupExists(ctx, id)
		if err != nil {
			return nil, nil, err
		}
		if !exists {
			return nil, nil, ErrInvalidPermissionGroupModels
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result, writer, nil
}

func mergeImportedModelPermissionGroupIDs(ctx context.Context, writer modelPermissionGroupWriter, platformModelID uint, groupIDs []uint) ([]uint, error) {
	currentIDs, err := writer.ListModelManualGroupIDs(ctx, platformModelID)
	if err != nil {
		return nil, err
	}
	seen := make(map[uint]struct{}, len(currentIDs)+len(groupIDs))
	result := make([]uint, 0, len(currentIDs)+len(groupIDs))
	for _, id := range currentIDs {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	for _, id := range groupIDs {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
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
		return nil, fmt.Errorf("%w: %w", ErrRemoteModelsUnavailable, err)
	}
	return items, nil
}

func syncedUpstreamModel(
	upstream *domainchannel.Upstream,
	item llm.ModelItem,
	bindingCode string,
	lastSyncedAt *time.Time,
	protocol string,
	kindsJSON string,
) *domainchannel.UpstreamModel {
	name := strings.TrimSpace(item.ID)
	rawJSON, _ := json.Marshal(map[string]string{
		"id":       name,
		"owned_by": strings.TrimSpace(item.OwnedBy),
	})
	vendor := normalizeModelVendor(item.OwnedBy, name, upstream.Name, upstream.BaseURL)
	return &domainchannel.UpstreamModel{
		UpstreamID:        upstream.ID,
		BindingCode:       bindingCode,
		UpstreamModelName: name,
		Vendor:            vendor,
		Icon:              normalizeModelIcon("", vendor, name),
		SuggestedProtocol: protocol,
		KindsJSON:         kindsJSON,
		Status:            "active",
		Source:            "sync",
		LastSyncedAt:      lastSyncedAt,
		RawJSON:           string(rawJSON),
	}
}

func (s *Service) importSingleUpstreamModel(ctx context.Context, upstreamItem *domainchannel.Upstream, input ImportUpstreamModelItemInput) (ImportUpstreamModelResultView, error) {
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

	kindsJSON := strings.TrimSpace(input.KindsJSON)
	if kindsJSON == "" {
		kindsJSON = inferKindsJSON(platformModelName)
	}
	explicitProtocols := append([]string{}, input.Protocols...)
	if len(explicitProtocols) == 0 && strings.TrimSpace(input.Protocol) != "" {
		explicitProtocols = append(explicitProtocols, input.Protocol)
	}
	protocols, err := resolveRouteProtocols(explicitProtocols, upstreamItem.Compatible, upstreamItem.ProtocolDefaultsJSON, kindsJSON)
	if err != nil {
		return ImportUpstreamModelResultView{}, err
	}
	result := ImportUpstreamModelResultView{
		UpstreamModelName: upstreamModelName,
		PlatformModelName: platformModelName,
		CreatedPlatform:   createdPlatform,
		Protocols:         protocols,
	}
	for _, protocol := range protocols {
		if !s.routeExists(ctx, upstreamItem.ID, platformModelName, upstreamModelName, protocol) {
			result.CreatedRoutes++
		} else {
			result.ExistingRoutes++
		}
	}
	status := input.Status
	priority := input.Priority
	weight := 1
	routeSource := "import"
	catalogSource := "sync"
	view, err := s.UpsertUpstreamModel(ctx, upstreamItem.ID, UpsertUpstreamModelInput{
		PlatformModelName: platformModelName,
		UpstreamModelName: upstreamModelName,
		Protocols:         protocols,
		KindsJSON:         kindsJSON,
		Status:            &status,
		Priority:          &priority,
		Weight:            &weight,
		Source:            &routeSource,
		CatalogSource:     &catalogSource,
	})
	if err != nil {
		return ImportUpstreamModelResultView{}, err
	}
	result.BindingCode = view.BindingCode
	result.PlatformModelID = view.PlatformModelID
	result.CreatedRoute = result.CreatedRoutes > 0
	return result, nil
}

func (s *Service) routeExists(ctx context.Context, upstreamID uint, platformModelName string, upstreamModelName string, protocol string) bool {
	_, err := s.repo.GetUpstreamModelRouteByNames(ctx, upstreamID, platformModelName, upstreamModelName, protocol)
	return err == nil
}
