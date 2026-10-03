package modelsdev

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/security"
)

// TestBuiltinReasoningSnapshotIsUsable 防止发版时内置快照损坏或误删主流厂商数据。
func TestBuiltinReasoningSnapshotIsUsable(t *testing.T) {
	snapshot, err := BuiltinReasoningSnapshot()
	if err != nil {
		t.Fatalf("decode builtin snapshot: %v", err)
	}
	if len(snapshot.Entries) < 1000 {
		t.Fatalf("builtin snapshot looks truncated: %d entries", len(snapshot.Entries))
	}
	if snapshot.FetchedAt.IsZero() || snapshot.Source == "" {
		t.Fatalf("builtin snapshot misses metadata: %#v", snapshot)
	}
	providers := map[string]bool{}
	for _, entry := range snapshot.Entries {
		providers[entry.Provider] = true
	}
	for _, provider := range []string{"openai", "anthropic", "google"} {
		if !providers[provider] {
			t.Fatalf("builtin snapshot has no %s models", provider)
		}
	}
}

func TestClientFetch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/missing" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	client := New(security.NewStrictOutboundPolicy(false))
	body, err := client.Fetch(context.Background(), server.URL+"/api.json")
	if err != nil || string(body) != `{"ok":true}` {
		t.Fatalf("Fetch() = %q, %v", body, err)
	}
	if _, err := client.Fetch(context.Background(), server.URL+"/missing"); err == nil {
		t.Fatal("expected non-2xx response to fail")
	}
}
