package filecache

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenRouterPricingCacheStoreAndLoad(t *testing.T) {
	root := t.TempDir()
	cache := NewOpenRouterPricingCache(root)

	if _, found, err := cache.Load(t.Context()); err != nil || found {
		t.Fatalf("missing cache: found=%v err=%v", found, err)
	}
	payload := []byte(`{"fetchedAt":"2026-08-04T00:00:00Z","items":[{"id":"model"}]}`)
	if err := cache.Store(t.Context(), payload); err != nil {
		t.Fatalf("store cache: %v", err)
	}
	loaded, found, err := cache.Load(t.Context())
	if err != nil || !found {
		t.Fatalf("load cache: found=%v err=%v", found, err)
	}
	if string(loaded) != string(payload) {
		t.Fatalf("loaded payload = %q, want %q", loaded, payload)
	}
	info, err := os.Stat(filepath.Join(root, catalogsDir, openRouterPricingCacheFile))
	if err != nil {
		t.Fatalf("stat cache: %v", err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Fatalf("cache permissions = %o, want 644", info.Mode().Perm())
	}
}

func TestOpenRouterPricingCacheHonorsCanceledContext(t *testing.T) {
	cache := NewOpenRouterPricingCache(t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := cache.Store(ctx, []byte(`{"items":[]}`)); err == nil {
		t.Fatal("expected canceled store to fail")
	}
}

// 升级后旧位置（storage/admin）的已同步定价会迁移到 storage/catalogs，不需要重新联网。
func TestOpenRouterPricingCacheMigratesLegacyLocation(t *testing.T) {
	root := t.TempDir()
	legacy := filepath.Join(root, filepath.FromSlash(legacyOpenRouterPricingCacheRelPath))
	payload := []byte(`{"fetchedAt":"2026-08-04T00:00:00Z","items":[{"id":"model"}]}`)
	if err := os.MkdirAll(filepath.Dir(legacy), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, payload, 0o644); err != nil {
		t.Fatal(err)
	}

	loaded, found, err := NewOpenRouterPricingCache(root).Load(t.Context())
	if err != nil || !found || string(loaded) != string(payload) {
		t.Fatalf("expected the legacy cache to be migrated, got %q found=%v err=%v", loaded, found, err)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Fatalf("legacy file must be moved, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, catalogsDir, openRouterPricingCacheFile)); err != nil {
		t.Fatalf("expected the cache at the new location: %v", err)
	}
}
