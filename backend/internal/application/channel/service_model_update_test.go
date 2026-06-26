package channel

import (
	"context"
	"testing"

	domainchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/channel"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/cache/memory"
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

func TestUpdateModelUpstreamSourceUpdatesRouteCircuitSettings(t *testing.T) {
	repo := &modelUpdateRepo{
		model: domainchannel.PlatformModel{
			ID:                1,
			PlatformModelName: "gpt-5.1",
			Vendor:            "openai",
			KindsJSON:         `["chat"]`,
			Icon:              "openai",
			AccessScope:       "public",
			Status:            "active",
		},
		source: repository.ChannelModelSourceRow{
			PlatformModelRoute: domainchannel.PlatformModelRoute{
				ID:              9,
				PlatformModelID: 1,
				UpstreamModelID: 7,
				Protocol:        "openai_responses",
				Status:          "active",
				Priority:        1,
				Weight:          1,
			},
			UpstreamID:             3,
			UpstreamName:           "OpenAI",
			BaseURL:                "https://api.openai.com/v1",
			BindingCode:            "upm_7",
			UpstreamModelName:      "gpt-5.1",
			UpstreamModelKindsJSON: `["chat"]`,
			UpstreamModelStatus:    "active",
		},
	}
	service := NewService(config.Config{}, repo, nil, nil)

	threshold := 4
	duration := 15
	window := 5
	view, err := service.UpdateModelUpstreamSource(context.Background(), 1, 9, UpdateModelUpstreamSourceInput{
		CbFailureThreshold: &threshold,
		CbDurationMin:      &duration,
		CbWindowMin:        &window,
	})
	if err != nil {
		t.Fatalf("UpdateModelUpstreamSource() error = %v", err)
	}
	if repo.lastRouteUpdate.CbFailureThreshold == nil || *repo.lastRouteUpdate.CbFailureThreshold != threshold {
		t.Fatalf("expected threshold update %d, got %#v", threshold, repo.lastRouteUpdate.CbFailureThreshold)
	}
	if repo.lastRouteUpdate.CbDurationMin == nil || *repo.lastRouteUpdate.CbDurationMin != duration {
		t.Fatalf("expected duration update %d, got %#v", duration, repo.lastRouteUpdate.CbDurationMin)
	}
	if repo.lastRouteUpdate.CbWindowMin == nil || *repo.lastRouteUpdate.CbWindowMin != window {
		t.Fatalf("expected window update %d, got %#v", window, repo.lastRouteUpdate.CbWindowMin)
	}
	if view.CbFailureThreshold != threshold || view.CbDurationMin != duration || view.CbWindowMin != window {
		t.Fatalf("expected returned source circuit settings, got %#v", view)
	}
}

func TestListModelsNormalizesCircuitOpenSourceCount(t *testing.T) {
	ctx := context.Background()
	cache := memory.NewChannelCache(memory.New())
	if err := cache.OpenModelCircuit(ctx, 10, bindingCircuitKey("upm_a")); err != nil {
		t.Fatalf("OpenModelCircuit() error = %v", err)
	}
	repo := &modelUpdateRepo{
		modelRows: []repository.ChannelModelListRow{
			{
				PlatformModel: domainchannel.PlatformModel{
					ID:                1,
					PlatformModelName: "gpt-test",
					Status:            "active",
				},
				SourceCount:       2,
				ActiveSourceCount: 2,
			},
		},
		sources: []repository.ChannelModelSourceRow{
			{PlatformModelRoute: domainchannel.PlatformModelRoute{ID: 1, Status: "active"}, UpstreamID: 10, BindingCode: "upm_a", UpstreamStatus: "active", UpstreamModelStatus: "active"},
			{PlatformModelRoute: domainchannel.PlatformModelRoute{ID: 2, Status: "active"}, UpstreamID: 11, BindingCode: "upm_b", UpstreamStatus: "active", UpstreamModelStatus: "active"},
		},
	}
	service := NewService(config.Config{}, repo, cache, nil)

	items, _, err := service.ListModels(ctx, 1, 20, ListModelsInput{})
	if err != nil {
		t.Fatalf("ListModels() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected one model, got %d", len(items))
	}
	if items[0].ActiveSourceCount != 1 {
		t.Fatalf("expected one active source after circuit normalization, got %d", items[0].ActiveSourceCount)
	}
}

func TestListUpstreamsNormalizesCircuitOpenModelCount(t *testing.T) {
	ctx := context.Background()
	cache := memory.NewChannelCache(memory.New())
	if err := cache.OpenModelCircuit(ctx, 1, bindingCircuitKey("upm_a")); err != nil {
		t.Fatalf("OpenModelCircuit() error = %v", err)
	}
	repo := &modelUpdateRepo{
		upstreamRows: []repository.ChannelUpstreamListRow{
			{
				Upstream: domainchannel.Upstream{
					ID:     1,
					Name:   "openrouter",
					Status: "active",
				},
				ModelsCount:       2,
				ActiveModelsCount: 2,
			},
		},
		activeBindingCodes: []string{"upm_a", "upm_b"},
	}
	service := NewService(config.Config{}, repo, cache, nil)

	items, _, err := service.ListUpstreams(ctx, 1, 20, ListUpstreamsInput{})
	if err != nil {
		t.Fatalf("ListUpstreams() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected one upstream, got %d", len(items))
	}
	if items[0].ActiveModelsCount != 1 {
		t.Fatalf("expected one active upstream model after circuit normalization, got %d", items[0].ActiveModelsCount)
	}
}

type modelUpdateRepo struct {
	model              domainchannel.PlatformModel
	modelRows          []repository.ChannelModelListRow
	upstreamRows       []repository.ChannelUpstreamListRow
	activeBindingCodes []string
	source             repository.ChannelModelSourceRow
	sources            []repository.ChannelModelSourceRow
	lastUpdate         repository.UpdateChannelModelInput
	lastRouteUpdate    repository.UpdateChannelPlatformRouteInput
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

func (r *modelUpdateRepo) GetUpstreamListRowByID(context.Context, uint) (*repository.ChannelUpstreamListRow, error) {
	return nil, repository.ErrNotFound
}

func (r *modelUpdateRepo) ListUpstreams(context.Context, repository.ListChannelUpstreamsInput) ([]repository.ChannelUpstreamListRow, int64, error) {
	return r.upstreamRows, int64(len(r.upstreamRows)), nil
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
	if input.CbFailureThreshold != nil {
		r.model.CbFailureThreshold = *input.CbFailureThreshold
	}
	if input.CbDurationMin != nil {
		r.model.CbDurationMin = *input.CbDurationMin
	}
	if input.CbWindowMin != nil {
		r.model.CbWindowMin = *input.CbWindowMin
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
	if len(r.modelRows) > 0 {
		row := r.modelRows[0]
		return &row, nil
	}
	return &repository.ChannelModelListRow{PlatformModel: r.model}, nil
}

func (r *modelUpdateRepo) GetModelByName(context.Context, string) (*domainchannel.PlatformModel, error) {
	return nil, repository.ErrNotFound
}

func (r *modelUpdateRepo) GetActiveModelByName(context.Context, string) (*domainchannel.PlatformModel, error) {
	return nil, repository.ErrNotFound
}

func (r *modelUpdateRepo) ListModels(context.Context, repository.ListChannelModelsInput) ([]repository.ChannelModelListRow, int64, error) {
	return r.modelRows, int64(len(r.modelRows)), nil
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
	if r.source.ID == 0 {
		return nil, repository.ErrNotFound
	}
	source := r.source
	return &source, nil
}

func (r *modelUpdateRepo) ListPlatformModelRoutesByPair(context.Context, uint, uint, uint) ([]domainchannel.PlatformModelRoute, error) {
	return nil, nil
}

func (r *modelUpdateRepo) GetPlatformModelRouteByID(context.Context, uint, uint) (*domainchannel.PlatformModelRoute, error) {
	return nil, repository.ErrNotFound
}

func (r *modelUpdateRepo) UpdatePlatformModelRouteByID(_ context.Context, _ uint, _ uint, input repository.UpdateChannelPlatformRouteInput) error {
	r.lastRouteUpdate = input
	if input.Protocol != nil {
		r.source.Protocol = *input.Protocol
	}
	if input.Status != nil {
		r.source.Status = *input.Status
	}
	if input.Priority != nil {
		r.source.Priority = *input.Priority
	}
	if input.Weight != nil {
		r.source.Weight = *input.Weight
	}
	if input.CbFailureThreshold != nil {
		r.source.CbFailureThreshold = *input.CbFailureThreshold
	}
	if input.CbDurationMin != nil {
		r.source.CbDurationMin = *input.CbDurationMin
	}
	if input.CbWindowMin != nil {
		r.source.CbWindowMin = *input.CbWindowMin
	}
	return nil
}

func (r *modelUpdateRepo) DeletePlatformModelRoute(context.Context, uint, uint) error {
	return nil
}

func (r *modelUpdateRepo) ListModelUpstreamSources(context.Context, string, int, int) ([]repository.ChannelModelSourceRow, int64, error) {
	return r.sources, int64(len(r.sources)), nil
}

func (r *modelUpdateRepo) ListActiveRoutesByModel(context.Context, string) ([]repository.ChannelUpstreamRouteRow, error) {
	return nil, nil
}

func (r *modelUpdateRepo) ListActiveRouteBindingCodesForUpstream(context.Context, uint) ([]string, error) {
	return r.activeBindingCodes, nil
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
