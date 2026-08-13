package contentmoderation

import (
	"context"
	"errors"
	"testing"

	domaincm "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/contentmoderation"
	domainsettings "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/settings"
	"go.uber.org/zap"
)

type configTestSettingsRepo struct {
	items []domainsettings.SystemSetting
	err   error
}

type configTestProvider struct{}

func (configTestProvider) ValidateBaseURL(string) error { return nil }

func (configTestProvider) ModerateText(
	context.Context,
	ProviderConfig,
	string,
	[]string,
	string,
) (*Response, error) {
	return nil, nil
}

func (configTestProvider) ModerateImages(
	context.Context,
	ProviderConfig,
	[]ProviderImage,
	[]string,
	string,
) (*Response, error) {
	return nil, nil
}

func (r *configTestSettingsRepo) ListAll(context.Context) ([]domainsettings.SystemSetting, error) {
	return append([]domainsettings.SystemSetting(nil), r.items...), nil
}

func (r *configTestSettingsRepo) ListByNamespace(_ context.Context, namespace string) ([]domainsettings.SystemSetting, error) {
	if r.err != nil {
		return nil, r.err
	}
	items := make([]domainsettings.SystemSetting, 0, len(r.items))
	for _, item := range r.items {
		if item.Namespace == namespace {
			items = append(items, item)
		}
	}
	return items, nil
}

func TestBeginRunRecordsConfigurationFailureAsFailedOpen(t *testing.T) {
	settingsRepo := &configTestSettingsRepo{err: errors.New("database unavailable: sensitive detail")}
	moderationRepo := &coordinatorTestRepo{}
	service := NewService(settingsRepo, moderationRepo, "test-data-encryption-key", zap.NewNop())

	coordinator := service.BeginRun(context.Background(), RunMeta{RunID: "run_config_failure", UserID: 42})
	if coordinator == nil {
		t.Fatal("configuration failure must return a failed-open coordinator")
	}
	result := coordinator.WaitInputOnly(context.Background())
	if result.State != domaincm.ModerationStateFailedOpen {
		t.Fatalf("state = %q, want failed_open", result.State)
	}

	moderationRepo.mu.Lock()
	defer moderationRepo.mu.Unlock()
	if len(moderationRepo.events) != 1 {
		t.Fatalf("events = %d, want 1", len(moderationRepo.events))
	}
	event := moderationRepo.events[0]
	if event.ErrorCode != domaincm.ErrorCodeConfigMissing {
		t.Fatalf("error code = %q", event.ErrorCode)
	}
	if event.ErrorMessage != "content moderation configuration unavailable" {
		t.Fatalf("unsafe or unexpected persisted error = %q", event.ErrorMessage)
	}
	if moderationRepo.runState != domaincm.ModerationStateFailedOpen {
		t.Fatalf("run state = %q, want failed_open", moderationRepo.runState)
	}
}

func (r *configTestSettingsRepo) Upsert(_ context.Context, items []domainsettings.SystemSetting) error {
	for _, next := range items {
		replaced := false
		for index := range r.items {
			if r.items[index].Namespace == next.Namespace && r.items[index].Key == next.Key {
				r.items[index] = next
				replaced = true
				break
			}
		}
		if !replaced {
			r.items = append(r.items, next)
		}
	}
	return nil
}

func (r *configTestSettingsRepo) UpsertWithDescription(ctx context.Context, items []domainsettings.SystemSetting) error {
	return r.Upsert(ctx, items)
}

func (r *configTestSettingsRepo) Delete(_ context.Context, namespace, key string) error {
	for index, item := range r.items {
		if item.Namespace == namespace && item.Key == key {
			r.items = append(r.items[:index], r.items[index+1:]...)
			break
		}
	}
	return nil
}

func TestGetConfigRequiresExplicitEnabledState(t *testing.T) {
	repo := &configTestSettingsRepo{items: []domainsettings.SystemSetting{
		{Namespace: settingsNamespace, Key: keyPolicyJSON, Value: `{"inputTextCategories":["hate"]}`},
		{Namespace: settingsNamespace, Key: keyPolicyVersion, Value: "3"},
	}}
	service := NewService(repo, nil, "test-data-encryption-key", zap.NewNop())

	config, err := service.GetConfig(context.Background(), "superadmin")
	if err != nil {
		t.Fatalf("get config: %v", err)
	}
	if config.Enabled {
		t.Fatal("expected missing enabled setting to default to disabled")
	}
	if config.Policy.Version != 3 {
		t.Fatalf("expected policy version 3, got %d", config.Policy.Version)
	}
}

func TestGetConfigHonorsExplicitDisabledStateWithRetainedPolicy(t *testing.T) {
	repo := &configTestSettingsRepo{items: []domainsettings.SystemSetting{
		{Namespace: settingsNamespace, Key: keyEnabled, Value: "false"},
		{Namespace: settingsNamespace, Key: keyPolicyJSON, Value: `{"inputTextCategories":["hate"]}`},
	}}
	service := NewService(repo, nil, "test-data-encryption-key", zap.NewNop())

	config, err := service.GetConfig(context.Background(), "superadmin")
	if err != nil {
		t.Fatalf("get config: %v", err)
	}
	if config.Enabled {
		t.Fatal("expected explicit disabled state to override the retained policy")
	}
	if len(config.Policy.InputTextCategories) != 1 {
		t.Fatalf("expected retained policy, got %#v", config.Policy.InputTextCategories)
	}
}

func TestUpdateConfigDisablesWithoutClearingPolicy(t *testing.T) {
	repo := &configTestSettingsRepo{items: []domainsettings.SystemSetting{
		{Namespace: settingsNamespace, Key: keyEnabled, Value: "true"},
		{Namespace: settingsNamespace, Key: keyPolicyJSON, Value: `{"inputTextCategories":["hate"]}`},
	}}
	service := NewService(repo, nil, "test-data-encryption-key", zap.NewNop())
	enabled := false

	config, err := service.UpdateConfig(context.Background(), "superadmin", UpdateConfigInput{Enabled: &enabled})
	if err != nil {
		t.Fatalf("disable config: %v", err)
	}
	if config.Enabled {
		t.Fatal("expected moderation to be disabled")
	}
	if len(config.Policy.InputTextCategories) != 1 || config.Policy.InputTextCategories[0] != "hate" {
		t.Fatalf("expected policy to be retained, got %#v", config.Policy.InputTextCategories)
	}
}

func TestUpdateConfigRequiresServiceAndPolicyWhenEnabled(t *testing.T) {
	repo := &configTestSettingsRepo{}
	service := NewService(repo, nil, "test-data-encryption-key", zap.NewNop())
	service.SetProvider(configTestProvider{})
	enabled := true

	_, err := service.UpdateConfig(context.Background(), "superadmin", UpdateConfigInput{Enabled: &enabled})
	if !errors.Is(err, ErrServiceConfigRequired) {
		t.Fatalf("expected ErrServiceConfigRequired, got %v", err)
	}

	apiKey := "test-api-key"
	policy := Policy{InputTextCategories: []string{"hate"}}
	config, err := service.UpdateConfig(context.Background(), "superadmin", UpdateConfigInput{
		Enabled: &enabled,
		APIKey:  &apiKey,
		Policy:  &policy,
	})
	if err != nil {
		t.Fatalf("enable complete config: %v", err)
	}
	if !config.Enabled || !config.HasAPIKey {
		t.Fatalf("expected enabled config with API key, got %#v", config)
	}
}
