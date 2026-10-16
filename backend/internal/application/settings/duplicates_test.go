package settings

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	domainsettings "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/settings"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
)

// duplicateSettingsRepo 按行保存配置，可以表达唯一索引失效后同一 key 的多行数据。
type duplicateSettingsRepo struct {
	rows      []domainsettings.SystemSetting
	inserted  []domainsettings.SystemSetting
	removed   []uint
	removeErr error
}

func (r *duplicateSettingsRepo) ListAll(context.Context) ([]domainsettings.SystemSetting, error) {
	return slices.Clone(r.rows), nil
}

func (r *duplicateSettingsRepo) ListByNamespace(_ context.Context, namespace string) ([]domainsettings.SystemSetting, error) {
	var items []domainsettings.SystemSetting
	for _, item := range r.rows {
		if item.Namespace == namespace {
			items = append(items, item)
		}
	}
	return items, nil
}

func (r *duplicateSettingsRepo) Upsert(context.Context, []domainsettings.SystemSetting) error {
	return nil
}

func (r *duplicateSettingsRepo) UpsertWithDescription(_ context.Context, items []domainsettings.SystemSetting) error {
	r.inserted = append(r.inserted, items...)
	r.rows = append(r.rows, items...)
	return nil
}

func (r *duplicateSettingsRepo) Delete(context.Context, string, string) error {
	return nil
}

func (r *duplicateSettingsRepo) RemoveDuplicates(_ context.Context, ids []uint) error {
	if r.removeErr != nil {
		return r.removeErr
	}
	r.removed = append(r.removed, ids...)
	r.rows = slices.DeleteFunc(r.rows, func(item domainsettings.SystemSetting) bool {
		return slices.Contains(ids, item.ID)
	})
	return nil
}

var (
	configuredAt = time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC)
	reseededAt   = time.Date(2026, 9, 20, 9, 0, 0, 0, time.UTC)
)

// configuredEmbeddingHost 是管理员保存过的行：种子写入后被更新。
func configuredEmbeddingHost() domainsettings.SystemSetting {
	return domainsettings.SystemSetting{
		ID: 471, Namespace: "file", Key: "embedding_host", Value: "http://embedding:8000", ValueType: "string",
		CreatedAt: configuredAt.Add(-time.Hour), UpdatedAt: configuredAt,
	}
}

// reseededEmbeddingHost 是索引失效后种子再插入的默认值行。
func reseededEmbeddingHost() domainsettings.SystemSetting {
	return domainsettings.SystemSetting{
		ID: 3280, Namespace: "file", Key: "embedding_host", Value: "", ValueType: "string",
		CreatedAt: reseededAt, UpdatedAt: reseededAt,
	}
}

func TestResolveDuplicateSettingsKeepsConfiguredRowOverReseededDefault(t *testing.T) {
	for name, rows := range map[string][]domainsettings.SystemSetting{
		"configured first": {configuredEmbeddingHost(), reseededEmbeddingHost()},
		"configured last":  {reseededEmbeddingHost(), configuredEmbeddingHost()},
	} {
		t.Run(name, func(t *testing.T) {
			kept, groups := resolveDuplicateSettings(rows)
			if len(kept) != 1 || kept[0].ID != 471 {
				t.Fatalf("expected configured row 471 to be kept, got %+v", kept)
			}
			if len(groups) != 1 || len(groups[0].Removed) != 1 || groups[0].Removed[0].ID != 3280 {
				t.Fatalf("expected reseeded row 3280 to be removed, got %+v", groups)
			}
		})
	}
}

func TestResolveDuplicateSettingsTreatsNonDefaultValueAsConfigured(t *testing.T) {
	// 直接以非默认值插入的行没有更新时间差，只能靠与注册表默认值比较识别。
	imported := configuredEmbeddingHost()
	imported.CreatedAt = imported.UpdatedAt

	kept, _ := resolveDuplicateSettings([]domainsettings.SystemSetting{reseededEmbeddingHost(), imported})
	if kept[0].ID != imported.ID {
		t.Fatalf("expected imported row to be kept, got id %d", kept[0].ID)
	}
}

func TestResolveDuplicateSettingsPrefersLatestUpdate(t *testing.T) {
	older := configuredEmbeddingHost()
	newer := reseededEmbeddingHost()
	newer.Value = "http://embedding-new:8000"
	newer.UpdatedAt = reseededAt.Add(time.Minute)

	kept, _ := resolveDuplicateSettings([]domainsettings.SystemSetting{newer, older})
	if kept[0].ID != newer.ID {
		t.Fatalf("expected the latest update to win, got id %d", kept[0].ID)
	}
}

func TestResolveDuplicateSettingsBreaksTiesByLowestID(t *testing.T) {
	first := reseededEmbeddingHost()
	first.ID = 10
	second := reseededEmbeddingHost()
	second.ID = 20

	kept, groups := resolveDuplicateSettings([]domainsettings.SystemSetting{second, first})
	if kept[0].ID != 10 || groups[0].Removed[0].ID != 20 {
		t.Fatalf("expected lowest id to win a tie, kept %d removed %d", kept[0].ID, groups[0].Removed[0].ID)
	}
}

func TestResolveDuplicateSettingsUsesTimestampsForUnregisteredKeys(t *testing.T) {
	edited := domainsettings.SystemSetting{
		ID: 5, Namespace: "llm", Key: "rate_limit.defaults", Value: `{"backoff_base_sec":1}`,
		CreatedAt: configuredAt.Add(-time.Hour), UpdatedAt: configuredAt,
	}
	reseeded := domainsettings.SystemSetting{
		ID: 900, Namespace: "llm", Key: "rate_limit.defaults", Value: `{"backoff_base_sec":5}`,
		CreatedAt: reseededAt, UpdatedAt: reseededAt,
	}

	kept, _ := resolveDuplicateSettings([]domainsettings.SystemSetting{edited, reseeded})
	if kept[0].ID != edited.ID {
		t.Fatalf("expected edited llm row to be kept, got id %d", kept[0].ID)
	}
}

func TestSeedRemovesDuplicatesWithoutReinsertingExistingKeys(t *testing.T) {
	repo := &duplicateSettingsRepo{rows: []domainsettings.SystemSetting{reseededEmbeddingHost(), configuredEmbeddingHost()}}

	if err := NewService(repo, testKeyring()).Seed(context.Background()); err != nil {
		t.Fatalf("seed settings: %v", err)
	}
	if !slices.Equal(repo.removed, []uint{3280}) {
		t.Fatalf("expected reseeded row 3280 to be removed, got %v", repo.removed)
	}
	for _, item := range repo.inserted {
		if item.Namespace == "file" && item.Key == "embedding_host" {
			t.Fatal("seed inserted embedding_host again although it already exists")
		}
	}
	if len(repo.inserted) != len(defaultSettings())-1 {
		t.Fatalf("expected every other default to be seeded, got %d of %d", len(repo.inserted), len(defaultSettings())-1)
	}
}

func TestSeedContinuesWhenDuplicateRepairFails(t *testing.T) {
	repo := &duplicateSettingsRepo{
		rows:      []domainsettings.SystemSetting{configuredEmbeddingHost(), reseededEmbeddingHost()},
		removeErr: errors.New("must be owner of table system_settings"),
	}

	if err := NewService(repo, testKeyring()).Seed(context.Background()); err != nil {
		t.Fatalf("seed must not fail when repair fails: %v", err)
	}
	for _, item := range repo.inserted {
		if item.Namespace == "file" && item.Key == "embedding_host" {
			t.Fatal("seed inserted a third embedding_host row")
		}
	}
}

func TestApplyToUsesConfiguredRowRegardlessOfRowOrder(t *testing.T) {
	enabled := domainsettings.SystemSetting{
		ID: 470, Namespace: "file", Key: "embedding_enabled", Value: "true", ValueType: "bool",
		CreatedAt: configuredAt.Add(-time.Hour), UpdatedAt: configuredAt,
	}
	for name, rows := range map[string][]domainsettings.SystemSetting{
		"configured first": {enabled, configuredEmbeddingHost(), reseededEmbeddingHost()},
		"configured last":  {enabled, reseededEmbeddingHost(), configuredEmbeddingHost()},
	} {
		t.Run(name, func(t *testing.T) {
			runtime := config.NewRuntime(config.Config{})
			repo := &duplicateSettingsRepo{rows: rows}
			if err := NewRuntimeSettings(repo, nil, testKeyring()).ApplyTo(context.Background(), runtime); err != nil {
				t.Fatalf("apply settings: %v", err)
			}
			cfg := runtime.Snapshot()
			if cfg.EmbeddingHost != "http://embedding:8000" || !cfg.EmbeddingEnabled {
				t.Fatalf("expected configured embedding host, got enabled=%v host=%q", cfg.EmbeddingEnabled, cfg.EmbeddingHost)
			}
		})
	}
}

func TestRuntimeValuesByNamespaceUsesConfiguredRow(t *testing.T) {
	repo := &duplicateSettingsRepo{rows: []domainsettings.SystemSetting{configuredEmbeddingHost(), reseededEmbeddingHost()}}

	values, err := NewService(repo, testKeyring()).RuntimeValuesByNamespace(context.Background(), "file")
	if err != nil {
		t.Fatalf("runtime values: %v", err)
	}
	if values["embedding_host"] != "http://embedding:8000" {
		t.Fatalf("expected configured embedding host, got %q", values["embedding_host"])
	}
}
