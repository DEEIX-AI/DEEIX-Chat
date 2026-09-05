package channel

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	domainchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/channel"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/cache/memory"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

type routeResolutionRepositoryStub struct {
	repository.ChannelRepository
	model           domainchannel.PlatformModel
	routes          []repository.ChannelUpstreamRouteRow
	breakerDefaults domainchannel.BreakerDefaults
	breakerErr      error
	breakerLoads    int
}

func TestLocalAPIKeyCounterIsScopedToService(t *testing.T) {
	first := &Service{}
	second := &Service{}

	if index := first.nextLocalAPIKeyIndex(7); index != 0 {
		t.Fatalf("first index = %d, want 0", index)
	}
	if index := first.nextLocalAPIKeyIndex(7); index != 1 {
		t.Fatalf("second index = %d, want 1", index)
	}
	if index := second.nextLocalAPIKeyIndex(7); index != 0 {
		t.Fatalf("independent service index = %d, want 0", index)
	}
}

func (r *routeResolutionRepositoryStub) GetActiveModelByName(context.Context, string) (*domainchannel.PlatformModel, error) {
	model := r.model
	return &model, nil
}

func (r *routeResolutionRepositoryStub) ListActiveRoutesByModel(context.Context, string) ([]repository.ChannelUpstreamRouteRow, error) {
	return append([]repository.ChannelUpstreamRouteRow(nil), r.routes...), nil
}

func (r *routeResolutionRepositoryStub) GetBreakerDefaults(context.Context) (domainchannel.BreakerDefaults, error) {
	r.breakerLoads++
	return r.breakerDefaults, r.breakerErr
}

func (r *routeResolutionRepositoryStub) GetBreakerErrorClassification(context.Context) (domainchannel.BreakerErrorClassification, error) {
	return domainchannel.BreakerErrorClassification{
		CircuitErrors:   []string{"5xx", "timeout", "connection_error"},
		RateLimitErrors: []string{"429"},
		IgnoreErrors:    []string{"4xx"},
	}, nil
}

func (r *routeResolutionRepositoryStub) GetRateLimitDefaults(context.Context) (domainchannel.RateLimitDefaults, error) {
	return domainchannel.RateLimitDefaults{BackoffBaseSec: 5, BackoffMaxSec: 60, BackoffMultiplier: 2}, nil
}

func TestLoadBreakerDefaultsPreservesLastKnownGoodValueOnRefreshFailure(t *testing.T) {
	repo := &routeResolutionRepositoryStub{breakerDefaults: domainchannel.BreakerDefaults{Enabled: true}}
	service := &Service{repo: repo}

	if !service.loadBreakerDefaults(t.Context()).Enabled {
		t.Fatal("expected initial enabled breaker defaults")
	}
	repo.breakerErr = errors.New("database unavailable")
	expireBreakerDefaultsCache(service)
	if !service.loadBreakerDefaults(t.Context()).Enabled {
		t.Fatal("expected refresh failure to preserve last known good breaker defaults")
	}
	if repo.breakerLoads != 2 {
		t.Fatalf("breaker defaults loaded %d times, want 2", repo.breakerLoads)
	}
}

func TestLoadBreakerDefaultsUsesShortLivedCache(t *testing.T) {
	repo := &routeResolutionRepositoryStub{breakerDefaults: domainchannel.BreakerDefaults{Enabled: true}}
	service := &Service{repo: repo}

	first := service.loadBreakerDefaults(t.Context())
	second := service.loadBreakerDefaults(t.Context())
	if !first.Enabled || !second.Enabled {
		t.Fatal("expected enabled breaker defaults")
	}
	if repo.breakerLoads != 1 {
		t.Fatalf("breaker defaults loaded %d times, want 1", repo.breakerLoads)
	}
	expireBreakerDefaultsCache(service)
	if !service.loadBreakerDefaults(t.Context()).Enabled || repo.breakerLoads != 2 {
		t.Fatalf("expected invalidation to reload defaults, loads=%d", repo.breakerLoads)
	}
}

func expireBreakerDefaultsCache(service *Service) {
	service.breakerDefaultsMu.Lock()
	service.breakerDefaultsValidUntil = time.Time{}
	service.breakerDefaultsMu.Unlock()
}

func TestResolveRouteExcludesPreviouslyAttemptedRoutes(t *testing.T) {
	const encryptionKey = "test-data-encryption-key-32-bytes"
	apiKeysEnc, err := encryptAPIKeys(encryptionKey, `{"strategy":"failover","keys":[{"key":"sk-test","status":"active"}]}`)
	if err != nil {
		t.Fatalf("encryptAPIKeys() error = %v", err)
	}

	repo := &routeResolutionRepositoryStub{
		model: domainchannel.PlatformModel{
			ID:                10,
			PlatformModelName: "test-model",
			AccessScope:       ModelAccessScopePublic,
		},
		routes: []repository.ChannelUpstreamRouteRow{
			{
				RouteID:           1,
				UpstreamModelID:   101,
				UpstreamID:        201,
				PlatformModelID:   10,
				PlatformModelName: "test-model",
				ModelKindsJSON:    `["chat"]`,
				Protocol:          llm.AdapterOpenAIChatCompletions,
				BaseURL:           "https://first.example.com/v1",
				APIKeysEnc:        apiKeysEnc,
				BindingCode:       "first-binding",
				UpstreamModelName: "first-model",
				Weight:            1,
				RoutePriority:     1,
			},
			{
				RouteID:           2,
				UpstreamModelID:   102,
				UpstreamID:        202,
				PlatformModelID:   10,
				PlatformModelName: "test-model",
				ModelKindsJSON:    `["chat"]`,
				Protocol:          llm.AdapterOpenAIChatCompletions,
				BaseURL:           "https://second.example.com/v1",
				APIKeysEnc:        apiKeysEnc,
				BindingCode:       "second-binding",
				UpstreamModelName: "second-model",
				Weight:            1,
				RoutePriority:     1,
			},
		},
	}
	service := newTestService(
		config.Config{DataEncryptionKey: encryptionKey},
		repo,
		nil,
		memory.NewChannelCache(memory.New()),
		nil,
	)

	route, err := service.ResolveRoute(t.Context(), ResolveRouteInput{
		PlatformModelName: "test-model",
		TaskType:          TaskTypeChat,
		Scope:             RouteScopeUser,
		ExcludedRouteIDs:  []uint{0, 1, 1},
	})
	if err != nil {
		t.Fatalf("ResolveRoute() error = %v", err)
	}
	if route.RouteID != 2 {
		t.Fatalf("ResolveRoute() route ID = %d, want 2", route.RouteID)
	}
}

func TestResolveRouteIgnoresCircuitStateWhenBreakerDisabled(t *testing.T) {
	const encryptionKey = "test-data-encryption-key-32-bytes"
	apiKeysEnc, err := encryptAPIKeys(encryptionKey, `{"strategy":"failover","keys":[{"key":"sk-test","status":"active"}]}`)
	if err != nil {
		t.Fatalf("encryptAPIKeys() error = %v", err)
	}

	repo := &routeResolutionRepositoryStub{
		model: domainchannel.PlatformModel{ID: 10, PlatformModelName: "test-model", AccessScope: ModelAccessScopePublic},
		routes: []repository.ChannelUpstreamRouteRow{{
			RouteID: 1, UpstreamModelID: 101, UpstreamID: 201, PlatformModelID: 10,
			PlatformModelName: "test-model", ModelKindsJSON: `["chat"]`, Protocol: llm.AdapterOpenAIChatCompletions,
			BaseURL: "https://example.com/v1", APIKeysEnc: apiKeysEnc, BindingCode: "binding", UpstreamModelName: "model", Weight: 1, RoutePriority: 1,
		}},
		breakerDefaults: domainchannel.BreakerDefaults{Enabled: false},
	}
	cache := memory.NewChannelCache(memory.New())
	if err := cache.OpenUpstreamCircuit(t.Context(), 201); err != nil {
		t.Fatalf("OpenUpstreamCircuit() error = %v", err)
	}
	service := newTestService(config.Config{DataEncryptionKey: encryptionKey}, repo, nil, cache, nil)

	route, err := service.ResolveRoute(t.Context(), ResolveRouteInput{PlatformModelName: "test-model", TaskType: TaskTypeChat})
	if err != nil {
		t.Fatalf("ResolveRoute() error = %v", err)
	}
	if route.RouteID != 1 {
		t.Fatalf("ResolveRoute() route ID = %d, want 1", route.RouteID)
	}
}

func TestResolveRouteHonorsCircuitStateWhenBreakerEnabled(t *testing.T) {
	const encryptionKey = "test-data-encryption-key-32-bytes"
	apiKeysEnc, err := encryptAPIKeys(encryptionKey, `{"strategy":"failover","keys":[{"key":"sk-test","status":"active"}]}`)
	if err != nil {
		t.Fatalf("encryptAPIKeys() error = %v", err)
	}
	repo := &routeResolutionRepositoryStub{
		model: domainchannel.PlatformModel{ID: 10, PlatformModelName: "test-model", AccessScope: ModelAccessScopePublic},
		routes: []repository.ChannelUpstreamRouteRow{{
			RouteID: 1, UpstreamModelID: 101, UpstreamID: 201, PlatformModelID: 10,
			PlatformModelName: "test-model", ModelKindsJSON: `["chat"]`, Protocol: llm.AdapterOpenAIChatCompletions,
			BaseURL: "https://example.com/v1", APIKeysEnc: apiKeysEnc, BindingCode: "binding", UpstreamModelName: "model", Weight: 1, RoutePriority: 1,
		}},
		breakerDefaults: domainchannel.BreakerDefaults{Enabled: true},
	}
	cache := memory.NewChannelCache(memory.New())
	if err := cache.OpenUpstreamCircuit(t.Context(), 201); err != nil {
		t.Fatalf("OpenUpstreamCircuit() error = %v", err)
	}
	service := newTestService(config.Config{DataEncryptionKey: encryptionKey}, repo, nil, cache, nil)

	if _, err := service.ResolveRoute(t.Context(), ResolveRouteInput{PlatformModelName: "test-model", TaskType: TaskTypeChat}); !errors.Is(err, ErrAllRoutesUnavailable) {
		t.Fatalf("ResolveRoute() error = %v, want ErrAllRoutesUnavailable", err)
	}
}

func TestRateLimitBackoffOnlySkipsFailedRouteAndReportsCooldown(t *testing.T) {
	const encryptionKey = "test-data-encryption-key-32-bytes"
	apiKeysEnc, err := encryptAPIKeys(encryptionKey, `{"strategy":"failover","keys":[{"key":"sk-test","status":"active"}]}`)
	if err != nil {
		t.Fatalf("encryptAPIKeys() error = %v", err)
	}
	repo := &routeResolutionRepositoryStub{
		model: domainchannel.PlatformModel{ID: 10, PlatformModelName: "test-model", AccessScope: ModelAccessScopePublic},
		routes: []repository.ChannelUpstreamRouteRow{
			{
				RouteID: 1, UpstreamModelID: 101, UpstreamID: 201, PlatformModelID: 10,
				PlatformModelName: "test-model", ModelKindsJSON: `["chat"]`, Protocol: llm.AdapterOpenAIChatCompletions,
				BaseURL: "https://example.com/v1", APIKeysEnc: apiKeysEnc, BindingCode: "first", UpstreamModelName: "first-model", Weight: 1, RoutePriority: 1,
			},
			{
				RouteID: 2, UpstreamModelID: 102, UpstreamID: 201, PlatformModelID: 10,
				PlatformModelName: "test-model", ModelKindsJSON: `["chat"]`, Protocol: llm.AdapterOpenAIChatCompletions,
				BaseURL: "https://example.com/v1", APIKeysEnc: apiKeysEnc, BindingCode: "second", UpstreamModelName: "second-model", Weight: 1, RoutePriority: 2,
			},
		},
	}
	cache := memory.NewChannelCache(memory.New())
	service := newTestService(config.Config{DataEncryptionKey: encryptionKey}, repo, nil, cache, nil)

	first, err := service.ResolveRoute(t.Context(), ResolveRouteInput{PlatformModelName: "test-model", TaskType: TaskTypeChat})
	if err != nil || first.RouteID != 1 {
		t.Fatalf("first ResolveRoute() route = %#v, error = %v", first, err)
	}
	service.MarkRouteFailure(t.Context(), first, &llm.UpstreamError{StatusCode: http.StatusTooManyRequests})

	second, err := service.ResolveRoute(t.Context(), ResolveRouteInput{PlatformModelName: "test-model", TaskType: TaskTypeChat})
	if err != nil || second.RouteID != 2 {
		t.Fatalf("second ResolveRoute() route = %#v, error = %v", second, err)
	}
	service.MarkRouteFailure(t.Context(), second, &llm.UpstreamError{StatusCode: http.StatusTooManyRequests})

	_, err = service.ResolveRoute(t.Context(), ResolveRouteInput{PlatformModelName: "test-model", TaskType: TaskTypeChat})
	var limitedErr *RoutesRateLimitedError
	if !errors.As(err, &limitedErr) || limitedErr.RetryAfter <= 0 {
		t.Fatalf("ResolveRoute() error = %#v, want route rate-limit error", err)
	}

	service.MarkRouteSuccess(t.Context(), first)
	recovered, err := service.ResolveRoute(t.Context(), ResolveRouteInput{PlatformModelName: "test-model", TaskType: TaskTypeChat})
	if err != nil || recovered.RouteID != 1 {
		t.Fatalf("recovered ResolveRoute() route = %#v, error = %v", recovered, err)
	}
}

func TestUpstreamRetryAfterSecondsSupportsSecondsAndHTTPDate(t *testing.T) {
	now := time.Date(2026, time.August, 26, 12, 0, 0, 0, time.UTC)
	withHeader := func(value string) error {
		return &llm.UpstreamError{StatusCode: http.StatusTooManyRequests, Debug: &llm.UpstreamDebugSnapshot{
			Response: llm.UpstreamDebugResponse{Headers: map[string]string{"retry-after": value}},
		}}
	}
	if got := upstreamRetryAfterSeconds(withHeader("12"), now); got != 12 {
		t.Fatalf("seconds Retry-After = %d, want 12", got)
	}
	if got := upstreamRetryAfterSeconds(withHeader(now.Add(25*time.Second).Format(http.TimeFormat)), now); got != 25 {
		t.Fatalf("date Retry-After = %d, want 25", got)
	}
}

func TestMergeHeaderJSONRouteOverridesHeaderCaseInsensitively(t *testing.T) {
	merged := mergeHeaderJSON(
		`{"X-Conversation-Id":"upstream","X-Static":"fixed"}`,
		`{"x-conversation-id":"route"}`,
	)
	var headers map[string]string
	if err := json.Unmarshal([]byte(merged), &headers); err != nil {
		t.Fatalf("unmarshal merged headers: %v", err)
	}
	if len(headers) != 2 {
		t.Fatalf("expected two merged headers, got %#v", headers)
	}
	if got := headers["x-conversation-id"]; got != "route" {
		t.Fatalf("expected route header to override upstream casing, got %q", got)
	}
	if got := headers["X-Static"]; got != "fixed" {
		t.Fatalf("expected unrelated upstream header to remain, got %q", got)
	}
}

func TestShouldFailoverRoute(t *testing.T) {
	tests := []struct {
		name  string
		cause error
		want  bool
	}{
		{name: "nil", cause: nil, want: false},
		{name: "canceled", cause: context.Canceled, want: false},
		{name: "wrapped canceled", cause: errors.Join(errors.New("request failed"), context.Canceled), want: false},
		{name: "deadline", cause: context.DeadlineExceeded, want: true},
		{name: "EOF", cause: io.EOF, want: true},
		{name: "unexpected EOF", cause: io.ErrUnexpectedEOF, want: true},
		{name: "network", cause: &net.DNSError{IsTimeout: true}, want: true},
		{name: "accepted stream failure", cause: llm.MarkRequestAccepted(io.ErrUnexpectedEOF), want: false},
		{name: "request timeout", cause: &llm.UpstreamError{StatusCode: http.StatusRequestTimeout}, want: true},
		{name: "rate limited", cause: &llm.UpstreamError{StatusCode: http.StatusTooManyRequests}, want: true},
		{name: "server error", cause: &llm.UpstreamError{StatusCode: http.StatusBadGateway}, want: true},
		{name: "bad request", cause: &llm.UpstreamError{StatusCode: http.StatusBadRequest}, want: false},
		{name: "unauthorized", cause: &llm.UpstreamError{StatusCode: http.StatusUnauthorized}, want: false},
		{name: "generic", cause: errors.New("validation failed"), want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := ShouldFailoverRoute(test.cause); got != test.want {
				t.Fatalf("ShouldFailoverRoute() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestBindingCircuitKeyUsesBindingCode(t *testing.T) {
	if got := bindingCircuitKey("upm_42"); got != "upstream-model-upm_42" {
		t.Fatalf("expected binding-level circuit key, got %q", got)
	}
	if got := bindingCircuitKey("upm:42"); got != "upstream-model-upm-42" {
		t.Fatalf("expected colon-free binding-level circuit key, got %q", got)
	}
	if got := bindingCircuitKey(""); got != "" {
		t.Fatalf("expected empty key for empty binding code, got %q", got)
	}
}

func TestRemoveCandidateUsesUpstreamModelIDInsteadOfPlatformModelName(t *testing.T) {
	items := []routeCandidate{
		{row: repository.ChannelUpstreamRouteRow{UpstreamID: 1, UpstreamModelID: 10, PlatformModelName: "gpt-5.5"}},
		{row: repository.ChannelUpstreamRouteRow{UpstreamID: 1, UpstreamModelID: 11, PlatformModelName: "gpt-5.5"}},
		{row: repository.ChannelUpstreamRouteRow{UpstreamID: 2, UpstreamModelID: 10, PlatformModelName: "gpt-5.5"}},
	}

	got := removeCandidate(items, 1, 10)
	if len(got) != 2 {
		t.Fatalf("expected only one route candidate removed, got %d", len(got))
	}
	for _, item := range got {
		if item.row.UpstreamID == 1 && item.row.UpstreamModelID == 10 {
			t.Fatalf("route candidate was not removed: %#v", got)
		}
	}
}

func TestBuildResolvedRouteSnapshotsModelIdentity(t *testing.T) {
	route := buildResolvedRoute(repository.ChannelUpstreamRouteRow{
		RouteID:           5,
		PlatformModelID:   9,
		PlatformModelName: "gpt-5.5",
		UpstreamModelID:   7,
		UpstreamID:        3,
		UpstreamName:      "OpenAI Official",
		BindingCode:       "upm_abc",
		ModelVendor:       "openai",
		ModelIcon:         "openai",
		UpstreamModelName: "gpt-5.5-20260501",
		Protocol:          "openai_responses",
	}, "sk-test")

	if route.RouteID != 5 || route.PlatformModelID != 9 || route.UpstreamModelID != 7 {
		t.Fatalf("expected route identity snapshot, got %#v", route)
	}
	if route.UpstreamModel != "gpt-5.5-20260501" {
		t.Fatalf("expected upstream model name, got %q", route.UpstreamModel)
	}
	if route.PlatformModelName != "gpt-5.5" || route.BindingCode != "upm_abc" || route.ModelVendor != "openai" || route.ModelIcon != "openai" {
		t.Fatalf("expected platform model snapshot, got %#v", route)
	}
}

func TestRecordCircuitFailureUsesPlatformModelDefaults(t *testing.T) {
	cache := memory.NewChannelCache(memory.New())
	service := &Service{
		repo:  &modelUpdateRepo{},
		cache: cache,
	}
	route := &ResolvedRoute{
		UpstreamID:                      1,
		BindingCode:                     "upm_abc",
		PlatformModelCbFailureThreshold: 1,
		PlatformModelCbDurationMin:      1,
		PlatformModelCbWindowMin:        1,
	}

	service.recordCircuitFailure(context.Background(), route, domainchannel.BreakerDefaults{
		ModelFailureThreshold: 3,
		ModelDurationMin:      1,
		ModelWindowMin:        1,
	})

	open, _ := cache.QueryModelCircuitStatus(context.Background(), 1, "upstream-model-upm_abc")
	if !open {
		t.Fatal("expected platform model default threshold to open circuit")
	}
}

func TestRecordCircuitFailureRouteOverrideBeatsPlatformModelDefaults(t *testing.T) {
	cache := memory.NewChannelCache(memory.New())
	service := &Service{
		repo:  &modelUpdateRepo{},
		cache: cache,
	}
	route := &ResolvedRoute{
		UpstreamID:                      1,
		BindingCode:                     "upm_abc",
		PlatformModelCbFailureThreshold: 1,
		PlatformModelCbDurationMin:      1,
		PlatformModelCbWindowMin:        1,
		ModelCbFailureThreshold:         2,
		ModelCbDurationMin:              1,
		ModelCbWindowMin:                1,
	}

	service.recordCircuitFailure(context.Background(), route, domainchannel.BreakerDefaults{
		ModelFailureThreshold: 3,
		ModelDurationMin:      1,
		ModelWindowMin:        1,
	})

	open, _ := cache.QueryModelCircuitStatus(context.Background(), 1, "upstream-model-upm_abc")
	if open {
		t.Fatal("expected route override threshold to keep circuit closed after one failure")
	}
}

func TestRecordCircuitFailurePlatformModelPolicyEnforcedBeatsRouteOverride(t *testing.T) {
	cache := memory.NewChannelCache(memory.New())
	service := &Service{
		repo:  &modelUpdateRepo{},
		cache: cache,
	}
	route := &ResolvedRoute{
		UpstreamID:                      1,
		BindingCode:                     "upm_abc",
		PlatformModelCbPolicyMode:       "enforced",
		PlatformModelCbFailureThreshold: 1,
		PlatformModelCbDurationMin:      1,
		PlatformModelCbWindowMin:        1,
		ModelCbFailureThreshold:         2,
		ModelCbDurationMin:              1,
		ModelCbWindowMin:                1,
	}

	service.recordCircuitFailure(context.Background(), route, domainchannel.BreakerDefaults{
		ModelFailureThreshold: 3,
		ModelDurationMin:      1,
		ModelWindowMin:        1,
	})

	open, _ := cache.QueryModelCircuitStatus(context.Background(), 1, "upstream-model-upm_abc")
	if !open {
		t.Fatal("expected enforced platform model policy to open circuit after one failure")
	}
}

func TestMarkRouteFailureDoesNotTripDisabledBreaker(t *testing.T) {
	cache := memory.NewChannelCache(memory.New())
	repo := &modelUpdateRepo{breakerDefaults: domainchannel.BreakerDefaults{
		Enabled:               false,
		ModelFailureThreshold: 1,
		ModelDurationMin:      1,
		ModelWindowMin:        1,
	}}
	service := &Service{repo: repo, cache: cache}
	route := &ResolvedRoute{UpstreamID: 1, BindingCode: "upm_abc"}

	service.MarkRouteFailure(t.Context(), route, &llm.UpstreamError{StatusCode: http.StatusBadGateway})

	if open, _ := cache.QueryModelCircuitStatus(t.Context(), 1, bindingCircuitKey("upm_abc")); open {
		t.Fatal("expected disabled breaker not to open")
	}
}

func TestReleaseGrantedRouteProbesOnlyReleasesGrantedScopes(t *testing.T) {
	cache := &releaseProbeCache{}
	service := &Service{cache: cache}

	service.releaseGrantedRouteProbes(context.Background(), &ResolvedRoute{
		UpstreamID:           1,
		UpstreamModelID:      2,
		BindingCode:          "upm_abc",
		UpstreamProbeGranted: false,
		ModelProbeGranted:    true,
	})

	if len(cache.calls) != 1 {
		t.Fatalf("expected one probe release, got %d", len(cache.calls))
	}
	if cache.calls[0].upstreamID != 1 || cache.calls[0].modelKey != "upstream-model-upm_abc" {
		t.Fatalf("unexpected probe release call: %#v", cache.calls[0])
	}
}

type releaseProbeCall struct {
	upstreamID uint
	modelKey   string
}

type releaseProbeCache struct {
	repository.ChannelCacheRepository
	calls []releaseProbeCall
}

func (c *releaseProbeCache) ReleaseRouteProbes(_ context.Context, upstreamID uint, modelKey string) error {
	c.calls = append(c.calls, releaseProbeCall{upstreamID: upstreamID, modelKey: modelKey})
	return nil
}

func TestRouteScopeAllowsModelAccessDefaultsToUserScope(t *testing.T) {
	for _, scope := range []string{"", "unknown", RouteScopeUser} {
		if routeScopeAllowsModelAccess(scope, ModelAccessScopeInternal) {
			t.Fatalf("scope %q should not access internal model", scope)
		}
		if !routeScopeAllowsModelAccess(scope, ModelAccessScopePublic) {
			t.Fatalf("scope %q should access public model", scope)
		}
	}
}

func TestRouteScopeAllowsInternalModelForInternalScope(t *testing.T) {
	if !routeScopeAllowsModelAccess(RouteScopeInternal, ModelAccessScopeInternal) {
		t.Fatalf("internal scope should access internal model")
	}
}

func TestApplyModelSourceCircuitStatusPrefersUpstreamCircuit(t *testing.T) {
	cache := memory.NewChannelCache(memory.New())
	service := &Service{
		repo:  &modelUpdateRepo{breakerDefaults: domainchannel.BreakerDefaults{Enabled: true}},
		cache: cache,
	}
	ctx := context.Background()

	if err := cache.OpenModelCircuit(ctx, 1, "upstream-model-upm_abc"); err != nil {
		t.Fatalf("OpenModelCircuit() error = %v", err)
	}
	view := ModelUpstreamSourceView{
		UpstreamID:  1,
		BindingCode: "upm_abc",
	}
	service.applyModelSourceCircuitStatus(ctx, &view)
	if !view.CircuitOpen {
		t.Fatal("expected model circuit to be visible")
	}
	if view.CircuitScope != "source" {
		t.Fatalf("expected source circuit scope, got %q", view.CircuitScope)
	}

	if err := cache.OpenUpstreamCircuit(ctx, 1); err != nil {
		t.Fatalf("OpenUpstreamCircuit() error = %v", err)
	}
	service.applyModelSourceCircuitStatus(ctx, &view)
	if !view.CircuitOpen {
		t.Fatal("expected upstream circuit to be visible")
	}
	if view.CircuitScope != "upstream" {
		t.Fatalf("expected upstream circuit scope, got %q", view.CircuitScope)
	}
}
