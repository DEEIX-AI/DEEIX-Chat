package channel

import (
	"context"
	"testing"

	domainchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/channel"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

func TestUpdateModelResetsIconToAutoWhenExplicitlyEmpty(t *testing.T) {
	repo := &modelUpdateRepo{
		model: domainchannel.PlatformModel{
			ID:                1,
			PlatformModelName: "claude-sonnet-4.5",
			Vendor:            "anthropic",
			KindsJSON:         `["chat"]`,
			Icon:              "openai",
			AccessScope:       "public",
			Status:            "active",
		},
	}
	service := NewService(config.Config{}, repo, nil, nil)

	emptyIcon := ""
	view, err := service.UpdateModel(context.Background(), 1, UpdateModelInput{Icon: &emptyIcon})
	if err != nil {
		t.Fatalf("UpdateModel() error = %v", err)
	}
	if repo.lastUpdate.Icon == nil {
		t.Fatal("expected icon update field to be present")
	}
	if *repo.lastUpdate.Icon != "claude" {
		t.Fatalf("expected auto icon, got %q", *repo.lastUpdate.Icon)
	}
	if view.Icon != "claude" {
		t.Fatalf("expected returned model icon to be auto icon, got %q", view.Icon)
	}
}

type modelUpdateRepo struct {
	model      domainchannel.PlatformModel
	lastUpdate repository.UpdateChannelModelInput
}

func (r *modelUpdateRepo) CreateUpstream(context.Context, *domainchannel.Upstream) error {
	return nil
}

func (r *modelUpdateRepo) UpdateUpstream(context.Context, uint, repository.UpdateChannelUpstreamInput) error {
	return nil
}

func (r *modelUpdateRepo) GetUpstreamByID(context.Context, uint) (*domainchannel.Upstream, error) {
	return nil, repository.ErrNotFound
}

func (r *modelUpdateRepo) ListUpstreams(context.Context, repository.ListChannelUpstreamsInput) ([]repository.ChannelUpstreamListRow, int64, error) {
	return nil, 0, nil
}

func (r *modelUpdateRepo) CreateModel(context.Context, *domainchannel.PlatformModel) error {
	return nil
}

func (r *modelUpdateRepo) UpdateModel(_ context.Context, _ uint, input repository.UpdateChannelModelInput) error {
	r.lastUpdate = input
	if input.PlatformModelName != nil {
		r.model.PlatformModelName = *input.PlatformModelName
	}
	if input.Vendor != nil {
		r.model.Vendor = *input.Vendor
	}
	if input.KindsJSON != nil {
		r.model.KindsJSON = *input.KindsJSON
	}
	if input.Icon != nil {
		r.model.Icon = *input.Icon
	}
	if input.CapabilitiesJSON != nil {
		r.model.CapabilitiesJSON = *input.CapabilitiesJSON
	}
	if input.SystemPrompt != nil {
		r.model.SystemPrompt = *input.SystemPrompt
	}
	if input.AccessScope != nil {
		r.model.AccessScope = *input.AccessScope
	}
	if input.Status != nil {
		r.model.Status = *input.Status
	}
	if input.Description != nil {
		r.model.Description = *input.Description
	}
	return nil
}

func (r *modelUpdateRepo) ReorderModels(context.Context, []uint) error {
	return nil
}

func (r *modelUpdateRepo) GetModelByID(context.Context, uint) (*domainchannel.PlatformModel, error) {
	model := r.model
	return &model, nil
}

func (r *modelUpdateRepo) GetModelListRowByID(context.Context, uint) (*repository.ChannelModelListRow, error) {
	return &repository.ChannelModelListRow{PlatformModel: r.model}, nil
}

func (r *modelUpdateRepo) GetModelByName(context.Context, string) (*domainchannel.PlatformModel, error) {
	return nil, repository.ErrNotFound
}

func (r *modelUpdateRepo) GetActiveModelByName(context.Context, string) (*domainchannel.PlatformModel, error) {
	return nil, repository.ErrNotFound
}

func (r *modelUpdateRepo) ListModels(context.Context, repository.ListChannelModelsInput) ([]repository.ChannelModelListRow, int64, error) {
	return nil, 0, nil
}

func (r *modelUpdateRepo) UpsertUpstreamModel(context.Context, *domainchannel.UpstreamModel) error {
	return nil
}

func (r *modelUpdateRepo) GetUpstreamModelByID(context.Context, uint, uint) (*domainchannel.UpstreamModel, error) {
	return nil, repository.ErrNotFound
}

func (r *modelUpdateRepo) GetUpstreamModelByUpstreamName(context.Context, uint, string) (*domainchannel.UpstreamModel, error) {
	return nil, repository.ErrNotFound
}

func (r *modelUpdateRepo) UpdateUpstreamModelByID(context.Context, uint, uint, repository.UpdateChannelUpstreamModelInput) error {
	return nil
}

func (r *modelUpdateRepo) DeleteUpstreamModel(context.Context, uint, uint) error {
	return nil
}

func (r *modelUpdateRepo) MarkMissingSyncedUpstreamModelsInactive(context.Context, uint, []string) (int64, error) {
	return 0, nil
}

func (r *modelUpdateRepo) ListUpstreamModels(context.Context, uint, repository.ListChannelUpstreamModelsInput) ([]repository.ChannelUpstreamModelListRow, int64, error) {
	return nil, 0, nil
}

func (r *modelUpdateRepo) ListUpstreamModelsByNames(context.Context, uint, []string) ([]repository.ChannelUpstreamModelListRow, error) {
	return nil, nil
}

func (r *modelUpdateRepo) GetUpstreamModelRouteByID(context.Context, uint, uint) (*repository.ChannelUpstreamModelListRow, error) {
	return nil, repository.ErrNotFound
}

func (r *modelUpdateRepo) GetUpstreamModelRouteByNames(context.Context, uint, string, string, string) (*repository.ChannelUpstreamModelListRow, error) {
	return nil, repository.ErrNotFound
}

func (r *modelUpdateRepo) UpsertPlatformModelRoute(context.Context, *domainchannel.PlatformModelRoute) error {
	return nil
}

func (r *modelUpdateRepo) GetModelUpstreamSourceByRouteID(context.Context, string, uint) (*repository.ChannelModelSourceRow, error) {
	return nil, repository.ErrNotFound
}

func (r *modelUpdateRepo) ListPlatformModelRoutesByPair(context.Context, uint, uint, uint) ([]domainchannel.PlatformModelRoute, error) {
	return nil, nil
}

func (r *modelUpdateRepo) GetPlatformModelRouteByID(context.Context, uint, uint) (*domainchannel.PlatformModelRoute, error) {
	return nil, repository.ErrNotFound
}

func (r *modelUpdateRepo) UpdatePlatformModelRouteByID(context.Context, uint, uint, repository.UpdateChannelPlatformRouteInput) error {
	return nil
}

func (r *modelUpdateRepo) DeletePlatformModelRoute(context.Context, uint, uint) error {
	return nil
}

func (r *modelUpdateRepo) ListModelUpstreamSources(context.Context, string, int, int) ([]repository.ChannelModelSourceRow, int64, error) {
	return nil, 0, nil
}

func (r *modelUpdateRepo) ListActiveRoutesByModel(context.Context, string) ([]repository.ChannelUpstreamRouteRow, error) {
	return nil, nil
}

func (r *modelUpdateRepo) ListActiveRouteBindingCodesForUpstream(context.Context, uint) ([]string, error) {
	return nil, nil
}

func (r *modelUpdateRepo) GetLLMSetting(context.Context, string) (*domainchannel.LLMSetting, error) {
	return nil, repository.ErrNotFound
}

func (r *modelUpdateRepo) ListLLMSettings(context.Context) ([]domainchannel.LLMSetting, error) {
	return nil, nil
}

func (r *modelUpdateRepo) UpsertLLMSetting(context.Context, *domainchannel.LLMSetting) error {
	return nil
}

func (r *modelUpdateRepo) GetBreakerErrorClassification(context.Context) (domainchannel.BreakerErrorClassification, error) {
	return domainchannel.BreakerErrorClassification{}, nil
}

func (r *modelUpdateRepo) GetBreakerDefaults(context.Context) (domainchannel.BreakerDefaults, error) {
	return domainchannel.BreakerDefaults{}, nil
}

func (r *modelUpdateRepo) GetRateLimitDefaults(context.Context) (domainchannel.RateLimitDefaults, error) {
	return domainchannel.RateLimitDefaults{}, nil
}

func (r *modelUpdateRepo) DeleteUpstreamCascade(context.Context, uint) error {
	return nil
}

func (r *modelUpdateRepo) DeleteModelCascade(context.Context, uint) error {
	return nil
}

var _ repository.ChannelRepository = (*modelUpdateRepo)(nil)
