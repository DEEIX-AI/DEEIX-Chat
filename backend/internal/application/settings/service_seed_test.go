package settings

import (
	"context"
	"testing"

	domainsettings "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/settings"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
)

type settingsSeedRepo struct {
	items map[string]domainsettings.SystemSetting
}

func newSettingsSeedRepo(items ...domainsettings.SystemSetting) *settingsSeedRepo {
	repo := &settingsSeedRepo{items: map[string]domainsettings.SystemSetting{}}
	for _, item := range items {
		repo.items[item.Namespace+":"+item.Key] = item
	}
	return repo
}

func (r *settingsSeedRepo) ListAll(context.Context) ([]domainsettings.SystemSetting, error) {
	items := make([]domainsettings.SystemSetting, 0, len(r.items))
	for _, item := range r.items {
		items = append(items, item)
	}
	return items, nil
}

func (r *settingsSeedRepo) ListByNamespace(_ context.Context, namespace string) ([]domainsettings.SystemSetting, error) {
	items := make([]domainsettings.SystemSetting, 0)
	for _, item := range r.items {
		if item.Namespace == namespace {
			items = append(items, item)
		}
	}
	return items, nil
}

func (r *settingsSeedRepo) Upsert(_ context.Context, items []domainsettings.SystemSetting) error {
	for _, item := range items {
		r.items[item.Namespace+":"+item.Key] = item
	}
	return nil
}

func (r *settingsSeedRepo) UpsertWithDescription(_ context.Context, items []domainsettings.SystemSetting) error {
	for _, item := range items {
		key := item.Namespace + ":" + item.Key
		if _, ok := r.items[key]; !ok {
			r.items[key] = item
		}
	}
	return nil
}

func (r *settingsSeedRepo) Delete(_ context.Context, namespace string, key string) error {
	delete(r.items, namespace+":"+key)
	return nil
}

func TestSeedMigratesLegacyDefaultAllowedMIMETypes(t *testing.T) {
	repo := newSettingsSeedRepo(domainsettings.SystemSetting{
		Namespace: "file",
		Key:       "allowed_mime_types",
		Value:     legacyDefaultAllowedMIMETypes,
		ValueType: "string",
	})
	service := NewService(repo, "")

	if err := service.Seed(context.Background(), config.Config{}); err != nil {
		t.Fatalf("seed settings: %v", err)
	}
	got := repo.items["file:allowed_mime_types"].Value
	if got != defaultAllowedMIMETypes {
		t.Fatalf("expected legacy MIME defaults to migrate, got %q", got)
	}
}

func TestSeedKeepsCustomAllowedMIMETypes(t *testing.T) {
	custom := "image/png,text/plain"
	repo := newSettingsSeedRepo(domainsettings.SystemSetting{
		Namespace: "file",
		Key:       "allowed_mime_types",
		Value:     custom,
		ValueType: "string",
	})
	service := NewService(repo, "")

	if err := service.Seed(context.Background(), config.Config{}); err != nil {
		t.Fatalf("seed settings: %v", err)
	}
	got := repo.items["file:allowed_mime_types"].Value
	if got != custom {
		t.Fatalf("expected custom MIME defaults to stay unchanged, got %q", got)
	}
}
