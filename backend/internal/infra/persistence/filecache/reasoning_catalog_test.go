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

func TestReasoningCatalogCacheRoundTrip(t *testing.T) {
	root := t.TempDir()
	cache := NewReasoningCatalogCache(root)
	ctx := context.Background()

	if _, found, err := cache.Load(ctx); err != nil || found {
		t.Fatalf("expected empty cache, got found=%v err=%v", found, err)
	}
	snapshot := domainchannel.ReasoningCatalogSnapshot{
		Source:    "https://models.dev/api.json",
		FetchedAt: time.Date(2025, 9, 30, 8, 0, 0, 0, time.UTC),
		Entries: []domainchannel.ReasoningCatalogEntry{{
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

	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(reasoningCatalogCacheRelPath)), []byte("corrupt"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, found, err := cache.Load(ctx); err == nil || found {
		t.Fatalf("expected corrupt cache to fail, got found=%v err=%v", found, err)
	}
}
