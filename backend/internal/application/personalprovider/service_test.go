package personalprovider

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	appaudit "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/audit"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
	domainpersonalprovider "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/personalprovider"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

const testDataKey = "personal-provider-test-data-key"

// memoryRepo 是按归属过滤的内存仓储，行为与 SQL 实现一致：不属于调用者的记录一律视为不存在。
type memoryRepo struct {
	mu    sync.Mutex
	items []domainpersonalprovider.Provider
}

func (r *memoryRepo) ListByOwner(_ context.Context, owner uint) ([]domainpersonalprovider.Provider, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var result []domainpersonalprovider.Provider
	for _, item := range r.items {
		if item.OwnerUserID == owner {
			result = append(result, item)
		}
	}
	return result, nil
}

func (r *memoryRepo) CountByOwner(ctx context.Context, owner uint) (int64, error) {
	items, _ := r.ListByOwner(ctx, owner)
	return int64(len(items)), nil
}

func (r *memoryRepo) GetByOwner(_ context.Context, owner uint, publicID string) (*domainpersonalprovider.Provider, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, item := range r.items {
		if item.OwnerUserID == owner && item.PublicID == publicID {
			copied := item
			return &copied, nil
		}
	}
	return nil, repository.ErrNotFound
}

func (r *memoryRepo) Create(_ context.Context, item *domainpersonalprovider.Provider) (*domainpersonalprovider.Provider, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	copied := *item
	copied.ID = uint(len(r.items) + 1)
	r.items = append(r.items, copied)
	return &copied, nil
}

func (r *memoryRepo) UpdateByOwner(ctx context.Context, owner uint, publicID string, patch repository.PersonalProviderPatch) (*domainpersonalprovider.Provider, error) {
	r.mu.Lock()
	found := false
	for index := range r.items {
		item := &r.items[index]
		if item.OwnerUserID != owner || item.PublicID != publicID {
			continue
		}
		found = true
		if patch.Name != nil {
			item.Name = *patch.Name
		}
		if patch.Icon != nil {
			item.Icon = *patch.Icon
		}
		if patch.APIKeyEnc != nil {
			item.APIKeyEnc = *patch.APIKeyEnc
		}
		if patch.KeyHint != nil {
			item.KeyHint = *patch.KeyHint
		}
		if patch.Models != nil {
			item.Models = *patch.Models
		}
		if patch.Status != nil {
			item.Status = *patch.Status
		}
		if patch.LastError != nil {
			item.LastError = *patch.LastError
		}
		if patch.LastCheckedAt != nil {
			item.LastCheckedAt = patch.LastCheckedAt
		}
	}
	r.mu.Unlock()
	if !found {
		return nil, repository.ErrNotFound
	}
	return r.GetByOwner(ctx, owner, publicID)
}

func (r *memoryRepo) DeleteByOwner(_ context.Context, owner uint, publicID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for index, item := range r.items {
		if item.OwnerUserID == owner && item.PublicID == publicID {
			r.items = append(r.items[:index], r.items[index+1:]...)
			return nil
		}
	}
	return repository.ErrNotFound
}

func (r *memoryRepo) ListForAdmin(context.Context, repository.PersonalProviderAdminFilter, int, int) ([]domainpersonalprovider.Provider, int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]domainpersonalprovider.Provider(nil), r.items...), int64(len(r.items)), nil
}

func (r *memoryRepo) SetStatusByPublicIDs(_ context.Context, ids []string, status string) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var affected int64
	for index := range r.items {
		for _, id := range ids {
			if r.items[index].PublicID == id {
				r.items[index].Status = status
				affected++
			}
		}
	}
	return affected, nil
}

func (r *memoryRepo) SetStatusByHost(_ context.Context, host string, status string) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var affected int64
	for index := range r.items {
		if r.items[index].Host == host {
			r.items[index].Status = status
			affected++
		}
	}
	return affected, nil
}

func (r *memoryRepo) DeleteByPublicIDs(context.Context, []string) (int64, error) { return 0, nil }

type fakeLister struct {
	mu     sync.Mutex
	calls  []llm.RouteConfig
	models []llm.ModelItem
	err    error
}

func (f *fakeLister) ListModels(_ context.Context, route llm.RouteConfig) ([]llm.ModelItem, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, route)
	return f.models, f.err
}

type recordingAudit struct {
	mu      sync.Mutex
	entries []appaudit.WriteInput
}

func (r *recordingAudit) Write(_ context.Context, input appaudit.WriteInput) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = append(r.entries, input)
}

type serviceFixture struct {
	service *Service
	repo    *memoryRepo
	lister  *fakeLister
	audit   *recordingAudit
	runtime *config.Runtime
}

func newFixture(t *testing.T, mutate func(*config.Config)) serviceFixture {
	t.Helper()
	cfg := config.Config{
		DataEncryptionKey:           testDataKey,
		PersonalProvidersEnabled:    true,
		PersonalProvidersMaxPerUser: 3,
	}
	if mutate != nil {
		mutate(&cfg)
	}
	runtime := config.NewRuntime(cfg)
	repo := &memoryRepo{}
	lister := &fakeLister{models: []llm.ModelItem{{ID: "gpt-4o"}, {ID: "o4-mini"}, {ID: "bad model"}}}
	audit := &recordingAudit{}
	service := NewService(runtime, repo, lister)
	service.SetAuditWriter(audit)
	service.now = func() time.Time { return time.Unix(1_700_000_000, 0) }
	return serviceFixture{service: service, repo: repo, lister: lister, audit: audit, runtime: runtime}
}

func validCreate() CreateInput {
	return CreateInput{
		Name:     "My OpenAI",
		Protocol: llm.AdapterOpenAIChatCompletions,
		BaseURL:  "https://api.openai.com/v1/",
		APIKey:   "sk-test-abcdefghijklmnop1234",
		Models:   []ModelInput{{Name: "gpt-4o"}},
	}
}

func TestCreateEncryptsKeyAndNeverStoresPlaintext(t *testing.T) {
	fixture := newFixture(t, nil)
	item, err := fixture.service.Create(context.Background(), 1, validCreate(), RequestMeta{})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if item.BaseURL != "https://api.openai.com/v1" || item.Host != "api.openai.com" {
		t.Fatalf("endpoint not normalized: %q %q", item.BaseURL, item.Host)
	}
	if strings.Contains(item.APIKeyEnc, "sk-test") || !strings.HasPrefix(item.APIKeyEnc, "v1:") {
		t.Fatalf("key stored in plaintext: %q", item.APIKeyEnc)
	}
	if item.KeyHint != "sk-••••1234" {
		t.Fatalf("key hint = %q", item.KeyHint)
	}
	if len(item.Models) != 1 || item.Models[0].Name != "gpt-4o" || strings.Join(item.Models[0].Protocols, ",") != "openai_chat_completions" {
		t.Fatalf("models = %#v", item.Models)
	}
	if call := fixture.lister.calls[0]; !call.UntrustedEndpoint {
		t.Fatal("probe must use the untrusted endpoint client")
	}
	for _, entry := range fixture.audit.entries {
		if strings.Contains(strings.ToLower(stringify(entry.Detail)), "sk-test") {
			t.Fatalf("audit entry leaked the key: %#v", entry)
		}
	}
}

func stringify(value any) string {
	switch typed := value.(type) {
	case map[string]any:
		var builder strings.Builder
		for key, item := range typed {
			builder.WriteString(key)
			builder.WriteString("=")
			if text, ok := item.(string); ok {
				builder.WriteString(text)
			}
			builder.WriteString(";")
		}
		return builder.String()
	default:
		return ""
	}
}

// 普通条目只匹配域名本身，"*." 前缀才匹配子域名（且不含域名本身）。
func TestBlockedHostMatching(t *testing.T) {
	cfg := config.Config{PersonalProvidersBlockedHosts: "example.com, *.relay.example.org\nEXACT.example.net."}
	policy := policyFromConfig(cfg)
	cases := map[string]bool{
		"example.com":           true,
		"Example.COM.":          true,
		"api.example.com":       false,
		"notexample.com":        false,
		"relay.example.org":     false,
		"a.relay.example.org":   true,
		"a.b.relay.example.org": true,
		"evilrelay.example.org": false,
		"exact.example.net":     true,
		"sub.exact.example.net": false,
	}
	for host, want := range cases {
		if got := policy.isBlockedHost(host); got != want {
			t.Errorf("isBlockedHost(%q) = %v, want %v", host, got, want)
		}
	}
}

func TestCreateRejectsUnsafeAddresses(t *testing.T) {
	fixture := newFixture(t, func(cfg *config.Config) {
		cfg.PersonalProvidersBlockedHosts = "evil.example.com\n*.bad.example.net"
	})
	cases := map[string]error{
		"http://api.openai.com/v1":            ErrInvalidBaseURL,
		"https://user:pass@api.openai.com/v1": ErrInvalidBaseURL,
		"https://api.openai.com/v1?x=1":       ErrInvalidBaseURL,
		"ftp://api.openai.com":                ErrInvalidBaseURL,
		"https://127.0.0.1/v1":                ErrBlockedHost,
		"https://localhost/v1":                ErrBlockedHost,
		"https://10.0.0.5/v1":                 ErrBlockedHost,
		"https://169.254.169.254/latest":      ErrBlockedHost,
		"https://100.64.0.10/v1":              ErrBlockedHost,
		"https://[::1]/v1":                    ErrBlockedHost,
		"https://metadata.google.internal/":   ErrBlockedHost,
		"https://evil.example.com/v1":         ErrBlockedHost,
		"https://EVIL.example.com./v1":        ErrBlockedHost,
		"https://api.bad.example.net/v1":      ErrBlockedHost,
		"https://a.b.bad.example.net/v1":      ErrBlockedHost,
	}
	for address, want := range cases {
		input := validCreate()
		input.BaseURL = address
		if _, err := fixture.service.Create(context.Background(), 1, input, RequestMeta{}); !errors.Is(err, want) {
			t.Errorf("%s: err = %v, want %v", address, err, want)
		}
	}
	if len(fixture.lister.calls) != 0 {
		t.Fatalf("rejected addresses must never be contacted, got %d calls", len(fixture.lister.calls))
	}
}

func TestCreateRejectsHeaderInjectionInKey(t *testing.T) {
	fixture := newFixture(t, nil)
	input := validCreate()
	input.APIKey = "sk-abc\r\nX-Injected: 1"
	if _, err := fixture.service.Create(context.Background(), 1, input, RequestMeta{}); !errors.Is(err, ErrInvalidAPIKey) {
		t.Fatalf("err = %v, want ErrInvalidAPIKey", err)
	}
}

func TestCreateOnlyAcceptsModelsTheUpstreamReports(t *testing.T) {
	fixture := newFixture(t, nil)
	input := validCreate()
	input.Models = []ModelInput{{Name: "gpt-4o"}, {Name: "not-offered"}}
	if _, err := fixture.service.Create(context.Background(), 1, input, RequestMeta{}); !errors.Is(err, ErrInvalidModels) {
		t.Fatalf("err = %v, want ErrInvalidModels", err)
	}
}

func TestFeatureGate(t *testing.T) {
	disabled := newFixture(t, func(cfg *config.Config) { cfg.PersonalProvidersEnabled = false })
	if _, err := disabled.service.Create(context.Background(), 1, validCreate(), RequestMeta{}); !errors.Is(err, ErrFeatureDisabled) {
		t.Fatalf("disabled feature: err = %v", err)
	}
	access, err := disabled.service.GetAccess(context.Background(), 1)
	if err != nil || access.Enabled {
		t.Fatalf("access = %#v, %v", access, err)
	}

	enabled := newFixture(t, nil)
	if _, err := enabled.service.Create(context.Background(), 2, validCreate(), RequestMeta{}); err != nil {
		t.Fatalf("every user may add a provider once enabled: %v", err)
	}
}

func TestLinkImportFollowsTheFeatureSwitch(t *testing.T) {
	input := validCreate()
	input.FromLink = true
	item, err := newFixture(t, nil).service.Create(context.Background(), 1, input, RequestMeta{})
	if err != nil || item.Source != domainpersonalprovider.SourceLink {
		t.Fatalf("link import = %#v, %v", item, err)
	}
	disabled := newFixture(t, func(cfg *config.Config) { cfg.PersonalProvidersEnabled = false })
	if _, err := disabled.service.Create(context.Background(), 1, input, RequestMeta{}); !errors.Is(err, ErrFeatureDisabled) {
		t.Fatalf("err = %v, want ErrFeatureDisabled", err)
	}
}

func TestPerUserLimit(t *testing.T) {
	fixture := newFixture(t, func(cfg *config.Config) { cfg.PersonalProvidersMaxPerUser = 1 })
	if _, err := fixture.service.Create(context.Background(), 1, validCreate(), RequestMeta{}); err != nil {
		t.Fatal(err)
	}
	second := validCreate()
	second.APIKey = "sk-test-second-key-00000000000"
	if _, err := fixture.service.Create(context.Background(), 1, second, RequestMeta{}); !errors.Is(err, ErrLimitReached) {
		t.Fatalf("err = %v, want ErrLimitReached", err)
	}
	if _, err := fixture.service.Create(context.Background(), 2, validCreate(), RequestMeta{}); err != nil {
		t.Fatalf("limit must be per user: %v", err)
	}
}

func TestUpstreamErrorsAreClassifiedWithoutLeakingUpstreamText(t *testing.T) {
	fixture := newFixture(t, nil)
	fixture.lister.err = &llm.UpstreamError{StatusCode: 401, Message: "invalid key sk-test-abcdefghijklmnop1234", Body: "sk-test-abcdefghijklmnop1234"}
	_, err := fixture.service.Probe(context.Background(), 1, CandidateInput{
		Protocol: llm.AdapterOpenAIChatCompletions,
		BaseURL:  "https://api.openai.com/v1",
		APIKey:   "sk-test-abcdefghijklmnop1234",
	})
	if !errors.Is(err, ErrUpstreamRejected) {
		t.Fatalf("err = %v, want ErrUpstreamRejected", err)
	}
	if strings.Contains(err.Error(), "sk-test") {
		t.Fatalf("error leaked upstream text: %v", err)
	}
	fixture.lister.err = &llm.UpstreamError{StatusCode: 503}
	if _, err := fixture.service.Probe(context.Background(), 1, CandidateInput{
		Protocol: llm.AdapterOpenAIChatCompletions,
		BaseURL:  "https://api.openai.com/v1",
		APIKey:   "sk-test-abcdefghijklmnop1234",
	}); !errors.Is(err, ErrUpstreamUnavailable) {
		t.Fatalf("err = %v, want ErrUpstreamUnavailable", err)
	}
}

func TestResolveIsScopedToTheOwnerAndCurrentPolicy(t *testing.T) {
	fixture := newFixture(t, nil)
	item, err := fixture.service.Create(context.Background(), 1, validCreate(), RequestMeta{})
	if err != nil {
		t.Fatal(err)
	}
	ref := domainpersonalprovider.FormatModelRef(item.PublicID, "gpt-4o")

	route, err := fixture.service.Resolve(context.Background(), 1, ref)
	if err != nil {
		t.Fatalf("owner resolve: %v", err)
	}
	if route.APIKey != "sk-test-abcdefghijklmnop1234" || route.Model != "gpt-4o" {
		t.Fatalf("route = %#v", route)
	}

	if _, err := fixture.service.Resolve(context.Background(), 2, ref); !errors.Is(err, ErrModelUnavailable) {
		t.Fatalf("another user resolving the ref: err = %v", err)
	}
	if _, err := fixture.service.Resolve(context.Background(), 1, domainpersonalprovider.FormatModelRef(item.PublicID, "o4-mini")); !errors.Is(err, ErrModelUnavailable) {
		t.Fatalf("model not enabled by the user: err = %v", err)
	}

	// 管理员事后把域名加入禁止列表，已保存的服务立即不可用。
	cfg := fixture.runtime.Snapshot()
	cfg.PersonalProvidersBlockedHosts = "*.openai.com"
	fixture.runtime.Store(cfg)
	if _, err := fixture.service.Resolve(context.Background(), 1, ref); !errors.Is(err, ErrModelUnavailable) {
		t.Fatalf("blocked host: err = %v", err)
	}
	cfg.PersonalProvidersBlockedHosts = ""
	cfg.PersonalProvidersEnabled = false
	fixture.runtime.Store(cfg)
	if _, err := fixture.service.Resolve(context.Background(), 1, ref); !errors.Is(err, ErrModelUnavailable) {
		t.Fatalf("feature disabled: err = %v", err)
	}
}

// 密文绑定到所属用户与服务：直接改库把 Key 挪到别的记录上，解密失败而不是被别的服务使用。
func TestStoredKeysOnlyDecryptOnTheirOwnRecord(t *testing.T) {
	fixture := newFixture(t, nil)
	first, err := fixture.service.Create(context.Background(), 1, validCreate(), RequestMeta{})
	if err != nil {
		t.Fatal(err)
	}
	other := validCreate()
	other.APIKey = "sk-other-abcdefghijklmnop9876"
	second, err := fixture.service.Create(context.Background(), 1, other, RequestMeta{})
	if err != nil {
		t.Fatal(err)
	}

	fixture.repo.mu.Lock()
	fixture.repo.items[1].APIKeyEnc = first.APIKeyEnc
	fixture.repo.items[0].OwnerUserID = 2
	fixture.repo.mu.Unlock()

	for _, check := range []struct {
		name  string
		owner uint
		ref   string
	}{
		{"ciphertext copied to another provider", 1, domainpersonalprovider.FormatModelRef(second.PublicID, "gpt-4o")},
		{"provider moved to another user", 2, domainpersonalprovider.FormatModelRef(first.PublicID, "gpt-4o")},
	} {
		if _, err := fixture.service.Resolve(context.Background(), check.owner, check.ref); !errors.Is(err, ErrModelUnavailable) {
			t.Fatalf("%s: err = %v", check.name, err)
		}
	}
}

func TestSuspendedProvidersCannotBeReenabledByTheUser(t *testing.T) {
	fixture := newFixture(t, nil)
	item, err := fixture.service.Create(context.Background(), 1, validCreate(), RequestMeta{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.service.AdminSuspendHost(context.Background(), 99, "api.openai.com", RequestMeta{}); err != nil {
		t.Fatal(err)
	}
	enabled := true
	if _, err := fixture.service.Update(context.Background(), 1, item.PublicID, UpdateInput{Enabled: &enabled}, RequestMeta{}); !errors.Is(err, ErrSuspended) {
		t.Fatalf("err = %v, want ErrSuspended", err)
	}
	if _, err := fixture.service.Resolve(context.Background(), 1, domainpersonalprovider.FormatModelRef(item.PublicID, "gpt-4o")); !errors.Is(err, ErrModelUnavailable) {
		t.Fatalf("suspended provider still resolves: %v", err)
	}
	models, err := fixture.service.ListActiveModels(context.Background(), 1)
	if err != nil || len(models) != 0 {
		t.Fatalf("suspended provider still listed: %#v, %v", models, err)
	}
}

func TestDeleteWorksAfterTheFeatureIsTurnedOff(t *testing.T) {
	fixture := newFixture(t, nil)
	item, err := fixture.service.Create(context.Background(), 1, validCreate(), RequestMeta{})
	if err != nil {
		t.Fatal(err)
	}
	cfg := fixture.runtime.Snapshot()
	cfg.PersonalProvidersEnabled = false
	fixture.runtime.Store(cfg)
	if err := fixture.service.Delete(context.Background(), 1, item.PublicID, RequestMeta{}); err != nil {
		t.Fatalf("delete after disable: %v", err)
	}
	if err := fixture.service.Delete(context.Background(), 2, item.PublicID, RequestMeta{}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("other user deleting: err = %v", err)
	}
}

// ---------------------------------------------------------------------------
// 组合路由
// ---------------------------------------------------------------------------

type fakePlatform struct {
	resolveCalls int
	failures     int
}

func (f *fakePlatform) ResolveRoute(context.Context, channel.ResolveRouteInput) (*channel.ResolvedRoute, error) {
	f.resolveCalls++
	return &channel.ResolvedRoute{PlatformModelName: "gpt-platform", UpstreamID: 7, RouteID: 3}, nil
}

func (f *fakePlatform) ResolveDefaultRoute(context.Context, channel.ResolveRouteInput) (*channel.ResolvedRoute, error) {
	return &channel.ResolvedRoute{PlatformModelName: "default"}, nil
}

func (f *fakePlatform) MarkRouteFailure(context.Context, *channel.ResolvedRoute, error) { f.failures++ }
func (f *fakePlatform) MarkRouteSuccess(context.Context, *channel.ResolvedRoute)        {}
func (f *fakePlatform) ListActiveModels(context.Context, uint) ([]channel.ModelView, error) {
	return nil, nil
}

// BuildExternalRoute 使用真实实现（零值服务没有内置目录），以覆盖按任务选协议的规则。
func (f *fakePlatform) BuildExternalRoute(input channel.ExternalRouteInput) (*channel.ResolvedRoute, error) {
	return (&channel.Service{}).BuildExternalRoute(input)
}

func TestRouteResolverNeverFallsBackToPlatformForPersonalRefs(t *testing.T) {
	fixture := newFixture(t, nil)
	item, err := fixture.service.Create(context.Background(), 1, validCreate(), RequestMeta{})
	if err != nil {
		t.Fatal(err)
	}
	platform := &fakePlatform{}
	resolver := NewRouteResolver(platform, fixture.service)
	ref := domainpersonalprovider.FormatModelRef(item.PublicID, "gpt-4o")

	route, err := resolver.ResolveRoute(context.Background(), channel.ResolveRouteInput{PlatformModelName: ref, TaskType: channel.TaskTypeChat, UserID: 1})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if !route.UntrustedEndpoint || route.UpstreamModel != "gpt-4o" || route.PlatformModelName != ref {
		t.Fatalf("route = %#v", route)
	}

	for name, input := range map[string]channel.ResolveRouteInput{
		"other user":             {PlatformModelName: ref, TaskType: channel.TaskTypeChat, UserID: 2},
		"chat model, image task": {PlatformModelName: ref, TaskType: channel.TaskTypeImageGeneration, UserID: 1},
		"failover":               {PlatformModelName: ref, TaskType: channel.TaskTypeChat, UserID: 1, ExcludedRouteIDs: []uint{1}},
		"malformed ref":          {PlatformModelName: "personal:nope", TaskType: channel.TaskTypeChat, UserID: 1},
		"internal no uid":        {PlatformModelName: ref, TaskType: channel.TaskTypeChat, Scope: channel.RouteScopeInternal},
	} {
		if _, err := resolver.ResolveRoute(context.Background(), input); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
	if platform.resolveCalls != 0 {
		t.Fatalf("personal refs reached the platform router %d times", platform.resolveCalls)
	}

	if _, err := resolver.ResolveRoute(context.Background(), channel.ResolveRouteInput{PlatformModelName: "gpt-platform", TaskType: channel.TaskTypeChat, UserID: 1}); err != nil || platform.resolveCalls != 1 {
		t.Fatalf("platform models must be delegated: %v, calls=%d", err, platform.resolveCalls)
	}

	resolver.MarkRouteFailure(context.Background(), route, errors.New("boom"))
	if platform.failures != 0 {
		t.Fatal("personal route failures must not touch platform circuit breakers")
	}
}

func TestCreateRejectsTheSameKeyForTheSameAddress(t *testing.T) {
	fixture := newFixture(t, nil)
	if _, err := fixture.service.Create(context.Background(), 1, validCreate(), RequestMeta{}); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.service.Create(context.Background(), 1, validCreate(), RequestMeta{}); !errors.Is(err, ErrDuplicate) {
		t.Fatalf("err = %v, want ErrDuplicate", err)
	}
	rotated := validCreate()
	rotated.APIKey = "sk-test-another-key-0000000000"
	if _, err := fixture.service.Create(context.Background(), 1, rotated, RequestMeta{}); err != nil {
		t.Fatalf("a different key on the same address must be allowed: %v", err)
	}
	if _, err := fixture.service.Create(context.Background(), 2, validCreate(), RequestMeta{}); err != nil {
		t.Fatalf("another user's identical key must be allowed: %v", err)
	}
}

func TestIconIsOptionalAndOnlyAcceptsSlugs(t *testing.T) {
	fixture := newFixture(t, nil)
	input := validCreate()
	input.Icon = " OpenAI "
	item, err := fixture.service.Create(context.Background(), 1, input, RequestMeta{})
	if err != nil || item.Icon != "openai" {
		t.Fatalf("create with icon = %#v, %v", item, err)
	}
	pixel := "https://evil.example/pixel.png"
	if _, err := fixture.service.Update(context.Background(), 1, item.PublicID, UpdateInput{Icon: &pixel}, RequestMeta{}); !errors.Is(err, ErrInvalidIcon) {
		t.Fatalf("url icon: err = %v, want ErrInvalidIcon", err)
	}
	auto := ""
	updated, err := fixture.service.Update(context.Background(), 1, item.PublicID, UpdateInput{Icon: &auto}, RequestMeta{})
	if err != nil || updated.Icon != "" {
		t.Fatalf("reset icon = %#v, %v", updated, err)
	}
}

// 每个模型单独指定协议：图片模型按名称推断为图片协议，任务按模型协议路由，非法组合被拒绝。
func TestModelsCarryTheirOwnProtocols(t *testing.T) {
	fixture := newFixture(t, nil)
	fixture.lister.models = []llm.ModelItem{{ID: "gpt-4o"}, {ID: "gpt-image-1"}, {ID: "relay-draw"}}
	input := validCreate()
	input.Models = []ModelInput{
		{Name: "gpt-4o"},
		{Name: "gpt-image-1"},
		{Name: "relay-draw", Protocols: []string{llm.AdapterOpenAIImageGenerations}},
	}
	item, err := fixture.service.Create(context.Background(), 1, input, RequestMeta{})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	got := map[string]string{}
	for _, model := range item.Models {
		got[model.Name] = strings.Join(model.Protocols, ",")
	}
	want := map[string]string{
		"gpt-4o":      llm.AdapterOpenAIChatCompletions,
		"gpt-image-1": llm.AdapterOpenAIImageGenerations + "," + llm.AdapterOpenAIImageEdits,
		"relay-draw":  llm.AdapterOpenAIImageGenerations,
	}
	for name, protocols := range want {
		if got[name] != protocols {
			t.Errorf("%s protocols = %q, want %q", name, got[name], protocols)
		}
	}

	resolver := NewRouteResolver(&fakePlatform{}, fixture.service)
	resolve := func(model string, task string) (*channel.ResolvedRoute, error) {
		return resolver.ResolveRoute(context.Background(), channel.ResolveRouteInput{
			PlatformModelName: domainpersonalprovider.FormatModelRef(item.PublicID, model), TaskType: task, UserID: 1,
		})
	}
	for _, check := range []struct {
		model, task, protocol string
	}{
		{"gpt-image-1", channel.TaskTypeImageGeneration, llm.AdapterOpenAIImageGenerations},
		{"gpt-image-1", channel.TaskTypeImageEdit, llm.AdapterOpenAIImageEdits},
		{"relay-draw", channel.TaskTypeImageGeneration, llm.AdapterOpenAIImageGenerations},
		{"gpt-4o", channel.TaskTypeChat, llm.AdapterOpenAIChatCompletions},
	} {
		route, err := resolve(check.model, check.task)
		if err != nil || route.Protocol != check.protocol || !route.UntrustedEndpoint {
			t.Errorf("%s/%s: route = %#v, err = %v", check.model, check.task, route, err)
		}
	}
	for _, check := range [][2]string{
		{"gpt-image-1", channel.TaskTypeChat},
		{"relay-draw", channel.TaskTypeImageEdit},
		{"gpt-4o", channel.TaskTypeImageGeneration},
	} {
		if _, err := resolve(check[0], check[1]); !errors.Is(err, channel.ErrRouteNotFound) {
			t.Errorf("%s/%s: err = %v, want ErrRouteNotFound", check[0], check[1], err)
		}
	}

	for name, protocols := range map[string][]string{
		"unknown":          {"made_up_protocol"},
		"not implemented":  {"openai_video_generations"},
		"bad combination":  {llm.AdapterOpenAIChatCompletions, llm.AdapterOpenAIImageGenerations},
		"cross-vendor set": {llm.AdapterOpenAIImageGenerations, llm.AdapterXAIImageEdits},
	} {
		models := []ModelInput{{Name: "gpt-4o", Protocols: protocols}}
		if _, err := fixture.service.Update(context.Background(), 1, item.PublicID, UpdateInput{Models: &models}, RequestMeta{}); !errors.Is(err, ErrInvalidModelProtocol) {
			t.Errorf("%s: err = %v, want ErrInvalidModelProtocol", name, err)
		}
	}
}

func TestAllowedProtocolsAreTheStreamingChatProtocols(t *testing.T) {
	want := []string{
		llm.AdapterOpenAIChatCompletions,
		llm.AdapterOpenAIResponses,
		llm.AdapterAnthropicMessages,
		llm.AdapterGoogleGenerateContent,
		llm.AdapterGeminiInteractions,
		llm.AdapterXAIResponses,
		llm.AdapterOpenRouterChat,
		llm.AdapterOpenRouterResponses,
	}
	got := AllowedProtocols()
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("AllowedProtocols() = %v, want %v", got, want)
	}
	for _, protocol := range got {
		if !llm.IsImplementedAdapter(protocol) || !llm.SupportsStreamingAdapter(protocol) {
			t.Fatalf("%s must be an implemented streaming adapter", protocol)
		}
	}
	// 服务协议用于拉取模型目录，只能是对话协议；图片、视频协议在单个模型上指定。
	for _, media := range []string{llm.AdapterOpenAIImageGenerations, llm.AdapterOpenRouterImages, llm.AdapterGoogleImageGeneration, llm.AdapterXAIImage, llm.AdapterXAIVideo} {
		if isAllowedProtocol(media) {
			t.Fatalf("media protocol %s must not be a provider protocol", media)
		}
	}
	for _, protocol := range ModelProtocols() {
		if !llm.IsImplementedAdapter(protocol) {
			t.Fatalf("model protocol %s must be implemented", protocol)
		}
	}
}

// 同一条服务更换 API Key 后仍可正常使用，且只会用新 Key。
func TestReplacingTheKeyKeepsTheProviderUsable(t *testing.T) {
	fixture := newFixture(t, nil)
	item, err := fixture.service.Create(context.Background(), 1, validCreate(), RequestMeta{})
	if err != nil {
		t.Fatal(err)
	}
	newKey := "sk-rotated-abcdefghijklmn5678"
	updated, err := fixture.service.Update(context.Background(), 1, item.PublicID, UpdateInput{APIKey: &newKey}, RequestMeta{})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.PublicID != item.PublicID || updated.APIKeyEnc == item.APIKeyEnc {
		t.Fatalf("expected the same record with a new ciphertext")
	}
	if updated.KeyHint != "sk-••••5678" {
		t.Fatalf("key hint = %q", updated.KeyHint)
	}
	resolver := NewRouteResolver(&fakePlatform{}, fixture.service)
	ref := domainpersonalprovider.FormatModelRef(item.PublicID, "gpt-4o")
	route, err := resolver.ResolveRoute(context.Background(), channel.ResolveRouteInput{PlatformModelName: ref, TaskType: channel.TaskTypeChat, UserID: 1})
	if err != nil {
		t.Fatalf("resolve after replacing the key: %v", err)
	}
	if route.APIKey != newKey {
		t.Fatalf("route uses %q, want the new key", route.APIKey)
	}
	if _, err := fixture.service.ListAvailableModels(context.Background(), 1, item.PublicID); err != nil {
		t.Fatalf("re-check after replacing the key: %v", err)
	}
}

// 审计会同时写入应用日志，Key 的任何片段（包括打码提示里保留的首尾几位）都不应出现在其中。
func TestAuditEntriesCarryNoPartOfTheKey(t *testing.T) {
	fixture := newFixture(t, nil)
	item, err := fixture.service.Create(context.Background(), 1, validCreate(), RequestMeta{})
	if err != nil {
		t.Fatal(err)
	}
	rotated := "sk-rotated-abcdefghijklmn5678"
	if _, err := fixture.service.Update(context.Background(), 1, item.PublicID, UpdateInput{APIKey: &rotated}, RequestMeta{}); err != nil {
		t.Fatal(err)
	}
	if err := fixture.service.Delete(context.Background(), 1, item.PublicID, RequestMeta{}); err != nil {
		t.Fatal(err)
	}
	if len(fixture.audit.entries) != 3 {
		t.Fatalf("expected create, update and delete entries, got %d", len(fixture.audit.entries))
	}
	// 两个 Key 的结尾 4 位、打码符号，以及提示字段本身。
	for _, fragment := range []string{"1234", "5678", "••••", "key_hint", "sk-"} {
		for _, entry := range fixture.audit.entries {
			if strings.Contains(stringify(entry.Detail), fragment) {
				t.Fatalf("audit entry %s contains %q: %#v", entry.Action, fragment, entry.Detail)
			}
		}
	}
	if detail := fixture.audit.entries[1].Detail.(map[string]any); detail["key_rotated"] != true || detail["host"] != "api.openai.com" {
		t.Fatalf("the update entry must still say that the key changed and where: %#v", detail)
	}
}
