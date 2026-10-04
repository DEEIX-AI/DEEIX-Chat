package channel

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	domainchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/channel"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

func reasoningTestEntries() []domainchannel.ModelCatalogEntry {
	budgetMin := 1024
	return []domainchannel.ModelCatalogEntry{
		{Provider: "openai", ModelID: "gpt-5", CanonicalID: "openai/gpt-5", Reasoning: true, Options: []domainchannel.ReasoningCatalogOption{
			{Type: domainchannel.ReasoningCatalogOptionEffort, Values: []string{"minimal", "low", "medium", "high"}},
		}},
		{Provider: "anthropic", ModelID: "claude-sonnet-4-5", Reasoning: true, Options: []domainchannel.ReasoningCatalogOption{
			{Type: domainchannel.ReasoningCatalogOptionBudget, Min: &budgetMin},
		}, InputModalities: []string{"text", "image", "pdf"}, ContextWindow: 200_000},
		// 只有输入模态、没有推理选项的条目：不得影响推理匹配。
		{Provider: "deepseek", ModelID: "deepseek-chat", InputModalities: []string{"text"}},
	}
}

func reasoningTestSnapshot(source string, fetchedAt time.Time, entries []domainchannel.ModelCatalogEntry) *domainchannel.ModelCatalogSnapshot {
	return &domainchannel.ModelCatalogSnapshot{Source: source, FetchedAt: fetchedAt, Entries: entries}
}

type fakeModelCatalogFetcher struct {
	entries []domainchannel.ModelCatalogEntry
	err     error
	calls   int
}

func (f *fakeModelCatalogFetcher) FetchCatalog(context.Context) ([]domainchannel.ModelCatalogEntry, error) {
	f.calls++
	return f.entries, f.err
}

type memoryModelCatalogCache struct {
	snapshot *domainchannel.ModelCatalogSnapshot
	loadErr  error
	stored   int
}

func (c *memoryModelCatalogCache) Load(context.Context) (domainchannel.ModelCatalogSnapshot, bool, error) {
	if c.loadErr != nil || c.snapshot == nil {
		return domainchannel.ModelCatalogSnapshot{}, false, c.loadErr
	}
	return *c.snapshot, true, nil
}

func (c *memoryModelCatalogCache) Store(_ context.Context, snapshot domainchannel.ModelCatalogSnapshot) error {
	c.snapshot = &snapshot
	c.stored++
	return nil
}

func TestLoadModelCatalogUsesStorageAndSeedsFromBuiltin(t *testing.T) {
	older := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	newer := older.Add(48 * time.Hour)
	builtin := reasoningTestSnapshot("builtin-source", newer, reasoningTestEntries())
	remote := func(at time.Time) *domainchannel.ModelCatalogSnapshot {
		snapshot := reasoningTestSnapshot("cache", at, reasoningTestEntries())
		snapshot.Origin = ModelCatalogOriginRemote
		return snapshot
	}

	tests := []struct {
		name       string
		cache      *memoryModelCatalogCache
		wantOrigin string
		wantSeeded bool
	}{
		{name: "missing file is seeded from builtin", cache: &memoryModelCatalogCache{}, wantOrigin: ModelCatalogOriginBuiltin, wantSeeded: true},
		{name: "older file is replaced by builtin", cache: &memoryModelCatalogCache{snapshot: remote(older)}, wantOrigin: ModelCatalogOriginBuiltin, wantSeeded: true},
		{name: "newer file wins", cache: &memoryModelCatalogCache{snapshot: remote(newer.Add(time.Hour))}, wantOrigin: ModelCatalogOriginRemote},
		{name: "empty file is reseeded", cache: &memoryModelCatalogCache{snapshot: reasoningTestSnapshot("cache", newer.Add(time.Hour), nil)}, wantOrigin: ModelCatalogOriginBuiltin, wantSeeded: true},
		{name: "unreadable file is reseeded", cache: &memoryModelCatalogCache{loadErr: errors.New("disk")}, wantOrigin: ModelCatalogOriginBuiltin, wantSeeded: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := newTestService(config.Config{}, nil, nil, nil, nil)
			service.SetModelCatalogSources(nil, tt.cache, builtin)
			service.loadModelCatalog(context.Background())
			status := service.ModelCatalogStatus()
			if status.Origin != tt.wantOrigin || status.ModelCount != 3 || status.FetchedAt == nil {
				t.Fatalf("unexpected status: %#v", status)
			}
			if seeded := tt.cache.stored > 0; seeded != tt.wantSeeded {
				t.Fatalf("seeded = %v, want %v", seeded, tt.wantSeeded)
			}
			if tt.wantSeeded && tt.cache.snapshot.Origin != ModelCatalogOriginBuiltin {
				t.Fatalf("expected seeded file to be marked builtin, got %#v", tt.cache.snapshot)
			}
		})
	}
}

func TestRefreshModelCatalog(t *testing.T) {
	builtin := reasoningTestSnapshot("builtin-source", time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), reasoningTestEntries()[:1])

	t.Run("failure keeps current catalog", func(t *testing.T) {
		fetcher := &fakeModelCatalogFetcher{err: errors.New("dial tcp: timeout")}
		service := newTestService(config.Config{}, nil, nil, nil, nil)
		service.SetModelCatalogSources(fetcher, nil, builtin)
		service.loadModelCatalog(context.Background())
		status, err := service.RefreshModelCatalog(context.Background())
		if !errors.Is(err, ErrModelCatalogFetchFailed) {
			t.Fatalf("expected fetch failure, got %v", err)
		}
		if status.Origin != ModelCatalogOriginBuiltin || status.ModelCount != 1 || !strings.Contains(status.LastError, "timeout") {
			t.Fatalf("expected builtin catalog with recorded error, got %#v", status)
		}
	})

	t.Run("invalid payload is a failure", func(t *testing.T) {
		fetcher := &fakeModelCatalogFetcher{}
		service := newTestService(config.Config{}, nil, nil, nil, nil)
		service.SetModelCatalogSources(fetcher, nil, builtin)
		if _, err := service.RefreshModelCatalog(context.Background()); !errors.Is(err, ErrModelCatalogFetchFailed) {
			t.Fatalf("expected empty catalog to fail, got %v", err)
		}
	})

	t.Run("success replaces catalog and stores cache", func(t *testing.T) {
		fetcher := &fakeModelCatalogFetcher{entries: reasoningTestEntries()}
		cache := &memoryModelCatalogCache{}
		service := newTestService(config.Config{}, nil, nil, nil, nil)
		service.SetModelCatalogSources(fetcher, cache, builtin)
		service.loadModelCatalog(context.Background())
		service.modelsDevCatalog.lastError = "previous failure"

		status, err := service.RefreshModelCatalog(context.Background())
		if err != nil {
			t.Fatalf("RefreshModelCatalog() error = %v", err)
		}
		if status.Origin != ModelCatalogOriginRemote || status.ModelCount != 3 || status.LastError != "" {
			t.Fatalf("unexpected status: %#v", status)
		}
		if fetcher.calls != 1 {
			t.Fatalf("expected one fetch, got %d", fetcher.calls)
		}
		// 第一次写入是启动时的内置种子，第二次是刷新结果。
		if cache.stored != 2 || cache.snapshot.Origin != ModelCatalogOriginRemote {
			t.Fatalf("expected refreshed catalog to replace the seeded file, got %d writes %#v", cache.stored, cache.snapshot)
		}
		// 重启后从缓存恢复同一份目录。
		restarted := newTestService(config.Config{}, nil, nil, nil, nil)
		restarted.SetModelCatalogSources(nil, cache, builtin)
		restarted.loadModelCatalog(context.Background())
		if got := restarted.ModelCatalogStatus(); got.Origin != ModelCatalogOriginRemote || got.ModelCount != 3 {
			t.Fatalf("expected cached catalog after restart, got %#v", got)
		}
	})
}

func TestClaimModelCatalogRefresh(t *testing.T) {
	now := time.Now()
	newService := func(fetcher modelCatalogFetcher, fetchedAt time.Time) *Service {
		service := newTestService(config.Config{}, nil, nil, nil, nil)
		service.SetModelCatalogSources(fetcher, nil, reasoningTestSnapshot("builtin", fetchedAt, reasoningTestEntries()))
		service.loadModelCatalog(context.Background())
		return service
	}

	t.Run("fresh catalog is not refetched", func(t *testing.T) {
		service := newService(&fakeModelCatalogFetcher{}, now.Add(-time.Hour))
		if service.claimModelCatalogRefresh(now) {
			t.Fatal("fresh catalog must not be refetched")
		}
	})

	t.Run("without fetcher nothing is claimed", func(t *testing.T) {
		service := newService(nil, now.Add(-25*time.Hour))
		if service.claimModelCatalogRefresh(now) {
			t.Fatal("expected no claim without a fetcher")
		}
	})

	t.Run("stale catalog is claimed once and reported as refreshing", func(t *testing.T) {
		service := newService(&fakeModelCatalogFetcher{entries: reasoningTestEntries()}, now.Add(-25*time.Hour))
		if !service.claimModelCatalogRefresh(now) {
			t.Fatal("expected stale catalog to be claimed")
		}
		if service.claimModelCatalogRefresh(now) {
			t.Fatal("concurrent views must not claim a second refresh")
		}
		if !service.ModelCatalogStatus().Refreshing {
			t.Fatal("expected status to report the pending refresh")
		}
		service.runClaimedModelCatalogRefresh(context.Background())
		status := service.ModelCatalogStatus()
		if status.Refreshing || status.Origin != ModelCatalogOriginRemote {
			t.Fatalf("expected refreshed remote catalog, got %#v", status)
		}
	})

	t.Run("failure waits for the retry interval", func(t *testing.T) {
		fetcher := &fakeModelCatalogFetcher{err: errors.New("dial tcp: timeout")}
		service := newService(fetcher, now.Add(-25*time.Hour))
		if !service.claimModelCatalogRefresh(now) {
			t.Fatal("expected stale catalog to be claimed")
		}
		service.runClaimedModelCatalogRefresh(context.Background())
		if fetcher.calls != 1 || service.ModelCatalogStatus().LastError == "" {
			t.Fatalf("expected one failed fetch with a recorded error, got %d calls", fetcher.calls)
		}
		if service.claimModelCatalogRefresh(time.Now().Add(30 * time.Minute)) {
			t.Fatal("failed refresh must not be retried within the retry interval")
		}
		if !service.claimModelCatalogRefresh(time.Now().Add(modelCatalogRetryInterval + time.Minute)) {
			t.Fatal("expected a retry after the retry interval")
		}
	})
}

func newCatalogTestService(t *testing.T, repo repository.ChannelRepository) *Service {
	t.Helper()
	service := newTestService(config.Config{}, repo, nil, nil, nil)
	service.SetModelCatalogSources(nil, nil, reasoningTestSnapshot("builtin", time.Now(), reasoningTestEntries()))
	service.loadModelCatalog(context.Background())
	return service
}

func TestModelReasoningResolverUsesCatalogAsFallback(t *testing.T) {
	service := newCatalogTestService(t, nil)
	resolver := service.ModelCapabilityResolver()

	undeclared := ModelView{PlatformModelName: "gpt-5", Vendor: "openai", ProtocolsJSON: `["openai"]`, CapabilitiesJSON: `{}`}
	info := resolver.Resolve(undeclared)
	if info.Source != domainchannel.ReasoningSourceCatalog || info.View == nil || info.View.Source != domainchannel.ReasoningSourceCatalog {
		t.Fatalf("expected catalog capability, got %#v", info)
	}
	if strings.Join(info.View.Levels, ",") != "minimal,low,medium,high" {
		t.Fatalf("unexpected catalog levels: %#v", info)
	}

	explicit := undeclared
	explicit.CapabilitiesJSON = `{"reasoning":{"format":"openai","levels":["low","high"],"default":"high"}}`
	info = resolver.Resolve(explicit)
	if info.Source != domainchannel.ReasoningSourceExplicit || strings.Join(info.View.Levels, ",") != "low,high" {
		t.Fatalf("expected explicit declaration to win, got %#v", info)
	}

	noRoutes := undeclared
	noRoutes.ProtocolsJSON = `[]`
	if info := resolver.Resolve(noRoutes); info.View != nil {
		t.Fatalf("expected no capability without routes, got %#v", info)
	}
}

func TestCatalogReasoningForRoute(t *testing.T) {
	service := newCatalogTestService(t, nil)
	route := &ResolvedRoute{PlatformModelName: "claude-sonnet-4-5", ModelVendor: "anthropic", Protocol: "anthropic", ModelCapabilitiesJSON: `{}`}
	capability := service.catalogReasoningForRoute(route)
	if capability == nil || capability.Format != domainchannel.ReasoningFormatAnthropicBudget {
		t.Fatalf("expected anthropic budget capability, got %#v", capability)
	}
	route.ModelCapabilitiesJSON = `{"reasoning":{"format":"anthropic_effort","levels":["low"],"default":"low"}}`
	if capability := service.catalogReasoningForRoute(route); capability != nil {
		t.Fatalf("expected explicit declaration to skip catalog lookup, got %#v", capability)
	}
}

func TestCatalogInputModalitiesForRouteAndResolver(t *testing.T) {
	service := newCatalogTestService(t, nil)

	claude := &ResolvedRoute{PlatformModelName: "claude-sonnet-4-5", ModelVendor: "anthropic", Protocol: "anthropic", ModelCapabilitiesJSON: `{}`}
	if got := strings.Join(service.catalogInputModalitiesForRoute(claude), ","); got != "text,image,pdf" {
		t.Fatalf("expected catalog modalities for claude, got %q", got)
	}
	deepseek := &ResolvedRoute{PlatformModelName: "deepseek-chat", ModelVendor: "deepseek", Protocol: "openai_chat_completions"}
	if got := strings.Join(service.catalogInputModalitiesForRoute(deepseek), ","); got != "text" {
		t.Fatalf("expected text-only catalog entry, got %q", got)
	}
	if capability := service.catalogReasoningForRoute(deepseek); capability != nil {
		t.Fatalf("modality-only entries must not produce reasoning, got %#v", capability)
	}
	unknown := &ResolvedRoute{PlatformModelName: "in-house-model", ModelVendor: "custom", Protocol: "openai_chat_completions"}
	if got := service.catalogInputModalitiesForRoute(unknown); got != nil {
		t.Fatalf("unmatched models must stay unknown, got %#v", got)
	}

	resolver := service.ModelCapabilityResolver()
	info := resolver.Resolve(ModelView{PlatformModelName: "claude-sonnet-4-5", Vendor: "anthropic", ProtocolsJSON: `["anthropic"]`, CapabilitiesJSON: `{}`})
	if info.InputModalities.Source != domainchannel.InputModalitiesSourceCatalog || !info.InputModalities.Supports(domainchannel.InputModalityPDF) {
		t.Fatalf("expected catalog modalities on the model view, got %#v", info.InputModalities)
	}
	info = resolver.Resolve(ModelView{PlatformModelName: "claude-sonnet-4-5", Vendor: "anthropic", ProtocolsJSON: `["anthropic"]`, CapabilitiesJSON: `{"inputModalities":["image"]}`})
	if info.InputModalities.Source != domainchannel.InputModalitiesSourceExplicit || info.InputModalities.Supports(domainchannel.InputModalityPDF) {
		t.Fatalf("expected explicit modalities to override the catalog, got %#v", info.InputModalities)
	}
}

func TestResolveModelCatalogForAdminForm(t *testing.T) {
	service := newCatalogTestService(t, nil)
	ctx := context.Background()

	// 目录命中：模态与上下文窗口来自目录；推理能力按协议由目录推导，并给出同格式的自定义模板。
	claude := service.ResolveModelCatalog(ctx, ModelCatalogResolveInput{Name: "claude-sonnet-4-5", Vendor: "anthropic", Protocols: []string{"anthropic"}})
	if !claude.Matched || claude.Provider != "anthropic" || strings.Join(claude.InputModalities, ",") != "text,image,pdf" || claude.ContextWindow != 200_000 {
		t.Fatalf("expected the anthropic entry, got %#v", claude)
	}
	if claude.AutoReasoning == nil || claude.AutoReasoningSource != domainchannel.ReasoningSourceCatalog || claude.AutoReasoning.Format != domainchannel.ReasoningFormatAnthropicBudget {
		t.Fatalf("expected catalog budget reasoning, got %#v (%s)", claude.AutoReasoning, claude.AutoReasoningSource)
	}
	template := claude.ReasoningTemplate
	if template == nil || template.Format != domainchannel.ReasoningFormatAnthropicBudget || template.Budgets["medium"] != claude.AutoReasoning.Budgets["medium"] || template.Budgets["minimal"] < domainchannel.AnthropicMinThinkingBudget {
		t.Fatalf("unexpected template: %#v", template)
	}

	// 显式 reasoning 声明不影响「自动识别」的结果。
	explicit := service.ResolveModelCatalog(ctx, ModelCatalogResolveInput{
		Name: "gpt-5", Vendor: "openai", Protocols: []string{"openai"},
		CapabilitiesJSON: `{"reasoning":{"format":"openai","levels":["high"],"default":"high"}}`,
	})
	if explicit.AutoReasoning == nil || strings.Join(explicit.AutoReasoning.Levels, ",") != "minimal,low,medium,high" || explicit.ContextWindow != 0 {
		t.Fatalf("expected catalog levels ignoring the explicit declaration, got %#v", explicit)
	}
	if explicit.ReasoningTemplate == nil || strings.Join(explicit.ReasoningTemplate.Levels, ",") != "none,minimal,low,medium,high,xhigh,max" {
		t.Fatalf("expected the openai template, got %#v", explicit.ReasoningTemplate)
	}

	// 未命中目录：无模态与推理能力，但仍按协议给出可自定义的模板；没有协议时不给模板。
	unknown := service.ResolveModelCatalog(ctx, ModelCatalogResolveInput{Name: "in-house-model", Vendor: "custom", Protocols: []string{"google"}})
	if unknown.Matched || unknown.AutoReasoning != nil || unknown.ReasoningTemplate == nil || unknown.ReasoningTemplate.Format != domainchannel.ReasoningFormatGeminiLevel {
		t.Fatalf("unexpected resolution for unknown model: %#v", unknown)
	}
	if bare := service.ResolveModelCatalog(ctx, ModelCatalogResolveInput{Name: "in-house-model"}); bare.ReasoningTemplate != nil {
		t.Fatalf("expected no template without protocols, got %#v", bare.ReasoningTemplate)
	}
}
