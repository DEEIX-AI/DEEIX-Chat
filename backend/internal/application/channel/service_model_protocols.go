package channel

import (
	"context"
	"errors"
	"strings"

	domainchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/channel"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

// SetModelProtocols 原子替换平台模型全部上游绑定的协议集合。
func (s *Service) SetModelProtocols(ctx context.Context, modelID uint, input SetModelProtocolsInput) (*ModelView, error) {
	if len(input.Protocols) == 0 {
		return nil, ErrProtocolRequired
	}
	if len(input.Protocols) > 2 {
		return nil, ErrInvalidRouteProtocolCombination
	}
	seenProtocols := make(map[string]struct{}, len(input.Protocols))
	for _, raw := range input.Protocols {
		protocol := strings.TrimSpace(strings.ToLower(raw))
		if protocol == "" {
			return nil, ErrInvalidAdapter
		}
		if _, exists := seenProtocols[protocol]; exists {
			return nil, ErrInvalidRouteProtocolCombination
		}
		seenProtocols[protocol] = struct{}{}
	}
	kindsJSON, err := normalizeKindsJSON(input.KindsJSON)
	if err != nil {
		return nil, err
	}

	var view *ModelView
	err = s.repo.WithinTransaction(ctx, func(txRepo repository.ChannelRepository) error {
		modelItem, txErr := txRepo.GetModelByID(ctx, modelID)
		if txErr != nil {
			return txErr
		}
		sources, txErr := txRepo.ListModelUpstreamSourcesForUpdate(ctx, modelItem.PlatformModelName)
		if txErr != nil {
			return txErr
		}
		bindings := groupModelSourceBindings(sources)
		if len(bindings) == 0 {
			return ErrUpstreamModelNotFound
		}

		replacements := make([]modelProtocolReplacement, 0, len(bindings))
		for _, binding := range bindings {
			protocols, resolveErr := resolveRouteProtocols(
				input.Protocols,
				binding.template.UpstreamCompatible,
				binding.template.UpstreamProtocolDefaultsJSON,
				kindsJSON,
			)
			if resolveErr != nil {
				return resolveErr
			}
			replacements = append(replacements, modelProtocolReplacement{binding: binding, protocols: protocols})
		}

		if txErr = txRepo.UpdateModel(ctx, modelID, repository.UpdateChannelModelInput{KindsJSON: &kindsJSON}); txErr != nil {
			return txErr
		}
		routeSets := make([]repository.ReplaceChannelPlatformRoutesInput, 0, len(replacements))
		for _, item := range replacements {
			desiredRoutes := make([]domainchannel.PlatformModelRoute, 0, len(item.protocols))
			for _, protocol := range item.protocols {
				source := item.binding.template
				if existing, ok := item.binding.sourcesByProtocol[protocol]; ok {
					source = existing
				}
				desiredRoutes = append(desiredRoutes, modelSourceReplacementRoute(source, protocol))
			}
			routeSets = append(routeSets, repository.ReplaceChannelPlatformRoutesInput{
				UpstreamID:       item.binding.template.UpstreamID,
				ExistingRouteIDs: item.binding.routeIDs,
				Routes:           desiredRoutes,
			})
		}
		if _, txErr = txRepo.ReplacePlatformModelRoutes(ctx, routeSets); txErr != nil {
			return txErr
		}

		row, txErr := txRepo.GetModelListRowByID(ctx, modelID)
		if txErr != nil {
			return txErr
		}
		result := s.toModelView(*row)
		views := []ModelView{result}
		if txErr = s.normalizeModelAvailabilityWithRepo(ctx, txRepo, views); txErr != nil {
			return txErr
		}
		view = &views[0]
		return nil
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

type modelSourceBinding struct {
	template          repository.ChannelModelSourceRow
	sourcesByProtocol map[string]repository.ChannelModelSourceRow
	routeIDs          []uint
}

type modelProtocolReplacement struct {
	binding   modelSourceBinding
	protocols []string
}

func groupModelSourceBindings(sources []repository.ChannelModelSourceRow) []modelSourceBinding {
	type bindingKey struct {
		upstreamID      uint
		upstreamModelID uint
	}
	bindings := make([]modelSourceBinding, 0)
	indexes := make(map[bindingKey]int)
	for _, source := range sources {
		key := bindingKey{upstreamID: source.UpstreamID, upstreamModelID: source.UpstreamModelID}
		index, ok := indexes[key]
		if !ok {
			index = len(bindings)
			indexes[key] = index
			bindings = append(bindings, modelSourceBinding{
				template:          source,
				sourcesByProtocol: make(map[string]repository.ChannelModelSourceRow),
			})
		}
		bindings[index].sourcesByProtocol[strings.TrimSpace(strings.ToLower(source.Protocol))] = source
		bindings[index].routeIDs = append(bindings[index].routeIDs, source.ID)
	}
	return bindings
}

func modelSourceReplacementRoute(source repository.ChannelModelSourceRow, protocol string) domainchannel.PlatformModelRoute {
	return domainchannel.PlatformModelRoute{
		PlatformModelID:    source.PlatformModelID,
		UpstreamModelID:    source.UpstreamModelID,
		Protocol:           protocol,
		Status:             source.Status,
		Priority:           source.Priority,
		Weight:             source.Weight,
		Source:             source.Source,
		CbFailureThreshold: source.CbFailureThreshold,
		CbDurationMin:      source.CbDurationMin,
		CbWindowMin:        source.CbWindowMin,
		HeadersJSON:        source.HeadersJSON,
	}
}
