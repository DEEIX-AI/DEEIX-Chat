package objectstorage

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/objectstore"
)

func TestRuntimeProviderRequiresFactory(t *testing.T) {
	provider := NewRuntimeProvider(config.NewRuntime(config.Config{}), nil)
	if _, err := provider.Open(t.Context()); !errors.Is(err, ErrProviderNotConfigured) {
		t.Fatalf("Open() error = %v, want ErrProviderNotConfigured", err)
	}
}

func TestRuntimeProviderCachesStoreUntilStorageConfigChanges(t *testing.T) {
	runtime := config.NewRuntime(config.Config{StorageBackend: objectstore.BackendLocal, StorageRootDir: t.TempDir()})
	var mu sync.Mutex
	factoryCalls := 0
	provider := NewRuntimeProvider(runtime, func(_ context.Context, cfg config.Config) (objectstore.Store, error) {
		mu.Lock()
		factoryCalls++
		mu.Unlock()
		return objectstore.NewLocal(cfg.StorageRootDir), nil
	})

	const workers = 16
	var wait sync.WaitGroup
	wait.Add(workers)
	for range workers {
		go func() {
			defer wait.Done()
			if _, err := provider.Open(t.Context()); err != nil {
				t.Errorf("open cached store: %v", err)
			}
		}()
	}
	wait.Wait()
	if factoryCalls != 1 {
		t.Fatalf("factory calls = %d, want 1", factoryCalls)
	}

	next := runtime.Snapshot()
	next.RateLimitRPM++
	runtime.Store(next)
	if _, err := provider.Open(t.Context()); err != nil {
		t.Fatalf("open after unrelated config change: %v", err)
	}
	if factoryCalls != 1 {
		t.Fatalf("unrelated config change rebuilt store: calls=%d", factoryCalls)
	}

	next.StorageRootDir = t.TempDir()
	runtime.Store(next)
	if _, err := provider.Open(t.Context()); err != nil {
		t.Fatalf("open after storage config change: %v", err)
	}
	if factoryCalls != 2 {
		t.Fatalf("storage config change did not rebuild store: calls=%d", factoryCalls)
	}
}
