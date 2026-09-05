package conversation

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
	appusersettings "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/usersettings"
	domainusersettings "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/usersettings"
	memorycache "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/cache/memory"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

type mutableUserSettingsRepository struct {
	repository.ConversationRepository
	mu        sync.RWMutex
	values    map[uint]map[string]string
	beforeGet func()
}

func (r *mutableUserSettingsRepository) GetUserSettingValue(_ context.Context, userID uint, key string) (string, error) {
	r.mu.RLock()
	value := r.values[userID][key]
	r.mu.RUnlock()
	if r.beforeGet != nil {
		r.beforeGet()
	}
	return value, nil
}

func (r *mutableUserSettingsRepository) GetUserSettingValues(_ context.Context, userID uint, keys []string) (map[string]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	values := make(map[string]string, len(keys))
	for _, key := range keys {
		values[key] = r.values[userID][key]
	}
	return values, nil
}

func (r *mutableUserSettingsRepository) ListByUserID(_ context.Context, userID uint) ([]domainusersettings.UserSetting, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	values := r.values[userID]
	items := make([]domainusersettings.UserSetting, 0, len(values))
	for key, value := range values {
		items = append(items, domainusersettings.UserSetting{UserID: userID, Key: key, Value: value})
	}
	return items, nil
}

func (r *mutableUserSettingsRepository) Upsert(_ context.Context, items []domainusersettings.UserSetting) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, item := range items {
		if r.values[item.UserID] == nil {
			r.values[item.UserID] = make(map[string]string)
		}
		r.values[item.UserID][item.Key] = item.Value
	}
	return nil
}

func (r *mutableUserSettingsRepository) setValue(userID uint, key, value string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.values[userID][key] = value
}

type failingUpsertUserSettingsRepository struct {
	*mutableUserSettingsRepository
}

func (r *failingUpsertUserSettingsRepository) Upsert(_ context.Context, _ []domainusersettings.UserSetting) error {
	return errors.New("upsert failed")
}

type userSettingTestRepository interface {
	repository.ConversationRepository
	repository.UserSettingsRepository
}

func newUserSettingTestServices(repo userSettingTestRepository, runtimeCfg *config.Runtime) (*Service, *appusersettings.Service, repository.UserSettingCacheRepository) {
	cache := memorycache.New()
	conversationService := &Service{cfg: runtimeCfg, repo: repo, cache: cache}
	settingsService := appusersettings.NewService(repo)
	settingsService.SetCacheRefresher(conversationService.RefreshUserSettingCache)
	return conversationService, settingsService, cache
}

// TestIssue589UserSettingChangesTakeEffectImmediately covers every setting reported in #589.
func TestIssue589UserSettingChangesTakeEffectImmediately(t *testing.T) {
	const userID uint = 17
	ctx := context.Background()
	runtimeCfg := config.NewRuntime(config.Config{ContextCompactEnabled: true})
	repo := &mutableUserSettingsRepository{
		values: map[uint]map[string]string{
			userID: {
				"chat.reasoning_content_passback": "true",
				"chat.context_compact_auto":       "true",
				"chat.file_mode":                  "auto",
			},
		},
	}
	conversationService, settingsService, cache := newUserSettingTestServices(repo, runtimeCfg)

	if !conversationService.reasoningContentPassbackEnabled(ctx, userID, &channel.ResolvedRoute{ReasoningContentPassback: true}) {
		t.Fatal("expected initial reasoning passback to be enabled")
	}
	if !conversationService.resolveContextCompactionPolicy(ctx, runtimeCfg.Snapshot(), userID).EffectiveEnabled() {
		t.Fatal("expected initial context compaction to be enabled")
	}
	if value, err := conversationService.getUserSettingCached(ctx, userID, "chat.file_mode"); err != nil || value != "auto" {
		t.Fatalf("initial file mode = %q (err %v), want auto", value, err)
	}

	if _, err := settingsService.PatchSettings(ctx, userID, map[string]string{
		"chat.reasoning_content_passback": "false",
		"chat.context_compact_auto":       "false",
		"chat.file_mode":                  "rag",
	}); err != nil {
		t.Fatalf("patch settings: %v", err)
	}

	for key, want := range map[string]string{
		"chat.reasoning_content_passback": "false",
		"chat.context_compact_auto":       "false",
		"chat.file_mode":                  "rag",
	} {
		version, err := cache.GetUserSettingCacheVersion(ctx, userID, key, userSettingCacheTTL)
		if err != nil || version == "" {
			t.Fatalf("cache version for %q = %q (err %v), want a version", key, version, err)
		}
		value, ok, err := cache.GetUserSettingCache(ctx, userID, key, version)
		if err != nil || !ok || value != want {
			t.Fatalf("refreshed cache for %q = %q (ok %v, err %v), want %q", key, value, ok, err, want)
		}
	}

	if conversationService.reasoningContentPassbackEnabled(ctx, userID, &channel.ResolvedRoute{ReasoningContentPassback: true}) {
		t.Fatal("expected patched reasoning passback to be disabled")
	}
	if conversationService.resolveContextCompactionPolicy(ctx, runtimeCfg.Snapshot(), userID).EffectiveEnabled() {
		t.Fatal("expected patched context compaction to be disabled")
	}
	if value, err := conversationService.getUserSettingCached(ctx, userID, "chat.file_mode"); err != nil || value != "rag" {
		t.Fatalf("updated file mode = %q (err %v), want rag", value, err)
	}
}

func TestConversationSettingsCacheDoesNotRepopulateAfterRefresh(t *testing.T) {
	const userID uint = 19
	ctx := context.Background()
	readStarted := make(chan struct{})
	releaseRead := make(chan struct{})
	var blockFirstRead atomic.Bool
	blockFirstRead.Store(true)
	repo := &mutableUserSettingsRepository{
		values: map[uint]map[string]string{
			userID: {"chat.file_mode": "auto"},
		},
		beforeGet: func() {
			if blockFirstRead.CompareAndSwap(true, false) {
				close(readStarted)
				<-releaseRead
			}
		},
	}
	conversationService, settingsService, cache := newUserSettingTestServices(repo, nil)

	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		if value, err := conversationService.getUserSettingCached(ctx, userID, "chat.file_mode"); err != nil || value != "auto" {
			t.Errorf("initial concurrent read = %q (err %v), want auto", value, err)
		}
	}()

	<-readStarted
	if _, err := settingsService.PatchSettings(ctx, userID, map[string]string{"chat.file_mode": "rag"}); err != nil {
		t.Fatalf("patch settings: %v", err)
	}
	close(releaseRead)
	<-readDone

	version, err := cache.GetUserSettingCacheVersion(ctx, userID, "chat.file_mode", userSettingCacheTTL)
	if err != nil || version == "" {
		t.Fatalf("cache version = %q (err %v), want a version", version, err)
	}
	value, ok, err := cache.GetUserSettingCache(ctx, userID, "chat.file_mode", version)
	if err != nil || !ok || value != "rag" {
		t.Fatalf("current cache = %q (ok %v, err %v), want rag", value, ok, err)
	}
	if value, err := conversationService.getUserSettingCached(ctx, userID, "chat.file_mode"); err != nil || value != "rag" {
		t.Fatalf("updated file mode = %q (err %v), want rag", value, err)
	}
}

func TestConversationSettingsRefreshIsSharedAcrossServiceInstances(t *testing.T) {
	const userID uint = 20
	ctx := context.Background()
	repo := &mutableUserSettingsRepository{
		values: map[uint]map[string]string{
			userID: {"chat.file_mode": "auto"},
		},
	}
	sharedCache := memorycache.New()
	firstConversationService := &Service{repo: repo, cache: sharedCache}
	secondConversationService := &Service{repo: repo, cache: sharedCache}
	settingsService := appusersettings.NewService(repo)
	settingsService.SetCacheRefresher(firstConversationService.RefreshUserSettingCache)

	if value, err := secondConversationService.getUserSettingCached(ctx, userID, "chat.file_mode"); err != nil || value != "auto" {
		t.Fatalf("initial second-instance read = %q (err %v), want auto", value, err)
	}
	if _, err := settingsService.PatchSettings(ctx, userID, map[string]string{"chat.file_mode": "full_context"}); err != nil {
		t.Fatalf("patch settings: %v", err)
	}
	if value, err := secondConversationService.getUserSettingCached(ctx, userID, "chat.file_mode"); err != nil || value != "full_context" {
		t.Fatalf("second-instance read after refresh = %q (err %v), want full_context", value, err)
	}
}

func TestConversationSettingsCacheSurvivesFailedUpsert(t *testing.T) {
	const userID uint = 18
	ctx := context.Background()
	base := &mutableUserSettingsRepository{
		values: map[uint]map[string]string{
			userID: {"chat.file_mode": "auto"},
		},
	}
	repo := &failingUpsertUserSettingsRepository{mutableUserSettingsRepository: base}
	conversationService, settingsService, cache := newUserSettingTestServices(repo, nil)

	if value, err := conversationService.getUserSettingCached(ctx, userID, "chat.file_mode"); err != nil || value != "auto" {
		t.Fatalf("initial cached file mode = %q (err %v), want auto", value, err)
	}
	versionBefore, err := cache.GetUserSettingCacheVersion(ctx, userID, "chat.file_mode", userSettingCacheTTL)
	if err != nil {
		t.Fatalf("cache version before failed upsert: %v", err)
	}
	base.setValue(userID, "chat.file_mode", "rag")

	if _, err := settingsService.PatchSettings(ctx, userID, map[string]string{"chat.file_mode": "full_context"}); err == nil {
		t.Fatal("expected patch settings to fail")
	}
	versionAfter, err := cache.GetUserSettingCacheVersion(ctx, userID, "chat.file_mode", userSettingCacheTTL)
	if err != nil || versionAfter != versionBefore {
		t.Fatalf("cache version after failed upsert = %q (err %v), want unchanged %q", versionAfter, err, versionBefore)
	}
	if value, err := conversationService.getUserSettingCached(ctx, userID, "chat.file_mode"); err != nil || value != "auto" {
		t.Fatalf("cached file mode after failed upsert = %q (err %v), want auto", value, err)
	}
}
