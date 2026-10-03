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

func reasoningTestEntries() []domainchannel.ReasoningCatalogEntry {
	budgetMin := 1024
	return []domainchannel.ReasoningCatalogEntry{
		{Provider: "openai", ModelID: "gpt-5", CanonicalID: "openai/gpt-5", Reasoning: true, Options: []domainchannel.ReasoningCatalogOption{
			{Type: domainchannel.ReasoningCatalogOptionEffort, Values: []string{"minimal", "low", "medium", "high"}},
		}},
		{Provider: "anthropic", ModelID: "claude-sonnet-4-5", Reasoning: true, Options: []domainchannel.ReasoningCatalogOption{
			{Type: domainchannel.ReasoningCatalogOptionBudget, Min: &budgetMin},
		}},
	}
}

func reasoningTestSnapshot(source string, fetchedAt time.Time, entries []domainchannel.ReasoningCatalogEntry) *domainchannel.ReasoningCatalogSnapshot {
	return &domainchannel.ReasoningCatalogSnapshot{Source: source, FetchedAt: fetchedAt, Entries: entries}
}

type fakeReasoningCatalogFetcher struct {
	entries []domainchannel.ReasoningCatalogEntry
	err     error
	calls   int
}

func (f *fakeReasoningCatalogFetcher) FetchReasoningCatalog(context.Context) ([]domainchannel.ReasoningCatalogEntry, error) {
	f.calls++
	return f.entries, f.err
}

type memoryReasoningCatalogCache struct {
	snapshot *domainchannel.ReasoningCatalogSnapshot
	loadErr  error
	stored   int
}

func (c *memoryReasoningCatalogCache) Load(context.Context) (domainchannel.ReasoningCatalogSnapshot, bool, error) {
	if c.loadErr != nil || c.snapshot == nil {
		return domainchannel.ReasoningCatalogSnapshot{}, false, c.loadErr
	}
	return *c.snapshot, true, nil
}

func (c *memoryReasoningCatalogCache) Store(_ context.Context, snapshot domainchannel.ReasoningCatalogSnapshot) error {
	c.snapshot = &snapshot
	c.stored++
	return nil
}

func TestLoadReasoningCatalogUsesStorageAndSeedsFromBuiltin(t *testing.T) {
	older := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	newer := older.Add(48 * time.Hour)
	builtin := reasoningTestSnapshot("builtin-source", newer, reasoningTestEntries())
	remote := func(at time.Time) *domainchannel.ReasoningCatalogSnapshot {
		snapshot := reasoningTestSnapshot("cache", at, reasoningTestEntries())
		snapshot.Origin = ReasoningCatalogOriginRemote
		return snapshot
	}

	tests := []struct {
		name       string
		cache      *memoryReasoningCatalogCache
		wantOrigin string
		wantSeeded bool
	}{
		{name: "missing file is seeded from builtin", cache: &memoryReasoningCatalogCache{}, wantOrigin: ReasoningCatalogOriginBuiltin, wantSeeded: true},
		{name: "older file is replaced by builtin", cache: &memoryReasoningCatalogCache{snapshot: remote(older)}, wantOrigin: ReasoningCatalogOriginBuiltin, wantSeeded: true},
		{name: "newer file wins", cache: &memoryReasoningCatalogCache{snapshot: remote(newer.Add(time.Hour))}, wantOrigin: ReasoningCatalogOriginRemote},
		{name: "empty file is reseeded", cache: &memoryReasoningCatalogCache{snapshot: reasoningTestSnapshot("cache", newer.Add(time.Hour), nil)}, wantOrigin: ReasoningCatalogOriginBuiltin, wantSeeded: true},
		{name: "unreadable file is reseeded", cache: &memoryReasoningCatalogCache{loadErr: errors.New("disk")}, wantOrigin: ReasoningCatalogOriginBuiltin, wantSeeded: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := newTestService(config.Config{}, nil, nil, nil, nil)
			service.SetReasoningCatalogSources(nil, tt.cache, builtin)
			service.loadReasoningCatalog(context.Background())
			status := service.ReasoningCatalogStatus()
			if status.Origin != tt.wantOrigin || status.ModelCount != 2 || status.FetchedAt == nil {
				t.Fatalf("unexpected status: %#v", status)
			}
			if seeded := tt.cache.stored > 0; seeded != tt.wantSeeded {
				t.Fatalf("seeded = %v, want %v", seeded, tt.wantSeeded)
			}
			if tt.wantSeeded && tt.cache.snapshot.Origin != ReasoningCatalogOriginBuiltin {
				t.Fatalf("expected seeded file to be marked builtin, got %#v", tt.cache.snapshot)
			}
		})
	}
}

func TestRefreshReasoningCatalog(t *testing.T) {
	builtin := reasoningTestSnapshot("builtin-source", time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), reasoningTestEntries()[:1])

	t.Run("failure keeps current catalog", func(t *testing.T) {
		fetcher := &fakeReasoningCatalogFetcher{err: errors.New("dial tcp: timeout")}
		service := newTestService(config.Config{}, nil, nil, nil, nil)
		service.SetReasoningCatalogSources(fetcher, nil, builtin)
		service.loadReasoningCatalog(context.Background())
		status, err := service.RefreshReasoningCatalog(context.Background())
		if !errors.Is(err, ErrReasoningCatalogFetchFailed) {
			t.Fatalf("expected fetch failure, got %v", err)
		}
		if status.Origin != ReasoningCatalogOriginBuiltin || status.ModelCount != 1 || !strings.Contains(status.LastError, "timeout") {
			t.Fatalf("expected builtin catalog with recorded error, got %#v", status)
		}
	})

	t.Run("invalid payload is a failure", func(t *testing.T) {
		fetcher := &fakeReasoningCatalogFetcher{}
		service := newTestService(config.Config{}, nil, nil, nil, nil)
		service.SetReasoningCatalogSources(fetcher, nil, builtin)
		if _, err := service.RefreshReasoningCatalog(context.Background()); !errors.Is(err, ErrReasoningCatalogFetchFailed) {
			t.Fatalf("expected empty catalog to fail, got %v", err)
		}
	})

	t.Run("success replaces catalog and stores cache", func(t *testing.T) {
		fetcher := &fakeReasoningCatalogFetcher{entries: reasoningTestEntries()}
		cache := &memoryReasoningCatalogCache{}
		service := newTestService(config.Config{}, nil, nil, nil, nil)
		service.SetReasoningCatalogSources(fetcher, cache, builtin)
		service.loadReasoningCatalog(context.Background())
		service.reasoningCatalog.lastError = "previous failure"

		status, err := service.RefreshReasoningCatalog(context.Background())
		if err != nil {
			t.Fatalf("RefreshReasoningCatalog() error = %v", err)
		}
		if status.Origin != ReasoningCatalogOriginRemote || status.ModelCount != 2 || status.LastError != "" {
			t.Fatalf("unexpected status: %#v", status)
		}
		if fetcher.calls != 1 {
			t.Fatalf("expected one fetch, got %d", fetcher.calls)
		}
		// 第一次写入是启动时的内置种子，第二次是刷新结果。
		if cache.stored != 2 || cache.snapshot.Origin != ReasoningCatalogOriginRemote {
			t.Fatalf("expected refreshed catalog to replace the seeded file, got %d writes %#v", cache.stored, cache.snapshot)
		}
		// 重启后从缓存恢复同一份目录。
		restarted := newTestService(config.Config{}, nil, nil, nil, nil)
		restarted.SetReasoningCatalogSources(nil, cache, builtin)
		restarted.loadReasoningCatalog(context.Background())
		if got := restarted.ReasoningCatalogStatus(); got.Origin != ReasoningCatalogOriginRemote || got.ModelCount != 2 {
			t.Fatalf("expected cached catalog after restart, got %#v", got)
		}
	})
}

func TestClaimReasoningCatalogRefresh(t *testing.T) {
	now := time.Now()
	newService := func(fetcher reasoningCatalogFetcher, fetchedAt time.Time) *Service {
		service := newTestService(config.Config{}, nil, nil, nil, nil)
		service.SetReasoningCatalogSources(fetcher, nil, reasoningTestSnapshot("builtin", fetchedAt, reasoningTestEntries()))
		service.loadReasoningCatalog(context.Background())
		return service
	}

	t.Run("fresh catalog is not refetched", func(t *testing.T) {
		service := newService(&fakeReasoningCatalogFetcher{}, now.Add(-time.Hour))
		if service.claimReasoningCatalogRefresh(now) {
			t.Fatal("fresh catalog must not be refetched")
		}
	})

	t.Run("without fetcher nothing is claimed", func(t *testing.T) {
		service := newService(nil, now.Add(-25*time.Hour))
		if service.claimReasoningCatalogRefresh(now) {
			t.Fatal("expected no claim without a fetcher")
		}
	})

	t.Run("stale catalog is claimed once and reported as refreshing", func(t *testing.T) {
		service := newService(&fakeReasoningCatalogFetcher{entries: reasoningTestEntries()}, now.Add(-25*time.Hour))
		if !service.claimReasoningCatalogRefresh(now) {
			t.Fatal("expected stale catalog to be claimed")
		}
		if service.claimReasoningCatalogRefresh(now) {
			t.Fatal("concurrent views must not claim a second refresh")
		}
		if !service.ReasoningCatalogStatus().Refreshing {
			t.Fatal("expected status to report the pending refresh")
		}
		service.runClaimedReasoningCatalogRefresh(context.Background())
		status := service.ReasoningCatalogStatus()
		if status.Refreshing || status.Origin != ReasoningCatalogOriginRemote {
			t.Fatalf("expected refreshed remote catalog, got %#v", status)
		}
	})

	t.Run("failure waits for the retry interval", func(t *testing.T) {
		fetcher := &fakeReasoningCatalogFetcher{err: errors.New("dial tcp: timeout")}
		service := newService(fetcher, now.Add(-25*time.Hour))
		if !service.claimReasoningCatalogRefresh(now) {
			t.Fatal("expected stale catalog to be claimed")
		}
		service.runClaimedReasoningCatalogRefresh(context.Background())
		if fetcher.calls != 1 || service.ReasoningCatalogStatus().LastError == "" {
			t.Fatalf("expected one failed fetch with a recorded error, got %d calls", fetcher.calls)
		}
		if service.claimReasoningCatalogRefresh(time.Now().Add(30 * time.Minute)) {
			t.Fatal("failed refresh must not be retried within the retry interval")
		}
		if !service.claimReasoningCatalogRefresh(time.Now().Add(reasoningCatalogRetryInterval + time.Minute)) {
			t.Fatal("expected a retry after the retry interval")
		}
	})
}

func newCatalogTestService(t *testing.T, repo repository.ChannelRepository) *Service {
	t.Helper()
	service := newTestService(config.Config{}, repo, nil, nil, nil)
	service.SetReasoningCatalogSources(nil, nil, reasoningTestSnapshot("builtin", time.Now(), reasoningTestEntries()))
	service.loadReasoningCatalog(context.Background())
	return service
}

func TestModelReasoningResolverUsesCatalogAsFallback(t *testing.T) {
	service := newCatalogTestService(t, nil)
	resolver := service.ModelReasoningResolver()

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
