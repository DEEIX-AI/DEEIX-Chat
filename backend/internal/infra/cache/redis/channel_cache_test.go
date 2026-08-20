package cache

import (
	"context"
	"reflect"
	"testing"
)

func TestIsCircuitStateKey(t *testing.T) {
	tests := map[string]bool{
		"cb:u:1:open":                       true,
		"cb:u:1:until":                      true,
		"cb:u:1:probe":                      true,
		"cb:u:1:fails":                      true,
		"cb:u:1:m:upstream-model-abc:open":  true,
		"cb:u:1:m:upstream-model-abc:fails": true,
		"cb:u:1:last_error":                 false,
		"cb:u:1:last_failure_at":            false,
		"cb:u:1:last_success_at":            false,
	}
	for key, want := range tests {
		if got := isCircuitStateKey(key); got != want {
			t.Fatalf("isCircuitStateKey(%q) = %v, want %v", key, got, want)
		}
	}
}

func TestResetCircuitStateKeysScansAllPagesAndPreservesIndependentKeys(t *testing.T) {
	pages := map[uint64]struct {
		keys []string
		next uint64
	}{
		0: {keys: []string{"cb:u:1:open", "cb:u:1:last_error", "cb:u:1:m:model:fails"}, next: 7},
		7: {keys: []string{"cb:u:2:until", "cb:u:2:last_success_at"}, next: 0},
	}
	deleted := make([]string, 0)
	err := resetCircuitStateKeys(
		t.Context(),
		func(_ context.Context, cursor uint64) ([]string, uint64, error) {
			page := pages[cursor]
			return page.keys, page.next, nil
		},
		func(_ context.Context, keys []string) error {
			deleted = append(deleted, keys...)
			return nil
		},
	)
	if err != nil {
		t.Fatalf("resetCircuitStateKeys() error = %v", err)
	}
	want := []string{"cb:u:1:open", "cb:u:1:m:model:fails", "cb:u:2:until"}
	if !reflect.DeepEqual(deleted, want) {
		t.Fatalf("deleted keys = %#v, want %#v", deleted, want)
	}
}
