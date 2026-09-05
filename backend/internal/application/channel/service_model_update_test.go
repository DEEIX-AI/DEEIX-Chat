package channel

import (
	"context"
	"errors"
	"reflect"
	"testing"

	domainchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/channel"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/cache/memory"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
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
	service := newTestService(config.Config{}, repo, repo, nil, nil)

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

func TestUpdateModelUsesCatalogVendorAndOptionalDisplayGroup(t *testing.T) {
	repo := &modelUpdateRepo{
		model: domainchannel.PlatformModel{
			ID:                1,
			PlatformModelName: "acme-chat",
			Vendor:            "unknown",
			KindsJSON:         `["chat"]`,
			AccessScope:       "public",
			Status:            "active",
		},
	}
	service := newTestService(config.Config{}, repo, repo, nil, nil)

	vendor := "acme-ai"
	displayGroupID := uint(7)
	view, err := service.UpdateModel(context.Background(), 1, UpdateModelInput{
		Vendor:         &vendor,
		DisplayGroupID: &displayGroupID,
	})
	if err != nil {
		t.Fatalf("UpdateModel() error = %v", err)
	}
	if repo.lastUpdate.Vendor == nil || *repo.lastUpdate.Vendor != vendor {
		t.Fatalf("expected vendor %q, got %#v", vendor, repo.lastUpdate.Vendor)
	}
	if repo.lastUpdate.DisplayGroupID == nil || *repo.lastUpdate.DisplayGroupID != displayGroupID {
		t.Fatalf("expected display group %d, got %#v", displayGroupID, repo.lastUpdate.DisplayGroupID)
	}
	if view.DisplayGroupID == nil || *view.DisplayGroupID != displayGroupID {
		t.Fatalf("expected returned display group %d, got %#v", displayGroupID, view.DisplayGroupID)
	}

	clearGroupID := uint(0)
	view, err = service.UpdateModel(context.Background(), 1, UpdateModelInput{DisplayGroupID: &clearGroupID})
	if err != nil {
		t.Fatalf("UpdateModel(clear group) error = %v", err)
	}
	if repo.lastUpdate.DisplayGroupID == nil || *repo.lastUpdate.DisplayGroupID != 0 {
		t.Fatalf("expected explicit group clear, got %#v", repo.lastUpdate.DisplayGroupID)
	}
	if view.DisplayGroupID != nil {
		t.Fatalf("expected returned display group to be nil, got %#v", view.DisplayGroupID)
	}
}

func TestUpdateModelRejectsInvalidModelCapsWithDedicatedError(t *testing.T) {
	repo := &modelUpdateRepo{
		model: domainchannel.PlatformModel{
			ID:                1,
			PlatformModelName: "custom-model",
			Vendor:            "openai",
			KindsJSON:         `["chat"]`,
			AccessScope:       "public",
			Status:            "active",
		},
	}
	service := newTestService(config.Config{}, repo, repo, nil, nil)
	capabilities := `{"contextWindow":4096,"maxOutputTokens":4096}`

	_, err := service.UpdateModel(context.Background(), 1, UpdateModelInput{CapabilitiesJSON: &capabilities})
	if !errors.Is(err, ErrInvalidModelCapsConfig) {
		t.Fatalf("UpdateModel() error = %v, want ErrInvalidModelCapsConfig", err)
	}
}

func TestUpdateModelClearsAutomaticContextWindowWhenIdentityChanges(t *testing.T) {
	repo := &modelUpdateRepo{
		model: domainchannel.PlatformModel{
			ID:                1,
			PlatformModelName: "claude-sonnet-4.5",
			Vendor:            "anthropic",
			KindsJSON:         `["chat"]`,
			CapabilitiesJSON:  `{"contextWindow":200000,"_deeixContextWindowMode":"auto","maxOutputTokens":8192}`,
			AccessScope:       "public",
			Status:            "active",
		},
	}
	service := newTestService(config.Config{}, repo, repo, nil, nil)
	name := "claude-sonnet-4.6"

	_, err := service.UpdateModel(context.Background(), 1, UpdateModelInput{PlatformModelName: &name})
	if err != nil {
		t.Fatalf("UpdateModel() error = %v", err)
	}
	if repo.lastUpdate.CapabilitiesJSON == nil {
		t.Fatal("expected stale automatic context window to be cleared")
	}
	want := `{"maxOutputTokens":8192}`
	if *repo.lastUpdate.CapabilitiesJSON != want {
		t.Fatalf("CapabilitiesJSON = %q, want %q", *repo.lastUpdate.CapabilitiesJSON, want)
	}
}

func TestUpdateModelPreservesManualContextWindowWhenIdentityChanges(t *testing.T) {
	repo := &modelUpdateRepo{
		model: domainchannel.PlatformModel{
			ID:                1,
			PlatformModelName: "private-model-v1",
			Vendor:            "unknown",
			KindsJSON:         `["chat"]`,
			CapabilitiesJSON:  `{"contextWindow":256000}`,
			AccessScope:       "public",
			Status:            "active",
		},
	}
	service := newTestService(config.Config{}, repo, repo, nil, nil)
	name := "private-model-v2"

	_, err := service.UpdateModel(context.Background(), 1, UpdateModelInput{PlatformModelName: &name})
	if err != nil {
		t.Fatalf("UpdateModel() error = %v", err)
	}
	if repo.lastUpdate.CapabilitiesJSON != nil {
		t.Fatalf("manual capabilities must be preserved, got update %q", *repo.lastUpdate.CapabilitiesJSON)
	}
	if repo.model.CapabilitiesJSON != `{"contextWindow":256000}` {
		t.Fatalf("manual capabilities changed to %q", repo.model.CapabilitiesJSON)
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
	service := newTestService(config.Config{}, repo, repo, nil, nil)

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

func TestParseCircuitBreakerDefaultsValidatesAndAppliesDefaults(t *testing.T) {
	for _, value := range []string{
		`{"enabled":false,"model_failure_threshold":5}`,
		`{"model_failure_threshold":5}`,
	} {
		if _, err := parseCircuitBreakerDefaults(value); err != nil {
			t.Fatalf("parseCircuitBreakerDefaults(%q) error = %v", value, err)
		}
	}
	for _, value := range []string{
		`[]`,
		`null`,
		`{"enabled":null}`,
		`{"enabled":"false"}`,
		`{"model_failure_threshold":"5"}`,
		`{"model_failure_threshold":-1}`,
		`{"upstream_window_min":-1}`,
		`{"upstream_threshold_logic":"xor"}`,
	} {
		if _, err := parseCircuitBreakerDefaults(value); !errors.Is(err, ErrInvalidJSONConfig) {
			t.Fatalf("parseCircuitBreakerDefaults(%q) error = %v, want ErrInvalidJSONConfig", value, err)
		}
	}
	parsed, err := parseCircuitBreakerDefaults(`{"enabled":true,"model_failure_threshold":9}`)
	if err != nil || !parsed.Enabled || parsed.ModelFailureThreshold != 9 || parsed.ModelDurationMin != 15 {
		t.Fatalf("unexpected parsed defaults: %#v, error=%v", parsed, err)
	}
}

func TestUpdateCircuitBreakerDefaultsClearsExistingStates(t *testing.T) {
	cache := memory.NewChannelCache(memory.New())
	if err := cache.OpenUpstreamCircuit(t.Context(), 1); err != nil {
		t.Fatalf("OpenUpstreamCircuit() error = %v", err)
	}
	repo := &modelUpdateRepo{llmSetting: domainchannel.LLMSetting{
		Key:   "circuit_breaker.defaults",
		Value: `{"enabled":true}`,
	}}
	service := newTestService(config.Config{}, repo, repo, cache, nil)

	if _, err := service.UpdateLLMSetting(t.Context(), "circuit_breaker.defaults", `{"enabled":false}`); err != nil {
		t.Fatalf("UpdateLLMSetting() error = %v", err)
	}
	if open, _ := cache.QueryUpstreamCircuitStatus(t.Context(), 1); open {
		t.Fatal("expected setting update to clear existing circuit state")
	}
}

func TestUpdateCircuitBreakerDefaultsDoesNotClearEnabledStateBeforeFailedWrite(t *testing.T) {
	cache := memory.NewChannelCache(memory.New())
	if err := cache.OpenUpstreamCircuit(t.Context(), 1); err != nil {
		t.Fatalf("OpenUpstreamCircuit() error = %v", err)
	}
	writeErr := errors.New("write failed")
	repo := &modelUpdateRepo{
		llmSetting: domainchannel.LLMSetting{
			Key:   "circuit_breaker.defaults",
			Value: `{"enabled":true,"model_failure_threshold":5}`,
		},
		upsertLLMSettingErr: writeErr,
	}
	service := newTestService(config.Config{}, repo, repo, cache, nil)

	if _, err := service.UpdateLLMSetting(t.Context(), "circuit_breaker.defaults", `{"enabled":true,"model_failure_threshold":7}`); !errors.Is(err, writeErr) {
		t.Fatalf("UpdateLLMSetting() error = %v, want %v", err, writeErr)
	}
	if open, _ := cache.QueryUpstreamCircuitStatus(t.Context(), 1); !open {
		t.Fatal("expected failed enabled-to-enabled update to preserve existing circuit state")
	}
}

func TestUpdateCircuitBreakerDefaultsClearsStateBeforeEnabling(t *testing.T) {
	cache := memory.NewChannelCache(memory.New())
	if err := cache.OpenUpstreamCircuit(t.Context(), 1); err != nil {
		t.Fatalf("OpenUpstreamCircuit() error = %v", err)
	}
	repo := &modelUpdateRepo{llmSetting: domainchannel.LLMSetting{
		Key:   "circuit_breaker.defaults",
		Value: `{"enabled":false}`,
	}}
	service := newTestService(config.Config{}, repo, repo, cache, nil)

	if _, err := service.UpdateLLMSetting(t.Context(), "circuit_breaker.defaults", `{"enabled":true}`); err != nil {
		t.Fatalf("UpdateLLMSetting() error = %v", err)
	}
	if open, _ := cache.QueryUpstreamCircuitStatus(t.Context(), 1); open {
		t.Fatal("expected enabling to start from a clean circuit state")
	}
	if !service.loadBreakerDefaults(t.Context()).Enabled {
		t.Fatal("expected successful update to enable the local breaker cache immediately")
	}
}

func TestOpenCircuitRejectsWhenBreakerDisabled(t *testing.T) {
	repo := &modelUpdateRepo{
		breakerDefaults: domainchannel.BreakerDefaults{Enabled: false},
		upstream:        domainchannel.Upstream{ID: 1},
		upstreamModelRoute: repository.ChannelUpstreamModelListRow{
			UpstreamModel: domainchannel.UpstreamModel{ID: 1, UpstreamID: 1, BindingCode: "upm_1"},
			RouteID:       1,
		},
	}
	service := newTestService(config.Config{}, repo, repo, memory.NewChannelCache(memory.New()), nil)

	if err := service.OpenUpstreamCircuit(t.Context(), 1); !errors.Is(err, ErrCircuitBreakerDisabled) {
		t.Fatalf("OpenUpstreamCircuit() error = %v, want ErrCircuitBreakerDisabled", err)
	}
	if err := service.OpenUpstreamModelCircuit(t.Context(), 1, 1); !errors.Is(err, ErrCircuitBreakerDisabled) {
		t.Fatalf("OpenUpstreamModelCircuit() error = %v, want ErrCircuitBreakerDisabled", err)
	}
}

func TestOpenCircuitValidatesTargetBeforeGlobalState(t *testing.T) {
	repo := &modelUpdateRepo{breakerDefaults: domainchannel.BreakerDefaults{Enabled: false}}
	service := newTestService(config.Config{}, repo, repo, memory.NewChannelCache(memory.New()), nil)

	if err := service.OpenUpstreamCircuit(t.Context(), 1); !errors.Is(err, ErrUpstreamNotFound) {
		t.Fatalf("OpenUpstreamCircuit() error = %v, want ErrUpstreamNotFound", err)
	}
	if err := service.OpenUpstreamModelCircuit(t.Context(), 1, 1); !errors.Is(err, ErrUpstreamModelNotFound) {
		t.Fatalf("OpenUpstreamModelCircuit() error = %v, want ErrUpstreamModelNotFound", err)
	}
}

func TestListModelsNormalizesCircuitOpenSourceCount(t *testing.T) {
	ctx := context.Background()
	cache := memory.NewChannelCache(memory.New())
	if err := cache.OpenModelCircuit(ctx, 10, bindingCircuitKey("upm_a")); err != nil {
		t.Fatalf("OpenModelCircuit() error = %v", err)
	}
	repo := &modelUpdateRepo{
		breakerDefaults: domainchannel.BreakerDefaults{Enabled: true},
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
	service := newTestService(config.Config{}, repo, repo, cache, nil)

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

func TestListModelsSkipsCircuitSourceQueriesWhenBreakerDisabled(t *testing.T) {
	repo := &modelUpdateRepo{
		breakerDefaults: domainchannel.BreakerDefaults{Enabled: false},
		modelRows: []repository.ChannelModelListRow{{
			PlatformModel: domainchannel.PlatformModel{ID: 1, PlatformModelName: "gpt-test", Status: "active"},
			SourceCount:   2, ActiveSourceCount: 2,
		}},
	}
	service := newTestService(config.Config{}, repo, repo, memory.NewChannelCache(memory.New()), nil)

	items, _, err := service.ListModels(t.Context(), 1, 20, ListModelsInput{})
	if err != nil {
		t.Fatalf("ListModels() error = %v", err)
	}
	if len(items) != 1 || items[0].ActiveSourceCount != 2 {
		t.Fatalf("unexpected models: %#v", items)
	}
	if repo.sourceListCalls != 0 {
		t.Fatalf("source list calls = %d, want 0", repo.sourceListCalls)
	}
}

func TestListUpstreamsNormalizesCircuitOpenModelCount(t *testing.T) {
	ctx := context.Background()
	cache := memory.NewChannelCache(memory.New())
	if err := cache.OpenModelCircuit(ctx, 1, bindingCircuitKey("upm_a")); err != nil {
		t.Fatalf("OpenModelCircuit() error = %v", err)
	}
	repo := &modelUpdateRepo{
		breakerDefaults: domainchannel.BreakerDefaults{Enabled: true},
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
	service := newTestService(config.Config{}, repo, repo, cache, nil)

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

func TestSetModelsDisplayGroupNormalizesIDsAndMapsRepositoryErrors(t *testing.T) {
	repo := &modelUpdateRepo{}
	service := newTestService(config.Config{}, repo, repo, nil, nil)

	if err := service.SetModelsDisplayGroup(t.Context(), []uint{3, 3, 7}, 9); err != nil {
		t.Fatalf("SetModelsDisplayGroup() error = %v", err)
	}
	if !reflect.DeepEqual(repo.lastDisplayGroupModelIDs, []uint{3, 7}) || repo.lastDisplayGroupID != 9 {
		t.Fatalf("unexpected display group assignment: ids=%v group=%d", repo.lastDisplayGroupModelIDs, repo.lastDisplayGroupID)
	}

	repo.setDisplayGroupErr = repository.ErrNotFound
	if err := service.SetModelsDisplayGroup(t.Context(), []uint{3}, 99); !errors.Is(err, ErrModelDisplayGroupNotFound) {
		t.Fatalf("expected display group not found, got %v", err)
	}
	repo.setDisplayGroupErr = repository.ErrInvalidInput
	if err := service.SetModelsDisplayGroup(t.Context(), []uint{3}, 9); !errors.Is(err, ErrInvalidModelDisplayGroup) {
		t.Fatalf("expected invalid display group assignment, got %v", err)
	}
}

func TestDeleteModelVendorMapsStructuredBlockers(t *testing.T) {
	repo := &modelUpdateRepo{deleteVendorErr: &repository.ModelVendorDeleteBlockedError{
		Reason:         repository.ModelVendorDeleteReasonReferencedModels,
		ReferenceCount: 2,
		Models:         []repository.ModelVendorReference{{ID: 7, PlatformModelName: "acme-chat"}},
	}}
	service := newTestService(config.Config{}, repo, repo, nil, nil)

	err := service.DeleteModelVendor(t.Context(), "acme")
	var blocked *ModelVendorDeleteBlockedError
	if !errors.As(err, &blocked) || !errors.Is(err, ErrModelVendorInUse) {
		t.Fatalf("expected model vendor in-use error, got %v", err)
	}
	if blocked.ReferenceCount != 2 || len(blocked.Models) != 1 || blocked.Models[0].PlatformModelName != "acme-chat" {
		t.Fatalf("unexpected blocker details: %#v", blocked)
	}
}

func TestSetModelProtocolsReplacesEveryBindingInOneTransaction(t *testing.T) {
	templateRoute := modelProtocolSource(1, 10, 100, "openai_image_edits")
	templateRoute.Priority = 2
	templateRoute.Weight = 3
	retainedRoute := modelProtocolSource(2, 10, 100, "openai_image_generations")
	retainedRoute.Status = "inactive"
	retainedRoute.Priority = 7
	retainedRoute.Weight = 11
	retainedRoute.Source = "manual"
	retainedRoute.CbFailureThreshold = 13
	retainedRoute.CbDurationMin = 17
	retainedRoute.CbWindowMin = 19
	retainedRoute.HeadersJSON = `{"X-Route":"generation"}`
	repo := &modelUpdateRepo{
		model: domainchannel.PlatformModel{
			ID:                1,
			PlatformModelName: "image-model",
			KindsJSON:         `["image_gen","image_edit"]`,
			Status:            "active",
		},
		sources: []repository.ChannelModelSourceRow{
			templateRoute,
			retainedRoute,
			modelProtocolSource(3, 20, 200, "openai_image_generations"),
		},
	}
	service := newTestService(config.Config{}, repo, repo, nil, nil)

	view, err := service.SetModelProtocols(t.Context(), 1, SetModelProtocolsInput{
		Protocols: []string{"openai_image_generations"},
		KindsJSON: `["image_gen"]`,
	})
	if err != nil {
		t.Fatalf("SetModelProtocols() error = %v", err)
	}
	if !repo.transactionCommitted {
		t.Fatal("expected protocol update transaction to commit")
	}
	if view.KindsJSON != `["image_gen"]` {
		t.Fatalf("expected updated kinds, got %q", view.KindsJSON)
	}
	if len(repo.routeReplacements) != 2 {
		t.Fatalf("expected two complete binding replacements, got %d", len(repo.routeReplacements))
	}
	if !reflect.DeepEqual(repo.routeReplacements[0].ExistingRouteIDs, []uint{1, 2}) {
		t.Fatalf("expected complete first binding route IDs, got %v", repo.routeReplacements[0].ExistingRouteIDs)
	}
	if !reflect.DeepEqual(repo.routeReplacements[1].ExistingRouteIDs, []uint{3}) {
		t.Fatalf("expected complete second binding route IDs, got %v", repo.routeReplacements[1].ExistingRouteIDs)
	}
	for _, replacement := range repo.routeReplacements {
		if len(replacement.Routes) != 1 || replacement.Routes[0].Protocol != "openai_image_generations" {
			t.Fatalf("unexpected replacement routes: %#v", replacement.Routes)
		}
	}
	preserved := repo.routeReplacements[0].Routes[0]
	if preserved.Status != retainedRoute.Status ||
		preserved.Priority != retainedRoute.Priority ||
		preserved.Weight != retainedRoute.Weight ||
		preserved.Source != retainedRoute.Source ||
		preserved.CbFailureThreshold != retainedRoute.CbFailureThreshold ||
		preserved.CbDurationMin != retainedRoute.CbDurationMin ||
		preserved.CbWindowMin != retainedRoute.CbWindowMin ||
		preserved.HeadersJSON != retainedRoute.HeadersJSON {
		t.Fatalf("expected retained protocol configuration to be preserved, got %#v", preserved)
	}
}

func TestSetModelProtocolsDoesNotLimitSourceCount(t *testing.T) {
	const sourceCount = 1001
	sources := make([]repository.ChannelModelSourceRow, 0, sourceCount)
	for index := 0; index < sourceCount; index++ {
		sources = append(sources, modelProtocolSource(uint(index+1), uint(index+10), uint(index+100), "openai_responses"))
	}
	repo := &modelUpdateRepo{
		model:   domainchannel.PlatformModel{ID: 1, PlatformModelName: "large-model", KindsJSON: `["chat"]`, Status: "active"},
		sources: sources,
	}
	service := newTestService(config.Config{}, repo, repo, nil, nil)

	if _, err := service.SetModelProtocols(t.Context(), 1, SetModelProtocolsInput{
		Protocols: []string{"openai_responses"},
		KindsJSON: `["chat"]`,
	}); err != nil {
		t.Fatalf("SetModelProtocols() error = %v", err)
	}
	if len(repo.routeReplacements) != sourceCount {
		t.Fatalf("expected all %d bindings to be replaced, got %d", sourceCount, len(repo.routeReplacements))
	}
}

func TestSetModelProtocolsRollsBackWhenAReplacementConflicts(t *testing.T) {
	repo := &modelUpdateRepo{
		model: domainchannel.PlatformModel{ID: 1, PlatformModelName: "conflict-model", KindsJSON: `["chat"]`, Status: "active"},
		sources: []repository.ChannelModelSourceRow{
			modelProtocolSource(1, 10, 100, "openai_chat_completions"),
			modelProtocolSource(2, 20, 200, "openai_chat_completions"),
		},
		replaceErrAt: 2,
	}
	service := newTestService(config.Config{}, repo, repo, nil, nil)

	_, err := service.SetModelProtocols(t.Context(), 1, SetModelProtocolsInput{
		Protocols: []string{"openai_responses"},
		KindsJSON: `["chat"]`,
	})
	if !errors.Is(err, ErrUpstreamModelBindingChanged) {
		t.Fatalf("expected binding changed error, got %v", err)
	}
	if repo.transactionCommitted {
		t.Fatal("expected outer transaction to roll back")
	}
}

func TestSetModelProtocolsRejectsMalformedExplicitSets(t *testing.T) {
	service := newTestService(config.Config{}, &modelUpdateRepo{}, &modelUpdateRepo{}, nil, nil)
	tests := []struct {
		name      string
		protocols []string
		want      error
	}{
		{name: "missing", want: ErrProtocolRequired},
		{name: "blank", protocols: []string{" "}, want: ErrInvalidAdapter},
		{name: "normalized duplicate", protocols: []string{"openai_responses", " OPENAI_RESPONSES "}, want: ErrInvalidRouteProtocolCombination},
		{name: "too many", protocols: []string{"openai_responses", "openai_chat_completions", "anthropic_messages"}, want: ErrInvalidRouteProtocolCombination},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := service.SetModelProtocols(t.Context(), 1, SetModelProtocolsInput{
				Protocols: test.protocols,
				KindsJSON: `["chat"]`,
			})
			if !errors.Is(err, test.want) {
				t.Fatalf("expected %v, got %v", test.want, err)
			}
		})
	}
}

func modelProtocolSource(routeID uint, upstreamID uint, upstreamModelID uint, protocol string) repository.ChannelModelSourceRow {
	return repository.ChannelModelSourceRow{
		PlatformModelRoute: domainchannel.PlatformModelRoute{
			ID:              routeID,
			PlatformModelID: 1,
			UpstreamModelID: upstreamModelID,
			Protocol:        protocol,
			Status:          "active",
			Priority:        1,
			Weight:          1,
		},
		UpstreamID:         upstreamID,
		UpstreamCompatible: "openai",
	}
}

func TestReconcileRemoteModelSnapshotSoftlyReconcilesManagedCatalog(t *testing.T) {
	repo := &modelUpdateRepo{upstreamModels: map[string]domainchannel.UpstreamModel{
		"returning": {
			ID: 1, UpstreamID: 9, BindingCode: "returning-code", UpstreamModelName: "returning", Status: "inactive", Source: "import",
		},
		"manual-model": {
			ID: 2, UpstreamID: 9, BindingCode: "manual-code", UpstreamModelName: "manual-model", Status: "inactive", Source: "manual",
		},
		"removed": {
			ID: 3, UpstreamID: 9, BindingCode: "removed-code", UpstreamModelName: "removed", Status: "active", Source: "sync",
		},
	}}
	service := newTestService(config.Config{}, repo, repo, nil, nil)
	result, err := service.reconcileRemoteModelSnapshot(t.Context(), &domainchannel.Upstream{
		ID: 9, Name: "test", Compatible: "openai", BaseURL: "https://example.com",
	}, []llm.ModelItem{
		{ID: " new-model ", OwnedBy: "openai"},
		{ID: "returning", OwnedBy: "openai"},
		{ID: "manual-model", OwnedBy: "custom"},
		{ID: "new-model", OwnedBy: "duplicate"},
		{ID: " "},
	}, false)
	if err != nil {
		t.Fatalf("reconcile snapshot: %v", err)
	}
	if result.TotalUpstream != 3 || result.CreatedUpstreamModels != 1 || result.ExistingUpstreamModels != 2 {
		t.Fatalf("unexpected sync counts: %+v", result)
	}
	if result.ReactivatedModels != 1 || result.ProtectedUpstreamModels != 1 || result.InactivatedModels != 1 {
		t.Fatalf("unexpected availability counts: %+v", result)
	}
	if result.UpdatedUpstreamModels != 0 || result.UnchangedUpstreamModels != 0 {
		t.Fatalf("expected exclusive catalog categories, got %+v", result)
	}
	if categorized := result.CreatedUpstreamModels + result.UpdatedUpstreamModels + result.ReactivatedModels + result.UnchangedUpstreamModels + result.ProtectedUpstreamModels; categorized != result.TotalUpstream {
		t.Fatalf("categorized remote models = %d, want %d", categorized, result.TotalUpstream)
	}
	if got := repo.upstreamModels["returning"]; got.Status != "active" || got.Source != "sync" {
		t.Fatalf("expected legacy imported model to be restored and migrated, got %+v", got)
	}
	if got := repo.upstreamModels["manual-model"]; got.Status != "inactive" || got.Source != "manual" {
		t.Fatalf("expected manual model to remain untouched, got %+v", got)
	}
	if got := repo.upstreamModels["removed"]; got.Status != "inactive" {
		t.Fatalf("expected missing managed model to be inactive, got %+v", got)
	}
}

func TestReconcileRemoteModelSnapshotRequiresConfirmationForEmptyCatalog(t *testing.T) {
	repo := &modelUpdateRepo{upstreamModels: map[string]domainchannel.UpstreamModel{
		"existing": {ID: 1, UpstreamID: 9, UpstreamModelName: "existing", Status: "active", Source: "sync"},
	}}
	service := newTestService(config.Config{}, repo, repo, nil, nil)
	upstream := &domainchannel.Upstream{ID: 9}

	if _, err := service.reconcileRemoteModelSnapshot(t.Context(), upstream, nil, false); !errors.Is(err, ErrEmptyRemoteModels) {
		t.Fatalf("expected empty snapshot error, got %v", err)
	}
	if repo.catalogApplyCalls != 0 {
		t.Fatalf("empty snapshot changed data without confirmation")
	}

	result, err := service.reconcileRemoteModelSnapshot(t.Context(), upstream, nil, true)
	if err != nil {
		t.Fatalf("confirmed empty snapshot: %v", err)
	}
	if result.InactivatedModels != 1 || repo.upstreamModels["existing"].Status != "inactive" {
		t.Fatalf("expected confirmed empty snapshot to deactivate managed catalog, got %+v", result)
	}
}

func TestBuildUpstreamModelSyncPlanSeparatesCatalogActions(t *testing.T) {
	upstream := &domainchannel.Upstream{ID: 9, Name: "test", Compatible: "openai", BaseURL: "https://example.com"}
	unchangedItem := llm.ModelItem{ID: "unchanged", OwnedBy: "openai"}
	unchangedKinds := inferKindsJSON(unchangedItem.ID)
	unchangedProtocol, err := resolveRouteProtocol("", upstream.Compatible, upstream.ProtocolDefaultsJSON, unchangedKinds)
	if err != nil {
		t.Fatalf("resolve unchanged protocol: %v", err)
	}
	unchanged := *syncedUpstreamModel(upstream, unchangedItem, "unchanged-code", nil, unchangedProtocol, unchangedKinds)
	unchanged.ID = 1
	updated := unchanged
	updated.ID = 2
	updated.BindingCode = "updated-code"
	updated.UpstreamModelName = "updated"
	updated.Vendor = "stale-vendor"
	updated.RawJSON = `{}`

	plan, err := buildUpstreamModelSyncPlan(
		upstream,
		[]llm.ModelItem{
			{ID: "added", OwnedBy: "openai"},
			{ID: "manual", OwnedBy: "custom"},
			{ID: "reactivated", OwnedBy: "openai"},
			unchangedItem,
			{ID: "updated", OwnedBy: "openai"},
		},
		[]domainchannel.UpstreamModel{
			unchanged,
			updated,
			{ID: 3, UpstreamID: 9, BindingCode: "reactivated-code", UpstreamModelName: "reactivated", Status: "inactive", Source: "sync"},
			{ID: 4, UpstreamID: 9, BindingCode: "removed-code", UpstreamModelName: "removed", Status: "active", Source: "sync"},
		},
		map[string]repositoryUpstreamModelSnapshot{
			"manual": {BindingCode: "manual-code", Status: "active"},
		},
	)
	if err != nil {
		t.Fatalf("build sync plan: %v", err)
	}
	if !reflect.DeepEqual(plan.AddedModels, []string{"added"}) ||
		!reflect.DeepEqual(plan.UpdatedModels, []string{"updated"}) ||
		!reflect.DeepEqual(plan.ReactivatedModels, []string{"reactivated"}) ||
		!reflect.DeepEqual(plan.InactivatedModels, []string{"removed"}) ||
		!reflect.DeepEqual(plan.UnchangedModels, []string{"unchanged"}) ||
		!reflect.DeepEqual(plan.ProtectedModels, []string{"manual"}) {
		t.Fatalf("unexpected sync plan: %+v", plan)
	}
	if remoteModelsSnapshotID([]llm.ModelItem{{ID: "a"}}) == remoteModelsSnapshotID([]llm.ModelItem{{ID: "b"}}) {
		t.Fatal("different remote snapshots produced the same identifier")
	}
}

type modelUpdateRepo struct {
	model                    domainchannel.PlatformModel
	upstream                 domainchannel.Upstream
	upstreamModelRoute       repository.ChannelUpstreamModelListRow
	modelRows                []repository.ChannelModelListRow
	upstreamRows             []repository.ChannelUpstreamListRow
	activeBindingCodes       []string
	source                   repository.ChannelModelSourceRow
	sources                  []repository.ChannelModelSourceRow
	sourceListCalls          int
	lastUpdate               repository.UpdateChannelModelInput
	lastRouteUpdate          repository.UpdateChannelPlatformRouteInput
	lastDisplayGroupModelIDs []uint
	lastDisplayGroupID       uint
	setDisplayGroupErr       error
	deleteVendorErr          error
	transactionCommitted     bool
	routeReplacements        []repository.ReplaceChannelPlatformRoutesInput
	replaceErrAt             int
	breakerDefaults          domainchannel.BreakerDefaults
	llmSetting               domainchannel.LLMSetting
	upsertLLMSettingErr      error
	upstreamModels           map[string]domainchannel.UpstreamModel
	catalogApplyCalls        int
}

func (r *modelUpdateRepo) WithinTransaction(ctx context.Context, fn func(repository.ChannelRepository) error) error {
	err := fn(r)
	r.transactionCommitted = err == nil
	return err
}

func (r *modelUpdateRepo) CreateUpstream(context.Context, *domainchannel.Upstream) error {
	return nil
}

func (r *modelUpdateRepo) UpdateUpstream(context.Context, uint, repository.UpdateChannelUpstreamInput) error {
	return nil
}

func (r *modelUpdateRepo) GetUpstreamByID(context.Context, uint) (*domainchannel.Upstream, error) {
	if r.upstream.ID == 0 {
		return nil, ErrUpstreamNotFound
	}
	item := r.upstream
	return &item, nil
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
	if input.DisplayGroupID != nil {
		if *input.DisplayGroupID == 0 {
			r.model.DisplayGroupID = nil
		} else {
			value := *input.DisplayGroupID
			r.model.DisplayGroupID = &value
		}
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

func (r *modelUpdateRepo) GetActiveRoutableModelKindsJSON(context.Context, string) (string, bool, error) {
	return "", false, nil
}

func (r *modelUpdateRepo) ListModels(context.Context, repository.ListChannelModelsInput) ([]repository.ChannelModelListRow, int64, error) {
	return r.modelRows, int64(len(r.modelRows)), nil
}

func (r *modelUpdateRepo) UpsertUpstreamModel(_ context.Context, item *domainchannel.UpstreamModel) error {
	if r.upstreamModels != nil {
		stored := *item
		if existing, ok := r.upstreamModels[item.UpstreamModelName]; ok {
			stored.ID = existing.ID
		} else if stored.ID == 0 {
			stored.ID = uint(len(r.upstreamModels) + 1)
		}
		r.upstreamModels[item.UpstreamModelName] = stored
	}
	return nil
}

func (r *modelUpdateRepo) CreateUpstreamModel(context.Context, *domainchannel.UpstreamModel) error {
	return nil
}

func (r *modelUpdateRepo) GetUpstreamModelByID(context.Context, uint, uint) (*domainchannel.UpstreamModel, error) {
	return nil, repository.ErrNotFound
}

func (r *modelUpdateRepo) GetUpstreamModelByUpstreamName(_ context.Context, upstreamID uint, name string) (*domainchannel.UpstreamModel, error) {
	if item, ok := r.upstreamModels[name]; ok && item.UpstreamID == upstreamID {
		result := item
		return &result, nil
	}
	return nil, ErrUpstreamModelNotFound
}

func (r *modelUpdateRepo) DeleteUpstreamModel(context.Context, uint, uint) error {
	return nil
}

func (r *modelUpdateRepo) ListManagedUpstreamModels(_ context.Context, upstreamID uint) ([]domainchannel.UpstreamModel, error) {
	items := make([]domainchannel.UpstreamModel, 0)
	for _, item := range r.upstreamModels {
		if item.UpstreamID == upstreamID && (item.Source == "sync" || item.Source == "import") {
			items = append(items, item)
		}
	}
	return items, nil
}

func (r *modelUpdateRepo) ApplyUpstreamModelCatalogChanges(_ context.Context, upstreamID uint, input repository.ApplyUpstreamModelCatalogChangesInput) (int64, error) {
	r.catalogApplyCalls++
	for _, item := range input.Create {
		stored := item
		if stored.ID == 0 {
			stored.ID = uint(len(r.upstreamModels) + 1)
		}
		r.upstreamModels[stored.UpstreamModelName] = stored
	}
	for _, item := range input.Update {
		r.upstreamModels[item.UpstreamModelName] = item
	}
	inactiveIDs := make(map[uint]struct{}, len(input.InactivateIDs))
	for _, id := range input.InactivateIDs {
		inactiveIDs[id] = struct{}{}
	}
	var count int64
	for name, item := range r.upstreamModels {
		if item.UpstreamID != upstreamID || item.Status != "active" || (item.Source != "sync" && item.Source != "import") {
			continue
		}
		if _, exists := inactiveIDs[item.ID]; !exists {
			continue
		}
		item.Status = "inactive"
		r.upstreamModels[name] = item
		count++
	}
	return count, nil
}

func (r *modelUpdateRepo) ListUpstreamModels(context.Context, uint, repository.ListChannelUpstreamModelsInput) ([]repository.ChannelUpstreamModelListRow, int64, error) {
	return nil, 0, nil
}

func (r *modelUpdateRepo) ListUpstreamModelsByNames(_ context.Context, upstreamID uint, names []string) ([]repository.ChannelUpstreamModelListRow, error) {
	items := make([]repository.ChannelUpstreamModelListRow, 0)
	for _, name := range names {
		if item, exists := r.upstreamModels[name]; exists && item.UpstreamID == upstreamID {
			items = append(items, repository.ChannelUpstreamModelListRow{UpstreamModel: item})
		}
	}
	return items, nil
}

func (r *modelUpdateRepo) GetUpstreamModelRouteByID(context.Context, uint, uint) (*repository.ChannelUpstreamModelListRow, error) {
	if r.upstreamModelRoute.RouteID == 0 {
		return nil, ErrUpstreamModelNotFound
	}
	item := r.upstreamModelRoute
	return &item, nil
}

func (r *modelUpdateRepo) GetUpstreamModelRouteByNames(context.Context, uint, string, string, string) (*repository.ChannelUpstreamModelListRow, error) {
	return nil, repository.ErrNotFound
}

func (r *modelUpdateRepo) UpsertPlatformModelRoute(context.Context, *domainchannel.PlatformModelRoute) error {
	return nil
}

func (r *modelUpdateRepo) ReplacePlatformModelRoutes(_ context.Context, inputs []repository.ReplaceChannelPlatformRoutesInput) ([]domainchannel.PlatformModelRoute, error) {
	replaced := make([]domainchannel.PlatformModelRoute, 0)
	for _, input := range inputs {
		r.routeReplacements = append(r.routeReplacements, input)
		if r.replaceErrAt > 0 && len(r.routeReplacements) == r.replaceErrAt {
			return nil, repository.ErrConflict
		}
		replaced = append(replaced, input.Routes...)
	}
	return replaced, nil
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
	r.sourceListCalls++
	return r.sources, int64(len(r.sources)), nil
}

func (r *modelUpdateRepo) ListModelUpstreamSourcesForUpdate(context.Context, string) ([]repository.ChannelModelSourceRow, error) {
	return r.sources, nil
}

func (r *modelUpdateRepo) ListActiveRoutesByModel(context.Context, string) ([]repository.ChannelUpstreamRouteRow, error) {
	return nil, nil
}

func (r *modelUpdateRepo) ListActiveRouteBindingCodesForUpstream(context.Context, uint) ([]string, error) {
	return r.activeBindingCodes, nil
}

func (r *modelUpdateRepo) GetLLMSetting(_ context.Context, key string) (*domainchannel.LLMSetting, error) {
	if r.llmSetting.Key != key {
		return nil, repository.ErrNotFound
	}
	item := r.llmSetting
	return &item, nil
}

func (r *modelUpdateRepo) ListLLMSettings(context.Context) ([]domainchannel.LLMSetting, error) {
	return nil, nil
}

func (r *modelUpdateRepo) UpsertLLMSetting(_ context.Context, item *domainchannel.LLMSetting) error {
	if r.upsertLLMSettingErr != nil {
		return r.upsertLLMSettingErr
	}
	r.llmSetting = *item
	if item.Key == "circuit_breaker.defaults" {
		if parsed, err := parseCircuitBreakerDefaults(item.Value); err == nil {
			r.breakerDefaults = parsed
		}
	}
	return nil
}

func (r *modelUpdateRepo) GetBreakerErrorClassification(context.Context) (domainchannel.BreakerErrorClassification, error) {
	return domainchannel.BreakerErrorClassification{}, nil
}

func (r *modelUpdateRepo) GetBreakerDefaults(context.Context) (domainchannel.BreakerDefaults, error) {
	return r.breakerDefaults, nil
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

func (r *modelUpdateRepo) CreateModelVendor(_ context.Context, item *domainchannel.ModelVendor) error {
	item.ID = 1
	return nil
}

func (r *modelUpdateRepo) UpdateModelVendor(context.Context, string, repository.UpdateModelVendorInput) error {
	return nil
}

func (r *modelUpdateRepo) DeleteModelVendor(context.Context, string) error {
	return r.deleteVendorErr
}

func (r *modelUpdateRepo) GetModelVendorByKey(_ context.Context, key string) (*domainchannel.ModelVendor, error) {
	return &domainchannel.ModelVendor{ID: 1, Key: key, Name: key}, nil
}

func (r *modelUpdateRepo) ListModelVendors(context.Context, repository.ListModelVendorsInput) ([]domainchannel.ModelVendor, int64, error) {
	return nil, 0, nil
}

func (r *modelUpdateRepo) CreateModelDisplayGroup(_ context.Context, item *domainchannel.ModelDisplayGroup, _ []uint) error {
	item.ID = 1
	return nil
}

func (r *modelUpdateRepo) UpdateModelDisplayGroup(context.Context, uint, repository.UpdateModelDisplayGroupInput) error {
	return nil
}

func (r *modelUpdateRepo) SetModelsDisplayGroup(_ context.Context, modelIDs []uint, groupID uint) error {
	r.lastDisplayGroupModelIDs = append([]uint(nil), modelIDs...)
	r.lastDisplayGroupID = groupID
	return r.setDisplayGroupErr
}

func (r *modelUpdateRepo) GetModelDisplayGroupByID(_ context.Context, groupID uint) (*domainchannel.ModelDisplayGroup, error) {
	return &domainchannel.ModelDisplayGroup{ID: groupID, Name: "group"}, nil
}

func (r *modelUpdateRepo) ListModelDisplayGroups(context.Context, repository.ListModelDisplayGroupsInput) ([]domainchannel.ModelDisplayGroup, int64, error) {
	return nil, 0, nil
}

func (r *modelUpdateRepo) DeleteModelDisplayGroup(context.Context, uint) error {
	return nil
}

var _ repository.ChannelRepository = (*modelUpdateRepo)(nil)
var _ repository.ModelPresentationRepository = (*modelUpdateRepo)(nil)
