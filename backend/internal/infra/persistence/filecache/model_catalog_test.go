package filecache

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	domainchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/channel"
)

func TestModelCatalogCacheRoundTrip(t *testing.T) {
	root := t.TempDir()
	cache := NewModelCatalogCache(root)
	ctx := context.Background()

	if _, found, err := cache.Load(ctx); err != nil || found {
		t.Fatalf("expected empty cache, got found=%v err=%v", found, err)
	}
	snapshot := domainchannel.ModelCatalogSnapshot{
		Source:    "https://models.dev/api.json",
		FetchedAt: time.Date(2025, 9, 30, 8, 0, 0, 0, time.UTC),
		Entries: []domainchannel.ModelCatalogEntry{{
			Provider: "openai", ModelID: "gpt-5", Reasoning: true,
			Options: []domainchannel.ReasoningCatalogOption{{Type: domainchannel.ReasoningCatalogOptionEffort, Values: []string{"low", "high"}}},
		}},
	}
	if err := cache.Store(ctx, snapshot); err != nil {
		t.Fatalf("Store() error = %v", err)
	}
	loaded, found, err := cache.Load(ctx)
	if err != nil || !found || !reflect.DeepEqual(loaded, snapshot) {
		t.Fatalf("Load() = %#v, %v, %v", loaded, found, err)
	}

	legacy := `{"version":1,"models":[{"provider":"openai","id":"gpt-5","options":[{"type":"toggle"}]}]}`
	if err := os.WriteFile(filepath.Join(root, catalogsDir, modelCatalogCacheFile), []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, found, err := cache.Load(ctx); err != nil || found {
		t.Fatalf("expected outdated cache to read as missing, got found=%v err=%v", found, err)
	}

	if err := os.WriteFile(filepath.Join(root, catalogsDir, modelCatalogCacheFile), []byte("corrupt"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, found, err := cache.Load(ctx); err == nil || found {
		t.Fatalf("expected corrupt cache to fail, got found=%v err=%v", found, err)
	}
}

// 旧位置的 v1 缓存会迁移到 storage/catalogs，但因格式过旧被视为缺失，由内置快照重新写入。
func TestModelCatalogCacheMigratesLegacyLocation(t *testing.T) {
	root := t.TempDir()
	legacy := filepath.Join(root, filepath.FromSlash(legacyModelCatalogCacheRelPath))
	if err := os.MkdirAll(filepath.Dir(legacy), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, []byte(`{"version":1,"models":[{"provider":"openai","id":"gpt-5","options":[{"type":"toggle"}]}]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, found, err := NewModelCatalogCache(root).Load(context.Background()); err != nil || found {
		t.Fatalf("expected the outdated legacy cache to read as missing, got found=%v err=%v", found, err)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Fatalf("legacy file must be moved, got %v", err)
	}
}
