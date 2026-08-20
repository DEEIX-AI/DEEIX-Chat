package memory

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestUserSettingCacheVersionsIsolateStaleValues(t *testing.T) {
	ctx := context.Background()
	cache := New()
	const userID uint = 7
	const key = "chat.file_mode"

	initialVersion, err := cache.GetUserSettingCacheVersion(ctx, userID, key, time.Minute)
	if err != nil {
		t.Fatalf("get initial version: %v", err)
	}
	if err := cache.SetUserSettingCache(ctx, userID, key, initialVersion, "auto", time.Minute); err != nil {
		t.Fatalf("set initial value: %v", err)
	}
	version, err := cache.AdvanceUserSettingCacheVersion(ctx, userID, key, time.Minute)
	if err != nil || version == "" || version == initialVersion {
		t.Fatalf("advanced version = %q (err %v), want a new version", version, err)
	}
	if value, ok, err := cache.GetUserSettingCache(ctx, userID, key, version); err != nil || ok {
		t.Fatalf("new version unexpectedly resolved stale value %q (ok %v, err %v)", value, ok, err)
	}
	if err := cache.SetUserSettingCache(ctx, userID, key, version, "rag", time.Minute); err != nil {
		t.Fatalf("set refreshed value: %v", err)
	}
	if value, ok, err := cache.GetUserSettingCache(ctx, userID, key, version); err != nil || !ok || value != "rag" {
		t.Fatalf("refreshed value = %q (ok %v, err %v), want rag", value, ok, err)
	}
}

func TestUserSettingCacheConcurrentVersionAdvance(t *testing.T) {
	ctx := context.Background()
	cache := New()
	const advances = 100

	var wg sync.WaitGroup
	versions := make(chan string, advances)
	wg.Add(advances)
	for range advances {
		go func() {
			defer wg.Done()
			version, err := cache.AdvanceUserSettingCacheVersion(ctx, 9, "chat.file_mode", time.Minute)
			if err != nil {
				t.Errorf("advance version: %v", err)
				return
			}
			versions <- version
		}()
	}
	wg.Wait()
	close(versions)

	seen := make(map[string]struct{}, advances)
	for version := range versions {
		seen[version] = struct{}{}
	}
	if len(seen) != advances {
		t.Fatalf("unique versions = %d, want %d", len(seen), advances)
	}
	current, err := cache.GetUserSettingCacheVersion(ctx, 9, "chat.file_mode", time.Minute)
	if err != nil {
		t.Fatalf("get current version: %v", err)
	}
	if _, ok := seen[current]; !ok {
		t.Fatalf("current version %q was not produced by an advance", current)
	}
}
